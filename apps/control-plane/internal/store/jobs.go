package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Job is the full job row plus derived read-time fields.
type Job struct {
	ID, Namespace, HandlerName, Status, CreatedBy                      string
	HandlerVersion, Priority, AttemptsUsed, MaxAttempts, TimeoutSeconds int
	AvailableAt, CreatedAt, UpdatedAt                                   time.Time
	CancelRequestedAt, TerminalAt, RunAfter                             *time.Time
	LeaseExpiresAt, HardDeadlineAt                                      *time.Time
	ErrorCode, ErrorMessage, FailureClass                               *string
	RetriedFromJobID, RetriedAsJobID, CurrentAttemptID                  *string
	LeaseGeneration, Version                                            int64
	Payload, Result                                                     json.RawMessage
	UnschedulableReason                                                 *string
	UnschedulableSince                                                  *time.Time
	AsOf                                                                time.Time
}

// jobSelect reads jobs with the derived unschedulable reason (state-machine.md, bottom). $1 is the
// liveness timeout interval; callers append their own WHERE using further parameters.
const jobSelect = `SELECT j.id, j.namespace, j.handler_name, j.handler_version, j.status, j.priority, j.attempts_used,
	j.max_attempts, j.available_at, j.cancel_requested_at, j.terminal_at, j.error_code, j.retried_from_job_id,
	j.failure_class, j.created_at, j.updated_at, j.version, j.payload, j.result, j.error_message, j.run_after,
	j.timeout_seconds, j.current_attempt_id, j.lease_generation, j.lease_expires_at, j.hard_deadline_at, j.created_by,
	(SELECT r.id FROM jobs r WHERE r.retried_from_job_id = j.id), u.reason,
	CASE WHEN u.reason IS NOT NULL THEN GREATEST(j.available_at, u.last_seen) END, clock_timestamp()
FROM jobs j LEFT JOIN LATERAL (
	SELECT CASE WHEN j.status <> 'queued' THEN NULL
		WHEN count(w.id) = 0 THEN 'no_capable_worker'
		WHEN count(w.id) FILTER (WHERE w.last_heartbeat_at > now() - $1::interval) = 0 THEN 'capable_workers_offline'
		WHEN count(w.id) FILTER (WHERE w.last_heartbeat_at > now() - $1::interval AND w.status = 'active') = 0
			THEN 'capable_workers_draining' END AS reason,
		max(w.last_heartbeat_at) AS last_seen
	FROM worker_capabilities c JOIN workers w ON w.id = c.worker_id AND w.status <> 'deregistered'
	WHERE j.status = 'queued' AND c.handler_name = j.handler_name AND c.handler_version = j.handler_version) u ON true`

func scanJob(r pgx.Row) (Job, error) {
	var j Job
	var payload, result []byte
	err := r.Scan(&j.ID, &j.Namespace, &j.HandlerName, &j.HandlerVersion, &j.Status, &j.Priority, &j.AttemptsUsed,
		&j.MaxAttempts, &j.AvailableAt, &j.CancelRequestedAt, &j.TerminalAt, &j.ErrorCode, &j.RetriedFromJobID,
		&j.FailureClass, &j.CreatedAt, &j.UpdatedAt, &j.Version, &payload, &result, &j.ErrorMessage, &j.RunAfter,
		&j.TimeoutSeconds, &j.CurrentAttemptID, &j.LeaseGeneration, &j.LeaseExpiresAt, &j.HardDeadlineAt, &j.CreatedBy,
		&j.RetriedAsJobID, &j.UnschedulableReason, &j.UnschedulableSince, &j.AsOf)
	j.Payload, j.Result = payload, result
	return j, err
}

// AdmitParams is a validated submission (ADR-0004).
type AdmitParams struct {
	Principal, Namespace, HandlerName string
	HandlerVersion, Priority          int
	MaxAttempts, TimeoutSeconds       int
	Payload                           json.RawMessage // compact UTF-8 JSON, <= 64 KiB
	RunAfter                          *time.Time
	IdempotencyKey                    string
	Fingerprint                       []byte // SHA-256 of the canonical submission; required with a key
}

var errIdemExists = errors.New("idempotency key exists")

