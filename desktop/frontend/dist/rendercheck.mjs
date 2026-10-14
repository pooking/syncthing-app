// Loads the real app.js against a stubbed DOM and Wails runtime, runs the boot
// sequence, and reports which screen ends up visible. This is the only way to
// see a blank window without a window.
//
// Run: node desktop/frontend/dist/rendercheck.mjs
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import vm from 'node:vm';

const here = dirname(fileURLToPath(import.meta.url));

function makeEl(id, classes = []) {
  const set = new Set(classes);
  const el = {
    id,
    children: [],
    textContent: '',
    innerHTML: '',
    className: '',
    title: '',
    src: '',
    value: '',
    checked: false,
    placeholder: '',
    alt: '',
    dataset: {},
    style: {},
    scrollTop: 0,
    scrollHeight: 0,
    _listeners: {},
  };
  el.classList = {
    add: (...c) => c.forEach((x) => set.add(x)),
    remove: (...c) => c.forEach((x) => set.delete(x)),
    contains: (c) => set.has(c),
    toggle: (c, on) => (on ? set.add(c) : set.delete(c)),
  };
  el.addEventListener = (type, fn) => { el._listeners[type] = fn; };
  el.append = (...kids) => kids.forEach((k) => el.children.push(k));
  el.querySelector = () => makeEl('sel');
  el.querySelectorAll = () => [];
  return el;
}

// Trailing commas in the object literals below are intentional.
const MARKUP_CLASSES = {
  boot: ['boot', 'hidden'],
  app: ['hidden'],
  wizard: ['wizard', 'hidden'],
  'boot-anyway': ['btn', 'hidden'],
  'status-reload': ['pill', 'hidden'],
};

function load(scenario) {
  const els = new Map();
  const get = (id) => {
    if (!els.has(id)) els.set(id, makeEl(id, MARKUP_CLASSES[id] || []));
    return els.get(id);
  };
  const tabs = ['folders', 'devices', 'settings', 'logs'].map((name) => {
    const el = makeEl('panel-' + name);
    el.dataset.tab = name;
    return el;
  });

  const bootLog = [];
  const fileDrop = [];
  const sandbox = {
    console: { log() {}, error() {}, warn() {} },
    document: {
      getElementById: get,
      createElement: (tag) => makeEl('<' + tag + '>'),
      querySelectorAll: (sel) => (sel === '.tab' ? tabs : []),
      querySelector: () => makeEl('sel'),
      addEventListener: (type, fn) => { if (type === 'DOMContentLoaded') sandbox._domReady = fn; },
      documentElement: { lang: '' },
    },
    window: {
      addEventListener() {},
      go: { desktop: { App: {
        DeviceID: async () => scenario.deviceID,
        ShortDeviceID: async () => scenario.deviceID.slice(0, 7),
        DataDir: async () => 'C:\\data',
        GetState: async () => JSON.stringify(scenario.snapshot),
        SetupStatus: async () => JSON.stringify(scenario.setup),
        GetSettings: async () => JSON.stringify({ language: 'en' }),
        // Go marshals a nil slice as null, and these two are nil until
        // something populates them; scenario.nilLists reproduces that.
        ListeningPorts: async () => (scenario.nilLists ? 'null' : '[]'),
        GetLogs: async () => (scenario.nilLists ? 'null' : '[]'),
        BootLog: (step) => { bootLog.push(step); },
      } } },
      runtime: { EventsOn() {}, OnFileDrop: (cb, useDropTarget) => { fileDrop.push({ cb, useDropTarget }); } },
      location: { reload() {} },
    },
    navigator: { clipboard: { writeText: async () => {} } },
    t: (key, params) => (params ? key : key),
    setLanguage() {},
    currentLanguage: () => 'en',
    applyStaticText() {},
    esc: (s) => String(s ?? ''),
    clamp: (n) => Math.max(0, Math.min(100, Number(n) || 0)),
    setTimeout, clearTimeout,
    alert() {}, confirm: () => true, prompt: () => null,
    Promise, JSON, Object, Array, String, Number, Math, Error, Set, Map, Date,
  };
  sandbox.window.window = sandbox.window;
  sandbox.globalThis = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(readFileSync(join(here, 'app.js'), 'utf8'), sandbox);

  return { sandbox, get, bootLog, fileDrop };
}

const DEVICE = 'AAAAAAA-BBBBBBB-CCCCCCC-DDDDDDD-EEEEEEE-FFFFFFF-GGGGGGG-HHHHHHH';
const PEER = 'XQGLU27-3DOOWBS-LBBPXAK-4UUVENT-7C2YWKY-4WNWUEE-6KWZUFN-UXKDIAT';

