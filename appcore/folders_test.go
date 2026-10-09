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

	"github.com/syncthing/syncthing/lib/config"
)

func TestSanitizeID(t *testing.T) {
	cases := map[string]string{
		"My Documents":  "my-documents",
		"photos_2024":   "photos-2024",
		"UPPER":         "upper",
		"!!!":           "folder",
		"":              "folder",
		"a b  c":        "a-b--c",
		"trailing---":   "trailing",
		"MiXeD-Case_09": "mixed-case-09",
	}
	for in, want := range cases {
		if got := sanitizeID(in); got != want {
			t.Errorf("sanitizeID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewFolderIDUniqueness(t *testing.T) {
	existing := []config.FolderConfiguration{{ID: "photos"}, {ID: "photos-1"}}
	if got := newFolderID("/x/photos", existing); got != "photos-2" {
		t.Errorf("newFolderID collision handling = %q, want photos-2", got)
	}
}

func TestSamePathWithTrailingSeparator(t *testing.T) {
	dir := t.TempDir()
	if !samePath(dir, dir+string(filepath.Separator)) {
		t.Errorf("samePath should treat %q and its trailing-separator form as equal", dir)
	}
}

func TestNormalizeFolderPathRejectsFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, problem := normalizeFolderPath(file); problem == nil {
		t.Error("normalizeFolderPath should reject a regular file")
	} else if problem.Code != CodeNotADirectory {
		t.Errorf("file rejected with %q, want %q", problem.Code, CodeNotADirectory)
	}
	if got, problem := normalizeFolderPath(dir); problem != nil || got == "" {
		t.Errorf("normalizeFolderPath(%q) returned problem %v; want the directory accepted", dir, problem)
	}
	if _, problem := normalizeFolderPath(filepath.Join(dir, "missing")); problem == nil {
		t.Error("normalizeFolderPath should reject a missing path")
	} else if problem.Code != CodePathMissing {
		t.Errorf("missing path rejected with %q, want %q", problem.Code, CodePathMissing)
	}
}

func TestIsNested(t *testing.T) {
	sep := string(filepath.Separator)
	parent := filepath.Join("a", "b")
	child := filepath.Join("a", "b", "c")
	sibling := filepath.Join("a", "bb")

	if !isNested(child, parent) {
		t.Error("child should be nested in parent")
	}
	if isNested(sibling, parent) {
		t.Error("sibling must not be treated as nested")
	}
	_ = sep
}

func TestRingBufferKeepsNewest(t *testing.T) {
	r := NewRingBuffer(3)
	for _, line := range []string{"one\n", "two\n", "three\n", "four\n"} {
		if _, err := r.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	got := r.Lines(0)
	want := []string{"two", "three", "four"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
	if tail := r.Lines(2); len(tail) != 2 || tail[1] != "four" {
		t.Errorf("Lines(2) = %v, want the two newest lines", tail)
	}
}
