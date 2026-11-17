// Package poc is a throwaway proof of the claim / fence / cancel SQL patterns (ADR-0002, ADR-0003).
// shortcut: no RenewLease/ReportFailure/admission validation; M2 store implements the full state table.
package poc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Denial is a refused worker or operator action; Reason is a state-machine.md code.
type Denial struct{ Reason string }

func (d *Denial) Error() string { return "denied: " + d.Reason }

type Store struct {
	Pool        *pgxpool.Pool
	LeaseTTL    time.Duration
	CancelGrace time.Duration
	// Failpoint, when set, is called at named points inside transactions; a non-nil error aborts the tx.
	Failpoint func(name string) error
}

func (s *Store) fail(name string) error {
	if s.Failpoint == nil {
		return nil
	}
	return s.Failpoint(name)
}

type Worker struct{ ID, Incarnation string }

// Lease is the fencing tuple a worker receives from Claim.
type Lease struct {
	Worker
	JobID, AttemptID string
	Generation       int64
}

func inTx(ctx context.Context, p *pgxpool.Pool, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, p, fn) // READ COMMITTED (server default); rollback on error
}

// NewWorker creates a credential and a registered worker advertising one handler.
func (s *Store) NewWorker(ctx context.Context, handler string, capacity int) (Worker, error) {
	var w Worker
	err := inTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var cred string
		if err := tx.QueryRow(ctx, `INSERT INTO worker_credentials (name, secret_hash, prefix)
			VALUES ('poc', sha256(gen_random_uuid()::text::bytea), 'pocx') RETURNING id`).Scan(&cred); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO workers (name, credential_id, current_incarnation_id, capacity, protocol_version)
			VALUES ('poc', $1, gen_random_uuid(), $2, 1) RETURNING id, current_incarnation_id`, cred, capacity).Scan(&w.ID, &w.Incarnation); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO worker_capabilities VALUES ($1, $2, 1)`, w.ID, handler)
		return err
	})
	return w, err
}

// Admit inserts a queued job and its 'admitted' event in one transaction.
func (s *Store) Admit(ctx context.Context, handler string, maxAttempts int) (string, error) {
	var id string
	err := inTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO jobs (handler_name, handler_version, payload, status, available_at, max_attempts, created_by)
			VALUES ($1, 1, '{}', 'queued', now(), $2, 'system') RETURNING id`, handler, maxAttempts).Scan(&id); err != nil {
			return err
		}
		if err := s.fail("admit.after_job_insert"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, to_status, actor)
			VALUES ($1, 1, 'admitted', 'queued', 'system')`, id)
		return err
	})
	return id, err
}

