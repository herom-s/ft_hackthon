package database

import (
	"fmt"
	"sync"
	"time"
)

// User represents a user in the system
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Password     string    `json:"-"` // Never expose password
	GiteaRepoURL string    `json:"gitea_repo_url,omitempty"`
	GiteaToken   string    `json:"-"`
	Rating       int       `json:"rating"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const DefaultRating = 1200

// Job represents a grading job
type Job struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	CommitSHA     string    `json:"commit_sha"`
	GiteaCloneURL string    `json:"gitea_clone_url,omitempty"`
	Suite         string    `json:"suite,omitempty"`
	Status        string    `json:"status"` // queued, processing, completed, failed
	Message       string    `json:"message"`
	Result        *Result   `json:"result,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Result represents the grading result
type Result struct {
	ParserSuccess bool              `json:"parser_success"`
	BenchmarkMs   int               `json:"benchmark_ms"`
	FinalScore    int               `json:"final_score"`
	Details       string            `json:"details,omitempty"`
	Challenges    []ChallengeDetail `json:"challenges,omitempty"`
	CodeChecksum  string            `json:"code_checksum,omitempty"`
}

// ChallengeDetail represents a single challenge result
type ChallengeDetail struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Passed      bool   `json:"passed"`
	Points      int    `json:"points"`
	TestsRun    int    `json:"tests_run"`
	TestsPassed int    `json:"tests_passed"`
	BenchmarkMs int    `json:"benchmark_ms"`
	Details     string `json:"details,omitempty"`
}

// LeaderboardEntry represents a user's score on the leaderboard
type LeaderboardEntry struct {
	Username    string `json:"username"`
	Score       int    `json:"score"`
	BenchmarkMs int    `json:"benchmark_ms"`
	JobID       string `json:"job_id"`
	Rating      int    `json:"rating"`
}

// JobStatus constants
const (
	JobStatusQueued     = "queued"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
	JobStatusError      = "error"
)

// Database interface defines database operations
type Database interface {
	// User operations
	CreateUser(user *User) error
	GetUser(userID string) (*User, error)
	GetUserByUsername(username string) (*User, error)
	UpdateUser(user *User) error
	DeleteUser(userID string) error

	// Job operations
	CreateJob(job *Job) error
	GetJob(jobID string) (*Job, error)
	GetJobsByUser(userID string) ([]*Job, error)
	UpdateJob(job *Job) error
	GetPendingJobs(limit int) ([]*Job, error)
	ClaimJobs(workerID string, limit int) ([]*Job, error)
	ReleaseStuckJobs(timeoutMinutes int) (int, error)

	// Result operations
	SaveResult(jobID string, result *Result) error
	GetResult(jobID string) (*Result, error)

	// Token operations
	CreateToken(token, userID string) error
	GetUserIDByToken(token string) (string, error)
	DeleteToken(token string) error

	// Leaderboard
	GetLeaderboard(suite string, limit int) ([]*LeaderboardEntry, error)

	// Rating
	UpdateRating(userID string, newRating int) error
	GetSuiteScores(suite string) ([]int, error)

	// Plagiarism
	CheckPlagiarism(suite string) ([]*PlagiarismGroup, error)

	// Health check
	Ping() error
	Close() error
}

// PlagiarismGroup represents a group of submissions with the same checksum
type PlagiarismGroup struct {
	Checksum  string   `json:"checksum"`
	UserCount int      `json:"user_count"`
	Users     []string `json:"users"`
	JobIDs    []string `json:"job_ids"`
}

// InMemoryDB is an in-memory implementation of Database
type InMemoryDB struct {
	mu     sync.RWMutex
	users  map[string]*User
	jobs   map[string]*Job
	tokens map[string]string // token -> userID
}

// NewInMemoryDB creates a new in-memory database
func NewInMemoryDB() *InMemoryDB {
	return &InMemoryDB{
		users:  make(map[string]*User),
		jobs:   make(map[string]*Job),
		tokens: make(map[string]string),
	}
}

// User operations
func (db *InMemoryDB) CreateUser(user *User) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if user.ID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	if _, exists := db.users[user.ID]; exists {
		return fmt.Errorf("user already exists")
	}

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	db.users[user.ID] = user
	return nil
}

func (db *InMemoryDB) GetUser(userID string) (*User, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	user, exists := db.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

func (db *InMemoryDB) GetUserByUsername(username string) (*User, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for _, user := range db.users {
		if user.Username == username {
			return user, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}

func (db *InMemoryDB) UpdateUser(user *User) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.users[user.ID]; !exists {
		return fmt.Errorf("user not found")
	}

	user.UpdatedAt = time.Now()
	db.users[user.ID] = user
	return nil
}

func (db *InMemoryDB) DeleteUser(userID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.users[userID]; !exists {
		return fmt.Errorf("user not found")
	}

	delete(db.users, userID)
	return nil
}

// Job operations
func (db *InMemoryDB) CreateJob(job *Job) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if job.ID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if _, exists := db.jobs[job.ID]; exists {
		return fmt.Errorf("job already exists")
	}

	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	job.Status = JobStatusQueued

	db.jobs[job.ID] = job
	return nil
}

func (db *InMemoryDB) GetJob(jobID string) (*Job, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	job, exists := db.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found")
	}

	return job, nil
}

func (db *InMemoryDB) GetJobsByUser(userID string) ([]*Job, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var jobs []*Job
	for _, job := range db.jobs {
		if job.UserID == userID {
			jobs = append(jobs, job)
		}
	}

	return jobs, nil
}

