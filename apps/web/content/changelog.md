## Unreleased (pre-1.0)

No version of CLUSTERX has been released. Nothing below is a release, a version number or a promise of a date. This is the list of milestones that have been merged into the main branch, taken from the repository history. Dates are UTC and are the commit dates.

> [!NOTE] The tag `contract-m1-draft.4` marks an internal contract snapshot. It is not a release.

### M4: engine (merged 2026-11-18, commit 2633090)

- Durable job admission: a job is acknowledged only after its transaction commits. Idempotent submission with `Idempotency-Key`.
- Atomic claims with leases and fencing, attempt budget, retry with backoff, recovery of expired leases, due-job promotion, cooperative cancellation in the engine, worker drain and deregistration.
- Worker gRPC service, the Go worker SDK and a reference worker with three example handlers: `hash_sha256`, `sleep_ms` and `fail_n_times`.
- REST `/api/v1` subset served with direct API tokens: create job, list jobs, get job, list attempts, list events, list and get workers, list handlers, get config. API tokens and worker credentials with a bootstrap CLI. OpenAPI `1.0.0-draft.5`.
- Database migration 0002: least-privilege runtime role, delete and truncate guards on history, a log-expiry exception, and commit-time coupling between a running job and its active attempt.
- A local Docker Compose starter: PostgreSQL 18, one-shot migrate, control plane, three workers and `bootstrap.sh` (commit 7e399d0).

### M3: foundations (merged 2026-11-18, commits 50dd3a5 and 07869e0)

- Go foundation: control plane, worker and SDK skeletons, protobuf code generation, initial migration, health endpoints and container images.
- Web foundation: Next.js application, the `@clusterx/ui` component library, route shells for the public site and console, the data-provider seam, the isolated demo harness and a skeleton of the server-side proxy that answers 501. A continuous-integration job for web lint, type check, tests, build and end-to-end tests (commit ad54425).

### M2: design system (merged 2026-11-18, commit 7a523cf)

- Design tokens, information architecture, component specifications, the Execution Atlas grammar, the motion matrix, the accessibility plan and static prototypes.

### M1: contracts (merged 2026-11-18, commits c48b62f and 2ea9d0b)

- The job state machine, architecture decision records 0001 to 0006, the initial database schema, the worker protocol, OpenAPI `1.0.0-draft.4`, a race proof-of-concept, and the frontend route inventory.

## Migration notes

None. There is no released version to migrate from, so there are no upgrade steps and no compatibility promise. Database migrations are forward-only and numbered; migrations 0001 and 0002 exist.

## Not yet in this history

- Browser sessions, GitHub sign-in and the authenticated web console (milestones M5 and M6) are not merged.
- Production deployment, backup and restore, and benchmarks (M8) are not done.
- Workflow definitions are not part of 1.0.
