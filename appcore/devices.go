// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"strings"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/protocol"
	"github.com/syncthing/syncthing/lib/structutil"
)

// MyDevice returns this instance's own device entry.
func (a *App) MyDevice() DeviceState {
	myID := a.cfg.MyID()
	d, _ := a.cfg.Device(myID)
	return DeviceState{
		ID:   myID.String(),
		Name: d.Name,
	}
}

// Devices returns the configured devices, including this one.
func (a *App) Devices() []DeviceState {
	return a.state.Snapshot().Devices
}

// AddDevice pairs with a remote device by ID. name is optional and defaults to
// the short device ID.
func (a *App) AddDevice(idStr, name string) (DeviceState, error) {
	id, err := protocol.DeviceIDFromString(strings.TrimSpace(idStr))
	if err != nil {
		return DeviceState{}, a.errf(CodeInvalidDeviceID, err)
	}
	if id == a.cfg.MyID() {
		return DeviceState{}, ErrSelfDevice
	}
	if existing, ok := a.cfg.Device(id); ok {
		// Refresh the name if the caller supplied one and it changed.
		if name != "" && name != existing.Name {
			existing.Name = name
			if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
				cfg.SetDevice(existing)
			}); err != nil {
				return DeviceState{}, err
			}
			a.state.RefreshSoon()
		}
		return DeviceState{ID: id.String(), Name: existing.Name}, nil
	}

	if name == "" {
		name = id.Short().String()
	}
	device := a.cfg.DefaultDevice().Copy()
	device.DeviceID = id
	device.Name = name
	if len(device.Addresses) == 0 {
		device.Addresses = []string{"dynamic"}
	}

	if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
		cfg.SetDevice(device)
	}); err != nil {
		return DeviceState{}, a.errf(CodeSaveConfig, err)
	}

	a.state.RefreshSoon()
	return DeviceState{ID: id.String(), Name: name}, nil
}

// RemoveDevice unpairs a device.
func (a *App) RemoveDevice(idStr string) error {
	id, err := protocol.DeviceIDFromString(idStr)
	if err != nil {
		return a.errf(CodeInvalidDeviceID, err)
	}
	if id == a.cfg.MyID() {
		return ErrSelfDevice
	}
	waiter, err := a.cfg.RemoveDevice(id)
	if err != nil {
		return err
	}
	if waiter != nil {
		waiter.Wait()
	}
	a.state.Refresh()
	return nil
}

// RenameDevice updates the display name of a paired device.
// RenameDevice changes the display name this app uses for a paired device.
//
// The name is local. There is no protocol message that renames a device on the
// other side, and syncthing does the opposite instead: on every connection a
// device announces its own name, and a peer adopts it when its stored name for
// that device is empty or when OverwriteRemoteDevNames is on. That option is
// off here, so a name set in this app sticks until the device is removed and
// re-added. What the other side calls itself is its own business, and what the
// hub calls this device is likewise the hub's copy of the name announced at
// connect time — see SetDeviceName.
func (a *App) RenameDevice(idStr, name string) error {
	id, err := protocol.DeviceIDFromString(idStr)
	if err != nil {
		return a.errf(CodeInvalidDeviceID, err)
	}
	device, ok := a.cfg.Device(id)
	if !ok {
		return a.errf(CodeUnknownDevice, idStr)
	}
	device.Name = name
	if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
		cfg.SetDevice(device)
	}); err != nil {
		return err
	}
	a.state.RefreshSoon()
	return nil
}

// SetDevicePaused pauses or resumes syncing with a device.
func (a *App) SetDevicePaused(idStr string, paused bool) error {
	id, err := protocol.DeviceIDFromString(idStr)
	if err != nil {
		return a.errf(CodeInvalidDeviceID, err)
	}
	device, ok := a.cfg.Device(id)
	if !ok {
		return a.errf(CodeUnknownDevice, idStr)
	}
	device.Paused = paused
	if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
		cfg.SetDevice(device)
	}); err != nil {
		return err
	}
	a.state.RefreshSoon()
	return nil
}

