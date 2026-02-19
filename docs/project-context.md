---
project_name: 'command-center'
user_name: 'Kenny'
date: '2026-02-19'
sections_completed: ['technology_stack', 'critical_implementation_rules']
existing_patterns_found: 8
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Security — PUBLIC REPOSITORY

This project is on a **public Git repository**. Every agent must enforce:

- **Zero secrets in code or git history** — no API keys, tokens, passwords, certificates, or private keys in any file that is or could be committed
- **No hardcoded credentials** — use environment variables or external config files listed in `.gitignore`
- **Commit hygiene** — review all staged files for accidental secret inclusion before every commit
- **Input validation at system boundaries** — validate all external input (HTTP requests, Kubernetes API responses, user input)
- **Dependency vigilance** — only use well-known, actively maintained packages; verify package names to avoid typosquatting
- **No sensitive infrastructure details** — do not put internal URLs, IPs, cluster names, or namespace details in code comments or logs

## Technology Stack & Versions

| Technology | Version | Notes |
|-----------|---------|-------|
| Go | 1.26 | Standard library only for HTTP (`net/http`). No third-party routers. |
| SvelteKit | 2.50.x | adapter-static v3, SPA mode (`ssr=false`, `fallback: 'index.html'`) |
| Svelte | 5.49.x | Runes: `$props()` not `export let`, `onclick` not `on:click` |
| Tailwind CSS | 4.1.x | `@tailwindcss/vite` plugin, `@import "tailwindcss"` in app.css |
| TypeScript | strict | Frontend only, in `web/` |
| Vitest | 4.x | Frontend tests with `@testing-library/svelte` |

## Critical Implementation Rules

### Go Patterns
- **`net/http` only** — no chi, gorilla/mux, gin, or any third-party router. Use `http.NewServeMux()` with Go 1.22+ method routing.
- **`cmd/command-center/main.go` is wiring only** — no business logic. Wire packages, start server, handle shutdown.
- **No pre-created empty packages** — only create packages when their story demands them.
- **Error handling** — return errors, never panic in library code. Only `log.Fatal` in main.
- **Naming** — files: `snake_case.go`, packages: lowercase single word, exports: PascalCase.
- **Tests** — co-located `_test.go` files next to source.

### SvelteKit Patterns
- **Svelte 5 conventions** — `$props()`, `onclick`, `.svelte.ts` for rune files.
- **Component naming** — PascalCase for components.
- **Module naming** — camelCase for TypeScript modules.
- **Tests** — co-located `.test.ts` files next to source, run with `vitest`.

### Project Structure
- Go source at root, SvelteKit in `web/`
- Entrypoint: `cmd/command-center/main.go`
- Embed: `embed.go` at root with `//go:embed all:web/build`
- Build: `make build` (runs `npm ci && npm run build` then `go build`)
- Tests: `make test` (runs Go and frontend tests)
