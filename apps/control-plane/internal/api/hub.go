package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Message is one StreamMessage frame (OpenAPI). Hints are best effort: they may be dropped, coalesced or
// delayed; clients reconcile from HTTP snapshots (FR-RT-01, AC-16).
type Message struct {
	Type             string  `json:"type"`
	Entity           string  `json:"entity,omitempty"`
	ID               *string `json:"id,omitempty"`
	Version          *int64  `json:"version,omitempty"`
	Reason           string  `json:"reason,omitempty"`
	ServerTime       string  `json:"server_time,omitempty"`
	SessionExpiresAt string  `json:"session_expires_at,omitempty"`
	ExpiresAt        string  `json:"expires_at,omitempty"`
}

// MarshalJSON emits exactly the members of the frame's StreamMessage variant (hint carries id null for
// entity=metrics).
func (m Message) MarshalJSON() ([]byte, error) {
	switch m.Type {
	case "hint":
		var v int64
		if m.Version != nil {
			v = *m.Version
		}
		return json.Marshal(struct {
			Type    string  `json:"type"`
			Entity  string  `json:"entity"`
			ID      *string `json:"id"`
			Version int64   `json:"version"`
		}{m.Type, m.Entity, m.ID, v})
	case "resync":
		return json.Marshal(struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		}{m.Type, m.Reason})
	case "hello":
		return json.Marshal(struct {
			Type             string `json:"type"`
			ServerTime       string `json:"server_time"`
			SessionExpiresAt string `json:"session_expires_at"`
		}{m.Type, m.ServerTime, m.SessionExpiresAt})
	case "auth_expiring":
		return json.Marshal(struct {
			Type      string `json:"type"`
			ExpiresAt string `json:"expires_at"`
		}{m.Type, m.ExpiresAt})
	}
	type plain Message
	return json.Marshal(plain(m))
}

func hint(entity, id string, version int64) Message {
	m := Message{Type: "hint", Entity: entity, Version: &version}
	if id != "" {
		m.ID = &id
	} else {
		m.ID = nil
	}
	return m
}

// SubscriberBuffer bounds the queued messages per stream. On overflow the queue is discarded and the
// stream receives one resync{buffer_overflow} (slow consumer drop, no unbounded memory).
const SubscriberBuffer = 256

// Hub fans hints out to WebSocket subscribers. Per entity id it forwards only increasing versions, so
// a subscriber never sees a hint older than one it already got (reordering is absorbed server side).
type Hub struct {
	Log *slog.Logger

	mu       sync.Mutex
	subs     map[*subscriber]struct{}
	last     map[string]int64 // entity:id -> highest version published
	lastSeen map[string]time.Time
	revalGen chan struct{}
	closed   bool
}

type subscriber struct {
	mu       sync.Mutex
	queue    []Message
	overflow bool
	notify   chan struct{}
	key      string   // session id or token id (stream count limit)
	scopes   []string // principal scopes; nil = every hint (internal tests)
}

// allows reports whether the subscriber may see m: a hint needs the read scope of its entity
// (RV5-07), so a jobs:read-only token stream never sees worker or metrics hints.
func (s *subscriber) allows(m Message) bool {
	if m.Type != "hint" || s.scopes == nil {
		return true
	}
	need := map[string]string{"job": "jobs:read", "worker": "workers:read", "metrics": "metrics:read"}[m.Entity]
	return need != "" && slices.Contains(s.scopes, need)
}

// NewHub returns an empty hub.
func NewHub(log *slog.Logger) *Hub {
	return &Hub{Log: log, subs: map[*subscriber]struct{}{}, last: map[string]int64{}, lastSeen: map[string]time.Time{},
		revalGen: make(chan struct{})}
}

// MaxStreamsPerPrincipal is the per-session / per-token stream limit (close 4429).
const MaxStreamsPerPrincipal = 5

func (h *Hub) subscribe(key string, scopes ...string) (*subscriber, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for s := range h.subs {
		if s.key == key {
			n++
		}
	}
	if n >= MaxStreamsPerPrincipal || h.closed {
		return nil, false
	}
	s := &subscriber{notify: make(chan struct{}, 1), key: key, scopes: scopes}
	h.subs[s] = struct{}{}
	return s, true
}

func (h *Hub) unsubscribe(s *subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
}

