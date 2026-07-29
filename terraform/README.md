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
| DigitalOcean  | `DIGITALOCEAN_TOKEN` |
| AWS           | `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` (or `~/.aws/credentials`) |
| GCP           | `TF_VAR_project=my-project` in `.env`; auth via `gcloud auth application-default login` |
| Azure         | `az login`; `SSH_PUBLIC_KEY_PATH` in `.env` |

## Common env vars (all providers)

| Variable | Required | Description |
|----------|----------|-------------|
| `CLOUD_PROVIDER` | ✓ | `digitalocean`, `aws`, `gcp`, or `azure` |
| `DOMAIN` | — | Enables Let's Encrypt cert (no `--insecure` needed) |
| `SSH_KEY_NAME` | DO/AWS | SSH key name at provider (keypair name for AWS) |
| `SSH_PUBLIC_KEY_PATH` | Azure/GCP | Path to public key file |

## Structure

```
terraform/
├── digitalocean/    # DigitalOcean Droplet + Firewall
├── aws/             # AWS EC2 + Security Group
├── gcp/             # GCP Compute Engine + Firewall
└── azure/           # Azure VM + NSG + VNet
```

Each directory is self-contained with its own `main.tf`, `providers.tf`, and `variables.tf`.
