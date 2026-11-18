package sdk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"
)

// ProtocolVersion is the worker protocol minor revision this SDK speaks.
const ProtocolVersion = 1

// Version is reported to the coordinator as sdk_version.
const Version = "0.5.0-m5"

// Cancellation causes visible to handlers through context.Cause(ctx).
var (
	ErrLeaseLost       = errors.New("clusterx: lease lost; the result will not be committed")
	ErrCancelRequested = errors.New("clusterx: cancellation requested by an operator")
	ErrShutdown        = errors.New("clusterx: worker shutting down")
)

// WorkerOptions configure a [Worker].
type WorkerOptions struct {
	Name       string        // 1..100 chars
	Capacity   int           // 1..256 concurrent attempts
	Hostname   string        // informational
	WorkerID   string        // optional: re-register this worker id (takes over its credential binding)
	DrainGrace time.Duration // on shutdown: time running handlers get before their contexts are canceled
	ClaimWait  time.Duration // long-poll per Claim (default 5 s, max 20 s)
	Logger     *slog.Logger
	// OnRegistered, when set, is called with the server-assigned worker id after each successful
	// registration, e.g. to persist it so that a fast restart re-registers the same worker immediately
	// instead of waiting for the previous incarnation's liveness to lapse (CR-M5-01 #4).
	OnRegistered func(workerID string)
}

// Worker runs the claim / renew / report loop against the coordinator over gRPC. Delivery is
// at-least-once: a lost lease cancels the handler's context, but the handler may still have caused
// side effects, and the attempt can be executed again elsewhere.
type Worker struct {
	c   workerv1.WorkerServiceClient
	reg *Registry
	o   WorkerOptions
	log *slog.Logger

	id         *workerv1.WorkerIdentity
	renewAfter time.Duration
	heartbeat  time.Duration
}

// NewWorker binds a registry to a gRPC connection. The connection must carry the worker credential.
func NewWorker(conn grpc.ClientConnInterface, reg *Registry, o WorkerOptions) *Worker {
	if o.ClaimWait <= 0 {
		o.ClaimWait = 5 * time.Second
	}
	if o.ClaimWait > 20*time.Second {
		o.ClaimWait = 20 * time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return &Worker{c: workerv1.NewWorkerServiceClient(conn), reg: reg, o: o, log: o.Logger}
}

// WorkerID returns the server-assigned id after registration ("" before).
func (w *Worker) WorkerID() string {
	if w.id == nil {
		return ""
	}
	return w.id.WorkerId
}

func reasonOf(err error) string {
	for _, d := range status.Convert(err).Details() {
		if ei, ok := d.(*errdetails.ErrorInfo); ok {
			return ei.Reason
		}
	}
	return ""
}

// transient reports errors worth retrying with the same request.
func transient(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted:
		return true
	}
	return false
}

func newIncarnation() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (w *Worker) register(ctx context.Context) error {
	var caps []*workerv1.HandlerCapability
	for _, k := range w.reg.Keys() {
		caps = append(caps, &workerv1.HandlerCapability{Name: k.Name, Version: int32(k.Version)})
	}
	if len(caps) == 0 {
		return errors.New("clusterx: no handlers registered")
	}
	inc := newIncarnation()
	backoff := 200 * time.Millisecond
	for {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		r, err := w.c.Register(cctx, &workerv1.RegisterRequest{WorkerId: w.o.WorkerID, IncarnationId: inc, Name: w.o.Name,
			ProtocolVersion: ProtocolVersion, SdkVersion: Version, Hostname: w.o.Hostname, Capacity: int32(w.o.Capacity), Capabilities: caps})
		cancel()
		if err == nil {
			w.id = &workerv1.WorkerIdentity{WorkerId: r.WorkerId, IncarnationId: inc}
			w.renewAfter = r.GetRenewAfter().AsDuration()
			if lt := r.GetLeaseTtl().AsDuration() / 2; w.renewAfter <= 0 || w.renewAfter > lt {
				w.renewAfter = lt
			}
			w.heartbeat = r.GetHeartbeatInterval().AsDuration()
			if w.heartbeat <= 0 {
				w.heartbeat = 10 * time.Second
			}
			w.log.Info("registered", "worker_id", r.WorkerId, "incarnation_id", inc, "renew_after", w.renewAfter.String())
			w.o.WorkerID = r.WorkerId
			if w.o.OnRegistered != nil {
				w.o.OnRegistered(r.WorkerId)
			}
			return nil
		}
		// A persisted worker id bound to another credential (e.g. after credential rotation): register
		// as a new worker instead of failing forever.
		if reasonOf(err) == "credential_mismatch" && w.o.WorkerID != "" {
			w.log.Warn("stored worker id is not bound to this credential; registering a new worker")
			w.o.WorkerID = ""
			continue
		}
		// A live previous incarnation still holds the credential: wait for its liveness to lapse.
		if !transient(err) && reasonOf(err) != "incarnation_mismatch" {
			return fmt.Errorf("clusterx: register: %w", err)
		}
		w.log.Warn("register retry", "code", status.Code(err).String(), "reason", reasonOf(err))
		if !sleepCtx(ctx, backoff) {
			return ctx.Err()
		}
		backoff = min(backoff*2, 5*time.Second)
	}
}

