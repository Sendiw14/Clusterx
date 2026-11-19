package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/auth"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/catalog"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

// Contract version this server implements (OpenAPI info.version).
const APIVersion = "1.0.0-draft.7"

// Bounds (ADR-0004, OpenAPI Config.limits).
const (
	MaxBodyBytes    = 80 << 10
	MaxPayloadBytes = 64 << 10
	MaxJSONDepth    = 32
)

// V1 wires every /api/v1 operation of OpenAPI 1.0.0-draft.7. Authentication: direct API tokens and BFF
// service assertions (ADR-0005/0006); authorization per docs/security/permissions.md, enforced in Go.
type V1 struct {
	Store       *store.Store
	Catalog     *catalog.Catalog
	Environment string
	Version     string
	Commit      string
	Timing      ConfigTiming
	Log         *slog.Logger
	// Verifier checks BFF assertions; nil or keyless rejects every assertion (tokens still work).
	Verifier *auth.Verifier
	Sessions store.SessionTiming
	// Rate limits (in-memory, single coordinator). Nil disables the limit.
	Mutations    *Limiter // per principal, every mutating request
	AuthFailures *Limiter // per TCP peer, consumed by failed authentication
	Logins       *Limiter // per GitHub id, POST /sessions
	// Hub fans stream hints out to WebSocket clients; nil disables /api/v1/stream (503).
	Hub *Hub
	// StreamOrigins is the WebSocket Origin allowlist; a request without Origin (server-to-server BFF) is allowed.
	StreamOrigins []string
}

// ConfigTiming is reported by GET /config.
type ConfigTiming struct {
	HeartbeatInterval, LivenessTimeout, LeaseTTL, CancelGrace, DrainGrace time.Duration
}

type principalKey struct{}

// route registers per-method handlers for path and a method-less fallback answering 405 (M3-2).
func route(mux *http.ServeMux, path string, methods map[string]http.HandlerFunc) {
	allow := make([]string, 0, len(methods))
	for m, h := range methods {
		mux.HandleFunc(m+" "+path, h)
		allow = append(allow, m)
		if m == http.MethodGet {
			allow = append(allow, http.MethodHead)
		}
	}
	sort.Strings(allow)
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", strings.Join(allow, ", "))
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	})
}

func (v *V1) register(mux *http.ServeMux) {
	read := func(scope string) access { return access{scope: scope} }
	write := func(scope string) access { return access{scope: scope} }
	sess := access{sessionOnly: true}
	opSess := access{sessionOnly: true, operator: true}
	route(mux, "/api/v1/jobs", map[string]http.HandlerFunc{
		http.MethodPost: v.guard(write("jobs:write"), v.createJob),
		http.MethodGet:  v.guard(read("jobs:read"), v.listJobs),
	})
	route(mux, "/api/v1/jobs/{id}", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("jobs:read"), v.getJob)})
	route(mux, "/api/v1/jobs/{id}/attempts", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("jobs:read"), v.listAttempts)})
	route(mux, "/api/v1/jobs/{id}/events", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("jobs:read"), v.listEvents)})
	route(mux, "/api/v1/jobs/{id}/cancel", map[string]http.HandlerFunc{http.MethodPost: v.guard(write("jobs:write"), v.cancelJob)})
	route(mux, "/api/v1/jobs/{id}/retry", map[string]http.HandlerFunc{http.MethodPost: v.guard(write("jobs:write"), v.retryJob)})
	route(mux, "/api/v1/workers", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("workers:read"), v.listWorkers)})
	route(mux, "/api/v1/workers/{id}", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("workers:read"), v.getWorker)})
	route(mux, "/api/v1/workers/{id}/drain", map[string]http.HandlerFunc{http.MethodPost: v.guard(write("workers:write"), v.drainWorker)})
	route(mux, "/api/v1/handlers", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("jobs:read"), v.listHandlers)})
	route(mux, "/api/v1/config", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("*:read"), v.getConfig)})
	route(mux, "/api/v1/metrics/overview", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("metrics:read"), v.metricsOverview)})
	route(mux, "/api/v1/metrics/timeseries", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("metrics:read"), v.metricsTimeseries)})
	route(mux, "/api/v1/audit", map[string]http.HandlerFunc{http.MethodGet: v.guard(read("audit:read"), v.listAudit)})
	route(mux, "/api/v1/sessions", map[string]http.HandlerFunc{http.MethodPost: v.createSession})
	route(mux, "/api/v1/sessions/current", map[string]http.HandlerFunc{http.MethodDelete: v.guard(sess, v.deleteSession)})
	route(mux, "/api/v1/me", map[string]http.HandlerFunc{http.MethodGet: v.guard(sess, v.getMe)})
	route(mux, "/api/v1/tokens", map[string]http.HandlerFunc{
		http.MethodGet:  v.guard(opSess, v.listTokens),
		http.MethodPost: v.guard(opSess, v.createToken),
	})
	route(mux, "/api/v1/tokens/{id}/revoke", map[string]http.HandlerFunc{http.MethodPost: v.guard(opSess, v.revokeToken)})
	route(mux, "/api/v1/worker-credentials", map[string]http.HandlerFunc{
		http.MethodGet:  v.guard(opSess, v.listWorkerCredentials),
		http.MethodPost: v.guard(opSess, v.createWorkerCredential),
	})
	route(mux, "/api/v1/worker-credentials/{id}/revoke", map[string]http.HandlerFunc{http.MethodPost: v.guard(opSess, v.revokeWorkerCredential)})
	route(mux, "/api/v1/stream", map[string]http.HandlerFunc{http.MethodGet: v.guard(access{scope: "jobs:read", stream: true}, v.stream)})
}

