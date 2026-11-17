# CLUSTERX — Requirements Register (M0)

Source: `docs/product/MASTER_PROMPT.md` R2 (adopted by owner 2026-11-17, OD-01). Section refs are `§n`.
This register classifies every normative requirement in the master. It adds no scope. Status lives in `traceability.md`.

Priority: **P0** = release-blocking correctness/security; **P1** = required for 1.0; **P2** = required but lower risk.
Release: **A** Alpha · **B** Beta · **RC** · **1.1** (separately gated) · **X** explicitly excluded.

## Functional — engine (backend lane)

| ID | Requirement | § | Pri | Rel | AC |
|---|---|---|---|---|---|
| FR-ENG-01 | Durable typed job admission; 2xx only after commit | 5, 6 | P0 | A | AC-01, AC-08 |
| FR-ENG-02 | States `scheduled, queued, running, retry_wait, succeeded, failed, canceled`; terminal immutable while retained | 5 | P0 | A | AC-03, AC-06 |
| FR-ENG-03 | Full transition table incl. denials, generated from one source | 5 | P0 | A | AC-02..06 |
| FR-ENG-04 | Atomic claim: job→running + attempt + lease generation in one transaction | 5 | P0 | A | AC-02 |
| FR-ENG-05 | Worker-pull gRPC: Register, Claim, RenewLease, ReportResult/Failure, PollCancellation, Drain/Deregister, Heartbeat | 6 | P0 | A | AC-09, AC-20 |
| FR-ENG-06 | Handler registry, typed name + schema version, allowlisted precompiled handlers only | 4, 5 | P0 | A | AC-10, AC-12 |
| FR-ENG-07 | Delayed jobs via `run_after`; DB clock authoritative for due/expiry | 5 | P1 | A | AC-10 |
| FR-ENG-08 | Priority 0–9 (9 highest); tie-break `(priority DESC, available_at, created_at, id)` among eligible rows | 5 | P1 | A | AC-10 |
| FR-ENG-09 | Unschedulable jobs stay visible with actionable reason (not a state) | 5 | P1 | A | AC-10 |
| FR-ENG-10 | Leases + heartbeat (distinct), fencing by attempt/generation/incarnation/expiry/deadline | 5 | P0 | B | AC-03, AC-04, AC-09 |
| FR-ENG-11 | Bounded automatic retries; `max_attempts` 1–5 incl. first run, default 3; backoff+jitter with injectable clock | 5 | P0 | B | AC-05 |
| FR-ENG-12 | Manual retry of terminal `failed` creates new linked job (`retried_from_job_id`), idempotent action key, audited | 5 | P1 | B | AC-15 |
| FR-ENG-13 | Cooperative cancel; commit-order precedence between cancel request and success | 5 | P0 | B | AC-06 |
| FR-ENG-14 | Worker drain: stop claims, grace period, then normal cancel/lease recovery; SIGTERM | 5 | P1 | B | AC-11 |
| FR-ENG-15 | Lease expiry / hard deadline recovery; no job permanently `running` | 5 | P0 | B | AC-04 |
| FR-ENG-16 | Stale/duplicate results rejected + bounded diagnostic event | 5 | P0 | B | AC-03, AC-09 |
| FR-ENG-17 | Idempotent submission scoped principal+namespace+key, canonical fingerprint, 24 h, 409 on mismatch | 5 | P0 | A | AC-07 |
| FR-ENG-18 | Bounds: payload 64 KiB, result 128 KiB, bounded logs; explicit over-limit behavior | 5 | P0 | A | AC-12 |
| FR-ENG-19 | Worker identity: `worker_id` + `incarnation_id`, capabilities, capacity, credential hash/scope/expiry/revocation | 4 | P0 | B | AC-09 |
| FR-ENG-20 | Transactional per-job event log with monotonic `version` | 6 | P0 | A | AC-16 |
| FR-ENG-21 | Go worker SDK: registration, schema validation, ctx cancellation, typed retryable errors, logging | 6 | P1 | A | AC-20 |
| FR-ENG-22 | DB outage: no ack of uncommitted work, readiness fails, claims pause, clean resume | 6 | P0 | B | AC-08 |
| FR-ENG-23 | Retention cleanup (logs 7 d, history 30 d, audit 90 d — ADR to finalize), FK-safe | 6 | P1 | RC | AC-17 |
| FR-ENG-24 | Forward-only expand/contract migrations; backup/restore | 6 | P1 | RC | AC-17 |

