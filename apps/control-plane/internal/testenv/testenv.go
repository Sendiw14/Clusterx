// Package testenv builds a migrated PostgreSQL database, a store with the default catalogue and,
// optionally, a gRPC worker server, for integration tests of packages above the store.
package testenv

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/catalog"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/migrate"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/pgtest"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/workerrpc"
	"github.com/Sendiw14/Clusterx/apps/control-plane/migrations"
)

// Discard is a logger that drops everything.
var Discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// Env is a migrated database with a store.
type Env struct {
	DSN   string
	Store *store.Store
	Migs  []migrate.Migration
}

// New creates the database, applies migrations and opens a pool of maxConns.
func New(t testing.TB, timing store.Timing, maxConns int32) *Env {
	t.Helper()
	dsn := pgtest.NewDB(t)
	ctx := context.Background()
	migs, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	c, err := store.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrate.Apply(ctx, c, migs, Discard); err != nil {
		t.Fatal(err)
	}
	_ = c.Close(ctx)
	pool := OpenPool(t, dsn, maxConns)
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	return &Env{DSN: dsn, Store: store.New(pool, timing, cat), Migs: migs}
}

// OpenPool opens a runtime pool closed at test end.
func OpenPool(t testing.TB, dsn string, maxConns int32) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = maxConns
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// FastTiming has short leases and tiny backoff for tests.
func FastTiming(lease time.Duration) store.Timing {
	t := store.DefaultTiming()
	t.LeaseTTL = lease
	t.HeartbeatInterval = 2 * time.Second
	t.LivenessTimeout = 6 * time.Second
	t.Backoff = store.Backoff{Base: 10 * time.Millisecond, Cap: 50 * time.Millisecond}
	return t
}

// ServeGRPC starts the worker service on a loopback port; returns its address.
func (e *Env) ServeGRPC(t testing.TB, tweak ...func(*workerrpc.Options)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	o := workerrpc.Options{ClaimPoll: 20 * time.Millisecond}
	for _, f := range tweak {
		f(&o)
	}
	s := workerrpc.NewServer(e.Store, Discard, o)
	go func() { _ = s.Serve(ln) }()
	t.Cleanup(s.Stop)
	return ln.Addr().String()
}

type bearer string

func (b bearer) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + string(b)}, nil
}
func (bearer) RequireTransportSecurity() bool { return false }

// Dial connects to addr with the given worker credential (plaintext loopback).
func Dial(t testing.TB, addr, credential string) *grpc.ClientConn {
	t.Helper()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if credential != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(bearer(credential)))
	}
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}
