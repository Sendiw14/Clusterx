package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ctx = context.Background()

// AC-02 (store level): >= 20 concurrent claimants on one job establish exactly one lease.
func TestAC02_ConcurrentClaimantsSingleJob(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	workers := make([]WorkerRef, 24)
	for i := range workers {
		workers[i] = newWorker(t, s, 4, "h")
	}
	for round := range 5 {
		id := admit(t, s, "h")
		var wg sync.WaitGroup
		var mu sync.Mutex
		won := 0
		for _, w := range workers {
			wg.Go(func() {
				got, err := s.Claim(ctx, w, 1, nil)
				if err != nil {
					t.Errorf("claim: %v", err)
					return
				}
				mu.Lock()
				won += len(got)
				mu.Unlock()
			})
		}
		wg.Wait()
		if won != 1 {
			t.Fatalf("round %d: %d leases, want 1", round, won)
		}
		if n := q[int](t, s, `SELECT count(*) FROM job_attempts WHERE job_id = $1`, id); n != 1 {
			t.Fatalf("attempts = %d", n)
		}
		// finish it so the next round has a single queued job
		a := q[string](t, s, `SELECT current_attempt_id::text FROM jobs WHERE id = $1`, id)
		wid := q[string](t, s, `SELECT worker_id::text FROM job_attempts WHERE id = $1`, a)
		var w WorkerRef
		for _, x := range workers {
			if x.WorkerID == wid {
				w = x
			}
		}
		if _, err := s.ReportSuccess(ctx, AttemptRef{WorkerRef: w, JobID: id, AttemptID: a, Generation: 1}, json.RawMessage(`{}`), ""); err != nil {
			t.Fatal(err)
		}
	}
}

// AC-02: a failpoint at any step leaves no partial job/attempt/event write.
func TestAC02_FailpointsLeaveNoPartialWrites(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 4, "h")
	boom := errors.New("injected")
	for _, fp := range []string{"admit.after_job_insert", "admit.before_commit"} {
		s.Failpoint = func(n string) error {
			if n == fp {
				return boom
			}
			return nil
		}
		_, _, err := s.Admit(ctx, AdmitParams{Principal: "token:t", Namespace: "default", HandlerName: "h", HandlerVersion: 1,
			MaxAttempts: 3, TimeoutSeconds: 60, Payload: json.RawMessage(`{}`), IdempotencyKey: "k", Fingerprint: make([]byte, 32)})
		if !errors.Is(err, boom) {
			t.Fatalf("%s: err=%v", fp, err)
		}
		if n := q[int](t, s, `SELECT (SELECT count(*) FROM jobs) + (SELECT count(*) FROM job_events) + (SELECT count(*) FROM submission_idempotency)`); n != 0 {
			t.Fatalf("%s: %d rows left", fp, n)
		}
	}
	s.Failpoint = nil
	id := admit(t, s, "h")
	for _, fp := range []string{"claim.after_job_update", "claim.after_attempt_insert", "claim.before_commit"} {
		s.Failpoint = func(n string) error {
			if n == fp {
				return boom
			}
			return nil
		}
		if _, err := s.Claim(ctx, w, 1, nil); !errors.Is(err, boom) {
			t.Fatalf("%s: err=%v", fp, err)
		}
		if st := q[string](t, s, `SELECT status || version FROM jobs WHERE id = $1`, id); st != "queued1" {
			t.Fatalf("%s: job %s", fp, st)
		}
		if n := q[int](t, s, `SELECT (SELECT count(*) FROM job_attempts) + (SELECT count(*) FROM job_events WHERE version > 1)`); n != 0 {
			t.Fatalf("%s: %d partial rows", fp, n)
		}
	}
	s.Failpoint = nil
	a := claimOne(t, s, w)
	s.Failpoint = func(n string) error {
		if n == "report.before_event" {
			return boom
		}
		return nil
	}
	if _, err := s.ReportSuccess(ctx, a, json.RawMessage(`{"ok":true}`), ""); !errors.Is(err, boom) {
		t.Fatalf("report: %v", err)
	}
	if st := q[string](t, s, `SELECT j.status || '/' || a.state FROM jobs j JOIN job_attempts a ON a.id = j.current_attempt_id WHERE j.id = $1`, id); st != "running/active" {
		t.Fatalf("after failed report: %s", st)
	}
}