func writeErrorDetails(w http.ResponseWriter, r *http.Request, status int, code, msg string, details map[string]any) {
	writeJSON(w, status, Error{Code: code, Message: msg, Details: details, RequestID: RequestID(r.Context())})
}

func principal(r *http.Request) store.Principal {
	p, _ := r.Context().Value(principalKey{}).(store.Principal)
	return p
}

// fail maps store errors to the contract: denials to their codes, DB loss to 503, else 500.
func (v *V1) fail(w http.ResponseWriter, r *http.Request, err error) {
	if d, ok := errors.AsType[*store.Denial](err); ok {
		switch d.Reason {
		case "job_not_found":
			writeError(w, r, http.StatusNotFound, "job_not_found", "Job not found.")
			return
		case "worker_not_found":
			writeError(w, r, http.StatusNotFound, "worker_not_found", "Worker not found.")
			return
		case "expired":
			term, exp, _ := strings.Cut(d.Detail, "|")
			writeErrorDetails(w, r, http.StatusGone, "expired", "This job's data has expired.",
				map[string]any{"terminal_at": term, "expired_at": exp, "retention_days": 30})
			return
		case "token_not_found":
			writeError(w, r, http.StatusNotFound, "token_not_found", "Token not found.")
			return
		case "worker_credential_not_found":
			writeError(w, r, http.StatusNotFound, "worker_credential_not_found", "Worker credential not found.")
			return
		case "user_disabled", "session_revoked":
			v.unauthenticatedReason(w, r, d.Reason)
			return
		case "validation_failed":
			writeErrorDetails(w, r, http.StatusUnprocessableEntity, "validation_failed", "Request has invalid fields.",
				map[string]any{"fields": []fieldErr{{Field: "/", Rule: "range", Message: d.Detail}}})
			return
		case "payload_too_large":
			writeErrorDetails(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Payload exceeds 65536 bytes.",
				map[string]any{"limit_bytes": MaxPayloadBytes, "field": "payload"})
			return
		}
	}
	if c, ok := errors.AsType[*store.Conflict](err); ok {
		d := map[string]any{"status": c.Status, "version": c.Version}
		for k, x := range c.Extra {
			d[k] = x
		}
		writeErrorDetails(w, r, http.StatusConflict, c.Reason, conflictMessage[c.Reason], d)
		return
	}
	if store.IsUnavailable(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "2")
		writeError(w, r, http.StatusServiceUnavailable, "unavailable",
			"Service temporarily unavailable. The request was not acknowledged; retry with the same Idempotency-Key.")
		return
	}
	v.Log.Error("api internal error", "request_id", RequestID(r.Context()), "err", err)
	writeError(w, r, http.StatusInternalServerError, "internal", "Something went wrong. Quote the request id when reporting it.")
}

// ---- validation -------------------------------------------------------------------------------

type fieldErr struct {
	Field   string         `json:"field"`
	Rule    string         `json:"rule"`
	Message string         `json:"message"`
	Limit   map[string]any `json:"limit,omitempty"`
}

type verrs []fieldErr

func (e *verrs) add(field, rule, msg string, limit map[string]any) {
	if len(*e) < 50 {
		*e = append(*e, fieldErr{field, rule, msg, limit})
	}
}

func (e verrs) write(w http.ResponseWriter, r *http.Request) bool {
	if len(e) == 0 {
		return false
	}
	writeErrorDetails(w, r, http.StatusUnprocessableEntity, "validation_failed", "Request has invalid fields.",
		map[string]any{"fields": e})
	return true
}

