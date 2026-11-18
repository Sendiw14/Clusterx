package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// JobFilter is a validated GET /jobs query. Zero values mean "no filter".
type JobFilter struct {
	Statuses, Unschedulable, FailureClasses  []string
	HandlerName, RetriedFrom                 string
	HandlerVersion, PriorityMin, PriorityMax *int
	CreatedAfter, CreatedBefore              *time.Time // after inclusive, before exclusive; default last 7 days
	AvailableAfter, AvailableBefore          *time.Time
	Sort                                     string // -created_at (default), created_at, -available_at, available_at, -priority
	After                                    *Cursor
	Limit                                    int
}

// Cursor is the keyset position of the last row of a page.
type Cursor struct {
	T        time.Time
	Priority int
	Name     string
	ID       string
}

type sqlb struct {
	args  []any
	conds []string
}

func (b *sqlb) arg(v any) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", len(b.args))
}

func (b *sqlb) where(c string) { b.conds = append(b.conds, c) }

// ListJobs returns one page (limit rows) and whether another page exists.
func (s *Store) ListJobs(ctx context.Context, f JobFilter) ([]Job, bool, time.Time, error) {
	b := &sqlb{args: []any{s.Timing.LivenessTimeout}}
	before := "now()"
	if f.CreatedBefore != nil {
		before = b.arg(*f.CreatedBefore) + "::timestamptz"
		b.where("j.created_at < " + before)
	}
	if f.CreatedAfter != nil {
		b.where("j.created_at >= " + b.arg(*f.CreatedAfter))
	} else {
		b.where("j.created_at >= " + before + " - interval '7 days'")
	}
	if len(f.Statuses) > 0 {
		b.where("j.status = ANY(" + b.arg(f.Statuses) + "::text[])")
	}
	if f.HandlerName != "" {
		b.where("j.handler_name = " + b.arg(f.HandlerName))
	}
	if f.HandlerVersion != nil {
		b.where("j.handler_version = " + b.arg(*f.HandlerVersion))
	}
	if f.PriorityMin != nil {
		b.where("j.priority >= " + b.arg(*f.PriorityMin))
	}
	if f.PriorityMax != nil {
		b.where("j.priority <= " + b.arg(*f.PriorityMax))
	}
	if f.RetriedFrom != "" {
		b.where("j.retried_from_job_id = " + b.arg(f.RetriedFrom) + "::uuid")
	}
	if len(f.FailureClasses) > 0 {
		b.where("j.failure_class = ANY(" + b.arg(f.FailureClasses) + "::text[])")
	}
	if len(f.Unschedulable) > 0 {
		b.where("u.reason = ANY(" + b.arg(f.Unschedulable) + "::text[])")
	}
	if f.AvailableAfter != nil {
		b.where("j.available_at >= " + b.arg(*f.AvailableAfter))
	}
	if f.AvailableBefore != nil {
		b.where("j.available_at < " + b.arg(*f.AvailableBefore))
	}
	var order string
	switch f.Sort {
	case "", "-created_at":
		order = "j.created_at DESC, j.id DESC"
		if f.After != nil {
			b.where("(j.created_at, j.id) < (" + b.arg(f.After.T) + ", " + b.arg(f.After.ID) + "::uuid)")
		}
	case "created_at":
		order = "j.created_at, j.id"
		if f.After != nil {
			b.where("(j.created_at, j.id) > (" + b.arg(f.After.T) + ", " + b.arg(f.After.ID) + "::uuid)")
		}
	case "-available_at":
		order = "j.available_at DESC, j.id DESC"
		if f.After != nil {
			b.where("(j.available_at, j.id) < (" + b.arg(f.After.T) + ", " + b.arg(f.After.ID) + "::uuid)")
		}
	case "available_at":
		order = "j.available_at, j.id"
		if f.After != nil {
			b.where("(j.available_at, j.id) > (" + b.arg(f.After.T) + ", " + b.arg(f.After.ID) + "::uuid)")
		}
	case "-priority":
		order = "j.priority DESC, j.available_at, j.id"
		if f.After != nil {
			p := b.arg(f.After.Priority)
			b.where("(j.priority < " + p + " OR (j.priority = " + p + " AND (j.available_at, j.id) > (" +
				b.arg(f.After.T) + ", " + b.arg(f.After.ID) + "::uuid)))")
		}
	default:
		return nil, false, time.Time{}, fmt.Errorf("store: unknown sort %q", f.Sort)
	}
	sql := jobSelect + " WHERE " + strings.Join(b.conds, " AND ") + " ORDER BY " + order + " LIMIT " + b.arg(f.Limit+1)
	rows, err := s.Pool.Query(ctx, sql, b.args...)
	if err != nil {
		return nil, false, time.Time{}, classifyDBError(err)
	}
	jobs, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Job, error) { return scanJob(r) })
	if err != nil {
		return nil, false, time.Time{}, classifyDBError(err)
	}
	more := len(jobs) > f.Limit
	if more {
		jobs = jobs[:f.Limit]
	}
	asOf := time.Time{}
	if len(jobs) > 0 {
		asOf = jobs[0].AsOf
	} else if asOf, err = s.Now(ctx); err != nil {
		return nil, false, time.Time{}, err
	}
	return jobs, more, asOf, nil
}

