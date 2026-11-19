package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// SessionSecretPrefix starts every session secret (OpenAPI SessionCreated.secret: cxs_<base64url 256-bit>).
const SessionSecretPrefix = "cxs_"

// SessionTiming are the Go-enforced session lifetimes (ADR-0006 §3; owner may tune).
type SessionTiming struct {
	Absolute time.Duration // sessions.expires_at = created + Absolute
	Idle     time.Duration // sessions.idle_timeout_seconds
}

// DefaultSessionTiming is idle 2 h, absolute 12 h.
func DefaultSessionTiming() SessionTiming {
	return SessionTiming{Absolute: 12 * time.Hour, Idle: 2 * time.Hour}
}

// Session is a session row joined with its user.
type Session struct {
	ID, UserID, Login, Role   string
	CreatedAt, ExpiresAt      time.Time
	LastActivityAt, IdleUntil time.Time
	IdleTimeoutSeconds        int
}

// SessionCreated is returned once by CreateSession.
type SessionCreated struct {
	Session
	Secret string
}

// AuditMeta carries request context for audit rows.
type AuditMeta struct {
	RequestID string
	Kid       string
}

func newSessionSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return SessionSecretPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

const sessionCols = `s.id, s.user_id, u.github_login, u.role, s.created_at, s.expires_at, s.last_activity_at,
	s.last_activity_at + make_interval(secs => s.idle_timeout_seconds), s.idle_timeout_seconds`

func scanSession(r pgx.Row, s *Session, extra ...any) error {
	return r.Scan(append([]any{&s.ID, &s.UserID, &s.Login, &s.Role, &s.CreatedAt, &s.ExpiresAt, &s.LastActivityAt,
		&s.IdleUntil, &s.IdleTimeoutSeconds}, extra...)...)
}

