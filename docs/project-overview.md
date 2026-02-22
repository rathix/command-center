# Project Overview — Command Center

**Generated:** 2026-02-21

## Purpose

Command Center is a Kubernetes service dashboard for homelab operators. It provides a single-pane-of-glass view of Kubernetes Ingress-discovered services with real-time health monitoring, delivered as a self-contained Go binary with an embedded SvelteKit SPA.

## Project Classification

| Field | Value |
|-|-|
| Repository Type | Multi-part |
| Architecture | Go backend + SvelteKit frontend (embedded SPA) |
| Deployment | Single binary, Docker (distroless), mTLS-secured |
| Status | v1.0.0 — Epic 8 complete (MVP) |

## Technology Stack Summary

| Layer | Technology | Version |
|-|-|-|
| Backend Language | Go | 1.26 |
| HTTP Server | Standard library `net/http` | - |
| Kubernetes Client | k8s.io/client-go | v0.35.1 |
| Frontend Framework | SvelteKit (adapter-static) | ^2.50.2 |
| UI Framework | Svelte 5 (runes) | ^5.49.2 |
| Styling | Tailwind CSS v4 | ^4.2.0 |
| Build Tool | Vite | ^7.3.1 |
| Testing | Go test + Vitest | - |
| Container | Docker (multi-stage → distroless) | - |
| CI/CD | GitHub Actions → GHCR | - |
| Security | mTLS (auto-generated certs), secrets encryption (Argon2id + AES-256-GCM), OIDC authentication, Trivy, SLSA | - |

## Architecture Overview

```
Browser ◄──mTLS──► Go Server ◄──► Kubernetes API
  │                    │
  │◄──SSE events───────│
  │                    │
  SvelteKit SPA    embed.FS
  (Svelte 5)       (compiled in)
```

**Key architectural decisions:**
- **Single binary**: Frontend embedded via `//go:embed` — no separate web server needed
- **No third-party router**: Standard library `net/http.NewServeMux()` with Go 1.22+ patterns
- **SSE over WebSocket**: Simpler protocol for unidirectional server → client streaming
- **Svelte 5 runes**: `$state` for fine-grained reactivity without traditional stores
- **mTLS by default**: Security-first for homelab exposure

## Repository Structure

| Part | Path | Type | Purpose |
|-|-|-|-|
| server | `/` (root) | backend | Go binary: K8s watcher, health checker, SSE broker, HTTP server |
| web | `/web` | web | SvelteKit SPA: dashboard UI with real-time service display |

## Detailed Documentation

- [Architecture — Server](./architecture-server.md)
- [Architecture — Web](./architecture-web.md)
- [Integration Architecture](./integration-architecture.md)
- [Source Tree Analysis](./source-tree-analysis.md)
- [Component Inventory](./component-inventory.md)
- [Development Guide](./development-guide.md)

## Epic History

| Epic | Focus | Status |
|-|-|-|
| Epic 1 | Deployable shell (scaffold, mTLS, config, UI shell, Docker, CI/CD) | Complete |
| Epic 2 | Service discovery (K8s watcher, SSE streaming, frontend client, service list) | Complete |
| Epic 3 | Health monitoring (HTTP checker, status indicators, sorting, tooltips) | Complete |
| Epic 4 | Resilience (SSE reconnection, state caching, data freshness) | Complete |
| Epic 5 | Service grouping & icons (group headers, service icons, grouped list) | Complete |
| Epic 6 | Custom services & configuration (YAML config, file watcher, hot-reload) | Complete |
| Epic 7 | Health history persistence (JSONL writer/reader/pruner, startup restoration) | Complete |
| Epic 8 | Authenticated health checks (secrets encryption, OIDC client, endpoint discovery, auth retry, SSE status, frontend indicators) | Complete |
