// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Package desktop glues the embedded Syncthing core (appcore) to a native
// window and a system tray icon. It exposes a small method set to the web
// frontend through Wails bindings; all business logic lives in appcore.
package desktop

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/syncthing/syncthing/appcore"
	"github.com/syncthing/syncthing/desktop/tray"
)

// Options configures the desktop shell.
type Options struct {
	Portable  bool
	DataDir   string
	Minimized bool
}

// App is the Wails-bound application object.
type App struct {
	opts Options

	core *appcore.App

	ctxMu sync.Mutex
	ctx   context.Context

	trayOnce sync.Once
	quitting chan struct{}
	quitOnce sync.Once
}

// New starts the embedded core and returns the shell. The window is created by
// the caller (cmd/syncthing-app) so this package stays free of main-package
// concerns.
func New(opts Options) (*App, error) {
	core, err := appcore.Start(appcore.Options{
		DataDir:  opts.DataDir,
		Portable: opts.Portable,
	})
	if err != nil {
		return nil, err
	}

	a := &App{
		opts:     opts,
		core:     core,
		quitting: make(chan struct{}),
	}
	return a, nil
}

// OnStartup wires the frontend to the state bridge.
func (a *App) OnStartup(ctx context.Context) {
	a.ctxMu.Lock()
	a.ctx = ctx
	a.ctxMu.Unlock()

	a.core.State().Subscribe(func(s appcore.Snapshot) {
		a.emit("state", appcore.SnapshotJSON(s))
	})

	// Native drag-and-drop: dropping directories onto the window shares them.
	wailsruntime.OnFileDrop(ctx, func(_, _ int, paths []string) {
		if len(paths) == 0 {
			return
		}
		a.emit("dropped", a.DropFolders(paths))
	})

	go a.runTray()
}

// OnDomReady is called once the frontend has loaded.
func (a *App) OnDomReady(_ context.Context) {
	slog.Info("Frontend loaded")
	a.emit("ready", "{}")
}

// BootLog records a frontend startup step in the app log. A GUI build has no
// console, so when the window comes up empty this journal is what says how far
// the frontend got. Failures are not the caller's problem.
func (a *App) BootLog(step string) {
	slog.Info("Frontend boot", "step", step)
}

// OnBeforeClose intercepts the window close button and hides the window
// instead, leaving the app running in the tray.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	select {
	case <-a.quitting:
		return false // real quit: let the window close
	default:
	}
	// A minimized start (login autostart) should not steal focus; the window
	// stays hidden and the tray carries the status.
	if !a.opts.Minimized {
		wailsruntime.WindowShow(ctx)
	}
	wailsruntime.WindowHide(ctx)
	a.emit("hidden", "{}")
	return true // prevent the close
}

// OnShutdown stops the embedded core gracefully.
func (a *App) OnShutdown(_ context.Context) {
	a.quitOnce.Do(func() { close(a.quitting) })
	a.core.Stop()
}

func (a *App) emit(event string, data string) {
	a.ctxMu.Lock()
	ctx := a.ctx
	a.ctxMu.Unlock()
	if ctx == nil {
		return
	}
	wailsruntime.EventsEmit(ctx, event, data)
}

// ---- Methods exposed to the frontend -------------------------------------

// GetState returns the current snapshot as JSON.
func (a *App) GetState() string {
	return appcore.SnapshotJSON(a.core.State().Snapshot())
}

// GetSettings returns the settings as JSON.
func (a *App) GetSettings() string {
	return jsonString(a.core.Settings())
}

// SaveSettings persists settings supplied as JSON.
func (a *App) SaveSettings(json string) error {
	var s appcore.Settings
	if err := jsonUnmarshal(json, &s); err != nil {
		return err
	}
	return a.core.SaveSettings(s)
}

// SetHubShareNewFolders adds or removes the built-in hub as a member of shared
// folders. Applied immediately rather than with Save: it reaches folders that
// are already shared.
func (a *App) SetHubShareNewFolders(share bool) error {
	return a.core.SetHubShareNewFolders(share)
}

// DeviceID returns the full local device ID.
func (a *App) DeviceID() string { return a.core.LocalDeviceID() }

// ShortDeviceID returns the 7-character local device ID.
func (a *App) ShortDeviceID() string { return a.core.ShortDeviceID() }

// Version returns the app version string.
func (a *App) Version() string { return a.core.Version() }

// DataDir returns the resolved data directory.
func (a *App) DataDir() string { return a.core.DataDir() }

// MoveDataDir relocates the data directory: it validates the target, records
// the request and restarts the app, which performs the copy before opening
// anything. The call does not return on success, because the process exits.
func (a *App) MoveDataDir(path string) error { return a.core.MoveDataDir(path) }

