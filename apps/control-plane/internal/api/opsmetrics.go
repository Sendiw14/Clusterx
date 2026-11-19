package api

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

// OpsCollector exports the alerting gauges (infra/monitoring/alerts.yml) from one DB snapshot per
// scrape. Labels are fixed enums (status, state), so cardinality is bounded. A failed snapshot
// exports only clusterx_db_up 0.
type OpsCollector struct {
	Snapshot func(context.Context) (store.Ops, error)
	Timeout  time.Duration
}

var (
	descDBUp       = prometheus.NewDesc("clusterx_db_up", "1 if the control plane could read the database during this scrape.", nil, nil)
	descDBSize     = prometheus.NewDesc("clusterx_db_size_bytes", "pg_database_size of the CLUSTERX database.", nil, nil)
	descJobs       = prometheus.NewDesc("clusterx_jobs", "Jobs by status (active statuses: all rows; terminal: last 30 days).", []string{"status"}, nil)
	descQueueAge   = prometheus.NewDesc("clusterx_queue_oldest_age_seconds", "Age of the oldest queued job since it became eligible (0 when the queue is empty).", nil, nil)
	descUnsched    = prometheus.NewDesc("clusterx_jobs_unschedulable", "Queued jobs with no live, active worker supporting their handler.", nil, nil)
	descWorkers    = prometheus.NewDesc("clusterx_workers", "Non-deregistered workers by liveness (live, stale) and draining count.", []string{"state"}, nil)
	descWorkerLost = prometheus.NewDesc("clusterx_workers_recently_lost", "Workers whose heartbeat stopped within the last hour without deregistering.", nil, nil)
	descAttempts   = prometheus.NewDesc("clusterx_attempts_finished_5m", "Attempts that ended in the last 5 minutes, by attempt state.", []string{"state"}, nil)
)

// Describe implements prometheus.Collector.
func (c *OpsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{descDBUp, descDBSize, descJobs, descQueueAge, descUnsched, descWorkers, descWorkerLost, descAttempts} {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (c *OpsCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	o, err := c.Snapshot(ctx)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(descDBUp, prometheus.GaugeValue, 0)
		return
	}
	g := func(d *prometheus.Desc, v float64, l ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, l...)
	}
	g(descDBUp, 1)
	g(descDBSize, float64(o.DatabaseSizeBytes))
	for st, n := range o.ByStatus {
		g(descJobs, float64(n), st)
	}
	age := 0.0
	if o.OldestQueuedAge != nil && *o.OldestQueuedAge > 0 {
		age = *o.OldestQueuedAge
	}
	g(descQueueAge, age)
	g(descUnsched, float64(o.UnschedulableQueued))
	g(descWorkers, float64(o.WorkersLive), "live")
	g(descWorkers, float64(o.WorkersStale), "stale")
	g(descWorkers, float64(o.WorkersDraining), "draining")
	g(descWorkerLost, float64(o.WorkersRecentlyLost))
	for st, n := range o.AttemptsFinished5m {
		g(descAttempts, float64(n), st)
	}
}
