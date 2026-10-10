// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package desktop

import (
	"testing"

	"github.com/syncthing/syncthing/desktop/brand"
)

// The mark is drawn in code, and the interface embeds a copy of it that was
// written out at some point. Nothing at run time notices when the two drift
// apart: the header would just keep showing the older drawing. This is the
// check that catches it, in the same spirit as the frontend's catalog checks.
func TestEmbeddedMarkMatchesTheCode(t *testing.T) {
	onDisk, err := Assets.ReadFile("frontend/dist/mark.svg")
	if err != nil {
		t.Fatalf("the embedded assets have no mark.svg: %v", err)
	}
	if want := brand.SVG(512, brand.DefaultTheme()); string(onDisk) != want {
		t.Error("frontend/dist/mark.svg is stale — regenerate it with\n" +
			"    go run -tags tools ./desktop/brand/preview -write")
	}
}

// The interface asks for the mark by name, so it has to be in the bundle that
// gets embedded. A missing file here is a broken image in the window.
func TestFrontendAssetsAreEmbedded(t *testing.T) {
	for _, name := range []string{
		"frontend/dist/index.html",
		"frontend/dist/app.js",
		"frontend/dist/i18n.js",
		"frontend/dist/style.css",
		"frontend/dist/mark.svg",
	} {
		if _, err := Assets.ReadFile(name); err != nil {
			t.Errorf("%s is not embedded: %v", name, err)
		}
	}
}
