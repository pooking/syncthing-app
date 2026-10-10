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

// The owl is drawn from geometry, so what is worth testing are the properties
// of that geometry rather than any particular pixel: that it is centred, that
// it fills the space it is given without touching the edges, that the eyes are
// really holes, and that small sizes still read as an owl.

func alphaAt(img *image.NRGBA, x, y int) uint8 {
	if !(image.Point{x, y}).In(img.Bounds()) {
		return 0
	}
	return img.Pix[img.PixOffset(x, y)+3]
}

// coverage reports the bounding box of everything with any opacity and the
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

func TestOwlIsDrawnAndCentred(t *testing.T) {
	const size = 256
	img := Mark(size, DefaultTheme())

	bounds, ratio := coverage(img)
	if bounds.Empty() {
		t.Fatal("the owl drew nothing")
	}
	if ratio < 0.15 {
		t.Errorf("only %.1f%% of the image is drawn; the owl is nearly invisible", ratio*100)
	}
	if ratio > 0.8 {
		t.Errorf("%.1f%% of the image is drawn; the owl should not fill its square", ratio*100)
	}

	// The owl stands on the vertical centre line, so the drawn pixels have to
	// balance either side of it. A shape pushed off centre is the classic
	// result of a coordinate typo.
	if got := bounds.Min.X + bounds.Max.X; abs(got-(size-1)) > 2 {
		t.Errorf("the owl is not centred horizontally: bounds %v in a %d square", bounds, size)
	}

	// It must not touch any edge: an icon that does is clipped in some contexts.
	if bounds.Min.X == 0 || bounds.Min.Y == 0 || bounds.Max.X == size || bounds.Max.Y == size {
		t.Errorf("the owl touches the edge of the image: %v", bounds)
	}
}

// TestEyesAreHoles is the test that matters most. The eyes are what make the
// shape an owl rather than a blob with two ears, and they are cut out of the
// body rather than drawn on top of it — so if the cut-out ever stops working,
// the owl becomes a solid lump and nothing else would notice.
//
// Every size is checked, because the eye is drawn differently at each: a plain
// hole below the detail threshold, and a hole with a pupil back inside it
// above it.
func TestEyesAreHoles(t *testing.T) {
	for _, size := range []int{16, 24, 32, 48, 256} {
		img := Mark(size, DefaultTheme())
		scale := float64(size) / canvas

		for _, dx := range []float64{-eyeDX, eyeDX} {
			centre := image.Pt(int((cx+dx)*scale), int(eyeY*scale))
			radius := int(eyeR * scale)
			if radius < 1 {
				t.Fatalf("at %dpx the eye is smaller than a pixel", size)
			}
			if !hasTransparentWithin(img, centre, radius) {
				t.Errorf("at %dpx nothing inside the eye at %v is transparent, so the eye is not cut out",
					size, centre)
			}
		}
	}
}

// hasTransparentWithin reports whether any pixel within radius of centre is
// transparent.
func hasTransparentWithin(img *image.NRGBA, centre image.Point, radius int) bool {
	for y := centre.Y - radius; y <= centre.Y+radius; y++ {
		for x := centre.X - radius; x <= centre.X+radius; x++ {
			if alphaAt(img, x, y) == 0 {
				return true
			}
		}
	}
	return false
}

// TestPupilsComeBackAtLargerSizes is the other half of that: above the detail
// threshold the pupil fills the middle of the eye back in, so the owl has a
// stare rather than two blank holes. A pupil that stopped being drawn would
// otherwise pass every other test.
func TestPupilsComeBackAtLargerSizes(t *testing.T) {
	const size = 256
	img := Mark(size, DefaultTheme())
	scale := float64(size) / canvas

	for _, dx := range []float64{-eyeDX, eyeDX} {
		x := int((cx + dx) * scale)
		y := int(eyeY * scale)
		if a := alphaAt(img, x, y); a == 0 {
			t.Errorf("the pupil at (%d,%d) is missing: the eye is a plain hole", x, y)
		}
	}

	// And at a small size it is deliberately absent.
	small := Mark(16, DefaultTheme())
	scale = 16.0 / canvas
	x := int((cx - eyeDX) * scale)
	y := int(eyeY * scale)
	if a := alphaAt(small, x, y); a != 0 {
		t.Errorf("at 16px the pupil is drawn anyway (alpha %d); it is narrower than a pixel there", a)
	}
}

