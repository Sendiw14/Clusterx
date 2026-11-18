#!/usr/bin/env node
// Renders prototypes headlessly: screenshots + offline/overflow/console checks.
// Setup (once, scratch dir):  npm i playwright@1 && npx playwright install chromium
// Run:  PW_MODULE=/path/to/node_modules/playwright/index.mjs node docs/design/tools/render.mjs [--only name] [--checks-only]
// Network is blocked for everything that is not file://, so any external request is recorded as a failure.
import { fileURLToPath, pathToFileURL } from 'node:url';
import { dirname, join } from 'node:path';
import { mkdirSync, writeFileSync } from 'node:fs';

const here = dirname(fileURLToPath(import.meta.url));
const proto = join(here, '../prototypes');
const shots = join(here, '../screenshots');
const evidence = join(here, '../evidence');
mkdirSync(shots, { recursive: true });
const { chromium } = await import(process.env.PW_MODULE ? pathToFileURL(process.env.PW_MODULE).href : 'playwright');

const only = process.argv.includes('--only') ? process.argv[process.argv.indexOf('--only') + 1] : null;
const checksOnly = process.argv.includes('--checks-only');

// name, file, hash, width, height, extra JS run before capture, clip selector (optional)
const S = [
  ['home-390', 'home', '#noauto', 390, 844], ['home-1440', 'home', '#noauto', 1440, 900],
  ['home-atlas-step5-1440', 'home', '#noauto', 1440, 900, 'CLX.atlasGo(5)', '#atlas-h'],
  ['home-atlas-step8-1440', 'home', '#noauto', 1440, 900, 'CLX.atlasGo(8)', '#atlas-h'],
  ['login-390', 'login', '', 390, 844], ['login-1440', 'login', '', 1440, 900],
  ['login-no-entitlement-390', 'login', '#state=no_entitlement', 390, 844],
  ['overview-390', 'overview', '', 390, 844], ['overview-1440', 'overview', '', 1440, 900],
  ['overview-stale-1440', 'overview', '#state=stale', 1440, 900],
  ['overview-first-use-390', 'overview', '#state=first_use', 390, 844],
  ['jobs-390', 'jobs', '', 390, 844], ['jobs-1440', 'jobs', '', 1440, 900],
  ['jobs-empty-filter-1440', 'jobs', '#state=empty_filter', 1440, 900],
  ['jobs-error-1440', 'jobs', '#state=error', 1440, 900],
  ['jobs-stale-390', 'jobs', '#state=stale', 390, 844],
  ['job-detail-390', 'job-detail', '', 390, 844], ['job-detail-1440', 'job-detail', '', 1440, 900],
  ['job-detail-failed-1440', 'job-detail', '#state=failed', 1440, 900],
  ['job-detail-viewer-1440', 'job-detail', '#state=failed&role=viewer', 1440, 900],
  ['job-detail-expired-1440', 'job-detail', '#state=expired', 1440, 900],
  ['job-detail-unschedulable-390', 'job-detail', '#state=unschedulable', 390, 844],
  ['workers-390', 'workers', '', 390, 844], ['workers-1440', 'workers', '', 1440, 900],
  ['workers-100-1440', 'workers', '#scale=100&view=graph', 1440, 900],
  ['workers-none-1440', 'workers', '#state=none', 1440, 900],
];
// overflow/offline checks at all agreed widths for the default state of every page (+ key states at 320)
const PAGES = ['home', 'login', 'overview', 'jobs', 'job-detail', 'workers'];
const WIDTHS = [320, 360, 390, 768, 1024, 1440, 1920];
const browser = await chromium.launch();
const report = { generated: new Date().toISOString(), browser: 'chromium ' + browser.version(), pages: [], failures: [] };

