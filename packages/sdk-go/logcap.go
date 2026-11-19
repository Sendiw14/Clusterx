package sdk

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"unicode/utf8"
)

// MaxLogTailBytes is the coordinator's attempt log_tail bound (ADR-0004). The SDK keeps a little more
// than this (logCaptureBytes) so that the coordinator, which keeps the last MaxLogTailBytes, can tell
// that the log was cut and set log_truncated.
const MaxLogTailBytes = 16384

const logCaptureBytes = MaxLogTailBytes + 1024

// tailBuffer keeps the last max bytes written to it, cut at a UTF-8 boundary.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - t.max; over > 0 {
		for over < len(t.buf) && !utf8.RuneStart(t.buf[over]) {
			over++
		}
		t.buf = append(t.buf[:0:0], t.buf[over:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// sensitiveKey reports attribute keys whose values are never written to the captured log or the
// worker log (CR-M5-01 #5). Handlers must still not log secrets in messages: payloads are not for
// secrets (ADR-0004), and the coordinator stores log_tail for 7 days.
func sensitiveKey(k string) bool {
	k = strings.ToLower(k)
	for _, s := range []string{"secret", "token", "password", "passwd", "authorization", "credential", "cookie", "api_key", "apikey", "private_key"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func redactAttr(_ []string, a slog.Attr) slog.Attr {
	if sensitiveKey(a.Key) {
		return slog.String(a.Key, "[redacted]")
	}
	return a
}

// teeHandler sends each record to every handler that is enabled for its level.
type teeHandler []slog.Handler

func (t teeHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range t {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (t teeHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range t {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (t teeHandler) WithAttrs(as []slog.Attr) slog.Handler {
	out := make(teeHandler, len(t))
	for i, h := range t {
		out[i] = h.WithAttrs(as)
	}
	return out
}

func (t teeHandler) WithGroup(name string) slog.Handler {
	out := make(teeHandler, len(t))
	for i, h := range t {
		out[i] = h.WithGroup(name)
	}
	return out
}

// redactingHandler applies redactAttr to record and logger attributes before delegating.
type redactingHandler struct{ slog.Handler }

func (r redactingHandler) Handle(ctx context.Context, rec slog.Record) error {
	out := slog.NewRecord(rec.Time, rec.Level, rec.Message, rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(redactAttr(nil, a))
		return true
	})
	return r.Handler.Handle(ctx, out)
}

func (r redactingHandler) WithAttrs(as []slog.Attr) slog.Handler {
	red := make([]slog.Attr, len(as))
	for i, a := range as {
		red[i] = redactAttr(nil, a)
	}
	return redactingHandler{r.Handler.WithAttrs(red)}
}

func (r redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{r.Handler.WithGroup(name)}
}

// newAttemptLogger returns the logger handed to a handler (Job.Log) and the buffer that captures its
// output as the attempt's log_tail. Records also go to the worker's own logger, with job attributes.
func newAttemptLogger(base *slog.Logger) (*slog.Logger, *tailBuffer) {
	buf := &tailBuffer{max: logCaptureBytes}
	capture := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug, ReplaceAttr: redactAttr})
	return slog.New(redactingHandler{teeHandler{capture, base.Handler()}}), buf
}
