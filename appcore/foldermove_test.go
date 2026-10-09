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

func TestValidateFolderMoveTarget(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "elsewhere")
	if got, err := validateFolderMoveTarget(current, target); err != nil {
		t.Errorf("a fresh directory should be accepted, got %v", err)
	} else if got != target {
		t.Errorf("resolved target = %q, want %q", got, target)
	}

	rejects := []struct {
		name   string
		target string
		code   Code
	}{
		{"empty", "", CodeEmptyPath},
		{"whitespace", "  ", CodeEmptyPath},
		{"the same directory", current, CodeFolderMoveSame},
		{"the same directory with a separator", current + string(filepath.Separator), CodeFolderMoveSame},
		{"inside the folder", filepath.Join(current, "sub"), CodeFolderMoveNested},
		{"an existing file", file, CodeNotADirectory},
		{"a filesystem root", filepath.VolumeName(dir) + string(filepath.Separator), CodeFilesystemRoot},
	}
	for _, tc := range rejects {
		_, err := validateFolderMoveTarget(current, tc.target)
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

func TestMoveFolderData(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	mustWrite(t, filepath.Join(src, "a.txt"), "A")
	mustWrite(t, filepath.Join(src, "sub", "b.txt"), "B")
	// Something already in the destination, to check the merge and the replace
	// behaviour below.
	mustWrite(t, filepath.Join(dst, "keep.txt"), "K")
	mustWrite(t, filepath.Join(dst, "extra.txt"), "E")

	if err := moveFolderData(src, dst, false); err != nil {
		t.Fatalf("moveFolderData: %v", err)
	}
	for _, rel := range []string{"a.txt", filepath.Join("sub", "b.txt")} {
		if !fileExists(filepath.Join(dst, rel)) {
			t.Errorf("%s did not arrive", rel)
		}
	}
	if fileExists(src) {
		t.Error("the source directory should be gone after a successful move")
	}
	// Merging leaves the destination's own files alone.
	if !fileExists(filepath.Join(dst, "extra.txt")) {
		t.Error("a merge must not delete what was already in the destination")
	}
}

func TestMoveFolderDataReplace(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	mustWrite(t, filepath.Join(src, "a.txt"), "A")
	mustWrite(t, filepath.Join(dst, "extra.txt"), "E")
	mustWrite(t, filepath.Join(dst, "sub", "gone.txt"), "G")
	mustWrite(t, filepath.Join(dst, "sub", "kept.txt"), "K")
	mustWrite(t, filepath.Join(src, "sub", "kept.txt"), "K")

	if err := moveFolderData(src, dst, true); err != nil {
		t.Fatalf("moveFolderData: %v", err)
	}
	if fileExists(filepath.Join(dst, "extra.txt")) {
		t.Error("replace should delete files the source does not have")
	}
	if fileExists(filepath.Join(dst, "sub", "gone.txt")) {
		t.Error("replace should descend into subdirectories")
	}
	if !fileExists(filepath.Join(dst, "sub", "kept.txt")) {
		t.Error("replace must keep files the source also has")
	}
}

// TestMoveFolderDataRefusesBroadPaths pins the guard that stops a hand-edited
// configuration from moving and then deleting something unrecoverable.
func TestMoveFolderDataRefusesBroadPaths(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	mustWrite(t, filepath.Join(src, "a.txt"), "A")

	refused := []struct{ name, from, to string }{
		{"empty source", "", filepath.Join(dir, "x")},
		{"a drive root", filepath.VolumeName(dir) + string(filepath.Separator), filepath.Join(dir, "x")},
		{"the folder into itself", src, filepath.Join(src, "inner")},
		{"the folder onto itself", src, src},
	}
	for _, tc := range refused {
		if err := moveFolderData(tc.from, tc.to, false); err == nil {
			t.Errorf("%s: accepted, want a refusal", tc.name)
		}
	}
	if !fileExists(filepath.Join(src, "a.txt")) {
		t.Error("a refused move must leave the source untouched")
	}
}

func TestIsBroadPath(t *testing.T) {
	// What counts as broad is platform-specific, so the expectations are built
	// from the same sources the check uses rather than hard-coded.
	if !isBroadPath("") {
		t.Error("the empty path should be refused")
	}
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	if !isBroadPath(root) {
		t.Errorf("the filesystem root %q should be refused", root)
	}
	for _, m := range filesystemMounts() {
		if !isBroadPath(m) {
			t.Errorf("the mount point %q should be refused", m)
		}
	}
	for _, ordinary := range []string{filepath.Join("a", "b", "c"), t.TempDir()} {
		if isBroadPath(ordinary) {
			t.Errorf("%q should be allowed", ordinary)
		}
	}
}

func TestSameFilesystem(t *testing.T) {
	// Two directories made by the same call are on the same filesystem by
	// construction, whatever the platform.
	a, b := t.TempDir(), t.TempDir()
	if !sameFilesystem(a, b) {
		t.Errorf("sameFilesystem(%q, %q) = false, want true", a, b)
	}
	// A path that does not exist yet is judged by its nearest existing parent,
	// which is what a move destination looks like.
	if !sameFilesystem(a, filepath.Join(a, "not", "created", "yet")) {
		t.Error("a destination under an existing directory must compare as the same filesystem")
	}
}

func TestDirUsage(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "12345")
	mustWrite(t, filepath.Join(dir, "sub", "b.txt"), "123")

	files, bytes, err := dirUsage(dir)
	if err != nil {
		t.Fatalf("dirUsage: %v", err)
	}
	if files != 2 {
		t.Errorf("files = %d, want 2", files)
	}
	if bytes != 8 {
		t.Errorf("bytes = %d, want 8", bytes)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
