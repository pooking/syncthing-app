// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package appcore

import (
	"os/exec"
	"strconv"
)

// spawnRestart starts the executable again shortly from now, detached so it
// outlives this process. The short delay is what gives this process time to
// release its single-instance lock before the new one looks for it.
func spawnRestart(exe string) error {
	cmd := exec.Command("sh", "-c",
		"sleep "+strconv.Itoa(restartDelaySeconds)+" ; exec "+shellQuote(exe))
	if err := cmd.Start(); err != nil {
		return err
	}
	// Not waited for on purpose: the child must outlive this process.
	return cmd.Process.Release()
}
