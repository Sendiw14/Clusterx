package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
)

// RV5-04: a transition whose transaction commits long after its event's occurred_at is still hinted
// (visibility cursor, no clock look-back), without a resync.
func TestFeedLateCommitIsHinted(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 8)
	ctx := context.Background()
	j, _, err := env.Store.Admit(ctx, store.AdmitParams{Principal: "token:t", Namespace: "default", HandlerName: "sleep_ms",
		HandlerVersion: 1, MaxAttempts: 3, TimeoutSeconds: 60, Payload: json.RawMessage(`{"ms":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(nil)
	sub, _ := hub.subscribe("session:x")
	fctx, stop := context.WithCancel(ctx)
	defer stop()
	go (&Feed{Pool: env.Store.Pool, Hub: hub, Interval: 50 * time.Millisecond, Log: testenv.Discard}).Run(fctx)
	tx, err := env.Store.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var v int64
	if err := tx.QueryRow(ctx, `UPDATE jobs SET status = 'canceled', terminal_at = clock_timestamp(), cancel_requested_at = clock_timestamp(),
		failure_class = 'canceled_by_operator', version = version + 1, updated_at = clock_timestamp() WHERE id = $1 RETURNING version`, j.ID).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO job_events (job_id, version, event_type, from_status, to_status, actor, occurred_at)
		VALUES ($1, $2, 'canceled', 'queued', 'canceled', 'user:x', clock_timestamp() - interval '1 hour')`, j.ID, v); err != nil {
		t.Fatal(err)
	}
	// Another transaction commits meanwhile, so the polls' snapshots list ours as in progress (xip)
	// with xmax beyond it: a cursor of "txid >= previous xmax" would skip it after its commit.
	if _, _, err := env.Store.Admit(ctx, store.AdmitParams{Principal: "token:t", Namespace: "default", HandlerName: "sleep_ms",
		HandlerVersion: 1, MaxAttempts: 3, TimeoutSeconds: 60, Payload: json.RawMessage(`{"ms":1}`)}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond) // many polls while the transaction is open
	_ = sub.take()
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, m := range sub.take() {
			if m.Type == "resync" {
				t.Fatalf("unexpected resync %q", m.Reason)
			}
			if m.Type == "hint" && m.Entity == "job" && m.ID != nil && *m.ID == j.ID && *m.Version == v {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("late-committed transition was never hinted")
}

// RV5-04: overflow is the only gap; it resyncs, at most once per ResyncEvery.
func TestFeedOverflowResyncIsBounded(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 8)
	ctx := context.Background()
	hub := NewHub(nil)
	sub, _ := hub.subscribe("session:x")
	fctx, stop := context.WithCancel(ctx)
	defer stop()
	go (&Feed{Pool: env.Store.Pool, Hub: hub, Interval: 20 * time.Millisecond, MaxRows: 1, ResyncEvery: time.Hour,
		Log: testenv.Discard}).Run(fctx)
	time.Sleep(100 * time.Millisecond)
	for range 3 {
		for range 3 {
			if _, _, err := env.Store.Admit(ctx, store.AdmitParams{Principal: "token:t", Namespace: "default", HandlerName: "sleep_ms",
				HandlerVersion: 1, MaxAttempts: 3, TimeoutSeconds: 60, Payload: json.RawMessage(`{"ms":1}`)}); err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	resyncs := 0
	for _, m := range sub.take() {
		if m.Type == "resync" && m.Reason == "gap_detected" {
			resyncs++
		}
	}
	if resyncs != 1 {
		t.Fatalf("resyncs = %d, want exactly 1 (overflow, rate-bounded)", resyncs)
	}
}

// RV5R-03(b): a snapshot whose xmax is below the previous cursor's (restore / failover) is a gap.
// RV5R-06: worker ids no longer listed are dropped from the version map.
func TestFeedXidRegressionAndWorkerPrune(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 8)
	ctx := context.Background()
	f := &Feed{Pool: env.Store.Pool, Hub: NewHub(nil), MaxRows: 10}
	var snap string
	if err := env.Store.Pool.QueryRow(ctx, `SELECT pg_current_snapshot()::text`).Scan(&snap); err != nil {
		t.Fatal(err)
	}
	workers := map[string]int64{"00000000-0000-0000-0000-00000000dead": 3}
	if gap, err := f.poll(ctx, &snap, workers); err != nil || gap {
		t.Fatalf("normal poll: gap=%v err=%v", gap, err)
	}
	if _, ok := workers["00000000-0000-0000-0000-00000000dead"]; ok {
		t.Fatal("vanished worker id not pruned")
	}
	future := "999999999999:999999999999:"
	if gap, err := f.poll(ctx, &future, workers); err != nil || !gap {
		t.Fatalf("xid regression not reported as a gap: gap=%v err=%v", gap, err)
	}
	if future == "999999999999:999999999999:" {
		t.Fatal("cursor not adopted after regression")
	}
}
