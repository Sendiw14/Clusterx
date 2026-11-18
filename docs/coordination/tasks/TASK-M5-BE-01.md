# TASK-M5-BE-01 — Reliability & security: mutations, sessions/BFF assertions, stream, audit, metrics, outage tests

- Milestone: M5 (Beta). Requirement IDs: FR-ENG-12, 13, 14, 15, 16, 19, 22, 23; FR-API-01 (remaining ops), FR-SEC-01..07, 11; FR-RT-01 (server side); AC-03, AC-04, AC-06, AC-08, AC-09, AC-11, AC-12, AC-13, AC-16 (server), AC-17 (retention part).
- Owner: backend lane `claude-opus-5-5`, effort medium (requested). Branch `m5/backend`, worktree `.worktrees/backend`. Base: main `7e399d0` (contract `1.0.0-draft.5`).
- Includes **CR-M5-01** (below). Temporary `api/openapi.yaml` ownership for CR items only → `1.0.0-draft.6`.
- Allowed: as TASK-M4-BE-01 + `infra/compose/**` only for adding env vars your new config needs (list them in the handoff).
- Reviewer: independent Opus-high critical review (TASK-M5-RV-01) runs on your final SHA; P0/P1 block M5 exit.

## CR-M5-01 (from HANDOFF-M4-BE-01 open issues)

1. gRPC reason `invalid_argument` added to state-machine.md reason list.
2. New reason `credential_mismatch` (valid credential naming another credential's worker) instead of `credential_revoked`.
3. Recovery that exhausts the budget records event `attempts_exhausted` with `failure_class` per ADR (keep `lease_expired`/`deadline_exceeded` as attempt outcome). Document.
4. Reference worker persists `worker_id` (file path config, RO-fs friendly via mounted volume or env) so fast restarts re-register without waiting for liveness timeout; or document and test the alternative.
5. SDK captures bounded handler logs into `log_tail` (size cap per ADR-0004, secrets never logged).

## Deliverables

1. REST mutations: `POST /jobs/{id}/cancel` (pending → canceled; running → cancel_requested + worker notified; terminal → defined no-op/409), `POST /jobs/{id}/retry` (terminal failed → new linked job, idempotent action key, audited), `POST /workers/{id}/drain` (grace, then normal recovery). Tokens + worker-credential REST (list/create/revoke, one-time secret, owner-authority rule RV01-02).
2. Sessions per ADR-0006/CCR-10: BFF assertion verification (Ed25519 or HMAC per ADR — key from config/file, audience, TTL, `jti` replay cache, `sub`↔session binding RV01-11), `POST /sessions`, `DELETE /sessions/current`, `GET /me`, idle (`last_activity_at`) + absolute expiry enforced in Go. Provide a **test-only** assertion minting helper and a dev CLI `clusterx-control-plane dev-assertion` usable by the web lane locally (refuses in production).
3. `GET /api/v1/stream` WebSocket: auth on upgrade (session via BFF assertion, or API token), origin check, hints `{type, entity, id, version}`, `hello`/`resync`/`auth_expiring`, revalidation ≤30 s (sessions and tokens, RV01-13), close codes per contract; bounded fan-out, slow consumer drop → `resync`.
4. `GET /audit` with filters + viewer masking; audit rows for every privileged action (same transaction where ADR requires, e.g. `session.create`).
5. `GET /metrics/overview`, `GET /metrics/timeseries` derived from DB events/aggregates with explicit freshness; bounded windows/buckets.
6. Rate limiting returning 429 + `Retry-After` on mutation and auth endpoints (in-memory token bucket, single coordinator — document).
7. Retention job (`clusterx_retention` role, `clusterx_expire_attempt_logs`, tombstones) runnable as `clusterx-control-plane retention run` + scheduled loop option.
8. Tests: AC-03 (stale success after expiry+reclaim, service level), AC-04 with **real process `kill -9`** of a worker binary, AC-06 via REST cancel, AC-08 (DB outage: stop PG mid-run → readiness 503, admission 503 not 2xx, claims pause, resume after restart with no corruption), AC-09 full, AC-11 REST drain + SIGTERM, AC-13 matrix (operator/viewer/token/session/CSRF-n/a-in-Go/WebSocket after logout), AC-16 server side (hint ordering/gap → resync), retention test, rate-limit test. Mutant evidence for new security predicates.

## Acceptance

All modules vet + `-race` green on PG18; compose stack still boots (`infra/compose/bootstrap.sh` exit 0 on clean volume); redocly lint 0 errors on draft.6; handoff with commands/exit codes, AC table, mutant table, executed curl additions.
