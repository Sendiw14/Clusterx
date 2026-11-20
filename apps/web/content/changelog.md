## Unreleased (pre-1.0)

No version of CLUSTERX has been released. Nothing below is a release, a version number or a promise of a date. This is the list of milestones that have been merged into the main branch, taken from the repository history. Dates are UTC and are the commit dates.

> [!NOTE] The tag `contract-m1-draft.4` marks an internal contract snapshot. It is not a release.

### M9 audit fixes (web lane)

- Public pages are prerendered again. A change made while hardening the console had turned every route into a per-request render; the build output and an end-to-end test now check that the public routes are static, and the Lighthouse lab figures were measured again.
- Unmatched URLs are answered by a per-request 404, so the 404 of `/app`, `/login` and `/auth` still carries the nonce policy and hydrates.
- This changelog and the documentation pages were corrected against the repository's evidence. They had said that M8 was not done and that kills, outages, backup and restore were not run; the retention statement now says when the pass actually runs (hourly in the production example, only if you schedule it in the local stack).
- The end-to-end suite runs in the Chromium, Firefox and WebKit engines of Playwright (not branded Chrome, Edge or Safari builds).
- Console response, route-transition and live-update times were measured on one laptop. The numbers are proposals only: the owner has not frozen any budget.

### M9: web hardening, release documentation, audit and review fixes (merged 2026-11-20)

- The console-only nonce policy now covers every console path in any letter case, a role change re-seals the session cookie, and the stream relay frees a slot when a client half-closes.
- README, SECURITY, CONTRIBUTING, CODE_OF_CONDUCT, a traceability table, a release-gate record and issue templates.
- After an independent review of the production example: the control plane runs an hourly retention pass there, Caddy refuses unusual HTTP methods and runs as a non-root user, the gRPC and metrics listeners are scoped to their own networks, metrics scrapes are bounded, and a production configuration refuses to disable the mutation rate limit. A tenth alert rule covers a down node exporter. These were proven locally only.
- An independent read-only audit (`AUDIT-M9-RV-01`) recommended **NO-GO for 1.0**. It found no critical defect in the engine; the blockers are evidence, documentation and CI gaps plus decisions that belong to the owner. This is a recommendation, not a release decision.

### M8: operations (merged 2026-11-20)

All of it was run **locally only** (macOS with a Colima VM, arm64). No reference host, public domain, real GitHub sign-in or amd64 machine was used.

- Monitoring: Prometheus configuration, nine alert rules with an incident runbook, and extra `clusterx_*` metrics on the internal metrics listener. Each alert was induced locally. No Alertmanager is shipped, so nothing sends notifications until you wire one up.
- A backup and restore drill (`pg_dump` archive, restore into a new database) and a 0002 to 0004 migration drill. Recovery point and recovery time were measured on the drill stack and are not frozen objectives.
- Benchmarks with 1, 3 and 5 workers and a 1,000-job exact reconciliation, measured on a small development VM. They are not capacity claims. See [Reliability](/docs/reliability).
- A production example (`infra/production`): Caddy TLS, TLS between the web server and control plane and between workers and control plane, secrets as files, no published database port. It was proven on one machine with a self-signed certificate authority.
- A threat model and an OWASP ASVS 5.0 mapping, both drafts for review, and supply-chain scan scripts.
- Continuous-integration workflows exist for tests and security scans. **They have never been run on GitHub**: the repository has not been pushed.

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

- Real GitHub sign-in has not been run; the web console has only been tested against a mock OAuth server. The same is true of production-mode cookies in a browser.
- The production example has not been run on a real host, with a public domain or on amd64.
- The GitHub Actions workflows (continuous integration and supply-chain scans) have never run. OSV scanning and an SBOM have not been run locally either.
- An independent review (REVIEW-M9-RV-02) covered the M9 web hardening and the M8 production example: no critical or high issue; its medium findings are fixed but not yet re-reviewed. No manual screen-reader or keyboard-only walkthrough has been done.
- Service objectives, recovery point and time objectives and console performance budgets are not frozen.
- Workflow definitions are not part of 1.0.
