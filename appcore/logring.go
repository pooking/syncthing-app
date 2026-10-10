// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package appcore

import (
	"strings"
	"sync"
)

// RingBuffer is a fixed-size in-memory log sink. It implements io.Writer so it
// can be plugged straight into a slog handler, and keeps the most recent lines
// for display in the app's log view.
type RingBuffer struct {
	mu      sync.Mutex
	lines   []string
	max     int
	partial strings.Builder
}

func NewRingBuffer(max int) *RingBuffer {
	if max <= 0 {
		max = 1000
	}
	return &RingBuffer{max: max, lines: make([]string, 0, max)}
}

func (r *RingBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range p {
		if b == '\n' {
			r.appendLine(strings.TrimRight(r.partial.String(), "\r"))
			r.partial.Reset()
		} else {
			r.partial.WriteByte(b)
		}
	}
	return len(p), nil
}

func (r *RingBuffer) appendLine(line string) {
	if len(r.lines) >= r.max {
		copy(r.lines, r.lines[1:])
		r.lines = r.lines[:len(r.lines)-1]
	}
	r.lines = append(r.lines, line)
}

// Lines returns the most recent lines, oldest first, capped at limit (0 = all).
func (r *RingBuffer) Lines(limit int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := 0
	if limit > 0 && len(r.lines) > limit {
		start = len(r.lines) - limit
	}
	// Never nil: this goes straight to the UI, which treats a null as "not a
	// list" rather than as an empty one.
	out := make([]string, 0, len(r.lines)-start)
	out = append(out, r.lines[start:]...)
	return out
}

// String returns the buffered log as a single newline-joined blob.
func (r *RingBuffer) String() string {
	return strings.Join(r.Lines(0), "\n")
}