// Worker is a worker row with derived fields.
type Worker struct {
	ID, Name, Status, IncarnationID                        string
	Live, CredentialRevoked                                bool
	Version                                                int64
	Capacity, ActiveAttempts, ProtocolVersion              int
	SDKVersion, Hostname                                   *string
	Capabilities                                           []Capability
	RegisteredAt, LastSeenAt                               time.Time
	DrainRequestedAt, DrainDeadlineAt, CredentialRevokedAt *time.Time
	AsOf                                                   time.Time
}

const workerSelect = `SELECT w.id, w.name, w.status, w.current_incarnation_id,
	w.status <> 'deregistered' AND w.last_heartbeat_at > now() - $1::interval,
	c.revoked_at IS NOT NULL OR (c.expires_at IS NOT NULL AND c.expires_at <= now()), w.version, w.capacity,
	(SELECT count(*) FROM job_attempts a WHERE a.worker_id = w.id AND a.state = 'active'), w.protocol_version,
	w.sdk_version, w.hostname,
	coalesce((SELECT array_agg(k.handler_name || '@' || k.handler_version ORDER BY k.handler_name, k.handler_version)
		FROM worker_capabilities k WHERE k.worker_id = w.id), '{}'),
	w.registered_at, w.last_heartbeat_at, w.drain_requested_at, w.drain_deadline_at, c.revoked_at, clock_timestamp()
FROM workers w JOIN worker_credentials c ON c.id = w.credential_id`

func scanWorker(r pgx.Row) (Worker, error) {
	var w Worker
	var caps []string
	err := r.Scan(&w.ID, &w.Name, &w.Status, &w.IncarnationID, &w.Live, &w.CredentialRevoked, &w.Version, &w.Capacity,
		&w.ActiveAttempts, &w.ProtocolVersion, &w.SDKVersion, &w.Hostname, &caps, &w.RegisteredAt, &w.LastSeenAt,
		&w.DrainRequestedAt, &w.DrainDeadlineAt, &w.CredentialRevokedAt, &w.AsOf)
	for _, c := range caps {
		i := strings.LastIndexByte(c, '@')
		var v int
		_, _ = fmt.Sscan(c[i+1:], &v)
		w.Capabilities = append(w.Capabilities, Capability{Name: c[:i], Version: v})
	}
	return w, err
}

// WorkerFilter is a validated GET /workers query.
type WorkerFilter struct {
	Statuses    []string
	Liveness    string // live | stale | ""
	HandlerName string
	Sort        string // -last_seen_at (default), name, -registered_at
	After       *Cursor
	Limit       int
}