func TestReportSuccessAndDuplicates(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 4, "h")
	other := newWorker(t, s, 4, "h")
	admit(t, s, "h")
	a := claimOne(t, s, w)
	res := json.RawMessage(`{"v":1}`)
	rep, err := s.ReportSuccess(ctx, a, res, "log line")
	if err != nil || rep.Outcome != OutcomeSucceeded || rep.AlreadyFinal {
		t.Fatalf("first report: %+v %v", rep, err)
	}
	if rep, err := s.ReportSuccess(ctx, a, res, ""); err != nil || !rep.AlreadyFinal || rep.Outcome != OutcomeSucceeded {
		t.Fatalf("retry: %+v %v", rep, err)
	}
	_, err = s.ReportSuccess(ctx, a, json.RawMessage(`{"v":2}`), "")
	wantDenied(t, err, "result_mismatch")
	stolen := a
	stolen.WorkerRef = other
	_, err = s.ReportSuccess(ctx, stolen, res, "")
	wantDenied(t, err, "stale_attempt")
	_, err = s.ReportFailure(ctx, a, FailureParams{Code: "x"})
	wantDenied(t, err, "job_terminal")
	if v := q[int64](t, s, `SELECT version FROM jobs WHERE id = $1`, a.JobID); v != 3 {
		t.Fatalf("version = %d, want 3 (admitted, claimed, succeeded)", v)
	}
	if n := q[int](t, s, `SELECT count(*) FROM audit_events WHERE action = 'worker.report_success' AND outcome = 'rejected'`); n != 2 {
		t.Fatalf("audit rows = %d", n)
	}
}

// AC-09 + NF-06 N6: fence denials, including a revoked worker retrying its committed success.
func TestAC09_FenceDenials(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	res := json.RawMessage(`{}`)
	t.Run("lease_expired", func(t *testing.T) {
		w := newWorker(t, s, 4, "le")
		admit(t, s, "le")
		a := claimOne(t, s, w)
		expireLease(t, s, a.JobID, false)
		_, err := s.ReportSuccess(ctx, a, res, "")
		wantDenied(t, err, "lease_expired")
		_, err = s.RenewLease(ctx, a)
		wantDenied(t, err, "lease_expired")
	})
	t.Run("deadline_exceeded", func(t *testing.T) {
		w := newWorker(t, s, 4, "dl")
		admit(t, s, "dl")
		a := claimOne(t, s, w)
		expireLease(t, s, a.JobID, true)
		_, err := s.ReportSuccess(ctx, a, res, "")
		wantDenied(t, err, "deadline_exceeded")
	})
	t.Run("incarnation_mismatch", func(t *testing.T) {
		w := newWorker(t, s, 4, "im")
		admit(t, s, "im")
		a := claimOne(t, s, w)
		inc := NewUUID()
		if _, err := s.Register(ctx, w.CredentialID, RegisterParams{WorkerID: w.WorkerID, IncarnationID: inc, Name: "w", ProtocolVersion: 1, Capacity: 4,
			Capabilities: []Capability{{"im", 1}}}); err != nil {
			t.Fatal(err)
		}
		_, err := s.ReportSuccess(ctx, a, res, "")
		wantDenied(t, err, "incarnation_mismatch")
		_, err = s.Claim(ctx, w, 1, nil)
		wantDenied(t, err, "incarnation_mismatch")
	})
	t.Run("worker_deregistered", func(t *testing.T) {
		w := newWorker(t, s, 4, "dr")
		admit(t, s, "dr")
		a := claimOne(t, s, w)
		if left, err := s.Deregister(ctx, w); err != nil || left != 1 {
			t.Fatalf("deregister = %d %v", left, err)
		}
		_, err := s.ReportSuccess(ctx, a, res, "")
		wantDenied(t, err, "worker_deregistered")
	})
	t.Run("credential_revoked", func(t *testing.T) {
		w := newWorker(t, s, 4, "cr")
		admit(t, s, "cr")
		a := claimOne(t, s, w)
		if err := s.RevokeWorkerCredential(ctx, w.CredentialID); err != nil {
			t.Fatal(err)
		}
		_, err := s.ReportSuccess(ctx, a, res, "")
		wantDenied(t, err, "credential_revoked")
		_, err = s.Claim(ctx, w, 1, nil)
		wantDenied(t, err, "credential_revoked")
	})
	t.Run("N6_revoked_retry_of_committed_success", func(t *testing.T) {
		w := newWorker(t, s, 4, "n6")
		admit(t, s, "n6")
		a := claimOne(t, s, w)
		if _, err := s.ReportSuccess(ctx, a, res, ""); err != nil {
			t.Fatal(err)
		}
		if err := s.RevokeWorkerCredential(ctx, w.CredentialID); err != nil {
			t.Fatal(err)
		}
		_, err := s.ReportSuccess(ctx, a, res, "")
		wantDenied(t, err, "credential_revoked")
	})
	t.Run("other_credential_for_worker", func(t *testing.T) {
		w := newWorker(t, s, 4, "oc")
		x := newWorker(t, s, 4, "oc")
		admit(t, s, "oc")
		a := claimOne(t, s, w)
		a.CredentialID = x.CredentialID
		_, err := s.ReportSuccess(ctx, a, res, "")
		wantDenied(t, err, "credential_mismatch")
		_, err = s.RenewLease(ctx, a)
		wantDenied(t, err, "credential_mismatch")
		_, err = s.Heartbeat(ctx, a.WorkerRef)
		wantDenied(t, err, "credential_mismatch")
		// Register naming w's worker id with x's (valid) credential.
		_, err = s.Register(ctx, x.CredentialID, RegisterParams{WorkerID: w.WorkerID, IncarnationID: NewUUID(), Name: "x",
			ProtocolVersion: 1, Capacity: 1, Capabilities: []Capability{{"oc", 1}}})
		wantDenied(t, err, "credential_mismatch")
	})
	t.Run("stale_generation", func(t *testing.T) {
		w := newWorker(t, s, 4, "sg")
		admit(t, s, "sg")
		a := claimOne(t, s, w)
		a.Generation++
		_, err := s.ReportSuccess(ctx, a, res, "")
		wantDenied(t, err, "stale_attempt")
	})
}

