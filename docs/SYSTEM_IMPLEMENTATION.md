# Database, Handler & Worker Implementation

## Overview

Complete implementation of the three core system layers:

1. **Database Layer** - Data models and persistence
2. **Handler Layer** - REST API endpoints (including WebSocket)
3. **Worker Layer** - Background job processing

---

## 1. Database Layer (`internal/database/database.go`)

### Data Models

#### User Model
```go
type User struct {
    ID           string    // Unique identifier
    Username     string    // Unique username
    Email        string    // Email address
    Password     string    // Hashed password (hidden in JSON)
    GiteaRepoURL string    // Gitea repository URL
    GiteaToken   string    // Gitea access token
    Rating       int       // Elo rating (default 1200)
    CreatedAt    time.Time // Creation timestamp
    UpdatedAt    time.Time // Last update timestamp
}
```

#### Job Model
```go
type Job struct {
    ID            string    // Unique job ID
    UserID        string    // Owner user ID
    CommitSHA     string    // Git commit hash
    GiteaCloneURL string    // Gitea clone URL for the repo
    Suite         string    // Test suite name
    Status        string    // queued|processing|completed|failed|error
    Message       string    // Status message
    Result        *Result   // Grading result (if completed)
    CreatedAt     time.Time // Submission time
    UpdatedAt     time.Time // Last update time
}
```

#### Result Model
```go
type Result struct {
    ParserSuccess bool              // Whether parser passed
    BenchmarkMs   int               // Execution time in ms
    FinalScore    int               // Final score (0-100)
    Details       string            // Detailed grading report
    Challenges    []ChallengeDetail // Per-challenge results
    CodeChecksum  string            // SHA256 hash of submitted code
}

type ChallengeDetail struct {
    Name        string // Challenge name
    Title       string // Display title
    Passed      bool   // Whether challenge passed
    Points      int    // Points earned
    TestsRun    int    // Total tests run
    TestsPassed int    // Tests passed
    Details     string // Per-challenge details
}
```

#### LeaderboardEntry Model
```go
type LeaderboardEntry struct {
    Username    string // User display name
    Score       int    // Best score for the suite
    BenchmarkMs int    // Best benchmark time
    JobID       string // Job ID for the best result
    Rating      int    // User's Elo rating
}
```

#### PlagiarismGroup Model
```go
type PlagiarismGroup struct {
    Checksum  string   // Code checksum (SHA256)
    UserCount int      // Number of users sharing this checksum
    Users     []string // Usernames
    JobIDs    []string // Associated job IDs
}
```

### Database Interface

```go
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

    // Health
    Ping() error
    Close() error
}
```

### InMemoryDB Implementation

Thread-safe in-memory implementation using sync.RWMutex:

- **User Management**
  - Create, retrieve, update, delete users
  - Unique username checking
  - Timestamp management

- **Job Management**
  - Create jobs in queued status
  - Query jobs by ID or user
  - Get pending jobs for worker processing
  - Update job status

- **Result Storage**
  - Save results and mark job as completed
  - Retrieve results for display

**Usage:**
```go
db := database.NewInMemoryDB()
defer db.Close()

// Create user
user := &database.User{ID: "123", Username: "john"}
db.CreateUser(user)

// Create job
job := &database.Job{ID: "job-1", UserID: "123", CommitSHA: "abc123"}
db.CreateJob(job)
```

---

## 2. Handler Layer (`internal/handler/handler.go` + `websocket.go`)

### APIHandler Structure

```go
type APIHandler struct {
    db    database.Database
    gitea gitea.ClientInterface
    suitesPath string
}
```

### Implemented REST Endpoints

#### Health Check
```
GET /api/v1/health
Response: {"status": "ok"}
```

#### Login
```
POST /api/v1/auth/login
Request:  {"username": "user", "password": "pass"}
Response: {"token": "...", "user": "user"}
Status:   200 OK
```

#### Register
```
POST /api/v1/auth/register
Request:  {"username": "user", "password": "pass"}
Response: {"token": "...", "user": "user", "gitea_clone_url": "...", "gitea_token": "..."}
Status:   201 Created
```

