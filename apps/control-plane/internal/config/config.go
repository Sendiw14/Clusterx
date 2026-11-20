// Package config loads control-plane settings from the environment and fails closed:
// any missing or invalid setting is an error, and production adds stricter rules
// (TLS, non-default secrets). NFR-OPS-01.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
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
	DatabaseURL     string // runtime role (clusterx_runtime member): no DDL, no TRUNCATE (CR-M4-01 NF-04)
	MigrateURL      string // migration role (schema owner); falls back to DatabaseURL in development only
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
	Timing          Timing

	// M5 (TASK-M5-BE-01)
	BFFJWKSFile       string        // Ed25519 public keys (JWKS) of the BFF; empty = assertions rejected
	SessionIdle       time.Duration // idle timeout (ADR-0006 §3)
	SessionAbsolute   time.Duration // absolute session lifetime
	StreamOrigins     []string      // WebSocket Origin allowlist (empty: only Origin-less server clients)
	RateMutations     float64       // per principal per second (burst 2x); 0 disables
	RateAuthFailures  float64       // failed authentications per TCP peer per minute (burst = value)
	RateLogins        float64       // session creations per GitHub id per minute (burst = value)
	RetentionURL      string        // clusterx_retention member DSN; enables the retention loop/command
	RetentionInterval time.Duration // 0 = no in-process loop (run `retention run` from cron instead)
}

// Timing is the ADR-0003 timing configuration, validated against its documented bounds.
type Timing struct {
	HeartbeatInterval, LivenessTimeout, LeaseTTL, CancelGrace, DrainGrace time.Duration
	SchedulerTick, BackoffBase, BackoffCap                                time.Duration
}

// Timing variable names.
const (
	VarHeartbeatInterval = "CLUSTERX_HEARTBEAT_INTERVAL"
	VarLivenessTimeout   = "CLUSTERX_LIVENESS_TIMEOUT"
	VarLeaseTTL          = "CLUSTERX_LEASE_TTL"
	VarCancelGrace       = "CLUSTERX_CANCEL_GRACE"
	VarDrainGrace        = "CLUSTERX_DRAIN_GRACE"
	VarSchedulerTick     = "CLUSTERX_SCHEDULER_TICK"
	VarBackoffBase       = "CLUSTERX_BACKOFF_BASE"
	VarBackoffCap        = "CLUSTERX_BACKOFF_CAP"
)

