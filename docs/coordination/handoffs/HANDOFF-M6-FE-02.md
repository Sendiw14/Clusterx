# HANDOFF — TASK-M6-FE-02 (REVIEW-M6-RV-01 fixes + docs truth update)

- **Task:** `docs/coordination/tasks/TASK-M6-FE-02.md`. Source review: `docs/coordination/reviews/REVIEW-M6-RV-01.md` (PASS conditional; RV6-01/02/03 P2, RV6-09 P2 release, RV6-04..08 and RV6-10 P3).
- **Author:** frontend lane, model `claude-sonnet-5-5`; effort `high` requested, effective effort not observable (UNVERIFIED).
- **Branch / base:** `m6fix/frontend`, worktree `.worktrees/frontend`, base main `128873b`. No push, no merge.
- **Contract:** `api/openapi.yaml` `1.0.0-draft.7`, tag `contract-m6-draft.7`, SHA-256 `d61d5042480d080418d6ec44725c8c0e88d9625f007b1cdc6fffea8d86270453`. Unchanged: `git diff 128873b..HEAD` touches nothing under `api/`, `packages/sdk-ts`, `apps/control-plane`, `apps/worker`, `infra`, `docs/{architecture,security,product}`.

## Commits (excluding this handoff commit, whose id a file cannot hold)

| SHA | Items |
|---|---|
| `90b8d8d` | RV6-01 (positive allowlist, encoded separators), RV6-02 (streaming body cap) |
| `c8cb901` | RV6-03 (upstream socket leak), RV6-08 (slot reserved before dial, per-process ceiling) |
| `063da12` | RV6-09 (nonce CSP for `/app/*`, `/login`, `/auth/*`) |
| `ee37ff2` | RV6-04, RV6-05, RV6-06, RV6-07 (+ a racy existing expiry spec made deterministic) |
| `1781e16` | Docs truth update, drain dialog text, stream spec pacing, site spec M1..M6 |

RV6-01 and RV6-02 share `route.ts` and are in one commit; each was green before the commit (unit tests, lint, typecheck, build, its live spec file plus `console-security.spec.ts`).

## Per-item status

