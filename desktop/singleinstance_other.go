// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package desktop

// Windows is the only platform where the running instance can be reached
// directly: its single-instance lock is a named mutex and its window can be
// found by name, so a second launch can hand the job over. Wails implements the
// same feature elsewhere through a D-Bus name on Linux and a lock file plus an
// accessibility call on macOS, and neither is something this package can reach
// without cgo. Elsewhere a second launch is left as it was.

// SecondInstanceRunning reports false, so a launch on these platforms always
// starts normally.
func SecondInstanceRunning() bool { return false }

// WakeSecondInstance does nothing. It has no instance to talk to.
func WakeSecondInstance() error { return nil }
