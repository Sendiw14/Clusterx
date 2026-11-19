package api_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/api"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/auth"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/catalog"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
)

// m5env is a v1env whose handler also verifies BFF assertions (test key) and serves the M5 routes.
type m5env struct {
	*v1env
	priv ed25519.PrivateKey
	v1   *api.V1
}

func newM5(t *testing.T, tweak ...func(*api.V1)) *m5env {
	t.Helper()
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 12)
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Default()
	v := &api.V1{Store: env.Store, Catalog: cat, Environment: "test", Version: "test", Commit: "0000000", Log: testenv.Discard,
		Timing: api.ConfigTiming{HeartbeatInterval: 10 * time.Second, LivenessTimeout: 30 * time.Second, LeaseTTL: 30 * time.Second,
			CancelGrace: 15 * time.Second, DrainGrace: time.Minute},
		Verifier: auth.NewVerifier(map[string]ed25519.PublicKey{"k1": priv.Public().(ed25519.PublicKey)}),
		Sessions: store.DefaultSessionTiming(), Hub: api.NewHub(testenv.Discard)}
	for _, f := range tweak {
		f(v)
	}
	e := &m5env{v1env: &v1env{t: t, env: env}, priv: priv, v1: v}
	e.h = api.NewHandler(func(context.Context) error { return nil }, testenv.Discard, nil, v)
	e.op = e.token(1, "operator", "jobs:read", "jobs:write", "workers:read", "workers:write", "audit:read", "metrics:read")
	e.view = e.token(2, "viewer", "jobs:read", "workers:read", "audit:read", "metrics:read")
	return e
}

type sess struct {
	userID, sid, secret, role string
}

// login provisions an allowlisted user and creates a session through POST /api/v1/sessions.
func (e *m5env) login(gh int64, role string) sess {
	e.t.Helper()
	if _, err := e.env.Store.UpsertUser(context.Background(), gh, fmt.Sprint("u", gh), role); err != nil {
		e.t.Fatal(err)
	}
	rec := e.do("POST", "/api/v1/sessions", e.mint(auth.Claims{Sub: fmt.Sprint("github:", gh), Purpose: "login"}),
		fmt.Sprintf(`{"github_login":"user%d"}`, gh))
	e.want(rec, 201, "")
	if rec.Header().Get("Cache-Control") != "no-store" {
		e.t.Fatal("session create must be no-store")
	}
	b := decode[map[string]any](e.t, rec)
	return sess{userID: b["user"].(map[string]any)["id"].(string), sid: b["session_id"].(string), secret: b["secret"].(string), role: b["role"].(string)}
}

func (e *m5env) mint(c auth.Claims) string {
	e.t.Helper()
	s, err := auth.Mint("k1", e.priv, c, time.Now())
	if err != nil {
		e.t.Fatal(err)
	}
	return s
}

// as mints a fresh normal assertion for s (role hint = role).
func (e *m5env) as(s sess) string {
	return e.mint(auth.Claims{Sub: "user:" + s.userID, Sid: s.sid, Role: s.role})
}

func (e *m5env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.env.Store.Pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *m5env) q(sql string, args ...any) string {
	e.t.Helper()
	var s string
	if err := e.env.Store.Pool.QueryRow(context.Background(), sql, args...).Scan(&s); err != nil {
		e.t.Fatalf("%s: %v", sql, err)
	}
	return s
}

func (e *m5env) reason(rec *httptest.ResponseRecorder, status int, code, reason string) {
	e.t.Helper()
	er := e.want(rec, status, code)
	if er.Details["reason"] != reason {
		e.t.Fatalf("reason %v want %s (%s)", er.Details["reason"], reason, rec.Body)
	}
}

