package sdk

import (
	"context"
	"log/slog"
)

// Job describes the attempt a handler is executing.
type Job struct {
	// ID is stable across attempts of the same job; use it as the idempotency key for side effects.
	ID string
	// AttemptID identifies this execution attempt.
	AttemptID string
	// Attempt is the 1-based attempt number; MaxAttempts includes the first run.
	Attempt     int
	MaxAttempts int
	// Handler is the name and version the job was submitted for.
	Handler HandlerKey
	// Log is the attempt logger. Its output (text format, attributes with secret-like keys redacted)
	// is also captured, bounded to the last 16 KiB, and reported as the attempt's log_tail; it also goes
	// to the worker log. Never log secrets: the coordinator keeps log_tail for 7 days. Nil outside a Worker.
	Log *slog.Logger
}

// Handler executes one job attempt with a typed payload P and returns a typed result R.
//
// P is decoded from the job's JSON payload with unknown fields rejected; R is encoded as JSON
// and must be at most [MaxResultBytes]. Return a [RetryableError] or [PermanentError] to control
// retry behaviour.
type Handler[P, R any] interface {
	Handle(ctx context.Context, job Job, payload P) (R, error)
}

// HandlerFunc adapts an ordinary function to the [Handler] interface.
type HandlerFunc[P, R any] func(ctx context.Context, job Job, payload P) (R, error)

// Handle calls f(ctx, job, payload).
func (f HandlerFunc[P, R]) Handle(ctx context.Context, job Job, payload P) (R, error) {
	return f(ctx, job, payload)
}
