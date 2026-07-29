# ft_hackthon — Hackathon Automated Grading System

Automated grading system for hackathon projects. Docker services: **traefik** (TLS + Let's Encrypt), **api** (REST + WebSocket), **worker** (background grading), **backup** (PostgreSQL dumps).

## Quick Start

```bash
git clone <repo> && cd ft_hackthon
make docker-up                         # Build & start all services
make docker-cli-binary                 # Extract CLI from Docker image
./bin/ft_hackthon-cli --insecure login
./bin/ft_hackthon-cli --insecure grademe
```

## Deploy to a Cloud VM

Set `CLOUD_PROVIDER` and provider credentials in `.env`, then:

```bash
make deploy
make dns                          # (optional) update DNS to VM IP
```

Traefik auto-provisions a Let's Encrypt cert when `DOMAIN` is set.

### DigitalOcean

```env
CLOUD_PROVIDER=digitalocean
DIGITALOCEAN_TOKEN=your_api_token
SSH_KEY_NAME=your_key_name        # must exist in DO account
DOMAIN=                           # (optional) enables Let's Encrypt
```

SSH: `ssh root@<ip>`

### AWS

```env
CLOUD_PROVIDER=aws
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
SSH_KEY_NAME=42School              # EC2 key pair name (created via aws ec2 create-key-pair)
DOMAIN=                            # (optional) enables Let's Encrypt
```

Credentials via `~/.aws/credentials` or env vars. SSH: `ssh -i ~/.ssh/42School.pem ubuntu@<ip>`

### GCP

```env
CLOUD_PROVIDER=gcp
TF_VAR_project=my-project-id       # GCP project ID
SSH_KEY_NAME=your-username         # GCP username (from SSH key comment)
SSH_PUBLIC_KEY_PATH=~/.ssh/id_ed25519.pub
DOMAIN=                            # (optional) enables Let's Encrypt
```

GCP auth: `gcloud auth application-default login`. SSH: `ssh <username>@<ip>`

### Azure

```env
CLOUD_PROVIDER=azure
SSH_PUBLIC_KEY_PATH=~/.ssh/id_ed25519.pub
DOMAIN=                            # (optional) enables Let's Encrypt
```

Azure auth: `az login`. SSH: `ssh ftuser@<ip>`

## Download the CLI

Pre-built binaries on the [releases page](https://github.com/herom-s/ft_hackthon/releases):

```bash
go install github.com/herom-s/ft_hackthon/cmd/ft_hackthon@latest

# Or direct download
curl -LO https://github.com/herom-s/ft_hackthon/releases/latest/download/ft_hackthon-linux-amd64
chmod +x ft_hackthon-linux-amd64 && ./ft_hackthon-linux-amd64
```

## Configuration (`.env`)

| Variable | Default | Description |
|----------|---------|-------------|
| `DOMAIN` | `""` | Your domain → enables Let's Encrypt HTTPS |
| `DIGITALOCEAN_TOKEN` | — | API token for `make deploy` |
| `CLOUD_PROVIDER` | `digitalocean` | Cloud for `make deploy` (aws, gcp, azure) |
| `API_PORT` | `8000` | Internal API port |
| `TRAEFIK_HTTPS_PORT` | `8343` | Host HTTPS port (TLS) |
| `POSTGRES_USER` | `ft_hackthon` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `ft_hackthon` | PostgreSQL password |
| `GITEA_ADMIN_USER` | `ft_hackthon` | Gitea admin username |
| `GITEA_ADMIN_PASSWORD` | `changeme123` | Gitea admin password |
| `GITEA_ORG` | `moulinerie` | Gitea organization |
| `GITEA_PUBLIC_URL` | `http://localhost:3000` | Public Gitea URL (set by cloud-init on deploy) |
| `WORKER_ID` | hostname | Worker instance identity |
| `BACKUP_INTERVAL` | `86400` | Backup interval (seconds) |
| `BACKUP_RETENTION_DAYS` | `7` | Backup retention |

## Docker

```bash
make docker-up       # Build + start
make docker-down     # Stop
make docker-restart  # Rebuild + restart
make docker-logs     # Tail logs
make docker-clean    # Stop + remove volumes (resets all data)
make docker-cli-binary  # Extract CLI from Docker image
```

## Development

```bash
make build           # Build all Go binaries
make test            # Run tests
make fmt             # gofmt
make lint            # golangci-lint
make clean           # Remove binaries
```

## CI/CD

GitHub Actions workflows in `.github/workflows/`:

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| **CI** (`ci.yml`) | Push/PR to `main` | `go build ./...`, `go vet ./...`, `go test ./...`, golangci-lint |
| **Release** (`release.yml`) | Tag `v*` | Builds CLI binaries for linux/darwin × amd64/arm64, creates GitHub Release, deploys to VM |
| **Deploy** (`deploy.yml`) | Push to `main` | Pulls code on VM, rebuilds Docker containers |

### Required GitHub Secrets

For auto-deploy to work, add these in the repo → **Settings → Secrets and variables → Actions**:

| Secret | Value |
|--------|-------|
| `VM_HOST` | Your VM IP (e.g. `165.227.118.186`) |
| `VM_SSH_KEY` | Private SSH key for root access (use `SSH_KEY_NAME` from `.env`) |

### Creating a release

```bash
git tag v1.2.3
git push origin v1.2.3
```

## Project Structure

```
cmd/        ft_hackthon/ (CLI), api/, worker/
internal/   client/, config/, database/, gitea/, grader/, handler/, worker/
terraform/  digitalocean/, aws/, gcp/, azure/  (OpenTofu configs per cloud)
traefik/     Traefik config + entrypoint (auto Let's Encrypt)
testsuites/ Suite definitions (suite.yml per hackathon)
docs/       User guide, API ref, architecture, operations, development
```

## Documentation

- [User Guide](docs/USER_GUIDE.md)
- [API Reference](docs/API.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Development](docs/DEVELOPMENT.md)
- [System Operations](docs/SYSTEM_OPERATIONS.md)

## License

MIT
