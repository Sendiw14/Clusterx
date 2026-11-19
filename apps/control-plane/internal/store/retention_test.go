package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// loginRole creates a LOGIN role in group and returns a pool connected as it.
func loginRole(t *testing.T, s *Store, group string) *pgxpool.Pool {
	t.Helper()
	role := "cx_" + group[9:12] + "_" + NewUUID()[:8]
	exec(t, s, `CREATE ROLE `+role+` LOGIN IN ROLE `+group)
	t.Cleanup(func() {
		_, _ = s.Pool.Exec(context.Background(), `DROP OWNED BY `+role)
		_, _ = s.Pool.Exec(context.Background(), `DROP ROLE `+role)
	})
	cfg := s.Pool.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.MaxConns = 2
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// age rewrites timestamps of fixtures (superuser, guard triggers disabled for the fixture statement only).
func age(t *testing.T, s *Store, table, trigger, sql string, args ...any) {
	t.Helper()
	exec(t, s, `ALTER TABLE `+table+` DISABLE TRIGGER `+trigger)
	exec(t, s, sql, args...)
	exec(t, s, `ALTER TABLE `+table+` ENABLE TRIGGER `+trigger)
}

// Retention (ADR-0004, ticket deliverable 7): runs as clusterx_retention, deletes only past each floor,
// writes tombstones in the same statement, nulls lineage, expires logs, and is refused to the runtime role.
func TestRetentionPass(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 8, "h")
	ok := json.RawMessage(`{"ok":true}`)
	finish := func(perm bool) string {
		id := admit(t, s, "h", func(p *AdmitParams) { p.MaxAttempts = 1 })
		a := claimOne(t, s, w)
		var err error
		if perm {
			_, err = s.ReportFailure(ctx, a, FailureParams{Permanent: true, Code: "x", Message: "m", LogTail: "log line"})
		} else {
			_, err = s.ReportSuccess(ctx, a, ok, "log line")
		}
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	oldDone, youngDone, oldParent := finish(false), finish(false), finish(true)
	child, _, err := s.ManualRetry(ctx, oldParent, "user:x", "", AuditMeta{})
	if err != nil {
		t.Fatal(err)
	}
	running := admit(t, s, "h")
	claimOne(t, s, w)
	for _, id := range []string{oldDone, oldParent} {
		age(t, s, "jobs", "jobs_terminal_immutable", `UPDATE jobs SET
			updated_at = now() - interval '31 days', terminal_at = now() - interval '31 days' WHERE id = $1`, id)
	}
	age(t, s, "job_attempts", "job_attempts_finished_immutable", `UPDATE job_attempts SET started_at = now() - interval '9 days',
		finished_at = now() - interval '8 days' WHERE job_id = $1`, youngDone)
	age(t, s, "audit_events", "audit_events_append_only", `UPDATE audit_events SET occurred_at = now() - interval '91 days'
		WHERE id = (SELECT id FROM audit_events ORDER BY occurred_at LIMIT 1)`)
	exec(t, s, `INSERT INTO job_tombstones (job_id, terminal_at, expired_at) VALUES (uuidv7(), now() - interval '200 days', now() - interval '100 days')`)
	uid, err := s.UpsertUser(ctx, 77, "u77", "viewer")
	if err != nil {
		t.Fatal(err)
	}
	exec(t, s, `INSERT INTO sessions (user_id, secret_hash, created_at, last_activity_at, expires_at)
		VALUES ($1, sha256('a'), now() - interval '20 days', now() - interval '20 days', now() - interval '8 days'),
		       ($1, sha256('b'), now() - interval '1 day', now() - interval '1 day', now() - interval '1 hour')`, uid)
	exec(t, s, `INSERT INTO submission_idempotency (principal, namespace, idem_key, fingerprint, job_id, created_at, expires_at)
		VALUES ('p', 'default', 'k', sha256('x'), $1, now() - interval '2 days', now() - interval '1 day')`, running)
	auditBefore := q[int](t, s, `SELECT count(*) FROM audit_events`)

	// The runtime role may not run retention deletes.
	rt := loginRole(t, s, "clusterx_runtime")
	if _, err := RunRetention(ctx, rt, RetentionBounds{Batch: 10, MaxBatches: 2}); err == nil {
		t.Fatal("runtime role ran retention")
	} else if pe := (*pgconn.PgError)(nil); !errors.As(err, &pe) || pe.Code != "42501" {
		t.Fatalf("runtime retention error: %v", err)
	}

	ret := loginRole(t, s, "clusterx_retention")
	r, err := RunRetention(ctx, ret, RetentionBounds{Batch: 1, MaxBatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	want := RetentionReport{AttemptLogsExpired: 1, JobsDeleted: 2, AuditDeleted: 1, TombstonesDeleted: 1, IdempotencyDeleted: 1,
		SessionsDeleted: 1, Complete: true}
	if r != want {
		t.Fatalf("report %+v want %+v", r, want)
	}
	for _, id := range []string{oldDone, oldParent} {
		if _, err := s.GetJob(ctx, id); !IsDenial(err, "expired") {
			t.Fatalf("deleted job %s: %v, want 410 expired", id, err)
		}
		if n := q[int](t, s, `SELECT (SELECT count(*) FROM job_events WHERE job_id = $1) + (SELECT count(*) FROM job_attempts WHERE job_id = $1)`, id); n != 0 {
			t.Fatalf("history of %s left: %d", id, n)
		}
	}
	if got := q[*string](t, s, `SELECT retried_from_job_id::text FROM jobs WHERE id = $1`, child.ID); got != nil {
		t.Fatal("child lineage not nulled")
	}
	for _, id := range []string{youngDone, running} {
		if _, err := s.GetJob(ctx, id); err != nil {
			t.Fatalf("young/non-terminal job %s removed: %v", id, err)
		}
	}
	if ok := q[bool](t, s, `SELECT bool_and(log_tail IS NULL AND log_expired) FROM job_attempts WHERE job_id = $1`, youngDone); !ok {
		t.Fatal("8-day-old attempt log not expired")
	}
	if n := q[int](t, s, `SELECT count(*) FROM audit_events`); n != auditBefore-1 {
		t.Fatalf("audit rows %d want %d", n, auditBefore-1)
	}
	// Floors hold whatever the retention role does directly.
	for _, sql := range []string{`DELETE FROM jobs WHERE id = '` + youngDone + `'`, `DELETE FROM audit_events`} {
		var pe *pgconn.PgError
		if _, err := ret.Exec(ctx, sql); !errors.As(err, &pe) || pe.Code != "23514" {
			t.Fatalf("%s: %v, want check_violation", sql, err)
		}
	}
	// A second pass is a no-op.
	if r2, err := RunRetention(ctx, ret, DefaultRetentionBounds()); err != nil || (r2 != RetentionReport{Complete: true}) {
		t.Fatalf("second pass %+v %v", r2, err)
	}
	// Bounded: MaxBatches stops a pass early and says so.
	w2 := newWorker(t, s, 8, "h2")
	for range 3 {
		id := admit(t, s, "h2", func(p *AdmitParams) { p.MaxAttempts = 1 })
		if _, err := s.ReportSuccess(ctx, claimOne(t, s, w2), ok, ""); err != nil {
			t.Fatal(err)
		}
		age(t, s, "jobs", "jobs_terminal_immutable", `UPDATE jobs SET
			updated_at = now() - interval '31 days', terminal_at = now() - interval '31 days' WHERE id = $1`, id)
	}
	if r3, err := RunRetention(ctx, ret, RetentionBounds{Batch: 1, MaxBatches: 2}); err != nil || r3.JobsDeleted != 2 || r3.Complete {
		t.Fatalf("bounded pass %+v %v", r3, err)
	}
	_ = pgx.ErrNoRows
}
