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

### Choosing a different data directory

**Settings → Data → Move…** relocates everything: pick a directory and the app
copies its config, keys and database there and restarts. Because the device
certificate comes along, the device ID and every pairing survive the move.
Shared *folders* are not touched — they live wherever they already live.

The old directory is left behind as a backup; delete it yourself once the new
one is confirmed working.

Which directory is in use is decided in this order:

1. `-data DIR` on the command line
2. portable mode (`-portable`, or the marker file beside the executable)
3. the path recorded in `<platform default>\data-dir.txt`
4. the platform default itself

The choice has to be recorded outside the data directory, since reading the
data directory's own settings would require already knowing where it is. Hence
`data-dir.txt`, a one-line file in the platform default directory. Deleting it
is not a way to move the data — it just makes the app start in the default
directory and look like a fresh install.

A move happens in two steps because the database cannot be copied while it is
open: the request is written into the current directory as `migrate-to.txt`,
the app restarts, and the copy runs before anything is opened. If the app is
killed between the two, the next start performs the migration.

### Moving a shared folder

Changing the data directory does not move the folders being shared — those sit
wherever you put them, which is usually the point. To relocate one, use
**Folders → the folder's Move… button**.

The folder is paused-free but offline-by-definition while it moves: the data is
copied to the new directory first and only removed from the old one once every
file has arrived, so an interrupted move leaves the original intact and the
configuration still pointing at it. Afterwards syncthing re-indexes the new
location.

Two things worth knowing before confirming:

- **Other devices are not involved.** They keep their own copies; this only
  changes where *this* machine keeps its copy. Nothing is re-downloaded.
- **The destination's existing contents are merged**, not replaced. So if you
  point a folder at a directory that already has files, you get the union of the
  two. The confirmation names the file count and size first, and asks separately
  whether to delete files in the destination that the folder does not have.

Moving between two places on the same filesystem needs no extra space and is
near-instant; across filesystems it is a genuine copy, and the confirmation
reports the size so you can judge whether there is room.

`cmd/syncthing-cli` can do the same headlessly with `-move-id` and
`-move-folder`, and prints the same size report first.

### Files in the data directory

| File | Contents |
|---|---|
| `config.xml` | Syncthing configuration |
| `cert.pem`, `key.pem` | device identity (the device ID derives from these) |
| `index-v2/` | SQLite database |
| `syncthing-app.log` | application log, also shown in the Logs tab |
| `syncthing.lock` | single-instance lock |
| `.setup-complete` | written after the first-run wizard finishes |
| `hub-device-adopted` | written once, when the built-in hub was paired; its presence is what leaves the device list alone afterwards |
| `prefs.json` | the app's own settings: language, and whether new folders join the hub |
| `migrate-to.txt` | transient; present only between "move requested" and the restart that performs it |

`data-dir.txt`, which names the data directory, lives in the *platform default*
directory rather than in here — see above.

### The built-in hub

The app ships paired with one device that is not the user's:

```
kb-server  TTJYEAS-SGAEE5C-DBZ3HFD-DXNYYKN-SAOU4TP-QWF2B4M-NJT5I4P-GQ5Y3QN
           tcp://101.69.254.66:22000
```

It runs on a server, and it is what lets two peers reach each other when a direct
connection between them is not possible — a relay in syncthing's sense, using the
same protocol. It is added to the configuration on the first start, *before* the
engine is created, so its connections are never turned away as an unknown device,
and it is pinned to a literal address rather than left to global discovery, which
is unreliable where the app is deployed. `appcore/hub.go` holds the constants.

It is added once, ever. A `hub-device-adopted` marker in the data directory
records it, so un-pairing the hub afterwards sticks, and an existing
installation that predates the hub keeps the device list its user built.

