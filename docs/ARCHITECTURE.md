# ft_hackthon CLI Architecture

## Overview

The `ft_hackthon` CLI is designed to provide a seamless, interactive experience for hackathon participants submitting their projects for automated grading. It supports both an interactive REPL and a non-interactive CI/CD mode.

## Components

### 1. Command Layer (REPL + Cobra)

Located in `cmd/ft_hackthon/repl.go` and `cmd/ft_hackthon/main.go`, the command layer provides:

- **Interactive REPL** - Tab completion, history, aliases, per-command help
- **Non-interactive mode** - Direct command execution for CI/CD pipelines
- **Flag Management** - Global flags (--api-url, --insecure, --verbose, --json, --quiet, --non-interactive)
- **Help Generation** - Per-command help with usage and examples

**Available Commands:**
- `login` - Authenticate with credentials
- `register` - Create a new account (prompts for hackathon selection)
- `grademe` - Submit project for grading
- `batch` - Submit multiple projects or all commits
- `status` - Check job status
- `submissions` - View submission history per challenge
- `leaderboard` - View top scorers for a hackathon
- `diff` - View code submitted for a job
- `plagiarism` - Check for duplicate submissions
- `report` - Show submission analytics and trends
- `rating` - Display current Elo rating
- `logout` - Clear authentication
- `whoami` - Display current user
- `version` - Show version info

### 2. Client Layer

#### api.go - API Communication

Handles all HTTP communication with the backend server:

```go
// Create API client
apiClient := client.NewAPIClient("http://localhost:8000/api/v1")

// Login
resp, err := apiClient.Login(username, password)

// Submit for grading
submitResp, err := apiClient.Submit(commitSHA, suite)

// Get job status
statusResp, err := apiClient.GetStatus(jobID)
```

**Key Features:**
- Automatic token management
- Timeout handling
- Error response parsing
- Git integration (extracting commit SHA)

#### websocket.go - WebSocket Client

Provides real-time status updates with automatic fallback to HTTP polling:

```go
wsc := client.NewWSClient(baseURL, token)
err := wsc.ListenStatus(jobID, func(s *StatusResponse) {
    // Called on each status change
})
```

#### auth.go - Authentication Management

Manages the authentication workflow:

```go
authManager := client.NewAuthManager(apiClient)

// Login flow
err := authManager.Login()

// Check authentication
isAuth, err := authManager.IsAuthenticated()
```

**Key Features:**
- Secure password input (masked terminal input)
- Token storage in `~/.ft_hackthon/config.json`
- Configuration persistence
- User session management

#### submit.go - Grading Submission

Orchestrates the complete submission and polling workflow:

```go
submitManager := client.NewSubmitManager(apiClient)
err := submitManager.SubmitGradeJob()
```

**Workflow:**
1. Verify authentication
2. Check git repository validity
3. Extract commit SHA with `git rev-parse HEAD`
4. Send submission request to API
5. Poll for status updates every 2 seconds (or use WebSocket)
6. Display results in formatted output

#### batch.go - Batch Submission

Submit multiple projects or all commits of a project:

```go
// Submit multiple directories
results := sm.BatchSubmit(dirs, false)

// Submit all commits
results := sm.SubmitAllCommits(projectDir)
```

#### analytics.go - Submission Reports

Generate per-challenge analytics and trends:

```go
sm.GenerateReport(client.ReportOptions{
    DaysBack:  30,
    ShowTrend: true,
})
```

#### ui.go - Terminal UI Rendering

Provides formatted terminal output (all ASCII, no emoji):

```go
ui := client.NewTerminalUI()

// Print status updates
ui.PrintStatusUpdate(statusResp)

// Print final results
ui.PrintGradeResult(result)

// Print progress bar
ui.PrintProgress(current, total, "Submitting")
```

**Output Examples:**

Status Update:
```
STATUS: Queued - Waiting for grader availability...
```

Final Result:
```
==================================================
             GRADING RESULTS
==================================================

 Parser Success ........................... YES
 Benchmark Speed ......................... 150 ms
 Final Score ............................ 95 points

==================================================
```

### 3. Configuration Layer

Located in `internal/config/config.go`:

- **Token Storage** - Secure storage in `~/.ft_hackthon/config.json`
- **Permission Management** - Restricts config file to user only (0600)
- **Config Lifecycle** - Load, save, clear operations

```go
// Save configuration
cfg := &config.Config{
    Token: token,
    User: username,
}
config.SaveConfig(cfg)

// Load configuration
cfg, err := config.LoadConfig()

// Check authentication status
isAuth, err := config.IsAuthenticated()
```

## Data Flow

### Login Flow

```
User Input (username/password)
    |
Masked Password Prompt (terminal.ReadPassword)
    |
API Request: POST /api/v1/auth/login
    |
Auth Token + Gitea Credentials Response
    |
Save to ~/.ft_hackthon/config.json (mode 0600)
    |
Display Success Message
```

### Grading Submission Flow

