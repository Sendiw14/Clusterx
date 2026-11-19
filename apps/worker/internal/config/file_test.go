package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RV5-09: CLUSTERX_WORKER_CREDENTIAL_FILE.
func TestCredentialFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cred")
	if err := os.WriteFile(p, []byte(cred+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(env(prod, VarCredential, "<unset>", VarCredential+"_FILE", p))
	if err != nil || c.Credential != cred {
		t.Fatalf("file credential: %v", err)
	}
	if _, err := Load(env(prod, VarCredential+"_FILE", p)); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("both set: %v", err)
	}
	if _, err := Load(env(prod, VarCredential, "<unset>", VarCredential+"_FILE", p+"x")); err == nil || strings.Contains(err.Error(), cred) {
		t.Fatalf("missing file: %v", err)
	}
}
