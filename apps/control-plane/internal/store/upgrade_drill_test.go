package store

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/migrate"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/pgtest"
	"github.com/Sendiw14/Clusterx/apps/control-plane/migrations"
)

// seed0002 is data as written by the M4 (schema 0002) engine: one job in every status, finished and
// active attempts, contiguous events and an idempotency row. The running job's lease already expired.
const seed0002 = `
INSERT INTO worker_credentials (id, name, secret_hash, prefix) VALUES
  ('00000000-0000-7000-8000-0000000000c1', 'old', sha256('old'::bytea), 'cxw_old0');
INSERT INTO workers (id, name, credential_id, current_incarnation_id, capacity, protocol_version, last_heartbeat_at) VALUES
  ('00000000-0000-7000-8000-0000000000a1', 'old-worker', '00000000-0000-7000-8000-0000000000c1',
   '00000000-0000-7000-8000-0000000000b1', 4, 1, now() - interval '1 hour');
INSERT INTO worker_capabilities VALUES ('00000000-0000-7000-8000-0000000000a1', 'hash_sha256', 1);

BEGIN;
INSERT INTO jobs (id, handler_name, handler_version, payload, status, run_after, available_at, attempts_used, lease_generation,
  current_attempt_id, lease_expires_at, hard_deadline_at, cancel_requested_at, result, error_code, failure_class, terminal_at,
  created_by, version) VALUES
 ('00000000-0000-7000-8000-000000000001', 'hash_sha256', 1, '{"data":"q"}', 'queued', NULL, now() - interval '1 minute', 0, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'token:old', 1),
 ('00000000-0000-7000-8000-000000000002', 'hash_sha256', 1, '{"data":"s"}', 'scheduled', now() + interval '1 hour', now() + interval '1 hour', 0, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'token:old', 1),
 ('00000000-0000-7000-8000-000000000003', 'hash_sha256', 1, '{"data":"r"}', 'running', NULL, now() - interval '5 minutes', 1, 1, '00000000-0000-7000-8000-0000000000e3', now() + interval '1 minute', now() + interval '10 minutes', NULL, NULL, NULL, NULL, NULL, 'token:old', 3),
 ('00000000-0000-7000-8000-000000000004', 'hash_sha256', 1, '{"data":"x"}', 'running', NULL, now() - interval '5 minutes', 1, 1, '00000000-0000-7000-8000-0000000000e4', now() - interval '1 minute', now() + interval '10 minutes', NULL, NULL, NULL, NULL, NULL, 'token:old', 2),
 ('00000000-0000-7000-8000-000000000005', 'hash_sha256', 1, '{"data":"ok"}', 'running', NULL, now() - interval '5 minutes', 1, 1, '00000000-0000-7000-8000-0000000000e5', now() + interval '1 minute', now() + interval '10 minutes', NULL, NULL, NULL, NULL, NULL, 'token:old', 3),
 ('00000000-0000-7000-8000-000000000006', 'hash_sha256', 1, '{"data":"f"}', 'running', NULL, now() - interval '5 minutes', 1, 1, '00000000-0000-7000-8000-0000000000e6', now() + interval '1 minute', now() + interval '10 minutes', NULL, NULL, NULL, NULL, NULL, 'token:old', 3),
 ('00000000-0000-7000-8000-000000000007', 'hash_sha256', 1, '{"data":"c"}', 'canceled', NULL, now() - interval '5 minutes', 0, 0, NULL, NULL, NULL, now() - interval '4 minutes', NULL, NULL, 'canceled_by_operator', now() - interval '4 minutes', 'token:old', 2);
-- Attempts are born active as the current lease (RV01-05 insert guard), then finished, as the engine does.
INSERT INTO job_attempts (id, job_id, attempt_number, lease_generation, worker_id, worker_incarnation_id, started_at)
  SELECT ('00000000-0000-7000-8000-0000000000e' || n)::uuid, ('00000000-0000-7000-8000-00000000000' || n)::uuid, 1, 1,
         '00000000-0000-7000-8000-0000000000a1', '00000000-0000-7000-8000-0000000000b1', now() - interval '3 minutes'
  FROM generate_series(3, 6) n;
UPDATE job_attempts SET state = 'failed', error_class = 'retryable', error_code = 'flaky', finished_at = now() - interval '2 minutes'
  WHERE id = '00000000-0000-7000-8000-0000000000e3';
UPDATE job_attempts SET state = 'succeeded', finished_at = now() - interval '2 minutes' WHERE id = '00000000-0000-7000-8000-0000000000e5';
UPDATE job_attempts SET state = 'failed', error_class = 'permanent', error_code = 'bad_input', finished_at = now() - interval '2 minutes'
  WHERE id = '00000000-0000-7000-8000-0000000000e6';
UPDATE jobs SET status = 'retry_wait', available_at = now() + interval '10 minutes', current_attempt_id = NULL, lease_expires_at = NULL,
  hard_deadline_at = NULL WHERE id = '00000000-0000-7000-8000-000000000003';
UPDATE jobs SET status = 'succeeded', result = '{"sha256":"aa"}', terminal_at = now() - interval '2 minutes', current_attempt_id = NULL,
  lease_expires_at = NULL, hard_deadline_at = NULL WHERE id = '00000000-0000-7000-8000-000000000005';
UPDATE jobs SET status = 'failed', error_code = 'bad_input', failure_class = 'permanent', terminal_at = now() - interval '2 minutes',
  current_attempt_id = NULL, lease_expires_at = NULL, hard_deadline_at = NULL WHERE id = '00000000-0000-7000-8000-000000000006';
INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor) VALUES
 ('00000000-0000-7000-8000-000000000001', 1, 'admitted', NULL, 'queued', 'token:old'),
 ('00000000-0000-7000-8000-000000000002', 1, 'admitted', NULL, 'scheduled', 'token:old'),
 ('00000000-0000-7000-8000-000000000003', 1, 'admitted', NULL, 'queued', 'token:old'),
 ('00000000-0000-7000-8000-000000000003', 2, 'claimed', 'queued', 'running', 'worker:old'),
 ('00000000-0000-7000-8000-000000000003', 3, 'failed_retryable', 'running', 'retry_wait', 'worker:old'),
 ('00000000-0000-7000-8000-000000000004', 1, 'admitted', NULL, 'queued', 'token:old'),
 ('00000000-0000-7000-8000-000000000004', 2, 'claimed', 'queued', 'running', 'worker:old'),
 ('00000000-0000-7000-8000-000000000005', 1, 'admitted', NULL, 'queued', 'token:old'),
 ('00000000-0000-7000-8000-000000000005', 2, 'claimed', 'queued', 'running', 'worker:old'),
 ('00000000-0000-7000-8000-000000000005', 3, 'succeeded', 'running', 'succeeded', 'worker:old'),
 ('00000000-0000-7000-8000-000000000006', 1, 'admitted', NULL, 'queued', 'token:old'),
 ('00000000-0000-7000-8000-000000000006', 2, 'claimed', 'queued', 'running', 'worker:old'),
 ('00000000-0000-7000-8000-000000000006', 3, 'failed_permanent', 'running', 'failed', 'worker:old'),
 ('00000000-0000-7000-8000-000000000007', 1, 'admitted', NULL, 'queued', 'token:old'),
 ('00000000-0000-7000-8000-000000000007', 2, 'canceled', 'queued', 'canceled', 'user:old');
INSERT INTO submission_idempotency (principal, namespace, idem_key, fingerprint, job_id, expires_at) VALUES
 ('token:old', 'default', 'k1', sha256('k1'::bytea), '00000000-0000-7000-8000-000000000001', now() + interval '1 day');
COMMIT;`

