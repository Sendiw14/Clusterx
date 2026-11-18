// Package store owns PostgreSQL access for the control plane (ADR-0001).
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session settings from ADR-0001 §6 applied to every runtime connection.
var runtimeParams = map[string]string{
	"application_name":                    "clusterx-control-plane",
	"statement_timeout":                   "5000",  // ms
	"idle_in_transaction_session_timeout": "10000", // ms
	"synchronous_commit":                  "on",
	"timezone":                            "UTC",
}

func apply(cc *pgx.ConnConfig) {
	for k, v := range runtimeParams {
		cc.RuntimeParams[k] = v
	}
}

// OpenPool creates the runtime connection pool. It does not wait for the database: an
// unreachable database at start-up makes readiness fail instead of crashing the process.
func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: invalid database URL") // the error may echo the password
	}
	apply(cfg.ConnConfig)
	return pgxpool.NewWithConfig(ctx, cfg)
}

// Connect opens a single connection with the runtime settings, for migrations.
func Connect(ctx context.Context, dsn string) (*pgx.Conn, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: invalid database URL")
	}
	apply(cfg)
	cfg.RuntimeParams["application_name"] = "clusterx-migrate"
	return pgx.ConnectConfig(ctx, cfg)
}
