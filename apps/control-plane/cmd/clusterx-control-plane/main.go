// Command clusterx-control-plane runs the CLUSTERX coordinator: REST/health on the public
// HTTP listener, Prometheus metrics on the internal listener, and the worker gRPC service.
//
// Usage:
//
//	clusterx-control-plane          # serve (default)
//	clusterx-control-plane migrate  # apply pending migrations and exit
//	clusterx-control-plane token create --role operator --name NAME --owner-github-id N --owner-login LOGIN [--scopes a,b] [--expires-in-days 90]
//	clusterx-control-plane token revoke --id UUID
//	clusterx-control-plane worker-credential create --name NAME [--expires-in-days 0]
//	clusterx-control-plane worker-credential revoke --id UUID
//
// The create commands print the secret once on stdout; only its SHA-256 is stored. There are no
// default credentials.
//
// Configuration is read from CLUSTERX_* environment variables and fails closed
// (internal/config).
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/api"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/catalog"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/config"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/migrate"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/scheduler"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/workerrpc"
	"github.com/Sendiw14/Clusterx/apps/control-plane/migrations"
)

// version and commit are set at build time with -ldflags "-X main.version=... -X main.commit=...".
// commit "0000000" means the build did not record a commit.
var (
	version = "dev"
	commit  = "0000000"
)

func main() {
	slog.SetDefault(newLogger(slog.LevelInfo))
	if err := run(os.Args[1:]); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	log := newLogger(cfg.LogLevel).With("service", "clusterx-control-plane", "version", version)
	slog.SetDefault(log)

	migs, err := migrate.Load(migrations.FS)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	switch {
	case len(args) == 1 && args[0] == "migrate":
		return runMigrations(ctx, cfg, migs, log)
	case len(args) == 0 || (len(args) == 1 && args[0] == "serve"):
		return serve(ctx, cfg, migs, log)
	case len(args) >= 2 && (args[0] == "token" || args[0] == "worker-credential"):
		return runAdmin(ctx, cfg, args, os.Stdout)
	default:
		return fmt.Errorf("usage: clusterx-control-plane [serve|migrate|token create|token revoke|worker-credential create|worker-credential revoke]")
	}
}

func runMigrations(ctx context.Context, cfg config.Config, migs []migrate.Migration, log *slog.Logger) error {
	conn, err := store.Connect(ctx, cfg.MigrationURL())
	if err != nil {
		return fmt.Errorf("migrate: connect: %w", err)
	}
	defer conn.Close(context.WithoutCancel(ctx))
	n, err := migrate.Apply(ctx, conn, migs, log)
	if err != nil {
		return err
	}
	log.Info("migrations complete", "applied", n, "schema_version", len(migs))
	return nil
}

