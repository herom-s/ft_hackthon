# Complete System Operation Guide

## Architecture Overview

```
                     FT_HACKTHON
           Automated Hackathon Grading System
    |                  |                  |           |
    v                  v                  v           v
+--------+       +----------+       +----------+ +----------+
|  CLI   |       |   API    |       |  WORKER  | |  GITEA   |
| Client |       |  Server  |       |  Engine  | |   Repo   |
|(HTTP/WS)|      |  (HTTP)  |       |(Polling) | |(SSH/HTTP)|
+----+---+       +----+-----+       +-----+----+ +----+-----+
    |                 |                   |           |
    +-----------------+-------------------+           |
                      |                               |
              +-------v--------+                      |
              |   PostgreSQL   |<----------------------+
              |   Database     |
              +----------------+
```

## Running the Complete System

### Prerequisites

- Go 1.21+ (go.mod specifies 1.25)
- Make
- Git (for example with real repositories)
- curl (for API testing)

### Quick Start (Single Terminal)

```bash
# Build everything
cd <repo-dir>
make build

# In one terminal, start the API
./bin/ft_hackthon-api

# In another terminal, start the worker
./bin/ft_hackthon-worker

# In a third terminal, use the CLI
ft_hackthon login
ft_hackthon grademe
```

### Running with Make

```bash
# Terminal 1: API Server
make run-api

# Terminal 2: Worker
make run-worker

# Terminal 3: CLI
make run-cli ARGS="login"
make run-cli ARGS="grademe"
```

---

## Complete Flow Example

### Scenario: Student Submits Project for Grading

#### Step 1: Student Logs In

**User Command:**
```bash
$ ft_hackthon login
Username: alice@example.com
Password: ************

Authenticating...
+ Successfully logged in as alice@example.com
+ Token saved to ~/.ft_hackthon/config.json
```

**System Flow:**
```
CLI
  +- Prompt for credentials
  +- POST /api/v1/auth/login {username, password}
       +- API Handler receives request
       +- Creates random token
       +- Returns token
  +- Save token to ~/.ft_hackthon/config.json
```

**API Logs:**
```
127.0.0.1 POST /api/v1/auth/login
```

#### Step 2: Student Navigates to Project

**User Commands:**
```bash
$ cd ~/my-hackathon-project
$ git status
On branch main
nothing to commit, working tree clean
```

#### Step 3: Student Submits for Grading

**User Command:**
```bash
$ ft_hackthon grademe
Pushed commit: a1b2c3d4e5f6a7b8c9d0
+ Job ID: job-abc123def456

Waiting for grading to complete...

STATUS: Queued - Waiting for grader availability...
STATUS: Processing - Running benchmarks and tests...
STATUS: Completed!

==================================================
             GRADING RESULTS
==================================================

 Parser Success ........................... YES
 Benchmark Speed ......................... 145 ms
 Final Score ............................ 90 points

==================================================

Details:
+ Parser validation: PASSED
  - All test cases passed
  - Code structure: Valid
+ Benchmark: 145 ms
  - Performance: Very Good

+ Grading completed successfully!
```

**System Flow:**

```
CLI
  +- Load token from config
  +- Verify git repo: git rev-parse --git-dir
  +- Get commit SHA: git rev-parse HEAD
  +- POST /api/v1/grade/submit {commit_sha}
       +- API Handler
            +- Extract user from token
            +- Create Job object
            |  +- ID: "job-abc123def456"
            |  +- UserID: "user_alice"
            |  +- CommitSHA: "a1b2c3d4e5f6a7b8c9d0"
            |  +- Status: "queued"
            +- Store in Database
                 +- Return 202 Accepted with Job ID

Database Update:
  jobs["job-abc123def456"] = {
    ID: "job-abc123def456",
    UserID: "user_alice",
    CommitSHA: "a1b2c3d4e5f6a7b8c9d0",
    Status: "queued",
    Message: "Waiting for grader availability",
    CreatedAt: 2026-06-03T10:30:00Z
  }
```

**API Logs:**
```
127.0.0.1 POST /api/v1/grade/submit
```

#### Step 4: CLI Monitors Status

**Using WebSocket (primary):**
```
CLI connects to WS /ws/grade/status/job-abc123def456

Server pushes real-time updates:
  -> {"status": "queued"}
  -> Display: STATUS: Queued - Waiting for grader availability...
  -> {"status": "processing"}
  -> Display: STATUS: Processing - Running benchmarks and tests...
  -> {"status": "completed", "result": {...}}
  -> Display results
```

**Fallback HTTP polling (if WebSocket unavailable):**

```
CLI (Polling Loop - every 2 seconds)

Attempt 1:
  +- GET /api/v1/grade/status/job-abc123def456
       +- API: Returns status = "queued"
  +- Display: STATUS: Queued - Waiting for grader availability...

Attempt 2:
  +- GET /api/v1/grade/status/job-abc123def456
       +- API: Returns status = "processing"
  +- Display: STATUS: Processing - Running benchmarks and tests...

Attempt 3-5:
  +- (Continue polling, status still "processing")

Attempt 6:
  +- GET /api/v1/grade/status/job-abc123def456
       +- API: Returns status = "completed" with result
  +- Display results and exit polling loop
```

