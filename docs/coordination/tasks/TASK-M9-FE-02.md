# TASK-M9-FE-02 — Audit fixes (web): static public pages, docs truth, browsers, performance

- Owner: frontend lane `claude-sonnet-5-5`, effort high. Branch `m9fix/frontend` from main. Allowed: `apps/web/**`, `packages/ui/**`, `docs/design/**`, `docs/testing/evidence/web/**`, handoff.
- Input: `docs/coordination/reviews/AUDIT-M9-RV-01.md` (F-05, F-06, F-08, F-09, F-20), `docs/testing/release-gates.md`.

| Item | Required outcome |
|---|---|
| F-05 | Public routes prerendered again (no `connection()` in root not-found; keep nonce CSP on console/login/auth and a hydrating 404). Prove with `next build` route table (○/● for public). Re-measure Lighthouse (lab) for `/`, `/docs`, `/demo`, `/architecture` mobile+desktop; update `lighthouse-*.md`. Fix the `next.config.ts` comment. |
| F-06 | `content/changelog.md` and `content/docs/deployment.md` (and any other web docs) match main: M8 merged (alerts, backup/restore drill, production TLS example — all local-only, reference host NOT RUN), CI defined but never run. Only state what `docs/operations/*`, `docs/testing/*` and HANDOFF-M8 show. |
| F-08 | Run the E2E suite (public + live console) in Firefox and WebKit projects too; fix real failures; record results. |
| F-09 | Measure console budgets (p95 jobs-table filter/sort response, route transitions, topology interaction with 100 workers, live-update staleness) with documented hardware; save raw data; propose (don't freeze) numbers — OD-06. |
| F-20 | `/docs/quickstart` explains console-via-E2E-harness path (no GitHub needed) honestly. |

Acceptance: web lint/typecheck/test/build/e2e (all three browsers) exit 0; handoff `HANDOFF-M9-FE-02.md`.
