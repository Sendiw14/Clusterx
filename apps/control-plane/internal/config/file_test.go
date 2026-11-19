package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RV5-09: every secret env has a *_FILE variant; both set is an error; unreadable files fail closed
// without echoing contents.
func TestSecretFileVariants(t *testing.T) {
	dir := t.TempDir()
	write := func(name, v string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(v+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	db := write("db", "postgres://clusterx_app:Zq8vN3x7Lr2pTt6Wk9yB@db:5432/clusterx")
	mig := write("mig", "postgres://admin:Ab8vN3x7Lr2pTt6Wk9yC@db:5432/clusterx")
	ret := write("ret", "postgres://ret:Cd8vN3x7Lr2pTt6Wk9yD@db:5432/clusterx")
	tok := write("tok", goodToken)
	c, err := Load(env(map[string]string{VarEnv: "development", VarDatabaseURL + "_FILE": db, VarMigrateURL + "_FILE": mig,
		VarRetentionURL + "_FILE": ret, VarMetricsToken + "_FILE": tok}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.DatabaseURL, "clusterx_app") || !strings.Contains(c.MigrateURL, "admin") ||
		!strings.Contains(c.RetentionURL, "ret:") || c.MetricsToken != goodToken {
		t.Fatalf("file values not loaded: %+v", c)
	}
	if _, err := Load(env(devBase, VarDatabaseURL+"_FILE", db)); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("both set: %v", err)
	}
	_, err = Load(env(devBase, VarDatabaseURL, "<unset>", VarDatabaseURL+"_FILE", filepath.Join(dir, "missing")))
	if err == nil || strings.Contains(err.Error(), "Zq8v") {
		t.Fatalf("missing file: %v", err)
	}
}
