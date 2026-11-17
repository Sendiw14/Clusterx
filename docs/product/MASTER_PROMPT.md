# CLUSTERX — Final Unified Implementation Master Prompt

**Revision:** R2 · CLUSTERX product release target v1.0 · audit date 2026-11-17  
**Repository:** https://github.com/Sendiw14/Clusterx  
**Product:** Open-source distributed task-processing engine, operator console, editorial public website, documentation, reproducible demonstrations  
**Identity:** **Precision Under Motion** — white-first; original **Execution Atlas** visual grammar  
**Status:** Candidate single-source master contract. Becomes the approved repository baseline only after owner adoption and ADR sign-off. It specifies implementation and verification; it is not evidence of code, tests, entitlement, deployment, or production readiness.

---

## 0. Mission, authority, and operating agreement

Act as a **staff-level distributed-systems architect, Go engineer, full-stack technical lead, product designer, creative frontend director, accessibility specialist, security engineer, QA/reliability lead, DevOps engineer, and open-source maintainer**. Deliver a **genuine, self-hostable, single-organization distributed background-job platform** that reviewers can clone, run, inspect, benchmark, and understand. The Go engine is the engineering centerpiece; the original polished web experience makes its real behavior easy to learn and operate.

Do not build a dashboard that merely imitates distributed processing. Do not quietly remove required functionality, invent test results, display synthetic telemetry as live, promise exactly-once external side effects, or mistake extensive scaffolding for working software. **Correctness and honest guarantees outrank visual spectacle; exceptional frontend craft is still required for every shipped page.**

**Instruction precedence:** (1) safety requirements and current explicit owner approvals; (2) this single master contract, including its model-ownership and acceptance rules; (3) approved, versioned ADRs, API/schema contracts and design artifacts that refine but do not silently override this contract; (4) task tickets and implementation details. Prior plans and audits are reference material, not competing sources of truth. When an ADR modifies this baseline, update the requirements matrix, affected design/contracts/tests, and decision register in the same change. Never silently reinterpret a normative invariant.

**First actions:** obtain documented owner adoption of this master for project scope (not publication); inspect current repository HEAD, branches, README, licenses, existing work, local toolchains, and permissions. On 2026-11-17 the connected GitHub repository was public and empty; **recheck—this can change**. Preserve user work. Work in small reviewable increments; never force-push, delete, publish, spend money, grant external access, or deploy to live infrastructure without explicit owner authorization. If a tool or credential is unavailable, finish testable local work and report the blocked operation accurately.

**Evidence vocabulary:** `proposed → specified → approved → implemented → integrated → tested → verified → released`. Record each status separately in `docs/product/traceability.md`; neither a prompt nor generated code proves the later states. If a test could not run, state **NOT RUN**, not PASS. State assumptions, blockers, changes, command output, tests, and evidence after each milestone. Prefer straightforward, proven solutions over novelty. Avoid prematurely introducing Kubernetes, Kafka, extra queues, microservices, AI agents, untrusted executable tasks, unnecessary 3D, or a second source of truth.

## 1. Multi-model engineering governance, file ownership, and integration

**This section is normative for all AI coding sessions.** The master prompt in `docs/product/MASTER_PROMPT.md` is the **only product/quality authority**; `CLAUDE.md`, `.claude/agents/*.md`, settings, tickets and handoff templates are operational aids that must never add conflicting product scope. An owner-approved ADR may refine details only if synchronized with this master, generated contracts, tests, and traceability in the same change. Agent sessions cannot independently approve their own changes, licensing, releases or expenses.

### 1.1 Model availability, routing and settings

As of 2026-11-17 Anthropic documents `claude-sonnet-5-5` and `claude-opus-5-5` model IDs. These **are not proof of local account entitlement, provider mapping or configured tool access**. Before every initial session or model switch, capture `claude --version`, model via `/model`, effort via `/effort`, actual account/provider availability via `/status`, and current `ultracode`/workflow availability. Prefer a Claude Code version >= **2.1.284** if using Ultracode with non-`xhigh` effort. `ultracode` is an optional workflow orchestration setting, **not** the effort value. `--effort ultracode` sets `xhigh`; it does not represent Sonnet `high` or Opus `medium`. Provider-specific IDs may differ; select only identifiers confirmed locally. If unavailable, use isolated manual sessions, request owner approval for a compatible fallback, and **never silently relabel a different model**. The user's preferred settings must be applied explicitly; platform API defaults and Claude Code defaults are not equivalent.

| Work | Author / default | Effort | Review & escalation |
|---|---|---|---|
| Product scoping, risk/ADR arbitration, database/API architecture, integration lead | Opus 5.5 | medium | Owner approves nonreversible scope, costs, identity or security exceptions |
| Go coordinator, worker SDK, PostgreSQL, state machine, gRPC, REST, auth, infrastructure | Opus 5.5 | medium | **Separate Opus high session** for transactional races, unsafe migrations, auth and release blockers |
| White-first brand, UX, Execution Atlas, responsive screens, web components, charts, animation, docs presentation | Sonnet 5.5 | high | Independent visual/accessibility reviewer; Opus validates technical claims/data semantics |
| Web-side session/BFF code and frontend authorization UI | Sonnet 5.5 implementation, Opus contract owner | high | Opus security review; direct Go API authorization is backend-owned |
| Frontend integration/E2E/visual regression, motion failures | Sonnet 5.5 | medium/high | Opus reviews API state, failures and permission accuracy |
| Go unit/integration/race/chaos, DB failpoint and protocol tests | Opus 5.5 | medium/high by risk | Separate critical reviewer; deterministic tools and CI are final evidence |
| CI, deployment, release integration, dependency matrix and shared generated outputs | Opus 5.5 integration lead | medium | Cross-lane review, human release approval |
| Obvious copy/test snapshot/refactoring tasks | Sonnet medium or deterministic tools | medium | Upgrade only if failures/complexity justify spending |

Use **two primary active editing lanes** (frontend and backend), one integration custodian, and a short-lived read-only reviewer for high-risk work. Ultracode is optional for complex bounded work, not mandatory for trivial tasks; keep `workflowSizeGuideline=small` as guidance **not an agent-count hard cap**. Never spawn parallel agents merely because a feature exists. Track real usage if available, review effort after rework, and prefer passing tests and maintainable code over minimized token count. Independently reviewed does not mean independently verified by a human; mark evidence accurately.

### 1.2 Exclusive path ownership and separation of duties

- **Frontend:** `apps/web/**`, `packages/ui/**`, `docs/design/**`, browser E2E, frontend snapshots and visual assets. No changes to canonical Go, SQL migrations, OpenAPI/proto or security-critical backend policy without contract change.
- **Backend:** `apps/control-plane/**`, `apps/worker/**`, `packages/sdk-go/**`, `packages/proto/**`, authoritative migration/schema, Go tests and backend ADR proposals. No changes to frontend design systems without lane handoff.
- **Integrator:** root workspaces/config, `.github/**`, `infra/**`, `CLAUDE.md`, `.claude/settings.json`, `docs/product/MASTER_PROMPT.md`, product traceability, decision register, shared OpenAPI schema, generated `packages/sdk-ts/**`, branch integration and release evidence. Backend proposes and implements Go-side contracts; integrator owns freeze/version/generation; frontend can request contract modifications and review typed examples.
- **Reviewer:** operates read-only in a separate context/branch whenever possible; submits findings and failing-test recommendations, never approves its own authored changes. Release decisions remain the owner's responsibility.

