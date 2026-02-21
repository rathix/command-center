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
┌─────────────────────────────────────────────────────┐
│ Go Backend (server)                                  │
│                                                      │
│  ┌──────────┐    ┌─────────┐    ┌──────────────┐    │
│  │ K8s      │───▶│ State   │───▶│ SSE Broker   │────┼──── GET /api/events ───▶ Browser
│  │ Watcher  │    │ Store   │    │              │    │         (EventSource)
│  └──────────┘    └────┬────┘    └──────────────┘    │
│                       │                              │
│  ┌──────────┐         │                              │
│  │ Health   │─────────┘                              │
│  │ Checker  │                                        │
│  └──────────┘                                        │
└─────────────────────────────────────────────────────┘

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

1. **K8s Watcher** watches Ingress resources → discovers/removes services
2. **Health Checker** periodically probes discovered service URLs
3. Both feed into **State Store** (thread-safe, mutex-protected)
4. Store emits events → **SSE Broker** broadcasts to all connected clients
5. Frontend **SSE Client** receives events via `EventSource` at `GET /api/events`
6. SSE Client updates **Service Store** (Svelte 5 `$state` rune)
7. Reactive UI re-renders automatically

### SSE Event Types

| Event Type | Direction | Payload | Purpose |
|-|-|-|-|
| `state` | Server → Client | Full service state snapshot | Initial connection / reconnection |
| `discovered` | Server → Client | New service details | Service added to K8s |
| `removed` | Server → Client | Service identifier | Service removed from K8s |
| `k8s_status` | Server → Client | K8s connection status | Watcher health indicator |
| `health_update` | Server → Client | Service health result | Health check completed |

### Connection Resilience

- **SSE auto-reconnect**: Frontend SSE client implements exponential backoff reconnection
- **State caching**: Backend caches last-known state so reconnecting clients get immediate snapshot
- **Data freshness**: Frontend displays staleness indicators when data is not current

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