func TestM5_SessionsLifecycle(t *testing.T) {
	e := newM5(t)
	op := e.login(100, "operator")
	if len(op.secret) != 4+43 || !strings.HasPrefix(op.secret, "cxs_") {
		t.Fatalf("secret shape %q", op.secret[:6])
	}
	if got := e.q(`SELECT count(*)::text FROM sessions WHERE secret_hash = sha256(convert_to($1, 'UTF8'))`, op.secret); got != "1" {
		t.Fatal("session secret hash not stored")
	}
	if got := e.q(`SELECT count(*)::text FROM audit_events WHERE action = 'session.create' AND outcome = 'allowed' AND category = 'security' AND details->>'kid' = 'k1'`); got != "1" {
		t.Fatalf("session.create audit rows = %s", got)
	}
	me := decode[map[string]any](t, e.do("GET", "/api/v1/me", e.as(op), ""))
	if me["role"] != "operator" || me["session"].(map[string]any)["id"] != op.sid {
		t.Fatalf("me: %v", me)
	}
	// Not allowlisted / disabled / wrong credential types.
	e.reason(e.do("POST", "/api/v1/sessions", e.mint(auth.Claims{Sub: "github:999", Purpose: "login"}), `{"github_login":"x"}`), 403, "forbidden", "not_allowlisted")
	e.login(101, "viewer")
	e.exec(`UPDATE users SET disabled_at = now() WHERE github_user_id = 101`)
	e.reason(e.do("POST", "/api/v1/sessions", e.mint(auth.Claims{Sub: "github:101", Purpose: "login"}), `{"github_login":"x"}`), 403, "forbidden", "user_disabled")
	e.reason(e.do("POST", "/api/v1/sessions", e.op, `{"github_login":"x"}`), 403, "forbidden", "credential_not_allowed")
	e.reason(e.do("POST", "/api/v1/sessions", e.as(op), `{"github_login":"x"}`), 403, "forbidden", "credential_not_allowed")
	e.reason(e.do("GET", "/api/v1/me", e.op, ""), 403, "forbidden", "credential_not_allowed")
	// A login assertion is single-use and valid nowhere else.
	login := e.mint(auth.Claims{Sub: "github:100", Purpose: "login"})
	e.want(e.do("POST", "/api/v1/sessions", login, `{"github_login":"again"}`), 201, "")
	e.reason(e.do("POST", "/api/v1/sessions", login, `{"github_login":"again"}`), 401, "unauthenticated", "assertion_invalid")
	e.reason(e.do("GET", "/api/v1/me", e.mint(auth.Claims{Sub: "github:100", Purpose: "login"}), ""), 401, "unauthenticated", "assertion_invalid")

	// RV01-11: an assertion naming another user's sid is invalid (and audited).
	other := e.login(102, "viewer")
	e.reason(e.do("GET", "/api/v1/me", e.mint(auth.Claims{Sub: "user:" + other.userID, Sid: op.sid, Role: "viewer"}), ""), 401, "unauthenticated", "assertion_invalid")
	if e.q(`SELECT count(*)::text FROM audit_events WHERE action = 'assertion.sid_mismatch'`) != "1" {
		t.Fatal("sid mismatch not audited")
	}
	// Role hint is never trusted above the DB role: a viewer claiming operator stays a viewer.
	forged := e.mint(auth.Claims{Sub: "user:" + other.userID, Sid: other.sid, Role: "operator"})
	e.want(e.do("POST", "/api/v1/jobs", forged, okJob), 403, "forbidden")
	if e.q(`SELECT count(*)::text FROM audit_events WHERE action = 'assertion.role_mismatch'`) != "1" {
		t.Fatal("role mismatch not audited")
	}
	// Mutations consume the jti; reads may reuse an assertion until exp.
	a := e.as(op)
	e.want(e.do("POST", "/api/v1/jobs", a, okJob), 201, "")
	e.reason(e.do("POST", "/api/v1/jobs", a, okJob), 401, "unauthenticated", "assertion_invalid")
	r := e.as(op)
	e.want(e.do("GET", "/api/v1/jobs", r, ""), 200, "")
	e.want(e.do("GET", "/api/v1/jobs", r, ""), 200, "")
	// Expired, wrong audience, wrong key.
	e.reason(e.do("GET", "/api/v1/me", e.mint(auth.Claims{Sub: "user:" + op.userID, Sid: op.sid, Role: "operator",
		Iat: time.Now().Unix() - 80, Exp: time.Now().Unix() - 20}), ""), 401, "unauthenticated", "assertion_expired")
	e.reason(e.do("GET", "/api/v1/me", e.mint(auth.Claims{Aud: "x", Sub: "user:" + op.userID, Sid: op.sid, Role: "operator"}), ""), 401, "unauthenticated", "assertion_invalid")
	_, otherKey, _ := ed25519.GenerateKey(rand.Reader)
	bad, _ := auth.Mint("k1", otherKey, auth.Claims{Sub: "user:" + op.userID, Sid: op.sid, Role: "operator"}, time.Now())
	e.reason(e.do("GET", "/api/v1/me", bad, ""), 401, "unauthenticated", "assertion_invalid")

	// Idle and absolute expiry enforced in Go/DB (GAP-SESS-01).
	idle := e.login(103, "viewer")
	e.exec(`UPDATE sessions SET created_at = now() - interval '3 hours', last_activity_at = now() - interval '2 hours 1 second' WHERE id = $1`, idle.sid)
	er := e.want(e.do("GET", "/api/v1/me", e.as(idle), ""), 401, "unauthenticated")
	if er.Details["reason"] != "session_expired" || er.Details["expiry"] != "idle" {
		t.Fatalf("idle: %v", er.Details)
	}
	abs := e.login(104, "viewer")
	e.exec(`UPDATE sessions SET created_at = now() - interval '13 hours', last_activity_at = now(), expires_at = now() - interval '1 second' WHERE id = $1`, abs.sid)
	er = e.want(e.do("GET", "/api/v1/me", e.as(abs), ""), 401, "unauthenticated")
	if er.Details["reason"] != "session_expired" || er.Details["expiry"] != "absolute" {
		t.Fatalf("absolute: %v", er.Details)
	}
	// Activity bump at most once per 60 s.
	act := e.login(105, "viewer")
	e.exec(`UPDATE sessions SET created_at = now() - interval '10 minutes', last_activity_at = now() - interval '5 minutes' WHERE id = $1`, act.sid)
	e.want(e.do("GET", "/api/v1/me", e.as(act), ""), 200, "")
	if e.q(`SELECT (last_activity_at > now() - interval '5 seconds')::text FROM sessions WHERE id = $1`, act.sid) != "true" {
		t.Fatal("last_activity_at not bumped")
	}
	// Disabled user's live session ends at once.
	e.exec(`UPDATE users SET disabled_at = now() WHERE github_user_id = 105`)
	e.reason(e.do("GET", "/api/v1/me", e.as(act), ""), 401, "unauthenticated", "user_disabled")
	// Logout: next request 401 session_revoked; audited.
	rec := e.do("DELETE", "/api/v1/sessions/current", e.as(op), "")
	e.want(rec, 204, "")
	e.reason(e.do("GET", "/api/v1/me", e.as(op), ""), 401, "unauthenticated", "session_revoked")
	if e.q(`SELECT count(*)::text FROM audit_events WHERE action = 'session.revoke' AND target_id = $1`, op.sid) != "1" {
		t.Fatal("logout not audited")
	}
}

