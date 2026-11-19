# TASK-M6-FE-01 — Operator console on live data: BFF sessions, all /app routes, stream + reconciliation

- Milestone: M6 console. Requirement IDs: FR-WEB-02, FR-WEB-03, FR-WEB-06 (live topology), FR-WEB-08, FR-WEB-09, FR-RT-01 (client), FR-SEC-01/02/06/07 (web side); AC-13 (web), AC-15, AC-16, AC-21, AC-22, AC-23.
- Owner: frontend lane `claude-sonnet-5-5`, effort high (requested). Opus owns the auth contract (ADR-0006) and reviews BFF security. Branch `m6/frontend`, worktree `.worktrees/frontend`. Base: main after M7 merge. Contract: `contract-m5-draft.6` (generated `@clusterx/sdk-ts` — import only).
- Allowed: `apps/web/**`, `packages/ui/**`, `docs/design/**`, `docs/coordination/handoffs/HANDOFF-M6-FE-01.md`; lockfile changes from `pnpm install` for web/ui only. Read-only: Go code, `api/**`, `infra/**`, `docs/architecture/**`, `docs/security/**`.

## BFF and login (ADR-0006, HANDOFF-M5-BE-01 "Web lane" section)

1. GitHub OAuth authorization-code + PKCE + state using a maintained library (e.g. `arctic` or `openid-client` — record choice/version). Endpoints (authorize/token/user) configurable so tests point at a **local mock OAuth server** run by Playwright; no bypass/dev login route in the app. Production config fails closed without client id/secret.
2. After callback: sign Ed25519 BFF assertion (`purpose=login`, closed claim set, ≤60 s, fresh `jti`), `POST /api/v1/sessions`, store `cxs_` session secret only server-side in an encrypted HttpOnly, Secure (except localhost), SameSite=Lax cookie (or server-side store — justify); session regeneration; logout → `DELETE /sessions/current` + cookie clear.
3. Every proxied call: fresh request assertion (`sub`, `sid`, `role`, `rid`), BFF passes Go status/headers/error envelope through unchanged (BFF-02); BFF-owned errors `upstream_auth` (502), `csrf_failed` (403). CSRF: double-submit or Origin check on all mutating `/bff/**`. 401 reason mapping per HANDOFF-M1-FE-02. Private key from file path env, never in client bundle (add a test that greps the client build output for key material/`cxs_`).
4. WebSocket: BFF-side relay or server-side subscription per ADR-0006 (hints only), client reconnect with backoff, snapshot reconciliation on hello/resync/gap, polling fallback, stale/disconnected indicators (AC-16).

## Console routes (all 11 from route-inventory.md, real data)

`/app` (KPIs from `/metrics/overview`, live indicator, recent jobs, throughput chart with data table, accessible topology + list), `/app/jobs` (server filters/sort/cursor, dead-letter filter, unschedulable reason), `/app/jobs/new` (handler catalogue + per-version JSON Schema validation, scheduling, idempotency key, 201/200 replay/409/422/413), `/app/jobs/[id]` (attempts, lease, events timeline, log_tail, cancel/retry with confirmation, lineage, 410 expired, viewer masking), `/app/workers` + `/[id]` (liveness, drain with grace, revoked/disconnected), `/app/observability` (timeseries, freshness notice, data tables), `/app/activity` (`/audit` filters, masking), `/app/settings`, `/app/settings/tokens` (tokens + worker credentials: create with one-time reveal, revoke; rotate = create+revoke), `/app/settings/preferences`. Error/edge states S-* per states-matrix. No fake controls.

## Tests

Playwright **live E2E against the real stack**: Go control plane + PG18 (+ ≥1 real worker binary) started by the test harness (reuse `apps/control-plane` binaries via `go build` into a temp dir, own PG cluster or the compose stack on alternate ports — never the integrator's running `clusterx` project), mock OAuth server, BFF keys generated with `clusterx-control-plane dev-keygen`, users via `user upsert`. Cover: login operator/viewer/not-allowlisted, submit → worker runs → succeeded in UI, cancel running, manual retry lineage, drain, token create/reveal-once/revoke, viewer denied + masked, session expiry/logout → stream closed, stream drop → reconcile (AC-16), CSRF rejection, 429 handling. Plus axe on every console route (live data), 320/1440 overflow, client-bundle secret scan. Replace the `/` console preview with a real capture (labelled with capture date/commit).

## Acceptance

`pnpm --filter web lint typecheck test build e2e` exit 0 including live E2E; handoff lists exact harness commands, versions, axe results, screenshots under `docs/design/screenshots/m6/`, NOT RUN items. Opus security review of BFF follows (TASK-M6-RV-01).