A task ticket may grant **temporary, explicit file ownership transfers**; a branch name alone does not enforce isolation. Do not write concurrently to the same file or to generated outputs with conflicting generators. Do not permit direct pushes to protected `main` when repository settings support protection. Use one owner per ticket and one feature branch/worktree per independent task. When the repository is empty, obtain owner authorization for an initial commit before creating Git worktrees; inspect HEAD/permissions again before any push.

### 1.3 Contract-first handoff and change protocol

1. Lead issues one scoped ticket with requirement IDs, user journey, dependencies, permitted paths, expected behaviors and tests.
2. Backend/lead writes **versioned source of truth**: SQL state/constraints, OpenAPI operations/examples/errors, protobuf messages/deadlines, RBAC and event/realtime semantics. Frontend signs off consumer requirements and difficult empty/error/permission states.
3. Integrator records a contract version or immutable commit SHA and freezes it for the relevant sprint. Generated clients are produced from that source; **never hand-edit generated files**.
4. Frontend may implement with explicitly marked fixtures while backend implements real services independently; frontend cannot mark a feature integrated while using mocks. Use a transport abstraction that switches to the generated client without rewriting page logic.
5. Each PR carries a handoff: ticket/requirement IDs, contract SHA, commits/file list, commands/results, screenshots where relevant, security/a11y impact, outstanding failures, next dependency and reviewer. A CI result is evidence only if actually run against the submitted commit.
6. Integration lead rebases sequentially on frozen contracts, regenerates typed clients, runs API+DB+browser E2E and negative permission/reconnect tests, reviews UI against actual persisted state, and updates traceability. API changes after freeze require an explicit change request and regression tests on **both** lanes.
7. Escalate P0 conflicts before affected code. For ordinary reversible uncertainty, document a default and validation test; do not invent owner approval.

### 1.4 Definition of done for model tasks and release

A model-authored task is **ready for review** when unit/contract tests pass, required files are within its ownership, docs/traceability are updated, mock use is labeled, and its handoff is complete. It is **integrated** only when both API and UI agree under live E2E on authorized and denied paths. It is **verified** only when release-relevant checks actually passed with evidence. No model, agent team, or chain of model approvals may overrule failed deterministic tests or unresolved critical security findings.

**Workflow acceptance additions:** MW-01 every task has one owner, explicit paths, dependency and review; MW-02 frozen schema change rebuilds generated SDK and reruns cross-lane E2E; MW-03 two worktrees can merge without unreviewed shared-file overwrite; MW-04 each model/effort/Ultracode setting is confirmed from actual session evidence or marked unavailable; MW-05 reviewer never self-approves; MW-06 release report reconciles all requirement/test/contract states; MW-07 token spend and agent count are monitored when supported, but no fabricated budget numbers; MW-08 owner approval gates block destructive/public/cost actions.

## 2. Product contract, users, and bounded scope

**Primary users:** (1) self-hosting developer integrating bounded trusted jobs, (2) operator diagnosing queue/worker failures, (3) read-only viewer reviewing cluster health, (4) portfolio evaluator exploring a safe isolated demonstration. Prioritize the developer/operator flow; evaluator access must never expose production data. Key journeys: install → register worker → submit typed job → inspect attempts/results; identify delayed/failed/unschedulable jobs; drain worker; observe lease recovery; use API/SDK; inspect monitoring; verify documentation.

**North-star demonstrator:** from a fresh checkout, start a coordinator, persistent PostgreSQL, web console, and **three independent worker containers**; submit **1,000 deterministic bounded jobs**; show real queue/worker metrics; terminate a worker during execution; recover expired attempts; demonstrate a late stale completion being rejected; add workers and measure throughput with honest hardware/fixture disclosure. The demo illustrates **at-least-once** processing, not exactly-once side effects.

**Delivery stages, not permission to ship incomplete 1.0:**

- **Alpha (engine):** durable admission, basic job types, DB schema, authenticated worker claims, result recording, minimal CLI/curl guide, correctness tests.
- **Beta (reliability/security):** leases, fencing, retry, cancellation, drain, worker credential lifecycle, role-protected REST API, outage tests.
- **Release candidate (product):** complete 1.0 routes, live state, separate safe demo, polished creative public site/docs, observability, security/accessibility/performance/deployment evidence.
- **1.0 release:** all 1.0 blocking acceptance gates satisfied and documented; no known critical vulnerability or untested required reliability claim.
- **1.1, separately gated:** bounded versioned DAG workflow definitions, cycle validation, dependencies, execution, propagation, recovery, graph editor and workflow-runs UI.

**1.0 required:** durable typed jobs; independent Go workers; worker-pull gRPC; atomic claims; PostgreSQL-owned state; delayed jobs; priority 0–9 (**9 highest**); bounded automatic retries; manual terminal retry via a **new linked job**; cooperative cancel; worker drain; heartbeat/lease/fencing; handler registry and versioned schemas; bounded results/logs; REST/OpenAPI; Go worker SDK and generated TS API client; operator/viewer auth; real jobs/workers/metrics/events in authenticated console; best-effort live notifications with reconciliation; production-safe documentation; isolated read-only public demo; complete white-first marketing/docs experience; Docker Compose, CI, security, accessibility, failure, benchmark, backup/restore and GitHub artifacts.

**1.1 only:** workflows and DAG editor; no nonfunctional 1.1 controls in 1.0. **Explicitly excluded from 1.0:** arbitrary shell/code upload or execution, user-supplied URLs for fetching, untrusted container sandboxing, multi-tenant SaaS, billing, cron, distributed HA leader election, autoscaling, cross-region failover, large binary artifacts, a second authoritative queue, unlimited payloads/logs, and exactly-once side effects. Label anything else `future/experimental` rather than implied 1.0.

## 3. Verified 2026 technology baseline and compatibility rules

Use a **tested compatibility set**, not a collection of independently newest packages. Baseline checked against official sources on **2026-11-17** (reverify release and security advisories at actual implementation time):

| Area | Default or candidate | Decision rule |
|---|---|---|
| Go services and SDK | Supported patched **Go 1.27.x** (1.27.4 at baseline) | Pin image/toolchain patch and run `go test -race` |
| Durable database | **PostgreSQL 18.7** production baseline | PostgreSQL 19 was RC as of audit date; don't adopt prerelease DB in production |
| Node.js | Supported **26.x LTS** (specific patch to verify at setup) | Confirm Next/CI compatibility; pin patch |
| Web framework | Patched **Next.js 16.x Active LTS** App Router | Use modern async request APIs, `proxy.ts` only where applicable; never rely on proxy for authorization |
| React | Compatible patched **React 19.x** (19.3 baseline) | Respect selected Next peer requirements |
| TypeScript | **7.x candidate**, otherwise stable compatible **6.x** | Adopt only if Next, ESLint, editor, codegen, Vitest and build pass together |
| Styling | **Tailwind CSS 4.x** (4.3 baseline) | Use current CSS-first token conventions, not copied v3 setup |
| UI motion | Supported **Motion 14.x** | Use sparingly; reduced-motion and interruption tests |
| Data visualizations | Accessible chart library and graph renderer selected via ADR | Avoid huge initial bundles; graph view must have equivalent list/table |
| APIs | REST + **OpenAPI 3.2.1 if generator-compatible**, otherwise 3.1.x; **gRPC/protobuf** | Generate/validate contracts and typed clients in CI; prefer interoperable tested proto edition |
| Observability | OTel stable traces/metrics, structured logs, Prometheus-compatible metrics | Avoid unsupported signal features; bounded metric cardinality |
| Local/deployment | Docker Compose Specification, Linux reference VM, TLS reverse proxy | Use `docker compose`; no legacy Compose V1 instructions |

