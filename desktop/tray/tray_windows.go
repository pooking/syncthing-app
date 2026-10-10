// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package tray

import (
	"bytes"
	"unsafe"

	"fyne.io/systray"
	"golang.org/x/sys/windows"

	"github.com/syncthing/syncthing/desktop/brand"
)

// setIcon renders the status mark and hands it to the tray. systray on Windows
// loads the bytes from a temporary file, so an ICO is what it wants; the mark
// is drawn at several sizes so a scaled display gets a sharp one rather than a
// magnified 16-pixel image.
func setIcon(status Status, tooltip string) {
	systray.SetTooltip(tooltip)
	systray.SetIcon(IconICO(status, brand.TraySizes))
}

// stderrWriter sends background errors to the debugger. A GUI build has no
// console attached, so writing to stderr would be lost.
type stderrWriter struct{}

var outputDebugStringW = windows.NewLazySystemDLL("kernel32.dll").NewProc("OutputDebugStringW")

func (stderrWriter) Write(p []byte) (int, error) {
	msg, err := windows.UTF16PtrFromString(string(bytes.TrimRight(p, "\r\n")))
	if err == nil {
		_, _, _ = outputDebugStringW.Call(uintptr(unsafe.Pointer(msg)))
	}
	return len(p), nil
}
