# Integration Architecture

**Project:** Command Center
**Generated:** 2026-02-21

## Overview

Command Center is a two-part system — a Go backend and a SvelteKit frontend — deployed as a single binary. The frontend is compiled to static files and embedded into the Go binary via `//go:embed`, eliminating the need for a separate web server or reverse proxy.

## Part Communication

### Build-Time Integration: Embedded Frontend

```
web/ (SvelteKit)                    Go Binary
┌─────────────────┐                ┌──────────────────┐
│ npm run build    │───embed.FS───▶│ embed.go          │
│ → web/build/     │               │ //go:embed        │
│   index.html     │               │ web/build          │
│   _app/          │               │                    │
│   ...            │               │ static.go serves   │
└─────────────────┘                │ files from FS      │
                                   └──────────────────┘
```

- `embed.go` declares `//go:embed web/build` directive
- `internal/server/static.go` serves embedded files as an SPA (fallback to `index.html`)
- In dev mode, `internal/server/devproxy.go` proxies to Vite dev server at `localhost:5173`

### Runtime Integration: SSE Event Stream

```
┌──────────────────────────────────────────────────────────────────┐
│ Go Backend (server)                                               │
│                                                                   │
│  ┌────────────┐                                                   │
│  │ Config     │──────────────────────────▶ Health Checker          │
│  │ Loader     │  (custom services)        (config-driven checks)  │
│  └────────────┘                                                   │
│                                                                   │
│  ┌────────────┐    ┌────────────┐                                 │
│  │ Secrets    │───▶│ Auth       │──────────▶ Health Checker        │
│  │ Decryptor  │    │ (OIDC)     │  (token)   (authenticated retry)│
│  └────────────┘    └────────────┘                                 │
│                                                                   │
│  ┌──────────┐    ┌─────────┐    ┌──────────────┐                  │
│  │ K8s      │───▶│ State   │───▶│ SSE Broker   │──── /api/events ▶│ Browser
│  │ Watcher  │    │ Store   │    │              │                  │
│  └──────────┘    └────┬────┘    └──────────────┘                  │
│                       │                                           │
│  ┌──────────┐         │         ┌──────────────┐                  │
│  │ Health   │─────────┘────────▶│ History      │                  │
│  │ Checker  │                   │ Persistence  │                  │
│  └──────────┘                   │ (JSONL)      │                  │
│                                 └──────────────┘                  │
└──────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│ SvelteKit Frontend (web)                             │
│                                                      │
│  ┌──────────┐    ┌──────────────┐    ┌───────────┐  │
│  │ SSE      │───▶│ Service      │───▶│ UI        │  │
│  │ Client   │    │ Store        │    │ Components │  │
│  │          │    │ ($state)     │    │           │  │
│  └──────────┘    └──────────────┘    └───────────┘  │
└─────────────────────────────────────────────────────┘
```

**Data flow:**

1. **Config Loader** reads YAML config → registers custom services with Health Checker
2. **Secrets Decryptor** decrypts secrets file → **Auth (OIDC)** acquires tokens
3. **K8s Watcher** watches Ingress resources → discovers/removes services
4. **Health Checker** periodically probes discovered service URLs, with authenticated retry via OIDC tokens when available
5. Both K8s Watcher and Health Checker feed into **State Store** (thread-safe, mutex-protected)
6. Health Checker results are also persisted to **History Persistence** (JSONL files in data-dir)
7. Store emits events → **SSE Broker** broadcasts to all connected clients
8. Frontend **SSE Client** receives events via `EventSource` at `GET /api/events`
9. SSE Client updates **Service Store** (Svelte 5 `$state` rune)
10. Reactive UI re-renders automatically

### OIDC Authentication Flow

When OIDC is configured (via config + encrypted secrets):

1. Secrets decryption resolves OIDC client credentials
2. OIDC client performs token acquisition from the issuer
3. Health checker uses acquired tokens for authenticated retries against protected endpoints
4. OIDC status is included in `state` event payloads (`oidcStatus` field)
5. Endpoint discovery uses OIDC issuer's well-known configuration

### SSE Event Types

| Event Type | Direction | Payload | Purpose |
|-|-|-|-|
| `state` | Server → Client | Full service state snapshot | Initial connection / reconnection |
| `discovered` | Server → Client | New service details | Service added to K8s |
| `removed` | Server → Client | Service identifier | Service removed from K8s |
| `k8sStatus` | Server → Client | K8s connection status | Watcher health indicator |
| `update` | Server → Client | Updated service details | Service health or metadata changed |

Note: The `state` event payload includes an optional `oidcStatus` field when OIDC is configured.

### Connection Resilience

- **SSE auto-reconnect**: Frontend SSE client implements exponential backoff reconnection
- **State caching**: Backend caches last-known state so reconnecting clients get immediate snapshot
- **Data freshness**: Frontend displays staleness indicators when data is not current
- **Configuration hot-reload**: Config file changes are detected and applied without restart
- **Health history persistence**: Health check results are persisted to JSONL files in data-dir, surviving restarts

## Shared Dependencies

| Dependency | Used By | Purpose |
|-|-|-|
| Service type definition | Both (Go struct + TS type) | Service entity representation |
| SSE event schema | Both (Go serialization + TS parsing) | Event contract |
| Health status enum | Both (Go constants + TS union types) | Status classification |

## Development Integration

| Mode | Frontend Serving | SSE Endpoint | How |
|-|-|-|-|
| Production | `embed.FS` (compiled into binary) | `:8443/api/events` | Single binary |
| Development | Vite dev server (`:5173`) | `:8443/api/events` | `make dev` starts both, Go proxies `/` to Vite |
