# Development Guide

## Setting Up Development Environment

### Prerequisites

- Go 1.21 or higher (go.mod specifies 1.25)
- Git
- Make
- Linux/macOS (Windows WSL2 recommended)

### Quick Start

```bash
# Clone the repository
git clone <repository-url>
cd <repo-dir>

# Install dependencies
make deps

# Build all components
make build

# Or build just the CLI
make build-cli
```

## Project Structure

```
ft_hackthon/
├── cmd/
│   ├── api/main.go             # REST API server
│   ├── ft_hackthon/
│   │   ├── main.go             # Entry point + init
│   │   ├── commands.go         # 13 Cobra command definitions
│   │   ├── repl.go             # Interactive REPL loop
│   │   └── helpers.go          # Gitea/suite helpers
│   └── worker/main.go          # Background worker
├── internal/
│   ├── client/                 # CLI client
│   │   ├── api.go, auth.go, submit.go, ui.go
│   ├── config/config.go        # Config management
│   ├── database/
│   │   ├── database.go         # Interface + InMemoryDB
│   │   └── postgres.go         # PostgreSQL implementation
│   ├── gitea/
│   │   ├── gitea.go            # Gitea API client
│   │   └── interface.go        # ClientInterface
│   ├── grader/
│   │   ├── grader.go           # Suite/challenge config, scoring
│   │   └── run.go              # Grade execution
│   ├── handler/handler.go      # API request handlers
│   └── worker/worker.go        # Job processor
├── docs/                       # Documentation
├── testsuites/                 # Test suite definitions
├── Makefile, go.mod, Dockerfile, docker-compose.yml
└── README.md
```

## Development Workflow

### 1. Building

```bash
# Build all components
make build

# Build specific component
make build-cli      # CLI tool
make build-api      # API server
make build-worker   # Worker engine

# Build with debug flags
make build-dev

# Watch for changes and rebuild
make watch
```

### 2. Running

```bash
# Start API server
make run-api

# Start worker
make run-worker

# Run CLI with arguments
make run-cli ARGS="login"
make run-cli ARGS="grademe"
make run-cli ARGS="--help"
```

### 3. Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# The coverage report will be generated as coverage.html
```

### 4. Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Run go vet
make vet

# All checks
make fmt lint vet test
```

## Making Changes

### Adding a New CLI Command

1. Create the command handler in `cmd/ft_hackthon/commands.go`:

```go
var myCmd = &cobra.Command{
    Use:   "mycommand",
    Short: "Description of my command",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Implementation
        return nil
    },
}
```

2. Register the command in `cmd/ft_hackthon/main.go`'s `init()` function:

```go
func init() {
    rootCmd.AddCommand(myCmd)
}
```

3. Test the new command:

```bash
make build-cli
./bin/ft_hackthon mycommand
```

### Adding Client Functionality

1. Add your function to the appropriate package in `internal/client/`:

```go
// In api.go
func (ac *APIClient) MyNewFunction() error {
    // Implementation
    return nil
}
```

2. Use it from a command:

```go
apiClient := client.NewAPIClient(apiBaseURL)
err := apiClient.MyNewFunction()
```

### Extending the API

1. Add handler in `internal/handler/handler.go`
2. Implement in `cmd/api/main.go`
3. Update the client API in `internal/client/api.go`
4. Document in `docs/API.md`

## Error Handling Best Practices

Always wrap errors with context:

```go
// Good
result, err := someFunction()
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Avoid
if err != nil {
    return err
}
```

## Configuration Development

The CLI stores configuration in `~/.ft_hackthon/config.json`:

```go
// Load configuration
cfg, err := config.LoadConfig()

// Modify configuration
cfg.Token = newToken

// Save configuration
err := config.SaveConfig(cfg)
```

## API Client Development

The API client is in `internal/client/api.go`:

```go
// Create client
client := client.NewAPIClient(baseURL)

// Set token
client.SetToken(token)

// Make requests
response, err := client.Login(username, password)
```

## Terminal UI Development

Terminal output is handled by `internal/client/ui.go`:

```go
ui := client.NewTerminalUI()

// Print formatted output
ui.PrintError("Error message")
ui.PrintSuccess("Success message")
ui.PrintStatusUpdate(status)
ui.PrintGradeResult(result)
```

## Debugging

### Enable Verbose Logging

```bash
ft_hackthon --verbose login
```

### Print Debug Information

```go
if verbose {
    fmt.Printf("Debug: %+v\n", variable)
}
```

### Using Debugger

```bash
# Build with debug flags
make build-dev

# Use Delve debugger
dlv exec ./bin/ft_hackthon-dev
```

## Git Workflow

1. Create feature branch: `git checkout -b feature/my-feature`
2. Make changes and commit: `git commit -am "Add my feature"`
3. Format and test: `make fmt lint vet test`
4. Push branch: `git push origin feature/my-feature`
5. Create pull request

## Common Commands

