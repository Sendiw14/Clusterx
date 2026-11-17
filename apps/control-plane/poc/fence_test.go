package poc

// TASK-M1-BE-04: fence-clause, reply-contract, schema-guard, token-authority and forced-ordering tests
// (RV01-01..09). Each fence case names the mutant (REVIEW-M1-RV-01 table) it must kill.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func execErr(sql string, args ...any) error {
	_, err := pool.Exec(context.Background(), sql, args...)
	return err
}

// snapshot captures everything a denied report must leave untouched.
func snapshot(t *testing.T, job string) string {
	return q[string](t, `SELECT j.status || '/' || j.version || '/' || coalesce(j.result::text, '-') || '/' ||
		coalesce((SELECT string_agg(state, ',' ORDER BY attempt_number) FROM job_attempts WHERE job_id = j.id), '-') || '/' ||
		(SELECT count(*) FROM job_events WHERE job_id = j.id) FROM jobs j WHERE j.id = $1`, job)
}

// RV01-01: one denial per fence clause on ReportSuccess. No job/attempt/event change, one audit row.
func TestAC09_ReportFenceDenials(t *testing.T) {
	ctx := context.Background()
	cred := `(SELECT credential_id FROM workers WHERE id = $1)`
	cases := []struct {
		name, mutant, want string
		timeout            time.Duration
		setup              func(t *testing.T, l *Lease)
	}{
		{"credential revoked", "M1", "credential_revoked", 0, func(t *testing.T, l *Lease) {
			exec(t, `UPDATE worker_credentials SET revoked_at = now() WHERE id = `+cred, l.ID)
		}},
		{"credential expired", "M1", "credential_revoked", 0, func(t *testing.T, l *Lease) {
			exec(t, `UPDATE worker_credentials SET expires_at = created_at + interval '1 microsecond' WHERE id = `+cred, l.ID)
		}},
		{"worker re-registered (old process reports)", "M2", "incarnation_mismatch", 0, func(t *testing.T, l *Lease) {
			exec(t, `UPDATE workers SET current_incarnation_id = gen_random_uuid(), version = version + 1 WHERE id = $1`, l.ID)
		}},
		{"attempt held by another incarnation", "M3", "incarnation_mismatch", 0, func(t *testing.T, l *Lease) {
			l.Incarnation = q[string](t, `UPDATE workers SET current_incarnation_id = gen_random_uuid(), version = version + 1
				WHERE id = $1 RETURNING current_incarnation_id::text`, l.ID)
		}},
		{"worker deregistered", "M5", "worker_deregistered", 0, func(t *testing.T, l *Lease) {
			exec(t, `UPDATE workers SET status = 'deregistered', deregistered_at = now(), version = version + 1 WHERE id = $1`, l.ID)
		}},
		{"hard deadline passed", "M4", "deadline_exceeded", time.Second, func(t *testing.T, l *Lease) {
			time.Sleep(1100 * time.Millisecond)
			// Try to keep the lease alive past the deadline. The CHECK lease_expires_at <= hard_deadline_at rejects
			// this (so M4 alone is equivalent); without that CHECK only the fence's deadline clause stops the report.
			_ = execErr(`UPDATE jobs SET lease_expires_at = hard_deadline_at + interval '1 minute' WHERE id = $1`, l.JobID)
		}},
	}
	for _, c := range cases {
		t.Run(c.mutant+" "+c.name, func(t *testing.T) {
			s := newStore(t, 30*time.Second)
			s.Timeout = c.timeout
			job, _ := s.Admit(ctx, handler, 3)
			ls, err := s.Claim(ctx, workers(t, s, 1)[0], 1)
			if err != nil || len(ls) != 1 {
				t.Fatal(ls, err)
			}
			l := ls[0]
			before := snapshot(t, job)
			c.setup(t, &l)
			if out, err := s.ReportSuccess(ctx, l, `{"x":1}`); !isDenial(err, c.want) {
				t.Fatalf("report = %v, %v; want denied %s", out, err, c.want)
			}
			if after := snapshot(t, job); after != before {
				t.Fatalf("denied report changed state: %s -> %s", before, after)
			}
			if n := q[int](t, `SELECT count(*) FROM audit_events WHERE target_id = $1 AND details->>'reason' = $2`, job, c.want); n != 1 {
				t.Fatalf("%d audit rows for %s, want 1", n, c.want)
			}
		})
	}
}