// Admit inserts the job and its 'admitted' event (and the idempotency row) in one transaction (E1).
// It returns replayed=true for a same-scope, same-fingerprint, unexpired key. Denials:
// idempotency_conflict (Detail = original job id), validation_failed (Detail = "run_after"),
// payload_too_large.
func (s *Store) Admit(ctx context.Context, p AdmitParams) (Job, bool, error) {
	for range 3 {
		var id string
		err := s.inTx(ctx, false, func(tx pgx.Tx) error {
			if p.RunAfter != nil {
				var tooFar bool
				if err := tx.QueryRow(ctx, `SELECT $1::timestamptz > now() + interval '30 days'`, *p.RunAfter).Scan(&tooFar); err != nil {
					return err
				}
				if tooFar {
					return &Denial{Reason: "validation_failed", Detail: "run_after"}
				}
			}
			var status string
			if err := tx.QueryRow(ctx, `INSERT INTO jobs (namespace, handler_name, handler_version, payload, priority, status, run_after,
					available_at, max_attempts, timeout_seconds, created_by, created_at, updated_at)
				VALUES ($1, $2, $3, $4::jsonb, $5, CASE WHEN $6::timestamptz > now() THEN 'scheduled' ELSE 'queued' END, $6,
					CASE WHEN $6::timestamptz > now() THEN $6::timestamptz ELSE now() END, $7, $8, $9, now(), now())
				RETURNING id, status`,
				p.Namespace, p.HandlerName, p.HandlerVersion, string(p.Payload), p.Priority, p.RunAfter, p.MaxAttempts,
				p.TimeoutSeconds, p.Principal).Scan(&id, &status); err != nil {
				return err
			}
			if err := s.fail("admit.after_job_insert"); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, to_status, actor, occurred_at)
				VALUES ($1, 1, 'admitted', $2, $3, clock_timestamp())`, id, status, p.Principal); err != nil {
				return err
			}
			if p.IdempotencyKey != "" {
				tag, err := tx.Exec(ctx, `INSERT INTO submission_idempotency (principal, namespace, idem_key, fingerprint, job_id, created_at, expires_at)
					VALUES ($1, $2, $3, $4, $5, now(), now() + $6::interval)
					ON CONFLICT (principal, namespace, idem_key) DO UPDATE
						SET fingerprint = EXCLUDED.fingerprint, job_id = EXCLUDED.job_id, created_at = EXCLUDED.created_at,
						    expires_at = EXCLUDED.expires_at
						WHERE submission_idempotency.expires_at <= now()`,
					p.Principal, p.Namespace, p.IdempotencyKey, p.Fingerprint, id, s.Timing.IdempotencyTTL)
				if err != nil {
					return err
				}
				if tag.RowsAffected() == 0 {
					return errIdemExists // roll back the job; the key is held by an unexpired submission
				}
			}
			return s.fail("admit.before_commit")
		})
		switch {
		case err == nil:
			j, err := s.GetJob(ctx, id)
			return j, false, err
		case errors.Is(err, errIdemExists):
			var fp []byte
			var orig string
			err := s.Pool.QueryRow(ctx, `SELECT fingerprint, job_id FROM submission_idempotency
				WHERE principal = $1 AND namespace = $2 AND idem_key = $3 AND expires_at > now()`,
				p.Principal, p.Namespace, p.IdempotencyKey).Scan(&fp, &orig)
			if isNoRows(err) {
				continue // expired or removed in between: admit again
			}
			if err != nil {
				return Job{}, false, classifyDBError(err)
			}
			if !bytes.Equal(fp, p.Fingerprint) {
				return Job{}, false, &Denial{Reason: "idempotency_conflict", Detail: orig}
			}
			j, err := s.GetJob(ctx, orig)
			return j, true, err
		default:
			var pe *pgconn.PgError
			if errors.As(err, &pe) && pe.Code == "23514" && pe.ConstraintName == "jobs_payload_check" {
				return Job{}, false, deny("payload_too_large")
			}
			return Job{}, false, err
		}
	}
	return Job{}, false, fmt.Errorf("%w: idempotency key contention", ErrUnavailable)
}

// GetJob reads one job. Denials: job_not_found, expired (Detail = RFC 3339 terminal_at|expired_at).
func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	j, err := scanJob(s.Pool.QueryRow(ctx, jobSelect+` WHERE j.id = $2`, s.Timing.LivenessTimeout, id))
	if isNoRows(err) {
		return Job{}, s.missingJob(ctx, id)
	}
	return j, classifyDBError(err)
}

// missingJob distinguishes retention expiry (410) from never-existed (404).
func (s *Store) missingJob(ctx context.Context, id string) error {
	var terminal, expired time.Time
	err := s.Pool.QueryRow(ctx, `SELECT terminal_at, expired_at FROM job_tombstones WHERE job_id = $1`, id).Scan(&terminal, &expired)
	if isNoRows(err) {
		return deny("job_not_found")
	}
	if err != nil {
		return classifyDBError(err)
	}
	return &Denial{Reason: "expired", Detail: terminal.UTC().Format(time.RFC3339Nano) + "|" + expired.UTC().Format(time.RFC3339Nano)}
}

// Attempt is one job_attempts row.
type Attempt struct {
	ID, JobID, WorkerID, IncarnationID, State          string
	Number                                              int
	LeaseGeneration                                     int64
	ErrorClass, ErrorCode, ErrorMessage, LogTail        *string
	LogTruncated, LogExpired                            bool
	LeaseExpiresAt                                      *time.Time
	StartedAt                                           time.Time
	FinishedAt                                          *time.Time
}

// ListAttempts returns the attempts of a job by attempt number and the job version.
func (s *Store) ListAttempts(ctx context.Context, jobID string) ([]Attempt, int64, error) {
	var version int64
	if err := s.Pool.QueryRow(ctx, `SELECT version FROM jobs WHERE id = $1`, jobID).Scan(&version); err != nil {
		if isNoRows(err) {
			return nil, 0, s.missingJob(ctx, jobID)
		}
		return nil, 0, classifyDBError(err)
	}
	rows, err := s.Pool.Query(ctx, `SELECT a.id, a.job_id, a.worker_id, a.worker_incarnation_id, a.state, a.attempt_number,
			a.lease_generation, a.error_class, a.error_code, a.error_message, a.log_tail, a.log_truncated, a.log_expired,
			CASE WHEN a.state = 'active' THEN j.lease_expires_at END, a.started_at, a.finished_at
		FROM job_attempts a JOIN jobs j ON j.id = a.job_id WHERE a.job_id = $1 ORDER BY a.attempt_number`, jobID)
	if err != nil {
		return nil, 0, classifyDBError(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Attempt, error) {
		var a Attempt
		return a, r.Scan(&a.ID, &a.JobID, &a.WorkerID, &a.IncarnationID, &a.State, &a.Number, &a.LeaseGeneration,
			&a.ErrorClass, &a.ErrorCode, &a.ErrorMessage, &a.LogTail, &a.LogTruncated, &a.LogExpired, &a.LeaseExpiresAt,
			&a.StartedAt, &a.FinishedAt)
	})
	return out, version, classifyDBError(err)
}

// Event is one job_events row.
type Event struct {
	JobID, EventType, ToStatus, Actor string
	Version                           int64
	FromStatus, AttemptID             *string
	OccurredAt                        time.Time
	Data                              json.RawMessage
}

// ListEvents returns up to limit events with version > after, the job version and whether more exist.
func (s *Store) ListEvents(ctx context.Context, jobID string, after int64, limit int) ([]Event, int64, bool, error) {
	var version int64
	if err := s.Pool.QueryRow(ctx, `SELECT version FROM jobs WHERE id = $1`, jobID).Scan(&version); err != nil {
		if isNoRows(err) {
			return nil, 0, false, s.missingJob(ctx, jobID)
		}
		return nil, 0, false, classifyDBError(err)
	}
	rows, err := s.Pool.Query(ctx, `SELECT job_id, event_type, to_status, actor, version, from_status, attempt_id, occurred_at, data
		FROM job_events WHERE job_id = $1 AND version > $2 ORDER BY version LIMIT $3`, jobID, after, limit+1)
	if err != nil {
		return nil, 0, false, classifyDBError(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Event, error) {
		var e Event
		var data []byte
		err := r.Scan(&e.JobID, &e.EventType, &e.ToStatus, &e.Actor, &e.Version, &e.FromStatus, &e.AttemptID, &e.OccurredAt, &data)
		e.Data = data
		return e, err
	})
	if err != nil {
		return nil, 0, false, classifyDBError(err)
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return out, version, more, nil
}
