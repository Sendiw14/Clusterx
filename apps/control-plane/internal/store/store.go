package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"time"

	"github.com/jackc/puddle/v2"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Denial is a refused worker or API action. Reason is a state-machine.md code (worker side) or an
// OpenAPI Error.code (REST side). Denials never change job, attempt or event rows.
type Denial struct {
	Reason string
	Detail string
}

func (d *Denial) Error() string {
	if d.Detail != "" {
		return "denied: " + d.Reason + ": " + d.Detail
	}
	return "denied: " + d.Reason
}

func deny(reason string) error { return &Denial{Reason: reason} }

// IsDenial reports whether err is a *Denial with the given reason ("" matches any).
func IsDenial(err error, reason string) bool {
	d, ok := errors.AsType[*Denial](err)
	return ok && (reason == "" || d.Reason == reason)
}

// Timing holds the durations the store adds to the DB clock (ADR-0003). Go never compares its own
// wall clock with stored timestamps.
type Timing struct {
	LeaseTTL          time.Duration
	CancelGrace       time.Duration
	DrainGraceMax     time.Duration
	HeartbeatInterval time.Duration
	LivenessTimeout   time.Duration
	IdempotencyTTL    time.Duration
	Backoff           Backoff
}

// DefaultTiming returns the ADR-0003 starting values.
func DefaultTiming() Timing {
	return Timing{
		LeaseTTL:          30 * time.Second,
		CancelGrace:       15 * time.Second,
		DrainGraceMax:     time.Hour,
		HeartbeatInterval: 10 * time.Second,
		LivenessTimeout:   30 * time.Second,
		IdempotencyTTL:    24 * time.Hour,
		Backoff:           Backoff{Base: 2 * time.Second, Cap: 5 * time.Minute},
	}
}

// Backoff is full-jitter exponential backoff: delay = min(Cap, Base*2^(n-1)) * rand[0.5, 1.0).
// Rand is injectable for reproducible tests (AC-05); nil uses math/rand/v2.
type Backoff struct {
	Base, Cap time.Duration
	Rand      func() float64
}

// Delay returns the delay before attempt n+1 after n attempts were used (n >= 1).
func (b Backoff) Delay(n int) time.Duration {
	if n < 1 {
		n = 1
	}
	d := b.Base
	for i := 1; i < n && d < b.Cap; i++ {
		d *= 2
	}
	if d > b.Cap {
		d = b.Cap
	}
	r := rand.Float64
	if b.Rand != nil {
		r = b.Rand
	}
	f := 0.5 + 0.5*r()
	if f >= 1 {
		f = 0.999999
	}
	return time.Duration(float64(d) * f)
}

// Store implements every job, attempt, event and worker mutation of state-machine.md as one short
// READ COMMITTED transaction with CAS predicates and affected-row checks.
//
// Global lock order (NF-01, ADR-0003): worker_credentials -> workers -> jobs -> job_attempts, ascending
// id within a class. A transaction that share-locks a worker or credential row never writes that row
// later in the same transaction (usage/heartbeat bumps run in their own transaction).
type Store struct {
	Pool    *pgxpool.Pool
	Timing  Timing
	Catalog Catalog
	// Failpoint, when set, is called at named points inside transactions; a non-nil error aborts the tx.
	Failpoint func(name string) error
}

// New returns a Store with the given timing.
func New(pool *pgxpool.Pool, t Timing, c Catalog) *Store {
	return &Store{Pool: pool, Timing: t, Catalog: c}
}

func (s *Store) fail(name string) error {
	if s.Failpoint == nil {
		return nil
	}
	return s.Failpoint(name)
}

// maxTxAttempts bounds the server-side retry of deadlock (40P01) and serialization (40001) aborts.
const maxTxAttempts = 3

// ErrUnavailable wraps database failures that the caller should report as 503 / UNAVAILABLE.
var ErrUnavailable = errors.New("store: database unavailable")

// inTx runs fn in a READ COMMITTED transaction, retrying the whole transaction on 40P01/40001.
// fn must be idempotent with respect to Go-side state (it is re-run from scratch).
func (s *Store) inTx(ctx context.Context, worker bool, fn func(pgx.Tx) error) error {
	var err error
	for i := range maxTxAttempts {
		err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
			if worker {
				// Worker mutations wait at most 1 s for a lock (ADR-0003 revocation bound).
				if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '1s'`); err != nil {
					return err
				}
			}
			return fn(tx)
		})
		if !retryable(err) {
			break
		}
		if i < maxTxAttempts-1 {
			time.Sleep(time.Duration(5*(i+1)) * time.Millisecond)
		}
	}
	return classifyDBError(err)
}

func retryable(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && (pe.Code == "40P01" || pe.Code == "40001")
}

// classifyDBError maps connection loss, lock timeouts and exhausted retries to ErrUnavailable.
func classifyDBError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*Denial](err); ok {
		return err
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch {
		case pe.Code == "40P01", pe.Code == "40001", pe.Code == "55P03", pe.Code == "57014",
			pe.Code[:2] == "08", pe.Code == "57P01", pe.Code == "57P02", pe.Code == "57P03", pe.Code == "53300":
			return fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if connLost(err) {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return err
}

// connLost reports transport/pool failures that carry no server SQLSTATE (RV5-01): the server or
// network went away, so the outcome is "not acknowledged" (503 / UNAVAILABLE), never 500.
func connLost(err error) bool {
	var ce *pgconn.ConnectError
	var ne net.Error
	return errors.As(err, &ce) || pgconn.SafeToRetry(err) || pgconn.Timeout(err) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &ne) ||
		errors.Is(err, pgconn.ErrConnClosed) || errors.Is(err, puddle.ErrClosedPool)
}

// IsUnavailable reports whether err (from any store path, classified or raw) means the database
// is unavailable. Transport layers use it so an unclassified read path cannot leak a 500.
func IsUnavailable(err error) bool { return errors.Is(classifyDBError(err), ErrUnavailable) }

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// Now returns the database clock.
func (s *Store) Now(ctx context.Context) (time.Time, error) {
	var t time.Time
	err := s.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&t)
	return t, classifyDBError(err)
}

func jsonOrNull(m map[string]any) any {
	if len(m) == 0 {
		return nil // SQL NULL, not JSON null
	}
	return m
}
