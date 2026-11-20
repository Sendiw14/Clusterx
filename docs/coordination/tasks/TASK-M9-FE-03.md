# TASK-M9-FE-03 — Re-audit fixes (web copy) + bundle size

- Owner: frontend lane `claude-sonnet-5-5`, effort high. Branch `m9fix3/frontend` from main. Allowed: `apps/web/**`, `docs/testing/evidence/web/**`, handoff.
- Input: `docs/coordination/reviews/AUDIT-M9-RV-03.md` (RV10-01, RV10-02, RV10-06, F-09 bundle size).

| Item | Required outcome |
|---|---|
| RV10-01 | Quickstart, deployment page, `commands.ts`: clean-clone reproduction is verified on **Linux arm64 (Ubuntu VM)** by the independent audit; on macOS it works only with the checkout under `$HOME` (an outside-`$HOME` attempt failed). Don't claim a macOS clean-clone audit pass. |
| RV10-02 | `/docs/api` "executed / not served" labels derive from the routes actually registered in `apps/control-plane/internal/api/v1.go` (or a list kept in sync by a test that parses it), not the stale M4 curl-guide list; cancel/retry/drain/tokens/etc. shown as served. Fix `/docs/concepts` (cancel/retry served) and `/docs/workers` (handler log capture via `Job.Log` shipped in M5, bounded). Fix the two tests that assert the wrong label. |
| RV10-06 | Resolve the reliability-page wording conflict named in the audit. |
| F-09 | Measure client JS bundle size per route group (public, docs, demo, console) from `next build` output; save raw data under `docs/testing/evidence/web/`; propose a budget (OD-06, not frozen). |

Acceptance: web lint/typecheck/test/build exit 0; E2E docs/site/static-public/routes specs in all three browsers green; handoff `HANDOFF-M9-FE-03.md` with a claim→evidence table.
