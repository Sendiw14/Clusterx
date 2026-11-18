// Package basicjobs holds the reference handlers of the CLUSTERX example deployment. They are
// deterministic and bounded: no shell, no URL fetches, no filesystem access (MASTER §5, AC-12).
//
//   - sleep_ms v1: sleeps {"ms": 0..60000}, honoring context cancellation; returns {"slept_ms": n}.
//   - hash_sha256 v1: hashes {"data": string <= 16384 bytes}; returns {"sha256": "<hex>"}.
//   - fail_n_times v1: fails retryably while the attempt number is <= {"n": 0..5} (permanently instead
//     when "permanent" is true), then returns {"attempt": k}. Deterministic across processes.
package basicjobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sdk "github.com/Sendiw14/Clusterx/packages/sdk-go"
)

// Limits enforced by the handlers themselves (the coordinator enforces the same via the schemas).
const (
	MaxSleepMS   = 60000
	MaxHashBytes = 16384
	MaxFailN     = 5
)

type SleepIn struct {
	MS int `json:"ms"`
}
type SleepOut struct {
	SleptMS int `json:"slept_ms"`
}
type HashIn struct {
	Data string `json:"data"`
}
type HashOut struct {
	SHA256 string `json:"sha256"`
}
type FailIn struct {
	N         int  `json:"n"`
	Permanent bool `json:"permanent,omitempty"`
}
type FailOut struct {
	Attempt int `json:"attempt"`
}

// Sleep is sleep_ms v1.
func Sleep(ctx context.Context, _ sdk.Job, in SleepIn) (SleepOut, error) {
	if in.MS < 0 || in.MS > MaxSleepMS {
		return SleepOut{}, sdk.Permanent("invalid_payload", fmt.Errorf("ms must be 0..%d", MaxSleepMS))
	}
	t := time.NewTimer(time.Duration(in.MS) * time.Millisecond)
	defer t.Stop()
	select {
	case <-t.C:
		return SleepOut{SleptMS: in.MS}, nil
	case <-ctx.Done():
		return SleepOut{}, sdk.Retryable("interrupted", context.Cause(ctx))
	}
}

// Hash is hash_sha256 v1.
func Hash(_ context.Context, _ sdk.Job, in HashIn) (HashOut, error) {
	if len(in.Data) > MaxHashBytes {
		return HashOut{}, sdk.Permanent("invalid_payload", fmt.Errorf("data exceeds %d bytes", MaxHashBytes))
	}
	sum := sha256.Sum256([]byte(in.Data))
	return HashOut{SHA256: hex.EncodeToString(sum[:])}, nil
}

// FailNTimes is fail_n_times v1.
func FailNTimes(_ context.Context, job sdk.Job, in FailIn) (FailOut, error) {
	if in.N < 0 || in.N > MaxFailN {
		return FailOut{}, sdk.Permanent("invalid_payload", fmt.Errorf("n must be 0..%d", MaxFailN))
	}
	if job.Attempt <= in.N {
		err := fmt.Errorf("planned failure %d of %d", job.Attempt, in.N)
		if in.Permanent {
			return FailOut{}, sdk.Permanent("planned_failure", err)
		}
		return FailOut{}, sdk.Retryable("planned_failure", err)
	}
	return FailOut{Attempt: job.Attempt}, nil
}

// Register adds the three handlers to reg.
func Register(reg *sdk.Registry) error {
	return errors.Join(
		sdk.Register(reg, "sleep_ms", 1, sdk.HandlerFunc[SleepIn, SleepOut](Sleep)),
		sdk.Register(reg, "hash_sha256", 1, sdk.HandlerFunc[HashIn, HashOut](Hash)),
		sdk.Register(reg, "fail_n_times", 1, sdk.HandlerFunc[FailIn, FailOut](FailNTimes)),
	)
}

// Specs is the catalogue entry of each handler (payload schemas mirror the Go types and limits).
func Specs() []sdk.HandlerSpec {
	return []sdk.HandlerSpec{
		{Name: "fail_n_times", Version: 1, DefaultTimeoutSeconds: 60,
			Description: "Fails retryably (or permanently) while attempt <= n, then succeeds. Deterministic test handler.",
			PayloadSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["n"],"properties":{"n":{"type":"integer","minimum":0,"maximum":5},"permanent":{"type":"boolean"}}}`),
			ResultSchema:  json.RawMessage(`{"type":"object","properties":{"attempt":{"type":"integer"}}}`)},
		{Name: "hash_sha256", Version: 1, DefaultTimeoutSeconds: 30,
			Description: "SHA-256 of a bounded string.",
			PayloadSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["data"],"properties":{"data":{"type":"string","maxLength":16384}}}`),
			ResultSchema:  json.RawMessage(`{"type":"object","properties":{"sha256":{"type":"string"}}}`)},
		{Name: "sleep_ms", Version: 1, DefaultTimeoutSeconds: 120,
			Description: "Sleeps for ms milliseconds (bounded), honoring cancellation.",
			PayloadSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["ms"],"properties":{"ms":{"type":"integer","minimum":0,"maximum":60000}}}`),
			ResultSchema:  json.RawMessage(`{"type":"object","properties":{"slept_ms":{"type":"integer"}}}`)},
	}
}
