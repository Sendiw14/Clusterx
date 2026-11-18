package workerrpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/workerrpc"
	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"
)

func TestServesWorkerServiceUnimplemented(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := workerrpc.NewServer(nil)
	go func() { _ = s.Serve(ln) }()
	defer s.Stop()

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := workerv1.NewWorkerServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := c.Register(ctx, &workerv1.RegisterRequest{IncarnationId: "x"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("Register: want Unimplemented, got %v", err)
	}
	// Oversized request is rejected by the 256 KiB receive limit before reaching the service.
	big := &workerv1.ReportResultRequest{ResultJson: make([]byte, workerrpc.MaxMessageBytes+1)}
	if _, err := c.ReportResult(ctx, big); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized: want ResourceExhausted, got %v", err)
	}
}
