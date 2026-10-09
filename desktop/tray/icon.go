// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Package tray renders the status icon and menu that live in the system tray.
// It is deliberately UI-framework agnostic: it takes a plain Status value and
// reports clicks through callbacks.
package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// Status is the coarse state the tray icon communicates.
type Status int

const (
	StatusIdle Status = iota
	StatusSyncing
	StatusPaused
	StatusError
	StatusOffline
)

func (s Status) String() string {
	switch s {
	case StatusSyncing:
		return "syncing"
	case StatusPaused:
		return "paused"
	case StatusError:
		return "error"
	case StatusOffline:
		return "offline"
	default:
		return "idle"
	}
}

// palette per status. Chosen to stay readable on both light and dark trays.
var palette = map[Status]color.NRGBA{
	StatusIdle:    {R: 0x2f, G: 0xa8, B: 0x4f, A: 0xff}, // green
	StatusSyncing: {R: 0x2b, G: 0x7c, B: 0xd6, A: 0xff}, // blue
	StatusPaused:  {R: 0xd9, G: 0xa4, B: 0x1e, A: 0xff}, // amber
	StatusError:   {R: 0xd0, G: 0x3a, B: 0x2f, A: 0xff}, // red
	StatusOffline: {R: 0x8a, G: 0x8a, B: 0x8a, A: 0xff}, // grey
}

// IconPNG renders a circular status badge. The icon is generated rather than
// shipped as a file so every platform gets the same art without binary assets.
func IconPNG(status Status, size int) []byte {
	if size <= 0 {
		size = 32
	}
	c := palette[status]
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	cx := float64(size-1) / 2
	cy := float64(size-1) / 2

	// A ring plus a filled centre, leaving a small gap so the badge reads as a
	// status pip rather than a flat dot.
	outer := cx
	inner := outer * 0.62

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			d := sqrt(dx*dx + dy*dy)
			switch {
			case d <= inner:
				img.SetNRGBA(x, y, c)
			case d <= outer && d >= inner*1.12:
				img.SetNRGBA(x, y, c)
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// sqrt is a small Newton-Raphson square root, kept local so the icon generator
// pulls in nothing but image/png.
func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 24; i++ {
		x = (x + v/x) / 2
	}
	return x
}
