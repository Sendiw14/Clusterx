-- CLUSTERX control plane — migration 0001 (DRAFT, M1, TASK-M1-BE-01; amended in place by TASK-M1-BE-04
-- because nothing is deployed yet — RV01-04/05/06/07, RV01-02 token authority view, GAP-SESS-01)
-- Target: PostgreSQL 18.x (uses built-in uuidv7()).
-- Source contract: docs/product/MASTER_PROMPT.md R2 §5–§7; ADR-0002/0003/0004.
-- Forward-only, expand/contract (ADR-0004). Apply inside one transaction:
--   psql -v ON_ERROR_STOP=1 --single-transaction -f 0001_init.sql
-- Least-privilege runtime role is created out of band (cluster-level object). It gets SELECT/INSERT/UPDATE
-- on these tables and DELETE only on sessions/submission_idempotency; retention runs as a separate role with
-- DELETE on jobs/audit_events/job_tombstones. Independently of grants, triggers below refuse to delete the
-- history (attempts, events) of a retained job and audit rows younger than the 90-day floor (RV01-06).
-- All timestamps are timestamptz (stored UTC). The DB clock (now()) is authoritative
-- for every due/expiry comparison (ADR-0003).

-- ---------------------------------------------------------------------------
-- Humans, sessions, direct API tokens
-- ---------------------------------------------------------------------------

CREATE TABLE users (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    github_user_id  bigint NOT NULL UNIQUE CHECK (github_user_id > 0),  -- numeric ID allowlist (FR-SEC-01)
    github_login    text   NOT NULL CHECK (length(github_login) BETWEEN 1 AND 39),
    role            text   NOT NULL CHECK (role IN ('operator', 'viewer')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    disabled_at     timestamptz
);

-- Server-held human session (BFF). Only a hash of the session secret is stored.
CREATE TABLE sessions (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    secret_hash   bytea NOT NULL UNIQUE CHECK (octet_length(secret_hash) = 32),  -- SHA-256
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,                      -- absolute expiry
    -- idle expiry (GAP-SESS-01): valid iff last_activity_at + idle_timeout_seconds > now(); Go bumps
    -- last_activity_at at most once per 60 s per session on authenticated requests.
    last_activity_at      timestamptz NOT NULL DEFAULT now(),
    idle_timeout_seconds  integer NOT NULL DEFAULT 7200 CHECK (idle_timeout_seconds BETWEEN 60 AND 86400),
    revoked_at    timestamptz,
    CHECK (expires_at > created_at),
    CHECK (last_activity_at >= created_at)
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

-- Direct API tokens (FR-SEC-03). Secret shown once; only SHA-256 of a >=256-bit random secret stored.
CREATE TABLE api_tokens (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    name          text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    owner_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    role          text NOT NULL CHECK (role IN ('operator', 'viewer')),
    scopes        text[] NOT NULL CHECK (cardinality(scopes) BETWEEN 1 AND 16),
    secret_hash   bytea NOT NULL UNIQUE CHECK (octet_length(secret_hash) = 32),
    prefix        text NOT NULL CHECK (length(prefix) BETWEEN 4 AND 16),  -- non-secret display prefix
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz,
    revoked_at    timestamptz,
    last_used_at  timestamptz,
    CHECK (expires_at IS NULL OR expires_at > created_at)
);

-- ---------------------------------------------------------------------------
-- Workers
-- ---------------------------------------------------------------------------

-- Operator-issued worker credential (FR-SEC-05). Hash only.
CREATE TABLE worker_credentials (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    name          text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    secret_hash   bytea NOT NULL UNIQUE CHECK (octet_length(secret_hash) = 32),
    prefix        text NOT NULL CHECK (length(prefix) BETWEEN 4 AND 16),
    created_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz,
    revoked_at    timestamptz,
    revoked_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    last_used_at  timestamptz,
    CHECK (expires_at IS NULL OR expires_at > created_at)
);

-- One row per logical worker. A new process registers a new incarnation_id; every
-- attempt held by an older incarnation is fenced (ADR-0003).
CREATE TABLE workers (
    id                      uuid PRIMARY KEY DEFAULT uuidv7(),
    name                    text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    credential_id           uuid NOT NULL REFERENCES worker_credentials (id) ON DELETE RESTRICT,
    current_incarnation_id  uuid NOT NULL,
    status                  text NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'draining', 'deregistered')),
    capacity                integer NOT NULL CHECK (capacity BETWEEN 1 AND 256),
    protocol_version        integer NOT NULL CHECK (protocol_version >= 1),
    sdk_version             text CHECK (length(sdk_version) <= 64),
    hostname                text CHECK (length(hostname) <= 255),
    registered_at           timestamptz NOT NULL DEFAULT now(),
    last_heartbeat_at       timestamptz NOT NULL DEFAULT now(),
    drain_requested_at      timestamptz,
    drain_deadline_at       timestamptz,
    deregistered_at         timestamptz,
    version                 bigint NOT NULL DEFAULT 1 CHECK (version >= 1),  -- bumped on status/incarnation change, not on heartbeat
    UNIQUE (current_incarnation_id),
    CHECK ((status = 'draining') = (drain_requested_at IS NOT NULL AND drain_deadline_at IS NOT NULL)
           OR status = 'deregistered'),
    CHECK ((status = 'deregistered') = (deregistered_at IS NOT NULL))
);
CREATE INDEX workers_credential_idx ON workers (credential_id);
CREATE INDEX workers_liveness_idx ON workers (last_heartbeat_at) WHERE status <> 'deregistered';