// RV01-01: Claim denies every non-authorized worker identity and creates nothing.
func TestAC09_ClaimDenials(t *testing.T) {
	ctx := context.Background()
	for name, setup := range map[string]string{
		"revoked":      `UPDATE worker_credentials SET revoked_at = now() WHERE id = (SELECT credential_id FROM workers WHERE id = $1)`,
		"expired":      `UPDATE worker_credentials SET expires_at = created_at + interval '1 microsecond' WHERE id = (SELECT credential_id FROM workers WHERE id = $1)`,
		"superseded":   `UPDATE workers SET current_incarnation_id = gen_random_uuid() WHERE id = $1`,
		"draining":     `UPDATE workers SET status = 'draining', drain_requested_at = now(), drain_deadline_at = now() + interval '1 min' WHERE id = $1`,
		"deregistered": `UPDATE workers SET status = 'deregistered', deregistered_at = now() WHERE id = $1`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newStore(t, 30*time.Second)
			job, _ := s.Admit(ctx, handler, 3)
			w := workers(t, s, 1)[0]
			exec(t, setup, w.ID)
			if ls, err := s.Claim(ctx, w, 1); !isDenial(err, "worker_not_authorized") {
				t.Fatalf("claim = %v, %v", ls, err)
			}
			if st := snapshot(t, job); !strings.HasPrefix(st, "queued/1/") {
				t.Fatalf("job after denied claim = %s", st)
			}
		})
	}
}

// RV01-01 (M11): lease expiry with a committed cancel request finalizes canceled, not retry.
func TestE8_CancelRequestedThenLeaseExpiry(t *testing.T) {
	s := newStore(t, 30*time.Second)
	s.CancelGrace = 100 * time.Millisecond
	ctx := context.Background()
	job, _ := s.Admit(ctx, handler, 3)
	if _, err := s.Claim(ctx, workers(t, s, 1)[0], 1); err != nil {
		t.Fatal(err)
	}
	if r, err := s.RequestCancel(ctx, job, "user:op"); err != nil || r.HTTP != 202 {
		t.Fatal(r, err)
	}
	time.Sleep(200 * time.Millisecond) // lease was capped at cancel grace
	if n, err := s.RecoverExpired(ctx, 0); err != nil || n != 1 {
		t.Fatalf("recover = %d, %v", n, err)
	}
	if st := q[string](t, `SELECT j.status || '/' || j.failure_class || '/' || a.state FROM jobs j JOIN job_attempts a ON a.job_id = j.id WHERE j.id = $1`, job); st != "canceled/canceled_by_operator/lease_expired" {
		t.Fatalf("after recovery = %s", st)
	}
	if ev := eventTypes(t, job); ev != "admitted,claimed,cancel_requested,canceled" {
		t.Fatalf("events = %s", ev)
	}
	assertCoherent(t, job)
}

// RV01-04 (mutant M-lock): a revocation cannot commit while a report that already passed lockIdentity is in flight,
// so "revoked before the report commits" always means "report denied".
func TestRV0104_RevocationSerializesWithInFlightReport(t *testing.T) {
	s := newStore(t, 30*time.Second)
	ctx := context.Background()
	job, _ := s.Admit(ctx, handler, 3)
	w := workers(t, s, 1)[0]
	ls, _ := s.Claim(ctx, w, 1)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `SELECT 1 FROM jobs WHERE id = $1 FOR UPDATE`, job); err != nil {
		t.Fatal(err)
	}
	var rep Reply
	var rerr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); rep, rerr = s.ReportSuccess(ctx, ls[0], `{}`) }()
	waitLockWaiters(t, 1) // report holds identity share locks, waits for the job row lock
	revoked := make(chan struct{})
	go func() {
		_ = execErr(`UPDATE worker_credentials SET revoked_at = now() WHERE id = (SELECT credential_id FROM workers WHERE id = $1)`, w.ID)
		close(revoked)
	}()
	waitLockWaiters(t, 2)
	select {
	case <-revoked:
		t.Fatal("revocation committed while a report holding the identity lock was in flight")
	case <-time.After(200 * time.Millisecond):
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	<-revoked
	if rerr != nil || rep.Outcome != Succeeded {
		t.Fatalf("report = %v, %v (it was ordered before the revocation)", rep, rerr)
	}
	if !q[bool](t, `SELECT (SELECT revoked_at FROM worker_credentials WHERE id = (SELECT credential_id FROM workers WHERE id = $2))
		>= (SELECT terminal_at FROM jobs WHERE id = $1)`, job, w.ID) {
		t.Fatal("revoked_at precedes terminal_at although the report committed first")
	}
}