const CONFIGURED = {
  phase: 'ready',
  deviceID: DEVICE,
  myName: 'POO',
  folders: [
    { id: '00_全局', label: '00_全局', path: 'C:\\Sync\\00_全局', type: 'sendreceive', state: 'idle', paused: false, progress: 100, needItems: 0, needBytes: 0, globalBytes: 1, localBytes: 1, errors: 0 },
    { id: '王琼', label: '王琼', path: 'C:\\Sync\\王琼', type: 'sendreceive', state: 'scanning', paused: false, progress: 42.5, needItems: 12, needBytes: 4096, globalBytes: 8192, localBytes: 1024, errors: 2 },
  ],
  devices: [
    { id: DEVICE, name: 'POO', connected: false, paused: false, introducer: false },
    { id: PEER, name: 'kb-server', connected: true, paused: false, introducer: false, address: '192.168.1.9:22000', lastSeen: '2026-10-10T08:00:00+08:00' },
  ],
  pendingDevices: [{ id: PEER, name: 'phone', address: '10.0.0.5:22000', time: '2026-10-10T07:00:00+08:00' }],
  pendingFolders: [{ id: 'photos', label: 'Photos', deviceID: PEER, deviceName: 'kb-server', offeredAt: '2026-10-10T07:30:00+08:00' }],
  activeItems: 2,
  totalNeedBytes: 4096,
  totalNeedItems: 12,
  errors: ['王琼: 2 errors'],
};

// The shape the Go side actually produces before anything is offered: nil
// slices marshal as null, which is what the renderers have to survive.
const FRESH = {
  phase: 'starting',
  deviceID: DEVICE,
  myName: '',
  folders: [],
  devices: [{ id: DEVICE, name: '', connected: false, paused: false, introducer: false }],
  pendingDevices: [],
  pendingFolders: null,
  activeItems: 0,
  totalNeedBytes: 0,
  totalNeedItems: 0,
  errors: [],
};

const NO_SETUP = { needsSetup: false, dataDir: 'C:\\data', defaultSyncDir: 'C:\\Sync' };
const WANTS_SETUP = { needsSetup: true, deviceName: '', dataDir: 'C:\\data', defaultSyncDir: 'C:\\Sync' };

let failures = 0;
function check(name, ok, detail = '') {
  if (!ok) failures++;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${!ok && detail ? `  — ${detail}` : ''}`);
}

async function run(label, snapshot, setup, nilLists = true) {
  const { sandbox, get, bootLog, fileDrop } = load({ snapshot, setup, deviceID: DEVICE, nilLists });
  let threw = '';
  try {
    await sandbox._domReady();
  } catch (e) {
    threw = (e && e.message) || String(e);
  }
  await new Promise((r) => setTimeout(r, 30));
  const shown = ['boot', 'app', 'wizard'].filter((id) => !get(id).classList.contains('hidden'));
  console.log(`\n--- ${label} ---`);
  console.log(`    journal: ${bootLog.join(' > ')}`);
  if (threw) console.log(`    boot() threw: ${threw}`);
  return { shown, bootLog, threw, get, fileDrop };
}

const cases = [
  ['configured instance, core ready', CONFIGURED, NO_SETUP],
  ['configured instance, core still starting', { ...CONFIGURED, phase: 'starting' }, NO_SETUP],
  ['nothing offered yet (nil pending lists)', FRESH, NO_SETUP],
  ['first run, wizard needed', FRESH, WANTS_SETUP],
  ['every Go list marshalled as null', CONFIGURED, NO_SETUP],
];

for (const [label, snapshot, setup] of cases) {
  const { shown, threw, bootLog, fileDrop } = await run(label, snapshot, setup);
  check(`${label}: something is visible`, shown.length > 0, 'every screen hidden — blank window');
  check(`${label}: boot() did not throw`, !threw, threw);
  check(`${label}: journal reached the end`, bootLog.includes('done') || bootLog.includes('wizard shown'), bootLog.join(' > '));
  // A dropped file navigates the window to itself unless Wails' own drop
  // listener is installed, and Wails only installs it on the page's request.
  // Exactly one request, without the drop-target filter: anywhere in the window
  // accepts a drop.
  check(`${label}: the page asked Wails to listen for drops`,
    fileDrop.length === 1 && fileDrop[0].useDropTarget === false, `OnFileDrop calls: ${JSON.stringify(fileDrop.map((c) => c.useDropTarget))}`);
}

console.log(failures ? `\n${failures} check(s) failed` : '\nno blank-window path found');
process.exit(failures ? 1 : 0);
