# Source Tree Analysis

**Project:** Command Center
**Type:** Multi-part (Go backend + SvelteKit frontend)
**Generated:** 2026-02-21

## Annotated Directory Tree

```
command-center/
├── cmd/
│   ├── command-center/
│   │   ├── main.go              # ★ Entry point: config, K8s watcher, SSE broker, HTTP server
│   │   └── main_test.go         # Server lifecycle tests
├── internal/
│   ├── certs/
│   │   ├── generator.go         # Auto-generated mTLS certificates (CA, server, client)
│   │   └── generator_test.go
│   ├── config/
│   │   ├── loader.go            # YAML config loader with hot-reload watcher
│   │   └── loader_test.go
│   ├── health/
│   │   ├── checker.go           # HTTP health checker with InternalURL support
│   │   └── checker_test.go
│   ├── history/
│   │   ├── writer.go            # JSONL health history persistence (write, read, prune)
│   │   └── writer_test.go
│   ├── k8s/
│   │   ├── watcher.go           # Kubernetes Ingress watcher (informer-based)
│   │   └── watcher_test.go
│   ├── server/
│   │   ├── static.go            # SPA static file handler (serves embedded frontend)
│   │   ├── static_test.go
│   │   ├── devproxy.go          # Dev mode proxy → Vite dev server (localhost:5173)
│   │   └── devproxy_test.go
│   ├── session/
│   │   ├── tracker.go           # SSE session tracking
│   │   └── tracker_test.go
│   ├── sse/
│   │   ├── broker.go            # SSE event broker (client management, broadcasting)
│   │   ├── broker_test.go
│   │   ├── events.go            # SSE event type definitions and serialization
│   │   └── events_test.go
│   └── state/
│       ├── store.go             # Thread-safe service state store with event tracking
│       └── store_test.go
├── web/                          # SvelteKit frontend (Part: web)
│   ├── src/
│   │   ├── app.css              # Global styles (Tailwind v4 imports, Catppuccin Mocha theme)
│   │   ├── app.html             # HTML template
│   │   ├── app.d.ts             # SvelteKit type declarations
│   │   ├── app.test.ts          # App-level tests
│   │   ├── lib/
│   │   │   ├── index.ts         # Lib barrel exports
│   │   │   ├── types.ts         # Shared TypeScript type definitions (Service, SSE events)
│   │   │   ├── types.test.ts
│   │   │   ├── sseClient.ts     # SSE client with auto-reconnect logic
│   │   │   ├── sseClient.test.ts
│   │   │   ├── serviceStore.svelte.ts    # Svelte 5 $state rune store for services
│   │   │   ├── serviceStore.svelte.test.ts
│   │   │   ├── serviceStore.structural.test.ts
│   │   │   ├── formatRelativeTime.ts     # Relative time formatting utility
│   │   │   ├── formatRelativeTime.test.ts
│   │   │   └── components/
│   │   │       ├── GroupHeader.svelte      # Group section headers with collapse
│   │   │       ├── GroupHeader.test.ts
│   │   │       ├── GroupedServiceList.svelte # Grouped service list container
│   │   │       ├── GroupedServiceList.test.ts
│   │   │       ├── ServiceIcon.svelte     # Service icon with fallback
│   │   │       ├── ServiceIcon.test.ts
│   │   │       ├── ServiceRow.svelte      # Individual service row (name, URL, status)
│   │   │       ├── ServiceRow.test.ts
│   │   │       ├── StatusBar.svelte       # Connection status bar (SSE state, freshness)
│   │   │       ├── StatusBar.test.ts
│   │   │       ├── HoverTooltip.svelte    # Diagnostic hover tooltips
│   │   │       ├── HoverTooltip.test.ts
│   │   │       └── tui/
│   │   │           ├── TuiDot.svelte      # TUI-style status indicator dot
│   │   │           └── TuiDot.test.ts
│   │   └── routes/
│   │       ├── +layout.svelte   # Root layout (SSE connection, global structure)
│   │       ├── +layout.ts       # Layout data loader (prerender config)
│   │       ├── +page.svelte     # ★ Main dashboard page
│   │       ├── layout.test.ts
│   │       └── page.test.ts
│   ├── svelte.config.js         # SvelteKit config (adapter-static, SPA fallback)
│   ├── vite.config.ts           # Vite config (Tailwind plugin, Vitest setup)
│   ├── tsconfig.json            # TypeScript config
│   ├── eslint.config.js         # ESLint config
│   ├── vitest-setup.ts          # Test setup (jest-dom matchers)
│   ├── package.json             # Frontend dependencies and scripts
│   └── package-lock.json
├── embed.go                     # //go:embed directive for web/build → embed.FS
├── embed_test.go
├── Dockerfile                   # Multi-stage: Node 22 → Go 1.26 → distroless
├── Makefile                     # Build targets: build, test, dev, clean, docker
├── .github/workflows/
│   └── publish.yml              # CI/CD: lint, test, build, push to GHCR, Trivy scan
├── go.mod                       # Go module (k8s.io/client-go, golang.org/x/crypto, gopkg.in/yaml.v3)
├── go.sum
├── .env.example                 # Environment variable template
├── README.md                    # Comprehensive project documentation
├── CLAUDE.md                    # AI assistant instructions
├── AGENTS.md                    # AI agent configuration
├── GEMINI.md                    # Gemini AI configuration
└── docs/
    ├── api-contracts.md         # API endpoint contracts
    ├── architecture-server.md   # Server architecture documentation
    ├── architecture-web.md      # Web frontend architecture documentation
    ├── component-inventory.md   # Component inventory and hierarchy
    ├── data-models.md           # Data model definitions
    ├── deployment-guide.md      # Deployment and operations guide
    ├── development-guide.md     # Development workflow guide
    ├── index.md                 # Documentation index
    ├── integration-architecture.md # Integration architecture
    ├── project-context.md       # Project context reference
    ├── project-overview.md      # Project overview
    ├── project-scan-report.json # Automated project scan data
    └── source-tree-analysis.md  # This file
```