func (db *InMemoryDB) UpdateJob(job *Job) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.jobs[job.ID]; !exists {
		return fmt.Errorf("job not found")
	}

	job.UpdatedAt = time.Now()
	db.jobs[job.ID] = job
	return nil
}

func (db *InMemoryDB) GetPendingJobs(limit int) ([]*Job, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var jobs []*Job
	for _, job := range db.jobs {
		if job.Status == JobStatusQueued || job.Status == JobStatusProcessing {
			jobs = append(jobs, job)
			if len(jobs) >= limit {
				break
			}
		}
	}

	return jobs, nil
}

// Result operations
func (db *InMemoryDB) GetLeaderboard(suite string, limit int) ([]*LeaderboardEntry, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	best := make(map[string]*LeaderboardEntry)
	for _, job := range db.jobs {
		if job.Suite != suite || job.Status != JobStatusCompleted || job.Result == nil {
			continue
		}
		existing, ok := best[job.UserID]
		if !ok || job.Result.FinalScore > existing.Score {
			user, _ := db.GetUser(job.UserID)
			username := job.UserID
			rating := DefaultRating
			if user != nil {
				username = user.Username
				if user.Rating > 0 {
					rating = user.Rating
				}
			}
			best[job.UserID] = &LeaderboardEntry{
				Username:    username,
				Score:       job.Result.FinalScore,
				BenchmarkMs: job.Result.BenchmarkMs,
				JobID:       job.ID,
				Rating:      rating,
			}
		}
	}

	entries := make([]*LeaderboardEntry, 0, len(best))
	for _, e := range best {
		entries = append(entries, e)
	}

	// Sort by score descending
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Score > entries[i].Score {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func (db *InMemoryDB) SaveResult(jobID string, result *Result) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	job, exists := db.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found")
	}

	job.Result = result
	job.Status = JobStatusCompleted
	job.UpdatedAt = time.Now()
	return nil
}

func (db *InMemoryDB) GetResult(jobID string) (*Result, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	job, exists := db.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found")
	}

	if job.Result == nil {
		return nil, fmt.Errorf("no result available")
	}

	return job.Result, nil
}

// Token operations
func (db *InMemoryDB) CreateToken(token, userID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.tokens[token] = userID
	return nil
}

func (db *InMemoryDB) GetUserIDByToken(token string) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	userID, exists := db.tokens[token]
	if !exists {
		return "", fmt.Errorf("token not found")
	}
	return userID, nil
}

func (db *InMemoryDB) DeleteToken(token string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.tokens, token)
	return nil
}

// Ping checks database connectivity
func (db *InMemoryDB) Ping() error {
	return nil // In-memory DB is always available
}

func (db *InMemoryDB) ClaimJobs(workerID string, limit int) ([]*Job, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var claimed []*Job
	for _, job := range db.jobs {
		if job.Status != JobStatusQueued {
			continue
		}
		now := time.Now()
		job.Status = JobStatusProcessing
		job.Message = "Processing..."
		job.UpdatedAt = now
		claimed = append(claimed, job)
		if len(claimed) >= limit {
			break
		}
	}
	return claimed, nil
}

func (db *InMemoryDB) ReleaseStuckJobs(timeoutMinutes int) (int, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	count := 0
	for _, job := range db.jobs {
		if job.Status == JobStatusProcessing && time.Since(job.UpdatedAt) > time.Duration(timeoutMinutes)*time.Minute {
			job.Status = JobStatusQueued
			job.UpdatedAt = time.Now()
			count++
		}
	}
	return count, nil
}

// UpdateRating updates a user's Elo rating
func (db *InMemoryDB) UpdateRating(userID string, newRating int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	user, exists := db.users[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	user.Rating = newRating
	user.UpdatedAt = time.Now()
	return nil
}

// GetSuiteScores returns all final scores for completed jobs in a suite
func (db *InMemoryDB) GetSuiteScores(suite string) ([]int, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var scores []int
	for _, job := range db.jobs {
		if job.Suite == suite && job.Status == JobStatusCompleted && job.Result != nil {
			scores = append(scores, job.Result.FinalScore)
		}
	}
	return scores, nil
}

// CheckPlagiarism groups completed jobs in a suite by code checksum
func (db *InMemoryDB) CheckPlagiarism(suite string) ([]*PlagiarismGroup, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	groups := make(map[string]*PlagiarismGroup)
	for _, job := range db.jobs {
		if job.Suite != suite || job.Status != JobStatusCompleted || job.Result == nil || job.Result.CodeChecksum == "" {
			continue
		}
		cs := job.Result.CodeChecksum
		g, ok := groups[cs]
		if !ok {
			g = &PlagiarismGroup{Checksum: cs}
			groups[cs] = g
		}
		user, _ := db.GetUser(job.UserID)
		username := job.UserID
		if user != nil {
			username = user.Username
		}
		g.Users = append(g.Users, username)
		g.JobIDs = append(g.JobIDs, job.ID)
		g.UserCount = len(g.Users)
	}

	result := make([]*PlagiarismGroup, 0, len(groups))
	for _, g := range groups {
		if g.UserCount >= 2 {
			result = append(result, g)
		}
	}
	return result, nil
}

// Close closes the database connection
func (db *InMemoryDB) Close() error {
	return nil // In-memory DB doesn't need to close
}
