# Changelog

All notable changes to Command Center are documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioned per [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] — 2026-02-21 — MVP

Kubernetes service dashboard for homelab operators. Single Go binary serving a
SvelteKit SPA with mTLS security, real-time health monitoring, and connection
resilience.

### Epic 1 — Project Scaffold & Infrastructure

- Go 1.26 server with `net/http`, SvelteKit SPA embedded via `//go:embed`
- Auto-generated mTLS certificates (CA, server, client) with TLS 1.3 minimum
- CLI flags and environment variable configuration with precedence rules
- Graceful shutdown with signal handling
- Multi-stage Docker build producing a distroless container image
- GitHub Actions CI/CD pipeline publishing to GHCR
- Catppuccin Mocha terminal aesthetic for the frontend

### Epic 2 — Service Discovery & Dashboard (v0.2.0)

- Kubernetes Ingress watcher using informer-based watch with automatic reconnection
- Thread-safe service state store with fan-out subscriptions and deep copying
- Server-Sent Events broker for real-time server → client streaming
- SvelteKit dashboard with Svelte 5 runes (`$state`) for fine-grained reactivity
- Service cards displaying hostname, URL, and Ingress metadata

### Epic 3 — Health Monitoring & Status (v0.3.0)

- HTTP health checker with configurable interval (`--health-interval`)
- Visual health status indicators (Healthy, Degraded, Unknown) on service cards
- Problems-first sorting: unhealthy services surface to the top
- Health-state section labels grouping services by status
- Stable service positions during live updates to prevent layout thrash
- App version display with build-time injection and `--version` CLI flag

### Epic 4 — Resilience & Data Freshness (v0.4.0)

- SSE automatic reconnection with exponential backoff and jitter
- Client-side state caching across reconnections
- Kubernetes API connectivity monitoring with health status propagation
- Data freshness tracking with staleness indicators on service cards
- Connection status banner showing real-time backend connectivity
- Enhanced tooltip with freshness timestamps and live re-sorting

### Post-Epic Polish (v0.4.1 → v1.0.0)

- Comprehensive project documentation (architecture, API contracts, data models, guides)
- Test coverage for SSE events, Store.Update subscriptions, and API endpoint routing
- Structural test hardening and type guard validation from adversarial review
- CI optimization with parallel test jobs, linting, and path filtering

[1.0.0]: https://github.com/rathix/command-center/releases/tag/v1.0.0
