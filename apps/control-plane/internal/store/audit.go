package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AuditRow is one audit_events row. Details must be server-generated and bounded (ADR-0004: 4 KiB).
type AuditRow struct {
	Category, Action, Outcome, Actor string
	TargetType, TargetID             string
	RequestID                        string
	Details                          map[string]any
}

func insertAudit(ctx context.Context, tx pgx.Tx, a AuditRow) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id, request_id, occurred_at, details)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, NULLIF($7, ''), clock_timestamp(), $8)`,
		a.Category, a.Action, a.Outcome, truncate(a.Actor, 200), a.TargetType, nullUUID(a.TargetID), truncate(a.RequestID, 64), jsonOrNull(a.Details))
	return err
}

// Audit appends one row in its own transaction (denials and security events outside a mutation tx).
func (s *Store) Audit(ctx context.Context, a AuditRow) error {
	return s.inTx(ctx, false, func(tx pgx.Tx) error { return insertAudit(ctx, tx, a) })
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// AuditEvent is a row of GET /api/v1/audit.
type AuditEvent struct {
	ID, Category, Action, Outcome, Actor string
	TargetType, TargetID, RequestID      *string
	OccurredAt                           time.Time
	Details                              json.RawMessage
}

// AuditFilter selects audit rows; Categories empty = all. Viewer callers pass only "privileged".
type AuditFilter struct {
	Categories []string
	TargetType string
	TargetID   string
	From, To   *time.Time
	Limit      int
	After      *Cursor // (occurred_at, id) of the last row of the previous page
}

// ListAudit pages audit_events by (occurred_at DESC, id DESC) using audit_events_time_idx.
func (s *Store) ListAudit(ctx context.Context, f AuditFilter) ([]AuditEvent, bool, time.Time, error) {
	b := &sqlb{}
	if len(f.Categories) > 0 {
		b.where("category = ANY(" + b.arg(f.Categories) + ")")
	}
	if f.TargetType != "" {
		b.where("target_type = " + b.arg(f.TargetType))
	}
	if f.TargetID != "" {
		b.where("target_id = " + b.arg(f.TargetID) + "::uuid")
	}
	if f.From != nil {
		b.where("occurred_at >= " + b.arg(*f.From))
	}
	if f.To != nil {
		b.where("occurred_at < " + b.arg(*f.To))
	}
	if f.After != nil {
		b.where(fmt.Sprintf("(occurred_at, id) < (%s, %s::uuid)", b.arg(f.After.T), b.arg(f.After.ID)))
	}
	where := ""
	if len(b.conds) > 0 {
		where = "WHERE " + strings.Join(b.conds, " AND ")
	}
	var out []AuditEvent
	var asOf time.Time
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '5s'`); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&asOf); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id, category, action, outcome, actor, target_type, target_id::text, request_id, occurred_at, details
			FROM audit_events `+where+` ORDER BY occurred_at DESC, id DESC LIMIT `+b.arg(f.Limit+1), b.args...)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (AuditEvent, error) {
			var e AuditEvent
			err := r.Scan(&e.ID, &e.Category, &e.Action, &e.Outcome, &e.Actor, &e.TargetType, &e.TargetID, &e.RequestID, &e.OccurredAt, &e.Details)
			return e, err
		})
		return err
	})
	if err != nil {
		return nil, false, time.Time{}, classifyDBError(err)
	}
	more := len(out) > f.Limit
	if more {
		out = out[:f.Limit]
	}
	return out, more, asOf, nil
}
