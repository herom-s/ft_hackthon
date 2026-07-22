package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAlertsHandler(t *testing.T) {
	// Reset checker state
	checker.mu.Lock()
	checker.alerts = nil
	checker.lastRun = time.Time{}
	checker.mu.Unlock()

	t.Run("GET returns empty alerts", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
		AlertsHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		if body["count"] != float64(0) {
			t.Errorf("expected 0 alerts, got %v", body["count"])
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/api/v1/alerts", nil)
		AlertsHandler(w, r)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("POST ack with invalid body returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader("not-json"))
		AlertsHandler(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("POST ack returns accepted", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader(`{"id":"alert-1"}`))
		r.Header.Set("Content-Type", "application/json")
		AlertsHandler(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestAlertChecker_evaluate(t *testing.T) {
	checker.mu.Lock()
	checker.alerts = nil
	checker.lastRun = time.Time{}
	checker.mu.Unlock()

	t.Run("no alerts with low error rate", func(t *testing.T) {
		metrics.mu.Lock()
		metrics.total = 100
		metrics.statusCount[200] = 98
		metrics.statusCount[500] = 2
		metrics.mu.Unlock()

		checker.mu.Lock()
		checker.lastRun = time.Time{}
		checker.alerts = nil
		checker.mu.Unlock()

		checker.evaluate()

		checker.mu.Lock()
		alertCount := len(checker.alerts)
		checker.mu.Unlock()

		if alertCount != 0 {
			t.Errorf("expected 0 alerts for 2%% error rate, got %d", alertCount)
		}
	})

	t.Run("generates alert for high error rate", func(t *testing.T) {
		metrics.mu.Lock()
		metrics.total = 100
		metrics.statusCount[200] = 90
		metrics.statusCount[500] = 10
		metrics.mu.Unlock()

		checker.mu.Lock()
		checker.lastRun = time.Time{}
		checker.alerts = nil
		checker.mu.Unlock()

		checker.evaluate()

		checker.mu.Lock()
		alertCount := len(checker.alerts)
		hasWarning := false
		for _, a := range checker.alerts {
			if a.Severity == AlertWarning && strings.Contains(a.Message, "High error rate") {
				hasWarning = true
				break
			}
		}
		checker.mu.Unlock()

		if alertCount == 0 {
			t.Fatal("expected at least 1 alert")
		}
		if !hasWarning {
			t.Error("expected high error rate alert")
		}
	})
}
