package store

import (
	"context"
	"testing"
	"time"
)

func TestOpsSnapshotEmptyDB(t *testing.T) {
	s := newTestStore(t, time.Second)
	o, err := s.OpsSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if o.DatabaseSizeBytes <= 0 || o.WorkersRecentlyLost != 0 || len(o.AttemptsFinished5m) != 5 || o.ByStatus["queued"] != 0 {
		t.Fatalf("unexpected snapshot %+v", o)
	}
}
