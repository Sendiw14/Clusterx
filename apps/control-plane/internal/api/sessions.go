package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/auth"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

var githubLoginRe = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)

// createSession implements POST /api/v1/sessions (ADR-0006 §5). Only a BFF login assertion
// (purpose=login, sub=github:<id>, single-use jti) is accepted; API tokens and normal assertions get
// 403 credential_not_allowed.
func (v *V1) createSession(w http.ResponseWriter, r *http.Request) {
	bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	switch {
	case !ok || bearer == "":
		v.authFailed(w, r, auth.ReasonInvalid)
		return
	case !auth.LooksLikeAssertion(bearer):
		writeErrorDetails(w, r, http.StatusForbidden, "forbidden", "Sessions cannot be managed with an API token.",
			map[string]any{"reason": "credential_not_allowed"})
		return
	}
	c, err := v.Verifier.Verify(bearer, true)
	if e, ok := errors.AsType[*auth.Error](err); ok {
		v.Log.Warn("login assertion rejected", "request_id", RequestID(r.Context()), "reason", e.Reason, "detail", e.Detail)
		v.authFailed(w, r, e.Reason)
		return
	} else if err != nil {
		v.fail(w, r, err)
		return
	}
	if !c.Login() {
		writeErrorDetails(w, r, http.StatusForbidden, "forbidden", "Sessions are created with a login assertion only.",
			map[string]any{"reason": "credential_not_allowed"})
		return
	}
	if ok, wait := v.Logins.Allow("github:" + strconv.FormatInt(c.GitHubID(), 10)); !ok {
		rateLimited(w, r, wait)
		return
	}
	fields, ok := readObject(w, r, true, "github_login")
	if !ok {
		return
	}
	var e verrs
	if fields["github_login"] == nil {
		e.add("/github_login", "required", "is required", nil)
	}
	login := stringField(fields["github_login"], "/github_login", githubLoginRe, "", &e)
	if e.write(w, r) {
		return
	}
	sc, err := v.Store.CreateSession(r.Context(), c.GitHubID(), login, v.Sessions, store.AuditMeta{RequestID: RequestID(r.Context()), Kid: c.Kid})
	if d, ok := errors.AsType[*store.Denial](err); ok && (d.Reason == "not_allowlisted" || d.Reason == "user_disabled") {
		msg := "This GitHub account is not allowed to sign in."
		if d.Reason == "user_disabled" {
			msg = "This account is disabled."
		}
		writeErrorDetails(w, r, http.StatusForbidden, "forbidden", msg, map[string]any{"reason": d.Reason})
		return
	}
	if err != nil {
		v.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"session_id": sc.ID, "secret": sc.Secret,
		"user": map[string]string{"id": sc.UserID, "github_login": sc.Login}, "role": sc.Role,
		"expires_at": tv(sc.ExpiresAt), "idle_timeout_seconds": sc.IdleTimeoutSeconds, "idle_expires_at": tv(sc.IdleUntil),
	})
}

// deleteSession implements DELETE /api/v1/sessions/current (logout). Open streams of this session close
// 4401 at their next re-validation (<= 30 s) — immediately when the hub is told here.
func (v *V1) deleteSession(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if err := v.Store.RevokeSession(r.Context(), p.Session.ID, p.Ref, meta(r)); err != nil {
		if store.IsDenial(err, "session_revoked") {
			v.unauthenticatedReason(w, r, "session_revoked")
			return
		}
		v.fail(w, r, err)
		return
	}
	v.Hub.Revalidate()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(HeaderRequestID, RequestID(r.Context()))
	w.WriteHeader(http.StatusNoContent)
}

func (v *V1) getMe(w http.ResponseWriter, r *http.Request) {
	s := principal(r).Session
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]string{"id": s.UserID, "github_login": s.Login}, "role": s.Role,
		"session": map[string]any{"id": s.ID, "created_at": tv(s.CreatedAt), "expires_at": tv(s.ExpiresAt),
			"idle_timeout_seconds": s.IdleTimeoutSeconds, "last_activity_at": tv(s.LastActivityAt), "idle_expires_at": tv(s.IdleUntil)},
	})
}

// ---- tokens and worker credentials (operator session only) -------------------------------------

func tokenView(t store.APIToken) map[string]any {
	return map[string]any{"id": t.ID, "name": t.Name, "role": t.Role, "scopes": t.Scopes, "prefix": t.Prefix,
		"owner_user_id": t.OwnerUserID, "created_at": tv(t.CreatedAt), "expires_at": ts(t.ExpiresAt),
		"revoked_at": ts(t.RevokedAt), "last_used_at": ts(t.LastUsedAt)}
}