func TestM5_CancelRetryDrain(t *testing.T) {
	e := newM5(t)
	ctx := context.Background()
	op := e.login(200, "operator")
	vw := e.login(201, "viewer")
	submit := func(body string) string {
		return decode[map[string]any](t, e.want2(e.do("POST", "/api/v1/jobs", e.op, body), 201))["id"].(string)
	}
	// Pending cancel: 200 canceled, repeat 200 no-op, audit exactly once.
	id := submit(okJob)
	e.want(e.do("POST", "/api/v1/jobs/"+id+"/cancel", e.as(vw), ""), 403, "forbidden")
	e.want(e.do("POST", "/api/v1/jobs/"+id+"/cancel", e.view, ""), 403, "forbidden")
	if e.q(`SELECT count(*)::text FROM audit_events WHERE action = 'authz.denied' AND category = 'security' AND outcome = 'denied'`) != "2" {
		t.Fatal("denied mutations not audited")
	}
	j := decode[map[string]any](t, e.want2(e.do("POST", "/api/v1/jobs/"+id+"/cancel", e.as(op), ""), 200))
	if j["status"] != "canceled" || j["failure_class"] != "canceled_by_operator" {
		t.Fatalf("cancel: %v", j)
	}
	e.want(e.do("POST", "/api/v1/jobs/"+id+"/cancel", e.op, ""), 200, "")
	if e.q(`SELECT count(*)::text FROM audit_events WHERE action = 'job.cancel' AND target_id = $1`, id) != "1" {
		t.Fatal("job.cancel audit count")
	}
	// Running cancel: 202, cancel_requested_at set; the worker's success can no longer commit (AC-06 via REST).
	w := newRefWorker(t, e.env.Store)
	run := submit(`{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":10}}`)
	att := w.claim(t, run)
	j = decode[map[string]any](t, e.want2(e.do("POST", "/api/v1/jobs/"+run+"/cancel", e.op, ""), 202))
	if j["status"] != "running" || j["cancel_requested_at"] == nil {
		t.Fatalf("running cancel: %v", j)
	}
	rep, err := e.env.Store.ReportSuccess(ctx, att, json.RawMessage(`{"slept_ms":10}`), "")
	if err != nil || rep.Outcome != store.OutcomeCanceled {
		t.Fatalf("late success after REST cancel: %+v %v", rep, err)
	}
	if e.q(`SELECT status FROM jobs WHERE id = $1`, run) != "canceled" {
		t.Fatal("job not canceled")
	}
	// Terminal success: 409 job_terminal with {status, version}; success never rolled back.
	done := submit(okJob)
	w.succeed(t, w.claim(t, done))
	er := e.want(e.do("POST", "/api/v1/jobs/"+done+"/cancel", e.op, ""), 409, "job_terminal")
	if er.Details["status"] != "succeeded" || er.Details["version"] == nil || er.Details["terminal_at"] == nil {
		t.Fatalf("409 details: %v", er.Details)
	}
	// Manual retry: only failed; new linked job; idempotent key; duplicate refused.
	e.want(e.do("POST", "/api/v1/jobs/"+done+"/retry", e.op, ""), 409, "retry_not_allowed")
	failed := submit(`{"handler_name":"fail_n_times","handler_version":1,"payload":{"n":1,"permanent":true},"max_attempts":1}`)
	w.fail(t, w.claim(t, failed))
	e.want(e.do("POST", "/api/v1/jobs/"+failed+"/retry", e.view, ""), 403, "forbidden")
	rec := e.do("POST", "/api/v1/jobs/"+failed+"/retry", e.as(op), "", "Idempotency-Key", "r-1")
	child := decode[map[string]any](t, e.want2(rec, 201))
	if child["retried_from_job_id"] != failed || child["status"] != "queued" || child["attempts_used"].(float64) != 0 {
		t.Fatalf("child: %v", child)
	}
	rec = e.do("POST", "/api/v1/jobs/"+failed+"/retry", e.op, "", "Idempotency-Key", "r-1")
	// Same key but another principal (token vs session): not a replay of that principal's key.
	e.want(rec, 409, "retry_already_exists")
	rec = e.do("POST", "/api/v1/jobs/"+failed+"/retry", e.as(op), "", "Idempotency-Key", "r-1")
	if rec.Code != 200 || rec.Header().Get("Idempotent-Replayed") != "true" || decode[map[string]any](t, rec)["id"] != child["id"] {
		t.Fatalf("replay: %d %s", rec.Code, rec.Body)
	}
	er = e.want(e.do("POST", "/api/v1/jobs/"+failed+"/retry", e.as(op), "", "Idempotency-Key", "r-2"), 409, "retry_already_exists")
	if er.Details["retried_as_job_id"] != child["id"] || er.Details["status"] != "failed" {
		t.Fatalf("exists details: %v", er.Details)
	}
	if e.q(`SELECT string_agg(event_type, ',' ORDER BY version) FROM job_events WHERE job_id = $1`, child["id"]) != "retry_created" {
		t.Fatal("child events")
	}
	if e.q(`SELECT count(*)::text FROM audit_events WHERE action = 'job.manual_retry' AND target_id = $1`, failed) != "1" {
		t.Fatal("manual retry audit")
	}
	e.want(e.do("POST", "/api/v1/jobs/0193a0f2-7c1e-7a10-8000-0000000000aa/retry", e.op, ""), 404, "job_not_found")

	// Drain: 202 draining; repeat keeps the deadline; claims denied; deregistered 409.
	path := "/api/v1/workers/" + w.ref.WorkerID + "/drain"
	e.want(e.do("POST", path, e.view, `{"grace_seconds":5}`), 403, "forbidden")
	e.want(e.do("POST", path, e.op, `{"grace_seconds":9999}`), 422, "validation_failed")
	e.want(e.do("POST", path, e.op, `{"nope":1}`), 422, "validation_failed")
	d1 := decode[map[string]any](t, e.want2(e.do("POST", path, e.op, `{"grace_seconds":5}`), 202))
	d2 := decode[map[string]any](t, e.want2(e.do("POST", path, e.op, ""), 202))
	if d1["status"] != "draining" || d1["drain_deadline_at"] != d2["drain_deadline_at"] {
		t.Fatalf("drain: %v / %v", d1, d2)
	}
	if _, err := e.env.Store.Claim(ctx, w.ref, 1, []store.Capability{{Name: "hash_sha256", Version: 1}}); !store.IsDenial(err, "worker_draining") {
		t.Fatalf("claim while draining: %v", err)
	}
	if _, err := e.env.Store.Deregister(ctx, w.ref); err != nil {
		t.Fatal(err)
	}
	e.want(e.do("POST", path, e.op, ""), 409, "worker_deregistered")
	e.want(e.do("POST", "/api/v1/workers/0193a0f2-7c1e-7a10-8000-0000000000aa/drain", e.op, ""), 404, "worker_not_found")
	if e.q(`SELECT count(*)::text FROM audit_events WHERE action = 'worker.drain'`) != "1" {
		t.Fatal("drain audit")
	}
}