var (
	namespaceRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)
	handlerRe   = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,62}$`)
	idemKeyRe   = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
	uuidRe      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func decodeAny(raw []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	if d.More() {
		return nil, errors.New("trailing data")
	}
	return v, nil
}

// depthAndNUL returns the nesting depth and whether any string (or key) contains NUL.
func depthAndNUL(v any) (int, bool) {
	switch x := v.(type) {
	case []any:
		d, nul := 0, false
		for _, e := range x {
			dd, n := depthAndNUL(e)
			d, nul = max(d, dd), nul || n
		}
		return d + 1, nul
	case map[string]any:
		d, nul := 0, false
		for k, e := range x {
			dd, n := depthAndNUL(e)
			d, nul = max(d, dd), nul || n || strings.ContainsRune(k, 0)
		}
		return d + 1, nul
	case string:
		return 0, strings.ContainsRune(x, 0)
	}
	return 0, false
}

func intField(raw json.RawMessage, field string, lo, hi int, def int, e *verrs) int {
	if raw == nil {
		return def
	}
	var n json.Number
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&n); err != nil {
		e.add(field, "type", "must be an integer", nil)
		return def
	}
	i, err := strconv.Atoi(string(n))
	if err != nil {
		e.add(field, "type", "must be an integer", nil)
		return def
	}
	if i < lo || i > hi {
		e.add(field, "range", fmt.Sprintf("must be between %d and %d", lo, hi), map[string]any{"min": lo, "max": hi})
	}
	return i
}

func stringField(raw json.RawMessage, field string, re *regexp.Regexp, def string, e *verrs) string {
	if raw == nil {
		return def
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		e.add(field, "type", "must be a string", nil)
		return def
	}
	if !re.MatchString(s) {
		e.add(field, "pattern", "does not match "+re.String(), nil)
	}
	return s
}

// ---- POST /api/v1/jobs ------------------------------------------------------------------------

var jobCreateFields = []string{"namespace", "handler_name", "handler_version", "payload", "priority", "run_after", "max_attempts", "timeout_seconds"}

func (v *V1) createJob(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Content-Type must be application/json.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeErrorDetails(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body exceeds 81920 bytes.",
				map[string]any{"limit_bytes": MaxBodyBytes, "field": "body"})
			return
		}
		writeError(w, r, http.StatusBadRequest, "bad_request", "Could not read the request body.")
		return
	}
	var fields map[string]json.RawMessage
	if !utf8.Valid(body) || json.Unmarshal(body, &fields) != nil || fields == nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Request body is not a valid JSON object.")
		return
	}
	var e verrs
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if !slices.Contains(jobCreateFields, k) {
			e.add("/"+k, "unknown_field", "is not allowed", nil)
		}
	}
	ns := stringField(fields["namespace"], "/namespace", namespaceRe, "default", &e)
	if fields["handler_name"] == nil {
		e.add("/handler_name", "required", "is required", nil)
	}
	hn := stringField(fields["handler_name"], "/handler_name", handlerRe, "", &e)
	if fields["handler_version"] == nil {
		e.add("/handler_version", "required", "is required", nil)
	}
	hv := intField(fields["handler_version"], "/handler_version", 1, 1000, 0, &e)
	prio := intField(fields["priority"], "/priority", 0, 9, 0, &e)
	maxA := intField(fields["max_attempts"], "/max_attempts", 1, 5, 3, &e)
	var runAfter *time.Time
	if raw := fields["run_after"]; raw != nil && string(raw) != "null" {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			e.add("/run_after", "type", "must be an RFC 3339 date-time or null", nil)
		} else if t, err := time.Parse(time.RFC3339Nano, s); err != nil {
			e.add("/run_after", "format", "must be an RFC 3339 date-time", nil)
		} else {
			t = t.UTC().Truncate(time.Microsecond)
			runAfter = &t
		}
	}
	entry, known := v.Catalog.Lookup(hn, hv)
	defTimeout := 600
	if known {
		defTimeout = entry.Spec.DefaultTimeoutSeconds
	}
	timeout := intField(fields["timeout_seconds"], "/timeout_seconds", 1, 86400, defTimeout, &e)
	var payload any
	var compact bytes.Buffer
	if raw, ok := fields["payload"]; !ok {
		e.add("/payload", "required", "is required", nil)
	} else {
		if err := json.Compact(&compact, raw); err != nil {
			writeError(w, r, http.StatusBadRequest, "bad_request", "Request body is not valid JSON.")
			return
		}
		if compact.Len() > MaxPayloadBytes {
			writeErrorDetails(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Payload exceeds 65536 bytes.",
				map[string]any{"limit_bytes": MaxPayloadBytes, "field": "payload"})
			return
		}
		payload, _ = decodeAny(compact.Bytes())
		if depth, nul := depthAndNUL(payload); depth > MaxJSONDepth {
			e.add("/payload", "range", "nesting depth exceeds 32", map[string]any{"max": MaxJSONDepth})
		} else if nul {
			e.add("/payload", "format", "strings must not contain NUL characters", nil)
		}
	}
	if hn != "" && hv != 0 && !known && handlerRe.MatchString(hn) {
		e.add("/handler_name", "registered", fmt.Sprintf("unknown handler %s@%d", hn, hv), nil)
	}
	if known && payload != nil && len(e) == 0 {
		for _, fe := range entry.Validate(payload, "/payload") {
			e.add(fe.Pointer, "schema", fe.Message, nil)
		}
	}
	key := r.Header.Get("Idempotency-Key")
	if _, present := r.Header["Idempotency-Key"]; present && !idemKeyRe.MatchString(key) {
		e.add("header:Idempotency-Key", "pattern", "must match ^[A-Za-z0-9_.:-]{1,128}$", nil)
	}
	if e.write(w, r) {
		return
	}
	p := principal(r)
	params := store.AdmitParams{Principal: p.Ref, Namespace: ns, HandlerName: hn, HandlerVersion: hv, Priority: prio,
		MaxAttempts: maxA, TimeoutSeconds: timeout, Payload: compact.Bytes(), RunAfter: runAfter, IdempotencyKey: key}
	if key != "" {
		var ra any
		if runAfter != nil {
			ra = runAfter.Format(time.RFC3339Nano)
		}
		fp, err := fingerprint(map[string]any{"namespace": ns, "handler_name": hn, "handler_version": hv, "payload": payload,
			"priority": prio, "run_after": ra, "max_attempts": maxA, "timeout_seconds": timeout})
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "bad_request", "Payload cannot be canonicalized.")
			return
		}
		params.Fingerprint = fp
	}
	job, replayed, err := v.Store.Admit(r.Context(), params)
	if err != nil {
		if d, ok := errors.AsType[*store.Denial](err); ok {
			switch d.Reason {
			case "idempotency_conflict":
				writeErrorDetails(w, r, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was used with a different request.",
					map[string]any{"job_id": d.Detail})
				return
			case "validation_failed":
				e.add("/run_after", "too_far_in_future", "must be at most 30 days ahead (DB clock)", map[string]any{"max_days": 30})
				e.write(w, r)
				return
			}
		}
		v.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/jobs/"+job.ID)
	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
		writeJSON(w, http.StatusOK, jobView(job, p.Role))
		return
	}
	writeJSON(w, http.StatusCreated, jobView(job, p.Role))
}

// ---- views ------------------------------------------------------------------------------------

func ts(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}

func tv(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func rawOrNull(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("null")
	}
	return b
}

type jobSummaryJSON struct {
	ID                  string  `json:"id"`
	Namespace           string  `json:"namespace"`
	HandlerName         string  `json:"handler_name"`
	HandlerVersion      int     `json:"handler_version"`
	Status              string  `json:"status"`
	Priority            int     `json:"priority"`
	AttemptsUsed        int     `json:"attempts_used"`
	MaxAttempts         int     `json:"max_attempts"`
	AvailableAt         string  `json:"available_at"`
	CancelRequestedAt   *string `json:"cancel_requested_at"`
	TerminalAt          *string `json:"terminal_at"`
	ErrorCode           *string `json:"error_code"`
	RetriedFromJobID    *string `json:"retried_from_job_id"`
	UnschedulableReason *string `json:"unschedulable_reason"`
	UnschedulableSince  *string `json:"unschedulable_since"`
	FailureClass        *string `json:"failure_class"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	Version             int64   `json:"version"`
}

