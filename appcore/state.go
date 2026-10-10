// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"encoding/json"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/syncthing/syncthing/internal/slogutil"
	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/events"
	"github.com/syncthing/syncthing/lib/model"
	"github.com/syncthing/syncthing/lib/protocol"
	"github.com/syncthing/syncthing/lib/syncthing"
)

// Phase describes the overall lifecycle state of the instance.
type Phase string

const (
	PhaseStarting Phase = "starting"
	PhaseReady    Phase = "ready"
	PhaseError    Phase = "error"
)

// FolderState is the UI-facing view of a single shared folder.
type FolderState struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Path         string `json:"path"`
	Type         string `json:"type"`
	Paused       bool   `json:"paused"`
	State        string `json:"state"`
	StateChanged string `json:"stateChanged,omitempty"`
	Errors       int    `json:"errors"`
	NeedBytes    int64  `json:"needBytes"`
	NeedItems    int    `json:"needItems"`
	GlobalBytes  int64  `json:"globalBytes"`
	LocalBytes   int64  `json:"localBytes"`
	// Progress is 0..100 for the local device's view of this folder.
	Progress float64 `json:"progress"`
}

// DeviceState is the UI-facing view of a paired device.
type DeviceState struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Connected  bool   `json:"connected"`
	Paused     bool   `json:"paused"`
	LastSeen   string `json:"lastSeen,omitempty"`
	Address    string `json:"address,omitempty"`
	Introducer bool   `json:"introducer"`
}

// PendingDevice is a device that wants to connect but is not yet configured.
type PendingDevice struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Time    string `json:"time"`
}

// PendingFolder is a folder another device wants to share with us.
type PendingFolder struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	DeviceID   string `json:"deviceID"`
	DeviceName string `json:"deviceName"`
	OfferedAt  string `json:"offeredAt"`
}

// Snapshot is a full, self-contained picture of the instance. UIs render it
// directly; the tray derives its icon from it.
type Snapshot struct {
	Phase           Phase           `json:"phase"`
	DeviceID        string          `json:"deviceID"`
	MyName          string          `json:"myName"`
	Folders         []FolderState   `json:"folders"`
	Devices         []DeviceState   `json:"devices"`
	PendingDevices  []PendingDevice `json:"pendingDevices"`
	PendingFolders  []PendingFolder `json:"pendingFolders"`
	ActiveItems     int             `json:"activeItems"`
	TotalNeedBytes  int64           `json:"totalNeedBytes"`
	TotalNeedItems  int             `json:"totalNeedItems"`
	Errors          []string        `json:"errors"`
	AutoStart       bool            `json:"autoStart"`
	GeneratedAtUnix int64           `json:"generatedAtUnix"`
	// UpdatedAt is bumped whenever the snapshot changes.
	UpdatedAtUnix int64 `json:"updatedAtUnix"`
}

// StateBridge subscribes to syncthing's event bus, folds the events into a
// Snapshot and notifies observers. It is the single source of truth shared by
// the desktop window, the tray icon and (later) the mobile bindings.
type StateBridge struct {
	evLogger  events.Logger
	internals *syncthing.Internals
	cfg       config.Wrapper

	mu           sync.Mutex
	snapshot     Snapshot
	folderStates map[string]*FolderState
	activeItems  int
	observers    map[int]func(Snapshot)

	sub     events.Subscription
	stop    chan struct{}
	notify  chan struct{}
	rebuild chan struct{}
	wg      sync.WaitGroup
	nextID  int
}

// NewStateBridge creates a bridge. Call Start to begin consuming events.
func NewStateBridge(evLogger events.Logger, internals *syncthing.Internals, cfg config.Wrapper) *StateBridge {
	return &StateBridge{
		evLogger:  evLogger,
		internals: internals,
		cfg:       cfg,
		observers: make(map[int]func(Snapshot)),
		stop:      make(chan struct{}),
		notify:    make(chan struct{}, 1),
		rebuild:   make(chan struct{}, 1),
		snapshot: Snapshot{
			Phase:    PhaseStarting,
			DeviceID: cfg.MyID().String(),
		},
		folderStates: make(map[string]*FolderState),
	}
}

