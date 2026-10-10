// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

// The frontend renders the state that the Go side pushes; it holds no business
// logic, so the same markup can be reused by a mobile shell later. All visible
// text goes through t() (see i18n.js).

const api = () => window.go.desktop.App;
const rt = () => window.runtime;

let state = { folders: [], devices: [], pendingFolders: [], pendingDevices: [], errors: [] };
let myID = '';

const $ = (id) => document.getElementById(id);

document.addEventListener('DOMContentLoaded', () => {
  // Last line of defence for a blank window. A failure below would otherwise
  // leave every screen hidden with nothing to show for it, and a GUI build has
  // no console to report to, so the panic screen is written into the document
  // itself. Nothing here is async: the handler must not miss a synchronous
  // throw before the first await.
  wireTabs();
  wireActions();
  wireWizard();
  window.addEventListener('error', (e) => panicScreen(e.message + '\n' + (e.error?.stack || '')));
  window.addEventListener('unhandledrejection', (e) => panicScreen(String(e.reason)));
  boot().catch((e) => panicScreen((e && e.stack) || String(e)));
});

// panicScreen shows whatever went wrong in place of the UI, so a failure is
// readable instead of blank. Selecting the window and pressing Ctrl+C copies it.
function panicScreen(text) {
  try {
    const el = $('boot');
    el.classList.remove('hidden');
    el.innerHTML = '<div class="boot-card"><h1>SyncthingApp could not draw its window</h1>' +
      '<p class="hint">This is a bug. Copy this and send it on:</p>' +
      '<pre class="logs" id="panic-text"></pre></div>';
    $('panic-text').textContent = String(text);
  } catch (e) {
    // Nothing else can be done from inside a broken document.
  }
}

async function boot() {
  // Each step is journaled, because a GUI build has no console: if the window
  // ends up empty, this is what says which step it stopped at.
  bootLog('start');
  myID = await api().DeviceID();
  $('my-id').textContent = myID;
  $('device-line').textContent = `${await api().ShortDeviceID()} · ${await api().DataDir()}`;
  $('data-dir').textContent = await api().DataDir();
  $('data-dir').title = await api().DataDir();
  bootLog('identity ok');

  rt().EventsOn('state', (json) => {
    applyState(JSON.parse(json));
  });
  rt().EventsOn('dropped', (json) => showDropResult(JSON.parse(json)));

  wireBoot();
  bootLog('events wired');

  // Pull the snapshot once instead of waiting for the next event. Events are
  // not buffered: the one the Go side emits at startup, which is the one that
  // carries "ready", arrives before this listener exists and is lost. Without
  // this the splash would wait for a state change that may never come.
  const snapshot = await api().GetState();
  bootLog(`state ${JSON.parse(snapshot).phase}`);
  applyState(JSON.parse(snapshot));
  bootLog('rendered');

  const setup = JSON.parse(await api().SetupStatus());
  await adoptLanguage();
  bootLog('language ok');
  if (setup.needsSetup) {
    showWizard(setup);
    // The wizard is a usable screen on its own, so it is shown even mid-start;
    // this only adds a way out if the core never finishes coming up.
    scheduleStuckHints();
    bootLog('wizard shown');
    return;
  }
  await startApp();
  bootLog('done');
}

// bootLog appends a line to the app log through a Go binding. It is best effort:
// a failure to log must never be the reason the window stays blank.
function bootLog(step) {
  try {
    api().BootLog(step);
  } catch (e) {
    // Older builds have no such binding; the journal is not essential.
  }
}

// ---- startup splash ----

// The window is created before the embedded instance finishes starting, so for
// the first moments every list is legitimately empty. Showing the real UI then
// makes it look like the configuration was lost, so the splash covers it until
// the core reports itself ready.
let bootState = null;
let bootTimer = null;

function wireBoot() {
  $('boot-anyway').addEventListener('click', () => hideBoot());
  $('status-reload').addEventListener('click', () => location.reload());
}

