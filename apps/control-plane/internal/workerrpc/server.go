// Package workerrpc serves the worker gRPC protocol (clusterx.worker.v1). In this foundation
// slice every RPC answers codes.Unimplemented via the generated stubs; claim, lease and report
// logic arrive in later tickets.
package workerrpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"
)

// MaxMessageBytes is the gRPC message limit in each direction (ADR-0004).
const MaxMessageBytes = 256 << 10

type service struct {
	workerv1.UnimplementedWorkerServiceServer
}

// NewServer returns a gRPC server with the worker service registered. creds may be nil only
// for a loopback listener; config enforces that in production.
func NewServer(creds credentials.TransportCredentials) *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(MaxMessageBytes),
		grpc.MaxSendMsgSize(MaxMessageBytes),
	}
	if creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}
	s := grpc.NewServer(opts...)
	workerv1.RegisterWorkerServiceServer(s, &service{})
	return s
}
