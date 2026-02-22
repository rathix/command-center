# Component Inventory

**Project:** Command Center
**Generated:** 2026-02-21

## Server Components (Go Packages)

### Domain Packages (`internal/`)

| Package | Primary Type | Purpose | Dependencies |
|-|-|-|-|
| `auth` | OIDCClient, EndpointDiscoverer | OIDC client and endpoint discovery | net/http (stdlib), config |
| `certs` | Generator | mTLS certificate generation and management | crypto (stdlib) |
| `config` | Loader, Watcher | YAML config loader with hot-reload watcher | gopkg.in/yaml.v3 |
| `health` | Checker | HTTP health checking for discovered services | net/http (stdlib), state |
| `history` | Writer, Reader, Pruner | JSONL health history persistence | state, os (stdlib) |
| `k8s` | Watcher | Kubernetes Ingress resource watcher | k8s.io/client-go, state |
| `secrets` | LoadSecrets, OIDCCredentials | Encrypted secrets file decryption | golang.org/x/crypto |
| `server` | SPAHandler, DevProxy | HTTP serving (embedded SPA + dev proxy) | net/http (stdlib), embed |
| `session` | Tracker | SSE session tracking | sync (stdlib) |
| `sse` | Broker | SSE event broadcasting to browser clients | state, net/http (stdlib) |
| `state` | Store, Service, Event | Thread-safe service state management | sync (stdlib) |

### Application Bootstrap

| Package | File | Purpose |
|-|-|-|
| `cmd/command-center` | main.go | Config parsing, dependency wiring, server lifecycle |
| `cmd/encrypt-secrets` | main.go | CLI tool for encrypting secrets files |
| root | embed.go | `//go:embed` directive for frontend build |

## Web Components (Svelte)

### UI Components (`web/src/lib/components/`)

| Component | Category | Props/Inputs | Purpose |
|-|-|-|-|
| `GroupHeader.svelte` | Layout | group | Group section headers with collapse toggle |
| `GroupedServiceList.svelte` | Layout | services, connectionStatus | Grouped service list container |
| `ServiceIcon.svelte` | Display | service | Service icon with fallback |
| `ServiceRow.svelte` | Display | service | Individual service: name, URL, health dot, tooltip trigger |
| `StatusBar.svelte` | Display | connectionStatus, freshness | SSE connection state and data freshness indicator |
| `HoverTooltip.svelte` | Overlay | service | Diagnostic detail popup (health info, timestamps, errors) |
| `tui/TuiDot.svelte` | Display | status | Terminal-style colored status dot (green/yellow/red) |

### Component Hierarchy

```
+layout.svelte
  └── +page.svelte
      ├── StatusBar
      └── GroupedServiceList
          ├── GroupHeader (per group)
          └── ServiceRow (per service)
              ├── TuiDot
              ├── ServiceIcon
              └── HoverTooltip (on hover)
```

### Library Modules (`web/src/lib/`)

| Module | Category | Purpose |
|-|-|-|
| `types.ts` | Types | TypeScript definitions: Service, SSE events, health status, OIDCStatus |
| `sseClient.ts` | Service | EventSource wrapper with auto-reconnect |
| `serviceStore.svelte.ts` | State | Svelte 5 `$state` rune store for services |
| `formatRelativeTime.ts` | Utility | Human-readable relative timestamps |
| `index.ts` | Export | Barrel exports for lib modules |

## Design System Elements

| Element | Implementation | Notes |
|-|-|-|
| Color theme | Catppuccin Mocha | Dark terminal aesthetic via Tailwind CSS v4 custom properties |
| Status indicators | TuiDot component | Green (healthy), yellow (degraded), red (unhealthy), gray (unknown) |
| Typography | Monospace-influenced | Terminal/TUI feel |
| Layout | Information-dense | Minimal chrome, data-focused |
| Interactions | Hover tooltips | Non-intrusive diagnostic details |