```bash
# Clean build artifacts
make clean

# Setup development environment
make setup

# Display help
make help

# Display project info
make info

# Check code format
go fmt ./...

# Check for issues
go vet ./...

# Update dependencies
go get -u ./...
go mod tidy
```

## Testing Guide

### Unit Tests

Create test files alongside implementation:

```go
// example.go
func Add(a, b int) int {
    return a + b
}

// example_test.go
func TestAdd(t *testing.T) {
    result := Add(2, 3)
    expected := 5
    if result != expected {
        t.Errorf("expected %d, got %d", expected, result)
    }
}
```

Run tests:

```bash
make test
```

### Integration Tests

Test full workflows:

```go
func TestLoginAndGrade(t *testing.T) {
    // Setup
    client := client.NewAPIClient(testURL)
    
    // Execute
    resp, err := client.Login(testUser, testPass)
    
    // Assert
    if err != nil {
        t.Fatalf("login failed: %v", err)
    }
}
```

## Continuous Integration

The project includes a Makefile that can be used in CI:

```bash
make setup
make fmt lint vet test build
```

## Performance Optimization

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof
go tool pprof mem.prof
```

### Benchmarking

```go
func BenchmarkLogin(b *testing.B) {
    for i := 0; i < b.N; i++ {
        client.Login(user, pass)
    }
}
```

## Release Process

```bash
# Update version in cmd/ft_hackthon/commands.go
# Tag release
git tag v1.0.0
git push origin v1.0.0

# Build release binaries
make clean build
```

## Documentation

- **Code Comments** - Explain complex logic
- **Function Documentation** - Document public functions
- **Examples** - Provide usage examples
- **README** - Overview and quick start
- **ARCHITECTURE.md** - System design
- **API.md** - API specification

## Troubleshooting

### Build Failures

```bash
# Clean and rebuild
make clean build

# Check Go version
go version  # Should be 1.21+

# Update dependencies
go mod tidy
```

### Test Failures

```bash
# Run specific test
go test -run TestName ./path/to/package

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Import Issues

```bash
# Download dependencies
go mod download

# Vendor dependencies
go mod vendor

# Tidy go.mod
go mod tidy
```

## Adding a New Test Suite

The ft_hackthon grading system uses a generic test suite architecture. Test suites are stored as directories under `TESTSUITES_PATH`. Each suite directory contains:

- `suite.yml` — descriptor file (required)
- Zero or more test files (e.g. `test.c`, `test.py`, etc.)

Test suites are distributed as **a separate Docker image** that the main ft_hackthon images mount at runtime. This means admins can develop, build, and publish their own test suites independently — no ft_hackthon code changes needed.

### Directory structure

```
my-testsuites/
  libft/
    suite.yml
    test.c
  py-challenge/
    suite.yml
    test.py
```

### Suite Descriptor (`suite.yml`)

```yaml
name: my-problem
language: c
detect:
  - libft.h
build: gcc -o {binary} {suite_files} {workspace_files} -lm
run: "{binary}"
```

| Field      | Description |
|------------|-------------|
| `name`     | Human-readable problem name |
| `language` | Language label (informational) |
| `detect`   | **Required.** List of filenames the workspace **must contain** for this suite to be selected. The first suite whose detect files all exist in the workspace is used. |
| `build`    | Shell command to compile/build the test. Set to `""` if no build step is needed. |
| `run`      | Shell command to execute the test. Must return exit code 0 for pass, non-zero for fail. |

### Template Variables

| Variable           | Expands to |
|--------------------|------------|
| `{binary}`         | Path to a temp file (for the compiled binary) |
| `{workspace}`      | Path to the workspace directory (user's solution) |
| `{workspace_files}` | Space-separated list of all `*.c` files in the workspace |
| `{suite}`          | Path to this test suite's directory |
| `{suite_files}`    | Space-separated list of all `*.c` files in the suite directory |

### Dockerfile

Create a `Dockerfile` in your test suites directory:

```dockerfile
FROM alpine:latest
LABEL org.ft_hackthon.testsuites="1"
COPY . /var/ft_hackthon/testsuites
```

The `LABEL` is optional but helps identify ft_hackthon-compatible images.

### Build and publish

```sh
docker build -t my-org/my-testsuites my-testsuites/
docker push my-org/my-testsuites
```

### Using a Custom Test Suites Image

In docker-compose, replace the `testsuites` service's build with your image:

```yaml
services:
  testsuites:
    image: my-org/my-testsuites
    ...
```

### Quick Start: Adding a New Problem

1. Create a directory under your test suites source: `my-testsuites/new-problem/`
2. Write `suite.yml` with detect rules
3. Write your test file(s)
4. Rebuild the Docker image: `make docker-build-testsuites`
5. Restart services: `make docker-up`

No Go code changes needed — the grader discovers suites automatically at startup.

## Additional Resources

- Go Documentation: https://golang.org/doc/
- Cobra Documentation: https://cobra.dev/
- Resty Documentation: https://github.com/go-resty/resty
- Go Best Practices: https://golang.org/doc/effective_go
