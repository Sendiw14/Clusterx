package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/api"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/migrate"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/pgtest"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/migrations"
)

const token = "test-scrape-token-0123456789abcdef0123"

func get(t *testing.T, h http.Handler, path string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) api.Error {
	t.Helper()
	var e api.Error
	dec := json.NewDecoder(rec.Body)
	dec.DisallowUnknownFields() // Error schema has additionalProperties: false
	if err := dec.Decode(&e); err != nil {
		t.Fatalf("error body is not the Error schema: %v", err)
	}
	if e.RequestID == "" || e.RequestID != rec.Header().Get(api.HeaderRequestID) {
		t.Fatalf("request_id %q does not match header %q", e.RequestID, rec.Header().Get(api.HeaderRequestID))
	}
	return e
}

func logger(buf *bytes.Buffer) *slog.Logger {
	if buf == nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return slog.New(slog.NewJSONHandler(buf, nil))
}

func TestLiveAndReadyWithFakeCheck(t *testing.T) {
	var readyErr error
	h := api.NewHandler(func(context.Context) error { return readyErr }, logger(nil), nil)

	if rec := get(t, h, "/health/live"); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("live: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, h, "/health/ready"); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"status":"ready"}` {
		t.Fatalf("ready: %d %s", rec.Code, rec.Body)
	}
	readyErr = errors.New("secret internal detail")
	rec := get(t, h, "/health/ready")
	if rec.Code != 503 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("not ready: %d headers=%v", rec.Code, rec.Header())
	}
	if strings.Contains(rec.Body.String(), "secret internal detail") {
		t.Fatal("readiness leaks internal error")
	}
	if e := decodeError(t, rec); e.Code != "unavailable" {
		t.Fatalf("code=%q", e.Code)
	}
}

func TestRequestID(t *testing.T) {
	var buf bytes.Buffer
	h := api.NewHandler(func(context.Context) error { return nil }, logger(&buf), nil)

	if got := get(t, h, "/health/live", api.HeaderRequestID, "client-abc.1:2").Header().Get(api.HeaderRequestID); got != "client-abc.1:2" {
		t.Fatalf("valid id not echoed: %q", got)
	}
	for _, bad := range []string{"has space", "a\x00b", strings.Repeat("x", 65), "ünï"} {
		got := get(t, h, "/health/live", api.HeaderRequestID, bad).Header().Get(api.HeaderRequestID)
		if got == bad || !strings.HasPrefix(got, "req_") || len(got) != 28 {
			t.Fatalf("bad id %q -> %q", bad, got)
		}
	}
	if got := get(t, h, "/health/live").Header().Get(api.HeaderRequestID); !strings.HasPrefix(got, "req_") {
		t.Fatalf("no generated id: %q", got)
	}
	// Access log is JSON with request_id and the route pattern (not the raw path).
	first, _, _ := strings.Cut(buf.String(), "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(first), &entry); err != nil {
		t.Fatalf("log line not JSON: %q", first)
	}
	if entry["request_id"] != "client-abc.1:2" || entry["route"] != "GET /health/live" || entry["status"] != float64(200) {
		t.Fatalf("log entry: %v", entry)
	}
}

func TestMetricsRequiresBearer(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := api.NewMetrics(reg)
	pub := api.NewHandler(func(context.Context) error { return nil }, logger(nil), m)
	get(t, pub, "/health/live")
	get(t, pub, "/no/such/route/123")

	h := api.NewMetricsHandler(reg, token, logger(nil), m)
	for name, hdr := range map[string][]string{
		"none":       nil,
		"wrong":      {"Authorization", "Bearer nope"},
		"prefix":     {"Authorization", "Bearer " + token[:10]},
		"basic":      {"Authorization", "Basic " + token},
		"no scheme":  {"Authorization", token},
		"lowercase":  {"Authorization", "bearer " + token},
		"extra tail": {"Authorization", "Bearer " + token + "x"},
	} {
		rec := get(t, h, "/metrics", hdr...)
		if rec.Code != 401 || !strings.HasPrefix(rec.Header().Get("WWW-Authenticate"), "Bearer") {
			t.Fatalf("%s: code=%d", name, rec.Code)
		}
		if e := decodeError(t, rec); e.Code != "unauthenticated" {
			t.Fatalf("%s: code=%q", name, e.Code)
		}
	}
	rec := get(t, h, "/metrics", "Authorization", "Bearer "+token)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `clusterx_http_requests_total{code="200",route="GET /health/live",server="api"} 1`) ||
		!strings.Contains(body, `route="unmatched"`) || strings.Contains(body, "/no/such/route") {
		t.Fatalf("metrics: %d\n%s", rec.Code, body)
	}

	disabled := api.NewMetricsHandler(reg, "", logger(nil), nil)
	for _, hdr := range [][]string{nil, {"Authorization", "Bearer "}, {"Authorization", "Bearer x"}} {
		if rec := get(t, disabled, "/metrics", hdr...); rec.Code != 401 {
			t.Fatalf("empty configured token must deny, got %d", rec.Code)
		}
	}
}

