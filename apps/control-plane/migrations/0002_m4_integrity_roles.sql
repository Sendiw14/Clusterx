-- CLUSTERX control plane — migration 0002 (CR-M4-01, TASK-M4-BE-01).
-- Forward-only. 0001_init.sql is a shipped migration and stays byte-identical.
-- Items: NF-02 (clock_timestamp defaults), NF-03 (log_tail retention path), NF-04 (least-privilege roles,
-- delete/truncate guards), NF-05 (running <=> exactly one active attempt, enforced at commit).
-- Requires a migration role that owns the schema objects and may CREATE ROLE (or the two group roles
-- below already exist). Login users are created out of band and granted one of the group roles:
--   CREATE ROLE clusterx_app LOGIN PASSWORD '<generated>' IN ROLE clusterx_runtime;
--   CREATE ROLE clusterx_retention_job LOGIN PASSWORD '<generated>' IN ROLE clusterx_retention;

-- ---------------------------------------------------------------------------
-- NF-02: event and audit timestamps default to the statement-time clock read after lock waits.
-- Writers also pass clock_timestamp() explicitly; `version` (not occurred_at) is the authoritative order.
-- ---------------------------------------------------------------------------
ALTER TABLE job_events ALTER COLUMN occurred_at SET DEFAULT clock_timestamp();
ALTER TABLE audit_events ALTER COLUMN occurred_at SET DEFAULT clock_timestamp();

-- ---------------------------------------------------------------------------
-- NF-03: attempt log flags and the only post-final attempt mutation (7-day log_tail expiry).
-- ---------------------------------------------------------------------------
ALTER TABLE job_attempts ADD COLUMN log_truncated boolean NOT NULL DEFAULT false;
ALTER TABLE job_attempts ADD COLUMN log_expired boolean NOT NULL DEFAULT false;
ALTER TABLE job_attempts ADD CONSTRAINT job_attempts_log_expired_check CHECK (NOT log_expired OR log_tail IS NULL);

-- Finished attempts are immutable, except exactly: log_tail <non-null> -> NULL together with
-- log_expired false -> true, on an attempt finished at least 7 days ago. Every other column must be unchanged.
CREATE OR REPLACE FUNCTION clusterx_forbid_finished_attempt_update() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    probe job_attempts%ROWTYPE;
BEGIN
    IF OLD.state = 'active' THEN
        RETURN NEW;
    END IF;
    probe := NEW;
    probe.log_tail := OLD.log_tail;
    probe.log_expired := OLD.log_expired;
    IF probe IS NOT DISTINCT FROM OLD
       AND OLD.log_tail IS NOT NULL AND NEW.log_tail IS NULL
       AND NOT OLD.log_expired AND NEW.log_expired
       AND OLD.finished_at <= now() - interval '7 days' THEN
        RETURN NEW;  -- retention log expiry (ADR-0004)
    END IF;
    RAISE EXCEPTION 'attempt % is finished (%) and immutable', OLD.id, OLD.state
        USING ERRCODE = 'check_violation';
END $$;

-- Retention entry point for log expiry; returns the number of attempts whose log was expired.
CREATE FUNCTION clusterx_expire_attempt_logs(max_rows integer) RETURNS integer
LANGUAGE sql AS $$
    WITH due AS (
        SELECT id FROM job_attempts
        WHERE state <> 'active' AND log_tail IS NOT NULL AND finished_at <= now() - interval '7 days'
        ORDER BY finished_at LIMIT LEAST(GREATEST(max_rows, 0), 10000)
        FOR UPDATE SKIP LOCKED),
    upd AS (
        UPDATE job_attempts a SET log_tail = NULL, log_expired = true FROM due WHERE a.id = due.id RETURNING 1)
    SELECT count(*)::integer FROM upd
$$;

-- ---------------------------------------------------------------------------
-- NF-04: jobs may only be deleted by retention (terminal for >= 30 days); TRUNCATE is refused on
-- history tables whatever the grants.
-- ---------------------------------------------------------------------------
CREATE FUNCTION clusterx_guard_job_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.terminal_at IS NULL OR OLD.terminal_at > now() - interval '30 days' THEN
        RAISE EXCEPTION 'job % is not past the 30-day terminal retention floor', OLD.id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN OLD;
END $$;
CREATE TRIGGER jobs_retention_delete_only BEFORE DELETE ON jobs
    FOR EACH ROW EXECUTE FUNCTION clusterx_guard_job_delete();

