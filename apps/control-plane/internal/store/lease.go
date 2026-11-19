package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ClaimedAttempt is everything a worker needs to execute and fence one attempt.
type ClaimedAttempt struct {
	JobID, AttemptID, HandlerName              string
	LeaseGeneration                            int64
	AttemptNumber, MaxAttempts, HandlerVersion int
	Payload                                    json.RawMessage
	LeaseExpiresAt, HardDeadlineAt, Now        time.Time
}

// Claim leases up to max eligible queued jobs to the worker in one transaction (E3).
// Eligibility: status queued, available_at <= now(), attempts_used < max_attempts, handler capability
// advertised by the worker (and in supported, when non-empty). Order among eligible rows:
// priority DESC, available_at, created_at, id. FOR UPDATE SKIP LOCKED gives no strict global FIFO.
// The candidate scan is a per-capability top-N on jobs_claimable_idx (RV01-15).
func (s *Store) Claim(ctx context.Context, w WorkerRef, max int, supported []Capability) ([]ClaimedAttempt, error) {
	for _, c := range supported {
		if s.Catalog != nil && !s.Catalog.Known(c.Name, c.Version) {
			return nil, &Denial{Reason: "unknown_handler", Detail: fmt.Sprintf("%s@v%d", c.Name, c.Version)}
		}
	}
	names := make([]string, 0, len(supported))
	versions := make([]int32, 0, len(supported))
	for _, c := range supported {
		names = append(names, c.Name)
		versions = append(versions, int32(c.Version))
	}
	var out []ClaimedAttempt
	err := s.inTx(ctx, true, func(tx pgx.Tx) error {
		out = nil
		// Lock order: credential (share) -> worker (update; serializes this worker's capacity) -> jobs.
		var credOK bool
		err := tx.QueryRow(ctx, `SELECT c.revoked_at IS NULL AND (c.expires_at IS NULL OR c.expires_at > clock_timestamp())
			FROM worker_credentials c WHERE c.id = $1 FOR SHARE`, w.CredentialID).Scan(&credOK)
		if isNoRows(err) || (err == nil && !credOK) {
			return deny("credential_revoked")
		} else if err != nil {
			return err
		}
		var status string
		var free int
		err = tx.QueryRow(ctx, `SELECT w.status, w.capacity - (SELECT count(*) FROM job_attempts a WHERE a.worker_id = w.id AND a.state = 'active')
			FROM workers w WHERE w.id = $1 AND w.credential_id = $2 AND w.current_incarnation_id = $3 FOR UPDATE`,
			w.WorkerID, w.CredentialID, w.IncarnationID).Scan(&status, &free)
		if isNoRows(err) {
			return identityDenial(ctx, tx, w)
		} else if err != nil {
			return err
		}
		switch status {
		case "draining":
			return deny("worker_draining")
		case "deregistered":
			return deny("worker_deregistered")
		}
		if free < max {
			max = free
		}
		if max <= 0 {
			return nil // capacity_exhausted: empty claim, OK
		}
		rows, err := tx.Query(ctx, `SELECT j.id FROM (
				SELECT handler_name, handler_version FROM worker_capabilities
				WHERE worker_id = $1 AND (cardinality($3::text[]) = 0 OR (handler_name, handler_version) IN
					(SELECT * FROM unnest($3::text[], $4::int[])))) c
			CROSS JOIN LATERAL (
				SELECT q.id, q.priority, q.available_at, q.created_at FROM jobs q
				WHERE q.status = 'queued' AND q.handler_name = c.handler_name AND q.handler_version = c.handler_version
				  AND q.available_at <= now() AND q.attempts_used < q.max_attempts
				ORDER BY q.priority DESC, q.available_at, q.created_at, q.id
				LIMIT $2 FOR UPDATE SKIP LOCKED) j
			ORDER BY j.priority DESC, j.available_at, j.created_at, j.id LIMIT $2`, w.WorkerID, max, names, versions)
		if err != nil {
			return err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, id := range ids {
			var a ClaimedAttempt
			var version int64
			var payload []byte
			// Job first (deferred FK + NF-05 coupling trigger), then the attempt.
			err := tx.QueryRow(ctx, `UPDATE jobs SET status = 'running', attempts_used = attempts_used + 1,
					lease_generation = lease_generation + 1, current_attempt_id = uuidv7(),
					hard_deadline_at = clock_timestamp() + make_interval(secs => timeout_seconds),
					lease_expires_at = LEAST(clock_timestamp() + $2::interval, clock_timestamp() + make_interval(secs => timeout_seconds)),
					version = version + 1, updated_at = clock_timestamp()
				WHERE id = $1 AND status = 'queued' AND attempts_used < max_attempts
				RETURNING id, current_attempt_id, attempts_used, max_attempts, lease_generation, version, handler_name,
					handler_version, payload, lease_expires_at, hard_deadline_at, clock_timestamp()`, id, s.Timing.LeaseTTL).
				Scan(&a.JobID, &a.AttemptID, &a.AttemptNumber, &a.MaxAttempts, &a.LeaseGeneration, &version, &a.HandlerName,
					&a.HandlerVersion, &payload, &a.LeaseExpiresAt, &a.HardDeadlineAt, &a.Now)
			if err != nil { // 0 rows is impossible under the row lock; abort the whole claim, never partial
				return fmt.Errorf("claim CAS on %s: %w", id, err)
			}
			a.Payload = payload
			if err := s.fail("claim.after_job_update"); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO job_attempts (id, job_id, attempt_number, lease_generation, worker_id,
					worker_incarnation_id, started_at) VALUES ($1, $2, $3, $4, $5, $6, clock_timestamp())`,
				a.AttemptID, id, a.AttemptNumber, a.LeaseGeneration, w.WorkerID, w.IncarnationID); err != nil {
				return err
			}
			if err := s.fail("claim.after_attempt_insert"); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, attempt_id, actor, occurred_at, data)
				VALUES ($1, $2, 'claimed', 'queued', 'running', $3, 'worker:' || $4, clock_timestamp(), jsonb_build_object('attempt_number', $5::int))`,
				id, version, a.AttemptID, w.WorkerID, a.AttemptNumber); err != nil {
				return err
			}
			out = append(out, a)
		}
		return s.fail("claim.before_commit")
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AttemptRef is the fencing tuple of every attempt-scoped worker call.
type AttemptRef struct {
	WorkerRef
	JobID, AttemptID string
	Generation       int64
}

func (r AttemptRef) args() []any {
	return []any{r.JobID, r.AttemptID, r.Generation, r.WorkerID, r.IncarnationID, r.CredentialID}
}

// fence is the ADR-0003 predicate inside the CAS statement. $1 job, $2 attempt, $3 generation,
// $4 worker, $5 incarnation, $6 credential. Time is clock_timestamp(), read after the row locks.
const fence = `j.id = $1 AND j.status = 'running' AND j.current_attempt_id = $2 AND j.lease_generation = $3
	AND j.lease_expires_at > clock_timestamp() AND j.hard_deadline_at > clock_timestamp()
	AND a.id = $2 AND a.job_id = j.id AND a.state = 'active' AND a.worker_id = $4 AND a.worker_incarnation_id = $5
	AND w.id = $4 AND w.current_incarnation_id = $5 AND w.status <> 'deregistered' AND w.credential_id = $6
	AND c.id = w.credential_id AND c.revoked_at IS NULL AND (c.expires_at IS NULL OR c.expires_at > clock_timestamp())`

// lockForWorkerMutation takes the global lock order: credential (share), worker (share), then the
// job row (update). Nothing here writes the share-locked rows (NF-01 H1).
func lockForWorkerMutation(ctx context.Context, tx pgx.Tx, r AttemptRef) error {
	if _, err := tx.Exec(ctx, `SELECT 1 FROM worker_credentials WHERE id = (SELECT credential_id FROM workers WHERE id = $1) FOR SHARE`, r.WorkerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM workers WHERE id = $1 FOR SHARE`, r.WorkerID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT 1 FROM jobs WHERE id = $1 FOR UPDATE`, r.JobID)
	return err
}

// Outcome is the committed result of a report.
type Outcome string

// Outcomes (proto Outcome).
const (
	OutcomeSucceeded      Outcome = "succeeded"
	OutcomeRetryScheduled Outcome = "retry_scheduled"
	OutcomeFailed         Outcome = "failed"
	OutcomeCanceled       Outcome = "canceled"
)

// Reply is the canonical worker reply (RV01-03).
type Reply struct {
	Outcome         Outcome
	AlreadyFinal    bool
	NextAvailableAt *time.Time
}

// Bounds (ADR-0004).
const (
	MaxResultBytes       = 128 << 10
	MaxErrorMessageBytes = 2048
	MaxLogTailBytes      = 16384
	MaxErrorCodeLen      = 64
)

const truncMarker = "…[truncated]"

// TruncateMessage keeps a UTF-8-safe prefix of at most MaxErrorMessageBytes with a marker.
func TruncateMessage(s string) string {
	s = strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "")
	if len(s) <= MaxErrorMessageBytes {
		return s
	}
	cut := MaxErrorMessageBytes - len(truncMarker)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + truncMarker
}

// TailLog keeps the last MaxLogTailBytes (UTF-8 safe) with a marker prefix; truncated reports a cut.
func TailLog(s string) (string, bool) {
	s = strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "")
	if len(s) <= MaxLogTailBytes {
		return s, false
	}
	const marker = "[truncated]…"
	start := len(s) - (MaxLogTailBytes - len(marker))
	for start < len(s) && !utf8.RuneStart(s[start]) {
		start++
	}
	return marker + s[start:], true
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ReportSuccess commits success iff the fence holds and no cancel request is committed (E5). If the
// fence holds but a cancel committed first, the job is finalized canceled (reply OK canceled). A
// same-attempt retry of the committed outcome replies AlreadyFinal. Any other case is denied with no
// job/attempt/event change and one audit row.
func (s *Store) ReportSuccess(ctx context.Context, r AttemptRef, result json.RawMessage, logTail string) (Reply, error) {
	if len(result) > MaxResultBytes {
		return Reply{}, deny("result_too_large")
	}
	logTail, truncated := TailLog(logTail)
	var rep Reply
	var denial error
	err := s.inTx(ctx, true, func(tx pgx.Tx) error {
		rep, denial = Reply{}, nil
		if err := lockForWorkerMutation(ctx, tx, r); err != nil {
			return err
		}
		var version int64
		err := tx.QueryRow(ctx, `UPDATE jobs j SET status = 'succeeded', result = $7::jsonb, terminal_at = clock_timestamp(),
				current_attempt_id = NULL, lease_expires_at = NULL, hard_deadline_at = NULL, version = j.version + 1,
				updated_at = clock_timestamp()
			FROM job_attempts a, workers w, worker_credentials c
			WHERE `+fence+` AND j.cancel_requested_at IS NULL RETURNING j.version`, append(r.args(), string(result))...).Scan(&version)
		if err == nil {
			rep = Reply{Outcome: OutcomeSucceeded}
			return s.finishAttempt(ctx, tx, r, version, finish{attemptState: "succeeded", event: "succeeded", to: "succeeded",
				result: result, logTail: logTail, truncated: truncated})
		}
		if !isNoRows(err) {
			return err
		}
		err = tx.QueryRow(ctx, `UPDATE jobs j SET status = 'canceled', terminal_at = clock_timestamp(), failure_class = 'canceled_by_operator',
				current_attempt_id = NULL, lease_expires_at = NULL, hard_deadline_at = NULL, version = j.version + 1,
				updated_at = clock_timestamp()
			FROM job_attempts a, workers w, worker_credentials c
			WHERE `+fence+` AND j.cancel_requested_at IS NOT NULL RETURNING j.version`, r.args()...).Scan(&version)
		if err == nil {
			rep = Reply{Outcome: OutcomeCanceled}
			return s.finishAttempt(ctx, tx, r, version, finish{attemptState: "canceled", event: "canceled", to: "canceled",
				result: result, logTail: logTail, truncated: truncated,
				data: map[string]any{"worker_reported": "success", "failure_class": "canceled_by_operator"}})
		}
		if !isNoRows(err) {
			return err
		}
		rep, denial, err = s.classify(ctx, tx, r, "worker.report_success", result, "")
		return err
	})
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23514" && pe.ConstraintName == "jobs_result_check" {
			return Reply{}, deny("result_too_large") // jsonb text form exceeds 128 KiB
		}
		return Reply{}, err
	}
	return rep, denial
}