Use `pnpm` lockfile, pinned Go modules, reproducible protobuf/OpenAPI generation and a single `docs/architecture/dependency-matrix.md` with exact tested versions, support horizon, rationale and compatibility evidence. Use current supported ESLint/Biome CLI—not removed `next lint`; `next build` does **not** replace a lint step. Use modern `proxy.ts` convention instead of deprecated middleware naming when relevant. Test Turbopack and plugins; fall back only with an ADR. Use current React/Next/Tailwind APIs, server-rendered shells and small client islands; do not adopt a compiler or optimization technique without evidence of benefit. Setup automation should check current official lifecycles and advisories rather than hard-code “odd Node releases aren't LTS.”

**Reference standards:** WCAG **2.2 AA** (WCAG 3 is draft); OWASP **ASVS 5.0.0**, OWASP Top 10:2025 and API Top 10:2023; IETF **RFC 9700** OAuth security BCP and **RFC 10017** browser-based apps; final NIST **SSDF 1.1** (1.2 draft); proportional SLSA **1.2** provenance guidance; pinned GitHub Actions and least privileges. OAuth 2.1 is still draft; do not assert conformance to a draft as a published RFC. Record applicable security clauses and test evidence, not a blanket “compliant” marketing claim.

## 4. Architecture and trust boundaries

**Reference topology:**

`Browser (HTTPS) → Next.js SSR/BFF (human session & authorization) → Go control plane (REST, gRPC, scheduler, event hub) ↔ PostgreSQL (ONLY authoritative jobs/leases/events)`

`Independent trusted Go workers → authenticated gRPC claim/renew/report → Go control plane`.

`Console live channel → authenticated best-effort WebSocket notification → HTTP snapshot reconciliation`.

Use **one modular Go control plane**, not fictional microservices. Go owns durable job admission, due-job promotion, transactional claims, fencing, retries, cancellation, recovery, worker registration, metrics and REST authorization. Workers pull jobs when capacity is available, execute only precompiled allowlisted handlers, and cannot mutate PostgreSQL directly. Next.js manages human sessions and presentation; its server-side BFF calls Go with **short-lived, audience-bound, signed service assertions** containing principal and role, verified by Go. Direct API clients use **separately issued, hashed, revocable, scoped tokens** validated by Go. Never trust arbitrary forwarded user/role headers. No Go/worker privilege tokens in browser JavaScript, analytics, screenshots or logs.

**Human authentication proposed default:** GitHub OAuth authorization-code flow with PKCE/state and a server-held session, restricted by explicit GitHub numeric user-ID allowlist plus assigned `operator` or `viewer` role. GitHub OAuth is **not** GitHub Actions OIDC. Use a maintained proven auth library; don't invent cryptography/session handling. Initial admin/operator entitlement is provisioned through a documented server-side secret/configuration step (not a shared default password). For local demo use isolated fixtures, not a production-bypass login. Owner must approve provider and first operator before deployment. Authorization is enforced in Go on every endpoint, including direct API calls and WebSockets. Operator may mutate job/worker and manage credentials; viewer is read-only. Define **field-level** visibility: sensitive payload/result/log fields are masked or denied for viewers even if actions are read-only; never treat secrets as allowable job payloads.

**Workers:** explicit per-process `worker_id` + unique `incarnation_id`, handler type/version/capabilities, capacity, enrollment status, credential hash/scope/expiry, revoked time, heartbeat/liveness time. A revoked identity is rejected on any new claim, renewal and result mutation; bound any authentication caching (prefer authoritative validation for security-critical mutation) and document precise revocation behavior. Use TLS for remotely accessible gRPC and deployed HTTP; protect internal network, ports and credential rotation. A revoked/disconnected worker may **still continue external work** temporarily: its stale database mutation must be fenced; no false physical-stop guarantee.

**Public `/demo`:** choose **fixture-backed read-only presentation with no requests to live Go control-plane endpoints**, clearly labeled illustrative/seeded. Real engine demonstration is instead reproducible via local Docker Compose. The public visual simulation must not be labeled live telemetry. No production data, live credentials or write routes from anonymous paths. Use shared UI primitives but separate data provider and network boundary; enforce with integration tests.

## 5. Normative job lifecycle, timing, and concurrency guarantees

**Job states in 1.0:** `scheduled`, `queued`, `running`, `retry_wait`, `succeeded`, `failed`, `canceled`. `blocked` and `skipped` reserved for 1.1. Additional orthogonal fields include `cancel_requested_at`, `available_at`, `current_attempt_id`, `lease_generation`, `hard_deadline_at`, and monotonic `version`. Terminal states (`succeeded`, `failed`, `canceled`) are **immutable while retained**; retention deletion is a separate documented lifecycle operation, not a state rewrite. No second dead-letter state/queue: dead-letter UI is a **filtered terminal `failed` view**.

**Required transition rules (define full generated state/event table, covering denials):**

| Event | Condition | Authoritative state change |
|---|---|---|
| Admit | Valid + committed payload; `run_after > DB NOW` | insert `scheduled`; else insert `queued` |
| Due promotion | `scheduled`/`retry_wait` due according to DB clock | `queued` |
| Claim | `queued`, eligible type/version/capacity, authorized worker; no current lease | `running`, create attempt and new lease generation **in one transaction** |
| Success | Current unexpired lease + generation + worker identity + hard deadline valid; no committed cancellation request | `succeeded`, finalize current attempt and append event atomically |
| Retryable failure | Valid current attempt; attempts remaining; no cancellation | `retry_wait` with bounded delayed availability, attempt finalized |
| Permanent/exhausted failure | Valid current attempt; not canceled | `failed`, attempt finalized |
| Lease expiry / timeout | Current lease expired or hard deadline reached | fence attempt; if cancel requested → `canceled`, else retry/fail by attempt budget |
| Cancel pending job | `queued`/`scheduled`/`retry_wait` | `canceled` atomically |
| Request running cancellation | `running` and request commits before completion | set `cancel_requested_at`; mark request event; notify worker; **subsequent success cannot commit** |
| Cancel acknowledged/lease or deadline expires | cancellation requested | finalize `canceled`, attempt and events; no later success accepted |
| Cancel after successful terminal commit | already terminal | defined no-op/conflict response; no rollback of success |
| Manual retry terminal failed job | authorized operator, eligible policy | create **new linked job** (`retried_from_job_id`) with its own attempt budget; old terminal record untouched |
| Duplicate/stale result | wrong attempt, generation, incarnation, expired lease, revoked credentials or terminal job | reject authoritative change; record bounded security/diagnostic event |

