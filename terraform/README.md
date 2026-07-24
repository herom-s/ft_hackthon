# OpenTofu — Deploy ft_hackthon on Any Cloud

## Usage

Add to `.env` at the project root:

```env
CLOUD_PROVIDER=digitalocean   # or aws, gcp, azure
DIGITALOCEAN_TOKEN=your_token # cloud-specific credentials
```

Then:

```bash
make deploy
```

`make deploy-info` shows the VM IP, `make deploy-destroy` tears it down.

## Provider credentials

Each provider reads its standard env vars (add to `.env`):

| Provider      | Env vars |
|---------------|----------|
| DigitalOcean  | `DIGITALOCEAN_TOKEN` |
| AWS           | `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` |
| GCP           | `GOOGLE_CREDENTIALS` (path to service account JSON) |
| Azure         | `ARM_CLIENT_ID` + `ARM_CLIENT_SECRET` + `ARM_SUBSCRIPTION_ID` + `ARM_TENANT_ID` |

## Structure

```
terraform/
├── digitalocean/    # DigitalOcean Droplet
├── aws/             # AWS EC2
├── gcp/             # GCP Compute Engine
└── azure/           # Azure VM
```

Each directory is self-contained with its own `main.tf`, `providers.tf`, and `variables.tf`.
