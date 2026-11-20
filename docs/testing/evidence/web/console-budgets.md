# Console performance: measurements and proposed budgets (AUDIT-M9 F-09, NFR-PERF-02/03)

**Status: measured in the lab, budgets PROPOSED and NOT FROZEN.** Freezing budgets is an owner decision (OD-06). Nothing here is a field or capacity claim: no real network, no real users, no reference host.

## What was measured

Spec: `apps/web/e2e/perf-console.spec.ts` (opt-in, `CLUSTERX_PERF=1`). Raw samples and run metadata (hardware, browser version, dataset) are in `console-perf-<browser>.json` next to this file. Command:

```
pnpm --filter web build
CLUSTERX_PERF=1 CLUSTERX_E2E_RATE_MUTATIONS=0 CLUSTERX_E2E_PG_DIR=<dir> \
  pnpm --filter web exec playwright test e2e/perf-console.spec.ts --project=<chromium|firefox|webkit>
```

Environment (the same for every number): Apple M4, 10 cores, 16 GiB, macOS (Darwin 27.0, arm64), Node 26.13.1. The production Next build served by `node server.ts`, the real Go control plane and two real workers, PostgreSQL 18 with `fsync=off`, everything on loopback on one machine. Headless browser, **no CPU or network throttling**. The mutation rate limit was disabled for seeding (`CLUSTERX_E2E_RATE_MUTATIONS=0`). Data: 3,000 jobs (about 10% scheduled, 54% succeeded `hash_sha256`, 20% succeeded `sleep_ms`, 10% retried then succeeded, 10% permanently failed) and 100 seeded worker rows (70 live, 15 stale, 10 draining, 5 deregistered) inserted by SQL on the throwaway cluster.

Timing is taken inside the page with `performance.now()` and `requestAnimationFrame`, so it includes paint at about 16 ms granularity. Percentiles are nearest-rank over the sample count shown (small samples: p95 of 15 is its second-largest value).

## Results

All values in milliseconds. The headline numbers are Chromium's; Firefox and WebKit ran the same spec on the same machine for comparison.

| Measurement | n | Chromium 159 (p50 / p95 / max) | Firefox 159 | WebKit 27.2 |
|---|---|---|---|---|
| Jobs list API response through the BFF (browser `fetch`, 10 filter and sort combinations, 50 rows, 3,000 jobs), ms | 200 | 2.9 / 3.9 / 12.4 | 4 / 5 / 10 | 4 / 5 / 9 |
| Same query direct to the Go API (no web server), ms | 200 | 1.4 / 2.2 / 2.6 | 1.6 / 2.6 / 4.7 | 1.5 / 2.6 / 5.1 |
| Filter or Sort changed in the page until the new table is painted, ms (includes Playwright `selectOption` overhead) | 40 | 25.3 / 41.4 / 43.1 | 48 / 86 / 95 | 35 / 43 / 45 |
| Route transition, client-side link click until the new page's `h1` shows, ms (6 console routes) | 60 | 61.9 / 66 / 75.4 | 36 / 45 / 46 | 46 / 69 / 71 |
| Route transition until the data has loaded and no skeleton is left, ms | 60 | 62.1 / 80.8 / 81.5 | 49 / 62 / 166 | 59 / 71 / 85 |
| Overview with 100 workers: topology panel and worker list painted, ms | 15 | 63.3 / 76.3 / 76.3 | 59 / 75 / 75 | 64 / 89 / 89 |
| Click a worker in the topology list until its detail page is ready, ms | 15 | 62.2 / 72.4 / 72.4 | 47 / 89 / 89 | 46 / 86 / 86 |
| Workers page with 100 rows painted, ms | 15 | 68.2 / 69.6 / 69.6 | 64 / 89 / 89 | 57 / 64 / 64 |
| Live: new job committed until the jobs list says "Jobs changed.", ms | 15 | 723.6 / 750.5 / 750.5 | 687 / 704 / 704 | 694 / 755 / 755 |
| Live: job finishes on the server until its detail page shows Succeeded, ms (server time is a 10 ms API poll) | 12 | 1162.4 / 1916.3 / 1916.3 | 1301 / 1973 / 1973 | 1131 / 1747 / 1747 |