All state changes touching jobs, attempts and durable job events MUST happen in a **single short PostgreSQL transaction**. Require DB constraints for status, attempt numbering, foreign keys, idempotency uniqueness and current-lease ownership; use CAS predicates (`job_id`, current attempt, generation, allowed status, lease expiry, deadline, cancellation policy) with `UPDATE ... RETURNING` and check affected row count. `FOR UPDATE SKIP LOCKED` may reduce queue contention but provides an inconsistent view and **does not promise strict global FIFO**. Document isolation and starvation limits. Race tests and failure injection must demonstrate invariants under conflicting operations and partially failed transactions.

**Time and execution:** database clock is authoritative for due/expiry comparisons. Suggested **configuration starting values, not verified universal defaults**: worker liveness heartbeat 10 s, attempt lease 30 s, worker renew before half lease life, scheduler due/recovery loop 1 s with bounded polling jitter, default attempt timeout 10 min, cancellation grace 15 s. Define permitted bounds and timeout/heartbeat margins through ADR and tests. Hard deadline limits **authorization**, not an uncooperative goroutine's physical execution. Worker handlers must accept `context.Context`, honor cancellation and use bounded IO. An uncooperative worker/container may require operator termination and attempts may overlap externally during expiry; document process-level isolation and idempotent side-effect patterns. **Do not claim forced task termination unless an actually isolated child process/container is stopped and tests prove the behavior.** Never claim a context deadline forcibly kills a Go handler.

**Retry:** `max_attempts` includes first run (proposed default 3, accepted 1–5), error classified retryable/permanent with bounded exponential backoff + jitter, no invisible fourth attempt. Attempt record unique `(job_id, attempt_number)`. Retry after timeout or crash consumes an attempt; no automatic retry once budget exhausted. Permanent failure terminal. Manual retry terminal `failed` only, creating a linked new job with explicit lineage, new idempotency scope, and audited operator action. Reject accidental duplicate manual retry using an idempotent action key.

**Scheduling:** priority 0–9 (9 highest), due time and capability/version matching first; deterministic tie-breaking by `(priority DESC, available_at ASC, created_at ASC, job_id ASC)` **among eligible claimable rows**. No claim before due. No capable worker ⇒ job stays visible with an actionable `unschedulable` reason/alert, not silent disappearance or a fictitious job state. Monitor starvation; fairness beyond documented eligibility is not promised. Worker capacity controls number of claims; draining stops new claims, allows current handlers to finish until a documented grace period and then invokes the normal cancellation/lease recovery policy. A crashed/drained worker must not leave jobs permanently `running`.

**Submission/idempotency:** typed handler name+schema version, bounded JSON (initial maximum **64 KiB** payload, **128 KiB** result), priority, `run_after` (bounded future), timeout and retry policy. Initial suggested idempotency retention 24 h scoped to principal + named namespace + key and canonical fingerprint of *all semantically relevant submission fields*; identical retries return same job; same key with differing fingerprint returns HTTP 409. Define exact response/body, expiry/cleanup and concurrency index. Validate safe numeric/time ranges and UTF-8/JSON limits; make result/log over-limit failure or truncation behavior explicit (never leak sensitive content). Do not allow task types to accept shell commands, arbitrary filesystem paths or unrestricted internal/external URL fetches.

## 6. Persistence, data lifecycle, and contracts

**Core tables:** `users`, `sessions`, `api_tokens`, `worker_credentials`, `workers`, `jobs`, `job_attempts`, `job_events`, `audit_events`, optional aggregate metric tables and `submission_idempotency`. Model key fields, indexes, FK constraints, unique indexes, `jobs.version`, attempt lease generation, timestamps and bounded textual output. Use UUID/sortable IDs consistently. Store UTC, display user-local time with explicit zone; English-only initial UI. Document user privacy, token hashing, payload redaction and a least-privilege DB role.

**Recommended indexing:** eligible queue `(status, priority DESC, available_at, created_at, id)` appropriate to actual plans; worker+incarnation lookup; jobs by type/status/created and `retried_from_job_id`; attempts by job+number; events by job+version; scoped idempotency uniqueness. Prove effective indexes with `EXPLAIN (ANALYZE, BUFFERS)` on seeded scale; adjust based on real query plans. Avoid an unbounded query or event stream.

**Retention baseline to finalize in ADR:** attempt logs 7 days, job metadata and normal history 30 days, privileged audit records 90 days, backups 14 days, metrics by cost/capacity. The figures are proposed defaults, not legal retention advice. Implement bounded cleanup, FK-safe deletion and expired-data UI, considering audit/tombstones and idempotency lifetimes. Use forward-only, versioned **expand/contract** migrations; back up before upgrades; rollback by tested recovery where schema rollback is unsafe. Define RPO/RTO objectives before release and report achieved results from a timed restore, not an assumed number. After DB outage do not acknowledge uncommitted jobs; HTTP readiness fails, worker claims pause, and processing resumes without data corruption on recovery.

**REST `/api/v1`** (auth, validation, status and typed error contract for every operation): `POST /jobs`, `GET /jobs`, `GET /jobs/{id}`, `GET /jobs/{id}/attempts`, `GET /jobs/{id}/events`, `POST /jobs/{id}/cancel`, `POST /jobs/{id}/retry`, `GET /workers`, `GET /workers/{id}`, `POST /workers/{id}/drain`, `GET /metrics/overview`, `GET /metrics/timeseries`, `GET /audit`, plus `/health/live`, `/health/ready` and protected `/metrics`. Implement cursor-paginated filters with stable sort/tie-break and documented consistency under concurrent inserts; bound page size and query time windows. All errors use `{ code, message, details?, request_id }`. Specify 400/401/403/404/409/413/422/429/503 behavior, retry headers where relevant, request-id propagation and OpenAPI examples. Generate the TS client from the published contract; check contract compatibility in CI.

**gRPC service:** Register (worker incarnation/capabilities/protocol), Claim (requested capacity and supported types/versions), RenewLease (job/attempt/generation/identity), ReportResult/ReportFailure, PollCancellation, Drain/Deregister, Heartbeat. Specify message size, deadlines, auth credentials, version compatibility, unknown handler rejection and idempotent result retries. A worker that loses gRPC access must never assume its lease is still authoritative. Clearly distinguish worker liveness heartbeat from per-attempt lease renewal. Publish an example Go handler SDK with registration, schema validation, context cancellation, typed retryable errors, structured logging and idempotency guidance. Use reproducible proto/codegen, not manually divergent contracts.

**Events/realtime:** Persist job events transactionally with a monotonic per-job `version` (or formally defined global offset). WebSocket messages are **hints**, not the durable event bus. On subscribe/reconnect/gap: fetch current API snapshot with version; reconcile charts, lists and details; switch to bounded polling when WebSocket is unavailable; mark stale and disconnected states visibly. Test dropped/reordered events and session expiry. Metrics are derived from DB events or well-defined aggregations, with disclosed freshness; Prometheus telemetry measures infrastructure, not transactional job truth. Limit metric label cardinality and retention.

## 7. Security, privacy, and supply chain — release blockers

Produce a STRIDE-style or equally systematic threat model for browser, BFF, direct API, gRPC, DB, workers, auth provider, public demo, CI/CD and deployment. Map applicable OWASP ASVS 5.0 controls, API authorization concerns, and the secure SDLC to tests. Defend against IDOR/BOLA, role escalation, CSRF, XSS/log injection, SSRF, token replay, session fixation, forged gRPC results, oversized inputs, denial of service, open redirects, unsafe deserialization, supply-chain substitution and secret disclosure.

