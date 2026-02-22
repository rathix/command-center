# Architecture — Server (Go Backend)

**Project:** Command Center
**Part:** server
**Type:** backend
**Generated:** 2026-02-21

## Executive Summary

The server is a Go 1.26 backend that serves as the core of Command Center. It watches Kubernetes Ingress resources, performs HTTP health checks on discovered services, maintains thread-safe state, and streams real-time updates to browser clients via Server-Sent Events (SSE). It also embeds and serves the compiled SvelteKit frontend as a single binary.

## Technology Stack

| Category | Technology | Version |
|-|-|-|
| Language | Go | 1.26 |
| HTTP | Standard library `net/http` | - |
| K8s Client | k8s.io/client-go | v0.35.1 |
| TLS | crypto/tls, crypto/x509 | stdlib |
| Crypto | golang.org/x/crypto (Argon2id) | - |
| Config | gopkg.in/yaml.v3 | v3 |
| Build | Make + go build | - |
| Container | Docker (distroless) | - |

## Architecture Pattern

**Service/Handler pattern** using Go standard library conventions:

- `cmd/command-center/` — Application bootstrap and configuration
- `cmd/encrypt-secrets/` — CLI tool for encrypting secrets files
- `internal/` — Domain packages (no external consumers)
- Each package has a single responsibility with clean interfaces
- No dependency injection framework — constructor functions with explicit dependencies
- No third-party HTTP router — `http.NewServeMux()` with Go 1.22+ patterns

## Package Architecture

### cmd/command-center/

Entry point and orchestration. Parses CLI flags and environment variables, initializes all internal components, wires dependencies, and manages graceful shutdown via `context.Context` and OS signal handling.

### cmd/encrypt-secrets/

CLI tool for encrypting secrets files. Takes a plaintext YAML secrets file and a passphrase, produces an encrypted secrets file using Argon2id key derivation and AES-256-GCM encryption.

### internal/certs/

Automatic mTLS certificate management. Generates self-signed CA, server, and client certificates on first run. Certificates are persisted to disk (`DATA_DIR/certs/`). Supports custom certificate paths as an alternative.

### internal/health/

HTTP health checker that periodically probes discovered service URLs. Configurable check interval via `HEALTH_INTERVAL`. Results feed into the state store, which triggers SSE updates.

### internal/k8s/

Kubernetes Ingress watcher using the informer pattern from client-go. Watches for Ingress resource events (add/update/delete) and translates them into service discovery events in the state store.

### internal/server/

HTTP serving layer with two modes:
- **Production** (`static.go`): Serves embedded SvelteKit build from `embed.FS` with SPA fallback routing
- **Development** (`devproxy.go`): Reverse proxy to Vite dev server at `localhost:5173` for HMR

### internal/sse/

Server-Sent Events implementation:
- **broker.go**: Client connection management, event broadcasting, initial state delivery
- **events.go**: Event type definitions (state, discovered, removed, k8s_status, health_update) and JSON serialization

### internal/state/

Thread-safe service state store using `sync.Mutex`. Stores discovered services with health status. Emits typed events on state changes that the SSE broker subscribes to.

### internal/auth/

OIDC client credentials flow for authenticated health checks. Includes endpoint discovery (`.well-known/openid-configuration`) and token management with caching and refresh.

### internal/config/

YAML configuration loading with file watcher for hot-reload. Supports custom service definitions, service grouping, and icon assignments. File changes trigger live reconfiguration without restart.

### internal/history/

JSONL-based health history persistence. Writer appends health check results, reader restores state on startup, pruner removes stale entries to bound file size. Designed for crash-safe operation.

### internal/secrets/

Encrypted secrets file decryption using Argon2id key derivation and AES-256-GCM authenticated encryption. Decrypts OIDC credentials and other sensitive configuration at startup.

### internal/session/

SSE session tracking. Manages client connection lifecycle, tracks active sessions, and provides middleware for session-aware request handling.

## API Design

| Endpoint | Method | Handler | Purpose |
|-|-|-|-|
| `/api/events` | GET | SSE Broker | EventSource stream for real-time updates |
| `/` | GET | SPA Handler | Catch-all serving embedded frontend |

## Data Architecture

### Key Data Structures