// want2 asserts a status and returns the recorder.
func (e *v1env) want2(rec *httptest.ResponseRecorder, status int) *httptest.ResponseRecorder {
	e.t.Helper()
	if rec.Code != status {
		e.t.Fatalf("status %d want %d: %s", rec.Code, status, rec.Body)
	}
	return rec
}

func TestM5_TokensAndWorkerCredentials(t *testing.T) {
	e := newM5(t)
	op := e.login(300, "operator")
	vw := e.login(301, "viewer")
	body := `{"name":"ci","role":"operator","scopes":["jobs:read","jobs:write"],"expires_in_days":30}`
	e.reason(e.do("POST", "/api/v1/tokens", e.op, body), 403, "forbidden", "credential_not_allowed")
	e.want(e.do("POST", "/api/v1/tokens", e.as(vw), body), 403, "forbidden")
	e.want(e.do("GET", "/api/v1/tokens", e.as(vw), ""), 403, "forbidden")
	e.want(e.do("POST", "/api/v1/tokens", e.as(op), `{"name":"v","role":"viewer","scopes":["jobs:write"]}`), 422, "validation_failed")
	rec := e.do("POST", "/api/v1/tokens", e.as(op), body, "Idempotency-Key", "tok-1")
	created := decode[map[string]any](t, e.want2(rec, 201))
	secret, _ := created["secret"].(string)
	if !strings.HasPrefix(secret, "cxt_") || rec.Header().Get("Cache-Control") != "no-store" || created["owner_user_id"] != op.userID {
		t.Fatalf("created: %v", created)
	}
	e.want(e.do("POST", "/api/v1/jobs", secret, okJob), 201, "")
	// Replay: 200 without the secret; different body with the same key: 409.
	rec = e.do("POST", "/api/v1/tokens", e.as(op), body, "Idempotency-Key", "tok-1")
	replay := decode[map[string]any](t, e.want2(rec, 200))
	if rec.Header().Get("Idempotent-Replayed") != "true" || replay["id"] != created["id"] || replay["secret"] != nil {
		t.Fatalf("replay: %v", replay)
	}
	e.want(e.do("POST", "/api/v1/tokens", e.as(op), strings.Replace(body, `"ci"`, `"cd"`, 1), "Idempotency-Key", "tok-1"), 409, "idempotency_conflict")
	// List never contains secrets or hashes.
	rec = e.do("GET", "/api/v1/tokens?limit=1", e.as(op), "")
	if strings.Contains(rec.Body.String(), "cxt_") && strings.Contains(rec.Body.String(), secret[13:]) || strings.Contains(rec.Body.String(), "hash") {
		t.Fatal("token list leaks secret material")
	}
	page := decode[map[string]any](t, e.want2(rec, 200))
	if len(page["items"].([]any)) != 1 || page["next_cursor"] == nil {
		t.Fatalf("token page: %v", page)
	}
	e.want(e.do("GET", "/api/v1/tokens?limit=1&cursor="+page["next_cursor"].(string), e.as(op), ""), 200, "")
	// Owner authority (RV01-02): an operator token of an owner demoted to viewer cannot write.
	e.exec(`UPDATE users SET role = 'viewer' WHERE github_user_id = 300`)
	e.want(e.do("POST", "/api/v1/jobs", secret, okJob), 403, "forbidden")
	e.want(e.do("POST", "/api/v1/tokens", e.mint(auth.Claims{Sub: "user:" + op.userID, Sid: op.sid, Role: "viewer"}), body), 403, "forbidden")
	e.exec(`UPDATE users SET role = 'operator' WHERE github_user_id = 300`)
	// Revoke: immediate 401 token_revoked; idempotent; audited once.
	path := "/api/v1/tokens/" + created["id"].(string) + "/revoke"
	r1 := decode[map[string]any](t, e.want2(e.do("POST", path, e.as(op), ""), 200))
	r2 := decode[map[string]any](t, e.want2(e.do("POST", path, e.as(op), ""), 200))
	if r1["revoked_at"] == nil || r1["revoked_at"] != r2["revoked_at"] {
		t.Fatalf("revoke: %v %v", r1, r2)
	}
	e.reason(e.do("GET", "/api/v1/jobs", secret, ""), 401, "unauthenticated", "token_revoked")
	e.want(e.do("POST", "/api/v1/tokens/0193a0f2-7c1e-7a10-8000-0000000000aa/revoke", e.as(op), ""), 404, "token_not_found")
	if e.q(`SELECT count(*)::text FROM audit_events WHERE action IN ('token.create', 'token.revoke') AND actor = $1`, "user:"+op.userID) != "2" {
		t.Fatal("token audit rows")
	}

	// Worker credentials.
	e.reason(e.do("POST", "/api/v1/worker-credentials", e.op, `{"name":"w"}`), 403, "forbidden", "credential_not_allowed")
	rec = e.do("POST", "/api/v1/worker-credentials", e.as(op), `{"name":"pool-a","expires_in_days":10}`)
	wc := decode[map[string]any](t, e.want2(rec, 201))
	wsecret := wc["secret"].(string)
	if !strings.HasPrefix(wsecret, "cxw_") || wc["created_by"] != op.userID {
		t.Fatalf("worker credential: %v", wc)
	}
	credID, err := e.env.Store.AuthenticateWorker(context.Background(), wsecret)
	if err != nil || credID != wc["id"] {
		t.Fatalf("issued credential does not authenticate: %v", err)
	}
	if _, err := e.env.Store.Register(context.Background(), credID, store.RegisterParams{IncarnationID: store.NewUUID(), Name: "w",
		ProtocolVersion: 1, Capacity: 1, Capabilities: []store.Capability{{Name: "hash_sha256", Version: 1}}}); err != nil {
		t.Fatal(err)
	}
	list := decode[map[string]any](t, e.want2(e.do("GET", "/api/v1/worker-credentials", e.as(op), ""), 200))
	item := list["items"].([]any)[0].(map[string]any)
	if len(item["bound_worker_ids"].([]any)) != 1 || item["secret"] != nil {
		t.Fatalf("credential list: %v", item)
	}
	e.want(e.do("POST", "/api/v1/worker-credentials/"+credID+"/revoke", e.as(op), ""), 200, "")
	if _, err := e.env.Store.AuthenticateWorker(context.Background(), wsecret); !store.IsDenial(err, "credential_revoked") {
		t.Fatalf("revoked worker credential still authenticates: %v", err)
	}
	if e.q(`SELECT revoked_by::text FROM worker_credentials WHERE id = $1`, credID) != op.userID {
		t.Fatal("revoked_by")
	}
}

