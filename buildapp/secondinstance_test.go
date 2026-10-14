// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build tools

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSecondInstanceNamesMatchWails guards the second launch's copy of Wails'
// names against the dependency moving them.
//
// On Windows a second launch has to reach the instance that is already running
// long before wails.Run is reachable, so desktop/singleinstance_windows.go
// opens the mutex and finds the message window by names it builds itself. Every
// one of them is silent when it is wrong: the check fails, the launch falls
// through to starting normally, and the user gets the same "already running"
// error this was written to remove — which looks exactly like a fix that never
// worked.
func TestSecondInstanceNamesMatchWails(t *testing.T) {
	wailsDir := filepath.Join(gomodcache(t), "github.com", "wailsapp", "wails",
		"v2@"+wailsVersion(t), "internal", "frontend", "desktop", "windows")

	singlePath := filepath.Join(wailsDir, "single_instance.go")
	single, err := os.ReadFile(singlePath)
	if err != nil {
		t.Skipf("cannot read %s (%v); skipping the cross-check", singlePath, err)
	}
	// The three names Wails derives from the UniqueId, and the tag it marks its
	// own WM_COPYDATA payload with.
	for _, want := range []string{
		`id := "wails-app-" + uniqueId`,
		`className := id + "-sic"`,
		`windowName := id + "-siw"`,
		`mutexName := id + "sim"`,
		"WMCOPYDATA_SINGLE_INSTANCE_DATA = 1542",
	} {
		if !strings.Contains(string(single), want) {
			t.Errorf("Wails no longer contains %q in %s; desktop/singleinstance_windows.go cannot reach a running instance",
				want, singlePath)
		}
	}

	constsPath := filepath.Join(wailsDir, "winc", "w32", "constants.go")
	consts, err := os.ReadFile(constsPath)
	if err != nil {
		t.Skipf("cannot read %s (%v); skipping the WM_COPYDATA check", constsPath, err)
	}
	if !regexp.MustCompile(`WM_COPYDATA\s*=\s*74\b`).Match(consts) {
		t.Errorf("Wails no longer defines WM_COPYDATA as 74 in %s; the second launch sends the wrong message", constsPath)
	}

	// Our own copy, derived from the same UniqueId the application passes.
	appPath := filepath.Join("..", "cmd", "osprey-sync-tool", "main.go")
	appMain, err := os.ReadFile(appPath)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`UniqueId:\s*"([^"]+)"`).FindSubmatch(appMain)
	if m == nil {
		t.Fatalf("no UniqueId found in %s; the names below cannot be derived", appPath)
	}
	uniqueID := string(m[1])

	ourPath := filepath.Join("..", "desktop", "singleinstance_windows.go")
	ours, err := os.ReadFile(ourPath)
	if err != nil {
		t.Fatal(err)
	}
	src := string(ours)
	for _, want := range []string{
		`uniqueID = "` + uniqueID + `"`,
		`"wails-app-" + uniqueID + "sim"`,
		`"wails-app-" + uniqueID + "-sic"`,
		`"wails-app-" + uniqueID + "-siw"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("%s no longer contains %q; a second launch would not reach the running instance", ourPath, want)
		}
	}
	for name, value := range map[string]string{"wmCopyData": "74", "wmCopyDataSingleInstance": "1542"} {
		re := regexp.MustCompile(name + `\s*=\s*` + value + `\b`)
		if !re.MatchString(src) {
			t.Errorf("%s no longer defines %s as %s", ourPath, name, value)
		}
	}
}
