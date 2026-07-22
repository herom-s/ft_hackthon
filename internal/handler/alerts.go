package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "info"
	AlertWarning  AlertSeverity = "warning"
	AlertCritical AlertSeverity = "critical"
)

type Alert struct {
	ID        string        `json:"id"`
	Severity  AlertSeverity `json:"severity"`
	Message   string        `json:"message"`
	CreatedAt time.Time     `json:"created_at"`
	Acked     bool          `json:"acked"`
}

type alertChecker struct {
	mu       sync.Mutex
	alerts   []Alert
	lastRun  time.Time
	interval time.Duration
}

var checker = &alertChecker{
	interval: 30 * time.Second,
}

func (ac *alertChecker) evaluate() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if time.Since(ac.lastRun) < ac.interval {
		return
	}
	ac.lastRun = time.Now()

	metrics.mu.Lock()
	total := metrics.total
	statusCounts := make(map[int]int64)
	for k, v := range metrics.statusCount {
		statusCounts[k] = v
	}
	metrics.mu.Unlock()

	if total > 0 {
		serverErrors := statusCounts[500] + statusCounts[502] + statusCounts[503]
		errRate := float64(serverErrors) / float64(total) * 100
		if errRate > 5 {
			ac.addAlert(AlertWarning, fmt.Sprintf("High error rate: %.1f%% (5xx: %d / total: %d)", errRate, serverErrors, total))
		}
		clientErrors := statusCounts[429]
		if clientErrors > 100 {
			ac.addAlert(AlertWarning, fmt.Sprintf("High rate-limiting: %d requests blocked", clientErrors))
		}
	}

	if len(ac.alerts) > 100 {
		ac.alerts = ac.alerts[len(ac.alerts)-50:]
	}
}

func (ac *alertChecker) addAlert(severity AlertSeverity, message string) {
	id := fmt.Sprintf("alert-%d", len(ac.alerts)+1)
	ac.alerts = append(ac.alerts, Alert{
		ID:        id,
		Severity:  severity,
		Message:   message,
		CreatedAt: time.Now(),
	})
	log.Printf("ALERT [%s] %s", severity, message)
}

func AlertsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		type ackReq struct {
			ID string `json:"id"`
		}
		var req ackReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		checker.mu.Lock()
		for i := range checker.alerts {
			if checker.alerts[i].ID == req.ID {
				checker.alerts[i].Acked = true
				break
			}
		}
		checker.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "acknowledged"})
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	checker.evaluate()

	checker.mu.Lock()
	alerts := make([]Alert, len(checker.alerts))
	copy(alerts, checker.alerts)
	checker.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}