// FailureParams is a ReportFailure request.
type FailureParams struct {
	Permanent         bool
	Code, Message     string
	LogTail           string
	CanceledByHandler bool
}

// ReportFailure finalizes the current attempt as failed (E6/E7): cancel requested => canceled;
// retryable with budget => retry_wait after backoff; retryable without budget => failed
// retryable_exhausted; permanent => failed permanent.
func (s *Store) ReportFailure(ctx context.Context, r AttemptRef, f FailureParams) (Reply, error) {
	code := f.Code
	if code == "" || len(code) > MaxErrorCodeLen {
		code = "handler_error"
	}
	msg := TruncateMessage(f.Message)
	logTail, truncated := TailLog(f.LogTail)
	class := "retryable"
	if f.Permanent {
		class = "permanent"
	}
	var rep Reply
	var denial error
	err := s.inTx(ctx, true, func(tx pgx.Tx) error {
		rep, denial = Reply{}, nil
		if err := lockForWorkerMutation(ctx, tx, r); err != nil {
			return err
		}
		var cancel bool
		var used, maxA int
		err := tx.QueryRow(ctx, `SELECT j.cancel_requested_at IS NOT NULL, j.attempts_used, j.max_attempts
			FROM jobs j, job_attempts a, workers w, worker_credentials c WHERE `+fence, r.args()...).Scan(&cancel, &used, &maxA)
		if isNoRows(err) {
			rep, denial, err = s.classify(ctx, tx, r, "worker.report_failure", nil, class)
			return err
		} else if err != nil {
			return err
		}
		var to, event, failureClass string
		var backoff time.Duration
		data := map[string]any{"error_code": code, "attempt_number": used}
		switch {
		case cancel:
			to, event, failureClass = "canceled", "canceled", "canceled_by_operator"
			data["worker_reported"] = "failure"
		case f.Permanent:
			to, event, failureClass = "failed", "failed_permanent", "permanent"
		case used < maxA:
			to, event = "retry_wait", "failed_retryable"
			backoff = s.Timing.Backoff.Delay(used)
			data["backoff_ms"] = backoff.Milliseconds()
		default:
			to, event, failureClass = "failed", "attempts_exhausted", "retryable_exhausted"
		}
		if failureClass != "" {
			data["failure_class"] = failureClass
		}
		var version int64
		var next time.Time
		err = tx.QueryRow(ctx, `UPDATE jobs j SET status = $7,
				terminal_at = CASE WHEN $7 IN ('failed', 'canceled') THEN clock_timestamp() END,
				available_at = CASE WHEN $7 = 'retry_wait' THEN clock_timestamp() + $8::interval ELSE j.available_at END,
				error_code = CASE WHEN $7 = 'canceled' THEN NULL ELSE $9 END,
				error_message = CASE WHEN $7 = 'canceled' THEN NULL ELSE $10 END,
				failure_class = $11, current_attempt_id = NULL, lease_expires_at = NULL, hard_deadline_at = NULL,
				version = j.version + 1, updated_at = clock_timestamp()
			FROM job_attempts a, workers w, worker_credentials c
			WHERE `+fence+` RETURNING j.version, j.available_at`,
			append(r.args(), to, backoff, code, nullIfEmpty(msg), nullIfEmpty(failureClass))...).Scan(&version, &next)
		if err != nil { // fence re-checked under the same job lock; 0 rows here is a bug, abort
			return fmt.Errorf("report failure CAS: %w", err)
		}
		switch to {
		case "canceled":
			rep = Reply{Outcome: OutcomeCanceled}
		case "retry_wait":
			rep = Reply{Outcome: OutcomeRetryScheduled, NextAvailableAt: &next}
		default:
			rep = Reply{Outcome: OutcomeFailed}
		}
		attemptState := "failed"
		if to == "canceled" {
			attemptState = "canceled"
		}
		return s.finishAttempt(ctx, tx, r, version, finish{attemptState: attemptState, event: event, to: to,
			errClass: class, errCode: code, errMsg: msg, logTail: logTail, truncated: truncated, data: data})
	})
	if err != nil {
		return Reply{}, err
	}
	return rep, denial
}

