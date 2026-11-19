package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Conflict is a 409 denial carrying the current {status, version} (state-machine.md "Conflict and
// expiry responses").
type Conflict struct {
	Reason  string
	Status  string
	Version int64
	Extra   map[string]any
}

func (c *Conflict) Error() string { return "conflict: " + c.Reason }

// CancelJob implements POST /jobs/{id}/cancel (E9) with the privileged audit row job.cancel in the same
// transaction. Already canceled is a 200 no-op (no new audit row, no event); succeeded/failed is a
// 409 job_terminal. Denials: job_not_found / expired.
func (s *Store) CancelJob(ctx context.Context, jobID, actor string, m AuditMeta) (CancelResult, error) {
	res, err := s.requestCancel(ctx, jobID, actor, &m)
	if IsDenial(err, "job_not_found") {
		return res, s.missingJob(ctx, jobID)
	}
	return res, err
}

// RequestCancel implements E9 without an audit row (scheduler-side and tests).
func (s *Store) RequestCancel(ctx context.Context, jobID, actor string) (CancelResult, error) {
	return s.requestCancel(ctx, jobID, actor, nil)
}

func (s *Store) requestCancel(ctx context.Context, jobID, actor string, m *AuditMeta) (CancelResult, error) {
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
		changed := false
		switch {
		case status == "scheduled" || status == "queued" || status == "retry_wait":
			if err := tx.QueryRow(ctx, `UPDATE jobs SET status = 'canceled', terminal_at = clock_timestamp(),
					cancel_requested_at = clock_timestamp(), failure_class = 'canceled_by_operator', cancel_requested_by = $2,
					version = version + 1, updated_at = clock_timestamp()
				WHERE id = $1 AND status IN ('scheduled', 'queued', 'retry_wait') RETURNING version`, jobID, actor).Scan(&version); err != nil {
				return err
			}
			res, changed = CancelResult{200, "canceled", version}, true
			if _, err := tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor, occurred_at, data)
				VALUES ($1, $2, 'canceled', $3, 'canceled', $4, clock_timestamp(), '{"failure_class":"canceled_by_operator"}')`, jobID, version, status, actor); err != nil {
				return err
			}
		case status == "running" && !requested:
			if err := tx.QueryRow(ctx, `UPDATE jobs SET cancel_requested_at = clock_timestamp(), cancel_requested_by = $2,
					lease_expires_at = LEAST(lease_expires_at, clock_timestamp() + $3::interval), version = version + 1,
					updated_at = clock_timestamp()
				WHERE id = $1 AND status = 'running' AND cancel_requested_at IS NULL RETURNING version`, jobID, actor, s.Timing.CancelGrace).Scan(&version); err != nil {
				return err
			}
			res, changed = CancelResult{202, "running", version}, true
			if _, err := tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor, occurred_at)
				VALUES ($1, $2, 'cancel_requested', 'running', 'running', $3, clock_timestamp())`, jobID, version, actor); err != nil {
				return err
			}
		case status == "running":
			res = CancelResult{202, "running", version}
		case status == "canceled":
			res = CancelResult{200, "canceled", version}
		default:
			res = CancelResult{409, status, version}
		}
		if m == nil || !changed {
			return nil
		}
		return insertAudit(ctx, tx, AuditRow{Category: "privileged", Action: "job.cancel", Outcome: "allowed", Actor: actor,
			TargetType: "job", TargetID: jobID, RequestID: m.RequestID,
			Details: map[string]any{"from_status": status, "http": res.HTTP, "version": version}})
	})
	return res, err
}