// CreateSession implements POST /api/v1/sessions after the login assertion was verified (ADR-0006 §5):
// allowlist check by numeric GitHub id, login refresh, session row with only SHA-256 of a fresh secret,
// and the security audit row session.create in the same transaction. Denials: not_allowlisted,
// user_disabled (both audited as security/denied, committed).
func (s *Store) CreateSession(ctx context.Context, githubID int64, login string, t SessionTiming, m AuditMeta) (SessionCreated, error) {
	secret, err := newSessionSecret()
	if err != nil {
		return SessionCreated{}, err
	}
	var out SessionCreated
	var denial error
	err = s.inTx(ctx, false, func(tx pgx.Tx) error {
		denial = nil
		var userID string
		var disabled bool
		err := tx.QueryRow(ctx, `SELECT id, disabled_at IS NOT NULL FROM users WHERE github_user_id = $1 FOR UPDATE`, githubID).Scan(&userID, &disabled)
		switch {
		case isNoRows(err):
			denial = deny("not_allowlisted")
		case err != nil:
			return err
		case disabled:
			denial = deny("user_disabled")
		}
		if denial != nil {
			reason := denial.(*Denial).Reason
			_, err := tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id, request_id, details)
				VALUES ('security', 'session.create', 'denied', $1, 'user', $2, NULLIF($3, ''),
				        jsonb_build_object('reason', $4::text, 'kid', $5::text))`,
				"github:"+strconv.FormatInt(githubID, 10), nullUUID(userID), m.RequestID, reason, m.Kid)
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET github_login = $2 WHERE id = $1 AND github_login IS DISTINCT FROM $2`, userID, login); err != nil {
			return err
		}
		err = scanSession(tx.QueryRow(ctx, `WITH ins AS (
				INSERT INTO sessions (user_id, secret_hash, created_at, last_activity_at, expires_at, idle_timeout_seconds)
				VALUES ($1, $2, clock_timestamp(), clock_timestamp(), clock_timestamp() + $3::interval, $4)
				RETURNING *)
			SELECT `+sessionCols+` FROM ins s JOIN users u ON u.id = s.user_id`,
			userID, HashSecret(secret), t.Absolute, int(t.Idle/time.Second)), &out.Session)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id, request_id, details)
			VALUES ('security', 'session.create', 'allowed', 'user:' || $1, 'session', $2, NULLIF($3, ''),
			        jsonb_build_object('kid', $4::text, 'github_user_id', $5::bigint))`,
			userID, out.ID, m.RequestID, m.Kid, githubID)
		return err
	})
	if err != nil {
		return SessionCreated{}, err
	}
	if denial != nil {
		return SessionCreated{}, denial
	}
	out.Secret = secret
	return out, nil
}

// SessionCheck is the outcome of AuthenticateSession.
type SessionCheck struct {
	Session
	SubMatches bool // sessions.user_id = sub (RV01-11)
}

// AuthenticateSession re-checks a verified assertion against PostgreSQL on every request (no cache):
// session exists, belongs to sub (RV01-11), user not disabled, not revoked, not past absolute expiry and
// not idle-expired (GAP-SESS-01). Denial reasons (OpenAPI details.reason): assertion_invalid (sid of
// another user), session_revoked, session_expired (Detail idle|absolute), user_disabled. With touch, it
// bumps last_activity_at at most once per 60 s in its own statement.
func (s *Store) AuthenticateSession(ctx context.Context, sid, sub string, touch bool) (Session, error) {
	var out Session
	var subOK, disabled, revoked, absExpired, idleExpired bool
	err := scanSession(s.Pool.QueryRow(ctx, `SELECT `+sessionCols+`, s.user_id = $2::uuid, u.disabled_at IS NOT NULL,
			s.revoked_at IS NOT NULL, s.expires_at <= clock_timestamp(),
			s.last_activity_at + make_interval(secs => s.idle_timeout_seconds) <= clock_timestamp()
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.id = $1`, sid, sub),
		&out, &subOK, &disabled, &revoked, &absExpired, &idleExpired)
	switch {
	case isNoRows(err):
		// Unknown sid: deleted by retention or never issued. The BFF treats it as signed out.
		return Session{}, deny("session_revoked")
	case err != nil:
		return Session{}, classifyDBError(err)
	case !subOK:
		return Session{}, &Denial{Reason: "assertion_invalid", Detail: "sid belongs to another user"}
	case disabled:
		return Session{}, deny("user_disabled")
	case revoked:
		return Session{}, deny("session_revoked")
	case absExpired:
		return Session{}, &Denial{Reason: "session_expired", Detail: "absolute"}
	case idleExpired:
		return Session{}, &Denial{Reason: "session_expired", Detail: "idle"}
	}
	if touch {
		var la time.Time
		err := s.Pool.QueryRow(ctx, `UPDATE sessions SET last_activity_at = clock_timestamp()
			WHERE id = $1 AND revoked_at IS NULL AND last_activity_at < clock_timestamp() - interval '60 seconds'
			RETURNING last_activity_at`, sid).Scan(&la)
		if err == nil {
			out.LastActivityAt = la
			out.IdleUntil = la.Add(time.Duration(out.IdleTimeoutSeconds) * time.Second)
		}
	}
	return out, nil
}

// RevokeSession implements DELETE /api/v1/sessions/current (logout) with audit session.revoke.
func (s *Store) RevokeSession(ctx context.Context, sid, actor string, m AuditMeta) error {
	return s.inTx(ctx, false, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = clock_timestamp() WHERE id = $1 AND revoked_at IS NULL`, sid)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return deny("session_revoked")
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id, request_id, details)
			VALUES ('security', 'session.revoke', 'allowed', $1, 'session', $2, NULLIF($3, ''), jsonb_build_object('kid', $4::text))`,
			actor, sid, m.RequestID, m.Kid)
		return err
	})
}

// UpsertUser provisions an allowlisted user (bootstrap CLI). An existing user's role is updated.
func (s *Store) UpsertUser(ctx context.Context, githubID int64, login, role string) (string, error) {
	var id string
	err := s.inTx(ctx, false, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO users (github_user_id, github_login, role) VALUES ($1, $2, $3)
			ON CONFLICT (github_user_id) DO UPDATE SET role = EXCLUDED.role RETURNING id`, githubID, login, role).Scan(&id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id, details)
			VALUES ('privileged', 'user.upsert', 'allowed', 'system:cli', 'user', $1, jsonb_build_object('role', $2::text))`, id, role)
		return err
	})
	return id, err
}

// SetUserDisabled disables or re-enables a user (bootstrap CLI).
func (s *Store) SetUserDisabled(ctx context.Context, githubID int64, disabled bool) error {
	return s.inTx(ctx, false, func(tx pgx.Tx) error {
		var id string
		err := tx.QueryRow(ctx, `UPDATE users SET disabled_at = CASE WHEN $2 THEN coalesce(disabled_at, clock_timestamp()) END
			WHERE github_user_id = $1 RETURNING id`, githubID, disabled).Scan(&id)
		if isNoRows(err) {
			return deny("user_not_found")
		} else if err != nil {
			return err
		}
		action := "user.enable"
		if disabled {
			action = "user.disable"
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id)
			VALUES ('privileged', $2, 'allowed', 'system:cli', 'user', $1)`, id, action)
		return err
	})
}

func nullUUID(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
