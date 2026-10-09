// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/syncthing/syncthing/lib/locations"
)

// The data directory can be relocated, but finding out where it lives cannot
// itself depend on the data directory. Two small marker files bridge that:
//
//   - dataDirPointerFile lives in the platform-default directory and holds the
//     chosen location; it is what makes the choice survive a restart.
//   - dataDirMigrateFile lives in the current directory and asks the *next*
//     start to copy everything across before opening anything.
//
// The split exists because the database cannot be copied while it is open:
// running SQLite carries WAL side files, so a live copy can be silently torn.
const (
	dataDirPointerFile = "data-dir.txt"
	dataDirMigrateFile = "migrate-to.txt"
)

// restartDelaySeconds is how long the relaunch helper waits before starting the
// app again, so the old process has released its locks. spawnRestart uses it.
const restartDelaySeconds = 2

// dataDirBaseDir returns the directory that holds the pointer file. It is
// deliberately not affected by the pointer itself, so it cannot move.
func dataDirBaseDir() (string, error) {
	return defaultDataDir()
}

// readPointerFile returns the path recorded in a one-line pointer file, or ""
// when the file is missing, empty or unreadable. A damaged marker is ignored
// rather than fatal: falling back to the default directory keeps the app
// running, and the setting can simply be chosen again.
func readPointerFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		return ""
	}
	abs, err := filepath.Abs(line)
	if err != nil {
		return ""
	}
	return abs
}

// writePointerFile records a path in a one-line file, creating the directory
// that holds it when necessary.
func writePointerFile(path, target string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(target+"\n"), 0o600)
}

// resolveDataDirIfNeeded is what resolveDataDir calls for the non-portable
// case: it honours the pointer file and completes a pending migration.
func resolveDataDirIfNeeded() (string, error) {
	base, err := dataDirBaseDir()
	if err != nil {
		return "", err
	}

	current := base
	if custom := readPointerFile(filepath.Join(base, dataDirPointerFile)); custom != "" {
		current = custom
	}
	// A migration recorded before the restart that follows it. Both files are
	// normally in the same directory, but the pointer may have been moved
	// since, so only the current directory is consulted for the request.
	pending := readPointerFile(filepath.Join(current, dataDirMigrateFile))
	if pending == "" || samePath(pending, current) {
		return current, nil
	}

	slog.Info("Completing data directory migration", "from", current, "to", pending)
	if err := copyDir(current, pending, map[string]bool{dataDirMigrateFile: true, "syncthing.lock": true}); err != nil {
		return "", fmt.Errorf("copy %s to %s: %w", current, pending, err)
	}
	// The pointer is written last and to the fixed base directory: until it
	// lands, a crash leaves the app on the old directory with the migration
	// still queued, which the next start retries.
	if err := writePointerFile(filepath.Join(base, dataDirPointerFile), pending); err != nil {
		return "", fmt.Errorf("record new data directory: %w", err)
	}
	_ = os.Remove(filepath.Join(current, dataDirMigrateFile))

	// locations must follow the new directory even though the caller sets the
	// base dir again afterwards; this keeps a direct call consistent.
	if err := setLocationBaseDirs(pending); err != nil {
		return "", err
	}
	return pending, nil
}

// MoveDataDir validates a new data directory, queues the migration and
// restarts the app so the copy happens before anything is opened. It returns
// only when the request cannot even be recorded; on success the process is
// already on its way out.
func (a *App) MoveDataDir(newPath string) error {
	// Portable mode has no pointer file to write, so relocating it from the UI
	// would silently do nothing: the next start would simply pick the portable
	// directory again. Refuse rather than pretend.
	if a.opts.Portable || PortableRequested() {
		return a.errf(CodeDataMovePortable)
	}

	folders := make([]string, 0, len(a.cfg.FolderList()))
	for _, f := range a.cfg.FolderList() {
		folders = append(folders, f.Path)
	}

	target, err := validateDataDirMove(a.dataDir, newPath, folders)
	if err != nil {
		if ue, ok := err.(*UserError); ok {
			return &localizedError{code: ue.Code, args: ue.Args, lang: a.Language()}
		}
		return err
	}

	if err := os.WriteFile(filepath.Join(a.dataDir, dataDirMigrateFile), []byte(target+"\n"), 0o600); err != nil {
		return a.errf(CodeDataMoveWrite, err)
	}
	slog.Info("Data directory migration requested", "from", a.dataDir, "to", target)

	if err := restartProcess(); err != nil {
		// The request is already recorded, so a manual restart still performs
		// the migration. Report it, but leave the marker in place.
		slog.Warn("Could not restart automatically after requesting a data directory move", "error", err)
		return a.errf(CodeRestart, err)
	}
	return nil
}

