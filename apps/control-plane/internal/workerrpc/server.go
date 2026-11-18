// Package workerrpc serves the worker gRPC protocol (clusterx.worker.v1). Every call is authenticated
// with a `cxw_` bearer credential; attempt-scoped calls are additionally fenced inside the store
// transaction (state-machine.md, ADR-0003). Errors carry google.rpc.ErrorInfo{domain:"clusterx.dev"}.
package workerrpc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"
)

// MaxMessageBytes is the gRPC message limit in each direction (ADR-0004).
const MaxMessageBytes = 256 << 10

// Protocol versions accepted from Register.protocol_version.
const (
	ProtocolCurrent      = 1
	ProtocolMinSupported = 1
)

// Domain of every ErrorInfo.
const Domain = "clusterx.dev"

// Server-side bounds per RPC (proto header; the client may set shorter deadlines).
var rpcTimeout = map[string]time.Duration{
	"Register": 10 * time.Second, "Heartbeat": 5 * time.Second, "Claim": 30 * time.Second,
	"RenewLease": 5 * time.Second, "ReportResult": 10 * time.Second, "ReportFailure": 10 * time.Second,
	"PollCancellation": 5 * time.Second, "Drain": 5 * time.Second, "Deregister": 5 * time.Second,
}

// MaxClaimWait bounds the Claim long-poll.
const MaxClaimWait = 20 * time.Second

type credKey struct{}

type service struct {
	workerv1.UnimplementedWorkerServiceServer
	st   *store.Store
	log  *slog.Logger
	poll time.Duration
}

// Options configure the server.
type Options struct {
	Creds     credentials.TransportCredentials // nil only for loopback (config enforces in production)
	ClaimPoll time.Duration                    // long-poll re-check interval; default 250 ms
}

// NewServer returns a gRPC server with the worker service registered.
func NewServer(st *store.Store, log *slog.Logger, o Options) *grpc.Server {
	svc := &service{st: st, log: log, poll: o.ClaimPoll}
	if svc.poll <= 0 {
		svc.poll = 250 * time.Millisecond
	}
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(MaxMessageBytes),
		grpc.MaxSendMsgSize(MaxMessageBytes),
		grpc.ChainUnaryInterceptor(svc.intercept),
	}
	if o.Creds != nil {
		opts = append(opts, grpc.Creds(o.Creds))
	}
	s := grpc.NewServer(opts...)
	workerv1.RegisterWorkerServiceServer(s, svc)
	return s
}

// intercept bounds the call, authenticates the credential (no cache) and logs without secrets.
func (s *service) intercept(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
	start := time.Now()
	method := info.FullMethod[strings.LastIndex(info.FullMethod, "/")+1:]
	if d, ok := rpcTimeout[method]; ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	var resp any
	credID, err := s.authenticate(ctx)
	if err == nil {
		resp, err = h(context.WithValue(ctx, credKey{}, credID), req)
	} else {
		err = s.toStatus(err)
	}
	code := status.Code(err)
	lvl := slog.LevelDebug
	if code != codes.OK {
		lvl = slog.LevelInfo
	}
	s.log.Log(ctx, lvl, "worker rpc", "method", method, "code", code.String(), "reason", reasonOf(err),
		"duration_ms", float64(time.Since(start).Microseconds())/1000)
	return resp, err
}

// authenticate resolves the bearer credential on every call (no cache).
func (s *service) authenticate(ctx context.Context) (string, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get("authorization")
	if len(vals) != 1 {
		return "", &store.Denial{Reason: "credential_revoked", Detail: "missing credential"}
	}
	secret, ok := strings.CutPrefix(vals[0], "Bearer ")
	if !ok {
		return "", &store.Denial{Reason: "credential_revoked", Detail: "missing credential"}
	}
	return s.st.AuthenticateWorker(ctx, secret)
}

func credOf(ctx context.Context) string {
	id, _ := ctx.Value(credKey{}).(string)
	return id
}

func reasonOf(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return ""
	}
	for _, d := range st.Details() {
		if ei, ok := d.(*errdetails.ErrorInfo); ok {
			return ei.Reason
		}
	}
	return ""
}

func codeFor(reason string) codes.Code {
	switch reason {
	case "credential_revoked":
		return codes.Unauthenticated
	case "credential_mismatch":
		return codes.PermissionDenied
	case "unknown_handler", "result_too_large", "invalid_argument":
		return codes.InvalidArgument
	case "rate_limited":
		return codes.ResourceExhausted
	case "unavailable":
		return codes.Unavailable
	}
	return codes.FailedPrecondition
}

