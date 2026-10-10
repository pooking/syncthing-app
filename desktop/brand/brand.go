// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Package brand draws OspreySyncTool's mark: a feather, for the osprey the app
// is named after.
//
// The art lives in code rather than in checked-in bitmaps so every place it
// appears — the embedded Windows icon resource, the tray, the window's
// favicon, the README — is drawn from one definition and cannot drift out of
// step. The geometry is authored in a 512x512 space and scaled on demand, so
// the same numbers give a legible 16-pixel tray icon and a crisp 512-pixel app
// icon.
//
// Two forms are drawn from that one geometry:
//
//   - Mark, the bare feather on a transparent square. This is the tray icon and
//     the inline logo in the interface, where the surrounding page supplies the
//     background.
//   - Tile, the same feather in white on a rounded gradient square, as an
//     application icon has to be: at 16 pixels in a taskbar a thin transparent
//     shape has almost no presence, while a solid tile is recognisable at a
//     glance.
//
// Detailed features — the quill down the middle, the gaps between the barbs —
// are dropped below a size threshold rather than shrunk into grey mush.
package brand

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/vector"
)

// canvas is the square the geometry is authored in. Every coordinate below is
// in these units; the drawing functions scale by size/canvas.
const canvas = 512.0

// The feather's skeleton, in canvas units. cx is the axis the whole mark is
// built on; the numbers were settled by rendering and looking, which is the
// only way any of this gets decided.
const (
	cx     = canvas / 2 // the feather stands on the vertical centre line
	tipY   = 54.0       // the point of the feather
	vaneY  = 396.0      // where the vane's edges converge again, onto the quill
	quillY = 456.0      // the bottom end of the bare quill
	maxHW  = 163.0      // the vane's half-width at its widest
)

// vaneEdgeEnd is where the outline stops following the profile. The profile
// reaches zero at 1.0, but sampling right up to it would leave the last segment
// almost parallel to the axis; stopping a shade short and closing with a small
// rounded cap gives the vane a tip that converges cleanly onto the quill.
const vaneEdgeEnd = 0.985

// detailMinSize is the smallest icon that gets the quill and the barb gaps.
// Below it those features are narrower than a pixel and only make the shape
// look noisy.
const detailMinSize = 32

// profileTable is the vane's half-width at each point along its length, as a
// fraction of maxHW: 0 at the point, near 1 at the widest. This is what makes
// the silhouette read as a feather — a long slow taper from a sharp point
// through the widest part at a bit under half way, then narrowing steadily back
// onto the quill. A profile that is symmetric about the middle, or that stays
// wide at the base, reads as a leaf.
//
// The samples are not evenly spaced, which is why vaneHalfWidth interpolates
// with finite-difference tangents rather than a uniform Catmull-Rom.
var profileTable = [][2]float64{
	{0.00, 0.000},
	{0.02, 0.042},
	{0.06, 0.128},
	{0.11, 0.245},
	{0.17, 0.375},
	{0.24, 0.515},
	{0.31, 0.630},
	{0.38, 0.715},
	{0.45, 0.766},
	{0.52, 0.785},
	{0.59, 0.778},
	{0.66, 0.748},
	{0.73, 0.694},
	{0.79, 0.622},
	{0.85, 0.522},
	{0.90, 0.410},
	{0.94, 0.288},
	{0.97, 0.166},
	{1.00, 0.000},
}

// Theme is the pair of colours the mark is drawn with. The vane is filled with
// a vertical ramp and the quill with a flat colour; the flat colour is what
// keeps the quill readable on top of the vane without outlining it.
type Theme struct {
	Vane  []color.NRGBA // sampled from the point (first) to the base (last)
	Quill color.NRGBA
}