// NF-06 N12: finished attempts are immutable (state-machine global rule 3).
func TestN12_FinishedAttemptImmutable(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 4, "h")
	admit(t, s, "h")
	a := claimOne(t, s, w)
	if _, err := s.ReportSuccess(ctx, a, json.RawMessage(`{}`), ""); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`UPDATE job_attempts SET error_code = 'x' WHERE id = $1`,
		`UPDATE job_attempts SET state = 'active', finished_at = NULL WHERE id = $1`,
		`UPDATE job_attempts SET log_tail = NULL, log_expired = true WHERE id = $1`, // young: not yet retention
	} {
		if _, err := s.Pool.Exec(ctx, sql, a.AttemptID); err == nil {
			t.Fatalf("%s: accepted", sql)
		}
	}
}

// NF-06 N13 + E8: recovery classifies deadline vs lease expiry and failure_class per budget.
func TestN13_RecoveryClassification(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 8, "h")
	one := func(o *AdmitParams) { o.MaxAttempts = 1 }
	cases := []struct {
		deadline, cancel bool
		opts             []admitOpt
		attempt, job, fc string
	}{
		{false, false, []admitOpt{one}, "lease_expired", "failed", "lease_expired"},
		{true, false, []admitOpt{one}, "deadline_exceeded", "failed", "timeout"},
		{true, false, nil, "deadline_exceeded", "retry_wait", ""},
		{false, false, nil, "lease_expired", "retry_wait", ""},
		{false, true, nil, "lease_expired", "canceled", "canceled_by_operator"},
	}
	for i, c := range cases {
		id := admit(t, s, "h", c.opts...)
		a := claimOne(t, s, w)
		if a.JobID != id {
			t.Fatalf("claimed %s want %s", a.JobID, id)
		}
		if c.cancel {
			if r, err := s.RequestCancel(ctx, id, "token:t"); err != nil || r.HTTP != 202 {
				t.Fatalf("cancel: %+v %v", r, err)
			}
		}
		expireLease(t, s, id, c.deadline)
		if n, err := s.RecoverExpired(ctx, 10); err != nil || n != 1 {
			t.Fatalf("case %d: recover = %d %v", i, n, err)
		}
		got := q[string](t, s, `SELECT a.state || '/' || j.status || '/' || coalesce(j.failure_class, '') FROM jobs j
			JOIN job_attempts a ON a.job_id = j.id WHERE j.id = $1`, id)
		if want := c.attempt + "/" + c.job + "/" + c.fc; got != want {
			t.Fatalf("case %d: %s, want %s", i, got, want)
		}
		// CR-M5-01 #3: exhaustion is recorded as attempts_exhausted with failure_class and the attempt outcome.
		ev := q[string](t, s, `SELECT event_type || '/' || coalesce(data->>'failure_class', '') || '/' || coalesce(data->>'attempt_state', '')
			FROM job_events WHERE job_id = $1 ORDER BY version DESC LIMIT 1`, id)
		wantEv := c.attempt + "//"
		switch {
		case c.job == "failed":
			wantEv = "attempts_exhausted/" + c.fc + "/" + c.attempt
		case c.job == "canceled":
			wantEv = "canceled/canceled_by_operator/"
		}
		if ev != wantEv {
			t.Fatalf("case %d: event %s, want %s", i, ev, wantEv)
		}
		if c.job == "retry_wait" { // clean the queue for the next case
			exec(t, s, `UPDATE jobs SET available_at = now() + interval '1 day' WHERE id = $1`, id)
		}
	}
}