function applyState(snapshot) {
  bootState = snapshot;
  state = normalise(snapshot);
  render();
  hideBootOnReady();
}

// asArray turns a Go slice that marshalled as null into an empty array. Go
// marshals a nil slice as null, and several of the lists the Go side sends are
// nil until something populates them, so any read of .length or .map on them
// would throw and take the rest of the caller with it.
function asArray(v) {
  return Array.isArray(v) ? v : [];
}

// normalise guarantees the arrays the renderers iterate over. Go marshals a nil
// slice as null, and the pending lists are nil until something is offered, so
// without this a render would throw on null.length and never finish — which
// shows up as an empty window.
function normalise(s) {
  if (!s) return state;
  for (const key of ['folders', 'devices', 'pendingDevices', 'pendingFolders', 'errors']) {
    s[key] = asArray(s[key]);
  }
  return s;
}

// showBoot covers the UI, but only while the core is known to be starting. Once
// it is up — or nothing is known at all, as during a frontend reload — the data
// on screen is real and there is nothing to hide.
function showBoot() {
  if (!isStarting()) return;
  $('boot').classList.remove('hidden');
  scheduleStuckHints();
}

function hideBoot() {
  removeStuckHints();
  $('boot').classList.add('hidden');
}

// isStarting reports whether the core is still coming up.
function isStarting() {
  return bootState !== null && bootState.phase === 'starting';
}

// hideBootOnReady uncovers the UI as soon as the core reports itself up.
function hideBootOnReady() {
  if (isStarting()) return false;
  hideBoot();
  return true;
}

// scheduleStuckHints offers a way out when startup drags on: whether the splash
// can be dismissed, and whether reloading the view is worth trying.
function scheduleStuckHints() {
  clearTimeout(bootTimer);
  bootTimer = setTimeout(() => {
    $('boot-detail').textContent = t('boot.slow');
    $('boot-anyway').classList.remove('hidden');
    $('status-reload').classList.remove('hidden');
  }, 15000);
}

function removeStuckHints() {
  clearTimeout(bootTimer);
  $('boot-anyway').classList.add('hidden');
  $('status-reload').classList.add('hidden');
}

// watchUntilReady asks the Go side directly while the core is starting, so the
// splash cannot stay up just because no state event arrived to clear it. It
// stops on its own once the phase is known, and gives up after two minutes.
async function watchUntilReady() {
  for (let i = 0; i < 240 && isStarting(); i++) {
    await new Promise((r) => setTimeout(r, 500));
    if (!isStarting()) break;
    try {
      applyState(JSON.parse(await api().GetState()));
    } catch (e) {
      // The backend is not answering; the stuck hints cover that case.
    }
  }
}

// adoptLanguage asks the Go side whether the user picked a language (recorded
// in the app's own prefs, not in syncthing's config) and applies it.
async function adoptLanguage() {
  const s = JSON.parse(await api().GetSettings());
  setLanguage(s.language || 'en');
  $('set-lang').value = currentLanguage();
  applyStaticText();
}

async function startApp() {
  $('wizard').classList.add('hidden');
  $('app').classList.remove('hidden');
  await loadSettings();
  await refreshLogs();
  render();
  // Discovery and the first scan continue after the window exists; cover them
  // until the core says it is up.
  showBoot();
  hideBootOnReady();
  watchUntilReady();
}

// ---- first-run wizard ----

function showWizard(setup) {
  $('app').classList.add('hidden');
  $('wizard').classList.remove('hidden');
  $('wiz-name').value = setup.deviceName || '';
  $('wiz-dir').value = setup.defaultSyncDir || '';
  $('wiz-storage').textContent = setup.portable
    ? t('wizard.portable', { dir: setup.dataDir })
    : t('wizard.standard', { dir: setup.dataDir });
}

