// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows && !darwin

package appcore

import (
	"os"
	"path/filepath"
	"syscall"
)

// filesystemKey identifies the filesystem a path lives on by its device number,
// which is what makes two paths under different mounts compare as different
// while two paths under one mount compare as the same.
func filesystemKey(path string) string {
	dir := deepestExisting(path)
	var st syscall.Stat_t
	if err := syscall.Stat(dir, &st); err != nil {
		// Without a device number, fall back to comparing top-level
		// directories, which is right for everything except nested mounts.
		return filepath.Dir(dir)
	}
	return string(rune(st.Dev))
}

func filesystemMounts() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	mounts := []string{"/", "/mnt", "/media", "/tmp"}
	if home != "" {
		mounts = append(mounts, home)
	}
	return mounts
}
