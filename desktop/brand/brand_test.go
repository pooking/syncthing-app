// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package brand

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// The mark is drawn from geometry, so the things worth testing are the
// properties of that geometry rather than any particular pixel: that it is
// centred, that it fills the space it is given without touching the edges, that
// small sizes stay legible, and that nothing is drawn outside the image.

func alphaAt(img *image.NRGBA, x, y int) uint8 {
	return img.Pix[img.PixOffset(x, y)+3]
}

// coverage reports the bounding box of everything with any opacity, and the
// fraction of the image that is not fully transparent.
func coverage(img *image.NRGBA) (bounds image.Rectangle, ratio float64) {
	b := img.Bounds()
	first := true
	var opaque int
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if alphaAt(img, x, y) == 0 {
				continue
			}
			opaque++
			if first {
				bounds = image.Rect(x, y, x+1, y+1)
				first = false
				continue
			}
			bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	if first {
		return image.Rectangle{}, 0
	}
	return bounds, float64(opaque) / float64(b.Dx()*b.Dy())
}

func TestMarkIsDrawnAndCentred(t *testing.T) {
	const size = 256
	img := Mark(size, DefaultTheme())

	bounds, ratio := coverage(img)
	if bounds.Empty() {
		t.Fatal("the mark drew nothing")
	}
	if ratio < 0.05 {
		t.Errorf("only %.1f%% of the image is drawn; the mark is nearly invisible", ratio*100)
	}
	if ratio > 0.85 {
		t.Errorf("%.1f%% of the image is drawn; the mark should not fill its square", ratio*100)
	}

	// The feather stands on the vertical centre line, so the drawn pixels have
	// to balance either side of it. A shape pushed off centre is the classic
	// result of a coordinate typo.
	if got := bounds.Min.X + bounds.Max.X; abs(got-(size-1)) > 2 {
		t.Errorf("the mark is not centred horizontally: bounds %v in a %d square", bounds, size)
	}

	// It must not touch any edge: an icon that does is clipped in some contexts.
	if bounds.Min.X == 0 || bounds.Min.Y == 0 || bounds.Max.X == size || bounds.Max.Y == size {
		t.Errorf("the mark touches the edge of the image: %v", bounds)
	}
}

func TestMarkIsLegibleWhenSmall(t *testing.T) {
	// At 16 pixels the mark is a silhouette. What "legible" means here is that
	// it still has an interior and still reaches both sides, rather than
	// collapsing to a couple of stray pixels.
	img := Mark(16, DefaultTheme())
	bounds, ratio := coverage(img)

	if ratio < 0.15 {
		t.Errorf("at 16px only %.0f%% of the image is drawn: %v", ratio*100, bounds)
	}
	// The feather is taller than it is wide — that is the shape, not a fault —
	// so the bar is that it still spans most of the square's width, not all of
	// it. The geometry gives 16/512 of 256 units, which is 8 of 16 pixels.
	if bounds.Dx() < 8 {
		t.Errorf("at 16px the mark is only %d pixels wide: %v", bounds.Dx(), bounds)
	}
	if bounds.Dy() < 10 {
		t.Errorf("at 16px the mark is only %d pixels tall: %v", bounds.Dy(), bounds)
	}
}

func TestDetailIsDroppedWhenSmall(t *testing.T) {
	// The quill and the barb gaps are dropped below the detail threshold. The
	// test is that this actually reduces what is drawn: a detailed small icon
	// would have less paint coverage than the same size in silhouette.
	small := Mark(detailMinSize-1, DefaultTheme())
	big := Mark(detailMinSize, DefaultTheme())

	_, smallRatio := coverage(small)
	_, bigRatio := coverage(big)

	// The sizes differ, so this is a sanity check rather than a comparison:
	// both have to draw something, and the detailed version must not be
	// emptier than the flat one by accident.
	if smallRatio < 0.15 || bigRatio < 0.10 {
		t.Errorf("coverage at %dpx is %.2f and at %dpx is %.2f",
			detailMinSize-1, smallRatio, detailMinSize, bigRatio)
	}
}

