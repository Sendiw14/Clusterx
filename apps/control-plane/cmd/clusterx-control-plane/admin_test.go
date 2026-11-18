package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/config"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
)

func field(out, key string) string {
	for _, l := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(l, key+"="); ok {
			return v
		}
	}
	return ""
}

func TestBootstrapCommands(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 4)
	ctx := context.Background()
	cfg := config.Config{DatabaseURL: env.DSN}
	var out bytes.Buffer
	if err := runAdmin(ctx, cfg, []string{"token", "create", "--role", "operator", "--name", "ops", "--owner-github-id", "42", "--owner-login", "octo"}, &out); err != nil {
		t.Fatal(err)
	}
	secret := field(out.String(), "secret")
	if !strings.HasPrefix(secret, "cxt_") || strings.Count(out.String(), secret) != 1 {
		t.Fatalf("output: %s", out.String())
	}
	p, err := env.Store.AuthenticateToken(ctx, secret)
	if err != nil || p.Role != "operator" || len(p.Scopes) != 6 {
		t.Fatalf("auth: %+v %v", p, err)
	}
	var stored int
	if err := env.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM api_tokens WHERE secret_hash = sha256(convert_to($1, 'UTF8')) AND prefix = $2`, secret, field(out.String(), "prefix")).Scan(&stored); err != nil || stored != 1 {
		t.Fatalf("hash stored: %d %v", stored, err)
	}
	if err := runAdmin(ctx, cfg, []string{"token", "create", "--role", "viewer", "--name", "v", "--owner-github-id", "43", "--owner-login", "v", "--scopes", "jobs:write"}, &out); err == nil {
		t.Fatal("viewer token with write scope accepted")
	}
	out.Reset()
	if err := runAdmin(ctx, cfg, []string{"worker-credential", "create", "--name", "w1"}, &out); err != nil {
		t.Fatal(err)
	}
	ws := field(out.String(), "secret")
	if _, err := env.Store.AuthenticateWorker(ctx, ws); err != nil {
		t.Fatal(err)
	}
	if err := runAdmin(ctx, cfg, []string{"worker-credential", "revoke", "--id", field(out.String(), "id")}, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Store.AuthenticateWorker(ctx, ws); err == nil {
		t.Fatal("revoked credential accepted")
	}
}
