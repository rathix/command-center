# Command Center

A Kubernetes service dashboard for homelab operators. Single Go binary serving a SvelteKit SPA with mTLS security and a Catppuccin Mocha terminal aesthetic.

> **Status:** v1.0.0 — MVP (service discovery, health monitoring, real-time updates, connection resilience).

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
make build              # Build frontend + Go binary → bin/command-center
make docker             # Build multi-stage Docker image
make test               # Run Go and frontend tests
make dev                # Dev mode: Vite HMR + Go server with --dev flag
make encrypt-secrets    # Build encrypt-secrets CLI tool → bin/encrypt-secrets
make clean              # Remove build artifacts
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
| `--config` | `CONFIG_FILE` | *(none)* | Path to YAML config file for custom services |
| `--history-file` | `HISTORY_FILE` | *(none)* | Path to history JSONL file |
| `--session-duration` | `SESSION_DURATION` | `24h` | Browser session cookie duration |
| `--secrets` | `SECRETS_FILE` | *(none)* | Path to encrypted secrets file |
| *(none)* | `SECRETS_KEY` | *(none)* | Decryption key for secrets file (env only) |
| `--dev` | `DEV` | `false` | Dev mode (Vite proxy, no TLS) |

See `.env.example` for a documented template.

## YAML Configuration

An optional YAML config file lets you define custom (non-Kubernetes) services, override properties of Kubernetes-discovered services, configure group metadata, and tune health checks — all without restarting.

Point to it with `--config /path/to/config.yaml` or `CONFIG_FILE=/path/to/config.yaml`.

**Hot-reload:** The file is watched via fsnotify. Edits are picked up automatically (~1 s debounce). If the new YAML is malformed, it is rejected and the last-known-good config stays active. Validation warnings (e.g. a service missing a required field) strip the invalid entry but keep the rest.

### Example config.yaml

```yaml
# Custom services — non-Kubernetes services to monitor
services:
  - name: truenas
    url: https://truenas.local
    group: storage
    displayName: TrueNAS
    icon: server
    healthUrl: https://truenas.local/api/v2.0/system/state
    expectedStatusCodes: [200]

  - name: pihole
    url: http://pihole.local
    group: network
    displayName: Pi-hole
    icon: shield

# Overrides — modify properties of Kubernetes-discovered services
# Match format: namespace/name (from the Ingress object)
overrides:
  - match: default/grafana
    displayName: Grafana Monitoring
    icon: chart-line
    healthUrl: https://grafana.local/api/health

  - match: media/jellyfin
    displayName: Jellyfin
    icon: film

# Groups — display metadata for service groups
groups:
  storage:
    displayName: Storage
    icon: database
    sortOrder: 1
  network:
    displayName: Network
    icon: wifi
    sortOrder: 2
  media:
    displayName: Media
    icon: play-circle
    sortOrder: 3

# Health — tune health check behavior (overrides --health-interval)
health:
  interval: 30s
  timeout: 10s
```

### Services

Each custom service requires `name`, `url`, and `group`. Optional fields:

| Field | Description |
|-|-|
| `displayName` | Label shown in the dashboard (defaults to `name`) |
| `icon` | Icon identifier for the UI |
| `healthUrl` | Full URL to probe for health checks instead of `url` |
| `expectedStatusCodes` | HTTP status codes treated as healthy (default: 200) |

Service names must be unique. Duplicates are stripped with a validation warning.

### Overrides

Override any Kubernetes-discovered service by matching its `namespace/name`. Only set the fields you want to change — unset fields keep their Kubernetes-discovered values. Removing an override restores the original values on the next reload.

### Groups

Groups referenced by services are created automatically. The `groups` map adds display metadata: a friendly name, icon, and sort order for the dashboard layout.

## Secrets Management

Sensitive values (e.g. OIDC client secrets) are stored in an encrypted secrets file. Use the `encrypt-secrets` CLI tool to encrypt a plaintext YAML file:

```bash
make encrypt-secrets
bin/encrypt-secrets -in secrets.yaml -out secrets.enc
```

**secrets.yaml format:**

```yaml
oidc:
  clientId: my-client-id
  clientSecret: ${OIDC_CLIENT_SECRET}
```

Values support `${ENV_VAR}` substitution — environment variables are resolved at load time.

At runtime, provide the encrypted file and decryption key:

```bash
export SECRETS_KEY=your-encryption-key
./bin/command-center --secrets secrets.enc
```

`SECRETS_KEY` is intentionally environment-variable only (no CLI flag) because CLI arguments are visible in `/proc/*/cmdline`.

## OIDC Authentication

OIDC authentication is optional. When configured, the health checker uses OIDC client credentials tokens for authenticated retries against protected endpoints.

Configure via YAML config file:

```yaml
oidc:
  issuerUrl: https://auth.example.com
  scopes:
    - openid
    - profile
```

The OIDC client discovers endpoints via the issuer's `.well-known/openid-configuration`. Token status is included in SSE `state` event payloads for frontend display.

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
- Named volume for `/data` persists certificates and health history across container restarts
- Graceful shutdown on `docker stop` (SIGTERM handled)

## Development

```bash
make dev
```

This starts the Vite dev server (port 5173) for frontend hot-reload and the Go server in `--dev` mode, which proxies frontend requests to Vite instead of serving embedded static files.

**Project structure:**

```
cmd/command-center/     Go entrypoint, config, server lifecycle
cmd/encrypt-secrets/    CLI tool for encrypting secrets files
internal/auth/          OIDC client credentials authentication
internal/certs/         TLS certificate generation and management
internal/config/        YAML config loading with hot-reload (fsnotify)
internal/health/        HTTP health checker with OIDC-authenticated retries
internal/history/       Health check history persistence (JSONL)
internal/k8s/           Kubernetes Ingress watcher (informer-based)
internal/secrets/       Encrypted secrets file decryption
internal/server/        HTTP handlers, SPA serving, dev proxy
internal/session/       Browser session cookie management
internal/sse/           Server-Sent Events broker and event types
internal/state/         Thread-safe service state store
web/src/                SvelteKit frontend (Svelte 5, TypeScript, Tailwind v4)
embed.go                //go:embed directive for web/build
Dockerfile              Multi-stage build: Node → Go → distroless
Makefile                Build, test, dev, docker targets
```

## Documentation

Full project documentation is available in [`docs/`](./docs/index.md), including architecture docs for both the Go backend and SvelteKit frontend, API contracts, data models, and development/deployment guides.

## License

MIT