func TestM5_AuditAndMetrics(t *testing.T) {
	e := newM5(t)
	op := e.login(400, "operator")
	vw := e.login(401, "viewer")
	id := decode[map[string]any](t, e.want2(e.do("POST", "/api/v1/jobs", e.op, okJob), 201))["id"].(string)
	e.want(e.do("POST", "/api/v1/jobs/"+id+"/cancel", e.as(op), ""), 200, "")
	e.want(e.do("POST", "/api/v1/jobs/"+id+"/cancel", e.as(vw), ""), 403, "forbidden")
	all := decode[map[string]any](t, e.want2(e.do("GET", "/api/v1/audit", e.as(op), ""), 200))
	cats := map[string]bool{}
	for _, x := range all["items"].([]any) {
		cats[x.(map[string]any)["category"].(string)] = true
	}
	if !cats["privileged"] || !cats["security"] {
		t.Fatalf("operator audit categories: %v", cats)
	}
	vpage := decode[map[string]any](t, e.want2(e.do("GET", "/api/v1/audit", e.as(vw), ""), 200))
	for _, x := range vpage["items"].([]any) {
		m := x.(map[string]any)
		if m["category"] != "privileged" || m["details"] != nil || m["masked_fields"].([]any)[0] != "details" {
			t.Fatalf("viewer audit row: %v", m)
		}
	}
	if len(decode[map[string]any](t, e.want2(e.do("GET", "/api/v1/audit?category=security", e.view, ""), 200))["items"].([]any)) != 0 {
		t.Fatal("viewer token saw security rows")
	}
	sub := decode[map[string]any](t, e.want2(e.do("GET", "/api/v1/audit?subject_type=job&subject_id="+id+"&limit=1", e.as(op), ""), 200))
	if len(sub["items"].([]any)) != 1 || sub["items"].([]any)[0].(map[string]any)["action"] != "job.cancel" {
		t.Fatalf("subject filter: %v", sub)
	}
	e.want(e.do("GET", "/api/v1/audit?category=nope", e.as(op), ""), 422, "validation_failed")
	e.want(e.do("GET", "/api/v1/audit?cursor=abc", e.as(op), ""), 400, "invalid_cursor")

	ov := decode[map[string]any](t, e.want2(e.do("GET", "/api/v1/metrics/overview", e.view, ""), 200))
	if ov["jobs_by_status"].(map[string]any)["canceled"].(float64) != 1 || ov["as_of"] == nil {
		t.Fatalf("overview: %v", ov)
	}
	e.want(e.do("POST", "/api/v1/jobs", e.op, okJob), 201, "")
	ov = decode[map[string]any](t, e.want2(e.do("GET", "/api/v1/metrics/overview", e.as(vw), ""), 200))
	if ov["unschedulable_jobs"].(float64) != 1 || ov["oldest_queued_age_seconds"] == nil {
		t.Fatalf("overview unschedulable: %v", ov)
	}
	from := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	to := time.Now().Add(2 * time.Minute).UTC().Format(time.RFC3339)
	ts := decode[map[string]any](t, e.want2(e.do("GET", "/api/v1/metrics/timeseries?metric=jobs_admitted&step=1m&from="+from+"&to="+to, e.view, ""), 200))
	var sum float64
	for _, p := range ts["points"].([]any) {
		pt := p.(map[string]any)
		if pt["v"] != nil {
			sum += pt["v"].(float64)
		}
		pt0, _ := time.Parse(time.RFC3339Nano, pt["t"].(string))
		asOf, _ := time.Parse(time.RFC3339Nano, ts["as_of"].(string))
		if pt0.Add(time.Minute).After(asOf) {
			t.Fatalf("bucket %s ends after as_of %s", pt["t"], ts["as_of"])
		}
	}
	if len(ts["points"].([]any)) < 9 {
		t.Fatalf("points: %d", len(ts["points"].([]any)))
	}
	_ = sum // the jobs were admitted in the current (incomplete) minute, which is absent by contract
	for q, rule := range map[string]string{
		"metric=nope&from=" + from + "&to=" + to: "enum",
		"metric=jobs_failed&step=1m&from=" + time.Now().Add(-8*24*time.Hour).UTC().Format(time.RFC3339) + "&to=" + to: "window_too_wide",
		"metric=jobs_failed&step=1m&from=" + time.Now().Add(-3*24*time.Hour).UTC().Format(time.RFC3339) + "&to=" + to: "range",
		"metric=jobs_failed&from=" + to + "&to=" + from:                                                               "window_inverted",
		"metric=jobs_failed&to=" + to: "required",
	} {
		er := e.want(e.do("GET", "/api/v1/metrics/timeseries?"+q, e.view, ""), 422, "validation_failed")
		if !strings.Contains(fmt.Sprint(er.Details), rule) {
			t.Fatalf("%s: %v want rule %s", q, er.Details, rule)
		}
	}
	e.want(e.do("GET", "/api/v1/metrics/overview", e.token(402, "viewer", "jobs:read"), ""), 403, "forbidden")
}