- **Human sessions:** established OAuth authorization-code+PKCE/state flow; secure, HttpOnly, appropriate SameSite cookies; session regeneration/expiry/logout; strict callback redirect validation; protected mutating BFF endpoints with CSRF/origin protections; secure content/security headers and CSP compatible with app; no exposed OAuth tokens in browser scripts.
- **Authorization:** Go independently checks scopes and roles for every API operation, object ID and sensitive field. Operator can submit/cancel/retry/drain and manage credentials; viewer cannot mutate, see raw restricted payloads/results or access token material. Document precise route/field matrix and test direct API bypass.
- **Service boundary:** Next BFF sends short-lived signed audience-bound service assertions over TLS, not unauthenticated identity headers. Direct client API tokens have separate scope, revocation, hashing and audit. WebSocket authorization and origin checks use short-lived session-bound authorization; reconnect after logout is rejected.
- **Worker credentials:** issue through authenticated operator flow; scope to worker actions; store only hashes where applicable; rotate/revoke with auditable events. Verify credentials on claim/renew/report. Test revocation against concurrently running attempts and re-registration; state exact revocation enforcement bound.
- **Isolation:** trusted precompiled task handlers only, strict JSON schemas, least-privilege nonroot worker containers, read-only filesystem where feasible, bounded CPU/RAM/pids, no privileged mounts, controlled egress; test adversarial payloads. A sandbox for arbitrary third-party code is NOT a 1.0 feature.
- **Build/release:** least-privilege GitHub Actions tokens, pin third-party actions to full commit SHAs, dependency and secret scanning, license checks, SBOM, build provenance/signature if publishing binaries/images; lock dependencies, container image digests, protected environment secrets, no committed secrets. Track security advisories and perform regular updates.
- **Security gates:** all critical known exploitable findings block release; high findings block unless owner formally accepts a documented, time-limited exception with mitigation; never conceal scans or claim blanket compliance. Isolated demo must pass anonymous write/private-read denial tests.

## 8. Product architecture: ALL pages, states, and journeys

**Common functional requirements:** real links, tested navigation, permission checks, meaningful errors and recovery actions, responsive reflow, accessible status indicators, skeleton/empty/error/offline/expired/no-data states, mobile operation, unambiguous UTC/local timestamps, bounded loading and helpful documentation. For **each** route generate a formal contract (`purpose`, `persona`, `sections`, `data source`, `actions`, `RBAC/field scope`, `responsive behavior`, `interaction/motion`, `loading/empty/error/success/offline`, `accessibility`, `E2E IDs`). No feature is done because the URL renders alone.

### Public pages (v1.0)

| Route | Required sections and interactions |
|---|---|
| `/` | Branded nav+wordmark, editorial hero with accurate at-least-once proposition and CTA, original interactive Execution Atlas flow, three concrete capability pillars, step-by-step architecture, worker-failure/retry story, verified console preview, quickstart code copy, limitations/FAQ, GitHub/docs CTA and substantial footer |
| `/architecture` | Layered topology with BFF/Go/Postgres/workers boundaries, readable diagram + alt, job/lease path, tradeoffs, failure domains, limitations and source links |
| `/docs` | Documentation index, core concepts map, search/nav and guided first steps |
| `/docs/quickstart` | Environment prerequisites, `docker compose` and bootstrap commands verified from clean checkout, first job, worker scale, recovery demo, expected results and troubleshooting |
| `/docs/concepts` | Job states, attempt versus job, leases, fencing, at-least-once, due jobs, priorities, cancellation and idempotency |
| `/docs/api` | Generated/versioned endpoint reference, auth/errors, payload examples, request IDs and API version policy |
| `/docs/workers` | Go SDK install, handler registration/versioning, error types, timeouts, credentials, local test examples |
| `/docs/reliability` | Retry budgets, recovery timelines, side-effect caveats, failure modes, guarantees and non-guarantees |
| `/docs/deployment` | Single-VM reference, TLS, secrets, compose, networking, backups, restore, upgrades, RPO/RTO and incident links |
| `/docs/security` | Authentication/roles, worker trust, demo separation, threat-model summary, reporting policy |
| `/changelog` | Actual shipped releases and migration notes; no fictitious versions |
| `/login` | GitHub sign-in, approval/no-entitlement state, auth error and logout/session outcomes; no unapproved default accounts |
| `/demo` | Clearly watermarked/declared **isolated fixture-based read-only** console tour; no production access and no misleading live badges |

### Authenticated operational pages (v1.0)

| Route | Required sections and interactions |
|---|---|
| `/app` | Summary KPIs (queue age/depth, running/succeeded/failed, workers), live/reconnecting indicator, topical alerts, recent jobs, truthful throughput chart, accessible worker topology and operational CTAs |
| `/app/jobs` | Server-filtered/sorted/cursor-paginated table, status/priority/type/time filters, empty/error/stale states, direct detail links, failed/dead-letter filter, no fake bulk controls |
| `/app/jobs/new` | Allowlisted handler select, schema/version and JSON editor with validation, scheduling/timezone, priority, attempts/timeout, permission checks, submit/pending/success/failure and duplicate handling |
| `/app/jobs/[id]` | Status, timestamps, payload display subject to RBAC, attempts, lease/retry/cancel state, bounded logs/results, versioned event timeline, copy ID, authorized cancel/manual retry and terminal lineage |
| `/app/workers` | Worker inventory, liveness, capability/version, capacity, drain state, realtime/last-seen, accessible topology/list toggle, filters and degraded/no-worker states |
| `/app/workers/[id]` | Identity/incarnation, permitted metadata, capacity, active attempts, heartbeat age, logs/events, drain confirmation, disconnected/revoked states |
| `/app/observability` | Charted throughput, queue age, attempts, failures, duration, time range, aggregation/freshness notice, graph data table/download if implemented; accessible text alternatives |
| `/app/activity` | Paginated job/audit event chronology, actor, outcome and correlation ID, RBAC-filtered sensitive detail |
| `/app/settings` | Environment/read-only operational config, documented limits and links to role-appropriate settings |
| `/app/settings/tokens` | Operator-only scoped token issuance/revocation/rotation with one-time secret display and audit; no accidental re-display |
| `/app/settings/preferences` | User-local timezone preference, reduced-motion/system preference respect, density preference only if implemented, session actions |

**1.1 gated:** `/app/workflows`, `/app/workflows/[id]`, `/app/workflow-runs/[id]`—only accessible after versioned DAG engine and real API/tests ship. Do not make unusable “coming soon” functionality part of the 1.0 navigation. Include 401/403/404/500, session-expiry, too-many-rows, no-capable-worker, no-metrics, data expired, mobile menu and screen-reader flows throughout.

### Shared UI component inventory

Brand shell/navigation/footer; docs sidebar/search/on-this-page; app sidebar/topbar/breadcrumbs/command search; form inputs, textarea/JSON editor, select/date-time picker, sliders only if justified, filter chips/tabs; buttons with loading/success/disabled/focus; tooltips/popovers/dialogs/toasts/drawers; status badges and legends; accessible tables/virtualized lists/pagination; charts and accessible data alternatives; topology graph and list mode; job timeline/attempt cards; token one-time reveal; code blocks/copy snippets; skeletons/empty/error/offline/reconnect states; responsive and reduced-motion variants. **Every control must actually work** or be omitted.

