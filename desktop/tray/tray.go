// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package tray

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/syncthing/syncthing/appcore"
)

// Handlers are the callbacks the tray needs from the application.
type Handlers struct {
	// Translate renders a message code in the active UI language.
	Translate   func(appcore.Code, ...any) string
	OnOpen      func()
	OnQuit      func()
	Updates     <-chan appcore.Snapshot
	Paused      func() bool
	TogglePause func() error
	DeviceID    func() string
}

// Run starts the tray. It blocks until the tray becomes unavailable (it does
// not return when the user quits; the process exits through the app instead).
func Run(h Handlers) error {
	var (
		menuOpen   *systray.MenuItem
		menuPause  *systray.MenuItem
		menuDevice *systray.MenuItem
		menuQuit   *systray.MenuItem
		ready      = make(chan struct{})
	)

	done := make(chan struct{})
	var once sync.Once

	go func() {
		defer func() {
			if r := recover(); r != nil {
				once.Do(func() { close(done) })
				fmt.Fprintln(stderrWriter{}, "tray panicked:", r)
			}
		}()
		systray.Run(func() {
			systray.SetTitle("SyncthingApp")
			systray.SetTooltip("SyncthingApp")

			menuOpen = systray.AddMenuItem(tr(h, appcore.CodeTrayOpen), "")
			menuPause = systray.AddMenuItemCheckbox(tr(h, appcore.CodeTrayPause), "", false)
			menuDevice = systray.AddMenuItem("", "")
			menuDevice.Disable()
			systray.AddSeparator()
			menuQuit = systray.AddMenuItem(tr(h, appcore.CodeTrayQuit), "")

			setIcon(StatusOffline, tr(h, appcore.CodeTrayStarting))
			close(ready)

			go func() {
				for range menuOpen.ClickedCh {
					if h.OnOpen != nil {
						h.OnOpen()
					}
				}
			}()
			go func() {
				for range menuPause.ClickedCh {
					if h.TogglePause == nil {
						continue
					}
					if err := h.TogglePause(); err != nil {
						fmt.Fprintln(stderrWriter{}, "toggle pause:", err)
						continue
					}
					if h.Paused != nil && h.Paused() {
						menuPause.Check()
					} else {
						menuPause.Uncheck()
					}
				}
			}()
			go func() {
				for range menuQuit.ClickedCh {
					if h.OnQuit != nil {
						h.OnQuit()
					}
					once.Do(func() { close(done) })
					return
				}
			}()
		}, func() {
			once.Do(func() { close(done) })
		})
	}()

	<-ready

	if h.DeviceID != nil {
		menuDevice.SetTitle(tr(h, appcore.CodeTrayDevice, h.DeviceID()))
	}
	if h.Paused != nil && h.Paused() {
		menuPause.Check()
	}

	// Follow state updates until the tray goes away.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var last appcore.Snapshot
	haveState := false

	for {
		select {
		case <-done:
			return fmt.Errorf("tray stopped")
		case s, ok := <-h.Updates:
			if !ok {
				return fmt.Errorf("state channel closed")
			}
			last, haveState = s, true
			applyStatus(h, last)
			if h.Paused != nil {
				if h.Paused() {
					menuPause.Check()
				} else {
					menuPause.Uncheck()
				}
			}
		case <-ticker.C:
			if haveState {
				applyStatus(h, last)
			}
		}
	}
}

func applyStatus(h Handlers, s appcore.Snapshot) {
	setIcon(statusFor(s), tooltipFor(h, s))
}

// tr renders a catalog entry, falling back to English when no translator was
// supplied (which only happens in tests).
func tr(h Handlers, code appcore.Code, args ...any) string {
	if h.Translate == nil {
		return appcore.Translate(appcore.DefaultLang, code, args...)
	}
	return h.Translate(code, args...)
}

// statusFor maps a snapshot onto the tray icon state, in priority order.
func statusFor(s appcore.Snapshot) Status {
	if s.Phase == appcore.PhaseError || len(s.Errors) > 0 {
		return StatusError
	}
	if allPaused(s) {
		return StatusPaused
	}
	if s.ActiveItems > 0 || anyFolderSyncing(s) {
		return StatusSyncing
	}
	if s.Phase == appcore.PhaseStarting || noDeviceConnected(s) {
		return StatusOffline
	}
	return StatusIdle
}

func anyFolderSyncing(s appcore.Snapshot) bool {
	for _, f := range s.Folders {
		if f.State == "syncing" || f.State == "scanning" {
			return true
		}
	}
	return false
}

func allPaused(s appcore.Snapshot) bool {
	myID := s.DeviceID
	var any bool
	for _, d := range s.Devices {
		if d.ID == myID {
			continue
		}
		any = true
		if !d.Paused {
			return false
		}
	}
	if !any {
		for _, f := range s.Folders {
			if !f.Paused {
				return false
			}
		}
		if len(s.Folders) == 0 {
			return false
		}
	}
	return true
}

func noDeviceConnected(s appcore.Snapshot) bool {
	var peers int
	for _, d := range s.Devices {
		if d.ID == s.DeviceID {
			continue
		}
		peers++
		if d.Connected {
			return false
		}
	}
	return peers > 0
}

func tooltipFor(h Handlers, s appcore.Snapshot) string {
	switch statusFor(s) {
	case StatusError:
		return tr(h, appcore.CodeTrayProblems, len(s.Errors))
	case StatusPaused:
		return tr(h, appcore.CodeTrayPaused)
	case StatusSyncing:
		if avg, count := progressTotals(s); count > 0 {
			return tr(h, appcore.CodeTraySyncingPct, int(avg))
		}
		return tr(h, appcore.CodeTraySyncing)
	case StatusOffline:
		if s.Phase == appcore.PhaseStarting {
			return tr(h, appcore.CodeTrayStarting)
		}
		return tr(h, appcore.CodeTrayNoDevices)
	default:
		if s.TotalNeedItems == 0 {
			return tr(h, appcore.CodeTrayUpToDate)
		}
		return tr(h, appcore.CodeTrayNeedItems, s.TotalNeedItems)
	}
}

// progressTotals averages folder progress across folders that are not paused.
// The second return value is the number of folders counted, so a tooltip can
// tell "no folders" from "0%".
func progressTotals(s appcore.Snapshot) (float64, int) {
	var sum float64
	n := 0
	for _, f := range s.Folders {
		if f.Paused {
			continue
		}
		sum += f.Progress
		n++
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), n
}
