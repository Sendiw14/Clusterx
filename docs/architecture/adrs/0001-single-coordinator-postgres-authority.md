# ADR-0001 — Single modular Go coordinator; PostgreSQL is the only authority

- Status: **Proposed** (TASK-M1-BE-01, 2026-11-17). Approval: owner + TASK-M1-RV-01 review.
- Requirements: FR-ENG-01, FR-ENG-04, FR-ENG-05, FR-ENG-22; AC-01, AC-02, AC-08. Master §4, §5, §6, §16.

## Context
The master requires one modular Go control plane, PostgreSQL-owned state, worker-pull gRPC, and
forbids Kafka/extra queues/microservices or a second source of truth.

## Decision
1. **One process** `clusterx-coordinator` (`apps/control-plane`) with internal packages:
   `api` (REST `/api/v1`), `workerrpc` (gRPC), `store` (pgx v5, hand-written SQL, no ORM),
   `scheduler` (due promotion + lease/deadline recovery loop), `auth`, `events` (WebSocket hint hub).
2. **PostgreSQL 18 is the only authoritative store** for jobs, attempts, leases, events, idempotency,
   workers, credentials, sessions, tokens and audit. In-memory state (WebSocket fan-out, caches) is
   derivable and disposable. No message broker, no Redis, no second queue.
3. **Workers pull** over gRPC; they never connect to PostgreSQL.
4. Scheduler loops run inside the coordinator, every 1 s ± 20 % jitter, each batch in its own short
   transaction with `FOR UPDATE SKIP LOCKED LIMIT 500`. They are safe to run concurrently (SKIP LOCKED +
   CAS), so a future second coordinator would not corrupt state — but **HA is not a 1.0 claim**.
5. Admission returns 2xx **only after COMMIT** of the job row + event (+ idempotency row).
6. DB access: pgx v5 pool, READ COMMITTED, statement timeout 5 s, transaction idle timeout 10 s,
   `synchronous_commit=on` (documented durability setting for AC-01).
7. DB outage: `/health/ready` fails, REST mutations return 503 `unavailable`, Claim returns
   UNAVAILABLE, scheduler loops back off; nothing is acknowledged that was not committed.

## Consequences
- Single coordinator = downtime during restarts/upgrades (disclosed limitation §16). Workers keep
  executing but their leases expire if the outage exceeds lease TTL; results are then fenced.
- `SKIP LOCKED` gives no strict global FIFO; priority order holds only among rows visible and
  unlocked at claim time; starvation of priority 0 under sustained priority-9 load is possible and
  monitored (R-07), not prevented.
- Throughput is bounded by PostgreSQL write capacity; numbers only after AC-19 benchmark.

## Alternatives rejected
Broker-based queue (Kafka/NATS/Redis): second source of truth. Workers with DB credentials: breaks
trust boundary (ADR-0005). Advisory locks for leases: invisible to constraints and lost on session drop.
