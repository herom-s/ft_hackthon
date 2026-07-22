package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ft_hackthon/internal/database"
	"github.com/ft_hackthon/internal/handler"
)

func TestStatusRouteHandler(t *testing.T) {
	db := database.NewInMemoryDB()
	db.CreateUser(&database.User{ID: "user1", Username: "alice"})
	db.CreateToken("testuser12", "user1")
	apiHandler := handler.NewAPIHandler(db)
	h := statusRouteHandler(apiHandler)

	t.Run("valid path", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/status/job123", nil)
		r.Header.Set("Authorization", "Bearer testuser12")
		h(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("wrong path prefix", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/wrong", nil)
		h(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestLogMiddleware(t *testing.T) {
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := logMiddleware(next)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	mw.ServeHTTP(w, r)

	if !handlerCalled {
		t.Error("expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func testMux(db database.Database) *http.ServeMux {
	apiHandler := handler.NewAPIHandler(db)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", apiHandler.HealthHandler)
	mux.HandleFunc("/api/v1/auth/login", apiHandler.LoginHandler)
	mux.HandleFunc("/api/v1/auth/register", apiHandler.RegisterHandler)
	mux.HandleFunc("/api/v1/grade/submit", apiHandler.SubmitHandler)
	mux.HandleFunc("/api/v1/grade/status/", statusRouteHandler(apiHandler))
	mux.HandleFunc("/api/v1/grade/jobs", apiHandler.JobsListHandler)
	mux.HandleFunc("/api/v1/grade/repo", apiHandler.RepoLinkHandler)
	mux.HandleFunc("/api/v1/grade/suites", apiHandler.ListSuitesHandler)
	mux.HandleFunc("/api/v1/grade/suites/", apiHandler.ChallengesHandler)
	mux.HandleFunc("/api/v1/grade/leaderboard/", apiHandler.LeaderboardHandler)
	mux.HandleFunc("/api/v1/grade/plagiarism/", apiHandler.PlagiarismHandler)
	mux.HandleFunc("/api/v1/user/me", apiHandler.UserInfoHandler)
	return mux
}

func TestRouteRegistration(t *testing.T) {
	db := database.NewInMemoryDB()
	mux := testMux(db)

	server := httptest.NewServer(mux)
	defer server.Close()

	routes := []struct {
		method string
		path   string
		code   int
		body   string
	}{
		{"GET", "/api/v1/health", http.StatusOK, ""},
		{"POST", "/api/v1/auth/login", http.StatusBadRequest, "{}"},
		{"POST", "/api/v1/auth/register", http.StatusBadRequest, "{}"},
		{"POST", "/api/v1/grade/submit", http.StatusUnauthorized, "{}"},
		{"GET", "/api/v1/grade/status/test", http.StatusUnauthorized, ""},
		{"GET", "/api/v1/grade/jobs", http.StatusUnauthorized, ""},
		{"POST", "/api/v1/grade/repo", http.StatusUnauthorized, "{}"},
		{"GET", "/api/v1/grade/suites", http.StatusOK, ""},
		{"GET", "/api/v1/grade/suites/unknown", http.StatusOK, ""},
		{"GET", "/api/v1/grade/leaderboard/test", http.StatusOK, ""},
		{"GET", "/api/v1/grade/plagiarism/test", http.StatusOK, ""},
		{"GET", "/api/v1/user/me", http.StatusUnauthorized, ""},
	}

	for _, route := range routes {
		var bodyReader *strings.Reader
		if route.body != "" {
			bodyReader = strings.NewReader(route.body)
		} else {
			bodyReader = strings.NewReader("")
		}
		r, err := http.NewRequest(route.method, server.URL+route.path, bodyReader)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("request to %s failed: %v", route.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != route.code {
			t.Errorf("%s %s: expected %d, got %d", route.method, route.path, route.code, resp.StatusCode)
		}
	}

	t.Run("health endpoint returns ok with checks", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/health")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var body map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&body)
		if body["status"] != "ok" {
			t.Errorf("expected ok, got %s", body["status"])
		}
		checks, ok := body["checks"].(map[string]interface{})
		if !ok {
			t.Fatal("expected checks object")
		}
		if checks["database"] != "healthy" {
			t.Errorf("expected database healthy, got %s", checks["database"])
		}
	})

	t.Run("suites endpoint returns empty array", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/grade/suites")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var body map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&body)
		suites, ok := body["suites"]
		if !ok {
			t.Error("expected 'suites' key in response")
		} else if suites == nil {
			t.Error("expected non-nil suites array")
		}
	})
}
