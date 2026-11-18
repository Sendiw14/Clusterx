// Command clusterx-worker is the CLUSTERX reference worker. In this foundation slice it loads
// configuration (fail closed), connects to the control plane over gRPC and waits for SIGTERM;
// it does not claim jobs yet.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sendiw14/Clusterx/apps/worker/internal/config"
	"github.com/Sendiw14/Clusterx/apps/worker/internal/rpcclient"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	slog.SetDefault(newLogger(slog.LevelInfo))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	log := newLogger(cfg.LogLevel).With("service", "clusterx-worker", "version", version, "worker_name", cfg.Name)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	conn, err := rpcclient.Dial(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Info("worker starting", "env", cfg.Env, "control_plane", cfg.Addr, "tls", !cfg.Insecure,
		"credential_set", cfg.Credential != "", "capacity", cfg.Capacity)

	if err := rpcclient.WaitReady(ctx, conn, log); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("shutdown before connecting")
			return nil
		}
		return err
	}

	<-ctx.Done()
	// Nothing to drain yet: no attempts are claimed in this slice. Later tickets send Drain,
	// finish or abandon attempts within cfg.ShutdownTimeout, then Deregister.
	log.Info("shutdown signal received; stopping")
	return nil
}

// newLogger returns a JSON logger on stdout with UTC timestamps.
func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				a.Value = slog.TimeValue(a.Value.Time().UTC())
			}
			return a
		},
	}))
}
