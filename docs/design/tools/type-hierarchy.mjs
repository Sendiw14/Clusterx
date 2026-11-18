// Computed-style type hierarchy check: heading size vs body size per prototype at 390 and 1440.
// Run: PW_MODULE=/path/to/playwright/index.mjs node docs/design/tools/type-hierarchy.mjs > docs/design/evidence/type-hierarchy.txt
import { fileURLToPath, pathToFileURL } from 'node:url';
import { dirname, join } from 'node:path';
const proto = join(dirname(fileURLToPath(import.meta.url)), '../prototypes');
const { chromium } = await import(process.env.PW_MODULE ? pathToFileURL(process.env.PW_MODULE).href : 'playwright');
const b = await chromium.launch();
for (const w of [390, 1440]) for (const f of ['home','overview','jobs','job-detail','workers','login']) {
  const p = await b.newPage({viewport:{width:w,height:900}});
  await p.goto(pathToFileURL(join(proto, f + '.html')).href + '#noauto'); await p.waitForTimeout(300);
  const r = await p.evaluate(() => { const fs = e => parseFloat(getComputedStyle(e).fontSize); const vis = e => e.getClientRects().length;
    const body = fs(document.body); const g = s => [...document.querySelectorAll(s)].filter(vis).map(fs);
    return {body, h1: Math.max(0,...g('h1')), h2: Math.max(0,...g('main h2')), h3: Math.max(0,...g('main h3')), kpi: Math.max(0,...g('.kpi-v'))}; });
  console.log(f, w, JSON.stringify(r), 'h1/body', (r.h1/r.body).toFixed(2), 'h2/body', (r.h2/r.body).toFixed(2));
  await p.close();
}
await b.close();
