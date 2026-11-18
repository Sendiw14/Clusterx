package store

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Scopes (OpenAPI Scope).
var AllScopes = []string{"jobs:read", "jobs:write", "workers:read", "workers:write", "metrics:read", "audit:read"}

// TokenParams creates an API token through the server-side bootstrap CLI. The owner user is
// provisioned from an explicit GitHub numeric id (FR-SEC-01 allowlist) when it does not exist yet.
type TokenParams struct {
	OwnerGitHubID int64
	OwnerLogin    string
	Role          string // ceiling: operator | viewer
	Name          string
	Scopes        []string
	ExpiresInDays int // 1..365
}

// CreateAPIToken validates and inserts a token; the secret is returned once and only its hash is stored.
func (s *Store) CreateAPIToken(ctx context.Context, p TokenParams) (CreatedSecret, error) {
	switch {
	case p.Role != "operator" && p.Role != "viewer":
		return CreatedSecret{}, fmt.Errorf("role must be operator or viewer")
	case p.OwnerGitHubID <= 0:
		return CreatedSecret{}, fmt.Errorf("owner GitHub numeric id must be > 0")
	case len(p.OwnerLogin) < 1 || len(p.OwnerLogin) > 39:
		return CreatedSecret{}, fmt.Errorf("owner login must be 1..39 characters")
	case len(p.Name) < 1 || len(p.Name) > 100:
		return CreatedSecret{}, fmt.Errorf("name must be 1..100 characters")
	case p.ExpiresInDays < 1 || p.ExpiresInDays > 365:
		return CreatedSecret{}, fmt.Errorf("expires-in-days must be 1..365")
	case len(p.Scopes) < 1 || len(p.Scopes) > 6:
		return CreatedSecret{}, fmt.Errorf("1..6 scopes required")
	}
	seen := map[string]bool{}
	for _, sc := range p.Scopes {
		if !slices.Contains(AllScopes, sc) || seen[sc] {
			return CreatedSecret{}, fmt.Errorf("invalid or duplicate scope %q", sc)
		}
		if p.Role == "viewer" && !strings.HasSuffix(sc, ":read") {
			return CreatedSecret{}, fmt.Errorf("viewer tokens may only hold *:read scopes (got %q)", sc)
		}
		seen[sc] = true
	}
	secret, prefix, hash, err := NewSecret(APITokenPrefix)
	if err != nil {
		return CreatedSecret{}, err
	}
	out := CreatedSecret{Prefix: prefix, Secret: secret}
	err = s.inTx(ctx, false, func(tx pgx.Tx) error {
		var owner string
		if err := tx.QueryRow(ctx, `INSERT INTO users (github_user_id, github_login, role) VALUES ($1, $2, $3)
			ON CONFLICT (github_user_id) DO UPDATE SET github_login = users.github_login RETURNING id`,
			p.OwnerGitHubID, p.OwnerLogin, p.Role).Scan(&owner); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO api_tokens (name, owner_user_id, role, scopes, secret_hash, prefix, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, now() + make_interval(days => $7::int)) RETURNING id, expires_at`,
			p.Name, owner, p.Role, p.Scopes, hash, prefix, p.ExpiresInDays).Scan(&out.ID, &out.ExpiresAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id, details)
			VALUES ('privileged', 'token.create', 'allowed', 'system:cli', 'api_token', $1,
			        jsonb_build_object('prefix', $2::text, 'role', $3::text))`, out.ID, prefix, p.Role)
		return err
	})
	return out, err
}

// RevokeAPIToken revokes a token by id (CLI; the REST route is M5).
func (s *Store) RevokeAPIToken(ctx context.Context, id string) error {
	return s.inTx(ctx, false, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE api_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return deny("token_not_found")
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id)
			VALUES ('privileged', 'token.revoke', 'allowed', 'system:cli', 'api_token', $1)`, id)
		return err
	})
}

// Principal is an authenticated API caller.
type Principal struct {
	Ref    string // "token:<uuid>"
	Role   string // effective role
	Scopes []string
}

// Has reports whether the principal holds scope.
func (p Principal) Has(scope string) bool { return slices.Contains(p.Scopes, scope) }

// AuthenticateToken resolves a direct API token through api_token_authority on every request (no
// cache). Denial reasons (OpenAPI details.reason): token_invalid, token_revoked, token_expired, user_disabled.
func (s *Store) AuthenticateToken(ctx context.Context, secret string) (Principal, error) {
	if !WellFormedSecret(APITokenPrefix, secret) {
		return Principal{}, deny("token_invalid")
	}
	var id, role, status string
	var scopes []string
	err := s.Pool.QueryRow(ctx, `SELECT token_id, effective_role, effective_scopes, status FROM api_token_authority WHERE secret_hash = $1`,
		HashSecret(secret)).Scan(&id, &role, &scopes, &status)
	if isNoRows(err) {
		return Principal{}, deny("token_invalid")
	}
	if err != nil {
		return Principal{}, classifyDBError(err)
	}
	if status != "ok" {
		return Principal{}, deny(status)
	}
	// Usage bump, throttled, in its own statement (ADR-0005: asynchronous, never in a mutation tx).
	_, _ = s.Pool.Exec(ctx, `UPDATE api_tokens SET last_used_at = now() WHERE id = $1
		AND (last_used_at IS NULL OR last_used_at < now() - interval '60 seconds')`, id)
	return Principal{Ref: "token:" + id, Role: role, Scopes: scopes}, nil
}
