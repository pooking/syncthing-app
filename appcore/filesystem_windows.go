// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build windows

package appcore

import "path/filepath"

// filesystemKey identifies the volume a path lives on, so two paths can be
// compared to decide whether a move between them is a rename or a real copy.
// The volume name ("C:", `\\server\share`) is enough on Windows.
func filesystemKey(path string) string {
	dir := deepestExisting(path)
	if vol := filepath.VolumeName(dir); vol != "" {
		return vol
	}
	return filepath.VolumeName(path)
}

// filesystemMounts is the set of shared namespaces "change the folder location"
// must refuse. Windows has no equivalent of /home; a drive root is already
// rejected as a broad path.
func filesystemMounts() []string {
	return nil
}