// AddFolders shares the given directories (used by the file picker; drag and
// drop calls DropFolders).
func (a *App) AddFolders(pathsJSON, peersJSON string) (string, error) {
	var paths, peers []string
	if err := jsonUnmarshal(pathsJSON, &paths); err != nil {
		return "", err
	}
	if peersJSON != "" {
		if err := jsonUnmarshal(peersJSON, &peers); err != nil {
			return "", err
		}
	}
	res, err := a.core.AddFolders(paths, peers)
	if err != nil {
		return "", err
	}
	return jsonString(res), nil
}

// DropFolders handles a drag-and-drop of filesystem paths onto the window.
func (a *App) DropFolders(paths []string) string {
	res, err := a.core.AddFolders(paths, nil)
	if err != nil {
		return jsonString(appcore.AddFolderResult{
			Rejected: []appcore.FolderConflict{{Path: "", Reason: err.Error()}},
		})
	}
	return jsonString(res)
}

// PickAndAddFolder opens a native directory chooser and shares the result.
func (a *App) PickAndAddFolder(peersJSON string) (string, error) {
	a.ctxMu.Lock()
	ctx := a.ctx
	a.ctxMu.Unlock()

	dir, err := wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title: a.core.T(appcore.CodeDialogPickFolder),
		// Start where shares live rather than wherever the dialog remembers.
		// Wails refuses a default directory that does not exist, so an
		// installation that has never created one simply gets the last
		// location back.
		DefaultDirectory: existingDir(a.core.DefaultSyncDir()),
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", nil // cancelled
	}
	return a.AddFolders(jsonString([]string{dir}), peersJSON)
}

// RemoveFolder stops sharing a folder.
func (a *App) RemoveFolder(id string, deleteDir bool) error {
	return a.core.RemoveFolder(id, deleteDir)
}

// SetFolderPaused pauses or resumes a folder.
func (a *App) SetFolderPaused(id string, paused bool) error {
	return a.core.SetFolderPaused(id, paused)
}

// RescanFolder triggers an immediate scan.
func (a *App) RescanFolder(id string) error { return a.core.RescanFolder(id) }

// FolderIgnores returns the ignore patterns of a folder as JSON.
func (a *App) FolderIgnores(id string) (string, error) {
	lines, err := a.core.FolderIgnores(id)
	if err != nil {
		return "", err
	}
	if lines == nil {
		lines = []string{}
	}
	return jsonString(lines), nil
}

// SetFolderIgnores replaces the ignore patterns of a folder.
func (a *App) SetFolderIgnores(id, patternsJSON string) error {
	var lines []string
	if err := jsonUnmarshal(patternsJSON, &lines); err != nil {
		return err
	}
	return a.core.SetFolderIgnores(id, lines)
}

// OpenFolder reveals a shared folder in the file manager.
func (a *App) OpenFolder(id string) error { return a.core.OpenFolder(id) }

// CheckFolderMove reports what moving a folder's data would involve, as JSON.
// The UI calls it before asking, so the confirmation can name the size.
func (a *App) CheckFolderMove(id, target string) (string, error) {
	check, err := a.core.CheckFolderMove(id, target)
	if err != nil {
		return "", err
	}
	return jsonString(check), nil
}

// MoveFolder relocates a folder's data and points syncthing at the new path.
func (a *App) MoveFolder(id, target string, replace bool) error {
	return a.core.MoveFolder(id, target, replace)
}

// AddDevice pairs with a remote device.
func (a *App) AddDevice(id, name string) error {
	_, err := a.core.AddDevice(id, name)
	return err
}

// RemoveDevice unpairs a device.
func (a *App) RemoveDevice(id string) error { return a.core.RemoveDevice(id) }

// RenameDevice changes a device's display name.
func (a *App) RenameDevice(id, name string) error { return a.core.RenameDevice(id, name) }

// SetDevicePaused pauses or resumes a device.
func (a *App) SetDevicePaused(id string, paused bool) error {
	return a.core.SetDevicePaused(id, paused)
}

// AcceptPendingDevice accepts a device that has been trying to connect.
func (a *App) AcceptPendingDevice(id string) error {
	return a.core.AcceptPendingDevice(id)
}

// AcceptPendingFolder accepts a folder offered by another device. When path is
// empty, a directory named after the folder is created under the default sync
// root — the same place the wizard offered, and the same one a new folder of
// the user's own would land in.
func (a *App) AcceptPendingFolder(deviceID, folderID, path string) error {
	if path == "" {
		path = filepath.Join(a.core.DefaultSyncDir(), folderID)
	}
	_, err := a.core.AcceptPendingFolder(deviceID, folderID, path)
	return err
}

