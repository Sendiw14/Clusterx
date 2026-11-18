package config

import (
	"log/slog"
	"maps"
	"strings"
	"testing"
	"time"
)

const goodToken = "0123456789abcdef0123456789abcdef-scrape"

func env(base map[string]string, overrides ...string) func(string) (string, bool) {
	m := maps.Clone(base)
	for i := 0; i+1 < len(overrides); i += 2 {
		if overrides[i+1] == "<unset>" {
			delete(m, overrides[i])
			continue
		}
		m[overrides[i]] = overrides[i+1]
	}
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

var devBase = map[string]string{
	VarEnv:         "development",
	VarDatabaseURL: "postgres://clusterx:clusterx@localhost:5432/clusterx",
}

var prodBase = map[string]string{
	VarEnv:          "production",
	VarDatabaseURL:  "postgres://clusterx:Zq8vN3x7Lr2pTt6Wk9yB@db:5432/clusterx?sslmode=require",
	VarMetricsToken: goodToken,
	VarGRPCTLSCert:  "/run/secrets/grpc.crt",
	VarGRPCTLSKey:   "/run/secrets/grpc.key",
	VarHTTPTLSCert:  "/run/secrets/http.crt",
	VarHTTPTLSKey:   "/run/secrets/http.key",
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(devBase))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.GRPCAddr != ":9090" || c.MetricsAddr != ":9091" || !c.MigrateOnStart ||
		c.LogLevel != slog.LevelInfo || c.ShutdownTimeout != 20*time.Second {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	p, err := Load(env(prodBase))
	if err != nil {
		t.Fatal(err)
	}
	if p.MigrateOnStart {
		t.Fatal("production must not migrate on start by default")
	}
}

func TestLoadFailsClosed(t *testing.T) {
	cases := []struct {
		name    string
		lookup  func(string) (string, bool)
		wantErr string // "" = valid
	}{
		{"missing env", env(devBase, VarEnv, "<unset>"), VarEnv + " is required"},
		{"unknown env", env(devBase, VarEnv, "prod"), "must be development or production"},
		{"missing db", env(devBase, VarDatabaseURL, "<unset>"), VarDatabaseURL + " is required"},
		{"bad db", env(devBase, VarDatabaseURL, "postgres://%zz"), "not a valid PostgreSQL"},
		{"bad bool", env(devBase, VarMigrateOnStart, "maybe"), VarMigrateOnStart + " must be a boolean"},
		{"bad addr", env(devBase, VarHTTPAddr, "8080"), VarHTTPAddr + " must be host:port"},
		{"addr clash", env(devBase, VarMetricsAddr, ":8080"), "must differ"},
		{"half tls", env(devBase, VarGRPCTLSCert, "/c.pem"), "must be set together"},
		{"short token dev", env(devBase, VarMetricsToken, "short"), "at least 32"},
		{"bad log level", env(devBase, VarLogLevel, "trace"), VarLogLevel},
		{"bad shutdown", env(devBase, VarShutdownTimeout, "10m"), VarShutdownTimeout},
		{"dev plaintext ok", env(devBase), ""},

		{"prod ok", env(prodBase), ""},
		{"prod no metrics token", env(prodBase, VarMetricsToken, "<unset>"), VarMetricsToken + " is required in production"},
		{"prod default metrics token", env(prodBase, VarMetricsToken, "changeme-changeme-changeme-changeme"), "default/example value"},
		{"prod default db password", env(prodBase, VarDatabaseURL, "postgres://clusterx:clusterx@db/clusterx"), "default/example password"},
		{"prod db password postgres", env(prodBase, VarDatabaseURL, "postgres://u:postgres@db/clusterx"), "default/example password"},
		{"prod grpc plaintext", env(prodBase, VarGRPCTLSCert, "<unset>", VarGRPCTLSKey, "<unset>"), "production requires " + VarGRPCTLSCert},
		{"prod grpc plaintext loopback", env(prodBase, VarGRPCTLSCert, "<unset>", VarGRPCTLSKey, "<unset>", VarGRPCAddr, "127.0.0.1:9090"), ""},
		{"prod grpc plaintext all ifaces", env(prodBase, VarGRPCTLSCert, "<unset>", VarGRPCTLSKey, "<unset>", VarGRPCAddr, "0.0.0.0:9090"), "production requires " + VarGRPCTLSCert},
		{"prod http plaintext", env(prodBase, VarHTTPTLSCert, "<unset>", VarHTTPTLSKey, "<unset>"), "production requires " + VarHTTPTLSCert},
		{"prod http offloaded", env(prodBase, VarHTTPTLSCert, "<unset>", VarHTTPTLSKey, "<unset>", VarHTTPTLSOffload, "true"), ""},
		{"prod http loopback", env(prodBase, VarHTTPTLSCert, "<unset>", VarHTTPTLSKey, "<unset>", VarHTTPAddr, "[::1]:8080"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(c.lookup)
			switch {
			case c.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case c.wantErr != "" && err == nil:
				t.Fatalf("want error containing %q, got nil", c.wantErr)
			case c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr):
				t.Fatalf("error %q does not contain %q", err, c.wantErr)
			}
		})
	}
}

func TestErrorsNeverContainSecrets(t *testing.T) {
	_, err := Load(env(prodBase, VarDatabaseURL, "postgres://u:clusterx@db/x", VarMetricsToken, "changeme-changeme-changeme-changeme-x"))
	if err == nil {
		t.Fatal("want error")
	}
	for _, s := range []string{"u:clusterx", "changeme-changeme"} {
		if strings.Contains(err.Error(), s) {
			t.Fatalf("error leaks secret %q: %v", s, err)
		}
	}
	_, err = Load(env(devBase, VarDatabaseURL, "postgres://u:s3cr3t@%zz"))
	if err == nil || strings.Contains(err.Error(), "s3cr3t") {
		t.Fatalf("parse error leaks secret or missing: %v", err)
	}
}

func TestIsLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:1": true, "localhost:1": true, "[::1]:1": true,
		":1": false, "0.0.0.0:1": false, "10.0.0.1:1": false, "bad": false,
	} {
		if got := IsLoopback(addr); got != want {
			t.Errorf("IsLoopback(%q)=%v want %v", addr, got, want)
		}
	}
}

func TestTimingAndMigrateURL(t *testing.T) {
	c, err := Load(env(devBase))
	if err != nil || c.Timing.LeaseTTL != 30*time.Second || c.Timing.LivenessTimeout != 30*time.Second || c.MigrationURL() != devBase[VarDatabaseURL] {
		t.Fatalf("defaults: %+v %v", c.Timing, err)
	}
	for name, kv := range map[string][]string{
		"lease too short":  {VarLeaseTTL, "1s"},
		"liveness < 3x hb": {VarHeartbeatInterval, "20s"},
		"cap < base":       {VarBackoffBase, "10s", VarBackoffCap, "1s"},
		"bad tick":         {VarSchedulerTick, "fast"},
		"bad migrate url":  {VarMigrateURL, "::"},
	} {
		if _, err := Load(env(devBase, kv...)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Load(env(prodBase, VarMigrateOnStart, "true")); err == nil || !strings.Contains(err.Error(), VarMigrateURL) {
		t.Fatalf("prod migrate-on-start without migrate URL: %v", err)
	}
	c, err = Load(env(devBase, VarMigrateURL, "postgres://owner:x@localhost/clusterx"))
	if err != nil || c.MigrationURL() == c.DatabaseURL {
		t.Fatalf("migrate url: %v", err)
	}
}
