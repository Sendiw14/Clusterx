package rpcclient_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Sendiw14/Clusterx/apps/worker/internal/config"
	"github.com/Sendiw14/Clusterx/apps/worker/internal/rpcclient"
	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"
)

const cred = "cxw_abcdef_0123456789abcdefghijklmnopqrstuv"

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

type fake struct {
	workerv1.UnimplementedWorkerServiceServer
}

func TestDialWaitReadyAndBearer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var gotAuth []string
	srv := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		mu.Lock()
		gotAuth = md.Get("authorization")
		mu.Unlock()
		return h(ctx, req)
	}))
	workerv1.RegisterWorkerServiceServer(srv, fake{})
	go func() { _ = srv.Serve(ln) }()
	defer srv.Stop()

	cfg := config.Config{Env: config.Development, Addr: ln.Addr().String(), Insecure: true, Credential: cred}
	conn, err := rpcclient.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rpcclient.WaitReady(ctx, conn, discard); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	_, err = workerv1.NewWorkerServiceClient(conn).Heartbeat(ctx, &workerv1.HeartbeatRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("Heartbeat: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(gotAuth) != 1 || gotAuth[0] != "Bearer "+cred {
		t.Fatalf("authorization metadata = %q", gotAuth)
	}
}

func TestWaitReadyHonoursContext(t *testing.T) {
	// Reserve a port and close it so nothing listens there.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	conn, err := rpcclient.Dial(config.Config{Addr: addr, Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := rpcclient.WaitReady(ctx, conn, discard); err == nil {
		t.Fatal("want context error while control plane is down")
	}
}

func TestTLSCredentialNotSentInPlaintext(t *testing.T) {
	// TLS mode against a plaintext server: the handshake fails, so the bearer is never sent.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	srv := grpc.NewServer()
	workerv1.RegisterWorkerServiceServer(srv, fake{})
	go func() { _ = srv.Serve(ln) }()
	defer srv.Stop()
	conn, err := rpcclient.Dial(config.Config{Addr: ln.Addr().String(), Credential: cred})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := workerv1.NewWorkerServiceClient(conn).Heartbeat(ctx, &workerv1.HeartbeatRequest{}); status.Code(err) != codes.Unavailable && status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("want Unavailable over a non-TLS server, got %v", err)
	}
}
