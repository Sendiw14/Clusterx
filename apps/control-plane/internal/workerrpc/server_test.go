package workerrpc_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/workerrpc"
	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"
)

var bg = context.Background()

func reason(err error) (codes.Code, string) {
	st := status.Convert(err)
	for _, d := range st.Details() {
		if ei, ok := d.(*errdetails.ErrorInfo); ok {
			return st.Code(), ei.Reason
		}
	}
	return st.Code(), ""
}

func wantErr(t *testing.T, err error, c codes.Code, r string) {
	t.Helper()
	if gc, gr := reason(err); gc != c || gr != r {
		t.Fatalf("err = %v; want %s/%s", err, c, r)
	}
}

type client struct {
	workerv1.WorkerServiceClient
	id *workerv1.WorkerIdentity
}

func register(t *testing.T, env *testenv.Env, addr string, capacity int32, caps ...string) client {
	t.Helper()
	cred, err := env.Store.CreateWorkerCredential(bg, "w", 0)
	if err != nil {
		t.Fatal(err)
	}
	c := workerv1.NewWorkerServiceClient(testenv.Dial(t, addr, cred.Secret))
	var hc []*workerv1.HandlerCapability
	for _, n := range caps {
		hc = append(hc, &workerv1.HandlerCapability{Name: n, Version: 1})
	}
	inc := store.NewUUID()
	r, err := c.Register(bg, &workerv1.RegisterRequest{IncarnationId: inc, Name: "w", ProtocolVersion: 1, Capacity: capacity, Capabilities: hc})
	if err != nil {
		t.Fatal(err)
	}
	if r.GetLeaseTtl().AsDuration() <= 0 || r.GetRenewAfter().AsDuration() != r.GetLeaseTtl().AsDuration()/2 {
		t.Fatalf("register reply %v", r)
	}
	return client{c, &workerv1.WorkerIdentity{WorkerId: r.GetWorkerId(), IncarnationId: inc}}
}

func admit(t *testing.T, env *testenv.Env, handler, payload string) string {
	t.Helper()
	j, _, err := env.Store.Admit(bg, store.AdmitParams{Principal: "token:t", Namespace: "default", HandlerName: handler,
		HandlerVersion: 1, MaxAttempts: 3, TimeoutSeconds: 60, Payload: []byte(payload)})
	if err != nil {
		t.Fatal(err)
	}
	return j.ID
}

func TestAuthAndLimits(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 10)
	addr := env.ServeGRPC(t)
	for name, cred := range map[string]string{"none": "", "garbage": "nope", "unknown": "cxw_00000000_" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		c := workerv1.NewWorkerServiceClient(testenv.Dial(t, addr, cred))
		_, err := c.Heartbeat(bg, &workerv1.HeartbeatRequest{})
		wantErr(t, err, codes.Unauthenticated, "credential_revoked")
		_ = name
	}
	w := register(t, env, addr, 4, "sleep_ms")
	big := &workerv1.ReportResultRequest{ResultJson: make([]byte, workerrpc.MaxMessageBytes+1)}
	if _, err := w.ReportResult(bg, big); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized: want ResourceExhausted, got %v", err)
	}
	_, err := w.Register(bg, &workerv1.RegisterRequest{IncarnationId: store.NewUUID(), Name: "w", ProtocolVersion: 2, Capacity: 1,
		Capabilities: []*workerv1.HandlerCapability{{Name: "sleep_ms", Version: 1}}})
	wantErr(t, err, codes.FailedPrecondition, "protocol_unsupported")
	_, err = w.Register(bg, &workerv1.RegisterRequest{WorkerId: w.id.WorkerId, IncarnationId: store.NewUUID(), Name: "w", ProtocolVersion: 1,
		Capacity: 1, Capabilities: []*workerv1.HandlerCapability{{Name: "rm_rf", Version: 1}}})
	wantErr(t, err, codes.InvalidArgument, "unknown_handler")
	_, err = w.Claim(bg, &workerv1.ClaimRequest{Identity: w.id, MaxJobs: 0})
	wantErr(t, err, codes.InvalidArgument, "invalid_argument")
	// Another worker's identity with this credential.
	other := register(t, env, addr, 1, "sleep_ms")
	_, err = w.Heartbeat(bg, &workerv1.HeartbeatRequest{Identity: other.id})
	wantErr(t, err, codes.Unauthenticated, "credential_revoked")
}

