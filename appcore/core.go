// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// Package appcore embeds a full Syncthing instance and exposes a small,
// UI-friendly API on top of it. Both the desktop app and (later) the mobile
// bindings are thin shells over this package.
package appcore

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/gofrs/flock"

	"github.com/syncthing/syncthing/internal/db"
	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/events"
	"github.com/syncthing/syncthing/lib/locations"
	"github.com/syncthing/syncthing/lib/svcutil"
	"github.com/syncthing/syncthing/lib/syncthing"
	"github.com/thejerf/suture/v4"
)

// Options configures how the embedded instance is started.
type Options struct {
	// DataDir is the directory holding config.xml, keys and the database.
	// When empty a per-platform default (or a portable directory next to the
	// executable, if Portable) is used.
	DataDir string

	// Portable keeps all state next to the executable instead of in the
	// platform-default location.
	Portable bool

	// DeviceName is applied to the config on startup. When empty the
	// syncthing default (hostname) is kept.
	DeviceName string

	// SkipPortProbing keeps the default port numbers instead of probing for
	// free ones. Useful when the user wants a predictable listen port.
	SkipPortProbing bool

	// LogWriter receives syncthing's structured log output. When nil a log
	// file inside DataDir is used.
	LogWriter io.Writer
}

// App bundles the embedded syncthing instance with the helpers the UI needs.
type App struct {
	opts    Options
	dataDir string

	// mu guards the app's own preferences (currently just the language).
	mu   sync.Mutex
	lang Lang

	st          *syncthing.App
	cfg         config.Wrapper
	evLogger    events.Logger
	sdb         db.DB
	lock        *flock.Flock
	logFile     *os.File
	logRing     *RingBuffer
	earlyCancel context.CancelFunc
	early       *suture.Supervisor
	state       *StateBridge

	startOnce sync.Once
	stopOnce  sync.Once
}