type finish struct {
	attemptState, event, to   string
	result                    json.RawMessage
	errClass, errCode, errMsg string
	logTail                   string
	truncated                 bool
	data                      map[string]any
}

func (s *Store) finishAttempt(ctx context.Context, tx pgx.Tx, r AttemptRef, version int64, f finish) error {
	var hash any
	if f.result != nil {
		hash = sha256Bytes(f.result)
	}
	errClass := any(nil)
	if f.attemptState == "failed" {
		errClass = f.errClass
	}
	tag, err := tx.Exec(ctx, `UPDATE job_attempts SET state = $2, finished_at = clock_timestamp(), result_sha256 = $3,
			error_class = $4, error_code = $5, error_message = $6, log_tail = $7, log_truncated = $8
		WHERE id = $1 AND state = 'active'`,
		r.AttemptID, f.attemptState, hash, errClass, nullIfEmpty(f.errCode), nullIfEmpty(f.errMsg), nullIfEmpty(f.logTail), f.truncated)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("attempt %s not active", r.AttemptID)
	}
	if err := s.fail("report.before_event"); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, attempt_id, actor, occurred_at, data)
		VALUES ($1, $2, $3, 'running', $4, $5, 'worker:' || $6, clock_timestamp(), $7)`,
		r.JobID, version, f.event, f.to, r.AttemptID, r.WorkerID, jsonOrNull(f.data))
	return err
}

// classify picks the reply or denial after a 0-row CAS (state-machine.md order) and writes one bounded
// audit row for a denial. result is the reported success body (nil for failures); failClass is
// "retryable"/"permanent" for failure reports.
func (s *Store) classify(ctx context.Context, tx pgx.Tx, r AttemptRef, action string, result json.RawMessage, failClass string) (Reply, error, error) {
	var status, attemptState string
	var cur *string
	var gen int64
	var leaseOK, deadlineOK, owner, incOK, deregistered, credMatch, credValid, sameResult bool
	var attemptClass *string
	var hash []byte
	if result != nil {
		hash = sha256Bytes(result)
	}
	err := tx.QueryRow(ctx, `SELECT j.status, j.current_attempt_id::text, j.lease_generation,
			coalesce(j.lease_expires_at > clock_timestamp(), false), coalesce(j.hard_deadline_at > clock_timestamp(), false),
			a.state, (a.worker_id = $3 AND a.worker_incarnation_id = $4), (w.current_incarnation_id = $4),
			w.status = 'deregistered',
			w.credential_id = $5, (c.revoked_at IS NULL AND (c.expires_at IS NULL OR c.expires_at > clock_timestamp())),
			coalesce(a.result_sha256 = $6, false), a.error_class
		FROM jobs j JOIN job_attempts a ON a.id = $2 AND a.job_id = j.id
		JOIN workers w ON w.id = $3 JOIN worker_credentials c ON c.id = w.credential_id
		WHERE j.id = $1`, r.JobID, r.AttemptID, r.WorkerID, r.IncarnationID, r.CredentialID, hash).
		Scan(&status, &cur, &gen, &leaseOK, &deadlineOK, &attemptState, &owner, &incOK, &deregistered, &credMatch, &credValid, &sameResult, &attemptClass)
	terminal := status == "succeeded" || status == "failed" || status == "canceled"
	credOK := credMatch && credValid
	// credential_mismatch: a valid credential presented for a worker bound to another credential (CR-M5-01).
	credReason := "credential_revoked"
	if !credMatch {
		credReason = "credential_mismatch"
	}
	reason := ""
	switch {
	case isNoRows(err):
		reason = "stale_attempt"
	case err != nil:
		return Reply{}, nil, err
	case terminal && !owner: // someone else's attempt (RV01-07)
		reason = "stale_attempt"
	case terminal && !credOK:
		reason = credReason
	case status == "succeeded" && attemptState == "succeeded" && result != nil && sameResult:
		return Reply{Outcome: OutcomeSucceeded, AlreadyFinal: true}, nil, nil
	case status == "succeeded" && attemptState == "succeeded" && result != nil:
		reason = "result_mismatch"
	case status == "canceled" && attemptState == "canceled":
		return Reply{Outcome: OutcomeCanceled, AlreadyFinal: true}, nil, nil
	case status == "failed" && attemptState == "failed" && failClass != "" && attemptClass != nil && *attemptClass == failClass:
		return Reply{Outcome: OutcomeFailed, AlreadyFinal: true}, nil, nil
	case terminal:
		reason = "job_terminal"
	case status != "running" || cur == nil || *cur != r.AttemptID || gen != r.Generation:
		reason = "stale_attempt"
	case !deadlineOK:
		reason = "deadline_exceeded"
	case !leaseOK:
		reason = "lease_expired"
	case !owner || !incOK:
		reason = "incarnation_mismatch"
	case deregistered:
		reason = "worker_deregistered"
	case !credOK:
		reason = credReason
	default:
		reason = "stale_attempt"
	}
	if err := s.audit(ctx, tx, r, action, reason); err != nil {
		return Reply{}, nil, err
	}
	return Reply{}, deny(reason), nil
}

func auditCategory(reason string) string {
	switch reason {
	case "credential_revoked", "credential_mismatch", "incarnation_mismatch", "worker_deregistered", "result_mismatch":
		return "security"
	}
	return "diagnostic"
}

func (s *Store) audit(ctx context.Context, tx pgx.Tx, r AttemptRef, action, reason string) error {
	if s.DenialAudit != nil && !s.DenialAudit(r.CredentialID) {
		return nil // per-credential budget spent: a misbehaving worker cannot flood audit_events
	}
	_, err := tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id, occurred_at, details)
		VALUES ($6, $7, 'rejected', 'worker:' || $1, 'job', $2, clock_timestamp(),
		        jsonb_build_object('reason', $3::text, 'attempt_id', $4::text, 'lease_generation', $5::bigint))`,
		r.WorkerID, r.JobID, reason, r.AttemptID, r.Generation, auditCategory(reason), action)
	return err
}

