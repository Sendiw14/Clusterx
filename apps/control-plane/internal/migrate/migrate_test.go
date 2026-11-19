package migrate_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/migrate"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/pgtest"
	"github.com/Sendiw14/Clusterx/apps/control-plane/migrations"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestLoadEmbedded(t *testing.T) {
	migs, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 3 || migs[0].Name != "0001_init.sql" || migs[1].Name != "0002_m4_integrity_roles.sql" ||
		migs[2].Name != "0003_m5_metrics_indexes.sql" || len(migs[0].Checksum) != 64 {
		t.Fatalf("unexpected migrations: %+v", migs)
	}
}

func TestLoadRejectsBadSets(t *testing.T) {
	f := func(names ...string) fstest.MapFS {
		m := fstest.MapFS{}
		for _, n := range names {
			m[n] = &fstest.MapFile{Data: []byte("SELECT 1;")}
		}
		return m
	}
	for name, fsys := range map[string]fstest.MapFS{
		"empty":     f("README.md"),
		"gap":       f("0001_a.sql", "0003_c.sql"),
		"not one":   f("0002_b.sql"),
		"bad name":  f("1_a.sql"),
		"uppercase": f("0001_A.sql"),
		"duplicate": f("0001_a.sql", "0001_b.sql"),
	} {
		if _, err := migrate.Load(fsys); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func connect(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func TestApplyFreshThenIdempotent(t *testing.T) {
	dsn := pgtest.NewDB(t)
	ctx := context.Background()
	migs, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	c := connect(t, dsn)

	if err := migrate.Verify(ctx, c, migs); !errors.Is(err, migrate.ErrNotInitialized) {
		t.Fatalf("fresh Verify: want ErrNotInitialized, got %v", err)
	}
	n, err := migrate.Apply(ctx, c, migs, discard)
	if err != nil || n != len(migs) {
		t.Fatalf("first Apply = %d, %v", n, err)
	}
	if err := migrate.Verify(ctx, c, migs); err != nil {
		t.Fatalf("Verify after apply: %v", err)
	}
	// The DDL from 0001 is really there (spot-check core tables and the uuidv7 default).
	var tables int
	if err := c.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname='public'
		AND tablename IN ('jobs','job_attempts','job_events','workers','worker_credentials','users','sessions','api_tokens')`).Scan(&tables); err != nil || tables != 8 {
		t.Fatalf("core tables = %d, %v", tables, err)
	}

	n, err = migrate.Apply(ctx, connect(t, dsn), migs, discard)
	if err != nil || n != 0 {
		t.Fatalf("second Apply = %d, %v; want 0, nil", n, err)
	}
	var rows int
	if err := c.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&rows); err != nil || rows != len(migs) {
		t.Fatalf("schema_migrations rows = %d, %v", rows, err)
	}
}

func TestApplyConcurrentRunnersApplyOnce(t *testing.T) {
	dsn := pgtest.NewDB(t)
	migs, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	const runners = 6
	var wg sync.WaitGroup
	results := make([]int, runners)
	errs := make([]error, runners)
	for i := range runners {
		c := connect(t, dsn)
		wg.Go(func() { results[i], errs[i] = migrate.Apply(context.Background(), c, migs, discard) })
	}
	wg.Wait()
	total := 0
	for i := range runners {
		if errs[i] != nil {
			t.Fatalf("runner %d: %v", i, errs[i])
		}
		total += results[i]
	}
	if total != len(migs) {
		t.Fatalf("migration applied %d times, want exactly 1", total)
	}
}

func TestDriftIsRefused(t *testing.T) {
	dsn := pgtest.NewDB(t)
	ctx := context.Background()
	v1 := fstest.MapFS{"0001_a.sql": {Data: []byte("CREATE TABLE a (id int);")}}
	migs, _ := migrate.Load(v1)
	c := connect(t, dsn)
	if _, err := migrate.Apply(ctx, c, migs, discard); err != nil {
		t.Fatal(err)
	}

	// Edited applied file.
	edited, _ := migrate.Load(fstest.MapFS{"0001_a.sql": {Data: []byte("CREATE TABLE a (id bigint);")}})
	if _, err := migrate.Apply(ctx, c, edited, discard); !errors.Is(err, migrate.ErrDrift) {
		t.Fatalf("edited migration: want ErrDrift, got %v", err)
	}
	if err := migrate.Verify(ctx, c, edited); !errors.Is(err, migrate.ErrDrift) {
		t.Fatalf("Verify edited: want ErrDrift, got %v", err)
	}

	// Binary newer than DB: pending, then applied.
	v2 := fstest.MapFS{"0001_a.sql": v1["0001_a.sql"], "0002_b.sql": {Data: []byte("CREATE TABLE b (id int);")}}
	migs2, _ := migrate.Load(v2)
	if err := migrate.Verify(ctx, c, migs2); !errors.Is(err, migrate.ErrPending) {
		t.Fatalf("want ErrPending, got %v", err)
	}
	if n, err := migrate.Apply(ctx, c, migs2, discard); err != nil || n != 1 {
		t.Fatalf("apply v2 = %d, %v", n, err)
	}

	// DB newer than binary: refused.
	if err := migrate.Verify(ctx, c, migs); !errors.Is(err, migrate.ErrDrift) {
		t.Fatalf("old binary: want ErrDrift, got %v", err)
	}
}

func TestFailedMigrationLeavesNoTrace(t *testing.T) {
	dsn := pgtest.NewDB(t)
	ctx := context.Background()
	migs, _ := migrate.Load(fstest.MapFS{"0001_bad.sql": {Data: []byte("CREATE TABLE t (id int); SELECT 1/0;")}})
	c := connect(t, dsn)
	if _, err := migrate.Apply(ctx, c, migs, discard); err == nil || !strings.Contains(err.Error(), "0001_bad.sql") {
		t.Fatalf("want failure naming the file, got %v", err)
	}
	var exists bool
	if err := c.QueryRow(ctx, `SELECT to_regclass('public.t') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatalf("partial migration committed: exists=%v err=%v", exists, err)
	}
	if err := migrate.Verify(ctx, c, migs); !errors.Is(err, migrate.ErrPending) {
		t.Fatalf("want ErrPending after failure, got %v", err)
	}
}
