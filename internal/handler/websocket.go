package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ft_hackthon/internal/database"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *APIHandler) WebSocketStatusHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
	}

	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := h.db.GetUserIDByToken(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	path := r.URL.Path
	jobID := strings.TrimPrefix(path, "/ws/grade/status/")
	if jobID == "" || jobID == path {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	job, err := h.db.GetJob(jobID)
	if err != nil {
		conn.WriteJSON(map[string]string{"error": "Job not found"})
		return
	}
	if job.UserID != userID {
		conn.WriteJSON(map[string]string{"error": "Forbidden"})
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			job, err := h.db.GetJob(jobID)
			if err != nil {
				conn.WriteJSON(map[string]string{"error": "Job not found"})
				return
			}

			resp := StatusResponse{
				JobID:     job.ID,
				Status:    job.Status,
				Message:   job.Message,
				Suite:     job.Suite,
				CommitSHA: job.CommitSHA,
				CreatedAt: job.CreatedAt,
				Result:    job.Result,
			}

			data, _ := json.Marshal(resp)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

			if job.Status == database.JobStatusCompleted ||
				job.Status == database.JobStatusFailed ||
				job.Status == database.JobStatusError {
				conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
				return
			}
		}
	}
}

func (h *APIHandler) WebSocketJobsHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
	}

	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := h.db.GetUserIDByToken(token)
	if err != nil || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			jobs, err := h.db.GetJobsByUser(userID)
			if err != nil {
				continue
			}

			statusJobs := make([]StatusResponse, 0, len(jobs))
			for _, job := range jobs {
				statusJobs = append(statusJobs, StatusResponse{
					JobID:     job.ID,
					Status:    job.Status,
					Message:   job.Message,
					Suite:     job.Suite,
					CommitSHA: job.CommitSHA,
					CreatedAt: job.CreatedAt,
					Result:    job.Result,
				})
			}

			resp := map[string]interface{}{"jobs": statusJobs}
			data, _ := json.Marshal(resp)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}
}

func WSEndpoint(h *APIHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasPrefix(path, "/ws/grade/status/") {
			h.WebSocketStatusHandler(w, r)
			return
		}

		if path == "/ws/grade/jobs" {
			h.WebSocketJobsHandler(w, r)
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
	}
}
