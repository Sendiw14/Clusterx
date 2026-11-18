// Package sdk is the CLUSTERX Go worker SDK.
//
// A worker process registers trusted, precompiled handlers by name and version in a
// [Registry]. The coordinator only offers a job to a worker that advertised the job's exact
// handler name and version, and the SDK decodes the job payload into the handler's typed
// payload before calling it.
//
// Delivery is at-least-once. A handler can run more than once for the same job: after a
// crash, after a lost lease, or after a retryable failure. Handlers must therefore be
// idempotent with respect to their external side effects, for example by keying writes on
// [Job.ID]. Payloads and results are not a place for secrets.
//
// Handlers signal how a failure should be treated by returning a [RetryableError] (the job
// is retried while attempts remain) or a [PermanentError] (the job fails now). Any other
// non-nil error is treated as retryable. Handlers must stop promptly when ctx is canceled:
// cancellation means the lease was lost, the job was canceled or the worker is shutting down.
//
// This package does not contain the gRPC transport yet (TASK-M3-BE-01 skeleton).
package sdk
