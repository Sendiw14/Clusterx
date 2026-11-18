#!/usr/bin/env node
// Builds self-contained prototypes: docs/design/prototypes/_src/<page>.html -> docs/design/prototypes/<page>.html
// Inlines tokens.css (verbatim copy), shared.css, shared.js, sprite and partials. No network references are emitted.
// Usage: node docs/design/tools/build-prototypes.mjs [--check]   (--check exits 1 if outputs are stale)
import { readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const proto = join(here, '../prototypes');
const src = join(proto, '_src');
const tokens = readFileSync(join(here, '../../../packages/ui/tokens/tokens.css'), 'utf8');
const read = (f) => readFileSync(join(src, f), 'utf8');
const check = process.argv.includes('--check');

function includePartial(text) {
  return text.replace(/<!--@include ([\w.-]+)((?:\s+[\w-]+=[^\s>]+)*)\s*-->/g, (_, file, attrs) => {
    const vars = Object.fromEntries([...attrs.matchAll(/([\w-]+)=([^\s>]+)/g)].map((m) => [m[1], m[2]]));
    let body = read(file).replace(/\{\{(\w+)\}\}/g, (m, k) => vars[k] ?? '');
    body = includePartial(body);
    body = body.replace(/<a ([^>]*?)data-nav="([\w-]+)"([^>]*)>/g, (m, a, nav, b) => (vars.cur === nav && !m.includes('aria-current') ? `<a ${a}data-nav="${nav}"${b} aria-current="page">` : m));
    return body;
  });
}

// Standalone sprite for the asset inventory (same symbols the prototypes inline).
{
  const sprite = read('sprite.html').replace(/<svg[^>]*>/, '<svg xmlns="http://www.w3.org/2000/svg">');
  const out = join(here, '../assets/icons-sprite.svg');
  if (!check) writeFileSync(out, sprite);
}
let stale = 0;
for (const f of readdirSync(src).filter((x) => x.endsWith('.html') && !x.startsWith('p-') && x !== 'sprite.html')) {
  let html = includePartial(read(f));
  html = html
    .replace('<!--@tokens-->', () => `<style id="tokens">\n${tokens}</style>`)
    .replace('<!--@css-->', () => `<style id="shared">\n${read('shared.css')}</style>`)
    .replace('<!--@sprite-->', () => read('sprite.html'))
    .replace('<!--@js-->', () => `<script>\n${read('shared.js')}</script>`);
  const out = join(proto, f);
  if (check) {
    if (!existsSync(out) || readFileSync(out, 'utf8') !== html) { console.error('STALE ' + f); stale++; }
  } else {
    writeFileSync(out, html);
    const ext = [...html.matchAll(/(?:src|href)=["'](https?:)?\/\/[^"']+/g)].filter((m) => !m[0].includes('www.w3.org'));
    console.log(`built ${f} (${(html.length / 1024).toFixed(1)} KiB), external refs: ${ext.length}`);
    if (ext.length) { console.error('external reference found in ' + f); process.exitCode = 1; }
  }
}
if (stale) process.exit(1);
