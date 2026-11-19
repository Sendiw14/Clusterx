package config

import (
	"maps"
	"os"
	"strings"
	"testing"
)

const cred = "cxw_abcdef_0123456789abcdefghijklmnopqrstuv"

func env(base map[string]string, kv ...string) func(string) (string, bool) {
	m := maps.Clone(base)
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == "<unset>" {
			delete(m, kv[i])
		} else {
			m[kv[i]] = kv[i+1]
		}
	}
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

var dev = map[string]string{VarEnv: "development", VarAddr: "control-plane:9090", VarInsecure: "true"}
var prod = map[string]string{VarEnv: "production", VarAddr: "cp.internal:9090", VarCredential: cred}

func TestLoad(t *testing.T) {
	cases := []struct {
		name    string
		lookup  func(string) (string, bool)
		wantErr string
	}{
		{"dev ok", env(dev), ""},
		{"prod ok", env(prod), ""},
		{"missing env", env(dev, VarEnv, "<unset>"), VarEnv + " is required"},
		{"bad env", env(dev, VarEnv, "staging"), "must be development or production"},
		{"missing addr", env(dev, VarAddr, "<unset>"), VarAddr + " is required"},
		{"bad addr", env(dev, VarAddr, "nohost"), "must be host:port"},
		{"prod no credential", env(prod, VarCredential, "<unset>"), VarCredential + " is required in production"},
		{"bad credential prefix", env(dev, VarCredential, "cxt_"+cred[4:]), "must start with"},
		{"short credential", env(dev, VarCredential, "cxw_short"), "at least 32"},
		{"prod insecure remote", env(prod, VarInsecure, "true"), "not allowed in production"},
		{"prod insecure loopback", env(prod, VarInsecure, "true", VarAddr, "127.0.0.1:9090"), ""},
		{"insecure plus ca", env(dev, VarTLSCAFile, "/ca.pem"), "conflicts with TLS"},
		{"bad insecure", env(dev, VarInsecure, "yes please"), "must be a boolean"},
		{"capacity zero", env(dev, VarCapacity, "0"), VarCapacity},
		{"capacity big", env(dev, VarCapacity, "257"), VarCapacity},
		{"long name", env(dev, VarName, strings.Repeat("n", 101)), VarName},
		{"bad level", env(dev, VarLogLevel, "loud"), VarLogLevel},
		{"bad shutdown", env(dev, VarShutdownTimeout, "0s"), VarShutdownTimeout},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := Load(c.lookup)
			switch {
			case c.wantErr == "" && err != nil:
				t.Fatalf("unexpected: %v", err)
			case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
				t.Fatalf("want %q, got %v", c.wantErr, err)
			case err != nil && strings.Contains(err.Error(), cred[8:]):
				t.Fatalf("error leaks credential: %v", err)
			case err == nil && (cfg.Capacity != 4 || cfg.Name == ""):
				t.Fatalf("defaults: %+v", cfg)
			}
		})
	}
}

func TestWorkerIDFile(t *testing.T) {
	p := t.TempDir() + "/id"
	if id, err := ReadWorkerID(p); err != nil || id != "" {
		t.Fatalf("missing file: %q %v", id, err)
	}
	const id = "0193a0f2-7c1e-7a10-8000-0000000000a1"
	if err := WriteWorkerID(p, id); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadWorkerID(p); err != nil || got != id {
		t.Fatalf("round trip: %q %v", got, err)
	}
	if err := os.WriteFile(p, []byte("../../etc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadWorkerID(p); err == nil {
		t.Fatal("malformed id accepted")
	}
}