## Functional — API, auth, security

| ID | Requirement | § | Pri | Rel | AC |
|---|---|---|---|---|---|
| FR-API-01 | REST `/api/v1` operations listed in §6 with typed error `{code,message,details?,request_id}` | 6 | P0 | B | AC-15, AC-20 |
| FR-API-02 | Cursor pagination, stable sort, bounded page size/time window | 6 | P1 | B | AC-15 |
| FR-API-03 | OpenAPI (3.2.1 if generator-compatible else 3.1) source of truth; generated TS client; CI compat check | 6 | P1 | B | AC-20 |
| FR-API-04 | `/health/live`, `/health/ready`, protected `/metrics` | 6 | P1 | A | AC-08 |
| FR-SEC-01 | GitHub OAuth code+PKCE/state, server session, numeric-ID allowlist, roles operator/viewer (OD-03 pending) | 4, 7 | P0 | B | AC-13 |
| FR-SEC-02 | BFF→Go short-lived audience-bound signed service assertions; no trusted forwarded headers | 4, 7 | P0 | B | AC-13 |
| FR-SEC-03 | Direct API tokens: hashed, scoped, revocable, audited | 4, 7 | P0 | B | AC-13 |
| FR-SEC-04 | Go authorizes every op, object, sensitive field; viewer field masking | 4, 7 | P0 | B | AC-13 |
| FR-SEC-05 | Worker credentials: operator-issued, hashed, rotate/revoke, checked on claim/renew/report | 7 | P0 | B | AC-09 |
| FR-SEC-06 | WebSocket auth + origin check; reconnect after logout rejected | 7 | P0 | RC | AC-13 |
| FR-SEC-07 | CSRF/origin protection, secure cookies, CSP/security headers, redirect validation | 7 | P0 | RC | AC-13 |
| FR-SEC-08 | Worker container isolation: nonroot, RO fs, bounded resources, no shell/SSRF task types | 7 | P0 | RC | AC-12 |
| FR-SEC-09 | Threat model (STRIDE), ASVS 5.0 mapping, access matrix | 7 | P0 | RC | AC-25 |
| FR-SEC-10 | Supply chain: pinned actions by SHA, scans, SBOM, lockfiles, no secrets | 7 | P0 | RC | AC-25 |
| FR-SEC-11 | Audit events for privileged actions | 6, 7 | P1 | B | AC-13 |

## Functional — realtime, demo, web (frontend lane unless noted)