#### Submit Project
```
POST /api/v1/grade/submit
Headers:  Authorization: Bearer <token>
Request:  {"commit_sha": "abc123..."}
Response: {"job_id": "job-uuid", "status": "queued"}
Status:   202 Accepted
```

#### Check Status
```
GET /api/v1/grade/status/{job_id}
Headers:  Authorization: Bearer <token>
Response: {
    "job_id": "job-uuid",
    "status": "processing",
    "message": "Running tests...",
    "result": null
}
Status:   200 OK
```

### WebSocket Endpoints

#### Real-time Job Status
```
WS /ws/grade/status/{job_id}?token=<token>
```

Pushes `StatusResponse` JSON messages on status changes. Closes when job completes.

#### Real-time Jobs List
```
WS /ws/grade/jobs?token=<token>
```

Pushes `JobsListResponse` JSON messages every 5 seconds.

### Handler Features

- **Request Validation**
  - Required field checking
  - Format validation
  - Content-Type enforcement

- **Error Handling**
  - HTTP status codes
  - Structured error responses
  - User-friendly messages

- **Authentication**
  - Bearer token extraction
  - User ID lookup (via DB)
  - Ownership verification

- **Token Generation**
  - Cryptographically secure random tokens
  - 64-character hex format

**Usage:**
```go
db := database.NewInMemoryDB()
handler := handler.NewAPIHandler(db)

http.HandleFunc("/api/v1/auth/login", handler.LoginHandler)
http.HandleFunc("/api/v1/grade/submit", handler.SubmitHandler)
http.HandleFunc("/api/v1/grade/status/", handler.StatusHandler)
http.HandleFunc("/ws/", handler.WSEndpoint(handler))
```

---

## 3. Worker Layer (`internal/worker/worker.go`)

### Worker Structure

```go
type Worker struct {
    db           database.Database
    jobChannel   chan *database.Job
    done         chan bool
    pollInterval time.Duration
}
```

### Job Processing Flow

```
Start Worker
    |
Poll Database (every 5 seconds)
    |
Get Pending Jobs (max 5 at a time)
    |
For Each Job:
    +- Update status to "processing"
    +- gradeProject():
    |   +- Clone repo from Gitea (git clone)
    |   +- Checkout specific commit
    |   +- grader.Grade(): detect suite -> build -> run tests
    |   +- Convert result to database format
    +- Save result to database
    +- Update Elo rating (if parser passed)
    |   +- Get current user rating
    |   +- ComputeNewRating() based on score
    |   +- Save updated rating
    +- Job marked "completed"
```

### Grading Logic

**Score Calculation:**
- Score is based on challenge completion (sum of points from passed challenges)
- Benchmark time affects per-challenge points
- See `CalculateScore()` and `RatingFromBenchmark()` in the grader package

**Elo Rating:**
- Default rating: 1200
- K-factor: 32
- Rating adjusts based on final score relative to expected score
- Minimum rating: 100
- Only updated when parser passes

**Result Generation:**
```go
type Result struct {
    ParserSuccess: true/false            // From grader.Grade()
    BenchmarkMs:   150                   // Measured from test execution
    FinalScore:    95                    // Calculated from challenges
    Details:       "Grading Report: ..." // Detailed report string
    CodeChecksum:  "sha256-hash"         // SHA256 of submitted code
    Challenges:    []ChallengeResult     // Per-challenge breakdown
}
```

### Worker Operations

**Start Processing:**
```go
w := worker.NewWorker(db)
w.Start()  // Begins background polling
```

**Graceful Shutdown:**
```go
w.Stop()   // Stops processing, closes channels
```

**Job Processing:**
- Automatically polls database
- Processes up to 5 jobs per cycle
- Updates job status in real-time
- Generates detailed reports

**Features:**
- Thread-safe operation
- Graceful signal handling (Ctrl+C)
- Configurable poll interval
- Job batching
- Detailed logging
- Circuit breaker (3-state: closed/half-open/open)

---

## Integration with API Server

### API Server Startup (`cmd/api/main.go`)

```
1. Read DATABASE_URL from environment (required)
2. Initialize PostgreSQL connection (database.NewPostgresDB)
3. Create APIHandler
4. Optionally set TESTSUITES_PATH
5. Register 14 routes (REST + WebSocket)
6. Start HTTP Server on :8000
```