-- Handler capability per worker (typed name + schema version), used by the claim join.
CREATE TABLE worker_capabilities (
    worker_id        uuid NOT NULL REFERENCES workers (id) ON DELETE CASCADE,
    handler_name     text NOT NULL CHECK (handler_name ~ '^[a-z][a-z0-9_.-]{0,62}$'),
    handler_version  integer NOT NULL CHECK (handler_version BETWEEN 1 AND 1000),
    PRIMARY KEY (worker_id, handler_name, handler_version)
);
CREATE INDEX worker_capabilities_handler_idx ON worker_capabilities (handler_name, handler_version);

-- ---------------------------------------------------------------------------
-- Jobs
-- ---------------------------------------------------------------------------

CREATE TABLE jobs (
    id                    uuid PRIMARY KEY DEFAULT uuidv7(),
    namespace             text NOT NULL DEFAULT 'default' CHECK (namespace ~ '^[a-z][a-z0-9_-]{0,62}$'),
    handler_name          text NOT NULL CHECK (handler_name ~ '^[a-z][a-z0-9_.-]{0,62}$'),
    handler_version       integer NOT NULL CHECK (handler_version BETWEEN 1 AND 1000),
    payload               jsonb NOT NULL CHECK (octet_length(payload::text) <= 65536),  -- 64 KiB (ADR-0004)
    priority              smallint NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 9), -- 9 highest
    status                text NOT NULL
                          CHECK (status IN ('scheduled', 'queued', 'running', 'retry_wait',
                                            'succeeded', 'failed', 'canceled')),
    run_after             timestamptz,                 -- as submitted (NULL = immediately)
    available_at          timestamptz NOT NULL,        -- next eligible time (DB clock)
    max_attempts          smallint NOT NULL DEFAULT 3 CHECK (max_attempts BETWEEN 1 AND 5),
    attempts_used         smallint NOT NULL DEFAULT 0,
    timeout_seconds       integer NOT NULL DEFAULT 600 CHECK (timeout_seconds BETWEEN 1 AND 86400),
    -- current lease (only while running)
    current_attempt_id    uuid,
    lease_generation      bigint NOT NULL DEFAULT 0 CHECK (lease_generation >= 0),
    lease_expires_at      timestamptz,
    hard_deadline_at      timestamptz,
    -- cancellation (orthogonal field, not a state)
    cancel_requested_at   timestamptz,
    cancel_requested_by   text CHECK (length(cancel_requested_by) <= 200),
    -- outcome
    result                jsonb CHECK (result IS NULL OR octet_length(result::text) <= 131072), -- 128 KiB
    error_code            text CHECK (length(error_code) <= 64),
    error_message         text CHECK (length(error_message) <= 2048),
    failure_class         text CHECK (failure_class IN ('retryable_exhausted', 'permanent', 'timeout',
                                                        'lease_expired', 'canceled_by_operator')),
    terminal_at           timestamptz,
    -- lineage / provenance
    retried_from_job_id   uuid REFERENCES jobs (id) ON DELETE SET NULL,
    created_by            text NOT NULL CHECK (length(created_by) BETWEEN 1 AND 200), -- principal ref "user:<uuid>" | "token:<uuid>"
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    version               bigint NOT NULL DEFAULT 1 CHECK (version >= 1),  -- == latest job_events.version

    CHECK (attempts_used BETWEEN 0 AND max_attempts),            -- no hidden extra attempt
    CHECK (lease_generation >= attempts_used),
    -- running <=> a current lease exists
    CHECK ((status = 'running') = (current_attempt_id IS NOT NULL
                                   AND lease_expires_at IS NOT NULL
                                   AND hard_deadline_at IS NOT NULL)),
    CHECK (status <> 'running' OR attempts_used >= 1),
    CHECK (lease_expires_at IS NULL OR hard_deadline_at IS NULL OR lease_expires_at <= hard_deadline_at),
    -- terminal <=> terminal_at
    CHECK ((status IN ('succeeded', 'failed', 'canceled')) = (terminal_at IS NOT NULL)),
    CHECK (status = 'succeeded' OR result IS NULL),
    -- failure_class (CCR-M1 OQ-08): present exactly on failed/canceled jobs
    CHECK ((status = 'canceled') = (failure_class IS NOT DISTINCT FROM 'canceled_by_operator')),
    CHECK ((status = 'failed') = (failure_class IN ('retryable_exhausted', 'permanent', 'timeout', 'lease_expired'))),
    -- a cancel request only exists on running (pending ack) or canceled jobs
    CHECK (cancel_requested_at IS NULL OR status IN ('running', 'canceled')),
    CHECK (status <> 'succeeded' OR cancel_requested_at IS NULL),
    CHECK (retried_from_job_id IS NULL OR retried_from_job_id <> id)
);

