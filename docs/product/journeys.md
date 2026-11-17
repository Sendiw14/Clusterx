# CLUSTERX — Personas and Journeys (M0)

Source: MASTER_PROMPT R2 §2. Priority order: developer/operator first.

## Personas

| ID | Persona | Access | Goal |
|---|---|---|---|
| P-DEV | Self-hosting developer | Operator role + API token | Integrate bounded trusted jobs via SDK/API |
| P-OPS | Operator | Operator role | Diagnose queue/worker failures; cancel/retry/drain |
| P-VIEW | Read-only viewer | Viewer role | Review cluster health; no mutation, masked sensitive fields |
| P-EVAL | Portfolio evaluator | Anonymous | Explore `/demo` + docs; never touches production data |

## Journeys

| ID | Persona | Journey | Requirements | AC |
|---|---|---|---|---|
| J-01 | P-DEV | Clone → `docker compose up` → coordinator, PG, web, 3 workers healthy | NFR-OPS-02 | AC-26 |
| J-02 | P-DEV | Register worker with operator-issued credential | FR-ENG-19, FR-SEC-05 | AC-09 |
| J-03 | P-DEV | Submit typed job (API/SDK/console) → see attempts + result | FR-ENG-01, FR-API-01, FR-WEB-02 | AC-01, AC-15 |
| J-04 | P-DEV | Retry same idempotency key → same job; changed body → 409 | FR-ENG-17 | AC-07 |
| J-05 | P-OPS | Find delayed / failed / unschedulable jobs via filters | FR-ENG-09, FR-API-02 | AC-10, AC-15 |
| J-06 | P-OPS | Cancel running job; observe outcome vs concurrent success | FR-ENG-13 | AC-06 |
| J-07 | P-OPS | Manual retry of failed job → linked new job | FR-ENG-12 | AC-15 |
| J-08 | P-OPS | Drain worker; active attempts finish or recover | FR-ENG-14 | AC-11 |
| J-09 | P-OPS | Kill worker mid-job; watch lease expiry and recovery; stale late result rejected | FR-ENG-10, FR-ENG-15, FR-ENG-16 | AC-03, AC-04 |
| J-10 | P-OPS | Issue / rotate / revoke API token and worker credential (one-time reveal) | FR-SEC-03, FR-SEC-05 | AC-09, AC-13 |
| J-11 | P-OPS | Inspect observability charts with freshness notice + data table | FR-WEB-02, NFR-OBS-01 | AC-15, AC-24 |
| J-12 | P-VIEW | Browse jobs/workers; mutation controls absent/denied; payload masked | FR-SEC-04 | AC-13 |
| J-13 | P-VIEW/OPS | Live channel drops → stale indicator → snapshot reconciles | FR-RT-01 | AC-16 |
| J-14 | P-EVAL | `/` → Execution Atlas (labeled illustrative) → `/demo` read-only tour | FR-WEB-01, FR-DEMO-01 | AC-14 |
| J-15 | P-DEV | Read docs quickstart/concepts/API/workers; commands verified | FR-WEB-01 | AC-26, AC-28 |
| J-16 | Any | Session expiry / 401 / 403 / 404 / 500 recovery | FR-WEB-08 | AC-21 |
| J-17 | P-DEV | North-star: 1,000 jobs, 3 workers, kill one, add workers, honest metrics | NFR-SCALE-01 | AC-18, AC-19 |

## Scenarios needing M1 contract coverage

J-03, J-04, J-06, J-07, J-08, J-09, J-10, J-12, J-13 — each must appear in the state/event matrix, OpenAPI examples, or auth matrix before M1 exit.
