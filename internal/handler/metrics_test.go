package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/v1/grade/status/a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6", "/api/v1/grade/status/:id"},
		{"/api/v1/health", "/api/v1/health"},
		{"/api/v1/grade/suites", "/api/v1/grade/suites"},
		{"/api/v1/grade/status/", "/api/v1/grade/status"},
		{"/api/v1/grade/status/550e8400-e29b-41d4-a716-446655440000", "/api/v1/grade/status/:id"},
	}
	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMetricsHandler(t *testing.T) {
	// Reset metrics
	metrics.mu.Lock()
	metrics.total = 0
	metrics.statusCount = make(map[int]int64)
	metrics.pathCount = make(map[metricKey]int64)
	metrics.mu.Unlock()

	t.Run("GET returns metrics text", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
		MetricsHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "http_requests_total") {
			t.Error("expected http_requests_total in response")
		}
		ct := w.Header().Get("Content-Type")
		if ct != "text/plain; charset=utf-8" {
			t.Errorf("expected text/plain content type, got %s", ct)
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", nil)
		MetricsHandler(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestMetricsMiddleware(t *testing.T) {
	metrics.mu.Lock()
	metrics.total = 0
	metrics.statusCount = make(map[int]int64)
	metrics.pathCount = make(map[metricKey]int64)
	metrics.mu.Unlock()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	MetricsMiddleware(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected 'ok', got %s", w.Body.String())
	}

	// Verify metrics were recorded
	metrics.mu.Lock()
	if metrics.total != 1 {
		t.Errorf("expected 1 total request, got %d", metrics.total)
	}
	if metrics.statusCount[200] != 1 {
		t.Errorf("expected 1 status 200, got %d", metrics.statusCount[200])
	}
	metrics.mu.Unlock()
}

func TestMetricsMiddleware_capturesStatus(t *testing.T) {
	metrics.mu.Lock()
	metrics.total = 0
	metrics.statusCount = make(map[int]int64)
	metrics.pathCount = make(map[metricKey]int64)
	metrics.mu.Unlock()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	MetricsMiddleware(next).ServeHTTP(w, r)

	metrics.mu.Lock()
	if metrics.statusCount[404] != 1 {
		t.Errorf("expected 1 status 404, got %d", metrics.statusCount[404])
	}
	metrics.mu.Unlock()
}
