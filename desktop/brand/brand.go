// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Package brand draws OspreySyncTool's mark: an owl, in flat colour.
//
// The art lives in code rather than in checked-in bitmaps so every place it
// appears — the embedded Windows icon resource, the tray, the window's header,
// the README — is drawn from one definition and cannot drift out of step. The
// geometry is authored in a 512x512 space and scaled on demand, so the same
// numbers give a legible 16-pixel tray icon and a crisp 512-pixel app icon.
//
// Two forms are drawn from that one geometry:
//
//   - Mark, the owl on a transparent square: ink, for the tray and for the
//     logo in the interface, where the surrounding page supplies the
//     background.
//   - Tile, the same owl in white on a rounded blue square, as an application
//     icon has to be: at 16 pixels in a taskbar a thin shape on nothing has
//     almost no presence, while a solid tile is recognisable at a glance.
//
// Everything is a filled shape, and the face is cut out of the body rather than
// drawn on top of it: an eye is a hole, so whatever is behind the owl shows
// through. That is what makes one drawing work as white-on-blue in the taskbar,
// as ink on a light tray, and as ink on a dark one, with no second colour to
// pick.
//
// The belly marks and the branch are dropped below a size threshold rather than
// shrunk into grey mush.
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

// The owl's skeleton, in canvas units. The numbers were settled by rendering
// and looking, which is the only way this kind of shape gets decided.
const (
	cx      = canvas / 2 // the axis the owl stands on
	headTop = 100.0      // the apex of the head, between the ear tufts
	bodyBot = 412.0      // the bottom of the body, where the feet hang
	bodyHW  = 146.0      // the body's half-width at its widest
	eyeY    = 232.0      // the centres of the eyes
	eyeDX   = 62.0       // how far each eye sits off the axis
	eyeR    = 54.0       // the eyes' radius
	beakY   = 296.0      // the top of the beak
)

// detailMinSize is the smallest icon that gets the branch and the belly marks.
// Below it those features are narrower than a pixel and only make the shape
// look noisy.
const detailMinSize = 32

// Theme is the colour the owl is drawn in. There is only one: the face is cut
// out of the body, so the eyes take the colour of whatever the owl sits on.
type Theme struct {
	Body []color.NRGBA // sampled from the head (first) to the feet (last)
}

// DefaultTheme is the owl as it appears in the interface: the accent blue.
func DefaultTheme() Theme {
	return Theme{
		Body: []color.NRGBA{
			{R: 0x4a, G: 0x9a, B: 0xe8, A: 0xff}, // lighter than the accent, so the head does not read as flat
			{R: 0x2b, G: 0x7c, B: 0xd6, A: 0xff}, // --accent
			{R: 0x1d, G: 0x63, B: 0xb4, A: 0xff}, // darker, to ground the feet
		},
	}
}

// TileTheme is the app icon: the owl in white on the accent gradient.
func TileTheme() Theme {
	return Theme{Body: []color.NRGBA{{R: 0xff, G: 0xff, B: 0xff, A: 0xff}}}
}

// SolidTheme builds a single-colour owl, used by the tray so one shape carries
// the status colour.
func SolidTheme(c color.NRGBA) Theme {
	return Theme{Body: []color.NRGBA{c}}
}

// ---- Public images --------------------------------------------------------

