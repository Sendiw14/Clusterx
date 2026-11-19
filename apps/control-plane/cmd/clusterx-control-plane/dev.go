package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/auth"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/config"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

// errDevOnly is returned by the development helpers in production.
var errDevOnly = errors.New("development-only command refused: CLUSTERX_ENV=production")

// runDev implements the local helpers for the web lane (ADR-0006): `dev-keygen` writes an Ed25519 key
// pair (private JWK for the local BFF, public JWKS for CLUSTERX_BFF_JWKS_FILE) and `dev-assertion` mints
// one assertion with a private JWK. Both refuse to run when CLUSTERX_ENV=production. They never contact
// the database.
func runDev(cfg config.Config, args []string, out io.Writer) error {
	if cfg.Env == config.Production {
		return errDevOnly
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(out)
	switch args[0] {
	case "dev-keygen":
		dir := fs.String("out-dir", ".", "directory for bff-dev-private.jwk.json (0600) and bff-dev.jwks.json")
		kid := fs.String("kid", "dev-1", "key id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		pj, err := auth.PrivateJWK(*kid, priv)
		if err != nil {
			return err
		}
		pub, err := auth.PublicJWKS(*kid, priv)
		if err != nil {
			return err
		}
		privPath, pubPath := filepath.Join(*dir, "bff-dev-private.jwk.json"), filepath.Join(*dir, "bff-dev.jwks.json")
		if err := os.WriteFile(privPath, pj, 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(pubPath, pub, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "private_jwk=%s\npublic_jwks=%s\n# Set %s=%s on the coordinator. The private key is for local development only.\n",
			privPath, pubPath, config.VarBFFJWKSFile, pubPath)
		return nil
	case "dev-assertion":
		keyFile := fs.String("key-file", "", "private JWK written by dev-keygen")
		login := fs.Int64("login", 0, "mint a login assertion for this numeric GitHub id (POST /api/v1/sessions)")
		sub := fs.String("sub", "", "user:<uuid> (normal assertion)")
		sid := fs.String("sid", "", "session id (normal assertion)")
		role := fs.String("role", "viewer", "role hint (operator|viewer); Go uses the DB role")
		ttl := fs.Duration("ttl", 60*time.Second, "1s..60s")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		b, err := os.ReadFile(*keyFile)
		if err != nil {
			return err
		}
		kid, priv, err := auth.ParsePrivateJWK(b)
		if err != nil {
			return err
		}
		if *ttl < time.Second || *ttl > auth.MaxTTL {
			return fmt.Errorf("--ttl must be 1s..60s")
		}
		now := time.Now()
		c := auth.Claims{Exp: now.Add(*ttl).Unix()}
		if *login > 0 {
			c.Sub, c.Purpose = fmt.Sprintf("github:%d", *login), "login"
		} else {
			c.Sub, c.Sid, c.Role = *sub, *sid, *role
		}
		tok, err := auth.Mint(kid, priv, c, now)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, tok)
		return nil
	}
	return fmt.Errorf("unknown command %q", args[0])
}

// runRetentionOnce runs one bounded retention pass as the clusterx_retention role and prints the counts.
func runRetentionOnce(ctx context.Context, cfg config.Config, log *slog.Logger, out io.Writer) error {
	if cfg.RetentionURL == "" {
		return fmt.Errorf("%s is required (a login role in clusterx_retention)", config.VarRetentionURL)
	}
	pool, err := store.OpenPool(ctx, cfg.RetentionURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	r, err := store.RunRetention(ctx, pool, store.DefaultRetentionBounds())
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "attempt_logs_expired=%d\njobs_deleted=%d\naudit_deleted=%d\ntombstones_deleted=%d\nidempotency_deleted=%d\nsessions_deleted=%d\ncomplete=%t\n",
		r.AttemptLogsExpired, r.JobsDeleted, r.AuditDeleted, r.TombstonesDeleted, r.IdempotencyDeleted, r.SessionsDeleted, r.Complete)
	log.Info("retention pass", "report", r)
	return nil
}

// runRetentionLoop runs a pass every CLUSTERX_RETENTION_INTERVAL on its own pool until ctx ends.
func runRetentionLoop(ctx context.Context, cfg config.Config, log *slog.Logger) {
	var pool *pgxpool.Pool
	defer func() {
		if pool != nil {
			pool.Close()
		}
	}()
	t := time.NewTicker(cfg.RetentionInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if pool == nil {
			p, err := store.OpenPool(ctx, cfg.RetentionURL)
			if err != nil {
				log.Warn("retention: connect failed", "err", err)
				continue
			}
			pool = p
		}
		r, err := store.RunRetention(ctx, pool, store.DefaultRetentionBounds())
		if err != nil {
			log.Warn("retention pass failed", "err", err)
			continue
		}
		log.Info("retention pass", "report", r)
	}
}
