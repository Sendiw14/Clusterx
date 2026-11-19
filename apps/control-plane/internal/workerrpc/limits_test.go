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

// RV5R-04 / R06: revoked or expired worker credentials are lifecycle failures and never charge the
// per-peer guess bucket.
func TestWorkerRevokedCredentialNotChargedAsGuess(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 10)
	var charged atomic.Int32
	addr := env.ServeGRPC(t, func(o *workerrpc.Options) {
		o.AuthFailure = func(string) bool { charged.Add(1); return true }
	})
	cred, err := env.Store.CreateWorkerCredential(bg, "w", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.Store.Pool.Exec(bg, `UPDATE worker_credentials SET revoked_at = now() WHERE secret_hash IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	c := workerv1.NewWorkerServiceClient(testenv.Dial(t, addr, cred.Secret))
	for range 3 {
		_, err := c.Heartbeat(bg, &workerv1.HeartbeatRequest{})
		wantErr(t, err, codes.Unauthenticated, "credential_revoked")
	}
	if n := charged.Load(); n != 0 {
		t.Fatalf("revoked credential charged the guess bucket %d times", n)
	}
}
