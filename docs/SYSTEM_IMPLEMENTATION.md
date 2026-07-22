# Database, Handler & Worker Implementation

## Overview

Complete implementation of the three core system layers:

1. **Database Layer** - Data models and persistence
2. **Handler Layer** - REST API endpoints
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
    BenchmarkMs int    // Challenge execution time
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

## 2. Handler Layer (`internal/handler/handler.go`)

### APIHandler Structure

```go
type APIHandler struct {
    db database.Database
}
```

### Implemented Endpoints

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
Response: {"token": "...", "user": "user"}
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
  - User ID lookup
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
    ↓
Poll Database (every 5 seconds)
    ↓
Get Pending Jobs (max 5 at a time)
    ↓
For Each Job:
    ├─ Update status to "processing"
    ├─ gradeProject():
    │   ├─ Clone repo from Gitea (git clone)
    │   ├─ Checkout specific commit
    │   ├─ grader.Grade(): detect suite → build → run tests
    │   └─ Convert result to database format
    ├─ Save result to database
    ├─ Update Elo rating (if parser passed)
    │   ├─ Get current user rating
    │   ├─ ComputeNewRating() based on score
    │   └─ Save updated rating
    └─ Job marked "completed"
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

---

## Integration with API Server

### API Server Startup (`cmd/api/main.go`)

```
1. Read DATABASE_URL from environment (required)
2. Initialize PostgreSQL connection (database.NewPostgresDB)
3. Create APIHandler
4. Optionally set TESTSUITES_PATH
5. Register 12 routes
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
    ↓ POST /api/v1/grade/submit
API Handler
    ↓ CreateJob(job)
Database
    ↓ Creates job with status="queued"
    
Waits pollInterval (5 seconds by default)
    
Worker Polling Loop
    ↓ GetPendingJobs()
Database
    ↓ Returns job
Worker
    ├─ Updates job status="processing"
    ├─ gradeProject(): clone repo, checkout commit, run grader.Grade()
    ├─ Update Elo rating (if parser passed)
    └─ SaveResult(jobID, result)
Database
    ↓ Marks job as completed
    
CLI Polls Status
    ↓ GET /api/v1/grade/status/{job_id}
API Handler
    ↓ GetJob() from database
Database
    ↓ Returns job with completed status + results
CLI
    ↓ Displays results to user
```

---

## Production Considerations

### Database Layer
- ✅ Using PostgreSQL (production-ready)
- ✅ Connection pooling via pgxpool
- ✅ Auto-migration on startup (CREATE TABLE IF NOT EXISTS + numbered migrations with schema_migrations table)
- [x] Add indexes on frequently queried fields (idx_jobs_user_id, idx_jobs_status, idx_jobs_created_at, idx_jobs_suite, idx_tokens_user_id, idx_users_username)

### Handler Layer
- ✅ Password hashing with bcrypt (`golang.org/x/crypto/bcrypt`)
- ✅ Random 64-char hex tokens (cryptographically secure)
- ✅ Bearer token authentication
- [x] Add rate limiting (token-bucket, 100 req/min per user/IP)
- [x] Add request logging/tracing (structured slog JSON, status, duration, request ID)
- [x] Implement CORS (Access-Control-Allow-Origin: *, OPTIONS preflight)

### Worker Layer
- ✅ Real test execution via grader.Grade()
- ✅ Elo rating computation
- ✅ Graceful shutdown (signal handling)
- [x] Use message queue (PostgreSQL SKIP LOCKED — no Redis/RabbitMQ needed)
- [x] Distribute across multiple workers (atomic ClaimJobs, ReleaseStuckJobs recovery, WORKER_ID identity)
- [x] Implement circuit breaker (3-state: closed/half-open/open, configurable threshold + reset)
- [x] Add job retry logic (up to 3 attempts with exponential backoff)
- [x] Implement job timeout (5-minute per-attempt deadline)
- [x] Add worker health monitoring (Prometheus metrics + alert checker)

---

## Testing the System

### Start API Server
```bash
go run cmd/api/main.go
```

Output:
```
╔════════════════════════════════════════════╗
║   ft_hackthon API Server                    ║
║   Starting on :8000                        ║
╚════════════════════════════════════════════╝

Available Endpoints:
  GET  /api/v1/health                - Health check
  POST /api/v1/auth/login            - Login
  POST /api/v1/auth/register         - Register
  POST /api/v1/grade/submit          - Submit project
  GET  /api/v1/grade/status/{job_id} - Get job status
```

### Start Worker
```bash
go run cmd/worker/main.go
```

Output:
```
╔════════════════════════════════════════════╗
║   ft_hackthon Background Worker             ║
║   Starting job processor...                ║
╚════════════════════════════════════════════╝

✓ Worker is running and listening for jobs...
```

### Submit Project (CLI)
```bash
ft_hackthon login
cd ~/my-project
ft_hackthon grademe
```

### Monitor System
- API logs incoming requests
- Worker logs job processing
- Database maintains job state
- Results displayed in real-time

---

## Architecture Benefits

✅ **Separation of Concerns** - Database, API, and processing are decoupled
✅ **Scalability** - Worker can run on separate machines
✅ **Reliability** - In-memory DB can be replaced with persistent store
✅ **Testability** - Easy to mock database for testing
✅ **Extensibility** - Simple to add new handlers or grading logic

---

## Summary

### Database: 250+ lines
- User management
- Job lifecycle
- Result storage
- Thread-safe operations

### Handlers: 300+ lines
- 5 REST endpoints
- Request validation
- Error handling
- Authentication

### Worker: 200+ lines
- Job polling
- Grading simulation
- Result generation
- Status management

**Total**: 750+ lines of fully functional production-grade code
