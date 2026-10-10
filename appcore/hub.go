// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/protocol"
)

// The hub device is baked into the app rather than entered by hand on every
// machine: it lives on a server, is paired with everyone, and is the meeting
// point two peers can reach when a direct connection between them is not
// possible.
//
// Its address is pinned rather than left to global discovery, which is
// unreliable where the app is deployed; a pinned address also makes the hub
// reachable as soon as the app starts, with no discovery round trip. Matching
// syncthing's own syntax, "tcp://host:port" is a plain TCP address while
// "tcp://host:port,dynamic" would combine it with discovery.
const (
	hubDeviceIDStr = "TTJYEAS-SGAEE5C-DBZ3HFD-DXNYYKN-SAOU4TP-QWF2B4M-NJT5I4P-GQ5Y3QN"
	hubDeviceName  = "kb-server"
	hubAddress     = "tcp://101.69.254.66:22000"
)

// hubDevice returns the built-in hub's configuration entry. The second result
// is false when the constant does not parse, in which case the app carries on
// without a hub rather than refusing to start.
func hubDevice() (config.DeviceConfiguration, protocol.DeviceID, bool) {
	id, err := protocol.DeviceIDFromString(hubDeviceIDStr)
	if err != nil {
		slog.Error("Built-in hub device ID is not valid", "error", err)
		return config.DeviceConfiguration{}, protocol.EmptyDeviceID, false
	}
	return config.DeviceConfiguration{
		DeviceID:  id,
		Name:      hubDeviceName,
		Addresses: []string{hubAddress},
	}, id, true
}

// hubAdoptMarker records that the hub has been adopted. It sits in the data
// directory with the app's other files, and its presence is what makes adoption
// a one-time act: the hub can be unpaired afterwards and stays unpaired.
const hubAdoptMarker = "hub-device-adopted"

// adoptHubDevice runs on every start. It pairs the built-in hub once — the
// first time, before the engine is created, so a new installation is paired with
// the hub from its first launch and there is no window in which the hub's
// connections are turned away as an unknown device — and then keeps the folder
// template in step with the stored preference.
//
// Pairing is one-time: a marker in the data directory records it, so an existing
// installation that predates the hub keeps the device list its user built, and
// un-pairing the hub afterwards sticks.
//
// The template is different, and is why this is not simply skipped once the
// marker exists. A folder's own device list is fixed when the folder is created,
// so the template is the only thing that can carry "new folders also go to the
// hub". The preference is read from the app's own settings, and applying it here
// is what makes the switch survive a restart; it is a no-op when the template
// already matches.
func (a *App) adoptHubDevice(shareNewFolders bool) error {
	hub, hubID, ok := hubDevice()
	if !ok {
		return nil
	}

	// Running on the hub itself: there is nothing to pair with, but the folder
	// template and the marker are still this machine's own business.
	onHub := hubID == a.cfg.MyID()

	// Adoption is a one-time act, recorded in the data directory. Checking it
	// first is what makes an installation that predates the hub keep the device
	// list the user built, while the folder template is kept in step on every
	// start — a choice the user made in the UI must survive a restart.
	if !onHub && !fileExists(a.hubMarkerPath()) {
		add := true
		if _, already := a.cfg.Device(hubID); already {
			add = false // paired by hand already; just record the adoption
		}
		if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
			if add {
				cfg.SetDevice(hub)
			}
		}); err != nil {
			return a.errf(CodeSaveConfig, err)
		}
		if err := os.WriteFile(a.hubMarkerPath(), []byte(BuildVersion()+"\n"), 0o600); err != nil {
			// The pairing is done; a missing marker only means the next start
			// repeats an operation that is idempotent anyway.
			slog.Warn("Could not record the hub adoption", "error", err)
		}
		slog.Info("Built-in hub configured", "device", hubDeviceName, "address", hubAddress)
	}

	// The template is what folders created from now on are built from, so it is
	// set to the stored preference rather than only ever added to.
	if err := a.setHubTemplate(shareNewFolders); err != nil {
		return err
	}
	return nil
}

