package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/api"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/catalog"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
)

type v1env struct {
	t        *testing.T
	env      *testenv.Env
	h        http.Handler
	op, view string // operator / viewer tokens
}

func newV1(t *testing.T) *v1env {
	t.Helper()
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 10)
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	e := &v1env{t: t, env: env}
	e.h = e.handler(env.Store)
	e.op = e.token(1, "operator", "jobs:read", "jobs:write", "workers:read")
	e.view = e.token(2, "viewer", "jobs:read", "workers:read")
	_ = cat
	return e
}

func (e *v1env) handler(st *store.Store) http.Handler {
	cat, _ := catalog.Default()
	return api.NewHandler(func(context.Context) error { return nil }, logger(nil), nil, &api.V1{Store: st, Catalog: cat,
		Environment: "test", Version: "test", Commit: "0000000",
		Timing: api.ConfigTiming{HeartbeatInterval: 10 * time.Second, LivenessTimeout: 30 * time.Second, LeaseTTL: 30 * time.Second,
			CancelGrace: 15 * time.Second, DrainGrace: time.Minute}})
}

func (e *v1env) token(gh int64, role string, scopes ...string) string {
	e.t.Helper()
	c, err := e.env.Store.CreateAPIToken(context.Background(), store.TokenParams{OwnerGitHubID: gh, OwnerLogin: fmt.Sprint("u", gh),
		Role: role, Name: "t", Scopes: scopes, ExpiresInDays: 1})
	if err != nil {
		e.t.Fatal(err)
	}
	return c.Secret
}

func (e *v1env) do(method, path, token, body string, hdr ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func (e *v1env) want(rec *httptest.ResponseRecorder, status int, code string) api.Error {
	e.t.Helper()
	if rec.Code != status {
		e.t.Fatalf("status %d want %d: %s", rec.Code, status, rec.Body)
	}
	if code == "" {
		return api.Error{}
	}
	var er api.Error
	if err := json.Unmarshal(rec.Body.Bytes(), &er); err != nil || er.Code != code || er.RequestID == "" {
		e.t.Fatalf("error body %s want code %s", rec.Body, code)
	}
	return er
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("%v: %s", err, rec.Body)
	}
	return v
}

const okJob = `{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"abc"},"priority":5}`

func TestV1AuthMatrix(t *testing.T) {
	e := newV1(t)
	ctx := context.Background()
	revoked := e.token(3, "operator", "jobs:read")
	var id string
	if err := e.env.Store.Pool.QueryRow(ctx, `SELECT id FROM api_tokens WHERE prefix = $1`, revoked[:12]).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := e.env.Store.RevokeAPIToken(ctx, id); err != nil {
		t.Fatal(err)
	}
	disabled := e.token(4, "operator", "jobs:read")
	if _, err := e.env.Store.Pool.Exec(ctx, `UPDATE users SET disabled_at = now() WHERE github_user_id = 4`); err != nil {
		t.Fatal(err)
	}
	expired := e.token(5, "operator", "jobs:read")
	if _, err := e.env.Store.Pool.Exec(ctx, `UPDATE api_tokens SET created_at = now() - interval '2 days', expires_at = now() - interval '1 second' WHERE prefix = $1`, expired[:12]); err != nil {
		t.Fatal(err)
	}
	for tok, reason := range map[string]string{"": "token_invalid", "cxt_garbage": "token_invalid", revoked: "token_revoked",
		disabled: "user_disabled", expired: "token_expired", "cxw_" + revoked[4:]: "token_invalid"} {
		er := e.want(e.do("GET", "/api/v1/jobs", tok, ""), 401, "unauthenticated")
		if er.Details["reason"] != reason {
			t.Fatalf("token %.8q: reason %v want %s", tok, er.Details["reason"], reason)
		}
	}
	e.want(e.do("POST", "/api/v1/jobs", e.view, okJob), 403, "forbidden")
	readOnlyOp := e.token(6, "operator", "jobs:read")
	e.want(e.do("POST", "/api/v1/jobs", readOnlyOp, okJob), 403, "forbidden")
	e.want(e.do("GET", "/api/v1/workers", readOnlyOp, ""), 403, "forbidden")
	// Owner demoted to viewer: the operator token loses write scopes at once (RV01-02).
	demoted := e.token(7, "operator", "jobs:read", "jobs:write")
	if _, err := e.env.Store.Pool.Exec(ctx, `UPDATE users SET role = 'viewer' WHERE github_user_id = 7`); err != nil {
		t.Fatal(err)
	}
	e.want(e.do("POST", "/api/v1/jobs", demoted, okJob), 403, "forbidden")
	// Forged identity headers are ignored.
	e.want(e.do("GET", "/api/v1/jobs", "", "", "X-User", "admin", "X-Forwarded-User", "admin"), 401, "unauthenticated")
	// Typed errors for unknown routes and methods (M3-2).
	e.want(e.do("GET", "/api/v1/nope", e.op, ""), 404, "not_found_route")
	rec := e.do("DELETE", "/api/v1/jobs", e.op, "")
	e.want(rec, 405, "method_not_allowed")
	if rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("Allow: %q", rec.Header().Get("Allow"))
	}
	e.want(e.do("POST", "/health/live", "", ""), 405, "method_not_allowed")
	// M5 routes are not served.
	e.want(e.do("POST", "/api/v1/jobs/0193a0f2-7c1e-7a10-8000-0000000000a1/cancel", e.op, ""), 404, "not_found_route")
}

