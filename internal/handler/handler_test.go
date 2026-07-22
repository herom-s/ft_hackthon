package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ft_hackthon/internal/database"
	"github.com/ft_hackthon/internal/gitea"
	"golang.org/x/crypto/bcrypt"
)

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(h)
}

type mockGitea struct {
	adminToken      string
	createUserErr   error
	addToOrgErr     error
	repo            *gitea.CreateRepoResponse
	tokenResp       *gitea.CreateTokenResponse
	publicCloneURL  string
}

func (m *mockGitea) AdminToken() string                                          { return m.adminToken }
func (m *mockGitea) CreateGiteaUser(username, password string) error              { return m.createUserErr }
func (m *mockGitea) AddUserToOrg(username string) error                           { return m.addToOrgErr }
func (m *mockGitea) CreateUserRepo(username string) (*gitea.CreateRepoResponse, error) { return m.repo, nil }
func (m *mockGitea) CreateUserToken(username, password string) (*gitea.CreateTokenResponse, error) { return m.tokenResp, nil }
func (m *mockGitea) PublicCloneURL(username string) string                        { return m.publicCloneURL }

func setupTest(t *testing.T, opts ...*mockGitea) (*APIHandler, *database.InMemoryDB) {
	t.Helper()
	db := database.NewInMemoryDB()
	var h *APIHandler
	if len(opts) > 0 {
		h = NewAPIHandler(db, opts[0])
	} else {
		h = NewAPIHandler(db)
	}
	return h, db
}

