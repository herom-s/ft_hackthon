package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ft_hackthon/internal/config"
)

func setupConfig(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })
	os.Setenv("HOME", tmpDir)
	return tmpDir
}

func TestNewAPIClient(t *testing.T) {
	setupConfig(t)

	t.Run("with custom base URL", func(t *testing.T) {
		c := NewAPIClient("http://example.com/api")
		if c.baseURL != "http://example.com/api" {
			t.Errorf("expected http://example.com/api, got %s", c.baseURL)
		}
	})

	t.Run("with empty base URL defaults", func(t *testing.T) {
		c := NewAPIClient("")
		expected := "https://localhost:8443/api/v1"
		if c.baseURL != expected {
			t.Errorf("expected %s, got %s", expected, c.baseURL)
		}
	})
}

func TestSetGetToken(t *testing.T) {
	setupConfig(t)
	c := NewAPIClient("http://example.com")

	if tok := c.GetToken(); tok != "" {
		t.Errorf("expected empty token, got %s", tok)
	}

	c.SetToken("mytoken")
	if tok := c.GetToken(); tok != "mytoken" {
		t.Errorf("expected mytoken, got %s", tok)
	}
}

func TestLogin(t *testing.T) {
	setupConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["username"] != "alice" || body["password"] != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(LoginResponse{Token: "test-token", User: "alice"})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL + "/api/v1")
	client.client.SetTimeout(5 * time.Second)
	resp, err := client.Login("alice", "secret")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if resp.Token != "test-token" {
		t.Errorf("expected test-token, got %s", resp.Token)
	}
	if resp.User != "alice" {
		t.Errorf("expected alice, got %s", resp.User)
	}
	if client.token != "test-token" {
		t.Error("expected client token to be set")
	}
}

func TestLogin_Failure(t *testing.T) {
	setupConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL + "/api/v1")
	client.client.SetTimeout(5 * time.Second)
	_, err := client.Login("alice", "wrong")
	if err == nil {
		t.Fatal("expected error for invalid credentials")
	}
}

func TestSubmit(t *testing.T) {
	setupConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(SubmitResponse{JobID: "job-123", Status: "queued"})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL + "/api/v1")
	client.client.SetTimeout(5 * time.Second)

	t.Run("without auth", func(t *testing.T) {
		_, err := client.Submit("abc123", "")
		if err == nil {
			t.Fatal("expected error without auth")
		}
	})

	t.Run("with auth", func(t *testing.T) {
		client.SetToken("valid-token")
		resp, err := client.Submit("abc123", "")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if resp.JobID != "job-123" {
			t.Errorf("expected job-123, got %s", resp.JobID)
		}
	})
}

func TestGetStatus(t *testing.T) {
	setupConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(StatusResponse{
			JobID:   "job-123",
			Status:  "completed",
			Message: "Done",
			Result:  &GradeResult{ParserSuccess: true, BenchmarkMs: 100, FinalScore: 90},
		})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL + "/api/v1")
	client.client.SetTimeout(5 * time.Second)

	t.Run("without auth", func(t *testing.T) {
		_, err := client.GetStatus("job-123")
		if err == nil {
			t.Fatal("expected error without auth")
		}
	})

	t.Run("with auth", func(t *testing.T) {
		client.SetToken("token")
		resp, err := client.GetStatus("job-123")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if resp.Status != "completed" {
			t.Errorf("expected completed, got %s", resp.Status)
		}
		if resp.Result.FinalScore != 90 {
			t.Errorf("expected 90, got %d", resp.Result.FinalScore)
		}
	})
}

func TestNewSubmitManager(t *testing.T) {
	setupConfig(t)
	client := NewAPIClient("http://example.com")
	sm := NewSubmitManager(client)
	if sm == nil {
		t.Fatal("expected non-nil submit manager")
	}
	if sm.ui == nil {
		t.Error("expected non-nil terminal UI")
	}
}