func serve(ctx context.Context, cfg config.Config, migs []migrate.Migration, log *slog.Logger) error {
	if cfg.MigrateOnStart {
		if err := runMigrations(ctx, cfg, migs, log); err != nil {
			return err
		}
	}

	pool, err := store.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	httpMetrics := api.NewMetrics(reg)

	httpTLS, err := loadTLS(cfg.HTTPTLS)
	if err != nil {
		return fmt.Errorf("http tls: %w", err)
	}
	grpcTLS, err := loadTLS(cfg.GRPCTLS)
	if err != nil {
		return fmt.Errorf("grpc tls: %w", err)
	}

	cat, err := catalog.Default()
	if err != nil {
		return err
	}
	st := store.New(pool, storeTiming(cfg), cat)
	if cfg.Env == config.Production {
		switch err := st.CheckRuntimePrivileges(ctx); {
		case errors.Is(err, store.ErrPrivilegedRuntime):
			return err
		case err != nil:
			log.Warn("could not verify runtime role privileges at start-up", "err", err)
		}
	}
	v1 := &api.V1{Store: st, Catalog: cat, Environment: string(cfg.Env), Version: version, Commit: commit, Log: log,
		Timing: api.ConfigTiming{HeartbeatInterval: cfg.Timing.HeartbeatInterval, LivenessTimeout: cfg.Timing.LivenessTimeout,
			LeaseTTL: cfg.Timing.LeaseTTL, CancelGrace: cfg.Timing.CancelGrace, DrainGrace: cfg.Timing.DrainGrace}}
	apiSrv := newHTTPServer(cfg.HTTPAddr, api.NewHandler(api.DBReadiness(pool, migs), log, httpMetrics, v1), httpTLS, log)
	metricsSrv := newHTTPServer(cfg.MetricsAddr, api.NewMetricsHandler(reg, cfg.MetricsToken, log, httpMetrics), httpTLS, log)
	var creds credentials.TransportCredentials
	if grpcTLS != nil {
		creds = credentials.NewTLS(grpcTLS)
	}
	grpcSrv := workerrpc.NewServer(st, log, workerrpc.Options{Creds: creds})

	// Bind every listener before reporting started, so a port clash fails start-up.
	apiLn, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	metricsLn, err := net.Listen("tcp", cfg.MetricsAddr)
	if err != nil {
		apiLn.Close()
		return err
	}
	grpcLn, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		apiLn.Close()
		metricsLn.Close()
		return err
	}

	schedCtx, stopSched := context.WithCancel(ctx)
	defer stopSched()
	go scheduler.Run(schedCtx, st, cfg.Timing.SchedulerTick, log)

	errc := make(chan error, 3)
	go func() { errc <- serveHTTP(apiSrv, apiLn) }()
	go func() { errc <- serveHTTP(metricsSrv, metricsLn) }()
	go func() { errc <- grpcSrv.Serve(grpcLn) }()
	log.Info("control plane started", "env", cfg.Env, "http", apiLn.Addr().String(), "metrics", metricsLn.Addr().String(),
		"grpc", grpcLn.Addr().String(), "http_tls", httpTLS != nil, "grpc_tls", grpcTLS != nil, "schema_version", len(migs))

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case runErr = <-errc:
		log.Error("listener failed", "err", runErr)
	}
	stopSched()
	shutdown(cfg.ShutdownTimeout, log, pool, grpcSrv, apiSrv, metricsSrv)
	return runErr
}

// storeTiming maps validated configuration to store durations (ADR-0003).
func storeTiming(cfg config.Config) store.Timing {
	t := store.DefaultTiming()
	t.LeaseTTL = cfg.Timing.LeaseTTL
	t.CancelGrace = cfg.Timing.CancelGrace
	t.DrainGraceMax = cfg.Timing.DrainGrace
	t.HeartbeatInterval = cfg.Timing.HeartbeatInterval
	t.LivenessTimeout = cfg.Timing.LivenessTimeout
	t.Backoff = store.Backoff{Base: cfg.Timing.BackoffBase, Cap: cfg.Timing.BackoffCap}
	return t
}

func newHTTPServer(addr string, h http.Handler, tlsCfg *tls.Config, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
}

func serveHTTP(s *http.Server, ln net.Listener) error {
	var err error
	if s.TLSConfig != nil {
		err = s.ServeTLS(ln, "", "")
	} else {
		err = s.Serve(ln)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// shutdown stops accepting work, lets in-flight requests finish within timeout, then closes
// the pool. In-flight worker attempts are not affected: their leases expire and are recovered.
func shutdown(timeout time.Duration, log *slog.Logger, pool *pgxpool.Pool, g *grpc.Server, servers ...*http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{})
	go func() { g.GracefulStop(); close(done) }()
	for _, s := range servers {
		if err := s.Shutdown(ctx); err != nil {
			log.Warn("http shutdown incomplete", "addr", s.Addr, "err", err)
		}
	}
	select {
	case <-done:
	case <-ctx.Done():
		log.Warn("grpc graceful stop timed out; forcing")
		g.Stop()
	}
	pool.Close()
	log.Info("shutdown complete")
}

func loadTLS(f config.TLSFiles) (*tls.Config, error) {
	if !f.Enabled() {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(f.CertFile, f.KeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}, nil
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
