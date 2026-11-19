// Package chaos runs the real clusterx-control-plane and clusterx-worker binaries as OS processes and
// injects process-level faults: kill -9 of a worker mid-handler (AC-04, CR-M5-01 #4) and a PostgreSQL
// outage mid-run (AC-08). The binaries are built from this checkout into a temp dir. Tests are skipped
// (not passed) without CLUSTERX_PG_ADMIN_DSN (AC-04) or without initdb/pg_ctl on PATH (AC-08).
package chaos

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/pgtest"
)

var binOnce sync.Once
var binDir, binErr = "", error(nil)

// binaries builds both commands once per test process.
func binaries(t *testing.T) (cp, worker string) {
	t.Helper()
	binOnce.Do(func() {
		binDir, binErr = os.MkdirTemp("", "clusterx-chaos-bin-")
		if binErr != nil {
			return
		}
		for _, b := range []struct{ dir, out string }{{"../../cmd/clusterx-control-plane", "cp"}, {"../../../worker/cmd/clusterx-worker", "worker"}} {
			cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, b.out), ".")
			cmd.Dir = b.dir
			if out, err := cmd.CombinedOutput(); err != nil {
				binErr = fmt.Errorf("build %s: %v\n%s", b.dir, err, out)
				return
			}
		}
	})
	if binErr != nil {
		t.Fatal(binErr)
	}
	return filepath.Join(binDir, "cp"), filepath.Join(binDir, "worker")
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// syncBuf is a goroutine-safe log sink.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

type proc struct {
	cmd *exec.Cmd
	out *syncBuf
}

func start(t *testing.T, bin string, env []string, args ...string) *proc {
	t.Helper()
	p := &proc{cmd: exec.Command(bin, args...), out: &syncBuf{}}
	p.cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}, env...)
	p.cmd.Stdout, p.cmd.Stderr = p.out, p.out
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
		if t.Failed() {
			t.Logf("--- %s output ---\n%s", filepath.Base(bin), tail(p.out.String(), 4000))
		}
	})
	return p
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func run(t *testing.T, bin string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", filepath.Base(bin), args, err, out)
	}
	return string(out)
}

func kv(out, key string) string {
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), key+"="); ok {
			return v
		}
	}
	return ""
}

func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", d, what)
}

type coord struct {
	base, grpc string
	env        []string
	bin        string
	p          *proc
}

func startCoordinator(t *testing.T, cpBin, dsn string) *coord {
	t.Helper()
	c := &coord{bin: cpBin}
	hp, mp, gp := freePort(t), freePort(t), freePort(t)
	c.base = fmt.Sprintf("http://127.0.0.1:%d", hp)
	c.grpc = fmt.Sprintf("127.0.0.1:%d", gp)
	c.env = []string{"CLUSTERX_ENV=development", "CLUSTERX_DATABASE_URL=" + dsn,
		fmt.Sprintf("CLUSTERX_HTTP_ADDR=127.0.0.1:%d", hp), fmt.Sprintf("CLUSTERX_METRICS_ADDR=127.0.0.1:%d", mp),
		"CLUSTERX_GRPC_ADDR=" + c.grpc, "CLUSTERX_LEASE_TTL=5s", "CLUSTERX_HEARTBEAT_INTERVAL=2s", "CLUSTERX_LIVENESS_TIMEOUT=30s",
		"CLUSTERX_SCHEDULER_TICK=200ms", "CLUSTERX_BACKOFF_BASE=100ms", "CLUSTERX_BACKOFF_CAP=200ms", "CLUSTERX_SHUTDOWN_TIMEOUT=2s"}
	c.p = start(t, cpBin, c.env)
	waitFor(t, 30*time.Second, "coordinator ready", func() bool { return c.status("GET", "/health/ready", "", "") == 200 })
	return c
}

func (c *coord) do(method, path, tok, body string, hdr ...string) (int, map[string]any, http.Header) {
	req, _ := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	cl := http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return 0, nil, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return resp.StatusCode, m, resp.Header
}

func (c *coord) status(method, path, tok, body string) int {
	s, _, _ := c.do(method, path, tok, body)
	return s
}

func (c *coord) job(t *testing.T, tok, id string) map[string]any {
	t.Helper()
	_, m, _ := c.do("GET", "/api/v1/jobs/"+id, tok, "")
	return m
}

