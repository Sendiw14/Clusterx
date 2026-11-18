package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics are the HTTP server metrics. Labels use the matched route pattern, never the raw
// path, so cardinality is bounded by the number of routes.
type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewMetrics registers the HTTP metrics with reg.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "clusterx_http_requests_total", Help: "HTTP requests by server, route pattern and status code.",
		}, []string{"server", "route", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "clusterx_http_request_duration_seconds", Help: "HTTP request latency by server and route pattern.",
			Buckets: prometheus.DefBuckets,
		}, []string{"server", "route"}),
	}
	reg.MustRegister(m.requests, m.duration)
	return m
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// wrap adds request-id propagation, panic recovery, access logging and metrics.
// The route label is read after the mux ran (Request.Pattern is set on the shared request).
func wrap(server string, next http.Handler, log *slog.Logger, m *Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := requestIDFrom(r)
		w.Header().Set(HeaderRequestID, id)
		r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, id))
		rec := &statusRecorder{ResponseWriter: w}

		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				log.Error("panic in handler", "request_id", id, "panic", p)
				if rec.status == 0 {
					writeError(rec, r, http.StatusInternalServerError, "internal", "Internal error.")
				}
			}
			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			elapsed := time.Since(start)
			if m != nil {
				m.requests.WithLabelValues(server, route, strconv.Itoa(status)).Inc()
				m.duration.WithLabelValues(server, route).Observe(elapsed.Seconds())
			}
			log.LogAttrs(r.Context(), slog.LevelInfo, "http request",
				slog.String("server", server), slog.String("request_id", id), slog.String("method", r.Method),
				slog.String("route", route), slog.Int("status", status), slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000))
		}()
		next.ServeHTTP(rec, r)
	})
}
