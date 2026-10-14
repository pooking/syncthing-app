// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Second-instance detection, asked before the embedded core starts.
//
// Wails has a lock of its own for this, but it is taken inside wails.Run, and
// wails.Run cannot be reached before the core is up: the window's callbacks and
// bindings are methods on the App that the core returns. Starting the core
// first is what loses the race. The core takes the data directory's lock as its
// second act, so a second process fails there — before Wails has had any chance
// to notice that it is a second process at all — and the only thing left to do
// with the mistake is report it. What the user meant by clicking the shortcut,
// the Start menu entry or the executable was "show me the window".
//
// The two functions below ask Wails' own question early enough to act on the
// answer: hand the launch to the instance that already owns the data directory,
// and exit. Two copies could not run side by side anyway — Wails would exit the
// second one from inside wails.Run a moment later — so nothing that used to
// work stops working; the launch just stops being an error on the way.
//
// They duplicate four things out of Wails: the mutex name, the class and name
// of the message-only window that answers it, and the two message numbers.
// Nothing at run time would notice those drifting — a launch would simply go
// back to failing, which is the very state this was written to fix — so buildapp
// checks them against Wails' own source, in the same spirit as the installer's
// copy of the same mutex name.
const (
	// Wails derives all three names from the UniqueId that
	// cmd/osprey-sync-tool passes, as "wails-app-" + UniqueId plus a suffix.
	uniqueID = "ospreysynctool-main-window"

	wailsMutexName  = "wails-app-" + uniqueID + "sim"
	wailsWindowCls  = "wails-app-" + uniqueID + "-sic"
	wailsWindowName = "wails-app-" + uniqueID + "-siw"

	// Window messages: WM_COPYDATA, which is how one process hands a message to
	// another process's window, and the dwData tag Wails marks its own with.
	wmCopyData               = 74
	wmCopyDataSingleInstance = 1542

	// How long to wait for the running instance to take the message. This is a
	// second launch, not a protocol: if that window is wedged, starting up and
	// reporting the real problem beats hanging silently behind a click that
	// looked like it did nothing.
	sendTimeoutMillis = 5000
	smtoAbortIfHung   = 0x0002
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
)

// errNoInstanceWindow says the single-instance mutex is held but the window
// that is supposed to answer it is gone: the other process is on its way out.
var errNoInstanceWindow = errors.New("the running instance has no message window")

// SecondInstanceRunning reports whether another copy of this application is
// already running, by testing for the mutex Wails creates for it.
//
// A mutex is owned by a process, so this stays true for exactly as long as that
// process lives — including the state the application spends most of its time
// in, with the window hidden in the tray, which is also the state a second
// launch has to cope with.
func SecondInstanceRunning() bool {
	name, err := windows.UTF16PtrFromString(wailsMutexName)
	if err != nil {
		return false
	}
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		// Not held, or not ours to look at. Either way there is no instance to
		// hand the launch to, and starting is the right answer.
		return false
	}
	windows.CloseHandle(h)
	return true
}

// WakeSecondInstance asks the instance that is already running to bring its
// window to the front, with the same message a second copy of the application
// sends when Wails detects it — Wails' OnSecondInstanceLaunch ignores the
// payload and only shows the window, which is exactly what is wanted here.
//
// A failure means the running instance is quitting, or that something else
// holds the mutex name; the caller should go on to start normally rather than
// treat the launch as handed over. It never blocks for longer than
// sendTimeoutMillis.
func WakeSecondInstance() error {
	class, err := windows.UTF16PtrFromString(wailsWindowCls)
	if err != nil {
		return err
	}
	window, err := windows.UTF16PtrFromString(wailsWindowName)
	if err != nil {
		return err
	}
	hwnd, _, _ := procFindWindowW.Call(
		uintptr(unsafe.Pointer(class)),
		uintptr(unsafe.Pointer(window)),
	)
	if hwnd == 0 {
		return errNoInstanceWindow
	}

	message, err := marshalSecondInstanceData()
	if err != nil {
		return err
	}
	units, err := windows.UTF16FromString(message)
	if err != nil {
		return err
	}

	// The receiving window procedure reads this as the Win32 COPYDATASTRUCT,
	// so the layout is what matters, not the Go types.
	payload := copyDataStruct{
		dwData: wmCopyDataSingleInstance,
		// Wails counts the length as the string's size in bytes including its
		// terminator, plus one. That extra byte is Wails' arithmetic; it is
		// reproduced rather than corrected so that the message is the one
		// Wails sends, and stays valid if a future version of it reads cbData.
		cbData: uint32(len(units)*2 + 1),
		lpData: uintptr(unsafe.Pointer(&units[0])),
	}

	// SendMessageTimeout returns zero when it gave up. The third return value
	// is the calling goroutine's last-error, which is set on every successful
	// call too, so the result is what decides — and a timeout is a failure
	// worth reporting, since the window it waited on is not coming back.
	var result uintptr
	sent, _, _ := procSendMessageTimeoutW.Call(
		hwnd,
		wmCopyData,
		0,
		uintptr(unsafe.Pointer(&payload)),
		smtoAbortIfHung,
		sendTimeoutMillis,
		uintptr(unsafe.Pointer(&result)),
	)
	if sent == 0 {
		return errNoInstanceWindow
	}
	return nil
}

// copyDataStruct is the Win32 COPYDATASTRUCT.
type copyDataStruct struct {
	dwData uintptr
	cbData uint32
	lpData uintptr
}

// secondInstanceData is github.com/wailsapp/wails/v2/pkg/options.
// SecondInstanceData, which is the payload Wails sends with that message. It is
// declared here rather than imported because Wails only exports the type from
// the options package while the copy it actually sends is built in its internal
// frontend; the JSON is the contract between the two processes.
type secondInstanceData struct {
	Args             []string
	WorkingDirectory string
}

func marshalSecondInstanceData() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}
	// Wails fills both in, and the instance that receives this may one day use
	// them; sending them costs nothing.
	data, err := json.Marshal(secondInstanceData{Args: os.Args[1:], WorkingDirectory: wd})
	if err != nil {
		return "", err
	}
	return string(data), nil
}