// Mark draws the owl at the given size on a transparent square.
func Mark(size int, th Theme) *image.NRGBA {
	if size < 1 {
		size = 1
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	drawOwl(img, size, th, float64(size)/canvas, 0, 0)
	return img
}

// inset is how much of the tile the owl does not use, as a fraction of the
// tile's side on each edge. Enough to leave the rounded corners readable, and
// no more: the owl is the thing being recognised, and every pixel of margin is
// a pixel it does not get.
const inset = 0.12

// Tile draws the app icon: the owl in white, centred on a rounded square filled
// with a diagonal gradient from light to dark.
//
// The owl is drawn into the tile's own rasterizer rather than rendered small
// and pasted in, so the two shapes share one antialiasing pass. Pasted, the
// owl's edge picks up a second round of coverage on top of the tile's and the
// seam between them is visible at small sizes as a bright fringe.
func Tile(size int) *image.NRGBA {
	if size < 1 {
		size = 1
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	r := vector.NewRasterizer(size, size)
	tilePath().rasterize(r, float64(size)/canvas)
	r.Draw(img, img.Bounds(), &ramp{
		stops:  []color.NRGBA{{R: 0x4a, G: 0x9a, B: 0xe8, A: 0xff}, {R: 0x1d, G: 0x63, B: 0xb4, A: 0xff}},
		size:   size,
		linear: true,
	}, image.Point{})

	// The owl is composited over the tile as its own layer, so the face it has
	// cut out of itself shows the tile's gradient rather than the owl's colour.
	inner := int(math.Round(float64(size) * (1 - 2*inset)))
	if inner < 1 {
		inner = 1
	}
	off := (size - inner) / 2
	owl := image.NewNRGBA(image.Rect(0, 0, inner, inner))
	drawOwl(owl, inner, TileTheme(), float64(inner)/canvas, 0, 0)
	draw.Draw(img, image.Rect(off, off, off+inner, off+inner), owl, image.Point{}, draw.Over)
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

// SVG renders the owl as a standalone SVG document, for the interface and for
// looking at the art in a browser.
func SVG(size int, th Theme) string {
	var b bytes.Buffer
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="` + strconv.Itoa(size) +
		`" height="` + strconv.Itoa(size) + `" viewBox="0 0 512 512">`)

	// The body's gradient runs over the owl, not over the whole square, so the
	// head and the feet land on the sampled colours.
	b.WriteString(`<defs><linearGradient id="b" gradientUnits="userSpaceOnUse" x1="0" y1="` +
		trim(headTop) + `" x2="0" y2="` + trim(bodyBot) + `">`)
	for i, c := range th.Body {
		offset := 0.0
		if len(th.Body) > 1 {
			offset = float64(i) / float64(len(th.Body)-1)
		}
		b.WriteString(`<stop offset="` + trim(offset*100) + `%" stop-color="` + hex(c) + `"/>`)
	}
	b.WriteString(`</linearGradient>`)

	// The face is a mask, not a fill: an SVG has no "cut this out of that", so
	// the same subpaths that are punched out of the bitmap are painted black
	// into a white mask here.
	b.WriteString(`<mask id="face"><rect x="-16" y="-16" width="544" height="544" fill="#fff"/>`)
	for _, p := range facePaths(true) {
		b.WriteString(`<path fill="#000" d="` + p.d() + `"/>`)
	}
	b.WriteString(`</mask></defs>`)

	b.WriteString(`<g mask="url(#face)">`)
	b.WriteString(`<path fill="url(#b)" d="` + owlPath().d() + `"/>`)
	b.WriteString(`</g></svg>`)
	return b.String()
}

// ---- Drawing --------------------------------------------------------------

// drawOwl renders the owl into img, scaled out of the canvas space.
func drawOwl(img *image.NRGBA, size int, th Theme, scale, offX, offY float64) {
	body := image.NewNRGBA(img.Bounds())
	r := vector.NewRasterizer(size, size)
	owlPath().rasterizeAt(r, scale, offX, offY)
	r.Draw(body, body.Bounds(), &ramp{stops: th.Body, size: size}, image.Point{})
	// The face is rasterised into an alpha mask and multiplied into the owl
	// rather than folded into its outline: an eye is a hole, so the alpha has
	// to come off, and doing it here means the same geometry serves the tile,
	// the tray and the SVG.
	//
	// The eyes are never dropped, at any size. They are the whole reason the
	// shape reads as an owl; without them it is a blob with two ears.
	mask := image.NewAlpha(img.Bounds())
	r.Reset(size, size)
	r.DrawOp = draw.Src
	for _, p := range facePaths(size >= detailMinSize) {
		p.rasterizeAt(r, scale, offX, offY)
	}
	r.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: 0xff}), image.Point{})

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if a := mask.AlphaAt(x, y).A; a > 0 {
				i := body.PixOffset(x, y)
				body.Pix[i+3] = uint8(int(body.Pix[i+3]) * (255 - int(a)) / 255)
			}
		}
	}

	draw.Draw(img, img.Bounds(), body, image.Point{}, draw.Over)
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

// kappa is the control-point ratio that turns four cubics into a circle.
const kappa = 0.5522847498

