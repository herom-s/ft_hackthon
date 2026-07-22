package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ft_hackthon/internal/database"
	"github.com/ft_hackthon/internal/gitea"
	"github.com/ft_hackthon/internal/grader"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// APIHandler wraps HTTP handlers with database access
type APIHandler struct {
	db         database.Database
	gitea      gitea.ClientInterface
	suitesPath string
}

// SetSuitesPath sets the path to testsuite directories
func (h *APIHandler) SetSuitesPath(path string) {
	h.suitesPath = path
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(db database.Database, opts ...gitea.ClientInterface) *APIHandler {
	var gc gitea.ClientInterface
	if len(opts) > 0 {
		gc = opts[0]
	} else {
		gc = gitea.NewClient()
	}
	return &APIHandler{
		db:    db,
		gitea: gc,
	}
}

// HealthHandler handles health check requests
func (h *APIHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dbErr := h.db.Ping()

	status := "ok"
	code := http.StatusOK
	if dbErr != nil {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	resp := map[string]interface{}{
		"status": status,
		"checks": map[string]string{
			"database": checkStatus(dbErr),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}

func checkStatus(err error) string {
	if err != nil {
		return "unhealthy: " + err.Error()
	}
	return "healthy"
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token         string `json:"token"`
	User          string `json:"user"`
	GiteaCloneURL string `json:"gitea_clone_url,omitempty"`
	GiteaToken    string `json:"gitea_token,omitempty"`
}

// LoginHandler handles user authentication
func (h *APIHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token := generateToken()

	resp := LoginResponse{
		Token: token,
		User:  req.Username,
	}

	h.db.CreateToken(token, user.ID)

	if user.GiteaRepoURL != "" {
		resp.GiteaCloneURL = h.gitea.PublicCloneURL(req.Username)
	}
	if user.GiteaToken != "" {
		resp.GiteaToken = user.GiteaToken
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterHandler handles user registration with Gitea provisioning
func (h *APIHandler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	if _, err := h.db.GetUserByUsername(req.Username); err == nil {
		http.Error(w, "Username already exists", http.StatusConflict)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to secure password")
		return
	}

	userID := generateID()
	user := &database.User{
		ID:       userID,
		Username: req.Username,
		Password: string(hashedPassword),
		Rating:   database.DefaultRating,
	}

	// Provision Gitea resources
	if h.gitea != nil && h.gitea.AdminToken() != "" {
		if err := h.gitea.CreateGiteaUser(req.Username, req.Password); err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := h.gitea.AddUserToOrg(req.Username); err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Failed to add user to organization: "+err.Error())
			return
		}

		repo, err := h.gitea.CreateUserRepo(req.Username)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Failed to create repository: "+err.Error())
			return
		}

		tokenResp, err := h.gitea.CreateUserToken(req.Username, req.Password)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Failed to create access token: "+err.Error())
			return
		}

		user.GiteaRepoURL = repo.CloneURL
		user.GiteaToken = tokenResp.SHA1
	}

	if err := h.db.CreateUser(user); err != nil {
		http.Error(w, "Error creating user", http.StatusInternalServerError)
		return
	}

	apiToken := generateToken()
	h.db.CreateToken(apiToken, userID)

	resp := LoginResponse{
		Token: apiToken,
		User:  req.Username,
	}
	if user.GiteaRepoURL != "" {
		resp.GiteaCloneURL = h.gitea.PublicCloneURL(req.Username)
	}
	if user.GiteaToken != "" {
		resp.GiteaToken = user.GiteaToken
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// ListSuitesHandler returns available test suite names with time windows
func (h *APIHandler) ListSuitesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.suitesPath == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]SuiteInfo{"suites": {}})
		return
	}

	entries, err := os.ReadDir(h.suitesPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]SuiteInfo{"suites": {}})
		return
	}

	var suites []SuiteInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := grader.LoadSuite(filepath.Join(h.suitesPath, e.Name()))
		if err != nil {
			continue
		}
		active, msg := s.IsActive(time.Now())
		info := SuiteInfo{
			Name:     e.Name(),
			StartsAt: s.StartsAt,
			EndsAt:   s.EndsAt,
			Active:   active,
			Message:  msg,
		}
		suites = append(suites, info)
	}

	if suites == nil {
		suites = []SuiteInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"suites": suites})
}