**Settings → Hub device** also has a switch, on by default, that puts the hub in
the folder template, so folders created from then on are *copied* to the hub and
a new machine can fetch one with no other computer online. That is a backup, and
it costs space on the hub. Turning the switch on also joins folders that are
already shared; turning it off only stops future folders from joining, because
removing the hub from a folder that is syncing to it would delete the copy
stored there. Folders store their own device list, so the switch reaches a folder
only if it is changed while that folder is not the hub itself. A folder's device
list is fixed when the folder is created, which is why the template has to be
kept in step at startup rather than only when the switch is flipped.

Relays (`RelaysEnabled`) are a separate mechanism: those are the public relay
pool, used when a direct connection is impossible and the hub is not reachable.

### Where shared folders go

New shared folders get their root from the folder template, `Defaults.Folder.Path`
in `config.xml`. The first-run wizard sets it from its "Where should shared
folders live?" answer, defaulting to `Sync` in the user's home directory —
`C:\Users\<you>\Sync` on Windows. **Choosing a directory in the wizard creates
it**, including its parents; a folder root the user has to create by hand first
is a step with no reason behind it.

The same root is used for two other things, so all three agree:

- a folder another device offers is accepted into `<root>\<folder ID>` when the
  user does not pick a directory, and that directory is created;
- the "add a folder" and "move a folder" choosers open in the root.

`appcore.DefaultSyncDir` is the one place that answers "where is the root"; it
resolves a leading `~` (which is what syncthing's own default uses) rather than
handing the tilde to the filesystem, where it would become a literal directory
named `~`.

The root directory itself is not created until it is needed — the wizard, an
accepted folder, or a folder the user adds. An installation that is only ever
configured by hand therefore has no stray `Sync` directory.

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

**The window appears before the core is ready, behind a splash.** Starting the
instance takes a few seconds — the database opens, discovery runs, and the first
folder scan completes — and until it finishes every list is legitimately empty.
The splash covers that window so an empty folder list is never mistaken for a
lost configuration. It disappears when the core reports itself ready; if that
takes more than 15 seconds a button appears to dismiss it early.

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
| `-move-id ID` | folder to relocate (with `-move-folder`) |
| `-move-folder DIR` | move that folder's data to `DIR` |
| `-dump` | print the state snapshot as JSON and exit || `-quiet` | suppress the periodic state line |

## Development

```bash
go test ./appcore/...
go vet -tags "noassets" ./appcore/... ./desktop/...
gofmt -l appcore desktop buildapp cmd/syncthing-app cmd/syncthing-cli

# Frontend: parses, catalogs in step, every key resolves, and every t() call
# fills the placeholders in the text it selects
node desktop/frontend/dist/check.mjs

# Frontend: boots against a stubbed DOM and reports which screen is visible,
# which is how a blank window is caught without opening one
node desktop/frontend/dist/rendercheck.mjs
```

### When the window is blank

The frontend journals each startup step through the `BootLog` binding, and the
log records that the page loaded at all:

```
INF Frontend loaded
INF Frontend boot step=start
INF Frontend boot step=identity ok
INF Frontend boot step=events wired
INF Frontend boot step=state ready
INF Frontend boot step=rendered
```

The last line present says where it stopped. If `Frontend loaded` is missing, the
page never got that far — the fault is in the webview or the asset server, not in
the UI code. A hard failure is also drawn into the window itself by
`panicScreen`, so a broken build shows its error rather than nothing.

The frontend has no build step: edit `desktop/frontend/dist/` and rebuild. It
only renders state the Go side pushes, so it holds no logic — the same markup is
intended to be reusable by a mobile shell later.

### When a dialog shows `{name}`

`t()` substitutes the values it is given and leaves the rest of the text alone,
so a `t()` call that forgets one of the placeholders its message uses puts the
braces in front of the user. `check.mjs` checks this statically for literal keys,
in both directions: a call that leaves a placeholder unfilled, and a templated
message that no call asks for at all. Neither is an error at runtime, which is
why it needs a check rather than a guard.
