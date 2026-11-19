// Package config loads worker settings from the environment and fails closed (NFR-OPS-01).
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Env is the deployment environment; it must be set explicitly.
type Env string

const (
	Development Env = "development"
	Production  Env = "production"
)

// Variable names.
const (
	VarEnv             = "CLUSTERX_ENV"
	VarAddr            = "CLUSTERX_WORKER_CONTROL_PLANE_ADDR"
	VarCredential      = "CLUSTERX_WORKER_CREDENTIAL"
	VarTLSCAFile       = "CLUSTERX_WORKER_TLS_CA_FILE"
	VarTLSServerName   = "CLUSTERX_WORKER_TLS_SERVER_NAME"
	VarInsecure        = "CLUSTERX_WORKER_INSECURE"
	VarName            = "CLUSTERX_WORKER_NAME"
	VarCapacity        = "CLUSTERX_WORKER_CAPACITY"
	VarLogLevel        = "CLUSTERX_LOG_LEVEL"
	VarShutdownTimeout = "CLUSTERX_SHUTDOWN_TIMEOUT"
	VarWorkerID        = "CLUSTERX_WORKER_ID"      // optional fixed worker id (UUID) to re-register
	VarWorkerIDFile    = "CLUSTERX_WORKER_ID_FILE" // optional path persisting the server-assigned worker id
)

// CredentialPrefix starts every worker credential (ADR-0005: cxw_<prefix>_<secret>).
const CredentialPrefix = "cxw_"

// MinCredentialLen is the minimum accepted credential length.
const MinCredentialLen = 32

// Config is the validated worker configuration.
type Config struct {
	Env             Env
	Addr            string // control-plane gRPC host:port
	Credential      string // bearer credential; never logged
	Insecure        bool   // plaintext gRPC (development, or a loopback target)
	TLSCAFile       string // optional; system roots when empty
	TLSServerName   string // optional SNI / verification name override
	Name            string
	Capacity        int
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
	WorkerID        string // re-register this worker id (CR-M5-01 #4)
	WorkerIDFile    string // read at start, written after registration; must be on a writable volume
}

// Load reads and validates the configuration; lookup is os.LookupEnv in production code.
// Secret values never appear in errors.
func Load(lookup func(string) (string, bool)) (Config, error) {
	var errs []error
	get := func(k string) string { v, _ := lookup(k); return strings.TrimSpace(v) }
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	c := Config{
		Env:           Env(get(VarEnv)),
		Addr:          get(VarAddr),
		Credential:    get(VarCredential),
		TLSCAFile:     get(VarTLSCAFile),
		TLSServerName: get(VarTLSServerName),
		Name:          get(VarName),
	}
	switch c.Env {
	case Development, Production:
	case "":
		fail("%s is required (development or production)", VarEnv)
	default:
		fail("%s must be development or production, got %q", VarEnv, c.Env)
	}
	prod := c.Env == Production

	if c.Addr == "" {
		fail("%s is required", VarAddr)
	} else if _, _, err := net.SplitHostPort(c.Addr); err != nil {
		fail("%s must be host:port, got %q", VarAddr, c.Addr)
	}

	switch {
	case c.Credential == "" && prod:
		fail("%s is required in production", VarCredential)
	case c.Credential != "" && (!strings.HasPrefix(c.Credential, CredentialPrefix) || len(c.Credential) < MinCredentialLen):
		fail("%s must start with %q and be at least %d characters", VarCredential, CredentialPrefix, MinCredentialLen)
	}

	if s := get(VarInsecure); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			fail("%s must be a boolean, got %q", VarInsecure, s)
		}
		c.Insecure = b
	}
	if c.Insecure && prod && !IsLoopback(c.Addr) {
		fail("%s=true is not allowed in production unless %s is a loopback address", VarInsecure, VarAddr)
	}
	if c.Insecure && (c.TLSCAFile != "" || c.TLSServerName != "") {
		fail("%s conflicts with TLS settings", VarInsecure)
	}

	if c.Name == "" {
		h, err := os.Hostname()
		if err != nil || h == "" {
			h = "worker"
		}
		c.Name = h
	}
	if len(c.Name) > 100 {
		fail("%s must be at most 100 characters", VarName)
	}

	c.Capacity = 4
	if s := get(VarCapacity); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 256 {
			fail("%s must be an integer 1..256, got %q", VarCapacity, s)
		}
		c.Capacity = n
	}

	switch lv := strings.ToLower(get(VarLogLevel)); lv {
	case "", "info":
		c.LogLevel = slog.LevelInfo
	case "debug":
		c.LogLevel = slog.LevelDebug
	case "warn":
		c.LogLevel = slog.LevelWarn
	case "error":
		c.LogLevel = slog.LevelError
	default:
		fail("%s must be debug, info, warn or error, got %q", VarLogLevel, lv)
	}

	c.ShutdownTimeout = 30 * time.Second
	if s := get(VarShutdownTimeout); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d < time.Second || d > time.Hour {
			fail("%s must be a duration between 1s and 1h, got %q", VarShutdownTimeout, s)
		}
		c.ShutdownTimeout = d
	}

	c.WorkerID = strings.ToLower(get(VarWorkerID))
	if c.WorkerID != "" && !uuidRe.MatchString(c.WorkerID) {
		fail("%s must be a UUID", VarWorkerID)
	}
	c.WorkerIDFile = get(VarWorkerIDFile)
	if c.WorkerIDFile != "" && c.WorkerID != "" {
		fail("%s and %s are mutually exclusive", VarWorkerID, VarWorkerIDFile)
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return c, nil
}

// IsLoopback reports whether addr targets a loopback host.
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

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ReadWorkerID returns the worker id stored in path, "" if the file does not exist. A malformed file
// is an error (fail closed rather than guessing).
func ReadWorkerID(path string) (string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", VarWorkerIDFile, err)
	}
	id := strings.ToLower(strings.TrimSpace(string(b)))
	if !uuidRe.MatchString(id) {
		return "", fmt.Errorf("%s does not contain a worker id (UUID)", VarWorkerIDFile)
	}
	return id, nil
}

// WriteWorkerID stores id in path atomically (temp file + rename in the same directory).
func WriteWorkerID(path, id string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".worker-id-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(id + "\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