| Struct | Package | Purpose |
|-|-|-|
| Service | internal/state | Domain entity: name, URL, health status, timestamps |
| Store | internal/state | State container with event emission |
| Event | internal/state | Typed state change event |
| Broker | internal/sse | SSE client management and broadcasting |
| Watcher | internal/k8s | K8s Ingress informer wrapper |
| OIDCClient | internal/auth | Token acquisition via client credentials flow |
| EndpointDiscoverer | internal/auth | OIDC `.well-known` endpoint resolution |
| OIDCCredentials | internal/secrets | Decrypted OIDC client ID/secret pair |
| AppConfig | internal/config | Parsed YAML configuration (services, groups, icons) |
| HistoryWriter | internal/history | Append-only JSONL health result writer |
| HistoryReader | internal/history | JSONL health history reader for startup restoration |

### Data Flow

```
Kubernetes API → Watcher → State Store → SSE Broker → Browser (EventSource)
                              ↑
Config ──→ Health Checker ────┘
              ↑
Secrets ──→ Auth (OIDC)

History Writer ←── Health Checker
History Reader ──→ State Store (startup)
```

## Security Architecture

- **mTLS enforcement**: All connections require mutual TLS (TLS 1.3 minimum)
- **Auto-generated certificates**: Self-signed CA on first run, auto-renewal on expiry
- **Custom certificate support**: Override with user-provided CA, server cert, and key
- **Client authentication**: Browser must present client certificate signed by trusted CA
- **Distroless container**: Minimal attack surface in production image
- **Non-root execution**: Container runs as user 65532
- **Trivy scanning**: CVE detection in CI/CD pipeline
- **SLSA attestation**: Build provenance tracking

## Testing Strategy

| File | Package | Focus |
|-|-|-|
| main_test.go | cmd | Server lifecycle, configuration |
| generator_test.go | certs | Certificate generation, validation |
| checker_test.go | health | Health check logic |
| watcher_test.go | k8s | K8s informer behavior |
| static_test.go | server | SPA serving, fallback routing |
| devproxy_test.go | server | Dev proxy behavior |
| broker_test.go | sse | Client management, broadcasting |
| events_test.go | sse | Event serialization |
| store_test.go | state | State mutations, event emission |
| embed_test.go | root | Embed directive validation |
| oidc_test.go | auth | OIDC client credentials flow |
| endpoint_discovery_test.go | auth | OIDC endpoint resolution |
| loader_test.go | config | YAML config loading |
| watcher_test.go | config | File watcher hot-reload |
| registration_test.go | config | Service registration |
| writer_test.go | history | JSONL append writer |
| reader_test.go | history | History reader/restore |
| pruner_test.go | history | Stale entry pruning |
| decrypt_test.go | secrets | Secrets decryption |
| session_test.go | session | Session tracking |
| middleware_test.go | session | Session middleware |
| main_test.go | encrypt-secrets | Encrypt CLI tool |

## Configuration

All flags have environment variable equivalents. Precedence: CLI flag > env var > default.

| Flag | Env Var | Default | Purpose |
|-|-|-|-|
| `--listen-addr` | `LISTEN_ADDR` | `:8443` | Server bind address |
| `--kubeconfig` | `KUBECONFIG` | `~/.kube/config` | K8s config path |
| `--health-interval` | `HEALTH_INTERVAL` | `30s` | Health check period |
| `--data-dir` | `DATA_DIR` | `/data` | Certificate storage |
| `--log-format` | `LOG_FORMAT` | `json` | Log output format |
| `--tls-ca-cert` | `TLS_CA_CERT` | *(auto)* | Custom CA cert |
| `--tls-cert` | `TLS_CERT` | *(auto)* | Custom server cert |
| `--tls-key` | `TLS_KEY` | *(auto)* | Custom server key |
| `--config` | `CONFIG_FILE` | *(none)* | YAML configuration file path |
| `--secrets` | `SECRETS_FILE` | *(none)* | Encrypted secrets file path |
| - | `SECRETS_KEY` | *(none)* | Passphrase for secrets decryption |
| `--dev` | `DEV` | `false` | Enable dev mode (Vite proxy, no TLS) |

## Deployment Architecture

- **Single binary**: Go binary with embedded frontend (`embed.FS`)
- **Docker**: Multi-stage build → `gcr.io/distroless/static-debian12`
- **Registry**: `ghcr.io/rathix/command-center`
- **CI/CD**: GitHub Actions (test → build → push → scan → attest)
- **Runtime**: Kubernetes or Docker on TrueNAS homelab
