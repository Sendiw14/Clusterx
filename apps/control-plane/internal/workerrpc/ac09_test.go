package workerrpc_test

import (
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/testenv"
	workerv1 "github.com/Sendiw14/Clusterx/packages/proto/gen/go/clusterx/worker/v1"
)

// AC-09 over gRPC (M5 additions): a superseded process incarnation, a revoked credential and another
// credential's worker id cannot heartbeat, claim, renew or report.
func TestAC09_GRPCIdentityMatrix(t *testing.T) {
	env := testenv.New(t, testenv.FastTiming(10*time.Second), 10)
	addr := env.ServeGRPC(t)

	// Incarnation superseded by a re-register of the same worker id (fast restart, CR-M5-01 #4).
	old := register(t, env, addr, 2, "hash_sha256")
	admit(t, env, "hash_sha256", `{"data":"a"}`)
	ref := claimRef(t, old)
	newInc := store.NewUUID()
	if _, err := old.Register(bg, &workerv1.RegisterRequest{WorkerId: old.id.WorkerId, IncarnationId: newInc, Name: "w", ProtocolVersion: 1,
		Capacity: 2, Capabilities: []*workerv1.HandlerCapability{{Name: "hash_sha256", Version: 1}}}); err != nil {
		t.Fatal(err)
	}
	_, err := old.Heartbeat(bg, &workerv1.HeartbeatRequest{Identity: old.id})
	wantErr(t, err, codes.FailedPrecondition, "incarnation_mismatch")
	_, err = old.Claim(bg, &workerv1.ClaimRequest{Identity: old.id, MaxJobs: 1})
	wantErr(t, err, codes.FailedPrecondition, "incarnation_mismatch")
	_, err = old.RenewLease(bg, &workerv1.RenewLeaseRequest{Attempt: ref})
	wantErr(t, err, codes.FailedPrecondition, "incarnation_mismatch")
	_, err = old.ReportResult(bg, &workerv1.ReportResultRequest{Attempt: ref, ResultJson: []byte(`{"sha256":"x"}`)})
	wantErr(t, err, codes.FailedPrecondition, "incarnation_mismatch")

	// Revoked credential (operator revocation) mid-attempt: every call is denied.
	cred, err := env.Store.CreateWorkerCredential(bg, "r", 0)
	if err != nil {
		t.Fatal(err)
	}
	rc := workerv1.NewWorkerServiceClient(testenv.Dial(t, addr, cred.Secret))
	inc := store.NewUUID()
	reg, err := rc.Register(bg, &workerv1.RegisterRequest{IncarnationId: inc, Name: "r", ProtocolVersion: 1, Capacity: 1,
		Capabilities: []*workerv1.HandlerCapability{{Name: "hash_sha256", Version: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	rw := client{rc, &workerv1.WorkerIdentity{WorkerId: reg.WorkerId, IncarnationId: inc}}
	admit(t, env, "hash_sha256", `{"data":"b"}`)
	rref := claimRef(t, rw)
	credID, _ := env.Store.AuthenticateWorker(bg, cred.Secret)
	if _, err := env.Store.RevokeWorkerCredentialBy(bg, credID, mustUser(t, env), store.AuditMeta{}); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"heartbeat": func() error { _, e := rw.Heartbeat(bg, &workerv1.HeartbeatRequest{Identity: rw.id}); return e },
		"claim":     func() error { _, e := rw.Claim(bg, &workerv1.ClaimRequest{Identity: rw.id, MaxJobs: 1}); return e },
		"renew":     func() error { _, e := rw.RenewLease(bg, &workerv1.RenewLeaseRequest{Attempt: rref}); return e },
		"report": func() error {
			_, e := rw.ReportResult(bg, &workerv1.ReportResultRequest{Attempt: rref, ResultJson: []byte(`{"sha256":"y"}`)})
			return e
		},
	} {
		if c, r := reason(call()); c != codes.Unauthenticated || r != "credential_revoked" {
			t.Fatalf("%s with revoked credential: %s/%s", name, c, r)
		}
	}

	// A valid credential naming another credential's worker: credential_mismatch, never served.
	other := register(t, env, addr, 1, "hash_sha256")
	_, err = other.Claim(bg, &workerv1.ClaimRequest{Identity: &workerv1.WorkerIdentity{WorkerId: old.id.WorkerId, IncarnationId: newInc}, MaxJobs: 1})
	wantErr(t, err, codes.PermissionDenied, "credential_mismatch")
	_, err = other.Register(bg, &workerv1.RegisterRequest{WorkerId: old.id.WorkerId, IncarnationId: store.NewUUID(), Name: "x", ProtocolVersion: 1,
		Capacity: 1, Capabilities: []*workerv1.HandlerCapability{{Name: "hash_sha256", Version: 1}}})
	wantErr(t, err, codes.PermissionDenied, "credential_mismatch")
}

func mustUser(t *testing.T, env *testenv.Env) string {
	t.Helper()
	id, err := env.Store.UpsertUser(bg, 4242, "ops", "operator")
	if err != nil {
		t.Fatal(err)
	}
	return id
}
