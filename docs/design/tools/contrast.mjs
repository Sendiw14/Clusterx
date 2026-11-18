#!/usr/bin/env node
// WCAG 2.x contrast calculator for CLUSTERX tokens. Reads packages/ui/tokens/tokens.css (single source).
// Usage:  node docs/design/tools/contrast.mjs                 -> prints table, exit 1 if a REQUIRED pair fails
//         node docs/design/tools/contrast.mjs --inject FILE   -> also rewrites the block between the
//                                                                 <!-- CONTRAST:BEGIN --> / <!-- CONTRAST:END --> markers in FILE
// Relative luminance and ratio per WCAG 2.2 definition (sRGB, 0.04045 threshold).
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const css = readFileSync(join(here, '../../../packages/ui/tokens/tokens.css'), 'utf8');
const tok = {};
for (const m of css.matchAll(/--([a-z0-9-]+):\s*(#[0-9A-Fa-f]{6})\b/g)) tok[m[1]] = m[2].toUpperCase();
for (const m of css.matchAll(/--([a-z0-9-]+):\s*var\(--([a-z0-9-]+)\)/g)) tok[m[1]] = tok[m[2]];
const hex = (n) => { if (!tok[n]) throw new Error('unknown token ' + n); return tok[n]; };

const lin = (c) => { c /= 255; return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; };
const lum = (h) => { const n = parseInt(h.slice(1), 16); return 0.2126 * lin(n >> 16) + 0.7152 * lin((n >> 8) & 255) + 0.0722 * lin(n & 255); };
const ratio = (a, b) => { const x = lum(a), y = lum(b); return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05); };

const LIGHT = ['color-white', 'color-canvas', 'color-panel'];
const rows = []; // {kind, fg, bg, need, required, note}
const add = (kind, fg, bgs, need, note = '', required = true) => bgs.forEach((bg) => rows.push({ kind, fg, bg, need, required, note }));

// Text (AA 4.5:1)
add('text', 'color-ink', LIGHT, 4.5, 'primary text');
add('text', 'color-ink-2', LIGHT, 4.5, 'secondary text');
add('text', 'color-ink-3', LIGHT, 4.5, 'tertiary/meta text');
add('text', 'color-accent', [...LIGHT, 'color-accent-tint'], 4.5, 'links, accent text');
add('text', 'color-accent-ink', [...LIGHT, 'color-accent-tint'], 4.5, 'hover/active links, chips');
add('text', 'color-success-ink', [...LIGHT, 'color-success-tint'], 4.5, 'success text, chips');
add('text', 'color-warning-ink', [...LIGHT, 'color-warning-tint'], 4.5, 'warning text, chips');
add('text', 'color-danger-ink', [...LIGHT, 'color-danger-tint'], 4.5, 'danger text, chips');
add('text', 'color-danger', ['color-white', 'color-canvas', 'color-danger-tint'], 4.5, 'danger as text only where listed (white, canvas, tint)');
add('text', 'color-ink', ['color-accent-tint', 'color-success-tint', 'color-warning-tint', 'color-danger-tint'], 4.5, 'body text on tints');
add('text', 'color-ink-2', ['color-accent-tint', 'color-success-tint', 'color-warning-tint', 'color-danger-tint'], 4.5, 'secondary text on tints');
add('text', 'color-white', ['color-accent', 'color-accent-ink', 'color-danger', 'color-ink'], 4.5, 'text on filled buttons');
for (const c of ['code-ink', 'code-mute', 'code-key', 'code-str', 'code-num', 'code-err'].map((x) => 'color-' + x)) add('text', c, ['color-code-bg'], 4.5, 'code block syntax');

// Non-text (3:1)
add('non-text', 'color-line-strong', LIGHT, 3, 'control boundary');
add('non-text', 'color-focus', LIGHT, 3, 'focus ring');
add('non-text', 'color-focus-on-dark', ['color-code-bg'], 3, 'focus ring on code block');
for (const c of ['color-success', 'color-warning', 'color-danger', 'color-neutral', 'color-accent']) add('non-text', c, ['color-white', 'color-canvas'], 3, 'status glyph / chart line');
add('non-text', 'color-success', ['color-success-tint'], 3, 'glyph on its tint');
add('non-text', 'color-warning', ['color-warning-tint'], 3, 'glyph on its tint');
add('non-text', 'color-danger', ['color-danger-tint'], 3, 'glyph on its tint');
add('non-text', 'color-neutral', ['color-panel'], 3, 'glyph on panel');
add('non-text', 'color-accent', ['color-accent-tint'], 3, 'glyph on its tint');
add('non-text', 'color-ink-2', ['color-white'], 3, 'chart axis / tick');

// Documented rejections: measured, deliberately NOT used for the stated purpose.
const rej = [];
const rejectPair = (fg, bg, need, why) => rej.push({ fg, bg, need, why });
rejectPair('color-success', 'color-white', 4.5, 'starting token fails as text; use color-success-ink for text, base only for glyphs/bars');
rejectPair('color-warning', 'color-white', 4.5, 'starting token fails as text; use color-warning-ink for text, base only for glyphs/bars');
rejectPair('color-neutral', 'color-panel', 4.5, 'fails as text on panel; neutral state text uses color-ink-2');
rejectPair('color-ink-disabled', 'color-white', 4.5, 'disabled controls only; WCAG 1.4.3 exempts inactive components, state also stated in text');
rejectPair('color-line', 'color-white', 3, 'decorative divider only; never the sole boundary of an input or button');

const f = (n) => n.toFixed(2);
const out = [];
const fails = [];
out.push('| Kind | Foreground token | Hex | Background token | Hex | Ratio | Required | Result | Use |');
out.push('|---|---|---|---|---|---|---|---|---|');
for (const r of rows) {
  const v = ratio(hex(r.fg), hex(r.bg));
  const ok = v >= r.need;
  if (!ok && r.required) fails.push(r);
  out.push(`| ${r.kind} | \`${r.fg}\` | ${hex(r.fg)} | \`${r.bg}\` | ${hex(r.bg)} | ${f(v)} | ${r.need}:1 | ${ok ? 'PASS' : 'FAIL'} | ${r.note} |`);
}
const rout = ['| Foreground | Hex | Background | Hex | Ratio | Needs | Result | Decision |', '|---|---|---|---|---|---|---|---|'];
for (const r of rej) {
  const v = ratio(hex(r.fg), hex(r.bg));
  rout.push(`| \`${r.fg}\` | ${hex(r.fg)} | \`${r.bg}\` | ${hex(r.bg)} | ${f(v)} | ${r.need}:1 | ${v >= r.need ? 'PASS' : 'FAIL'} | ${r.why} |`);
}
const summary = `Pairs checked: ${rows.length} required (${rows.filter((r) => ratio(hex(r.fg), hex(r.bg)) >= r.need).length} pass, ${fails.length} fail); ${rej.length} documented rejections.`;
const block = ['### Required pairs', '', ...out, '', '### Measured and deliberately rejected for the stated use', '', ...rout, '', summary].join('\n');

const inj = process.argv.indexOf('--inject');
if (inj > -1) {
  const file = process.argv[inj + 1];
  const src = readFileSync(file, 'utf8');
  const b = '<!-- CONTRAST:BEGIN -->', e = '<!-- CONTRAST:END -->';
  const i = src.indexOf(b), j = src.indexOf(e);
  if (i < 0 || j < 0) throw new Error('markers not found in ' + file);
  writeFileSync(file, src.slice(0, i + b.length) + '\n' + block + '\n' + src.slice(j));
}
console.log(block);
if (fails.length) { console.error('FAIL: ' + fails.length + ' required pair(s) below threshold'); process.exit(1); }