// circle appends a closed circle. Some helpers below take it reversed, to punch
// it out of a shape rather than add to it.
func (p *path) circle(c vertex, r float64) {
	p.moveTo(vertex{c.x, c.y - r})
	p.cubeTo(
		vertex{c.x + r*kappa, c.y - r},
		vertex{c.x + r, c.y - r*kappa},
		vertex{c.x + r, c.y},
	)
	p.cubeTo(
		vertex{c.x + r, c.y + r*kappa},
		vertex{c.x + r*kappa, c.y + r},
		vertex{c.x, c.y + r},
	)
	p.cubeTo(
		vertex{c.x - r*kappa, c.y + r},
		vertex{c.x - r, c.y + r*kappa},
		vertex{c.x - r, c.y},
	)
	p.cubeTo(
		vertex{c.x - r, c.y - r*kappa},
		vertex{c.x - r*kappa, c.y - r},
		vertex{c.x, c.y - r},
	)
	p.close()
}

// circleReversed appends a circle wound the other way round. Inside a shape
// that is already wound clockwise the rasteriser's non-zero rule cancels the
// two, so the circle becomes a hole rather than a fill — which is how a pupil
// is drawn back into an eye that is itself a hole.
func (p *path) circleReversed(c vertex, r float64) {
	p.moveTo(vertex{c.x, c.y - r})
	p.cubeTo(
		vertex{c.x - r*kappa, c.y - r},
		vertex{c.x - r, c.y - r*kappa},
		vertex{c.x - r, c.y},
	)
	p.cubeTo(
		vertex{c.x - r, c.y + r*kappa},
		vertex{c.x - r*kappa, c.y + r},
		vertex{c.x, c.y + r},
	)
	p.cubeTo(
		vertex{c.x + r*kappa, c.y + r},
		vertex{c.x + r, c.y + r*kappa},
		vertex{c.x + r, c.y},
	)
	p.cubeTo(
		vertex{c.x + r, c.y - r*kappa},
		vertex{c.x + r*kappa, c.y - r},
		vertex{c.x, c.y - r},
	)
	p.close()
}

// owlPath is the owl's silhouette: the ear tufts, the head, the body and the
// two feet, as one closed shape. One outline rather than several overlapping
// ones, because overlapping fills show their seams wherever antialiasing meets.
func owlPath() path {
	const (
		shoulderY = 172.0 // where the head's arc gives way to the body's
		shoulderW = 116.0 // the body's half-width at the shoulders
		waistY    = 330.0 // where the body is at its widest
		soleY     = 412.0 // the body's flat underside, which the feet hang from
		soleX     = 92.0  // how far the underside runs before the feet
		toeX      = 70.0  // the feet, outer edge
		toeInX    = 30.0  // the feet, inner edge
		toeY      = 440.0 // the tips of the feet
		tuftX     = 126.0 // how far the ear tufts reach
		tuftY     = 70.0  // the tips of the ear tufts
	)

	var p path
	p.moveTo(vertex{cx - 92, headTop + 34})

	// Left ear tuft: out and up, then back in to the apex of the head.
	p.lineTo(vertex{cx - tuftX, tuftY})
	p.lineTo(vertex{cx - 34, headTop + 6})

	// The apex, and the right tuft.
	p.lineTo(vertex{cx, headTop})
	p.lineTo(vertex{cx + 34, headTop + 6})
	p.lineTo(vertex{cx + tuftX, tuftY})
	p.lineTo(vertex{cx + 92, headTop + 34})

	// Right side: over the cranium into the shoulder, then down the body.
	p.cubeTo(
		vertex{cx + 122, headTop + 40},
		vertex{cx + shoulderW, shoulderY - 40},
		vertex{cx + shoulderW, shoulderY},
	)
	p.cubeTo(
		vertex{cx + shoulderW + 8, 254},
		vertex{cx + bodyHW, 288},
		vertex{cx + bodyHW, waistY},
	)
	p.cubeTo(
		vertex{cx + bodyHW, 366},
		vertex{cx + 138, soleY - 4},
		vertex{cx + soleX, soleY},
	)

	// The right foot, hanging below the flat underside.
	p.lineTo(vertex{cx + toeX, soleY + 4})
	p.lineTo(vertex{cx + toeX, toeY - 4})
	p.cubeTo(
		vertex{cx + toeX, toeY + 2},
		vertex{cx + toeX - 8, toeY + 6},
		vertex{cx + toeX - 16, toeY + 6},
	)
	p.lineTo(vertex{cx + toeInX + 12, toeY + 6})
	p.cubeTo(
		vertex{cx + toeInX + 2, toeY + 4},
		vertex{cx + toeInX, toeY - 4},
		vertex{cx + toeInX, soleY + 4},
	)

	// The inner edge of the left foot, and the left foot.
	p.lineTo(vertex{cx - toeInX, soleY + 4})
	p.lineTo(vertex{cx - toeInX, toeY - 4})
	p.cubeTo(
		vertex{cx - toeInX, toeY + 2},
		vertex{cx - toeInX - 8, toeY + 6},
		vertex{cx - toeInX - 12, toeY + 6},
	)
	p.lineTo(vertex{cx - toeX + 16, toeY + 6})
	p.cubeTo(
		vertex{cx - toeX + 8, toeY + 2},
		vertex{cx - toeX, toeY - 4},
		vertex{cx - toeX, soleY + 4},
	)
	p.lineTo(vertex{cx - soleX, soleY})

	// Up the left side and back over the head to where the path started.
	p.cubeTo(
		vertex{cx - 138, soleY - 4},
		vertex{cx - bodyHW, 366},
		vertex{cx - bodyHW, waistY},
	)
	p.cubeTo(
		vertex{cx - bodyHW, 288},
		vertex{cx - shoulderW - 8, 254},
		vertex{cx - shoulderW, shoulderY},
	)
	p.cubeTo(
		vertex{cx - shoulderW, shoulderY - 40},
		vertex{cx - 122, headTop + 40},
		vertex{cx - 92, headTop + 34},
	)
	p.close()
	return p
}

