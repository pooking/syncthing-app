// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/protocol"
	"github.com/syncthing/syncthing/lib/structutil"
)

// FolderConflict describes a rejection or warning for a drop operation.
// Reason is already rendered in the user's language; Code and Args are carried
// along so the frontend can re-render it after a language switch.
type FolderConflict struct {
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
	Code   Code   `json:"code,omitempty"`
	Args   []any  `json:"args,omitempty"`
}

// AddFolderResult reports the outcome of adding one or more directories.
type AddFolderResult struct {
	Added    []FolderState    `json:"added"`
	Rejected []FolderConflict `json:"rejected"`
	Warnings []FolderConflict `json:"warnings"`
}

// Folders returns the currently configured folders as UI states.
func (a *App) Folders() []FolderState {
	return a.state.Snapshot().Folders
}

// AddFolder shares a single directory. shareWith lists optional peer device
// IDs that should also receive the folder. It returns an error suitable for
// display when the folder cannot be added.
func (a *App) AddFolder(path string, shareWith []string) (FolderState, error) {
	res, err := a.AddFolders([]string{path}, shareWith)
	if err != nil {
		return FolderState{}, err
	}
	if len(res.Rejected) > 0 {
		return FolderState{}, &localizedError{
			code: res.Rejected[0].Code, args: res.Rejected[0].Args, lang: a.Language(),
		}
	}
	if len(res.Added) == 0 {
		return FolderState{}, ErrDuplicateFolder
	}
	return res.Added[0], nil
}

// AddFolders shares several directories at once, as produced by a drag-and-drop
// operation. Directories that cannot be added are reported in the result
// rather than failing the whole operation.
func (a *App) AddFolders(paths []string, shareWith []string) (AddFolderResult, error) {
	res := AddFolderResult{}

	// Validate peer devices up front.
	peers := make([]protocol.DeviceID, 0, len(shareWith))
	for _, s := range shareWith {
		id, err := protocol.DeviceIDFromString(s)
		if err != nil {
			return res, a.errf(CodeInvalidDeviceID, err)
		}
		if _, ok := a.cfg.Device(id); !ok {
			return res, a.errf(CodeUnknownDevice, s)
		}
		peers = append(peers, id)
	}

	type toAdd struct {
		path  string
		id    string
		label string
	}
	var pending []toAdd
	existing := a.cfg.FolderList()

	for _, raw := range paths {
		path, problem := normalizeFolderPath(raw)
		if problem != nil {
			res.Rejected = append(res.Rejected, FolderConflict{
				Path:   raw,
				Reason: problem.Error(),
				Code:   problem.Code,
				Args:   problem.Args,
			})
			continue
		}

		if dup, ok := findFolderByPath(existing, path); ok {
			res.Rejected = append(res.Rejected, FolderConflict{
				Path:   raw,
				Reason: a.T(CodeAlreadyShared, dup.Label),
				Code:   CodeAlreadyShared,
				Args:   []any{dup.Label},
			})
			continue
		}
		already := false
		for _, p := range pending {
			if samePath(p.path, path) {
				already = true
				break
			}
		}
		if already {
			continue
		}

		// Nested sync is allowed but discouraged: report it as a warning.
		for _, f := range existing {
			if other, problem := normalizeFolderPath(f.Path); problem == nil && isNested(path, other) {
				res.Warnings = append(res.Warnings, FolderConflict{
					Path:   raw,
					Reason: a.T(CodeOverlaps, f.Label),
					Code:   CodeOverlaps,
					Args:   []any{f.Label},
				})
			}
		}

		pending = append(pending, toAdd{
			path:  path,
			id:    newFolderID(path, existing),
			label: filepath.Base(path),
		})
	}

	if len(pending) == 0 {
		return res, nil
	}

	myID := a.cfg.MyID()
	_, err := a.cfg.Modify(func(cfg *config.Configuration) {
		for _, p := range pending {
			folder := cfg.Defaults.Folder.Copy()
			structutil.SetDefaults(&folder)
			folder.ID = p.id
			folder.Label = p.label
			folder.Path = p.path
			// Own device plus any peers selected by the user. The config
			// preparation step guarantees our own device is present.
			folder.Devices = []config.FolderDeviceConfiguration{{DeviceID: myID}}
			for _, peer := range peers {
				folder.Devices = append(folder.Devices, config.FolderDeviceConfiguration{DeviceID: peer})
			}
			cfg.SetFolder(folder)
		}
	})
	if err != nil {
		return res, a.errf(CodeSaveConfig, err)
	}

	a.state.RefreshSoon()

	snap := a.state.Snapshot()
	for _, p := range pending {
		state := FolderState{ID: p.id, Label: p.label, Path: p.path, Progress: 100}
		for _, f := range snap.Folders {
			if f.ID == p.id {
				state = f
				break
			}
		}
		res.Added = append(res.Added, state)
	}
	return res, nil
}