// DefaultTheme is the mark as it appears in the interface: the same blue as the
// UI's accent, so the icon and the window belong to each other.
func DefaultTheme() Theme {
	return Theme{
		Vane: []color.NRGBA{
			{R: 0x7d, G: 0xbb, B: 0xf4, A: 0xff}, // lighter than the accent, so the point does not read as flat
			{R: 0x2b, G: 0x7c, B: 0xd6, A: 0xff}, // --accent
			{R: 0x1a, G: 0x5c, B: 0xa4, A: 0xff}, // darker, to ground the base
		},
		Quill: color.NRGBA{R: 0x16, G: 0x4b, B: 0x88, A: 0xff},
	}
}

// TileTheme is the app icon: a white feather on the accent gradient.
func TileTheme() (fg Theme, bg color.NRGBA) {
	return Theme{
		Vane:  []color.NRGBA{{R: 0xff, G: 0xff, B: 0xff, A: 0xff}},
		Quill: color.NRGBA{R: 0xd6, G: 0xe8, B: 0xfb, A: 0xff},
	}, color.NRGBA{R: 0x2b, G: 0x7c, B: 0xd6, A: 0xff}
}

// SolidTheme builds a single-hue theme, used by the tray so one shape carries
// the status colour. The vane ramps from a lightened to a darkened version of c
// and the quill sits well below it, which is the contrast DefaultTheme gets
// from three separate colours.
func SolidTheme(c color.NRGBA) Theme {
	return Theme{
		Vane: []color.NRGBA{
			blend(c, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, 0.26),
			c,
			blend(c, color.NRGBA{A: 0xff}, 0.20),
		},
		Quill: blend(c, color.NRGBA{A: 0xff}, 0.50),
	}
}

// blend mixes a towards b by t (0 keeps a, 1 returns b), alpha aside.
func blend(a, b color.NRGBA, t float64) color.NRGBA {
	mix := func(x, y uint8) uint8 {
		return uint8(math.Round(float64(x)*(1-t) + float64(y)*t))
	}
	return color.NRGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 0xff}
}

// ---- Public images --------------------------------------------------------

