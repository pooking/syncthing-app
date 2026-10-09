// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package desktop

import "embed"

// Assets holds the built frontend. It lives next to the frontend sources so
// the embed path stays short and the whole UI ships inside the binary.
//
//go:embed all:frontend/dist
var Assets embed.FS
