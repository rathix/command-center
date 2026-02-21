# Data Models

**Project:** Command Center
**Generated:** 2026-02-21

## Overview

Command Center has no persistent database. All state is held in-memory in a thread-safe store and streamed to clients via SSE. Data originates from the Kubernetes API (Ingress resources) and HTTP health checks.

## Go Data Structures (Server)

### Service (`internal/state`)

The core domain entity representing a discovered Kubernetes service.

| Field | Type | Description |
|-|-|-|
| Name | string | Service identifier (from Ingress metadata) |
| URL | string | Service URL (from Ingress spec rules) |
| Namespace | string | Kubernetes namespace |
| Health | HealthStatus | Current health state |
| StatusCode | int | Last HTTP health check status code |
| LatencyMs | int64 | Last health check latency |
| CheckedAt | time.Time | Timestamp of last health check |
| DiscoveredAt | time.Time | When the service was first discovered |

### HealthStatus (enum)

| Value | Meaning |
|-|-|
| `healthy` | HTTP check returned 2xx |
| `degraded` | HTTP check returned non-2xx but reachable |
| `unhealthy` | HTTP check failed (timeout, connection refused, etc.) |
| `unknown` | Not yet checked |

### Store (`internal/state`)

Thread-safe container for service state.

| Field | Type | Description |
|-|-|-|
| services | map[string]*Service | Service map keyed by name |
| mu | sync.Mutex | Thread-safety lock |
| subscribers | []chan Event | Event listeners (SSE broker) |

### Event (`internal/state`)

State change event emitted by the store.

| Field | Type | Description |
|-|-|-|
| Type | string | Event type (discovered, removed, health_update, k8s_status) |
| Payload | interface{} | Typed payload (varies by event type) |
| Timestamp | time.Time | When the event occurred |

### Broker (`internal/sse`)

SSE client connection manager.

| Field | Type | Description |
|-|-|-|
| clients | map[chan string]bool | Connected SSE clients |
| events | chan string | Inbound event channel from store |
| mu | sync.Mutex | Client map lock |

## TypeScript Types (Frontend)

### Service

Mirrors the Go `Service` struct for frontend rendering.

| Field | Type | Description |
|-|-|-|
| name | string | Service identifier |
| url | string | Service URL |
| namespace | string | Kubernetes namespace |
| health | HealthStatus | Current health state |
| statusCode | number | Last health check HTTP status |
| latencyMs | number | Last check latency |
| checkedAt | string | ISO timestamp of last check |
| discoveredAt | string | ISO timestamp of discovery |

### HealthStatus (union type)

```typescript
type HealthStatus = 'healthy' | 'degraded' | 'unhealthy' | 'unknown';
```

### SSE Event Payloads

| Type | Payload Shape | Description |
|-|-|-|
| StateEventPayload | `{ services: Record<string, Service>, k8s_status: string }` | Full state snapshot |
| DiscoveredEventPayload | `{ name: string, url: string, namespace: string }` | New service |
| RemovedEventPayload | `{ name: string }` | Removed service |
| K8sStatusPayload | `{ status: string }` | K8s connection state |
| HealthUpdatePayload | `Service` | Updated service health |

## Data Flow Diagram

```
Kubernetes API
    │
    ▼ (Ingress events)
K8s Watcher
    │
    ▼ (discover/remove)
State Store ◄─── Health Checker (health_update)
    │
    ▼ (events)
SSE Broker
    │
    ▼ (JSON over SSE)
Frontend Store ($state)
    │
    ▼ (reactive)
UI Components
```