func TestClaimRenewReportFlow(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(10*time.Second), 10)
	addr := env.ServeGRPC(t)
	w := register(t, env, addr, 4, "hash_sha256", "fail_n_times")
	// Long-poll: a job admitted during the wait is returned.
	var claim *workerv1.ClaimResponse
	var cerr error
	var wg sync.WaitGroup
	wg.Go(func() {
		claim, cerr = w.Claim(bg, &workerv1.ClaimRequest{Identity: w.id, MaxJobs: 2, Wait: durationpb.New(5 * time.Second)})
	})
	time.Sleep(200 * time.Millisecond)
	id := admit(t, env, "hash_sha256", `{"data":"abc"}`)
	wg.Wait()
	if cerr != nil || len(claim.GetAttempts()) != 1 || claim.Attempts[0].GetJobId() != id || string(claim.Attempts[0].GetPayloadJson()) != `{"data": "abc"}` {
		t.Fatalf("claim: %v %v", claim, cerr)
	}
	a := claim.Attempts[0]
	if a.GetLeaseTtl().AsDuration() <= 9*time.Second || a.GetTimeRemaining().AsDuration() <= 59*time.Second {
		t.Fatalf("lease fields: %v", a)
	}
	ref := &workerv1.AttemptRef{Identity: w.id, JobId: a.JobId, AttemptId: a.AttemptId, LeaseGeneration: a.LeaseGeneration}
	rl, err := w.RenewLease(bg, &workerv1.RenewLeaseRequest{Attempt: ref})
	if err != nil || rl.GetCancelRequested() {
		t.Fatalf("renew: %v %v", rl, err)
	}
	pc, err := w.PollCancellation(bg, &workerv1.PollCancellationRequest{Attempt: ref})
	if err != nil || pc.GetCancelRequested() || pc.GetJobTerminal() {
		t.Fatalf("poll: %v %v", pc, err)
	}
	_, err = w.ReportResult(bg, &workerv1.ReportResultRequest{Attempt: ref, ResultJson: []byte(`{bad`)})
	wantErr(t, err, codes.InvalidArgument, "invalid_argument")
	res := []byte(`{"sha256":"x"}`)
	rr, err := w.ReportResult(bg, &workerv1.ReportResultRequest{Attempt: ref, ResultJson: res})
	if err != nil || rr.GetOutcome() != workerv1.Outcome_OUTCOME_SUCCEEDED || rr.GetAlreadyFinal() {
		t.Fatalf("report: %v %v", rr, err)
	}
	rr, err = w.ReportResult(bg, &workerv1.ReportResultRequest{Attempt: ref, ResultJson: res})
	if err != nil || !rr.GetAlreadyFinal() {
		t.Fatalf("retry: %v %v", rr, err)
	}
	_, err = w.ReportResult(bg, &workerv1.ReportResultRequest{Attempt: ref, ResultJson: []byte(`{"sha256":"y"}`)})
	wantErr(t, err, codes.FailedPrecondition, "result_mismatch")
	_, err = w.RenewLease(bg, &workerv1.RenewLeaseRequest{Attempt: ref})
	wantErr(t, err, codes.FailedPrecondition, "job_terminal")

	// Failure path: retryable => RETRY_SCHEDULED with next_available_at.
	admit(t, env, "fail_n_times", `{"n":1}`)
	claim, err = w.Claim(bg, &workerv1.ClaimRequest{Identity: w.id, MaxJobs: 1})
	if err != nil || len(claim.Attempts) != 1 {
		t.Fatalf("claim 2: %v %v", claim, err)
	}
	a = claim.Attempts[0]
	ref = &workerv1.AttemptRef{Identity: w.id, JobId: a.JobId, AttemptId: a.AttemptId, LeaseGeneration: a.LeaseGeneration}
	_, err = w.ReportFailure(bg, &workerv1.ReportFailureRequest{Attempt: ref})
	wantErr(t, err, codes.InvalidArgument, "invalid_argument")
	fr, err := w.ReportFailure(bg, &workerv1.ReportFailureRequest{Attempt: ref, ErrorClass: workerv1.ErrorClass_ERROR_CLASS_RETRYABLE,
		ErrorCode: "planned_failure", ErrorMessage: "x"})
	if err != nil || fr.GetOutcome() != workerv1.Outcome_OUTCOME_RETRY_SCHEDULED || fr.GetNextAvailableAt() == nil {
		t.Fatalf("failure: %v %v", fr, err)
	}

	// Drain: claims denied; heartbeat reports DRAINING; deregister reports 0 left.
	dr, err := w.Drain(bg, &workerv1.DrainRequest{Identity: w.id, Grace: durationpb.New(time.Minute)})
	if err != nil || dr.GetDrainDeadline() == nil {
		t.Fatalf("drain: %v %v", dr, err)
	}
	_, err = w.Claim(bg, &workerv1.ClaimRequest{Identity: w.id, MaxJobs: 1})
	wantErr(t, err, codes.FailedPrecondition, "worker_draining")
	hb, err := w.Heartbeat(bg, &workerv1.HeartbeatRequest{Identity: w.id})
	if err != nil || hb.GetStatus() != workerv1.WorkerStatus_WORKER_STATUS_DRAINING {
		t.Fatalf("heartbeat: %v %v", hb, err)
	}
	de, err := w.Deregister(bg, &workerv1.DeregisterRequest{Identity: w.id})
	if err != nil || de.GetActiveAttemptsLeft() != 0 {
		t.Fatalf("deregister: %v %v", de, err)
	}
	_, err = w.Heartbeat(bg, &workerv1.HeartbeatRequest{Identity: w.id})
	wantErr(t, err, codes.FailedPrecondition, "worker_deregistered")
}

