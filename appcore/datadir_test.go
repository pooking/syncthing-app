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

func TestPointerFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", dataDirPointerFile)
	target := filepath.Join(dir, "elsewhere")

	if got := readPointerFile(path); got != "" {
		t.Errorf("missing pointer file returned %q, want empty", got)
	}
	if err := writePointerFile(path, target); err != nil {
		t.Fatalf("writePointerFile: %v", err)
	}
	if got := readPointerFile(path); got != target {
		t.Errorf("readPointerFile = %q, want %q", got, target)
	}
}

func TestPointerFileIgnoresDamage(t *testing.T) {
	cases := map[string]string{
		"empty":      "",
		"whitespace": "  \n\t\n",
	}
	for name, body := range cases {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := readPointerFile(path); got != "" {
			t.Errorf("%s pointer file returned %q, want empty", name, got)
		}
	}
}

// TestPointerFileResolvesRelative keeps a hand-edited marker usable: a relative
// path is resolved against the working directory rather than silently dropping
// the setting.
func TestPointerFileResolvesRelative(t *testing.T) {
	path := filepath.Join(t.TempDir(), dataDirPointerFile)
	if err := os.WriteFile(path, []byte("relative/dir\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := readPointerFile(path)
	if !filepath.IsAbs(got) {
		t.Fatalf("readPointerFile returned %q, want an absolute path", got)
	}
	if filepath.Base(got) != "dir" {
		t.Errorf("readPointerFile returned %q, want it to end in dir", got)
	}
}

func TestValidateDataDirMove(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current")
	shared := filepath.Join(dir, "shared")

	for _, d := range []string{current, shared} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Creating the directory is also the writability test, since a location we
	// cannot create is one we cannot use.
	fresh := filepath.Join(dir, "fresh")
	if got, err := validateDataDirMove(current, fresh, nil); err != nil {
		t.Errorf("a fresh directory should be accepted, got %v", err)
	} else if got != fresh {
		t.Errorf("resolved target = %q, want %q", got, fresh)
	}

	rejects := []struct {
		name   string
		newDir string
		folder []string
		code   Code
	}{
		{"empty", "", nil, CodeEmptyPath},
		{"whitespace", "   ", nil, CodeEmptyPath},
		{"same path", current, nil, CodeDataMoveSame},
		{"same path, trailing separator", current + string(filepath.Separator), nil, CodeDataMoveSame},
		{"an existing file", file, nil, CodeNotADirectory},
		{"a filesystem root", filepath.VolumeName(dir) + string(filepath.Separator), nil, CodeFilesystemRoot},
		{"inside a shared folder", filepath.Join(shared, "data"), []string{shared}, CodeDataMoveNested},
		{"containing a shared folder", dir, []string{shared}, CodeDataMoveNested},
	}
	for _, tc := range rejects {
		_, err := validateDataDirMove(current, tc.newDir, tc.folder)
		if err == nil {
			t.Errorf("%s: accepted, want %q", tc.name, tc.code)
			continue
		}
		ue, ok := err.(*UserError)
		if !ok {
			t.Errorf("%s: error is %T, want *UserError", tc.name, err)
			continue
		}
		if ue.Code != tc.code {
			t.Errorf("%s: rejected with %q, want %q", tc.name, ue.Code, tc.code)
		}
	}
}

// TestValidateDataDirMoveCreatesTarget pins the side effect the move depends
// on: a location that does not exist yet is created, and one that cannot be
// written is reported as such rather than failing later mid-migration.
func TestValidateDataDirMoveCreatesTarget(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current")
	if err := os.MkdirAll(current, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "created", "deep")

	got, err := validateDataDirMove(current, target, nil)
	if err != nil {
		t.Fatalf("validateDataDirMove: %v", err)
	}
	if got != target {
		t.Errorf("resolved target = %q, want %q", got, target)
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Errorf("target directory was not created: %v", err)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the writability probe was left behind: %v", entries)
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "out")

	mustWrite := func(rel, body string) {
		path := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("config.xml", "cfg")
	mustWrite("index-v2/main.db", "db")
	mustWrite(dataDirMigrateFile, "skip me")
	mustWrite("syncthing.lock", "skip me too")

	skip := map[string]bool{dataDirMigrateFile: true, "syncthing.lock": true}
	if err := copyDir(src, dst, skip); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	for _, rel := range []string{"config.xml", filepath.Join("index-v2", "main.db")} {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("%s was not copied: %v", rel, err)
			continue
		}
		want, _ := os.ReadFile(filepath.Join(src, rel))
		if string(got) != string(want) {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	for name := range skip {
		if fileExists(filepath.Join(dst, name)) {
			t.Errorf("%s should not have been copied", name)
		}
	}
}
