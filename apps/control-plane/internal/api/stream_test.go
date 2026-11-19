package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/api"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/auth"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
)

func dialStream(t *testing.T, srv *httptest.Server, bearer, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	h := http.Header{}
	if bearer != "" {
		h.Set("Authorization", "Bearer "+bearer)
	}
	if origin != "" {
		h.Set("Origin", origin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/stream", &websocket.DialOptions{HTTPHeader: h})
}

func readMsg(t *testing.T, c *websocket.Conn, d time.Duration) (map[string]any, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	_, b, err := c.Read(ctx)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("frame %s: %v", b, err)
	}
	return m, nil
}

// readUntil returns the first frame satisfying ok, failing after d.
func readUntil(t *testing.T, c *websocket.Conn, d time.Duration, ok func(map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		m, err := readMsg(t, c, time.Until(deadline))
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if ok(m) {
			return m
		}
	}
	t.Fatal("frame not received")
	return nil
}

func wantClose(t *testing.T, c *websocket.Conn, code websocket.StatusCode, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		_, err := readMsg(t, c, time.Until(deadline))
		if err == nil {
			continue
		}
		if got := websocket.CloseStatus(err); got != code {
			t.Fatalf("close status %d want %d (%v)", got, code, err)
		}
		return
	}
}

func TestM5_StreamAuthHintsAndRevalidation(t *testing.T) {
	old := api.StreamRevalidateEvery
	api.StreamRevalidateEvery = 300 * time.Millisecond
	t.Cleanup(func() { api.StreamRevalidateEvery = old })
	e := newM5(t, func(v *api.V1) { v.StreamOrigins = []string{"https://console.example"} })
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() {
		(&api.Feed{Pool: e.env.Store.Pool, Hub: e.v1.Hub, Interval: 50 * time.Millisecond, Log: testenv.Discard}).Run(ctx)
	})
	t.Cleanup(func() { cancel(); wg.Wait() })

	op := e.login(500, "operator")
	// Unauthenticated / bad origin / token without jobs:read are refused before the upgrade.
	if _, resp, err := dialStream(t, srv, "", ""); err == nil || resp.StatusCode != 401 {
		t.Fatalf("anonymous upgrade: %v", err)
	}
	if _, resp, err := dialStream(t, srv, e.as(op), "https://evil.example"); err == nil || resp.StatusCode != 403 {
		t.Fatalf("bad origin: %v", err)
	}
	if _, resp, err := dialStream(t, srv, e.token(501, "operator", "metrics:read"), ""); err == nil || resp.StatusCode != 403 {
		t.Fatalf("token without jobs:read: %v", err)
	}
	// Session stream: hello, then a hint for a REST mutation (via the hub and the DB feed).
	c, _, err := dialStream(t, srv, e.as(op), "https://console.example")
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	hello, err := readMsg(t, c, 2*time.Second)
	if err != nil || hello["type"] != "hello" || hello["session_expires_at"] == nil {
		t.Fatalf("hello: %v %v", hello, err)
	}
	id := decode[map[string]any](t, e.want2(e.do("POST", "/api/v1/jobs", e.op, okJob), 201))["id"].(string)
	h := readUntil(t, c, 3*time.Second, func(m map[string]any) bool { return m["type"] == "hint" && m["id"] == id })
	if h["entity"] != "job" || h["version"].(float64) != 1 {
		t.Fatalf("hint: %v", h)
	}
	e.want(e.do("POST", "/api/v1/jobs/"+id+"/cancel", e.op, ""), 200, "")
	h = readUntil(t, c, 3*time.Second, func(m map[string]any) bool { return m["type"] == "hint" && m["id"] == id })
	if h["version"].(float64) != 2 {
		t.Fatalf("cancel hint: %v", h)
	}
	// Clients may only send ping.
	if err := c.Write(context.Background(), websocket.MessageText, []byte(`{"type":"ping"}`)); err != nil {
		t.Fatal(err)
	}

	// Token stream: revoking the token closes it 4401; losing jobs:read (owner demoted keeps reads) stays open.
	tokSecret := e.token(502, "operator", "jobs:read")
	tc, _, err := dialStream(t, srv, tokSecret, "")
	if err != nil {
		t.Fatal(err)
	}
	defer tc.CloseNow()
	if m, err := readMsg(t, tc, 2*time.Second); err != nil || m["type"] != "hello" {
		t.Fatalf("token hello: %v %v", m, err)
	}
	e.exec(`UPDATE api_tokens SET revoked_at = now() WHERE owner_user_id = (SELECT id FROM users WHERE github_user_id = 502)`)
	wantClose(t, tc, api.CloseSessionEnded, 3*time.Second)

	// At most 5 streams per session: the 6th closes 4429.
	var extra []*websocket.Conn
	for range 4 {
		x, _, err := dialStream(t, srv, e.as(op), "")
		if err != nil {
			t.Fatal(err)
		}
		extra = append(extra, x)
	}
	sixth, _, err := dialStream(t, srv, e.as(op), "")
	if err != nil {
		t.Fatal(err)
	}
	wantClose(t, sixth, api.CloseTooMany, 3*time.Second)
	for _, x := range extra {
		x.CloseNow()
	}

	// AC-13: logout closes the open stream 4401 and a reconnect with the old session is refused (401).
	e.want(e.do("DELETE", "/api/v1/sessions/current", e.as(op), ""), 204, "")
	wantClose(t, c, api.CloseSessionEnded, 3*time.Second)
	if _, resp, err := dialStream(t, srv, e.as(op), ""); err == nil || resp.StatusCode != 401 {
		t.Fatalf("reconnect after logout: %v", err)
	}
}

