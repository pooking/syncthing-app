// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package appcore

import (
	"os/exec"
	"strconv"
	"syscall"
)

// spawnRestart starts the executable again a couple of seconds from now through
// a detached helper, so it outlives this process.
//
// "cmd start" is used rather than spawning the executable directly: the helper
// runs detached from a process that is about to exit, and the presence of "cmd"
// in the process tree never reaches the user — it is created with
// CREATE_NO_WINDOW, so a GUI build shows no console flash.
//
// The delay is a ping rather than timeout(1) because timeout refuses to run when
// its input is redirected, which is exactly the situation in a console-less
// build: it would give up instantly and the relaunched app would race this
// process for the single-instance lock.
func spawnRestart(exe string) error {
	// Neither command's exit code is checked; the app is launched either way,
	// which is the part that matters.
	script := "ping -n " + strconv.Itoa(restartDelaySeconds+1) + " 127.0.0.1 >nul & start \"\" " +
		quoteWindowsArg(exe)
	return runHidden("cmd", "/c", script)
}

func runHidden(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// The helper is intentionally not waited for: this process is on its way
	// out, and the child must outlive it. It is reaped by the OS instead of
	// leaving a zombie, since it was started detached.
	_ = cmd.Process.Release()
	return nil
}

// quoteWindowsArg wraps a path in double quotes for cmd.exe, where a bare path
// containing spaces would split into several arguments.
func quoteWindowsArg(s string) string {
	return `"` + s + `"`
}