// AC-05: max_attempts=3 => exactly 3 attempts, no hidden fourth; injected backoff is reproducible.
func TestAC05_BudgetExactlyThree(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 4, "h")
	id := admit(t, s, "h")
	for i := 1; i <= 4; i++ {
		if _, err := s.PromoteDue(ctx, 100); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
		if _, err := s.PromoteDue(ctx, 100); err != nil {
			t.Fatal(err)
		}
		got, err := s.Claim(ctx, w, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		if i == 4 {
			if len(got) != 0 {
				t.Fatalf("hidden fourth attempt claimed")
			}
			break
		}
		if len(got) != 1 || got[0].AttemptNumber != i {
			t.Fatalf("attempt %d: %+v", i, got)
		}
		rep, err := s.ReportFailure(ctx, AttemptRef{WorkerRef: w, JobID: id, AttemptID: got[0].AttemptID, Generation: got[0].LeaseGeneration},
			FailureParams{Code: "boom", Message: "x"})
		if err != nil {
			t.Fatal(err)
		}
		want := OutcomeRetryScheduled
		if i == 3 {
			want = OutcomeFailed
		}
		if rep.Outcome != want {
			t.Fatalf("attempt %d outcome %s", i, rep.Outcome)
		}
	}
	if got := q[string](t, s, `SELECT status || '/' || attempts_used || '/' || failure_class || '/' || error_code FROM jobs WHERE id = $1`, id); got != "failed/3/retryable_exhausted/boom" {
		t.Fatalf("job = %s", got)
	}
	// Rand()=0 => factor 0.5: backoffs 0.5 ms (n=1) and 1 ms (n=2).
	b := Backoff{Base: 2 * time.Second, Cap: 5 * time.Minute, Rand: func() float64 { return 0 }}
	if b.Delay(1) != time.Second || b.Delay(2) != 2*time.Second || b.Delay(20) != 150*time.Second {
		t.Fatalf("backoff %v %v %v", b.Delay(1), b.Delay(2), b.Delay(20))
	}
	b.Rand = func() float64 { return 0.999999999 }
	if d := b.Delay(1); d >= 2*time.Second || d < time.Second {
		t.Fatalf("jitter bound: %v", d)
	}
	if n := q[string](t, s, `SELECT string_agg(event_type, ',' ORDER BY version) FROM job_events WHERE job_id = $1`, id); n !=
		"admitted,claimed,failed_retryable,promoted,claimed,failed_retryable,promoted,claimed,attempts_exhausted" {
		t.Fatalf("events: %s", n)
	}
}