| ID | Status | What changed | Before (reviewer's probe ported) | After |
|---|---|---|---|---|
| RV6-01 | fixed, pending re-review | `src/server/proxy-path.ts`: allowlist of the 19 OpenAPI routes the console uses, parsed from the raw pathname; reject `%2F %5C %2E` any case, decoded `/ \ ? #` and control chars; segments re-encoded; built path checked with the URL parser and the `/api/v1/` prefix. Everything else (sessions, stream, health, unknown) is 404 `not_found_route`. | `x%2F..%2F..%2F..%2Fhealth%2Fready` → **200** (Go's health) | all 11 encoded probes + the destructive `DELETE x%2F..%2Fsessions%2Fcurrent` → 404, session survives; allowlisted routes still 200 |
| RV6-02 | fixed, pending re-review | `src/server/body.ts` `readCapped`: Content-Length over 1 MiB (or malformed) refused before reading; otherwise counted while streaming and cancelled at 1 MiB. | 256 MiB POST: **+728 MiB RSS**; chunked: 256 MiB written before any answer | RSS growth < 96 MiB (asserted), chunked body answered 413 after < 64 MiB written; under-cap body still 201 |
| RV6-03 | fixed, pending re-review | `stream-relay.ts`: the browser socket owns the slot for its lifetime and kills the upstream socket if it closes (or `ws` aborts the handshake) before hand-off; permanent `error` listener on upstream; early socket errors swallowed (an ECONNRESET during the dial otherwise showed as an uncaught exception in the web log; fixed). | 5 aborted/malformed upgrades then a legit socket → **closed 4429** | 6 aborted + 6 malformed upgrades (400) then a legit socket opens with `hello`; BFF still serves HTTP; no uncaught exception in server output |
| RV6-04 | fixed, pending re-review | single clock read for iat/nbf/exp. | fake clock: `exp - iat` = **62** (unit test fails on the old code) | `=== 60`, `nbf === iat` |
| RV6-05 | fixed, pending re-review | production requires `https:` `CLUSTERX_API_URL` unless the host is loopback. | `prod … ok = true http://10.0.0.5:8080` (reviewer) | unit test: http non-loopback refused, https and loopback accepted, development unchanged |
| RV6-06 | fixed, pending re-review | callback revokes a still-valid previous session of the browser (after the new one is created); `/auth/end` revokes the session it clears (best effort, 3 s timeout, never throws). Shared `revokeSession` in `upstream.ts`. | old cookie after re-login → `/bff/session` **200**; replay after `/auth/end` → **200** | both **401** (live spec) |
| RV6-07 | fixed, pending re-review; **needs integrator ADR amendment** | `cxs_` secret dropped from the sealed cookie (test asserts the key set is exactly `sid uid login role exp`). 1-byte key version prefix (first byte of SHA-256 of the key); optional `CLUSTERX_WEB_SESSION_KEY_PREVIOUS` opens old cookies and verifies old CSRF tokens during rotation, never seals. Pages and `/bff/session` always issue the current key's token. | no rotation, secret stored | unit tests: rotation, unrelated key refused, CSRF window |
| RV6-08 | fixed, pending re-review | slot reserved synchronously before the dial; per-process ceiling `MAX_SOCKETS_TOTAL = 256`. | 8 concurrent: 4 open, 4 Go 4429 (none BFF 429) | 8 concurrent on a clean session: 5 open, 3 refused by the BFF with HTTP 429; unit test of `reserveSlot` |
| RV6-09 | fixed, pending re-review | `src/proxy.ts` issues a per-request nonce (`script-src 'self' 'nonce-…' 'strict-dynamic'`, no `'unsafe-inline'`) for `/app/:path*`, `/login`, `/auth/:path*` and sets it on the request so Next stamps its own scripts; the static policy in `next.config.ts` now excludes those paths (exactly one CSP header there). `src/lib/csp.ts` builds both. Public pages are static and keep the old policy (`'unsafe-inline'`), because a per-request nonce would force dynamic rendering of every page. The one inline script (`js` class marker) is gone: `packages/ui/src/styles.css` uses `@media (scripting: enabled)` instead. | `script-src 'self' 'unsafe-inline'` | live spec: no `'unsafe-inline'`, `'strict-dynamic'`, fresh nonce per response (4 distinct), every inline `<script>` on four console routes carries a nonce, zero CSP violations (console messages and `securitypolicyviolation`), public pages still render with the static policy |
| RV6-10 | kept | start-up guard for the `didWebSocketSetup` takeover in `server.ts` is unchanged. **Note for the M8 dependency-upgrade runbook:** any `next` upgrade must be checked against that flag; `next` stays pinned at 16.4.6. | | |

New regression specs (all raw `node:http`/`node:net`, helpers in `e2e/raw.ts`): `console-proxy-hardening.spec.ts` (RV6-01/02), `console-stream-hardening.spec.ts` (RV6-03/08), `console-session-hardening.spec.ts` (RV6-06), `console-csp.spec.ts` (RV6-09). Unit: `proxy-hardening.test.ts`, `stream-relay.test.ts`, additions to `bff-core.test.ts`. "Before" runs: `scratchpad/fe02-before.log` (10 of 12 new live tests failed on `128873b` for the reasons in the table; the 2 that passed are controls (under-cap body, public pages)).

## Commands and results (final tree, commit `1781e16`)

| Command | Exit | Result |
|---|---|---|
| `pnpm --filter web lint` | 0 | |
| `pnpm --filter web typecheck` | 0 | |
| `pnpm --filter web test` | 0 | 13 files, 116 tests (was 11 / 99) |
| `pnpm --filter web build` | 0 | |
| `CLUSTERX_E2E_PG_DIR=…/scratchpad/pgfe2 pnpm --filter web e2e` | 0 | **161 passed, 54 skipped** (opt-in screenshot specs). Earlier full runs while fixing: one failure each time, both fixed (see below). |
| axe | | part of the e2e run: console routes on live data, dialogs, public set; evidence files `docs/design/evidence/axe-m6.*` regenerated with identical content (0 serious/critical, 0 of any impact as before) |

After the final run no listener remained on 55440/18180/19190/19191/19300/3100; the compose project `clusterx` and the other worktrees were not touched.

**Flakes found and fixed (not product bugs):**
1. `console-auth.spec.ts` "an expired session returns to /login with the reason": the open console page can notice the expiry itself during the 6 s wait (stream revalidation or poll), clearing its cookie, so the later `goto` lacked the reason. The spec now leaves the console (`about:blank`) before waiting. Passed 3/3 alone, failed once under full-suite load before the fix.
2. `console-stream-hardening.spec.ts`: slots are held until the browser socket closes (that is the fixed behaviour), and the open console holds one slot itself, so the spec navigates away first and paces its aborted upgrades (one run saw a legitimate BFF 429). Passed 4/4 consecutive runs after the change.

## Docs truth update (only what main's code and handoffs show)

`/docs/security` (sign-in, sessions, CSRF, assertions, proxy, CSP, key rotation built; rate limiting implemented; drain is not exclusion; threat table; review status), `/docs/workers` (new "Drain, restart and keeping a host out": drain is not exclusion, revoke the credential, each graceful restart is a new row, operator drain sticky to the id, worker id file), `/docs/quickstart` (`.secrets/` files, dev BFF key, optional ports/prefix keys, old-volume retention-login caveat), `/docs/deployment` (retention profile, `.secrets/`, `*_FILE` variants, dev BFF key, optional keys, new section for the web server's `CLUSTERX_*` configuration), `/docs/api` (two stale lines: sessions "not built", rate limiting "never returned"), `/changelog` (M5 and M6 entries, migrations 0001..0004, not-yet list updated). The console worker-drain dialog states the drain-is-not-exclusion rule. Every unrun item is marked UNVERIFIED or not run in the text.

## Needs from the integrator / contract owner

1. **RV6-07:** amend ADR-0006 section 3. The implemented design is a sealed `{sid, uid, login, role, exp}` cookie (no `cxs_` value; Go authenticates by `sid`), and `CLUSTERX_WEB_SESSION_KEY` is an impersonation key for any live `sid`, as sensitive as the BFF signing key (add it to the B1/B2 key-compromise residual). Rotation: set the new key and `CLUSTERX_WEB_SESSION_KEY_PREVIOUS`, remove the previous key after the longest session lifetime (12 h absolute by default). Signing-key rotation still needs a web restart.
2. Independent re-review of RV6-01/02/03/09 (+ P3s) before the M8 gate, as the ticket says.
3. `.github` CI is unchanged; the new live specs run in the existing `pnpm --filter web e2e` job (needs the same PG/Go toolchain as before).

## NOT RUN / limitations

- Real GitHub, TLS, production mode in a browser (`__Host-`/Secure cookies, https-only API URL live), Firefox and WebKit: unchanged from the M6 handoff. The https-API rule and key rotation are unit-tested only.
- Memory exhaustion to OOM (RV6-02): measured as RSS growth for a 256 MiB request, thresholds as above; not pushed to OOM.
- Browser behaviour of `@media (scripting: enabled)` was checked in Chromium only (the `.reveal` pages passed the existing site and axe specs). Browsers without the media feature (very old) show reveal content immediately, not hidden.
- CSP: `style-src` still has `'unsafe-inline'` (React inline styles); only `script-src` was in scope. Public static pages still allow inline scripts (see RV6-09).
- A fresh-clone compose run was not done (docs describe `infra/compose` from code and the M5 handoffs).
- **Docs still stale outside the ticket's list:** `/architecture` (page tables, failure-modes rows and the diagram's "NOT BUILT YET" tag still say the BFF and console are "Specified, not built"), the home page layer cards, and the public header comment "No session implementation yet". Not changed because the ticket scoped five pages; they contradict the new truth and should be a follow-up ticket (they need the diagram updated and `site.spec.ts` expectations reviewed).
- The M6 screenshots in `docs/design/screenshots/m6/` were not re-captured; no layout changed apart from one sentence in the drain dialog.