func TestTerminalUI_PrintStatusUpdate(t *testing.T) {
	ui := NewTerminalUI()

	tests := []struct {
		status  string
		message string
	}{
		{"queued", "Waiting in line"},
		{"processing", "Running tests"},
		{"completed", "Done!"},
		{"failed", "Something broke"},
		{"error", "System error"},
		{"unknown", "Status update"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			resp := &StatusResponse{Status: tt.status, Message: tt.message}
			ui.PrintStatusUpdate(resp)
		})
	}
}

func TestTerminalUI_PrintGradeResult(t *testing.T) {
	ui := NewTerminalUI()

	t.Run("with details", func(t *testing.T) {
		result := &GradeResult{
			ParserSuccess: true,
			BenchmarkMs:   150,
			FinalScore:    85,
			Details:       "All tests passed",
		}
		ui.PrintGradeResult(result)
	})

	t.Run("without details", func(t *testing.T) {
		result := &GradeResult{
			ParserSuccess: false,
			BenchmarkMs:   200,
			FinalScore:    0,
		}
		ui.PrintGradeResult(result)
	})
}

func TestTerminalUI_StyleMethods(t *testing.T) {
	ui := NewTerminalUI()

	ui.PrintHeader("Test Header")
	ui.PrintLoadingSpinner("Loading...")
}

func TestStatusEmoji(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"queued", "⏳"},
		{"processing", "⚙"},
		{"completed", "✓"},
		{"failed", "❌"},
		{"error", "❌"},
		{"unknown", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			emoji := getStatusEmoji(tt.status)
			if emoji != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, emoji)
			}
		})
	}
}

func TestStatusMessage(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"queued", "Queued"},
		{"processing", "Processing"},
		{"completed", "Completed"},
		{"failed", "Failed"},
		{"error", "Error"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			msg := getStatusMessage(tt.status)
			if !strings.HasPrefix(msg, tt.expected) {
				t.Errorf("expected prefix %q, got %q", tt.expected, msg)
			}
		})
	}
}

func TestCenterText(t *testing.T) {
	ui := NewTerminalUI()

	tests := []struct {
		text  string
		width int
	}{
		{"hello", 20},
		{"short", 5},
		{"exact_len!", 10},
		{"odd", 15},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%d", tt.text, tt.width), func(t *testing.T) {
			result := ui.centerText(tt.text, tt.width)
			if len(result) != tt.width {
				t.Errorf("expected length %d, got %d", tt.width, len(result))
			}
			// Text should be roughly centered (within 1 char of center)
			trimmed := strings.TrimSpace(result)
			if trimmed != tt.text {
				t.Errorf("expected text %q somewhere in result, got %q", tt.text, result)
			}
		})
	}
}

func TestPrintTableRow(t *testing.T) {
	ui := NewTerminalUI()
	ui.printTableRow("Score", "95", 40)
	ui.printTableRow("Very Long Label", "Short", 20)
}

func TestGetCurrentDir(t *testing.T) {
	dir, err := GetCurrentDir()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty directory")
	}
}

func TestConfigFunctions(t *testing.T) {
	setupConfig(t)

	// Verify config path is in the temp dir
	cfgPath, err := config.GetConfigPath()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	home := os.Getenv("HOME")
	expected := filepath.Join(home, ".ft_hackthon")
	if cfgPath != expected {
		t.Errorf("expected %s, got %s", expected, cfgPath)
	}
}

func setupAPITestServer(t *testing.T, handler http.HandlerFunc) (*APIClient, *httptest.Server) {
	t.Helper()
	s := httptest.NewServer(handler)
	c := NewAPIClient(s.URL + "/api/v1")
	c.client.SetTimeout(5 * time.Second)
	c.SetToken("test-token")
	return c, s
}

func TestGetLeaderboard(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(LeaderboardResponse{
			Entries: []LeaderboardEntry{
				{Username: "alice", Score: 100},
				{Username: "bob", Score: 80},
			},
		})
	})
	defer s.Close()

	resp, err := c.GetLeaderboard("libft")
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Entries))
	}
	if resp.Entries[0].Username != "alice" {
		t.Errorf("expected alice first, got %s", resp.Entries[0].Username)
	}
}

