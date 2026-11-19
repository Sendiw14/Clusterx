package workerrpc_test

import (
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/scheduler"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"
)

func claimRef(t *testing.T, w client) *workerv1.AttemptRef {
	t.Helper()
	r, err := w.Claim(bg, &workerv1.ClaimRequest{Identity: w.id, MaxJobs: 1})
	if err != nil || len(r.GetAttempts()) != 1 {
		t.Fatalf("claim: %v %v", r, err)
	}
	a := r.Attempts[0]
	return &workerv1.AttemptRef{Identity: w.id, JobId: a.JobId, AttemptId: a.AttemptId, LeaseGeneration: a.LeaseGeneration}
}

// AC-03 (service level, real gRPC + scheduler recovery): worker A's lease expires, the scheduler fences
// and requeues, worker B claims and completes; A's late success, renewal and failure are all rejected,
// A gets job_terminal (its own attempt, job already final; state-machine.md E5) — B's result is preserved, and the history is coherent.
func TestAC03_StaleSuccessAfterExpiryAndReclaim(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(5*time.Second), 10)
	addr := env.ServeGRPC(t)
	a := register(t, env, addr, 1, "hash_sha256")
	id := admit(t, env, "hash_sha256", `{"data":"x"}`)
	refA := claimRef(t, a)
	// The lease runs out (DB clock fixture), and the normal scheduler tick recovers it.
	if _, err := env.Store.Pool.Exec(bg, `UPDATE jobs SET lease_expires_at = clock_timestamp() - interval '1 ms' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, r, err := scheduler.Tick(bg, env.Store); err != nil || r != 1 {
		t.Fatalf("recovery: %d %v", r, err)
	}
	if _, p, err := scheduler.Tick(bg, env.Store); err != nil || p != 0 {
		// retry_wait -> queued once the (10 ms test) backoff passed
		t.Logf("promote: %d %v", p, err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, _, err := scheduler.Tick(bg, env.Store); err != nil {
		t.Fatal(err)
	}
	b := register(t, env, addr, 1, "hash_sha256")
	refB := claimRef(t, b)
	if refB.JobId != id || refB.LeaseGeneration != refA.LeaseGeneration+1 {
		t.Fatalf("B claimed %s gen %d", refB.JobId, refB.LeaseGeneration)
	}
	if _, err := b.ReportResult(bg, &workerv1.ReportResultRequest{Attempt: refB, ResultJson: []byte(`{"sha256":"from-b"}`)}); err != nil {
		t.Fatal(err)
	}
	_, err := a.ReportResult(bg, &workerv1.ReportResultRequest{Attempt: refA, ResultJson: []byte(`{"sha256":"from-a"}`)})
	wantErr(t, err, codes.FailedPrecondition, "job_terminal")
	_, err = a.RenewLease(bg, &workerv1.RenewLeaseRequest{Attempt: refA})
	wantErr(t, err, codes.FailedPrecondition, "job_terminal")
	_, err = a.ReportFailure(bg, &workerv1.ReportFailureRequest{Attempt: refA, ErrorClass: workerv1.ErrorClass_ERROR_CLASS_PERMANENT, ErrorCode: "x"})
	wantErr(t, err, codes.FailedPrecondition, "job_terminal")
	var got string
	if err := env.Store.Pool.QueryRow(bg, `SELECT j.status || '|' || (j.result->>'sha256') || '|' ||
			(SELECT string_agg(event_type, ',' ORDER BY version) FROM job_events WHERE job_id = j.id) || '|' ||
			(SELECT string_agg(attempt_number || ':' || state, ',' ORDER BY attempt_number) FROM job_attempts WHERE job_id = j.id) || '|' ||
			j.version || '=' || (SELECT max(version) FROM job_events WHERE job_id = j.id) || '|' ||
			(SELECT count(*) FROM audit_events WHERE target_id = j.id AND outcome = 'rejected' AND details->>'reason' = 'job_terminal')
		FROM jobs j WHERE j.id = $1`, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if want := "succeeded|from-b|admitted,claimed,lease_expired,promoted,claimed,succeeded|1:lease_expired,2:succeeded|6=6|3"; got != want {
		t.Fatalf("history\n got %s\nwant %s", got, want)
	}
}