CREATE FUNCTION clusterx_forbid_truncate() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'TRUNCATE of % is not allowed', TG_TABLE_NAME USING ERRCODE = 'insufficient_privilege';
END $$;
CREATE TRIGGER jobs_no_truncate BEFORE TRUNCATE ON jobs
    FOR EACH STATEMENT EXECUTE FUNCTION clusterx_forbid_truncate();
CREATE TRIGGER job_attempts_no_truncate BEFORE TRUNCATE ON job_attempts
    FOR EACH STATEMENT EXECUTE FUNCTION clusterx_forbid_truncate();
CREATE TRIGGER job_events_no_truncate BEFORE TRUNCATE ON job_events
    FOR EACH STATEMENT EXECUTE FUNCTION clusterx_forbid_truncate();
CREATE TRIGGER audit_events_no_truncate BEFORE TRUNCATE ON audit_events
    FOR EACH STATEMENT EXECUTE FUNCTION clusterx_forbid_truncate();
CREATE TRIGGER job_tombstones_no_truncate BEFORE TRUNCATE ON job_tombstones
    FOR EACH STATEMENT EXECUTE FUNCTION clusterx_forbid_truncate();

-- ---------------------------------------------------------------------------
-- NF-05: at commit, a job is running iff it has exactly one active attempt and that attempt is
-- current_attempt_id. Deferred because Claim updates the job before inserting the attempt and
-- reports update the job before finishing the attempt. (At most one active attempt is already the
-- partial unique index job_attempts_one_active_per_job.)
-- ---------------------------------------------------------------------------
CREATE FUNCTION clusterx_check_job_attempt_coupling() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    jid uuid;
    st text;
    cur uuid;
    active_id uuid;
BEGIN
    IF TG_TABLE_NAME = 'jobs' THEN
        jid := NEW.id;
    ELSE
        jid := NEW.job_id;
    END IF;
    SELECT status, current_attempt_id INTO st, cur FROM jobs WHERE id = jid;
    IF NOT FOUND THEN
        RETURN NULL;  -- deleted later in the same transaction (retention)
    END IF;
    SELECT id INTO active_id FROM job_attempts WHERE job_id = jid AND state = 'active';
    IF (st = 'running') IS DISTINCT FROM (active_id IS NOT NULL)
       OR (st = 'running' AND active_id IS DISTINCT FROM cur) THEN
        RAISE EXCEPTION 'job % status % does not match its active attempt (%)', jid, st, active_id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER jobs_attempt_coupling
    AFTER INSERT OR UPDATE OF status, current_attempt_id ON jobs
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION clusterx_check_job_attempt_coupling();
CREATE CONSTRAINT TRIGGER job_attempts_job_coupling
    AFTER INSERT OR UPDATE OF state ON job_attempts
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION clusterx_check_job_attempt_coupling();

-- ---------------------------------------------------------------------------
-- NF-04: least-privilege group roles. clusterx_runtime: no DDL, no TRUNCATE, DELETE only where the
-- coordinator legitimately deletes (sessions, idempotency expiry, capability replacement on Register).
-- clusterx_retention: deletes terminal jobs (guarded above), audit rows (90-day trigger), tombstones,
-- expired idempotency/sessions, and expires logs through the function above.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    CREATE ROLE clusterx_runtime NOLOGIN;
EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL;
END $$;
DO $$
BEGIN
    CREATE ROLE clusterx_retention NOLOGIN;
EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL;
END $$;

REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
REVOKE ALL ON FUNCTION clusterx_expire_attempt_logs(integer) FROM PUBLIC;

GRANT SELECT ON schema_migrations TO clusterx_runtime, clusterx_retention;
GRANT SELECT, INSERT, UPDATE ON users, sessions, api_tokens, worker_credentials, workers, worker_capabilities,
    jobs, job_attempts, job_events, job_tombstones, submission_idempotency, audit_events TO clusterx_runtime;
GRANT DELETE ON sessions, submission_idempotency, worker_capabilities TO clusterx_runtime;
GRANT SELECT ON api_token_authority TO clusterx_runtime;

GRANT SELECT ON jobs, job_attempts, job_events, job_tombstones, audit_events, submission_idempotency, sessions
    TO clusterx_retention;
GRANT INSERT ON job_tombstones TO clusterx_retention;
GRANT UPDATE (log_tail, log_expired) ON job_attempts TO clusterx_retention;
GRANT DELETE ON jobs, job_tombstones, audit_events, submission_idempotency, sessions TO clusterx_retention;
GRANT EXECUTE ON FUNCTION clusterx_expire_attempt_logs(integer) TO clusterx_retention;