function wireWizard() {
  $('wiz-pick').addEventListener('click', async () => {
    // Open where the typed path points, if it is somewhere that exists: typing
    // a parent and then browsing into it is a common way to reach a subfolder.
    const dir = await api().PickDirectoryNear(t('wizard.pick'), $('wiz-dir').value.trim());
    if (dir) $('wiz-dir').value = dir;
  });

  // The path is typed rather than only picked, so make sure what the user sees
  // is what will be used. The Go side resolves and creates it; this only takes
  // off trailing slashes and whitespace, which are not a decision anyone makes
  // on purpose.
  $('wiz-dir').addEventListener('blur', () => {
    const el = $('wiz-dir');
    el.value = el.value.trim().replace(/[\\/]+$/, '');
  });

  $('wiz-done').addEventListener('click', async () => {
    const err = $('wiz-error');
    err.classList.add('hidden');
    try {
      await api().CompleteSetup(JSON.stringify({
        deviceName: $('wiz-name').value.trim(),
        syncDir: $('wiz-dir').value.trim(),
      }));
      await startApp();
    } catch (e) {
      err.textContent = String(e);
      err.classList.remove('hidden');
    }
  });
}

// ---- tabs ----

function wireTabs() {
  for (const tab of document.querySelectorAll('.tab')) {
    tab.addEventListener('click', () => {
      for (const other of document.querySelectorAll('.tab')) other.classList.remove('active');
      for (const panel of document.querySelectorAll('.panel')) panel.classList.remove('active');
      tab.classList.add('active');
      $('panel-' + tab.dataset.tab).classList.add('active');
      if (tab.dataset.tab === 'logs') refreshLogs();
    });
  }
}

function wireActions() {
  $('copy-id').addEventListener('click', async () => {
    await navigator.clipboard.writeText(myID);
    flash($('copy-id'), t('devices.copied'));
  });

  $('show-qr').addEventListener('click', async () => {
    const box = $('qr-box');
    if (!box.classList.contains('hidden')) {
      box.classList.add('hidden');
      return;
    }
    if (!$('qr-img').src) {
      $('qr-img').src = await api().DeviceIDQRCode();
    }
    box.classList.remove('hidden');
  });

  $('pick-folder').addEventListener('click', async () => {
    const res = await api().PickAndAddFolder('[]');
    if (res) showDropResult(JSON.parse(res));
  });

  // Highlight the drop zone while a drag hovers it.
  const dz = $('dropzone');
  for (const type of ['dragenter', 'dragover']) {
    document.addEventListener(type, (e) => { e.preventDefault(); dz.classList.add('hot'); });
  }
  for (const type of ['dragleave', 'drop']) {
    document.addEventListener(type, () => dz.classList.remove('hot'));
  }

  $('add-device').addEventListener('click', async () => {
    const id = $('new-device-id').value.trim();
    const name = $('new-device-name').value.trim();
    const err = $('device-error');
    err.classList.add('hidden');
    if (!id) return;
    try {
      await api().AddDevice(id, name);
      $('new-device-id').value = '';
      $('new-device-name').value = '';
    } catch (e) {
      err.textContent = String(e);
      err.classList.remove('hidden');
    }
  });

  $('set-lang').addEventListener('change', async () => {
    setLanguage($('set-lang').value);
    applyStaticText();
    render();
    await loadSettings();
  });

  $('save-settings').addEventListener('click', async () => {
    const payload = {
      deviceName: $('set-name').value.trim(),
      maxSendKbps: Number($('set-send').value) || 0,
      maxRecvKbps: Number($('set-recv').value) || 0,
      relaysEnabled: $('set-relays').checked,
      natEnabled: $('set-nat').checked,
      autoStart: $('set-autostart').checked,
      language: $('set-lang').value,
    };
    try {
      await api().SaveSettings(JSON.stringify(payload));
      flash($('settings-saved'), t('settings.saved'));
    } catch (e) {
      alert(t('settings.saveFailed', { err: e }));
    }
  });

  // The hub toggle applies on the spot rather than with Save: it reaches
  // folders that are already shared, so it is an action, not a stored value.
  $('set-hub-share').addEventListener('change', async () => {
    const on = $('set-hub-share').checked;
    try {
      await api().SetHubShareNewFolders(on);
      flash($('settings-saved'), t(on ? 'settings.hubShared' : 'settings.hubNotShared'));
    } catch (e) {
      $('set-hub-share').checked = !on;
      alert(t('settings.saveFailed', { err: e }));
    }
  });

  $('move-data').addEventListener('click', async () => {
    const err = $('data-dir-error');
    err.classList.add('hidden');
    const dir = await api().PickDirectory(t('settings.dataDirPick'));
    if (!dir) return;
    if (!confirm(t('settings.dataDirConfirm', { dir }))) return;
    try {
      // On success the app restarts itself, so there is nothing to report.
      await api().MoveDataDir(dir);
    } catch (e) {
      err.textContent = t('settings.dataDirFailed', { err: e });
      err.classList.remove('hidden');
    }
  });

  $('quit-app').addEventListener('click', () => api().Quit());
  $('refresh-logs').addEventListener('click', async () => {
    await refreshLogs();
    flash($('logs-updated'), t('logs.updated'));
  });
}