-- Attempts. Exactly one 'active' attempt per job at most (current-lease uniqueness).
CREATE TABLE job_attempts (
    id                     uuid PRIMARY KEY DEFAULT uuidv7(),
    job_id                 uuid NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    attempt_number         smallint NOT NULL CHECK (attempt_number BETWEEN 1 AND 5),
    lease_generation       bigint NOT NULL CHECK (lease_generation >= 1),
    worker_id              uuid NOT NULL REFERENCES workers (id) ON DELETE RESTRICT,
    worker_incarnation_id  uuid NOT NULL,
    state                  text NOT NULL DEFAULT 'active'
                           CHECK (state IN ('active', 'succeeded', 'failed', 'lease_expired',
                                            'deadline_exceeded', 'canceled')),
    error_class            text CHECK (error_class IN ('retryable', 'permanent')),
    error_code             text CHECK (length(error_code) <= 64),
    error_message          text CHECK (length(error_message) <= 2048),
    log_tail               text CHECK (octet_length(log_tail) <= 16384),  -- bounded; retention 7 d
    result_sha256          bytea CHECK (octet_length(result_sha256) = 32), -- reported result (also when discarded by a cancel, RV01-07/08)
    started_at             timestamptz NOT NULL DEFAULT now(),
    finished_at            timestamptz,
    UNIQUE (job_id, attempt_number),
    UNIQUE (job_id, lease_generation),
    UNIQUE (id, job_id),                                           -- target of jobs composite FK
    CHECK ((state = 'active') = (finished_at IS NULL)),
    CHECK (state <> 'failed' OR error_class IS NOT NULL)
);
CREATE UNIQUE INDEX job_attempts_one_active_per_job ON job_attempts (job_id) WHERE state = 'active';
CREATE INDEX job_attempts_worker_active_idx ON job_attempts (worker_id) WHERE state = 'active';

