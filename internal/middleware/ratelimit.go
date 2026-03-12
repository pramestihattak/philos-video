package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ipRateLimiter is a simple fixed-window, in-memory, per-IP rate limiter.
type ipRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rlBucket
	limit   int
	window  time.Duration
}

type rlBucket struct {
	count   int
	resetAt time.Time
}

// NewIPRateLimiter returns a middleware that allows at most limit requests per
// window per client IP. Excess requests receive 429 Too Many Requests.
func NewIPRateLimiter(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := &ipRateLimiter{
		entries: make(map[string]*rlBucket),
		limit:   limit,
		window:  window,
	}
	go rl.cleanup()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteIP(r)
			if !rl.allow(ip) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.entries[ip]
	if !ok || now.After(b.resetAt) {
		rl.entries[ip] = &rlBucket{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	if b.count >= rl.limit {
		return false
	}
	b.count++
	return true
}

// cleanup periodically removes expired buckets to prevent memory growth.
func (rl *ipRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window * 2)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rl.mu.Lock()
		for ip, b := range rl.entries {
			if now.After(b.resetAt) {
				delete(rl.entries, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// remoteIP extracts the host portion of r.RemoteAddr (strips the port).
func remoteIP(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	// Fallback: strip after last colon (IPv4 without port is unusual but handled).
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		return host[:idx]
	}
	return host
}
