// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/vitrun/qart/qr"
)

// QRPNG encodes text as a PNG QR code on a white background, scaled so each
// module is `scale` pixels. It returns nil when the text cannot be encoded.
func QRPNG(text string, scale int) []byte {
	if text == "" {
		return nil
	}
	if scale < 1 {
		scale = 4
	}

	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return nil
	}

	// A 4-module quiet zone keeps scanners happy.
	const quiet = 4
	size := (code.Size + 2*quiet) * scale

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	white := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	black := color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, white)
		}
	}

	for y := 0; y < code.Size; y++ {
		for x := 0; x < code.Size; x++ {
			if !code.Black(x, y) {
				continue
			}
			px := (x + quiet) * scale
			py := (y + quiet) * scale
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.SetNRGBA(px+dx, py+dy, black)
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
