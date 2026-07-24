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

```bash
echo 'DIGITALOCEAN_TOKEN=your_token' >> .env

# 2. Optionally set a domain for Let's Encrypt
echo 'DOMAIN=hackthon.yourdomain.com' >> .env

# 3. Deploy (creates a $12/mo droplet in nyc3)
make deploy

# 4. Use the CLI (--insecure only needed if no domain)
ft_hackthon --insecure --api-url https://<vm-ip>:8343/api/v1 login
```

### With a domain (no --insecure needed)

```bash
echo 'DOMAIN=hackthon.yourdomain.com' >> .env
make deploy-destroy && make deploy
```

Traefik auto-provisions a Let's Encrypt certificate.

**Important**: After first deploy with `DOMAIN` set, you must change your domain's nameservers at the registrar to DigitalOcean's (`ns1.digitalocean.com`, `ns2.digitalocean.com`, `ns3.digitalocean.com`). The A record is managed automatically by terraform.

Then:

```bash
ft_hackthon --api-url https://hackthon.yourdomain.com:8343/api/v1 login
```

See [docs/SYSTEM_OPERATIONS.md](docs/SYSTEM_OPERATIONS.md) for multi-cloud (AWS/GCP/Azure) via OpenTofu.

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
