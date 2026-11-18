package sdk

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAttemptLoggerBoundedAndRedacted(t *testing.T) {
	var base bytes.Buffer
	l, tail := newAttemptLogger(slog.New(slog.NewTextHandler(&base, nil)))
	l.Info("start", "api_token", "cxt_deadbeef_SECRETVALUE", "Authorization", "Bearer x", "n", 1)
	l.With("db_password", "hunter2").Info("with")
	for i := 0; i < 2000; i++ {
		l.Info("line", "i", i, "s", "é漢字")
	}
	got := tail.String()
	if len(got) > logCaptureBytes || len(got) <= MaxLogTailBytes {
		t.Fatalf("tail length %d, want (%d, %d]", len(got), MaxLogTailBytes, logCaptureBytes)
	}
	if !utf8.ValidString(got) {
		t.Fatal("tail is not valid UTF-8")
	}
	if !strings.Contains(got, "i=1999") {
		t.Fatal("tail lost the newest line")
	}
	for _, out := range []string{got, base.String()} {
		for _, leak := range []string{"SECRETVALUE", "Bearer x", "hunter2"} {
			if strings.Contains(out, leak) {
				t.Fatalf("secret %q leaked", leak)
			}
		}
	}
	if !strings.Contains(base.String(), "api_token=[redacted]") {
		t.Fatal("worker log did not receive the redacted record")
	}
}

func TestAttemptLoggerShortLogKept(t *testing.T) {
	l, tail := newAttemptLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	l.Info("hello", "k", "v")
	if s := tail.String(); !strings.Contains(s, "msg=hello k=v") {
		t.Fatalf("tail = %q", s)
	}
}