| ID | Requirement | § | Pri | Rel | AC |
|---|---|---|---|---|---|
| FR-RT-01 | WebSocket hints only; HTTP snapshot w/ version authoritative; polling fallback; stale/disconnected visible | 6 | P0 | RC | AC-16 |
| FR-DEMO-01 | `/demo` fixture-backed, read-only, no requests to Go control plane, labeled illustrative | 4 | P0 | RC | AC-14 |
| FR-WEB-01 | Public routes: `/`, `/architecture`, `/docs`, `/docs/{quickstart,concepts,api,workers,reliability,deployment,security}`, `/changelog`, `/login`, `/demo` | 8 | P1 | RC | AC-21 |
| FR-WEB-02 | Console routes: `/app`, `/app/jobs`, `/app/jobs/new`, `/app/jobs/[id]`, `/app/workers`, `/app/workers/[id]`, `/app/observability`, `/app/activity`, `/app/settings`, `/app/settings/tokens`, `/app/settings/preferences` | 8 | P1 | RC | AC-15, AC-21 |
| FR-WEB-03 | Per-route contract (purpose, persona, sections, data, actions, RBAC, responsive, motion, states, a11y, E2E IDs) | 8 | P1 | RC | AC-21 |
| FR-WEB-04 | Shared component inventory, every control functional or omitted | 8 | P1 | RC | AC-21 |
| FR-WEB-05 | White-first "Precision Under Motion" identity, tokens per §9 | 9 | P1 | RC | AC-23 |
| FR-WEB-06 | Execution Atlas: illustrative marketing variant + data-backed console topology at 1/10/100 workers, list equivalent | 9 | P1 | RC | AC-23 |
| FR-WEB-07 | Motion matrix with reduced-motion fallbacks | 9 | P1 | RC | AC-22 |
| FR-WEB-08 | Error/edge flows 401/403/404/500, session expiry, too-many-rows, no-capable-worker, no-metrics, expired data | 8 | P1 | RC | AC-15, AC-21 |
| FR-WEB-09 | BFF/session UI (Sonnet implements under Opus auth contract) | 1.1, 7 | P0 | RC | AC-13 |

## Non-functional

| ID | Requirement | § | Pri | Rel | AC |
|---|---|---|---|---|---|
| NFR-A11Y-01 | WCAG 2.2 AA, 320 px reflow, keyboard, SR manual walkthrough | 10 | P0 | RC | AC-22 |
| NFR-RESP-01 | Widths 320/360/390/768/1024/1440/1920; browser matrix defined pre-release | 10 | P1 | RC | AC-23 |
| NFR-PERF-01 | Public LCP ≤2.5 s, INP ≤200 ms, CLS ≤0.1 (field or labeled lab) | 10 | P1 | RC | AC-24 |
| NFR-PERF-02 | Console budgets frozen after baseline before RC | 10 | P1 | RC | AC-24 |
| NFR-PERF-03 | Backend SLOs/RPO/RTO frozen after pilot (OD-06) | 11, 12 | P1 | RC | AC-17, AC-19 |
| NFR-SCALE-01 | 1,000 jobs / 3 workers reconcile exactly; 1/3/5 scaling report p50/p95/p99 | 11 | P1 | RC | AC-18, AC-19 |
| NFR-OBS-01 | OTel traces/metrics, structured logs, bounded cardinality, alerts + runbooks | 3, 12 | P1 | RC | AC-27 |
| NFR-OPS-01 | Docker Compose local + production example, `.env.example` no creds, fail-closed prod config | 12 | P1 | RC | AC-26, AC-27 |
| NFR-OPS-02 | Clean-checkout reproducible demo (coordinator, PG, web, 3 workers) | 2, 12 | P1 | RC | AC-26 |
| NFR-DEP-01 | Pinned compatibility set + `docs/architecture/dependency-matrix.md` (OD-07) | 3 | P1 | M3 | AC-20 |
| NFR-DOC-01 | README, CONTRIBUTING, SECURITY, CHANGELOG, LICENSE (OD-02) match shipped code | 13 | P1 | 1.0 | AC-28 |
| NFR-REL-01 | All AC-01..AC-30 with evidence before 1.0 | 11 | P0 | 1.0 | AC-30 |

## Multi-model workflow

MW-01..MW-08 (§1.4) and MW-01..MW-05 (§11) apply to every task. Tracked in `traceability.md`.

## 1.1 (gated) and excluded

- **1.1:** FR-WF-01 DAG definitions, cycle validation, dependency execution, propagation, recovery, graph editor, `/app/workflows*`, states `blocked`/`skipped`. AC-29 asserts absence from 1.0.
- **Excluded (X):** shell/code upload, user-supplied fetch URLs, untrusted sandboxing, multi-tenant, billing, cron, HA leader election, autoscaling, cross-region, large binaries, second queue, unlimited payloads/logs, exactly-once side effects.

## Unclassified requirements

None. Every normative item in R2 §2–§13 maps to an ID above or to an AC directly.