// PendingDevices returns devices that want to connect but are not paired yet.
func (a *App) PendingDevices() []PendingDevice {
	return a.state.Snapshot().PendingDevices
}

// AcceptPendingDevice pairs with a device that has been trying to connect.
func (a *App) AcceptPendingDevice(idStr string) error {
	name := ""
	for _, p := range a.PendingDevices() {
		if p.ID == idStr {
			name = p.Name
			break
		}
	}
	_, err := a.AddDevice(idStr, name)
	return err
}

// PendingFolders returns folders other devices want to share with us.
func (a *App) PendingFolders() []PendingFolder {
	return a.state.Snapshot().PendingFolders
}

// AcceptPendingFolder adds a folder that another device has offered. path is
// where the files should live locally.
func (a *App) AcceptPendingFolder(deviceID, folderID, path string) (FolderState, error) {
	devID, err := protocol.DeviceIDFromString(deviceID)
	if err != nil {
		return FolderState{}, a.errf(CodeInvalidDeviceID, err)
	}
	path, problem := ensureFolderRoot(path)
	if problem != nil {
		return FolderState{}, &localizedError{code: problem.Code, args: problem.Args, lang: a.Language()}
	}
	if _, ok := a.cfg.Folder(folderID); ok {
		return FolderState{}, ErrDuplicateFolder
	}

	// The label the offering device used, if we still have it recorded.
	label := folderID
	if pf := a.state.Snapshot().PendingFolders; pf != nil {
		for _, p := range pf {
			if p.ID == folderID && p.DeviceID == deviceID {
				label = p.Label
				break
			}
		}
	}

	folder := a.cfg.DefaultFolder().Copy()
	structutil.SetDefaults(&folder)
	folder.ID = folderID
	folder.Label = label
	folder.Path = path
	folder.Devices = []config.FolderDeviceConfiguration{
		{DeviceID: a.cfg.MyID()},
		{DeviceID: devID},
	}

	if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
		cfg.SetFolder(folder)
	}); err != nil {
		return FolderState{}, a.errf(CodeSaveConfig, err)
	}

	a.state.RefreshSoon()
	return FolderState{ID: folderID, Label: label, Path: path, Progress: 100}, nil
}

// ShareFolderWith adds a device to an existing folder's device list.
func (a *App) ShareFolderWith(folderID, deviceID string) error {
	folder, ok := a.cfg.Folder(folderID)
	if !ok {
		return a.errf(CodeNoSuchFolder, folderID)
	}
	id, err := protocol.DeviceIDFromString(deviceID)
	if err != nil {
		return a.errf(CodeInvalidDeviceID, err)
	}
	for _, d := range folder.Devices {
		if d.DeviceID == id {
			return nil
		}
	}
	folder.Devices = append(folder.Devices, config.FolderDeviceConfiguration{DeviceID: id})
	if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
		cfg.SetFolder(folder)
	}); err != nil {
		return err
	}
	a.state.RefreshSoon()
	return nil
}

// UnshareFolderWith removes a device from an existing folder's device list.
func (a *App) UnshareFolderWith(folderID, deviceID string) error {
	folder, ok := a.cfg.Folder(folderID)
	if !ok {
		return a.errf(CodeNoSuchFolder, folderID)
	}
	id, err := protocol.DeviceIDFromString(deviceID)
	if err != nil {
		return a.errf(CodeInvalidDeviceID, err)
	}
	if id == a.cfg.MyID() {
		return ErrSelfDevice
	}
	kept := folder.Devices[:0]
	for _, d := range folder.Devices {
		if d.DeviceID != id {
			kept = append(kept, d)
		}
	}
	folder.Devices = kept
	if _, err := a.cfg.Modify(func(cfg *config.Configuration) {
		cfg.SetFolder(folder)
	}); err != nil {
		return err
	}
	a.state.RefreshSoon()
	return nil
}
