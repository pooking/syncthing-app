// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package desktop

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The same cross-check buildapp makes, run where the code lives: the second
// launch builds Wails' names itself (see singleinstance_windows.go), and a name
// that drifts does not fail — it silently stops matching, and the user gets the
// error this was written to remove.
func TestSecondInstanceNamesAreWailsNames(t *testing.T) {
	if got, want := wailsMutexName, "wails-app-ospreysynctool-main-windowsim"; got != want {
		t.Errorf("wailsMutexName = %q, want %q", got, want)
	}
	if got, want := wailsWindowCls, "wails-app-ospreysynctool-main-window-sic"; got != want {
		t.Errorf("wailsWindowCls = %q, want %q", got, want)
	}
	if got, want := wailsWindowName, "wails-app-ospreysynctool-main-window-siw"; got != want {
		t.Errorf("wailsWindowName = %q, want %q", got, want)
	}
}

// The UniqueId in cmd/osprey-sync-tool is what all three names are built from,
// so it is read from there rather than repeated. The installer's AppMutex is
// derived from the same literal, which is why a change to it has to be a
// deliberate one.
func TestUniqueIDMatchesTheApplication(t *testing.T) {
	path := filepath.Join("..", "cmd", "osprey-sync-tool", "main.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`UniqueId:\s*"([^"]+)"`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("no UniqueId found in %s", path)
	}
	if got := string(m[1]); got != uniqueID {
		t.Errorf("cmd/osprey-sync-tool passes UniqueId %q, but singleinstance_windows.go builds its names from %q", got, uniqueID)
	}
}