#### Step 5: Worker Processes Job

**Worker Flow (Independent of CLI):**

```
Worker Polling Loop (every 5 seconds)

Poll 1:
  +- GetPendingJobs(5)
       +- Find job with status="queued"
       +- Fetch job-abc123def456

Job Found:
  +- Update job.Status = "processing"
  +- Update job.Message = "Running parser tests and benchmarks..."
  +- Save to Database
  +- gradeProject(job):
       +- Create temp directory
       +- git clone <GiteaCloneURL> <tmpdir>/repo
       +- git checkout <CommitSHA>
       +- grader.Grade(workspaceDir, suite):
       |   +- Detect suite -> load suite config
       |   +- Build test binary (gcc/make)
       |   +- Run tests, measure benchmark time
       |   +- Calculate score, generate result
       +- Return *database.Result

Update Elo Rating (if parser passed):
  +- Get user's current rating (default 1200)
  +- ComputeNewRating(currentRating, score)
  +- Save updated rating

Save Result:
  +- job.Result = Result{
  |    ParserSuccess: true,
  |    BenchmarkMs: 145,
  |    FinalScore: 90,
  |    Details: "Grading Report: + Parser: PASSED..."
  |  }
  +- job.Status = "completed"
  +- job.UpdatedAt = now()
  +- Save to Database

Final Job State:
  jobs["job-abc123def456"] = {
    ID: "job-abc123def456",
    UserID: "user_alice",
    CommitSHA: "a1b2c3d4e5f6a7b8c9d0",
    Status: "completed",
    Result: {
      ParserSuccess: true,
      BenchmarkMs: 145,
      FinalScore: 90,
      Details: "..."
    },
    UpdatedAt: 2026-06-03T10:30:05Z
  }
```

**Worker Logs:**
```
Processing job: job-abc123def456 (commit: a1b2c3d4...)
Job completed: job-abc123def456 - Parser: true, Score: 90
```

---

## System Component Details

### API Server

**Startup:**
```bash
$ ./bin/ft_hackthon-api

+------------------------------------------+
|   ft_hackthon API Server                    |
|   Starting on :8000                        |
+------------------------------------------+

Available Endpoints:
  GET  /api/v1/health                - Health check
  POST /api/v1/auth/login            - Login
  POST /api/v1/auth/register         - Register
  POST /api/v1/grade/submit          - Submit project
  GET  /api/v1/grade/status/{job_id} - Get job status
  WS   /ws/grade/status/{job_id}     - Real-time job status
  WS   /ws/grade/jobs                 - Real-time jobs list
```

**Responsibilities:**
- Accept HTTP and WebSocket requests
- Validate authentication
- Create/update jobs in database
- Return status information
- Handle errors gracefully

**Database Connection:**
- Uses PostgreSQL (via DATABASE_URL env var)
- Connection pooling via pgxpool
- Auto-migration on startup

### Worker Engine

**Startup:**
```bash
$ ./bin/ft_hackthon-worker

+------------------------------------------+
|   ft_hackthon Background Worker             |
|   Starting job processor...                |
+------------------------------------------+

+ Worker is running and listening for jobs...
```

**Responsibilities:**
- Poll database for pending jobs (every 5 seconds)
- Clone repo from Gitea and checkout commit
- Execute real test suites via grader.Grade()
- Compute Elo rating updates
- Save results and update job status
- Handle graceful shutdown (Ctrl+C)

**Processing:**
- Checks database every 5 seconds
- Processes up to 5 jobs per cycle
- Updates status in real-time
- Circuit breaker for resilience

### CLI Client

**Commands:**
```
login              # Authenticate
register           # Create account
grademe            # Submit project
batch              # Submit multiple projects or all commits
status [job_id]    # List jobs or check status
submissions [ch]   # View submission history
diff <job_id>      # View submitted code
leaderboard <hack> # View top scorers
plagiarism <hack>  # Check for duplicates
report             # Submission analytics and trends
hooks              # Manage git hooks
logout             # Clear token
whoami             # Show current user
rating             # Show Elo rating
version            # Show version
help               # Show help
```

**Features:**
- Secure password input
- Token persistence
- Git integration
- Real-time monitoring (WebSocket)
- Batch submission
- Submission analytics
- Git hooks for automation
- CI/CD support (non-interactive, JSON output)

---

## Database State During Operation

### Initial State
```go
{
  users: {},
  jobs:  {}
}
```

### After Login
```go
{
  users: {
    "user_alice": {
      ID: "user_alice",
      Username: "alice@example.com",
      Password: "...",
      CreatedAt: 2026-06-03T10:00:00Z,
      UpdatedAt: 2026-06-03T10:00:00Z
    }
  },
  jobs: {}
}
```

