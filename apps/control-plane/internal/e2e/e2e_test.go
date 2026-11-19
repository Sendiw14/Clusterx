// Package e2e runs the coordinator (store + gRPC + scheduler) with real SDK workers over loopback
// gRPC against PostgreSQL 18. Workers are goroutine-isolated: each has its own credential, gRPC
// connection, registry and incarnation (AC-18 in-process; AC-04 preview; SIGTERM drain).
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/scheduler"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
	basicjobs "github.com/Sendiw14/Clusterx/examples/basic-jobs"
	sdk "github.com/Sendiw14/Clusterx/packages/sdk-go"
)

var bg = context.Background()

type cluster struct {
	env  *testenv.Env
	addr string
}

func newCluster(t *testing.T, lease time.Duration, conns int32) *cluster {
	t.Helper()
	env := testenv.New(t, testenv.FastTiming(lease), conns)
	addr := env.ServeGRPC(t)
	ctx, cancel := context.WithCancel(bg)
	var wg sync.WaitGroup
	wg.Go(func() { scheduler.Run(ctx, env.Store, 50*time.Millisecond, testenv.Discard) })
	t.Cleanup(func() { cancel(); wg.Wait() })
	return &cluster{env: env, addr: addr}
}

type worker struct {
	conn   *grpc.ClientConn
	w      *sdk.Worker
	cancel context.CancelFunc
	done   chan error
}

// startWorker runs one isolated SDK worker with its own credential and connection.
func (c *cluster) startWorker(t *testing.T, capacity int, grace time.Duration) *worker {
	t.Helper()
	cred, err := c.env.Store.CreateWorkerCredential(bg, "e2e", 0)
	if err != nil {
		t.Fatal(err)
	}
	conn := testenv.Dial(t, c.addr, cred.Secret)
	var reg sdk.Registry
	if err := basicjobs.Register(&reg); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(bg)
	w := &worker{conn: conn, cancel: cancel, done: make(chan error, 1),
		w: sdk.NewWorker(conn, &reg, sdk.WorkerOptions{Name: "e2e", Capacity: capacity, DrainGrace: grace,
			ClaimWait: 200 * time.Millisecond, Logger: testenv.Discard})}
	go func() { w.done <- w.w.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-w.done })
	return w
}

func (c *cluster) submit(t *testing.T, handler, payload string, maxAttempts int) string {
	t.Helper()
	j, _, err := c.env.Store.Admit(bg, store.AdmitParams{Principal: "token:e2e", Namespace: "default", HandlerName: handler,
		HandlerVersion: 1, MaxAttempts: maxAttempts, TimeoutSeconds: 60, Payload: json.RawMessage(payload)})
	if err != nil {
		t.Fatal(err)
	}
	return j.ID
}

