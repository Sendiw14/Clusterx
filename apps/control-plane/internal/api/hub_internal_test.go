package api

import "testing"

// A subscriber whose writer does not keep up loses its queue and receives exactly one
// resync{buffer_overflow}; memory stays bounded by SubscriberBuffer.
func TestHubOverflow(t *testing.T) {
	h := NewHub(nil)
	s, ok := h.subscribe("session:x")
	if !ok {
		t.Fatal("subscribe")
	}
	for i := range SubscriberBuffer * 3 {
		h.PublishJob("j", int64(i+1))
	}
	if n := len(s.queue); n != 0 || !s.overflow {
		t.Fatalf("queue %d overflow %v", n, s.overflow)
	}
	got := s.take()
	if len(got) != 1 || got[0].Type != "resync" || got[0].Reason != "buffer_overflow" {
		t.Fatalf("take after overflow: %+v", got)
	}
	h.PublishJob("j", 10_000)
	if got := s.take(); len(got) != 1 || got[0].Type != "hint" {
		t.Fatalf("stream did not recover after resync: %+v", got)
	}
	for range MaxStreamsPerPrincipal - 1 {
		if _, ok := h.subscribe("session:x"); !ok {
			t.Fatal("limit too low")
		}
	}
	if _, ok := h.subscribe("session:x"); ok {
		t.Fatal("6th stream accepted")
	}
}

// RV5-07: hints are filtered by the stream principal's read scopes.
func TestHubHintsFilteredByScope(t *testing.T) {
	h := NewHub(nil)
	jobsOnly, _ := h.subscribe("token:a", "jobs:read")
	all, _ := h.subscribe("token:b", "jobs:read", "workers:read", "metrics:read")
	none, _ := h.subscribe("token:c", []string{}...)
	h.PublishJob("j", 1)
	h.PublishWorker("w", 1)
	h.publish(hint("metrics", "", 0))
	h.Resync("gap_detected")
	got := func(s *subscriber) (out []string) {
		for _, m := range s.take() {
			out = append(out, m.Type+":"+m.Entity)
		}
		return out
	}
	if g := got(jobsOnly); len(g) != 2 || g[0] != "hint:job" || g[1] != "resync:" {
		t.Fatalf("jobs:read stream got %v", g)
	}
	if g := got(all); len(g) != 4 {
		t.Fatalf("full-scope stream got %v", g)
	}
	if g := got(none); len(g) != 1 || g[0] != "resync:" {
		t.Fatalf("scope-less stream got %v", g)
	}
}
