package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

// ---- GET /api/v1/audit ------------------------------------------------------------------------

var auditTargets = []string{"job", "worker", "worker_credential", "api_token", "user", "session"}

// listAudit applies the field matrix: operators see every category; viewers see privileged rows only,
// with details masked (permissions.md). A viewer asking for another category gets an empty page.
func (v *V1) listAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var e verrs
	f := store.AuditFilter{}
	cats := queryEnum(q, "category", []string{"privileged", "security", "diagnostic"}, &e)
	if len(cats) > 1 {
		e.add("?category", "range", "at most one value", map[string]any{"max": 1})
	}
	if s := q.Get("subject_type"); s != "" {
		queryEnum(q, "subject_type", auditTargets, &e)
		f.TargetType = s
	}
	if s := q.Get("subject_id"); s != "" {
		if !uuidRe.MatchString(s) {
			e.add("?subject_id", "format", "must be a UUID", nil)
		}
		f.TargetID = strings.ToLower(s)
	}
	f.From, f.To = queryTime(q, "from", &e), queryTime(q, "to", &e)
	if f.From != nil && f.To != nil && !f.To.After(*f.From) {
		e.add("?to", "window_inverted", "must be after from", nil)
	}
	f.Limit = queryInt(q, "limit", 1, 200, 50, &e)
	if e.write(w, r) {
		return
	}
	cur, ok := decodeCursor(q, "-occurred_at")
	if !ok {
		invalidCursor(w, r)
		return
	}
	f.After = cur
	p := principal(r)
	viewer := p.Role != "operator"
	f.Categories = cats
	if viewer {
		if len(cats) > 0 && cats[0] != "privileged" {
			writeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "next_cursor": nil, "as_of": tv(time.Now())})
			return
		}
		f.Categories = []string{"privileged"}
	}
	evs, more, asOf, err := v.Store.ListAudit(r.Context(), f)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	type auditJSON struct {
		ID           string          `json:"id"`
		Category     string          `json:"category"`
		Action       string          `json:"action"`
		Outcome      string          `json:"outcome"`
		Actor        string          `json:"actor"`
		SubjectType  *string         `json:"subject_type"`
		SubjectID    *string         `json:"subject_id"`
		RequestID    *string         `json:"request_id"`
		OccurredAt   string          `json:"occurred_at"`
		Details      json.RawMessage `json:"details"`
		MaskedFields []string        `json:"masked_fields"`
	}
	items := make([]auditJSON, 0, len(evs))
	for _, x := range evs {
		a := auditJSON{ID: x.ID, Category: x.Category, Action: x.Action, Outcome: x.Outcome, Actor: x.Actor, SubjectType: x.TargetType,
			SubjectID: x.TargetID, RequestID: x.RequestID, OccurredAt: tv(x.OccurredAt), Details: rawOrNull(x.Details), MaskedFields: []string{}}
		if viewer {
			a.Details, a.MaskedFields = json.RawMessage("null"), []string{"details"}
		}
		items = append(items, a)
	}
	var next *string
	if more && len(evs) > 0 {
		last := evs[len(evs)-1]
		next = encodeCursor(cursorJSON{S: "-occurred_at", F: filterHash(q), I: last.ID, T: tv(last.OccurredAt)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next, "as_of": tv(asOf)})
}

// ---- metrics ----------------------------------------------------------------------------------

func (v *V1) metricsOverview(w http.ResponseWriter, r *http.Request) {
	o, err := v.Store.MetricsOverview(r.Context())
	if err != nil {
		v.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of": tv(o.AsOf), "jobs_by_status": o.ByStatus,
		"terminal_counts_window":    map[string]any{"days": o.TerminalDays, "since": tv(o.TerminalSince)},
		"workers":                   map[string]int64{"live": o.WorkersLive, "stale": o.WorkersStale, "draining": o.WorkersDraining},
		"oldest_queued_age_seconds": o.OldestQueuedAge, "unschedulable_jobs": o.UnschedulableQueued,
	})
}

// Timeseries bounds (OpenAPI Config.limits).
const (
	MaxSeriesPoints = 2016
	MaxSeriesWindow = 7 * 24 * time.Hour
)

var seriesSteps = map[string]time.Duration{"1m": time.Minute, "5m": 5 * time.Minute, "1h": time.Hour}

func (v *V1) metricsTimeseries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var e verrs
	metric := q.Get("metric")
	if metric == "" {
		e.add("?metric", "required", "is required", nil)
	} else if !slices.Contains(store.SeriesMetrics(), metric) {
		e.add("?metric", "enum", "must be one of "+strings.Join(store.SeriesMetrics(), ", "), nil)
	}
	stepName := q.Get("step")
	if stepName == "" {
		stepName = "5m"
	}
	step, okStep := seriesSteps[stepName]
	if !okStep {
		e.add("?step", "enum", "must be one of 1m, 5m, 1h", nil)
	}
	for _, n := range []string{"from", "to"} {
		if q.Get(n) == "" {
			e.add("?"+n, "required", "is required", nil)
		}
	}
	from, to := queryTime(q, "from", &e), queryTime(q, "to", &e)
	if from != nil && to != nil && okStep {
		switch {
		case !to.After(*from):
			e.add("?to", "window_inverted", "must be after from", nil)
		case to.Sub(*from) > MaxSeriesWindow:
			e.add("?from", "window_too_wide", "window must be at most 7 days", map[string]any{"max_days": 7})
		case int(to.Sub(*from)/step) > MaxSeriesPoints:
			e.add("?step", "range", "too many points; use a larger step", map[string]any{"max_points": MaxSeriesPoints})
		case time.Since(*from) > store.TerminalWindowDays*24*time.Hour: // validation bound only
			e.add("?from", "retention_exceeded", "must be within the 30-day retention window", map[string]any{"max_days": store.TerminalWindowDays})
		}
	}
	if e.write(w, r) {
		return
	}
	// Align to the step so buckets are stable across calls.
	f, t := from.UTC().Truncate(step), to.UTC().Truncate(step)
	if t.Before(*to) {
		t = t.Add(step)
	}
	pts, asOf, err := v.Store.MetricsSeries(r.Context(), metric, f, t, step)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	type pointJSON struct {
		T string   `json:"t"`
		V *float64 `json:"v"`
	}
	out := make([]pointJSON, 0, len(pts))
	for _, p := range pts {
		out = append(out, pointJSON{tv(p.T), p.V})
	}
	writeJSON(w, http.StatusOK, map[string]any{"metric": metric, "step": stepName, "from": tv(f), "to": tv(t), "as_of": tv(asOf), "points": out})
}
