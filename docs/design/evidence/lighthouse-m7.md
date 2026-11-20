# Lab Web Vitals, M7 public pages (TASK-M7-FE-01, NFR-PERF-01)

> **Superseded for the shipped build by `lighthouse-m9.md`** (re-measured 2026-11-20 after AUDIT-M9 F-05 restored static rendering; adds `/architecture` and three runs per cell). This file is kept as the M7 record: its pages were static then, but a later change made them dynamic until the F-05 fix.

**Lab numbers only. Field data (CrUX, real-user LCP, INP, CLS) is unavailable: the site is not deployed and has no users.** Interaction to Next Paint (INP) is a field metric and was not measured; Total Blocking Time (TBT) is the lab proxy shown here.

- Tool: `npx lighthouse@13.6.0` (pinned), categories performance, accessibility, best-practices, seo.
- Target: the production build served by `next start` on `http://localhost:3300`, same machine, no CDN, no compression proxy, no network between client and server.
- Browser: Google Chrome for Testing (Playwright `chromium-1255` cache), headless, `--no-sandbox`.
- Throttling: Lighthouse defaults (`simulate`): mobile preset (Moto G Power profile, slow 4G, 4x CPU) and desktop preset.
- Machine: macOS arm64 laptop. One run per page and form factor (no median of several runs), so treat differences of a few points as noise.
- Raw extract: `lighthouse-m7.json` in this directory.

| Page | Preset | Perf | LCP | CLS | TBT | FCP | A11y | Best practices | SEO |
|---|---|---|---|---|---|---|---|---|---|
| `/` | mobile | 100 | 1.1 s | 0 | 10 ms | 1.1 s | 100 | 100 | 100 |
| `/` | desktop | 99 | 0.9 s | 0.001 | 0 ms | 0.3 s | 100 | 100 | 100 |
| `/docs` | mobile | 100 | 0.8 s | 0 | 0 ms | 0.8 s | 100 | 100 | 100 |
| `/docs` | desktop | 100 | 0.2 s | 0 | 0 ms | 0.2 s | 100 | 100 | 100 |
| `/demo` | mobile | 100 | 0.9 s | 0 | 10 ms | 0.9 s | 100 | 100 | 100 |
| `/demo` | desktop | 100 | 0.2 s | 0.021 | 0 ms | 0.2 s | 100 | 100 | 100 |

## Findings that changed the code

1. First run, `/` mobile: performance 87, LCP 4.1 s. The LCP element was the "Fig. 1" ledger plate in the hero, wrapped in the scroll-reveal component, so it stayed at opacity 0 until JavaScript hydrated. Fixed: the plate uses the CSS-only hero keyframe (`.hero-rise`), no script dependency.
2. Second run, `/` mobile: 90, LCP 3.6 s, LCP element the eyebrow text, waiting on web fonts. Fixed: preload of IBM Plex Mono 400 and Newsreader roman, which the hero uses.
3. Best-practices 96 (a 404 for `/favicon.ico` logged to the console). Fixed: `src/app/icon.svg` (the CLUSTERX mark).
4. `/demo` mobile once failed with Lighthouse `NO_NAVSTART` (a trace recording flake, not a page error); it was re-run and the figures above are from the successful run.

## Limits of this evidence

- Single run per cell; no variance estimate.
- Localhost serving removes real network latency, so absolute LCP on a real host will differ. The mobile preset's simulated throttling is the only network model here.
- Lighthouse scores say nothing about WCAG conformance; accessibility evidence is axe plus the Playwright keyboard tests, and manual screen-reader walkthroughs were not done.
- Pages not measured: `/architecture`, `/changelog`, `/docs/*` pages other than the index (the generated API page is the largest HTML document and was not measured).
- Client JavaScript size was not budgeted or measured separately.
