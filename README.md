# ft_hackthon -- Hackathon Automated Grading System

Automated grading system for hackathon projects. Four Docker services: **nginx** (TLS termination + load balancing), **api** (REST API), **worker** (background job processor), **backup** (PostgreSQL dumps).

## Quick Start

```bash
git clone <repo> && cd ft_hackthon
make docker-up                    # Build & start all services
./bin/ft_hackthon --insecure login  # Or: make run-cli
./bin/ft_hackthon --insecure grademe
```

See [docs/USER_GUIDE.md](docs/USER_GUIDE.md) for detailed usage.

## Components

| Component | Directory | Description |
|-----------|-----------|-------------|
| CLI | `cmd/ft_hackthon/` | Terminal client (interactive REPL + batch + CI/CD) |
| API | `cmd/api/` | REST API server (port 8000 internal, WebSocket support) |
| Worker | `cmd/worker/` | Background job processor (claims + grades jobs) |
| Backups | `scripts/backup.sh` | Periodic pg_dump via Docker Compose `backup` service |

## Docker (recommended)

```bash
make docker-up       # Build images + start all services
make docker-down     # Stop all containers
make docker-clean    # Stop + remove volumes (resets all data)
make docker-logs     # Tail all logs
make docker-restart  # Rebuild + restart
```

Services: `nginx:8000` (HTTP->HTTPS redirect) -> `nginx:8443` (TLS) -> `api:8000` -> `postgres:5432`. Worker connects to Postgres directly for job claiming (SKIP LOCKED).

## Development

```bash
make build           # Build all Go binaries
make test            # Run all tests
make fmt             # gofmt
make lint            # golangci-lint
make clean           # Remove binaries
```

### Dependencies

- `github.com/spf13/cobra` -- CLI framework
- `github.com/chzyer/readline` -- REPL with history & tab completion
- `github.com/go-resty/resty/v2` -- HTTP client
- `github.com/gorilla/websocket` -- WebSocket client & server
- `github.com/jackc/pgx/v5` -- PostgreSQL driver
- `gopkg.in/yaml.v3` -- YAML parsing
- `golang.org/x/crypto` -- bcrypt
- `golang.org/x/term` -- terminal input

## Project Structure

```
cmd/
  api/main.go              # REST API entry point
  worker/main.go           # Worker entry point
  ft_hackthon/             # CLI (main.go, repl.go, helpers.go)
internal/
  client/                  # API client (api.go, auth.go, submit.go, ui.go, websocket.go, batch.go, analytics.go, hooks.go)
  config/                  # Config management (config.go, server.go)
  database/                # DB interface + Postgres + InMemory + migrations
  gitea/                   # Gitea API client + interface
  grader/                  # Suite/challenge grading engine (run.go, grader.go)
  handler/                 # HTTP handlers + middleware + WebSocket
  worker/                  # Job processor + circuit breaker
testsuites/                # Test suite definitions (suite.yml per hackathon)
nginx/                     # nginx config + entrypoint (self-signed TLS)
scripts/                   # backup.sh, restore.sh
docs/                      # Full documentation
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check (DB ping) |
| POST | `/api/v1/auth/login` | Login |
| POST | `/api/v1/auth/register` | Register |
| POST | `/api/v1/grade/submit` | Submit project |
| GET | `/api/v1/grade/status/{id}` | Job status |
| GET | `/api/v1/grade/jobs` | List my jobs |
| GET | `/api/v1/grade/suites` | List test suites |
| GET | `/api/v1/grade/suites/{suite}/challenges` | List challenges |
| GET | `/api/v1/grade/leaderboard/{hackathon}` | Leaderboard |
| GET | `/api/v1/grade/plagiarism/{hackathon}` | Duplicate check |
| GET | `/api/v1/user/me` | Current user + rating |
| GET/POST | `/api/v1/alerts` | System alerts |
| GET | `/api/v1/metrics` | Prometheus metrics |
| WS | `/ws/grade/status/{job_id}` | Real-time job status |
| WS | `/ws/grade/jobs` | Real-time jobs list |

## CLI Commands

| Command | Description |
|---------|-------------|
| `login` | Authenticate with the server |
| `register` | Create a new account |
| `grademe` | Submit current project for grading |
| `batch` | Submit multiple projects or all commits |
| `status` | List your jobs or check a specific job |
| `submissions` | Show submission history per challenge |
| `leaderboard` | Show top scorers for a hackathon |
| `plagiarism` | Check for duplicate submissions |
| `diff` | View code submitted for a grading job |
| `report` | Show submission analytics and trends |
| `hooks` | Manage git hooks for auto-submission |
| `whoami` | Show current user |
| `rating` | Display your current Elo rating |
| `logout` | Clear stored session |
| `version` | Display version info |
| `help` | Show help |

## CLI Flags

| Flag | Description |
|------|-------------|
| `--api-url` | API base URL (default: https://localhost:8443/api/v1) |
| `--insecure` | Skip TLS certificate verification |
| `--verbose` | Enable verbose logging |
| `--json` | Output in JSON format (for CI/CD) |
| `--quiet` | Suppress non-essential output |
| `--non-interactive` | Run in non-interactive mode (for CI/CD) |

## Configuration

Environment variables (`.env`):

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `8000` | Internal API port |
| `NGINX_HTTP_PORT` | `8000` | Host HTTP port (redirects to HTTPS) |
| `NGINX_HTTPS_PORT` | `8443` | Host HTTPS port (TLS) |
| `DATABASE_URL` | `postgres://ft_hackthon:ft_hackthon@postgres:5432/ft_hackthon?sslmode=disable` | Postgres DSN |
| `GITEA_ADMIN_USER` | `admin` | Gitea admin username |
| `GITEA_ADMIN_PASSWORD` | `changeme123` | Gitea admin password |
| `GITEA_ORG` | `ft_hackthon` | Gitea organization |
| `GITEA_PUBLIC_URL` | `http://localhost:3000` | Public Gitea URL (for clone URLs) |
| `WORKER_ID` | hostname | Worker instance identity |
| `BACKUP_INTERVAL` | `86400` | Backup interval (seconds) |
| `BACKUP_RETENTION_DAYS` | `7` | Backup retention |

## Adding a Test Suite

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) (section "Adding a New Test Suite").

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `tls: certificate signed by unknown authority` | Add `--insecure` flag to CLI |
| `container name already in use` | `docker rm -f <container>` or `make docker-clean` |
| `connect: connection refused` | Ensure services are up: `make docker-up` |
| Gitea clone fails | Verify `GITEA_PUBLIC_URL` matches host-accessible address |
| Build fails | `make clean && make deps && make build` |

## Documentation

- [User Guide](docs/USER_GUIDE.md) -- CLI usage, workflows, FAQ
- [API Reference](docs/API.md) -- Full endpoint spec
- [Architecture](docs/ARCHITECTURE.md) -- System design
- [Development](docs/DEVELOPMENT.md) -- Setup, testing, adding suites
- [System Operations](docs/SYSTEM_OPERATIONS.md) -- Deployment, monitoring, backup

## License

MIT
