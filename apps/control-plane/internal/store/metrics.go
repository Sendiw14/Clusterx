package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Overview is GET /metrics/overview, read in one REPEATABLE READ snapshot so the counts agree with as_of.
type Overview struct {
	AsOf                time.Time
	ByStatus            map[string]int64
	TerminalSince       time.Time
	TerminalDays        int
	WorkersLive         int64
	WorkersStale        int64
	WorkersDraining     int64
	OldestQueuedAge     *float64
	UnschedulableQueued int64
}

// TerminalWindowDays is the window of the terminal counts (= terminal job retention).
const TerminalWindowDays = 30

// MetricsOverview derives current counts from the authoritative tables (never from Prometheus).
func (s *Store) MetricsOverview(ctx context.Context) (Overview, error) {
	o := Overview{ByStatus: map[string]int64{}, TerminalDays: TerminalWindowDays}
	for _, st := range []string{"scheduled", "queued", "running", "retry_wait", "succeeded", "failed", "canceled"} {
		o.ByStatus[st] = 0
	}
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '5s'`); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT now(), now() - make_interval(days => $1)`, TerminalWindowDays).Scan(&o.AsOf, &o.TerminalSince); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT status, count(*) FROM jobs
			WHERE status IN ('scheduled', 'queued', 'running', 'retry_wait') GROUP BY status
			UNION ALL
			SELECT status, count(*) FROM jobs WHERE terminal_at >= $1 GROUP BY status`, o.TerminalSince)
		if err != nil {
			return err
		}
		for rows.Next() {
			var st string
			var n int64
			if err := rows.Scan(&st, &n); err != nil {
				rows.Close()
				return err
			}
			o.ByStatus[st] += n
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT
				count(*) FILTER (WHERE status <> 'deregistered' AND last_heartbeat_at > now() - $1::interval),
				count(*) FILTER (WHERE status <> 'deregistered' AND last_heartbeat_at <= now() - $1::interval),
				count(*) FILTER (WHERE status = 'draining')
			FROM workers`, s.Timing.LivenessTimeout).Scan(&o.WorkersLive, &o.WorkersStale, &o.WorkersDraining); err != nil {
			return err
		}
		// Unschedulable (derived, state-machine.md): queued jobs with no live, active capable worker.
		return tx.QueryRow(ctx, `WITH q AS (
				SELECT handler_name, handler_version, count(*) n, min(available_at) oldest
				FROM jobs WHERE status = 'queued' GROUP BY 1, 2)
			SELECT EXTRACT(EPOCH FROM now() - min(q.oldest))::float8,
				coalesce(sum(q.n) FILTER (WHERE NOT EXISTS (
					SELECT 1 FROM worker_capabilities c JOIN workers w ON w.id = c.worker_id
					WHERE c.handler_name = q.handler_name AND c.handler_version = q.handler_version
					  AND w.status = 'active' AND w.last_heartbeat_at > now() - $1::interval)), 0)
			FROM q`, s.Timing.LivenessTimeout).Scan(&o.OldestQueuedAge, &o.UnschedulableQueued)
	})
	return o, classifyDBError(err)
}

// Point is one series bucket; V nil means "no samples" for percentile series.
type Point struct {
	T time.Time
	V *float64
}

// series SQL by metric: $1 from, $2 to, $3 step. Each returns (bucket start, value).
var seriesSQL = map[string]string{
	"jobs_admitted":     eventCount(`e.event_type IN ('admitted', 'retry_created')`),
	"jobs_succeeded":    eventCount(`e.event_type = 'succeeded'`),
	"jobs_failed":       eventCount(`e.to_status = 'failed' AND e.from_status IS DISTINCT FROM 'failed'`),
	"jobs_canceled":     eventCount(`e.event_type = 'canceled'`),
	"attempts_started":  eventCount(`e.event_type = 'claimed'`),
	"lease_expirations": attemptCount(`a.state IN ('lease_expired', 'deadline_exceeded')`),
	// queue age at first claim: started_at of attempt 1 minus job created_at (scheduled jobs: minus run_after).
	"queue_age_p95_seconds": `SELECT b.t, (SELECT percentile_cont(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM a.started_at - coalesce(j.run_after, j.created_at)))
		FROM job_attempts a JOIN jobs j ON j.id = a.job_id
		WHERE a.attempt_number = 1 AND a.started_at >= b.t AND a.started_at < b.t + $3::interval)::float8
		FROM generate_series($1::timestamptz, $2::timestamptz - $3::interval, $3::interval) b(t)`,
	"run_duration_p95_seconds": `SELECT b.t, (SELECT percentile_cont(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM a.finished_at - a.started_at))
		FROM job_attempts a WHERE a.finished_at >= b.t AND a.finished_at < b.t + $3::interval)::float8
		FROM generate_series($1::timestamptz, $2::timestamptz - $3::interval, $3::interval) b(t)`,
}

