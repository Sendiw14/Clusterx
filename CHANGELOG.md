# Changelog

No version has been released. Entries list merged milestones on `main`.

## Unreleased (pre-1.0)

- **M9 audit fixes** — public pages statically prerendered again, E2E in Chromium, Firefox and WebKit, console budget
  and Lighthouse re-measurements, production example runs hourly retention, Caddy refuses unusual methods and runs
  non-root, gRPC/metrics listeners scoped to private networks, bounded metrics scrapes, production refuses a disabled
  mutation rate limit, node-exporter-down alert, Docker build context excludes nested env files and test keys,
  CI installs PostgreSQL 18 and fails instead of skipping the DB-outage test. All verified locally; CI not yet run.
- **M8 operations** — Prometheus alert rules with runbooks, 1/3/5-worker benchmarks (Colima VM), backup/restore
  and 0002→0004 upgrade drills, local-verified production example (Caddy TLS, gRPC TLS, secrets as files),
  threat model, ASVS 5.0 mapping, supply-chain scan scripts and CI workflows defined (CI not yet run).
- **M9 web hardening** — console-only nonce CSP on every console path, role refresh after changes, relay slot fixes.
- **M6 operator console** — GitHub OAuth (PKCE) BFF with EdDSA service assertions, sealed cookies, CSRF, proxy
  allowlist, WebSocket relay with snapshot reconciliation; all `/app` routes on live data.
- **M7 public site** — home, architecture, docs with generated API reference and search, isolated fixture demo.
- **M5 reliability & security** — cancel/retry/drain, sessions, API tokens and worker credentials, audit log,
  metrics, rate limits, retention, live stream; DB-outage and `kill -9` recovery tests. Migrations 0003–0004.
- **M4 engine** — durable admission with idempotency, atomic claims, leases and fencing, retries, scheduler,
  worker SDK and reference worker, REST read/submit, local Docker Compose starter. Migration 0002.
- **M3 foundation** — monorepo, migrations runner, health endpoints, images, generated TS client, CI workflows defined (not yet run on GitHub).
- **M2 design** — white-first design system, tokens, Execution Atlas grammar, prototypes.
- **M0/M1** — requirements, journeys, ADR-0001…0006, state machine, schema 0001, contracts, race proofs.
