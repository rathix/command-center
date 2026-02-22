# Data Models

**Project:** Command Center
**Updated:** 2026-02-22

## Overview

Command Center holds all state in-memory in a thread-safe store and streams it to clients via SSE. Data originates from Kubernetes Ingress resources, a YAML config file (custom services), an encrypted secrets file (OIDC credentials), and HTTP health checks. Health-status transitions are persisted to a JSONL history file for startup restoration.

## Go Data Structures (Server)

### Service (`internal/state`)

The core domain entity representing a discovered or configured service.

| Field | Type | JSON | Description |
|-|-|-|-|
| Name | string | `name` | Service identifier (from Ingress metadata or config) |
| DisplayName | string | `displayName` | Human-readable label |
| OriginalDisplayName | string | `originalDisplayName` | Pre-override display name (omitted if empty) |
| Namespace | string | `namespace` | Kubernetes namespace |
| Group | string | `group` | Logical grouping key |
| URL | string | `url` | Service URL |
| Icon | string | `icon` | Icon identifier (omitted if empty) |
| Source | string | `source` | Origin: `"kubernetes"` or `"config"` |
| Status | HealthStatus | `status` | Current health state |
| HTTPCode | *int | `httpCode` | Last HTTP health check status code (nullable) |
| ResponseTimeMs | *int64 | `responseTimeMs` | Last health check latency in ms (nullable) |
| LastChecked | *time.Time | `lastChecked` | Timestamp of last health check (nullable) |
| LastStateChange | *time.Time | `lastStateChange` | Timestamp of last status transition (nullable) |
| ErrorSnippet | *string | `errorSnippet` | Truncated error message (nullable) |
| AuthMethod | string | `authMethod` | Auth strategy: `"oidc"` or empty (omitted if empty) |
| HealthURL | string | `healthUrl` | Custom health check URL (omitted if empty) |
| ExpectedStatusCodes | []int | `expectedStatusCodes` | Status codes treated as healthy (omitted if empty) |

### HealthStatus (enum)

| Value | Meaning |
|-|-|
| `healthy` | HTTP check returned expected status code |
| `unhealthy` | HTTP check failed (timeout, connection refused, non-expected status) |
| `authBlocked` | Health check blocked by authentication (OIDC required but unavailable) |
| `unknown` | Not yet checked |

### Store (`internal/state`)

Thread-safe container for service state.

| Field | Type | Description |
|-|-|-|
| services | map[string]Service | Service map keyed by `"namespace/name"` |
| mu | sync.RWMutex | Read-write lock |
| subs | map[chan Event]struct{} | Event subscribers (SSE broker) |
| k8sConnected | bool | Current K8s API connectivity |
| lastK8sEvent | time.Time | Last K8s connectivity change |
| configErrors | []string | Config validation errors |

### Event (`internal/state`)

State change event emitted by the store.

| Field | Type | Description |
|-|-|-|
| Type | EventType | Event kind (int enum) |
| Service | Service | Populated for Discovered/Updated events |
| Namespace | string | Populated for Removed events |
| Name | string | Populated for Removed events |

**EventType constants:**

| Value | Name | Description |
|-|-|-|
| 0 | EventDiscovered | New service added |
| 1 | EventRemoved | Service deleted |
| 2 | EventUpdated | Existing service updated (health, config override) |
| 3 | EventK8sStatus | K8s connectivity changed |
| 4 | EventConfigErrors | Config validation errors changed |

### OIDCStatus (`internal/sse`)

OIDC provider status included in state events.

| Field | Type | JSON | Description |
|-|-|-|-|
| Connected | bool | `connected` | Whether the provider is reachable |
| ProviderName | string | `providerName` | Derived provider name (e.g. `"PocketID"`) |
| TokenState | string | `tokenState` | One of `valid`, `refreshing`, `expired`, `error` |
| LastSuccess | *time.Time | `lastSuccess` | Last successful token acquisition (nullable) |

### OIDCCredentials (`internal/secrets`)

Decrypted OIDC client credentials loaded from the encrypted secrets file.

| Field | Type | Description |
|-|-|-|
| ClientID | string | OIDC client identifier |
| ClientSecret | string | OIDC client secret |

### OIDCClient (`internal/auth`)

Acquires and caches OIDC access tokens using the client credentials flow. Thread-safe. Key behavior:

- Lazily discovers the token endpoint via `/.well-known/openid-configuration`
- Caches tokens in memory with proactive refresh 30s before expiry
- Deduplicates concurrent token fetches via singleflight
- Exposes `GetStatus() *OIDCStatus` for SSE status snapshots

### EndpointStrategy (`internal/auth`)

Resolved health check approach for a service.

| Field | Type | Description |
|-|-|-|
| Type | string | `"healthEndpoint"` (direct probe) or `"oidcAuth"` (token required) |
| Endpoint | string | Full probe URL (only set when Type is `"healthEndpoint"`) |

### EndpointDiscoverer (`internal/auth`)

Probes common health endpoints (`/healthz`, `/health`, `/ping`, `/api/health`) and caches the result per service. Falls back to `"oidcAuth"` strategy when no unauthenticated health endpoint responds with 2xx.

### Config Types (`internal/config`)

**Config** — top-level YAML configuration:

