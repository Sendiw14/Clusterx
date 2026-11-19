# TASK-M9-FE-01 — Web release hardening + stale copy

- Owner: frontend lane `claude-sonnet-5-5`, effort high. Branch `m9/frontend` from main (`.worktrees/frontend`). Allowed: `apps/web/**`, `packages/ui/**`, `docs/design/**`, handoff. Contract unchanged (draft.7).
- Inputs: `docs/coordination/reviews/REVIEW-M6-RV-01.md` Round 2 (RV6B-01..05), ADR-0006 §2/§3 (amended on main).

| Item | Required outcome |
|---|---|
| RV6B-01 | Public and console sections use separate root layouts (or hard navigations) so `/app`, `/login`, `/auth/*` always load with the nonce CSP; live spec: navigate from a public page to `/app` and `/login` via UI links, assert response CSP has no `'unsafe-inline'` and an injected inline handler does not run. |
| RV6B-02 | Per ADR-0006 §2 decision: after a Go 403 and on `/bff/session`, call `GET /me` and re-seal the cookie with the DB role. Live spec: demote user mid-session → next requests carry viewer hint, audit `assertion.role_mismatch` rows stop after the refresh (≤1). |
| RV6B-03 | CSP (nonce policy) on every response under `/app*`, `/login*`, `/auth*` in any letter case and on their 404s; or normalise case with redirect. Live spec. |
| RV6B-05 | Relay: half-closed client releases its slot promptly (shorter close timeout / detect FIN); per-user cap across sessions. Spec or documented bound. |
| Stale copy | `/architecture` (diagram tags), home layer cards, public layout comment and any other "specified, not built" text now contradicted by main; changelog lists M5–M8 merged milestones only as they land (M8 pending — leave it out until integrator says merged). |
| RV6B-04 | Note-only for ops runbook (no change). |

Acceptance: `pnpm --filter web lint typecheck test build e2e` exit 0 (live E2E via `CLUSTERX_E2E_PG_DIR`), axe 0 serious/critical; handoff `HANDOFF-M9-FE-01.md`.
