# Changelog

All notable changes to Command Center are documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioned per [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] — 2026-02-22 — Multi-Signal Health Fusion

Replaces OIDC-based authentication checks with K8s-native health signals.
Fuses HTTP probes with EndpointSlice readiness and pod diagnostics to produce
composite health status. Hardens container security for production deployment.

### Epic 5 — Service Grouping & Icons

- Group services by Kubernetes namespace or custom group field
- Collapsible GroupHeader with per-group health summary counts
- GroupedServiceList replaces flat ServiceList for organized layout
- ServiceIcon component with favicon/initial-based icons
- Problems-first group sorting: unhealthy groups surface to the top

### Epic 6 — Custom Services & Configuration

- YAML configuration parser for custom (non-K8s) service definitions
- File watcher with hot-reload for config changes without restart
- Custom service registration with configurable health check URLs
- Source indicators (K8s/Custom) and config warning display in StatusBar

### Epic 7 — Health History Persistence

- JSONL history writer recording health state transitions to disk
- History reader restoring service state on startup for instant display
- Automatic history pruning with configurable retention period
- Pre-populated startup state: dashboard shows last-known status immediately

### Epic 9 — Container Security Hardening

- Kubeconfig permission validation on startup
- Dockerfile hardening: non-root user, read-only filesystem, security context
- Credential isolation audit confirming no secrets in image layers

### Epic 10 — Multi-Signal Health Fusion

- EndpointSlice watcher for real-time pod readiness tracking
- Composite health model fusing HTTP probes with K8s readiness signals
- Four-tier health status: healthy, degraded, unhealthy, unknown
- Pod diagnostic enrichment: CrashLoopBackOff detection, restart counts
- Auth-guarded service detection (401/403 + pods ready = healthy)
- Service model extended with `compositeStatus`, `readyEndpoints`, `totalEndpoints`, `authGuarded`, `podDiagnostic`
- Frontend: degraded yellow state, pod readiness in tooltips, auth shield glyph
- Four-tier group sorting: unhealthy > degraded > unknown > healthy
- StatusBar and GroupHeader aggregate counts include degraded status
- Removed OIDC/secrets infrastructure (superseded by K8s-native approach)

### Breaking Changes

- Removed OIDC client credentials flow and secrets decryption (Epic 8 superseded)
- SSE `discovered` event payload includes new required fields: `compositeStatus`, `readyEndpoints`, `totalEndpoints`, `authGuarded`, `podDiagnostic`
- `HealthStatus` type now includes `"degraded"` value
- Service sorting and `hasProblems` now use `compositeStatus` instead of `status`

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

[2.0.0]: https://github.com/rathix/command-center/releases/tag/v2.0.0
[1.0.0]: https://github.com/rathix/command-center/releases/tag/v1.0.0