func TestV1CreateValidationAndIdempotency(t *testing.T) {
	e := newV1(t)
	rec := e.do("POST", "/api/v1/jobs", e.op, okJob)
	e.want(rec, 201, "")
	j := decode[map[string]any](t, rec)
	if j["status"] != "queued" || rec.Header().Get("Location") != "/api/v1/jobs/"+j["id"].(string) || j["timeout_seconds"] != float64(30) {
		t.Fatalf("created: %v", j)
	}
	for name, c := range map[string]struct {
		body   string
		status int
		code   string
		field  string
		rule   string
	}{
		"bad json":      {`{"handler_name":`, 400, "bad_request", "", ""},
		"not object":    {`[1]`, 400, "bad_request", "", ""},
		"unknown field": {`{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"x"},"shell":"ls"}`, 422, "validation_failed", "/shell", "unknown_field"},
		"priority":      {`{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"x"},"priority":10}`, 422, "validation_failed", "/priority", "range"},
		"max_attempts":  {`{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"x"},"max_attempts":6}`, 422, "validation_failed", "/max_attempts", "range"},
		"unregistered":  {`{"handler_name":"thumbnail.render","handler_version":7,"payload":{}}`, 422, "validation_failed", "/handler_name", "registered"},
		"schema":        {`{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":"x"}}`, 422, "validation_failed", "/payload/ms", "schema"},
		"schema extra":  {`{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":1,"url":"http://169.254.169.254/"}}`, 422, "validation_failed", "/payload/url", "schema"},
		"missing":       {`{"handler_version":1,"payload":{}}`, 422, "validation_failed", "/handler_name", "required"},
		"run_after":     {`{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":1},"run_after":"` + time.Now().Add(31*24*time.Hour).UTC().Format(time.RFC3339) + `"}`, 422, "validation_failed", "/run_after", "too_far_in_future"},
		"nul":           {`{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"a\u0000b"}}`, 422, "validation_failed", "/payload", "format"},
		"payload 64KiB": {`{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"` + strings.Repeat("a", 65536) + `"}}`, 413, "payload_too_large", "", ""},
		"body 80KiB":    {`{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"` + strings.Repeat("a", 90000) + `"}}`, 413, "payload_too_large", "", ""},
		"deep":          {`{"handler_name":"hash_sha256","handler_version":1,"payload":` + strings.Repeat(`[`, 40) + strings.Repeat(`]`, 40) + `}`, 422, "validation_failed", "/payload", "range"},
	} {
		er := e.want(e.do("POST", "/api/v1/jobs", e.op, c.body), c.status, c.code)
		if c.field != "" {
			f := er.Details["fields"].([]any)[0].(map[string]any)
			if f["field"] != c.field || f["rule"] != c.rule {
				t.Fatalf("%s: %v", name, er.Details)
			}
		}
	}
	e.want(e.do("POST", "/api/v1/jobs", e.op, okJob, "Idempotency-Key", "bad key!"), 422, "validation_failed")
	req := httptest.NewRequest("POST", "/api/v1/jobs", strings.NewReader(okJob))
	req.Header.Set("Authorization", "Bearer "+e.op)
	rec = httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	e.want(rec, 400, "bad_request") // no Content-Type

	// AC-07 over HTTP: same key + semantically equal body (other key order, whitespace) replays.
	r1 := e.do("POST", "/api/v1/jobs", e.op, okJob, "Idempotency-Key", "order-42")
	e.want(r1, 201, "")
	r2 := e.do("POST", "/api/v1/jobs", e.op, `{ "priority": 5, "payload": {"data": "abc"}, "handler_version": 1, "handler_name": "hash_sha256" }`, "Idempotency-Key", "order-42")
	e.want(r2, 200, "")
	if r2.Header().Get("Idempotent-Replayed") != "true" || decode[map[string]any](t, r2)["id"] != decode[map[string]any](t, r1)["id"] {
		t.Fatalf("replay: %s", r2.Body)
	}
	er := e.want(e.do("POST", "/api/v1/jobs", e.op, strings.Replace(okJob, `"abc"`, `"abd"`, 1), "Idempotency-Key", "order-42"), 409, "idempotency_conflict")
	if er.Details["job_id"] != decode[map[string]any](t, r1)["id"] {
		t.Fatalf("conflict details: %v", er.Details)
	}
}