func credView(c store.WorkerCredential) map[string]any {
	ids := c.BoundWorkerIDs
	if ids == nil {
		ids = []string{}
	}
	return map[string]any{"id": c.ID, "name": c.Name, "prefix": c.Prefix, "created_by": c.CreatedBy, "created_at": tv(c.CreatedAt),
		"expires_at": ts(c.ExpiresAt), "revoked_at": ts(c.RevokedAt), "last_used_at": ts(c.LastUsedAt), "bound_worker_ids": ids}
}

func idemKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := r.Header.Get("Idempotency-Key")
	if _, present := r.Header["Idempotency-Key"]; present && !idemKeyRe.MatchString(key) {
		var e verrs
		e.add("header:Idempotency-Key", "pattern", "must match ^[A-Za-z0-9_.:-]{1,128}$", nil)
		e.write(w, r)
		return "", false
	}
	return key, true
}

// writeCreated answers a credential create: 201 with the one-time secret, or a 200 replay without it.
func writeCreated(w http.ResponseWriter, view map[string]any, secret string, replayed bool) {
	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
		writeJSON(w, http.StatusOK, view)
		return
	}
	view["secret"] = secret
	writeJSON(w, http.StatusCreated, view)
}

func (v *V1) createToken(w http.ResponseWriter, r *http.Request) {
	key, ok := idemKey(w, r)
	if !ok {
		return
	}
	fields, ok := readObject(w, r, true, "name", "role", "scopes", "expires_in_days")
	if !ok {
		return
	}
	var e verrs
	var tc store.TokenCreate
	for _, f := range []string{"name", "role", "scopes"} {
		if fields[f] == nil {
			e.add("/"+f, "required", "is required", nil)
		}
	}
	if fields["name"] != nil && (json.Unmarshal(fields["name"], &tc.Name) != nil || len(tc.Name) < 1 || len(tc.Name) > 100) {
		e.add("/name", "length", "must be a string of 1..100 characters", map[string]any{"min": 1, "max": 100})
	}
	if fields["role"] != nil && (json.Unmarshal(fields["role"], &tc.Role) != nil || (tc.Role != "operator" && tc.Role != "viewer")) {
		e.add("/role", "enum", "must be operator or viewer", nil)
	}
	if fields["scopes"] != nil {
		if json.Unmarshal(fields["scopes"], &tc.Scopes) != nil || len(tc.Scopes) < 1 || len(tc.Scopes) > 6 {
			e.add("/scopes", "length", "must be an array of 1..6 scopes", map[string]any{"min": 1, "max": 6})
		}
		seen := map[string]bool{}
		for i, sc := range tc.Scopes {
			valid := false
			for _, a := range store.AllScopes {
				valid = valid || a == sc
			}
			switch {
			case !valid || seen[sc]:
				e.add("/scopes/"+strconv.Itoa(i), "enum", "unknown or duplicate scope", nil)
			case tc.Role == "viewer" && !strings.HasSuffix(sc, ":read"):
				e.add("/scopes/"+strconv.Itoa(i), "scope_not_allowed", "viewer tokens may only hold *:read scopes", nil)
			}
			seen[sc] = true
		}
	}
	tc.ExpiresInDays = 90
	if raw := fields["expires_in_days"]; raw != nil && string(raw) != "null" {
		tc.ExpiresInDays = intField(raw, "/expires_in_days", 1, 365, 90, &e)
	}
	if e.write(w, r) {
		return
	}
	p := principal(r)
	canon, _ := json.Marshal(map[string]any{"name": tc.Name, "role": tc.Role, "scopes": tc.Scopes, "expires_in_days": tc.ExpiresInDays})
	t, secret, replayed, err := v.Store.CreateTokenFor(r.Context(), p.Session.UserID, tc, key, store.Fingerprint(canon), meta(r))
	if d, ok := errors.AsType[*store.Denial](err); ok {
		switch d.Reason {
		case "role_not_allowed":
			e.add("/role", "role_not_allowed", "a token cannot outrank its owner", nil)
			e.write(w, r)
			return
		case "idempotency_conflict":
			writeErrorDetails(w, r, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was used with a different request.",
				map[string]any{"token_id": d.Detail})
			return
		}
	}
	if err != nil {
		v.fail(w, r, err)
		return
	}
	writeCreated(w, tokenView(t), secret, replayed)
}

func pathUUID(w http.ResponseWriter, r *http.Request, code, msg string) (string, bool) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, r, http.StatusNotFound, code, msg)
		return "", false
	}
	return strings.ToLower(id), true
}

