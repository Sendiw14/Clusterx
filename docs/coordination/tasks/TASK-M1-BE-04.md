# TASK-M1-BE-04 — Review fixes + FE-02 gaps (M1 exit)

- Milestone: M1 exit. Owner: backend lane, `claude-opus-5-5`, effort medium (requested). Branch `m1/backend`, worktree `.worktrees/backend`.
- Inputs: `docs/coordination/reviews/REVIEW-M1-RV-01.md` (on main), `HANDOFF-M1-FE-02.md` §gaps + `consumer-requirements.md` §11.8 (on `m1/frontend@0e06a76`, read via `git show`).
- Allowed paths: as TASK-M1-BE-01 (incl. temporary `api/openapi.yaml` ownership) + `docs/coordination/handoffs/HANDOFF-M1-BE-04.md`.
- Re-review: RV-01 reviewer re-checks P1s and every item marked FIX below. Reviewer findings are not self-closable.

## Must FIX (blocks M1 exit)

| Item | Required outcome |
|---|---|
| RV01-01 | PoC tests for every fence clause: credential revoked, credential expired, worker incarnation mismatch, attempt incarnation mismatch, deregistered worker, hard deadline passed; recovery with cancel requested ⇒ `canceled`. Prove each kills its mutant (M1–M5, M11): record mutant → failing test in handoff. |
| RV01-02 | Token authority = `token.scopes ∩ owner's CURRENT role`, denied if owner `disabled_at` set or role revoked. Update ADR-0005, permissions.md, OpenAPI `ApiTokenCreate` text; add PoC/SQL test or precise spec for M5. |
| RV01-03 | One canonical worker reply for success-after-committed-cancel and its retry. Align state-machine.md, worker.proto and PoC; idempotent retry returns the same outcome. |

## FIX (P2, cheap now; contract must not change after freeze)

| Item | Required outcome |
|---|---|
| RV01-04 | Either lock worker/credential rows (`FOR SHARE`) and evaluate deadline with `clock_timestamp()` in the fence, or make ADR-0003 state the exact bound. Prefer the fix; add a test. |
| RV01-05 | DB enforces `attempt_number <= jobs.max_attempts` and attempt's job relationship; `max_attempts` immutable after admission. Test. |
| RV01-06 | Trigger blocks rewriting `retried_from_job_id` on terminal jobs; event/audit tables block DELETE except via the documented retention function/role. Test. |
| RV01-07, RV01-08 | Duplicate-report path checks worker identity; discarded success leaves a bounded diagnostic event. |
| RV01-09 | AC-06 test asserts both orderings occurred (or forces each deterministically); AC-03 made concurrent or justified. |
| RV01-11 | Session bound to asserting user: Go rejects assertion whose `sub` ≠ session owner. ADR-0006 + OpenAPI. |
| RV01-12 | ADR-0006: `jti` cache loss on restart bounded by assertion TTL; document. |
| RV01-13 | Streams opened with API token re-validated (≤30 s, same as sessions). Spec. |
| RV01-14 | Viewer masking applies to `JobEvent.data`; content bound documented. |
| Other RV01 P2s | Fix or record disposition with reason in handoff. RV01-15 → DEFER M4 (before AC-19). |

## FE-02 gaps to close in contract (draft.4)

SESS-01 (`sessions.last_activity_at`, Go enforces idle), BFF-01 (closed login outcome set), BFF-02 (BFF proxies Go status/headers/envelope unchanged), ERR-01/02 (`details.fields[].rule` enum, JSON-Pointer paths), SORT-01 (`sort` param + `available_after/before`, `failure_class` filters), CRED-01 (`Idempotency-Key` on credential creates), WRK-01 (`revoked_at`), TIME-01 (`as_of` on single-resource responses), OVR-01 (named window on overview counts), CFG-01 (environment, build commit, contract version in `/config`), DOC-01 (all listed text mismatches).

Deferred (do not implement): RT-01..03, AUDIT-01/02 → M6; OBS-01/02, RET-01 → M8.

## Acceptance

DDL applies on fresh PG18; `go vet`; `go test -race -count=3 ./poc/...` green; each new fence test fails against its mutant (evidence in handoff); redocly lint 0 errors; `info.version: 1.0.0-draft.4`.
