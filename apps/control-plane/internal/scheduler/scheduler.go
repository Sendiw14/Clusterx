// Package scheduler runs the coordinator's background loops: due promotion (E2), lease/deadline
// recovery (E8) and expired idempotency cleanup. Every comparison uses the DB clock; the Go ticker
// only paces the loop, with ±20 % jitter so several coordinators (not supported in 1.0) or restarts
// do not align.
package scheduler

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

// Batch bounds the work of one tick.
const Batch = 500

// Tick runs one pass and returns the counts.
func Tick(ctx context.Context, st *store.Store) (promoted int64, recovered int, err error) {
	if recovered, err = st.RecoverExpired(ctx, Batch); err != nil {
		return
	}
	promoted, err = st.PromoteDue(ctx, Batch)
	return
}

// Run loops until ctx ends. Errors are logged and the loop continues (DB outages are expected).
func Run(ctx context.Context, st *store.Store, tick time.Duration, log *slog.Logger) {
	lastCleanup := time.Now()
	for {
		d := time.Duration(float64(tick) * (0.8 + 0.4*rand.Float64()))
		select {
		case <-ctx.Done():
			return
		case <-time.After(d):
		}
		p, r, err := Tick(ctx, st)
		if err != nil && ctx.Err() == nil {
			log.Warn("scheduler tick failed", "err", err)
			continue
		}
		if p > 0 || r > 0 {
			log.Info("scheduler tick", "promoted", p, "recovered", r)
		}
		if time.Since(lastCleanup) > time.Minute {
			lastCleanup = time.Now()
			if n, err := st.CleanupIdempotency(ctx, 1000); err != nil {
				log.Warn("idempotency cleanup failed", "err", err)
			} else if n > 0 {
				log.Info("idempotency cleanup", "deleted", n)
			}
		}
	}
}