// RV01-04 (mutant M-clock): time in the fence is judged when the row is re-checked, not at transaction start.
func TestRV0104_DeadlineJudgedAtRecheckNotTxStart(t *testing.T) {
	s := newStore(t, 30*time.Second)
	s.Timeout = time.Second
	ctx := context.Background()
	job, _ := s.Admit(ctx, handler, 3)
	ls, _ := s.Claim(ctx, workers(t, s, 1)[0], 1)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `SELECT 1 FROM jobs WHERE id = $1 FOR UPDATE`, job); err != nil {
		t.Fatal(err)
	}
	var rerr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _, rerr = s.ReportSuccess(ctx, ls[0], `{}`) }() // starts well before the deadline
	waitLockWaiters(t, 1)
	time.Sleep(1200 * time.Millisecond) // hard deadline passes while the report waits
	_ = holder.Commit(ctx)
	wg.Wait()
	if !isDenial(rerr, "deadline_exceeded") {
		t.Fatalf("report after deadline = %v, want deadline_exceeded", rerr)
	}
	if st := q[string](t, `SELECT status FROM jobs WHERE id = $1`, job); st != "running" {
		t.Fatalf("status %s", st)
	}
}

// RV01-07: duplicate reports must come from the attempt's own worker and carry the same result.
func TestRV0107_DuplicateReportIdentityAndResult(t *testing.T) {
	s := newStore(t, 30*time.Second)
	ctx := context.Background()
	ws := workers(t, s, 2)
	job, _ := s.Admit(ctx, handler, 3)
	ls, _ := s.Claim(ctx, ws[0], 1)
	if r, err := s.ReportSuccess(ctx, ls[0], `{"a":1}`); err != nil || r.Outcome != Succeeded {
		t.Fatal(r, err)
	}
	forged := ls[0]
	forged.Worker = ws[1]
	if r, err := s.ReportSuccess(ctx, forged, `{"a":1}`); !isDenial(err, "stale_attempt") {
		t.Fatalf("other worker = %v, %v; want stale_attempt", r, err)
	}
	if r, err := s.ReportSuccess(ctx, ls[0], `{"a":2}`); !isDenial(err, "result_mismatch") {
		t.Fatalf("different result = %v, %v; want result_mismatch", r, err)
	}
	if r, err := s.ReportSuccess(ctx, ls[0], `{"a":1}`); err != nil || r != (Reply{Outcome: Succeeded, AlreadyFinal: true}) {
		t.Fatalf("same result retry = %v, %v", r, err)
	}
	if v := q[string](t, `SELECT result::text FROM jobs WHERE id = $1`, job); v != `{"a": 1}` {
		t.Fatalf("result = %s", v)
	}
}

// RV01-09: force each cancel/success commit order under real concurrency (both waiters queued on the row lock).
func TestAC06_ForcedOrderings(t *testing.T) {
	ctx := context.Background()
	for _, cancelFirst := range []bool{true, false} {
		name := map[bool]string{true: "cancel-first", false: "success-first"}[cancelFirst]
		t.Run(name, func(t *testing.T) {
			s := newStore(t, 30*time.Second)
			job, _ := s.Admit(ctx, handler, 3)
			ls, _ := s.Claim(ctx, workers(t, s, 1)[0], 1)
			holder, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(ctx)
			if _, err := holder.Exec(ctx, `SELECT 1 FROM jobs WHERE id = $1 FOR UPDATE`, job); err != nil {
				t.Fatal(err)
			}
			var cr CancelResult
			var rep Reply
			var cerr, serr error
			var wg sync.WaitGroup
			cancel := func() { defer wg.Done(); cr, cerr = s.RequestCancel(ctx, job, "user:op") }
			success := func() { defer wg.Done(); rep, serr = s.ReportSuccess(ctx, ls[0], `{}`) }
			first, second := success, cancel
			if cancelFirst {
				first, second = cancel, success
			}
			wg.Add(2)
			go first()
			waitLockWaiters(t, 1)
			go second()
			waitLockWaiters(t, 2)
			_ = holder.Commit(ctx)
			wg.Wait()
			if cerr != nil || serr != nil {
				t.Fatal(cerr, serr)
			}
			final := q[string](t, `SELECT status FROM jobs WHERE id = $1`, job)
			if cancelFirst && !(cr.HTTP == 202 && rep.Outcome == Canceled && final == "canceled") {
				t.Fatalf("cancel-first: cancel=%+v report=%v final=%s", cr, rep, final)
			}
			if !cancelFirst && !(cr.HTTP == 409 && rep.Outcome == Succeeded && final == "succeeded") {
				t.Fatalf("success-first: cancel=%+v report=%v final=%s", cr, rep, final)
			}
			assertCoherent(t, job)
		})
	}
}

