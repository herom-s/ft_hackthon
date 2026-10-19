# ft_hackthon

A self-hosted grading platform for hackathons. Students push code, the platform builds and tests it against predefined suites, and returns results through a CLI.

## How it works

```
┌───────────┐  submit   ┌──────┐  enqueue  ┌────────────┐  claim jobs  ┌──────────┐
│  CLI      │─────────▶ │ API  │─────────▶ │ PostgreSQL │◀──────────── │  Worker  │
│ (student) │ ◀──────── │      │ ◀──────── │            │  save result │ (grader) │
└───────────┘  status / └──┬───┘   read    └────────────┘ ───────────▶ └────┬─────┘
               WebSocket   │ provision repo                                │ git clone
                           ▼                                               ▼
                     ┌───────────────────────────────────────────────────────┐
                     │                         Gitea                         │
                     └───────────────────────────────────────────────────────┘
```

- **CLI** — interactive shell for login, submission, status, leaderboard
- **API** — REST + WebSocket server (auth, grading, user management)
- **Worker** — claims queued jobs directly from PostgreSQL, clones the commit from Gitea, runs the suite in a Docker sandbox, records results (it never calls the API)
- **Traefik** — reverse proxy with automatic Let's Encrypt TLS
- **Gitea** — hosts student repositories, auto-created per registration
- **Test suites** — YAML-defined build/run steps with multi-challenge support

## Installing / Getting started

```shell
git clone <repo> && cd ft_hackthon
make docker-up
make docker-cli-binary
./bin/ft_hackthon-cli --insecure
```

Starts all services (API, worker, database, Gitea) in Docker and extracts the CLI binary.

### Initial Configuration

```shell
cp .env.example .env
```

Defaults work for local development. Full reference in [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md#configuration).

## Usage

```shell
ft_hackthon --insecure
> register
> login
> grademe
> status
```

For CI/CD, pass the command as arguments (no prompts): `ft_hackthon --insecure grademe`

## Features

- Interactive REPL with tab completion
- Real-time WebSocket grading updates
- Leaderboard, analytics, plagiarism detection
- Parallel grading via auto-scaling worker pool
- Automatic TLS with Let's Encrypt
- Deploy to DigitalOcean, AWS, GCP, Azure

## Developing

```shell
make build          # Build all Go binaries
make test           # Run tests
make fmt lint       # Format and lint
```

### Deploying

```shell
make deploy         # Provisions VM + starts services
make dns            # Updates DNS to VM IP (optional)
```

## Links

- [User Guide](docs/USER_GUIDE.md) — installation, commands, workflows
- [Development Guide](docs/DEVELOPMENT.md) — build, test, deploy, CI/CD, env reference
- [System Operations](docs/SYSTEM_OPERATIONS.md) — production management
- [API Reference](docs/API.md) — REST and WebSocket endpoints
- [Architecture](docs/ARCHITECTURE.md) — system design

## Licensing

MIT. See `LICENSE` file for details.
