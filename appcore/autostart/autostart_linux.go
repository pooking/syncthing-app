// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows && !darwin && !android && !ios

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func desktopFilePath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "autostart", "syncthingapp.desktop"), nil
}

func enable(exePath string, args []string) error {
	path, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	exec := quoteDesktop(exePath)
	if len(args) > 0 {
		exec += " " + strings.Join(args, " ")
	}
	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=SyncthingApp\n" +
		"Comment=Keep folders in sync between devices\n" +
		"Exec=" + exec + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"

	return os.WriteFile(path, []byte(content), 0o644)
}

func disable() error {
	path, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func isEnabled() (bool, error) {
	path, err := desktopFilePath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// quoteDesktop wraps a path in double quotes per the Desktop Entry spec,
// escaping embedded quotes and backslashes.
func quoteDesktop(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

var _ = fmt.Sprintf