func TestOwlIsLegibleWhenSmall(t *testing.T) {
	// At 16 pixels the owl is a silhouette. What "legible" means here is that
	// it still spans most of its square rather than collapsing to a few stray
	// pixels.
	img := Mark(16, DefaultTheme())
	bounds, ratio := coverage(img)

	if ratio < 0.2 {
		t.Errorf("at 16px only %.0f%% of the image is drawn: %v", ratio*100, bounds)
	}
	if bounds.Dx() < 8 {
		t.Errorf("at 16px the owl is only %d pixels wide: %v", bounds.Dx(), bounds)
	}
	if bounds.Dy() < 10 {
		t.Errorf("at 16px the owl is only %d pixels tall: %v", bounds.Dy(), bounds)
	}
}

func TestTileIsOpaqueInsideAndTransparentOutsideTheCorners(t *testing.T) {
	const size = 256
	img := Tile(size)

	if a := alphaAt(img, size/2, size/2); a != 0xff {
		t.Errorf("the tile's centre is not opaque: alpha %d", a)
	}
	// The rounded corner has to actually be rounded, or the icon shows a square
	// block in a taskbar that uses round icons.
	if a := alphaAt(img, 1, 1); a != 0 {
		t.Errorf("the tile's corner is not transparent: alpha %d", a)
	}
	if a := alphaAt(img, size/2, 0); a != 0xff {
		t.Errorf("the tile's top edge is not opaque: alpha %d", a)
	}
}

func TestTileKeepsTheOwlInsideTheShape(t *testing.T) {
	// An owl drawn past the tile's edge would show as a white sliver in the
	// transparent corner.
	img := Tile(256)
	for _, p := range [][2]int{{2, 2}, {253, 2}, {2, 253}, {253, 253}} {
		if a := alphaAt(img, p[0], p[1]); a != 0 {
			t.Errorf("pixel %v in the tile's corner has alpha %d, so the shape is bleeding", p, a)
		}
	}
}

// TestTileFaceShowsTheTilesOwnColour checks that the eye holes in the tile are
// filled by the gradient rather than by white. It is the whole reason the owl
// is composited as its own layer instead of drawn straight onto the tile.
func TestTileFaceShowsTheTilesOwnColour(t *testing.T) {
	const size = 256
	img := Tile(size)
	scale := float64(size*(1-2*inset)) / canvas
	off := float64(size) * inset

	// Sampled in the middle of the eye's ring, between the pupil and the rim.
	x := int((cx-eyeDX)*scale + off)
	y := int((eyeY-eyeR*0.72)*scale + off)

	r, g, b, a := img.At(x, y).RGBA()
	if a>>8 < 200 {
		t.Fatalf("the eye at (%d,%d) is not opaque: alpha %d", x, y, a>>8)
	}
	// The tile's gradient is blue; white would be all three channels equal.
	if r>>8 > 200 && g>>8 > 200 && b>>8 > 200 {
		t.Errorf("the eye at (%d,%d) is white (%d,%d,%d); it should show the tile's gradient",
			x, y, r>>8, g>>8, b>>8)
	}
}

func TestSolidThemeIsOneColour(t *testing.T) {
	c := colorNRGBA(0x2f, 0xa8, 0x4f)
	th := SolidTheme(c)
	if len(th.Body) != 1 || th.Body[0] != c {
		t.Errorf("SolidTheme produced %v, want the colour it was given", th.Body)
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
	for i := 0; i < count; i++ {
		e := data[6+i*16 : 6+16*(i+1)]
		size := int(e[8]) | int(e[9])<<8 | int(e[10])<<16 | int(e[11])<<24
		off := int(e[12]) | int(e[13])<<8 | int(e[14])<<16 | int(e[15])<<24
		if off+size > len(data) {
			t.Errorf("image %d runs past the end: offset %d size %d of %d", i, off, size, len(data))
		}
	}
}

func TestSVGHoldsTheSameShapes(t *testing.T) {
	svg := SVG(512, DefaultTheme())
	for _, want := range []string{
		"<svg", "</svg>",
		`gradientUnits="userSpaceOnUse"`,
		"<mask", // the eyes are a mask, not a fill
		"<path",
	} {
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

func colorNRGBA(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}