// Claim locks up to max eligible queued jobs (SKIP LOCKED, §5 order) and leases them in one transaction.
func (s *Store) Claim(ctx context.Context, w Worker, max int) ([]Lease, error) {
	var leases []Lease
	err := inTx(ctx, s.Pool, func(tx pgx.Tx) error {
		leases = nil
		var free int
		// Lock the worker row so one worker's claims serialize against its capacity.
		err := tx.QueryRow(ctx, `SELECT w.capacity - (SELECT count(*) FROM job_attempts a WHERE a.worker_id = w.id AND a.state = 'active')
			FROM workers w JOIN worker_credentials c ON c.id = w.credential_id
			WHERE w.id = $1 AND w.current_incarnation_id = $2 AND w.status = 'active'
			  AND c.revoked_at IS NULL AND (c.expires_at IS NULL OR c.expires_at > now())
			FOR UPDATE OF w`, w.ID, w.Incarnation).Scan(&free)
		if errors.Is(err, pgx.ErrNoRows) {
			return &Denial{"worker_not_authorized"}
		} else if err != nil {
			return err
		}
		if free < max {
			max = free
		}
		if max <= 0 {
			return nil
		}
		rows, err := tx.Query(ctx, `SELECT j.id FROM jobs j
			WHERE j.status = 'queued' AND j.available_at <= now() AND j.attempts_used < j.max_attempts
			  AND EXISTS (SELECT 1 FROM worker_capabilities c WHERE c.worker_id = $1
			              AND c.handler_name = j.handler_name AND c.handler_version = j.handler_version)
			ORDER BY j.priority DESC, j.available_at, j.created_at, j.id
			LIMIT $2 FOR UPDATE OF j SKIP LOCKED`, w.ID, max)
		if err != nil {
			return err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, id := range ids {
			l := Lease{Worker: w, JobID: id}
			if err := tx.QueryRow(ctx, `INSERT INTO job_attempts (job_id, attempt_number, lease_generation, worker_id, worker_incarnation_id)
				SELECT id, attempts_used + 1, lease_generation + 1, $2, $3 FROM jobs WHERE id = $1
				RETURNING id, lease_generation`, id, w.ID, w.Incarnation).Scan(&l.AttemptID, &l.Generation); err != nil {
				return err
			}
			if err := s.fail("claim.after_attempt_insert"); err != nil {
				return err
			}
			var version int64
			err := tx.QueryRow(ctx, `UPDATE jobs SET status = 'running', attempts_used = attempts_used + 1,
				lease_generation = lease_generation + 1, current_attempt_id = $2,
				hard_deadline_at = now() + make_interval(secs => timeout_seconds),
				lease_expires_at = LEAST(now() + $3::interval, now() + make_interval(secs => timeout_seconds)),
				version = version + 1, updated_at = now()
				WHERE id = $1 AND status = 'queued' AND attempts_used < max_attempts
				RETURNING version`, id, l.AttemptID, s.LeaseTTL).Scan(&version)
			if err != nil { // 0 rows is impossible under the row lock; treat as abort, never partial
				return fmt.Errorf("claim CAS on %s: %w", id, err)
			}
			if err := s.fail("claim.after_job_update"); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, attempt_id, actor)
				VALUES ($1, $2, 'claimed', 'queued', 'running', $3, 'worker:' || $4)`, id, version, l.AttemptID, w.ID); err != nil {
				return err
			}
			leases = append(leases, l)
		}
		return nil
	})
	return leases, err
}

type Outcome string

const (
	Succeeded Outcome = "succeeded"
	Canceled  Outcome = "canceled"
)

// fence is the ADR-0003 predicate, evaluated inside the CAS statement. $1 job, $2 attempt, $3 generation, $4 worker, $5 incarnation.
const fence = `j.id = $1 AND j.status = 'running' AND j.current_attempt_id = $2 AND j.lease_generation = $3
	AND j.lease_expires_at > now() AND j.hard_deadline_at > now()
	AND a.id = $2 AND a.job_id = j.id AND a.state = 'active' AND a.worker_id = $4 AND a.worker_incarnation_id = $5
	AND w.id = $4 AND w.current_incarnation_id = $5 AND w.status <> 'deregistered'
	AND c.id = w.credential_id AND c.revoked_at IS NULL AND (c.expires_at IS NULL OR c.expires_at > now())`

// ReportSuccess commits success iff the fence holds and no cancel request is committed. If the fence holds but a
// cancel request won the commit race, the attempt is finalized as canceled instead (ADR-0002). Otherwise it is
// denied with no job/attempt/event change and one diagnostic audit row.
func (s *Store) ReportSuccess(ctx context.Context, l Lease, result string) (Outcome, error) {
	var out Outcome
	var denial *Denial
	args := []any{l.JobID, l.AttemptID, l.Generation, l.ID, l.Incarnation}
	err := inTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var version int64
		err := tx.QueryRow(ctx, `UPDATE jobs j SET status = 'succeeded', result = $6::jsonb, terminal_at = now(),
			current_attempt_id = NULL, lease_expires_at = NULL, hard_deadline_at = NULL, version = j.version + 1, updated_at = now()
			FROM job_attempts a, workers w, worker_credentials c
			WHERE `+fence+` AND j.cancel_requested_at IS NULL
			RETURNING j.version`, append(args, result)...).Scan(&version)
		if err == nil {
			out = Succeeded
			return s.finish(ctx, tx, l, version, "succeeded", "succeeded")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// Fence holds but cancel was committed first: this report is the cancellation acknowledgment.
		err = tx.QueryRow(ctx, `UPDATE jobs j SET status = 'canceled', terminal_at = now(), failure_class = 'canceled_by_operator',
			current_attempt_id = NULL, lease_expires_at = NULL, hard_deadline_at = NULL, version = j.version + 1, updated_at = now()
			FROM job_attempts a, workers w, worker_credentials c
			WHERE `+fence+` AND j.cancel_requested_at IS NOT NULL
			RETURNING j.version`, args...).Scan(&version)
		if err == nil {
			out = Canceled
			return s.finish(ctx, tx, l, version, "canceled", "canceled")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		reason, err := s.classify(ctx, tx, l)
		if err != nil {
			return err
		}
		if reason == "" { // idempotent duplicate of the committed success
			out = Succeeded
			return nil
		}
		denial = &Denial{reason}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id, details)
			VALUES ('diagnostic', 'worker.report_success', 'rejected', 'worker:' || $1, 'job', $2,
			        jsonb_build_object('reason', $3::text, 'attempt_id', $4::text, 'lease_generation', $5::bigint))`,
			l.ID, l.JobID, reason, l.AttemptID, l.Generation)
		return err
	})
	if err != nil {
		return "", err
	}
	if denial != nil {
		return "", denial
	}
	return out, nil
}

