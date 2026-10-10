// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/protocol"
)

// Settings is the subset of options the simple UI exposes.
type Settings struct {
	DeviceName     string `json:"deviceName"`
	MaxSendKbps    int    `json:"maxSendKbps"`
	MaxRecvKbps    int    `json:"maxRecvKbps"`
	RelaysEnabled  bool   `json:"relaysEnabled"`
	NATEnabled     bool   `json:"natEnabled"`
	LocalAnnounce  bool   `json:"localAnnounce"`
	GlobalAnnounce bool   `json:"globalAnnounce"`
	AutoStart      bool   `json:"autoStart"`
	// Language is the interface language, persisted by the app rather than by
	// syncthing.
	Language Lang `json:"language"`

	// The built-in hub. It is not editable here — the ID and address are
	// constants — so these are reported for display, and the sharing switch is
	// applied by SetHubShareNewFolders rather than by SaveSettings.
	HubDeviceID        string `json:"hubDeviceID"`
	HubName            string `json:"hubName"`
	HubAddress         string `json:"hubAddress"`
	HubShareNewFolders bool   `json:"hubShareNewFolders"`
}

// Settings returns the current user-facing settings.
func (a *App) Settings() Settings {
	opts := a.cfg.Options()
	name := ""
	if d, ok := a.cfg.Device(a.cfg.MyID()); ok {
		name = d.Name
	}
	s := Settings{
		DeviceName:    name,
		MaxSendKbps:   opts.MaxSendKbps,
		MaxRecvKbps:   opts.MaxRecvKbps,
		RelaysEnabled: opts.RelaysEnabled,
		NATEnabled:    opts.NATEnabled,
		Language:      a.Language(),
	}
	s.AutoStart, _ = a.AutoStartEnabled()
	if hub, id, ok := hubDevice(); ok {
		s.HubDeviceID = id.String()
		s.HubName = hub.Name
		s.HubAddress = hubAddress
		// The prefs value is what the user last chose; the template is what is
		// actually in effect, and they can diverge if the config was edited by
		// hand. The template wins, since that is what new folders get.
		s.HubShareNewFolders = a.hubInFolderTemplate()
	} else {
		s.HubName = hubDeviceName
	}
	return s
}

// SaveSettings applies the user-facing settings. Only non-zero/non-default
// fields need to be meaningful; the UI always sends the full struct.
func (a *App) SaveSettings(s Settings) error {
	_, err := a.cfg.Modify(func(cfg *config.Configuration) {
		cfg.Options.MaxSendKbps = s.MaxSendKbps
		cfg.Options.MaxRecvKbps = s.MaxRecvKbps
		cfg.Options.RelaysEnabled = s.RelaysEnabled
		cfg.Options.NATEnabled = s.NATEnabled
		// The app manages its own startup, never launch a browser.
		cfg.Options.StartBrowser = false
		cfg.Options.AutoUpgradeIntervalH = 0
		if s.DeviceName != "" {
			if d, _, ok := cfg.Device(a.cfg.MyID()); ok {
				d.Name = s.DeviceName
				cfg.SetDevice(d)
			}
		}
	})
	if err != nil {
		return a.errf(CodeSaveConfig, err)
	}
	if err := a.SetAutoStart(s.AutoStart); err != nil {
		return a.errf(CodeAutostart, err)
	}
	if s.Language != "" && s.Language != a.Language() {
		if err := a.SetLanguage(s.Language); err != nil {
			return err
		}
	}
	a.state.RefreshSoon()
	return nil
}

// PauseAll pauses or resumes syncing for every device and folder, mirroring
// the --paused startup flag. The app itself and discovery keep running.
func (a *App) PauseAll(paused bool) error {
	_, err := a.cfg.Modify(func(cfg *config.Configuration) {
		for i := range cfg.Devices {
			cfg.Devices[i].Paused = paused
		}
		for i := range cfg.Folders {
			cfg.Folders[i].Paused = paused
		}
	})
	if err != nil {
		return err
	}
	a.state.RefreshSoon()
	return nil
}

// Paused reports whether syncing is currently paused for all known peers.
func (a *App) Paused() bool {
	myID := a.cfg.MyID()
	var any bool
	for id, d := range a.cfg.Devices() {
		if id == myID {
			continue
		}
		any = true
		if !d.Paused {
			return false
		}
	}
	// With no peers paired yet, fall back to whether a shared folder is
	// explicitly paused so the tray still reflects the user's choice.
	if !any {
		for _, f := range a.cfg.FolderList() {
			if !f.Paused {
				return false
			}
		}
	}
	return true
}

// SetDeviceName changes the local device's display name.
//
// The name travels: it is carried in the handshake as `DeviceName` on every
// connection, so a peer that does not already know this device — or that has
// OverwriteRemoteDevNames on — picks the new one up the next time it connects.
// Nothing is pushed; it is announced, and the other end decides.
//
// A peer that has a name stored for this device and the option off keeps the
// old one until the device is removed and re-added there. The built-in hub is
// such a peer: what it calls this device is its own stored copy, and it was
// written the first time this device connected.
func (a *App) SetDeviceName(name string) error {
	if name == "" {
		return a.errf(CodeNameEmpty)
	}
	_, err := a.cfg.Modify(func(cfg *config.Configuration) {
		if d, _, ok := cfg.Device(a.cfg.MyID()); ok {
			d.Name = name
			cfg.SetDevice(d)
		}
	})
	if err != nil {
		return err
	}
	a.state.RefreshSoon()
	return nil
}

// LocalDeviceID returns the full device ID string.
func (a *App) LocalDeviceID() string {
	return a.cfg.MyID().String()
}

// ShortDeviceID returns the 7-character device ID used in the UI.
func (a *App) ShortDeviceID() string {
	return a.cfg.MyID().Short().String()
}

// LookupDeviceName returns the display name for a device ID, or the short ID
// when unknown.
func (a *App) LookupDeviceName(idStr string) string {
	id, err := protocol.DeviceIDFromString(idStr)
	if err != nil {
		return idStr
	}
	if d, ok := a.cfg.Device(id); ok && d.Name != "" {
		return d.Name
	}
	return id.Short().String()
}