-- The current attempt must belong to this job.
ALTER TABLE jobs
    ADD CONSTRAINT jobs_current_attempt_fk
    FOREIGN KEY (current_attempt_id, id) REFERENCES job_attempts (id, job_id)
    DEFERRABLE INITIALLY DEFERRED;  -- deferred so retention can delete job+attempts in any order

-- Claim path: eligible queued rows per handler, ordered by the §5 tie-break.
CREATE INDEX jobs_claimable_idx ON jobs (handler_name, handler_version, priority DESC, available_at, created_at, id)
    WHERE status = 'queued';
-- Due promotion of scheduled / retry_wait.
CREATE INDEX jobs_due_idx ON jobs (available_at) WHERE status IN ('scheduled', 'retry_wait');
-- Lease / deadline recovery.
CREATE INDEX jobs_lease_expiry_idx ON jobs (lease_expires_at) WHERE status = 'running';
-- Listing / filters (cursor pagination on (created_at DESC, id DESC)).
CREATE INDEX jobs_status_created_idx ON jobs (status, created_at DESC, id DESC);
CREATE INDEX jobs_handler_status_created_idx ON jobs (handler_name, status, created_at DESC, id DESC);
CREATE INDEX jobs_created_idx ON jobs (created_at DESC, id DESC);
CREATE INDEX jobs_terminal_idx ON jobs (terminal_at) WHERE terminal_at IS NOT NULL;  -- retention
-- At most one manual-retry child per failed job (duplicate manual retry guard, ADR-0004).
CREATE UNIQUE INDEX jobs_one_retry_child_idx ON jobs (retried_from_job_id) WHERE retried_from_job_id IS NOT NULL;

