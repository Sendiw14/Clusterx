package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

// Stream close codes (OpenAPI openStream).
const (
	CloseSessionEnded websocket.StatusCode = 4401
	CloseForbidden    websocket.StatusCode = 4403
	CloseIdle         websocket.StatusCode = 4408
	CloseTooMany      websocket.StatusCode = 4429
)

// Stream timing; variables so tests can shorten them.
var (
	StreamRevalidateEvery = 30 * time.Second // <= 30 s (ADR-0006 §4, RV01-13)
	StreamPingEvery       = 30 * time.Second
	StreamPongTimeout     = 10 * time.Second
	StreamWriteTimeout    = 5 * time.Second
	StreamAuthWarning     = 60 * time.Second // auth_expiring lead time
)

// stream implements GET /api/v1/stream after guard authenticated the upgrade request (session assertion
// or API token with jobs:read). Frames are StreamMessage JSON hints; snapshots stay authoritative.
func (v *V1) stream(w http.ResponseWriter, r *http.Request) {
	if v.Hub == nil || v.Hub.isClosed() {
		w.Header().Set("Retry-After", "5")
		writeError(w, r, http.StatusServiceUnavailable, "unavailable", "Live stream unavailable; poll the snapshots.")
		return
	}
	if o := r.Header.Get("Origin"); o != "" && !slices.Contains(v.StreamOrigins, o) {
		// Browsers never reach Go directly (ADR-0005 #6); a browser Origin must be allowlisted explicitly.
		writeErrorDetails(w, r, http.StatusForbidden, "forbidden", "Origin not allowed.", map[string]any{"reason": "origin_not_allowed"})
		return
	}
	p := principal(r)
	key, end := "", time.Time{}
	if p.Kind == "session" {
		key = "session:" + p.Session.ID
		end = sessionEnd(*p.Session)
	} else {
		key = p.Ref
		if p.TokenExpiresAt != nil {
			end = *p.TokenExpiresAt
		}
	}
	// The HTTP server's read/write timeouts must not cut a long-lived socket; liveness is ping/pong.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	// Origin was checked above; the library's same-host check would reject the BFF's server-side client.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return // Accept wrote the HTTP error
	}
	defer c.CloseNow()
	c.SetReadLimit(512)
	sub, ok := v.Hub.subscribe(key, append([]string{}, p.Scopes...)...) // non-nil: filter by scopes
	if !ok {
		_ = c.Close(CloseTooMany, "too many streams")
		return
	}
	defer v.Hub.unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.WithoutCancel(r.Context()))
	defer cancel()
	// Reader: the client may only send {"type":"ping"}; anything else closes the stream (policy).
	readErr := make(chan error, 1)
	go func() {
		for {
			typ, b, err := c.Read(ctx)
			if err != nil {
				readErr <- err
				return
			}
			var m struct {
				Type string `json:"type"`
			}
			if typ != websocket.MessageText || json.Unmarshal(b, &m) != nil || m.Type != "ping" {
				readErr <- errors.New("unexpected client frame")
				_ = c.Close(websocket.StatusPolicyViolation, "clients send only ping")
				return
			}
		}
	}()

	send := func(m Message) bool {
		b, _ := json.Marshal(m)
		wctx, wc := context.WithTimeout(ctx, StreamWriteTimeout)
		defer wc()
		if err := c.Write(wctx, websocket.MessageText, b); err != nil {
			return false // slow or gone consumer: drop the stream; the client reconnects and resyncs
		}
		return true
	}
	hello := Message{Type: "hello", ServerTime: tv(time.Now())}
	if !end.IsZero() {
		hello.SessionExpiresAt = tv(end)
	} else {
		hello.SessionExpiresAt = tv(time.Now().Add(24 * time.Hour))
	}
	if !send(hello) {
		return
	}

	reval := time.NewTicker(StreamRevalidateEvery)
	defer reval.Stop()
	ping := time.NewTicker(StreamPingEvery)
	defer ping.Stop()
	warned := false
	endTimer := make(<-chan time.Time)
	arm := func() {
		if !end.IsZero() {
			d := time.Until(end)
			if !warned && d > StreamAuthWarning {
				d -= StreamAuthWarning
			}
			endTimer = time.After(max(d, 0))
		}
	}
	arm()
	for {
		revalSig := v.Hub.revalidateSignal()
		select {
		case err := <-readErr:
			if websocket.CloseStatus(err) == -1 && ctx.Err() == nil {
				_ = c.Close(websocket.StatusPolicyViolation, "read error")
			}
			return
		case <-sub.notify:
			for _, m := range sub.take() {
				if !send(m) {
					return
				}
			}
		case <-ping.C:
			pctx, pc := context.WithTimeout(ctx, StreamPongTimeout)
			err := c.Ping(pctx)
			pc()
			if err != nil {
				_ = c.Close(CloseIdle, "pong timeout")
				return
			}
		case <-endTimer:
			if !warned && time.Until(end) > 0 {
				warned = true
				if !send(Message{Type: "auth_expiring", ExpiresAt: tv(end)}) {
					return
				}
				arm()
				continue
			}
			if code, ok := v.revalidateStream(ctx, p); !ok {
				_ = c.Close(code, "session ended")
				return
			}
			// Still valid (e.g. idle expiry moved because the user was active): re-arm from the DB.
			if p.Kind == "session" {
				if s, err := v.Store.AuthenticateSession(ctx, p.Session.ID, p.Session.UserID, false); err == nil {
					end, warned = sessionEnd(s), false
					arm()
				}
			}
		case <-reval.C:
			if code, ok := v.revalidateStream(ctx, p); !ok {
				_ = c.Close(code, "credential no longer valid")
				return
			}
		case <-revalSig:
			if v.Hub.isClosed() {
				_ = c.Close(websocket.StatusGoingAway, "server shutdown")
				return
			}
			if code, ok := v.revalidateStream(ctx, p); !ok {
				_ = c.Close(code, "credential no longer valid")
				return
			}
		}
	}
}

func sessionEnd(s store.Session) time.Time {
	if s.IdleUntil.Before(s.ExpiresAt) {
		return s.IdleUntil
	}
	return s.ExpiresAt
}

// revalidateStream re-checks the stream's credential against PostgreSQL: sessions (incl. sid↔sub
// binding) and API token authority (RV01-13). It returns the close code when the stream must end.
// A database error keeps the stream open (hints are harmless) until the next check.
func (v *V1) revalidateStream(ctx context.Context, p store.Principal) (websocket.StatusCode, bool) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if p.Kind == "session" {
		_, err := v.Store.AuthenticateSession(cctx, p.Session.ID, p.Session.UserID, false)
		if _, denied := errors.AsType[*store.Denial](err); denied {
			return CloseSessionEnded, false
		}
		return 0, true
	}
	tp, err := v.Store.TokenAuthority(cctx, p.TokenID)
	if _, denied := errors.AsType[*store.Denial](err); denied {
		return CloseSessionEnded, false
	}
	if err == nil && !tp.Has("jobs:read") {
		return CloseForbidden, false
	}
	return 0, true
}

// allowedOrigins parses a comma-separated origin allowlist.
func ParseOrigins(s string) []string {
	var out []string
	for _, o := range strings.Split(s, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}