// SnapshotJSON serialises a snapshot for transport to a UI layer (the desktop
// window and the future mobile bindings share this encoding).
func SnapshotJSON(s Snapshot) string {
	bs, err := json.Marshal(s)
	if err != nil {
		return `{"phase":"error"}`
	}
	return string(bs)
}

// SetInternals attaches the model internals once the app has started. Safe to
// call from any goroutine; the bridge takes its own lock.
func (s *StateBridge) SetInternals(i *syncthing.Internals) {
	s.mu.Lock()
	s.internals = i
	s.mu.Unlock()
}

// eventMask is the set of events that influence the snapshot. The high-volume
// change-detection events are deliberately excluded.
const eventMask = events.Starting |
	events.StartupComplete |
	events.DeviceConnected |
	events.DeviceDisconnected |
	events.DevicePaused |
	events.DeviceResumed |
	events.PendingDevicesChanged |
	events.PendingFoldersChanged |
	events.StateChanged |
	events.FolderSummary |
	events.FolderErrors |
	events.FolderPaused |
	events.FolderResumed |
	events.ItemStarted |
	events.ItemFinished |
	events.ConfigSaved |
	events.FolderScanProgress

func (s *StateBridge) Start() {
	s.sub = s.evLogger.Subscribe(eventMask)
	s.rebuildStatic()
	s.wg.Add(3)
	go s.consume()
	go s.dispatch()
	go s.rebuilder()
}

// rebuilder folds pending-device/folder/config changes into the snapshot. It is
// debounced so a burst of events results in a single database-backed rebuild.
func (s *StateBridge) rebuilder() {
	defer s.wg.Done()
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	for {
		select {
		case <-s.stop:
			return
		case <-s.rebuild:
			timer.Reset(250 * time.Millisecond)
		case <-timer.C:
			s.rebuildStatic()
		}
	}
}

func (s *StateBridge) Stop() {
	select {
	case <-s.stop:
		return
	default:
	}
	close(s.stop)
	if s.sub != nil {
		s.sub.Unsubscribe()
	}
	s.wg.Wait()
}

// Subscribe registers an observer and immediately delivers the current
// snapshot. The returned func removes the observer.
func (s *StateBridge) Subscribe(fn func(Snapshot)) func() {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.observers[id] = fn
	cur := s.snapshot
	s.mu.Unlock()

	fn(cur)

	return func() {
		s.mu.Lock()
		delete(s.observers, id)
		s.mu.Unlock()
	}
}

// Snapshot returns the current snapshot.
func (s *StateBridge) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot
}

// Refresh rebuilds the config-derived parts of the snapshot (folders, devices,
// pending entries) and notifies observers. Call after any config change.
func (s *StateBridge) Refresh() {
	s.rebuildStatic()
	s.changed()
}

func (s *StateBridge) consume() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stop:
			return
		case ev, ok := <-s.sub.C():
			if !ok {
				return
			}
			s.apply(ev)
		}
	}
}