// setHubTemplate adds or removes the hub in the folder template. It is a no-op
// when the template already matches, so the common path writes nothing.
func (a *App) setHubTemplate(share bool) error {
	_, hubID, ok := hubDevice()
	if !ok || hubID == a.cfg.MyID() {
		return nil
	}
	if a.hubInFolderTemplate() == share {
		return nil
	}
	if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
		applyHubToTemplate(cfg, hubID, share)
	}); err != nil {
		return a.errf(CodeSaveConfig, err)
	}
	slog.Info("Hub membership of the folder template set", "share", share)
	return nil
}

// applyHubToTemplate edits cfg.Defaults.Folder in place. Adding is idempotent.
func applyHubToTemplate(cfg *config.Configuration, hubID protocol.DeviceID, share bool) {
	tmpl := cfg.Defaults.Folder.Copy()
	kept := make([]config.FolderDeviceConfiguration, 0, len(tmpl.Devices)+1)
	hasHub := false
	for _, d := range tmpl.Devices {
		if d.DeviceID == hubID {
			hasHub = true
			if !share {
				continue
			}
		}
		kept = append(kept, d)
	}
	if share && !hasHub {
		kept = append(kept, config.FolderDeviceConfiguration{DeviceID: hubID})
	}
	tmpl.Devices = kept
	cfg.Defaults.Folder = tmpl
}

// hubInFolderTemplate reports whether the hub is currently a member of the
// folder template, which is what the UI shows as the state of the setting.
func (a *App) hubInFolderTemplate() bool {
	_, id, ok := hubDevice()
	if !ok {
		return false
	}
	for _, d := range a.cfg.DefaultFolder().Devices {
		if d.DeviceID == id {
			return true
		}
	}
	return false
}

// SetHubShareNewFolders changes whether folders are shared with the built-in
// hub, and persists the choice. Turning it on also adds the hub to folders that
// are already shared, since a backup that only covers folders created from now
// on would be a surprising answer to "back up my folders". Turning it off only
// stops future folders from joining: removing the hub from a folder that is
// already syncing to it would delete the copy there, which is not what a toggle
// should do behind the user's back.
func (a *App) SetHubShareNewFolders(share bool) error {
	_, hubID, ok := hubDevice()
	if !ok {
		return nil
	}

	if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
		applyHubToTemplate(cfg, hubID, share)

		if share {
			myID := a.cfg.MyID()
			for _, folder := range cfg.Folders {
				if slices.Contains(folder.DeviceIDs(), hubID) {
					continue
				}
				joined := folder.Copy()
				joined.Devices = append(joined.Devices, config.FolderDeviceConfiguration{DeviceID: hubID})
				// The hub must not be the only member: syncthing refuses a
				// folder we do not share with ourselves, and it would be a
				// pointless folder besides.
				if !slices.Contains(joined.DeviceIDs(), myID) {
					joined.Devices = append(joined.Devices, config.FolderDeviceConfiguration{DeviceID: myID})
				}
				cfg.SetFolder(joined)
			}
		}
	}); err != nil {
		return a.errf(CodeSaveConfig, err)
	}

	a.mu.Lock()
	a.hubShareNewFolders = share
	a.mu.Unlock()
	if err := a.savePrefs(); err != nil {
		slog.Warn("Could not save the hub sharing preference", "error", err)
	}

	slog.Info("Hub sharing for folders changed", "share", share, "appliedToExisting", share)
	a.state.RefreshSoon()
	return nil
}

// BuiltInDeviceName returns the display name of a device the app ships with,
// so the UI can label it without carrying the ID. Empty for any other device.
func BuiltInDeviceName(id string) string {
	if _, hubID, ok := hubDevice(); ok && hubID.String() == id {
		return hubDeviceName
	}
	return ""
}

func (a *App) hubMarkerPath() string { return filepath.Join(a.dataDir, hubAdoptMarker) }
