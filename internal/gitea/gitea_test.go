package gitea

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.baseURL != "http://gitea:3000" {
		t.Errorf("expected baseURL, got %s", c.baseURL)
	}
	if c.org != "moulinerie" {
		t.Errorf("expected moulinerie, got %s", c.org)
	}
}

func TestAdminToken(t *testing.T) {
	c := NewTestClient(nil, "http://test", "testorg", "test-admin-token")
	if tok := c.AdminToken(); tok != "test-admin-token" {
		t.Errorf("expected test-admin-token, got %s", tok)
	}
}

func TestPublicCloneURL(t *testing.T) {
	c := NewTestClient(nil, "http://gitea:3000", "myorg", "")
	url := c.PublicCloneURL("alice")
	expected := "http://gitea:3000/myorg/alice.git"
	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

func TestCreateGiteaUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/admin/users" {
			t.Errorf("expected /api/v1/admin/users, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "token admin-token" {
			t.Errorf("expected admin auth, got %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	if err := c.CreateGiteaUser("alice", "secret"); err != nil {
		t.Fatalf("CreateGiteaUser failed: %v", err)
	}
}

func TestCreateGiteaUser_AlreadyExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	if err := c.CreateGiteaUser("alice", "secret"); err != nil {
		t.Fatalf("expected nil for conflict, got %v", err)
	}
}

func TestCreateGiteaUser_WeakPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"PasswordIsRequired"}`))
	}))
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	err := c.CreateGiteaUser("alice", "short")
	if err == nil {
		t.Fatal("expected error for weak password")
	}
}

func TestCreateGiteaUser_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	err := c.CreateGiteaUser("alice", "secret")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestCreateUserRepo(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/orgs/myorg/repos" {
			t.Errorf("expected /api/v1/orgs/myorg/repos, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreateRepoResponse{
			CloneURL: fmt.Sprintf("%s/myorg/alice.git", serverURL),
			Name:     "alice",
		})
	}))
	serverURL = server.URL
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	repo, err := c.CreateUserRepo("alice")
	if err != nil {
		t.Fatalf("CreateUserRepo failed: %v", err)
	}
	if repo.Name != "alice" {
		t.Errorf("expected alice, got %s", repo.Name)
	}
	if repo.CloneURL == "" {
		t.Error("expected non-empty clone URL")
	}
}

func TestCreateUserRepo_Conflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	repo, err := c.CreateUserRepo("alice")
	if err != nil {
		t.Fatalf("expected nil for conflict, got %v", err)
	}
	if repo.Name != "alice" {
		t.Errorf("expected alice, got %s", repo.Name)
	}
}

func TestCreateUserToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreateTokenResponse{SHA1: "token-abc", Name: "ft_hackthon"})
	}))
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	token, err := c.CreateUserToken("alice", "secret")
	if err != nil {
		t.Fatalf("CreateUserToken failed: %v", err)
	}
	if token.SHA1 != "token-abc" {
		t.Errorf("expected token-abc, got %s", token.SHA1)
	}
}

func TestCreateUserToken_AlreadyExists(t *testing.T) {
	gotList := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gotList = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*CreateTokenResponse{
				{SHA1: "existing-token", Name: "ft_hackthon"},
			})
			return
		}
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message":"already exists"}`))
	}))
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	token, err := c.CreateUserToken("alice", "secret")
	if err != nil {
		t.Fatalf("expected to find existing token, got %v", err)
	}
	if token.SHA1 != "existing-token" {
		t.Errorf("expected existing-token, got %s", token.SHA1)
	}
	if !gotList {
		t.Error("expected list tokens call on conflict")
	}
}

func TestAddUserToOrg(t *testing.T) {
	teamReqCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			teamReqCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 42, "name": "Owners"},
			})
			return
		}
		if r.URL.Path == "/api/v1/teams/42/members/alice" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	if err := c.AddUserToOrg("alice"); err != nil {
		t.Fatalf("AddUserToOrg failed: %v", err)
	}
	if teamReqCount != 1 {
		t.Errorf("expected 1 team list request, got %d", teamReqCount)
	}
}

func TestAddUserToOrg_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "Owners"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := NewTestClient(server.Client(), server.URL, "myorg", "admin-token")
	err := c.AddUserToOrg("unknown")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestClientInterface(t *testing.T) {
	var _ ClientInterface = (*Client)(nil)
}
