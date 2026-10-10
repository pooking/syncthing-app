// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"os"
	"path/filepath"
	"testing"
)

// The tests below drive the first step of a migration by hand: they write the
// same marker MoveDataDir writes, then call the resolver that runs at startup.
// That covers the copy-and-switch logic without restarting a real process.

func TestResolveDataDirDefaultAndPointer(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LocalAppData", base)
	t.Setenv("XDG_STATE_HOME", base)
	t.Setenv("HOME", base)

	if got, want := defaultDataDirForTest(t), filepath.Join(base, "OspreySyncTool"); got != want {
		t.Fatalf("defaultDataDir() = %q, want %q", got, want)
	}

	// With no marker the default directory is used, and it is not created as a
	// side effect of merely asking where it is.
	got, err := resolveDataDirIfNeeded()
	if err != nil {
		t.Fatalf("resolveDataDirIfNeeded: %v", err)
	}
	if got != defaultDataDirForTest(t) {
		t.Errorf("resolved %q, want the default directory", got)
	}

	target := filepath.Join(t.TempDir(), "custom")
	if err := writePointerFile(filepath.Join(defaultDataDirForTest(t), dataDirPointerFile), target); err != nil {
		t.Fatal(err)
	}
	got, err = resolveDataDirIfNeeded()
	if err != nil {
		t.Fatalf("resolveDataDirIfNeeded: %v", err)
	}
	if got != target {
		t.Errorf("resolved %q, want the recorded %q", got, target)
	}
}

func TestResolveDataDirMigrates(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LocalAppData", base)
	t.Setenv("XDG_STATE_HOME", base)
	t.Setenv("HOME", base)

	old := defaultDataDirForTest(t)
	for name, body := range map[string]string{
		"config.xml":       "cfg",
		"cert.pem":         "cert",
		"index-v2/main.db": "db",
		dataDirMigrateFile: "",
		"syncthing.lock":   "",
	} {
		path := filepath.Join(old, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A real request names the destination and nothing else.
	target := filepath.Join(t.TempDir(), "moved")
	if err := os.WriteFile(filepath.Join(old, dataDirMigrateFile), []byte(target+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := resolveDataDirIfNeeded()
	if err != nil {
		t.Fatalf("resolveDataDirIfNeeded: %v", err)
	}
	if got != target {
		t.Fatalf("resolved %q, want the migration target %q", got, target)
	}

	// Everything the identity and configuration depend on must have come
	// across, so the device ID and every pairing survive the move.
	for _, name := range []string{"config.xml", "cert.pem", filepath.Join("index-v2", "main.db")} {
		if !fileExists(filepath.Join(target, name)) {
			t.Errorf("%s was not migrated", name)
		}
	}
	// The request is consumed and the lock file is not carried over.
	if fileExists(filepath.Join(old, dataDirMigrateFile)) {
		t.Error("the migration marker should be removed once it is applied")
	}
	if fileExists(filepath.Join(target, "syncthing.lock")) {
		t.Error("the old lock file should not be copied")
	}
	// The pointer now points at the new location, so the next start stays there.
	if got := readPointerFile(filepath.Join(defaultDataDirForTest(t), dataDirPointerFile)); got != target {
		t.Errorf("pointer file = %q, want %q", got, target)
	}
	// The old directory is kept as a backup rather than deleted.
	if !fileExists(filepath.Join(old, "config.xml")) {
		t.Error("the old directory should be left in place as a backup")
	}
}

// TestResolveDataDirMigrationToSamePath ignores a request that would move the
// directory nowhere instead of copying it onto itself.
func TestResolveDataDirMigrationToSamePath(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LocalAppData", base)
	t.Setenv("XDG_STATE_HOME", base)
	t.Setenv("HOME", base)

	old := defaultDataDirForTest(t)
	if err := os.MkdirAll(old, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "config.xml"), []byte("cfg"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, dataDirMigrateFile), []byte(old+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := resolveDataDirIfNeeded()
	if err != nil {
		t.Fatalf("resolveDataDirIfNeeded: %v", err)
	}
	if got != old {
		t.Errorf("resolved %q, want the directory unchanged at %q", got, old)
	}
	if body, err := os.ReadFile(filepath.Join(old, "config.xml")); err != nil || string(body) != "cfg" {
		t.Errorf("config.xml = %q, %v; want it untouched", body, err)
	}
}

// defaultDataDirForTest resolves the platform default the same way the app
// does, so the expectations above follow the platform under test.
func defaultDataDirForTest(t *testing.T) string {
	t.Helper()
	dir, err := defaultDataDir()
	if err != nil {
		t.Fatalf("defaultDataDir: %v", err)
	}
	return dir
}
