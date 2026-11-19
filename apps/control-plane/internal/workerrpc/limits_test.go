package workerrpc_test

import (
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/workerrpc"
)

// RV5-05 (gRPC side): unknown worker secrets drain the per-peer bucket; then guesses get
// RESOURCE_EXHAUSTED rate_limited, while a valid credential from the same peer is still served.
func TestWorkerAuthFailureLimit(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 10)
	var left atomic.Int32
	left.Store(2)
	addr := env.ServeGRPC(t, func(o *workerrpc.Options) {
		o.AuthFailure = func(peer string) bool {
			if peer != "127.0.0.1" {
				t.Errorf("peer key %q", peer)
			}
			return left.Add(-1) >= 0
		}
	})
	bad := workerv1.NewWorkerServiceClient(testenv.Dial(t, addr, "cxw_00000000_"+"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"))
	for range 2 {
		_, err := bad.Heartbeat(bg, &workerv1.HeartbeatRequest{})
		wantErr(t, err, codes.Unauthenticated, "credential_revoked")
	}
	_, err := bad.Heartbeat(bg, &workerv1.HeartbeatRequest{})
	wantErr(t, err, codes.ResourceExhausted, "rate_limited")
	w := register(t, env, addr, 1, "sleep_ms")
	if _, err := w.Heartbeat(bg, &workerv1.HeartbeatRequest{Identity: w.id}); err != nil {
		t.Fatalf("valid credential limited: %v", err)
	}
}
