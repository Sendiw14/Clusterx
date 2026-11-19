package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Catalog is the coordinator's compiled handler allowlist.
type Catalog interface {
	Known(name string, version int) bool
}

// Secret prefixes (ADR-0005).
const (
	WorkerCredentialPrefix = "cxw_"
	APITokenPrefix         = "cxt_"
)

// NewSecret returns "<kind><8 hex>_<43 base64url>" (256 random bits), its display prefix and SHA-256.
func NewSecret(kind string) (secret, prefix string, hash []byte, err error) {
	p := make([]byte, 4)
	s := make([]byte, 32)
	if _, err = rand.Read(p); err != nil {
		return
	}
	if _, err = rand.Read(s); err != nil {
		return
	}
	prefix = kind + hex.EncodeToString(p)
	secret = prefix + "_" + base64.RawURLEncoding.EncodeToString(s)
	return secret, prefix, HashSecret(secret), nil
}

// HashSecret is the stored form of a token or credential.
func HashSecret(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// WellFormedSecret checks shape and length before any DB lookup.
func WellFormedSecret(kind, s string) bool {
	if !strings.HasPrefix(s, kind) || len(s) != len(kind)+8+1+43 {
		return false
	}
	for _, r := range s[len(kind):] {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// CreatedSecret is returned once by credential/token creation.
type CreatedSecret struct {
	ID, Prefix, Secret string
	ExpiresAt          *time.Time
}

// CreateWorkerCredential issues a worker credential; only its hash is stored.
func (s *Store) CreateWorkerCredential(ctx context.Context, name string, expiresInDays int) (CreatedSecret, error) {
	if l := len(name); l < 1 || l > 100 {
		return CreatedSecret{}, fmt.Errorf("name must be 1..100 characters")
	}
	if expiresInDays < 0 || expiresInDays > 730 {
		return CreatedSecret{}, fmt.Errorf("expires-in-days must be 0 (never) .. 730")
	}
	secret, prefix, hash, err := NewSecret(WorkerCredentialPrefix)
	if err != nil {
		return CreatedSecret{}, err
	}
	out := CreatedSecret{Prefix: prefix, Secret: secret}
	err = s.inTx(ctx, false, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO worker_credentials (name, secret_hash, prefix, expires_at)
			VALUES ($1, $2, $3, CASE WHEN $4::int > 0 THEN now() + make_interval(days => $4::int) END)
			RETURNING id, expires_at`, name, hash, prefix, expiresInDays).Scan(&out.ID, &out.ExpiresAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id, details)
			VALUES ('privileged', 'worker_credential.create', 'allowed', 'system:cli', 'worker_credential', $1,
			        jsonb_build_object('prefix', $2::text))`, out.ID, prefix)
		return err
	})
	return out, err
}

// RevokeWorkerCredential revokes a credential (lock order: credential row only). In-flight reports that
// already hold the share lock commit first; every later mutation is denied (ADR-0003 revocation bound).
func (s *Store) RevokeWorkerCredential(ctx context.Context, id string) error {
	return s.inTx(ctx, false, func(tx pgx.Tx) error {
		// Lock first, then stamp: an UPDATE that waited behind a share lock whose holder did not modify the
		// row keeps the clock value computed before the wait (no EPQ re-projection), so revoked_at could
		// precede the terminal_at of a report that committed first (NF-02 class).
		if _, err := tx.Exec(ctx, `SELECT 1 FROM worker_credentials WHERE id = $1 FOR UPDATE`, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE worker_credentials SET revoked_at = clock_timestamp() WHERE id = $1 AND revoked_at IS NULL`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return deny("worker_credential_not_found")
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events (category, action, outcome, actor, target_type, target_id)
			VALUES ('privileged', 'worker_credential.revoke', 'allowed', 'system:cli', 'worker_credential', $1)`, id)
		return err
	})
}

// AuthenticateWorker resolves a presented worker credential to its id. It is re-checked inside every
// mutation transaction (no caching); this lookup only rejects unknown or revoked secrets early.
func (s *Store) AuthenticateWorker(ctx context.Context, secret string) (string, error) {
	if !WellFormedSecret(WorkerCredentialPrefix, secret) {
		return "", &Denial{Reason: "credential_revoked", Detail: "malformed credential"}
	}
	var id string
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT id, revoked_at IS NULL AND (expires_at IS NULL OR expires_at > clock_timestamp())
		FROM worker_credentials WHERE secret_hash = $1`, HashSecret(secret)).Scan(&id, &ok)
	if isNoRows(err) {
		return "", &Denial{Reason: "credential_revoked", Detail: "unknown credential"}
	}
	if err != nil {
		return "", classifyDBError(err)
	}
	if !ok {
		return "", &Denial{Reason: "credential_revoked", Detail: "credential revoked or expired"}
	}
	return id, nil
}

// WorkerRef identifies the calling worker process and the credential it authenticated with.
type WorkerRef struct {
	WorkerID, IncarnationID, CredentialID string
}

// Capability is a handler name and schema version.
type Capability struct {
	Name    string
	Version int
}

// RegisterParams is a Register request after shape validation.
type RegisterParams struct {
	WorkerID, IncarnationID, Name, SDKVersion, Hostname string
	ProtocolVersion, Capacity                           int
	Capabilities                                        []Capability
}

// Registered is the Register reply.
type Registered struct {
	WorkerID, IncarnationID string
}

// Register binds a process incarnation to a worker row. A credential is bound to one non-deregistered
// worker; a new incarnation of it is accepted when worker_id names it, or when the previous incarnation
// is no longer live. The new incarnation fences every attempt of the old one immediately.
func (s *Store) Register(ctx context.Context, credID string, p RegisterParams) (Registered, error) {
	for _, c := range p.Capabilities {
		if s.Catalog != nil && !s.Catalog.Known(c.Name, c.Version) {
			return Registered{}, &Denial{Reason: "unknown_handler", Detail: fmt.Sprintf("%s@v%d", c.Name, c.Version)}
		}
	}
	var out Registered
	err := s.inTx(ctx, false, func(tx pgx.Tx) error {
		var credOK bool
		err := tx.QueryRow(ctx, `SELECT revoked_at IS NULL AND (expires_at IS NULL OR expires_at > clock_timestamp())
			FROM worker_credentials WHERE id = $1 FOR SHARE`, credID).Scan(&credOK)
		if isNoRows(err) || (err == nil && !credOK) {
			return deny("credential_revoked")
		} else if err != nil {
			return err
		}
		target := p.WorkerID
		if target == "" {
			rows, err := tx.Query(ctx, `SELECT id, current_incarnation_id, last_heartbeat_at > clock_timestamp() - $2::interval
				FROM workers WHERE credential_id = $1 AND status <> 'deregistered' ORDER BY id FOR UPDATE`, credID, s.Timing.LivenessTimeout)
			if err != nil {
				return err
			}
			type cand struct {
				id, inc string
				live    bool
			}
			cands, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (cand, error) {
				var c cand
				return c, r.Scan(&c.id, &c.inc, &c.live)
			})
			if err != nil {
				return err
			}
			for _, c := range cands {
				if c.inc == p.IncarnationID { // retry of a Register whose reply was lost
					target = c.id
					break
				}
				if c.live {
					return &Denial{Reason: "incarnation_mismatch", Detail: "credential is bound to a live worker; pass its worker_id to re-register"}
				}
				target = c.id // take over a stale worker of this credential
			}
		}
		if target != "" {
			// Drain is sticky to the worker id and a deregistered id is never resurrected (RV5-02): a new
			// incarnation of a draining worker stays draining (Claim answers worker_draining); after
			// deregistration the process rejoins as a new worker row.
			var status, prevInc string
			var live bool
			err := tx.QueryRow(ctx, `SELECT status, current_incarnation_id, last_heartbeat_at > clock_timestamp() - $3::interval
				FROM workers WHERE id = $1 AND credential_id = $2 FOR UPDATE`, target, credID, s.Timing.LivenessTimeout).Scan(&status, &prevInc, &live)
			if isNoRows(err) {
				return &Denial{Reason: "credential_mismatch", Detail: "credential is not bound to this worker"}
			} else if err != nil {
				return err
			}
			if status == "deregistered" {
				target = ""
			} else if live && prevInc != p.IncarnationID {
				// Two processes sharing one id supersede each other; each takeover fences the other's attempts.
				if err := insertAudit(ctx, tx, AuditRow{Category: "security", Action: "worker.incarnation_takeover", Outcome: "allowed",
					Actor: "worker:" + target, TargetType: "worker", TargetID: target,
					Details: map[string]any{"previous_incarnation_id": prevInc, "incarnation_id": p.IncarnationID, "credential_id": credID}}); err != nil {
					return err
				}
			}
		}
		if target == "" {
			if err := tx.QueryRow(ctx, `INSERT INTO workers (name, credential_id, current_incarnation_id, capacity, protocol_version,
					sdk_version, hostname, registered_at, last_heartbeat_at)
				VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), clock_timestamp(), clock_timestamp()) RETURNING id`,
				p.Name, credID, p.IncarnationID, p.Capacity, p.ProtocolVersion, p.SDKVersion, p.Hostname).Scan(&target); err != nil {
				return err
			}
		} else {
			tag, err := tx.Exec(ctx, `UPDATE workers SET name = $3, current_incarnation_id = $4, capacity = $5, protocol_version = $6,
					sdk_version = NULLIF($7, ''), hostname = NULLIF($8, ''), last_heartbeat_at = clock_timestamp(),
					registered_at = CASE WHEN current_incarnation_id = $4 THEN registered_at ELSE clock_timestamp() END,
					version = version + CASE WHEN current_incarnation_id = $4 THEN 0 ELSE 1 END
				WHERE id = $1 AND credential_id = $2 AND status <> 'deregistered'`,
				target, credID, p.Name, p.IncarnationID, p.Capacity, p.ProtocolVersion, p.SDKVersion, p.Hostname)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return &Denial{Reason: "credential_mismatch", Detail: "credential is not bound to this worker"}
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM worker_capabilities WHERE worker_id = $1`, target); err != nil {
			return err
		}
		for _, c := range p.Capabilities {
			if _, err := tx.Exec(ctx, `INSERT INTO worker_capabilities (worker_id, handler_name, handler_version) VALUES ($1, $2, $3)
				ON CONFLICT DO NOTHING`, target, c.Name, c.Version); err != nil {
				return err
			}
		}
		out = Registered{WorkerID: target, IncarnationID: p.IncarnationID}
		return nil
	})
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return Registered{}, &Denial{Reason: "invalid_argument", Detail: "incarnation_id already used"}
		}
		return Registered{}, err
	}
	// Usage bump in its own transaction (NF-01: never write a share-locked identity row).
	_, _ = s.Pool.Exec(ctx, `UPDATE worker_credentials SET last_used_at = clock_timestamp() WHERE id = $1`, credID)
	return out, nil
}

