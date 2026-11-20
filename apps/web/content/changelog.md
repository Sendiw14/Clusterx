## Unreleased (pre-1.0)

No version of CLUSTERX has been released. Nothing below is a release, a version number or a promise of a date. This is the list of milestones that have been merged into the main branch, taken from the repository history. Dates are UTC and are the commit dates.

> [!NOTE] The tag `contract-m1-draft.4` marks an internal contract snapshot. It is not a release.

### M6 review fixes (merged 2026-11-20, commit 936bd59)

- The server-side proxy accepts only an allowlist of the API routes the console uses, request bodies are capped at 1 MiB while they are read, and the stream relay releases its sockets and reserves its slot before it dials.
- The console, sign-in and BFF auth routes use a per-request nonce Content-Security-Policy without `'unsafe-inline'` for scripts.
- The browser session cookie no longer holds the session secret and supports key rotation. Signing in again and signing out revoke the previous session. A production web server refuses a non-HTTPS control plane URL unless it is on loopback.
- A second independent review round passed the M6 exit and left release items (see below).

### M6: operator console (merged 2026-11-19, commit 0a6642b)

- The web console on live data: jobs list, detail, submit, cancel and retry, workers and drain, observability, activity, tokens and worker credentials, settings, and live updates that fall back to polling.
- The Next.js server: GitHub sign-in (OAuth with PKCE), sealed session cookie, CSRF protection, signed service assertions, a server-side proxy and a WebSocket relay.
- Live end-to-end tests against the real control plane and worker, with a mock OAuth server standing in for GitHub.

### M5: sessions, mutations and reliability (merged 2026-11-19, commit 03d774e)

- Browser sessions in the control plane, the mutating API (cancel, retry, drain, tokens and worker credentials), audit events, metrics, rate limits and a WebSocket stream of change hints.
- Review fixes: database connection loss is reported as unavailable instead of an internal error; a drained worker stays drained until it deregisters (see [Worker SDK](/docs/workers)); strict assertion parsing; idempotency and retention fixes. Migrations 0003 and 0004. OpenAPI `1.0.0-draft.7`.
- Local Docker Compose: secrets as files under `.secrets/`, a development key pair for the web server, an optional retention service, configurable ports and image prefix.

### M7: public site, documentation and demo (merged 2026-11-19, commit e5903a7)

- The public site, the architecture page, the documentation with a generated API reference and a build-time search, this changelog, and the isolated demo on fixture data.

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

None. There is no released version to migrate from, so there are no upgrade steps and no compatibility promise. Database migrations are forward-only and numbered; migrations 0001 to 0004 exist.

## Not yet in this history

- Real GitHub sign-in, TLS and production-mode cookies have not been run; the web console has only been tested against a mock OAuth server.
- The release items the second review round left open (the console and sign-in pages loaded from a public page under the looser policy, no CSP on letter-case variants of `/app`, `/login` and `/auth`, a stale role hint in the session cookie, a half-closed stream client holding a slot) are fixed in the M9 web hardening change. That change has had no independent re-review yet.
- Production deployment, backup and restore, and benchmarks (M8) are not done.
- Workflow definitions are not part of 1.0.
