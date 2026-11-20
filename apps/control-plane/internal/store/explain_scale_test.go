package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestExplainAtScale (M8, RV01-15 re-measure) seeds >=100k queued and >=100k terminal jobs, enables
// auto_explain (ANALYZE, BUFFERS) for new sessions of the test database, then runs the production code
// paths for claim, list, idempotent admission, the stream feed poll and the /metrics ops snapshot. The plans are written to the
// PostgreSQL server log; 20 concurrent claimants are timed. Opt-in: CLUSTERX_EXPLAIN_SCALE=1.
func TestExplainAtScale(t *testing.T) {
	if os.Getenv("CLUSTERX_EXPLAIN_SCALE") == "" {
		t.Skip("set CLUSTERX_EXPLAIN_SCALE=1 (needs a superuser DSN and server-log access)")
	}
	ctx := context.Background()
	dsn := migratedDB(t)
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := admin.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql[:min(60, len(sql))], err)
		}
	}
	start := time.Now()
	// 120k queued over 4 handlers (workers support 2 of them), priority 0..9, eligible in the past.
	exec(`INSERT INTO jobs (handler_name, handler_version, payload, priority, status, available_at, created_by, created_at)
		SELECT (ARRAY['hash_sha256','sleep_ms','report.daily','thumbnail.render'])[1 + i % 4], 1, '{"data":"x"}', i % 10, 'queued',
			now() - make_interval(secs => i % 3600), 'token:seed', now() - make_interval(secs => i % 3600)
		FROM generate_series(1, 120000) i`)
	// 150k terminal (succeeded) over the last 6 days, each with its admitted event, txid set.
	exec(`INSERT INTO jobs (handler_name, handler_version, payload, status, available_at, created_by, created_at, terminal_at, result)
		SELECT 'hash_sha256', 1, '{"data":"x"}', 'succeeded', t, 'token:seed', t, t + interval '1 second', '{}'
		FROM (SELECT now() - make_interval(secs => i * 3) t FROM generate_series(1, 150000) i) s`)
	exec(`INSERT INTO job_events (job_id, version, event_type, to_status, actor, occurred_at, txid)
		SELECT id, 1, 'admitted', status, 'token:seed', created_at, pg_current_xact_id() FROM jobs`)
	exec(`UPDATE jobs SET version = 1`)
	// 150k idempotency rows of one principal (the admission lookup is per principal+namespace+key).
	exec(`INSERT INTO submission_idempotency (principal, namespace, idem_key, fingerprint, job_id, created_at, expires_at)
		SELECT 'token:bench', 'default', 'k-' || row_number() OVER (), sha256(id::text::bytea), id, now(), now() + interval '1 day'
		FROM jobs WHERE status = 'succeeded'`)
	exec(`VACUUM ANALYZE jobs, job_events, submission_idempotency`)
	var counts string
	_ = admin.QueryRow(ctx, `SELECT format('queued=%s terminal=%s events=%s idempotency=%s',
		(SELECT count(*) FROM jobs WHERE status='queued'), (SELECT count(*) FROM jobs WHERE terminal_at IS NOT NULL),
		(SELECT count(*) FROM job_events), (SELECT count(*) FROM submission_idempotency))`).Scan(&counts)
	t.Logf("seeded in %s: %s", time.Since(start).Round(time.Millisecond), counts)

	// Workers are registered before auto_explain is on (their plans are not of interest).
	tm := DefaultTiming()
	workers := make([]WorkerRef, 20)
	{
		s := New(openPool(t, dsn, 30), tm, allKnown{})
		for i := range workers {
			workers[i] = newWorker(t, s, 256, "hash_sha256", "sleep_ms")
		}
		s.Pool.Close()
	}
	db := strings.TrimPrefix(dsnDB(t, dsn), "/")
	exec(fmt.Sprintf(`ALTER DATABASE %q SET session_preload_libraries = 'auto_explain'`, db))
	for _, kv := range []string{"log_min_duration = 0", "log_analyze = on", "log_buffers = on", "log_timing = on", "log_nested_statements = on", "log_format = text"} {
		exec(fmt.Sprintf(`ALTER DATABASE %q SET auto_explain.%s`, db, kv))
	}
	pool := openPool(t, dsn, 30)
	s := New(pool, tm, allKnown{})

	mark := func(name string) { _, _ = pool.Exec(ctx, `SELECT $1::text AS explain_marker`, "MARK "+name) }

	mark("claim single (8 rows)")
	if got, err := s.Claim(ctx, workers[0], 8, nil); err != nil || len(got) != 8 {
		t.Fatalf("claim: %d %v", len(got), err)
	}
	mark("list default page")
	if _, _, _, err := s.ListJobs(ctx, JobFilter{Limit: 50}); err != nil {
		t.Fatal(err)
	}
	mark("list status=queued handler=sleep_ms")
	if _, _, _, err := s.ListJobs(ctx, JobFilter{Limit: 50, Statuses: []string{"queued"}, HandlerName: "sleep_ms"}); err != nil {
		t.Fatal(err)
	}
	mark("idempotent admission replay (existing key)")
	payload := json.RawMessage(`{"data":"x"}`)
	fp := sha256.Sum256([]byte("new"))
	p := AdmitParams{Principal: "token:bench", Namespace: "default", HandlerName: "hash_sha256", HandlerVersion: 1, MaxAttempts: 3,
		TimeoutSeconds: 60, Payload: payload, IdempotencyKey: "k-777", Fingerprint: fp[:]}
	if _, _, err := s.Admit(ctx, p); err == nil {
		t.Log("note: replay with a different fingerprint was accepted") // expected: idempotency_conflict
	}
	mark("idempotent admission new key")
	p.IdempotencyKey = "k-new"
	if _, _, err := s.Admit(ctx, p); err != nil {
		t.Fatal(err)
	}
	mark("feed poll (copy of api.Feed.poll SQL)")
	var snap string
	_ = pool.QueryRow(ctx, `SELECT pg_current_snapshot()::text`).Scan(&snap)
	if _, _, err := s.Admit(ctx, AdmitParams{Principal: "token:x", Namespace: "default", HandlerName: "hash_sha256", HandlerVersion: 1,
		MaxAttempts: 3, TimeoutSeconds: 60, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `WITH s AS (SELECT pg_current_snapshot() AS cur)
		SELECT s.cur::text, pg_snapshot_xmax(s.cur) < pg_snapshot_xmax($1::pg_snapshot), e.job_id::text, e.version FROM s LEFT JOIN job_events e
		  ON e.txid >= pg_snapshot_xmin($1::pg_snapshot) AND NOT pg_visible_in_snapshot(e.txid, $1::pg_snapshot)
		LIMIT $2`, snap, 501)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	mark("ops snapshot (one /metrics scrape)")
	t0 := time.Now()
	if _, err := s.OpsSnapshot(ctx); err != nil {
		t.Fatal(err)
	}
	t.Logf("ops snapshot (auto_explain on): %s", time.Since(t0))
	mark("end")

	// 20 concurrent claimants, 10 claims of 8 each (1,600 attempts), timed client-side.
	var mu sync.Mutex
	var lats []time.Duration
	var wg sync.WaitGroup
	for _, w := range workers[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				t0 := time.Now()
				got, err := s.Claim(ctx, w, 8, nil)
				d := time.Since(t0)
				if err != nil || len(got) != 8 {
					t.Errorf("claim: %d %v", len(got), err)
					return
				}
				mu.Lock()
				lats = append(lats, d)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	slices.Sort(lats)
	pct := func(p float64) time.Duration { return lats[max(0, int(float64(len(lats))*p+0.999999)-1)] }
	t.Logf("concurrent claims (19 claimants x 10 x 8, auto_explain logging on): n=%d p50=%s p95=%s p99=%s max=%s",
		len(lats), pct(.5), pct(.95), pct(.99), lats[len(lats)-1])
}

func openPool(t *testing.T, dsn string, n int32) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	apply(cfg.ConnConfig)
	cfg.MaxConns = n
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func dsnDB(t *testing.T, dsn string) string {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Database
}