// AC-06: cancel vs success in both orders and concurrently; exactly one valid final outcome.
func TestAC06_CancelVersusSuccess(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 64, "h")
	res := json.RawMessage(`{"r":1}`)
	// cancel first
	admit(t, s, "h")
	a := claimOne(t, s, w)
	if r, _ := s.RequestCancel(ctx, a.JobID, "token:t"); r.HTTP != 202 {
		t.Fatalf("cancel %+v", r)
	}
	rep, err := s.ReportSuccess(ctx, a, res, "")
	if err != nil || rep.Outcome != OutcomeCanceled {
		t.Fatalf("success after cancel: %+v %v", rep, err)
	}
	if rep, err := s.ReportSuccess(ctx, a, res, ""); err != nil || !rep.AlreadyFinal || rep.Outcome != OutcomeCanceled {
		t.Fatalf("retry after cancel: %+v %v", rep, err)
	}
	// NF-02: terminal_at >= cancel_requested_at; events ordered by version agree with time.
	if bad := q[bool](t, s, `SELECT terminal_at < cancel_requested_at FROM jobs WHERE id = $1`, a.JobID); bad {
		t.Fatal("terminal_at before cancel_requested_at")
	}
	// success first
	admit(t, s, "h")
	a = claimOne(t, s, w)
	if _, err := s.ReportSuccess(ctx, a, res, ""); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.RequestCancel(ctx, a.JobID, "token:t"); r.HTTP != 409 || r.Status != "succeeded" {
		t.Fatalf("cancel after success %+v", r)
	}
	// concurrent
	for range 30 {
		admit(t, s, "h")
		a := claimOne(t, s, w)
		var wg sync.WaitGroup
		var rep Reply
		var rerr, cerr error
		wg.Go(func() { rep, rerr = s.ReportSuccess(ctx, a, res, "") })
		wg.Go(func() { _, cerr = s.RequestCancel(ctx, a.JobID, "token:t") })
		wg.Wait()
		if rerr != nil || cerr != nil {
			t.Fatalf("errors %v %v", rerr, cerr)
		}
		st := q[string](t, s, `SELECT status FROM jobs WHERE id = $1`, a.JobID)
		if (st == "succeeded") != (rep.Outcome == OutcomeSucceeded) || (st != "succeeded" && st != "canceled") {
			t.Fatalf("status %s, reply %s", st, rep.Outcome)
		}
	}
}

// NF-02: a report whose transaction began before a cancel, then waited for the job lock, stamps
// terminal_at and its event after the cancel (clock_timestamp after locks, not tx-start now()).
func TestNF02_TerminalAtAfterCancelRequested(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 4, "h")
	admit(t, s, "h")
	a := claimOne(t, s, w)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM jobs WHERE id = $1 FOR UPDATE`, a.JobID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		rep, err := s.ReportSuccess(ctx, a, json.RawMessage(`{}`), "")
		if err == nil && rep.Outcome != OutcomeCanceled {
			err = fmt.Errorf("outcome %s", rep.Outcome)
		}
		done <- err
	}()
	time.Sleep(150 * time.Millisecond)
	if _, err := tx.Exec(ctx, `UPDATE jobs SET cancel_requested_at = clock_timestamp(), version = version + 1 WHERE id = $1`, a.JobID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor)
		VALUES ($1, 3, 'cancel_requested', 'running', 'running', 'token:t')`, a.JobID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if ok := q[bool](t, s, `SELECT terminal_at >= cancel_requested_at FROM jobs WHERE id = $1`, a.JobID); !ok {
		t.Fatal("terminal_at < cancel_requested_at")
	}
	if ok := q[bool](t, s, `SELECT bool_and(ok) FROM (SELECT occurred_at >= lag(occurred_at) OVER (ORDER BY version) AS ok
		FROM job_events WHERE job_id = $1) x WHERE ok IS NOT NULL`, a.JobID); !ok {
		t.Fatal("event occurred_at order disagrees with version")
	}
}

