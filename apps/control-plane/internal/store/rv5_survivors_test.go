package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// RV5-06 / N12: a worker mutation waits at most lock_timeout (1 s, ADR-0003 revocation bound) for a
// row lock, then answers unavailable instead of blocking until statement_timeout.
func TestRV5_WorkerLockTimeoutBound(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	ctx := context.Background()
	w := newWorker(t, s, 2, "sleep_ms")
	id := admit(t, s, "sleep_ms")
	att := claimOne(t, s, w)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT 1 FROM jobs WHERE id = $1 FOR UPDATE`, id); err != nil {
		t.Fatal(err)
	}
	cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	start := time.Now()
	_, err = s.RenewLease(cctx, att)
	if el := time.Since(start); !errors.Is(err, ErrUnavailable) || el > 2500*time.Millisecond {
		t.Fatalf("blocked renew: err=%v after %s (want unavailable within ~1 s)", err, el)
	}
}

// RV5-06 / N18: cancelling a running job caps its lease at cancel_requested_at + cancel_grace (E9).
func TestRV5_RunningCancelCapsLease(t *testing.T) {
	s := newTestStore(t, 5*time.Minute)
	ctx := context.Background()
	w := newWorker(t, s, 2, "sleep_ms")
	id := admit(t, s, "sleep_ms")
	claimOne(t, s, w)
	if _, err := s.RequestCancel(ctx, id, "user:op"); err != nil {
		t.Fatal(err)
	}
	if ok := q[bool](t, s, `SELECT lease_expires_at <= cancel_requested_at + $2::interval FROM jobs WHERE id = $1`,
		id, s.Timing.CancelGrace); !ok {
		t.Fatal("running cancel did not cap the lease at cancel_requested_at + cancel_grace")
	}
}

// RV5-06 / N22: a credential revoked between authentication and the Register transaction is refused.
func TestRV5_RegisterRechecksRevokedCredential(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	ctx := context.Background()
	cred, err := s.CreateWorkerCredential(ctx, "t", 0)
	if err != nil {
		t.Fatal(err)
	}
	credID, err := s.AuthenticateWorker(ctx, cred.Secret)
	if err != nil {
		t.Fatal(err)
	}
	exec(t, s, `UPDATE worker_credentials SET revoked_at = clock_timestamp() WHERE id = $1`, credID)
	_, err = s.Register(ctx, credID, RegisterParams{IncarnationID: NewUUID(), Name: "w", ProtocolVersion: 1, Capacity: 1,
		Capabilities: []Capability{{"sleep_ms", 1}}})
	wantDenied(t, err, "credential_revoked")
}