// Subscribers returns the number of open streams (tests, metrics).
func (h *Hub) Subscribers() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

func (s *subscriber) push(m Message) {
	s.mu.Lock()
	if s.overflow {
		s.mu.Unlock()
		return
	}
	if len(s.queue) >= SubscriberBuffer {
		s.queue, s.overflow = nil, true
	} else {
		s.queue = append(s.queue, m)
	}
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// take returns the queued messages; after an overflow it returns a single resync.
func (s *subscriber) take() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overflow {
		s.overflow, s.queue = false, nil
		return []Message{{Type: "resync", Reason: "buffer_overflow"}}
	}
	q := s.queue
	s.queue = nil
	return q
}

// publish delivers m to every subscriber; hints for an entity id are forwarded only when newer.
func (h *Hub) publish(m Message) {
	if h == nil {
		return
	}
	h.mu.Lock()
	if m.Type == "hint" && m.ID != nil {
		k := m.Entity + ":" + *m.ID
		if v, ok := h.last[k]; ok && *m.Version <= v {
			h.mu.Unlock()
			return
		}
		h.last[k] = *m.Version
		h.lastSeen[k] = time.Now()
	}
	subs := make([]*subscriber, 0, len(h.subs))
	for s := range h.subs {
		if s.allows(m) {
			subs = append(subs, s)
		}
	}
	h.mu.Unlock()
	for _, s := range subs {
		s.push(m)
	}
}

// PublishJob / PublishWorker publish a change hint (nil-safe).
func (h *Hub) PublishJob(id string, version int64) { h.publish(hint("job", id, version)) }

// PublishWorker publishes a worker hint.
func (h *Hub) PublishWorker(id string, version int64) { h.publish(hint("worker", id, version)) }

// Resync tells every subscriber to refetch snapshots.
func (h *Hub) Resync(reason string) { h.publish(Message{Type: "resync", Reason: reason}) }

// Revalidate asks every stream to re-check its credential now (after logout or revocation) instead of
// waiting for the periodic re-check.
func (h *Hub) Revalidate() {
	if h == nil {
		return
	}
	h.mu.Lock()
	close(h.revalGen)
	h.revalGen = make(chan struct{})
	h.mu.Unlock()
}

func (h *Hub) revalidateSignal() <-chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.revalGen
}

// Close stops accepting subscribers (shutdown).
func (h *Hub) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.closed = true
	h.mu.Unlock()
	h.Revalidate()
}

func (h *Hub) isClosed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closed
}

func (h *Hub) pruneVersions(olderThan time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cut := time.Now().Add(-olderThan)
	for k, t := range h.lastSeen {
		if t.Before(cut) {
			delete(h.lastSeen, k)
			delete(h.last, k)
		}
	}
}

// ---- DB change feed ---------------------------------------------------------------------------

// Feed polls PostgreSQL for job events and worker version changes and publishes hints. It is the only
// source for changes made by workers and the scheduler (same process, but different code paths), so
// every committed transition reaches the hub within about one interval.
//
// The cursor is a transaction-visibility position, not a clock (RV5-04): each job_events row carries
// its writer's txid, and a poll reads the rows visible to its own snapshot but not to the previous
// poll's snapshot. A transaction that commits late is therefore read by the first poll after its
// commit, however long it ran. A failed poll keeps the cursor and loses nothing. Only overflow (more
// than MaxRows new rows; the cursor then jumps to the current snapshot) or an xid regression (xmax
// moved backwards: restore/failover) is a real gap: it broadcasts resync{gap_detected}, at most once
// per ResyncEvery. Cost: each poll scans the events with txid >= xmin(previous snapshot), so a session
// holding an xid for a long time (admin psql, long migration) makes polls re-scan everything written
// since it began (RV5R-03a, operational note; runtime sessions are capped by
// idle_in_transaction_session_timeout).
type Feed struct {
	Pool        *pgxpool.Pool
	Hub         *Hub
	Interval    time.Duration // default 1 s
	MaxRows     int           // per poll; default 2000
	ResyncEvery time.Duration // minimum spacing of gap resyncs; default 5 s
	Log         *slog.Logger
}

type evKey struct {
	job     string
	version int64
}