func withReason(c codes.Code, reason, msg string) error {
	st := status.New(c, msg)
	if d, err := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: Domain}); err == nil {
		st = d
	}
	return st.Err()
}

// toStatus maps store errors to gRPC status. Internal details are logged, never returned.
func (s *service) toStatus(err error) error {
	if err == nil {
		return nil
	}
	if d, ok := errors.AsType[*store.Denial](err); ok {
		msg := d.Reason
		if d.Detail != "" {
			msg += ": " + d.Detail
		}
		return withReason(codeFor(d.Reason), d.Reason, msg)
	}
	if errors.Is(err, store.ErrUnavailable) {
		return withReason(codes.Unavailable, "unavailable", "database unavailable; do not assume the lease is still valid")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "canceled")
	}
	s.log.Error("worker rpc internal error", "err", err)
	return status.Error(codes.Internal, "internal error")
}

func invalid(msg string) error { return withReason(codes.InvalidArgument, "invalid_argument", msg) }

var (
	uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	nameRe = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,62}$`)
	codeRe = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
)

func (s *service) worker(ctx context.Context, id *workerv1.WorkerIdentity) (store.WorkerRef, error) {
	if id == nil || !uuidRe.MatchString(id.GetWorkerId()) || !uuidRe.MatchString(id.GetIncarnationId()) {
		return store.WorkerRef{}, invalid("identity.worker_id and identity.incarnation_id must be UUIDs")
	}
	return store.WorkerRef{WorkerID: id.GetWorkerId(), IncarnationID: id.GetIncarnationId(), CredentialID: credOf(ctx)}, nil
}

func (s *service) attempt(ctx context.Context, a *workerv1.AttemptRef) (store.AttemptRef, error) {
	if a == nil {
		return store.AttemptRef{}, invalid("attempt is required")
	}
	w, err := s.worker(ctx, a.GetIdentity())
	if err != nil {
		return store.AttemptRef{}, err
	}
	if !uuidRe.MatchString(a.GetJobId()) || !uuidRe.MatchString(a.GetAttemptId()) || a.GetLeaseGeneration() < 1 {
		return store.AttemptRef{}, invalid("job_id, attempt_id must be UUIDs and lease_generation >= 1")
	}
	return store.AttemptRef{WorkerRef: w, JobID: a.GetJobId(), AttemptID: a.GetAttemptId(), Generation: a.GetLeaseGeneration()}, nil
}

func (s *service) Register(ctx context.Context, r *workerv1.RegisterRequest) (*workerv1.RegisterResponse, error) {
	switch {
	case r.GetProtocolVersion() < ProtocolMinSupported || r.GetProtocolVersion() > ProtocolCurrent:
		return nil, withReason(codes.FailedPrecondition, "protocol_unsupported", "protocol_version must be 1")
	case !uuidRe.MatchString(r.GetIncarnationId()):
		return nil, invalid("incarnation_id must be a UUID")
	case r.GetWorkerId() != "" && !uuidRe.MatchString(r.GetWorkerId()):
		return nil, invalid("worker_id must be empty or a UUID")
	case len(r.GetName()) < 1 || len(r.GetName()) > 100:
		return nil, invalid("name must be 1..100 characters")
	case len(r.GetSdkVersion()) > 64 || len(r.GetHostname()) > 255:
		return nil, invalid("sdk_version <= 64, hostname <= 255")
	case r.GetCapacity() < 1 || r.GetCapacity() > 256:
		return nil, invalid("capacity must be 1..256")
	case len(r.GetCapabilities()) < 1 || len(r.GetCapabilities()) > 64:
		return nil, invalid("1..64 capabilities required")
	}
	caps, err := capabilities(r.GetCapabilities())
	if err != nil {
		return nil, err
	}
	reg, err := s.st.Register(ctx, credOf(ctx), store.RegisterParams{WorkerID: r.GetWorkerId(), IncarnationID: r.GetIncarnationId(),
		Name: r.GetName(), SDKVersion: r.GetSdkVersion(), Hostname: r.GetHostname(), ProtocolVersion: int(r.GetProtocolVersion()),
		Capacity: int(r.GetCapacity()), Capabilities: caps})
	if err != nil {
		return nil, s.toStatus(err)
	}
	t := s.st.Timing
	return &workerv1.RegisterResponse{WorkerId: reg.WorkerID, IncarnationId: reg.IncarnationID,
		HeartbeatInterval: durationpb.New(t.HeartbeatInterval), LeaseTtl: durationpb.New(t.LeaseTTL),
		RenewAfter: durationpb.New(t.LeaseTTL / 2), ServerProtocolVersion: ProtocolCurrent}, nil
}

func capabilities(in []*workerv1.HandlerCapability) ([]store.Capability, error) {
	out := make([]store.Capability, 0, len(in))
	for _, c := range in {
		if !nameRe.MatchString(c.GetName()) || c.GetVersion() < 1 || c.GetVersion() > 1000 {
			return nil, invalid("capability name/version malformed")
		}
		out = append(out, store.Capability{Name: c.GetName(), Version: int(c.GetVersion())})
	}
	return out, nil
}

func workerStatus(s string) workerv1.WorkerStatus {
	switch s {
	case "active":
		return workerv1.WorkerStatus_WORKER_STATUS_ACTIVE
	case "draining":
		return workerv1.WorkerStatus_WORKER_STATUS_DRAINING
	case "deregistered":
		return workerv1.WorkerStatus_WORKER_STATUS_DEREGISTERED
	}
	return workerv1.WorkerStatus_WORKER_STATUS_UNSPECIFIED
}

func (s *service) Heartbeat(ctx context.Context, r *workerv1.HeartbeatRequest) (*workerv1.HeartbeatResponse, error) {
	w, err := s.worker(ctx, r.GetIdentity())
	if err != nil {
		return nil, err
	}
	st, err := s.st.Heartbeat(ctx, w)
	if err != nil {
		return nil, s.toStatus(err)
	}
	resp := &workerv1.HeartbeatResponse{Status: workerStatus(st.Status)}
	if st.DrainDeadline != nil {
		resp.DrainDeadline = timestamppb.New(*st.DrainDeadline)
	}
	return resp, nil
}

func (s *service) Claim(ctx context.Context, r *workerv1.ClaimRequest) (*workerv1.ClaimResponse, error) {
	w, err := s.worker(ctx, r.GetIdentity())
	if err != nil {
		return nil, err
	}
	if r.GetMaxJobs() < 1 || r.GetMaxJobs() > 256 {
		return nil, invalid("max_jobs must be 1..256")
	}
	if len(r.GetSupported()) > 64 {
		return nil, invalid("at most 64 supported capabilities")
	}
	caps, err := capabilities(r.GetSupported())
	if err != nil {
		return nil, err
	}
	wait := r.GetWait().AsDuration()
	if r.GetWait() == nil || wait < 0 {
		wait = 0
	}
	if wait > MaxClaimWait {
		wait = MaxClaimWait
	}
	deadline := time.Now().Add(wait)
	for {
		got, err := s.st.Claim(ctx, w, int(r.GetMaxJobs()), caps)
		if err != nil {
			return nil, s.toStatus(err)
		}
		if len(got) > 0 || !time.Now().Add(s.poll).Before(deadline) {
			resp := &workerv1.ClaimResponse{}
			for _, a := range got {
				resp.Attempts = append(resp.Attempts, &workerv1.ClaimedAttempt{
					JobId: a.JobID, AttemptId: a.AttemptID, LeaseGeneration: a.LeaseGeneration,
					AttemptNumber: int32(a.AttemptNumber), MaxAttempts: int32(a.MaxAttempts),
					Handler:     &workerv1.HandlerCapability{Name: a.HandlerName, Version: int32(a.HandlerVersion)},
					PayloadJson: a.Payload, LeaseTtl: durationpb.New(a.LeaseExpiresAt.Sub(a.Now)),
					LeaseExpiresAt: timestamppb.New(a.LeaseExpiresAt), HardDeadlineAt: timestamppb.New(a.HardDeadlineAt),
					TimeRemaining: durationpb.New(a.HardDeadlineAt.Sub(a.Now)),
				})
			}
			return resp, nil
		}
		select {
		case <-ctx.Done():
			return &workerv1.ClaimResponse{}, nil
		case <-time.After(s.poll):
		}
	}
}

func (s *service) RenewLease(ctx context.Context, r *workerv1.RenewLeaseRequest) (*workerv1.RenewLeaseResponse, error) {
	a, err := s.attempt(ctx, r.GetAttempt())
	if err != nil {
		return nil, err
	}
	l, err := s.st.RenewLease(ctx, a)
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &workerv1.RenewLeaseResponse{LeaseTtl: durationpb.New(l.ExpiresAt.Sub(l.Now)),
		LeaseExpiresAt: timestamppb.New(l.ExpiresAt), CancelRequested: l.CancelRequested}, nil
}

func outcome(o store.Outcome) workerv1.Outcome {
	switch o {
	case store.OutcomeSucceeded:
		return workerv1.Outcome_OUTCOME_SUCCEEDED
	case store.OutcomeRetryScheduled:
		return workerv1.Outcome_OUTCOME_RETRY_SCHEDULED
	case store.OutcomeFailed:
		return workerv1.Outcome_OUTCOME_FAILED
	case store.OutcomeCanceled:
		return workerv1.Outcome_OUTCOME_CANCELED
	}
	return workerv1.Outcome_OUTCOME_UNSPECIFIED
}

func (s *service) ReportResult(ctx context.Context, r *workerv1.ReportResultRequest) (*workerv1.ReportResultResponse, error) {
	a, err := s.attempt(ctx, r.GetAttempt())
	if err != nil {
		return nil, err
	}
	res := r.GetResultJson()
	if len(res) > store.MaxResultBytes {
		return nil, withReason(codes.InvalidArgument, "result_too_large", "result exceeds 131072 bytes; report a failure instead")
	}
	if !json.Valid(res) || strings.Contains(string(res), `\u0000`) {
		return nil, invalid("result_json must be valid UTF-8 JSON without NUL")
	}
	if len(r.GetLogTail()) > MaxMessageBytes {
		return nil, invalid("log_tail too large")
	}
	rep, err := s.st.ReportSuccess(ctx, a, res, r.GetLogTail())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &workerv1.ReportResultResponse{Outcome: outcome(rep.Outcome), AlreadyFinal: rep.AlreadyFinal}, nil
}

func (s *service) ReportFailure(ctx context.Context, r *workerv1.ReportFailureRequest) (*workerv1.ReportFailureResponse, error) {
	a, err := s.attempt(ctx, r.GetAttempt())
	if err != nil {
		return nil, err
	}
	var permanent bool
	switch r.GetErrorClass() {
	case workerv1.ErrorClass_ERROR_CLASS_RETRYABLE:
	case workerv1.ErrorClass_ERROR_CLASS_PERMANENT:
		permanent = true
	default:
		return nil, invalid("error_class is required")
	}
	code := r.GetErrorCode()
	if !codeRe.MatchString(code) {
		code = "handler_error"
	}
	rep, err := s.st.ReportFailure(ctx, a, store.FailureParams{Permanent: permanent, Code: code, Message: r.GetErrorMessage(),
		LogTail: r.GetLogTail(), CanceledByHandler: r.GetCanceledByHandler()})
	if err != nil {
		return nil, s.toStatus(err)
	}
	resp := &workerv1.ReportFailureResponse{Outcome: outcome(rep.Outcome), AlreadyFinal: rep.AlreadyFinal}
	if rep.NextAvailableAt != nil {
		resp.NextAvailableAt = timestamppb.New(*rep.NextAvailableAt)
	}
	return resp, nil
}

func (s *service) PollCancellation(ctx context.Context, r *workerv1.PollCancellationRequest) (*workerv1.PollCancellationResponse, error) {
	a, err := s.attempt(ctx, r.GetAttempt())
	if err != nil {
		return nil, err
	}
	l, err := s.st.PollCancellation(ctx, a)
	if err != nil {
		return nil, s.toStatus(err)
	}
	resp := &workerv1.PollCancellationResponse{CancelRequested: l.CancelRequested, JobTerminal: l.JobTerminal}
	if !l.ExpiresAt.IsZero() {
		resp.LeaseExpiresAt = timestamppb.New(l.ExpiresAt)
	}
	return resp, nil
}

func (s *service) Drain(ctx context.Context, r *workerv1.DrainRequest) (*workerv1.DrainResponse, error) {
	w, err := s.worker(ctx, r.GetIdentity())
	if err != nil {
		return nil, err
	}
	grace := r.GetGrace().AsDuration()
	if grace < 0 || grace > time.Hour {
		return nil, invalid("grace must be 0..3600s")
	}
	d, err := s.st.Drain(ctx, w, grace)
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &workerv1.DrainResponse{DrainDeadline: timestamppb.New(d)}, nil
}

func (s *service) Deregister(ctx context.Context, r *workerv1.DeregisterRequest) (*workerv1.DeregisterResponse, error) {
	w, err := s.worker(ctx, r.GetIdentity())
	if err != nil {
		return nil, err
	}
	left, err := s.st.Deregister(ctx, w)
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &workerv1.DeregisterResponse{ActiveAttemptsLeft: int32(left)}, nil
}