// AC-02 (service level): >= 20 concurrent gRPC claimants never hold more than one current lease per job.
func TestAC02_TwentyFourGRPCClaimants(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(30*time.Second), 60)
	addr := env.ServeGRPC(t)
	const claimants, jobs = 24, 200
	ws := make([]client, claimants)
	for i := range ws {
		ws[i] = register(t, env, addr, 16, "sleep_ms")
	}
	for range jobs {
		admit(t, env, "sleep_ms", `{"ms":0}`)
	}
	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for _, w := range ws {
		wg.Go(func() {
			for range 20 {
				r, err := w.Claim(bg, &workerv1.ClaimRequest{Identity: w.id, MaxJobs: 4})
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				for _, a := range r.Attempts {
					seen[a.JobId]++
				}
				mu.Unlock()
				for _, a := range r.Attempts {
					ref := &workerv1.AttemptRef{Identity: w.id, JobId: a.JobId, AttemptId: a.AttemptId, LeaseGeneration: a.LeaseGeneration}
					if _, err := w.ReportResult(bg, &workerv1.ReportResultRequest{Attempt: ref, ResultJson: []byte(`{}`)}); err != nil {
						t.Error(err)
					}
				}
			}
		})
	}
	wg.Wait()
	if len(seen) != jobs {
		t.Fatalf("claimed %d distinct jobs, want %d", len(seen), jobs)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("job %s leased %d times", id, n)
		}
	}
	var bad int
	if err := env.Store.Pool.QueryRow(bg, `SELECT count(*) FROM jobs WHERE status <> 'succeeded' OR attempts_used <> 1`).Scan(&bad); err != nil || bad != 0 {
		t.Fatalf("non-succeeded or multi-attempt jobs: %d %v", bad, err)
	}
}