func TestGetLeaderboard_Error(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer s.Close()

	_, err := c.GetLeaderboard("libft")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestGetUserInfo(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(UserInfoResponse{Username: "alice", Rating: 1200})
	})
	defer s.Close()

	info, err := c.GetUserInfo()
	if err != nil {
		t.Fatalf("GetUserInfo failed: %v", err)
	}
	if info.Username != "alice" {
		t.Errorf("expected alice, got %s", info.Username)
	}
	if info.Rating != 1200 {
		t.Errorf("expected 1200, got %d", info.Rating)
	}
}

func TestCheckPlagiarism(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(PlagiarismResponse{
			Groups: []PlagiarismGroup{
				{Checksum: "abc123", Users: []string{"alice", "bob"}},
			},
		})
	})
	defer s.Close()

	resp, err := c.CheckPlagiarism("libft")
	if err != nil {
		t.Fatalf("CheckPlagiarism failed: %v", err)
	}
	if len(resp.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(resp.Groups))
	}
	if len(resp.Groups[0].Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(resp.Groups[0].Users))
	}
}

func TestLinkRepo(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	defer s.Close()

	err := c.LinkRepo("/tmp/repo", "http://example.com/repo.git")
	if err != nil {
		t.Fatalf("LinkRepo failed: %v", err)
	}
}

func TestListSuites(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SuitesListResponse{
			Suites: []SuiteInfo{
				{Name: "libft", Active: true},
				{Name: "code-marathon", Active: true},
			},
		})
	})
	defer s.Close()

	resp, err := c.ListSuites()
	if err != nil {
		t.Fatalf("ListSuites failed: %v", err)
	}
	if len(resp.Suites) != 2 {
		t.Fatalf("expected 2 suites, got %d", len(resp.Suites))
	}
}

func TestGetChallenges(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ChallengesResponse{
			Challenges: []ChallengeInfo{
				{Name: "factorial", Points: 10},
			},
		})
	})
	defer s.Close()

	resp, err := c.GetChallenges("code-marathon")
	if err != nil {
		t.Fatalf("GetChallenges failed: %v", err)
	}
	if len(resp.Challenges) != 1 {
		t.Fatalf("expected 1 challenge, got %d", len(resp.Challenges))
	}
}

func TestGetJob(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(StatusResponse{
			JobID:   "job-1",
			Status:  "completed",
			Message: "Done",
		})
	})
	defer s.Close()

	job, err := c.GetJob("job-1")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if job.Status != "completed" {
		t.Errorf("expected completed, got %s", job.Status)
	}
}

func TestListJobs(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(JobsListResponse{
			Jobs: []StatusResponse{
				{JobID: "job-1", Status: "completed"},
				{JobID: "job-2", Status: "processing"},
			},
		})
	})
	defer s.Close()

	resp, err := c.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(resp.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(resp.Jobs))
	}
}

func TestNewAuthManager(t *testing.T) {
	setupConfig(t)
	c := NewAPIClient("http://example.com")
	am := NewAuthManager(c)
	if am.apiClient != c {
		t.Error("expected apiClient to be set")
	}
}

func TestAuthManager_Logout(t *testing.T) {
	setupConfig(t)
	config.SaveConfig(&config.Config{Token: "test-token", User: "alice"})

	c := NewAPIClient("http://example.com")
	am := NewAuthManager(c)

	if err := am.Logout(); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	auth, _ := config.IsAuthenticated()
	if auth {
		t.Error("expected not authenticated after logout")
	}
}

func TestAuthManager_IsAuthenticated(t *testing.T) {
	setupConfig(t)
	c := NewAPIClient("http://example.com")
	am := NewAuthManager(c)

	t.Run("not authenticated", func(t *testing.T) {
		auth, err := am.IsAuthenticated()
		if err != nil {
			t.Fatalf("IsAuthenticated failed: %v", err)
		}
		if auth {
			t.Error("expected false when not authenticated")
		}
	})

	t.Run("authenticated", func(t *testing.T) {
		config.SaveConfig(&config.Config{Token: "test-token"})
		auth, err := am.IsAuthenticated()
		if err != nil {
			t.Fatalf("IsAuthenticated failed: %v", err)
		}
		if !auth {
			t.Error("expected true when authenticated")
		}
	})
}

