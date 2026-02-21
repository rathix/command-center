# API Contracts

**Project:** Command Center
**Generated:** 2026-02-21

## Endpoints

### GET /api/events

**Type:** Server-Sent Events (SSE)
**Authentication:** mTLS client certificate (production) / None (dev mode)
**Content-Type:** `text/event-stream`

Establishes a persistent SSE connection. The server sends an initial full state snapshot, then streams incremental updates as services are discovered, removed, or health-checked.

#### Event Types

**`state` — Full State Snapshot**
Sent on initial connection and reconnection. Contains complete service map.

```
event: state
data: {"services": {"svc-1": {"name": "...", "url": "...", "health": "..."}, ...}, "k8s_status": "connected", "timestamp": "..."}
```

**`discovered` — Service Discovered**
Sent when a new Kubernetes Ingress resource is detected.

```
event: discovered
data: {"name": "my-service", "url": "https://my-service.example.com", "namespace": "default"}
```

**`removed` — Service Removed**
Sent when a Kubernetes Ingress resource is deleted.

```
event: removed
data: {"name": "my-service"}
```

**`k8s_status` — Kubernetes Connection Status**
Sent when the K8s watcher connection state changes.

```
event: k8s_status
data: {"status": "connected"}
```

**`health_update` — Health Check Result**
Sent after each health check cycle completes for a service.

```
event: health_update
data: {"name": "my-service", "health": "healthy", "status_code": 200, "latency_ms": 42, "checked_at": "..."}
```

### GET / (catch-all)

**Type:** Static file serving
**Authentication:** mTLS client certificate (production) / None (dev mode)

Serves the embedded SvelteKit SPA. All paths that don't match `/api/*` are served from `embed.FS` with `index.html` fallback for SPA client-side routing.

In dev mode (`--dev`), this route proxies all requests to the Vite dev server at `http://localhost:5173`.

## Connection Behavior

| Aspect | Behavior |
|-|-|
| Protocol | HTTP/1.1 with SSE (text/event-stream) |
| Keep-alive | Persistent connection until client disconnects |
| Reconnect | Client implements exponential backoff |
| Initial data | Server sends `state` event with full snapshot on connect |
| Compression | None (SSE standard) |
| Auth | mTLS — client must present valid certificate |
