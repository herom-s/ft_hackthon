package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ft_hackthon/internal/config"
	"github.com/go-resty/resty/v2"
	"gopkg.in/yaml.v3"
)

const (
	defaultAPITimeout  = 30
	maxPollingAttempts = 300 // 10 minutes total
)

var pollingInterval = 2 // seconds

// APIClient wraps the HTTP client for API communication
type APIClient struct {
	client  *resty.Client
	baseURL string
	token   string
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) *APIClient {
	if baseURL == "" {
		baseURL = os.Getenv("API_URL")
	}
	if baseURL == "" {
		baseURL = "https://localhost:8443/api/v1"
	}

	client := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(time.Duration(defaultAPITimeout) * time.Second)

	// Load token from config if available
	token, _ := config.GetToken()

	return &APIClient{
		client:  client,
		baseURL: baseURL,
		token:   token,
	}
}

// SetInsecureSkipVerify configures the client to skip TLS verification
// and switches to HTTP if the URL uses HTTPS (insecure mode implies
// the server may not have TLS configured).
func (ac *APIClient) SetInsecureSkipVerify() {
	ac.client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	if strings.HasPrefix(ac.baseURL, "https://") {
		ac.baseURL = "http://" + ac.baseURL[len("https://"):]
		ac.client.SetBaseURL(ac.baseURL)
	}
}

// SetToken sets the authentication token
func (ac *APIClient) SetToken(token string) {
	ac.token = token
	ac.client.SetAuthToken(token)
}

// GetToken returns the current token
func (ac *APIClient) GetToken() string {
	return ac.token
}

// LoginResponse represents the API login response
type LoginResponse struct {
	Token         string `json:"token"`
	User          string `json:"user"`
	GiteaCloneURL string `json:"gitea_clone_url,omitempty"`
	GiteaToken    string `json:"gitea_token,omitempty"`
}

