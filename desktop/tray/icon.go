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
	"image"
	"image/color"

	"github.com/syncthing/syncthing/desktop/brand"
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

// IconPNG renders the status icon: the app's feather, filled in the status
// colour, so the tray says what the app is doing in the app's own shape.
func IconPNG(status Status, size int) []byte {
	return brand.PNG(size, brand.SolidTheme(palette[status]))
}

// IconICO renders the status icon as a multi-size Windows icon.
func IconICO(status Status, sizes []int) []byte {
	return brand.ICO(sizes, func(s int) image.Image {
		return brand.Mark(s, brand.SolidTheme(palette[status]))
	})
}