// credentials creates an operator token and a worker credential with the CLI (secrets printed once).
func (c *coord) credentials(t *testing.T) (token, workerCred string) {
	t.Helper()
	out := run(t, c.bin, c.env, "token", "create", "--role", "operator", "--name", "chaos", "--owner-github-id", "1", "--owner-login", "chaos")
	wout := run(t, c.bin, c.env, "worker-credential", "create", "--name", "chaos-worker")
	return kv(out, "secret"), kv(wout, "secret")
}

func startWorker(t *testing.T, bin string, c *coord, cred, idFile string) *proc {
	t.Helper()
	return start(t, bin, []string{"CLUSTERX_ENV=development", "CLUSTERX_WORKER_CONTROL_PLANE_ADDR=" + c.grpc,
		"CLUSTERX_WORKER_CREDENTIAL=" + cred, "CLUSTERX_WORKER_INSECURE=true", "CLUSTERX_WORKER_CAPACITY=2",
		"CLUSTERX_WORKER_NAME=chaos", "CLUSTERX_WORKER_ID_FILE=" + idFile, "CLUSTERX_SHUTDOWN_TIMEOUT=2s"})
}

// AC-04 with a real process kill -9 (plus CR-M5-01 #4): the worker binary is SIGKILLed mid-handler; its
// attempt is fenced and expires (lease_expired), never stuck running; the restarted binary re-registers
// the persisted worker id at once (no liveness wait) and completes the retry.
func TestAC04_KillDashNineWorkerProcess(t *testing.T) {
	cpBin, wBin := binaries(t)
	c := startCoordinator(t, cpBin, pgtest.NewDB(t))
	tok, cred := c.credentials(t)
	idFile := filepath.Join(t.TempDir(), "worker-id")
	w1 := startWorker(t, wBin, c, cred, idFile)
	waitFor(t, 15*time.Second, "worker registered", func() bool { return strings.Contains(w1.out.String(), `"msg":"registered"`) })
	code, j, _ := c.do("POST", "/api/v1/jobs", tok, `{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":4000},"max_attempts":3}`)
	if code != 201 {
		t.Fatalf("submit: %d %v", code, j)
	}
	id := j["id"].(string)
	waitFor(t, 15*time.Second, "job running", func() bool { return c.job(t, tok, id)["status"] == "running" })
	firstWorker, _ := os.ReadFile(idFile)
	if err := w1.cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = w1.cmd.Wait()
	if ws, ok := w1.cmd.ProcessState.Sys().(syscall.WaitStatus); !ok || ws.Signal() != syscall.SIGKILL {
		t.Fatalf("worker did not die by SIGKILL: %v", w1.cmd.ProcessState)
	}
	killedAt := time.Now()
	w2 := startWorker(t, wBin, c, cred, idFile)
	waitFor(t, 3*time.Second, "restarted worker registered without waiting for liveness (30 s)", func() bool {
		return strings.Contains(w2.out.String(), `"msg":"registered"`)
	})
	if strings.Contains(w2.out.String(), "incarnation_mismatch") {
		t.Fatal("restart waited for the previous incarnation")
	}
	if again, _ := os.ReadFile(idFile); string(again) != string(firstWorker) {
		t.Fatalf("worker id changed across restart: %q -> %q", firstWorker, again)
	}
	waitFor(t, 30*time.Second, "job succeeded after recovery", func() bool { return c.job(t, tok, id)["status"] == "succeeded" })
	_, att, _ := c.do("GET", "/api/v1/jobs/"+id+"/attempts", tok, "")
	var states []string
	for _, a := range att["items"].([]any) {
		m := a.(map[string]any)
		states = append(states, fmt.Sprintf("%v:%v", m["attempt_number"], m["state"]))
	}
	if strings.Join(states, ",") != "1:lease_expired,2:succeeded" {
		t.Fatalf("attempts %v", states)
	}
	t.Logf("AC-04 raw: killed_at=%s recovered_and_succeeded_after=%s attempts=%v", killedAt.UTC().Format(time.RFC3339Nano),
		time.Since(killedAt).Round(time.Millisecond), states)
}

// pgCluster is a throwaway PostgreSQL 18 cluster that the test may stop and restart (AC-08).
type pgCluster struct {
	dir  string
	port int
}

func newCluster(t *testing.T) *pgCluster {
	t.Helper()
	if _, err := exec.LookPath("initdb"); err != nil {
		t.Skip("initdb/pg_ctl not on PATH; AC-08 needs a stoppable PostgreSQL 18 cluster")
	}
	c := &pgCluster{dir: filepath.Join(t.TempDir(), "pg"), port: freePort(t)}
	cmd := exec.Command("initdb", "-D", c.dir, "-U", "clusterx", "--auth=trust", "-E", "UTF8", "--no-instructions")
	cmd.Env = append(os.Environ(), "LC_ALL=en_US.UTF-8")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("initdb: %v\n%s", err, out)
	}
	c.start(t)
	t.Cleanup(func() { _ = exec.Command("pg_ctl", "-D", c.dir, "-m", "immediate", "stop").Run() })
	return c
}