// AC-07: idempotent admission.
func TestAC07_Idempotency(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	fp1, fp2 := make([]byte, 32), make([]byte, 32)
	fp2[0] = 1
	p := AdmitParams{Principal: "token:a", Namespace: "default", HandlerName: "h", HandlerVersion: 1, MaxAttempts: 3,
		TimeoutSeconds: 60, Payload: json.RawMessage(`{}`), IdempotencyKey: "key-1", Fingerprint: fp1}
	j1, replay, err := s.Admit(ctx, p)
	if err != nil || replay {
		t.Fatal(err, replay)
	}
	// concurrent identical submissions all replay the same job
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			j, replay, err := s.Admit(ctx, p)
			if err != nil || !replay || j.ID != j1.ID {
				t.Errorf("replay: %v %v %s", err, replay, j.ID)
			}
		})
	}
	wg.Wait()
	p2 := p
	p2.Fingerprint = fp2
	_, _, err = s.Admit(ctx, p2)
	if d, ok := errors.AsType[*Denial](err); !ok || d.Reason != "idempotency_conflict" || d.Detail != j1.ID {
		t.Fatalf("conflict: %v", err)
	}
	p3 := p
	p3.Principal = "token:b" // other scope
	if j, replay, err := s.Admit(ctx, p3); err != nil || replay || j.ID == j1.ID {
		t.Fatalf("other principal: %v %v", err, replay)
	}
	exec(t, s, `UPDATE submission_idempotency SET expires_at = now() - interval '1 second', created_at = now() - interval '2 seconds'
		WHERE principal = 'token:a'`)
	j4, replay, err := s.Admit(ctx, p2)
	if err != nil || replay || j4.ID == j1.ID {
		t.Fatalf("expired key reuse: %v %v", err, replay)
	}
	if n := q[int](t, s, `SELECT count(*) FROM jobs`); n != 3 {
		t.Fatalf("jobs = %d", n)
	}
}

