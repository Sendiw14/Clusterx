-- CLUSTERX control plane — migration 0003 (TASK-M5-BE-01). Forward-only, additive (expand only).
-- Indexes for DB-derived metrics (GET /api/v1/metrics/timeseries) and the retention sweeps; no table,
-- column, constraint or grant change. Plain CREATE INDEX inside the migration transaction (takes a SHARE
-- lock on each table for the build; acceptable at 1.0 scale, see ADR-0004 "Migrations").

-- Event-type series (admitted, succeeded, failed, canceled, claimed) bucketed by occurred_at.
CREATE INDEX job_events_occurred_idx ON job_events (occurred_at, event_type);
-- Attempt series: run duration (finished_at) and lease expirations; queue age (started_at).
CREATE INDEX job_attempts_finished_idx ON job_attempts (finished_at) WHERE finished_at IS NOT NULL;
CREATE INDEX job_attempts_started_idx ON job_attempts (started_at);
-- Session retention (expiry + 7 d) and tombstone retention (90 d).
CREATE INDEX sessions_expires_idx ON sessions (expires_at);
