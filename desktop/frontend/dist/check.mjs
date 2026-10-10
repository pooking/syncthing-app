// Checks the desktop frontend's script files without a browser: that they parse,
// that the two translation catalogs stay in step, and that every key the markup
// or the code asks for actually exists.
//
// Run with: node desktop/frontend/dist/check.mjs
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const read = (name) => readFileSync(join(here, name), 'utf8');

let failures = 0;
const check = (name, ok, detail = '') => {
  if (!ok) failures++;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${detail && !ok ? `  — ${detail}` : ''}`);
};

// The catalog is a plain object literal, so it can be evaluated directly.
const catalogSource = read('i18n.js').match(/const CATALOG = (\{[\s\S]*?\n\});/);
check('the catalog parses out of i18n.js', Boolean(catalogSource));
const CATALOG = eval(`(${catalogSource[1]})`);

const en = Object.keys(CATALOG.en);
const zh = Object.keys(CATALOG.zh);
check('both languages have the same number of keys', en.length === zh.length, `en=${en.length} zh=${zh.length}`);
const missingZh = en.filter((k) => !(k in CATALOG.zh));
const missingEn = zh.filter((k) => !(k in CATALOG.en));
check('no key is missing from either language', !missingZh.length && !missingEn.length,
  `zh missing ${missingZh}, en missing ${missingEn}`);

// Every element the markup translates must have a translation.
const html = read('index.html');
const htmlKeys = [...html.matchAll(/data-i18n(?:-ph|-title|-alt)?="([^"]+)"/g)].map((m) => m[1]);
const badHtml = htmlKeys.filter((k) => !(k in CATALOG.en));
check(`all ${htmlKeys.length} markup keys resolve`, !badHtml.length, badHtml.join(', '));

// A t() call with a literal key must supply a value for every {placeholder} in
// the text it selects. Missing one is not an error the code can throw: t()
// substitutes what it is given and leaves the rest alone, so the placeholder
// reaches the user verbatim — which is how "不属于"{name}"的文件" got into a
// confirmation dialog. Only literal keys are checked; calls that build the key
// at runtime ("folder." + state) cannot be resolved here.
const app = read('app.js');

// Params objects assigned to a name first, so `t(key, params)` resolves too.
const paramsObjects = new Map();
for (const m of app.matchAll(/(?:const|let)\s+([A-Za-z_$][\w$]*)\s*=\s*\{([^{}]*)\}/g)) {
  paramsObjects.set(m[1], m[2]);
}

function suppliedNames(arg) {
  const body = arg.startsWith('{') ? arg.slice(1, -1) : paramsObjects.get(arg);
  if (body === undefined) return null; // built elsewhere; not resolvable statically
  const names = new Set();
  for (const part of body.split(',')) {
    const named = part.match(/^\s*([A-Za-z_$][\w$]*)\s*:/);
    if (named) { names.add(named[1]); continue; }
    const shorthand = part.match(/^\s*([A-Za-z_$][\w$]*)\s*$/);
    if (shorthand) names.add(shorthand[1]);
  }
  return names;
}

const unfilled = [];
let callsChecked = 0;
for (const m of app.matchAll(/\bt\(\s*'([^']+)'\s*,\s*(\{[^{}]*\}|[A-Za-z_$][\w$]*)\s*\)/g)) {
  const supplied = suppliedNames(m[2]);
  if (!supplied) continue;
  callsChecked++;
  for (const lang of ['en', 'zh']) {
    const text = CATALOG[lang][m[1]];
    if (text === undefined) continue;
    for (const ph of new Set([...text.matchAll(/\{(\w+)\}/g)].map((p) => p[1]))) {
      if (!supplied.has(ph)) unfilled.push(`${m[1]} (${lang}) has no value for {${ph}}`);
    }
  }
}
check(`${callsChecked} t() calls fill every placeholder their text uses`,
  !unfilled.length, unfilled.join(', '));

// A message key holding a {placeholder} and asked for by nobody would ship the
// braces verbatim too, so an unused one is dead weight rather than harmless.
// Keys whose name is built at runtime are not literal calls; each prefix here
// has to actually appear concatenated inside a t() call, so this exception list
// cannot quietly grow into a hole.
const literalKeys = new Set([...app.matchAll(/\bt\(\s*'([^']+)'/g)].map((m) => m[1]));
const RUNTIME_PREFIXES = ['folder.', 'code.'];
const unbacked = RUNTIME_PREFIXES.filter((p) => !app.includes(`t('${p}' +`));
check('every runtime key prefix is really built at runtime',
  !unbacked.length, `${unbacked.join(', ')} is not concatenated into any t() call`);

const orphan = [];
for (const lang of ['en', 'zh']) {
  for (const [key, text] of Object.entries(CATALOG[lang])) {
    if (!/\{\w+\}/.test(text)) continue;
    if (literalKeys.has(key)) continue;
    if (RUNTIME_PREFIXES.some((p) => key.startsWith(p))) continue;
    orphan.push(`${key} (${lang})`);
  }
}
check('every templated key is asked for by a literal t() call',
  !orphan.length, orphan.join(', '));

// Folder state names are built at runtime as "folder." + state, so they are not
// visible to a plain t() search; check them against the states the Go side emits.
const stateNames = ['paused', 'idle', 'scanning', 'syncing', 'syncPreparing', 'error',
  'sync-preparing', 'sync-waiting', 'scan-waiting', 'cleaning'];
const badStates = stateNames.filter((n) => !(`folder.${n}` in CATALOG.en) || !(`folder.${n}` in CATALOG.zh));
check('every folder state has a label', !badStates.length, badStates.join(', '));

// The frontend re-renders a message itself only when it arrives inside a
// FolderConflict from a rejected drop or file picker selection. Everything else
// (failed saves, tray labels, move errors) reaches the user as text the Go side
// already translated into the active language. So the codes below are the ones
// that must exist here; the list is explicit because which codes travel in a
// FolderConflict is a property of appcore/folders.go, not something the catalog
// can be checked against mechanically.
const RENDERED_CODES = [
  'empty-path', 'path-missing', 'not-a-directory', 'cannot-resolve',
  'filesystem-root', 'already-shared', 'overlaps',
];
const missingRendered = RENDERED_CODES.filter((c) => !(`code.${c}` in CATALOG.en) || !(`code.${c}` in CATALOG.zh));
check(`all ${RENDERED_CODES.length} frontend-rendered codes are translated`,
  !missingRendered.length, missingRendered.join(', '));

// The other direction: a translation with no Go code behind it is stale.
const goSource = readFileSync(join(here, '..', '..', '..', 'appcore', 'i18n.go'), 'utf8');
const goCodes = new Set([...goSource.matchAll(/^\tCode\w+\s+Code = "([^"]+)"/gm)].map((m) => m[1]));
check(`found the Go message codes (${goCodes.size})`, goCodes.size > 20, 'the extraction pattern is stale');
const catalogCodes = en.filter((k) => k.startsWith('code.')).map((k) => k.slice('code.'.length));
const stale = catalogCodes.filter((c) => !goCodes.has(c));
check(`no translation outlives its Go code (${catalogCodes.length} checked)`, !stale.length, stale.join(', '));

// The scripts must at least parse.
for (const file of ['app.js', 'i18n.js']) {
  try {
    new Function(read(file));
    check(`${file} parses`, true);
  } catch (e) {
    check(`${file} parses`, false, e.message);
  }
}

console.log(failures ? `\n${failures} check(s) failed` : '\nall frontend checks passed');
process.exit(failures ? 1 : 0);
