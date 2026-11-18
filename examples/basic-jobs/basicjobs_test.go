package basicjobs

import (
	"context"
	"testing"
	"time"

	sdk "github.com/Sendiw14/Clusterx/packages/sdk-go"
)

func TestHandlers(t *testing.T) {
	var reg sdk.Registry
	if err := Register(&reg); err != nil {
		t.Fatal(err)
	}
	if len(reg.Keys()) != len(Specs()) {
		t.Fatalf("registry %v vs specs", reg.Keys())
	}
	for _, s := range Specs() {
		found := false
		for _, k := range reg.Keys() {
			found = found || k == s.Key()
		}
		if !found {
			t.Fatalf("spec %s not registered", s.Key())
		}
	}
	ctx := context.Background()
	job := func(name string, attempt int) sdk.Job {
		return sdk.Job{ID: "j", Attempt: attempt, MaxAttempts: 3, Handler: sdk.HandlerKey{Name: name, Version: 1}}
	}
	out, err := reg.Invoke(ctx, job("hash_sha256", 1), []byte(`{"data":"abc"}`))
	if err != nil || string(out) != `{"sha256":"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"}` {
		t.Fatalf("hash: %s %v", out, err)
	}
	if _, err := reg.Invoke(ctx, job("fail_n_times", 2), []byte(`{"n":2}`)); err == nil {
		t.Fatal("attempt 2 of n=2 should fail")
	} else if c, code := sdk.Classify(err); c != sdk.ClassRetryable || code != "planned_failure" {
		t.Fatalf("class %v %s", c, code)
	}
	if out, err := reg.Invoke(ctx, job("fail_n_times", 3), []byte(`{"n":2}`)); err != nil || string(out) != `{"attempt":3}` {
		t.Fatalf("attempt 3: %s %v", out, err)
	}
	if _, err := reg.Invoke(ctx, job("fail_n_times", 1), []byte(`{"n":1,"permanent":true}`)); err == nil {
		t.Fatal("permanent")
	} else if c, _ := sdk.Classify(err); c != sdk.ClassPermanent {
		t.Fatal("want permanent")
	}
	cctx, cancel := context.WithCancel(ctx)
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	start := time.Now()
	if _, err := reg.Invoke(cctx, job("sleep_ms", 1), []byte(`{"ms":60000}`)); err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("sleep not canceled: %v", err)
	}
	if _, err := reg.Invoke(ctx, job("sleep_ms", 1), []byte(`{"ms":60001}`)); err == nil {
		t.Fatal("unbounded sleep accepted")
	}
	if _, err := reg.Invoke(ctx, job("hash_sha256", 1), []byte(`{"data":"x","path":"/etc/passwd"}`)); err == nil {
		t.Fatal("unknown field accepted")
	}
}
