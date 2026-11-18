# TASK-M4-BE-01 — Engine: durable admission, atomic claims, registry, SDK, results

- Milestone: M4 engine (Alpha). Requirement IDs: FR-ENG-01..09, 17, 18, 20, 21; FR-API-01 (job/worker read+submit ops), FR-API-02; FR-SEC-03 (API tokens), FR-SEC-05 (worker credential verification on claim/renew/report); AC-01, AC-02, AC-07, AC-10, AC-18 (in-process), AC-20.
- Includes **CR-M4-01** (on main) in full.
- Owner: backend lane `claude-opus-5-5`, effort medium (requested). Branch `m4/backend`, worktree `.worktrees/backend`.
- Contract base: `contract-m1-draft.4` + CR-M4-01 → produce `1.0.0-draft.5`. Temporary ownership of `api/openapi.yaml` for CR items only.
- Allowed: `apps/control-plane/**`, `apps/worker/**`, `packages/sdk-go/**`, `packages/proto/**` (no proto contract change), `api/openapi.yaml` (CR only), `docs/architecture/**`, `docs/security/**`, `examples/**`, `docs/coordination/handoffs/HANDOFF-M4-BE-01.md`.
- Forbidden: root config, `.github/**`, `infra/**`, `packages/sdk-ts/**` (integrator regenerates), `apps/web/**`, `docs/design/**`, `docs/product/**`.

## Deliverables

1. `internal/store`: production store promoted from PoC (keep PoC tests or port them), all transitions per state-machine.md, CAS + row-count checks, lock order (NF-01), `clock_timestamp()` stamps (NF-02).
2. Migration `0002_*.sql` per CR-M4-01 (NF-03/04/05); runtime vs migration DB roles in config.
3. REST `/api/v1` (net/http, no framework): `POST /jobs` (validation, 64 KiB bound, idempotency scope + fingerprint, 201/200 `Idempotent-Replayed`/409), `GET /jobs` (cursor pagination, filters, sort per draft.4), `GET /jobs/{id}`, `/attempts`, `/events`, `GET /workers`, `GET /workers/{id}`, `GET /handlers`, `GET /config`; typed Error envelope with `request_id`; 410 via tombstones; viewer masking applied for token roles. Cancel/retry/drain/tokens/sessions/stream are M5 — return nothing (no route) rather than fake.
4. Auth for M4: API tokens (hashed, scoped, `api_token_authority` view) + server-side bootstrap command `clusterx-control-plane token create --role operator --name ...` printing the secret once. Worker credential issuance via same CLI for now (`worker-credential create`). No default credentials.
5. gRPC: Register, Heartbeat, Claim (capability/version/capacity, priority + due, `unschedulable_reason` computed), RenewLease, ReportResult, ReportFailure (retryable/permanent, backoff+jitter with injectable clock), PollCancellation (returns current flag), Deregister; credential + incarnation verified on every call; message size limits/deadlines per proto docs.
6. Scheduler loop: due promotion (`scheduled`/`retry_wait`→`queued`), lease-expiry recovery (retry/fail/cancel per budget), bounded jitter; DB clock only.
7. Worker binary + `packages/sdk-go` transport: registration, claim loop bounded by capacity, renew at half lease, report, ctx cancellation, SIGTERM drains local handlers within grace. Example handlers in `examples/basic-jobs`: deterministic bounded `sleep_ms`, `hash_sha256` (bounded input), `fail_n_times`; no shell/URL/filesystem.
8. Tests: unit + PG integration; **in-process multi-worker integration test**: control plane + 3 worker processes/goroutine-isolated clients over real gRPC, 1,000 jobs, reconcile submitted = terminal, attempts counts exact (AC-18 in-process); AC-01 (2xx only after commit; restart survives); AC-02 service-level (≥20 concurrent claimants via gRPC); AC-07; AC-10; worker killed mid-job → recovery (AC-04 preview).

## Acceptance

All modules `go vet` + `go test -race -count=1 ./...` green on PG18; mutant evidence for NF-06; redocly lint 0 errors on draft.5; docker images still build; handoff lists exact commands/exit codes, the bootstrap commands, and a curl guide that was actually executed against a running binary.
Reviewer: critical review in M5 security pass covers M4 code.
