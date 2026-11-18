// Package migrate applies the embedded forward-only SQL migrations.
//
// Applied versions are recorded in schema_migrations with a SHA-256 checksum of the file.
// Apply serializes concurrent runners with a session-level advisory lock, runs each pending
// migration and its bookkeeping row in one transaction, and refuses to start when an applied
// migration was edited, is missing from the binary, or the database is ahead of the binary.
// There is no down migration: recovery from a bad migration is restore from backup (ADR-0004).
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"regexp"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// LockKey is the pg_advisory_lock key held while migrating ("clusterx" in ASCII).
const LockKey int64 = 0x636c757374657278

const createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version     integer PRIMARY KEY CHECK (version > 0),
    name        text NOT NULL,
    checksum    text NOT NULL CHECK (checksum ~ '^[0-9a-f]{64}$'),
    applied_at  timestamptz NOT NULL DEFAULT now()
)`

// Migration is one embedded SQL file.
type Migration struct {
	Version  int
	Name     string // file name, e.g. 0001_init.sql
	SQL      string
	Checksum string // hex SHA-256 of the file bytes
}

var fileName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

// Load reads *.sql files from fsys. Versions must be contiguous from 1.
func Load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var out []Migration
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".sql" {
			continue
		}
		m := fileName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migrate: bad migration file name %q (want NNNN_name.sql)", e.Name())
		}
		v, _ := strconv.Atoi(m[1])
		b, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		out = append(out, Migration{Version: v, Name: e.Name(), SQL: string(b), Checksum: hex.EncodeToString(sum[:])})
	}
	slices.SortFunc(out, func(a, b Migration) int { return a.Version - b.Version })
	for i, m := range out {
		if m.Version != i+1 {
			return nil, fmt.Errorf("migrate: versions must be contiguous from 1; found %s at position %d", m.Name, i+1)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("migrate: no migrations found")
	}
	return out, nil
}

// Errors returned by Verify and Apply.
var (
	ErrNotInitialized = errors.New("migrate: schema_migrations does not exist")
	ErrPending        = errors.New("migrate: migrations pending")
	ErrDrift          = errors.New("migrate: applied migrations do not match this binary")
)

// Queryer is satisfied by *pgx.Conn, pgx.Tx and *pgxpool.Pool.
type Queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type applied struct {
	Version  int
	Name     string
	Checksum string
}

func readApplied(ctx context.Context, q Queryer) ([]applied, error) {
	rows, err := q.Query(ctx, `SELECT version, name, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "42P01" { // undefined_table
			return nil, ErrNotInitialized
		}
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[applied])
}

// compare checks that applied is a prefix of migs with equal checksums.
func compare(migs []Migration, got []applied) (pending []Migration, err error) {
	if len(got) > len(migs) {
		return nil, fmt.Errorf("%w: database is at version %d, binary knows %d", ErrDrift, got[len(got)-1].Version, len(migs))
	}
	for i, a := range got {
		m := migs[i]
		if a.Version != m.Version || a.Checksum != m.Checksum {
			return nil, fmt.Errorf("%w: version %d (%s) checksum %.12s… differs from embedded %s %.12s…",
				ErrDrift, a.Version, a.Name, a.Checksum, m.Name, m.Checksum)
		}
	}
	return migs[len(got):], nil
}

// Verify returns nil only if exactly migs are applied, unchanged. It never writes.
func Verify(ctx context.Context, q Queryer, migs []Migration) error {
	got, err := readApplied(ctx, q)
	if err != nil {
		return err
	}
	pending, err := compare(migs, got)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return fmt.Errorf("%w: %d (next %s)", ErrPending, len(pending), pending[0].Name)
	}
	return nil
}

// Apply runs pending migrations on a dedicated connection and returns how many it applied.
// Concurrent callers block on the advisory lock; the loser then finds nothing to do.
// The connection's statement_timeout is lifted for its lifetime, so pass a connection that
// is closed afterwards rather than one borrowed from the runtime pool.
func Apply(ctx context.Context, conn *pgx.Conn, migs []Migration, log *slog.Logger) (int, error) {
	if _, err := conn.Exec(ctx, `SET statement_timeout = 0`); err != nil {
		return 0, err
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, LockKey); err != nil {
		return 0, fmt.Errorf("migrate: acquire advisory lock: %w", err)
	}
	defer func() {
		// Use a fresh context: the lock must be released even if ctx was canceled.
		// If the unlock fails, closing the connection releases it.
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, LockKey)
	}()

	if _, err := conn.Exec(ctx, createTable); err != nil {
		return 0, fmt.Errorf("migrate: create schema_migrations: %w", err)
	}
	got, err := readApplied(ctx, conn)
	if err != nil {
		return 0, err
	}
	pending, err := compare(migs, got)
	if err != nil {
		return 0, err
	}
	for i, m := range pending {
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.SQL); err != nil { // no args: simple protocol, multi-statement
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`,
				m.Version, m.Name, m.Checksum)
			return err
		})
		if err != nil {
			return i, fmt.Errorf("migrate: apply %s: %w", m.Name, err)
		}
		log.Info("migration applied", "schema_version", m.Version, "name", m.Name, "checksum", m.Checksum)
	}
	return len(pending), nil
}
