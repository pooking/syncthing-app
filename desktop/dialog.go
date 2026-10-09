// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package desktop

import (
	"log/slog"
	"os"
)

// reportFatal surfaces an error to the user before the process exits.
//
// The desktop build is a GUI binary on Windows with no console attached, so
// writing to stderr would hide the message entirely; that platform shows a
// native dialog instead. Either way the error is written to the log, so a
// support request can be answered from the log file.
func reportFatal(title string, err error) {
	slog.Error(title, "error", err.Error())
	showFatalDialog(title, err)
}

// Fatal reports an error and exits with status 1. It is intended for failures
// that happen before the window exists.
func Fatal(title string, err error) {
	reportFatal(title, err)
	os.Exit(1)
}