| Field | Type | YAML key | Description |
|-|-|-|-|
| Services | []CustomService | `services` | Non-Kubernetes services to monitor |
| Overrides | []ServiceOverride | `overrides` | Property overrides for K8s-discovered services |
| Groups | map[string]GroupConfig | `groups` | Group display metadata |
| Health | HealthConfig | `health` | Health check interval/timeout |
| History | HistoryConfig | `history` | History retention settings |
| OIDC | OIDCConfig | `oidc` | OIDC provider configuration |

**OIDCConfig:**

| Field | Type | YAML key | Description |
|-|-|-|-|
| IssuerURL | string | `issuerUrl` | OIDC provider issuer URL |
| Scopes | []string | `scopes` | Token scopes to request |

**CustomService:**

| Field | Type | YAML key | Description |
|-|-|-|-|
| Name | string | `name` | Service identifier |
| URL | string | `url` | Service URL |
| Group | string | `group` | Logical group |
| DisplayName | string | `displayName` | Human-readable label |
| HealthURL | string | `healthUrl` | Custom health check URL |
| ExpectedStatusCodes | []int | `expectedStatusCodes` | Status codes treated as healthy |
| Icon | string | `icon` | Icon identifier |

**ServiceOverride:**

| Field | Type | YAML key | Description |
|-|-|-|-|
| Match | string | `match` | Service name pattern to match |
| DisplayName | string | `displayName` | Override display name |
| HealthURL | string | `healthUrl` | Override health check URL |
| ExpectedStatusCodes | []int | `expectedStatusCodes` | Override expected status codes |
| Icon | string | `icon` | Override icon |

### History Types (`internal/history`)

**TransitionRecord** — a single health-status transition persisted to JSONL:

| Field | Type | JSON | Description |
|-|-|-|-|
| Timestamp | time.Time | `ts` | When the transition occurred |
| ServiceKey | string | `svc` | Service key (`"namespace/name"`) |
| PrevStatus | HealthStatus | `prev` | Previous health status |
| NextStatus | HealthStatus | `next` | New health status |
| HTTPCode | *int | `code` | HTTP status code at transition (nullable) |
| ResponseMs | *int64 | `ms` | Response time at transition (nullable) |

**HistoryWriter** (interface): `Record(TransitionRecord) error`, `Close() error`
Implementations: `FileWriter` (append-only JSONL file), `NoopWriter` (discard).

**ReadHistory(path) -> map[string]TransitionRecord**: Reads JSONL file, returns latest record per service key.

**RestoreHistory(store, records, logger) -> *PendingHistory**: Applies records to existing services; returns pending records for services not yet discovered.

## TypeScript Types (Frontend)

### Service

| Field | Type | Description |
|-|-|-|
| name | string | Service identifier |
| displayName | string | Human-readable label |
| namespace | string | Kubernetes namespace |
| group | string | Logical grouping key |
| url | string | Service URL |
| icon | string \| null (optional) | Icon identifier |
| source | ServiceSource (optional) | `"kubernetes"` or `"config"` |
| status | HealthStatus | Current health state |
| httpCode | number \| null | Last health check HTTP status |
| responseTimeMs | number \| null | Last check latency in ms |
| lastChecked | string \| null | ISO timestamp of last check |
| lastStateChange | string \| null | ISO timestamp of last status transition |
| errorSnippet | string \| null | Truncated error message |
| healthUrl | string \| null (optional) | Custom health check URL |
| authMethod | string (optional) | Auth strategy (`"oidc"` or absent) |

### HealthStatus (union type)

```typescript
type HealthStatus = 'healthy' | 'unhealthy' | 'authBlocked' | 'unknown';
```

### OIDCStatus

| Field | Type | Description |
|-|-|-|
| connected | boolean | Whether the provider is reachable |
| providerName | string | Derived provider name |
| tokenState | `'valid' \| 'refreshing' \| 'expired' \| 'error'` | Current token state |
| lastSuccess | string \| null | ISO timestamp of last successful acquisition |

### StateEventPayload

| Field | Type | Description |
|-|-|-|
| appVersion | string | Server version string |
| services | Service[] | All current services |
| k8sConnected | boolean (optional) | K8s API connectivity |
| k8sLastEvent | string \| null (optional) | ISO timestamp of last K8s status change |
| healthCheckIntervalMs | number (optional) | Health check interval in ms |
| configErrors | string[] (optional) | Config validation errors |
| oidcStatus | OIDCStatus (optional) | OIDC provider status |

### SSE Event Types

| SSE Event Name | Payload Type | Description |
|-|-|-|
| `state` | StateEventPayload | Full state snapshot (on connect and config error changes) |
| `discovered` | DiscoveredEventPayload | New service detected |
| `update` | DiscoveredEventPayload | Existing service health/config updated |
| `removed` | `{ name: string, namespace: string }` | Service removed |
| `k8sStatus` | `{ k8sConnected: boolean, k8sLastEvent: string }` | K8s connection state change |

## Data Flow Diagram

```
Kubernetes API                  Config File (YAML)
    |                               |
    v (Ingress events)              v (custom services, overrides, groups)
K8s Watcher                     Config Loader
    |                               |
    +--------> State Store <--------+
                  ^    |
                  |    |
   Health Checker-+    +---> SSE Broker ---> Frontend Store ($state) ---> UI
       ^                        ^
       |                        |
   Endpoint Discoverer    OIDC Status Provider
       ^                        ^
       |                        |
   OIDC Client <--- Secrets Loader (encrypted file)
                        ^
                        |
                    SECRETS_KEY env var

   History Writer <--- State Store (transitions)
       |
       v (JSONL file)
   History Reader ---> RestoreHistory (on startup)
```