// identityDenial explains why a worker-scoped statement matched no row.
func identityDenial(ctx context.Context, q pgx.Tx, w WorkerRef) error {
	var credMatch, credOK, incOK bool
	var status string
	err := q.QueryRow(ctx, `SELECT w.credential_id = $3, c.revoked_at IS NULL AND (c.expires_at IS NULL OR c.expires_at > clock_timestamp()),
			w.current_incarnation_id = $2, w.status
		FROM workers w JOIN worker_credentials c ON c.id = w.credential_id WHERE w.id = $1`,
		w.WorkerID, w.IncarnationID, w.CredentialID).Scan(&credMatch, &credOK, &incOK, &status)
	switch {
	case isNoRows(err):
		return &Denial{Reason: "credential_mismatch", Detail: "credential is not bound to this worker"}
	case err != nil:
		return err
	case !credMatch:
		return &Denial{Reason: "credential_mismatch", Detail: "credential is not bound to this worker"}
	case !credOK:
		return deny("credential_revoked")
	case !incOK:
		return deny("incarnation_mismatch")
	case status == "deregistered":
		return deny("worker_deregistered")
	}
	return nil
}

// WorkerState is the Heartbeat/Drain reply.
type WorkerState struct {
	Status        string
	DrainDeadline *time.Time
}