// Lease is the RenewLease / PollCancellation reply.
type Lease struct {
	ExpiresAt       time.Time
	Now             time.Time
	CancelRequested bool
	JobTerminal     bool
}

// RenewLease extends the current lease (E4): LEAST(now+ttl, hard deadline, cancel_requested_at+grace).
// No event, no version bump. A draining worker past its drain deadline is denied worker_draining (E12).
func (s *Store) RenewLease(ctx context.Context, r AttemptRef) (Lease, error) {
	var out Lease
	var denial error
	err := s.inTx(ctx, true, func(tx pgx.Tx) error {
		out, denial = Lease{}, nil
		if err := lockForWorkerMutation(ctx, tx, r); err != nil {
			return err
		}
		var draining bool
		err := tx.QueryRow(ctx, `SELECT w.status = 'draining' AND w.drain_deadline_at <= clock_timestamp()
			FROM jobs j, job_attempts a, workers w, worker_credentials c WHERE `+fence, r.args()...).Scan(&draining)
		if isNoRows(err) {
			_, denial, err = s.classify(ctx, tx, r, "worker.renew_lease", nil, "")
			return err
		} else if err != nil {
			return err
		}
		if draining {
			denial = deny("worker_draining")
			return s.audit(ctx, tx, r, "worker.renew_lease", "worker_draining")
		}
		return tx.QueryRow(ctx, `UPDATE jobs j SET lease_expires_at = LEAST(clock_timestamp() + $7::interval, j.hard_deadline_at,
				COALESCE(j.cancel_requested_at + $8::interval, 'infinity'::timestamptz))
			FROM job_attempts a, workers w, worker_credentials c
			WHERE `+fence+` RETURNING j.lease_expires_at, clock_timestamp(), j.cancel_requested_at IS NOT NULL`,
			append(r.args(), s.Timing.LeaseTTL, s.Timing.CancelGrace)...).Scan(&out.ExpiresAt, &out.Now, &out.CancelRequested)
	})
	if err != nil {
		return Lease{}, err
	}
	if denial == nil && out.ExpiresAt.IsZero() {
		// classify returned an idempotent reply for a terminal attempt: a renewal of it is job_terminal.
		return Lease{}, deny("job_terminal")
	}
	return out, denial
}