// SuiteInfo represents a test suite with time window info
type SuiteInfo struct {
	Name     string `json:"name"`
	StartsAt string `json:"starts_at,omitempty"`
	EndsAt   string `json:"ends_at,omitempty"`
	Active   bool   `json:"active"`
	Message  string `json:"message,omitempty"`
}

// ChallengeInfo represents a challenge's metadata and subject
type ChallengeInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Points  int    `json:"points"`
	Subject string `json:"subject"`
}

// ChallengesHandler returns challenges and subjects for a suite
func (h *APIHandler) ChallengesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	suite := strings.TrimPrefix(r.URL.Path, "/api/v1/grade/suites/")
	suite = strings.TrimSuffix(suite, "/challenges")
	if suite == "" || suite == r.URL.Path {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]ChallengeInfo{"challenges": {}})
		return
	}

	challengesDir := filepath.Join(h.suitesPath, suite, "challenges")
	entries, err := os.ReadDir(challengesDir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]ChallengeInfo{"challenges": {}})
		return
	}

	var challenges []ChallengeInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		chDir := filepath.Join(challengesDir, e.Name())

		// Read challenge.yml
		var c struct {
			Name   string `yaml:"name"`
			Title  string `yaml:"title"`
			Points int    `yaml:"points"`
		}
		cf, err := os.Open(filepath.Join(chDir, "challenge.yml"))
		if err != nil {
			continue
		}
		if err := yaml.NewDecoder(cf).Decode(&c); err != nil {
			cf.Close()
			continue
		}
		cf.Close()

		// Read subject.txt
		subject := ""
		if sub, err := os.ReadFile(filepath.Join(chDir, "subject.txt")); err == nil {
			subject = string(sub)
		}

		challenges = append(challenges, ChallengeInfo{
			Name:    c.Name,
			Title:   c.Title,
			Points:  c.Points,
			Subject: subject,
		})
	}

	if challenges == nil {
		challenges = []ChallengeInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"challenges": challenges})
}

// LeaderboardHandler returns top scorers for a given hackathon
func (h *APIHandler) LeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	suite := strings.TrimPrefix(r.URL.Path, "/api/v1/grade/leaderboard/")
	if suite == "" || suite == r.URL.Path {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"entries": []database.LeaderboardEntry{}})
		return
	}

	entries, err := h.db.GetLeaderboard(suite, 20)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"entries": []database.LeaderboardEntry{}, "error": err.Error()})
		return
	}

	if entries == nil {
		entries = []*database.LeaderboardEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"entries": entries})
}

// PlagiarismHandler returns groups of submissions with identical code
func (h *APIHandler) PlagiarismHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	suite := strings.TrimPrefix(r.URL.Path, "/api/v1/grade/plagiarism/")
	if suite == "" || suite == r.URL.Path {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"groups": []database.PlagiarismGroup{}})
		return
	}

	groups, err := h.db.CheckPlagiarism(suite)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"groups": []database.PlagiarismGroup{}, "error": err.Error()})
		return
	}

	if groups == nil {
		groups = []*database.PlagiarismGroup{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"groups": groups})
}

// SubmitRequest represents a grading submission request
type SubmitRequest struct {
	CommitSHA string `json:"commit_sha"`
	Suite     string `json:"suite,omitempty"`
}

// SubmitResponse represents a grading submission response
type SubmitResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