-- Retention tombstones: lets GET /jobs/{id} answer 410 expired (not 404) after deletion (ADR-0004).
CREATE TABLE job_tombstones (
    job_id       uuid PRIMARY KEY,
    terminal_at  timestamptz NOT NULL,
    expired_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX job_tombstones_expired_idx ON job_tombstones (expired_at);

-- Per-job durable event log; version is monotonic per job and equals jobs.version after the change.
CREATE TABLE job_events (
    job_id       uuid NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    version      bigint NOT NULL CHECK (version >= 1),
    event_type   text NOT NULL CHECK (event_type IN (
                     'admitted', 'promoted', 'claimed', 'succeeded', 'failed_retryable',
                     'failed_permanent', 'attempts_exhausted', 'lease_expired', 'deadline_exceeded',
                     'cancel_requested', 'canceled', 'retry_created')),
    from_status  text,
    to_status    text NOT NULL,
    attempt_id   uuid,
    actor        text NOT NULL CHECK (length(actor) BETWEEN 1 AND 200),  -- "user:<id>" | "token:<id>" | "worker:<id>" | "system"
    occurred_at  timestamptz NOT NULL DEFAULT now(),
    data         jsonb CHECK (data IS NULL OR octet_length(data::text) <= 4096),
    PRIMARY KEY (job_id, version)
);

-- ---------------------------------------------------------------------------
-- Idempotency (FR-ENG-17) and audit
-- ---------------------------------------------------------------------------

-- Scope = principal + namespace + key. fingerprint = SHA-256 of canonical (RFC 8785) submission.
-- namespace 'manual-retry:<job_id>' is reserved for the retry action key.
CREATE TABLE submission_idempotency (
    principal     text NOT NULL CHECK (length(principal) BETWEEN 1 AND 200),
    namespace     text NOT NULL CHECK (length(namespace) BETWEEN 1 AND 120),
    idem_key      text NOT NULL CHECK (length(idem_key) BETWEEN 1 AND 128),
    fingerprint   bytea NOT NULL CHECK (octet_length(fingerprint) = 32),
    job_id        uuid NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    PRIMARY KEY (principal, namespace, idem_key),
    CHECK (expires_at > created_at)
);
CREATE INDEX submission_idempotency_expiry_idx ON submission_idempotency (expires_at);

-- Privileged actions (audit), security rejections and bounded diagnostics (stale results).
CREATE TABLE audit_events (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    category     text NOT NULL CHECK (category IN ('privileged', 'security', 'diagnostic')),
    action       text NOT NULL CHECK (length(action) BETWEEN 1 AND 64),
    outcome      text NOT NULL CHECK (outcome IN ('allowed', 'denied', 'rejected', 'error')),
    actor        text NOT NULL CHECK (length(actor) BETWEEN 1 AND 200),
    target_type  text CHECK (target_type IN ('job', 'worker', 'worker_credential', 'api_token', 'user', 'session')),
    target_id    uuid,
    request_id   text CHECK (length(request_id) <= 64),
    occurred_at  timestamptz NOT NULL DEFAULT now(),
    details      jsonb CHECK (details IS NULL OR octet_length(details::text) <= 4096)
);
CREATE INDEX audit_events_time_idx ON audit_events (occurred_at DESC, id DESC);
CREATE INDEX audit_events_target_idx ON audit_events (target_type, target_id, occurred_at DESC);

-- ---------------------------------------------------------------------------
-- Immutability guards (ADR-0002, ADR-0004)
-- ---------------------------------------------------------------------------

-- Terminal jobs are immutable while retained. Retention may DELETE; nothing may UPDATE,
-- except the FK action ON DELETE SET NULL of retried_from_job_id when the parent expires.
CREATE FUNCTION clusterx_forbid_terminal_job_update() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    probe jobs%ROWTYPE;
BEGIN
    probe := NEW;
    -- only the FK action (parent expired => NULL) may touch lineage; repointing is forbidden (RV01-06)
    IF OLD.retried_from_job_id IS NOT NULL AND NEW.retried_from_job_id IS NULL THEN
        probe.retried_from_job_id := OLD.retried_from_job_id;
    END IF;
    IF OLD.status IN ('succeeded', 'failed', 'canceled') AND probe IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'job % is terminal (%) and immutable', OLD.id, OLD.status
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER jobs_terminal_immutable BEFORE UPDATE ON jobs
    FOR EACH ROW EXECUTE FUNCTION clusterx_forbid_terminal_job_update();

-- Admission fields are immutable; the attempt budget only advances by one, on queued -> running (RV01-05).
CREATE FUNCTION clusterx_guard_job_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW.namespace, NEW.handler_name, NEW.handler_version, NEW.payload, NEW.max_attempts,
        NEW.timeout_seconds, NEW.created_by, NEW.created_at, NEW.run_after, NEW.priority)
       IS DISTINCT FROM
       (OLD.namespace, OLD.handler_name, OLD.handler_version, OLD.payload, OLD.max_attempts,
        OLD.timeout_seconds, OLD.created_by, OLD.created_at, OLD.run_after, OLD.priority) THEN
        RAISE EXCEPTION 'job % admission fields are immutable', OLD.id USING ERRCODE = 'check_violation';
    END IF;
    IF NEW.attempts_used <> OLD.attempts_used
       AND NOT (NEW.attempts_used = OLD.attempts_used + 1 AND OLD.status = 'queued' AND NEW.status = 'running'
                AND NEW.lease_generation = OLD.lease_generation + 1) THEN
        RAISE EXCEPTION 'job % attempt budget may only advance by one on claim', OLD.id USING ERRCODE = 'check_violation';
    END IF;
    IF NEW.lease_generation <> OLD.lease_generation AND NEW.attempts_used = OLD.attempts_used THEN
        RAISE EXCEPTION 'job % lease generation changes only with a claim', OLD.id USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER jobs_guard_update BEFORE UPDATE ON jobs
    FOR EACH ROW EXECUTE FUNCTION clusterx_guard_job_update();

