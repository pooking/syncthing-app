// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package desktop

import (
	"fmt"
	"os"
)

// showFatalDialog writes the failure to stderr; unlike Windows, these platforms
// give a launched GUI app somewhere for stderr to go.
func showFatalDialog(title string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, err)
}
