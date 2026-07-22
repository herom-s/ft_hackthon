# Complete System Operation Guide

## Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│                     FT_HACKTHON                          │
│           Automated Hackathon Grading System             │
└──────────────────────────────────────────────────────────┘
    │                  │                  │           │
    ▼                  ▼                  ▼           ▼
┌────────┐       ┌──────────┐       ┌──────────┐ ┌──────────┐
│  CLI   │       │   API    │       │  WORKER  │ │  GITEA   │
│ Client │       │  Server  │       │  Engine  │ │   Repo   │
│(HTTP)  │       │  (HTTP)  │       │(Polling) │ │(SSH/HTTP)│
└───┬────┘       └────┬─────┘       └─────┬────┘ └────┬─────┘
    │                 │                   │           │
    └─────────────────┼───────────────────┘           │
                      │                               │
              ┌───────▼────────┐                      │
              │   PostgreSQL   │◄─────────────────────┘
              │   Database     │
              └────────────────┘
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
Password: ••••••••••••
✓ Successfully logged in as alice@example.com
```

**System Flow:**
```
CLI
  └─ Prompt for credentials
  └─ POST /api/v1/auth/login {username, password}
       └─ API Handler receives request
       └─ Creates random token
       └─ Returns token
  └─ Save token to ~/.ft_hackthon/config.json
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
📦 Found git commit: a1b2c3d4e5f6a7b8c9d0
✓ Job ID: job-abc123def456

⏳ Waiting for grading to complete...
```

**System Flow:**

```
CLI
  ├─ Load token from config
  ├─ Verify git repo: git rev-parse --git-dir
  ├─ Get commit SHA: git rev-parse HEAD
  └─ POST /api/v1/grade/submit {commit_sha}
       └─ API Handler
            ├─ Extract user from token
            ├─ Create Job object
            │  ├─ ID: "job-abc123def456"
            │  ├─ UserID: "user_alice"
            │  ├─ CommitSHA: "a1b2c3d4e5f6a7b8c9d0"
            │  └─ Status: "queued"
            └─ Store in Database
                 └─ Return 202 Accepted with Job ID

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

#### Step 4: CLI Polls for Status

**System Flow:**

```
CLI (Polling Loop - every 2 seconds)

Attempt 1:
  └─ GET /api/v1/grade/status/job-abc123def456
       └─ API: Returns status = "queued"
  └─ Display: ⏳ STATUS: Queued - Waiting for grader availability...

Attempt 2:
  └─ GET /api/v1/grade/status/job-abc123def456
       └─ API: Returns status = "processing"
  └─ Display: ⚙ STATUS: Processing - Running benchmarks and tests...

Attempt 3-5:
  └─ (Continue polling, status still "processing")

Attempt 6:
  └─ GET /api/v1/grade/status/job-abc123def456
       └─ API: Returns status = "completed" with result
  └─ Display results and exit polling loop
```

#### Step 5: Worker Processes Job

**Worker Flow (Independent of CLI):**

```
Worker Polling Loop (every 5 seconds)

Poll 1:
  └─ GetPendingJobs(5)
       └─ Find job with status="queued" or "processing"
       └─ Fetch job-abc123def456

Job Found:
  ├─ Update job.Status = "processing"
  ├─ Update job.Message = "Running parser tests and benchmarks..."
  ├─ Save to Database
  └─ gradeProject(job):
       ├─ Create temp directory
       ├─ git clone <GiteaCloneURL> <tmpdir>/repo
       ├─ git checkout <CommitSHA>
       ├─ grader.Grade(workspaceDir, suite):
       │   ├─ Detect suite → load suite config
       │   ├─ Build test binary (gcc/make)
       │   ├─ Run tests, measure benchmark time
       │   └─ Calculate score, generate result
       └─ Return *database.Result

Update Elo Rating (if parser passed):
  ├─ Get user's current rating (default 1200)
  ├─ ComputeNewRating(currentRating, score)
  └─ Save updated rating

Save Result:
  ├─ job.Result = Result{
  │    ParserSuccess: true,
  │    BenchmarkMs: 145,
  │    FinalScore: 90,
  │    Details: "Grading Report: ✓ Parser: PASSED..."
  │  }
  ├─ job.Status = "completed"
  ├─ job.UpdatedAt = now()
  └─ Save to Database

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

#### Step 6: CLI Displays Results

**CLI Output:**
```
✓ STATUS: Completed!