// Run registers, then claims and executes jobs until ctx is canceled; it then drains: no new claims,
// running handlers get DrainGrace to finish, after which their contexts are canceled (cause
// ErrShutdown) and their failures reported, then the worker deregisters. Run returns nil after a
// clean shutdown and an error when the coordinator rejected the worker (revoked credential,
// superseded incarnation, deregistration).
func (w *Worker) Run(ctx context.Context) error {
	if err := w.register(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	hardCtx, hardCancel := context.WithCancelCause(context.WithoutCancel(ctx))
	defer hardCancel(ErrShutdown)
	loopCtx, stopLoops := context.WithCancelCause(ctx)
	defer stopLoops(nil)

	var fatal error
	var fatalOnce sync.Once
	setFatal := func(err error) {
		fatalOnce.Do(func() {
			fatal = err
			stopLoops(err)
			hardCancel(ErrLeaseLost) // identity is gone: every lease is unauthoritative
		})
	}

	var hb sync.WaitGroup
	hb.Go(func() { w.heartbeatLoop(loopCtx, setFatal) })

	var running sync.WaitGroup
	slots := make(chan struct{}, w.o.Capacity)
	backoff := 100 * time.Millisecond
claim:
	for loopCtx.Err() == nil {
		free := cap(slots) - len(slots)
		if free == 0 {
			select { // wait for a slot
			case slots <- struct{}{}:
				<-slots
			case <-loopCtx.Done():
				break claim
			}
			continue
		}
		cctx, cancel := context.WithTimeout(loopCtx, w.o.ClaimWait+10*time.Second)
		r, err := w.c.Claim(cctx, &workerv1.ClaimRequest{Identity: w.id, MaxJobs: int32(free), Wait: durationpb.New(w.o.ClaimWait)})
		cancel()
		if err != nil {
			if loopCtx.Err() != nil {
				break
			}
			switch reason := reasonOf(err); {
			case reason == "worker_draining":
				w.log.Info("coordinator is draining this worker; no new claims")
				<-loopCtx.Done()
				break claim
			case transient(err):
				w.log.Warn("claim failed; retrying", "code", status.Code(err).String())
				sleepCtx(loopCtx, backoff)
				backoff = min(backoff*2, 5*time.Second)
				continue
			default:
				setFatal(fmt.Errorf("clusterx: claim: %w", err))
				break claim
			}
		}
		backoff = 100 * time.Millisecond
		for _, a := range r.GetAttempts() {
			slots <- struct{}{}
			running.Go(func() {
				defer func() { <-slots }()
				w.execute(hardCtx, a)
			})
		}
	}

	if fatal == nil {
		w.shutdown(&running, hardCancel)
	} else {
		running.Wait()
	}
	stopLoops(nil)
	hb.Wait()
	return fatal
}

func (w *Worker) shutdown(running *sync.WaitGroup, hardCancel context.CancelCauseFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, err := w.c.Drain(ctx, &workerv1.DrainRequest{Identity: w.id, Grace: durationpb.New(w.o.DrainGrace)}); err != nil {
		w.log.Warn("drain request failed", "code", status.Code(err).String())
	}
	cancel()
	done := make(chan struct{})
	go func() { running.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(w.o.DrainGrace):
		w.log.Warn("drain grace elapsed; canceling running handlers")
		hardCancel(ErrShutdown)
		<-done
	}
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if r, err := w.c.Deregister(ctx, &workerv1.DeregisterRequest{Identity: w.id}); err != nil {
		w.log.Warn("deregister failed", "code", status.Code(err).String())
	} else {
		w.log.Info("deregistered", "active_attempts_left", r.GetActiveAttemptsLeft())
	}
}

func (w *Worker) heartbeatLoop(ctx context.Context, setFatal func(error)) {
	t := time.NewTicker(w.heartbeat)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := w.c.Heartbeat(cctx, &workerv1.HeartbeatRequest{Identity: w.id})
		cancel()
		if err != nil && !transient(err) && ctx.Err() == nil {
			setFatal(fmt.Errorf("clusterx: heartbeat: %w", err))
			return
		}
	}
}