// invariantsSQL returns one row of counters that must all be 0 on a coherent database.
const invariantsSQL = `SELECT
  (SELECT count(*) FROM jobs j WHERE attempts_used <> (SELECT count(*) FROM job_attempts a WHERE a.job_id = j.id)),
  (SELECT count(*) FROM jobs j WHERE version <> (SELECT count(*) FROM job_events e WHERE e.job_id = j.id)
      OR version <> (SELECT max(version) FROM job_events e WHERE e.job_id = j.id)),
  (SELECT count(*) FROM jobs j WHERE (status = 'running') <> EXISTS (SELECT 1 FROM job_attempts a WHERE a.job_id = j.id AND a.state = 'active'))`

func checkInvariants(t *testing.T, ctx context.Context, c *pgx.Conn, when string) {
	t.Helper()
	var a, b, r int
	if err := c.QueryRow(ctx, invariantsSQL).Scan(&a, &b, &r); err != nil {
		t.Fatal(err)
	}
	if a+b+r != 0 {
		t.Fatalf("%s: attempt_count_mismatch=%d event_version_mismatch=%d running_attempt_mismatch=%d", when, a, b, r)
	}
}

// TestUpgradeDrill0002To0004 (M8, AC-17 migration part): a populated schema-0002 database is upgraded
// with the shipped 0003 and 0004 migrations; invariants hold before and after, and the current engine
// recovers the expired running job and finishes the queued work.
func TestUpgradeDrill0002To0004(t *testing.T) {
	ctx := context.Background()
	dsn := pgtest.NewDB(t)
	migs, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if n, err := migrate.Apply(ctx, c, migs[:2], log); err != nil || n != 2 {
		t.Fatalf("apply 0001-0002: n=%d err=%v", n, err)
	}
	if _, err := c.Exec(ctx, seed0002); err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, ctx, c, "before upgrade")
	statuses := func() map[string]int {
		m := map[string]int{}
		rows, _ := c.Query(ctx, `SELECT status, count(*) FROM jobs GROUP BY 1`)
		for rows.Next() {
			var s string
			var n int
			_ = rows.Scan(&s, &n)
			m[s] = n
		}
		rows.Close()
		return m
	}
	before := statuses()

	t0 := time.Now()
	n, err := migrate.Apply(ctx, c, migs, log)
	if err != nil || n != len(migs)-2 {
		t.Fatalf("upgrade: applied %d err=%v", n, err)
	}
	t.Logf("upgrade applied %d migrations (%s) in %s", n, migs[len(migs)-1].Name, time.Since(t0).Round(time.Millisecond))
	checkInvariants(t, ctx, c, "after upgrade")
	if after := statuses(); len(after) != 7 || after["running"] != 1 || len(before) != len(after) {
		t.Fatalf("statuses changed: before %v after %v", before, after)
	}
	var nullTxid int
	_ = c.QueryRow(ctx, `SELECT count(*) FROM job_events WHERE txid IS NULL`).Scan(&nullTxid)
	if nullTxid != 15 {
		t.Fatalf("pre-0004 events with txid NULL = %d, want 15 (never fed, ADR-0004)", nullTxid)
	}

	// Current engine on the upgraded schema.
	tm := DefaultTiming()
	tm.Backoff = Backoff{Base: time.Millisecond, Cap: time.Millisecond, Rand: func() float64 { return 0 }}
	s := New(openPool(t, dsn, 10), tm, allKnown{})
	if r, err := s.RecoverExpired(ctx, 100); err != nil || r != 1 {
		t.Fatalf("recover expired: %d %v", r, err)
	}
	time.Sleep(20 * time.Millisecond) // the 1 ms retry backoff of the recovered job
	if n, err := s.PromoteDue(ctx, 100); err != nil || n != 1 {
		t.Fatalf("promote recovered job: %d %v", n, err)
	}
	w := newWorker(t, s, 4, "hash_sha256")
	got, err := s.Claim(ctx, w, 4, nil)
	if err != nil || len(got) != 2 {
		t.Fatalf("claim after upgrade: %d %v", len(got), err)
	}
	for _, a := range got {
		ref := AttemptRef{WorkerRef: w, JobID: a.JobID, AttemptID: a.AttemptID, Generation: a.LeaseGeneration}
		if _, err := s.ReportSuccess(ctx, ref, json.RawMessage(`{"sha256":"bb"}`), ""); err != nil {
			t.Fatal(err)
		}
	}
	checkInvariants(t, ctx, c, "after recovery and new work")
	var j4 string
	var attempts int
	_ = c.QueryRow(ctx, `SELECT status, attempts_used FROM jobs WHERE id = '00000000-0000-7000-8000-000000000004'`).Scan(&j4, &attempts)
	var expired int
	_ = c.QueryRow(ctx, `SELECT count(*) FROM job_attempts WHERE job_id = '00000000-0000-7000-8000-000000000004' AND state = 'lease_expired'`).Scan(&expired)
	if j4 != "succeeded" || attempts != 2 || expired != 1 {
		t.Fatalf("expired running job after upgrade: %s attempts=%d lease_expired=%d", j4, attempts, expired)
	}
}