## Critical Folders

| Folder | Purpose | Part |
|-|-|-|
| `cmd/command-center/` | Application entry point, config parsing, server lifecycle | server |
| `internal/certs/` | mTLS certificate generation and management | server |
| `internal/config/` | YAML config loader with hot-reload watcher | server |
| `internal/health/` | HTTP health checking for discovered K8s services | server |
| `internal/history/` | JSONL health history persistence | server |
| `internal/k8s/` | Kubernetes Ingress watcher (informer pattern) | server |
| `internal/server/` | HTTP handlers — SPA serving (prod) and Vite proxy (dev) | server |
| `internal/session/` | SSE session tracking | server |
| `internal/sse/` | Server-Sent Events broker and event types | server |
| `internal/state/` | Thread-safe service state store | server |
| `web/src/lib/` | Frontend core: stores, SSE client, types, utilities | web |
| `web/src/lib/components/` | Svelte UI components (GroupedServiceList, StatusBar, etc.) | web |
| `web/src/routes/` | SvelteKit routes (single-page dashboard) | web |

## Entry Points

| Entry Point | File | Purpose |
|-|-|-|
| Go binary | `cmd/command-center/main.go` | Server bootstrap: parse flags, init K8s watcher, SSE broker, health checker, start HTTPS server |
| Frontend SPA | `web/src/routes/+page.svelte` | Dashboard page rendering |
| Frontend layout | `web/src/routes/+layout.svelte` | SSE connection initialization, global layout |
| Embed bridge | `embed.go` | `//go:embed` directive connecting Go binary to compiled frontend |

## Integration Points

| From | To | Type | Details |
|-|-|-|-|
| web (SSE client) | server (SSE broker) | SSE (GET /api/events) | Real-time service state updates |
| server (SPA handler) | web (build output) | embed.FS | Go serves compiled SvelteKit as static files |
| server (K8s watcher) | server (state store) | In-process | Watcher discovers services → store updates state |
| server (state store) | server (SSE broker) | In-process | Store events → broker broadcasts to connected clients |
| server (health checker) | server (state store) | In-process | Health results → store updates service status |
