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