// Login authenticates the user and returns a token
func (ac *APIClient) Login(username, password string) (*LoginResponse, error) {
	payload := map[string]string{
		"username": username,
		"password": password,
	}

	var resp LoginResponse
	httpResp, err := ac.client.R().
		SetBody(payload).
		SetResult(&resp).
		Post("/auth/login")

	if err != nil {
		return nil, fmt.Errorf("login request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("login failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	ac.SetToken(resp.Token)
	return &resp, nil
}

// SubmitRequest represents the grade submission request
type SubmitRequest struct {
	CommitSHA string `json:"commit_sha"`
	Suite     string `json:"suite,omitempty"`
}

// SubmitResponse represents the API submit response
type SubmitResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

// Submit submits a grading request with the git commit SHA and optional suite
func (ac *APIClient) Submit(commitSHA, suite string) (*SubmitResponse, error) {
	if ac.token == "" {
		return nil, fmt.Errorf("not authenticated: please login first")
	}

	payload := SubmitRequest{
		CommitSHA: commitSHA,
		Suite:     suite,
	}

	var resp SubmitResponse
	httpResp, err := ac.client.R().
		SetAuthToken(ac.token).
		SetBody(payload).
		SetResult(&resp).
		Post("/grade/submit")

	if err != nil {
		return nil, fmt.Errorf("submit request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusAccepted && httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("submit failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return &resp, nil
}

// StatusResponse represents the API status response
type StatusResponse struct {
	JobID     string       `json:"job_id"`
	Status    string       `json:"status"`
	Message   string       `json:"message"`
	Suite     string       `json:"suite,omitempty"`
	CommitSHA string       `json:"commit_sha,omitempty"`
	CreatedAt string       `json:"created_at"`
	Result    *GradeResult `json:"result,omitempty"`
}

// GradeResult represents the final grading result from the API
type GradeResult struct {
	ParserSuccess bool              `json:"parser_success"`
	BenchmarkMs   int               `json:"benchmark_ms"`
	FinalScore    int               `json:"final_score"`
	Details       string            `json:"details,omitempty"`
	Challenges    []ChallengeDetail `json:"challenges,omitempty"`
	CodeChecksum  string            `json:"code_checksum,omitempty"`
}

// ChallengeDetail represents a single challenge result from the API
type ChallengeDetail struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Passed      bool   `json:"passed"`
	Points      int    `json:"points"`
	TestsRun    int    `json:"tests_run"`
	TestsPassed int    `json:"tests_passed"`
	Details     string `json:"details,omitempty"`
}

// GetStatus retrieves the status of a grading job
func (ac *APIClient) GetStatus(jobID string) (*StatusResponse, error) {
	if ac.token == "" {
		return nil, fmt.Errorf("not authenticated: please login first")
	}

	var resp StatusResponse
	httpResp, err := ac.client.R().
		SetAuthToken(ac.token).
		SetPathParam("job_id", jobID).
		SetResult(&resp).
		Get("/grade/status/{job_id}")

	if err != nil {
		return nil, fmt.Errorf("status request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("status request failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return &resp, nil
}

// JobsListResponse represents a list of user jobs
type JobsListResponse struct {
	Jobs []StatusResponse `json:"jobs"`
}

// LeaderboardEntry represents a single entry on the leaderboard
type LeaderboardEntry struct {
	Username    string `json:"username"`
	Score       int    `json:"score"`
	BenchmarkMs int    `json:"benchmark_ms"`
	JobID       string `json:"job_id"`
	Rating      int    `json:"rating"`
}

// LeaderboardResponse represents the leaderboard response
type LeaderboardResponse struct {
	Entries []LeaderboardEntry `json:"entries"`
}

// GetLeaderboard retrieves top scorers for a given hackathon
func (ac *APIClient) GetLeaderboard(hackathon string) (*LeaderboardResponse, error) {
	var resp LeaderboardResponse
	httpResp, err := ac.client.R().
		SetPathParam("hackathon", hackathon).
		SetResult(&resp).
		Get("/grade/leaderboard/{hackathon}")

	if err != nil {
		return nil, fmt.Errorf("leaderboard request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("leaderboard request failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return &resp, nil
}

// UserInfoResponse represents the authenticated user's info
type UserInfoResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Rating   int    `json:"rating"`
}

// GetUserInfo retrieves the authenticated user's information
func (ac *APIClient) GetUserInfo() (*UserInfoResponse, error) {
	if ac.token == "" {
		return nil, fmt.Errorf("not authenticated: please login first")
	}

	var resp UserInfoResponse
	httpResp, err := ac.client.R().
		SetAuthToken(ac.token).
		SetResult(&resp).
		Get("/user/me")

	if err != nil {
		return nil, fmt.Errorf("user info request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("user info request failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return &resp, nil
}

// PlagiarismGroup represents a group of users with identical code
type PlagiarismGroup struct {
	Checksum  string   `json:"checksum"`
	UserCount int      `json:"user_count"`
	Users     []string `json:"users"`
	JobIDs    []string `json:"job_ids"`
}

// PlagiarismResponse represents the plagiarism check response
type PlagiarismResponse struct {
	Groups []PlagiarismGroup `json:"groups"`
}

// CheckPlagiarism retrieves groups of submissions with identical code
func (ac *APIClient) CheckPlagiarism(hackathon string) (*PlagiarismResponse, error) {
	var resp PlagiarismResponse
	httpResp, err := ac.client.R().
		SetPathParam("hackathon", hackathon).
		SetResult(&resp).
		Get("/grade/plagiarism/{hackathon}")

	if err != nil {
		return nil, fmt.Errorf("plagiarism check failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("plagiarism check failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return &resp, nil
}

// RepoLinkRequest represents a repo linking request
type RepoLinkRequest struct {
	RepoPath  string `json:"repo_path"`
	RemoteURL string `json:"remote_url,omitempty"`
}

// LinkRepo links the current git repository to the authenticated user
func (ac *APIClient) LinkRepo(repoPath, remoteURL string) error {
	if ac.token == "" {
		return fmt.Errorf("not authenticated: please login first")
	}

	payload := RepoLinkRequest{
		RepoPath:  repoPath,
		RemoteURL: remoteURL,
	}

	httpResp, err := ac.client.R().
		SetAuthToken(ac.token).
		SetBody(payload).
		Post("/grade/repo")

	if err != nil {
		return fmt.Errorf("repo link request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK && httpResp.StatusCode() != http.StatusCreated {
		return fmt.Errorf("repo link failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return nil
}

// SuitesListResponse represents the list of available test suites
type SuitesListResponse struct {
	Suites []SuiteInfo `json:"suites"`
}

// SuiteInfo represents a test suite with time window info
type SuiteInfo struct {
	Name     string `json:"name"`
	StartsAt string `json:"starts_at,omitempty"`
	EndsAt   string `json:"ends_at,omitempty"`
	Active   bool   `json:"active"`
	Message  string `json:"message,omitempty"`
}

// ListSuites retrieves available test suite names from the API
func (ac *APIClient) ListSuites() (*SuitesListResponse, error) {
	var resp SuitesListResponse
	httpResp, err := ac.client.R().
		SetResult(&resp).
		Get("/grade/suites")

	if err != nil {
		return nil, fmt.Errorf("list suites request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("list suites request failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return &resp, nil
}

// ChallengesResponse represents a list of challenges with subjects
type ChallengesResponse struct {
	Challenges []ChallengeInfo `json:"challenges"`
}

// ChallengeInfo represents a challenge's metadata and subject
type ChallengeInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Points  int    `json:"points"`
	Subject string `json:"subject"`
}

// GetChallenges retrieves challenge list and subjects for a suite
func (ac *APIClient) GetChallenges(suite string) (*ChallengesResponse, error) {
	var resp ChallengesResponse
	httpResp, err := ac.client.R().
		SetPathParam("suite", suite).
		SetResult(&resp).
		Get("/grade/suites/{suite}/challenges")

	if err != nil {
		return nil, fmt.Errorf("challenges request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("challenges request failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return &resp, nil
}

// GetJob retrieves a single job's full details (including result)
func (ac *APIClient) GetJob(jobID string) (*StatusResponse, error) {
	if ac.token == "" {
		return nil, fmt.Errorf("not authenticated: please login first")
	}

	var resp StatusResponse
	httpResp, err := ac.client.R().
		SetAuthToken(ac.token).
		SetPathParam("job_id", jobID).
		SetResult(&resp).
		Get("/grade/status/{job_id}")

	if err != nil {
		return nil, fmt.Errorf("get job request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("get job request failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return &resp, nil
}

// ListJobs retrieves all grading jobs for the authenticated user
func (ac *APIClient) ListJobs() (*JobsListResponse, error) {
	if ac.token == "" {
		return nil, fmt.Errorf("not authenticated: please login first")
	}

	var resp JobsListResponse
	httpResp, err := ac.client.R().
		SetAuthToken(ac.token).
		SetResult(&resp).
		Get("/grade/jobs")

	if err != nil {
		return nil, fmt.Errorf("list jobs request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("list jobs request failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	return &resp, nil
}

// WorkspaceDir returns the path to the user's workspace repo
func WorkspaceDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, "ft_hackthon", "workspace"), nil
}

// CloneGiteaRepo clones the user's Gitea repository into the workspace.
// Does not create any files — the caller is responsible for setting up
// ft_hackthon.yml and making the initial commit.
func CloneGiteaRepo(cloneURL string) (string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.GiteaToken != "" {
		cloneURL = strings.Replace(cloneURL, "://", fmt.Sprintf("://%s@", cfg.GiteaToken), 1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	ws := filepath.Join(home, "ft_hackthon", "workspace")

	if _, err := os.Stat(ws); err == nil {
		os.RemoveAll(ws)
	}

	cmd := exec.Command("git", "clone", cloneURL, ws)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to clone Gitea repo: %w\n%s", err, output)
	}

	return ws, nil
}

// InitWorkspaceRepo creates ft_hackthon.yml, configures git, stages everything,
// commits, and pushes. This is the initial setup for a new workspace.
func InitWorkspaceRepo(ws, suite string) error {
	// Create ft_hackthon.yml with the selected suite
	cfgPath := filepath.Join(ws, "ft_hackthon.yml")
	cfg := struct {
		Suite string `yaml:"suite"`
	}{Suite: suite}
	f, err := os.Create(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to create ft_hackthon.yml: %w", err)
	}
	enc := yaml.NewEncoder(f)
	enc.Encode(cfg)
	enc.Close()
	f.Close()

	gitName := os.Getenv("USER")
	gitEmail := os.Getenv("EMAIL")
	if gitEmail == "" {
		gitEmail = fmt.Sprintf("%s@localhost", gitName)
	}

	runGit := func(args ...string) ([]byte, error) {
		c := exec.Command("git", args...)
		c.Dir = ws
		return c.CombinedOutput()
	}

	if _, err := runGit("config", "user.name", gitName); err != nil {
		return fmt.Errorf("git config user.name failed: %w", err)
	}
	if _, err := runGit("config", "user.email", gitEmail); err != nil {
		return fmt.Errorf("git config user.email failed: %w", err)
	}
	if out, err := runGit("add", "-A"); err != nil {
		return fmt.Errorf("git add failed: %w\n%s", err, out)
	}
	if out, err := runGit("commit", "--allow-empty", "-m", "Initial setup"); err != nil {
		if !strings.Contains(string(out), "nothing to commit") {
			return fmt.Errorf("git commit failed: %w\n%s", err, out)
		}
	}
	if out, err := runGit("push", "origin", "HEAD"); err != nil {
		return fmt.Errorf("git push failed: %w\n%s", err, out)
	}
	return nil
}

// PushToGitea commits and pushes the workspace state to Gitea
func PushToGitea(wsDir string) (string, error) {
	return commitAndPush(wsDir)
}

func commitAndPush(wsDir string) (string, error) {
	// Configure git user from system env
	gitName := os.Getenv("USER")
	gitEmail := os.Getenv("EMAIL")
	if gitEmail == "" {
		gitEmail = fmt.Sprintf("%s@localhost", gitName)
	}

	gitCmds := [][]string{
		{"config", "user.name", gitName},
		{"config", "user.email", gitEmail},
		{"add", "-A"},
		{"commit", "--allow-empty", "-m", "Submission"},
		{"push", "origin", "HEAD"},
	}

	for _, args := range gitCmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = wsDir
		if output, err := cmd.CombinedOutput(); err != nil {
			if args[0] == "commit" && strings.Contains(string(output), "nothing to commit") {
				continue
			}
			return "", fmt.Errorf("git %s failed: %w\n%s", args[0], err, output)
		}
	}

	sha, err := GetGitCommitSHAInDir(wsDir)
	if err != nil {
		return "", err
	}
	return sha, nil
}

// GetGitCommitSHAInDir executes `git rev-parse HEAD` in a specific directory
func GetGitCommitSHAInDir(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git commit SHA: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentDir returns the current working directory
func GetCurrentDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	return dir, nil
}