// RemoveFolder stops sharing a folder. When deleteDir is true the local copy of
// the files is deleted as well.
func (a *App) RemoveFolder(id string, deleteDir bool) error {
	folder, ok := a.cfg.Folder(id)
	if !ok {
		return a.errf(CodeNoSuchFolder, id)
	}
	waiter, err := a.cfg.RemoveFolder(id)
	if err != nil {
		return err
	}
	if waiter != nil {
		waiter.Wait()
	}
	if deleteDir {
		if _, problem := normalizeFolderPath(folder.Path); problem == nil {
			if err := os.RemoveAll(folder.Path); err != nil {
				return a.errf(CodeRemoveDirectory, err)
			}
		}
	}
	a.state.Refresh()
	return nil
}

// SetFolderPaused pauses or resumes a single folder.
func (a *App) SetFolderPaused(id string, paused bool) error {
	folder, ok := a.cfg.Folder(id)
	if !ok {
		return a.errf(CodeNoSuchFolder, id)
	}
	folder.Paused = paused
	waiter, err := a.cfg.Modify(func(cfg *config.Configuration) {
		cfg.SetFolder(folder)
	})
	if err != nil {
		return err
	}
	if waiter != nil {
		waiter.Wait()
	}
	a.state.RefreshSoon()
	return nil
}

// RescanFolder triggers an immediate scan of a folder.
func (a *App) RescanFolder(id string) error {
	if _, ok := a.cfg.Folder(id); !ok {
		return a.errf(CodeNoSuchFolder, id)
	}
	return a.st.Internals.ScanFolderSubdirs(id, nil)
}

// FolderIgnores returns the ignore patterns for a folder.
func (a *App) FolderIgnores(id string) ([]string, error) {
	lines, _, err := a.st.Internals.Ignores(id)
	return lines, err
}

// SetFolderIgnores replaces the ignore patterns for a folder.
func (a *App) SetFolderIgnores(id string, lines []string) error {
	if _, ok := a.cfg.Folder(id); !ok {
		return a.errf(CodeNoSuchFolder, id)
	}
	return a.st.Internals.SetIgnores(id, lines)
}

// OpenFolder opens a shared folder in the system file manager.
func (a *App) OpenFolder(id string) error {
	folder, ok := a.cfg.Folder(id)
	if !ok {
		return a.errf(CodeNoSuchFolder, id)
	}
	return openPath(folder.Path)
}

// normalizeFolderPath validates and cleans a dropped path. It returns a coded
// problem when the path cannot be used as a folder root.
func normalizeFolderPath(raw string) (string, *UserError) {
	if raw == "" {
		return "", NewUserError(CodeEmptyPath)
	}
	path, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", NewUserError(CodeCannotResolve)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", NewUserError(CodePathMissing)
	}
	if !info.IsDir() {
		return "", NewUserError(CodeNotADirectory)
	}
	if path == filepath.VolumeName(path)+string(filepath.Separator) {
		return "", NewUserError(CodeFilesystemRoot)
	}
	return path, nil
}

// findFolderByPath looks for an existing folder sharing the same directory.
func findFolderByPath(folders []config.FolderConfiguration, path string) (config.FolderConfiguration, bool) {
	for _, f := range folders {
		if samePath(f.Path, path) {
			return f, true
		}
	}
	return config.FolderConfiguration{}, false
}

// samePath compares two filesystem paths, honouring case-insensitivity on
// platforms where that applies.
func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// isNested reports whether one path is inside the other.
func isNested(a, b string) bool {
	sep := string(filepath.Separator)
	return strings.HasPrefix(a+sep, b+sep) || strings.HasPrefix(b+sep, a+sep)
}

// newFolderID derives a stable-looking, unique folder ID from the directory
// name, appending a short random suffix when needed.
func newFolderID(path string, existing []config.FolderConfiguration) string {
	taken := make(map[string]bool, len(existing))
	for _, f := range existing {
		taken[f.ID] = true
	}
	base := sanitizeID(filepath.Base(path))
	for i := 0; i < 100; i++ {
		id := base
		if i > 0 {
			id = fmt.Sprintf("%s-%d", base, i)
		}
		if !taken[id] {
			return id
		}
	}
	return base + "-" + randHex(4)
}

func sanitizeID(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "folder"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func randHex(n int) string {
	const hexdigits = "0123456789abcdef"
	buf := make([]byte, n/2+1)
	_, _ = rand.Read(buf)
	var b strings.Builder
	for _, v := range buf {
		b.WriteByte(hexdigits[v>>4])
		b.WriteByte(hexdigits[v&0x0f])
	}
	return b.String()[:n]
}

// sortedFolderPaths is a small helper used by tests and callers that need a
// deterministic listing.
func sortedFolderPaths(folders []FolderState) []string {
	out := make([]string, 0, len(folders))
	for _, f := range folders {
		out = append(out, f.Path)
	}
	sort.Strings(out)
	return out
}
