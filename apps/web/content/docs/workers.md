---
title: Worker SDK
lead: Register trusted Go handlers, classify failures, respect cancellation and run a worker. Every Go snippet here is copied from code in the repository.
description: The CLUSTERX Go worker SDK - handler registration and versioning, error types, timeouts, cancellation, worker credentials and local test examples.
blurb: Go SDK handlers, versions, error types, cancellation, worker credentials and local tests.
order: 3
---

Workers are Go processes that pull jobs from the control plane over gRPC and run **trusted, precompiled handlers**. A job is offered only to a worker that advertised that job's exact handler name and version. Workers have no database access.

> [!NOTE] The SDK lives in the repository at `packages/sdk-go` (Go package `sdk`, module path `github.com/Sendiw14/Clusterx/packages/sdk-go`). No module version has been published or tagged, so there is no `go get` instruction to give. The example module `examples/basic-jobs` uses a `replace` directive to the repository. Publishing the module is not yet done.

## Trust model

- **Handlers are trusted code.** They are compiled into the worker. The control plane accepts only handler names and versions in its own catalogue; there is no code upload, no shell and no URL fetching in 1.0.
- **Each worker has its own credential**, a `cxw_` secret that an operator creates. It is shown once, stored only as a SHA-256 hash, and can be revoked. A revoked credential is refused on any new claim, renewal or result.
- **Payloads are untrusted input.** Do not put secrets in a payload, a result or a log: they are stored and, for operators, displayed.

## Write a handler

A handler is a function from a typed payload to a typed result. The SDK decodes the job's JSON into your payload type, **rejecting unknown fields and trailing data**, runs your function, and encodes the result.

This is the `hash_sha256` handler from `examples/basic-jobs/basicjobs.go`:

```go
type HashIn struct {
	Data string `json:"data"`
}
type HashOut struct {
	SHA256 string `json:"sha256"`
}

// Hash is hash_sha256 v1.
func Hash(_ context.Context, _ sdk.Job, in HashIn) (HashOut, error) {
	if len(in.Data) > MaxHashBytes {
		return HashOut{}, sdk.Permanent("invalid_payload", fmt.Errorf("data exceeds %d bytes", MaxHashBytes))
	}
	sum := sha256.Sum256([]byte(in.Data))
	return HashOut{SHA256: hex.EncodeToString(sum[:])}, nil
}
```

The `sdk.Job` argument describes the attempt: `ID` (stable across attempts, use it as the idempotency key for side effects), `AttemptID`, `Attempt` (1-based), `MaxAttempts` (includes the first run), `Handler` and `Log`, the attempt's `*slog.Logger` (captured and bounded, see the note under Run a worker).

## Register handlers and versions

Register each handler under a name and a version in a `Registry`:

```go
func Register(reg *sdk.Registry) error {
	return errors.Join(
		sdk.Register(reg, "sleep_ms", 1, sdk.HandlerFunc[SleepIn, SleepOut](Sleep)),
		sdk.Register(reg, "hash_sha256", 1, sdk.HandlerFunc[HashIn, HashOut](Hash)),
		sdk.Register(reg, "fail_n_times", 1, sdk.HandlerFunc[FailIn, FailOut](FailNTimes)),
	)
}
```

| Rule | Value |
| --- | --- |
| Handler name | `^[a-z][a-z0-9_.-]{0,62}$` |
| Handler version | An integer from 1 to 1000 |
| Registering the same name and version twice | An error |
| Job payload | At most 64 KiB of JSON |
| Result | At most 128 KiB of JSON. Larger results fail the job permanently with `result_too_large`; they are never truncated. |

`Registry.Keys()` returns the registered handlers sorted by name and version. A worker advertises exactly these as its capabilities. To change a handler's payload shape, register a **new version** and keep the old one until its jobs have drained.

The control plane also needs a catalogue entry (`HandlerSpec`) for each handler, with a JSON Schema for the payload, so it can validate at submission. In the current build the catalogue is compiled in from `examples/basic-jobs`'s `Specs()`. A deployment-specific catalogue mechanism does not exist yet.

## Error types

Return an error to say how the engine should treat the failure:

| Return | Effect |
| --- | --- |
| `sdk.Retryable(code, err)` | The job is retried with backoff while attempts remain. |
| `sdk.Permanent(code, err)` | The job fails now, without using further attempts. |
| Any other non-nil error | Treated as retryable with the code `handler_error`. |
| A panic | Recovered and treated as retryable with the code `handler_panic`. |

The `code` must match `^[a-z][a-z0-9_.-]{0,63}$`; an invalid code is replaced by `handler_error`. If both kinds are wrapped in one chain, the permanent error wins. `sdk.Classify(err)` reports the class and code the SDK will use.

A payload that does not decode into your type, a result that cannot be encoded, and a result over the size limit are all treated as permanent failures.

## Timeouts, cancellation and lease loss

The context passed to your handler is canceled when any of these happens, and `context.Cause(ctx)` tells you which:

| Cause | Meaning |
| --- | --- |
| The context deadline | The job's hard deadline (`timeout_seconds`, default 600 s) has passed. |
| `sdk.ErrLeaseLost` | The lease could not be renewed. The result will not be committed. |
| `sdk.ErrCancelRequested` | An operator asked to cancel the job. |
| `sdk.ErrShutdown` | The worker is shutting down and the drain grace period ended. |