// PauseAll pauses or resumes all syncing.
func (a *App) PauseAll(paused bool) error { return a.core.PauseAll(paused) }

// IsPaused reports whether syncing is paused.
func (a *App) IsPaused() bool { return a.core.Paused() }

// AutoStartEnabled reports whether the app starts at login.
func (a *App) AutoStartEnabled() bool {
	on, _ := a.core.AutoStartEnabled()
	return on
}

// SetAutoStart registers or unregisters login startup.
func (a *App) SetAutoStart(enabled bool) error { return a.core.SetAutoStart(enabled) }

// GetLogs returns the most recent log lines as JSON.
func (a *App) GetLogs(tail int) string {
	return jsonString(a.core.LogRing().Lines(tail))
}

// ---- First-run setup ------------------------------------------------------

// SetupStatus reports whether the wizard still needs to run, as JSON.
func (a *App) SetupStatus() string {
	return jsonString(a.core.SetupStatus())
}

// CompleteSetup applies the wizard's choices.
func (a *App) CompleteSetup(optsJSON string) error {
	var opts appcore.SetupOptions
	if err := jsonUnmarshal(optsJSON, &opts); err != nil {
		return err
	}
	return a.core.CompleteSetup(opts)
}

// DefaultSyncDir returns the suggested root for shared folders.
func (a *App) DefaultSyncDir() string { return a.core.DefaultSyncDir() }

// DeviceIDQRCode returns a data-URL PNG QR code for this device's ID.
func (a *App) DeviceIDQRCode() string { return a.core.DeviceIDQRCode() }

// PickDirectory opens a native directory chooser and returns the path ("" if
// the user cancelled).
func (a *App) PickDirectory(title string) (string, error) {
	return a.PickDirectoryNear(title, "")
}

// PickDirectoryNear is PickDirectory, opening at near when that is a directory
// that exists. Typing a path and then browsing from it is a common way to reach
// a subdirectory, and the chooser remembering its last location instead would
// ignore what the user just wrote.
func (a *App) PickDirectoryNear(title, near string) (string, error) {
	a.ctxMu.Lock()
	ctx := a.ctx
	a.ctxMu.Unlock()
	if title == "" {
		title = a.core.T(appcore.CodeDialogPickFolder)
	}
	start := existingDir(near)
	if start == "" {
		start = existingDir(a.core.DefaultSyncDir())
	}
	return wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: start,
	})
}

// existingDir returns path if it is a directory, and "" otherwise. It exists
// because Wails rejects an OpenDialogOptions whose DefaultDirectory is missing,
// which would turn a first run into a failure to open the dialog at all.
func existingDir(path string) string {
	if path == "" {
		return ""
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return ""
}

// ListeningPorts returns the TCP ports this instance listens on.
func (a *App) ListeningPorts() string {
	return jsonString(a.core.ListeningPorts())
}

// ---- Logs and window ------------------------------------------------------

// ShowWindow brings the main window to the front.
func (a *App) ShowWindow() {
	a.ctxMu.Lock()
	ctx := a.ctx
	a.ctxMu.Unlock()
	wailsruntime.WindowShow(ctx)
	wailsruntime.WindowUnminimise(ctx)
}

// Quit shuts the app down for real (the tray "Quit" item and UI button).
func (a *App) Quit() {
	a.ctxMu.Lock()
	ctx := a.ctx
	a.ctxMu.Unlock()
	a.quitOnce.Do(func() { close(a.quitting) })
	wailsruntime.Quit(ctx)
}

// ---- Tray ----------------------------------------------------------------

const trayMenuQuit = "quit"

func (a *App) runTray() {
	a.trayOnce.Do(func() {
		go a.trayLoop()
	})
}

// trayLoop keeps the tray icon and menu in sync with the state snapshot.
func (a *App) trayLoop() {
	updates := make(chan appcore.Snapshot, 4)
	unsub := a.core.State().Subscribe(func(s appcore.Snapshot) {
		select {
		case updates <- s:
		default:
		}
	})
	defer unsub()

	if err := tray.Run(tray.Handlers{
		OnOpen:  func() { a.ShowWindow() },
		OnQuit:  func() { a.Quit() },
		Updates: updates,
		Paused:  func() bool { return a.core.Paused() },
		TogglePause: func() error {
			return a.core.PauseAll(!a.core.Paused())
		},
		DeviceID:  func() string { return a.core.LocalDeviceID() },
		Translate: func(code appcore.Code, args ...any) string { return a.core.T(code, args...) },
	}); err != nil {
		// A missing tray is not fatal: the window still works.
		fmt.Fprintln(os.Stderr, "tray unavailable:", err)
		<-a.quitting
	}
}