type jobJSON struct {
	jobSummaryJSON
	Payload          json.RawMessage `json:"payload"`
	Result           json.RawMessage `json:"result"`
	ErrorMessage     *string         `json:"error_message"`
	RunAfter         *string         `json:"run_after"`
	TimeoutSeconds   int             `json:"timeout_seconds"`
	CurrentAttemptID *string         `json:"current_attempt_id"`
	LeaseGeneration  int64           `json:"lease_generation"`
	LeaseExpiresAt   *string         `json:"lease_expires_at"`
	HardDeadlineAt   *string         `json:"hard_deadline_at"`
	CreatedBy        string          `json:"created_by"`
	RetriedAsJobID   *string         `json:"retried_as_job_id"`
	AsOf             string          `json:"as_of"`
	MaskedFields     []string        `json:"masked_fields"`
}

func summaryView(j store.Job) jobSummaryJSON {
	return jobSummaryJSON{ID: j.ID, Namespace: j.Namespace, HandlerName: j.HandlerName, HandlerVersion: j.HandlerVersion,
		Status: j.Status, Priority: j.Priority, AttemptsUsed: j.AttemptsUsed, MaxAttempts: j.MaxAttempts,
		AvailableAt: tv(j.AvailableAt), CancelRequestedAt: ts(j.CancelRequestedAt), TerminalAt: ts(j.TerminalAt),
		ErrorCode: j.ErrorCode, RetriedFromJobID: j.RetriedFromJobID, UnschedulableReason: j.UnschedulableReason,
		UnschedulableSince: ts(j.UnschedulableSince), FailureClass: j.FailureClass, CreatedAt: tv(j.CreatedAt),
		UpdatedAt: tv(j.UpdatedAt), Version: j.Version}
}

