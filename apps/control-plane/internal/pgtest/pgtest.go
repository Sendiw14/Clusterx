// Package pgtest creates throwaway PostgreSQL databases for integration tests.
// Tests are skipped (not passed) when CLUSTERX_PG_ADMIN_DSN is unset.
package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// EnvAdminDSN names a superuser DSN of a disposable PostgreSQL 18 cluster.
const EnvAdminDSN = "CLUSTERX_PG_ADMIN_DSN"

// NewDB creates an empty database and returns a DSN for it; the database is dropped when the
// test ends.
func NewDB(t testing.TB) string {
	t.Helper()
	admin := lookupAdmin(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ac, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("pgtest: connect admin: %v", err)
	}
	defer ac.Close(ctx)

	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "clusterx_it_" + hex.EncodeToString(b)
	if _, err := ac.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("pgtest: create database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c, err := pgx.Connect(ctx, admin)
		if err != nil {
			t.Errorf("pgtest: cleanup connect: %v", err)
			return
		}
		defer c.Close(ctx)
		if _, err := c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("pgtest: drop %s: %v", name, err)
		}
	})

	u, err := url.Parse(admin)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatalf("pgtest: %s must be a postgres:// URL", EnvAdminDSN)
	}
	u.Path = "/" + name
	return u.String()
}

func lookupAdmin(t testing.TB) string {
	t.Helper()
	admin := os.Getenv(EnvAdminDSN)
	if admin == "" {
		t.Skipf("%s not set; integration test needs real PostgreSQL 18", EnvAdminDSN)
	}
	return admin
}