// AC-01: 2xx only after commit (a failed commit acknowledges nothing); an acknowledged job survives a
// coordinator restart (new pool, new handler).
func TestAC01_AckOnlyAfterCommitAndRestart(t *testing.T) {
	e := newV1(t)
	e.env.Store.Failpoint = func(n string) error {
		if n == "admit.before_commit" {
			return fmt.Errorf("%w: injected", store.ErrUnavailable)
		}
		return nil
	}
	rec := e.do("POST", "/api/v1/jobs", e.op, okJob, "Idempotency-Key", "k1")
	e.want(rec, 503, "unavailable")
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("no Retry-After")
	}
	var n int
	if err := e.env.Store.Pool.QueryRow(context.Background(), `SELECT count(*) FROM jobs`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("jobs after failed commit: %d %v", n, err)
	}
	e.env.Store.Failpoint = func(string) error { return errors.New("boom") }
	e.want(e.do("POST", "/api/v1/jobs", e.op, okJob), 500, "internal")
	e.env.Store.Failpoint = nil
	rec = e.do("POST", "/api/v1/jobs", e.op, okJob, "Idempotency-Key", "k1")
	e.want(rec, 201, "")
	id := decode[map[string]any](t, rec)["id"].(string)
	// "Restart": a brand-new pool, store and handler against the same database.
	pool := testenv.OpenPool(t, e.env.DSN, 4)
	e.env.Store.Pool.Close()
	cat, _ := catalog.Default()
	e.h = e.handler(store.New(pool, testenv.FastTiming(30*time.Second), cat))
	rec = e.do("GET", "/api/v1/jobs/"+id, e.op, "")
	e.want(rec, 200, "")
	if decode[map[string]any](t, rec)["status"] != "queued" {
		t.Fatalf("after restart: %s", rec.Body)
	}
	e.want(e.do("POST", "/api/v1/jobs", e.op, okJob, "Idempotency-Key", "k1"), 200, "")
}