// Mark draws the bare feather at the given size on a transparent square.
func Mark(size int, th Theme) *image.NRGBA {
	if size < 1 {
		size = 1
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	drawFeather(img, size, th, float64(size)/canvas, 0, 0)
	return img
}

// inset is how much of the tile the feather does not use, as a fraction of the
// tile's side on each edge. A fifth of the width is what leaves the rounded
// corners readable while keeping the feather as large as it can be.
const inset = 0.13

// Tile draws the app icon: the feather in white, centred on a rounded square
// filled with a diagonal gradient from light to dark.
//
// The feather is drawn straight into the tile's own rasterizer at the tile's
// own resolution rather than rendered small and pasted in, so the two shapes
// share one antialiasing pass. Pasted, the feather's edge picks up a second
// round of coverage on top of the tile's and the seam between them is visible
// at small sizes as a bright fringe.
func Tile(size int) *image.NRGBA {
	if size < 1 {
		size = 1
	}
	fg, _ := TileTheme()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	r := vector.NewRasterizer(size, size)
	tilePath().rasterize(r, float64(size)/canvas)
	r.Draw(img, img.Bounds(), &ramp{
		stops:  []color.NRGBA{{R: 0x4a, G: 0x9a, B: 0xe8, A: 0xff}, {R: 0x1d, G: 0x63, B: 0xb4, A: 0xff}},
		size:   size,
		linear: true,
	}, image.Point{})

	// The feather is laid over a feathered copy of itself rather than drawn
	// directly into the tile: the white has to arrive with its own alpha, so
	// that the tile's gradient stays visible wherever the feather is less than
	// fully covered.
	inner := int(math.Round(float64(size) * (1 - 2*inset)))
	if inner < 1 {
		inner = 1
	}
	off := (size - inner) / 2
	feather := image.NewNRGBA(image.Rect(0, 0, inner, inner))
	drawFeather(feather, inner, fg, float64(inner)/canvas, 0, 0)
	draw.Draw(img, image.Rect(off, off, off+inner, off+inner), feather, image.Point{}, draw.Over)
	return img
}

// PNG encodes Mark at the given size.
func PNG(size int, th Theme) []byte { return encode(Mark(size, th)) }

func encode(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// TileImages returns the app icon rendered at each size, for a packer that
// wants a set of images rather than a finished .ico.
func TileImages(sizes []int) []image.Image {
	imgs := make([]image.Image, 0, len(sizes))
	for _, size := range sizes {
		imgs = append(imgs, Tile(size))
	}
	return imgs
}

// ICOSizes are the sizes baked into the executable's icon, smallest first.
// Windows picks the one that matches the view: 16 for the title bar and the
// taskbar's small mode, 256 for the extra-large tiles.
var ICOSizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

// TraySizes are the sizes worth having in a tray icon: the shell scales one
// down for a 100% display and picks a larger one on a scaled display, where a
// 16-pixel image would come out soft.
var TraySizes = []int{16, 20, 24, 32, 48}

// ICO builds a multi-size Windows icon, rendering each size with the given
// function. Each entry is PNG-compressed, which every Windows since Vista
// reads; winres and Wails' own packager write icons the same way.
func ICO(sizes []int, render func(int) image.Image) []byte {
	type entry struct {
		size int
		data []byte
	}
	var entries []entry
	for _, size := range sizes {
		if data := encode(render(size)); data != nil {
			entries = append(entries, entry{size, data})
		}
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(&buf, binary.LittleEndian, uint16(len(entries)))

	offset := 6 + 16*len(entries)
	for _, e := range entries {
		dim := byte(e.size)
		if e.size >= 256 {
			dim = 0 // 0 means 256 in an icon directory
		}
		buf.Write([]byte{dim, dim, 0, 0})
		binary.Write(&buf, binary.LittleEndian, uint16(1))           // colour planes
		binary.Write(&buf, binary.LittleEndian, uint16(32))          // bits per pixel
		binary.Write(&buf, binary.LittleEndian, uint32(len(e.data))) // payload size
		binary.Write(&buf, binary.LittleEndian, uint32(offset))      // payload offset
		offset += len(e.data)
	}
	for _, e := range entries {
		buf.Write(e.data)
	}
	return buf.Bytes()
}

// SVG renders the bare mark as a standalone SVG document, for the interface and
// for looking at the art in a browser.
func SVG(size int, th Theme) string {
	var b bytes.Buffer
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="` + strconv.Itoa(size) +
		`" height="` + strconv.Itoa(size) + `" viewBox="0 0 512 512">`)

	// The vane's gradient runs over the feather, not over the whole square, so
	// the point and the base land on the sampled colours.
	b.WriteString(`<defs><linearGradient id="v" gradientUnits="userSpaceOnUse" x1="0" y1="` +
		trim(tipY) + `" x2="0" y2="` + trim(vaneY) + `">`)
	for i, c := range th.Vane {
		offset := 0.0
		if len(th.Vane) > 1 {
			offset = float64(i) / float64(len(th.Vane)-1)
		}
		b.WriteString(`<stop offset="` + trim(offset*100) + `%" stop-color="` + hex(c) + `"/>`)
	}
	b.WriteString(`</linearGradient>`)
	b.WriteString(`<mask id="notches"><rect x="-8" y="-8" width="528" height="528" fill="#fff"/>` +
		`<path fill="#000" d="` + notchPath().d() + `"/></mask>`)
	b.WriteString(`</defs>`)

	b.WriteString(`<g mask="url(#notches)"><path fill="url(#v)" d="` + vanePath().d() + `"/></g>`)
	b.WriteString(`<path fill="` + hex(th.Quill) + `" d="` + quillPath().d() + `"/>`)
	b.WriteString(`</svg>`)
	return b.String()
}

// ---- Drawing --------------------------------------------------------------

// drawFeather renders the feather into img. The scale/offset arguments place it
// so Tile can draw it inside a smaller square without a second rasterizer pass.
func drawFeather(img *image.NRGBA, size int, th Theme, scale, offX, offY float64) {
	detailed := size >= detailMinSize

	r := vector.NewRasterizer(size, size)
	vanePath().rasterizeAt(r, scale, offX, offY)
	r.Draw(img, img.Bounds(), &ramp{stops: th.Vane, size: size}, image.Point{})

	if !detailed {
		return
	}

	r.Reset(size, size)
	quillPath().rasterizeAt(r, scale, offX, offY)
	r.Draw(img, img.Bounds(), image.NewUniform(th.Quill), image.Point{})

	// The barb gaps are rasterised into an alpha mask and multiplied into the
	// mark, rather than folded into the vane's outline: the gaps overshoot the
	// edge, so they stay the shape they were designed as instead of taking
	// whatever angle the outline happens to have there.
	mask := image.NewAlpha(img.Bounds())
	r.Reset(size, size)
	r.DrawOp = draw.Src
	notchPath().rasterizeAt(r, scale, offX, offY)
	r.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: 0xff}), image.Point{})

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if a := mask.AlphaAt(x, y).A; a > 0 {
				i := img.PixOffset(x, y)
				img.Pix[i+3] = uint8(int(img.Pix[i+3]) * (255 - int(a)) / 255)
			}
		}
	}
}