func TestM5_RateLimit(t *testing.T) {
	e := newM5(t, func(v *api.V1) {
		v.Mutations = api.NewLimiter(0.5, 3)
		v.AuthFailures = api.NewLimiter(0.01, 4)
	})
	for i := range 3 {
		e.want(e.do("POST", "/api/v1/jobs", e.op, okJob), 201, "")
		_ = i
	}
	rec := e.do("POST", "/api/v1/jobs", e.op, okJob)
	er := e.want(rec, 429, "rate_limited")
	if ra := rec.Header().Get("Retry-After"); ra == "" || ra == "0" || er.Details["retry_after_seconds"] == nil {
		t.Fatalf("Retry-After %q details %v", ra, er.Details)
	}
	// Reads are not limited by the mutation bucket; another principal has its own bucket.
	e.want(e.do("GET", "/api/v1/jobs", e.op, ""), 200, "")
	e.want(e.do("POST", "/api/v1/jobs", e.token(9, "operator", "jobs:write"), okJob), 201, "")
	// Failed authentication drains the per-peer bucket; then even valid credentials from that peer get 429.
	for range 4 {
		e.want(e.do("GET", "/api/v1/jobs", "cxt_bad", ""), 401, "unauthenticated")
	}
	e.want(e.do("GET", "/api/v1/jobs", e.op, ""), 429, "rate_limited")
	e.want(e.do("POST", "/api/v1/sessions", "x.y.z", `{}`), 429, "rate_limited")
}