// ---- rendering ----

function render() {
  renderStatus();
  renderFolders();
  renderDevices();
  renderPending();
}

function renderStatus() {
  const pill = $('status-pill');
  const syncing = state.folders.some((f) => f.state === 'syncing' || f.state === 'scanning');
  const paused = state.folders.length > 0 && state.folders.every((f) => f.paused);
  let cls = 'starting';
  let label = t('status.starting');
  if (state.errors && state.errors.length) { cls = 'error'; label = t('status.error'); }
  else if (paused) { cls = 'paused'; label = t('status.paused'); }
  else if (syncing) { cls = 'syncing'; label = t('status.syncing'); }
  else if (state.phase === 'ready') {
    cls = state.totalNeedItems ? 'paused' : 'ready';
    label = state.totalNeedItems ? t('status.outOfSync') : t('status.ready');
  }

  pill.className = 'pill ' + cls;
  pill.textContent = label;

  const bits = [];
  if (state.totalNeedItems) bits.push(t('status.needItems', { n: state.totalNeedItems }));
  if (state.activeItems) bits.push(t('status.transferring', { n: state.activeItems }));
  const online = state.devices.filter((d) => d.id !== myID && d.connected).length;
  const peers = state.devices.filter((d) => d.id !== myID).length;
  if (peers) bits.push(t('status.devicesOnline', { online, total: peers }));
  $('status-detail').textContent = bits.join(' · ');
}

function renderFolders() {
  const list = $('folder-list');
  list.innerHTML = '';
  $('folders-empty').classList.toggle('hidden', state.folders.length > 0);

  for (const f of state.folders) {
    const li = document.createElement('li');
    li.className = 'item';
    li.innerHTML = `
      <div class="grow">
        <div class="name">${esc(f.label || f.id)}</div>
        <div class="meta">${esc(f.path)}</div>
        <div class="meta">${esc(folderStatusText(f))}</div>
        <div class="bar"><i style="width:${clamp(f.progress)}%"></i></div>
      </div>
      <div class="actions"></div>`;

    const actions = li.querySelector('.actions');
    actions.append(
      button(t(f.paused ? 'folders.resume' : 'folders.pause'),
        () => api().SetFolderPaused(f.id, !f.paused)),
      button(t('folders.rescan'), () => api().RescanFolder(f.id)),
      button(t('folders.open'), () => api().OpenFolder(f.id)),
      button(t('folders.move'), () => moveFolder(f)),
      button(t('folders.stop'), async () => {
        if (confirm(t('folders.stopConfirm', { name: f.label || f.id }))) {
          await api().RemoveFolder(f.id, false);
        }
      }, 'danger'),
    );
    list.append(li);
  }
}

