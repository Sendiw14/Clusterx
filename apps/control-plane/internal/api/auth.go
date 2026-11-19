package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/auth"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

// access is the authorization rule of one route (docs/security/permissions.md).
type access struct {
	scope       string // required scope; "*:read" = any read scope; "" = none
	sessionOnly bool   // BFF session principals only; API tokens get 403 credential_not_allowed
	operator    bool   // operator role required (beyond the scope)
	stream      bool   // WebSocket upgrade: not activity, jti not consumed
}

func mutating(r *http.Request) bool {
	return r.Method != http.MethodGet && r.Method != http.MethodHead
}

// authenticate resolves the bearer credential: a direct API token (cxt_…) through api_token_authority, or
// a BFF service assertion verified in Go and re-checked against users/sessions in PostgreSQL on every
// request (ADR-0005/0006). Forwarded identity headers are never read. It returns the 401 reason on failure.
func (v *V1) authenticate(r *http.Request, a access) (store.Principal, string, error) {
	bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || bearer == "" {
		return store.Principal{}, "token_invalid", nil
	}
	if !auth.LooksLikeAssertion(bearer) {
		p, err := v.Store.AuthenticateToken(r.Context(), bearer)
		if d, ok := errors.AsType[*store.Denial](err); ok {
			return store.Principal{}, d.Reason, nil
		}
		return p, "", err
	}
	c, err := v.Verifier.Verify(bearer, mutating(r) && !a.stream)
	if e, ok := errors.AsType[*auth.Error](err); ok {
		v.Log.Warn("service assertion rejected", "request_id", RequestID(r.Context()), "reason", e.Reason, "detail", e.Detail)
		return store.Principal{}, e.Reason, nil
	} else if err != nil {
		return store.Principal{}, "", err
	}
	if c.Login() {
		// A login assertion is valid only for POST /api/v1/sessions.
		return store.Principal{}, "assertion_invalid", nil
	}
	sess, err := v.Store.AuthenticateSession(r.Context(), c.Sid, c.UserID(), !a.stream)
	if d, ok := errors.AsType[*store.Denial](err); ok {
		if d.Reason == "assertion_invalid" {
			v.securityEvent(r, "assertion.sid_mismatch", "user:"+c.UserID(), "session", c.Sid, map[string]any{"kid": c.Kid})
		}
		reason := d.Reason
		if d.Reason == "session_expired" {
			reason = "session_expired:" + d.Detail
		}
		return store.Principal{}, reason, nil
	} else if err != nil {
		return store.Principal{}, "", err
	}
	if c.Role != sess.Role {
		// The role claim is a hint only; the DB role wins (ADR-0006 §1). Logged as a security event.
		v.securityEvent(r, "assertion.role_mismatch", "user:"+sess.UserID, "user", sess.UserID,
			map[string]any{"claimed": c.Role, "effective": sess.Role, "kid": c.Kid})
	}
	return store.Principal{Ref: "user:" + sess.UserID, Role: sess.Role, Scopes: store.RoleScopes(sess.Role), Kind: "session",
		Session: &sess, Kid: c.Kid}, "", nil
}

// securityEvent appends a security audit row outside the request transaction; failures are logged only.
func (v *V1) securityEvent(r *http.Request, action, actor, targetType, targetID string, details map[string]any) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
	defer cancel()
	if err := v.Store.Audit(ctx, store.AuditRow{Category: "security", Action: action, Outcome: "denied", Actor: actor,
		TargetType: targetType, TargetID: targetID, RequestID: RequestID(r.Context()), Details: details}); err != nil {
		v.Log.Warn("security audit write failed", "request_id", RequestID(r.Context()), "action", action, "err", err)
	}
}

// authFailed answers a failed authentication. Only credential-guessing reasons (an unknown token or an
// assertion that does not verify) drain the per-peer bucket; session/token lifecycle reasons
// (expired, revoked, user disabled) do not, so stale tabs behind the BFF cannot lock out other users
// (RV5-05). An empty bucket turns further guesses into 429; a credential that verifies is never
// refused by this bucket.
func (v *V1) authFailed(w http.ResponseWriter, r *http.Request, reason string) {
	if reason == "token_invalid" || reason == auth.ReasonInvalid {
		if ok, wait := v.AuthFailures.Allow(clientIP(r)); !ok {
			rateLimited(w, r, wait)
			return
		}
	}
	v.unauthenticatedReason(w, r, reason)
}

func (v *V1) unauthenticatedReason(w http.ResponseWriter, r *http.Request, reason string) {
	details := map[string]any{"reason": reason}
	if base, expiry, ok := strings.Cut(reason, ":"); ok {
		details = map[string]any{"reason": base, "expiry": expiry}
	}
	w.Header().Set("WWW-Authenticate", `Bearer realm="clusterx"`)
	writeErrorDetails(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required.", details)
}

// guard authenticates, authorizes and rate-limits one route, then calls next with the principal.
func (v *V1) guard(a access, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, reason, err := v.authenticate(r, a)
		if err != nil {
			v.fail(w, r, err)
			return
		}
		if reason != "" {
			v.authFailed(w, r, reason)
			return
		}
		if msg, details := authorize(p, a); msg != "" {
			if mutating(r) {
				d := map[string]any{"method": r.Method, "route": r.Pattern}
				for k, x := range details {
					d[k] = x
				}
				v.securityEvent(r, "authz.denied", p.Ref, "", "", d)
			}
			writeErrorDetails(w, r, http.StatusForbidden, "forbidden", msg, details)
			return
		}
		if mutating(r) {
			if ok, wait := v.Mutations.Allow(p.Ref); !ok {
				rateLimited(w, r, wait)
				return
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	}
}

// authorize applies the permissions matrix; it returns "" when allowed.
func authorize(p store.Principal, a access) (string, map[string]any) {
	if a.sessionOnly && p.Kind != "session" {
		return "This operation is not available to API tokens.", map[string]any{"reason": "credential_not_allowed"}
	}
	if a.operator && p.Role != "operator" {
		return "Your role does not allow this action.", map[string]any{"required": "operator", "role": p.Role}
	}
	switch {
	case a.scope == "":
		return "", nil
	case a.scope == "*:read":
		if slices.ContainsFunc(p.Scopes, func(s string) bool { return strings.HasSuffix(s, ":read") }) {
			return "", nil
		}
	case p.Has(a.scope):
		return "", nil
	}
	msg := "Your token does not allow this action."
	if p.Kind == "session" {
		msg = "Your role does not allow this action."
	}
	return msg, map[string]any{"required": a.scope, "role": p.Role}
}
