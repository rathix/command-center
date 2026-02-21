# Component Inventory

**Project:** Command Center
**Generated:** 2026-02-21

## Server Components (Go Packages)

### Domain Packages (`internal/`)

| Package | Primary Type | Purpose | Dependencies |
|-|-|-|-|
| `state` | Store, Service, Event | Thread-safe service state management | sync (stdlib) |
| `k8s` | Watcher | Kubernetes Ingress resource watcher | k8s.io/client-go, state |
| `health` | Checker | HTTP health checking for discovered services | net/http (stdlib), state |
| `sse` | Broker | SSE event broadcasting to browser clients | state, net/http (stdlib) |
| `certs` | Generator | mTLS certificate generation and management | crypto (stdlib) |
| `server` | SPAHandler, DevProxy | HTTP serving (embedded SPA + dev proxy) | net/http (stdlib), embed |

### Application Bootstrap

| Package | File | Purpose |
|-|-|-|
| `cmd/command-center` | main.go | Config parsing, dependency wiring, server lifecycle |
| root | embed.go | `//go:embed` directive for frontend build |

## Web Components (Svelte)

### UI Components (`web/src/lib/components/`)

| Component | Category | Props/Inputs | Purpose |
|-|-|-|-|
| `ServiceList.svelte` | Layout | services, connectionStatus | Main container — sections, sorting (problems-first) |
| `ServiceRow.svelte` | Display | service | Individual service: name, URL, health dot, tooltip trigger |
| `StatusBar.svelte` | Display | connectionStatus, freshness | SSE connection state and data freshness indicator |
| `SectionLabel.svelte` | Layout | label | Section divider header (e.g., "Problems", "Healthy") |
| `HoverTooltip.svelte` | Overlay | service | Diagnostic detail popup (health info, timestamps, errors) |
| `tui/TuiDot.svelte` | Display | status | Terminal-style colored status dot (green/yellow/red) |

### Component Hierarchy

```
+layout.svelte
  └── +page.svelte
      ├── StatusBar
      └── ServiceList
          ├── SectionLabel (per section)
          └── ServiceRow (per service)
              ├── TuiDot
              └── HoverTooltip (on hover)
```

### Library Modules (`web/src/lib/`)

| Module | Category | Purpose |
|-|-|-|
| `types.ts` | Types | TypeScript definitions: Service, SSE events, health status |
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
