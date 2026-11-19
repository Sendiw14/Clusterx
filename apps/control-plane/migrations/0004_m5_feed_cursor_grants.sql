-- CLUSTERX control plane — migration 0004 (TASK-M5-BE-02). Forward-only, additive except for grant
-- narrowing. Schema contract change: integrator review required (job_events.txid).

-- RV5-04: visibility-based stream feed cursor. Each event records the writing transaction's id; the
-- feed reads rows "visible now but not visible in the previous poll's snapshot", so a late commit is
-- never skipped (no wall-clock look-back). Rows written before this migration keep txid NULL and are
-- never fed (they predate any running feed). Setting the default after ADD COLUMN avoids a rewrite.
ALTER TABLE job_events ADD COLUMN txid xid8;
ALTER TABLE job_events ALTER COLUMN txid SET DEFAULT pg_current_xact_id();
CREATE INDEX job_events_txid_idx ON job_events (txid) WHERE txid IS NOT NULL;

-- RV5-08: credential idempotency lookups (create audit rows carrying details.idempotency_key).
CREATE INDEX audit_events_idempotency_idx ON audit_events (action, actor, (details->>'idempotency_key'), occurred_at)
    WHERE details ? 'idempotency_key';

-- RV5-10: least privilege. The runtime only reads tombstones (retention writes them) and only
-- appends to job_events / audit_events.
REVOKE INSERT, UPDATE ON job_tombstones FROM clusterx_runtime;
REVOKE UPDATE ON job_events, audit_events FROM clusterx_runtime;
