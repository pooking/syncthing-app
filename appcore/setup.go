// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"encoding/base64"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/structutil"
)

// SetupStatus describes how far along first-run configuration is.
type SetupStatus struct {
	// NeedsSetup is true until the user has completed the first-run wizard.
	NeedsSetup bool `json:"needsSetup"`
	// DataDir is where config and database live.
	DataDir string `json:"dataDir"`
	// Portable is true when state lives next to the executable.
	Portable bool `json:"portable"`
	// DefaultSyncDir is the suggested root for shared folders.
	DefaultSyncDir string `json:"defaultSyncDir"`
	// DeviceID is this instance's device ID.
	DeviceID string `json:"deviceID"`
	// DeviceName is the current local device name.
	DeviceName string `json:"deviceName"`
	// Version is the app version string.
	Version string `json:"version"`
	// Language is the interface language currently in effect.
	Language Lang `json:"language"`
}

// SetupOptions are the choices made in the first-run wizard.
type SetupOptions struct {
	// DeviceName is the name shown to other devices.
	DeviceName string `json:"deviceName"`
	// SyncDir becomes the default folder template and the root used when
	// accepting folders others offer. Empty keeps the current setting.
	SyncDir string `json:"syncDir"`
}

// setupMarker is written after the wizard completes. Its presence is what
// distinguishes "never configured" from "configured".
const setupMarker = ".setup-complete"

// SetupStatus reports whether the first-run wizard still needs to be shown.
func (a *App) SetupStatus() SetupStatus {
	status := SetupStatus{
		NeedsSetup:     !fileExists(filepath.Join(a.dataDir, setupMarker)),
		DataDir:        a.dataDir,
		Portable:       a.opts.Portable || PortableRequested(),
		DefaultSyncDir: a.DefaultSyncDir(),
		DeviceID:       a.cfg.MyID().String(),
		DeviceName:     a.localDeviceName(),
		Version:        BuildVersion(),
		Language:       a.Language(),
	}
	return status
}

// CompleteSetup applies the wizard's choices and marks setup as done. It is
// safe to call again; it simply overwrites the choices.
func (a *App) CompleteSetup(opts SetupOptions) error {
	slog.Info("First-run setup completed", "syncDir", opts.SyncDir, "deviceName", opts.DeviceName)
	var syncDir string
	if opts.SyncDir != "" {
		dir, problem := normalizeFolderPath(opts.SyncDir)
		if problem != nil {
			return &localizedError{code: problem.Code, args: problem.Args, lang: a.Language()}
		}
		// The directory does not have to exist yet; a default folder root is
		// allowed to be created later.
		syncDir = dir
	}

	myID := a.cfg.MyID()
	_, err := a.cfg.Modify(func(cfg *config.Configuration) {
		if syncDir != "" {
			// The folder template drives the path suggested for new shares.
			tmpl := cfg.Defaults.Folder.Copy()
			structutil.SetDefaults(&tmpl)
			tmpl.Path = syncDir
			cfg.Defaults.Folder = tmpl
		}
		if opts.DeviceName != "" {
			if dev, _, ok := cfg.Device(myID); ok {
				dev.Name = opts.DeviceName
				cfg.SetDevice(dev)
			}
		}
	})
	if err != nil {
		return a.errf(CodeSaveConfig, err)
	}

	if syncDir != "" {
		if err := os.MkdirAll(syncDir, 0o755); err != nil {
			return a.errf(CodeCreateSyncDir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(a.dataDir, setupMarker), []byte(BuildVersion()+"\n"), 0o600); err != nil {
		return a.errf(CodeWriteSetup, err)
	}

	a.state.RefreshSoon()
	return nil
}

// DefaultSyncDir returns the directory suggested for new shares: an explicit
// setting if present, otherwise "Sync" in the user's home directory.
func (a *App) DefaultSyncDir() string {
	if p := a.cfg.DefaultFolder().Path; p != "" && p != "~" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(filepath.Dir(a.dataDir), "Sync")
	}
	return filepath.Join(home, "Sync")
}

// DeviceIDQRCode returns a PNG QR code containing the device ID, base64
// encoded so it can be used directly in an <img src="data:..."> tag.
func (a *App) DeviceIDQRCode() string {
	png := QRPNG(a.cfg.MyID().String(), 4)
	if png == nil {
		return ""
	}
	return "data:image/png;base64," + base64Std(png)
}

// ListeningPorts returns the TCP ports from the configured listen addresses.
func (a *App) ListeningPorts() []int {
	opts := a.cfg.Options()
	var ports []int
	for _, addr := range opts.RawListenAddresses {
		if p := portFromAddress(addr); p > 0 {
			ports = append(ports, p)
		}
	}
	return ports
}

func (a *App) localDeviceName() string {
	if d, ok := a.cfg.Device(a.cfg.MyID()); ok {
		return d.Name
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func base64Std(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// portFromAddress extracts the port from a syncthing listen address such as
// "tcp://0.0.0.0:22000" or "quic://:22000".
func portFromAddress(addr string) int {
	if i := strings.Index(addr, "://"); i >= 0 {
		addr = addr[i+3:]
	}
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return p
}
