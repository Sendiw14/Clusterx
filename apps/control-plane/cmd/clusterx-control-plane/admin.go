package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/config"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

// runAdmin implements the server-side bootstrap commands (FR-SEC-03, FR-SEC-05). They run with the
// runtime DSN, print the secret exactly once on out, and never log it.
func runAdmin(ctx context.Context, cfg config.Config, args []string, out io.Writer) error {
	pool, err := store.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	st := store.New(pool, store.DefaultTiming(), nil)
	fs := flag.NewFlagSet(args[0]+" "+args[1], flag.ContinueOnError)
	fs.SetOutput(out)
	switch args[0] + " " + args[1] {
	case "token create":
		role := fs.String("role", "", "operator | viewer (ceiling; effective authority also follows the owner's current role)")
		name := fs.String("name", "", "token name (1..100)")
		gh := fs.Int64("owner-github-id", 0, "owner's numeric GitHub user id (allowlisted user is created if absent)")
		login := fs.String("owner-login", "", "owner's GitHub login (used only when the user is created)")
		scopes := fs.String("scopes", "", "comma-separated scopes; default: all for operator, *:read for viewer")
		days := fs.Int("expires-in-days", 90, "1..365")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		sc := splitScopes(*scopes)
		if len(sc) == 0 {
			for _, s := range store.AllScopes {
				if *role == "operator" || strings.HasSuffix(s, ":read") {
					sc = append(sc, s)
				}
			}
		}
		c, err := st.CreateAPIToken(ctx, store.TokenParams{OwnerGitHubID: *gh, OwnerLogin: *login, Role: *role, Name: *name, Scopes: sc, ExpiresInDays: *days})
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "id=%s\nprefix=%s\nrole=%s\nscopes=%s\nexpires_at=%s\nsecret=%s\n# The secret is shown once and cannot be recovered.\n",
			c.ID, c.Prefix, *role, strings.Join(sc, ","), c.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"), c.Secret)
		return nil
	case "worker-credential create":
		name := fs.String("name", "", "credential name (1..100)")
		days := fs.Int("expires-in-days", 0, "0 = no expiry, else 1..730")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		c, err := st.CreateWorkerCredential(ctx, *name, *days)
		if err != nil {
			return err
		}
		exp := "never"
		if c.ExpiresAt != nil {
			exp = c.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		fmt.Fprintf(out, "id=%s\nprefix=%s\nexpires_at=%s\nsecret=%s\n# The secret is shown once and cannot be recovered.\n", c.ID, c.Prefix, exp, c.Secret)
		return nil
	case "user upsert":
		gh := fs.Int64("github-id", 0, "numeric GitHub user id to allowlist")
		login := fs.String("login", "", "GitHub login (display only)")
		role := fs.String("role", "", "operator | viewer")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *gh <= 0 || len(*login) < 1 || len(*login) > 39 || (*role != "operator" && *role != "viewer") {
			return fmt.Errorf("user upsert needs --github-id > 0, --login (1..39) and --role operator|viewer")
		}
		id, err := st.UpsertUser(ctx, *gh, *login, *role)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "user_id=%s\nrole=%s\n", id, *role)
		return nil
	case "user disable", "user enable":
		gh := fs.Int64("github-id", 0, "numeric GitHub user id")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if err := st.SetUserDisabled(ctx, *gh, args[1] == "disable"); err != nil {
			return err
		}
		fmt.Fprintf(out, "%sd=%d\n", args[1], *gh)
		return nil
	case "token revoke", "worker-credential revoke":
		id := fs.String("id", "", "id to revoke")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		var err error
		if args[0] == "token" {
			err = st.RevokeAPIToken(ctx, *id)
		} else {
			err = st.RevokeWorkerCredential(ctx, *id)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "revoked=%s\n", *id)
		return nil
	}
	return fmt.Errorf("unknown command %q", strings.Join(args[:2], " "))
}

func splitScopes(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
