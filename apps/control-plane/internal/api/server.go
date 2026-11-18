// Package api serves the control-plane HTTP surface: /health/live, /health/ready and the M4 /api/v1
// operations on the public listener, protected /metrics on the internal listener (OpenAPI
// 1.0.0-draft.5). Unknown routes answer 404 not_found_route and wrong methods 405
// method_not_allowed, both with the Error body (CR-M4-01 M3-2).
package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ReadinessCheck returns nil when the coordinator can serve: database reachable and schema
// migrations exactly at the version this binary expects.
type ReadinessCheck func(ctx context.Context) error

// ReadyTimeout bounds one readiness probe.
const ReadyTimeout = 2 * time.Second

// NewHandler returns the public HTTP handler: health endpoints and, when v1 is given, /api/v1.
func NewHandler(ready ReadinessCheck, log *slog.Logger, m *Metrics, v1 ...*V1) http.Handler {
	mux := http.NewServeMux()
	route(mux, "/health/live", map[string]http.HandlerFunc{http.MethodGet: func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}})
	route(mux, "/health/ready", map[string]http.HandlerFunc{http.MethodGet: func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), ReadyTimeout)
		defer cancel()
		if err := ready(ctx); err != nil {
			// The reason is logged, never returned: the endpoint is unauthenticated.
			log.Warn("not ready", "request_id", RequestID(r.Context()), "err", err)
			w.Header().Set("Retry-After", "5")
			writeError(w, r, http.StatusServiceUnavailable, "unavailable", "Not ready.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}})
	for _, v := range v1 {
		if v.Log == nil {
			v.Log = log
		}
		v.register(mux)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "not_found_route", "No such route.")
	})
	return wrap("api", mux, log, m)
}

// NewMetricsHandler returns the internal-listener handler serving GET /metrics from gatherer,
// guarded by a static bearer token. An empty token disables access entirely (always 401).
func NewMetricsHandler(gatherer prometheus.Gatherer, token string, log *slog.Logger, m *Metrics) http.Handler {
	want := sha256.Sum256([]byte(token))
	prom := promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelError)})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		// Compare fixed-length digests so neither content nor length leaks through timing.
		gotSum := sha256.Sum256([]byte(got))
		if token == "" || !ok || subtle.ConstantTimeCompare(gotSum[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="clusterx-metrics"`)
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required.")
			return
		}
		prom.ServeHTTP(w, r)
	})
	return wrap("metrics", mux, log, m)
}