func TestV1ReadsMaskingPaginationExpiry(t *testing.T) {
	e := newV1(t)
	ctx := context.Background()
	var ids []string
	for i := range 5 {
		rec := e.do("POST", "/api/v1/jobs", e.op, fmt.Sprintf(`{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"%d"},"priority":%d}`, i, i))
		e.want(rec, 201, "")
		ids = append(ids, decode[map[string]any](t, rec)["id"].(string))
	}
	// Operator vs viewer masking.
	op := decode[map[string]any](t, e.do("GET", "/api/v1/jobs/"+ids[0], e.op, ""))
	vw := e.do("GET", "/api/v1/jobs/"+ids[0], e.view, "")
	e.want(vw, 200, "")
	v := decode[map[string]any](t, vw)
	if op["payload"] == nil || len(op["masked_fields"].([]any)) != 0 || v["payload"] != nil || len(v["masked_fields"].([]any)) != 3 ||
		vw.Header().Get("ETag") != `W/"1"` || v["unschedulable_reason"] != "no_capable_worker" {
		t.Fatalf("masking: op=%v viewer=%v etag=%s", op, v, vw.Header().Get("ETag"))
	}
	// Pagination: 5 jobs, limit 2, default sort -created_at; then -priority.
	for _, sort := range []string{"", "-priority", "created_at"} {
		seen := map[string]bool{}
		path := "/api/v1/jobs?limit=2"
		if sort != "" {
			path += "&sort=" + sort
		}
		var order []float64
		for page := 0; ; page++ {
			rec := e.do("GET", path, e.view, "")
			e.want(rec, 200, "")
			body := decode[map[string]any](t, rec)
			for _, it := range body["items"].([]any) {
				m := it.(map[string]any)
				if seen[m["id"].(string)] {
					t.Fatalf("duplicate %s", m["id"])
				}
				seen[m["id"].(string)] = true
				order = append(order, m["priority"].(float64))
				if _, has := m["payload"]; has {
					t.Fatal("summary carries payload")
				}
			}
			next, _ := body["next_cursor"].(string)
			if next == "" {
				break
			}
			if page == 0 {
				e.want(e.do("GET", path+"&status=queued&cursor="+next, e.view, ""), 400, "invalid_cursor")
			}
			path = strings.SplitN(path, "&cursor=", 2)[0] + "&cursor=" + next
		}
		if len(seen) != 5 {
			t.Fatalf("sort %q: saw %d", sort, len(seen))
		}
		if sort == "-priority" && fmt.Sprint(order) != "[4 3 2 1 0]" {
			t.Fatalf("priority order %v", order)
		}
	}
	e.want(e.do("GET", "/api/v1/jobs?status=bogus&limit=500", e.view, ""), 422, "validation_failed")
	e.want(e.do("GET", "/api/v1/jobs?created_after=2026-01-01T00:00:00Z&created_before=2026-03-01T00:00:00Z", e.view, ""), 422, "validation_failed")
	e.want(e.do("GET", "/api/v1/jobs?cursor=garbage", e.view, ""), 400, "invalid_cursor")
	// Attempts and events.
	at := decode[map[string]any](t, e.do("GET", "/api/v1/jobs/"+ids[0]+"/attempts", e.view, ""))
	if len(at["items"].([]any)) != 0 || at["job_version"] != float64(1) {
		t.Fatalf("attempts %v", at)
	}
	ev := decode[map[string]any](t, e.do("GET", "/api/v1/jobs/"+ids[0]+"/events?after_version=0", e.view, ""))
	if len(ev["items"].([]any)) != 1 || ev["has_more"] != false {
		t.Fatalf("events %v", ev)
	}
	// 404 vs 410.
	e.want(e.do("GET", "/api/v1/jobs/0193a0f2-7c1e-7a10-8000-0000000000a1", e.op, ""), 404, "job_not_found")
	e.want(e.do("GET", "/api/v1/jobs/not-a-uuid", e.op, ""), 404, "job_not_found")
	if _, err := e.env.Store.Pool.Exec(ctx, `INSERT INTO job_tombstones (job_id, terminal_at) VALUES ('0193a0f2-7c1e-7a10-8000-0000000000b2', now() - interval '31 days')`); err != nil {
		t.Fatal(err)
	}
	e.want(e.do("GET", "/api/v1/jobs/0193a0f2-7c1e-7a10-8000-0000000000b2/events", e.op, ""), 410, "expired")
	// Workers, handlers, config.
	cred, _ := e.env.Store.CreateWorkerCredential(ctx, "w", 0)
	credID, _ := e.env.Store.AuthenticateWorker(ctx, cred.Secret)
	reg, err := e.env.Store.Register(ctx, credID, store.RegisterParams{IncarnationID: store.NewUUID(), Name: "w1", Hostname: "host-a",
		ProtocolVersion: 1, Capacity: 2, Capabilities: []store.Capability{{Name: "hash_sha256", Version: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	wl := decode[map[string]any](t, e.do("GET", "/api/v1/workers?liveness=live", e.view, ""))
	w0 := wl["items"].([]any)[0].(map[string]any)
	if w0["id"] != reg.WorkerID || w0["hostname"] != nil || w0["live"] != true {
		t.Fatalf("workers viewer: %v", w0)
	}
	wd := decode[map[string]any](t, e.do("GET", "/api/v1/workers/"+reg.WorkerID, e.op, ""))
	if wd["hostname"] != "host-a" || wd["active"] == nil || wd["as_of"] == nil {
		t.Fatalf("worker detail: %v", wd)
	}
	e.want(e.do("GET", "/api/v1/workers/0193a0f2-7c1e-7a10-8000-0000000000a1", e.op, ""), 404, "worker_not_found")
	hs := decode[map[string]any](t, e.do("GET", "/api/v1/handlers", e.view, ""))
	if len(hs["items"].([]any)) != 3 {
		t.Fatalf("handlers %v", hs)
	}
	cfg := decode[map[string]any](t, e.do("GET", "/api/v1/config", e.view, ""))
	if cfg["api_version"] != api.APIVersion {
		t.Fatalf("config %v", cfg)
	}
	if v2 := decode[map[string]any](t, e.do("GET", "/api/v1/jobs/"+ids[0], e.view, "")); v2["unschedulable_reason"] != nil {
		t.Fatalf("with a live capable worker: %v", v2["unschedulable_reason"])
	}
}
