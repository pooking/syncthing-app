# SyncthingApp

A portable desktop app around an embedded Syncthing core: a small native window
and a system tray icon instead of the web GUI. Drop folders onto the window to
share them, pair devices by pasting a device ID (or scanning a QR code), and
watch sync state from the tray.

Everything lives inside one executable. On Windows it is pure Go, so it
cross-compiles from any machine and needs no runtime to install.

## Layout

| Path | Purpose |
|---|---|
| `appcore/` | Platform-independent core: embeds a full Syncthing instance (`lib/syncthing`) and exposes a UI-friendly API. Also the future mobile bindings' entry point. |
| `appcore/autostart/` | Start-at-login, one implementation per platform (registry / LaunchAgent / `.desktop`). |
| `appcore/i18n.go` | Translation catalog for messages the Go side produces. |
| `desktop/` | Wails v2 shell: window, tray, and the bindings the frontend calls. |
| `desktop/frontend/dist/` | The UI (plain HTML/CSS/JS, no build step), embedded into the binary. |
| `desktop/frontend/dist/i18n.js` | Translation catalog for everything the UI displays. |
| `desktop/tray/` | Tray icon rendering and the status state machine. |
| `cmd/syncthing-app/` | The desktop app entry point. |
| `cmd/syncthing-cli/` | Headless harness for testing the core without a GUI. |
| `buildapp/` | Packaging tool. |

The only change to upstream code is two forwarding methods added to
`lib/syncthing/internals.go`.

## Building

```bash
go run -tags tools ./buildapp -zip
```

Output lands in `dist/`:

```
dist/SyncthingApp-windows-amd64.zip     portable archive (ship this)
dist/syncthing-app-windows-amd64.exe    bare binary
```

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `-goos` | host OS | target operating system |
| `-goarch` | host arch | target architecture |
| `-out` | `dist` | output directory |
| `-version` | derived from git | version to stamp |
| `-zip` | off | also write the portable archive |
| `-all` | off | build windows/amd64, linux/amd64, darwin/arm64 |
| `-cli-only` | off | build only `cmd/syncthing-cli` |

If git is unavailable or the ref does not look like a version, the stamp falls
back to `v0.0.0-dev.<date>`.

### Platform notes

**Windows** — pure Go; builds anywhere, no C toolchain, Windows 10+ with
WebView2 (preinstalled on Windows 11). The binary is a GUI-subsystem executable
(`-H windowsgui`), so launching it never opens a console window. Startup
failures are shown in a native dialog and written to the log.

**Linux and macOS** — Wails links GTK/WebKit on Linux and Cocoa on macOS, and
`fyne.io/systray` is cgo on both. These builds need a C toolchain and the
platform's GUI libraries, so run the build on that platform:

```bash
# on Linux (needs build-essential, libgtk-3-dev, libwebkit2gtk-4.0-dev,
#             libayatana-appindicator3-dev)
go run -tags tools ./buildapp -zip

# on macOS
go run -tags tools ./buildapp -zip
```

On a machine without a C toolchain, `buildapp` refuses these targets with a
clear message instead of failing deep inside cgo.

### Building by hand

The desktop app needs two build tags and, on Windows, the GUI subsystem flag:

```bash
# Windows
go build -tags "noassets,production" -ldflags "-H windowsgui" -o app.exe ./cmd/syncthing-app

# Linux / macOS (CGO_ENABLED=1)
go build -tags "noassets,production" -o app ./cmd/syncthing-app
```

- `production` is required by Wails; without it Wails compiles a stub that only
  pops up an error dialog.
- `noassets` skips embedding upstream's web GUI, which this app does not serve.

## Running

Double-click the executable. On first launch a wizard asks for a device name and
where shared folders should live; both can be changed later in Settings.

### Command line

| Flag | Meaning |
|---|---|
| `-portable` | keep all state next to the executable |
| `-data DIR` | use `DIR` for config and database |
| `-minimized` | start hidden in the tray (used by start-at-login) |
| `-show` | force the window to open even when `-minimized` is set |

### Where state is stored

