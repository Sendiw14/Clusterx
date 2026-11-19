package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

func TestOpsCollector(t *testing.T) {
	age := 42.0
	ok := &OpsCollector{Timeout: time.Second, Snapshot: func(context.Context) (store.Ops, error) {
		return store.Ops{Overview: store.Overview{ByStatus: map[string]int64{"queued": 3}, OldestQueuedAge: &age, WorkersStale: 1},
			AttemptsFinished5m: map[string]int64{"lease_expired": 2}, DatabaseSizeBytes: 10}, nil
	}}
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(ok)
	if n, err := testutil.GatherAndCount(reg); err != nil || n != 10 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	if err := testutil.GatherAndCompare(reg, strings.NewReader(`# HELP clusterx_queue_oldest_age_seconds Age of the oldest queued job since it became eligible (0 when the queue is empty).
# TYPE clusterx_queue_oldest_age_seconds gauge
clusterx_queue_oldest_age_seconds 42
`), "clusterx_queue_oldest_age_seconds"); err != nil {
		t.Fatal(err)
	}
	down := &OpsCollector{Timeout: time.Second, Snapshot: func(context.Context) (store.Ops, error) { return store.Ops{}, errors.New("down") }}
	if v := testutil.ToFloat64(down); v != 0 {
		t.Fatalf("db_up = %v, want 0", v)
	}
}