func TestMetricsBoundsConcurrentScrapes(t *testing.T) {
	g := struct{ entered, release chan struct{} }{make(chan struct{}, 1), make(chan struct{})}
	reg := prometheus.NewRegistry()
	// A collector that parks inside Gather until released (stands in for a slow DB-backed ops scrape).
	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "slow"}, func() float64 {
		g.entered <- struct{}{}
		<-g.release
		return 1
	}))
	h := api.NewMetricsHandler(reg, token, logger(nil), nil)
	done := make(chan int)
	go func() { done <- get(t, h, "/metrics", "Authorization", "Bearer "+token).Code }()
	<-g.entered // first scrape is inside Gather
	if rec := get(t, h, "/metrics", "Authorization", "Bearer "+token); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("second concurrent scrape: want 503, got %d", rec.Code)
	}
	close(g.release)
	if code := <-done; code != 200 {
		t.Fatalf("first scrape: %d", code)
	}
	go func() { <-g.entered }() // slot is free again
	if rec := get(t, h, "/metrics", "Authorization", "Bearer "+token); rec.Code != 200 {
		t.Fatalf("scrape after release: %d", rec.Code)
	}
}

func TestPanicIsContractError(t *testing.T) {
	h := api.NewHandler(func(context.Context) error { panic("boom") }, logger(nil), nil)
	rec := get(t, h, "/health/ready")
	if rec.Code != 500 {
		t.Fatalf("code=%d", rec.Code)
	}
	if e := decodeError(t, rec); e.Code != "internal" {
		t.Fatalf("code=%q", e.Code)
	}
}

// Readiness against real PostgreSQL 18: 503 before migrations, 200 after, 503 when the
// database is unreachable.
func TestReadyAgainstPostgres(t *testing.T) {
	dsn := pgtest.NewDB(t)
	ctx := context.Background()
	migs, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := store.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	h := api.NewHandler(api.DBReadiness(pool, migs), logger(nil), nil)

	if rec := get(t, h, "/health/ready"); rec.Code != 503 {
		t.Fatalf("unmigrated: want 503, got %d", rec.Code)
	}
	conn, err := store.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrate.Apply(ctx, conn, migs, logger(nil)); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close(ctx)
	if rec := get(t, h, "/health/ready"); rec.Code != 200 {
		t.Fatalf("migrated: want 200, got %d %s", rec.Code, rec.Body)
	}
	var sc, tz string
	if err := pool.QueryRow(ctx, `SELECT current_setting('statement_timeout'), current_setting('TimeZone')`).Scan(&sc, &tz); err != nil || sc != "5s" || tz != "UTC" {
		t.Fatalf("runtime params: statement_timeout=%q tz=%q err=%v", sc, tz, err)
	}

	// Database down: a pool pointed at a port with no listener.
	down, err := store.OpenPool(ctx, "postgres://clusterx@127.0.0.1:1/none?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer down.Close()
	hd := api.NewHandler(api.DBReadiness(down, migs), logger(nil), nil)
	rec := get(t, hd, "/health/ready")
	if rec.Code != 503 {
		t.Fatalf("db down: want 503, got %d", rec.Code)
	}
	decodeError(t, rec)
	if rec := get(t, hd, "/health/live"); rec.Code != 200 {
		t.Fatalf("live must not depend on DB, got %d", rec.Code)
	}
}