// RV01-09 (AC-03 concurrent): a stale report racing recovery at the lease boundary never both succeeds and is
// recovered; if neither wins in that instant the next recovery tick recovers it (adapted from reviewer probe
// TestRV_RecoverVsSuccessBoundary).
func TestAC03_StaleReportVsRecoveryConcurrent(t *testing.T) {
	s := newStore(t, 150*time.Millisecond)
	ctx := context.Background()
	w := workers(t, s, 1)[0]
	exec(t, `UPDATE workers SET capacity = 256`)
	tally := map[string]int{}
	for i := 0; i < 30; i++ {
		job, _ := s.Admit(ctx, handler, 3)
		l, err := s.Claim(ctx, w, 1)
		if err != nil || len(l) != 1 {
			t.Fatal(l, err)
		}
		time.Sleep(time.Duration(140+i%20) * time.Millisecond)
		var rep Reply
		var serr, rerr error
		var n int
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; rep, serr = s.ReportSuccess(ctx, l[0], `{}`) }()
		go func() { defer wg.Done(); <-start; n, rerr = s.RecoverExpired(ctx, 0) }()
		close(start)
		wg.Wait()
		if rerr != nil {
			t.Fatal(rerr)
		}
		st := q[string](t, `SELECT status FROM jobs WHERE id = $1`, job)
		switch {
		case serr == nil && rep.Outcome == Succeeded && n == 0 && st == "succeeded":
			tally["success"]++
		case serr != nil && n == 1 && st == "retry_wait":
			tally["recovered"]++
		case isDenial(serr, "lease_expired") && n == 0 && st == "running":
			// Report saw the lease expired by clock_timestamp() just before recovery's now() did; the next tick recovers.
			if m, err := s.RecoverExpired(ctx, 0); err != nil || m != 1 {
				t.Fatalf("iter %d: follow-up recovery = %d, %v", i, m, err)
			}
			tally["denied-then-recovered"]++
		default:
			t.Fatalf("iter %d inconsistent: rep=%v serr=%v n=%d st=%s", i, rep, serr, n, st)
		}
		assertCoherent(t, job)
		_, _ = s.RequestCancel(ctx, job, "user:op")
	}
	t.Logf("boundary tally: %v", tally)
}

