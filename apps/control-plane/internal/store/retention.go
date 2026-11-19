package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RetentionReport counts what one retention pass removed (ADR-0004 retention table).
type RetentionReport struct {
	AttemptLogsExpired int64 `json:"attempt_logs_expired"`
	JobsDeleted        int64 `json:"jobs_deleted"` // each leaves a tombstone (410 expired)
	AuditDeleted       int64 `json:"audit_deleted"`
	TombstonesDeleted  int64 `json:"tombstones_deleted"`
	IdempotencyDeleted int64 `json:"idempotency_deleted"`
	SessionsDeleted    int64 `json:"sessions_deleted"`
	Complete           bool  `json:"complete"` // false when a bound stopped the pass early
}

// Retention bounds: Batch rows per statement, MaxBatches statements per category per pass.
type RetentionBounds struct {
	Batch, MaxBatches int
}

// DefaultRetentionBounds deletes at most 20 x 1000 rows per category per pass.
func DefaultRetentionBounds() RetentionBounds { return RetentionBounds{Batch: 1000, MaxBatches: 20} }

// retentionSteps run as clusterx_retention: every statement is one short transaction, bounded by $1.
// The DB guards (migration 0002) refuse anything younger than its floor whatever the caller does:
// jobs_retention_delete_only (terminal >= 30 d), audit_events_no_young_delete (>= 90 d), the finished
// attempt trigger (log_tail only, >= 7 d). Tombstones are written in the same statement as the delete.
var retentionSteps = []struct {
	name string
	sql  string
}{
	{"attempt_logs", `SELECT clusterx_expire_attempt_logs($1)::bigint`},
	{"jobs", `WITH d AS (
			DELETE FROM jobs WHERE id IN (
				SELECT id FROM jobs WHERE terminal_at <= now() - interval '30 days' ORDER BY terminal_at LIMIT $1)
			RETURNING id, terminal_at),
		t AS (INSERT INTO job_tombstones (job_id, terminal_at, expired_at)
			SELECT id, terminal_at, clock_timestamp() FROM d ON CONFLICT (job_id) DO NOTHING RETURNING 1)
		SELECT count(*) FROM d`},
	{"audit", `WITH d AS (DELETE FROM audit_events WHERE id IN (
			SELECT id FROM audit_events WHERE occurred_at <= now() - interval '90 days' ORDER BY occurred_at LIMIT $1) RETURNING 1)
		SELECT count(*) FROM d`},
	{"tombstones", `WITH d AS (DELETE FROM job_tombstones WHERE job_id IN (
			SELECT job_id FROM job_tombstones WHERE expired_at <= now() - interval '90 days' ORDER BY expired_at LIMIT $1) RETURNING 1)
		SELECT count(*) FROM d`},
	{"idempotency", `WITH d AS (DELETE FROM submission_idempotency WHERE ctid IN (
			SELECT ctid FROM submission_idempotency WHERE expires_at <= now() LIMIT $1) RETURNING 1)
		SELECT count(*) FROM d`},
	{"sessions", `WITH d AS (DELETE FROM sessions WHERE id IN (
			SELECT id FROM sessions WHERE expires_at <= now() - interval '7 days' ORDER BY expires_at LIMIT $1) RETURNING 1)
		SELECT count(*) FROM d`},
}

// RunRetention performs one bounded pass with a pool whose role is a clusterx_retention member.
func RunRetention(ctx context.Context, pool *pgxpool.Pool, b RetentionBounds) (RetentionReport, error) {
	var r RetentionReport
	r.Complete = true
	dst := map[string]*int64{"attempt_logs": &r.AttemptLogsExpired, "jobs": &r.JobsDeleted, "audit": &r.AuditDeleted,
		"tombstones": &r.TombstonesDeleted, "idempotency": &r.IdempotencyDeleted, "sessions": &r.SessionsDeleted}
	for _, st := range retentionSteps {
		for i := 0; ; i++ {
			if i == b.MaxBatches {
				r.Complete = false
				break
			}
			var n int64
			if err := pool.QueryRow(ctx, st.sql, b.Batch).Scan(&n); err != nil {
				return r, fmt.Errorf("retention %s: %w", st.name, classifyDBError(err))
			}
			*dst[st.name] += n
			if n < int64(b.Batch) {
				break
			}
		}
	}
	return r, nil
}
