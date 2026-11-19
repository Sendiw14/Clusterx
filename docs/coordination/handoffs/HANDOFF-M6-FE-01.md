# HANDOFF — TASK-M6-FE-01 (operator console on live data: BFF sessions, all /app routes, stream, reconciliation)

- **Task:** `docs/coordination/tasks/TASK-M6-FE-01.md`. Requirements: FR-WEB-02/03/06/08/09, FR-RT-01 (client), FR-SEC-01/02/06/07 (web side); AC-13 (web), AC-15, AC-16, AC-21, AC-22, AC-23.
- **Author:** frontend lane, model `claude-sonnet-5-5`; effort `high` requested, effective effort not observable (UNVERIFIED, MW-04).
- **Branch / base:** `m6/frontend`, worktree `.worktrees/frontend`, base main `757062c`. No push, no merge.
- **Contract:** `api/openapi.yaml` `1.0.0-draft.6`, tag `contract-m5-draft.6`, SHA-256 `280b8238319b51eaceee58224598a413b31f40e9928fec4dc595ef7c4caaca6e`. `pnpm sdk:check` clean. `git diff 757062c..HEAD` touches nothing under `api/`, `packages/sdk-ts`, `apps/control-plane`, `apps/worker`, `infra`, `docs/{architecture,security,product}`.
- **Status: implemented AND integrated against the live stack** (real Go control plane, real worker binary, own PostgreSQL 18.7), with a **mock OAuth server** standing in for GitHub. Not tested against real GitHub, TLS, or the integrator's compose stack.
- **Integrator note received mid-task:** TASK-M5-BE-02 (`m5fix/backend`: auth-failure bucket RV5-05, drained workers staying drained RV5-02) is not in this build. I built against main as it is. Rebase onto the fixes before the final live run; nothing in the web code depends on either behaviour (the drain spec accepts 202 or 409 `worker_deregistered`; the harness sets rate limits high so RV5-05 cannot lock specs out).

## Commits

| SHA | Content |
|---|---|
| `98f6cdb` | BFF: config (fail closed), sealed cookies, GitHub OAuth+PKCE, EdDSA assertions, CSRF, passthrough proxy, logout, `/bff/session`, stream relay server, `StreamClient`, live runtime; unit tests |
| `46173c3` | Console routes on live data, mutations, tokens/credentials, observability, activity, realtime-driven refetch |
| `91e27a1` | Live E2E harness and console specs; fixes the specs found |
| `f5f3f10` | Id tail display (UUIDv7 prefixes collide), home capture provenance module, M6 screenshot spec |
| (this) | Real capture, screenshots, axe-m6 evidence, this handoff (a file cannot hold its own id; see return message) |

## What was built

