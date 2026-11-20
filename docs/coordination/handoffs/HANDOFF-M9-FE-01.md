# HANDOFF — TASK-M9-FE-01 (web release hardening and stale copy)

- **Task:** `docs/coordination/tasks/TASK-M9-FE-01.md`. Source: `docs/coordination/reviews/REVIEW-M6-RV-01.md` round 2 (RV6B-01..05); ADR-0006 section 2 as amended on main.
- **Author:** frontend lane, model `claude-sonnet-5-5`; effort `high` requested, effective effort not observable (UNVERIFIED).
- **Branch / base:** `m9/frontend`, worktree `.worktrees/frontend`, base main `303c194`. No push, no merge.
- **Contract:** `api/openapi.yaml` `1.0.0-draft.7`, tag `contract-m6-draft.7` (`ed2dc61`), file SHA-256 `d61d5042480d080418d6ec44725c8c0e88d9625f007b1cdc6fffea8d86270453`. Unchanged. `git diff --name-only 303c194..HEAD` touches only `apps/web/**`, `packages/ui/**`, `docs/design/**` (plus this handoff).
- **Status:** implemented and tested against the production build with Chromium on the live stack (own PG 18, Go control plane and workers built from this tree, mock OAuth). Not independently re-reviewed.

## Commits (excluding the handoff commit, whose id a file cannot hold)

| SHA | Items |
|---|---|
| `3e105b8` | RV6B-02 role hint refresh |
| `4ecf5ae` | RV6B-03 nonce CSP on every `/app`, `/login`, `/auth` path in any letter case |
| `bcf0b28` | RV6B-01 links from public pages into `/app` and `/login` load a new document |
| `4fd7dff` | RV6B-05 half-closed relay clients, per-user socket cap |
| `52ca940` | Regression found by the full suite in my own RV6B-03 change (proxy buffered BFF bodies), 404 hydration under nonce CSP, serial RSS specs |
| `04b1522` | Stale copy, changelog |
| `2f53858` | Recaptured `docs/design/screenshots/m7/{home,architecture,changelog,docs-security}-{390,1440}.png` |

## Per-item status

| ID | Status | What changed | Fail before | Pass after |
|---|---|---|---|---|
| RV6B-02 | fixed | `server/bff.ts`: after a Go 403 `asUser` calls `GET /me` and re-seals the cookie when the DB role differs (`roleCookie`); `/bff/session` does the same. The refresh `/me` call carries the role Go reported in the 403 `details.role` (a hint only), so the demotion writes one mismatch row, not two. `upstream.ts` parses `details.role`. | `m9-before.log`: cookie not re-sealed after the 403, not re-sealed on `/bff/session` | `e2e/console-role-refresh.spec.ts` (demote mid-session: 403 re-seals, rows written by the demotion <= 1, no more rows after 5 GETs and `/bff/session`; promote: `/bff/session` re-seals, <= 1 row, then none; unchanged role sets no cookie). Unit: 4 tests in `bff-core.test.ts` |
| RV6B-03 | fixed | `lib/csp.ts`: `NONCE_SEGMENTS`, case-insensitive `isNoncePath`, `STATIC_POLICY_SOURCE` derived from the same list. `proxy.ts` matcher now covers all page requests (not `_next`, `bff`, fonts, images, icon) and decides in code, so `/APP`, `/Login/x`, `/AUTH/x`, `/login/x` and their 404s get exactly one nonce policy. | 8 of 24 paths in `m9-before.log` had no CSP (`/APP`, `/APP/jobs`, `/App/x`, `/login/x`, `/LOGIN`, `/Login`, `/AUTH/x`, `/Auth/end`) | `e2e/console-csp-coverage.spec.ts` (15 nonce paths, 9 static paths, exactly one CSP header counted from raw headers); `csp.test.ts` (policy selection is a partition, matcher shape) |
| RV6B-01 | fixed | `ButtonLink reload` renders a plain anchor; used for the header CTA and the 404 Console button. Drawer CTA and footer Sign in are plain anchors. | `m9-before-01.log`: 18 of 19 failed (all four click sites, the crawl on all 13 public pages and the 404, the drawer) | `e2e/console-doc-boundary.spec.ts`: the four reviewer click paths (404 Console, header, footer, demo header) load a new document, response CSP has a nonce and no `'unsafe-inline'`, injected `onerror` handler blocked; a crawl clicks every `/app`, `/login`, `/auth` link on every public route and the 404; mobile drawer; a console-internal link stays client-side |
| RV6B-05 | fixed | `stream-relay.ts`: `socket.once("end")` destroys the browser socket before hand-off (slot and dial freed at once); `reserveSlot(sid, uid)` with `MAX_SOCKETS_PER_USER = 15` (session 5, process 256 unchanged). | stash of `stream-relay.ts`: both new unit tests fail (slot stuck past 1.5 s, per-user cap absent) | `stream-relay.test.ts`: real `attachStreamRelay` against a hung TCP upstream, FIN after the upgrade request frees the slot and the upstream socket within 1.5 s (was bounded by the 10 s handshake timeout); per-user cap test |
| Stale copy | fixed | `/architecture` layers, connection statuses, failure rows (live channel drop, BFF unreachable), diagram tag, legend and description; home layer cards and Inspection pillar; public layout comment; `not_implemented` error copy; `component-specs.md` preface; `/docs/security` review status and new bullets; changelog adds M7 and "M6 review fixes", M8 left out. | `m9-before-copy.log`: 3 of 3 | `site.spec.ts` (stale-copy sweep over 9 routes; architecture; changelog M1..M7, no `M8: `) |
| RV6B-04 | note only | No change. M8 runbook: edge proxy accepts only origin-form targets and GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS (`TRACE` and overlong UTF-8 give 500s in Next 16.4.6). | | |

