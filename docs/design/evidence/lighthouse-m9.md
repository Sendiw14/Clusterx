# Lab Web Vitals, public pages re-measured after the F-05 fix (TASK-M9-FE-02, NFR-PERF-01, AC-24)

**Lab numbers only. Field data (CrUX, real-user LCP, INP, CLS) does not exist: the site is not deployed and has no users.** INP is a field metric and was not measured; Total Blocking Time (TBT) is the lab proxy.

## Why this was measured again

`lighthouse-m7.md` was taken when the public pages were prerendered. A later change (`52ca940`) put `await connection()` in the root `not-found.tsx`, which turned every route into a per-request render, so the M7 figures stopped describing the build for a while (AUDIT-M9-RV-01 F-05). That intermediate state was **never measured**. The fix (commit `db84145`) makes the public routes static again: see `docs/testing/evidence/web/next-build-route-table.txt` (`○` for `/`, `/architecture`, `/changelog`, `/demo`, `/docs*`), and `apps/web/e2e/static-public.spec.ts`, which fails if a public route stops being prerendered or loses the static CSP. These measurements are of the fixed build.

## Method

- Tool: `npx lighthouse@13.6.0` (pinned), categories performance, accessibility, best-practices, seo, JSON output (`lighthouse-m9.json` here holds every run).
- Target: the production build served by `node server.ts` (the production entry that `pnpm --filter web start` runs) on `http://localhost:3300`, same machine, no CDN, no compression proxy, no network between client and server. The server had no control plane behind it: public pages only.
- Browser: Chrome for Testing from Playwright's `chromium-1255` cache (Playwright reports 159.0.8154.6; Lighthouse's emulated user-agent string says `HeadlessChrome/157`), headless, `--no-sandbox`.
- Throttling: Lighthouse defaults (`simulate`): mobile preset (Moto G Power profile, slow 4G, 4x CPU) and the desktop preset.
- Machine: Apple M4 (10 cores, 16 GiB), macOS arm64 (Darwin 27.0), Node 26.13.1. The machine was otherwise idle; no test suite ran during the runs.
- **Three runs per page and form factor; the table shows the median** and the last column the min-max of performance score and LCP over the three. Date 2026-11-20.

## Results (median of 3)

| Page | Preset | Perf | LCP | CLS | TBT | FCP | A11y | Best practices | SEO | Range over 3 runs (perf / LCP) |
|---|---|---|---|---|---|---|---|---|---|---|
| `/` | mobile | 100 | 1.1 s | 0 | 5 ms | 1.1 s | 100 | 100 | 100 | 100-100 / 1055-1092 ms |
| `/` | desktop | 99 | 0.9 s | 0.001 | 0 ms | 0.3 s | 100 | 100 | 100 | 99-99 / 897-900 ms |
| `/docs` | mobile | 100 | 0.8 s | 0 | 5 ms | 0.8 s | 100 | 100 | 100 | 100-100 / 755-755 ms |
| `/docs` | desktop | 100 | 0.2 s | 0 | 0 ms | 0.2 s | 100 | 100 | 100 | 100-100 / 204-205 ms |
| `/demo` | mobile | 100 | 0.9 s | 0 | 6 ms | 0.9 s | 100 | 100 | 100 | 100-100 / 905-906 ms |
| `/demo` | desktop | 100 | 0.2 s | 0.021 | 0 ms | 0.2 s | 100 | 100 | 100 | 100-100 / 245-245 ms |
| `/architecture` | mobile | 100 | 1.1 s | 0 | 5 ms | 1.1 s | 100 | 100 | 100 | 100-100 / 1053-1059 ms |
| `/architecture` | desktop | 100 | 0.3 s | 0 | 0 ms | 0.3 s | 100 | 100 | 100 | 100-100 / 284-286 ms |

The scores equal the M7 figures for `/`, `/docs` and `/demo` (the same numbers within a few ms); `/architecture` is new. Lighthouse in `simulate` mode is largely deterministic on one machine, so the three runs barely differ: the narrow ranges say the lab is repeatable, not that real users will see the same.

## Limits of this evidence

- Localhost serving removes real network latency; absolute LCP on a real host will differ. The mobile preset's simulated throttling is the only network model.
- Lighthouse scores say nothing about WCAG conformance (see `docs/design/evidence/axe-m6.md`; manual screen-reader and keyboard walkthroughs are NOT RUN).
- Not measured: `/changelog`, `/login`, `/docs/*` pages other than the index (the generated API page is the largest document).
- Client JavaScript size was not budgeted. The console's own timings are in `docs/testing/evidence/web/console-budgets.md`.
- Not a reference host and not amd64.
