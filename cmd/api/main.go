package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ft_hackthon/internal/config"
	"github.com/ft_hackthon/internal/database"
	"github.com/ft_hackthon/internal/handler"
)

func main() {
	cfg := config.LoadServerConfig()

	addr := ":" + cfg.APIPort
	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Printf("║   ft_hackthon API Server                    ║\n")
	fmt.Printf("║   Starting on %-30s ║\n", addr)
	fmt.Println("╚════════════════════════════════════════════╝")

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required (e.g. postgres://user:pass@localhost:5432/ft_hackthon?sslmode=disable)")
	}

	log.Printf("Connecting to PostgreSQL...")
	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	log.Println("Connected to PostgreSQL")
	defer db.Close()

	apiHandler := handler.NewAPIHandler(db)
	apiHandler.SetSuitesPath(cfg.TestSuitesPath)

	// Register routes
	http.HandleFunc("/api/v1/alerts", handler.AlertsHandler)
	http.HandleFunc("/api/v1/metrics", handler.MetricsHandler)
	http.HandleFunc("/api/v1/health", apiHandler.HealthHandler)
	http.HandleFunc("/api/v1/auth/login", apiHandler.LoginHandler)
	http.HandleFunc("/api/v1/auth/register", apiHandler.RegisterHandler)
	http.HandleFunc("/api/v1/grade/submit", apiHandler.SubmitHandler)
	http.HandleFunc("/api/v1/grade/status/", statusRouteHandler(apiHandler))
	http.HandleFunc("/api/v1/grade/jobs", apiHandler.JobsListHandler)
	http.HandleFunc("/api/v1/grade/repo", apiHandler.RepoLinkHandler)
	http.HandleFunc("/api/v1/grade/suites", apiHandler.ListSuitesHandler)
	http.HandleFunc("/api/v1/grade/suites/", apiHandler.ChallengesHandler)
	http.HandleFunc("/api/v1/grade/leaderboard/", apiHandler.LeaderboardHandler)
	http.HandleFunc("/api/v1/grade/plagiarism/", apiHandler.PlagiarismHandler)
	http.HandleFunc("/api/v1/user/me", apiHandler.UserInfoHandler)

	fmt.Println("\nAvailable Endpoints:")
	fmt.Println("  GET  /api/v1/alerts                - System alerts")
	fmt.Println("  POST /api/v1/alerts                - Acknowledge alert ({\"id\":\"...\"})")
	fmt.Println("  GET  /api/v1/metrics               - Prometheus metrics")
	fmt.Println("  GET  /api/v1/health                - Health check")
	fmt.Println("  POST /api/v1/auth/login            - Login")
	fmt.Println("  POST /api/v1/auth/register         - Register")
	fmt.Println("  POST /api/v1/grade/submit          - Submit project")
	fmt.Println("  GET  /api/v1/grade/status/{job_id} - Get job status")
	fmt.Println("  GET  /api/v1/grade/jobs            - List my jobs")
	fmt.Println("  POST /api/v1/grade/repo            - Link repository")
	fmt.Println("  GET  /api/v1/grade/suites          - List available test suites")
	fmt.Println("  GET  /api/v1/grade/suites/{suite}/challenges - List challenges + subjects")
	fmt.Println("  GET  /api/v1/grade/leaderboard/{hackathon} - Show top scorers")
	fmt.Println("  GET  /api/v1/grade/plagiarism/{hackathon}  - Check for duplicate submissions")
	fmt.Println("  GET  /api/v1/user/me               - Show current user info (includes rating)")
	fmt.Println()

	mux := http.DefaultServeMux
	if err := http.ListenAndServe(addr, handler.CORSMiddleware(handler.MetricsMiddleware(handler.RateLimitMiddleware(logMiddleware(mux))))); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// statusRouteHandler wraps the status handler to extract job ID from URL
func statusRouteHandler(h *handler.APIHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract job ID from path
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/v1/grade/status/") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		h.StatusHandler(w, r)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

func init() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if os.Getenv("DEBUG") == "1" {
		opts.Level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
}

// logMiddleware logs HTTP requests with status code, duration, and request ID.
func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := generateRequestID()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		r.Header.Set("X-Request-ID", reqID)
		rw.Header().Set("X-Request-ID", reqID)

		next.ServeHTTP(rw, r)

		slog.Info("request",
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}
