# Deployment Guide

**Project:** Command Center
**Generated:** 2026-02-21

## Docker Image

### Registry

Published to GitHub Container Registry: `ghcr.io/rathix/command-center`

### Image Tags

| Tag | Description |
|-|-|
| `latest` | Most recent tagged release (from `v*` tags) |
| `dev` | Most recent build from `main` branch |
| `sha-<short>` | Pinned to a specific commit |
| `x.y.z` | Semantic version (from `vx.y.z` tags) |
| `x.y` | Major.minor version |

### Image Details

| Property | Value |
|-|-|
| Base image | `gcr.io/distroless/static-debian12` |
| User | `65532:65532` (non-root) |
| Exposed port | `8443` |
| Entrypoint | `/command-center` |
| Architecture | linux/amd64 |

## Deployment Options

### Docker (Recommended for Homelab)

```bash
docker run -d \
  --name command-center \
  --restart unless-stopped \
  -p 8443:8443 \
  -v /path/to/kubeconfig:/home/nonroot/.kube/config:ro \
  -v command-center-data:/data \
  ghcr.io/rathix/command-center:latest
```

**Volume mounts:**
- Kubeconfig: Read-only mount for K8s API access
- Data volume: Persists auto-generated certificates and health history across restarts

### Docker with Custom Certificates

```bash
docker run -d \
  --name command-center \
  --restart unless-stopped \
  -p 8443:8443 \
  -v /path/to/kubeconfig:/home/nonroot/.kube/config:ro \
  -e TLS_CA_CERT=/certs/ca.crt \
  -e TLS_CERT=/certs/server.crt \
  -e TLS_KEY=/certs/server.key \
  -v /path/to/certs:/certs:ro \
  ghcr.io/rathix/command-center:latest
```

All three certificate paths must be provided together.

### Binary (Direct)

```bash
make build
./bin/command-center \
  --listen-addr :8443 \
  --kubeconfig ~/.kube/config \
  --data-dir /var/lib/command-center \
  --config /etc/command-center/config.yaml
```

## Certificate Setup

### Auto-Generated (Default)

On first startup, the server creates in `{data-dir}/certs/`:
- `ca.crt` / `ca.key` — Certificate authority (10-year validity)
- `server.crt` / `server.key` — Server identity (1-year, auto-renewed)
- `client.crt` / `client.key` — Browser client identity (1-year, auto-renewed)

### Client Certificate Installation

Extract client certificate from the container:
```bash
docker cp command-center:/data/certs/client.crt .
docker cp command-center:/data/certs/client.key .
```

Import into your browser:
- **Chrome/Edge:** Settings > Privacy > Security > Manage certificates > Import
- **Firefox:** Settings > Privacy > Certificates > View Certificates > Import
- **macOS Safari:** Double-click `.crt` to add to Keychain, then trust it

## Health History Persistence

Health check results are persisted as JSONL files in `{data-dir}/`. This allows the dashboard to display historical health data across restarts.

## CI/CD Pipeline

### GitHub Actions Workflow (`.github/workflows/publish.yml`)

**Trigger:** Push to `main`, version tags (`v*`), pull requests

**Jobs:**

| Job | Runs On | Purpose |
|-|-|-|
| `test-web` | ubuntu-latest | npm ci → lint → type-check → test → build frontend |
| `test-go` | ubuntu-latest | go test -v ./... |
| `publish` | ubuntu-latest | Docker build → GHCR push → Trivy scan → SLSA attestation |

**Publish conditions:**
- PRs: Tests only (no publish)
- Push to main: Build + push with `dev` and `sha-*` tags
- Version tags: Build + push with `latest`, version, and `sha-*` tags

**Security features:**
- Trivy vulnerability scanner (CRITICAL/HIGH, fails on findings)
- SLSA build provenance attestation
- Docker layer caching via GitHub Actions cache

## Monitoring

Command Center is the monitoring tool itself. To verify it's running:
- Browser: Navigate to `https://<host>:8443` (requires client cert)
- Docker: `docker logs command-center`
- Health: The SSE connection in-browser shows connection status and data freshness

## Graceful Shutdown

The server handles `SIGTERM` and `SIGINT` for clean shutdown:
- Stops accepting new SSE connections
- Closes existing SSE connections
- Stops K8s watcher
- Stops health checker
- Shuts down HTTP server with timeout