// ---- Geometry -------------------------------------------------------------

// vertex is a point in the canvas space.
type vertex struct{ x, y float64 }

type seg struct {
	op byte // 'M' move, 'L' line, 'C' cube, 'Z' close
	a  vertex
	b  vertex
	c  vertex
}

type path []seg

func (p *path) moveTo(a vertex)       { *p = append(*p, seg{op: 'M', a: a}) }
func (p *path) lineTo(a vertex)       { *p = append(*p, seg{op: 'L', a: a}) }
func (p *path) cubeTo(a, b, c vertex) { *p = append(*p, seg{op: 'C', a: a, b: b, c: c}) }
func (p *path) close()                { *p = append(*p, seg{op: 'Z'}) }

func (p path) rasterize(r *vector.Rasterizer, scale float64) {
	p.rasterizeAt(r, scale, 0, 0)
}

func (p path) rasterizeAt(r *vector.Rasterizer, scale, offX, offY float64) {
	pt := func(v vertex) (float32, float32) {
		return float32(v.x*scale + offX), float32(v.y*scale + offY)
	}
	for _, sg := range p {
		switch sg.op {
		case 'M':
			x, y := pt(sg.a)
			r.MoveTo(x, y)
		case 'L':
			x, y := pt(sg.a)
			r.LineTo(x, y)
		case 'C':
			ax, ay := pt(sg.a)
			bx, by := pt(sg.b)
			cxx, cyy := pt(sg.c)
			r.CubeTo(ax, ay, bx, by, cxx, cyy)
		case 'Z':
			r.ClosePath()
		}
	}
}

// d renders the path as an SVG path data attribute.
func (p path) d() string {
	var b bytes.Buffer
	for _, sg := range p {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		f := func(v vertex) string { return trim(v.x) + " " + trim(v.y) }
		switch sg.op {
		case 'M':
			b.WriteString("M" + f(sg.a))
		case 'L':
			b.WriteString("L" + f(sg.a))
		case 'C':
			b.WriteString("C" + f(sg.a) + " " + f(sg.b) + " " + f(sg.c))
		case 'Z':
			b.WriteString("Z")
		}
	}
	return b.String()
}

// vaneHalfWidth interpolates the profile table with cubic Hermite segments,
// using finite-difference tangents so the uneven sample spacing does not show
// up as corners.
func vaneHalfWidth(u float64) float64 {
	if u <= 0 || u >= 1 {
		return 0
	}
	i := 0
	for i < len(profileTable)-2 && profileTable[i+1][0] < u {
		i++
	}
	u0, f0 := profileTable[i][0], profileTable[i][1]
	u1, f1 := profileTable[i+1][0], profileTable[i+1][1]

	slope := func(k int) float64 {
		if k == 0 {
			return (profileTable[1][1] - profileTable[0][1]) / (profileTable[1][0] - profileTable[0][0])
		}
		if k == len(profileTable)-1 {
			n := len(profileTable)
			return (profileTable[n-1][1] - profileTable[n-2][1]) / (profileTable[n-1][0] - profileTable[n-2][0])
		}
		return (profileTable[k+1][1] - profileTable[k-1][1]) / (profileTable[k+1][0] - profileTable[k-1][0])
	}
	m0, m1 := slope(i), slope(i+1)

	h := u1 - u0
	t := (u - u0) / h
	t2, t3 := t*t, t*t*t
	return (2*t3-3*t2+1)*f0 + (t3-2*t2+t)*h*m0 + (-2*t3+3*t2)*f1 + (t3-t2)*h*m1
}