// Start boots an embedded syncthing instance: it resolves the data directory,
// takes the single-instance lock, loads/generates the certificate and config,
// opens the database and runs the app. It returns once the instance is ready.
func Start(opts Options) (*App, error) {
	a := &App{opts: opts}

	if err := a.resolveDataDir(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(a.dataDir, 0o700); err != nil {
		return nil, a.errf(CodeCreateDataDir, err)
	}

	// Everything syncthing persists must live under our data dir, and the
	// default folder template should point somewhere sensible for a portable
	// app. This has to happen before the first locations.Get call, which is
	// why it is the very first thing we do.
	if err := locations.SetBaseDir(locations.ConfigBaseDir, a.dataDir); err != nil {
		return nil, err
	}
	if err := locations.SetBaseDir(locations.DataBaseDir, a.dataDir); err != nil {
		return nil, err
	}
	if err := locations.Set(locations.DefFolder, filepath.Join(a.dataDir, "Sync")); err != nil {
		return nil, err
	}

	// One instance per data directory.
	a.lock = flock.New(locations.Get(locations.LockFile))
	locked, err := a.lock.TryLock()
	if err != nil {
		return nil, a.errf(CodeAcquireLock, err)
	}
	if !locked {
		return nil, ErrAlreadyRunning
	}

	if err := a.setupLogging(); err != nil {
		a.unlock()
		return nil, err
	}
	a.loadPrefs()

	slog.Info("Starting SyncthingApp", "dataDir", a.dataDir, "portable", a.opts.Portable, "os", runtime.GOOS)

	cert, err := syncthing.LoadOrGenerateCertificate(
		locations.Get(locations.CertFile),
		locations.Get(locations.KeyFile),
	)
	if err != nil {
		a.shutdown()
		return nil, a.errf(CodeCertificate, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.earlyCancel = cancel
	// The config wrapper must run as a suture service, otherwise Modify calls
	// block forever waiting on its command queue.
	a.early = suture.New("early", svcutil.SpecWithDebugLogger())
	a.early.ServeBackground(ctx)

	a.evLogger = events.NewLogger()
	a.early.Add(a.evLogger)

	cfgWrapper, err := syncthing.LoadConfigAtStartup(
		locations.Get(locations.ConfigFile), cert, a.evLogger, false, a.opts.SkipPortProbing,
	)
	if err != nil {
		a.shutdown()
		return nil, a.errf(CodeLoadConfig, err)
	}
	a.cfg = cfgWrapper
	a.early.Add(cfgWrapper)

	// The embedded app never serves the web GUI; the native window is the
	// whole user interface. StartBrowser stays off for the same reason.
	if _, err := cfgWrapper.Modify(func(cfg *config.Configuration) {
		cfg.GUI.Enabled = false
		cfg.Options.StartBrowser = false
		cfg.Options.AutoUpgradeIntervalH = 0
		if a.opts.DeviceName != "" {
			if dev, _, ok := cfg.Device(cfgWrapper.MyID()); ok {
				dev.Name = a.opts.DeviceName
				cfg.SetDevice(dev)
			}
		}
	}); err != nil {
		a.shutdown()
		return nil, a.errf(CodeApplyConfig, err)
	}

	a.sdb, err = syncthing.OpenDatabase(locations.Get(locations.Database), 0)
	if err != nil {
		a.shutdown()
		return nil, a.errf(CodeOpenDatabase, err)
	}

	a.st, err = syncthing.New(cfgWrapper, a.sdb, a.evLogger, cert, syncthing.Options{
		NoUpgrade: true,
	})
	if err != nil {
		a.shutdown()
		return nil, a.errf(CodeCreateApp, err)
	}

	a.state = NewStateBridge(a.evLogger, a.st.Internals, cfgWrapper)
	a.state.Start()

	if err := a.st.Start(); err != nil {
		a.shutdown()
		return nil, a.errf(CodeStartApp, err)
	}

	// The model internals only exist once the app has started.
	a.state.SetInternals(a.st.Internals)
	a.state.Refresh()

	slog.Info("SyncthingApp ready", "deviceID", a.cfg.MyID().String())
	return a, nil
}

// Wait blocks until the instance stops and returns its exit status.
func (a *App) Wait() svcutil.ExitStatus {
	return a.st.Wait()
}

// Stop shuts the instance down gracefully. Safe to call more than once.
func (a *App) Stop() {
	a.shutdown()
}

// DeviceID returns this instance's device ID.
func (a *App) DeviceID() string {
	return a.cfg.MyID().String()
}

// Config exposes the configuration wrapper for the feature helpers.
func (a *App) Config() config.Wrapper {
	return a.cfg
}

// Events exposes the event logger.
func (a *App) Events() events.Logger {
	return a.evLogger
}

// State exposes the aggregated state bridge.
func (a *App) State() *StateBridge {
	return a.state
}

// SetInternals attaches the model internals to the state bridge. The internals
// only exist after the app has started, and the bridge is created before that
// so it can catch the earliest events.
func (a *App) SetInternals(i *syncthing.Internals) {
	a.state.SetInternals(i)
}

// DataDir returns the resolved data directory.
func (a *App) DataDir() string {
	return a.dataDir
}

// LogRing returns the in-memory log buffer backing the UI log view.
func (a *App) LogRing() *RingBuffer {
	return a.logRing
}

func (a *App) shutdown() {
	a.stopOnce.Do(func() {
		if a.state != nil {
			a.state.Stop()
		}
		if a.st != nil {
			a.st.Stop(svcutil.ExitSuccess)
			a.st.Wait()
		}
		if a.earlyCancel != nil {
			a.earlyCancel()
		}
		if a.sdb != nil {
			_ = a.sdb.Close()
		}
		a.unlock()
		if a.logFile != nil {
			_ = a.logFile.Close()
		}
	})
}

func (a *App) unlock() {
	if a.lock != nil {
		_ = a.lock.Unlock()
		_ = os.Remove(a.lock.Path())
		a.lock = nil
	}
}

// portableMarker is the file next to the executable that switches the app into
// portable mode.
const portableMarker = "SyncthingApp.portable"

// PortableRequested reports whether the layout next to the executable asks for
// portable mode.
func PortableRequested() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(exe), portableMarker))
	return err == nil
}

func (a *App) resolveDataDir() error {
	if a.opts.DataDir != "" {
		abs, err := filepath.Abs(a.opts.DataDir)
		if err != nil {
			return err
		}
		a.dataDir = abs
		return nil
	}
	if a.opts.Portable || PortableRequested() {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		a.dataDir = filepath.Join(filepath.Dir(exe), "Data")
		return nil
	}
	dir, err := defaultDataDir()
	if err != nil {
		return err
	}
	a.dataDir = dir
	return nil
}

// defaultDataDir returns the per-platform state directory. It deliberately
// differs from the official syncthing location so this app never clashes with
// a separately installed syncthing (config, database and lock file).
func defaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LocalAppData")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "SyncthingApp"), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "SyncthingApp"), nil
	default:
		if state := os.Getenv("XDG_STATE_HOME"); state != "" {
			return filepath.Join(state, "syncthingapp"), nil
		}
		return filepath.Join(home, ".local", "state", "syncthingapp"), nil
	}
}

// setupLogging routes syncthing's slog output into a file inside the data
// directory plus an in-memory ring buffer that the UI can display.
func (a *App) setupLogging() error {
	a.logRing = NewRingBuffer(2000)

	writers := []io.Writer{a.logRing}
	if a.opts.LogWriter != nil {
		writers = append(writers, a.opts.LogWriter)
	} else {
		f, err := os.OpenFile(
			filepath.Join(a.dataDir, "syncthing-app.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600,
		)
		if err != nil {
			return a.errf(CodeOpenLog, err)
		}
		a.logFile = f
		writers = append(writers, f)
	}

	handler := slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))
	return nil
}