// AC-10: priority 9 > 0 among eligible rows, due time honored, unschedulable reasons.
func TestAC10_PriorityDueUnschedulable(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	low := admit(t, s, "h", func(p *AdmitParams) { p.Priority = 0 })
	future := time.Now().Add(time.Hour)
	admit(t, s, "h", func(p *AdmitParams) { p.Priority = 9; p.RunAfter = &future })
	high := admit(t, s, "h", func(p *AdmitParams) { p.Priority = 9 })
	orphan := admit(t, s, "nobody")
	reason := func(id string) string {
		j, err := s.GetJob(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if j.UnschedulableReason == nil {
			return ""
		}
		return *j.UnschedulableReason
	}
	if r := reason(high); r != "no_capable_worker" {
		t.Fatalf("before workers: %q", r)
	}
	w := newWorker(t, s, 4, "h")
	if r := reason(high); r != "" {
		t.Fatalf("with worker: %q", r)
	}
	if r := reason(orphan); r != "no_capable_worker" {
		t.Fatalf("orphan: %q", r)
	}
	got, err := s.Claim(ctx, w, 4, nil)
	if err != nil || len(got) != 2 || got[0].JobID != high || got[1].JobID != low {
		t.Fatalf("claim order: %+v %v", got, err)
	}
	h2 := admit(t, s, "h")
	if _, err := s.Drain(ctx, w, time.Minute); err != nil {
		t.Fatal(err)
	}
	if r := reason(h2); r != "capable_workers_draining" {
		t.Fatalf("draining: %q", r)
	}
	_, err = s.Claim(ctx, w, 1, nil)
	wantDenied(t, err, "worker_draining")
	exec(t, s, `UPDATE workers SET last_heartbeat_at = now() - interval '1 hour'`)
	if r := reason(h2); r != "capable_workers_offline" {
		t.Fatalf("offline: %q", r)
	}
	_, err = s.Claim(ctx, w, 1, []Capability{{"unknown.handler", 1}})
	wantDenied(t, err, "unknown_handler")
}

func TestRenewLeaseAndCancelGrace(t *testing.T) {
	s := newTestStore(t, 2*time.Second)
	s.Timing.CancelGrace = 500 * time.Millisecond
	w := newWorker(t, s, 4, "h")
	admit(t, s, "h")
	a := claimOne(t, s, w)
	l, err := s.RenewLease(ctx, a)
	if err != nil || l.CancelRequested || l.ExpiresAt.Sub(l.Now) < 1900*time.Millisecond {
		t.Fatalf("renew: %+v %v", l, err)
	}
	if v := q[int64](t, s, `SELECT version FROM jobs WHERE id = $1`, a.JobID); v != 2 {
		t.Fatalf("renew bumped version to %d", v)
	}
	if _, err := s.RequestCancel(ctx, a.JobID, "token:t"); err != nil {
		t.Fatal(err)
	}
	l, err = s.RenewLease(ctx, a)
	if err != nil || !l.CancelRequested || l.ExpiresAt.Sub(l.Now) > 500*time.Millisecond {
		t.Fatalf("renew after cancel: %+v %v", l, err)
	}
	p, err := s.PollCancellation(ctx, a)
	if err != nil || !p.CancelRequested {
		t.Fatalf("poll: %+v %v", p, err)
	}
	if _, err := s.ReportFailure(ctx, a, FailureParams{Code: "canceled", CanceledByHandler: true}); err != nil {
		t.Fatal(err)
	}
	p, err = s.PollCancellation(ctx, a)
	if err != nil || !p.JobTerminal || !p.CancelRequested {
		t.Fatalf("poll after: %+v %v", p, err)
	}
	_, err = s.RenewLease(ctx, a)
	wantDenied(t, err, "job_terminal")

	// E12: renewals are denied after the drain deadline.
	admit(t, s, "h")
	b := claimOne(t, s, w)
	if _, err := s.Drain(ctx, w, 0); err != nil {
		t.Fatal(err)
	}
	_, err = s.RenewLease(ctx, b)
	wantDenied(t, err, "worker_draining")
}

func TestRegisterBinding(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 4, "h")
	_, err := s.Register(ctx, w.CredentialID, RegisterParams{IncarnationID: NewUUID(), Name: "w", ProtocolVersion: 1, Capacity: 1})
	wantDenied(t, err, "incarnation_mismatch") // live worker bound to this credential
	// lost-reply retry with the same incarnation is idempotent
	if r, err := s.Register(ctx, w.CredentialID, RegisterParams{IncarnationID: w.IncarnationID, Name: "w", ProtocolVersion: 1, Capacity: 4}); err != nil || r.WorkerID != w.WorkerID {
		t.Fatalf("same incarnation: %+v %v", r, err)
	}
	exec(t, s, `UPDATE workers SET last_heartbeat_at = now() - interval '1 hour' WHERE id = $1`, w.WorkerID)
	r, err := s.Register(ctx, w.CredentialID, RegisterParams{IncarnationID: NewUUID(), Name: "w", ProtocolVersion: 1, Capacity: 1})
	if err != nil || r.WorkerID != w.WorkerID {
		t.Fatalf("stale takeover: %+v %v", r, err)
	}
	_, err = s.Register(ctx, w.CredentialID, RegisterParams{IncarnationID: NewUUID(), Name: "w", ProtocolVersion: 1, Capacity: 1,
		Capabilities: []Capability{{"unknown.handler", 1}}})
	wantDenied(t, err, "unknown_handler")
	if _, err := s.Heartbeat(ctx, w); !IsDenial(err, "incarnation_mismatch") {
		t.Fatalf("heartbeat old incarnation: %v", err)
	}
}

// NF-05: the database refuses a job leaving running while its attempt stays active, and vice versa.
func TestNF05_JobAttemptCoupling(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 4, "h")
	admit(t, s, "h")
	a := claimOne(t, s, w)
	_, err := s.Pool.Exec(ctx, `UPDATE jobs SET status = 'retry_wait', current_attempt_id = NULL, lease_expires_at = NULL,
		hard_deadline_at = NULL WHERE id = $1`, a.JobID)
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "23514" {
		t.Fatalf("job left running with active attempt: %v", err)
	}
	_, err = s.Pool.Exec(ctx, `UPDATE job_attempts SET state = 'failed', error_class = 'retryable', finished_at = now() WHERE id = $1`, a.AttemptID)
	if !errors.As(err, &pe) || pe.Code != "23514" {
		t.Fatalf("attempt finished while job running: %v", err)
	}
	// the claim batch still works for every other job
	admit(t, s, "h")
	admit(t, s, "h")
	if got, err := s.Claim(ctx, w, 3, nil); err != nil || len(got) != 2 {
		t.Fatalf("claim batch: %d %v", len(got), err)
	}
}