func TestM5_StreamSessionExpiry(t *testing.T) {
	oldW := api.StreamAuthWarning
	api.StreamAuthWarning = time.Second
	t.Cleanup(func() { api.StreamAuthWarning = oldW })
	e := newM5(t)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	s := e.login(600, "viewer")
	// Absolute expiry 2.5 s from now: auth_expiring ~1 s before, then 4401.
	e.exec(`UPDATE sessions SET expires_at = now() + interval '2500 milliseconds' WHERE id = $1`, s.sid)
	c, _, err := dialStream(t, srv, e.as(s), "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	readUntil(t, c, 3*time.Second, func(m map[string]any) bool { return m["type"] == "auth_expiring" && m["expires_at"] != nil })
	wantClose(t, c, api.CloseSessionEnded, 4*time.Second)
}

// AC-16 (server side): hints for one entity are forwarded only in increasing version order; a feed gap
// broadcasts resync{gap_detected}. Overflow is covered by TestHubOverflow (package api).
func TestM5_HubOrderingOverflowAndGap(t *testing.T) {
	e := newM5(t)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	hub := e.v1.Hub
	s := e.login(700, "viewer")
	c, _, err := dialStream(t, srv, e.as(s), "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if _, err := readMsg(t, c, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	const jid = "0193a0f2-7c1e-7a10-8000-0000000000a1"
	hub.PublishJob(jid, 3)
	hub.PublishJob(jid, 2) // reordered / stale: dropped
	hub.PublishJob(jid, 3) // duplicate: dropped
	hub.PublishJob(jid, 4)
	got := []float64{}
	for len(got) < 2 {
		m, err := readMsg(t, c, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if m["type"] == "hint" && m["id"] == jid {
			got = append(got, m["version"].(float64))
		}
	}
	if got[0] != 3 || got[1] != 4 {
		t.Fatalf("versions %v want [3 4]", got)
	}
	hub.Resync("gap_detected")
	readUntil(t, c, 5*time.Second, func(m map[string]any) bool { return m["type"] == "resync" && m["reason"] == "gap_detected" })
	// Metrics hints carry id null and version 0.
	b, _ := json.Marshal(api.Message{Type: "hint", Entity: "metrics", Version: new(int64)})
	if string(b) != `{"type":"hint","entity":"metrics","id":null,"version":0}` {
		t.Fatalf("metrics hint frame %s", b)
	}
	_ = auth.Issuer
}

func pad6(i int) string {
	s := strings.Repeat("0", 6) + itoa(i)
	return s[len(s)-6:]
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

// RV5R-04 / R07: the /stream endpoint subscribes with the principal's scopes, so a jobs:read token
// stream never receives a worker hint.
func TestStreamHintScopeWiring(t *testing.T) {
	e := newM5(t)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	c, _, err := dialStream(t, srv, e.token(601, "operator", "jobs:read"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if m, err := readMsg(t, c, 2*time.Second); err != nil || m["type"] != "hello" {
		t.Fatalf("hello: %v %v", m, err)
	}
	e.v1.Hub.PublishWorker("0193a0f2-0000-7000-8000-0000000000f1", 7)
	e.v1.Hub.PublishJob("0193a0f2-0000-7000-8000-0000000000f2", 1)
	m := readUntil(t, c, 3*time.Second, func(m map[string]any) bool { return m["type"] == "hint" })
	if m["entity"] != "job" {
		t.Fatalf("jobs:read stream received %v", m)
	}
}
