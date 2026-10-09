// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build darwin

package appcore

import (
	"strings"
)

// filesystemKey identifies the volume a path lives on. On macOS every mounted
// volume appears under /Volumes, so the mount point is the identifier: that
// makes "/Volumes/Backup/a" and "/Volumes/Backup/b" the same filesystem while
// "/Users/me/x" is a different one.
func filesystemKey(path string) string {
	dir := deepestExisting(path)
	for _, prefix := range []string{"/Volumes/", "/System/Volumes/"} {
		if !strings.HasPrefix(dir, prefix) {
			continue
		}
		rest := dir[len(prefix):]
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" {
			return prefix + strings.ToLower(rest)
		}
	}
	return "/"
}

func filesystemMounts() []string {
	return []string{"/", "/Volumes"}
}
