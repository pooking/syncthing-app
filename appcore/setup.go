// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"encoding/base64"
	"fmt"
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
		dir, problem := ensureFolderRoot(opts.SyncDir)
		if problem != nil {
			return &localizedError{code: problem.Code, args: problem.Args, lang: a.Language()}
		}
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

// DefaultSyncDir returns the directory suggested for new shares: the folder
// template's path, which is what the wizard's choice sets and what new folders
// are built from. It falls back to "Sync" in the user's home directory.
//
// The result is an absolute path. Callers put it in front of the user and
// create it, so a leading "~" left unexpanded would be written to disk as a
// literal directory name.
func (a *App) DefaultSyncDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = filepath.Dir(a.dataDir)
	}
	fallback := filepath.Join(home, "Sync")

	// A template that has not been through the config preparation step still
	// carries the literal "~" that syncthing's own default uses.
	p := a.cfg.DefaultFolder().Path
	if p == "" || p == "~" {
		return fallback
	}
	expanded, err := expandHome(p, home)
	if err != nil {
		return fallback
	}
	// A relative template would become a relative folder path in the config,
	// which resolves against whatever the working directory happens to be.
	if !filepath.IsAbs(expanded) {
		return fallback
	}
	return expanded
}

// expandHome resolves a leading tilde (plain "~", or "~/", "~\", "~/" with
// either slash) against home. Any other path is returned unchanged.
func expandHome(path, home string) (string, error) {
	if !strings.HasPrefix(path, "~") && !strings.HasPrefix(path, "～") {
		return path, nil
	}
	rest := path[len("~"):]
	if rest == "" {
		return home, nil
	}
	if rest[0] != '/' && rest[0] != '\\' {
		return "", fmt.Errorf("%q is not a home-relative path", path)
	}
	rest = strings.TrimLeft(rest, `/\`)
	if rest == "" {
		return home, nil
	}
	return filepath.Join(home, filepath.FromSlash(rest)), nil
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
// The result is never nil: it is marshalled straight to the UI, where a null
// would read as "not a list" and break the code that counts the entries.
func (a *App) ListeningPorts() []int {
	opts := a.cfg.Options()
	ports := []int{}
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