func eventCount(pred string) string {
	return `SELECT b.t, count(e.job_id)::float8 FROM generate_series($1::timestamptz, $2::timestamptz - $3::interval, $3::interval) b(t)
		LEFT JOIN job_events e ON e.occurred_at >= b.t AND e.occurred_at < b.t + $3::interval AND ` + pred + `
		GROUP BY b.t ORDER BY b.t`
}

func attemptCount(pred string) string {
	return `SELECT b.t, count(a.id)::float8 FROM generate_series($1::timestamptz, $2::timestamptz - $3::interval, $3::interval) b(t)
		LEFT JOIN job_attempts a ON a.finished_at >= b.t AND a.finished_at < b.t + $3::interval AND ` + pred + `
		GROUP BY b.t ORDER BY b.t`
}

// SeriesMetrics lists the supported timeseries metrics.
func SeriesMetrics() []string {
	return []string{"jobs_admitted", "jobs_succeeded", "jobs_failed", "jobs_canceled", "attempts_started",
		"lease_expirations", "queue_age_p95_seconds", "run_duration_p95_seconds"}
}

// MetricsSeries returns buckets [from, to) of width step that end at or before the DB clock (as_of);
// buckets after as_of are absent, not zero. The caller bounds the window and point count.
func (s *Store) MetricsSeries(ctx context.Context, metric string, from, to time.Time, step time.Duration) ([]Point, time.Time, error) {
	q, ok := seriesSQL[metric]
	if !ok {
		return nil, time.Time{}, fmt.Errorf("store: unknown metric %q", metric)
	}
	var out []Point
	var asOf time.Time
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '10s'`); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&asOf); err != nil {
			return err
		}
		end := to
		if asOf.Before(end) {
			// Only complete buckets: the last bucket must end at or before as_of.
			end = from.Add(asOf.Sub(from).Truncate(step))
		}
		if !end.After(from) {
			return nil
		}
		rows, err := tx.Query(ctx, q, from, end, step)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Point, error) {
			var p Point
			return p, r.Scan(&p.T, &p.V)
		})
		return err
	})
	return out, asOf, classifyDBError(err)
}

// Ops is the alerting snapshot exported on the internal /metrics listener (M8, NFR-OBS-01). It is read
// from the authoritative tables on each scrape; Prometheus only stores the gauges.
type Ops struct {
	Overview
	WorkersRecentlyLost int64            // non-deregistered, heartbeat older than the liveness timeout but < 1 h
	AttemptsFinished5m  map[string]int64 // attempts that ended in the last 5 minutes, by attempt state
	DatabaseSizeBytes   int64
}

// OpsSnapshot reads Ops. Any error means the database is unusable for the scrape.
func (s *Store) OpsSnapshot(ctx context.Context) (Ops, error) {
	o := Ops{AttemptsFinished5m: map[string]int64{}}
	var err error
	if o.Overview, err = s.MetricsOverview(ctx); err != nil {
		return o, err
	}
	for _, st := range []string{"succeeded", "failed", "lease_expired", "deadline_exceeded", "canceled"} {
		o.AttemptsFinished5m[st] = 0
	}
	err = pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '5s'`); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT
				(SELECT count(*) FROM workers WHERE status <> 'deregistered'
					AND last_heartbeat_at <= now() - $1::interval AND last_heartbeat_at > now() - interval '1 hour'),
				pg_database_size(current_database())`, s.Timing.LivenessTimeout).Scan(&o.WorkersRecentlyLost, &o.DatabaseSizeBytes); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT state, count(*) FROM job_attempts
			WHERE finished_at > now() - interval '5 minutes' GROUP BY state`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var st string
			var n int64
			if err := rows.Scan(&st, &n); err != nil {
				return err
			}
			o.AttemptsFinished5m[st] = n
		}
		return rows.Err()
	})
	return o, classifyDBError(err)
}
