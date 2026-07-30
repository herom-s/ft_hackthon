# ft_hackthon

> Automated grading system for hackathon projects

Students submit code via a CLI client and receive real-time grading results. Built for 42School-style hackathons with multi-cloud deployment.

## Installing / Getting started

```shell
git clone <repo> && cd ft_hackthon
make docker-up
make docker-cli-binary
./bin/ft_hackthon-cli --insecure
```

This starts all services (API, worker, database, Gitea) in Docker and extracts the CLI binary. Run the binary to enter the interactive shell.

### Initial Configuration

Copy `.env.example` to `.env` and adjust settings. The defaults work for local development:

```shell
cp .env.example .env
```

## Usage

```shell
ft_hackthon --insecure
> register
> login
> grademe
> status
```

Commands run inside the interactive REPL. For CI/CD automation, use `--non-interactive`:

```shell
ft_hackthon --non-interactive --insecure grademe
```

## Features

- Interactive CLI with tab completion
- Real-time WebSocket grading updates
- Leaderboard, analytics, and plagiarism detection
- Parallel grading via auto-scaling worker pool
- Automatic TLS with Let's Encrypt via Traefik
- Deploy to DigitalOcean, AWS, GCP, or Azure

## Developing

```shell
git clone <repo>
make build          # Build all Go binaries
make test           # Run tests
make fmt lint       # Format and lint
```

### Building

```shell
make build-cli      # CLI only
make build-api      # API server only
make build-worker   # Worker only
```

### Deploying

Set `CLOUD_PROVIDER` and credentials in `.env`, then:

```shell
make deploy
make dns            # Update DNS to VM IP (optional)
```

Traefik provisions a Let's Encrypt certificate when `DOMAIN` is set.

## Configuration

Full env reference in [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md#configuration).

Key variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DOMAIN` | `""` | Enables Let's Encrypt HTTPS |
| `CLOUD_PROVIDER` | `digitalocean` | Cloud target for `make deploy` |
| `TRAEFIK_HTTPS_PORT` | `8343` | Host HTTPS port |

## Links

- [User Guide](docs/USER_GUIDE.md)
- [Development Guide](docs/DEVELOPMENT.md)
- [System Operations](docs/SYSTEM_OPERATIONS.md)
- [API Reference](docs/API.md)
- [Architecture](docs/ARCHITECTURE.md)
- Repository: [github.com/herom-s/ft_hackthon](https://github.com/herom-s/ft_hackthon)

## Licensing

The code in this project is licensed under MIT license. See `LICENSE` file for details.
