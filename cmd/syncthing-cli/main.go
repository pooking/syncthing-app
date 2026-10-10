// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Command syncthing-cli is the headless harness for OspreySyncTool. The P0
// milestone ships a headless harness used to validate embedded startup, folder
// sharing and event aggregation before the GUI is layered on top.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/syncthing/syncthing/appcore"
)

func main() {
	var (
		dataDir   = flag.String("data", "", "data directory (default: platform state dir)")
		portable  = flag.Bool("portable", false, "keep all state next to the executable")
		name      = flag.String("name", "", "device name to advertise")
		addFolder = flag.String("add-folder", "", "share this directory, then stay running")
		addDevice = flag.String("add-device", "", "pair with this device ID (for -add-folder)")
		acceptDir = flag.String("accept", "", "accept all pending folders under this directory")
		wait      = flag.Duration("accept-wait", 20*time.Second, "how long to wait for pending folders with -accept")
		dump      = flag.Bool("dump", false, "print the state snapshot as JSON and exit")
		quiet     = flag.Bool("quiet", false, "suppress the periodic state line")
		moveTo    = flag.String("move-folder", "", "relocate the folder named by -move-id to this directory")
		moveID    = flag.String("move-id", "", "ID of the folder to relocate with -move-folder")
	)
	flag.Parse()

	app, err := appcore.Start(appcore.Options{
		DataDir:    *dataDir,
		Portable:   *portable,
		DeviceName: *name,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to start:", err)
		os.Exit(1)
	}
	defer app.Stop()

	fmt.Printf("Device ID: %s\n", app.DeviceID())
	fmt.Printf("Data dir:  %s\n", app.DataDir())
	// The name is what this device announces to peers, and it is the only thing
	// a peer can learn about it. Printing it here is what makes "the other side
	// shows a different name" a question with an answer: either this line is
	// wrong, or the peer is showing a copy of the name it stored earlier.
	fmt.Printf("Name:      %q\n", app.Settings().DeviceName)
	if s := app.Settings(); s.HubDeviceID != "" {
		fmt.Printf("Hub:       %s (%s) at %s, new folders shared: %v\n",
			s.HubName, s.HubDeviceID, s.HubAddress, s.HubShareNewFolders)
	}

	if *addDevice != "" {
		if _, err := app.AddDevice(*addDevice, ""); err != nil {
			fmt.Fprintln(os.Stderr, "add device:", err)
		} else {
			fmt.Printf("Paired with %s\n", *addDevice)
		}
	}

	if *addFolder != "" {
		var peers []string
		if *addDevice != "" {
			peers = append(peers, *addDevice)
		}
		res, err := app.AddFolders([]string{*addFolder}, peers)
		if err != nil {
			fmt.Fprintln(os.Stderr, "add folder:", err)
		}
		for _, f := range res.Added {
			fmt.Printf("Shared:   %s (%s)\n", f.Path, f.ID)
		}
		for _, r := range res.Rejected {
			fmt.Printf("  rejected %s: %s\n", r.Path, r.Reason)
		}
	}

	if *moveTo != "" {
		check, err := app.CheckFolderMove(*moveID, *moveTo)
		if err != nil {
			fmt.Fprintln(os.Stderr, "check move:", err)
		} else {
			fmt.Printf("Move check: %d file(s), %d byte(s), same filesystem: %v, target has files: %v\n",
				check.Files, check.Bytes, check.SameFilesystem, check.TargetExists)
		}
		if err := app.MoveFolder(*moveID, *moveTo, false); err != nil {
			fmt.Fprintln(os.Stderr, "move folder:", err)
		} else {
			fmt.Printf("Moved:     %s -> %s\n", check.Source, check.Target)
		}
	}

	if *acceptDir != "" {
		deadline := time.Now().Add(*wait)
		for time.Now().Before(deadline) {
			pending := app.PendingFolders()
			if len(pending) > 0 {
				for _, p := range pending {
					dest := filepath.Join(*acceptDir, p.ID)
					_ = os.MkdirAll(dest, 0o755)
					if _, err := app.AcceptPendingFolder(p.DeviceID, p.ID, dest); err != nil {
						fmt.Fprintf(os.Stderr, "accept %s: %v\n", p.ID, err)
					} else {
						fmt.Printf("Accepted: %s -> %s\n", p.ID, dest)
					}
				}
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	if *dump {
		bs, _ := json.MarshalIndent(app.State().Snapshot(), "", "  ")
		fmt.Println(string(bs))
		return
	}

	// The hub is paired from the first start, but it still has to be accepted
	// on the hub's side before it can connect. Until then it appears as a
	// pending device here, which is a single answer to "is it paired yet".
	// Running on an empty data directory and watching this line is how the
	// built-in hub is verified end to end without the GUI.
	if !*quiet {
		snap := app.State().Snapshot()
		hubState := "not configured"
		for _, d := range snap.Devices {
			if appcore.BuiltInDeviceName(d.ID) == "" {
				continue
			}
			hubState = "paired"
			if d.Connected {
				hubState = "connected"
			}
		}
		for _, p := range snap.PendingDevices {
			if appcore.BuiltInDeviceName(p.ID) != "" {
				hubState = "waiting to be accepted on the hub"
			}
		}
		fmt.Printf("Hub state: %s\n", hubState)
	}

	var unsub func()
	if !*quiet {
		unsub = app.State().Subscribe(func(s appcore.Snapshot) {
			fmt.Fprintf(os.Stderr, "[state] phase=%s folders=%d devices=%d need=%dB active=%d\n",
				s.Phase, len(s.Folders), len(s.Devices), s.TotalNeedBytes, s.ActiveItems)
		})
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	fmt.Println("Running. Press Ctrl+C to stop.")
	<-sig
	if unsub != nil {
		unsub()
	}
	fmt.Println("Stopping...")
}
