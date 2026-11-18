package sdk

import (
	"errors"
	"fmt"
	"regexp"
)

// Error codes produced by the SDK itself.
const (
	CodeHandlerError   = "handler_error"    // untyped handler error (retryable)
	CodeInvalidPayload = "invalid_payload"  // payload does not decode into the handler's type
	CodeResultEncoding = "result_encoding"  // result cannot be encoded as JSON
	CodeResultTooLarge = "result_too_large" // encoded result exceeds MaxResultBytes
	CodeHandlerPanic   = "handler_panic"    // handler panicked
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

// RetryableError marks a failure that may succeed on a later attempt. The coordinator schedules
// a retry with backoff while attempts remain; otherwise the job fails.
type RetryableError struct {
	Code string // machine-readable, ^[a-z][a-z0-9_.-]{0,63}$
	Err  error
}

func (e *RetryableError) Error() string { return "retryable " + e.Code + ": " + errString(e.Err) }
func (e *RetryableError) Unwrap() error { return e.Err }

// PermanentError marks a failure that will not succeed on retry, such as invalid input.
// The job fails without consuming further attempts.
type PermanentError struct {
	Code string // machine-readable, ^[a-z][a-z0-9_.-]{0,63}$
	Err  error
}

func (e *PermanentError) Error() string { return "permanent " + e.Code + ": " + errString(e.Err) }
func (e *PermanentError) Unwrap() error { return e.Err }

// Retryable returns a *RetryableError. An invalid code is replaced by [CodeHandlerError].
func Retryable(code string, err error) error {
	return &RetryableError{Code: sanitizeCode(code), Err: err}
}

// Permanent returns a *PermanentError. An invalid code is replaced by [CodeHandlerError].
func Permanent(code string, err error) error {
	return &PermanentError{Code: sanitizeCode(code), Err: err}
}

// Class is how the coordinator should treat a handler failure.
type Class int

const (
	ClassRetryable Class = iota + 1
	ClassPermanent
)

func (c Class) String() string {
	switch c {
	case ClassRetryable:
		return "retryable"
	case ClassPermanent:
		return "permanent"
	}
	return fmt.Sprintf("Class(%d)", int(c))
}

// Classify reports the failure class and code of a non-nil handler error. A PermanentError
// anywhere in the chain wins over a RetryableError; untyped errors are retryable with
// [CodeHandlerError].
func Classify(err error) (Class, string) {
	if pe, ok := errors.AsType[*PermanentError](err); ok {
		return ClassPermanent, sanitizeCode(pe.Code)
	}
	if re, ok := errors.AsType[*RetryableError](err); ok {
		return ClassRetryable, sanitizeCode(re.Code)
	}
	return ClassRetryable, CodeHandlerError
}

func sanitizeCode(code string) string {
	if codePattern.MatchString(code) {
		return code
	}
	return CodeHandlerError
}

func errString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
