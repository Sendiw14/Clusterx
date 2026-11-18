package sdk

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sync"
)

// Limits shared with the coordinator (ADR-0004).
const (
	MaxPayloadBytes = 64 << 10  // job payload JSON
	MaxResultBytes  = 128 << 10 // result JSON; larger results fail permanently, never truncated
	MinVersion      = 1
	MaxVersion      = 1000
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,62}$`)

// HandlerKey identifies a handler by name and version, e.g. {"image.resize", 2}.
type HandlerKey struct {
	Name    string
	Version int
}

func (k HandlerKey) String() string { return fmt.Sprintf("%s@v%d", k.Name, k.Version) }

// ErrUnknownHandler is returned by [Registry.Invoke] for a name and version that was not registered.
var ErrUnknownHandler = errors.New("clusterx: unknown handler")

type invokeFunc func(ctx context.Context, job Job, payload []byte) ([]byte, error)

// Registry holds the handlers a worker can execute. It is safe for concurrent use; the zero
// value is ready to use.
type Registry struct {
	mu       sync.RWMutex
	handlers map[HandlerKey]invokeFunc
}

// Register adds h under name and version. The name must match ^[a-z][a-z0-9_.-]{0,62}$ and the
// version must be 1..1000; registering the same name and version twice is an error.
func Register[P, R any](r *Registry, name string, version int, h Handler[P, R]) error {
	key := HandlerKey{Name: name, Version: version}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("clusterx: invalid handler name %q", name)
	}
	if version < MinVersion || version > MaxVersion {
		return fmt.Errorf("clusterx: handler %q version %d outside %d..%d", name, version, MinVersion, MaxVersion)
	}
	if h == nil {
		return fmt.Errorf("clusterx: nil handler for %s", key)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.handlers[key]; dup {
		return fmt.Errorf("clusterx: handler %s already registered", key)
	}
	if r.handlers == nil {
		r.handlers = make(map[HandlerKey]invokeFunc)
	}
	r.handlers[key] = adapt(h)
	return nil
}

// MustRegister is like [Register] but panics on error; intended for program initialization.
func MustRegister[P, R any](r *Registry, name string, version int, h Handler[P, R]) {
	if err := Register(r, name, version, h); err != nil {
		panic(err)
	}
}

// Keys returns the registered handlers sorted by name then version; a worker advertises these
// as its capabilities.
func (r *Registry) Keys() []HandlerKey {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]HandlerKey, 0, len(r.handlers))
	for k := range r.handlers {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b HandlerKey) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Version, b.Version))
	})
	return keys
}

// Invoke decodes payload, runs the handler registered for job.Handler and returns the encoded
// result. Decode, encode and size failures are returned as *PermanentError; a handler panic is
// recovered and returned as a *RetryableError with code [CodeHandlerPanic].
func (r *Registry) Invoke(ctx context.Context, job Job, payload []byte) ([]byte, error) {
	r.mu.RLock()
	fn, ok := r.handlers[job.Handler]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownHandler, job.Handler)
	}
	return fn(ctx, job, payload)
}

func adapt[P, R any](h Handler[P, R]) invokeFunc {
	return func(ctx context.Context, job Job, payload []byte) (out []byte, err error) {
		if len(payload) > MaxPayloadBytes {
			return nil, Permanent(CodeInvalidPayload, fmt.Errorf("payload is %d bytes, limit %d", len(payload), MaxPayloadBytes))
		}
		var p P
		dec := json.NewDecoder(bytes.NewReader(payload))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			return nil, Permanent(CodeInvalidPayload, err)
		}
		if dec.More() {
			return nil, Permanent(CodeInvalidPayload, errors.New("trailing data after JSON value"))
		}

		defer func() {
			if rec := recover(); rec != nil {
				out, err = nil, Retryable(CodeHandlerPanic, fmt.Errorf("panic: %v", rec))
			}
		}()
		res, err := h.Handle(ctx, job, p)
		if err != nil {
			return nil, err
		}
		out, err = json.Marshal(res)
		if err != nil {
			return nil, Permanent(CodeResultEncoding, err)
		}
		if len(out) > MaxResultBytes {
			return nil, Permanent(CodeResultTooLarge, fmt.Errorf("result is %d bytes, limit %d", len(out), MaxResultBytes))
		}
		return out, nil
	}
}
