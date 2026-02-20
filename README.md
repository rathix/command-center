# Command Center

A Kubernetes service dashboard for homelab operators. Single Go binary serving a SvelteKit SPA with mTLS security and a Catppuccin Mocha terminal aesthetic.

> **Status:** Deployable shell complete (Epic 1). Service discovery, health monitoring, and real-time updates are in progress.

## Quick Start

Pull the image from GHCR and run with your kubeconfig mounted:

```bash
docker run -d \
  -p 8443:8443 \
  -v /path/to/kubeconfig:/home/nonroot/.kube/config:ro \
  -v command-center-data:/data \
  ghcr.io/rathix/command-center:latest
```

On first start, the server generates a self-signed CA and TLS certificates in `/data/certs/`. Install the client certificate in your browser to access the dashboard:

```bash
# Copy certs from the container
docker cp <container>:/data/certs/client.crt .
docker cp <container>:/data/certs/client.key .
```

Import `client.crt` and `client.key` into your browser's certificate manager, then navigate to `https://<host>:8443`.

## Building from Source

**Prerequisites:** Go 1.26+, Node 22+

```bash
make build        # Build frontend + Go binary → bin/command-center
make docker       # Build multi-stage Docker image
make test         # Run Go and frontend tests
make dev          # Dev mode: Vite HMR + Go server with --dev flag
make clean        # Remove build artifacts
```

The build embeds the SvelteKit static output into the Go binary via `//go:embed`, producing a single self-contained executable.

## Configuration

All parameters support CLI flags and environment variables. Precedence: CLI flag > env var > default.

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--listen-addr` | `LISTEN_ADDR` | `:8443` | Server listen address |
| `--kubeconfig` | `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig |
| `--health-interval` | `HEALTH_INTERVAL` | `30s` | Health check interval (Go duration) |
| `--data-dir` | `DATA_DIR` | `/data` | Directory for auto-generated certificates |
| `--log-format` | `LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `--tls-ca-cert` | `TLS_CA_CERT` | *(auto-generated)* | Custom CA certificate path |
| `--tls-cert` | `TLS_CERT` | *(auto-generated)* | Custom server certificate path |
| `--tls-key` | `TLS_KEY` | *(auto-generated)* | Custom server key path |

See `.env.example` for a documented template.

## mTLS & Certificates

Command Center enforces mutual TLS on all connections. TLS 1.3 minimum.

**Auto-generated certificates (default):**

On first startup, the server creates a self-signed CA, server certificate, and client certificate in `{data-dir}/certs/`. The CA is valid for 10 years; leaf certificates are valid for 1 year and renewed automatically on expiry.

Generated files:
- `ca.crt` / `ca.key` — Certificate authority
- `server.crt` / `server.key` — Server identity
- `client.crt` / `client.key` — Browser client identity

Install `client.crt` and `client.key` in your browser to authenticate. The exact steps vary by browser:

- **Chrome/Edge:** Settings > Privacy and Security > Security > Manage certificates > Import
- **Firefox:** Settings > Privacy & Security > Certificates > View Certificates > Import
- **macOS Safari:** Double-click the `.crt` file to add to Keychain, then trust it

**Custom certificates:**

To use your own CA and server certificates, set all three paths:

```bash
docker run -d \
  -p 8443:8443 \
  -v /path/to/kubeconfig:/home/nonroot/.kube/config:ro \
  -e TLS_CA_CERT=/certs/ca.crt \
  -e TLS_CERT=/certs/server.crt \
  -e TLS_KEY=/certs/server.key \
  -v /path/to/certs:/certs:ro \
  ghcr.io/rathix/command-center:latest
```

All three must be provided together — partial configuration is rejected.

## Deployment

**Docker image tags:**

| Tag | Description |
|-----|-------------|
| `latest` | Most recent build from `main` |
| `sha-<short>` | Pinned to a specific commit |
| `x.y.z` | Semantic version (from `v*` tags) |

**Typical deployment on TrueNAS:**

```bash
docker pull ghcr.io/rathix/command-center:latest

docker run -d \
  --name command-center \
  --restart unless-stopped \
  -p 8443:8443 \
  -v /path/to/kubeconfig:/home/nonroot/.kube/config:ro \
  -v command-center-data:/data \
  ghcr.io/rathix/command-center:latest
```

- Kubeconfig mounted read-only — the server only reads from the Kubernetes API
- Named volume for `/data` persists certificates across container restarts
- Graceful shutdown on `docker stop` (SIGTERM handled)

## Development

```bash
make dev
```

This starts the Vite dev server (port 5173) for frontend hot-reload and the Go server in `--dev` mode, which proxies frontend requests to Vite instead of serving embedded static files.

**Project structure:**

```
cmd/command-center/     Go entrypoint, config, server lifecycle
internal/certs/         TLS certificate generation and management
internal/server/        HTTP handlers, SPA serving, dev proxy
web/src/                SvelteKit frontend (Svelte 5, TypeScript, Tailwind v4)
embed.go                //go:embed directive for web/build
Dockerfile              Multi-stage build: Node → Go → distroless
Makefile                Build, test, dev, docker targets
```

## License

MIT