func (c *pgCluster) start(t *testing.T) {
	t.Helper()
	cmd := exec.Command("pg_ctl", "-D", c.dir, "-l", c.dir+"/../pg.log", "-w", "-o",
		fmt.Sprintf("-p %d -k '' -c listen_addresses=localhost", c.port), "start")
	cmd.Env = append(os.Environ(), "LC_ALL=en_US.UTF-8")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pg start: %v\n%s", err, out)
	}
}

// stop simulates an abrupt database loss (immediate shutdown: no checkpoint, crash recovery on start).
func (c *pgCluster) stop(t *testing.T) {
	t.Helper()
	if out, err := exec.Command("pg_ctl", "-D", c.dir, "-m", "immediate", "-w", "stop").CombinedOutput(); err != nil {
		t.Fatalf("pg stop: %v\n%s", err, out)
	}
}

func (c *pgCluster) dsn(db string) string {
	return fmt.Sprintf("postgres://clusterx@localhost:%d/%s", c.port, db)
}

// AC-08: PostgreSQL is stopped mid-run. Readiness turns 503, admission answers 503 (never 2xx), claims
// pause; after restart the coordinator and worker resume without restarts of their own and every
// acknowledged job completes with coherent state; the refused submission does not exist.
func TestAC08_DatabaseOutageMidRun(t *testing.T) {
	cpBin, wBin := binaries(t)
	pg := newCluster(t)
	c := startCoordinator(t, cpBin, pg.dsn("postgres"))
	tok, cred := c.credentials(t)
	w := startWorker(t, wBin, c, cred, filepath.Join(t.TempDir(), "worker-id"))
	waitFor(t, 15*time.Second, "worker registered", func() bool { return strings.Contains(w.out.String(), `"msg":"registered"`) })
	acked := map[string]bool{}
	for i := range 8 {
		code, j, _ := c.do("POST", "/api/v1/jobs", tok, fmt.Sprintf(`{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":%d},"max_attempts":5}`, 300+200*i))
		if code != 201 {
			t.Fatalf("submit %d: %d %v", i, code, j)
		}
		acked[j["id"].(string)] = true
	}
	waitFor(t, 15*time.Second, "work in progress", func() bool {
		_, m, _ := c.do("GET", "/api/v1/jobs?status=running", tok, "")
		items, _ := m["items"].([]any)
		return len(items) > 0
	})
	outageAt := time.Now()
	pg.stop(t)
	waitFor(t, 10*time.Second, "readiness 503", func() bool { return c.status("GET", "/health/ready", "", "") == 503 })
	// Repeated admissions deterministically cover both outage error paths (RV5-01): the first ones hit
	// pooled connections whose server died (io.ErrUnexpectedEOF), later ones fail to connect.
	for i := range 12 {
		code, body, hdr := c.do("POST", "/api/v1/jobs", tok, `{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"during-outage"}}`,
			"Idempotency-Key", "outage-1")
		if code != 503 || body["code"] != "unavailable" {
			t.Fatalf("admission %d during outage: %d %v (want 503 unavailable, never 2xx)", i, code, body)
		}
		if hdr.Get("Retry-After") == "" {
			t.Fatal("503 without Retry-After")
		}
	}
	if strings.Contains(c.p.out.String(), `"api internal error"`) {
		t.Fatalf("coordinator logged an internal error during the outage:\n%s", tail(c.p.out.String(), 20))
	}
	if c.status("GET", "/health/live", "", "") != 200 {
		t.Fatal("liveness must not depend on the database")
	}
	time.Sleep(3 * time.Second) // workers keep trying: claims/renewals fail UNAVAILABLE
	pg.start(t)
	restartedAt := time.Now()
	waitFor(t, 30*time.Second, "readiness back", func() bool { return c.status("GET", "/health/ready", "", "") == 200 })
	// The client retries the refused submission with the same key: created exactly once now.
	code, body, _ := c.do("POST", "/api/v1/jobs", tok, `{"handler_name":"hash_sha256","handler_version":1,"payload":{"data":"during-outage"}}`,
		"Idempotency-Key", "outage-1")
	if code != 201 {
		t.Fatalf("resubmission after outage: %d %v (outage attempt must not have been committed)", code, body)
	}
	acked[body["id"].(string)] = true
	waitFor(t, 60*time.Second, "all acknowledged jobs terminal", func() bool {
		for id := range acked {
			if s, _ := c.job(t, tok, id)["status"].(string); s != "succeeded" && s != "failed" {
				return false
			}
		}
		return true
	})
	// Coherence: exactly the acknowledged jobs exist; per job version = event count = max event version,
	// attempts_used = attempt rows, no active attempt left, every job succeeded within its budget.
	_, list, _ := c.do("GET", "/api/v1/jobs?limit=200", tok, "")
	if n := len(list["items"].([]any)); n != len(acked) {
		t.Fatalf("jobs %d, acknowledged %d", n, len(acked))
	}
	for id := range acked {
		j := c.job(t, tok, id)
		if j["status"] != "succeeded" {
			t.Fatalf("job %s ended %v (%v)", id, j["status"], j["error_code"])
		}
		_, ev, _ := c.do("GET", "/api/v1/jobs/"+id+"/events?limit=200", tok, "")
		_, at, _ := c.do("GET", "/api/v1/jobs/"+id+"/attempts", tok, "")
		evs, atts := ev["items"].([]any), at["items"].([]any)
		if float64(len(evs)) != j["version"].(float64) || evs[len(evs)-1].(map[string]any)["version"] != j["version"] {
			t.Fatalf("job %s: version %v, %d events", id, j["version"], len(evs))
		}
		if float64(len(atts)) != j["attempts_used"].(float64) {
			t.Fatalf("job %s: attempts_used %v, %d rows", id, j["attempts_used"], len(atts))
		}
		for _, a := range atts {
			if a.(map[string]any)["state"] == "active" {
				t.Fatalf("job %s: active attempt left", id)
			}
		}
	}
	// Neither process restarted: both rode out the outage (the scheduler logged failed ticks meanwhile).
	if w.cmd.ProcessState != nil || c.p.cmd.ProcessState != nil {
		t.Fatal("a process exited during the outage")
	}
	if strings.Contains(c.p.out.String(), `"worker rpc internal error"`) {
		t.Fatalf("worker RPCs answered INTERNAL during the outage (want UNAVAILABLE):\n%s", tail(c.p.out.String(), 20))
	}
	if !strings.Contains(c.p.out.String(), "scheduler tick failed") {
		t.Fatal("coordinator log shows no paused scheduler work during the outage")
	}
	t.Logf("AC-08 raw: outage_s=%.1f readiness_back_after_restart_s=%.1f acknowledged=%d all_succeeded=true",
		restartedAt.Sub(outageAt).Seconds(), time.Since(restartedAt).Seconds(), len(acked))
	_ = context.Background
}

