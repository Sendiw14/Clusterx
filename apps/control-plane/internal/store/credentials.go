package store

import (
	"context"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// APIToken is an api_tokens row without secret material.
type APIToken struct {
	ID, Name, Role, Prefix, OwnerUserID string
	Scopes                              []string
	CreatedAt                           time.Time
	ExpiresAt, RevokedAt, LastUsedAt    *time.Time
}

// WorkerCredential is a worker_credentials row without secret material.
type WorkerCredential struct {
	ID, Name, Prefix                 string
	CreatedBy                        *string
	CreatedAt                        time.Time
	ExpiresAt, RevokedAt, LastUsedAt *time.Time
	BoundWorkerIDs                   []string
}

// Credential idempotency (OpenAPI createApiToken/createWorkerCredential: 24 h replay without the secret).
// submission_idempotency references jobs, so credential keys are recorded in the create audit row
// (details.idempotency_key, details.fingerprint) and looked up there; a transaction-scoped advisory lock
// on (actor, action, key) serializes concurrent duplicates. Audit rows are immutable and kept 90 days.
func credentialIdem(ctx context.Context, tx pgx.Tx, actor, action, key, fp string) (string, bool, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, actor+"|"+action+"|"+key); err != nil {
		return "", false, err
	}
	var target, oldFP string
	err := tx.QueryRow(ctx, `SELECT target_id::text, details->>'fingerprint' FROM audit_events
		WHERE action = $1 AND actor = $2 AND outcome = 'allowed' AND details ? 'idempotency_key'
		  AND details->>'idempotency_key' = $3
		  AND occurred_at > clock_timestamp() - interval '24 hours'
		ORDER BY occurred_at DESC LIMIT 1`, action, actor, key).Scan(&target, &oldFP)
	if isNoRows(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if oldFP != fp {
		return "", false, &Denial{Reason: "idempotency_conflict", Detail: target}
	}
	return target, true, nil
}

// TokenCreate is a validated POST /tokens body.
type TokenCreate struct {
	Name          string
	Role          string
	Scopes        []string
	ExpiresInDays int
}

func validateTokenCreate(p TokenCreate) error {
	switch {
	case p.Role != "operator" && p.Role != "viewer":
		return fmt.Errorf("role must be operator or viewer")
	case len(p.Name) < 1 || len(p.Name) > 100:
		return fmt.Errorf("name must be 1..100 characters")
	case p.ExpiresInDays < 1 || p.ExpiresInDays > 365:
		return fmt.Errorf("expires-in-days must be 1..365")
	case len(p.Scopes) < 1 || len(p.Scopes) > 6:
		return fmt.Errorf("1..6 scopes required")
	}
	seen := map[string]bool{}
	for _, sc := range p.Scopes {
		if !slices.Contains(AllScopes, sc) || seen[sc] {
			return fmt.Errorf("invalid or duplicate scope %q", sc)
		}
		if p.Role == "viewer" && !strings.HasSuffix(sc, ":read") {
			return fmt.Errorf("viewer tokens may only hold *:read scopes (got %q)", sc)
		}
		seen[sc] = true
	}
	return nil
}

// CreateTokenFor issues a token owned by ownerUserID (REST, operator session). The token's role ceiling
// may not exceed the owner's current role (RV01-02: a token never outranks its owner; enforced again at
// use time by api_token_authority). Denials: role_not_allowed, idempotency_conflict. replayed=true
// returns the original token without a secret.
func (s *Store) CreateTokenFor(ctx context.Context, ownerUserID string, p TokenCreate, key, fp string, m AuditMeta) (APIToken, string, bool, error) {
	if err := validateTokenCreate(p); err != nil {
		return APIToken{}, "", false, &Denial{Reason: "validation_failed", Detail: err.Error()}
	}
	secret, prefix, hash, err := NewSecret(APITokenPrefix)
	if err != nil {
		return APIToken{}, "", false, err
	}
	actor := "user:" + ownerUserID
	var id string
	replayed := false
	err = s.inTx(ctx, false, func(tx pgx.Tx) error {
		id, replayed = "", false
		var ownerRole string
		if err := tx.QueryRow(ctx, `SELECT role FROM users WHERE id = $1 AND disabled_at IS NULL FOR SHARE`, ownerUserID).Scan(&ownerRole); err != nil {
			if isNoRows(err) {
				return deny("user_disabled")
			}
			return err
		}
		if p.Role == "operator" && ownerRole != "operator" {
			return deny("role_not_allowed")
		}
		if key != "" {
			orig, ok, err := credentialIdem(ctx, tx, actor, "token.create", key, fp)
			if err != nil {
				return err
			}
			if ok {
				id, replayed = orig, true
				return nil
			}
		}
		if err := tx.QueryRow(ctx, `INSERT INTO api_tokens (name, owner_user_id, role, scopes, secret_hash, prefix, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, clock_timestamp(), clock_timestamp() + make_interval(days => $7::int)) RETURNING id`,
			p.Name, ownerUserID, p.Role, p.Scopes, hash, prefix, p.ExpiresInDays).Scan(&id); err != nil {
			return err
		}
		d := map[string]any{"prefix": prefix, "role": p.Role, "scopes": p.Scopes, "kid": m.Kid}
		if key != "" {
			d["idempotency_key"], d["fingerprint"] = key, fp
		}
		return insertAudit(ctx, tx, AuditRow{Category: "privileged", Action: "token.create", Outcome: "allowed", Actor: actor,
			TargetType: "api_token", TargetID: id, RequestID: m.RequestID, Details: d})
	})
	if err != nil {
		return APIToken{}, "", false, err
	}
	t, err := s.GetAPIToken(ctx, id)
	if replayed {
		return t, "", true, err
	}
	return t, secret, false, err
}

const tokenCols = `id, name, role, scopes, prefix, owner_user_id, created_at, expires_at, revoked_at, last_used_at`

func scanToken(r pgx.Row) (APIToken, error) {
	var t APIToken
	err := r.Scan(&t.ID, &t.Name, &t.Role, &t.Scopes, &t.Prefix, &t.OwnerUserID, &t.CreatedAt, &t.ExpiresAt, &t.RevokedAt, &t.LastUsedAt)
	return t, err
}

// GetAPIToken reads one token. Denial: token_not_found.
func (s *Store) GetAPIToken(ctx context.Context, id string) (APIToken, error) {
	t, err := scanToken(s.Pool.QueryRow(ctx, `SELECT `+tokenCols+` FROM api_tokens WHERE id = $1`, id))
	if isNoRows(err) {
		return APIToken{}, deny("token_not_found")
	}
	return t, classifyDBError(err)
}

// RevokeTokenBy revokes a token (idempotent: already revoked keeps revoked_at, no new audit row).
// The next request with it gets 401 token_revoked (no authentication cache).
func (s *Store) RevokeTokenBy(ctx context.Context, id, actor string, m AuditMeta) (APIToken, error) {
	err := s.inTx(ctx, false, func(tx pgx.Tx) error {
		var revoked bool
		err := tx.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM api_tokens WHERE id = $1 FOR UPDATE`, id).Scan(&revoked)
		if isNoRows(err) {
			return deny("token_not_found")
		} else if err != nil || revoked {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE api_tokens SET revoked_at = clock_timestamp() WHERE id = $1 AND revoked_at IS NULL`, id); err != nil {
			return err
		}
		return insertAudit(ctx, tx, AuditRow{Category: "privileged", Action: "token.revoke", Outcome: "allowed", Actor: actor,
			TargetType: "api_token", TargetID: id, RequestID: m.RequestID, Details: map[string]any{"kid": m.Kid}})
	})
	if err != nil {
		return APIToken{}, err
	}
	return s.GetAPIToken(ctx, id)
}

