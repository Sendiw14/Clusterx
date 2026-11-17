package poc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const handler = "poc.echo"

var pool *pgxpool.Pool

// TestMain creates a fresh database clusterx_poc, applies migration 0001 and drops it afterwards.
func TestMain(m *testing.M) {
	admin := os.Getenv("CLUSTERX_PG_ADMIN_DSN")
	if admin == "" {
		fmt.Println("SKIP: CLUSTERX_PG_ADMIN_DSN not set; PoC needs real PostgreSQL 18")
		os.Exit(m.Run()) // every test skips itself
	}
	ctx := context.Background()
	ac, err := pgx.Connect(ctx, admin)
	must(err)
	_, _ = ac.Exec(ctx, `DROP DATABASE IF EXISTS clusterx_poc WITH (FORCE)`)
	_, err = ac.Exec(ctx, `CREATE DATABASE clusterx_poc`)
	must(err)

	cfg, err := pgxpool.ParseConfig(admin)
	must(err)
	cfg.ConnConfig.Database = "clusterx_poc"
	cfg.MaxConns = 40
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	must(err)
	ddl, err := os.ReadFile("../migrations/0001_init.sql")
	must(err)
	_, err = pool.Exec(ctx, string(ddl)) // simple protocol: multi-statement
	must(err)

	code := m.Run()
	pool.Close()
	_, _ = ac.Exec(ctx, `DROP DATABASE IF EXISTS clusterx_poc WITH (FORCE)`)
	_ = ac.Close(ctx)
	os.Exit(code)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func newStore(t *testing.T, lease time.Duration) *Store {
	t.Helper()
	if pool == nil {
		t.Skip("CLUSTERX_PG_ADMIN_DSN not set")
	}
	_, err := pool.Exec(context.Background(), `TRUNCATE jobs, job_attempts, job_events, audit_events,
		submission_idempotency, worker_capabilities, workers, worker_credentials CASCADE`)
	if err != nil {
		t.Fatal(err)
	}
	return &Store{Pool: pool, LeaseTTL: lease, CancelGrace: 15 * time.Second}
}

func q[T any](t *testing.T, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func workers(t *testing.T, s *Store, n int) []Worker {
	t.Helper()
	ws := make([]Worker, n)
	for i := range ws {
		w, err := s.NewWorker(context.Background(), handler, 8)
		if err != nil {
			t.Fatal(err)
		}
		ws[i] = w
	}
	return ws
}

func eventTypes(t *testing.T, job string) string {
	return q[string](t, `SELECT string_agg(event_type, ',' ORDER BY version) FROM job_events WHERE job_id = $1`, job)
}

// assertCoherent checks the invariants every test relies on: version == #events == max(event version),
// at most one active attempt, and running <=> exactly one active attempt.
func assertCoherent(t *testing.T, job string) {
	t.Helper()
	ok := q[bool](t, `SELECT j.version = (SELECT count(*) FROM job_events e WHERE e.job_id = j.id)
		AND j.version = (SELECT max(version) FROM job_events e WHERE e.job_id = j.id)
		AND (j.status = 'running') = ((SELECT count(*) FROM job_attempts a WHERE a.job_id = j.id AND a.state = 'active') = 1)
		AND (SELECT count(*) FROM job_attempts a WHERE a.job_id = j.id AND a.state = 'active') <= 1
		FROM jobs j WHERE j.id = $1`, job)
	if !ok {
		t.Fatalf("job %s history incoherent", job)
	}
}

// AC-02: >= 20 simultaneous claimants, one job -> exactly one authorized lease.
func TestAC02_ConcurrentClaimantsSingleJob(t *testing.T) {
	s := newStore(t, 30*time.Second)
	ctx := context.Background()
	for round := 0; round < 20; round++ {
		job, err := s.Admit(ctx, handler, 3)
		if err != nil {
			t.Fatal(err)
		}
		ws := workers(t, s, 25)
		var wg sync.WaitGroup
		start := make(chan struct{})
		var mu sync.Mutex
		won := 0
		for _, w := range ws {
			wg.Add(1)
			go func(w Worker) {
				defer wg.Done()
				<-start
				ls, err := s.Claim(ctx, w, 1)
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				won += len(ls)
				mu.Unlock()
			}(w)
		}
		close(start)
		wg.Wait()
		if won != 1 {
			t.Fatalf("round %d: %d leases granted, want 1", round, won)
		}
		if n := q[int](t, `SELECT count(*) FROM job_attempts WHERE job_id = $1`, job); n != 1 {
			t.Fatalf("round %d: %d attempts, want 1", round, n)
		}
		assertCoherent(t, job)
	}
}

// AC-02: 20 claimants draining 200 jobs; every job leased exactly once, never two active attempts.
func TestAC02_ConcurrentClaimantsManyJobs(t *testing.T) {
	s := newStore(t, 30*time.Second)
	ctx := context.Background()
	const jobs = 200
	for i := 0; i < jobs; i++ {
		if _, err := s.Admit(ctx, handler, 3); err != nil {
			t.Fatal(err)
		}
	}
	ws := workers(t, s, 20)
	for i := range ws { // lift capacity so the pool can drain
		if _, err := pool.Exec(ctx, `UPDATE workers SET capacity = 256 WHERE id = $1`, ws[i].ID); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]int{}
	for _, w := range ws {
		wg.Add(1)
		go func(w Worker) {
			defer wg.Done()
			for {
				ls, err := s.Claim(ctx, w, 3)
				if err != nil {
					t.Error(err)
					return
				}
				if len(ls) == 0 {
					return
				}
				mu.Lock()
				for _, l := range ls {
					seen[l.JobID]++
				}
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	if len(seen) != jobs {
		t.Fatalf("leased %d distinct jobs, want %d", len(seen), jobs)
	}
	for j, n := range seen {
		if n != 1 {
			t.Fatalf("job %s leased %d times", j, n)
		}
	}
	if n := q[int](t, `SELECT count(*) FROM job_attempts`); n != jobs {
		t.Fatalf("%d attempts, want %d", n, jobs)
	}
	if n := q[int](t, `SELECT count(*) FROM jobs WHERE status <> 'running' OR version <> 2`); n != 0 {
		t.Fatalf("%d jobs not running@v2", n)
	}
}

// AC-02: failpoints at every step of a claim leave no partial job/attempt/event writes.
func TestAC02_FailpointsLeaveNoPartialWrites(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("injected failure")
	for _, point := range []string{"claim.after_attempt_insert", "claim.after_job_update"} {
		t.Run(point, func(t *testing.T) {
			s := newStore(t, 30*time.Second)
			job, err := s.Admit(ctx, handler, 3)
			if err != nil {
				t.Fatal(err)
			}
			w := workers(t, s, 1)[0]
			s.Failpoint = func(name string) error {
				if name == point {
					return boom
				}
				return nil
			}
			if _, err := s.Claim(ctx, w, 1); !errors.Is(err, boom) {
				t.Fatalf("claim err = %v, want injected", err)
			}
			if st := q[string](t, `SELECT status || '@' || version || '/' || attempts_used || '/' || lease_generation FROM jobs WHERE id = $1`, job); st != "queued@1/0/0" {
				t.Fatalf("job after failed claim = %s, want queued@1/0/0", st)
			}
			if n := q[int](t, `SELECT count(*) FROM job_attempts`); n != 0 {
				t.Fatalf("%d attempts after failed claim", n)
			}
			if ev := eventTypes(t, job); ev != "admitted" {
				t.Fatalf("events = %s", ev)
			}
			s.Failpoint = nil // the job is still claimable afterwards
			if ls, err := s.Claim(ctx, w, 1); err != nil || len(ls) != 1 || ls[0].Generation != 1 {
				t.Fatalf("reclaim = %v, %v", ls, err)
			}
			assertCoherent(t, job)
		})
	}
	t.Run("admit.after_job_insert", func(t *testing.T) {
		s := newStore(t, 30*time.Second)
		s.Failpoint = func(name string) error {
			if name == "admit.after_job_insert" {
				return boom
			}
			return nil
		}
		if _, err := s.Admit(ctx, handler, 3); !errors.Is(err, boom) {
			t.Fatalf("admit err = %v", err)
		}
		if n := q[int](t, `SELECT count(*) FROM jobs`); n != 0 {
			t.Fatalf("%d jobs after failed admit", n)
		}
	})
	t.Run("report.before_event", func(t *testing.T) {
		s := newStore(t, 30*time.Second)
		job, _ := s.Admit(ctx, handler, 3)
		w := workers(t, s, 1)[0]
		ls, err := s.Claim(ctx, w, 1)
		if err != nil || len(ls) != 1 {
			t.Fatal(ls, err)
		}
		s.Failpoint = func(name string) error {
			if name == "report.before_event" {
				return boom
			}
			return nil
		}
		if _, err := s.ReportSuccess(ctx, ls[0], `{"ok":true}`); !errors.Is(err, boom) {
			t.Fatalf("report err = %v", err)
		}
		if st := q[string](t, `SELECT j.status || '/' || a.state FROM jobs j JOIN job_attempts a ON a.job_id = j.id WHERE j.id = $1`, job); st != "running/active" {
			t.Fatalf("after failed report = %s", st)
		}
		assertCoherent(t, job)
	})
}

// AC-02 (DB guard): even a buggy writer cannot create a second active attempt.
func TestAC02_SchemaRejectsSecondActiveAttempt(t *testing.T) {
	s := newStore(t, 30*time.Second)
	ctx := context.Background()
	job, _ := s.Admit(ctx, handler, 3)
	ws := workers(t, s, 2)
	if _, err := s.Claim(ctx, ws[0], 1); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO job_attempts (job_id, attempt_number, lease_generation, worker_id, worker_incarnation_id)
		VALUES ($1, 2, 2, $2, $3)`, job, ws[1].ID, ws[1].Incarnation)
	if err == nil || !strings.Contains(err.Error(), "job_attempts_one_active_per_job") {
		t.Fatalf("second active attempt err = %v, want unique violation", err)
	}
}

// AC-03: A's lease expires, B reclaims and succeeds, A's late success is rejected.
func TestAC03_StaleSuccessRejectedAfterExpiryAndReclaim(t *testing.T) {
	s := newStore(t, 300*time.Millisecond)
	ctx := context.Background()
	job, _ := s.Admit(ctx, handler, 3)
	ws := workers(t, s, 2)
	a, b := ws[0], ws[1]

	la, err := s.Claim(ctx, a, 1)
	if err != nil || len(la) != 1 {
		t.Fatal(la, err)
	}
	time.Sleep(450 * time.Millisecond) // DB clock passes lease_expires_at

	// Late success before recovery: lease expired, nothing changes.
	if _, err := s.ReportSuccess(ctx, la[0], `{"by":"A"}`); !isDenial(err, "lease_expired") {
		t.Fatalf("A after expiry: %v, want lease_expired", err)
	}
	if n, err := s.RecoverExpired(ctx, 0); err != nil || n != 1 {
		t.Fatalf("recover = %d, %v", n, err)
	}
	if _, err := s.PromoteDue(ctx); err != nil {
		t.Fatal(err)
	}
	s.LeaseTTL = 30 * time.Second
	lb, err := s.Claim(ctx, b, 1)
	if err != nil || len(lb) != 1 || lb[0].Generation != 2 {
		t.Fatalf("B claim = %v, %v", lb, err)
	}
	if out, err := s.ReportSuccess(ctx, lb[0], `{"by":"B"}`); err != nil || out != Succeeded {
		t.Fatalf("B success = %v, %v", out, err)
	}
	if _, err := s.ReportSuccess(ctx, la[0], `{"by":"A"}`); !isDenial(err, "job_terminal") {
		t.Fatalf("A after B: %v, want job_terminal", err)
	}
	// Duplicate of B's committed success is idempotent and changes nothing.
	if out, err := s.ReportSuccess(ctx, lb[0], `{"by":"B"}`); err != nil || out != Succeeded {
		t.Fatalf("B duplicate = %v, %v", out, err)
	}

	if r := q[string](t, `SELECT result->>'by' FROM jobs WHERE id = $1`, job); r != "B" {
		t.Fatalf("result by %s, want B", r)
	}
	if st := q[string](t, `SELECT string_agg(attempt_number || ':' || state, ',' ORDER BY attempt_number) FROM job_attempts WHERE job_id = $1`, job); st != "1:lease_expired,2:succeeded" {
		t.Fatalf("attempts = %s", st)
	}
	if ev := eventTypes(t, job); ev != "admitted,claimed,lease_expired,promoted,claimed,succeeded" {
		t.Fatalf("events = %s", ev)
	}
	if n := q[int](t, `SELECT count(*) FROM audit_events WHERE category = 'diagnostic' AND target_id = $1`, job); n != 2 {
		t.Fatalf("%d diagnostic rows, want 2", n)
	}
	assertCoherent(t, job)
	// Terminal immutability is enforced by the DB, not only by the code path.
	if _, err := pool.Exec(ctx, `UPDATE jobs SET result = '{"by":"A"}' WHERE id = $1`, job); err == nil {
		t.Fatal("terminal job was updatable")
	}
}

// AC-06: cancel committed first -> success denied, job canceled; late callbacks denied.
func TestAC06_CancelThenSuccess(t *testing.T) {
	s := newStore(t, 30*time.Second)
	ctx := context.Background()
	job, _ := s.Admit(ctx, handler, 3)
	l, _ := s.Claim(ctx, workers(t, s, 1)[0], 1)
	if r, err := s.RequestCancel(ctx, job, "user:op"); err != nil || r.HTTP != 202 {
		t.Fatalf("cancel = %+v, %v", r, err)
	}
	if out, err := s.ReportSuccess(ctx, l[0], `{}`); err != nil || out != Canceled {
		t.Fatalf("success after cancel = %v, %v; want canceled", out, err)
	}
	if _, err := s.ReportSuccess(ctx, l[0], `{}`); !isDenial(err, "job_terminal") {
		t.Fatalf("late success = %v", err)
	}
	if r, _ := s.RequestCancel(ctx, job, "user:op"); r.HTTP != 200 {
		t.Fatalf("repeat cancel = %+v, want 200 no-op", r)
	}
	if st := q[string](t, `SELECT status FROM jobs WHERE id = $1`, job); st != "canceled" {
		t.Fatalf("status %s", st)
	}
	if ev := eventTypes(t, job); ev != "admitted,claimed,cancel_requested,canceled" {
		t.Fatalf("events = %s", ev)
	}
	assertCoherent(t, job)
}

// AC-06: success committed first -> cancel is a 409 no-op; success never rolled back.
func TestAC06_SuccessThenCancel(t *testing.T) {
	s := newStore(t, 30*time.Second)
	ctx := context.Background()
	job, _ := s.Admit(ctx, handler, 3)
	l, _ := s.Claim(ctx, workers(t, s, 1)[0], 1)
	if out, err := s.ReportSuccess(ctx, l[0], `{"v":1}`); err != nil || out != Succeeded {
		t.Fatal(out, err)
	}
	if r, err := s.RequestCancel(ctx, job, "user:op"); err != nil || r.HTTP != 409 || r.Status != "succeeded" {
		t.Fatalf("cancel after success = %+v, %v", r, err)
	}
	if st := q[string](t, `SELECT status || coalesce(cancel_requested_at::text, '') FROM jobs WHERE id = $1`, job); st != "succeeded" {
		t.Fatalf("status %s", st)
	}
	if ev := eventTypes(t, job); ev != "admitted,claimed,succeeded" {
		t.Fatalf("events = %s", ev)
	}
	assertCoherent(t, job)
}

// AC-06: concurrent cancel vs success yields exactly one terminal outcome, consistent on both sides.
func TestAC06_ConcurrentCancelVsSuccess(t *testing.T) {
	s := newStore(t, 30*time.Second)
	ctx := context.Background()
	w := workers(t, s, 1)[0]
	_, _ = pool.Exec(ctx, `UPDATE workers SET capacity = 256 WHERE id = $1`, w.ID)
	tally := map[string]int{}
	for i := 0; i < 100; i++ {
		job, _ := s.Admit(ctx, handler, 3)
		l, err := s.Claim(ctx, w, 1)
		if err != nil || len(l) != 1 {
			t.Fatal(l, err)
		}
		var cr CancelResult
		var out Outcome
		var cerr, serr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; cr, cerr = s.RequestCancel(ctx, job, "user:op") }()
		go func() { defer wg.Done(); <-start; out, serr = s.ReportSuccess(ctx, l[0], `{}`) }()
		close(start)
		wg.Wait()
		if cerr != nil || serr != nil {
			t.Fatalf("iter %d: cancel err %v, success err %v", i, cerr, serr)
		}
		final := q[string](t, `SELECT status FROM jobs WHERE id = $1`, job)
		switch {
		case cr.HTTP == 409 && out == Succeeded && final == "succeeded":
			tally["success-first"]++
		case cr.HTTP == 202 && out == Canceled && final == "canceled":
			tally["cancel-first"]++
		default:
			t.Fatalf("iter %d: inconsistent cancel=%+v success=%v final=%s", i, cr, out, final)
		}
		if n := q[int](t, `SELECT count(*) FROM job_events WHERE job_id = $1 AND event_type IN ('succeeded', 'canceled')`, job); n != 1 {
			t.Fatalf("iter %d: %d terminal events", i, n)
		}
		assertCoherent(t, job)
	}
	t.Logf("orderings observed: %v", tally)
}

func isDenial(err error, reason string) bool {
	var d *Denial
	return errors.As(err, &d) && d.Reason == reason
}

// AC-05 (partial) + OQ-08: max_attempts=3 yields exactly 3 attempts, then failed with failure_class; no 4th claim.
func TestBudgetExhaustedNoHiddenAttempt(t *testing.T) {
	s := newStore(t, 100*time.Millisecond)
	ctx := context.Background()
	job, _ := s.Admit(ctx, handler, 3)
	w := workers(t, s, 1)[0]
	for i := 1; i <= 3; i++ {
		ls, err := s.Claim(ctx, w, 1)
		if err != nil || len(ls) != 1 || ls[0].Generation != int64(i) {
			t.Fatalf("claim %d = %v, %v", i, ls, err)
		}
		time.Sleep(150 * time.Millisecond)
		if n, err := s.RecoverExpired(ctx, 0); err != nil || n != 1 {
			t.Fatalf("recover %d = %d, %v", i, n, err)
		}
		if _, err := s.PromoteDue(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if ls, err := s.Claim(ctx, w, 1); err != nil || len(ls) != 0 {
		t.Fatalf("4th claim = %v, %v; want none", ls, err)
	}
	if st := q[string](t, `SELECT status || '/' || failure_class || '/' || attempts_used FROM jobs WHERE id = $1`, job); st != "failed/lease_expired/3" {
		t.Fatalf("job = %s", st)
	}
	if n := q[int](t, `SELECT count(*) FROM job_attempts WHERE job_id = $1`, job); n != 3 {
		t.Fatalf("%d attempts", n)
	}
	assertCoherent(t, job)
}
