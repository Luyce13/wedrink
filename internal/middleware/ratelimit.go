package middleware

import (
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"wedrink/internal/models"
	"wedrink/internal/utils"
)

// TokenBucket implements the Token Bucket algorithm for rate limiting.
type TokenBucket struct {
	capacity   float64
	refillRate float64 // Tokens per second
	tokens     float64
	lastRefill time.Time
	lastAccess time.Time
	mu         sync.Mutex
}

func NewTokenBucket(capacity float64, refillRate float64) *TokenBucket {
	now := time.Now()
	return &TokenBucket{
		capacity:   capacity,
		refillRate: refillRate,
		tokens:     capacity,
		lastRefill: now,
		lastAccess: now,
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	tb.lastAccess = now

	// Refill tokens based on elapsed time
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = math.Min(tb.capacity, tb.tokens+(elapsed*tb.refillRate))
	tb.lastRefill = now

	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

// IPRateLimiter manages per-IP TokenBucket rate limiters.
type IPRateLimiter struct {
	capacity   float64
	refillRate float64
	buckets    map[string]*TokenBucket
	mu         sync.RWMutex
}

func NewIPRateLimiter(capacity float64, refillRate float64) *IPRateLimiter {
	limiter := &IPRateLimiter{
		capacity:   capacity,
		refillRate: refillRate,
		buckets:    make(map[string]*TokenBucket),
	}

	// Periodic cleanup goroutine to prune abandoned IP buckets after 10m of inactivity
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			limiter.cleanup(10 * time.Minute)
		}
	}()

	return limiter
}

func (lim *IPRateLimiter) getBucket(ip string) *TokenBucket {
	lim.mu.RLock()
	tb, exists := lim.buckets[ip]
	lim.mu.RUnlock()

	if exists {
		return tb
	}

	lim.mu.Lock()
	defer lim.mu.Unlock()

	// Double-check after acquiring write lock
	if tb, exists = lim.buckets[ip]; exists {
		return tb
	}

	tb = NewTokenBucket(lim.capacity, lim.refillRate)
	lim.buckets[ip] = tb
	return tb
}

func (lim *IPRateLimiter) cleanup(maxAge time.Duration) {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	now := time.Now()
	for ip, tb := range lim.buckets {
		tb.mu.Lock()
		if now.Sub(tb.lastAccess) > maxAge {
			delete(lim.buckets, ip)
		}
		tb.mu.Unlock()
	}
}

// Limit returns an HTTP middleware that limits requests per IP using the Token Bucket algorithm.
// capacity: Max burst capacity (e.g. 5 tokens)
// refillRate: Refill rate in tokens per second (e.g. 0.1 tokens/sec = 6 tokens/min)
func Limit(capacity float64, refillRate float64) func(http.Handler) http.Handler {
	limiter := NewIPRateLimiter(capacity, refillRate)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := utils.GetClientIP(r)
			bucket := limiter.getBucket(ip)

			if !bucket.Allow() {
				w.Header().Set("Retry-After", "30")
				w.Header().Set("HX-Trigger", "rateLimitExceeded")
				w.WriteHeader(http.StatusTooManyRequests)
				if r.Header.Get("HX-Request") == "true" {
					_, _ = w.Write([]byte(fmt.Sprintf(`<div class="alert-banner alert-error flex items-center justify-between p-3 mb-4 rounded-lg bg-rose-950/80 border border-rose-500/50 text-rose-200 text-xs font-semibold shadow-lg"><span>⚠ %s</span></div>`, models.ErrRateLimitExceeded.Error())))
				} else {
					_, _ = w.Write([]byte(models.ErrRateLimitExceeded.Error()))
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