func (c *cluster) q(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var s string
	if err := c.env.Store.Pool.QueryRow(bg, sql, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return s
}

func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// AC-18 (in-process): 1,000 jobs, 3 isolated workers over real gRPC; submitted = terminal and
// attempt counts are exact.
func TestAC18_ThousandJobsThreeWorkers(t *testing.T) {
	c := newCluster(t, 10*time.Second, 40)
	const total = 1000
	ids := make(map[string]int, total) // job id -> expected attempts
	start := time.Now()
	for i := range total {
		switch i % 4 {
		case 0:
			ids[c.submit(t, "sleep_ms", fmt.Sprintf(`{"ms":%d}`, i%5), 3)] = 1
		case 1:
			ids[c.submit(t, "hash_sha256", fmt.Sprintf(`{"data":"job-%d"}`, i), 3)] = 1
		case 2:
			ids[c.submit(t, "fail_n_times", `{"n":1}`, 3)] = 2
		case 3:
			ids[c.submit(t, "fail_n_times", `{"n":0}`, 3)] = 1
		}
	}
	submitted := time.Since(start)
	ws := []*worker{c.startWorker(t, 8, 5*time.Second), c.startWorker(t, 8, 5*time.Second), c.startWorker(t, 8, 5*time.Second)}
	waitFor(t, 3*time.Minute, "all jobs terminal", func() bool {
		return c.q(t, `SELECT count(*)::text FROM jobs WHERE terminal_at IS NOT NULL`) == fmt.Sprint(total)
	})
	elapsed := time.Since(start)
	for _, w := range ws {
		w.cancel()
	}
	for _, w := range ws {
		if err := <-w.done; err != nil {
			t.Fatalf("worker: %v", err)
		}
		w.done <- nil // for cleanup
	}
	// Reconcile.
	if got := c.q(t, `SELECT count(*)::text FROM jobs`); got != fmt.Sprint(total) {
		t.Fatalf("jobs = %s", got)
	}
	if got := c.q(t, `SELECT coalesce(string_agg(status || '=' || n, ',' ORDER BY status), '') FROM
		(SELECT status, count(*) n FROM jobs GROUP BY status) x`); got != fmt.Sprintf("succeeded=%d", total) {
		t.Fatalf("status counts: %s", got)
	}
	rows, err := c.env.Store.Pool.Query(bg, `SELECT j.id::text, j.attempts_used, (SELECT count(*) FROM job_attempts a WHERE a.job_id = j.id),
		(SELECT count(*) FROM job_attempts a WHERE a.job_id = j.id AND a.state = 'failed'),
		j.version, (SELECT count(*) FROM job_events e WHERE e.job_id = j.id), (SELECT max(version) FROM job_events e WHERE e.job_id = j.id)
		FROM jobs j`)
	if err != nil {
		t.Fatal(err)
	}
	var attempts, retries int
	for rows.Next() {
		var id string
		var used, nAtt, nFailed int
		var version, nEv, maxEv int64
		if err := rows.Scan(&id, &used, &nAtt, &nFailed, &version, &nEv, &maxEv); err != nil {
			t.Fatal(err)
		}
		if want := ids[id]; used != want || nAtt != want || nFailed != want-1 {
			t.Fatalf("job %s: attempts_used=%d rows=%d failed=%d want %d", id, used, nAtt, nFailed, want)
		}
		if nEv != version || maxEv != version {
			t.Fatalf("job %s: version %d, events %d, max %d", id, version, nEv, maxEv)
		}
		attempts += nAtt
		retries += nFailed
	}
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	perWorker := c.q(t, `SELECT string_agg(n::text, ',' ORDER BY n) FROM (SELECT count(*) n FROM job_attempts GROUP BY worker_id) x`)
	active := c.q(t, `SELECT count(*)::text FROM job_attempts WHERE state = 'active'`)
	t.Logf("AC-18 raw: submitted=%d terminal=%d succeeded=%d attempts=%d retries=%d active_attempts=%s attempts_per_worker=[%s] submit_wall=%s total_wall=%s",
		total, total, total, attempts, retries, active, perWorker, submitted.Round(time.Millisecond), elapsed.Round(time.Millisecond))
	if attempts != total+total/4 || retries != total/4 || active != "0" {
		t.Fatalf("attempts=%d retries=%d active=%s", attempts, retries, active)
	}
	// CR-M5-01 #5: handler logs reach log_tail through the SDK (fail_n_times logs every attempt).
	logged := c.q(t, `SELECT count(*)::text FROM job_attempts a JOIN jobs j ON j.id = a.job_id
		WHERE j.handler_name = 'fail_n_times' AND a.log_tail LIKE '%fail_n_times attempt%' AND NOT a.log_truncated`)
	if logged != fmt.Sprint(total/2+total/4) {
		t.Fatalf("attempts with captured log_tail = %s, want %d", logged, total/2+total/4)
	}
}

// AC-04 preview: a worker that loses the coordinator mid-handler is fenced; the attempt expires and
// the job is retried and completed by another worker; the stale report is never committed.
func TestAC04_WorkerLostMidJobRecovers(t *testing.T) {
	c := newCluster(t, 1*time.Second, 20)
	a := c.startWorker(t, 1, time.Second)
	id := c.submit(t, "sleep_ms", `{"ms":2500}`, 3)
	waitFor(t, 10*time.Second, "job running on worker A", func() bool {
		return c.q(t, `SELECT status FROM jobs WHERE id = $1`, id) == "running"
	})
	// "Kill" A's process boundary: its connection to the coordinator is gone for good.
	_ = a.conn.Close()
	waitFor(t, 10*time.Second, "lease expiry recovery", func() bool {
		return c.q(t, `SELECT count(*)::text FROM job_attempts WHERE job_id = $1 AND state = 'lease_expired'`, id) == "1"
	})
	c.startWorker(t, 1, time.Second)
	waitFor(t, 20*time.Second, "job succeeded on worker B", func() bool {
		return c.q(t, `SELECT status FROM jobs WHERE id = $1`, id) == "succeeded"
	})
	if got := c.q(t, `SELECT string_agg(attempt_number || ':' || state, ',' ORDER BY attempt_number) FROM job_attempts WHERE job_id = $1`, id); got != "1:lease_expired,2:succeeded" {
		t.Fatalf("attempts: %s", got)
	}
	if got := c.q(t, `SELECT count(DISTINCT worker_id)::text FROM job_attempts WHERE job_id = $1`, id); got != "2" {
		t.Fatalf("workers: %s", got)
	}
}

// SIGTERM drain: a worker asked to stop finishes a short handler within the grace period, and a
// handler longer than the grace is canceled and reported retryable (worker_shutdown), not lost.
func TestGracefulDrain(t *testing.T) {
	c := newCluster(t, 5*time.Second, 20)
	w := c.startWorker(t, 2, 700*time.Millisecond)
	short := c.submit(t, "sleep_ms", `{"ms":300}`, 3)
	long := c.submit(t, "sleep_ms", `{"ms":20000}`, 3)
	waitFor(t, 10*time.Second, "both running", func() bool {
		return c.q(t, `SELECT count(*)::text FROM jobs WHERE status = 'running'`) == "2"
	})
	w.cancel()
	if err := <-w.done; err != nil {
		t.Fatal(err)
	}
	w.done <- nil
	if got := c.q(t, `SELECT status FROM jobs WHERE id = $1`, short); got != "succeeded" {
		t.Fatalf("short job: %s", got)
	}
	if got := c.q(t, `SELECT j.status || '/' || a.error_code FROM jobs j JOIN job_attempts a ON a.job_id = j.id WHERE j.id = $1`, long); got != "retry_wait/worker_shutdown" &&
		got != "queued/worker_shutdown" {
		t.Fatalf("long job: %s", got)
	}
	if got := c.q(t, `SELECT status FROM workers`); got != "deregistered" {
		t.Fatalf("worker status: %s", got)
	}
}

// Cancellation: a running job canceled by an operator is observed by the handler through the lease
// renewal and finalized canceled.
func TestCancelRunningJob(t *testing.T) {
	c := newCluster(t, 1*time.Second, 20)
	c.startWorker(t, 1, time.Second)
	id := c.submit(t, "sleep_ms", `{"ms":30000}`, 3)
	waitFor(t, 10*time.Second, "running", func() bool { return c.q(t, `SELECT status FROM jobs WHERE id = $1`, id) == "running" })
	if r, err := c.env.Store.RequestCancel(bg, id, "token:e2e"); err != nil || r.HTTP != 202 {
		t.Fatalf("cancel: %+v %v", r, err)
	}
	waitFor(t, 10*time.Second, "canceled", func() bool { return c.q(t, `SELECT status FROM jobs WHERE id = $1`, id) == "canceled" })
	if got := c.q(t, `SELECT state FROM job_attempts WHERE job_id = $1`, id); got != "canceled" {
		t.Fatalf("attempt: %s (worker ack expected before lease expiry)", got)
	}
}