// ListWorkers returns one page of workers.
func (s *Store) ListWorkers(ctx context.Context, f WorkerFilter) ([]Worker, bool, time.Time, error) {
	b := &sqlb{args: []any{s.Timing.LivenessTimeout}, conds: []string{"true"}}
	if len(f.Statuses) > 0 {
		b.where("w.status = ANY(" + b.arg(f.Statuses) + "::text[])")
	}
	switch f.Liveness {
	case "live":
		b.where("w.status <> 'deregistered' AND w.last_heartbeat_at > now() - $1::interval")
	case "stale":
		b.where("NOT (w.status <> 'deregistered' AND w.last_heartbeat_at > now() - $1::interval)")
	}
	if f.HandlerName != "" {
		b.where("EXISTS (SELECT 1 FROM worker_capabilities k WHERE k.worker_id = w.id AND k.handler_name = " + b.arg(f.HandlerName) + ")")
	}
	var order string
	switch f.Sort {
	case "", "-last_seen_at":
		order = "w.last_heartbeat_at DESC, w.id DESC"
		if f.After != nil {
			b.where("(w.last_heartbeat_at, w.id) < (" + b.arg(f.After.T) + ", " + b.arg(f.After.ID) + "::uuid)")
		}
	case "name":
		order = "w.name, w.id"
		if f.After != nil {
			b.where("(w.name, w.id) > (" + b.arg(f.After.Name) + ", " + b.arg(f.After.ID) + "::uuid)")
		}
	case "-registered_at":
		order = "w.registered_at DESC, w.id DESC"
		if f.After != nil {
			b.where("(w.registered_at, w.id) < (" + b.arg(f.After.T) + ", " + b.arg(f.After.ID) + "::uuid)")
		}
	default:
		return nil, false, time.Time{}, fmt.Errorf("store: unknown sort %q", f.Sort)
	}
	rows, err := s.Pool.Query(ctx, workerSelect+" WHERE "+strings.Join(b.conds, " AND ")+" ORDER BY "+order+" LIMIT "+b.arg(f.Limit+1), b.args...)
	if err != nil {
		return nil, false, time.Time{}, classifyDBError(err)
	}
	ws, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Worker, error) { return scanWorker(r) })
	if err != nil {
		return nil, false, time.Time{}, classifyDBError(err)
	}
	more := len(ws) > f.Limit
	if more {
		ws = ws[:f.Limit]
	}
	var asOf time.Time
	if len(ws) > 0 {
		asOf = ws[0].AsOf
	} else if asOf, err = s.Now(ctx); err != nil {
		return nil, false, time.Time{}, err
	}
	return ws, more, asOf, nil
}

// ActiveAttempt is one row of WorkerDetail.active.
type ActiveAttempt struct {
	JobID, AttemptID, HandlerName string
	AttemptNumber                 int
	StartedAt, LeaseExpiresAt     time.Time
	CancelRequested               bool
}

// GetWorker returns a worker and its active attempts. Denial: worker_not_found.
func (s *Store) GetWorker(ctx context.Context, id string) (Worker, []ActiveAttempt, error) {
	w, err := scanWorker(s.Pool.QueryRow(ctx, workerSelect+` WHERE w.id = $2`, s.Timing.LivenessTimeout, id))
	if isNoRows(err) {
		return Worker{}, nil, deny("worker_not_found")
	}
	if err != nil {
		return Worker{}, nil, classifyDBError(err)
	}
	rows, err := s.Pool.Query(ctx, `SELECT a.job_id, a.id, j.handler_name, a.attempt_number, a.started_at, j.lease_expires_at,
			j.cancel_requested_at IS NOT NULL
		FROM job_attempts a JOIN jobs j ON j.id = a.job_id AND j.current_attempt_id = a.id
		WHERE a.worker_id = $1 AND a.state = 'active' ORDER BY a.started_at, a.id LIMIT 256`, id)
	if err != nil {
		return Worker{}, nil, classifyDBError(err)
	}
	act, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (ActiveAttempt, error) {
		var a ActiveAttempt
		return a, r.Scan(&a.JobID, &a.AttemptID, &a.HandlerName, &a.AttemptNumber, &a.StartedAt, &a.LeaseExpiresAt, &a.CancelRequested)
	})
	return w, act, classifyDBError(err)
}

// CapableCounts is, per handler version, the non-deregistered workers advertising it and how many are live and active.
type CapableCounts struct{ Capable, Live int }

// HandlerCapacity returns CapableCounts keyed by "name@version" and the DB time.
func (s *Store) HandlerCapacity(ctx context.Context) (map[string]CapableCounts, time.Time, error) {
	rows, err := s.Pool.Query(ctx, `SELECT k.handler_name || '@' || k.handler_version, count(*),
			count(*) FILTER (WHERE w.status = 'active' AND w.last_heartbeat_at > now() - $1::interval)
		FROM worker_capabilities k JOIN workers w ON w.id = k.worker_id AND w.status <> 'deregistered'
		GROUP BY 1`, s.Timing.LivenessTimeout)
	if err != nil {
		return nil, time.Time{}, classifyDBError(err)
	}
	out := map[string]CapableCounts{}
	for rows.Next() {
		var k string
		var c CapableCounts
		if err := rows.Scan(&k, &c.Capable, &c.Live); err != nil {
			rows.Close()
			return nil, time.Time{}, classifyDBError(err)
		}
		out[k] = c
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, classifyDBError(err)
	}
	now, err := s.Now(ctx)
	return out, now, err
}