// execute runs one attempt: renews at renewAfter, cancels the handler when the lease is lost or a
// cancel is requested, and reports exactly one outcome (with retries on transient errors while the
// lease may still be valid by the worker's own monotonic clock).
func (w *Worker) execute(parent context.Context, a *workerv1.ClaimedAttempt) {
	log := w.log.With("job_id", a.JobId, "attempt_id", a.AttemptId, "attempt", a.AttemptNumber)
	ref := &workerv1.AttemptRef{Identity: w.id, JobId: a.JobId, AttemptId: a.AttemptId, LeaseGeneration: a.LeaseGeneration}
	var mu sync.Mutex
	leaseUntil := time.Now().Add(a.GetLeaseTtl().AsDuration()) // monotonic
	validUntil := func() time.Time { mu.Lock(); defer mu.Unlock(); return leaseUntil }

	hctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	if rem := a.GetTimeRemaining().AsDuration(); rem > 0 {
		var c2 context.CancelFunc
		hctx, c2 = context.WithTimeout(hctx, rem)
		defer c2()
	}
	renewDone := make(chan struct{})
	stopRenew := make(chan struct{})
	go func() {
		defer close(renewDone)
		every := w.renewAfter
		for {
			select {
			case <-stopRenew:
				return
			case <-time.After(every):
			}
			cctx, c := context.WithTimeout(context.Background(), 5*time.Second)
			r, err := w.c.RenewLease(cctx, &workerv1.RenewLeaseRequest{Attempt: ref})
			c()
			switch {
			case err == nil:
				mu.Lock()
				leaseUntil = time.Now().Add(r.GetLeaseTtl().AsDuration())
				mu.Unlock()
				every = w.renewAfter
				if r.CancelRequested {
					cancel(ErrCancelRequested)
				}
			case transient(err):
				every = time.Second
				if time.Now().After(validUntil()) {
					cancel(ErrLeaseLost)
					return
				}
			default:
				log.Info("lease lost", "reason", reasonOf(err))
				cancel(ErrLeaseLost)
				return
			}
		}
	}()

	jlog, tail := newAttemptLogger(log)
	job := Job{ID: a.JobId, AttemptID: a.AttemptId, Attempt: int(a.AttemptNumber), MaxAttempts: int(a.MaxAttempts),
		Handler: HandlerKey{Name: a.GetHandler().GetName(), Version: int(a.GetHandler().GetVersion())}, Log: jlog}
	out, herr := w.reg.Invoke(hctx, job, a.PayloadJson)
	close(stopRenew)
	<-renewDone
	cause := context.Cause(hctx)
	if errors.Is(cause, ErrLeaseLost) {
		log.Info("discarding outcome: lease lost")
		return
	}

	report := func(call func(context.Context) error) {
		backoff := 100 * time.Millisecond
		for {
			cctx, c := context.WithTimeout(context.Background(), 10*time.Second)
			err := call(cctx)
			c()
			if err == nil {
				return
			}
			if !transient(err) || time.Now().After(validUntil()) {
				log.Info("report not accepted", "code", status.Code(err).String(), "reason", reasonOf(err))
				return
			}
			time.Sleep(backoff)
			backoff = min(backoff*2, 2*time.Second)
		}
	}
	if herr == nil {
		report(func(c context.Context) error {
			r, err := w.c.ReportResult(c, &workerv1.ReportResultRequest{Attempt: ref, ResultJson: out, LogTail: tail.String()})
			if err == nil {
				log.Debug("reported", "outcome", r.Outcome.String(), "already_final", r.AlreadyFinal)
			}
			return err
		})
		return
	}
	class, code := Classify(herr)
	canceled := errors.Is(cause, ErrCancelRequested)
	switch {
	case errors.Is(herr, ErrUnknownHandler):
		class, code = ClassPermanent, "unknown_handler"
	case canceled:
		code = "canceled"
	case errors.Is(cause, ErrShutdown):
		class, code = ClassRetryable, "worker_shutdown"
	case errors.Is(hctx.Err(), context.DeadlineExceeded):
		class, code = ClassRetryable, "deadline_exceeded"
	}
	ec := workerv1.ErrorClass_ERROR_CLASS_RETRYABLE
	if class == ClassPermanent {
		ec = workerv1.ErrorClass_ERROR_CLASS_PERMANENT
	}
	msg := herr.Error()
	report(func(c context.Context) error {
		r, err := w.c.ReportFailure(c, &workerv1.ReportFailureRequest{Attempt: ref, ErrorClass: ec, ErrorCode: code,
			ErrorMessage: msg, LogTail: tail.String(), CanceledByHandler: canceled})
		if err == nil {
			log.Debug("reported failure", "outcome", r.Outcome.String(), "code", code)
		}
		return err
	})
}