// refWorker drives the store directly as one registered worker.
type refWorker struct {
	st  *store.Store
	ref store.WorkerRef
}

func newRefWorker(t *testing.T, st *store.Store) *refWorker {
	t.Helper()
	ctx := context.Background()
	c, err := st.CreateWorkerCredential(ctx, "t", 0)
	if err != nil {
		t.Fatal(err)
	}
	credID, _ := st.AuthenticateWorker(ctx, c.Secret)
	inc := store.NewUUID()
	var caps []store.Capability
	for _, s := range []string{"sleep_ms", "hash_sha256", "fail_n_times"} {
		caps = append(caps, store.Capability{Name: s, Version: 1})
	}
	reg, err := st.Register(ctx, credID, store.RegisterParams{IncarnationID: inc, Name: "ref", ProtocolVersion: 1, Capacity: 8, Capabilities: caps})
	if err != nil {
		t.Fatal(err)
	}
	return &refWorker{st: st, ref: store.WorkerRef{WorkerID: reg.WorkerID, IncarnationID: inc, CredentialID: credID}}
}

func (w *refWorker) claim(t *testing.T, jobID string) store.AttemptRef {
	t.Helper()
	got, err := w.st.Claim(context.Background(), w.ref, 8, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range got {
		if a.JobID == jobID {
			return store.AttemptRef{WorkerRef: w.ref, JobID: a.JobID, AttemptID: a.AttemptID, Generation: a.LeaseGeneration}
		}
	}
	t.Fatalf("job %s not claimed (%d claimed)", jobID, len(got))
	return store.AttemptRef{}
}

func (w *refWorker) succeed(t *testing.T, a store.AttemptRef) {
	t.Helper()
	if _, err := w.st.ReportSuccess(context.Background(), a, json.RawMessage(`{"ok":true}`), ""); err != nil {
		t.Fatal(err)
	}
}

func (w *refWorker) fail(t *testing.T, a store.AttemptRef) {
	t.Helper()
	if _, err := w.st.ReportFailure(context.Background(), a, store.FailureParams{Permanent: true, Code: "planned_failure", Message: "x"}); err != nil {
		t.Fatal(err)
	}
}

var _ = http.MethodGet