Route transitions, data-ready p95 per route (ms, n=10 each): `/app/*` rows below, Chromium / Firefox / WebKit.

| Route | Chromium | Firefox | WebKit |
|---|---|---|---|
| `/app` | 63.1 | 62 | 85 |
| `/app/jobs` | 75.4 | 166 | 71 |
| `/app/workers` | 57.5 | 54 | 71 |
| `/app/observability` | 81.5 | 61 | 78 |
| `/app/activity` | 63.5 | 51 | 62 |
| `/app/settings` | 62.9 | 54 | 61 |

The Workers page with 100 rows is 100 table rows and 1400 DOM elements in total. Stack, hardware and dataset are recorded inside each JSON file.

## What the numbers do and do not say

- Everything is on loopback on a fast laptop with PostgreSQL `fsync=off`, an idle machine and warm caches. The interactive numbers (tens of milliseconds) are dominated by the browser and local loopback; real networks add round trips on top of each request (one for a filter or sort, one RSC fetch plus the data fetches for a route). **They are not a statement about user experience on a phone and 4G, and a reference host was not measured.**
- Live-update staleness is the only measurement that is not tiny. The notice that the jobs list is out of date arrives about 0.7 s after the commit (p50, all three browsers) and a job detail page shows a finished job after about 1.1 to 1.3 s (p50) and up to about 2 s (max). That includes the stream path and, on detail pages, the one-second coalescing window in `useResource` (`COALESCE_MS = 1000`). It was not broken down further. Polling fallback staleness (stream down) was **not measured**.
- The workers list view asks for `limit: 100` and renders no pagination control (source read, `features/workers.tsx`). With more than 100 workers the table shows the first 100 only. That is a functional limit to decide on, not a timing result.
- Not measured: console JavaScript bundle size, real-user INP, memory growth over a long session, the topology SVG with more than 100 workers (the panel shows at most 20 workers in its list), a throttled CPU or network, jobs lists beyond 3,000 rows, many simultaneous viewers.
- Sample sizes are small (15 and 12 for the interaction and live measurements): p95 there is the second-largest value, not a stable estimate.

## Proposed console budgets (NOT FROZEN, owner decision OD-06)

The proposals leave about one to two orders of magnitude of headroom over the lab numbers for real networks and slower machines, and they are meant to be re-measured on the reference host before anyone freezes them. A budget is only useful if a test can fail on it, so each names how it would be checked.

| Budget | Proposed target | Lab measurement | How to check it |
|---|---|---|---|
| Jobs list response (filter or sort), server plus BFF | p95 under 250 ms at 3,000 jobs | p95 3.9 to 5 ms | `perf-console.spec.ts` on the reference host, jobs table at 50,000 rows |
| Filter or sort to a painted table | p95 under 400 ms | p95 41.4 to 86 ms | same spec |
| Console route transition to data ready | p95 under 600 ms | p95 80.8 to 71 ms | same spec |
| Overview and workers page with 100 workers | p95 under 800 ms; a pagination decision above 100 workers | p95 69.6 to 89 ms (table), 89 ms (topology, worst) | same spec |
| "Jobs changed" notice after a commit | p95 under 3 s | p95 750.5 to 755 ms | same spec |
| Job detail shows a terminal state | p95 under 5 s | p95 1747 to 1973 ms | same spec |
| Public pages (Lighthouse lab, mobile) | performance 90 or more, LCP under 2.5 s | 100, LCP 0.8 to 1.1 s | `docs/design/evidence/lighthouse-m9.md` |
| Console JS bundle | **no proposal: not measured** | not measured | needs a measurement first |