// moveFolder relocates a folder's data. The size is reported before anything
// happens, because a move across filesystems is a real copy that can take a
// long time and needs room at the destination.
async function moveFolder(f) {
  const name = f.label || f.id;
  const dir = await api().PickDirectory(t('folders.movePick', { name }));
  if (!dir) return;

  let check;
  try {
    check = JSON.parse(await api().CheckFolderMove(f.id, dir));
  } catch (e) {
    alert(t('folders.moveFailed', { err: e }));
    return;
  }
  if (!check || check.bytes === undefined) check = { ...check, bytes: 0 };

  // Every message below names the folder, so the name has to be in here; a
  // missing key is not an error, it just leaves "{name}" in the dialog.
  const params = { name, target: check.target, files: check.files, size: formatBytes(check.bytes) };
  if (check.targetExists) {
    if (!confirm(t('folders.moveMergeConfirm', params))) return;
  } else if (!confirm(t('folders.moveConfirm', params))) {
    return;
  }

  const replace = check.targetExists
    ? confirm(t('folders.moveReplaceConfirm', params))
    : false;

  try {
    await api().MoveFolder(f.id, dir, replace);
  } catch (e) {
    alert(t('folders.moveFailed', { err: e }));
  }
}

// formatBytes renders a byte count for the confirmation text.
function formatBytes(n) {
  n = Number(n) || 0;
  if (n < 1024) return n + ' B';
  const units = ['KB', 'MB', 'GB', 'TB'];
  let i = -1;
  do { n /= 1024; i++; } while (n >= 1024 && i < units.length - 1);
  return (n < 10 ? n.toFixed(1) : Math.round(n)) + ' ' + units[i];
}

function folderStatusText(f) {
  if (f.paused) return t('folder.paused');
  const parts = [t('folder.' + (f.state || 'idle'), {}) ];
  if (f.needItems) parts.push(t('folder.pending', { n: f.needItems }));
  if (f.errors) parts.push(t('folder.errors', { n: f.errors }));
  if (f.state !== 'syncing' && !f.needItems) parts.push(t('folder.inSync'));
  return parts.join(' · ');
}

function renderDevices() {
  const list = $('device-list');
  list.innerHTML = '';

  for (const d of state.devices) {
    if (d.id === myID) continue;
    const li = document.createElement('li');
    li.className = 'item';
    const status = (d.connected ? t('devices.connected') : t('devices.offline')) +
      (d.paused ? ' · ' + t('devices.paused') : '');
    li.innerHTML = `
      <span class="dot ${d.connected ? 'on' : ''}"></span>
      <div class="grow">
        <div class="name">${esc(d.name || d.id.slice(0, 7))}</div>
        <div class="meta">${esc(d.id)}</div>
        <div class="meta">${esc(status)}</div>
      </div>
      <div class="actions"></div>`;

    const actions = li.querySelector('.actions');
    actions.append(
      button(t('devices.rename'), async () => {
        const name = prompt(t('devices.renamePrompt'), d.name || '');
        if (name !== null) await api().RenameDevice(d.id, name);
      }),
      button(t(d.paused ? 'folders.resume' : 'folders.pause'),
        () => api().SetDevicePaused(d.id, !d.paused)),
      button(t('devices.remove'), async () => {
        if (confirm(t('devices.removeConfirm', { name: d.name || d.id.slice(0, 7) }))) {
          await api().RemoveDevice(d.id);
        }
      }, 'danger'),
    );
    list.append(li);
  }
}

