// Package config loads control-plane settings from the environment and fails closed:
// any missing or invalid setting is an error, and production adds stricter rules
// (TLS, non-default secrets). NFR-OPS-01.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Env is the deployment environment. It must be set explicitly; there is no default.
type Env string

const (
	Development Env = "development"
	Production  Env = "production"
)

// MinSecretLen is the minimum length of the metrics scrape token.
const MinSecretLen = 32

// TLSFiles names a PEM certificate and private key. Both or neither must be set.
type TLSFiles struct {
	CertFile string
	KeyFile  string
}

// Enabled reports whether TLS is configured.
func (t TLSFiles) Enabled() bool { return t.CertFile != "" }

// Config is the validated control-plane configuration.
type Config struct {
	Env             Env
	DatabaseURL     string
	MigrateOnStart  bool
	HTTPAddr        string
	HTTPTLS         TLSFiles
	HTTPTLSOffload  bool // TLS terminated by a reverse proxy on a private network (production only meaning)
	MetricsAddr     string
	MetricsToken    string
	GRPCAddr        string
	GRPCTLS         TLSFiles
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
}

// Variable names.
const (
	VarEnv             = "CLUSTERX_ENV"
	VarDatabaseURL     = "CLUSTERX_DATABASE_URL"
	VarMigrateOnStart  = "CLUSTERX_MIGRATE_ON_START"
	VarHTTPAddr        = "CLUSTERX_HTTP_ADDR"
	VarHTTPTLSCert     = "CLUSTERX_HTTP_TLS_CERT_FILE"
	VarHTTPTLSKey      = "CLUSTERX_HTTP_TLS_KEY_FILE"
	VarHTTPTLSOffload  = "CLUSTERX_HTTP_TLS_OFFLOADED"
	VarMetricsAddr     = "CLUSTERX_METRICS_ADDR"
	VarMetricsToken    = "CLUSTERX_METRICS_TOKEN"
	VarGRPCAddr        = "CLUSTERX_GRPC_ADDR"
	VarGRPCTLSCert     = "CLUSTERX_GRPC_TLS_CERT_FILE"
	VarGRPCTLSKey      = "CLUSTERX_GRPC_TLS_KEY_FILE"
	VarLogLevel        = "CLUSTERX_LOG_LEVEL"
	VarShutdownTimeout = "CLUSTERX_SHUTDOWN_TIMEOUT"
)

// knownDefaults are placeholder secrets from examples and tutorials; production refuses them.
var knownDefaults = []string{"postgres", "password", "clusterx", "changeme", "secret", "example", "admin", "test"}