// Variable names.
const (
	VarEnv             = "CLUSTERX_ENV"
	VarDatabaseURL     = "CLUSTERX_DATABASE_URL"
	VarMigrateURL      = "CLUSTERX_MIGRATE_DATABASE_URL"
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

	VarBFFJWKSFile       = "CLUSTERX_BFF_JWKS_FILE"
	VarSessionIdle       = "CLUSTERX_SESSION_IDLE_TIMEOUT"
	VarSessionAbsolute   = "CLUSTERX_SESSION_ABSOLUTE_TTL"
	VarStreamOrigins     = "CLUSTERX_STREAM_ALLOWED_ORIGINS"
	VarRateMutations     = "CLUSTERX_RATE_MUTATIONS_PER_SECOND"
	VarRateAuthFailures  = "CLUSTERX_RATE_AUTH_FAILURES_PER_MINUTE"
	VarRateLogins        = "CLUSTERX_RATE_LOGINS_PER_MINUTE"
	VarRetentionURL      = "CLUSTERX_RETENTION_DATABASE_URL"
	VarRetentionInterval = "CLUSTERX_RETENTION_INTERVAL"
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
	// secret reads k, or the file named by k_FILE (RV5-09: secrets mounted as files, not visible in
	// `docker inspect`). Setting both is an error; trailing whitespace is trimmed.
	secret := func(k string) string {
		path := get(k + "_FILE")
		if path == "" {
			return get(k)
		}
		if get(k) != "" {
			fail("set %s or %s_FILE, not both", k, k)
			return ""
		}
		b, err := os.ReadFile(path)
		if err != nil {
			fail("%s_FILE: cannot read %q", k, path)
			return ""
		}
		return strings.TrimSpace(string(b))
	}

	c := Config{
		Env:          Env(get(VarEnv)),
		DatabaseURL:  secret(VarDatabaseURL),
		MigrateURL:   secret(VarMigrateURL),
		HTTPAddr:     orDefault(get(VarHTTPAddr), ":8080"),
		MetricsAddr:  orDefault(get(VarMetricsAddr), ":9091"),
		MetricsToken: secret(VarMetricsToken),
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

	if c.MigrateURL != "" {
		if pc, err := pgconn.ParseConfig(c.MigrateURL); err != nil {
			fail("%s is not a valid PostgreSQL connection string", VarMigrateURL)
		} else if prod && isKnownDefault(pc.Password) {
			fail("%s uses a default/example password; set a generated secret in production", VarMigrateURL)
		}
	} else if prod && c.MigrateOnStart {
		fail("%s is required in production when %s=true (the runtime role cannot run DDL)", VarMigrateURL, VarMigrateOnStart)
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

	dur := func(k string, def, lo, hi time.Duration) time.Duration {
		v := get(k)
		if v == "" {
			return def
		}
		d, err := time.ParseDuration(v)
		if err != nil || d < lo || d > hi {
			fail("%s must be a duration between %s and %s, got %q", k, lo, hi, v)
			return def
		}
		return d
	}
	c.Timing = Timing{
		HeartbeatInterval: dur(VarHeartbeatInterval, 10*time.Second, 2*time.Second, time.Minute),
		LivenessTimeout:   dur(VarLivenessTimeout, 30*time.Second, 6*time.Second, 10*time.Minute),
		LeaseTTL:          dur(VarLeaseTTL, 30*time.Second, 5*time.Second, 300*time.Second),
		CancelGrace:       dur(VarCancelGrace, 15*time.Second, time.Second, 120*time.Second),
		DrainGrace:        dur(VarDrainGrace, 60*time.Second, 0, time.Hour),
		SchedulerTick:     dur(VarSchedulerTick, time.Second, 200*time.Millisecond, 10*time.Second),
		BackoffBase:       dur(VarBackoffBase, 2*time.Second, 100*time.Millisecond, time.Minute),
		BackoffCap:        dur(VarBackoffCap, 5*time.Minute, 100*time.Millisecond, time.Hour),
	}
	if c.Timing.LivenessTimeout < 3*c.Timing.HeartbeatInterval {
		fail("%s must be at least 3 x %s", VarLivenessTimeout, VarHeartbeatInterval)
	}
	if c.Timing.BackoffCap < c.Timing.BackoffBase {
		fail("%s must be >= %s", VarBackoffCap, VarBackoffBase)
	}

	c.BFFJWKSFile = get(VarBFFJWKSFile)
	c.SessionIdle = dur(VarSessionIdle, 2*time.Hour, time.Minute, 24*time.Hour)
	c.SessionAbsolute = dur(VarSessionAbsolute, 12*time.Hour, time.Hour, 7*24*time.Hour)
	if c.SessionAbsolute < c.SessionIdle {
		fail("%s must be >= %s", VarSessionAbsolute, VarSessionIdle)
	}
	for _, o := range strings.Split(get(VarStreamOrigins), ",") {
		if o = strings.TrimSpace(o); o == "" {
			continue
		}
		if !strings.HasPrefix(o, "https://") && !(strings.HasPrefix(o, "http://") && !prod) || strings.HasSuffix(o, "/") {
			fail("%s entries must be scheme://host[:port] origins (https in production), got %q", VarStreamOrigins, o)
		}
		c.StreamOrigins = append(c.StreamOrigins, o)
	}
	num := func(k string, def, lo, hi float64) float64 {
		v := get(k)
		if v == "" {
			return def
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < lo || f > hi {
			fail("%s must be a number between %g and %g, got %q", k, lo, hi, v)
			return def
		}
		return f
	}
	c.RateMutations = num(VarRateMutations, 20, 0, 10000)
	if prod && c.RateMutations == 0 {
		// 0 disables the limiter (benchmark overlay only, development env); never silently in production (RV9-08).
		fail("%s=0 disables the mutation rate limit and is not allowed in production", VarRateMutations)
	}
	c.RateAuthFailures = num(VarRateAuthFailures, 60, 1, 100000)
	c.RateLogins = num(VarRateLogins, 10, 1, 1000)
	c.RetentionURL = secret(VarRetentionURL)
	if c.RetentionURL != "" {
		if pc, err := pgconn.ParseConfig(c.RetentionURL); err != nil {
			fail("%s is not a valid PostgreSQL connection string", VarRetentionURL)
		} else if prod && isKnownDefault(pc.Password) {
			fail("%s uses a default/example password; set a generated secret in production", VarRetentionURL)
		}
	}
	c.RetentionInterval = dur(VarRetentionInterval, 0, 0, 24*time.Hour)
	if c.RetentionInterval > 0 && c.RetentionInterval < time.Minute {
		fail("%s must be 0 (off) or at least 1m", VarRetentionInterval)
	}
	if c.RetentionInterval > 0 && c.RetentionURL == "" {
		fail("%s requires %s (a clusterx_retention member)", VarRetentionInterval, VarRetentionURL)
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return c, nil
}

// MigrationURL is the DSN used by `migrate` and migrate-on-start.
func (c Config) MigrationURL() string {
	if c.MigrateURL != "" {
		return c.MigrateURL
	}
	return c.DatabaseURL
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