// vaneAt maps a profile position to a point on the feather's axis: its y, and
// the vane's half-width there.
func vaneAt(u float64) (y, halfWidth float64) {
	return tipY + u*(vaneY-tipY), maxHW * vaneHalfWidth(u)
}

// vaneEdgeSamples returns the right-hand edge of the vane, from beside the point
// down to where the outline converges on the quill.
func vaneEdgeSamples() []vertex {
	const samples = 150
	edge := make([]vertex, 0, samples)
	for i := 1; i <= samples; i++ {
		y, hw := vaneAt(vaneEdgeEnd * float64(i) / samples)
		edge = append(edge, vertex{cx + hw, y})
	}
	return edge
}

// vanePath is the closed outline of the vane: down the right side from the
// point, across a narrow rounded base, and back up the left side.
func vanePath() path {
	edge := vaneEdgeSamples()
	last := edge[len(edge)-1]
	baseHW := last.x - cx

	var p path
	p.moveTo(vertex{cx, tipY})
	for _, v := range edge {
		p.lineTo(v)
	}
	p.cubeTo(
		vertex{cx + baseHW*0.62, last.y + (vaneY - last.y) + 14},
		vertex{cx + baseHW*0.26, vaneY + 16},
		vertex{cx, vaneY + 16},
	)
	p.cubeTo(
		vertex{cx - baseHW*0.26, vaneY + 16},
		vertex{cx - baseHW*0.62, last.y + (vaneY - last.y) + 14},
		vertex{cx - baseHW, last.y},
	)
	for i := len(edge) - 2; i >= 0; i-- {
		p.lineTo(vertex{2*cx - edge[i].x, edge[i].y})
	}
	p.close()
	return p
}

// quillPath is the bare stalk: the rachis running the length of the feather,
// drawn on top of the vane, and the quill sticking out below it.
func quillPath() path {
	const (
		r     = 9.0
		top   = tipY + 22
		kappa = 0.5522847498 // control-point ratio for a quarter-circle cubic
	)

	var p path
	p.moveTo(vertex{cx - r, top})
	p.lineTo(vertex{cx - r, quillY - r})
	p.cubeTo( // round cap at the bottom
		vertex{cx - r, quillY - r + r*kappa},
		vertex{cx - r*kappa, quillY},
		vertex{cx, quillY},
	)
	p.cubeTo(
		vertex{cx + r*kappa, quillY},
		vertex{cx + r, quillY - r + r*kappa},
		vertex{cx + r, quillY - r},
	)
	p.lineTo(vertex{cx + r, top})
	p.cubeTo( // round cap at the top, hidden inside the vane
		vertex{cx + r, top - r + r*kappa},
		vertex{cx + r*kappa, top - r},
		vertex{cx, top - r},
	)
	p.cubeTo(
		vertex{cx - r*kappa, top - r},
		vertex{cx - r, top - r + r*kappa},
		vertex{cx - r, top},
	)
	p.close()
	return p
}

// notchCenters are the profile positions of the gaps between the barbs. They
// run down both edges of the vane, and they are what stop the shape reading as
// one smooth leaf.
var notchCenters = []float64{0.26, 0.42, 0.58, 0.73}

const (
	notchDepth = 30.0 // how far into the vane a gap cuts, in canvas units
	notchHalf  = 0.055
	notchSkew  = 0.018 // the gap leans this far down the feather, as barbs do
)

