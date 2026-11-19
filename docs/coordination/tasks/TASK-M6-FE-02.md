# TASK-M6-FE-02 — REVIEW-M6-RV-01 fixes + docs truth update

- Owner: frontend lane `claude-sonnet-5-5`, effort high. Branch `m6fix/frontend` from main (`.worktrees/frontend`). Allowed: `apps/web/**`, `packages/ui/**`, `docs/design/**`, handoff. Contract unchanged (draft.7).
- Re-review: Opus reviewer re-checks RV6-01/02/03/09 (+ P3s) before the M8 security gate.

| Item | Required outcome |
|---|---|
| RV6-01 (P2) | Proxy: positive allowlist of the OpenAPI path templates the console uses (else 404 `not_found_route`); reject raw `%2F`/`%5C`/`%2E` (any case) and decoded `/ \ ? #` in segments; re-encode segments; assert final pathname starts with `/api/v1/` and equals the built path. Live spec with raw `node:http` sending the 4 encoded probes from the review → 404. |
| RV6-02 (P2) | Reject `Content-Length` > 1 MiB before reading; otherwise stream-count bytes and abort at 1 MiB (chunked). Unit test (2 MiB streamed → ≤ ~1 MiB read) + live spec. |
| RV6-03 (P2) | Stream relay: check browser socket before dial; close upstream when browser closes; always keep an `error` listener on upstream; or complete browser handshake first then dial. Live spec: 6 aborted/malformed upgrades then a working socket. |
| RV6-04 | Single clock read for iat/nbf/exp; fake-clock unit test asserting `exp - iat === 60`. |
| RV6-05 | Production requires `https:` API URL unless loopback; unit test. |
| RV6-06 | Callback revokes a still-valid previous session best-effort; `/auth/end` revokes best-effort or never shows `signed_out` without revocation. Live spec. |
| RV6-07 | Drop `sec` (`cxs_`) from the sealed cookie; add 1-byte key-version prefix with old-key acceptance window. (Integrator amends ADR-0006 §3.) |
| RV6-08 | Reserve relay slot synchronously before dial; per-process ceiling. Live spec (8 concurrent → 5 open, rest BFF 429). |
| RV6-09 (release) | Nonce-based CSP (`'nonce-…' 'strict-dynamic'`, no `'unsafe-inline'` in `script-src`) via `src/proxy.ts` for `/app/*`, `/login`, `/auth/*` at minimum; keep public pages working (static policy allowed there only if no inline scripts need it — prefer nonce everywhere if feasible without breaking static rendering). Live spec asserting no `'unsafe-inline'` on console responses. |
| RV6-10 | Keep start-up guard; note in handoff. |
| Docs truth | `/docs/security`, `/docs/workers`, `/docs/quickstart`, `/docs/deployment`, `/changelog`: sessions/BFF now built (M6), drain is **not** exclusion (revoke credential to keep a host out; a drained worker restarted under a supervisor returns as a new row), compose `.secrets/` dir + dev BFF key + `retention` profile + optional `.env` keys (`CLUSTERX_HTTP_PORT`, `CLUSTERX_METRICS_PORT`, `CLUSTERX_IMAGE_PREFIX`). Console worker-drain dialog states the same. Only state what main's code/handoffs show. |

Acceptance: `pnpm --filter web lint typecheck test build e2e` exit 0 (live E2E with `CLUSTERX_E2E_PG_DIR`), axe still 0 serious/critical; handoff `HANDOFF-M6-FE-02.md`.
