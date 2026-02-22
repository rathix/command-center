# Command Center — Project Documentation Index

**Generated:** 2026-02-21
**Scan Level:** Quick

## Project Overview

- **Type:** Multi-part (Go backend + SvelteKit frontend, embedded as single binary)
- **Primary Languages:** Go 1.26, TypeScript (Svelte 5)
- **Architecture:** Service/handler pattern (backend) + component-based SPA (frontend)
- **Deployment:** Single binary, Docker (distroless), mTLS-secured
- **Version:** v1.0.0 (MVP — Epic 8 complete)

## Quick Reference

### Server (Go Backend)

- **Type:** backend
- **Tech Stack:** Go 1.26, stdlib `net/http`, k8s.io/client-go v0.35.1
- **Root:** `/` (cmd/, internal/, embed.go)
- **Entry Point:** `cmd/command-center/main.go`
- **Packages:** certs, config, health, history, k8s, server, session, sse, state

### Web (SvelteKit Frontend)

- **Type:** web
- **Tech Stack:** SvelteKit ^2.50.2, Svelte 5, TypeScript, Tailwind CSS v4, Vite ^7.3.1
- **Root:** `/web`
- **Entry Point:** `web/src/routes/+page.svelte`
- **Components:** GroupedServiceList, GroupHeader, ServiceIcon, ServiceRow, StatusBar, HoverTooltip, TuiDot

## Generated Documentation

### Architecture
- [Project Overview](./project-overview.md)
- [Architecture — Server](./architecture-server.md)
- [Architecture — Web](./architecture-web.md)
- [Integration Architecture](./integration-architecture.md)
- [Source Tree Analysis](./source-tree-analysis.md)

### Reference
- [API Contracts](./api-contracts.md)
- [Data Models](./data-models.md)
- [Component Inventory](./component-inventory.md)

### Operations
- [Development Guide](./development-guide.md)
- [Deployment Guide](./deployment-guide.md)

## Existing Documentation

- [README](../README.md) — Comprehensive project readme (quick start, build, config, mTLS, deployment)
- [CLAUDE.md](../CLAUDE.md) — AI assistant instructions and project conventions
- [.env.example](../.env.example) — Environment variable template with documentation
- [CI/CD Pipeline](../.github/workflows/publish.yml) — GitHub Actions: test, build, publish to GHCR

## Planning Artifacts (BMAD)

- [PRD](../_bmad-output/planning-artifacts/prd.md) — Product Requirements Document
- [Architecture Plan](../_bmad-output/planning-artifacts/architecture.md) — Original architecture design
- [Epics](../_bmad-output/planning-artifacts/epics.md) — Epic breakdown and stories
- [UX Design Spec](../_bmad-output/planning-artifacts/ux-design-specification.md) — Design specification
- [Product Brief](../_bmad-output/planning-artifacts/product-brief-command-center-2026-02-19.md) — Initial product brief

## Getting Started

### For Development
```bash
git clone https://github.com/rathix/command-center.git
cd command-center
cd web && npm ci && cd ..
make dev
```
See [Development Guide](./development-guide.md) for full setup instructions.

### For Deployment
```bash
docker run -d -p 8443:8443 \
  -v /path/to/kubeconfig:/home/nonroot/.kube/config:ro \
  -v command-center-data:/data \
  ghcr.io/rathix/command-center:latest
```
See [Deployment Guide](./deployment-guide.md) for certificate setup and configuration.

### For AI-Assisted Development
When creating brownfield features, provide this index as context. Key references:
- **Full-stack changes:** [Integration Architecture](./integration-architecture.md) + both architecture docs
- **Backend only:** [Architecture — Server](./architecture-server.md) + [API Contracts](./api-contracts.md)
- **Frontend only:** [Architecture — Web](./architecture-web.md) + [Component Inventory](./component-inventory.md)
- **New SSE events:** [API Contracts](./api-contracts.md) + [Data Models](./data-models.md)
