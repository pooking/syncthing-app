// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package tray

import (
	"bytes"
	"fmt"
	"unsafe"

	"fyne.io/systray"
	"golang.org/x/sys/windows"
)

// setIcon renders the status badge and hands it to the tray. systray on
// Windows loads the bytes from a temporary file, so a single-image ICO is the
// most reliable container.
func setIcon(status Status, tooltip string) {
	systray.SetTooltip(tooltip)
	pngData := IconPNG(status, 48)
	ico, err := pngToICO(pngData)
	if err != nil || len(ico) == 0 {
		return
	}
	systray.SetIcon(ico)
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

// pngToICO wraps PNG data in a single-image ICO container.
func pngToICO(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image")
	}

	var buf bytes.Buffer
	// ICONDIR: reserved, type=icon, count=1
	buf.Write([]byte{0, 0, 1, 0, 1, 0})
	// ICONDIRENTRY: 48x48, 32bpp, payload at offset 22
	buf.WriteByte(48)                   // width
	buf.WriteByte(48)                   // height
	buf.WriteByte(0)                    // palette colours
	buf.WriteByte(0)                    // reserved
	buf.Write([]byte{1, 0})             // colour planes
	buf.Write([]byte{32, 0})            // bits per pixel
	buf.Write(u32le(uint32(len(data)))) // payload size
	buf.Write(u32le(22))                // payload offset
	buf.Write(data)
	return buf.Bytes(), nil
}

func u32le(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}