// validateDataDirMove is the pure part of MoveDataDir: it rejects a target
// that cannot work before anything is written.
func validateDataDirMove(current, newPath string, folderPaths []string) (string, error) {
	if strings.TrimSpace(newPath) == "" {
		return "", NewUserError(CodeEmptyPath)
	}
	target, err := filepath.Abs(filepath.Clean(newPath))
	if err != nil {
		return "", NewUserError(CodeCannotResolve)
	}
	if target == filepath.VolumeName(target)+string(filepath.Separator) {
		return "", NewUserError(CodeFilesystemRoot)
	}
	if samePath(target, current) {
		return "", NewUserError(CodeDataMoveSame)
	}
	// An existing file (as opposed to a directory) can never be a data dir.
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		return "", NewUserError(CodeNotADirectory)
	}
	// The data directory must not sit inside a shared folder or contain one:
	// syncthing would then be indexing the database it is writing to.
	for _, p := range folderPaths {
		if p == "" {
			continue
		}
		if isNested(target, p) {
			return "", NewUserError(CodeDataMoveNested, filepath.Base(p))
		}
	}
	// Creating the directory is also the writability test, since a location we
	// cannot create is one we cannot use.
	if err := os.MkdirAll(target, 0o700); err != nil {
		return "", NewUserError(CodeDataMoveWrite, err)
	}
	probe := filepath.Join(target, "."+dataDirMigrateFile+".probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return "", NewUserError(CodeDataMoveWrite, err)
	}
	_ = os.Remove(probe)
	return target, nil
}

// copyDir copies the contents of src into dst, creating dst as needed. Names in
// skip are not copied, which keeps the pending-migration marker from being
// carried along and dropping the lock file that belongs to the old location.
func copyDir(src, dst string, skip map[string]bool) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	for _, entry := range entries {
		if skip[entry.Name()] {
			continue
		}
		from := filepath.Join(src, entry.Name())
		to := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(from, to, skip); err != nil {
				return err
			}
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			// A symlinked entry may point outside the data directory; copying
			// the link itself is both cheaper and safer than following it.
			target, err := os.Readlink(from)
			if err != nil {
				return err
			}
			_ = os.Remove(to)
			if err := os.Symlink(target, to); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

// restartProcess launches a second copy of this executable and lets the caller
// exit, so a queued migration is applied by a fresh start. The delay gives the
// current process time to give up its single-instance locks first, and the
// helper is spawned detached so no console flashes up behind the window.
//
// It returns once the helper is running, which does not guarantee the app comes
// back: a helper that cannot find the executable reports nothing. The pending
// marker is what makes that recoverable — a manual start still migrates.
func restartProcess() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return spawnRestart(exe)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// setLocationBaseDirs points syncthing's locations at a data directory. It is
// also the first thing Start does; here it is needed because resolveDataDir may
// move the directory after Start has begun.
func setLocationBaseDirs(dir string) error {
	if err := locations.SetBaseDir(locations.ConfigBaseDir, dir); err != nil {
		return err
	}
	if err := locations.SetBaseDir(locations.DataBaseDir, dir); err != nil {
		return err
	}
	return locations.Set(locations.DefFolder, filepath.Join(dir, "Sync"))
}