-- An attempt can only be born as the current lease of a running job, numbered by the job's budget (RV01-05).
-- Claim therefore updates the job first (current_attempt_id = new id; FK is deferred) and inserts the attempt second.
CREATE FUNCTION clusterx_guard_attempt_insert() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    j jobs%ROWTYPE;
BEGIN
    SELECT * INTO j FROM jobs WHERE id = NEW.job_id;
    IF NEW.state <> 'active' OR j.status IS DISTINCT FROM 'running' OR j.current_attempt_id IS DISTINCT FROM NEW.id
       OR NEW.attempt_number <> j.attempts_used OR NEW.lease_generation <> j.lease_generation
       OR NEW.attempt_number > j.max_attempts THEN
        RAISE EXCEPTION 'attempt % does not match the current lease of job %', NEW.id, NEW.job_id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER job_attempts_guard_insert BEFORE INSERT ON job_attempts
    FOR EACH ROW EXECUTE FUNCTION clusterx_guard_attempt_insert();

-- History of a retained job cannot be deleted; only the FK cascade from DELETE jobs (retention) removes it.
CREATE FUNCTION clusterx_forbid_history_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM jobs WHERE id = OLD.job_id) THEN
        RAISE EXCEPTION '% rows of retained job % cannot be deleted', TG_TABLE_NAME, OLD.job_id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN OLD;
END $$;
CREATE TRIGGER job_attempts_no_delete BEFORE DELETE ON job_attempts
    FOR EACH ROW EXECUTE FUNCTION clusterx_forbid_history_delete();
CREATE TRIGGER job_events_no_delete BEFORE DELETE ON job_events
    FOR EACH ROW EXECUTE FUNCTION clusterx_forbid_history_delete();

-- Audit rows are deletable only past the 90-day retention floor (ADR-0004).
CREATE FUNCTION clusterx_forbid_young_audit_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.occurred_at > now() - interval '90 days' THEN
        RAISE EXCEPTION 'audit event % is younger than the 90-day retention floor', OLD.id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN OLD;
END $$;
CREATE TRIGGER audit_events_no_young_delete BEFORE DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION clusterx_forbid_young_audit_delete();

-- Finished attempts are immutable.
CREATE FUNCTION clusterx_forbid_finished_attempt_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.state <> 'active' THEN
        RAISE EXCEPTION 'attempt % is finished (%) and immutable', OLD.id, OLD.state
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER job_attempts_finished_immutable BEFORE UPDATE ON job_attempts
    FOR EACH ROW EXECUTE FUNCTION clusterx_forbid_finished_attempt_update();

-- Event and audit logs are append-only.
CREATE FUNCTION clusterx_forbid_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = 'check_violation';
END $$;
CREATE TRIGGER job_events_append_only BEFORE UPDATE ON job_events
    FOR EACH ROW EXECUTE FUNCTION clusterx_forbid_update();
CREATE TRIGGER audit_events_append_only BEFORE UPDATE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION clusterx_forbid_update();

-- ---------------------------------------------------------------------------
-- Effective API token authority (RV01-02): scopes ∩ owner's CURRENT role, evaluated on every request.
-- A token never outranks its owner: demoting the owner strips write scopes at once; disabling the owner
-- (or revoking/expiring the token) yields a non-'ok' status and the request is rejected with 401.
-- ---------------------------------------------------------------------------
CREATE VIEW api_token_authority AS
SELECT t.id AS token_id,
       t.secret_hash,
       t.owner_user_id,
       CASE WHEN t.role = 'operator' AND u.role = 'operator' THEN 'operator' ELSE 'viewer' END AS effective_role,
       ARRAY(SELECT sc FROM unnest(t.scopes) sc
             WHERE sc LIKE '%:read' OR (t.role = 'operator' AND u.role = 'operator')) AS effective_scopes,
       CASE WHEN t.revoked_at IS NOT NULL THEN 'token_revoked'
            WHEN t.expires_at IS NOT NULL AND t.expires_at <= now() THEN 'token_expired'
            WHEN u.disabled_at IS NOT NULL THEN 'user_disabled'
            ELSE 'ok' END AS status
FROM api_tokens t JOIN users u ON u.id = t.owner_user_id;