// RV01-05 / RV01-06 and the lease<=deadline CHECK: DB guards hold even against direct SQL.
func TestSchemaGuards(t *testing.T) {
	s := newStore(t, 30*time.Second)
	ctx := context.Background()
	w := workers(t, s, 1)[0]
	running, _ := s.Admit(ctx, handler, 3)
	ls, _ := s.Claim(ctx, w, 1)
	queued, _ := s.Admit(ctx, handler, 3)
	failedParent, _ := s.Admit(ctx, handler, 3)
	terminal, _ := s.Admit(ctx, handler, 3)
	_, _ = s.RequestCancel(ctx, terminal, "user:op")

	mustFail := map[string]struct {
		sql  string
		args []any
	}{
		"(a) attempt numbered past budget": {`INSERT INTO job_attempts (job_id, attempt_number, lease_generation, worker_id, worker_incarnation_id, state, error_class, finished_at)
			VALUES ($1, 5, 9, $2, $3, 'failed', 'retryable', now())`, []any{running, w.ID, w.Incarnation}},
		"(b) max_attempts raised after admission": {`UPDATE jobs SET max_attempts = 5 WHERE id = $1`, []any{running}},
		"(b) payload rewritten":                   {`UPDATE jobs SET payload = '{"evil":1}' WHERE id = $1`, []any{queued}},
		"(b) budget skipped":                      {`UPDATE jobs SET attempts_used = 2, lease_generation = 2 WHERE id = $1`, []any{queued}},
		"(c) active attempt on queued job": {`INSERT INTO job_attempts (job_id, attempt_number, lease_generation, worker_id, worker_incarnation_id)
			VALUES ($1, 1, 1, $2, $3)`, []any{queued, w.ID, w.Incarnation}},
		"(d) lineage repointed on terminal job": {`UPDATE jobs SET retried_from_job_id = $2 WHERE id = $1`, []any{terminal, failedParent}},
		"(e) events of retained job deleted":    {`DELETE FROM job_events WHERE job_id = $1`, []any{terminal}},
		"(f) attempt of retained job deleted":   {`DELETE FROM job_attempts WHERE job_id = $1`, []any{running}},
		"(g) young audit row deleted":           {`INSERT INTO audit_events (category, action, outcome, actor) VALUES ('privileged', 'x', 'allowed', 'system'); DELETE FROM audit_events`, nil},
		"(h) lease beyond hard deadline":        {`UPDATE jobs SET lease_expires_at = hard_deadline_at + interval '1 minute' WHERE id = $1`, []any{running}},
	}
	for name, c := range mustFail {
		if err := execErr(c.sql, c.args...); err == nil {
			t.Errorf("%s: accepted, want rejection", name)
		}
	}
	// Partial unique index still guards on its own (trigger disabled inside a rolled-back tx).
	tx, _ := pool.Begin(ctx)
	_, _ = tx.Exec(ctx, `ALTER TABLE job_attempts DISABLE TRIGGER job_attempts_guard_insert`)
	_, err := tx.Exec(ctx, `INSERT INTO job_attempts (job_id, attempt_number, lease_generation, worker_id, worker_incarnation_id) VALUES ($1, 2, 2, $2, $3)`, running, w.ID, w.Incarnation)
	_ = tx.Rollback(ctx)
	if err == nil || !strings.Contains(err.Error(), "job_attempts_one_active_per_job") {
		t.Errorf("second active attempt with trigger off: %v, want unique violation", err)
	}
	// Legitimate paths still work: retention deletes a terminal job (cascade + tombstone) and nulls child lineage.
	exec(t, `UPDATE jobs SET retried_from_job_id = $2 WHERE id = $1`, queued, terminal) // non-terminal child may be linked
	if _, err := s.RequestCancel(ctx, queued, "user:op"); err != nil {
		t.Fatal(err)
	}
	exec(t, `WITH d AS (DELETE FROM jobs WHERE id = $1 RETURNING id, terminal_at)
		INSERT INTO job_tombstones (job_id, terminal_at) SELECT id, terminal_at FROM d`, terminal)
	if v := q[bool](t, `SELECT retried_from_job_id IS NULL FROM jobs WHERE id = $1`, queued); !v {
		t.Error("child lineage not nulled by retention")
	}
	_ = ls
}

// RV01-02: token authority = scopes ∩ owner's current role; disabled owner or revoked token => not ok.
func TestRV0102_TokenAuthorityFollowsOwner(t *testing.T) {
	newStore(t, time.Second)
	owner := q[string](t, `INSERT INTO users (github_user_id, github_login, role) VALUES (42, 'op', 'operator') RETURNING id::text`)
	tok := q[string](t, `INSERT INTO api_tokens (name, owner_user_id, role, scopes, secret_hash, prefix)
		VALUES ('ci', $1, 'operator', ARRAY['jobs:write','jobs:read'], sha256('t'), 'cxt_ab') RETURNING id::text`, owner)
	auth := func() string {
		return q[string](t, `SELECT effective_role || '/' || array_to_string(effective_scopes, ',') || '/' || status
			FROM api_token_authority WHERE token_id = $1`, tok)
	}
	steps := []struct{ sql, want string }{
		{``, "operator/jobs:write,jobs:read/ok"},
		{`UPDATE users SET role = 'viewer' WHERE id = $1`, "viewer/jobs:read/ok"},
		{`UPDATE users SET role = 'operator', disabled_at = now() WHERE id = $1`, "operator/jobs:write,jobs:read/user_disabled"},
		{`UPDATE users SET disabled_at = NULL WHERE id = $1`, "operator/jobs:write,jobs:read/ok"},
	}
	for _, st := range steps {
		if st.sql != "" {
			exec(t, st.sql, owner)
		}
		if got := auth(); got != st.want {
			t.Fatalf("after %q: %s, want %s", st.sql, got, st.want)
		}
	}
	exec(t, `UPDATE api_tokens SET revoked_at = now() WHERE id = $1`, tok)
	if got := auth(); !strings.HasSuffix(got, "/token_revoked") {
		t.Fatalf("revoked token: %s", got)
	}
}

// waitLockWaiters blocks until n backends of this database wait on a heavyweight lock.
func waitLockWaiters(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if q[int](t, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`) >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d lock waiters", n)
}
