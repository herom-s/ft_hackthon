package handler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimiter_allow(t *testing.T) {
	rl := newRateLimiter(3, 1*time.Minute)

	t.Run("allows requests up to limit", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			ok, remaining, _ := rl.allow("key-a")
			if !ok {
				t.Fatalf("request %d should be allowed", i+1)
			}
			expected := 2 - i
			if remaining != expected {
				t.Errorf("request %d: expected remaining %d, got %d", i+1, expected, remaining)
			}
		}
	})

	t.Run("blocks after limit", func(t *testing.T) {
		ok, remaining, _ := rl.allow("key-a")
		if ok {
			t.Error("request should be blocked")
		}
		if remaining != 0 {
			t.Errorf("expected remaining 0, got %d", remaining)
		}
	})

	t.Run("different keys have independent limits", func(t *testing.T) {
		ok, remaining, _ := rl.allow("key-b")
		if !ok {
			t.Error("different key should be allowed")
		}
		if remaining != 2 {
			t.Errorf("expected remaining 2, got %d", remaining)
		}
	})

	t.Run("resets after window", func(t *testing.T) {
		rl := newRateLimiter(1, 50*time.Millisecond)
		ok, _, _ := rl.allow("key-c")
		if !ok {
			t.Fatal("first request should be allowed")
		}
		ok, _, _ = rl.allow("key-c")
		if ok {
			t.Fatal("second request should be blocked")
		}
		time.Sleep(60 * time.Millisecond)
		ok, _, _ = rl.allow("key-c")
		if !ok {
			t.Error("request after window should be allowed")
		}
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("sets rate limit headers on success", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		RateLimitMiddleware(next).ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Header().Get("X-RateLimit-Limit") != strconv.Itoa(DefaultRateLimit) {
			t.Errorf("expected X-RateLimit-Limit header")
		}
		if w.Header().Get("X-RateLimit-Remaining") == "" {
			t.Errorf("expected X-RateLimit-Remaining header")
		}
		if w.Header().Get("X-RateLimit-Reset") == "" {
			t.Errorf("expected X-RateLimit-Reset header")
		}
	})

	t.Run("blocks when rate limit exceeded", func(t *testing.T) {
		// Use a limiter with very low limit by re-creating middleware each request
		// Since middleware creates its own limiter, we need to use the same instance
		limiter := newRateLimiter(1, 1*time.Minute)
		mw := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key := r.RemoteAddr
				if auth := r.Header.Get("Authorization"); auth != "" {
					key = auth
				}
				allowed, remaining, resetIn := limiter.allow(key)
				w.Header().Set("X-RateLimit-Limit", "1")
				w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
				w.Header().Set("X-RateLimit-Reset", strconv.Itoa(resetIn))
				if !allowed {
					http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
					return
				}
				next.ServeHTTP(w, r)
			})
		}

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// First request should succeed
		w1 := httptest.NewRecorder()
		r1 := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		mw(next).ServeHTTP(w1, r1)
		if w1.Code != http.StatusOK {
			t.Errorf("first request: expected 200, got %d", w1.Code)
		}

		// Second request should be blocked
		w2 := httptest.NewRecorder()
		r2 := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		mw(next).ServeHTTP(w2, r2)
		if w2.Code != http.StatusTooManyRequests {
			t.Errorf("second request: expected 429, got %d", w2.Code)
		}
	})

	t.Run("uses token as key when Bearer auth present", func(t *testing.T) {
		limiter := newRateLimiter(1, 1*time.Minute)
		mw := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key := r.RemoteAddr
				if auth := r.Header.Get("Authorization"); auth != "" {
					key = auth
				}
				allowed, _, _ := limiter.allow(key)
				if !allowed {
					http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
					return
				}
				next.ServeHTTP(w, r)
			})
		}

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Request with token should be keyed by token, not IP
		w1 := httptest.NewRecorder()
		r1 := httptest.NewRequest(http.MethodGet, "/api/v1/grade/jobs", nil)
		r1.Header.Set("Authorization", "Bearer token123")
		mw(next).ServeHTTP(w1, r1)
		if w1.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w1.Code)
		}

		// Same token should be blocked (limit 1)
		w2 := httptest.NewRecorder()
		r2 := httptest.NewRequest(http.MethodGet, "/api/v1/grade/jobs", nil)
		r2.Header.Set("Authorization", "Bearer token123")
		mw(next).ServeHTTP(w2, r2)
		if w2.Code != http.StatusTooManyRequests {
			t.Errorf("expected 429 for same token, got %d", w2.Code)
		}

		// Different token should still be allowed (independent limit)
		w3 := httptest.NewRecorder()
		r3 := httptest.NewRequest(http.MethodGet, "/api/v1/grade/jobs", nil)
		r3.Header.Set("Authorization", "Bearer token456")
		mw(next).ServeHTTP(w3, r3)
		if w3.Code != http.StatusOK {
			t.Errorf("expected 200 for different token, got %d", w3.Code)
		}
	})
}
