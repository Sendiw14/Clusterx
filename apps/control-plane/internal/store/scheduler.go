package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func sha256Bytes(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

// PromoteDue moves due scheduled / retry_wait jobs to queued (E2), at most limit per call, in one
// statement (lock, update, append events). Selection uses the DB clock now().
func (s *Store) PromoteDue(ctx context.Context, limit int) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `WITH due AS (
			SELECT id, status FROM jobs WHERE status IN ('scheduled', 'retry_wait') AND available_at <= now()
			ORDER BY available_at LIMIT $1 FOR UPDATE SKIP LOCKED),
		upd AS (
			UPDATE jobs j SET status = 'queued', version = j.version + 1, updated_at = clock_timestamp()
			FROM due WHERE j.id = due.id AND j.status = due.status
			RETURNING j.id, j.version, due.status AS from_status)
		INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor, occurred_at)
		SELECT id, version, 'promoted', from_status, 'queued', 'system', clock_timestamp() FROM upd`, limit)
	if err != nil {
		return 0, classifyDBError(err)
	}
	return tag.RowsAffected(), nil
}

// RecoverExpired implements E8, one job per short transaction, at most limit jobs. A running job whose
// lease or hard deadline passed (DB clock) has its attempt fenced (lease_expired / deadline_exceeded) and
// becomes canceled (cancel requested), retry_wait (budget left, with backoff) or failed (attempts_exhausted).
func (s *Store) RecoverExpired(ctx context.Context, limit int) (int, error) {
	n := 0
	for n < limit {
		found := false
		err := s.inTx(ctx, false, func(tx pgx.Tx) error {
			found = false
			var id, attempt string
			var cancel, deadline bool
			var used, maxA int
			err := tx.QueryRow(ctx, `SELECT id, current_attempt_id, cancel_requested_at IS NOT NULL, attempts_used, max_attempts,
					hard_deadline_at <= now()
				FROM jobs WHERE status = 'running' AND (lease_expires_at <= now() OR hard_deadline_at <= now())
				ORDER BY lease_expires_at LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &attempt, &cancel, &used, &maxA, &deadline)
			if isNoRows(err) {
				return nil
			} else if err != nil {
				return err
			}
			found = true
			attemptState := "lease_expired"
			if deadline {
				attemptState = "deadline_exceeded"
			}
			to, event, failureClass, errCode := "retry_wait", attemptState, "", ""
			var backoff time.Duration
			data := map[string]any{"attempt_number": used}
			switch {
			case cancel:
				to, event, failureClass = "canceled", "canceled", "canceled_by_operator"
			case used < maxA:
				backoff = s.Timing.Backoff.Delay(used)
				data["backoff_ms"] = backoff.Milliseconds()
			default:
				// Budget exhausted: the attempt outcome stays lease_expired / deadline_exceeded (attempt row and
				// data.attempt_state); the job event is attempts_exhausted with the failure_class (CR-M5-01 #3).
				to, errCode, event = "failed", "attempts_exhausted", "attempts_exhausted"
				data["attempt_state"] = attemptState
				failureClass = "lease_expired"
				if deadline {
					failureClass = "timeout"
				}
			}
			if failureClass != "" {
				data["failure_class"] = failureClass
			}
			if errCode != "" {
				data["error_code"] = errCode
			}
			var version int64
			if err := tx.QueryRow(ctx, `UPDATE jobs SET status = $3,
					terminal_at = CASE WHEN $3 IN ('failed', 'canceled') THEN clock_timestamp() END,
					available_at = CASE WHEN $3 = 'retry_wait' THEN clock_timestamp() + $4::interval ELSE available_at END,
					error_code = NULLIF($5, ''), error_message = NULL, failure_class = NULLIF($6, ''),
					current_attempt_id = NULL, lease_expires_at = NULL, hard_deadline_at = NULL,
					version = version + 1, updated_at = clock_timestamp()
				WHERE id = $1 AND status = 'running' AND current_attempt_id = $2 RETURNING version`,
				id, attempt, to, backoff, errCode, failureClass).Scan(&version); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `UPDATE job_attempts SET state = $2, finished_at = clock_timestamp() WHERE id = $1 AND state = 'active'`,
				attempt, attemptState)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return errAttemptNotActive
			}
			_, err = tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, attempt_id, actor, occurred_at, data)
				VALUES ($1, $2, $3, 'running', $4, $5, 'system', clock_timestamp(), $6)`, id, version, event, to, attempt, jsonOrNull(data))
			return err
		})
		if err != nil || !found {
			return n, err
		}
		n++
	}
	return n, nil
}