## Decisions and findings the integrator should know

1. **Separate root layouts were tried and dropped.** The reviewer's preferred fix needs `experimental.globalNotFound` (no top-level layout). A 404 rendered that way gets no request nonce, so its scripts are blocked under the nonce CSP and it does not hydrate (seen live: CSP violations on `/app/x`, mobile Menu dead). The ticket allows hard navigations, so the boundary is plain anchors plus the crawl spec. Residual: a new `next/link` to `/app`, `/login` or `/auth` added to a public page would be soft; the crawl spec fails on it.
2. **The 404 did not hydrate under the nonce CSP before this ticket either** (`/_not-found` was prerendered, so no nonce). `not-found.tsx` now calls `connection()` and renders per request (`/_not-found` is dynamic in the build). Spec: `/app/does-not-exist`, `/APP/x`, `/login/x` give 404, no CSP violation, Menu works.
3. **My own RV6B-03 change regressed RV6-02**: a proxy buffers request bodies, so the broad matcher made `/bff` POSTs buffer (chunked spec: 256 MiB written before the 413). Caught by the full e2e run, fixed by excluding `bff/`; `csp.test.ts` pins it.
4. **RSS spec flake, cause is a hypothesis.** `console-proxy-hardening` "chunked body cut off at the cap" failed with +103 and +127 MiB against the 96 MiB limit, once in about 6 runs, while the Content-Length 256 MiB test ran in parallel in another worker against the same server. The two RSS tests now run serially (5 of 5 pass alone, final full run passes). I did not run main's code to confirm it was already flaky there.
5. **`/bff/session` role refresh and the console layout:** server-rendered pages call `/me` with the cookie's hint and cannot set cookies, so the first page load after a role change can still write one mismatch row before the client's `/bff/session` call re-seals. The stream relay seals its assertion from the cookie too, so an upgrade before the refresh can write one more. The row rate is bounded by that first refresh, not zero.
6. **Role refresh adds one `/me` call per Go 403** (rare; 403 on a role-correct user returns no cookie).
7. **Demo path** stayed `src/app/demo` (the lint and boundary test rely on it); no layout moves remain in the diff.
8. `docs/design/evidence/axe-m6.*` regenerate on every full e2e run; the final full run produced content identical to the committed files (0 serious, 0 critical).

## Commands and results (committed tree before the screenshot commit; the screenshot commit changes only PNGs)

| Command | Exit | Result |
|---|---|---|
| `pnpm --filter web lint` | 0 | |
| `pnpm --filter web typecheck` | 0 | |
| `pnpm --filter web test` | 0 | 14 files, 127 tests (was 13 / 116) |
| `pnpm --filter web build` | 0 | `/_not-found` dynamic, `/login` and `/app/*` dynamic, public pages static |
| `CLUSTERX_E2E_PG_DIR=…/scratchpad/pgfe3 pnpm --filter web e2e` | 0 | **211 passed, 54 skipped** (opt-in screenshot specs), 1.3 min. axe: 0 serious, 0 critical |
| `SCREENSHOTS_M7=1 … playwright test screenshots-m7 -g "(home\|architecture\|changelog\|docs-security) @"` | 0 | 8 passed, PNGs committed |

Logs under the session scratchpad: `m9-before.log`, `m9-before-01.log`, `m9-before-copy.log` (fail-before), `m9-final-*.log`, `m9-e2e-full.log` (earlier run with the two issues above), `m9-final-e2e.log`. Intermediate runs that failed: full run 1 (4 failures: the 404 hydration spec that exposed finding 2, and finding 3/4). After each run no listener remained on 55440/18180/19190/19191/19300/3100; compose project `clusterx` and the other worktrees were not touched.

## Not run / limitations

- No independent re-review of any of this. Real GitHub, TLS, production mode (`__Host-`/Secure) in a browser, Firefox and WebKit: not run, as before.
- RV6B-05(b) is covered by the unit test of `reserveSlot`, not by a live flood of 50+ sessions.
- Half-close is tested on the relay with a stand-in hung upstream, not with the real Go server (which answers too fast to hold a slot).
- `style-src 'unsafe-inline'` remains in both policies; public pages keep `script-src 'unsafe-inline'` (accepted by the reviewer if the console never runs inside a public document, which RV6B-01 now enforces by full loads).
- Screenshots recaptured only for the four pages whose copy changed; the 404 page, M6 console screenshots and design docs other than `component-specs.md` were not touched.
- Changelog has no M9 entry: it lists merged milestones only; the integrator adds it on merge.

## Needs from the integrator

1. Independent re-review of RV6B-01..03/05 (and the `/bff` matcher exclusion).
2. Merge decision for the M8 runbook note (RV6B-04).
3. After merging M8, add M8 to `/changelog` and update the `/docs/deployment` production section; this ticket did not touch them.
