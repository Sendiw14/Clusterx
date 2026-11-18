package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/scheduler"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
)

// The loop promotes due scheduled jobs (E2) using the DB clock.
func TestRunPromotesDueJobs(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(5*time.Second), 5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var due time.Time
	if err := env.Store.Pool.QueryRow(ctx, `SELECT now() + interval '300 milliseconds'`).Scan(&due); err != nil {
		t.Fatal(err)
	}
	j, _, err := env.Store.Admit(ctx, store.AdmitParams{Principal: "token:t", Namespace: "default", HandlerName: "sleep_ms",
		HandlerVersion: 1, MaxAttempts: 1, TimeoutSeconds: 10, Payload: []byte(`{"ms":1}`), RunAfter: &due})
	if err != nil || j.Status != "scheduled" {
		t.Fatalf("admit: %v %s", err, j.Status)
	}
	go scheduler.Run(ctx, env.Store, 50*time.Millisecond, testenv.Discard)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := env.Store.GetJob(ctx, j.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == "queued" {
			if got.Version != 2 {
				t.Fatalf("version %d", got.Version)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("not promoted")
}
