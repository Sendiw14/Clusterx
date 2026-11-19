package api_test

import (
	"context"
	"testing"

	"github.com/Sendiw14/Clusterx/apps/control-plane/internal/store"
)

// RV5-06 / N08 and N28: viewers never see log_tail; "*:read" routes need a read scope.
func TestRV5_LogTailMaskAndAnyReadScope(t *testing.T) {
	e := newM5(t)
	id := decode[map[string]any](t, e.want2(e.do("POST", "/api/v1/jobs", e.op, okJob), 201))["id"].(string)
	w := newRefWorker(t, e.env.Store)
	a := w.claim(t, id)
	if _, err := e.env.Store.ReportFailure(context.Background(), a, store.FailureParams{Permanent: true, Code: "x", Message: "m",
		LogTail: "secret log line"}); err != nil {
		t.Fatal(err)
	}
	attempt := func(tok string) map[string]any {
		at := decode[map[string]any](t, e.want2(e.do("GET", "/api/v1/jobs/"+id+"/attempts", tok, ""), 200))
		return at["items"].([]any)[0].(map[string]any)
	}
	if op := attempt(e.op); op["log_tail"] != "secret log line\n" && op["log_tail"] != "secret log line" {
		t.Fatalf("operator log_tail: %v", op["log_tail"])
	}
	v := attempt(e.view)
	masked := v["masked_fields"].([]any)
	if v["log_tail"] != nil || len(masked) != 2 || masked[1] != "log_tail" {
		t.Fatalf("viewer attempt not masked: %v", v)
	}
	e.want(e.do("GET", "/api/v1/config", e.token(77, "operator", "jobs:write"), ""), 403, "forbidden")
	e.want(e.do("GET", "/api/v1/config", e.token(78, "viewer", "metrics:read"), ""), 200, "")
}