| Mode | Location |
|---|---|
| Portable | `Data/` beside the executable |
| Windows | `%LocalAppData%\SyncthingApp` |
| macOS | `~/Library/Application Support/SyncthingApp` |
| Linux | `~/.local/state/syncthingapp` |

Portable mode turns on when the `-portable` flag is given **or** a file named
`SyncthingApp.portable` sits next to the executable. The `-zip` archive includes
that marker, which is why the archive is the thing to distribute: extracting it
gives a self-contained, self-updating-free folder.

These paths deliberately differ from upstream Syncthing's, so this app never
shares a config, database or lock file with a separately installed Syncthing.

### Files in the data directory

| File | Contents |
|---|---|
| `config.xml` | Syncthing configuration |
| `cert.pem`, `key.pem` | device identity (the device ID derives from these) |
| `index-v2/` | SQLite database |
| `syncthing-app.log` | application log, also shown in the Logs tab |
| `syncthing.lock` | single-instance lock |
| `.setup-complete` | written after the first-run wizard finishes |

## Languages

The interface ships in English and Chinese. On first launch the language follows
the operating system (Windows locale, `AppleLocale` on macOS, `LANG`/`LC_ALL`
on Linux); the choice can be changed at any time in **Settings → Language**.

The selection is stored in the app's own `prefs.json` in the data directory, not
in syncthing's `config.xml`, so it never travels with a synced configuration.

Two translation catalogs have to stay in step:

- `appcore/i18n.go` — messages the Go side produces (folder rejection reasons,
  failed operations, tray menu and tooltip). Errors carry a message code, so the
  frontend can re-render them after a language switch rather than showing a
  string frozen in the language that was active when it was produced.
- `desktop/frontend/dist/i18n.js` — everything the UI itself displays.

`go test ./appcore/` fails if a code exists in one language but not the other.

## Behaviour notes

**The web GUI is off.** The embedded instance sets `gui.enabled=false`, so
nothing listens on port 8384 — which also means the app never collides with a
separately running Syncthing's GUI. The device protocol port is still used;
check the Settings tab for the value, and the first-run wizard warns if the
port is already taken.

**Auto-upgrade is off** in three places: the `noupgrade` build tag, the runtime
option, and the config. Updating means replacing the executable.

**Closing the window minimises to the tray**; the app keeps syncing. Quit from
the tray menu or the Settings tab, both of which shut the core down gracefully
before exiting.

**Only one instance runs per data directory.** Launching again brings the
existing window to the front.

**Tray icon colours**: green idle, blue syncing, amber paused, red error, grey
starting or no device connected.

## Testing the core without a GUI

`cmd/syncthing-cli` drives `appcore` directly and prints state transitions:

```bash
go build -o cli ./cmd/syncthing-cli

# First run prints the device ID and exits.
./cli -data /tmp/a -name DeviceA -dump

# Share a folder with a peer.
./cli -data /tmp/a -name DeviceA \
      -add-device <PEER_DEVICE_ID> -add-folder /tmp/share

# On the peer: pair back and accept whatever gets offered.
./cli -data /tmp/b -name DeviceB \
      -add-device <DEVICE_A_ID> -accept /tmp/accept -accept-wait 120s
```

| Flag | Meaning |
|---|---|
| `-data DIR` | data directory |
| `-portable` | keep state next to the executable |
| `-name NAME` | device name to advertise |
| `-add-device ID` | pair with a device |
| `-add-folder DIR` | share a directory (combine with `-add-device`) |
| `-accept DIR` | accept pending folders under `DIR` |
| `-accept-wait D` | how long to wait for them (default 20s) |
| `-dump` | print the state snapshot as JSON and exit |
| `-quiet` | suppress the periodic state line |

## Development

```bash
go test ./appcore/...
go vet -tags "noassets" ./appcore/... ./desktop/...
gofmt -l appcore desktop buildapp cmd/syncthing-app cmd/syncthing-cli
```

The frontend has no build step: edit `desktop/frontend/dist/` and rebuild. It
only renders state the Go side pushes, so it holds no logic — the same markup is
intended to be reusable by a mobile shell later.
