package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/puddle/v2"
)

// RV5-01: transport/pool failures without a SQLSTATE are "unavailable"; app errors are not.
func TestClassifyDBErrorConnectionLoss(t *testing.T) {
	unavailable := []error{
		fmt.Errorf("failed to deallocate cached statement(s): %w", io.ErrUnexpectedEOF),
		io.EOF,
		&net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")},
		fmt.Errorf("x: %w", pgconn.ErrConnClosed),
		puddle.ErrClosedPool,
		&pgconn.PgError{Code: "57P01"},
		&pgconn.PgError{Code: "08006"},
	}
	for _, e := range unavailable {
		if !errors.Is(classifyDBError(e), ErrUnavailable) || !IsUnavailable(e) {
			t.Errorf("%T %v: not classified unavailable", e, e)
		}
	}
	other := []error{pgx.ErrNoRows, &pgconn.PgError{Code: "23505"}, context.Canceled, &Denial{Reason: "x"}, errors.New("app")}
	for _, e := range other {
		if IsUnavailable(e) {
			t.Errorf("%v: wrongly classified unavailable", e)
		}
	}
}