// Heartbeat records liveness. It never extends any lease (ADR-0003).
func (s *Store) Heartbeat(ctx context.Context, w WorkerRef) (WorkerState, error) {
	var out WorkerState
	err := s.inTx(ctx, false, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `UPDATE workers w SET last_heartbeat_at = clock_timestamp()
			FROM worker_credentials c
			WHERE w.id = $1 AND w.current_incarnation_id = $2 AND w.credential_id = $3 AND w.status <> 'deregistered'
			  AND c.id = w.credential_id AND c.revoked_at IS NULL AND (c.expires_at IS NULL OR c.expires_at > clock_timestamp())
			RETURNING w.status, w.drain_deadline_at`, w.WorkerID, w.IncarnationID, w.CredentialID).Scan(&out.Status, &out.DrainDeadline)
		if isNoRows(err) {
			if d := identityDenial(ctx, tx, w); d != nil {
				return d
			}
			return deny("worker_deregistered")
		}
		return err
	})
	return out, err
}

// Drain stops new claims for the worker; renewals are allowed until the drain deadline (E12).
func (s *Store) Drain(ctx context.Context, w WorkerRef, grace time.Duration) (time.Time, error) {
	if grace < 0 {
		grace = 0
	}
	if grace > s.Timing.DrainGraceMax {
		grace = s.Timing.DrainGraceMax
	}
	var deadline time.Time
	err := s.inTx(ctx, false, func(tx pgx.Tx) error {
		if d := identityDenial(ctx, tx, w); d != nil {
			return d
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM workers WHERE id = $1 FOR UPDATE`, w.WorkerID); err != nil {
			return err // lock before stamping (see RevokeWorkerCredential)
		}
		err := tx.QueryRow(ctx, `UPDATE workers SET status = 'draining', drain_requested_at = clock_timestamp(),
				drain_deadline_at = clock_timestamp() + $3::interval, version = version + 1
			WHERE id = $1 AND current_incarnation_id = $2 AND status = 'active' RETURNING drain_deadline_at`,
			w.WorkerID, w.IncarnationID, grace).Scan(&deadline)
		if isNoRows(err) {
			err = tx.QueryRow(ctx, `SELECT drain_deadline_at FROM workers WHERE id = $1 AND status = 'draining'`, w.WorkerID).Scan(&deadline)
			if isNoRows(err) {
				return deny("worker_deregistered")
			}
		}
		return err
	})
	return deadline, err
}

// Deregister ends the worker; its remaining active attempts are recovered at lease expiry.
func (s *Store) Deregister(ctx context.Context, w WorkerRef) (int, error) {
	var left int
	err := s.inTx(ctx, false, func(tx pgx.Tx) error {
		if d := identityDenial(ctx, tx, w); d != nil && !IsDenial(d, "worker_deregistered") {
			return d
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM workers WHERE id = $1 FOR UPDATE`, w.WorkerID); err != nil {
			return err // lock before stamping (see RevokeWorkerCredential)
		}
		if _, err := tx.Exec(ctx, `UPDATE workers SET status = 'deregistered', deregistered_at = clock_timestamp(), version = version + 1
			WHERE id = $1 AND current_incarnation_id = $2 AND status <> 'deregistered'`, w.WorkerID, w.IncarnationID); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT count(*) FROM job_attempts WHERE worker_id = $1 AND state = 'active'`, w.WorkerID).Scan(&left)
	})
	return left, err
}

// NewUUID returns a random (version 4) UUID string.
func NewUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
