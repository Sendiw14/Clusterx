// Command driver submits the fixed CLUSTERX benchmark workload (docs/testing/benchmarks.md) through the
// public REST API and records per-request admission latency. It is stdlib-only and deterministic: job i
// always gets the same handler and payload. Completion, reconciliation and end-to-end latency are read
// from the database afterwards (scripts/bench/reconcile.sql), never inferred here.
//
//	GOWORK=off go run . -api http://127.0.0.1:18090 -token-file <file> -n 1000 -c 16 -out admission.csv
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

// Job returns the handler and payload of job i (the documented mix, per block of 10):
// 0-5 hash_sha256 (1 attempt), 6-7 sleep_ms 100 (1 attempt), 8 fail_n_times n=1 (2 attempts, succeeds),
// 9 fail_n_times n=1 permanent (1 attempt, failed).
func Job(i int) (string, any) {
	switch i % 10 {
	case 6, 7:
		return "sleep_ms", map[string]any{"ms": 100}
	case 8:
		return "fail_n_times", map[string]any{"n": 1}
	case 9:
		return "fail_n_times", map[string]any{"n": 1, "permanent": true}
	default:
		return "hash_sha256", map[string]any{"data": fmt.Sprintf("bench-%06d", i)}
	}
}

func main() {
	api := flag.String("api", "http://127.0.0.1:18090", "control-plane base URL")
	tokenFile := flag.String("token-file", "", "file holding the operator API token")
	n := flag.Int("n", 1000, "jobs")
	c := flag.Int("c", 16, "concurrent submitters")
	run := flag.String("run", "bench", "run label (idempotency-key prefix)")
	out := flag.String("out", "admission.csv", "per-request CSV: index,handler,status,latency_ms")
	flag.Parse()
	tok, err := os.ReadFile(*tokenFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "token file:", err)
		os.Exit(2)
	}
	token := strings.TrimSpace(string(tok))
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: *c}}

	type rec struct {
		handler string
		status  int
		lat     time.Duration
	}
	recs := make([]rec, *n)
	idx := make(chan int)
	var wg sync.WaitGroup
	start := time.Now()
	for range *c {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				h, p := Job(i)
				body, _ := json.Marshal(map[string]any{"handler_name": h, "handler_version": 1, "payload": p, "max_attempts": 3})
				req, _ := http.NewRequest(http.MethodPost, *api+"/api/v1/jobs", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", fmt.Sprintf("%s-%06d", *run, i))
				t0 := time.Now()
				resp, err := client.Do(req)
				r := rec{handler: h, lat: time.Since(t0)}
				if err == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					r.status = resp.StatusCode
				}
				recs[i] = r
			}
		}()
	}
	for i := range *n {
		idx <- i
	}
	close(idx)
	wg.Wait()
	wall := time.Since(start)

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Fprintln(f, "index,handler,status,latency_ms")
	var lats []float64
	codes := map[int]int{}
	for i, r := range recs {
		fmt.Fprintf(f, "%d,%s,%d,%.3f\n", i, r.handler, r.status, float64(r.lat.Microseconds())/1000)
		codes[r.status]++
		if r.status == http.StatusCreated {
			lats = append(lats, float64(r.lat.Microseconds())/1000)
		}
	}
	f.Close()
	slices.Sort(lats)
	pct := func(p float64) float64 {
		if len(lats) == 0 {
			return 0
		}
		return lats[max(0, int(math.Ceil(p*float64(len(lats))))-1)] // nearest rank
	}
	fmt.Printf("submitted=%d accepted_201=%d status_counts=%v wall_s=%.3f admit_rate_per_s=%.1f admission_ms p50=%.2f p95=%.2f p99=%.2f max=%.2f\n",
		*n, codes[http.StatusCreated], codes, wall.Seconds(), float64(codes[http.StatusCreated])/wall.Seconds(), pct(.50), pct(.95), pct(.99), pct(1))
	if codes[http.StatusCreated] != *n {
		os.Exit(1)
	}
}