// SubmitHandler handles project submission for grading
func (h *APIHandler) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user from auth header
	userID := h.extractUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CommitSHA == "" {
		http.Error(w, "Commit SHA required", http.StatusBadRequest)
		return
	}

	// Check contest time window if suite is specified
	if req.Suite != "" && h.suitesPath != "" {
		s, err := grader.LoadSuite(filepath.Join(h.suitesPath, req.Suite))
		if err == nil {
			active, msg := s.IsActive(time.Now())
			if !active {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{
					"error":   "Submission period is closed",
					"message": msg,
				})
				return
			}
		}
	}

	// Look up user for Gitea clone URL
	user, _ := h.db.GetUser(userID)
	giteaCloneURL := ""
	if user != nil && user.GiteaRepoURL != "" && user.GiteaToken != "" {
		giteaCloneURL = strings.Replace(
			user.GiteaRepoURL,
			"://",
			fmt.Sprintf("://%s@", user.GiteaToken),
			1,
		)
	}

	// Create job
	jobID := generateID()
	job := &database.Job{
		ID:            jobID,
		UserID:        userID,
		CommitSHA:     req.CommitSHA,
		GiteaCloneURL: giteaCloneURL,
		Suite:         req.Suite,
		Status:        database.JobStatusQueued,
		Message:       "Waiting for grader availability",
	}

	if err := h.db.CreateJob(job); err != nil {
		http.Error(w, "Error creating job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(SubmitResponse{
		JobID:  jobID,
		Status: database.JobStatusQueued,
	})
}

// StatusResponse represents the job status response
type StatusResponse struct {
	JobID     string           `json:"job_id"`
	Status    string           `json:"status"`
	Message   string           `json:"message"`
	Suite     string           `json:"suite,omitempty"`
	CommitSHA string           `json:"commit_sha,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	Result    *database.Result `json:"result,omitempty"`
}

// StatusHandler handles job status requests
func (h *APIHandler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user from auth header
	userID := h.extractUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract job ID from URL
	jobID := strings.TrimPrefix(r.URL.Path, "/api/v1/grade/status/")

	// Get job
	job, err := h.db.GetJob(jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if job.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error     string    `json:"error"`
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// RespondWithError sends an error response
func RespondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	resp := ErrorResponse{
		Error:     message,
		Code:      fmt.Sprintf("ERR_%d", code),
		Timestamp: time.Now().UTC(),
	}

	json.NewEncoder(w).Encode(resp)
}

// JobsListResponse represents a list of jobs
type JobsListResponse struct {
	Jobs []StatusResponse `json:"jobs"`
}

// JobsListHandler returns all jobs for the authenticated user
func (h *APIHandler) JobsListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := h.extractUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	jobs, err := h.db.GetJobsByUser(userID)
	if err != nil {
		http.Error(w, "Error fetching jobs", http.StatusInternalServerError)
		return
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

	resp := JobsListResponse{Jobs: statusJobs}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// UserInfoResponse represents the authenticated user's info
type UserInfoResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Rating   int    `json:"rating"`
}

// UserInfoHandler returns the authenticated user's information
func (h *APIHandler) UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := h.extractUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	rating := user.Rating
	if rating == 0 {
		rating = database.DefaultRating
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserInfoResponse{
		ID:       user.ID,
		Username: user.Username,
		Rating:   rating,
	})
}

// RepoLinkRequest represents a repo linking request
type RepoLinkRequest struct {
	RepoPath  string `json:"repo_path"`
	RemoteURL string `json:"remote_url,omitempty"`
}

// RepoLinkHandler links a git repository to the authenticated user
func (h *APIHandler) RepoLinkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := h.extractUserID(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req RepoLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepoPath == "" {
		http.Error(w, "Repo path required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "linked",
		"repo_path": req.RepoPath,
		"user_id":   userID,
	})
}

// Helper functions

// generateToken generates a random JWT-like token
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random token: %v", err))
	}
	return hex.EncodeToString(b)
}

// generateID generates a random ID
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random ID: %v", err))
	}
	return hex.EncodeToString(b)
}

// extractUserID extracts user ID from Authorization header via DB token lookup
func (h *APIHandler) extractUserID(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	userID, err := h.db.GetUserIDByToken(parts[1])
	if err != nil {
		return ""
	}
	return userID
}