**BFF (all under `apps/web/src/server`, plain Node ESM so the Next handlers and `server.ts` share it):**
`config.ts` (production is the default; needs https origin, client id/secret, 32-byte key, key file; OAuth endpoint overrides refused in production), `crypto.ts` (AES-256-GCM seal with purpose as AAD, HMAC CSRF token), `assertion.ts` (`jose` EdDSA; header `{alg, typ, kid}`; closed claim set; 60 s; fresh 128-bit jti; login assertion has `purpose=login`, no sid/role), `upstream.ts` (one retry on `assertion_expired`), `oauth.ts` (`openid-client`, S256, state), `cookies.ts`, `bff.ts` (gate, error envelope, 401 mapping), `stream-relay.ts`.
Routes: `GET /auth/github` (starts, sealed 10-min state cookie), `GET /auth/callback`, `GET /auth/end` (clears this browser's cookie, lands on `/login` with a closed-set reason), `GET /bff/session`, `POST /bff/logout`, `/bff/api/v1/*` (proxy; `sessions*` and `stream` are not reachable), `/bff/stream` (WebSocket via `server.ts`; plain GET answers 426).
**Session storage (justification):** stateless sealed cookie `__Host-clusterx_session` (plain `clusterx_session` on http localhost), HttpOnly, SameSite=Lax, Path=/, Max-Age = absolute expiry. Revocation lives in Go (logout, disable and expiry take effect on the next call; E2E replays a stolen cookie after logout and gets 401). The role shown always comes from Go `/me`, never the cookie. The cookie holds `{sid, uid, login, role hint, cxs_ secret, exp}` sealed; the `cxs_` secret is not needed by Go for later requests (it checks `sid`) and is kept only because ADR-0006 section 3 names it as the cookie value.
**Console:** `/app` (KPIs, throughput chart with Table toggle, accessible topology SVG plus list, recent jobs), `/app/jobs` (server filters, status, handler, priority, unschedulable reason, sort, dead-letter view, cursor stack, "Jobs changed. Refresh list" instead of reordering), `/app/jobs/new` (handler catalogue, per-version schema check, scheduling, idempotency key, 201/200 replay/409/422/413/429 outcomes, per-tab sessionStorage draft), `/app/jobs/[id]` (attempts, lease fields, events with data, log tail, cancel 200/202 and retry with confirmation, lineage both ways, 410, viewer masking), `/app/workers` and `/[id]` (liveness, revoked, drain with grace), `/app/observability` (all 8 metrics, 4 ranges, freshness notice, data tables), `/app/activity` (`/audit` filters, cursor, masking), `/app/settings`, `/app/settings/tokens` (tokens and worker credentials: create, blocking one-time reveal, revoke, rotate = create then offer revoke), `/app/settings/preferences` (adds session info). No dead controls; unavailable controls state why (offline).
**Realtime (AC-16):** hints only cause refetch (throttled to 1/s); `resync`, reconnect (`hello` after a drop) and polling ticks always refetch; reconnect with jittered exponential backoff (1 s to 30 s), polling every 15 s after 2 failed attempts; 4401 ends the stream and leaves for `/login`; 4403 falls back to polling; `auth_expiring` shows a banner; indicator states live, reconnecting, polling, stale (no successful call for 45 s), offline.
**Decisions worth a look:** (1) no `ajv`: it compiles with `new Function`, which the CSP forbids, so `src/lib/json-schema.ts` checks a documented subset and the server stays authoritative (my first Ajv version silently failed under CSP; the live spec caught it). (2) The id shown for jobs and workers is the **last** 8 characters: UUIDv7 prefixes are identical for jobs created together. (3) `server.ts` sets Next's internal `didWebSocketSetup` flag: Next otherwise installs its own `upgrade` listener that ends every unknown upgrade and killed the relay's sockets. It throws at start if the flag disappears (Next upgrade), and the stream specs fail loudly.

## Dependencies added (exact versions, `apps/web/package.json`)

`openid-client 6.9.0` (OAuth authorization code + PKCE), `jose 6.2.18` (EdDSA signing), `ws 8.23.0` (relay), dev `@types/ws 8.18.2`. Tooling used: Node 26.13.1, pnpm 9.15.9, Next 16.4.6, Playwright 1.65.1 (Chromium), `@axe-core/playwright 4.14.0`, Go 1.27.4, PostgreSQL 18.7. `ajv` was added then removed.

## Commands and results (final tree, before this handoff commit)

| Command | Exit | Result |
|---|---|---|
| `pnpm --filter web lint` | 0 | 0 problems |
| `pnpm --filter web typecheck` | 0 | |
| `pnpm --filter web test` | 0 | 11 files, 99 tests (new: BFF core 27, StreamClient 9, schema 3) |
| `pnpm --filter web build` | 0 | |
| `CLUSTERX_E2E_PG_DIR=<dir> pnpm --filter web e2e` | 0 | **149 passed, 54 skipped** (opt-in screenshot specs). Five consecutive full runs after the fixes: 149 passed each. **One earlier full run had one failure** (a `toHaveCount` assertion; the test name was lost when I re-ran) that did not reproduce in the four later runs: treat as a possible flake. |
| `CAPTURE_M6=1 pnpm exec playwright test e2e/screenshots-m6.spec.ts` | 0 | 4 passed; 35 PNGs |

**Harness (reproducible):** `pnpm --filter web build` first, then `CLUSTERX_E2E_PG_DIR=/some/dir pnpm --filter web e2e`. Global setup (`e2e/stack/stack.ts`): `go build` of `apps/control-plane/cmd/clusterx-control-plane` and `apps/worker/cmd/clusterx-worker` into `apps/web/.e2e/bin`; `initdb` (LC_ALL=en_US.UTF-8, `--auth=trust`) and `pg_ctl` on **55440**, TCP only (`-k ''`), roles as in `infra/compose/initdb`; `dev-keygen` for the BFF key; `migrate`; `user upsert` for operator/viewer/limited/expiring/victim and a disabled user; Go on **18180/19190/19191**; two workers (`e2e-worker-1`, `e2e-worker-drain`) with their own credentials; mock OAuth on **19300** (enforces S256, state, client secret, single-use codes; the identity comes from a per-browser-context cookie so parallel specs never share a user); web `node server.ts` on **3100**. Teardown stops workers, Go, the mock and PostgreSQL (also when setup throws). I verified no process of mine survives (the PostgreSQL on 55432 belongs to another lane and was not touched). Never used or stopped the integrator's compose project or port 8080.

## Live E2E coverage (what actually ran against the live stack)

Login operator, viewer, not-allowlisted (no cookie set), disabled, GitHub denial, GitHub 5xx, forged callback (state), `next` clamp, unauthenticated redirect; cookie flags; logout revokes in Go and a replayed cookie gets 401; expired session (session row expired in SQL) returns to `/login?reason=session_expired&next=` and resumes after sign-in; account disabled mid-session ends the open console through the stream (4401). Submit in the UI, worker runs it, UI shows Succeeded with the real SHA-256 without reload; API outcomes 201/200 replay/409/422/413 pass through unchanged; cancel running (202, then Canceled; backing out changes nothing); cancel finished (409 with state); retry lineage both ways, once; non-failed retry 409; list filters, dead-letter, sort; 404 and 410 (tombstone); drain (confirmation, validation, state, idempotent); token create, reveal once, use against Go, revoke (401 after), rotate; worker credential create and revoke; viewer masking (payload and result absent from DOM and API), no controls, 403 for writes, privileged-only activity. CSRF (missing/foreign Origin, missing/foreign token, cross-session token, logout, proxy denies sessions and stream). Client bundle and served HTML contain no key material, `cxs_`/`cxt_`/`cxw_`, OAuth secret or env names. Stream: Live indicator, hints, jobs list offers refresh, drop and reconnect reconciles to the snapshot, never-connecting stream falls back to polling, `auth_expiring` banner, logout closes the relayed socket with 4401, relay refuses no cookie (401), foreign or missing Origin (403), tampered cookie (401), sixth socket (429). Live 429 (a burst from a dedicated user hits Go's mutation limit; `Retry-After` and body pass through), simulated 429/500/503/502 states, offline, request id copy. **axe on every console route with live data** (operator 1440 and 320, viewer 1440 and 390), job detail succeeded and failed, worker detail, forbidden state and four dialogs: **0 violations of any impact** (`docs/design/evidence/axe-m6.md`, 116 scans including the public set). Overflow at 320 and 1440 on every console route. Reduced motion: no running animation.

Bugs the live specs found and I fixed: Next's upgrade listener killing relay sockets; refetch after reconnect lost because every status change re-subscribed (AC-16 reconcile); Ajv under CSP; 24 px target size on checkboxes; copy button accessible name; subscribing before the stream exists.

## Evidence

Screenshots `docs/design/screenshots/m6/` (35: every route at 1440 and 390, retry/create-token/reveal dialogs with the secret masked, viewer masked and forbidden, simulated 503 state (labelled simulated in its file name), no-entitlement login). The `/` console preview is replaced by a real capture `apps/web/public/images/console-jobs.png`, labelled on the page with date `2026-11-19` and commit `f5f3f10` (provenance in `src/content/console-capture.ts`; the tree differs from that commit only by the screenshot spec). Axe evidence `docs/design/evidence/axe-m6.md|json`; `axe-m7.*` untouched.

## NOT RUN / limitations

- Real GitHub, TLS (web to Go, browser to web), production-mode boot with `__Host-` Secure cookies live (config fail-closed is unit-tested; only the http-localhost cookie variant ran live).
- Idle and absolute session expiry by elapsed time (simulated by editing `sessions.expires_at`); `auth_expiring` came from an injected frame, not Go.
- Network-level duplicate or reordered stream frames (client logic unit-tested, not injected live); `resync{buffer_overflow|gap_detected}` from Go not forced live.
- Live 429 on the login bucket; RV5-05 and RV5-02 behaviours (waiting for `m5fix/backend`).
- Manual screen-reader and keyboard walkthrough, Firefox and WebKit, Lighthouse and Web Vitals for the console, INP.
- `/app` throughput chart with a populated series in a screenshot (the capture's last-hour data is only a few buckets).
- Opt-in M3 screenshot spec `e2e/screenshots.spec.ts` targets the old unauthenticated console and is obsolete (skipped by default; not updated).
- `pnpm dev` (Turbopack) has no `/bff/stream` relay: the console falls back to polling. `pnpm dev:live` runs `server.ts` in development.
- Key rotation needs a web restart (the signing key is cached after first read).
- Login does not revoke a previous session of the same browser (fixation is handled by always issuing a fresh cookie value; the old Go session expires normally).

## For the Opus security review (TASK-M6-RV-01)

1. `server/stream-relay.ts`: Origin and cookie only (no CSRF on GET upgrade, by design); per-session cap of 5 is in process memory; frames copied verbatim; the `didWebSocketSetup` takeover in `server.ts`.
2. Sealed cookie: single key derives encryption and CSRF keys (HMAC labels); no key rotation story; `cxs_` secret inside the sealed value is unused by Go and could be dropped; cookie `exp` equals Go's absolute expiry.
3. CSRF token is exposed in rendered HTML and `/bff/session` (an HMAC of `sid`, not a secret from its own user).
4. Proxy: forwards any `/api/v1/*` path except `sessions*` and `stream` (so `/me` is reachable), 1 MiB body cap, forwards only `content-type`, `accept`, `idempotency-key`; passes Go bodies unchanged; maps session-ended 401 to a BFF 401 and clears the cookie; `assertion_invalid` and repeated `assertion_expired` are 502 and never end the session.
5. OAuth callback: state compared in constant time; PKCE verifier and `next` live in a sealed 10-minute cookie cleared on every outcome; `next` clamped to `/app...` same origin; the redirect URL used for the code exchange is rebuilt on the configured origin; user id must be a positive safe integer.
6. Logging: request ids, status, reason codes and error class names only; no assertion, cookie, secret or GitHub token is logged (please verify).
7. CSP still has `script-src 'unsafe-inline'` (unchanged from M3; nonce policy deferred).
8. Mock OAuth and plain test credentials exist only in the E2E harness and `playwright.config.ts`; the private JWK is generated per run under `apps/web/.e2e/` (git-ignored).