// ManualRetry implements POST /jobs/{id}/retry (E11): a terminal failed parent gets one new linked child
// job (queued, fresh attempt budget, same submission fields) whose first event is retry_created; the
// parent row is untouched. The parent row lock serializes concurrent retries. With an idempotency key
// (namespace manual-retry:<parent>, ADR-0004) a repeat returns the child (replayed=true); a child that
// already exists otherwise is 409 retry_already_exists. The privileged audit row job.manual_retry is in
// the same transaction.
func (s *Store) ManualRetry(ctx context.Context, parentID, actor, key string, m AuditMeta) (Job, bool, error) {
	var childID string
	replayed := false
	fp := sha256Bytes([]byte("manual-retry:" + parentID))
	err := s.inTx(ctx, false, func(tx pgx.Tx) error {
		childID, replayed = "", false
		var status string
		var version int64
		var child *string
		err := tx.QueryRow(ctx, `SELECT status, version, (SELECT c.id::text FROM jobs c WHERE c.retried_from_job_id = j.id)
			FROM jobs j WHERE id = $1 FOR UPDATE`, parentID).Scan(&status, &version, &child)
		if isNoRows(err) {
			return deny("job_not_found")
		} else if err != nil {
			return err
		}
		if status != "failed" {
			return &Conflict{Reason: "retry_not_allowed", Status: status, Version: version}
		}
		if child != nil {
			if key != "" {
				var orig string
				err := tx.QueryRow(ctx, `SELECT job_id FROM submission_idempotency WHERE principal = $1 AND namespace = $2
					AND idem_key = $3 AND expires_at > now()`, actor, "manual-retry:"+parentID, key).Scan(&orig)
				if err == nil && orig == *child {
					childID, replayed = orig, true
					return nil
				} else if err != nil && !isNoRows(err) {
					return err
				}
			}
			return &Conflict{Reason: "retry_already_exists", Status: status, Version: version,
				Extra: map[string]any{"retried_as_job_id": *child}}
		}
		if err := tx.QueryRow(ctx, `INSERT INTO jobs (namespace, handler_name, handler_version, payload, priority, status,
				available_at, max_attempts, timeout_seconds, created_by, retried_from_job_id, created_at, updated_at)
			SELECT namespace, handler_name, handler_version, payload, priority, 'queued', clock_timestamp(), max_attempts,
				timeout_seconds, $2, id, clock_timestamp(), clock_timestamp()
			FROM jobs WHERE id = $1 RETURNING id`, parentID, actor).Scan(&childID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, to_status, actor, occurred_at, data)
			VALUES ($1, 1, 'retry_created', 'queued', $2, clock_timestamp(), jsonb_build_object('retried_from_job_id', $3::text))`,
			childID, actor, parentID); err != nil {
			return err
		}
		if key != "" {
			if _, err := tx.Exec(ctx, `INSERT INTO submission_idempotency (principal, namespace, idem_key, fingerprint, job_id, created_at, expires_at)
				VALUES ($1, $2, $3, $4, $5, now(), now() + $6::interval)
				ON CONFLICT (principal, namespace, idem_key) DO UPDATE SET fingerprint = EXCLUDED.fingerprint, job_id = EXCLUDED.job_id,
					created_at = EXCLUDED.created_at, expires_at = EXCLUDED.expires_at`,
				actor, "manual-retry:"+parentID, key, fp, childID, s.Timing.IdempotencyTTL); err != nil {
				return err
			}
		}
		if err := s.fail("retry.before_commit"); err != nil {
			return err
		}
		return insertAudit(ctx, tx, AuditRow{Category: "privileged", Action: "job.manual_retry", Outcome: "allowed", Actor: actor,
			TargetType: "job", TargetID: parentID, RequestID: m.RequestID, Details: map[string]any{"retried_as_job_id": childID}})
	})
	if IsDenial(err, "job_not_found") {
		return Job{}, false, s.missingJob(ctx, parentID)
	}
	if err != nil {
		return Job{}, false, err
	}
	j, err := s.GetJob(ctx, childID)
	return j, replayed, err
}

// OperatorDrain implements POST /workers/{id}/drain (E12 operator path): status draining with
// drain_deadline_at = now + grace; an already draining worker keeps its original deadline (idempotent,
// no audit row). New claims are denied at once (worker_draining); renewals are denied after the
// deadline, so the remaining attempts are recovered by lease expiry (E8), not canceled. A deregistered
// worker is 409 worker_deregistered. Audit worker.drain in the same transaction.
func (s *Store) OperatorDrain(ctx context.Context, workerID string, grace time.Duration, actor string, m AuditMeta) error {
	if grace < 0 {
		grace = 0
	}
	if grace > s.Timing.DrainGraceMax && s.Timing.DrainGraceMax > 0 {
		grace = s.Timing.DrainGraceMax
	}
	return s.inTx(ctx, false, func(tx pgx.Tx) error {
		var status string
		var version int64
		err := tx.QueryRow(ctx, `SELECT status, version FROM workers WHERE id = $1 FOR UPDATE`, workerID).Scan(&status, &version)
		if isNoRows(err) {
			return deny("worker_not_found")
		} else if err != nil {
			return err
		}
		switch status {
		case "deregistered":
			return &Conflict{Reason: "worker_deregistered", Status: status, Version: version}
		case "draining":
			return nil
		}
		var deadline time.Time
		if err := tx.QueryRow(ctx, `UPDATE workers SET status = 'draining', drain_requested_at = clock_timestamp(),
				drain_deadline_at = clock_timestamp() + $2::interval, version = version + 1
			WHERE id = $1 AND status = 'active' RETURNING drain_deadline_at, version`, workerID, grace).Scan(&deadline, &version); err != nil {
			return err
		}
		return insertAudit(ctx, tx, AuditRow{Category: "privileged", Action: "worker.drain", Outcome: "allowed", Actor: actor,
			TargetType: "worker", TargetID: workerID, RequestID: m.RequestID,
			Details: map[string]any{"grace_seconds": int(grace / time.Second), "drain_deadline_at": deadline.UTC().Format(time.RFC3339Nano)}})
	})
}