func (s *Store) finish(ctx context.Context, tx pgx.Tx, l Lease, version int64, attemptState, event string) error {
	tag, err := tx.Exec(ctx, `UPDATE job_attempts SET state = $2, finished_at = now() WHERE id = $1 AND state = 'active'`, l.AttemptID, attemptState)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("attempt %s not active", l.AttemptID)
	}
	if err := s.fail("report.before_event"); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, attempt_id, actor)
		VALUES ($1, $2, $3, 'running', $4, $5, 'worker:' || $6)`, l.JobID, version, event, attemptState, l.AttemptID, l.ID)
	return err
}

// classify picks the denial code after a 0-row CAS (state-machine.md order). "" means idempotent duplicate.
func (s *Store) classify(ctx context.Context, tx pgx.Tx, l Lease) (string, error) {
	var status, attemptState string
	var cur *string
	var gen int64
	var leaseOK, deadlineOK, incOK, deregistered, credOK bool
	err := tx.QueryRow(ctx, `SELECT j.status, j.current_attempt_id::text, j.lease_generation,
			coalesce(j.lease_expires_at > now(), false), coalesce(j.hard_deadline_at > now(), false),
			a.state, (a.worker_id = $3 AND a.worker_incarnation_id = $4 AND w.current_incarnation_id = $4),
			w.status = 'deregistered', (c.revoked_at IS NULL AND (c.expires_at IS NULL OR c.expires_at > now()))
		FROM jobs j JOIN job_attempts a ON a.id = $2 AND a.job_id = j.id
		JOIN workers w ON w.id = $3 JOIN worker_credentials c ON c.id = w.credential_id
		WHERE j.id = $1`, l.JobID, l.AttemptID, l.ID, l.Incarnation).
		Scan(&status, &cur, &gen, &leaseOK, &deadlineOK, &attemptState, &incOK, &deregistered, &credOK)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "stale_attempt", nil
	case err != nil:
		return "", err
	case status == "succeeded" && attemptState == "succeeded":
		return "", nil
	case status == "succeeded" || status == "failed" || status == "canceled":
		return "job_terminal", nil
	case status != "running" || cur == nil || *cur != l.AttemptID || gen != l.Generation:
		return "stale_attempt", nil
	case !leaseOK:
		return "lease_expired", nil
	case !deadlineOK:
		return "deadline_exceeded", nil
	case !incOK:
		return "incarnation_mismatch", nil
	case deregistered:
		return "worker_deregistered", nil
	case !credOK:
		return "credential_revoked", nil
	}
	return "stale_attempt", nil
}

// CancelResult mirrors the REST contract: HTTP 200 canceled / 202 requested / 409 job_terminal.
type CancelResult struct {
	HTTP   int
	Status string
}

// RequestCancel implements E9. The row lock makes the branch deterministic; each write is still a CAS.
func (s *Store) RequestCancel(ctx context.Context, jobID, actor string) (CancelResult, error) {
	var res CancelResult
	err := inTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var status string
		var requested bool
		err := tx.QueryRow(ctx, `SELECT status, cancel_requested_at IS NOT NULL FROM jobs WHERE id = $1 FOR UPDATE`, jobID).Scan(&status, &requested)
		if errors.Is(err, pgx.ErrNoRows) {
			return &Denial{"job_not_found"}
		} else if err != nil {
			return err
		}
		var version int64
		switch {
		case status == "scheduled" || status == "queued" || status == "retry_wait":
			if err := tx.QueryRow(ctx, `UPDATE jobs SET status = 'canceled', terminal_at = now(), cancel_requested_at = now(),
				failure_class = 'canceled_by_operator',
				cancel_requested_by = $2, version = version + 1, updated_at = now()
				WHERE id = $1 AND status IN ('scheduled', 'queued', 'retry_wait') RETURNING version`, jobID, actor).Scan(&version); err != nil {
				return err
			}
			res = CancelResult{200, "canceled"}
			_, err = tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor)
				VALUES ($1, $2, 'canceled', $3, 'canceled', $4)`, jobID, version, status, actor)
			return err
		case status == "running" && !requested:
			// Cap the lease at the cancel grace so recovery finalizes even without a worker ack.
			if err := tx.QueryRow(ctx, `UPDATE jobs SET cancel_requested_at = now(), cancel_requested_by = $2,
				lease_expires_at = LEAST(lease_expires_at, now() + $3::interval), version = version + 1, updated_at = now()
				WHERE id = $1 AND status = 'running' AND cancel_requested_at IS NULL RETURNING version`, jobID, actor, s.CancelGrace).Scan(&version); err != nil {
				return err
			}
			res = CancelResult{202, "running"}
			_, err = tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor)
				VALUES ($1, $2, 'cancel_requested', 'running', 'running', $3)`, jobID, version, actor)
			return err
		case status == "running":
			res = CancelResult{202, "running"}
		case status == "canceled":
			res = CancelResult{200, "canceled"}
		default:
			res = CancelResult{409, status}
		}
		return nil
	})
	return res, err
}

// RecoverExpired implements E8 one job per short transaction. Returns the number of jobs recovered.
func (s *Store) RecoverExpired(ctx context.Context, backoff time.Duration) (int, error) {
	n := 0
	for {
		found := false
		err := inTx(ctx, s.Pool, func(tx pgx.Tx) error {
			var id, attempt string
			var cancel, budget, deadline bool
			err := tx.QueryRow(ctx, `SELECT id, current_attempt_id, cancel_requested_at IS NOT NULL,
					attempts_used < max_attempts, hard_deadline_at <= now()
				FROM jobs WHERE status = 'running' AND (lease_expires_at <= now() OR hard_deadline_at <= now())
				ORDER BY lease_expires_at LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &attempt, &cancel, &budget, &deadline)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			} else if err != nil {
				return err
			}
			found = true
			attemptState, event := "lease_expired", "lease_expired"
			if deadline {
				attemptState, event = "deadline_exceeded", "deadline_exceeded"
			}
			if _, err := tx.Exec(ctx, `UPDATE job_attempts SET state = $2, finished_at = now() WHERE id = $1 AND state = 'active'`, attempt, attemptState); err != nil {
				return err
			}
			to := "failed"
			switch {
			case cancel:
				to, event = "canceled", "canceled"
			case budget:
				to = "retry_wait"
			}
			var version int64
			if err := tx.QueryRow(ctx, `UPDATE jobs SET status = $3,
				terminal_at = CASE WHEN $3 IN ('failed', 'canceled') THEN now() END,
				available_at = CASE WHEN $3 = 'retry_wait' THEN now() + $4::interval ELSE available_at END,
				error_code = CASE WHEN $3 = 'failed' THEN 'attempts_exhausted' END,
				failure_class = CASE WHEN $3 = 'canceled' THEN 'canceled_by_operator'
				                     WHEN $3 = 'failed' AND $5 THEN 'timeout'
				                     WHEN $3 = 'failed' THEN 'lease_expired' END,
				current_attempt_id = NULL, lease_expires_at = NULL, hard_deadline_at = NULL,
				version = version + 1, updated_at = now()
				WHERE id = $1 AND status = 'running' AND current_attempt_id = $2 RETURNING version`, id, attempt, to, backoff, deadline).Scan(&version); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, attempt_id, actor)
				VALUES ($1, $2, $3, 'running', $4, $5, 'system')`, id, version, event, to, attempt)
			return err
		})
		if err != nil || !found {
			return n, err
		}
		n++
	}
}

// PromoteDue implements E2 as one statement (lock, update, append events).
func (s *Store) PromoteDue(ctx context.Context) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `WITH due AS (
			SELECT id, status FROM jobs WHERE status IN ('scheduled', 'retry_wait') AND available_at <= now()
			ORDER BY available_at LIMIT 500 FOR UPDATE SKIP LOCKED),
		upd AS (
			UPDATE jobs j SET status = 'queued', version = j.version + 1, updated_at = now()
			FROM due WHERE j.id = due.id
			RETURNING j.id, j.version, due.status AS from_status)
		INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor)
		SELECT id, version, 'promoted', from_status, 'queued', 'system' FROM upd`)
	return tag.RowsAffected(), err
}