### After Submission
```go
{
  users: { /* ... */ },
  jobs: {
    "job-abc123def456": {
      ID: "job-abc123def456",
      UserID: "user_alice",
      CommitSHA: "a1b2c3d4e5f6...",
      Status: "queued",
      Message: "Waiting for grader availability",
      Result: null,
      CreatedAt: 2026-06-03T10:30:00Z,
      UpdatedAt: 2026-06-03T10:30:00Z
    }
  }
}
```

### After Processing
```go
{
  users: { /* ... */ },
  jobs: {
    "job-abc123def456": {
      ID: "job-abc123def456",
      UserID: "user_alice",
      CommitSHA: "a1b2c3d4e5f6...",
      Status: "completed",
      Message: "",
      Result: {
        ParserSuccess: true,
        BenchmarkMs: 145,
        FinalScore: 90,
        Details: "..."
      },
      CreatedAt: 2026-06-03T10:30:00Z,
      UpdatedAt: 2026-06-03T10:30:05Z
    }
  }
}
```

---

## Testing the System

### Method 1: Manual End-to-End

```bash
# Terminal 1
make run-api

# Terminal 2
make run-worker

# Terminal 3
make run-cli ARGS="login"
make run-cli ARGS="grademe"
```

### Method 2: Batch Submission

```bash
# Submit multiple projects
./bin/ft_hackthon batch /path/to/project1 /path/to/project2

# Submit all commits of a project
./bin/ft_hackthon batch --all-commits .
```

### Method 3: Submission Analytics

```bash
# Show stats for last 30 days
./bin/ft_hackthon report

# Show stats with trend for last 7 days
./bin/ft_hackthon report --days=7 --trend
```

### Method 4: WebSocket Testing

```bash
# Using websocat tool
websocat "ws://localhost:8000/ws/grade/status/job-abc123?token=<token>"
```

### Method 5: API Testing with cURL

```bash
# Health check
curl http://localhost:8000/api/v1/health

# Login
curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"user","password":"pass"}'

# Submit
curl -X POST http://localhost:8000/api/v1/grade/submit \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"commit_sha":"abc123..."}'

# Check status
curl http://localhost:8000/api/v1/grade/status/<job_id> \
  -H "Authorization: Bearer <token>"
```

### Method 6: CI/CD Integration

```bash
# Non-interactive grading
./bin/ft_hackthon --non-interactive --insecure grademe

# JSON output for programmatic parsing
./bin/ft_hackthon --json --non-interactive --insecure status
```

---

## Troubleshooting

### Port 8000 Already in Use

```bash
# Find process on port 8000
lsof -i :8000

# Kill it
kill -9 <PID>
```

### Worker Not Processing Jobs

- Check if worker is running
- Verify API server is accessible
- Check worker logs for errors
- Restart both services

### API Returns 401 Unauthorized

- Ensure token is passed in Authorization header
- Format: `Authorization: Bearer <token>`
- Token must be from same session

### Database Seems Empty

- In-memory database resets on restart
- All data is lost
- Use persistent database for production

### WebSocket Connection Fails

- The CLI automatically falls back to HTTP polling
- Ensure the server supports WebSocket (check /ws/ endpoints)
- Verify token is valid

---

## Performance Characteristics

| Operation | Time |
|-----------|------|
| Login | ~100-200ms |
| Submit | ~100-200ms |
| Grading (real tests) | 10-60 seconds (varies) |
| Status poll | ~50-100ms |
| WebSocket update | ~10-50ms |
| Full cycle (submit->complete) | 5-10 seconds |

---

## Production Deployment Checklist

- [x] PostgreSQL (already implemented via pgxpool)
- [x] Random 64-char hex tokens (not JWT, cryptographically secure)
- [x] bcrypt password hashing
- [x] Use environment variables for config
- [x] Implement rate limiting
- [x] Add request logging/tracing
- [x] Implement health checks (DB ping, component status response)
- [x] Add database migrations
- [x] Set up load balancing for API (nginx reverse proxy)
- [x] Scale worker with multiple instances (PostgreSQL SKIP LOCKED)
- [x] Implement job queue (PostgreSQL as queue)
- [x] Add monitoring and alerts
- [x] Configure HTTPS/TLS
- [x] Set up backup/recovery
- [x] Add WebSocket support for real-time updates
- [x] Add CI/CD integration (JSON output, non-interactive mode, exit codes)
- [x] Add batch submission support
- [x] Add submission analytics and reports
- [x] Add git hooks for automation

---

## Summary

The complete ft_hackthon system consists of three independent services:

1. **API Server** - Handles HTTP/WebSocket requests and manages database
2. **Worker Engine** - Processes jobs in the background
3. **CLI Client** - User interface for submission, monitoring, batch ops, analytics, and automation

All three work together to provide a seamless, real-time grading experience.

**Start time**: Under 5 seconds
**Latency**: Under 200ms per request
**Throughput**: Limited by worker processing capacity
**Scalability**: Can add more worker instances