func TestHealthHandler(t *testing.T) {
	h, _ := setupTest(t)

	t.Run("GET returns ok with healthy db", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		h.HealthHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
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

	t.Run("wrong method returns 405", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
		h.HealthHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestLoginHandler(t *testing.T) {
	h, db := setupTest(t)

	// Create a user for login tests
	db.CreateUser(&database.User{ID: "test_user1", Username: "alice", Password: hashPassword(t, "secret")})

	t.Run("POST with valid credentials", func(t *testing.T) {
		body := `{"username":"alice","password":"secret"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.LoginHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp LoginResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Token == "" {
			t.Error("expected non-empty token")
		}
		if resp.User != "alice" {
			t.Errorf("expected alice, got %s", resp.User)
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
		h.LoginHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		body := `{"username":""}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.LoginHandler(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("not-json"))
		r.Header.Set("Content-Type", "application/json")
		h.LoginHandler(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("wrong password returns 401", func(t *testing.T) {
		body := `{"username":"alice","password":"wrong"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.LoginHandler(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("non-existent user returns 401", func(t *testing.T) {
		body := `{"username":"nonexistent","password":"secret"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.LoginHandler(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})
}

func TestLoginHandler_WithGiteaUser(t *testing.T) {
	mock := &mockGitea{
		adminToken:     "admin-token-123",
		publicCloneURL: "http://public/org/user.git",
	}
	h, db := setupTest(t, mock)

	db.CreateUser(&database.User{
		ID:           "gitea_user1",
		Username:     "giteauser",
		Password:     hashPassword(t, "secret"),
		GiteaRepoURL: "http://gitea/org/giteauser.git",
		GiteaToken:   "user-token-abc",
	})

	t.Run("login returns gitea fields", func(t *testing.T) {
		body := `{"username":"giteauser","password":"secret"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.LoginHandler(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp LoginResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.GiteaCloneURL != "http://public/org/user.git" {
			t.Errorf("expected clone URL, got %s", resp.GiteaCloneURL)
		}
		if resp.GiteaToken != "user-token-abc" {
			t.Errorf("expected gitea token, got %s", resp.GiteaToken)
		}
	})
}

func TestRegisterHandler(t *testing.T) {
	h, db := setupTest(t)

	t.Run("POST with valid credentials", func(t *testing.T) {
		body := `{"username":"bob","password":"secret"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.RegisterHandler(w, r)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		var resp LoginResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Token == "" {
			t.Error("expected non-empty token")
		}
		if resp.User != "bob" {
			t.Errorf("expected bob, got %s", resp.User)
		}

		// Verify user was created in DB
		user, err := db.GetUserByUsername("bob")
		if err != nil {
			t.Fatalf("expected user bob to exist: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("secret")); err != nil {
			t.Errorf("password mismatch: %v", err)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/register", nil)
		h.RegisterHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		body := `{"username":""}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.RegisterHandler(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader("not-json"))
		r.Header.Set("Content-Type", "application/json")
		h.RegisterHandler(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestRegisterHandler_WithGitea(t *testing.T) {
	mock := &mockGitea{
		adminToken: "admin-token-123",
		repo:       &gitea.CreateRepoResponse{CloneURL: "http://gitea/org/user.git", Name: "testuser"},
		tokenResp:  &gitea.CreateTokenResponse{SHA1: "user-token-abc"},
		publicCloneURL: "http://public/org/user.git",
	}
	h, db := setupTest(t, mock)

	t.Run("full registration with gitea provisioning", func(t *testing.T) {
		body := `{"username":"testuser","password":"ValidPass1!"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.RegisterHandler(w, r)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		var resp LoginResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Token == "" {
			t.Error("expected non-empty token")
		}
		if resp.User != "testuser" {
			t.Errorf("expected testuser, got %s", resp.User)
		}
		if resp.GiteaCloneURL != "http://public/org/user.git" {
			t.Errorf("expected clone URL, got %s", resp.GiteaCloneURL)
		}
		if resp.GiteaToken != "user-token-abc" {
			t.Errorf("expected gitea token, got %s", resp.GiteaToken)
		}

		user, err := db.GetUserByUsername("testuser")
		if err != nil {
			t.Fatalf("expected user to exist: %v", err)
		}
		if user.GiteaRepoURL != "http://gitea/org/user.git" {
			t.Errorf("expected repo URL, got %s", user.GiteaRepoURL)
		}
		if user.GiteaToken != "user-token-abc" {
			t.Errorf("expected gitea token, got %s", user.GiteaToken)
		}
	})
}

func TestSubmitHandler(t *testing.T) {
	h, db := setupTest(t)

	// Create a user and their token
	db.CreateUser(&database.User{ID: "user1", Username: "alice"})
	db.CreateToken("testuser12", "user1")

	t.Run("POST with valid request", func(t *testing.T) {
		body := `{"commit_sha":"abc123def456"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/submit", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer testuser12")
		h.SubmitHandler(w, r)

		if w.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
		}

		var resp SubmitResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.JobID == "" {
			t.Error("expected non-empty job ID")
		}
		if resp.Status != database.JobStatusQueued {
			t.Errorf("expected queued, got %s", resp.Status)
		}
	})

	t.Run("no auth header", func(t *testing.T) {
		body := `{"commit_sha":"abc"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/submit", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.SubmitHandler(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("missing commit SHA", func(t *testing.T) {
		body := `{}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/submit", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer testuser12")
		h.SubmitHandler(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/submit", nil)
		h.SubmitHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestStatusHandler(t *testing.T) {
	h, db := setupTest(t)

	// Create a user and job
	db.CreateUser(&database.User{ID: "user1", Username: "alice"})
	db.CreateToken("testuser12", "user1")
	db.CreateJob(&database.Job{ID: "j1", UserID: "user1", CommitSHA: "abc", Status: database.JobStatusQueued})

	t.Run("GET valid job", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/status/j1", nil)
		r.Header.Set("Authorization", "Bearer testuser12")
		h.StatusHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp StatusResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.JobID != "j1" {
			t.Errorf("expected j1, got %s", resp.JobID)
		}
	})

	t.Run("no auth header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/status/j1", nil)
		h.StatusHandler(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/status/j1", nil)
		r.Header.Set("Authorization", "Bearer testuser12")
		h.StatusHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("non-existing job", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/status/j999", nil)
		r.Header.Set("Authorization", "Bearer testuser12")
		h.StatusHandler(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("forbidden - wrong user", func(t *testing.T) {
		db.CreateUser(&database.User{ID: "user2", Username: "eve"})
		db.CreateToken("otheruser", "user2")
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/status/j1", nil)
		r.Header.Set("Authorization", "Bearer otheruser")
		h.StatusHandler(w, r)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}

func TestRespondWithError(t *testing.T) {
	w := httptest.NewRecorder()
	RespondWithError(w, http.StatusBadRequest, "bad request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "bad request" {
		t.Errorf("expected 'bad request', got '%s'", resp.Error)
	}
	if resp.Code != "ERR_400" {
		t.Errorf("expected ERR_400, got %s", resp.Code)
	}
	if resp.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestExtractUserID(t *testing.T) {
	h, db := setupTest(t)
	db.CreateToken("validtoken123", "user1")

	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"valid Bearer token", "Bearer validtoken123", "user1"},
		{"no auth header", "", ""},
		{"unknown token", "Bearer nonexistent", ""},
		{"malformed header", "Basic abc", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			result := h.extractUserID(r)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	token := generateToken()
	if len(token) != 64 {
		t.Errorf("expected 64 hex chars, got %d: %s", len(token), token)
	}

	token2 := generateToken()
	if token == token2 {
		t.Error("expected consecutive calls to produce different tokens")
	}
}

func TestGenerateID(t *testing.T) {
	id := generateID()
	if len(id) != 32 {
		t.Errorf("expected 32 hex chars, got %d: %s", len(id), id)
	}

	id2 := generateID()
	if id == id2 {
		t.Error("expected consecutive calls to produce different IDs")
	}
}

func TestSetSuitesPath(t *testing.T) {
	h, _ := setupTest(t)

	h.SetSuitesPath("/tmp/test-suites")
	if h.suitesPath != "/tmp/test-suites" {
		t.Errorf("expected /tmp/test-suites, got %s", h.suitesPath)
	}
}

func TestListSuitesHandler(t *testing.T) {
	t.Run("empty suites path returns empty list", func(t *testing.T) {
		h, _ := setupTest(t)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/suites", nil)
		h.ListSuitesHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		suites, ok := body["suites"].([]interface{})
		if !ok {
			t.Error("expected suites to be an array")
		}
		if len(suites) != 0 {
			t.Errorf("expected empty suites, got %d", len(suites))
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		h, _ := setupTest(t)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/suites", nil)
		h.ListSuitesHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("with suites directory returns suites", func(t *testing.T) {
		tmpDir := t.TempDir()
		suiteDir := filepath.Join(tmpDir, "test-suite-1")
		os.MkdirAll(suiteDir, 0755)
		os.WriteFile(filepath.Join(suiteDir, "suite.yml"), []byte("name: test-suite-1\nlanguage: c\ndetect: []\nbuild: gcc -o main main.c\nrun: ./main\n"), 0644)

		h, _ := setupTest(t)
		h.SetSuitesPath(tmpDir)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/suites", nil)
		h.ListSuitesHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		suites, ok := body["suites"].([]interface{})
		if !ok {
			t.Fatal("expected suites to be an array")
		}
		if len(suites) != 1 {
			t.Fatalf("expected 1 suite, got %d", len(suites))
		}
		suite := suites[0].(map[string]interface{})
		if suite["name"] != "test-suite-1" {
			t.Errorf("expected test-suite-1, got %v", suite["name"])
		}
	})
}

func TestChallengesHandler(t *testing.T) {
	t.Run("empty suite returns empty challenges", func(t *testing.T) {
		h, _ := setupTest(t)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/suites//challenges", nil)
		h.ChallengesHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		challenges, ok := body["challenges"].([]interface{})
		if !ok {
			t.Error("expected challenges to be an array")
		}
		if len(challenges) != 0 {
			t.Errorf("expected empty challenges, got %d", len(challenges))
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		h, _ := setupTest(t)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/suites/s1/challenges", nil)
		h.ChallengesHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("with challenges directory returns challenges", func(t *testing.T) {
		tmpDir := t.TempDir()

		suiteDir := filepath.Join(tmpDir, "s1")
		os.MkdirAll(suiteDir, 0755)
		os.WriteFile(filepath.Join(suiteDir, "suite.yml"), []byte("name: s1\nlanguage: c\ndetect: []\nbuild: echo\nrun: echo\n"), 0644)

		ch1Dir := filepath.Join(suiteDir, "challenges", "ch1")
		os.MkdirAll(ch1Dir, 0755)
		os.WriteFile(filepath.Join(ch1Dir, "challenge.yml"), []byte("name: ch1\ntitle: Challenge 1\npoints: 10\n"), 0644)
		os.WriteFile(filepath.Join(ch1Dir, "subject.txt"), []byte("Solve this challenge"), 0644)

		ch2Dir := filepath.Join(suiteDir, "challenges", "ch2")
		os.MkdirAll(ch2Dir, 0755)
		os.WriteFile(filepath.Join(ch2Dir, "challenge.yml"), []byte("name: ch2\ntitle: Challenge 2\npoints: 20\n"), 0644)
		os.WriteFile(filepath.Join(ch2Dir, "subject.txt"), []byte("Another challenge"), 0644)

		h, _ := setupTest(t)
		h.SetSuitesPath(tmpDir)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/suites/s1/challenges", nil)
		h.ChallengesHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		challenges, ok := body["challenges"].([]interface{})
		if !ok {
			t.Fatal("expected challenges to be an array")
		}
		if len(challenges) != 2 {
			t.Fatalf("expected 2 challenges, got %d", len(challenges))
		}
	})
}

func TestLeaderboardHandler(t *testing.T) {
	t.Run("empty suite returns empty entries", func(t *testing.T) {
		h, _ := setupTest(t)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/leaderboard/", nil)
		h.LeaderboardHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		entries, ok := body["entries"].([]interface{})
		if !ok {
			t.Error("expected entries to be an array")
		}
		if len(entries) != 0 {
			t.Errorf("expected empty entries, got %d", len(entries))
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		h, _ := setupTest(t)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/leaderboard/s1", nil)
		h.LeaderboardHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("with leaderboard data returns entries", func(t *testing.T) {
		h, db := setupTest(t)

		db.CreateUser(&database.User{ID: "u1", Username: "alice", Rating: 1300})
		db.CreateUser(&database.User{ID: "u2", Username: "bob", Rating: 1100})

		db.CreateJob(&database.Job{
			ID: "j1", UserID: "u1", Suite: "s1",
		})
		db.SaveResult("j1", &database.Result{FinalScore: 90, BenchmarkMs: 100})

		db.CreateJob(&database.Job{
			ID: "j2", UserID: "u2", Suite: "s1",
		})
		db.SaveResult("j2", &database.Result{FinalScore: 80, BenchmarkMs: 200})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/leaderboard/s1", nil)
		h.LeaderboardHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		entries, ok := body["entries"].([]interface{})
		if !ok {
			t.Fatal("expected entries to be an array")
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
	})
}

func TestPlagiarismHandler(t *testing.T) {
	t.Run("empty suite returns empty groups", func(t *testing.T) {
		h, _ := setupTest(t)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/plagiarism/", nil)
		h.PlagiarismHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		groups, ok := body["groups"].([]interface{})
		if !ok {
			t.Error("expected groups to be an array")
		}
		if len(groups) != 0 {
			t.Errorf("expected empty groups, got %d", len(groups))
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		h, _ := setupTest(t)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/plagiarism/s1", nil)
		h.PlagiarismHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("with plagiarism data returns groups", func(t *testing.T) {
		h, db := setupTest(t)

		db.CreateUser(&database.User{ID: "u1", Username: "alice"})
		db.CreateUser(&database.User{ID: "u2", Username: "bob"})

		db.CreateJob(&database.Job{
			ID: "j1", UserID: "u1", Suite: "s1",
		})
		db.SaveResult("j1", &database.Result{FinalScore: 90, CodeChecksum: "abc123"})

		db.CreateJob(&database.Job{
			ID: "j2", UserID: "u2", Suite: "s1",
		})
		db.SaveResult("j2", &database.Result{FinalScore: 80, CodeChecksum: "abc123"})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/plagiarism/s1", nil)
		h.PlagiarismHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var body map[string]interface{}
		json.NewDecoder(w.Body).Decode(&body)
		groups, ok := body["groups"].([]interface{})
		if !ok {
			t.Fatal("expected groups to be an array")
		}
		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
	})
}

func TestJobsListHandler(t *testing.T) {
	h, db := setupTest(t)

	db.CreateUser(&database.User{ID: "user1", Username: "alice"})
	db.CreateToken("token123", "user1")

	db.CreateJob(&database.Job{ID: "j1", UserID: "user1", CommitSHA: "abc", Status: database.JobStatusQueued})
	db.CreateJob(&database.Job{ID: "j2", UserID: "user1", CommitSHA: "def", Status: database.JobStatusCompleted, Result: &database.Result{FinalScore: 85}})

	t.Run("GET returns jobs list", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/jobs", nil)
		r.Header.Set("Authorization", "Bearer token123")
		h.JobsListHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp JobsListResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if len(resp.Jobs) != 2 {
			t.Fatalf("expected 2 jobs, got %d", len(resp.Jobs))
		}
	})

	t.Run("no auth header returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/jobs", nil)
		h.JobsListHandler(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/jobs", nil)
		r.Header.Set("Authorization", "Bearer token123")
		h.JobsListHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestUserInfoHandler(t *testing.T) {
	h, db := setupTest(t)

	db.CreateUser(&database.User{ID: "user1", Username: "alice", Rating: 1350})
	db.CreateToken("token123", "user1")

	t.Run("GET returns user info with rating", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/userinfo", nil)
		r.Header.Set("Authorization", "Bearer token123")
		h.UserInfoHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp UserInfoResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.ID != "user1" {
			t.Errorf("expected user1, got %s", resp.ID)
		}
		if resp.Username != "alice" {
			t.Errorf("expected alice, got %s", resp.Username)
		}
		if resp.Rating != 1350 {
			t.Errorf("expected 1350, got %d", resp.Rating)
		}
	})

	t.Run("rating 0 uses default rating", func(t *testing.T) {
		db.CreateUser(&database.User{ID: "user2", Username: "bob"})
		db.CreateToken("token456", "user2")

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/userinfo", nil)
		r.Header.Set("Authorization", "Bearer token456")
		h.UserInfoHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp UserInfoResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Rating != database.DefaultRating {
			t.Errorf("expected %d, got %d", database.DefaultRating, resp.Rating)
		}
	})

	t.Run("no auth header returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/userinfo", nil)
		h.UserInfoHandler(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/userinfo", nil)
		r.Header.Set("Authorization", "Bearer token123")
		h.UserInfoHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestRepoLinkHandler(t *testing.T) {
	h, db := setupTest(t)

	db.CreateUser(&database.User{ID: "user1", Username: "alice"})
	db.CreateToken("token123", "user1")

	t.Run("POST with valid repo_path", func(t *testing.T) {
		body := `{"repo_path":"git@github.com:alice/repo.git"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/repo/link", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer token123")
		h.RepoLinkHandler(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]string
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["status"] != "linked" {
			t.Errorf("expected linked, got %s", resp["status"])
		}
		if resp["repo_path"] != "git@github.com:alice/repo.git" {
			t.Errorf("expected repo path, got %s", resp["repo_path"])
		}
		if resp["user_id"] != "user1" {
			t.Errorf("expected user1, got %s", resp["user_id"])
		}
	})

	t.Run("no auth header returns 401", func(t *testing.T) {
		body := `{"repo_path":"git@github.com:alice/repo.git"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/repo/link", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.RepoLinkHandler(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("missing repo_path returns 400", func(t *testing.T) {
		body := `{}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/grade/repo/link", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer token123")
		h.RepoLinkHandler(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/grade/repo/link", nil)
		r.Header.Set("Authorization", "Bearer token123")
		h.RepoLinkHandler(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}
