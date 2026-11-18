package store

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/migrate"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/pgtest"
	"github.com/Sendiw14/Clusterx/apps/control-plane/migrations"
)

type allKnown struct{}

func (allKnown) Known(name string, v int) bool { return name != "unknown.handler" }

// migratedDB returns a DSN of a fresh database with every migration applied (superuser).
func migratedDB(t *testing.T) string {
	t.Helper()
	dsn := pgtest.NewDB(t)
	ctx := context.Background()
	migs, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	if _, err := migrate.Apply(ctx, c, migs, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	return dsn
}

func newTestStore(t *testing.T, lease time.Duration) *Store {
	t.Helper()
	dsn := migratedDB(t)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	apply(cfg.ConnConfig)
	cfg.MaxConns = 60
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tm := DefaultTiming()
	tm.LeaseTTL = lease
	tm.Backoff = Backoff{Base: time.Millisecond, Cap: 4 * time.Millisecond, Rand: func() float64 { return 0 }}
	return New(pool, tm, allKnown{})
}

func q[T any](t *testing.T, s *Store, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := s.Pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func exec(t *testing.T, s *Store, sql string, args ...any) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// newWorker issues a credential and registers one incarnation with the given capabilities.
func newWorker(t *testing.T, s *Store, capacity int, caps ...string) WorkerRef {
	t.Helper()
	ctx := context.Background()
	cred, err := s.CreateWorkerCredential(ctx, "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	credID, err := s.AuthenticateWorker(ctx, cred.Secret)
	if err != nil {
		t.Fatal(err)
	}
	var cs []Capability
	for _, c := range caps {
		cs = append(cs, Capability{Name: c, Version: 1})
	}
	inc := NewUUID()
	reg, err := s.Register(ctx, credID, RegisterParams{IncarnationID: inc, Name: "w", ProtocolVersion: 1, Capacity: capacity, Capabilities: cs})
	if err != nil {
		t.Fatal(err)
	}
	return WorkerRef{WorkerID: reg.WorkerID, IncarnationID: inc, CredentialID: credID}
}

type admitOpt func(*AdmitParams)

func admit(t *testing.T, s *Store, handler string, opts ...admitOpt) string {
	t.Helper()
	p := AdmitParams{Principal: "token:test", Namespace: "default", HandlerName: handler, HandlerVersion: 1,
		MaxAttempts: 3, TimeoutSeconds: 600, Payload: json.RawMessage(`{}`)}
	for _, o := range opts {
		o(&p)
	}
	j, _, err := s.Admit(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return j.ID
}

func claimOne(t *testing.T, s *Store, w WorkerRef) AttemptRef {
	t.Helper()
	got, err := s.Claim(context.Background(), w, 1, nil)
	if err != nil || len(got) != 1 {
		t.Fatalf("claim = %v, %v; want 1", got, err)
	}
	return AttemptRef{WorkerRef: w, JobID: got[0].JobID, AttemptID: got[0].AttemptID, Generation: got[0].LeaseGeneration}
}

func wantDenied(t *testing.T, err error, reason string) {
	t.Helper()
	if !IsDenial(err, reason) {
		t.Fatalf("err = %v; want denied %s", err, reason)
	}
}

// expireLease forces the lease (and optionally the deadline) into the past as a test fixture.
func expireLease(t *testing.T, s *Store, jobID string, deadline bool) {
	t.Helper()
	if deadline {
		exec(t, s, `UPDATE jobs SET lease_expires_at = now() - interval '1 second', hard_deadline_at = now() - interval '1 second' WHERE id = $1`, jobID)
		return
	}
	exec(t, s, `UPDATE jobs SET lease_expires_at = now() - interval '1 second' WHERE id = $1`, jobID)
}