// Page is a keyset page request over (created_at DESC, id DESC).
type Page struct {
	IncludeRevoked bool
	Limit          int
	After          *Cursor
}

// ListAPITokens pages tokens by (created_at DESC, id DESC).
func (s *Store) ListAPITokens(ctx context.Context, p Page) ([]APIToken, bool, time.Time, error) {
	b := &sqlb{conds: []string{"true"}}
	if !p.IncludeRevoked {
		b.where("revoked_at IS NULL")
	}
	if p.After != nil {
		b.where("(created_at, id) < (" + b.arg(p.After.T) + ", " + b.arg(p.After.ID) + "::uuid)")
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+tokenCols+` FROM api_tokens WHERE `+strings.Join(b.conds, " AND ")+
		` ORDER BY created_at DESC, id DESC LIMIT `+b.arg(p.Limit+1), b.args...)
	if err != nil {
		return nil, false, time.Time{}, classifyDBError(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (APIToken, error) { return scanToken(r) })
	if err != nil {
		return nil, false, time.Time{}, classifyDBError(err)
	}
	more := len(out) > p.Limit
	if more {
		out = out[:p.Limit]
	}
	now, err := s.Now(ctx)
	return out, more, now, err
}

// CreateWorkerCredentialBy issues a worker credential from the REST flow. expiresInDays 0 = no expiry.
func (s *Store) CreateWorkerCredentialBy(ctx context.Context, name string, expiresInDays int, actorUserID, key, fp string, m AuditMeta) (WorkerCredential, string, bool, error) {
	if l := len(name); l < 1 || l > 100 {
		return WorkerCredential{}, "", false, &Denial{Reason: "validation_failed", Detail: "name must be 1..100 characters"}
	}
	if expiresInDays < 0 || expiresInDays > 730 {
		return WorkerCredential{}, "", false, &Denial{Reason: "validation_failed", Detail: "expires_in_days must be 1..730"}
	}
	secret, prefix, hash, err := NewSecret(WorkerCredentialPrefix)
	if err != nil {
		return WorkerCredential{}, "", false, err
	}
	actor := "user:" + actorUserID
	var id string
	replayed := false
	err = s.inTx(ctx, false, func(tx pgx.Tx) error {
		id, replayed = "", false
		if key != "" {
			orig, ok, err := credentialIdem(ctx, tx, actor, "worker_credential.create", key, fp)
			if err != nil {
				return err
			}
			if ok {
				id, replayed = orig, true
				return nil
			}
		}
		if err := tx.QueryRow(ctx, `INSERT INTO worker_credentials (name, secret_hash, prefix, created_by, created_at, expires_at)
			VALUES ($1, $2, $3, $4, clock_timestamp(), CASE WHEN $5::int > 0 THEN clock_timestamp() + make_interval(days => $5::int) END)
			RETURNING id`, name, hash, prefix, actorUserID, expiresInDays).Scan(&id); err != nil {
			return err
		}
		d := map[string]any{"prefix": prefix, "kid": m.Kid}
		if key != "" {
			d["idempotency_key"], d["fingerprint"] = key, fp
		}
		return insertAudit(ctx, tx, AuditRow{Category: "privileged", Action: "worker_credential.create", Outcome: "allowed", Actor: actor,
			TargetType: "worker_credential", TargetID: id, RequestID: m.RequestID, Details: d})
	})
	if err != nil {
		return WorkerCredential{}, "", false, err
	}
	c, err := s.GetWorkerCredential(ctx, id)
	if replayed {
		return c, "", true, err
	}
	return c, secret, false, err
}

const credCols = `c.id, c.name, c.prefix, c.created_by::text, c.created_at, c.expires_at, c.revoked_at, c.last_used_at,
	coalesce((SELECT array_agg(w.id::text ORDER BY w.id) FROM workers w WHERE w.credential_id = c.id), '{}')`

func scanCred(r pgx.Row) (WorkerCredential, error) {
	var c WorkerCredential
	err := r.Scan(&c.ID, &c.Name, &c.Prefix, &c.CreatedBy, &c.CreatedAt, &c.ExpiresAt, &c.RevokedAt, &c.LastUsedAt, &c.BoundWorkerIDs)
	return c, err
}

// GetWorkerCredential reads one credential. Denial: worker_credential_not_found.
func (s *Store) GetWorkerCredential(ctx context.Context, id string) (WorkerCredential, error) {
	c, err := scanCred(s.Pool.QueryRow(ctx, `SELECT `+credCols+` FROM worker_credentials c WHERE c.id = $1`, id))
	if isNoRows(err) {
		return WorkerCredential{}, deny("worker_credential_not_found")
	}
	return c, classifyDBError(err)
}

// RevokeWorkerCredentialBy revokes from the REST flow (lock, then stamp; see RevokeWorkerCredential).
// Idempotent: already revoked keeps revoked_at and writes no audit row.
func (s *Store) RevokeWorkerCredentialBy(ctx context.Context, id, actorUserID string, m AuditMeta) (WorkerCredential, error) {
	err := s.inTx(ctx, false, func(tx pgx.Tx) error {
		var revoked bool
		err := tx.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM worker_credentials WHERE id = $1 FOR UPDATE`, id).Scan(&revoked)
		if isNoRows(err) {
			return deny("worker_credential_not_found")
		} else if err != nil || revoked {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE worker_credentials SET revoked_at = clock_timestamp(), revoked_by = $2
			WHERE id = $1 AND revoked_at IS NULL`, id, actorUserID); err != nil {
			return err
		}
		return insertAudit(ctx, tx, AuditRow{Category: "privileged", Action: "worker_credential.revoke", Outcome: "allowed",
			Actor: "user:" + actorUserID, TargetType: "worker_credential", TargetID: id, RequestID: m.RequestID, Details: map[string]any{"kid": m.Kid}})
	})
	if err != nil {
		return WorkerCredential{}, err
	}
	return s.GetWorkerCredential(ctx, id)
}