**Cancellation is cooperative.** Your handler must watch `ctx.Done()` and return promptly. The engine can stop *accepting* a result, but it cannot stop a goroutine. This is how `sleep_ms` does it:

```go
select {
case <-t.C:
	return SleepOut{SleptMS: in.MS}, nil
case <-ctx.Done():
	return SleepOut{}, sdk.Retryable("interrupted", context.Cause(ctx))
}
```

A handler may still have caused side effects when its lease was lost, and the attempt can run again elsewhere. See [Reliability](/docs/reliability).

## Run a worker

`sdk.NewWorker` binds a registry to a gRPC connection that carries the worker credential, and `Run` registers, claims, renews, reports, and on shutdown drains and deregisters:

```go
w := sdk.NewWorker(conn, &reg, sdk.WorkerOptions{Name: "worker-1", Capacity: 4})
err := w.Run(ctx)
```

`WorkerOptions` has `Name` (1 to 100 characters), `Capacity` (1 to 256 concurrent attempts), `Hostname`, `DrainGrace`, `ClaimWait` and a `Logger`. On shutdown the worker stops claiming, gives running handlers `DrainGrace` to finish, then cancels them with `sdk.ErrShutdown`, reports their failures and deregisters.

The reference worker, `apps/worker/cmd/clusterx-worker`, runs the three example handlers and is configured by environment variables:

| Variable | Purpose |
| --- | --- |
| `CLUSTERX_WORKER_CONTROL_PLANE_ADDR` | gRPC address of the control plane. |
| `CLUSTERX_WORKER_CREDENTIAL` | The worker's `cxw_` credential. |
| `CLUSTERX_WORKER_NAME` | Display name. |
| `CLUSTERX_WORKER_CAPACITY` | Concurrent attempts. |
| `CLUSTERX_WORKER_INSECURE` | `true` allows plaintext. For a private local network only. |
| `CLUSTERX_WORKER_TLS_CA_FILE`, `CLUSTERX_WORKER_TLS_SERVER_NAME` | TLS settings for a non-loopback connection. |
| `CLUSTERX_SHUTDOWN_TIMEOUT` | Drain grace on shutdown. |
| `CLUSTERX_LOG_LEVEL`, `CLUSTERX_ENV` | Logging and environment. |

> [!NOTE] **Handler logs.** Capture shipped in M5. Log through `job.Log` and the SDK keeps the last 16 KiB of that output, with secret-like attribute keys redacted, and reports it as the attempt's `log_tail`. The control plane keeps it for 7 days. Output written any other way (for example `fmt.Println`) is not captured. Never log secrets.

## Drain, restart and keeping a host out

A drain (from the console, the API, or a worker's own shutdown) stops new claims for that worker. Running attempts may finish until the drain grace ends; after that the lease path recovers them.

- **Drain is not exclusion.** A drained worker deregisters and exits. If a supervisor (`restart: always`, Kubernetes) restarts it, it joins again as a **new worker row** and takes new claims. To keep a host out, **revoke its worker credential**.
- **Every graceful restart creates a new worker row** (stop, self-drain, deregister, start). Expect the worker list to grow by one row per restart; old rows show as deregistered.
- An operator drain sticks to that worker id: if the same process re-registers while still draining, it stays draining and gets no claims until it deregisters.
- A worker can keep its id across restarts with `CLUSTERX_WORKER_ID` or `CLUSTERX_WORKER_ID_FILE` (needs a writable volume; the compose workers have a read-only filesystem, so this is not wired there).

> [!VERIFIED] Behaviour as recorded in ADR-0003 (RV5-02, RV5R-01) and covered by the control plane's backend tests (HANDOFF-M5-BE-02). It was not re-run for this page.

## Create and revoke worker credentials

The control plane's CLI creates credentials. The secret is printed once, as a line starting with `secret=`, and is never stored in plain text:

```sh
clusterx-control-plane worker-credential create --name worker-1
clusterx-control-plane worker-credential revoke --id <uuid>
```

> [!VERIFIED] Both commands were run in the M4 curl guide against built binaries. On the compose stack, `bootstrap.sh` calls `create` for you three times (see the [Quickstart](/docs/quickstart)). A credential bound to a live worker refuses a new incarnation until the old one's liveness lapses, so a fast restart of the reference worker may wait up to the liveness timeout (30 s by default).

## Test handlers locally

Handlers are plain functions, so test them without a coordinator by calling `Registry.Invoke`. The repository's own test does exactly this: it registers the three handlers, invokes `hash_sha256` and checks the digest, checks that `fail_n_times` fails retryably on attempt 2 of 2, that a `permanent` payload classifies as permanent, that `sleep_ms` returns when its context is canceled, and that an unknown field in a payload is refused.

```sh
cd examples/basic-jobs
go test -race ./...
```

> [!VERIFIED] The backend lane ran `go vet` and `go test -race` for `examples/basic-jobs`, `packages/sdk-go` and `apps/worker` with exit 0 (HANDOFF-M4-BE-01). The integrator has not re-run them for this page.

> [!UNVERIFIED] An end-to-end test of your own handler against a running control plane needs a catalogue entry in the coordinator, which today means editing the repository. That workflow is not documented because it has not been exercised by anyone outside the backend lane.
