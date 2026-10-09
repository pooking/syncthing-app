// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Package autostart enables or disables launching the app when the user logs
// in, using the native mechanism of each platform.
package autostart

// appName is the identity used in the registry key, launch agent and desktop
// entry. It must stay stable so that Enable/Disable/IsEnabled agree.
const appName = "SyncthingApp"

// Enable registers the app to start at login.
func Enable(exePath string, args []string) error {
	return enable(exePath, args)
}

// Disable removes the login item, if present. It is not an error to call it
// when nothing is registered.
func Disable() error {
	return disable()
}

// IsEnabled reports whether the app is currently registered to start at login.
func IsEnabled() (bool, error) {
	return isEnabled()
}
