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

func TestPortFromAddress(t *testing.T) {
	cases := map[string]int{
		"tcp://0.0.0.0:22000":                     22000,
		"quic://:22000":                           22000,
		"tcp://[::]:22000":                        22000,
		"dynamic+https://relays.example/endpoint": 0,
		"":              0,
		"tcp://0.0.0.0": 0,
	}
	for in, want := range cases {
		if got := portFromAddress(in); got != want {
			t.Errorf("portFromAddress(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestQRPNG(t *testing.T) {
	if got := QRPNG("", 4); got != nil {
		t.Error("QRPNG of an empty string should be nil")
	}
	png := QRPNG("ABCDEFG-1234567", 3)
	if len(png) == 0 {
		t.Fatal("QRPNG produced no data")
	}
	// PNG magic number.
	if string(png[1:4]) != "PNG" {
		t.Errorf("QRPNG did not produce a PNG (header %q)", png[:4])
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	if fileExists(path) {
		t.Error("fileExists should be false for a missing file")
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !fileExists(path) {
		t.Error("fileExists should be true for an existing file")
	}
}
