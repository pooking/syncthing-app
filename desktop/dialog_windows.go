// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package desktop

import (
	"fmt"
	"syscall"
	"unsafe"
)

var procMessageBoxW = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")

const (
	mbOK            = 0x00000000
	mbIconError     = 0x00000010
	mbSetForeground = 0x00010000
	mbTopmost       = 0x00040000
)

// showFatalDialog puts a message box on screen. A GUI binary has no console, so
// this is the only way the user sees a startup failure. The message is also
// written to the log by the caller, so it survives the dialog being dismissed.
func showFatalDialog(title string, err error) {
	text := fmt.Sprintf("%s\n\n%s", title, err)

	titlePtr, errT := syscall.UTF16PtrFromString(title)
	textPtr, errB := syscall.UTF16PtrFromString(text)
	if errT != nil || errB != nil {
		return
	}

	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(mbOK|mbIconError|mbSetForeground|mbTopmost),
	)
}
