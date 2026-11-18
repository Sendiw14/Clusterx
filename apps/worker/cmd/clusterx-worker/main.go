// Command clusterx-worker is the CLUSTERX reference worker. It loads configuration (fail closed),
// connects to the control plane over gRPC, registers the examples/basic-jobs handlers and runs the
// SDK claim/renew/report loop. SIGTERM/SIGINT drains: no new claims, running handlers get
// CLUSTERX_SHUTDOWN_TIMEOUT to finish, then they are canceled and reported, then the worker deregisters.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	basicjobs "github.com/Sendiw14/Clusterx/examples/basic-jobs"
	sdk "github.com/Sendiw14/Clusterx/packages/sdk-go"

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

	var reg sdk.Registry
	if err := basicjobs.Register(&reg); err != nil {
		return err
	}
	host, _ := os.Hostname()
	opts := sdk.WorkerOptions{Name: cfg.Name, Capacity: cfg.Capacity, Hostname: host, DrainGrace: cfg.ShutdownTimeout,
		Logger: log, WorkerID: cfg.WorkerID}
	if cfg.WorkerIDFile != "" {
		// CR-M5-01 #4: a restarted process re-registers the same worker immediately (new incarnation fences
		// the old one) instead of waiting for the previous incarnation's liveness timeout.
		if opts.WorkerID, err = config.ReadWorkerID(cfg.WorkerIDFile); err != nil {
			return err
		}
		opts.OnRegistered = func(id string) {
			if err := config.WriteWorkerID(cfg.WorkerIDFile, id); err != nil {
				log.Warn("could not persist worker id; a fast restart waits for the liveness timeout", "err", err)
			}
		}
	}
	w := sdk.NewWorker(conn, &reg, opts)
	err = w.Run(ctx)
	log.Info("worker stopped", "err", err)
	return err
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
