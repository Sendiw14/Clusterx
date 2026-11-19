package api

import (
	"context"
	"encoding/json"
	"log/slog"
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
	key      string // session id or token id (stream count limit)
}

// NewHub returns an empty hub.
func NewHub(log *slog.Logger) *Hub {
	return &Hub{Log: log, subs: map[*subscriber]struct{}{}, last: map[string]int64{}, lastSeen: map[string]time.Time{},
		revalGen: make(chan struct{})}
}

// MaxStreamsPerPrincipal is the per-session / per-token stream limit (close 4429).
const MaxStreamsPerPrincipal = 5

func (h *Hub) subscribe(key string) (*subscriber, bool) {
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
	s := &subscriber{notify: make(chan struct{}, 1), key: key}
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
		subs = append(subs, s)
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
// every committed transition reaches the hub within about one interval. A poll that fails, or that
// finds more rows than it can forward, broadcasts resync{gap_detected}: clients refetch.
type Feed struct {
	Pool     *pgxpool.Pool
	Hub      *Hub
	Interval time.Duration // default 1 s
	Lookback time.Duration // overlap re-read to catch late commits; default 10 s
	MaxRows  int           // per poll; default 2000
	Log      *slog.Logger
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
	if f.Lookback <= 0 {
		f.Lookback = 10 * time.Second
	}
	if f.MaxRows <= 0 {
		f.MaxRows = 2000
	}
	var watermark time.Time
	seen := map[evKey]time.Time{}
	workers := map[string]int64{}
	gap := false
	lastPrune := time.Now()
	t := time.NewTicker(f.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if f.Hub.Subscribers() == 0 && !watermark.IsZero() {
			// Nobody listens: keep only the watermark moving so a new subscriber does not get a backlog.
			if err := f.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&watermark); err != nil {
				gap = true
			}
			continue
		}
		n, err := f.poll(ctx, &watermark, seen, workers)
		switch {
		case err != nil:
			if ctx.Err() == nil && !gap {
				f.Log.Warn("stream feed poll failed; clients will be told to resync", "err", err)
			}
			gap = true
			continue
		case gap || n > f.MaxRows:
			f.Hub.Resync("gap_detected")
			gap = false
		}
		if time.Since(lastPrune) > time.Minute {
			lastPrune = time.Now()
			for k, at := range seen {
				if time.Since(at) > 2*f.Lookback {
					delete(seen, k)
				}
			}
			f.Hub.pruneVersions(10 * time.Minute)
		}
	}
}

// poll reads events committed since watermark-lookback and the current worker versions.
func (f *Feed) poll(ctx context.Context, watermark *time.Time, seen map[evKey]time.Time, workers map[string]int64) (int, error) {
	var now time.Time
	if err := f.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return 0, err
	}
	if watermark.IsZero() {
		*watermark = now
	}
	rows, err := f.Pool.Query(ctx, `SELECT job_id::text, version FROM job_events WHERE occurred_at > $1
		ORDER BY occurred_at LIMIT $2`, watermark.Add(-f.Lookback), f.MaxRows+1)
	if err != nil {
		return 0, err
	}
	var evs []evKey
	for rows.Next() {
		var k evKey
		if err := rows.Scan(&k.job, &k.version); err != nil {
			rows.Close()
			return 0, err
		}
		evs = append(evs, k)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	sort.Slice(evs, func(i, j int) bool {
		if evs[i].job != evs[j].job {
			return evs[i].job < evs[j].job
		}
		return evs[i].version < evs[j].version
	})
	fresh := 0
	for _, k := range evs {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = time.Now()
		fresh++
		f.Hub.PublishJob(k.job, k.version)
	}
	if fresh > 0 {
		f.Hub.publish(hint("metrics", "", 0))
	}
	*watermark = now

	wrows, err := f.Pool.Query(ctx, `SELECT id::text, version FROM workers
		WHERE status <> 'deregistered' OR deregistered_at > now() - interval '10 minutes'`)
	if err != nil {
		return len(evs), err
	}
	for wrows.Next() {
		var id string
		var v int64
		if err := wrows.Scan(&id, &v); err != nil {
			wrows.Close()
			return len(evs), err
		}
		if old, ok := workers[id]; !ok || old != v {
			workers[id] = v
			f.Hub.PublishWorker(id, v)
		}
	}
	return len(evs), wrows.Err()
}