function renderPending() {
  const pdev = $('pending-devices');
  $('pending-card').classList.toggle('hidden', !state.pendingDevices.length);
  pdev.innerHTML = '';
  for (const p of state.pendingDevices) {
    const li = document.createElement('li');
    li.className = 'item';
    li.innerHTML = `
      <div class="grow">
        <div class="name">${esc(p.name || p.id.slice(0, 7))}</div>
        <div class="meta">${esc(p.id)}</div>
      </div>
      <div class="actions"></div>`;
    li.querySelector('.actions').append(
      button(t('devices.accept'), () => api().AcceptPendingDevice(p.id)),
    );
    pdev.append(li);
  }

  const pf = $('pending-folders');
  $('pending-folders-card').classList.toggle('hidden', !state.pendingFolders.length);
  pf.innerHTML = '';
  for (const p of state.pendingFolders) {
    const li = document.createElement('li');
    li.className = 'item';
    li.innerHTML = `
      <div class="grow">
        <div class="name">${esc(p.label || p.id)}</div>
        <div class="meta">${esc(t('devices.offeredBy', {
          name: p.deviceName || p.deviceID.slice(0, 7),
        }))}</div>
      </div>
      <div class="actions"></div>`;
    li.querySelector('.actions').append(
      button(t('devices.accept'), () => api().AcceptPendingFolder(p.deviceID, p.id, '')),
    );
    pf.append(li);
  }
}

// ---- helpers ----

// renderReason prefers the message code the backend sent, so a rejected drop
// reads in the language the user just switched to; the pre-rendered text is
// the fallback for older responses.
function renderReason(item) {
  if (item.code) return t('code.' + item.code, expandArgs(item.args));
  return item.reason || '';
}

// expandArgs turns the positional argument list into the named placeholders
// the catalog uses.
function expandArgs(args) {
  if (!args || !args.length) return {};
  return { name: args[0], n: args[0], id: args[0], path: args[0], err: args[0] };
}

function showDropResult(res) {
  const el = $('folder-conflict');
  const lines = [];
  for (const r of res.rejected || []) lines.push(`${r.path}: ${renderReason(r)}`);
  for (const w of res.warnings || []) {
    lines.push(`${t('folders.note')} — ${w.path}: ${renderReason(w)}`);
  }
  if (lines.length) {
    el.textContent = lines.join('\n');
    el.classList.remove('hidden');
  } else {
    el.classList.add('hidden');
  }
  // Only switch to the Folders tab when something actually happened.
  if ((res.added || []).length) {
    document.querySelector('.tab[data-tab="folders"]').click();
  }
}

async function loadSettings() {
  const ports = asArray(JSON.parse(await api().ListeningPorts()));
  $('ports-line').textContent = ports.length
    ? t('settings.ports', { ports: ports.join(', ') })
    : t('settings.noPorts');

  const s = JSON.parse(await api().GetSettings());
  $('set-name').value = s.deviceName || '';
  $('set-send').value = s.maxSendKbps || 0;
  $('set-recv').value = s.maxRecvKbps || 0;
  $('set-relays').checked = !!s.relaysEnabled;
  $('set-nat').checked = !!s.natEnabled;
  $('set-autostart').checked = !!s.autoStart;
  if (s.language) $('set-lang').value = s.language;

  // The built-in hub is not a setting the user typed, so it is reported rather
  // than offered as a field: name, ID and the address it is pinned to.
  $('hub-line').textContent = t('settings.hubLine', { name: s.hubName, id: s.hubDeviceID, address: s.hubAddress });
  $('set-hub-share').checked = !!s.hubShareNewFolders;
}

async function refreshLogs() {
  const lines = asArray(JSON.parse(await api().GetLogs(400)));
  const el = $('logs');
  el.textContent = lines.join('\n');
  el.scrollTop = el.scrollHeight;
}

function button(label, onClick, cls = '') {
  const b = document.createElement('button');
  b.className = 'btn small ' + cls;
  b.textContent = label;
  b.addEventListener('click', onClick);
  return b;
}

function flash(el, text) {
  const prev = el.textContent;
  el.textContent = text;
  el.classList.remove('hidden');
  setTimeout(() => {
    el.textContent = prev;
    el.classList.add('hidden');
  }, 1500);
}

function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

function clamp(n) {
  return Math.max(0, Math.min(100, Number(n) || 0));
}
