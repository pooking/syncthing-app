// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"os"

	"github.com/syncthing/syncthing/appcore/autostart"
)

// AutoStartEnabled reports whether the app is registered to run at login.
func (a *App) AutoStartEnabled() (bool, error) {
	return autostart.IsEnabled()
}

// SetAutoStart registers or unregisters the app for login startup. The current
// executable is used, started minimized so only the tray icon appears.
func (a *App) SetAutoStart(enabled bool) error {
	if !enabled {
		return autostart.Disable()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return autostart.Enable(exe, []string{"--minimized"})
}

// ExecutablePath returns the path of the running binary.
func (a *App) ExecutablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

// Version returns the version string reported by the build.
func (a *App) Version() string {
	return BuildVersion()
}
