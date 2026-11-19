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