// jobView applies the field matrix: viewers get payload, result and error_message as null + masked_fields.
func jobView(j store.Job, role string) jobJSON {
	out := jobJSON{jobSummaryJSON: summaryView(j), Payload: rawOrNull(j.Payload), Result: rawOrNull(j.Result),
		ErrorMessage: j.ErrorMessage, RunAfter: ts(j.RunAfter), TimeoutSeconds: j.TimeoutSeconds,
		CurrentAttemptID: j.CurrentAttemptID, LeaseGeneration: j.LeaseGeneration, LeaseExpiresAt: ts(j.LeaseExpiresAt),
		HardDeadlineAt: ts(j.HardDeadlineAt), CreatedBy: j.CreatedBy, RetriedAsJobID: j.RetriedAsJobID, AsOf: tv(j.AsOf),
		MaskedFields: []string{}}
	if role != "operator" {
		out.Payload, out.Result, out.ErrorMessage = json.RawMessage("null"), json.RawMessage("null"), nil
		out.MaskedFields = []string{"payload", "result", "error_message"}
	}
	return out
}

// ---- GET jobs ---------------------------------------------------------------------------------

func validJobID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, r, http.StatusNotFound, "job_not_found", "Job not found.")
		return "", false
	}
	return strings.ToLower(id), true
}

func (v *V1) getJob(w http.ResponseWriter, r *http.Request) {
	id, ok := validJobID(w, r)
	if !ok {
		return
	}
	j, err := v.Store.GetJob(r.Context(), id)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	w.Header().Set("ETag", fmt.Sprintf(`W/"%d"`, j.Version))
	writeJSON(w, http.StatusOK, jobView(j, principal(r).Role))
}

type attemptJSON struct {
	ID              string   `json:"id"`
	JobID           string   `json:"job_id"`
	AttemptNumber   int      `json:"attempt_number"`
	LeaseGeneration int64    `json:"lease_generation"`
	WorkerID        string   `json:"worker_id"`
	IncarnationID   string   `json:"worker_incarnation_id"`
	State           string   `json:"state"`
	ErrorClass      *string  `json:"error_class"`
	ErrorCode       *string  `json:"error_code"`
	ErrorMessage    *string  `json:"error_message"`
	LogTail         *string  `json:"log_tail"`
	LogTruncated    bool     `json:"log_truncated"`
	LogExpired      bool     `json:"log_expired"`
	LeaseExpiresAt  *string  `json:"lease_expires_at"`
	StartedAt       string   `json:"started_at"`
	FinishedAt      *string  `json:"finished_at"`
	MaskedFields    []string `json:"masked_fields"`
}

func (v *V1) listAttempts(w http.ResponseWriter, r *http.Request) {
	id, ok := validJobID(w, r)
	if !ok {
		return
	}
	as, version, err := v.Store.ListAttempts(r.Context(), id)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	viewer := principal(r).Role != "operator"
	items := make([]attemptJSON, 0, len(as))
	for _, a := range as {
		x := attemptJSON{ID: a.ID, JobID: a.JobID, AttemptNumber: a.Number, LeaseGeneration: a.LeaseGeneration, WorkerID: a.WorkerID,
			IncarnationID: a.IncarnationID, State: a.State, ErrorClass: a.ErrorClass, ErrorCode: a.ErrorCode, ErrorMessage: a.ErrorMessage,
			LogTail: a.LogTail, LogTruncated: a.LogTruncated, LogExpired: a.LogExpired, LeaseExpiresAt: ts(a.LeaseExpiresAt),
			StartedAt: tv(a.StartedAt), FinishedAt: ts(a.FinishedAt), MaskedFields: []string{}}
		if viewer {
			x.ErrorMessage, x.LogTail, x.MaskedFields = nil, nil, []string{"error_message", "log_tail"}
		}
		items = append(items, x)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "job_version": version})
}

