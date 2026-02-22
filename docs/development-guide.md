# Development Guide

**Project:** Command Center
**Generated:** 2026-02-21

## Prerequisites

| Requirement | Version | Purpose |
|-|-|-|
| Go | 1.26+ | Backend compilation |
| Node.js | 22+ | Frontend build tooling |
| npm | (bundled with Node) | Package management |
| Docker or Podman | Latest | Container builds (optional for dev) |
| kubectl + kubeconfig | - | Kubernetes API access (runtime requirement) |

## Environment Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/rathix/command-center.git
   cd command-center
   ```

2. **Install frontend dependencies:**
   ```bash
   cd web && npm ci && cd ..
   ```

3. **Configure environment (optional):**
   ```bash
   cp .env.example .env
   # Edit .env as needed — all values have sensible defaults
   ```

4. **Ensure kubeconfig is accessible:**
   ```bash
   # Default: ~/.kube/config
   # Override: KUBECONFIG=/path/to/config or --kubeconfig flag
   ```

## Development Workflow

### Local Development

```bash
make dev
```

This starts two processes:
- **Vite dev server** on port `5173` with hot module replacement (HMR)
- **Go server** on port `8443` in `--dev` mode (proxies `/` to Vite, no TLS)

Frontend changes reload instantly. Backend changes require restarting `make dev`.

### Build

```bash
make build
```

1. Builds frontend: `cd web && npm ci && npm run build`
2. Compiles Go binary with embedded frontend: `go build -ldflags ... -o bin/command-center`
3. Output: `bin/command-center`

### Testing

```bash
make test
```

Runs both test suites:
- **Go tests**: `go test ./...` (11 packages)
- **Frontend tests**: `cd web && npx vitest run` (15 test files, 297 tests)

Individual test commands:
```bash
# Go tests only
go test ./...
go test -v ./internal/state/...    # Specific package

# Frontend tests only
cd web && npx vitest run
cd web && npx vitest run --watch   # Watch mode
```

### Docker Build

```bash
make docker
```

Multi-stage build: Node 22 → Go 1.26 → distroless image.

### Linting & Type Checking

```bash
cd web
npm run lint          # ESLint
npm run check         # svelte-check (TypeScript)
npm run format        # Prettier auto-format
```

## Make Targets

| Target | Command | Purpose |
|-|-|-|
| `build` | `make build` | Build frontend + Go binary → `bin/command-center` |
| `test` | `make test` | Run all Go and frontend tests |
| `dev` | `make dev` | Dev mode with Vite HMR + Go server |
| `docker` | `make docker` | Build Docker image |
| `clean` | `make clean` | Remove `bin/`, `web/build/`, `web/.svelte-kit/` |

## Project Conventions

### Go Backend
- Standard library `net/http` — no third-party routers
- `internal/` packages — unexported to external consumers
- Constructor functions for dependency injection (no DI framework)
- `context.Context` for cancellation and graceful shutdown
- Table-driven tests

### SvelteKit Frontend
- Svelte 5 runes (`$state`, `$derived`) — not legacy stores
- TypeScript strict mode
- Tailwind v4 utility classes (no component CSS)
- Co-located tests (`Component.svelte` → `Component.test.ts`)
- Catppuccin Mocha color theme

### Versioning
- Semantic versioning with git tags
- Story complete → patch bump (v0.2.1)
- Epic complete → minor bump (v0.3.0)
- Version injected at build time via `-ldflags`

### CI/CD Pipeline
- Trigger: push to `main`, version tags (`v*`), PRs
- Jobs: `test-web` → `test-go` → `publish`
- Publish: Docker build → GHCR push → Trivy scan → SLSA attestation
- PRs get tests only (no publish)

## Configuration Reference

All flags support environment variable equivalents. Precedence: CLI flag > env var > default.

| Flag | Env Var | Default | Description |
|-|-|-|-|
| `--listen-addr` | `LISTEN_ADDR` | `:8443` | Server listen address |
| `--kubeconfig` | `KUBECONFIG` | `~/.kube/config` | Kubeconfig path |
| `--health-interval` | `HEALTH_INTERVAL` | `30s` | Health check interval |
| `--data-dir` | `DATA_DIR` | `/data` | Certificate storage directory |
| `--log-format` | `LOG_FORMAT` | `json` | Log format (json/text) |
| `--tls-ca-cert` | `TLS_CA_CERT` | *(auto)* | Custom CA certificate |
| `--tls-cert` | `TLS_CERT` | *(auto)* | Custom server certificate |
| `--tls-key` | `TLS_KEY` | *(auto)* | Custom server key |
| `--dev` | `DEV` | `false` | Dev mode (Vite proxy, no TLS) |
| `--config` | `CONFIG_FILE` | *(none)* | Path to YAML config file for custom services |
| `--history-file` | `HISTORY_FILE` | *(none)* | Path to history JSONL file |
| `--session-duration` | `SESSION_DURATION` | `24h` | Browser session cookie duration |

## Common Tasks

### Adding a new internal package
1. Create directory under `internal/`
2. Add package with exported types/functions
3. Wire into `cmd/command-center/main.go`
4. Add tests (`*_test.go`)

### Adding a new Svelte component
1. Create `ComponentName.svelte` in `web/src/lib/components/`
2. Create co-located `ComponentName.test.ts`
3. Import and use in parent component

### Adding a new SSE event type
1. Define Go event struct in `internal/sse/events.go`
2. Add event emission in `internal/state/store.go`
3. Add TypeScript type in `web/src/lib/types.ts`
4. Handle in `web/src/lib/serviceStore.svelte.ts`
