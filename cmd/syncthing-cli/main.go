// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Command syncthing-app is the portable SyncthingApp. The P0 milestone ships a
// headless harness used to validate embedded startup, folder sharing and
// event aggregation before the GUI is layered on top.
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