// AC-11 (process level): SIGTERM to the worker binary drains: a handler longer than
// CLUSTERX_SHUTDOWN_TIMEOUT is canceled and reported retryable (worker_shutdown), the worker deregisters
// and exits 0; the job is not lost.
func TestAC11_WorkerSIGTERM(t *testing.T) {
	cpBin, wBin := binaries(t)
	c := startCoordinator(t, cpBin, pgtest.NewDB(t))
	tok, cred := c.credentials(t)
	w := startWorker(t, wBin, c, cred, filepath.Join(t.TempDir(), "worker-id"))
	waitFor(t, 15*time.Second, "worker registered", func() bool { return strings.Contains(w.out.String(), `"msg":"registered"`) })
	_, j, _ := c.do("POST", "/api/v1/jobs", tok, `{"handler_name":"sleep_ms","handler_version":1,"payload":{"ms":30000}}`)
	id := j["id"].(string)
	waitFor(t, 15*time.Second, "job running", func() bool { return c.job(t, tok, id)["status"] == "running" })
	if err := w.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- w.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker exit: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("worker did not exit after SIGTERM")
	}
	_, at, _ := c.do("GET", "/api/v1/jobs/"+id+"/attempts", tok, "")
	a := at["items"].([]any)[0].(map[string]any)
	if a["state"] != "failed" || a["error_code"] != "worker_shutdown" || a["error_class"] != "retryable" {
		t.Fatalf("attempt after SIGTERM: %v", a)
	}
	if s := c.job(t, tok, id)["status"]; s != "retry_wait" && s != "queued" {
		t.Fatalf("job after SIGTERM: %v", s)
	}
	_, ws, _ := c.do("GET", "/api/v1/workers", tok, "")
	if st := ws["items"].([]any)[0].(map[string]any)["status"]; st != "deregistered" {
		t.Fatalf("worker status %v", st)
	}
}
