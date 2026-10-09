// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package appcore

import (
	"os/exec"
	"strings"
)

// platformLanguage reads the locale macOS keeps in its preferences. On Linux the
// POSIX variables checked by DetectSystemLanguage are already authoritative, so
// there is nothing more to ask and an empty result is fine.
func platformLanguage() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
