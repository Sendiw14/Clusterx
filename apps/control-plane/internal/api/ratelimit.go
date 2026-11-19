package api

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Limiter is an in-memory token bucket per key (FR-ENG-22). It is per process: CLUSTERX runs a single
// coordinator (ADR-0001), so there is no shared state to coordinate. Restarting the coordinator resets
// the buckets. The number of tracked keys is bounded; when full, idle (refilled) buckets are evicted,
// and if that is not enough the new key is refused (fail closed: 429).
type Limiter struct {
	Rate  float64 // tokens per second
	Burst float64
	Max   int // tracked keys
	Now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewLimiter returns a limiter allowing rate events/s with the given burst.
func NewLimiter(rate, burst float64) *Limiter {
	return &Limiter{Rate: rate, Burst: burst, Max: 10000, buckets: map[string]*bucket{}}
}

func (l *Limiter) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

// Allow takes one token for key. When none is available it returns false and the wait until one is.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	return l.take(key, true)
}

// Peek reports whether key has a token without taking it.
func (l *Limiter) Peek(key string) (bool, time.Duration) {
	return l.take(key, false)
}

func (l *Limiter) take(key string, consume bool) (bool, time.Duration) {
	if l == nil || l.Rate <= 0 {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.Max {
			for k, x := range l.buckets {
				if x.tokens+now.Sub(x.last).Seconds()*l.Rate >= l.Burst {
					delete(l.buckets, k)
				}
			}
			if len(l.buckets) >= l.Max {
				return false, time.Second
			}
		}
		b = &bucket{tokens: l.Burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = math.Min(l.Burst, b.tokens+now.Sub(b.last).Seconds()*l.Rate)
	b.last = now
	if b.tokens < 1 {
		return false, time.Duration((1 - b.tokens) / l.Rate * float64(time.Second))
	}
	if consume {
		b.tokens--
	}
	return true, 0
}

// rateLimited writes 429 rate_limited with Retry-After in whole seconds (at least 1).
func rateLimited(w http.ResponseWriter, r *http.Request, wait time.Duration) {
	secs := int(math.Ceil(wait.Seconds()))
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	writeErrorDetails(w, r, http.StatusTooManyRequests, "rate_limited", "Too many requests.", map[string]any{"retry_after_seconds": secs})
}

// clientIP is the TCP peer address. Forwarded headers are not trusted (ADR-0005); behind the BFF every
// browser shares the BFF's address, so per-IP limits apply only to failed authentication.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