func (v *V1) revokeToken(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "token_not_found", "Token not found.")
	if !ok {
		return
	}
	t, err := v.Store.RevokeTokenBy(r.Context(), id, principal(r).Ref, meta(r))
	if err != nil {
		v.fail(w, r, err)
		return
	}
	v.Hub.Revalidate() // token streams close 4401 without waiting for the 30 s re-check
	writeJSON(w, http.StatusOK, tokenView(t))
}

// pageParams parses include_revoked, limit and a (created_at, id) cursor.
func pageParams(w http.ResponseWriter, r *http.Request) (store.Page, bool) {
	q := r.URL.Query()
	var e verrs
	p := store.Page{Limit: queryInt(q, "limit", 1, 200, 50, &e)}
	switch s := q.Get("include_revoked"); s {
	case "", "false":
	case "true":
		p.IncludeRevoked = true
	default:
		e.add("?include_revoked", "type", "must be true or false", nil)
	}
	if e.write(w, r) {
		return p, false
	}
	cur, ok := decodeCursor(q, "-created_at")
	if !ok {
		invalidCursor(w, r)
		return p, false
	}
	p.After = cur
	return p, true
}

func pageCursor(r *http.Request, more bool, created time.Time, id string) *string {
	if !more {
		return nil
	}
	return encodeCursor(cursorJSON{S: "-created_at", F: filterHash(r.URL.Query()), I: id, T: tv(created)})
}

func (v *V1) listTokens(w http.ResponseWriter, r *http.Request) {
	pg, ok := pageParams(w, r)
	if !ok {
		return
	}
	ts, more, asOf, err := v.Store.ListAPITokens(r.Context(), pg)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(ts))
	var next *string
	for _, t := range ts {
		items = append(items, tokenView(t))
	}
	if len(ts) > 0 {
		next = pageCursor(r, more, ts[len(ts)-1].CreatedAt, ts[len(ts)-1].ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next, "as_of": tv(asOf)})
}

func (v *V1) createWorkerCredential(w http.ResponseWriter, r *http.Request) {
	key, ok := idemKey(w, r)
	if !ok {
		return
	}
	fields, ok := readObject(w, r, true, "name", "expires_in_days")
	if !ok {
		return
	}
	var e verrs
	var name string
	if fields["name"] == nil {
		e.add("/name", "required", "is required", nil)
	} else if json.Unmarshal(fields["name"], &name) != nil || len(name) < 1 || len(name) > 100 {
		e.add("/name", "length", "must be a string of 1..100 characters", map[string]any{"min": 1, "max": 100})
	}
	days := 180
	if raw, present := fields["expires_in_days"]; present {
		if string(raw) == "null" {
			days = 0 // explicit null: no expiry
		} else {
			days = intField(raw, "/expires_in_days", 1, 730, 180, &e)
		}
	}
	if e.write(w, r) {
		return
	}
	p := principal(r)
	canon, _ := json.Marshal(map[string]any{"name": name, "expires_in_days": days})
	c, secret, replayed, err := v.Store.CreateWorkerCredentialBy(r.Context(), name, days, p.Session.UserID, key, store.Fingerprint(canon), meta(r))
	if d, ok := errors.AsType[*store.Denial](err); ok && d.Reason == "idempotency_conflict" {
		writeErrorDetails(w, r, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was used with a different request.",
			map[string]any{"worker_credential_id": d.Detail})
		return
	}
	if err != nil {
		v.fail(w, r, err)
		return
	}
	writeCreated(w, credView(c), secret, replayed)
}

func (v *V1) revokeWorkerCredential(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "worker_credential_not_found", "Worker credential not found.")
	if !ok {
		return
	}
	c, err := v.Store.RevokeWorkerCredentialBy(r.Context(), id, principal(r).Session.UserID, meta(r))
	if err != nil {
		v.fail(w, r, err)
		return
	}
	for _, wid := range c.BoundWorkerIDs {
		if x, _, err := v.Store.GetWorker(r.Context(), wid); err == nil {
			v.Hub.PublishWorker(wid, x.Version)
		}
	}
	writeJSON(w, http.StatusOK, credView(c))
}

func (v *V1) listWorkerCredentials(w http.ResponseWriter, r *http.Request) {
	pg, ok := pageParams(w, r)
	if !ok {
		return
	}
	cs, more, asOf, err := v.Store.ListWorkerCredentials(r.Context(), pg)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(cs))
	var next *string
	for _, c := range cs {
		items = append(items, credView(c))
	}
	if len(cs) > 0 {
		next = pageCursor(r, more, cs[len(cs)-1].CreatedAt, cs[len(cs)-1].ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next, "as_of": tv(asOf)})
}