// ListWorkerCredentials pages credentials by (created_at DESC, id DESC).
func (s *Store) ListWorkerCredentials(ctx context.Context, p Page) ([]WorkerCredential, bool, time.Time, error) {
	b := &sqlb{conds: []string{"true"}}
	if !p.IncludeRevoked {
		b.where("c.revoked_at IS NULL")
	}
	if p.After != nil {
		b.where("(c.created_at, c.id) < (" + b.arg(p.After.T) + ", " + b.arg(p.After.ID) + "::uuid)")
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+credCols+` FROM worker_credentials c WHERE `+strings.Join(b.conds, " AND ")+
		` ORDER BY c.created_at DESC, c.id DESC LIMIT `+b.arg(p.Limit+1), b.args...)
	if err != nil {
		return nil, false, time.Time{}, classifyDBError(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (WorkerCredential, error) { return scanCred(r) })
	if err != nil {
		return nil, false, time.Time{}, classifyDBError(err)
	}
	more := len(out) > p.Limit
	if more {
		out = out[:p.Limit]
	}
	now, err := s.Now(ctx)
	return out, more, now, err
}

// Fingerprint returns the hex SHA-256 used for credential idempotency.
func Fingerprint(b []byte) string { return hex.EncodeToString(sha256Bytes(b)) }
