// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package tray

import (
	"os"

	"fyne.io/systray"
)

func setIcon(status Status, tooltip string) {
	systray.SetTooltip(tooltip)
	systray.SetIcon(IconPNG(status, 64))
}

// stderrWriter forwards background errors to stderr, which is where a launched
// desktop app on Unix-like systems expects diagnostics.
type stderrWriter struct{}

func (stderrWriter) Write(p []byte) (int, error) {
	return os.Stderr.Write(p)
}