async function open(file, hash, width, height, rm = false) {
  const ctx = await browser.newContext({ viewport: { width, height }, timezoneId: 'UTC', locale: 'en-GB', reducedMotion: rm ? 'reduce' : 'no-preference', deviceScaleFactor: 1 });
  const page = await ctx.newPage();
  const bad = [], errs = [], seen = [];
  await page.route('**/*', (r) => { const u = r.request().url(); if (u.startsWith('file:') || u.startsWith('data:') || u.startsWith('about:')) { seen.push(u); return r.continue(); } bad.push(u); return r.abort(); });
  page.on('console', (m) => { if (m.type() === 'error') errs.push(m.text()); });
  page.on('pageerror', (e) => errs.push('pageerror: ' + e.message));
  await page.goto(pathToFileURL(join(proto, file + '.html')).href + hash);
  await page.waitForLoadState('load');
  return { ctx, page, bad, errs };
}
async function settle(page) {
  await page.evaluate(async () => { const h = document.documentElement.scrollHeight; for (let y = 0; y < h; y += 500) { window.scrollTo(0, y); await new Promise((r) => setTimeout(r, 40)); } window.scrollTo(0, 0); });
  await page.waitForTimeout(450);
}

if (!checksOnly) {
  for (const [name, file, hash, w, h, js, clip] of S) {
    if (only && name !== only) continue;
    const { ctx, page } = await open(file, hash, w, h);
    await page.waitForTimeout(700);
    await settle(page);
    if (js) { await page.evaluate(js); await page.waitForTimeout(900); }
    if (clip) {
      const el = await page.locator(clip).first().locator('xpath=ancestor::section[1]');
      await el.screenshot({ path: join(shots, name + '.png') });
    } else await page.screenshot({ path: join(shots, name + '.png'), fullPage: true });
    console.log('shot', name);
    await ctx.close();
  }
}

for (const file of (only ? [] : PAGES)) {
  const rows = [];
  for (const w of WIDTHS) {
    const { ctx, page, bad, errs } = await open(file, '#noauto', w, 900);
    await page.waitForTimeout(300);
    const m = await page.evaluate(() => ({ sw: document.documentElement.scrollWidth, cw: document.documentElement.clientWidth, title: document.title, h1: [...document.querySelectorAll('h1')].filter((e) => e.getClientRects().length).length, main: document.querySelectorAll('main').length, proto: /design prototype — illustrative data/.test(document.body.innerText), langs: document.documentElement.lang }));
    const overflow = m.sw > m.cw;
    rows.push({ width: w, scrollWidth: m.sw, clientWidth: m.cw, horizontalOverflow: overflow, externalRequests: bad.length, consoleErrors: errs.length });
    if (overflow) report.failures.push(`${file}@${w}: horizontal overflow ${m.sw}>${m.cw}`);
    if (bad.length) report.failures.push(`${file}@${w}: external requests ${bad.join(',')}`);
    if (errs.length) report.failures.push(`${file}@${w}: console errors ${errs.join(' | ')}`);
    if (m.h1 !== 1 || m.main !== 1 || !m.proto) report.failures.push(`${file}@${w}: structure h1=${m.h1} main=${m.main} protoLabel=${m.proto}`);
    await ctx.close();
  }
  // reduced motion: no running animations/transform on load after settle
  const { ctx, page } = await open(file, '#noauto', 390, 844, true);
  await page.waitForTimeout(600);
  const anims = await page.evaluate(() => document.getAnimations().filter((a) => a.playState === 'running').map((a) => (a.animationName || a.transitionProperty || '?')));
  const rmok = anims.length === 0;
  if (!rmok) report.failures.push(`${file}: reduced-motion still running animations: ${anims.join(',')}`);
  await ctx.close();
  report.pages.push({ file, widths: rows, reducedMotionRunningAnimations: anims });
}
await browser.close();
if (!only) writeFileSync(join(evidence, 'render-check.json'), JSON.stringify(report, null, 2));
console.log(report.failures.length ? 'FAILURES:\n' + report.failures.join('\n') : 'all render checks passed');
process.exitCode = report.failures.length ? 1 : 0;