**Routes Registered:**
```
GET  /api/v1/health
POST /api/v1/auth/login
POST /api/v1/auth/register
POST /api/v1/grade/submit
GET  /api/v1/grade/status/{job_id}
GET  /api/v1/grade/jobs
POST /api/v1/grade/repo
GET  /api/v1/grade/suites
GET  /api/v1/grade/suites/{suite}/challenges
GET  /api/v1/grade/leaderboard/{hackathon}
GET  /api/v1/grade/plagiarism/{hackathon}
GET  /api/v1/user/me
WS   /ws/grade/status/{job_id}
WS   /ws/grade/jobs
```

### Worker Engine (`cmd/worker/main.go`)

```
1. Initialize Database (same as API)
2. Create Worker
3. Start Background Processing
4. Wait for Interrupt Signal (Ctrl+C)
5. Graceful Shutdown
```

---

## Data Flow Example

### User Submits Project

```
CLI (ft_hackthon)
    | POST /api/v1/grade/submit
API Handler
    | CreateJob(job)
Database
    | Creates job with status="queued"

    Waits pollInterval (5 seconds by default)

Worker Polling Loop
    | GetPendingJobs()
Database
    | Returns job
Worker
    +- Updates job status="processing"
    +- gradeProject(): clone repo, checkout commit, run grader.Grade()
    +- Update Elo rating (if parser passed)
    +- SaveResult(jobID, result)
Database
    | Marks job as completed

CLI Monitors (WebSocket preferred, HTTP polling fallback)
    | WS /ws/grade/status/{job_id}
    | or GET /api/v1/grade/status/{job_id}
API Handler
    | GetJob() from database
Database
    | Returns job with completed status + results
CLI
    | Displays results to user
```

---

## Production Considerations

### Database Layer
- [x] Using PostgreSQL (production-ready)
- [x] Connection pooling via pgxpool
- [x] Auto-migration on startup (CREATE TABLE IF NOT EXISTS + numbered migrations with schema_migrations table)
- [x] Add indexes on frequently queried fields

### Handler Layer
- [x] Password hashing with bcrypt
- [x] Random 64-char hex tokens
- [x] Bearer token authentication
- [x] Add rate limiting
- [x] Add request logging/tracing
- [x] Implement CORS
- [x] WebSocket support for real-time updates

### Worker Layer
- [x] Real test execution via grader.Grade()
- [x] Elo rating computation
- [x] Graceful shutdown
- [x] PostgreSQL SKIP LOCKED job claiming
- [x] Multi-worker support
- [x] Circuit breaker
- [x] Job retry logic (up to 3 attempts)
- [x] Job timeout (5-minute)
- [x] Worker health monitoring

---

## Testing the System

### Start API Server
```bash
go run cmd/api/main.go
```

### Start Worker
```bash
go run cmd/worker/main.go
```

### Submit Project (CLI)
```bash
ft_hackthon login
cd ~/my-project
ft_hackthon grademe
```

### Batch Submission
```bash
ft_hackthon batch ../project1 ../project2
ft_hackthon batch --all-commits .
```

### Submission Analytics
```bash
ft_hackthon report --trend --days=30
```

### Monitor System
- API logs incoming requests
- Worker logs job processing
- Database maintains job state
- Results displayed in real-time (WebSocket)

---

## Architecture Benefits

[x] **Separation of Concerns** - Database, API, and processing are decoupled
[x] **Scalability** - Worker can run on separate machines
[x] **Reliability** - In-memory DB can be replaced with persistent store
[x] **Testability** - Easy to mock database for testing
[x] **Extensibility** - Simple to add new handlers or grading logic

---

## Summary

### Database: 250+ lines
- User management
- Job lifecycle
- Result storage
- Thread-safe operations

### Handlers: 350+ lines
- 12 REST endpoints
- 2 WebSocket endpoints
- Request validation
- Error handling
- Authentication

### Worker: 200+ lines
- Job polling
- Grading execution
- Result generation
- Status management
- Circuit breaker

**Total**: 800+ lines of fully functional production-grade code
