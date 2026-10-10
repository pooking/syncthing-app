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

func TestPortFromAddress(t *testing.T) {
	cases := map[string]int{
		"tcp://0.0.0.0:22000":                     22000,
		"quic://:22000":                           22000,
		"tcp://[::]:22000":                        22000,
		"dynamic+https://relays.example/endpoint": 0,
		"":              0,
		"tcp://0.0.0.0": 0,
	}
	for in, want := range cases {
		if got := portFromAddress(in); got != want {
			t.Errorf("portFromAddress(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestQRPNG(t *testing.T) {
	if got := QRPNG("", 4); got != nil {
		t.Error("QRPNG of an empty string should be nil")
	}
	png := QRPNG("ABCDEFG-1234567", 3)
	if len(png) == 0 {
		t.Fatal("QRPNG produced no data")
	}
	// PNG magic number.
	if string(png[1:4]) != "PNG" {
		t.Errorf("QRPNG did not produce a PNG (header %q)", png[:4])
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	if fileExists(path) {
		t.Error("fileExists should be false for a missing file")
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !fileExists(path) {
		t.Error("fileExists should be true for an existing file")
	}
}

// TestExpandHome covers the "~" that syncthing's own default folder path uses.
// A leading tilde reaching the filesystem would be created as a literal
// directory named "~", so it has to be resolved rather than passed through.
func TestExpandHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	cases := []struct {
		in   string
		want string
	}{
		{"~", home},
		{"~/", home},
		{`~\`, home},
		{"~/Sync", filepath.Join(home, "Sync")},
		{`~\Sync`, filepath.Join(home, "Sync")},
		{"~/Sync/nested", filepath.Join(home, "Sync", "nested")},
		{filepath.Join(home, "Sync"), filepath.Join(home, "Sync")},
		{"", ""},
		{"/absolute/path", "/absolute/path"},
	}
	for _, c := range cases {
		got, err := expandHome(c.in, home)
		if err != nil {
			t.Errorf("expandHome(%q) returned %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("expandHome(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// "~user" is not a home-relative path in the sense we support, and guessing
	// would silently point a folder somewhere unexpected.
	if _, err := expandHome("~someone/Sync", home); err == nil {
		t.Error("expandHome accepted ~someone/Sync")
	}
}

// TestEnsureFolderRootCreatesOnlyTheNamedDirectory checks the rule that keeps a
// folder root from being invented against the wrong place: the directory the
// user named is created, an inner one is not.
func TestEnsureFolderRootCreatesOnlyTheNamedDirectory(t *testing.T) {
	base := t.TempDir()

	// Missing entirely, including its parent: created.
	nested := filepath.Join(base, "not", "there", "yet")
	got, problem := ensureFolderRoot(nested)
	if problem != nil {
		t.Fatalf("ensureFolderRoot(%q) failed: %v", nested, problem)
	}
	if got != nested {
		t.Errorf("ensureFolderRoot returned %q, want %q", got, nested)
	}
	if info, err := os.Stat(nested); err != nil || !info.IsDir() {
		t.Errorf("%q was not created", nested)
	}

	// Already there: kept as is.
	if _, problem := ensureFolderRoot(base); problem != nil {
		t.Errorf("ensureFolderRoot on an existing directory failed: %v", problem)
	}

	// A file is never a folder root.
	file := filepath.Join(base, "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, problem := ensureFolderRoot(file); problem == nil || problem.Code != CodeNotADirectory {
		t.Errorf("ensureFolderRoot on a file gave %v, want %v", problem, CodeNotADirectory)
	}

	// Empty and root are both rejected rather than created.
	if _, problem := ensureFolderRoot("   "); problem == nil || problem.Code != CodeEmptyPath {
		t.Errorf("ensureFolderRoot(\"   \") gave %v, want %v", problem, CodeEmptyPath)
	}
	root := filepath.VolumeName(base) + string(filepath.Separator)
	if _, problem := ensureFolderRoot(root); problem == nil || problem.Code != CodeFilesystemRoot {
		t.Errorf("ensureFolderRoot(%q) gave %v, want %v", root, problem, CodeFilesystemRoot)
	}
}