═══════════════════════════════════════════════════
            GRADING RESULTS
═══════════════════════════════════════════════════

 Parser Success ........................... ✓ YES
 Benchmark Speed ......................... 145 ms
 Final Score ............................ 90 points

═══════════════════════════════════════════════════

Grading Report:
✓ Parser validation: PASSED
  - All test cases passed
  - Code structure: Valid
✓ Benchmark: 145 ms
  - Performance: Very Good

✓ Grading completed successfully!
```

---

## System Component Details

### API Server

**Startup:**
```bash
$ ./bin/ft_hackthon-api

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

**Responsibilities:**
- Accept HTTP requests
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

╔════════════════════════════════════════════╗
║   ft_hackthon Background Worker             ║
║   Starting job processor...                ║
╚════════════════════════════════════════════╝

✓ Worker is running and listening for jobs...
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
- Simulates 2-second grading per job
- Updates status in real-time

### CLI Client

**Commands:**
```
ft_hackthon login              # Authenticate
ft_hackthon register           # Create account
ft_hackthon grademe            # Submit project
ft_hackthon status [job_id]    # List jobs or check status
ft_hackthon logout             # Clear token
ft_hackthon whoami             # Show current user
ft_hackthon leaderboard <hack> # View top scorers
ft_hackthon submissions [ch]   # View submission history
ft_hackthon diff <job_id>      # View submitted code
ft_hackthon plagiarism <hack>  # Check for duplicates
ft_hackthon rating             # Show Elo rating
ft_hackthon version            # Show version
ft_hackthon help               # Show help
```

**Features:**
- Secure password input
- Token persistence
- Git integration
- Real-time polling
- Beautiful output formatting

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
      Message: "Waiting for grader availability",
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

### Method 2: Integration Test

```bash
bash scripts/integration-test.sh
```

### Method 3: API Testing with cURL

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

---

## Performance Characteristics

| Operation | Time |
|-----------|------|
| Login | ~100-200ms |
| Submit | ~100-200ms |
| Grading (real tests) | 10-60 seconds (varies) |
| Status poll | ~50-100ms |
| Full cycle (submit→complete) | 5-10 seconds |

---

## Production Deployment Checklist

- [x] PostgreSQL (already implemented via pgxpool)
- [x] Random 64-char hex tokens (not JWT, cryptographically secure)
- [x] bcrypt password hashing
- [x] Use environment variables for config
- [x] Implement rate limiting
- [x] Add request logging/tracing
- [x] Implement health checks (DB ping, component status response)
- [x] Add database migrations (numbered steps, schema_migrations tracking table)
- [x] Set up load balancing for API (nginx reverse proxy, upstream to api:8000, Docker DNS round-robin)
- [x] Scale worker with multiple instances (PostgreSQL SKIP LOCKED safe claiming, automatic stuck job recovery, WORKER_ID env var)
- [x] Implement job queue (PostgreSQL as queue with SKIP LOCKED — no Redis/RabbitMQ needed)
- [x] Add monitoring and alerts (Prometheus metrics endpoint, threshold-based alert checker, alert acknowledgment API)
- [x] Configure HTTPS/TLS (nginx SSL termination, self-signed cert generation, HTTP→HTTPS redirect, TLSv1.2/1.3)
- [x] Set up backup/recovery (automated PostgreSQL dump via backup service, retention pruning, restore script)

---

## Summary

The complete ft_hackthon system consists of three independent services:

1. **API Server** - Handles HTTP requests and manages database
2. **Worker Engine** - Processes jobs in the background
3. **CLI Client** - User interface for submission and monitoring

All three work together to provide a seamless, real-time grading experience.

**Start time**: Under 5 seconds
**Latency**: Under 200ms per request
**Throughput**: Limited by worker processing capacity
**Scalability**: Can add more worker instances
