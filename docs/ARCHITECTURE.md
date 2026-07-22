# ft_hackthon CLI Architecture

## Overview

The `ft_hackthon` CLI is designed to provide a seamless, interactive experience for hackathon participants submitting their projects for automated grading.

## Components

### 1. Command Layer (Cobra Framework)

Located in `cmd/ft_hackthon/commands.go` and `cmd/ft_hackthon/repl.go`, the command layer provides:

- **Command Registration** - All CLI commands are registered with Cobra
- **Flag Management** - Global and command-specific flags
- **Help Generation** - Automatic help text generation
- **Error Handling** - Consistent error reporting

**Available Commands:**
- `login` - Authenticate with credentials
- `register` - Create a new account (prompts for hackathon selection)
- `grademe` - Submit project for grading
- `status` - Check job status
- `leaderboard` - View top scorers for a hackathon
- `submissions` - View submission history per challenge
- `diff` - View code submitted for a job
- `plagiarism` - Check for duplicate submissions
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
5. Poll for status updates every 2 seconds
6. Display results in formatted output

#### ui.go - Terminal UI Rendering

Provides formatted terminal output:

```go
ui := client.NewTerminalUI()

// Print status updates
ui.PrintStatusUpdate(statusResp)

// Print final results
ui.PrintGradeResult(result)

// Utility functions
ui.PrintError("Error message")
ui.PrintSuccess("Success message")
```

**Output Examples:**

Status Update:
```
⏳ STATUS: Queued - Waiting for grader availability...
```

Final Result:
```
═══════════════════════════════════════════════
            GRADING RESULTS
═══════════════════════════════════════════════

 Parser Success ........................... ✓ YES
 Benchmark Speed ......................... 150 ms
 Final Score ............................ 95 points

═══════════════════════════════════════════════
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
    ↓
Masked Password Prompt (terminal.ReadPassword)
    ↓
API Request: POST /api/v1/auth/login
    ↓
Auth Token + Gitea Credentials Response
    ↓
Save to ~/.ft_hackthon/config.json (mode 0600)
    ↓
Display Success Message
```

### Grading Submission Flow

```
User: ft_hackthon grademe
    ↓
Load Token from Config
    ↓
Read ft_hackthon.yml for suite (if present)
    ↓
Copy project files to workspace (~/ft_hackthon/workspace)
    ↓
git add, commit, push to Gitea
    ↓
Get Commit SHA (git rev-parse HEAD)
    ↓
API Request: POST /api/v1/grade/submit
    ↓
Job ID Response (202 Accepted)
    ↓
Start Polling Loop
    ↓
Every 2 seconds: GET /api/v1/grade/status/{job_id}
    ↓
Display Status Updates
    ↓
On Completion: Display Formatted Results
```

### Status Polling Loop

The polling mechanism has the following characteristics:

- **Interval** - 2 seconds between requests
- **Max Attempts** - 300 attempts (10 minutes total)
- **States**:
  - `queued` - Waiting for grader availability
  - `processing` - Tests and benchmarks are running
  - `completed` - Grading finished successfully
  - `failed` - Grading failed (tests did not pass)
  - `error` - Grading encountered a system error

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

# Logout
./bin/ft_hackthon logout
```

### Custom API Endpoint

```bash
ft_hackthon --api-url http://custom-server:8000/api/v1 login
ft_hackthon --api-url http://custom-server:8000/api/v1 grademe
```

## Performance Characteristics

- **Login** - ~500ms (network dependent)
- **Submission** - ~100ms (fast API call)
- **Status Check** - ~200ms per poll
- **Total Time to Results** - Depends on queue and processing time

## Future Enhancements

### Enhanced TUI

Implement `github.com/charmbracelet/bubbletea` for:
- Interactive progress bars
- Animated spinners
- Real-time log streaming
- Keyboard navigation

### Advanced Features

- Batch submission
- Job history
- Webhook notifications
- WebSocket support for real-time updates
- Detailed analytics and reports

### Integration

- CI/CD system integration
- Git hooks integration
- IDE plugin support
- Slack/Discord notifications

## Troubleshooting

### "Not authenticated" Error

```bash
ft_hackthon login
```

### API Connection Failed

```bash
# Check if API is running
curl http://localhost:8000/api/v1/health

# Use custom endpoint if needed
ft_hackthon --api-url http://your-api:8000/api/v1 grademe
```

### Git Repository Error

```bash
# Ensure you're in a git directory
git status

# Initialize if needed
git init
```

### Token Expired

```bash
# Re-login to refresh token
ft_hackthon login
```

## Code Organization Best Practices

1. **Separation of Concerns** - Each file handles a specific responsibility
2. **Error Propagation** - Errors bubble up with context
3. **Configuration Isolation** - All config in dedicated package
4. **Clean Interfaces** - Simple, focused public APIs
5. **Documentation** - Comprehensive comments and examples