var errAttemptNotActive = errors.New("store: current attempt is not active")

// CancelResult mirrors the REST contract: HTTP 200 canceled / 202 requested / 409 job_terminal.
type CancelResult struct {
	HTTP    int
	Status  string
	Version int64
}

// RequestCancel implements E9 (REST route arrives in M5; used by tests now). The job row lock makes the
// branch deterministic; each write is a CAS. Timestamps are clock_timestamp() after the lock (NF-02).
func (s *Store) RequestCancel(ctx context.Context, jobID, actor string) (CancelResult, error) {
	var res CancelResult
	err := s.inTx(ctx, false, func(tx pgx.Tx) error {
		var status string
		var requested bool
		var version int64
		err := tx.QueryRow(ctx, `SELECT status, cancel_requested_at IS NOT NULL, version FROM jobs WHERE id = $1 FOR UPDATE`, jobID).
			Scan(&status, &requested, &version)
		if isNoRows(err) {
			return deny("job_not_found")
		} else if err != nil {
			return err
		}
		switch {
		case status == "scheduled" || status == "queued" || status == "retry_wait":
			if err := tx.QueryRow(ctx, `UPDATE jobs SET status = 'canceled', terminal_at = clock_timestamp(),
					cancel_requested_at = clock_timestamp(), failure_class = 'canceled_by_operator', cancel_requested_by = $2,
					version = version + 1, updated_at = clock_timestamp()
				WHERE id = $1 AND status IN ('scheduled', 'queued', 'retry_wait') RETURNING version`, jobID, actor).Scan(&version); err != nil {
				return err
			}
			res = CancelResult{200, "canceled", version}
			_, err = tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor, occurred_at, data)
				VALUES ($1, $2, 'canceled', $3, 'canceled', $4, clock_timestamp(), '{"failure_class":"canceled_by_operator"}')`, jobID, version, status, actor)
			return err
		case status == "running" && !requested:
			if err := tx.QueryRow(ctx, `UPDATE jobs SET cancel_requested_at = clock_timestamp(), cancel_requested_by = $2,
					lease_expires_at = LEAST(lease_expires_at, clock_timestamp() + $3::interval), version = version + 1,
					updated_at = clock_timestamp()
				WHERE id = $1 AND status = 'running' AND cancel_requested_at IS NULL RETURNING version`, jobID, actor, s.Timing.CancelGrace).Scan(&version); err != nil {
				return err
			}
			res = CancelResult{202, "running", version}
			_, err = tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor, occurred_at)
				VALUES ($1, $2, 'cancel_requested', 'running', 'running', $3, clock_timestamp())`, jobID, version, actor)
			return err
		case status == "running":
			res = CancelResult{202, "running", version}
		case status == "canceled":
			res = CancelResult{200, "canceled", version}
		default:
			res = CancelResult{409, status, version}
		}
		return nil
	})
	return res, err
}

// CleanupIdempotency deletes up to limit expired idempotency rows (ADR-0004, 24 h retention).
func (s *Store) CleanupIdempotency(ctx context.Context, limit int) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM submission_idempotency WHERE ctid IN (
		SELECT ctid FROM submission_idempotency WHERE expires_at <= now() LIMIT $1 FOR UPDATE SKIP LOCKED)`, limit)
	if err != nil {
		return 0, classifyDBError(err)
	}
	return tag.RowsAffected(), nil
}
