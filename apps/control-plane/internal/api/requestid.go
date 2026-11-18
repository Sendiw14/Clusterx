package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
)

// HeaderRequestID carries the request correlation id in both directions.
const HeaderRequestID = "X-Request-Id"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

type ctxKey struct{}

// RequestID returns the request id stored by the middleware, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// newRequestID returns "req_" plus 24 hex characters (96 random bits).
func newRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}

// requestIDFrom echoes a well-formed inbound X-Request-Id; anything else is replaced, so ids
// in logs can never carry injected control characters.
func requestIDFrom(r *http.Request) string {
	if id := r.Header.Get(HeaderRequestID); validRequestID.MatchString(id) {
		return id
	}
	return newRequestID()
}