type eventJSON struct {
	JobID      string          `json:"job_id"`
	Version    int64           `json:"version"`
	EventType  string          `json:"event_type"`
	FromStatus *string         `json:"from_status"`
	ToStatus   string          `json:"to_status"`
	AttemptID  *string         `json:"attempt_id"`
	Actor      string          `json:"actor"`
	OccurredAt string          `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

func (v *V1) listEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := validJobID(w, r)
	if !ok {
		return
	}
	var e verrs
	q := r.URL.Query()
	after := int64(0)
	if s := q.Get("after_version"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 0 {
			e.add("?after_version", "range", "must be an integer >= 0", map[string]any{"min": 0})
		}
		after = n
	}
	limit := queryInt(q, "limit", 1, 200, 100, &e)
	if e.write(w, r) {
		return
	}
	evs, version, more, err := v.Store.ListEvents(r.Context(), id, after, limit)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	items := make([]eventJSON, 0, len(evs))
	for _, ev := range evs {
		items = append(items, eventJSON{JobID: ev.JobID, Version: ev.Version, EventType: ev.EventType, FromStatus: ev.FromStatus,
			ToStatus: ev.ToStatus, AttemptID: ev.AttemptID, Actor: ev.Actor, OccurredAt: tv(ev.OccurredAt), Data: rawOrNull(ev.Data)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "job_version": version, "has_more": more})
}

func queryInt(q url.Values, name string, lo, hi, def int, e *verrs) int {
	s := q.Get(name)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		e.add("?"+name, "type", "must be an integer", nil)
		return def
	}
	if n < lo || n > hi {
		e.add("?"+name, "range", fmt.Sprintf("must be between %d and %d", lo, hi), map[string]any{"min": lo, "max": hi})
	}
	return n
}

func queryTime(q url.Values, name string, e *verrs) *time.Time {
	s := q.Get(name)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		e.add("?"+name, "format", "must be an RFC 3339 date-time", nil)
		return nil
	}
	return &t
}

func queryEnum(q url.Values, name string, allowed []string, e *verrs) []string {
	vals := q[name]
	for _, v := range vals {
		if !slices.Contains(allowed, v) {
			e.add("?"+name, "enum", "must be one of "+strings.Join(allowed, ", "), nil)
		}
	}
	return vals
}

// ---- cursors ----------------------------------------------------------------------------------

type cursorJSON struct {
	S string `json:"s"`
	F string `json:"f"`
	T string `json:"t,omitempty"`
	P int    `json:"p,omitempty"`
	N string `json:"n,omitempty"`
	I string `json:"i"`
}

// filterHash binds a cursor to the query (minus cursor and limit).
func filterHash(q url.Values) string {
	c := url.Values{}
	for k, v := range q {
		if k != "cursor" && k != "limit" {
			s := slices.Clone(v)
			sort.Strings(s)
			c[k] = s
		}
	}
	h := sha256.Sum256([]byte(c.Encode()))
	return hex.EncodeToString(h[:8])
}

func encodeCursor(c cursorJSON) *string {
	b, _ := json.Marshal(c)
	s := base64.RawURLEncoding.EncodeToString(b)
	return &s
}

func decodeCursor(q url.Values, sort string) (*store.Cursor, bool) {
	s := q.Get("cursor")
	if s == "" {
		return nil, true
	}
	if len(s) > 512 {
		return nil, false
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	var c cursorJSON
	if err != nil || json.Unmarshal(b, &c) != nil || c.S != sort || c.F != filterHash(q) || !uuidRe.MatchString(c.I) {
		return nil, false
	}
	out := &store.Cursor{Priority: c.P, Name: c.N, ID: c.I}
	if c.T != "" {
		t, err := time.Parse(time.RFC3339Nano, c.T)
		if err != nil {
			return nil, false
		}
		out.T = t
	}
	return out, true
}

func invalidCursor(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusBadRequest, "invalid_cursor", "Cursor does not match the current filters.")
}

var (
	jobStatuses   = []string{"scheduled", "queued", "running", "retry_wait", "succeeded", "failed", "canceled"}
	unschedReason = []string{"no_capable_worker", "capable_workers_offline", "capable_workers_draining"}
	failureClass  = []string{"retryable_exhausted", "permanent", "timeout", "lease_expired", "canceled_by_operator"}
	jobSorts      = []string{"-created_at", "created_at", "-available_at", "available_at", "-priority"}
)

func (v *V1) listJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var e verrs
	f := store.JobFilter{Sort: "-created_at"}
	f.Statuses = queryEnum(q, "status", jobStatuses, &e)
	if len(f.Statuses) > 7 {
		e.add("?status", "range", "at most 7 values", map[string]any{"max": 7})
	}
	f.Unschedulable = queryEnum(q, "unschedulable_reason", unschedReason, &e)
	f.FailureClasses = queryEnum(q, "failure_class", failureClass, &e)
	if s := q.Get("sort"); s != "" {
		f.Sort = s
		queryEnum(q, "sort", jobSorts, &e)
	}
	if s := q.Get("handler_name"); s != "" {
		if !handlerRe.MatchString(s) {
			e.add("?handler_name", "pattern", "does not match "+handlerRe.String(), nil)
		}
		f.HandlerName = s
	}
	if q.Has("handler_version") {
		n := queryInt(q, "handler_version", 1, 1000, 0, &e)
		f.HandlerVersion = &n
		if f.HandlerName == "" {
			e.add("?handler_version", "required", "requires handler_name", nil)
		}
	}
	if q.Has("priority_min") {
		n := queryInt(q, "priority_min", 0, 9, 0, &e)
		f.PriorityMin = &n
	}
	if q.Has("priority_max") {
		n := queryInt(q, "priority_max", 0, 9, 9, &e)
		f.PriorityMax = &n
	}
	if s := q.Get("retried_from_job_id"); s != "" {
		if !uuidRe.MatchString(s) {
			e.add("?retried_from_job_id", "format", "must be a UUID", nil)
		}
		f.RetriedFrom = s
	}
	f.CreatedAfter, f.CreatedBefore = queryTime(q, "created_after", &e), queryTime(q, "created_before", &e)
	f.AvailableAfter, f.AvailableBefore = queryTime(q, "available_after", &e), queryTime(q, "available_before", &e)
	const maxWindow = 31 * 24 * time.Hour
	switch a, b := f.CreatedAfter, f.CreatedBefore; {
	case a != nil && b != nil && !b.After(*a):
		e.add("?created_before", "window_inverted", "must be after created_after", nil)
	case a != nil && b != nil && b.Sub(*a) > maxWindow:
		e.add("?created_after", "window_too_wide", "window must be at most 31 days", map[string]any{"max_days": 31})
	case a != nil && b == nil && time.Since(*a) > maxWindow: // validation bound only, not an authorization decision
		e.add("?created_after", "window_too_wide", "window must be at most 31 days", map[string]any{"max_days": 31})
	}
	f.Limit = queryInt(q, "limit", 1, 200, 50, &e)
	if e.write(w, r) {
		return
	}
	cur, ok := decodeCursor(q, f.Sort)
	if !ok {
		invalidCursor(w, r)
		return
	}
	f.After = cur
	jobs, more, asOf, err := v.Store.ListJobs(r.Context(), f)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	items := make([]jobSummaryJSON, 0, len(jobs))
	for _, j := range jobs {
		items = append(items, summaryView(j))
	}
	var next *string
	if more && len(jobs) > 0 {
		last := jobs[len(jobs)-1]
		c := cursorJSON{S: f.Sort, F: filterHash(q), I: last.ID, T: tv(last.CreatedAt)}
		switch f.Sort {
		case "-available_at", "available_at":
			c.T = tv(last.AvailableAt)
		case "-priority":
			c.T, c.P = tv(last.AvailableAt), last.Priority
		}
		next = encodeCursor(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next, "as_of": tv(asOf)})
}

// ---- workers ----------------------------------------------------------------------------------

type capJSON struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type workerJSON struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Status              string    `json:"status"`
	Live                bool      `json:"live"`
	Version             int64     `json:"version"`
	IncarnationID       string    `json:"incarnation_id"`
	Capacity            int       `json:"capacity"`
	ActiveAttempts      int       `json:"active_attempts"`
	ProtocolVersion     int       `json:"protocol_version"`
	SDKVersion          *string   `json:"sdk_version"`
	Hostname            *string   `json:"hostname"`
	Capabilities        []capJSON `json:"capabilities"`
	RegisteredAt        string    `json:"registered_at"`
	LastSeenAt          string    `json:"last_seen_at"`
	DrainRequestedAt    *string   `json:"drain_requested_at"`
	DrainDeadlineAt     *string   `json:"drain_deadline_at"`
	CredentialRevoked   bool      `json:"credential_revoked"`
	CredentialRevokedAt *string   `json:"credential_revoked_at"`
	MaskedFields        []string  `json:"masked_fields"`
}

func workerView(x store.Worker, role string) workerJSON {
	caps := make([]capJSON, 0, len(x.Capabilities))
	for _, c := range x.Capabilities {
		caps = append(caps, capJSON{c.Name, c.Version})
	}
	out := workerJSON{ID: x.ID, Name: x.Name, Status: x.Status, Live: x.Live, Version: x.Version, IncarnationID: x.IncarnationID,
		Capacity: x.Capacity, ActiveAttempts: x.ActiveAttempts, ProtocolVersion: x.ProtocolVersion, SDKVersion: x.SDKVersion,
		Hostname: x.Hostname, Capabilities: caps, RegisteredAt: tv(x.RegisteredAt), LastSeenAt: tv(x.LastSeenAt),
		DrainRequestedAt: ts(x.DrainRequestedAt), DrainDeadlineAt: ts(x.DrainDeadlineAt), CredentialRevoked: x.CredentialRevoked,
		CredentialRevokedAt: ts(x.CredentialRevokedAt), MaskedFields: []string{}}
	if role != "operator" {
		out.Hostname, out.MaskedFields = nil, []string{"hostname"}
	}
	return out
}

func (v *V1) listWorkers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var e verrs
	f := store.WorkerFilter{Sort: "-last_seen_at"}
	f.Statuses = queryEnum(q, "status", []string{"active", "draining", "deregistered"}, &e)
	if s := q.Get("liveness"); s != "" {
		queryEnum(q, "liveness", []string{"live", "stale"}, &e)
		f.Liveness = s
	}
	if s := q.Get("sort"); s != "" {
		queryEnum(q, "sort", []string{"-last_seen_at", "name", "-registered_at"}, &e)
		f.Sort = s
	}
	if s := q.Get("handler_name"); s != "" {
		if !handlerRe.MatchString(s) {
			e.add("?handler_name", "pattern", "does not match "+handlerRe.String(), nil)
		}
		f.HandlerName = s
	}
	f.Limit = queryInt(q, "limit", 1, 200, 50, &e)
	if e.write(w, r) {
		return
	}
	cur, ok := decodeCursor(q, f.Sort)
	if !ok {
		invalidCursor(w, r)
		return
	}
	f.After = cur
	ws, more, asOf, err := v.Store.ListWorkers(r.Context(), f)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	role := principal(r).Role
	items := make([]workerJSON, 0, len(ws))
	for _, x := range ws {
		items = append(items, workerView(x, role))
	}
	var next *string
	if more && len(ws) > 0 {
		last := ws[len(ws)-1]
		c := cursorJSON{S: f.Sort, F: filterHash(q), I: last.ID, T: tv(last.LastSeenAt)}
		switch f.Sort {
		case "name":
			c.T, c.N = "", last.Name
		case "-registered_at":
			c.T = tv(last.RegisteredAt)
		}
		next = encodeCursor(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next, "as_of": tv(asOf)})
}

func (v *V1) getWorker(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, r, http.StatusNotFound, "worker_not_found", "Worker not found.")
		return
	}
	x, act, err := v.Store.GetWorker(r.Context(), strings.ToLower(id))
	if err != nil {
		v.fail(w, r, err)
		return
	}
	type activeJSON struct {
		JobID           string `json:"job_id"`
		AttemptID       string `json:"attempt_id"`
		AttemptNumber   int    `json:"attempt_number"`
		HandlerName     string `json:"handler_name"`
		StartedAt       string `json:"started_at"`
		LeaseExpiresAt  string `json:"lease_expires_at"`
		CancelRequested bool   `json:"cancel_requested"`
	}
	active := make([]activeJSON, 0, len(act))
	for _, a := range act {
		active = append(active, activeJSON{a.JobID, a.AttemptID, a.AttemptNumber, a.HandlerName, tv(a.StartedAt), tv(a.LeaseExpiresAt), a.CancelRequested})
	}
	writeJSON(w, http.StatusOK, struct {
		workerJSON
		Active []activeJSON `json:"active"`
		AsOf   string       `json:"as_of"`
	}{workerView(x, principal(r).Role), active, tv(x.AsOf)})
}

// ---- catalogue and config ---------------------------------------------------------------------

func (v *V1) listHandlers(w http.ResponseWriter, r *http.Request) {
	counts, asOf, err := v.Store.HandlerCapacity(r.Context())
	if err != nil {
		v.fail(w, r, err)
		return
	}
	type versionJSON struct {
		Version               int             `json:"version"`
		PayloadSchema         json.RawMessage `json:"payload_schema"`
		ResultSchema          json.RawMessage `json:"result_schema"`
		DefaultTimeoutSeconds int             `json:"default_timeout_seconds"`
		CapableWorkers        int             `json:"capable_workers"`
		LiveCapableWorkers    int             `json:"live_capable_workers"`
	}
	type handlerJSON struct {
		Name        string        `json:"name"`
		Description *string       `json:"description"`
		Versions    []versionJSON `json:"versions"`
	}
	var items []handlerJSON
	for _, s := range v.Catalog.Specs() {
		if len(items) == 0 || items[len(items)-1].Name != s.Name {
			d := s.Description
			items = append(items, handlerJSON{Name: s.Name, Description: &d})
		}
		c := counts[fmt.Sprintf("%s@%d", s.Name, s.Version)]
		h := &items[len(items)-1]
		h.Versions = append(h.Versions, versionJSON{s.Version, s.PayloadSchema, rawOrNull(s.ResultSchema), s.DefaultTimeoutSeconds, c.Capable, c.Live})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "as_of": tv(asOf)})
}

func (v *V1) getConfig(w http.ResponseWriter, r *http.Request) {
	sec := func(d time.Duration) int { return int(d / time.Second) }
	writeJSON(w, http.StatusOK, map[string]any{
		"api_version": APIVersion, "contract_version": APIVersion, "environment": v.Environment,
		"build": map[string]string{"version": v.Version, "commit": v.Commit},
		"limits": map[string]int{
			"payload_max_bytes": MaxPayloadBytes, "result_max_bytes": store.MaxResultBytes, "request_body_max_bytes": MaxBodyBytes,
			"log_tail_max_bytes": store.MaxLogTailBytes, "max_attempts_min": 1, "max_attempts_max": 5, "max_attempts_default": 3,
			"timeout_seconds_max": 86400, "priority_min": 0, "priority_max": 9, "page_size_max": 200, "page_size_default": 50,
			"jobs_list_window_max_days": 31, "run_after_max_days": 30, "timeseries_points_max": 2016,
			"timeseries_window_max_days": 7, "idempotency_ttl_hours": 24,
		},
		"retention": map[string]int{"attempt_logs_days": 7, "terminal_jobs_days": 30, "audit_days": 90, "tombstone_days": 90},
		"timing": map[string]int{
			"heartbeat_interval_seconds": sec(v.Timing.HeartbeatInterval), "liveness_timeout_seconds": sec(v.Timing.LivenessTimeout),
			"lease_ttl_seconds": sec(v.Timing.LeaseTTL), "cancel_grace_seconds": sec(v.Timing.CancelGrace),
			"drain_grace_default_seconds": sec(v.Timing.DrainGrace),
		},
	})
}
