package auth

import (
	"strings"
	"sync"
	"time"
)

// LoginLimiter is an in-process token bucket keyed by lowercase email.
//
// Defaults: capacity 5 attempts, refill 1 attempt / 60s. That gives a stable
// user a generous burst window (slow-typing a password) while choking a brute
// forcer to ~1 attempt/minute after a short warmup.
//
// In-process state means a multi-instance deployment doesn't share counters
// (a brute-force attempt distributed across replicas slips through more
// quickly). Acceptable for the single-shop local deploy this app targets; if
// the deployment topology changes, replace with Redis or a similar shared
// store without changing the call sites.
type LoginLimiter struct {
	capacity int
	refill   time.Duration

	mu      sync.Mutex
	buckets map[string]*loginBucket
}

type loginBucket struct {
	tokens   float64
	lastFill time.Time
}

func NewLoginLimiter(capacity int, refill time.Duration) *LoginLimiter {
	if capacity <= 0 {
		capacity = 5
	}
	if refill <= 0 {
		refill = 60 * time.Second
	}
	return &LoginLimiter{
		capacity: capacity,
		refill:   refill,
		buckets:  make(map[string]*loginBucket),
	}
}

// Allow returns true if the email may make an attempt right now, and deducts
// one token. Returns false when the bucket is empty.
func (l *LoginLimiter) Allow(email string) bool {
	key := strings.ToLower(strings.TrimSpace(email))
	if key == "" {
		return true // don't block missing emails — let the caller surface a 401
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		b = &loginBucket{tokens: float64(l.capacity), lastFill: now}
		l.buckets[key] = b
	}
	// Refill based on elapsed time.
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed / l.refill.Seconds()
	if b.tokens > float64(l.capacity) {
		b.tokens = float64(l.capacity)
	}
	b.lastFill = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