// Load reads and validates the configuration. lookup is os.LookupEnv in production code.
// All problems are reported together; secret values never appear in errors.
func Load(lookup func(string) (string, bool)) (Config, error) {
	var errs []error
	get := func(k string) string {
		v, _ := lookup(k)
		return strings.TrimSpace(v)
	}
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	c := Config{
		Env:          Env(get(VarEnv)),
		DatabaseURL:  get(VarDatabaseURL),
		HTTPAddr:     orDefault(get(VarHTTPAddr), ":8080"),
		MetricsAddr:  orDefault(get(VarMetricsAddr), ":9091"),
		MetricsToken: get(VarMetricsToken),
		GRPCAddr:     orDefault(get(VarGRPCAddr), ":9090"),
		HTTPTLS:      TLSFiles{CertFile: get(VarHTTPTLSCert), KeyFile: get(VarHTTPTLSKey)},
		GRPCTLS:      TLSFiles{CertFile: get(VarGRPCTLSCert), KeyFile: get(VarGRPCTLSKey)},
	}

	switch c.Env {
	case Development, Production:
	case "":
		fail("%s is required (development or production)", VarEnv)
	default:
		fail("%s must be development or production, got %q", VarEnv, c.Env)
	}
	prod := c.Env == Production

	parseBool := func(k string, def bool) bool {
		s := get(k)
		if s == "" {
			return def
		}
		b, err := strconv.ParseBool(s)
		if err != nil {
			fail("%s must be a boolean, got %q", k, s)
		}
		return b
	}
	c.MigrateOnStart = parseBool(VarMigrateOnStart, !prod)
	c.HTTPTLSOffload = parseBool(VarHTTPTLSOffload, false)

	if c.DatabaseURL == "" {
		fail("%s is required", VarDatabaseURL)
	} else if pc, err := pgconn.ParseConfig(c.DatabaseURL); err != nil {
		fail("%s is not a valid PostgreSQL connection string", VarDatabaseURL) // err may echo the password
	} else if prod && isKnownDefault(pc.Password) {
		fail("%s uses a default/example password; set a generated secret in production", VarDatabaseURL)
	}

	for _, a := range []struct{ name, val string }{{VarHTTPAddr, c.HTTPAddr}, {VarMetricsAddr, c.MetricsAddr}, {VarGRPCAddr, c.GRPCAddr}} {
		if _, _, err := net.SplitHostPort(a.val); err != nil {
			fail("%s must be host:port, got %q", a.name, a.val)
		}
	}
	if c.HTTPAddr == c.MetricsAddr || c.HTTPAddr == c.GRPCAddr || c.MetricsAddr == c.GRPCAddr {
		fail("%s, %s and %s must differ", VarHTTPAddr, VarMetricsAddr, VarGRPCAddr)
	}

	checkTLSPair := func(t TLSFiles, certVar, keyVar string) {
		if (t.CertFile == "") != (t.KeyFile == "") {
			fail("%s and %s must be set together", certVar, keyVar)
		}
	}
	checkTLSPair(c.HTTPTLS, VarHTTPTLSCert, VarHTTPTLSKey)
	checkTLSPair(c.GRPCTLS, VarGRPCTLSCert, VarGRPCTLSKey)

	switch {
	case c.MetricsToken == "" && prod:
		fail("%s is required in production", VarMetricsToken)
	case c.MetricsToken != "" && len(c.MetricsToken) < MinSecretLen:
		fail("%s must be at least %d characters", VarMetricsToken, MinSecretLen)
	case prod && isKnownDefault(c.MetricsToken):
		fail("%s uses a default/example value", VarMetricsToken)
	}

	if prod {
		// Workers authenticate with bearer credentials: never in plaintext off the host (ADR-0005).
		if !c.GRPCTLS.Enabled() && !IsLoopback(c.GRPCAddr) {
			fail("production requires %s/%s unless %s is loopback-only", VarGRPCTLSCert, VarGRPCTLSKey, VarGRPCAddr)
		}
		if !c.HTTPTLS.Enabled() && !c.HTTPTLSOffload && !IsLoopback(c.HTTPAddr) {
			fail("production requires %s/%s, or %s=true behind a TLS reverse proxy, unless %s is loopback-only",
				VarHTTPTLSCert, VarHTTPTLSKey, VarHTTPTLSOffload, VarHTTPAddr)
		}
	}

	switch lv := strings.ToLower(orDefault(get(VarLogLevel), "info")); lv {
	case "debug":
		c.LogLevel = slog.LevelDebug
	case "info":
		c.LogLevel = slog.LevelInfo
	case "warn":
		c.LogLevel = slog.LevelWarn
	case "error":
		c.LogLevel = slog.LevelError
	default:
		fail("%s must be debug, info, warn or error, got %q", VarLogLevel, lv)
	}

	c.ShutdownTimeout = 20 * time.Second
	if s := get(VarShutdownTimeout); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d < time.Second || d > 5*time.Minute {
			fail("%s must be a duration between 1s and 5m, got %q", VarShutdownTimeout, s)
		}
		c.ShutdownTimeout = d
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return c, nil
}

// IsLoopback reports whether a listen address binds only to a loopback interface.
// An empty host (all interfaces) is not loopback.
func IsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isKnownDefault(s string) bool {
	l := strings.ToLower(s)
	for _, d := range knownDefaults {
		if l == d || strings.HasPrefix(l, "changeme") || strings.HasPrefix(l, "replace") {
			return true
		}
	}
	return false
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
