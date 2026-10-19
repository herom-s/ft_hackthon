# OpenTofu — Deploy ft_hackthon on Any Cloud

## Usage

Set `CLOUD_PROVIDER` in `.env` (root of repo), add provider credentials, then:

```bash
make deploy          # create or update
make deploy-info     # show VM IP, SSH command, API URL
make deploy-destroy  # tear down
make dns             # point name.com domain at VM IP
```

## Provider credentials

| Provider      | Env vars & setup |
|---------------|------------------|
| AWS           | `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` (or `~/.aws/credentials`) |
| GCP           | `TF_VAR_project=my-project` in `.env`; auth via `gcloud auth application-default login` |
| Azure         | `az login`; `SSH_PUBLIC_KEY_PATH` in `.env` |

## Common env vars (all providers)

| Variable | Required | Description |
|----------|----------|-------------|
| `CLOUD_PROVIDER` | ✓ | `aws`, `gcp`, or `azure` |
| `DOMAIN` | — | Enables Let's Encrypt cert (no `--insecure` needed) |
| `SSH_KEY_NAME` | AWS/GCP | Key pair name (AWS) or SSH username (GCP) |
| `SSH_PUBLIC_KEY_PATH` | Azure/GCP | Path to public key file |

## Structure

```
terraform/
├── aws/             # AWS EC2 + Security Group
├── gcp/             # GCP Compute Engine + Firewall
└── azure/           # Azure VM + NSG + VNet
```

Each directory is self-contained with its own `main.tf`, `providers.tf`, and `variables.tf`.

## Integration tests (no cloud account)

`make test-deploy` deploys every module against local emulators from [floci](https://floci.io)
(`floci` for AWS, `floci-az`, `floci-gcp`), checks each emulator's own API for the instance,
firewall ports and cloud-init, then destroys the stack and checks it is gone. Needs Docker,
OpenTofu and Go. `make floci-down` stops the emulators.

```
terraform/test/
├── docker-compose.yml   # pinned emulators on ports 14566 (AWS), 14577 (Azure), 14588 (GCP)
├── deploy_test.go       # go test -tags integration ./terraform/test/
└── <cloud>/             # copied next to the real module at test time
    ├── floci.tf             # emulator endpoint variables (+ GCP fixtures)
    └── floci_override.tf    # OpenTofu override: points the provider at the emulator
```

The modules themselves are tested unmodified; only the provider settings are overridden.
Known emulator gaps the tests work around:

| Cloud | Gap | Workaround |
|-------|-----|------------|
| GCP | No auto-mode `default` network, no public OS images | `floci.tf` creates a custom network and an `ubuntu-2404-lts-amd64` image |
| GCP | Small machine-type catalog without `e2-small` | Test passes `size = e2-standard-2` |
| Azure | azurerm only talks HTTPS | `floci-az` runs with TLS; the test trusts its cert via `SSL_CERT_FILE` |

floci's EC2 runs each instance as a Docker container and leaves a `floci-aws-ec2-volume-helper`
container behind; remove it with `docker rm -f floci-aws-ec2-volume-helper` if it bothers you.
Ports default to 145xx so a floci already running on 45xx is untouched; override with
`FLOCI_AWS_PORT` / `FLOCI_AZ_PORT` / `FLOCI_GCP_PORT` and the matching `FLOCI_*_ENDPOINT`.
