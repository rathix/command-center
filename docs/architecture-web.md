# Architecture — Web (SvelteKit Frontend)

**Project:** Command Center
**Part:** web
**Type:** web
**Generated:** 2026-02-21

## Executive Summary

The web frontend is a SvelteKit single-page application (SPA) that provides a terminal-aesthetic dashboard for monitoring Kubernetes services. Built with Svelte 5 (runes), TypeScript, and Tailwind CSS v4 using a Catppuccin Mocha color theme. It connects to the Go backend via Server-Sent Events for real-time service state updates.

## Technology Stack

| Category | Technology | Version |
|-|-|-|
| Framework | SvelteKit | ^2.50.2 |
| UI Framework | Svelte | ^5.49.2 (runes) |
| Language | TypeScript | ^5.9.3 |
| Styling | Tailwind CSS | ^4.2.0 (v4 Vite plugin) |
| Build Tool | Vite | ^7.3.1 |
| Testing | Vitest | ^4.0.18 |
| Test Library | @testing-library/svelte | ^5.3.1 |
| Linting | ESLint + Prettier | ^10 / ^3.8 |

## Architecture Pattern

**Component-based SPA** with reactive state management:

- SvelteKit with `adapter-static` (generates static HTML, no SSR)
- SPA fallback routing (`index.html` fallback for all routes)
- Svelte 5 runes (`$state`, `$derived`) for reactive state
- Unidirectional data flow: SSE events → store → components

## Application Structure

### Routes (`src/routes/`)

Single-page application with one route:

| Route | File | Purpose |
|-|-|-|
| `/` | `+page.svelte` | Main dashboard — renders service list |
| (layout) | `+layout.svelte` | Root layout — initializes SSE connection |
| (layout) | `+layout.ts` | Prerender configuration |

### Core Library (`src/lib/`)

| Module | Purpose |
|-|-|
| `types.ts` | TypeScript type definitions (Service, SSE event payloads, health status) |
| `sseClient.ts` | EventSource wrapper with auto-reconnect and exponential backoff |
| `serviceStore.svelte.ts` | Svelte 5 `$state` rune store — manages service list, handles SSE events |
| `formatRelativeTime.ts` | Utility for human-readable relative timestamps |
| `index.ts` | Barrel exports |

### Components (`src/lib/components/`)

| Component | Purpose | Key Features |
|-|-|-|
| `ServiceList.svelte` | Service list container | Problems-first sorting, section labels |
| `ServiceRow.svelte` | Individual service row | Name, URL, health status dot, hover tooltip trigger |
| `StatusBar.svelte` | Connection status display | SSE connection state, data freshness/staleness |
| `SectionLabel.svelte` | Section divider headers | Visual grouping of services by status |
| `HoverTooltip.svelte` | Diagnostic detail overlay | Health check details, timestamps, error info |
| `tui/TuiDot.svelte` | Terminal-style status dot | Color-coded health indicator (green/yellow/red) |

## State Management

### Service Store (`serviceStore.svelte.ts`)

Uses Svelte 5 `$state` rune for fine-grained reactivity:

- **Services map**: `$state` object mapping service IDs to Service objects
- **Derived values**: Sorted service lists, status counts, freshness indicators
- **Event handlers**: Process incoming SSE events (state, discovered, removed, health_update)
- **Connection status**: Tracks SSE connection state (connecting, connected, disconnected)

### Data Flow

```
SSE EventSource
    │
    ▼
sseClient.ts (connection management, reconnect)
    │
    ▼
serviceStore.svelte.ts ($state updates)
    │
    ▼
Components (reactive re-rendering)
```

## Design System

### Catppuccin Mocha Theme

Terminal-aesthetic UI using the Catppuccin Mocha dark color palette via Tailwind CSS v4 custom properties. The design evokes a terminal/TUI feel with:

- Monospace-influenced typography
- Status dots resembling terminal indicators
- Minimal, information-dense layout
- Dark background with muted accent colors

### Component Patterns

- **Composition**: Components are composed via slots and props, not inheritance
- **Reactivity**: Svelte 5 runes (`$state`, `$derived`) instead of stores
- **Styling**: Tailwind utility classes, no component CSS files
- **Testing**: Co-located test files (`Component.test.ts` alongside `Component.svelte`)

## SSE Client (`sseClient.ts`)

Manages the EventSource connection to `GET /api/events`:

- **Auto-reconnect**: Exponential backoff on connection loss
- **Initial state**: Processes full state snapshot on (re)connection
- **Event parsing**: Deserializes JSON payloads into typed objects
- **Connection status**: Exposes connection state for StatusBar display
- **Cleanup**: Proper EventSource teardown on component unmount

## Testing Strategy

| Test File | Focus |
|-|-|
| `app.test.ts` | App-level configuration |
| `page.test.ts` | Dashboard page rendering |
| `layout.test.ts` | Layout initialization |
| `ServiceList.test.ts` | List rendering, sorting, sections |
| `ServiceRow.test.ts` | Row display, status indicators |
| `StatusBar.test.ts` | Connection status display |
| `SectionLabel.test.ts` | Section header rendering |
| `HoverTooltip.test.ts` | Tooltip behavior |
| `TuiDot.test.ts` | Status dot color mapping |
| `sseClient.test.ts` | SSE connection management |
| `serviceStore.svelte.test.ts` | Store mutations, event handling |
| `serviceStore.structural.test.ts` | Store API shape validation |
| `types.test.ts` | Type guard validation |
| `formatRelativeTime.test.ts` | Time formatting logic |

**Testing approach:**
- Vitest with jsdom environment
- @testing-library/svelte for component tests
- jest-dom matchers for DOM assertions
- Co-located tests (test file next to source)

## Build & Development

| Command | Purpose |
|-|-|
| `npm run dev` | Vite dev server with HMR (port 5173) |
| `npm run build` | Production build (adapter-static → `web/build/`) |
| `npm test` | Run Vitest test suite |
| `npm run lint` | ESLint check |
| `npm run check` | svelte-check type validation |
| `npm run format` | Prettier formatting |