// Run polls until ctx ends.
func (f *Feed) Run(ctx context.Context) {
	if f.Interval <= 0 {
		f.Interval = time.Second
	}
	if f.MaxRows <= 0 {
		f.MaxRows = 2000
	}
	if f.ResyncEvery <= 0 {
		f.ResyncEvery = 5 * time.Second
	}
	var snap string // pg_snapshot text of the previous poll; "" = not started
	workers := map[string]int64{}
	gap, failing := false, false
	var lastResync time.Time
	lastPrune := time.Now()
	t := time.NewTicker(f.Interval)
	defer t.Stop()
	// Polls run with or without subscribers (publishing to nobody is free), so a client that subscribes
	// mid-interval still gets the hints of transactions that commit around its subscription.
	_ = f.Pool.QueryRow(ctx, `SELECT pg_current_snapshot()::text`).Scan(&snap)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if snap == "" {
			_ = f.Pool.QueryRow(ctx, `SELECT pg_current_snapshot()::text`).Scan(&snap)
			continue
		}
		overflow, err := f.poll(ctx, &snap, workers)
		gap = gap || overflow
		if err != nil {
			if ctx.Err() == nil && !failing {
				f.Log.Warn("stream feed poll failed; retrying from the same cursor", "err", err)
			}
			failing = true
			continue
		}
		failing = false
		if gap && time.Since(lastResync) >= f.ResyncEvery {
			f.Hub.Resync("gap_detected")
			gap, lastResync = false, time.Now()
		}
		if time.Since(lastPrune) > time.Minute {
			lastPrune = time.Now()
			f.Hub.pruneVersions(10 * time.Minute)
		}
	}
}

// poll publishes the job events committed since the snapshot *snap and advances it; it reports
// overflow when more than MaxRows rows were new (the excess is skipped). Then it publishes worker
// version changes.
func (f *Feed) poll(ctx context.Context, snap *string, workers map[string]int64) (bool, error) {
	// pg_current_snapshot() is the statement's own snapshot, so "visible now" and the new cursor agree.
	// regressed: the current snapshot's xmax is below the previous one (PITR restore or replica
	// failover under a running coordinator, RV5R-03b); rows may have been skipped, so resync.
	rows, err := f.Pool.Query(ctx, `WITH s AS (SELECT pg_current_snapshot() AS cur)
		SELECT s.cur::text, pg_snapshot_xmax(s.cur) < pg_snapshot_xmax($1::pg_snapshot), e.job_id::text, e.version FROM s LEFT JOIN job_events e
		  ON e.txid >= pg_snapshot_xmin($1::pg_snapshot) AND NOT pg_visible_in_snapshot(e.txid, $1::pg_snapshot)
		LIMIT $2`, *snap, f.MaxRows+1)
	if err != nil {
		return false, err
	}
	var cur string
	var regressed bool
	var evs []evKey
	for rows.Next() {
		var job *string
		var version *int64
		if err := rows.Scan(&cur, &regressed, &job, &version); err != nil {
			rows.Close()
			return false, err
		}
		if job != nil {
			evs = append(evs, evKey{*job, *version})
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	overflow := len(evs) > f.MaxRows || regressed
	sort.Slice(evs, func(i, j int) bool {
		if evs[i].job != evs[j].job {
			return evs[i].job < evs[j].job
		}
		return evs[i].version < evs[j].version
	})
	for _, k := range evs {
		f.Hub.PublishJob(k.job, k.version)
	}
	if len(evs) > 0 {
		f.Hub.publish(hint("metrics", "", 0))
	}
	*snap = cur

	wrows, err := f.Pool.Query(ctx, `SELECT id::text, version FROM workers
		WHERE status <> 'deregistered' OR deregistered_at > now() - interval '10 minutes'`)
	if err != nil {
		return overflow, err
	}
	present := make(map[string]bool, len(workers))
	for wrows.Next() {
		var id string
		var v int64
		if err := wrows.Scan(&id, &v); err != nil {
			wrows.Close()
			return overflow, err
		}
		present[id] = true
		if old, ok := workers[id]; !ok || old != v {
			workers[id] = v
			f.Hub.PublishWorker(id, v)
		}
	}
	if err := wrows.Err(); err != nil {
		return overflow, err
	}
	for id := range workers { // rows no longer listed (deregistered > 10 min, RV5R-06)
		if !present[id] {
			delete(workers, id)
		}
	}
	return overflow, nil
}