## 9. Distinctive creative direction: white-first **Precision Under Motion**

Avoid generic dashboard templates and dark cyberpunk tropes. Build a cohesive, editorial and technical identity: large precise typography, intentional section rhythm, generous whitespace, clean 1-pixel borders, original infrastructure illustrations, functional data visualization and understated depth. Benchmark craft against leading editorial/developer platforms **without copying** their graphics, layout, illustrations, wording or motion.

**Starting color tokens, subject to measured AA contrast:** `#FFFFFF` primary, `#F7F9FC` canvas, `#EEF2F7` subtle panels, `#111827` ink, `#475569` secondary, `#E2E8F0` borders, `#2457E7` cobalt accent, `#0C8A63` success, `#A96B08` warning, `#C33444` danger. Avoid status by color alone; use text/icons and accessible legends. Dark code blocks allowed as a deliberate contrast, not the entire visual theme. Use a premium well-supported sans and consistent mono for technical values; define fluid type scale, column grid, spacing, borders, shadows, icon stroke, card/table densities and chart encoding. Any typeface/image/illustration assets must have usable licenses and responsive exports.

**Execution Atlas:** own an original diagram system for `job`, `queue`, `attempt`, `lease`, `worker`, `recovery`, and `result` with standardized shapes, connectors, directed arrows, statuses, labels and time context. The interactive marketing Atlas can be deterministic and **labeled illustrative**; console topology is backed by actual data. Graph at 1, 10 and 100 workers must support zoom/pan, automatic clustering/aggregation, detail focus and accessible synchronized table/list view. Reconnection must not create a visually convincing but stale diagram.

**Motion specification** (document exact matrix per component; following ranges are starting budgets): hero reveal 450–650 ms, editorial section entrances 180–350 ms, hover/focus 120–180 ms, popovers/dialogs 160–240 ms, route changes 150–250 ms. Use transform/opacity and meaningful state changes, controlled stagger and easing; graph pulses only when a job is active. Define trigger, initial/end state, delay, duration, ease, interruption, reversal, repeated-mount behavior, scroll-up/down behavior, viewport/visibility behavior and `prefers-reduced-motion` fallback. Avoid scroll hijacking, excessive animation libraries, layout thrash and endless status loops; critical console controls must not wait for an animation.

**Design artifacts before major frontend work:** information architecture, desktop/mobile wireframes for all route families, high-fidelity hero/overview/jobs/detail/worker states, design tokens, reusable component specs, responsive variants, states matrix, icon/vector asset inventory, Execution Atlas node/edge grammar, animation storyboard/matrix and accessibility audit plan. Challenge aesthetic originality and practical clarity independently. Screenshots and demo animation must match what actually ships.

## 10. Accessibility, usability, performance, browser support

**WCAG 2.2 AA** is the release target: semantic landmarks, proper headings, native controls, label/error relationships, focus order and visibility, keyboard navigation including graph alternative, modal trap/restoration, status announcements without overload, 4.5:1 normal-text and 3:1 applicable non-text contrast, no color-only status, accessible legends and chart tables, target sizes where applicable, reduced motion, 200% zoom and **equivalent 320 CSS-pixel reflow (often tested at 400% desktop zoom)** except legitimate two-dimensional-content exceptions. Manual NVDA/VoiceOver + keyboard walkthroughs complement axe; no claim of conformance without criterion-by-criterion proof.

Test widths **320, 360, 390, 768, 1024, 1440, 1920 CSS px**, dense/empty/error screens, long names, RTL is not a 1.0 requirement, and supported current versions of Chrome, Firefox, Safari and Edge. Define an exact supported browser matrix before release. English-only 1.0; UTC storage, user-local display with timezone and DST tests. For mobile console, preserve critical submit, inspect, cancel/retry and worker detail functions; collapse complex graphs into accessible summaries/lists rather than hiding essential data.

**Public performance targets:** p75 real-user LCP ≤2.5 s, INP ≤200 ms, CLS ≤0.1 **where sufficient field data exists**; otherwise report lab/DevTools results and mark field metrics unavailable. Enforce responsive images, font strategy, code splitting, selective hydration, fast server response and reduced JS. **Console:** measure p95 job-table filter/sort response, route transitions, topology interaction and live-update staleness against benchmarks; freeze numeric budgets after baseline, before RC, with hardware/test fixture and pass/fail evidence. Avoid broad context rerenders, unbounded DOMs and hot-loop WebSocket updates.

## 11. Testing, evidence, and measurable acceptance criteria

Implement tests **with** features, not only after them. Use Go table-driven/unit/property tests, `-race`, DB integration against real PostgreSQL, API and protobuf contract tests, worker SDK examples, browser Playwright, axe + manual accessibility, visual regression, dependency and SAST/secret scans, failure injection, deterministic clock/failpoint tests, capacity testing, and deployment/restore rehearsals.

**Mandatory release acceptance IDs:**

| ID | Executable criterion |
|---|---|
| AC-01 | Job submission receives 2xx **only after commit**; acknowledged job survives coordinator restart under documented PostgreSQL durability settings |
| AC-02 | ≥20 simultaneous claimants cannot establish >1 **currently authorized** lease per job; transaction failpoints leave no partial job/attempt/event writes |
| AC-03 | Worker A expires, B claims/completes, then A submits stale success: A rejected, authoritative final result preserved, history coherent |
| AC-04 | Kill worker mid-handler; expired attempt becomes retryable or terminal per policy, never indefinitely stuck `running` |
| AC-05 | `max_attempts=3` yields exactly 3 automatic attempts and no hidden fourth; bounded jitter/backoff reproducible with injected clock |
| AC-06 | Success versus cancel races yield exactly one valid final outcome under documented commit-order precedence; late callbacks denied |
| AC-07 | Repeated identical scoped idempotency submissions yield same job; different body same active key yields 409; expiration tested |
| AC-08 | DB outage/coordinator restart never acknowledges uncommitted work; recovery resumes scheduled/expired jobs without corrupt state |
| AC-09 | Worker heartbeat, attempt renewal, revoked identity and process-incarnation mismatches cannot grant forbidden claim/renew/report |
| AC-10 | Capability mismatch/unavailable handler leaves accounted visible unschedulable job; priority 9>0 among eligible rows and due time honored |
| AC-11 | Worker drain stops claims; active attempts finish or recover by documented timeout; graceful SIGTERM behavior tested |
| AC-12 | Bounded input/output/logs and overload replies protect DB/worker memory/CPU/disk; adversarial payloads cannot shell/SSRF/escape |
| AC-13 | Operator succeeds; viewer mutation, sensitive field reads and direct API bypass are rejected; CSRF/session/websocket tests pass |
| AC-14 | Anonymous `/demo` cannot read/write production API/data/credentials; seeded/illustrative content is accurately labeled |
| AC-15 | Console create/list/detail/cancel/retry/drain charts show real persisted state; API errors/stale/denied/no-worker cases actionable |
| AC-16 | Dropped/reordered/disconnected live messages reconcile from HTTP snapshots; no permanently stale dashboard |
| AC-17 | Migration from populated DB and tested backup/restore preserve invariants; timestamped RPO/RTO drill is reported |
| AC-18 | 1,000-job test with 3 workers reconciles submitted, running, terminal, attempts and retry counts exactly; show raw metrics, not guessed throughput |
| AC-19 | 1/3/5-worker scaling test repeats fixed workload and environment; report p50/p95/p99, resource use, DB contention and limits, not unsupported linear scale |
| AC-20 | API/SDK/codegen, worker/Go protocol compatibility and versioned schema tests pass in CI |
| AC-21 | All required public/app routes and controls have real navigation, meaningful state handling and accessible keyboard journeys |
| AC-22 | WCAG 2.2 AA checklist plus manual screen reader, reduced motion, 320 CSS-px reflow and focus tests have evidence |
| AC-23 | Responsive screenshots at agreed sizes match approved design intent; all graph functionality has alternative text/list access |
| AC-24 | Public Web Vitals measured correctly; console interaction/staleness thresholds and bundle checks pass under documented conditions |
| AC-25 | Security threat model, access matrix, dependency/supply chain checks and secret-redaction canaries pass; critical findings unresolved = release block |
| AC-26 | Clean checkout supported-host install + first job + 3 workers + failure/recovery + API/docs succeeds using published commands |
| AC-27 | TLS, auth bootstrap, container isolation, alerts/runbooks and DB backup/restore exercise verified on reference host |
| AC-28 | README links, screenshots, source license, CI results, release notes, documentation and public technical claims match shipped code |
| AC-29 | v1.1 routes absent/feature-gated and no v1.0 marketing claims imply unimplemented DAG workflows or HA |
| AC-30 | All required FR/NFR/route IDs map to implementation, automated/manual tests, actual evidence and recorded final disposition |
| MW-01 | Each agent task has one current file owner, a unique branch/ticket and an approved contract dependency |
| MW-02 | Contract change updates source schema, generated client, examples, backend/consumer tests and versioned compatibility notes in one reviewed integration |
| MW-03 | Parallel worktrees merge without uncontrolled shared-file edits; reviewer is separate from code author |
| MW-04 | Session logs record actual model/effort/Ultracode capability or explicitly document fallback; no unsupported setting is claimed active |
| MW-05 | Human approval is evidenced before external costs, production credentials, publication, destructive Git operations or acceptance of high-risk security exceptions |

