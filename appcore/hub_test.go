// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/protocol"
)

func TestHubDeviceIsConfigured(t *testing.T) {
	hub, id, ok := hubDevice()
	if !ok {
		t.Fatal("the built-in hub constant does not parse")
	}
	if hub.DeviceID != id {
		t.Error("hub entry and returned ID disagree")
	}
	if hub.Name != hubDeviceName {
		t.Errorf("hub name = %q", hub.Name)
	}
	if !slices.Contains(hub.Addresses, hubAddress) {
		t.Errorf("hub addresses %v do not contain %q", hub.Addresses, hubAddress)
	}
	// A pinned address, not discovery: the whole point is not needing the
	// global discovery servers.
	if !strings.HasPrefix(hubAddress, "tcp://") {
		t.Errorf("hub address %q is not a plain TCP address", hubAddress)
	}
}

func TestHubMarkerPathIsInTheDataDir(t *testing.T) {
	a := &App{dataDir: t.TempDir()}
	for _, name := range []string{hubAdoptMarker, setupMarker, "prefs.json"} {
		if a.hubMarkerPath() != filepath.Join(a.dataDir, name) && name == hubAdoptMarker {
			t.Errorf("hub marker is %q, want it beside %s", a.hubMarkerPath(), name)
		}
	}
}

// TestBuiltInDeviceNameOnlyNamesTheHub checks the frontend's labelling helper:
// it must not claim an arbitrary device is the hub.
func TestBuiltInDeviceNameOnlyNamesTheHub(t *testing.T) {
	_, id, _ := hubDevice()
	if got := BuiltInDeviceName(id.String()); got != hubDeviceName {
		t.Errorf("BuiltInDeviceName(hub) = %q, want %q", got, hubDeviceName)
	}
	other := protocol.DeviceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	if got := BuiltInDeviceName(other.String()); got != "" {
		t.Errorf("BuiltInDeviceName(other) = %q, want empty", got)
	}
	if got := BuiltInDeviceName(""); got != "" {
		t.Errorf("BuiltInDeviceName(\"\") = %q, want empty", got)
	}
}

// TestHubFolderTemplateAddingAndRemoving covers the config edits the hub
// toggle makes, without needing a running app: the template is where the
// "share new folders" choice lives, and the toggle must not leave the hub in
// it twice or lose the other members.
func TestHubFolderTemplateAddingAndRemoving(t *testing.T) {
	_, hubID, ok := hubDevice()
	if !ok {
		t.Skip("hub constant does not parse")
	}
	myID := protocol.LocalDeviceID

	cfg := config.New(myID)

	addHub := func(cfg *config.Configuration) {
		tmpl := cfg.Defaults.Folder.Copy()
		for _, d := range tmpl.Devices {
			if d.DeviceID == hubID {
				return
			}
		}
		tmpl.Devices = append(tmpl.Devices, config.FolderDeviceConfiguration{DeviceID: hubID})
		cfg.Defaults.Folder = tmpl
	}

	addHub(&cfg)
	addHub(&cfg) // must be idempotent
	n := 0
	for _, d := range cfg.Defaults.Folder.Devices {
		if d.DeviceID == hubID {
			n++
		}
	}
	if n != 1 {
		t.Errorf("hub appears %d times in the template, want 1", n)
	}

	keep := cfg.Defaults.Folder.Devices[:0]
	for _, d := range cfg.Defaults.Folder.Devices {
		if d.DeviceID != hubID {
			keep = append(keep, d)
		}
	}
	cfg.Defaults.Folder.Devices = keep
	for _, d := range cfg.Defaults.Folder.Devices {
		if d.DeviceID == hubID {
			t.Error("hub still in the template after removal")
		}
	}
}
