package e2e

import (
	"context"
	"encoding/json"
	"io"
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

// rest serves /api/v1 for the cluster and returns an operator token.
func (c *cluster) rest(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	cat, _ := catalog.Default()
	h := api.NewHandler(func(context.Context) error { return nil }, testenv.Discard, nil, &api.V1{Store: c.env.Store, Catalog: cat,
		Environment: "test", Version: "test", Commit: "0000000", Log: testenv.Discard, Hub: api.NewHub(testenv.Discard),
		Timing: api.ConfigTiming{DrainGrace: time.Minute}})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	tok, err := c.env.Store.CreateAPIToken(bg, store.TokenParams{OwnerGitHubID: 1, OwnerLogin: "op", Role: "operator", Name: "e2e",
		Scopes: store.AllScopes, ExpiresInDays: 1})
	if err != nil {
		t.Fatal(err)
	}
	return srv, tok.Secret
}

func post(t *testing.T, srv *httptest.Server, tok, path, body string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return resp.StatusCode, m
}

// AC-11 (REST): an operator drain stops new claims at once; a short handler finishes; a handler longer
// than the drain grace loses its lease when renewals are denied after the deadline and is recovered by
// lease expiry (not canceled), then completed by another worker; the drained SDK worker deregisters and
// Run returns nil.
func TestAC11_RESTDrain(t *testing.T) {
	c := newCluster(t, 2*time.Second, 20)
	srv, tok := c.rest(t)
	w := c.startWorker(t, 2, 30*time.Second)
	short := c.submit(t, "sleep_ms", `{"ms":400}`, 3)
	long := c.submit(t, "sleep_ms", `{"ms":5000}`, 3)
	waitFor(t, 10*time.Second, "both running", func() bool {
		return c.q(t, `SELECT count(*)::text FROM jobs WHERE status = 'running'`) == "2"
	})
	wid := c.q(t, `SELECT id::text FROM workers`)
	code, body := post(t, srv, tok, "/api/v1/workers/"+wid+"/drain", `{"grace_seconds":1}`)
	if code != 202 || body["status"] != "draining" {
		t.Fatalf("drain: %d %v", code, body)
	}
	queued := c.submit(t, "hash_sha256", `{"data":"after-drain"}`, 3)
	select {
	case err := <-w.done:
		if err != nil {
			t.Fatalf("drained worker Run: %v", err)
		}
		w.done <- nil
	case <-time.After(20 * time.Second):
		t.Fatal("drained worker did not stop")
	}
	if got := c.q(t, `SELECT status FROM jobs WHERE id = $1`, short); got != "succeeded" {
		t.Fatalf("short: %s", got)
	}
	if got := c.q(t, `SELECT count(*)::text FROM job_attempts WHERE job_id = $1`, queued); got != "0" {
		t.Fatal("draining worker claimed a new job")
	}
	waitFor(t, 15*time.Second, "long attempt recovered by lease expiry", func() bool {
		return c.q(t, `SELECT coalesce(string_agg(state, ','), '') FROM job_attempts WHERE job_id = $1`, long) == "lease_expired"
	})
	if got := c.q(t, `SELECT status FROM workers WHERE id = $1`, wid); got != "deregistered" {
		t.Fatalf("worker status %s", got)
	}
	c.startWorker(t, 2, time.Second)
	waitFor(t, 25*time.Second, "work completes elsewhere", func() bool {
		return c.q(t, `SELECT count(*)::text FROM jobs WHERE id IN ($1, $2) AND status = 'succeeded'`, queued, long) == "2"
	})
	if got := c.q(t, `SELECT count(*)::text FROM audit_events WHERE action = 'worker.drain' AND target_id = $1`, wid); got != "1" {
		t.Fatalf("drain audit rows %s", got)
	}
}

// AC-06 via REST: cancel of a running job is committed (202) before the handler's success; the SDK
// observes it on renewal, the job ends canceled, and the success is never committed.
func TestAC06_RESTCancelVersusSuccess(t *testing.T) {
	c := newCluster(t, 1*time.Second, 20)
	srv, tok := c.rest(t)
	c.startWorker(t, 1, time.Second)
	id := c.submit(t, "sleep_ms", `{"ms":3000}`, 3)
	waitFor(t, 10*time.Second, "running", func() bool { return c.q(t, `SELECT status FROM jobs WHERE id = $1`, id) == "running" })
	code, body := post(t, srv, tok, "/api/v1/jobs/"+id+"/cancel", "")
	if code != 202 || body["cancel_requested_at"] == nil {
		t.Fatalf("cancel: %d %v", code, body)
	}
	waitFor(t, 10*time.Second, "canceled", func() bool { return c.q(t, `SELECT status FROM jobs WHERE id = $1`, id) == "canceled" })
	if got := c.q(t, `SELECT (result IS NULL)::text || '/' || failure_class FROM jobs WHERE id = $1`, id); got != "true/canceled_by_operator" {
		t.Fatalf("outcome %s", got)
	}
	if code, body := post(t, srv, tok, "/api/v1/jobs/"+id+"/cancel", ""); code != 200 || body["status"] != "canceled" {
		t.Fatalf("repeat cancel: %d %v", code, body)
	}
}
