package store

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// NF-01: concurrent reports, renewals, heartbeats, claims, drain, deregister, revoke and recovery
// on one worker never surface a deadlock (40P01) or an internal error.
func TestNF01_NoDeadlockAcrossWorkerPaths(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	for round := range 3 {
		w := newWorker(t, s, 64, "h")
		for range 48 {
			admit(t, s, "h")
		}
		got, err := s.Claim(ctx, w, 48, nil)
		if err != nil || len(got) != 48 {
			t.Fatalf("claim: %d %v", len(got), err)
		}
		var wg sync.WaitGroup
		var bad atomic.Value
		check := func(err error) {
			if err != nil && !IsDenial(err, "") {
				bad.Store(err)
			}
		}
		for i, a := range got {
			ref := AttemptRef{WorkerRef: w, JobID: a.JobID, AttemptID: a.AttemptID, Generation: a.LeaseGeneration}
			wg.Go(func() {
				if i%3 == 0 {
					_, err := s.RenewLease(ctx, ref)
					check(err)
				}
				if i%2 == 0 {
					_, err := s.ReportSuccess(ctx, ref, json.RawMessage(`{}`), "")
					check(err)
				} else {
					_, err := s.ReportFailure(ctx, ref, FailureParams{Code: "x"})
					check(err)
				}
			})
		}
		for range 10 {
			wg.Go(func() { _, err := s.Heartbeat(ctx, w); check(err) })
			wg.Go(func() { _, err := s.Claim(ctx, w, 2, nil); check(err) })
			wg.Go(func() { _, err := s.RecoverExpired(ctx, 5); check(err) })
		}
		wg.Go(func() {
			switch round {
			case 0:
				_, err := s.Drain(ctx, w, time.Minute)
				check(err)
			case 1:
				_, err := s.Deregister(ctx, w)
				check(err)
			case 2:
				check(s.RevokeWorkerCredential(ctx, w.CredentialID))
			}
		})
		wg.Wait()
		if e := bad.Load(); e != nil {
			t.Fatalf("round %d: %v", round, e)
		}
	}
}

// NF-01: 40P01 / 40001 are retried server-side a bounded number of times, then reported unavailable.
func TestNF01_DeadlockRetryBounded(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	calls := 0
	err := s.inTx(ctx, false, func(pgx.Tx) error {
		calls++
		if calls < 2 {
			return &pgconn.PgError{Code: "40P01"}
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("retry once: calls=%d err=%v", calls, err)
	}
	calls = 0
	err = s.inTx(ctx, false, func(pgx.Tx) error { calls++; return &pgconn.PgError{Code: "40001"} })
	if !errors.Is(err, ErrUnavailable) || calls != maxTxAttempts {
		t.Fatalf("bounded: calls=%d err=%v", calls, err)
	}
	calls = 0
	err = s.inTx(ctx, false, func(pgx.Tx) error { calls++; return &pgconn.PgError{Code: "23505"} })
	if errors.Is(err, ErrUnavailable) || calls != 1 {
		t.Fatalf("non-retryable retried: calls=%d err=%v", calls, err)
	}
}

// NF-01 H1/H2 regression: a real deadlock between two transactions that take the lock order
// in opposite directions is retried by inTx and both complete.
func TestNF01_RealDeadlockIsRetried(t *testing.T) {
	s := newTestStore(t, 30*time.Second)
	a := admit(t, s, "h")
	b := admit(t, s, "h")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	start := make(chan struct{})
	for i, order := range [][2]string{{a, b}, {b, a}} {
		wg.Go(func() {
			<-start
			first := true
			errs[i] = s.inTx(ctx, false, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, `SELECT 1 FROM jobs WHERE id = $1 FOR UPDATE`, order[0]); err != nil {
					return err
				}
				if first {
					first = false
					time.Sleep(100 * time.Millisecond)
				}
				_, err := tx.Exec(ctx, `SELECT 1 FROM jobs WHERE id = $1 FOR UPDATE`, order[1])
				return err
			})
		})
	}
	close(start)
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("deadlock not retried: %v / %v", errs[0], errs[1])
	}
}