// NF-03: log_tail expiry is the only post-final attempt mutation.
func TestNF03_LogRetention(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 4, "h")
	admit(t, s, "h")
	a := claimOne(t, s, w)
	if _, err := s.ReportSuccess(ctx, a, json.RawMessage(`{}`), "some output"); err != nil {
		t.Fatal(err)
	}
	if n := q[int](t, s, `SELECT clusterx_expire_attempt_logs(100)`); n != 0 {
		t.Fatalf("young log expired: %d", n)
	}
	// Age the attempt (test fixture: disable the user trigger for this single fixture update).
	exec(t, s, `ALTER TABLE job_attempts DISABLE TRIGGER job_attempts_finished_immutable`)
	exec(t, s, `UPDATE job_attempts SET started_at = now() - interval '9 days', finished_at = now() - interval '8 days' WHERE id = $1`, a.AttemptID)
	exec(t, s, `ALTER TABLE job_attempts ENABLE TRIGGER job_attempts_finished_immutable`)
	if _, err := s.Pool.Exec(ctx, `UPDATE job_attempts SET log_tail = NULL, log_expired = true, error_code = 'x' WHERE id = $1`, a.AttemptID); err == nil {
		t.Fatal("log expiry with another column change accepted")
	}
	if n := q[int](t, s, `SELECT clusterx_expire_attempt_logs(100)`); n != 1 {
		t.Fatalf("expired = %d", n)
	}
	if ok := q[bool](t, s, `SELECT log_tail IS NULL AND log_expired FROM job_attempts WHERE id = $1`, a.AttemptID); !ok {
		t.Fatal("log not expired")
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE job_attempts SET log_expired = false WHERE id = $1`, a.AttemptID); err == nil {
		t.Fatal("un-expire accepted")
	}
}

// NF-04: least-privilege runtime role and retention-only deletes.
func TestNF04_PrivilegesAndDeleteGuards(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	w := newWorker(t, s, 4, "h")
	id := admit(t, s, "h")
	claimOne(t, s, w)
	if _, err := s.Pool.Exec(ctx, `DELETE FROM jobs WHERE id = $1`, id); err == nil {
		t.Fatal("delete of running job accepted")
	}
	for _, tbl := range []string{"jobs", "job_events", "job_attempts", "audit_events", "job_tombstones"} {
		if _, err := s.Pool.Exec(ctx, `TRUNCATE `+tbl+` CASCADE`); err == nil {
			t.Fatalf("TRUNCATE %s accepted (superuser)", tbl)
		}
	}
	// A login role in clusterx_runtime.
	role := "cx_rt_" + NewUUID()[:8]
	exec(t, s, `CREATE ROLE `+role+` LOGIN IN ROLE clusterx_runtime`)
	t.Cleanup(func() {
		_, _ = s.Pool.Exec(context.Background(), `REASSIGN OWNED BY `+role+` TO CURRENT_USER`)
		_, _ = s.Pool.Exec(context.Background(), `DROP OWNED BY `+role)
		_, _ = s.Pool.Exec(context.Background(), `DROP ROLE `+role)
	})
	cfg := s.Pool.Config().ConnConfig.Copy()
	cfg.User = role
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, sql := range []string{
		`DELETE FROM jobs`, `DELETE FROM job_events`, `DELETE FROM job_attempts`, `DELETE FROM audit_events`,
		`TRUNCATE submission_idempotency`, `CREATE TABLE x (a int)`, `ALTER TABLE jobs DROP COLUMN priority`,
		`DROP TRIGGER jobs_terminal_immutable ON jobs`, `INSERT INTO schema_migrations VALUES (99, 'x', repeat('0', 64))`,
	} {
		_, err := conn.Exec(ctx, sql)
		var pe *pgconn.PgError
		if !errors.As(err, &pe) || (pe.Code != "42501" && pe.Code != "42P01") {
			t.Errorf("runtime role %q: err=%v, want permission denied", sql, err)
		}
	}
	var n int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("runtime SELECT: %d %v", n, err)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM submission_idempotency WHERE expires_at < now()`); err != nil {
		t.Fatalf("runtime idempotency cleanup: %v", err)
	}
}