**Gates:** For each security finding, track exploitability, affected surface, owner, remediation, retest, and any expiring high-risk exception requiring explicit human approval. Critical exploitable findings cannot be waived by an AI agent. PR checks: format, lint, strict types, unit, database integration, API contract, basic auth, tests for touched functionality, secret scans. Release checks: race + concurrency, chaos/recovery, security, complete E2E, a11y manual audit, screenshot QA, measured performance, deployment and restore exercise, SBOM/license, release docs. Nightly tests may supplement but must not replace a required release gate. Record environments, exact commands, counts, failing cases and **NOT RUN**. No skipped critical tests; exceptions must be approved and scoped explicitly. Establish quantitative backend queue/admission/recovery/CPU/RAM thresholds after baseline and freeze them before RC; if not met, report blocker, not fake success.

## 12. Deployment, operations, upgrade, and maintenance

**Canonical reference:** one Linux **amd64** VM (test arm64 separately before claiming support), TLS reverse proxy (e.g., Caddy), Next.js app, one always-on Go coordinator, PostgreSQL persistent named volume/managed storage, N trusted worker containers, protected internal monitoring. Local macOS/Windows may be supported for **development through Docker** after verification; don't claim native production support without tests. Provide minimal and recommended RAM/vCPU/disk after measurement; publish exposed ports, firewall rules, private networks, health/readiness, startup ordering, container nonroot privileges and secrets flow. No assumption that a serverless web host runs long-lived Go/gRPC worker services.

Provide a working, reproducible `docker compose` local starter and a production example with HTTPS and secure secrets. Separate local-demo config from production. Provide `.env.example` **without credentials**, production validation that fails closed, migrations, backup commands, restore to new DB, upgrade/compatibility sequence, rollback/restore runbook and disk retention monitoring. Define SLOs/RPO/RTO prior to RC; report measured achieved values. Demonstrate actionable alerts on DB unavailable, queue age, worker offline, repeated lease expiries, failure rate and disk exhaustion; link runbooks, not just charts.

Maintain monthly dependency/security review, periodic upgrade/recovery rehearsal, release version policy and protocol compatibility matrix, responsible security reporting, repository issue templates, contributor guidance and actual changelog. Introduce a new queue, extra services, object storage, HA, autoscaling or AI only on evidence and as separately planned scope. Long-term sustainability means compatibility and operability, not perpetual use of the latest experimental stack.

## 13. Monorepo, documentation, and delivery artifacts

Suggested repository:

```text
Clusterx/
  apps/
    web/                    # public site, docs, authenticated console, read-only demo
    control-plane/          # Go API, gRPC, scheduler, event hub, migrations
    worker/                 # trusted worker reference executable
  packages/
    proto/                  # generated contracts / versioning
    sdk-go/                 # handler registration, claim, lease, result
    sdk-ts/                 # generated typed REST client
    ui/                     # tokens, accessible reusable components
  examples/
    basic-jobs/
    worker-recovery/
  infra/
    compose/
    proxy/
    monitoring/
  docs/
    product/requirements.md  product/journeys.md  product/traceability.md
    architecture/overview.md  architecture/state-machine.md
    architecture/api-contracts.md  architecture/dependency-matrix.md
    architecture/adrs/
    design/route-inventory.md  design/brand-and-tokens.md
    design/motion-and-assets.md  design/execution-atlas.md
    security/threat-model.md  security/permissions.md  security/verification.md
    testing/test-matrix.md  testing/benchmarks.md  testing/release-gates.md
    operations/deployment.md  operations/backup-restore.md
    operations/upgrades.md  operations/incident-runbook.md
  .github/workflows/
  README.md  CONTRIBUTING.md  CODE_OF_CONDUCT.md
  SECURITY.md  LICENSE  CHANGELOG.md  .env.example
```

Use idiomatic Go workspaces/modules, strict TS and clear ownership of generated files. Required source-control rules: no secrets, consistent formatting, dependency locks, minimal meaningful comments, reproducible generated contracts, safe default branch protection and reviewable commits. No empty directories or TODO placeholders masquerading as implemented features. **Owner selects MIT or Apache-2.0 (or another compatible license) before publication**; never silently assert license authority.

**Canonical workflow artifact rules:** Keep `CLAUDE.md` to a compact pointer to this master and its owner/permission policy; agent profile frontmatter may set model/effort but never redefine scope. Tickets/handoffs are execution logs, not alternate master prompts. Maintain contract version in every ticket, and serialize shared-file merges. **Before major coding, create and review:** `docs/product/requirements.md` (each FR/NFR with priority and release), `docs/product/journeys.md`, `docs/product/traceability.md`, ADRs, complete state/event matrix and schema/DDL/invariants, OpenAPI/protobuf contract drafts, auth trust boundary diagram, threat model, design route contracts, responsive high-fidelity examples, motion/assets matrix, platform/version matrix, benchmark plan, CI matrix and deployment architecture. These are gates, not endless speculative documentation; use focused throwaway technical PoCs to validate high-risk SQL/worker behavior.

## 14. Milestone order and exit gates