// facePaths are the holes cut out of the owl: the eyes, the beak, and the belly
// marks. They are subtracted rather than drawn, so they take the colour of
// whatever the owl is placed on.
func facePaths(detailed bool) []path {
	var out []path

	// The eyes, as two large circles. Big and close together is what makes a
	// bird read as an owl rather than as a generic round bird.
	//
	// The pupils are wound the other way round inside the same path, so they
	// cancel out of it and stay part of the owl rather than part of the hole.
	// Below the detail threshold a pupil is narrower than a pixel and would
	// only soften the eye's edge, so the eye stays a plain hole there.
	var eyes path
	eyes.circle(vertex{cx - eyeDX, eyeY}, eyeR)
	eyes.circle(vertex{cx + eyeDX, eyeY}, eyeR)
	if detailed {
		eyes.circleReversed(vertex{cx - eyeDX, eyeY}, eyeR*0.46)
		eyes.circleReversed(vertex{cx + eyeDX, eyeY}, eyeR*0.46)
	}
	out = append(out, eyes)

	// The beak: a short downward triangle between and below the eyes.
	var beak path
	beak.moveTo(vertex{cx, beakY + 44})
	beak.lineTo(vertex{cx - 22, beakY})
	beak.lineTo(vertex{cx + 22, beakY})
	beak.close()
	out = append(out, beak)

	// The belly, as short chevrons: enough to say "feathers" without drawing a
	// texture that would turn to noise at icon sizes.
	if detailed {
		for _, row := range []struct{ y, hw, drop float64 }{
			{354, 56, 20},
			{396, 42, 16},
		} {
			var v path
			v.moveTo(vertex{cx - row.hw, row.y})
			v.lineTo(vertex{cx, row.y + row.drop})
			v.lineTo(vertex{cx + row.hw, row.y})
			v.lineTo(vertex{cx + row.hw - 18, row.y - 13})
			v.lineTo(vertex{cx, row.y + row.drop - 13})
			v.lineTo(vertex{cx - row.hw + 18, row.y - 13})
			v.close()
			out = append(out, v)
		}
	}

	return out
}

// tilePath is a rounded square covering the whole canvas, with corners rounded
// in the proportion a modern application icon uses.
func tilePath() path {
	const radius = canvas * 0.225
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
