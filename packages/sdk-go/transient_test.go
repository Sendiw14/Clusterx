package sdk

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"
)

// outageConn fakes a coordinator whose DB is down: Register succeeds, Claim and Heartbeat fail with
// INTERNAL / UNAVAILABLE.
type outageConn struct{ claims, heartbeats atomic.Int32 }

func (c *outageConn) Invoke(_ context.Context, method string, _, reply any, _ ...grpc.CallOption) error {
	switch {
	case strings.HasSuffix(method, "/Register"):
		r := reply.(*workerv1.RegisterResponse)
		r.WorkerId = "00000000-0000-0000-0000-000000000001"
		r.LeaseTtl = durationpb.New(30 * time.Second)
		r.HeartbeatInterval = durationpb.New(20 * time.Millisecond)
		return nil
	case strings.HasSuffix(method, "/Claim"):
		if c.claims.Add(1)%2 == 0 {
			return status.Error(codes.Unavailable, "database unavailable")
		}
		return status.Error(codes.Internal, "internal error")
	case strings.HasSuffix(method, "/Heartbeat"):
		c.heartbeats.Add(1)
		return status.Error(codes.Internal, "internal error")
	}
	return nil // Drain, Deregister
}

func (c *outageConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// RV5-01: INTERNAL / UNAVAILABLE on Claim and Heartbeat back off and retry; the worker never exits.
func TestRunSurvivesCoordinatorInternalErrors(t *testing.T) {
	var r Registry
	MustRegister(&r, "noop", 1, HandlerFunc[struct{}, struct{}](func(context.Context, Job, struct{}) (struct{}, error) {
		return struct{}{}, nil
	}))
	conn := &outageConn{}
	w := NewWorker(conn, &r, WorkerOptions{Name: "t", Capacity: 1, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	if err := w.Run(ctx); err != nil {
		t.Fatalf("Run exited with %v during a coordinator outage", err)
	}
	if conn.claims.Load() < 3 || conn.heartbeats.Load() < 3 {
		t.Fatalf("not retried: claims=%d heartbeats=%d", conn.claims.Load(), conn.heartbeats.Load())
	}
}