| Stage | Deliverable | Required exit evidence |
|---|---|---|
| **M0 discovery** | Repo/environment audit, personas, scenarios, v1/v1.1 scope, owner decisions, traceability and risk register | Scope/priority/assumption ledger, no unclassified requirements |
| **M1 architecture** | ADRs, state/lease/cancel/retry contracts, DB constraints, API/proto, auth boundaries, race proof | Core invariants review; working focused contention/timeout experiments where needed |
| **M2 design** | All route contracts, approved white-first assets/screens, Execution Atlas grammar, component/motion/a11y matrix | Design review; each route/state/permission mapped |
| **M3 foundation** | Monorepo, Docker, migrations, config, CI, generated contracts, health | Clean boot, lint, typecheck, build, DB smoke tests |
| **M4 engine** | Durable admission, atomic worker claims, registry, SDK, execution results | Real multi-worker/DB integration passes |
| **M5 reliability/security** | Leases, fencing, retry, cancellation, drain, auth/roles, audit | Race/chaos/security scenarios pass |
| **M6 console** | All v1 operational pages, real data, WebSocket+snapshot, accessible topology | E2E tasks and permission/error paths pass |
| **M7 public craft** | Original marketing, docs, illustrations, refined motion, safe demo | All pages functional, visual/a11y/perf review passed |
| **M8 operations** | Monitoring, performance, security, load, backup/restore, VM deploy/upgrade rehearsal | Measured benchmark; blocking test matrix and documented limitations |
| **M9 release** | README, screenshots/video, license, examples, tags, CHANGELOG, architecture, traceability | Independent clean-checkout reviewer can reproduce demonstration |
| **M10 separate 1.1** | DAG definitions, validation, execution, persistence, graph editor | DAG failure/recovery and accessible graph tests pass before 1.1 claims |

At every stage report **(a)** implemented files/changes, **(b)** tests run with exact results, **(c)** requirement IDs moved to what statuses, **(d)** risks and unresolved decisions, **(e)** screenshots/prototypes if relevant, **(f)** next entry gate. Refuse to label a P0 unresolved architecture as approved; do not stall harmless planning for decisions that don't affect it. For a scope change, document impact, tests and owner approval.

## 15. Final independent audit and response format

Before v1.0 release, re-audit **product fit; all routes/sections/controls; genuine worker execution; concurrency invariants; data lifecycle; permissions; browser/BFF/API/gRPC trust; demo isolation; accessibility; performance; image/illustration rights; responsiveness; motion/reduced motion; deployment; upgrade/recovery; CI; supply chain; documentation; license; GitHub clarity; future compatibility**. Challenge the design and architecture for unnecessary complexity. Confirm actual evidence; repair blockers and rerun affected tests. Do not invent approval.

Return a concise structured status at each gate and completion:

1. **Implemented:** modules, behavior, routes, requirement IDs and changed file paths.
2. **Verified:** exact automated/manual test commands and outcomes, screenshots, security/a11y reports, load raw data, deployment/restore proof.
3. **Decisions:** approved ADRs, compatibility matrix, rationale for any deviation.
4. **Remaining:** deferred/experimental/blocked/NOT RUN, known bugs and limitations.
5. **Usage:** local quickstart, sample job, worker scaling, troubleshooting and cleanup.
6. **Release readiness:** explicit GO/NO-GO with failing gates and owner approvals; links/URLs only if actually built or deployed.

**One final quality rule:** The outcome must be a **real Go distributed job platform with an original white-first polished web experience**, not a collection of impressive-looking untested artifacts. Strong instructions are necessary but insufficient: demonstrate every consequential guarantee under test.

## 16. Owner decision register, experimental gates, and limits

**Needs explicit owner approval before affected live use/publication:** open-source license, production GitHub OAuth authorized identities/first operator, domain/hosting provider and budget, use of external OAuth/monitoring services, any deployment/publishing/credentials. If not approved, continue fully local work with safe defaults and mark the external release blocked. **Never provision a cost or expose a live endpoint by inference.**

**Must be proven before freezing associated implementation:** exact `Go/Next/React/TypeScript/Tailwind` combination; SQL contention/index/invariants; heartbeat/lease/timeout values; safe handler cancellation behavior; protobuf and OpenAPI generator support; protocol-version skew; DB restore; worker revocation behavior; public-demo isolation; accessibility; UI latency; release capacity/SLO/RPO/RTO. These are requirements, not currently passing tests.

**Known project limitations to communicate:** at-least-once execution may repeat external side effects; stale fencing protects authoritative DB writes but cannot unrun a handler; single coordinator has downtime; Postgres-based priority scheduling has eligibility/fairness limits; hard deadlines aren't arbitrary goroutine-kill; JSON result/log limits and retention expire information; public demo is illustrative/isolated, not live telemetry; v1.1 workflows aren't in 1.0; no assured performance numbers until benchmarks. These limits are acceptable only when disclosed and tested to the promised scope.

**Finalization result:** This is a **consolidated, actionable implementation specification**, not a claim that the repository has passed tests or that all owner-dependent conditions are resolved. Implementation and release are permitted only through the stated gates. Do not promise zero defects or absolute future-proofing.

## Official references to verify again during implementation

- https://go.dev/doc/devel/release
- https://www.postgresql.org/about/news/
- https://www.postgresql.org/docs/18/sql-select.html
- https://nodejs.org/en/about/previous-releases
- https://nextjs.org/support-policy
- https://nextjs.org/docs/app/guides/upgrading/version-16
- https://react.dev/versions
- https://www.typescriptlang.org/
- https://tailwindcss.com/blog
- https://motion.dev/changelog
- https://spec.openapis.org/oas/latest.html
- https://asvs.owasp.org/
- https://www.w3.org/TR/WCAG22/
- https://datatracker.ietf.org/doc/rfc9700/
- https://datatracker.ietf.org/doc/rfc10017/
- https://csrc.nist.gov/projects/ssdf/publications
- https://docs.github.com/en/actions/reference/security/secure-use
- https://web.dev/articles/vitals

## Appendix A — Task and handoff contract (copy into templates)

**TASK:** ID, phase, FR/NFR/AC IDs, outcome, customer impact, owner/model/effort, approved contract commit, dependencies, allowed paths, forbidden paths, acceptance tests, security/privacy effects, expected artifact, reviewer, priority/estimated cost envelope, blocked decisions.

**HANDOFF:** task ID, author, actual model/effort, branch+commit, changed files, contract SHA, generated files, exact commands + exit codes, PASS/FAIL/NOT RUN, evidence paths, screenshots or API traces, limitation/regression risk, next owner, reviewer notes, traceability status, approval needs.

**Change request:** source contract revision, compatibility impact, all affected consumer/backend work, needed migration/backfill, E2E matrix, rollout order, approval and rollback plan.

**Release gate:** one human owner decides GO/NO-GO from documented CI evidence, threat assessment, accessibility tests, deployment/restore drill, defect list, licensing and publicly defensible claims.

## Appendix B — Multi-model verified reference links

- https://platform.claude.com/docs/en/models/overview
- https://code.claude.com/docs/en/model-config
- https://code.claude.com/docs/en/settings-reference
- https://code.claude.com/docs/en/sub-agents
- https://code.claude.com/docs/en/common-workflows
- https://code.claude.com/docs/en/agent-teams
