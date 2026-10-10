// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build tools

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestWailsIconIDMatches guards wailsAppIconID against moving.
//
// Wails looks its window icon up by a resource ID it keeps unexported, and the
// lookup is silent when it misses: the executable shows its own icon in
// Explorer while the window and the taskbar button fall back to the generic
// application icon. Nothing else would notice, so it is checked against the
// dependency's own source.
func TestWailsIconIDMatches(t *testing.T) {
	path := filepath.Join(gomodcache(t), "github.com", "wailsapp", "wails",
		"v2@"+wailsVersion(t), "internal", "frontend", "desktop", "windows", "winc", "app.go")

	src, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("cannot read Wails' source (%v); skipping the cross-check", err)
	}

	m := regexp.MustCompile(`AppIconID\s*=\s*(\d+)`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("no AppIconID found in %s; Wails has changed how the app icon is selected", path)
	}
	got, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatal(err)
	}
	if got != wailsAppIconID {
		t.Errorf("Wails now looks the window icon up by ID %d, but this build writes it as ID %d",
			got, wailsAppIconID)
	}
}

func gomodcache(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("GOMODCACHE"); dir != "" {
		return dir
	}
	out, err := runGo(t, "env", "GOMODCACHE")
	if err != nil {
		t.Skipf("cannot locate the module cache: %v", err)
	}
	return out
}

func wailsVersion(t *testing.T) string {
	t.Helper()
	out, err := runGo(t, "list", "-m", "-f", "{{.Version}}", "github.com/wailsapp/wails/v2")
	if err != nil {
		t.Skipf("cannot read the Wails version: %v", err)
	}
	return out
}

// runGo runs a go command and returns its trimmed stdout.
func runGo(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot()
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
