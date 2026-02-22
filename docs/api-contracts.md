# API Contracts

**Project:** Command Center
**Updated:** 2026-02-22

## Endpoints

### GET /api/events

**Type:** Server-Sent Events (SSE)
**Authentication:** mTLS client certificate (production) / None (dev mode)
**Content-Type:** `text/event-stream`

Establishes a persistent SSE connection. The server sends an initial full state snapshot, then streams incremental updates as services are discovered, removed, or health-checked.

#### Event Types

**`state` — Full State Snapshot**
Sent on initial connection, reconnection, and when config errors change. Contains complete service list, K8s status, health check interval, and optional OIDC status.

```
event: state
data: {"appVersion":"0.8.0","services":[{"name":"my-service","displayName":"My Service","namespace":"default","group":"apps","url":"https://my-service.example.com","source":"kubernetes","status":"healthy","httpCode":200,"responseTimeMs":42,"lastChecked":"2026-02-22T10:00:00Z","lastStateChange":"2026-02-22T09:55:00Z","errorSnippet":null,"authMethod":"oidc"}],"k8sConnected":true,"k8sLastEvent":"2026-02-22T09:50:00Z","healthCheckIntervalMs":30000,"configErrors":[],"oidcStatus":{"connected":true,"providerName":"PocketID","tokenState":"valid","lastSuccess":"2026-02-22T09:59:00Z"}}
```

**`discovered` — Service Discovered**
Sent when a new Kubernetes Ingress resource is detected or a config service is loaded for the first time.

```
event: discovered
data: {"name":"my-service","displayName":"My Service","namespace":"default","group":"apps","url":"https://my-service.example.com","source":"kubernetes","status":"unknown","httpCode":null,"responseTimeMs":null,"lastChecked":null,"lastStateChange":null,"errorSnippet":null,"authMethod":"oidc"}
```

**`update` — Service Updated**
Sent when an existing service's health check completes or its config properties change.

```
event: update
data: {"name":"my-service","displayName":"My Service","namespace":"default","group":"apps","url":"https://my-service.example.com","source":"kubernetes","status":"healthy","httpCode":200,"responseTimeMs":42,"lastChecked":"2026-02-22T10:00:00Z","lastStateChange":"2026-02-22T09:55:00Z","errorSnippet":null,"authMethod":"oidc"}
```

**`removed` — Service Removed**
Sent when a Kubernetes Ingress resource is deleted.

```
event: removed
data: {"name":"my-service","namespace":"default"}
```

**`k8sStatus` — Kubernetes Connection Status**
Sent when the K8s watcher connection state changes.

```
event: k8sStatus
data: {"k8sConnected":true,"k8sLastEvent":"2026-02-22T09:50:00Z"}
```

#### Event Payload Summary

| Event | Payload Fields |
|-|-|
| `state` | `appVersion`, `services[]`, `k8sConnected`, `k8sLastEvent`, `healthCheckIntervalMs`, `configErrors[]`, `oidcStatus?` |
| `discovered` | `name`, `displayName`, `namespace`, `group`, `url`, `icon?`, `source`, `status`, `httpCode`, `responseTimeMs`, `lastChecked`, `lastStateChange`, `errorSnippet`, `authMethod?` |
| `update` | Same fields as `discovered` |
| `removed` | `name`, `namespace` |
| `k8sStatus` | `k8sConnected`, `k8sLastEvent` |

### GET / (catch-all)

**Type:** Static file serving
**Authentication:** mTLS client certificate (production) / None (dev mode)

Serves the embedded SvelteKit SPA. All paths that don't match `/api/*` are served from `embed.FS` with `index.html` fallback for SPA client-side routing.

In dev mode (`--dev`), this route proxies all requests to the Vite dev server at `http://localhost:5173`.

## Configuration

### CLI Flags

| Flag | Env Var | Description |
|-|-|-|
| `--dev` | — | Enable dev mode (Vite proxy, relaxed auth) |
| `--config` | — | Path to YAML config file |
| `--secrets` | `SECRETS_FILE` | Path to encrypted secrets file |
| — | `SECRETS_KEY` | Passphrase to decrypt the secrets file |

The secrets file uses AES-256-GCM encryption with an Argon2id-derived key. It contains OIDC client credentials (`clientId`, `clientSecret`). When `--secrets` is not provided, OIDC authentication is disabled.

## Connection Behavior

| Aspect | Behavior |
|-|-|
| Protocol | HTTP/1.1 with SSE (text/event-stream) |
| Keep-alive | Persistent connection until client disconnects |
| Keepalive comments | Server sends `:keepalive` comment every 15s when idle |
| Reconnect | Client implements exponential backoff |
| Initial data | Server sends `state` event with full snapshot on connect |
| Compression | None (SSE standard) |
| Auth | mTLS — client must present valid certificate |
