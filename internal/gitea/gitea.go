package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const tokenFilePath = "/token/admin-token.txt"

type Client struct {
	baseURL    string
	publicURL  string
	adminUser  string
	adminPass  string
	org        string
	hc         *http.Client
	readAdmin  func() string
}

func NewClient() *Client {
	baseURL := os.Getenv("GITEA_URL")
	if baseURL == "" {
		baseURL = "http://gitea:3000"
	}
	publicURL := os.Getenv("GITEA_PUBLIC_URL")
	if publicURL == "" {
		publicURL = "http://localhost:3000"
	}
	org := os.Getenv("GITEA_ORG")
	if org == "" {
		org = "moulinerie"
	}
	adminUser := os.Getenv("GITEA_ADMIN_USER")
	if adminUser == "" {
		adminUser = "ft_hackthon"
	}
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		publicURL: strings.TrimRight(publicURL, "/"),
		adminUser: adminUser,
		adminPass: os.Getenv("GITEA_ADMIN_PASSWORD"),
		org:       org,
		hc:        &http.Client{},
		readAdmin: readAdminToken,
	}
}

func readAdminToken() string {
	data, err := os.ReadFile(tokenFilePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// NewTestClient creates a Client with a custom HTTP client and base URL for testing.
func NewTestClient(hc *http.Client, baseURL, org string, adminToken string) *Client {
	return &Client{
		baseURL:   baseURL,
		publicURL: baseURL,
		adminUser: "admin",
		adminPass: "password",
		org:       org,
		hc:        hc,
		readAdmin: func() string { return adminToken },
	}
}

func (c *Client) AdminToken() string {
	if c.readAdmin != nil {
		return c.readAdmin()
	}
	return ""
}

type CreateRepoResponse struct {
	CloneURL string `json:"clone_url"`
	Name     string `json:"name"`
}

func (c *Client) CreateUserRepo(username string) (*CreateRepoResponse, error) {
	body := map[string]interface{}{
		"name":           username,
		"auto_init":      false,
		"private":        false,
		"default_branch": "main",
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("create repo marshal: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/orgs/%s/repos", c.baseURL, c.org)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	c.setAuth(req)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create repo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return &CreateRepoResponse{
			CloneURL: fmt.Sprintf("%s/%s/%s.git", c.baseURL, c.org, username),
			Name:     username,
		}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create repo: status %d: %s", resp.StatusCode, string(body))
	}

	var cr CreateRepoResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decode repo response: %w", err)
	}
	return &cr, nil
}

type CreateTokenResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	SHA1 string `json:"sha1"`
}

func (c *Client) CreateUserToken(username, password string) (*CreateTokenResponse, error) {
	body := map[string]interface{}{
		"name":   "ft_hackthon",
		"scopes": []string{"write:repository", "read:repository", "write:user"},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("create token marshal: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/users/%s/tokens", c.baseURL, username)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, password)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var ct CreateTokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&ct); err != nil {
			return nil, fmt.Errorf("decode token response: %w", err)
		}
		return &ct, nil
	}

	body2, _ := io.ReadAll(resp.Body)
	bodyStr := string(body2)

	if strings.Contains(bodyStr, "already exists") {
		tokens, err := c.listUserTokens(username)
		if err != nil {
			return nil, err
		}
		for _, t := range tokens {
			if t.Name == "ft_hackthon" {
				return t, nil
			}
		}
	}

	return nil, fmt.Errorf("create token: status %d: %s", resp.StatusCode, bodyStr)
}

func (c *Client) listUserTokens(username string) ([]*CreateTokenResponse, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s/tokens", c.baseURL, username)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	if token := c.AdminToken(); token != "" {
		req.Header.Set("Authorization", "token "+token)
	} else {
		req.SetBasicAuth(c.adminUser, c.adminPass)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokens []*CreateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (c *Client) AddUserToOrg(username string) error {
	url := fmt.Sprintf("%s/api/v1/teams/%s/members/%s", c.baseURL, c.getTeamID(), username)
	req, _ := http.NewRequest("PUT", url, nil)
	c.setAuth(req)

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("add user to org: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("add user to org: org or user not found")
	}
	body2, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("add user to org: status %d: %s", resp.StatusCode, string(body2))
}

func (c *Client) getTeamID() string {
	url := fmt.Sprintf("%s/api/v1/orgs/%s/teams", c.baseURL, c.org)
	req, _ := http.NewRequest("GET", url, nil)
	c.setAuth(req)
	resp, err := c.hc.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var teams []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
		return ""
	}
	for _, t := range teams {
		if t.Name == "Owners" {
			return fmt.Sprintf("%d", t.ID)
		}
	}
	if len(teams) > 0 {
		return fmt.Sprintf("%d", teams[0].ID)
	}
	return "1"
}

func (c *Client) CreateGiteaUser(username, password string) error {
	body := map[string]interface{}{
		"username":             username,
		"password":             password,
		"email":                fmt.Sprintf("%s@example.com", username),
		"must_change_password": false,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("create gitea user marshal: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/admin/users", c.baseURL)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	c.setAuth(req)

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("create gitea user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		msg := string(body)
		if strings.Contains(msg, "PasswordIsRequired") {
			return fmt.Errorf("password is too weak: minimum 8 characters with at least one uppercase letter, one lowercase letter, one digit, and one special character")
		}
		return fmt.Errorf("create gitea user: status %d: %s", resp.StatusCode, msg)
	}

	return nil
}

func (c *Client) PublicCloneURL(username string) string {
	return fmt.Sprintf("%s/%s/%s.git", c.publicURL, c.org, username)
}

func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if token := c.AdminToken(); token != "" {
		req.Header.Set("Authorization", "token "+token)
		return
	}
	if c.adminUser != "" && c.adminPass != "" {
		req.SetBasicAuth(c.adminUser, c.adminPass)
	}
}
