// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Command osprey-sync-tool is the portable desktop app: a small native window and
// a tray icon wrapped around an embedded Syncthing core.
package main

import (
	"errors"
	"flag"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/syncthing/syncthing/appcore"
	"github.com/syncthing/syncthing/desktop"
)

func main() {
	var (
		portable  = flag.Bool("portable", false, "keep all state next to the executable")
		dataDir   = flag.String("data", "", "data directory (default: platform state dir)")
		minimized = flag.Bool("minimized", false, "start hidden in the tray")
		showAll   = flag.Bool("show", false, "force the window to be shown")
	)
	flag.Parse()

	// Asked before the core starts, because the answer stops mattering once it
	// has: the core takes the data directory's lock, and the launch below would
	// then be reported as a conflict rather than as the second launch it is.
	// See desktop/singleinstance_windows.go.
	if desktop.SecondInstanceRunning() && desktop.WakeSecondInstance() == nil {
		return
	}

	app, err := desktop.New(desktop.Options{
		Portable:  *portable,
		DataDir:   *dataDir,
		Minimized: *minimized && !*showAll,
	})
	if err != nil {
		if errors.Is(err, appcore.ErrAlreadyRunning) {
			// Another instance holds the data directory lock. A launch that
			// asks for a second, different data directory is refused by that
			// same lock, and it is a genuine conflict — the two cannot share
			// the instance that is already running, so there is nothing to do
			// but say so.
			desktop.Fatal("OspreySyncTool is already running", errors.New("another instance owns this data directory"))
		}
		desktop.Fatal("OspreySyncTool could not start", err)
	}

	err = wails.Run(&options.App{
		Title:             "OspreySyncTool",
		Width:             980,
		Height:            680,
		MinWidth:          760,
		MinHeight:         520,
		DisableResize:     false,
		Frameless:         false,
		StartHidden:       *minimized && !*showAll,
		HideWindowOnClose: true,
		AssetServer:       &assetserver.Options{Assets: desktop.Assets},
		DragAndDrop:       &options.DragAndDrop{EnableFileDrop: true},
		OnStartup:         app.OnStartup,
		OnDomReady:        app.OnDomReady,
		OnBeforeClose:     app.OnBeforeClose,
		OnShutdown:        app.OnShutdown,
		Bind:              []any{app},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "ospreysynctool-main-window",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				// A second launch just reveals the window of the first one.
				app.ShowWindow()
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})
	if err != nil {
		desktop.Fatal("OspreySyncTool stopped unexpectedly", err)
	}
}
