// Package rpcclient opens the worker's gRPC connection to the control plane.
package rpcclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/Sendiw14/Clusterx/apps/worker/internal/config"
)

// MaxMessageBytes matches the coordinator's gRPC limit in each direction (ADR-0004).
const MaxMessageBytes = 256 << 10

// bearer attaches `authorization: Bearer <credential>` to every call (ADR-0005).
type bearer struct {
	token      string
	requireTLS bool
}

func (b bearer) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + b.token}, nil
}

// RequireTransportSecurity makes grpc-go refuse to send the credential over plaintext unless
// plaintext was explicitly allowed by configuration.
func (b bearer) RequireTransportSecurity() bool { return b.requireTLS }

// Dial creates a client connection. It does not block; use WaitReady.
func Dial(cfg config.Config) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageBytes), grpc.MaxCallSendMsgSize(MaxMessageBytes)),
		grpc.WithUserAgent("clusterx-worker"),
	}
	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tc, err := tlsConfig(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tc)))
	}
	if cfg.Credential != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(bearer{token: cfg.Credential, requireTLS: !cfg.Insecure}))
	}
	return grpc.NewClient(cfg.Addr, opts...)
}

func tlsConfig(cfg config.Config) (*tls.Config, error) {
	tc := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.TLSServerName}
	if cfg.TLSCAFile != "" {
		pem, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", config.VarTLSCAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s contains no PEM certificates", config.VarTLSCAFile)
		}
		tc.RootCAs = pool
	}
	return tc, nil
}

// WaitReady triggers connection and blocks until the channel is READY or ctx ends, logging
// each state change. gRPC retries with its own exponential backoff in between.
func WaitReady(ctx context.Context, conn *grpc.ClientConn, log *slog.Logger) error {
	conn.Connect()
	start := time.Now()
	for {
		s := conn.GetState()
		if s == connectivity.Ready {
			log.Info("connected to control plane", "target", conn.Target(), "after", time.Since(start).Round(time.Millisecond).String())
			return nil
		}
		if s == connectivity.TransientFailure {
			log.Warn("control plane unreachable; retrying", "target", conn.Target())
		}
		if s == connectivity.Idle {
			conn.Connect()
		}
		if !conn.WaitForStateChange(ctx, s) {
			return ctx.Err()
		}
	}
}