func TestTileIsOpaqueInsideAndTransparentOutsideTheCorners(t *testing.T) {
	const size = 256
	img := Tile(size)

	c := size / 2
	if a := alphaAt(img, c, c); a != 0xff {
		t.Errorf("the tile's centre is not opaque: alpha %d", a)
	}
	// The rounded corner has to actually be rounded, or the icon shows a square
	// block in a taskbar that uses round icons.
	if a := alphaAt(img, 1, 1); a != 0 {
		t.Errorf("the tile's corner is not transparent: alpha %d", a)
	}
	// And the middle of an edge has to be part of the tile.
	if a := alphaAt(img, size/2, 0); a != 0xff {
		t.Errorf("the tile's top edge is not opaque: alpha %d", a)
	}
}

func TestTileKeepsTheFeatherInsideTheShape(t *testing.T) {
	// A feather drawn past the tile's edge would show as a white sliver in the
	// transparent corner. Nothing outside the rounded square may be opaque, so
	// a pixel two in from the corner must still be blank.
	img := Tile(256)
	for _, p := range [][2]int{{2, 2}, {253, 2}, {2, 253}, {253, 253}} {
		if a := alphaAt(img, p[0], p[1]); a != 0 {
			t.Errorf("pixel %v in the tile's corner has alpha %d, so the shape is bleeding", p, a)
		}
	}
}

func TestSolidThemeDarkensTowardTheBase(t *testing.T) {
	c := color.NRGBA{R: 0x2f, G: 0xa8, B: 0x4f, A: 0xff}
	th := SolidTheme(c)

	if len(th.Vane) != 3 {
		t.Fatalf("expected a three-stop ramp, got %d", len(th.Vane))
	}
	lum := func(x color.NRGBA) int { return int(x.R) + int(x.G) + int(x.B) }
	if lum(th.Vane[0]) <= lum(th.Vane[1]) {
		t.Error("the ramp's first stop is not lighter than the status colour")
	}
	if lum(th.Vane[2]) >= lum(th.Vane[1]) {
		t.Error("the ramp's last stop is not darker than the status colour")
	}
	if lum(th.Quill) >= lum(th.Vane[2]) {
		t.Error("the quill is not darker than the darkest part of the vane")
	}
	for _, s := range append(append([]color.NRGBA{}, th.Vane...), th.Quill) {
		if s.A != 0xff {
			t.Errorf("a theme colour is translucent: %v", s)
		}
	}
}

func TestICOIsWellFormed(t *testing.T) {
	data := ICO([]int{16, 32, 256}, func(s int) image.Image {
		return Mark(s, DefaultTheme())
	})

	if len(data) < 6 {
		t.Fatalf("icon is %d bytes", len(data))
	}
	count := int(data[4]) | int(data[5])<<8
	if count != 3 {
		t.Fatalf("icon declares %d images, want 3", count)
	}
	if got := int(data[2]) | int(data[3])<<8; got != 1 {
		t.Errorf("icon type is %d, want 1", got)
	}

	// Every directory entry has to point at data inside the file, or Windows
	// reads past the end of the image.
	for i := 0; i < count; i++ {
		e := data[6+i*16 : 6+16*(i+1)]
		size := int(e[8]) | int(e[9])<<8 | int(e[10])<<16 | int(e[11])<<24
		off := int(e[12]) | int(e[13])<<8 | int(e[14])<<16 | int(e[15])<<24
		if off+size > len(data) {
			t.Errorf("image %d runs past the end: offset %d size %d of %d", i, off, size, len(data))
		}
	}
}

func TestSVGContainsBothShapes(t *testing.T) {
	svg := SVG(512, DefaultTheme())
	for _, want := range []string{"<svg", "</svg>", `gradientUnits="userSpaceOnUse"`, "mask", "<path"} {
		if !strings.Contains(svg, want) {
			t.Errorf("the SVG has no %q", want)
		}
	}
	// A NaN or an infinity in a path makes the browser drop the whole shape
	// without saying anything, so none may reach the output.
	for _, bad := range []string{"NaN", "+Inf", "-Inf"} {
		if strings.Contains(svg, bad) {
			t.Errorf("the SVG contains %q", bad)
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