func TestAuthManager_GetCurrentUser(t *testing.T) {
	setupConfig(t)
	c := NewAPIClient("http://example.com")
	am := NewAuthManager(c)

	t.Run("no user", func(t *testing.T) {
		user, err := am.GetCurrentUser()
		if err != nil {
			t.Fatalf("GetCurrentUser failed: %v", err)
		}
		if user != "" {
			t.Errorf("expected empty user, got %s", user)
		}
	})

	t.Run("with user", func(t *testing.T) {
		config.SaveConfig(&config.Config{Token: "test-token", User: "bob"})
		user, err := am.GetCurrentUser()
		if err != nil {
			t.Fatalf("GetCurrentUser failed: %v", err)
		}
		if user != "bob" {
			t.Errorf("expected bob, got %s", user)
		}
	})
}

func TestSubmitManager_NewSubmitManager(t *testing.T) {
	setupConfig(t)
	c := NewAPIClient("http://example.com")
	sm := NewSubmitManager(c)
	if sm.apiClient != c {
		t.Error("expected apiClient to be set")
	}
	if sm.ui == nil {
		t.Error("expected ui to be set")
	}
}

func TestReadSuiteConfig(t *testing.T) {
	dir := t.TempDir()

	t.Run("no config file", func(t *testing.T) {
		if suite := readSuiteConfig(dir); suite != "" {
			t.Errorf("expected empty, got %s", suite)
		}
	})

	t.Run("with valid config", func(t *testing.T) {
		cfgPath := filepath.Join(dir, "ft_hackthon.yml")
		os.WriteFile(cfgPath, []byte("suite: libft-tester\n"), 0644)
		if suite := readSuiteConfig(dir); suite != "libft-tester" {
			t.Errorf("expected libft-tester, got %s", suite)
		}
	})

	t.Run("with invalid yaml", func(t *testing.T) {
		cfgPath := filepath.Join(dir, "ft_hackthon.yml")
		os.WriteFile(cfgPath, []byte("not: :yaml"), 0644)
		if suite := readSuiteConfig(dir); suite != "" {
			t.Errorf("expected empty, got %s", suite)
		}
	})
}

func TestSubmitManager_HasGiteaConfig(t *testing.T) {
	setupConfig(t)
	c := NewAPIClient("http://example.com")
	sm := NewSubmitManager(c)

	t.Run("no gitea config", func(t *testing.T) {
		if sm.HasGiteaConfig() {
			t.Error("expected false without gitea config")
		}
	})

	t.Run("with gitea config", func(t *testing.T) {
		config.SaveConfig(&config.Config{
			Token:         "test-token",
			GiteaCloneURL: "http://gitea/org/repo.git",
			GiteaToken:    "gitea-token",
		})
		if !sm.HasGiteaConfig() {
			t.Error("expected true with gitea config")
		}
	})
}

func TestSubmitManager_PollStatus(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(StatusResponse{
			JobID:   "job-1",
			Status:  "completed",
			Message: "Done",
			Result: &GradeResult{
				ParserSuccess: true,
				BenchmarkMs:   100,
				FinalScore:    85,
			},
		})
	})
	defer s.Close()

	sm := NewSubmitManager(c)

	// Override polling interval for quick test
	pollingInterval = 1

	if err := sm.PollStatus("job-1"); err != nil {
		t.Fatalf("PollStatus failed: %v", err)
	}
}

func TestSubmitManager_PollStatus_Failed(t *testing.T) {
	setupConfig(t)
	c, s := setupAPITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(StatusResponse{
			JobID:   "job-1",
			Status:  "failed",
			Message: "Tests failed",
		})
	})
	defer s.Close()

	sm := NewSubmitManager(c)
	pollingInterval = 1

	if err := sm.PollStatus("job-1"); err == nil {
		t.Fatal("expected error for failed job")
	}
}
