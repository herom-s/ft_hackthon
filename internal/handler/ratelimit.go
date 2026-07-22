package handler

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultRateLimit  = 100
	DefaultRateWindow = 1 * time.Minute
)

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

type visitor struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(key string) (bool, int, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, ok := rl.visitors[key]

	if !ok || now.After(v.resetAt) {
		rl.visitors[key] = &visitor{
			count:   1,
			resetAt: now.Add(rl.window),
		}
		return true, rl.limit - 1, int(rl.window.Seconds())
	}

	v.count++
	remaining := rl.limit - v.count
	if remaining < 0 {
		remaining = 0
	}
	resetIn := int(time.Until(v.resetAt).Seconds())
	if resetIn < 0 {
		resetIn = 0
	}
	return v.count <= rl.limit, remaining, resetIn
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		for key, v := range rl.visitors {
			if time.Now().After(v.resetAt) {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

func RateLimitMiddleware(next http.Handler) http.Handler {
	limiter := newRateLimiter(DefaultRateLimit, DefaultRateWindow)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}

		allowed, remaining, resetIn := limiter.allow(key)
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(DefaultRateLimit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.Itoa(resetIn))

		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(resetIn))
			http.Error(w, `{"error":"rate limit exceeded","code":"RATE_LIMITED"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