// notchPath is the union of the barb gaps: wedges that reach out past the
// vane's edge and taper inwards to a point.
func notchPath() path {
	var p path
	for _, u := range notchCenters {
		_, hw := vaneAt(u)
		outer := hw + 60
		// The gap is a wedge: full depth where it opens onto the edge, closing
		// to a point inside the vane. Cutting it with a straight hypotenuse
		// instead would leave a triangle with the blunt side against the
		// outline, which reads as a bite taken out rather than a part.
		inner := hw - notchDepth
		y0 := tipY + (u-notchHalf)*(vaneY-tipY)
		y1 := tipY + (u+notchHalf)*(vaneY-tipY)
		ym := tipY + (u+notchSkew)*(vaneY-tipY)

		p.moveTo(vertex{cx + outer, y0})
		p.lineTo(vertex{cx + inner, ym})
		p.lineTo(vertex{cx + outer, y1})
		p.close()

		p.moveTo(vertex{cx - outer, y0})
		p.lineTo(vertex{cx - inner, ym})
		p.lineTo(vertex{cx - outer, y1})
		p.close()
	}
	return p
}

// tilePath is a rounded square covering the whole canvas, with corners rounded
// in the proportion a modern application icon uses.
func tilePath() path {
	const (
		radius = canvas * 0.225
		kappa  = 0.5522847498
	)
	r := radius
	k := radius * kappa
	const (
		l = 0
		t = 0
		w = canvas
		b = canvas
	)

	var p path
	p.moveTo(vertex{l + r, t})
	p.lineTo(vertex{w - r, t})
	p.cubeTo(vertex{w - r + k, t}, vertex{w, t + r - k}, vertex{w, t + r})
	p.lineTo(vertex{w, b - r})
	p.cubeTo(vertex{w, b - r + k}, vertex{w - r + k, b}, vertex{w - r, b})
	p.lineTo(vertex{l + r, b})
	p.cubeTo(vertex{l + r - k, b}, vertex{l, b - r + k}, vertex{l, b - r})
	p.lineTo(vertex{l, t + r})
	p.cubeTo(vertex{l, t + r - k}, vertex{l + r - k, t}, vertex{l + r, t})
	p.close()
	return p
}

// ---- Fills ----------------------------------------------------------------

// ramp is a colour ramp used as a fill source. It is an image.Image rather than
// a backing bitmap, so the rasterizer samples it per pixel without allocating a
// full-size buffer.
type ramp struct {
	stops []color.NRGBA
	size  int
	// linear makes the ramp run diagonally rather than straight down. It is
	// only used for the tile, where a diagonal reads as light falling on the
	// top-left corner.
	linear bool
}

func (g *ramp) ColorModel() color.Model { return color.NRGBAModel }
func (g *ramp) Bounds() image.Rectangle { return image.Rect(0, 0, g.size, g.size) }

func (g *ramp) At(x, y int) color.Color {
	if len(g.stops) == 0 {
		return color.NRGBA{}
	}
	if len(g.stops) == 1 {
		return g.stops[0]
	}
	t := float64(y) / float64(max(g.size-1, 1))
	if g.linear {
		t = (float64(x) + float64(y)) / float64(max(2*(g.size-1), 1))
	}
	t *= float64(len(g.stops) - 1)

	i := int(t)
	if i >= len(g.stops)-1 {
		return g.stops[len(g.stops)-1]
	}
	f := t - float64(i)
	a, b := g.stops[i], g.stops[i+1]
	mix := func(x, y uint8) uint8 {
		return uint8(math.Round(float64(x)*(1-f) + float64(y)*f))
	}
	return color.NRGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 0xff}
}

// trim formats a coordinate with at most two decimals and no trailing zeros,
// which keeps the generated SVG readable.
func trim(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// hex formats a colour as #rrggbb.
func hex(c color.NRGBA) string {
	const digits = "0123456789abcdef"
	out := []byte{'#', 0, 0, 0, 0, 0, 0}
	for i, b := range []uint8{c.R, c.G, c.B} {
		out[1+i*2] = digits[b>>4]
		out[2+i*2] = digits[b&0x0f]
	}
	return string(out)
}