func (s *StateBridge) apply(ev events.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	needRebuild := false
	switch ev.Type {
	case events.StartupComplete:
		s.snapshot.Phase = PhaseReady
	case events.StateChanged:
		var d struct {
			Folder string `json:"folder"`
			To     string `json:"to"`
			From   string `json:"from"`
		}
		if decode(ev.Data, &d) {
			if f := s.folderStates[d.Folder]; f != nil {
				f.State = d.To
			}
		}
	case events.FolderPaused, events.FolderResumed:
		var d struct {
			ID string `json:"id"`
		}
		if decode(ev.Data, &d) {
			if f := s.folderStates[d.ID]; f != nil {
				f.Paused = ev.Type == events.FolderPaused
			}
		}
	case events.FolderSummary:
		var d model.FolderSummaryEventData
		if decode(ev.Data, &d) && d.Summary != nil {
			if f := s.folderStates[d.Folder]; f != nil {
				sum := d.Summary
				f.Errors = sum.Errors
				f.NeedBytes = sum.NeedBytes
				f.NeedItems = sum.NeedTotalItems
				f.GlobalBytes = sum.GlobalBytes
				f.LocalBytes = sum.LocalBytes
				if sum.GlobalBytes > 0 {
					f.Progress = float64(sum.GlobalBytes-sum.NeedBytes) / float64(sum.GlobalBytes) * 100
				} else {
					f.Progress = 100
				}
			}
		}
	case events.FolderErrors:
		var d struct {
			Folder string `json:"folder"`
		}
		if decode(ev.Data, &d) {
			if f := s.folderStates[d.Folder]; f != nil {
				f.Errors = 1
			}
		}
	case events.DeviceConnected:
		var d struct {
			ID   string `json:"id"`
			Addr string `json:"addr"`
		}
		if decode(ev.Data, &d) {
			s.setDeviceConnected(d.ID, true, d.Addr)
		}
	case events.DeviceDisconnected:
		var d struct {
			ID string `json:"id"`
		}
		if decode(ev.Data, &d) {
			s.setDeviceConnected(d.ID, false, "")
		}
	case events.ItemStarted:
		s.activeItems++
	case events.ItemFinished:
		if s.activeItems > 0 {
			s.activeItems--
		}
	case events.PendingDevicesChanged, events.PendingFoldersChanged, events.ConfigSaved:
		// These change the set of configured/pending entries, which is
		// derived from the config and the observed database rather than from
		// the event payload. Fold them in by asking for a rebuild; it is
		// debounced so a burst of changes costs one rebuild.
		needRebuild = true
	}
	s.snapshot.ActiveItems = s.activeItems
	s.notifyLocked()
	if needRebuild {
		select {
		case s.rebuild <- struct{}{}:
		default:
		}
	}
}

