package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

var conflictMessage = map[string]string{
	"job_terminal":         "Job already finished.",
	"retry_not_allowed":    "Only failed jobs can be retried.",
	"retry_already_exists": "This job was already retried.",
	"worker_deregistered":  "Worker is deregistered.",
}

// smallBody bounds JSON bodies of non-job mutations.
const smallBody = 8 << 10

// readObject reads an optional (or required) JSON object body with known fields only. It writes the
// error response and returns ok=false on failure. An absent body yields an empty map when optional.
func readObject(w http.ResponseWriter, r *http.Request, required bool, known ...string) (map[string]json.RawMessage, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, smallBody))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeErrorDetails(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", fmt.Sprintf("Request body exceeds %d bytes.", smallBody),
				map[string]any{"limit_bytes": smallBody, "field": "body"})
			return nil, false
		}
		writeError(w, r, http.StatusBadRequest, "bad_request", "Could not read the request body.")
		return nil, false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		if required {
			writeError(w, r, http.StatusBadRequest, "bad_request", "Request body is required.")
			return nil, false
		}
		return map[string]json.RawMessage{}, true
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Content-Type must be application/json.")
		return nil, false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || fields == nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Request body is not a valid JSON object.")
		return nil, false
	}
	var e verrs
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		found := false
		for _, kk := range known {
			found = found || kk == k
		}
		if !found {
			e.add("/"+k, "unknown_field", "is not allowed", nil)
		}
	}
	if e.write(w, r) {
		return nil, false
	}
	return fields, true
}

func meta(r *http.Request) store.AuditMeta {
	return store.AuditMeta{RequestID: RequestID(r.Context()), Kid: principal(r).Kid}
}

// ---- POST /jobs/{id}/cancel -------------------------------------------------------------------

func (v *V1) cancelJob(w http.ResponseWriter, r *http.Request) {
	id, ok := validJobID(w, r)
	if !ok {
		return
	}
	p := principal(r)
	res, err := v.Store.CancelJob(r.Context(), id, p.Ref, meta(r))
	if err != nil {
		v.fail(w, r, err)
		return
	}
	if res.HTTP == http.StatusConflict {
		j, err := v.Store.GetJob(r.Context(), id)
		d := map[string]any{"status": res.Status, "version": res.Version}
		if err == nil && j.TerminalAt != nil {
			d["terminal_at"] = tv(*j.TerminalAt)
		}
		writeErrorDetails(w, r, http.StatusConflict, "job_terminal", conflictMessage["job_terminal"], d)
		return
	}
	v.Hub.PublishJob(id, res.Version)
	j, err := v.Store.GetJob(r.Context(), id)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	writeJSON(w, res.HTTP, jobView(j, p.Role))
}

// ---- POST /jobs/{id}/retry --------------------------------------------------------------------

func (v *V1) retryJob(w http.ResponseWriter, r *http.Request) {
	id, ok := validJobID(w, r)
	if !ok {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if _, present := r.Header["Idempotency-Key"]; present && !idemKeyRe.MatchString(key) {
		var e verrs
		e.add("header:Idempotency-Key", "pattern", "must match ^[A-Za-z0-9_.:-]{1,128}$", nil)
		e.write(w, r)
		return
	}
	p := principal(r)
	j, replayed, err := v.Store.ManualRetry(r.Context(), id, p.Ref, key, meta(r))
	if err != nil {
		v.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/jobs/"+j.ID)
	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
		writeJSON(w, http.StatusOK, jobView(j, p.Role))
		return
	}
	v.Hub.PublishJob(j.ID, j.Version)
	writeJSON(w, http.StatusCreated, jobView(j, p.Role))
}

// ---- POST /workers/{id}/drain -----------------------------------------------------------------

func (v *V1) drainWorker(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, r, http.StatusNotFound, "worker_not_found", "Worker not found.")
		return
	}
	id = strings.ToLower(id)
	fields, ok := readObject(w, r, false, "grace_seconds")
	if !ok {
		return
	}
	var e verrs
	grace := intField(fields["grace_seconds"], "/grace_seconds", 0, 3600, int(v.Timing.DrainGrace/time.Second), &e)
	if e.write(w, r) {
		return
	}
	p := principal(r)
	if err := v.Store.OperatorDrain(r.Context(), id, time.Duration(grace)*time.Second, p.Ref, meta(r)); err != nil {
		v.fail(w, r, err)
		return
	}
	x, _, err := v.Store.GetWorker(r.Context(), id)
	if err != nil {
		v.fail(w, r, err)
		return
	}
	v.Hub.PublishWorker(id, x.Version)
	writeJSON(w, http.StatusAccepted, workerView(x, p.Role))
}
