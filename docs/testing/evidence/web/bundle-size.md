# Web client JavaScript size per route group (F-09 bundle size, TASK-M9-FE-03)

Measured from the production build, not estimated. Raw data: `bundle-size.json` (same folder). Reproduce: `pnpm --filter web build`, then `cd apps/web && node scripts/bundle-size.mjs ../../docs/testing/evidence/web/bundle-size.json`.

## Method and limits

- Source: `.next/diagnostics/route-bundle-stats.json`, written by `next build` (Next 16.4.6, Turbopack). It lists, per route, the chunks the browser loads on first navigation. The script reads those chunk files and measures raw bytes, gzip level 9 and Brotli quality 11. For every route the script checks that its raw total equals Next's own `firstLoadUncompressedJsBytes` and fails if not (it did not fail).
- Build: this repository at `da506db` plus the TASK-M9-FE-03 changes, on macOS arm64, Node 26.13.1, pnpm 9.15.9. Sizes depend on the source and the compiler, not on the machine.
- **What is not measured:** JavaScript loaded after first navigation (dynamic imports, route prefetch of other routes), CSS, fonts, images, HTML, and the WebSocket stream. Real transfer sizes depend on the host's compression: the production Caddy example compresses, but this audit did not capture served sizes. `gzip -9` and `brotli -11` are lower bounds of what a server will send.
- The legacy polyfill file (112,594 B raw, 39,373 B gzip, 35,158 B Brotli) is listed apart: Next loads it only for browsers that need it.
- All eleven `/app/*` routes report the **same chunk set** (593,277 B). Turbopack shares one set for the console, so per-page console cost cannot be split from this data. Build output of the other groups is also near-identical (498,182 B is the floor for every route).

## Results (first-load JS, KB = 1000 B)

| Group | Routes | Raw | gzip -9 | Brotli -11 |
|---|---|---|---|---|
| public (static) | `/architecture`, `/changelog` | 498.2 | 148.3 | 127.1 |
| public (static) | `/` | 519.1 | 154.3 | 132.3 |
| docs (static) | `/docs` | 505.5 | 151.0 | 129.5 |
| docs (static) | `/docs/api`, `/concepts`, `/deployment`, `/quickstart`, `/reliability`, `/security`, `/workers` | 506.7 | 151.7 | 130.0 |
| demo (static) | `/demo` | 553.0 | 164.8 | 141.5 |
| console (dynamic) | all 11 `/app/*` routes | 593.3 | 177.8 | 153.2 |
| auth (dynamic) | `/login` | 498.2 | 148.3 | 127.1 |
| not found | `/_not-found`, `/[...unmatched]` | 498.2 | 148.3 | 127.1 |

Reading it: about 498 KB raw (148 KB gzip) is common to every route. That floor is what the framework and the shared layout ship; this data does not say how it splits between React, Next and project code, and nobody has investigated it. The demo adds about 55 KB raw (16 KB gzip) over the floor, the console about 95 KB raw (30 KB gzip).

## Proposed budgets (proposal only: not frozen, owner decision OD-06)

First-load JS, gzip -9, roughly 10 to 15 percent above the measured value so that ordinary changes pass and a new dependency does not:

| Group | Measured | Proposed ceiling |
|---|---|---|
| public and docs | 148 to 154 KB | 170 KB |
| demo | 165 KB | 190 KB |
| console (`/app/*`) | 178 KB | 205 KB |

No CI check enforces these numbers. A gate would be a few lines around `scripts/bundle-size.mjs`; it was not added, because the ceilings are not agreed. These numbers say nothing about runtime speed. Lab Lighthouse and console p95 timings are in `docs/design/evidence/lighthouse-m9.md` and `console-budgets.md`.