// rebuildStatic recomputes everything derived from config/database rather than
// events: folder list, device list and pending entries.
func (s *StateBridge) rebuildStatic() {
	folders := s.cfg.FolderList()
	devices := s.cfg.DeviceList()
	myID := s.cfg.MyID()

	s.mu.Lock()
	defer s.mu.Unlock()
	internals := s.internals

	// Folders, preserving live state where we already have it.
	seen := make(map[string]bool, len(folders))
	out := make([]FolderState, 0, len(folders))
	for _, f := range folders {
		seen[f.ID] = true
		st, ok := s.folderStates[f.ID]
		if !ok {
			st = &FolderState{ID: f.ID, Progress: 100}
			s.folderStates[f.ID] = st
		}
		st.Label = f.Label
		st.Path = f.Path
		st.Type = string(f.Type)
		st.Paused = f.Paused
		if st.State == "" {
			if st.Paused {
				st.State = "paused"
			} else {
				st.State = "idle"
			}
		}
		out = append(out, *st)
	}
	for id := range s.folderStates {
		if !seen[id] {
			delete(s.folderStates, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	s.snapshot.Folders = out

	// Devices, including our own entry.
	devOut := make([]DeviceState, 0, len(devices))
	for _, d := range devices {
		ds := DeviceState{
			ID:         d.DeviceID.String(),
			Name:       d.Name,
			Paused:     d.Paused,
			Introducer: d.Introducer,
		}
		if d.DeviceID == myID {
			s.snapshot.MyName = d.Name
		}
		if prev := findDevice(s.snapshot.Devices, ds.ID); prev != nil {
			ds.Connected = prev.Connected
			ds.LastSeen = prev.LastSeen
			ds.Address = prev.Address
		}
		devOut = append(devOut, ds)
	}
	sort.Slice(devOut, func(i, j int) bool {
		if devOut[i].ID == myID.String() {
			return true
		}
		if devOut[j].ID == myID.String() {
			return false
		}
		return devOut[i].Name < devOut[j].Name
	})
	s.snapshot.Devices = devOut

	// Pending devices (they want to connect to us).
	if internals != nil {
		known := make(map[protocol.DeviceID]bool, len(devices))
		for _, d := range devices {
			known[d.DeviceID] = true
		}
		if pending, err := internals.PendingDevices(); err == nil {
			pd := make([]PendingDevice, 0, len(pending))
			for id, o := range pending {
				if known[id] {
					continue // already accepted
				}
				pd = append(pd, PendingDevice{
					ID:      id.String(),
					Name:    o.Name,
					Address: o.Address,
					Time:    o.Time.Format(time.RFC3339),
				})
			}
			sort.Slice(pd, func(i, j int) bool { return pd[i].Time > pd[j].Time })
			s.snapshot.PendingDevices = pd
		}
	}

	// Pending folders offered by known devices. Initialised rather than left
	// nil: this travels to the UI as JSON, where null is not an empty list.
	pf := make([]PendingFolder, 0)
	if internals != nil {
		for _, d := range devices {
			if d.DeviceID == myID {
				continue
			}
			offered, err := internals.PendingFolders(d.DeviceID)
			if err != nil {
				continue
			}
			for folderID, info := range offered {
				if _, ok := s.cfg.Folder(folderID); ok {
					continue // already accepted
				}
				label := folderID
				var offeredAt time.Time
				if obs, ok := info.OfferedBy[d.DeviceID]; ok {
					if obs.Label != "" {
						label = obs.Label
					}
					offeredAt = obs.Time
				}
				at := ""
				if !offeredAt.IsZero() {
					at = offeredAt.Format(time.RFC3339)
				}
				pf = append(pf, PendingFolder{
					ID:         folderID,
					Label:      label,
					DeviceID:   d.DeviceID.String(),
					DeviceName: d.Name,
					OfferedAt:  at,
				})
			}
		}
	}
	sort.Slice(pf, func(i, j int) bool { return pf[i].OfferedAt > pf[j].OfferedAt })
	s.snapshot.PendingFolders = pf

	// Aggregate need totals and error list.
	var needBytes int64
	var needItems int
	errs := make([]string, 0)
	for _, f := range out {
		needBytes += f.NeedBytes
		needItems += f.NeedItems
		if f.Errors > 0 {
			errs = append(errs, f.Label+": "+plural(f.Errors, "error"))
		}
	}
	s.snapshot.TotalNeedBytes = needBytes
	s.snapshot.TotalNeedItems = needItems
	s.snapshot.Errors = errs

	s.notifyLocked()
}

func (s *StateBridge) setDeviceConnected(id string, connected bool, addr string) {
	for i := range s.snapshot.Devices {
		if s.snapshot.Devices[i].ID == id {
			s.snapshot.Devices[i].Connected = connected
			if connected {
				s.snapshot.Devices[i].Address = addr
				s.snapshot.Devices[i].LastSeen = time.Now().Format(time.RFC3339)
			}
			return
		}
	}
}

// notifyLocked schedules an observer notification. Must be called with the
// mutex held. Calls are coalesced by dispatch.
func (s *StateBridge) notifyLocked() {
	s.snapshot.GeneratedAtUnix = time.Now().Unix()
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// changed is notifyLocked for callers that do not hold the mutex.
func (s *StateBridge) changed() {
	s.mu.Lock()
	s.notifyLocked()
	s.mu.Unlock()
}

func (s *StateBridge) dispatch() {
	defer s.wg.Done()
	const debounce = 150 * time.Millisecond
	timer := time.NewTimer(debounce)
	if !timer.Stop() {
		<-timer.C
	}
	for {
		select {
		case <-s.stop:
			return
		case <-s.notify:
			timer.Reset(debounce)
		case <-timer.C:
			s.mu.Lock()
			s.snapshot.UpdatedAtUnix = time.Now().Unix()
			snap := s.snapshot
			obs := make([]func(Snapshot), 0, len(s.observers))
			for _, fn := range s.observers {
				obs = append(obs, fn)
			}
			s.mu.Unlock()
			for _, fn := range obs {
				fn(snap)
			}
		}
	}
}

// RefreshSoon triggers a rebuild of config-derived state after a short delay,
// giving the config wrapper time to persist. Used after Modify calls.
func (s *StateBridge) RefreshSoon() {
	go func() {
		time.Sleep(300 * time.Millisecond)
		s.Refresh()
	}()
}

func findDevice(devices []DeviceState, id string) *DeviceState {
	for i := range devices {
		if devices[i].ID == id {
			return &devices[i]
		}
	}
	return nil
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// decode round-trips an event payload through JSON into a typed struct. It is
// tolerant of the several shapes syncthing uses for event data (maps, typed
// structs) and returns false when the payload cannot be understood.
func decode(data any, out any) bool {
	if data == nil {
		return false
	}
	bs, err := json.Marshal(data)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(bs, out); err != nil {
		slog.Debug("Failed to decode event payload", slogutil.Error(err))
		return false
	}
	return true
}
