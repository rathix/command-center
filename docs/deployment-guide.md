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
| Base image | `gcr.io/distroless/static-debian12:nonroot` |
| User | `nonroot:nonroot` (UID 65532) |
| Exposed port | `8443` |
| Entrypoint | `/command-center` |
| Architecture | linux/amd64 |

## Deployment Options

### Docker (Recommended for Homelab)

```bash
docker run -d \
  --name command-center \
  --restart unless-stopped \
  --read-only \
  --security-opt no-new-privileges:true \
  --cap-drop ALL \
  -p 8443:8443 \
  -v /path/to/kubeconfig:/data/kubeconfig:ro \
  -v command-center-data:/data \
  -e KUBECONFIG=/data/kubeconfig \
  ghcr.io/rathix/command-center:latest
```

**Volume mounts:**
- Kubeconfig (`/data/kubeconfig:ro`): Read-only mount for K8s API access
- Data volume (`command-center-data:/data`): Writable — persists auto-generated certificates and health history

**Security flags:**
- `--read-only`: Read-only root filesystem — the container cannot write except to volume mounts
- `--security-opt no-new-privileges:true`: Prevents privilege escalation via setuid/setgid binaries
- `--cap-drop ALL`: Drops all Linux capabilities (none are needed — port 8443 > 1024)

### Docker with Custom Certificates

```bash
docker run -d \
  --name command-center \
  --restart unless-stopped \
  --read-only \
  --security-opt no-new-privileges:true \
  --cap-drop ALL \
  -p 8443:8443 \
  -v /path/to/kubeconfig:/data/kubeconfig:ro \
  -e KUBECONFIG=/data/kubeconfig \
  -e TLS_CA_CERT=/certs/ca.crt \
  -e TLS_CERT=/certs/server.crt \
  -e TLS_KEY=/certs/server.key \
  -v /path/to/certs:/certs:ro \
  ghcr.io/rathix/command-center:latest
```

All three certificate paths must be provided together.

### Container Security

The container is hardened with defense-in-depth:

| Layer | Enforcement | Purpose |
|-|-|-|
| Base image | `distroless/static-debian12:nonroot` | No shell, no package manager, minimal attack surface |
| User | `nonroot:nonroot` (UID 65532) | Non-root process — cannot modify system files |
| Root filesystem | `--read-only` / `readOnlyRootFilesystem: true` | Prevents runtime file modification |
| Capabilities | `--cap-drop ALL` / `drop: [ALL]` | No Linux capabilities granted |
| Privilege escalation | `--security-opt no-new-privileges` / `allowPrivilegeEscalation: false` | No setuid/setgid execution |

**Writable paths** (via volume mounts only):

| Path | Purpose | Mount type |
|-|-|-|
| `/data/certs/` | Auto-generated TLS certificates (CA, server, client) | Named volume (read-write) |
| `/data/history.jsonl` | Health check history persistence | Named volume (read-write) |

**Read-only mounts:**

| Path | Purpose |
|-|-|
| `/data/kubeconfig` | Kubernetes API access |
| `/data/config.yaml` | Optional YAML config file |

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
