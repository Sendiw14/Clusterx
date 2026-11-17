# CLUSTERX — Risk and Assumption Register (M0)

| ID | Risk / assumption | Impact | Mitigation / validation | Owner | Gate |
|---|---|---|---|---|---|
| R-01 | Cancel vs success race produces two terminal outcomes | P0 data | Single-transaction CAS; race PoC (M1) | Backend | M1 |
| R-02 | >1 authorized lease under contention | P0 data | `FOR UPDATE SKIP LOCKED` + unique current-lease constraint; 20-claimant PoC | Backend | M1 |
| R-03 | Stale worker result overwrites authoritative state | P0 data | Fencing predicate on attempt/generation/incarnation/expiry; stale-result PoC | Backend | M1 |
| R-04 | Context deadline mistaken for forced kill | P0 honesty | Docs + stubborn-handler test; no forced-stop claim | Backend | M5 |
| R-05 | Trust boundary inconsistency browser/BFF/Go/worker/demo | P0 security | Auth ADR + negative tests | Backend + Integrator | M1/M5 |
| R-06 | Exact toolchain combo (Go 1.27, Next 16, TS 7, Tailwind 4, OpenAPI 3.2.1 generators) incompatible | P1 | Dependency matrix after clean build (OD-07) | Integrator | M3 |
| R-07 | SKIP LOCKED ≠ global FIFO; starvation of low priority | P1 | Document; starvation monitor | Backend | M4 |
| R-08 | WebSocket overclaimed as authoritative | P1 | Snapshot+version reconciliation contract | Backend + Frontend | M1/M6 |
| R-09 | Demo accidentally calls production API | P0 security | Separate data provider; network-isolation test | Frontend | M7 |
| R-10 | Owner decisions OD-02/03/04 unresolved | Blocks publish/deploy | Local-only defaults | Owner | Publication |
| R-11 | Local env lacks Docker; PG 14 not 18.7 (2026-11-17) | Blocks M3 compose, M1 PoC fidelity | Owner approved brew install Go + PG18; Docker pending | Owner | M1/M3 |
| R-12 | Ultracode activation not observable from this session | MW-04 | Record as UNVERIFIED; owner checks `/status` | Owner | MW-04 |
| R-13 | Subagent lanes share one parent context (not independent sessions) | MW-03/05 | Separate worktrees per lane; reviewer subagent never authors | Integrator | Ongoing |
| R-14 | Retention deletion vs immutable terminal history | P1 | Retention ADR; tombstone/idempotency lifetimes | Backend | M1 ADR |
| R-15 | No SLO numbers until pilot | P1 | OD-06; report measured only | Integrator | RC |

## Assumptions (default, reversible)

- A-01: Local-only until OD-04. No deploy, no spend.
- A-02: UUIDv7 for IDs (sortable) — validate in schema ADR.
- A-03: OpenAPI 3.1 used unless 3.2.1 generator compatibility proven in M3.
- A-04: sqlc/pgx-style Go DB access; no ORM — confirm in ADR.