// PollCancellation reports the cancel flag for the caller's current attempt (E10). Read-only.
func (s *Store) PollCancellation(ctx context.Context, r AttemptRef) (Lease, error) {
	var out Lease
	err := s.Pool.QueryRow(ctx, `SELECT j.lease_expires_at, clock_timestamp(), j.cancel_requested_at IS NOT NULL
		FROM jobs j, job_attempts a, workers w, worker_credentials c WHERE `+fence, r.args()...).
		Scan(&out.ExpiresAt, &out.Now, &out.CancelRequested)
	if err == nil {
		return out, nil
	}
	if !isNoRows(err) {
		return Lease{}, classifyDBError(err)
	}
	var status string
	var owner bool
	err = s.Pool.QueryRow(ctx, `SELECT j.status, a.worker_id = $3 FROM jobs j JOIN job_attempts a ON a.id = $2 AND a.job_id = j.id
		WHERE j.id = $1`, r.JobID, r.AttemptID, r.WorkerID).Scan(&status, &owner)
	switch {
	case isNoRows(err):
		return Lease{}, deny("stale_attempt")
	case err != nil:
		return Lease{}, classifyDBError(err)
	case owner && (status == "succeeded" || status == "failed"):
		return Lease{JobTerminal: true}, nil
	case owner && status == "canceled":
		return Lease{JobTerminal: true, CancelRequested: true}, nil
	}
	// Non-terminal: same denial order (and diagnostic audit row) as mutations.
	var d error
	err = s.inTx(ctx, false, func(tx pgx.Tx) error {
		var e error
		_, d, e = s.classify(ctx, tx, r, "worker.poll_cancellation", nil, "")
		return e
	})
	if err != nil {
		return Lease{}, err
	}
	if d == nil {
		d = deny("stale_attempt")
	}
	return Lease{}, d
}
