# axe-core results, M6 (live console on real data) and the public site, docs and demo

Run by `pnpm --filter web e2e` (Playwright Chromium, `@axe-core/playwright`, tags wcag2a, wcag2aa, wcag21a, wcag21aa, wcag22aa).
Automated checks cover only part of WCAG: no conformance is claimed (manual screen-reader and keyboard walkthroughs are NOT RUN).

Scans: 116 (public routes and demo as in M7; every console route on live data as operator at 1440 and 320 and as viewer at 1440 and 390, job detail succeeded and failed, worker detail, the forbidden state, and the retry, create-token, one-time-reveal and jump-to dialogs open). Totals: critical 0, serious 0, moderate 0, minor 0.

| Scan | Width | Critical | Serious | Moderate | Minor | Rules (nodes) |
|---|---|---|---|---|---|---|
| `/` | 320 | 0 | 0 | 0 | 0 | none |
| `/` | 390 | 0 | 0 | 0 | 0 | none |
| `/` | 1024 | 0 | 0 | 0 | 0 | none |
| `/` | 1440 | 0 | 0 | 0 | 0 | none |
| `/ [menu open]` | 320 | 0 | 0 | 0 | 0 | none |
| `/app (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/activity (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/activity (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/activity (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app/activity (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/jobs (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/jobs (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/jobs (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app/jobs (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/[id] (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/[id] (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/[id] (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/[id] (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/[id] (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/[id] (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/[id] (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/[id] (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/new (forbidden)` | 1280 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/new (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/jobs/new (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/observability (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/observability (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/observability (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app/observability (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/settings (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/settings (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/settings (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app/settings (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/settings/preferences (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/settings/preferences (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/settings/preferences (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app/settings/preferences (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/settings/tokens (forbidden)` | 1280 | 0 | 0 | 0 | 0 | none |
| `/app/settings/tokens (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/settings/tokens (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/workers (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/workers (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/workers (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app/workers (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/workers/[id] (live operator)` | 320 | 0 | 0 | 0 | 0 | none |
| `/app/workers/[id] (live operator)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/app/workers/[id] (live viewer)` | 390 | 0 | 0 | 0 | 0 | none |
| `/app/workers/[id] (live viewer)` | 1440 | 0 | 0 | 0 | 0 | none |
| `/architecture` | 320 | 0 | 0 | 0 | 0 | none |
| `/architecture` | 390 | 0 | 0 | 0 | 0 | none |
| `/architecture` | 1024 | 0 | 0 | 0 | 0 | none |
| `/architecture` | 1440 | 0 | 0 | 0 | 0 | none |
| `/changelog` | 320 | 0 | 0 | 0 | 0 | none |
| `/changelog` | 390 | 0 | 0 | 0 | 0 | none |
| `/changelog` | 1024 | 0 | 0 | 0 | 0 | none |
| `/changelog` | 1440 | 0 | 0 | 0 | 0 | none |
| `/demo` | 320 | 0 | 0 | 0 | 0 | none |
| `/demo` | 390 | 0 | 0 | 0 | 0 | none |
| `/demo` | 1024 | 0 | 0 | 0 | 0 | none |
| `/demo` | 1440 | 0 | 0 | 0 | 0 | none |
| `/demo [operator/Config]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [operator/job detail]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [operator/Jobs]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [operator/Overview]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [operator/Recovery replay]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [operator/Workers]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [viewer/Config]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [viewer/job detail]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [viewer/Jobs]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [viewer/Overview]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [viewer/Recovery replay]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/demo [viewer/Workers]` | 1280 | 0 | 0 | 0 | 0 | none |
| `/docs` | 320 | 0 | 0 | 0 | 0 | none |
| `/docs` | 390 | 0 | 0 | 0 | 0 | none |
| `/docs` | 1024 | 0 | 0 | 0 | 0 | none |
| `/docs` | 1440 | 0 | 0 | 0 | 0 | none |
| `/docs/api` | 320 | 0 | 0 | 0 | 0 | none |
| `/docs/api` | 390 | 0 | 0 | 0 | 0 | none |
| `/docs/api` | 1024 | 0 | 0 | 0 | 0 | none |
| `/docs/api` | 1440 | 0 | 0 | 0 | 0 | none |
| `/docs/concepts` | 320 | 0 | 0 | 0 | 0 | none |
| `/docs/concepts` | 390 | 0 | 0 | 0 | 0 | none |
| `/docs/concepts` | 1024 | 0 | 0 | 0 | 0 | none |
| `/docs/concepts` | 1440 | 0 | 0 | 0 | 0 | none |
| `/docs/concepts [search open]` | 1440 | 0 | 0 | 0 | 0 | none |
| `/docs/deployment` | 320 | 0 | 0 | 0 | 0 | none |
| `/docs/deployment` | 390 | 0 | 0 | 0 | 0 | none |
| `/docs/deployment` | 1024 | 0 | 0 | 0 | 0 | none |
| `/docs/deployment` | 1440 | 0 | 0 | 0 | 0 | none |
| `/docs/quickstart` | 320 | 0 | 0 | 0 | 0 | none |
| `/docs/quickstart` | 390 | 0 | 0 | 0 | 0 | none |
| `/docs/quickstart` | 1024 | 0 | 0 | 0 | 0 | none |
| `/docs/quickstart` | 1440 | 0 | 0 | 0 | 0 | none |
| `/docs/reliability` | 320 | 0 | 0 | 0 | 0 | none |
| `/docs/reliability` | 390 | 0 | 0 | 0 | 0 | none |
| `/docs/reliability` | 1024 | 0 | 0 | 0 | 0 | none |
| `/docs/reliability` | 1440 | 0 | 0 | 0 | 0 | none |
| `/docs/security` | 320 | 0 | 0 | 0 | 0 | none |
| `/docs/security` | 390 | 0 | 0 | 0 | 0 | none |
| `/docs/security` | 1024 | 0 | 0 | 0 | 0 | none |
| `/docs/security` | 1440 | 0 | 0 | 0 | 0 | none |
| `/docs/workers` | 320 | 0 | 0 | 0 | 0 | none |
| `/docs/workers` | 390 | 0 | 0 | 0 | 0 | none |
| `/docs/workers` | 1024 | 0 | 0 | 0 | 0 | none |
| `/docs/workers` | 1440 | 0 | 0 | 0 | 0 | none |
| `/login` | 320 | 0 | 0 | 0 | 0 | none |
| `/login` | 390 | 0 | 0 | 0 | 0 | none |
| `/login` | 1024 | 0 | 0 | 0 | 0 | none |
| `/login` | 1440 | 0 | 0 | 0 | 0 | none |
| `create token dialog` | 1280 | 0 | 0 | 0 | 0 | none |
| `jump-to dialog` | 1280 | 0 | 0 | 0 | 0 | none |
| `retry confirm dialog` | 1280 | 0 | 0 | 0 | 0 | none |
| `token reveal dialog` | 1280 | 0 | 0 | 0 | 0 | none |