```
User: ft_hackthon grademe
    |
Load Token from Config
    |
Read ft_hackthon.yml for suite (if present)
    |
Copy project files to workspace (~/ft_hackthon/workspace)
    |
git add, commit, push to Gitea
    |
Get Commit SHA (git rev-parse HEAD)
    |
API Request: POST /api/v1/grade/submit
    |
Job ID Response (202 Accepted)
    |
Start Monitoring (WebSocket preferred, HTTP polling fallback)
    |
Real-time Status Updates
    |
On Completion: Display Formatted Results
```

### Status Monitoring

The monitoring mechanism has the following characteristics:

- **Primary**: WebSocket (`/ws/grade/status/{job_id}`) for real-time updates
- **Fallback**: HTTP polling every 2 seconds
- **Max Attempts**: 300 attempts (10 minutes total)
- **States**:
  - `queued` - Waiting for grader availability
  - `processing` - Tests and benchmarks are running
  - `completed` - Grading finished successfully
  - `failed` - Grading failed (tests did not pass)
  - `error` - Grading encountered a system error

### Batch Submission Flow

```
User: ft_hackthon batch <dir1> <dir2> ...
    |
For each directory:
  | Verify it's a git repo
  | Get commit SHA
  | Copy files to workspace
  | Push to Gitea
  | Submit to API
    |
Display batch results summary
```

## CI/CD Integration

The CLI supports non-interactive mode for CI/CD pipelines. All REPL commands are available:

```bash
# Register (non-interactive prompts for username/password)
ft_hackthon --non-interactive --insecure register

# Login (non-interactive prompts for credentials)
ft_hackthon --non-interactive --insecure login

# Submit for grading
ft_hackthon --non-interactive --insecure grademe

# Check status (with optional job ID)
ft_hackthon --json --non-interactive --insecure status
ft_hackthon --non-interactive --insecure status <job_id>

# Batch submission
ft_hackthon --non-interactive --insecure batch ~/projects/project-a
ft_hackthon --non-interactive --insecure batch --all-commits .

# View submissions
ft_hackthon --non-interactive --insecure submissions
ft_hackthon --non-interactive --insecure diff <job_id>

# Leaderboard, plagiarism, reports
ft_hackthon --non-interactive --insecure leaderboard libft-tester
ft_hackthon --non-interactive --insecure plagiarism libft-tester
ft_hackthon --non-interactive --insecure report --trend

# Auth management (login/register are interactive token flows)
ft_hackthon --non-interactive --insecure whoami
ft_hackthon --non-interactive --insecure rating
ft_hackthon --non-interactive --insecure logout

# JSON output for programmatic parsing (supported on most commands)
ft_hackthon --json --non-interactive --insecure status
ft_hackthon --json --non-interactive --insecure submissions
ft_hackthon --json --non-interactive --insecure leaderboard libft-tester

# Exit codes (0 = success, 1 = failure)
ft_hackthon --non-interactive --insecure grademe || exit 1
```

## Error Handling

The CLI implements comprehensive error handling:

1. **API Errors** - Display error messages from server
2. **Git Errors** - Detect non-git directories
3. **Authentication Errors** - Prompt for re-authentication
4. **Network Errors** - Display connection failures
5. **Timeout Errors** - Notify if polling exceeds max duration

## Security Considerations

### Token Management

- **Storage Location** - `~/.ft_hackthon/config.json`
- **File Permissions** - 0600 (owner read/write only)
- **Format** - JSON with token and username
- **Lifecycle** - Loaded on demand, cleared on logout

### Password Input

- **Masking** - Uses `golang.org/x/term.ReadPassword`
- **No Echo** - Password not displayed or logged
- **Immediate Clearing** - Password cleared from memory after use

### API Communication

- **Authentication** - Auth token in Authorization header (Bearer <token>)
- **HTTPS Ready** - Support for secure connections
- **Token Validation** - Server-side verification

## Testing the CLI

### Manual Testing

```bash
# Build the CLI
make build-cli

# Test login
./bin/ft_hackthon login
# Enter credentials

# Verify authentication
./bin/ft_hackthon whoami

# Test grading submission
cd ~/my-project  # Must be a git repo
../ft_hackthon/bin/ft_hackthon grademe

# Check job status
./bin/ft_hackthon status job-id-here

# Test batch submission
./bin/ft_hackthon batch ../project1 ../project2

# Test analytics
./bin/ft_hackthon report --trend

# Logout
./bin/ft_hackthon logout
```

### Custom API Endpoint

```bash
ft_hackthon --api-url http://custom-server:8000/api/v1 login
ft_hackthon --api-url http://custom-server:8000/api/v1 grademe
```

### CI/CD Pipeline

```bash
./bin/ft_hackthon --non-interactive --json --insecure status
./bin/ft_hackthon --non-interactive --insecure grademe
```

## Performance Characteristics

- **Login** - ~500ms (network dependent)
- **Submission** - ~100ms (fast API call)
- **Status Check** - ~200ms per poll
- **Total Time to Results** - Depends on queue and processing time

## Terminology

All terminal output uses ASCII characters only:
- `+` indicates success or completion
- `-` indicates failure or error
- `*` indicates neutral or in-progress status
- `[!]` indicates a warning
