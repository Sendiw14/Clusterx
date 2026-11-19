package store

import (
	"context"
	"testing"
	"time"
)

func reRegister(t *testing.T, s *Store, w WorkerRef) WorkerRef {
	t.Helper()
	inc := NewUUID()
	r, err := s.Register(context.Background(), w.CredentialID, RegisterParams{WorkerID: w.WorkerID, IncarnationID: inc, Name: "w",
		ProtocolVersion: 1, Capacity: 2, Capabilities: []Capability{{"sleep_ms", 1}}})
	if err != nil {
		t.Fatal(err)
	}
	return WorkerRef{WorkerID: r.WorkerID, IncarnationID: inc, CredentialID: w.CredentialID}
}

// RV5-02: an operator drain is sticky to the worker id; re-registering does not undo it.
func TestRV5_OperatorDrainSurvivesReRegister(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	ctx := context.Background()
	w := newWorker(t, s, 2, "sleep_ms")
	if err := s.OperatorDrain(ctx, w.WorkerID, time.Hour, "user:op", AuditMeta{}); err != nil {
		t.Fatal(err)
	}
	w2 := reRegister(t, s, w)
	if w2.WorkerID != w.WorkerID {
		t.Fatalf("draining worker got a new id")
	}
	if st := q[string](t, s, `SELECT status FROM workers WHERE id = $1`, w.WorkerID); st != "draining" {
		t.Fatalf("status after re-register = %s, want draining", st)
	}
	admit(t, s, "sleep_ms")
	_, err := s.Claim(ctx, w2, 1, nil)
	wantDenied(t, err, "worker_draining")

	// After deregistration the id is never resurrected: the process rejoins as a new active worker.
	if _, err := s.Deregister(ctx, w2); err != nil {
		t.Fatal(err)
	}
	w3 := reRegister(t, s, w2)
	if w3.WorkerID == w.WorkerID {
		t.Fatal("deregistered worker row resurrected")
	}
	if st := q[string](t, s, `SELECT status FROM workers WHERE id = $1`, w.WorkerID); st != "deregistered" {
		t.Fatalf("old row status = %s", st)
	}
	if got, err := s.Claim(ctx, w3, 1, nil); err != nil || len(got) != 1 {
		t.Fatalf("new worker claim: %d %v", len(got), err)
	}
}

// RV5-02: two processes sharing one worker id: the newest incarnation supersedes, the older is fenced,
// and each live takeover is a security audit row.
func TestRV5_ConcurrentIncarnationsNewestWins(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	ctx := context.Background()
	a := newWorker(t, s, 2, "sleep_ms")
	admit(t, s, "sleep_ms")
	att := claimOne(t, s, a)
	b := reRegister(t, s, a)
	_, err := s.Claim(ctx, a, 1, nil)
	wantDenied(t, err, "incarnation_mismatch")
	_, err = s.RenewLease(ctx, att)
	wantDenied(t, err, "incarnation_mismatch")
	a2 := reRegister(t, s, a) // the old process re-registers: it is now the newest, b is fenced
	_, err = s.Claim(ctx, b, 1, nil)
	wantDenied(t, err, "incarnation_mismatch")
	if _, err := s.Claim(ctx, a2, 1, nil); err != nil {
		t.Fatal(err)
	}
	if n := q[int64](t, s, `SELECT count(*) FROM audit_events WHERE action = 'worker.incarnation_takeover' AND category = 'security'
		AND target_id = $1`, a.WorkerID); n != 2 {
		t.Fatalf("takeover audit rows = %d, want 2", n)
	}
}

// RV5-11: worker denial audit rows are rate-limited per credential; the denial is still answered.
func TestRV5_DenialAuditRateLimited(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	ctx := context.Background()
	w := newWorker(t, s, 2, "sleep_ms")
	admit(t, s, "sleep_ms")
	att := claimOne(t, s, w)
	budget := 2
	s.DenialAudit = func(cred string) bool {
		if cred != w.CredentialID {
			t.Errorf("keyed by %q", cred)
		}
		budget--
		return budget >= 0
	}
	stale := att
	stale.Generation++
	for range 5 {
		_, err := s.RenewLease(ctx, stale)
		wantDenied(t, err, "stale_attempt")
	}
	if n := q[int64](t, s, `SELECT count(*) FROM audit_events WHERE actor = 'worker:' || $1::text AND outcome = 'rejected'`, w.WorkerID); n != 2 {
		t.Fatalf("denial audit rows = %d, want 2", n)
	}
}

// RV5R-01 (reviewer proof, round 2): Register with a deregistered worker_id goes through the same
// one-live-worker-per-credential selection as an empty worker_id.
func TestRV5R_DeregisteredIdBypassesSingleLiveBinding(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	ctx := context.Background()
	w := newWorker(t, s, 2, "sleep_ms")
	if _, err := s.Deregister(ctx, w); err != nil {
		t.Fatal(err)
	}
	reg := func(workerID string) (Registered, error) {
		return s.Register(ctx, w.CredentialID, RegisterParams{WorkerID: workerID, IncarnationID: NewUUID(), Name: "w",
			ProtocolVersion: 1, Capacity: 2, Capabilities: []Capability{{"sleep_ms", 1}}})
	}
	live, err := reg("") // normal rejoin: one live row
	if err != nil {
		t.Fatal(err)
	}
	_, err = reg("")
	wantDenied(t, err, "incarnation_mismatch")
	for range 10 {
		r, err := reg(w.WorkerID)
		wantDenied(t, err, "incarnation_mismatch")
		if r.WorkerID != "" && r.WorkerID != live.WorkerID {
			t.Fatalf("extra worker row %s", r.WorkerID)
		}
	}
	if n := q[int64](t, s, `SELECT count(*) FROM workers WHERE credential_id = $1 AND status <> 'deregistered'`, w.CredentialID); n != 1 {
		t.Fatalf("credential bound to %d non-deregistered workers, want 1", n)
	}
	// Once the live row is stale, the deregistered id takes it over (no new row).
	exec(t, s, `UPDATE workers SET last_heartbeat_at = clock_timestamp() - interval '1 hour' WHERE id = $1`, live.WorkerID)
	if r, err := reg(w.WorkerID); err != nil || r.WorkerID != live.WorkerID {
		t.Fatalf("stale takeover via deregistered id: %v %v", r.WorkerID, err)
	}
}

// RV5R-02 (reviewer proof, round 2): the live-takeover audit row is inside the per-credential budget.
func TestRV5R_TakeoverAuditBudgeted(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	ctx := context.Background()
	w := newWorker(t, s, 2, "sleep_ms")
	s.DenialAudit = func(string) bool { return false } // budget fully spent
	for range 50 {
		if _, err := s.Register(ctx, w.CredentialID, RegisterParams{WorkerID: w.WorkerID, IncarnationID: NewUUID(), Name: "w",
			ProtocolVersion: 1, Capacity: 2, Capabilities: []Capability{{"sleep_ms", 1}}}); err != nil {
			t.Fatal(err)
		}
	}
	if n := q[int64](t, s, `SELECT count(*) FROM audit_events WHERE action = 'worker.incarnation_takeover'`); n != 0 {
		t.Fatalf("takeover audit rows with the budget spent: %d", n)
	}
}
