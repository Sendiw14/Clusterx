# CLUSTERX — Architecture overview (M1 draft)

Status: **specified**. Decisions: ADR-0001..0005. Master §4–§6.

- **One Go coordinator** (`apps/control-plane`): REST `/api/v1` (OpenAPI `api/openapi.yaml`), gRPC
  `clusterx.worker.v1.WorkerService` (`packages/proto`), scheduler loops (due promotion, lease/deadline
  recovery, retention), WebSocket hint hub, auth.
- **PostgreSQL 18** is the only authority (`apps/control-plane/migrations/0001_init.sql`).
- **Workers** pull via gRPC, run allowlisted precompiled handlers, never touch the DB.
- **Next.js BFF** owns human sessions and calls Go with signed short-lived assertions.

## Job path
`POST /jobs` → tx{insert job (`scheduled`|`queued`) + event `admitted` + idempotency row} → 201 after
commit → scheduler promotes due rows → worker `Claim` → tx{lock eligible rows `FOR UPDATE SKIP LOCKED`
in `(priority DESC, available_at, created_at, id)` order, insert attempt, job→`running`, event} →
worker renews lease, executes → `ReportResult` → tx{fenced CAS → `succeeded`, attempt finalized, event}.

## Failure domains
| Failure | Effect | Recovery |
|---|---|---|
| Worker crash | leases stop renewing | scheduler expires lease → retry_wait/failed/canceled |
| Worker partition | worker may keep executing | its reports are fenced after lease expiry |
| Coordinator restart | API/gRPC unavailable; nothing acknowledged uncommitted | leases may expire during outage; recovery on start |
| PostgreSQL down | readiness fails, 503, claims paused | resume from committed state |

## Documented limits
At-least-once only; fencing cannot unrun a handler; single coordinator (no HA in 1.0); SKIP LOCKED is
not global FIFO and low priority can starve; hard deadline is an authorization bound, not a kill.
