---
project_name: 'command-center'
user_name: 'Kenny'
date: '2026-02-27'
sections_completed: ['technology_stack', 'critical_implementation_rules', 'architecture_patterns', 'testing_rules', 'code_quality_style', 'development_workflow', 'critical_dont_miss']
status: 'complete'
rule_count: 52
optimized_for_llm: true
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
|-|-|-|
| Go | 1.26 | Standard library only for HTTP (`net/http`). No third-party routers. |
| k8s.io/client-go | v0.35.1 | Informer/lister pattern for Ingress + EndpointSlice watches |
| fsnotify | v1.9.0 | Config file hot-reload with debounce |
| gopkg.in/yaml.v3 | v3.0.1 | Config parsing (custom services, groups) |
| SvelteKit | 2.50.x | adapter-static v3, SPA mode (`ssr=false`, `fallback: 'index.html'`) |
| Svelte | 5.49.x | Runes: `$props()` not `export let`, `onclick` not `on:click` |
| Tailwind CSS | 4.x | `@tailwindcss/vite` plugin, `@import "tailwindcss"` in app.css |
| TypeScript | strict | Frontend only, in `web/` |
| Vite | 7.3.x | Build tool with Tailwind and SvelteKit plugins |
| Vitest | 4.x | Frontend tests with `@testing-library/svelte`, jsdom environment |
| ESLint | 10 | Flat config with typescript-eslint + eslint-plugin-svelte |
| Prettier | 3.8 | Tabs, single quotes, no trailing comma, printWidth 100 |

## Versioning

Semantic versioning with git tags tied to the story/epic lifecycle:

- **Story complete** — patch bump (v0.2.1, v0.2.2, ...)
- **Epic complete** — minor bump (v0.3.0, v0.4.0, ...)
- **Feature-complete** — major bump to v1.0.0 when all epics are done

After pushing a story or epic commit, create an annotated tag and push it:
`git tag -a vX.Y.Z -m "description" && git push origin vX.Y.Z`

Current: v2.0.0 (multi-signal health fusion — Epic 10 complete).

## Critical Implementation Rules

### Go Patterns
- **`net/http` only** — no chi, gorilla/mux, gin, or any third-party router. Use `http.NewServeMux()` with Go 1.22+ method routing.
- **`cmd/command-center/main.go` is wiring only** — no business logic. Wire packages, start server, handle shutdown.
- **No pre-created empty packages** — only create packages when their story demands them.
- **Error handling** — return errors, never panic in library code. Only `log.Fatal` in main.
- **Naming** — files: `snake_case.go`, packages: lowercase single word, exports: PascalCase.
- **Tests** — co-located `_test.go` files next to source.
- **Resilience** — Use `atomic.Bool` or `sync.Mutex` for all cross-goroutine flags/state.
- **Race Detection** — `go test -race ./...` is mandatory for all Go changes.
- **Interfaces at the consumer** — define interfaces where they're used, not where implemented (e.g., `StateSource` in `sse/`, `StateUpdater` in `k8s/`, `HTTPProber` in `health/`). Keeps packages decoupled.
- **Structured logging** — use `log/slog` exclusively (no `log` or `fmt.Print` for operational output). Pass `*slog.Logger` via constructor injection.
- **Functional options** — use `WithXxx` option functions for optional config (see `config.WithDebounce`).

### SvelteKit Patterns
- **Svelte 5 conventions** — `$props()`, `onclick`, `.svelte.ts` for rune files.
- **Runes (`$state`, `$derived`) lose reactivity when exported from `.svelte.ts` modules** — exporting a rune value directly compiles without errors but consumers silently get stale data. Wrap in getter functions instead: `const val = $derived(...); export function getVal() { return val; }`. See `web/src/lib/serviceStore.svelte.ts` for the established pattern.
- **`$state(Map)` and `$state(Set)` reactivity gotcha** — mutations like `.set()`, `.delete()`, or `.add()` on a proxied Map/Set do not consistently trigger `$derived` or `$derived.by` updates. **Workaround:** Reassign the entire Map/Set after mutation: `myMap.set(k, v); myMap = myMap;`.
- **`_resetForTesting()` Completeness** — Every new `$state` field in a store must be reset in its `_resetForTesting()` helper. Enforced by structural tests (`web/src/lib/serviceStore.structural.test.ts`).
- **Payload Validation** — SSE payloads must be strictly validated with type guards (e.g., `isStatePayload`, `isRecord`, `isHealthStatus`) before being applied to the store. Zero-tolerance for malformed data.
- **Component naming** — PascalCase for components.
- **Module naming** — camelCase for TypeScript modules.
- **Tests** — co-located `.test.ts` files next to source, run with `vitest`.

### Architecture Patterns
- **Unidirectional data flow** — K8s watcher + config loader → state store → health checker → SSE broker → frontend. Never write state backwards.
- **State store is the single source of truth** — `internal/state.Store` holds all service state behind a `sync.Mutex`. All reads/writes go through its methods (`All()`, `AddOrUpdate()`, `Remove()`, `Update()`). No package caches its own copy.
- **SSE broker fan-out** — `internal/sse.Broker` subscribes to state events and fans out to all connected clients. Each client gets a full state snapshot on connect, then incremental events. Keepalive every 15s.
- **K8s informer pattern** — `internal/k8s.Watcher` uses `SharedInformerFactory` with listers, not raw API calls. `atomic.Bool` tracks connection and cache-sync status. `EndpointSliceWatcher` runs alongside for readiness data.
- **Config hot-reload** — `internal/config.Watcher` uses fsnotify on the parent directory with 1s debounce. Callback receives `(*Config, []error)` — always handle both a valid config with stripped entries AND validation errors.
- **Composite health status** — `internal/health/composite.go` fuses HTTP probe result + endpoint readiness + pod diagnostics into a single `compositeStatus` per service. Never set `compositeStatus` directly — it's computed.

### Testing Rules

**Go:**
- Co-located `_test.go` files in the same package (white-box testing).
- `go test -race ./...` is mandatory — never skip the race detector.
- `make test` creates a stub `web/build/index.html` so `embed.FS` doesn't break Go tests.
- Use interface-based test doubles (e.g., `HTTPProber`, `StateReader`, `StateWriter`) — no mocking frameworks.
- Prefer table-driven tests with `t.Run` subtests for coverage of edge cases.

**Frontend (Vitest + Testing Library):**
- Co-located `.test.ts` files next to source (e.g., `ServiceRow.test.ts` beside `ServiceRow.svelte`).
- jsdom environment configured in `vite.config.ts`, setup file at `web/vitest-setup.ts`.
- Use `@testing-library/svelte` `render()` + queries (`getByRole`, `getByText`) — no direct DOM manipulation.
- `@testing-library/jest-dom` matchers available globally via tsconfig types.
- Structural tests enforce store completeness — `serviceStore.structural.test.ts` asserts `_resetForTesting()` covers all `$state` fields and checks getter counts.
- Always call `_resetForTesting()` in `beforeEach` for store tests to prevent cross-test state leakage.
- SSE client tests must validate type guards for every payload shape — no trusting external data.

### Code Quality & Style

**Prettier (frontend):**
- Tabs for indentation (not spaces)
- Single quotes, no trailing commas
- Print width: 100 characters
- Svelte files use `prettier-plugin-svelte` parser
- Run: `cd web && npx prettier --write .`

**ESLint (frontend):**
- Flat config (`web/eslint.config.js`) — ESLint 10 + typescript-eslint + eslint-plugin-svelte
- `svelte/prefer-svelte-reactivity` is **off** (project uses manual reactivity patterns in stores)
- `svelte/no-navigation-without-resolve` is **off**
- Run: `cd web && npx eslint .`

**Go formatting:**
- `gofmt` / `goimports` — standard Go formatting, no custom config
- No linter config file (no golangci-lint) — rely on `go vet` and race detector

**File organization:**
- Go: `internal/` for all packages, `cmd/command-center/` for entrypoint, `embed.go` at root
- Frontend: `web/src/lib/` for stores/utils/types, `web/src/lib/components/` for UI components, `web/src/routes/` for pages
- Components get their own file + co-located test (e.g., `StatusBar.svelte` + `StatusBar.test.ts`)

### Development Workflow

**Build & Run:**
- `make build` — builds frontend (`npm ci && npm run build` in `web/`), then Go binary to `bin/command-center`
- `make dev` — runs Vite dev server + Go backend concurrently (Go uses `--dev` flag for proxy mode)
- `make test` — runs both Go and frontend tests (stubs `web/build/` first for embed)
- `make docker` — container build with version injection via `--build-arg`

**Version injection:**
- `Version` var in `main.go` set via `-ldflags "-X main.Version=$(VERSION)"`
- `VERSION` derived from `git describe --tags --always --dirty`

**Embed pipeline:**
- `embed.go` at project root uses `//go:embed all:web/build`
- Frontend must be built before Go compilation — `make build` handles ordering
- For Go tests without a full frontend build, `make test` creates a stub `web/build/index.html`

**Dev mode:**
- `--dev` flag on Go binary enables `server.DevProxy` which reverse-proxies to Vite's dev server
- Allows hot-reload of frontend while Go backend serves API and SSE endpoints

### Critical Don't-Miss Rules

**Anti-patterns:**
- **No third-party routers** — `net/http` only. This is the most common agent mistake.
- **No `export let` in Svelte** — Svelte 5 uses `$props()`. `export let` compiles but is legacy Svelte 4.
- **No direct `compositeStatus` assignment** — always computed by `health/composite.go` from fused signals.
- **No `log.Fatal` outside main** — library packages return errors; only `main()` calls `log.Fatal`.
- **No caching state outside the store** — packages read from `state.Store` via interfaces, never maintain shadow copies.

**Edge cases:**
- **Empty/missing config file is valid** — `config.Load` returns empty `Config{}` with no errors for missing or empty files. Don't treat this as a failure.
- **K8s watcher may not be running** — the binary works without a kubeconfig (config-only services still function). Guard against nil watcher.
- **SSE reconnect** — frontend `sseClient.ts` handles reconnect. On reconnect, the broker sends a full `state` event (not deltas), so the frontend replaces all services via `replaceAll()`.
- **`time.Time` JSON serialization** — Go's `*time.Time` serializes to RFC 3339. Frontend `parseLastChecked()` validates with `new Date()` + `isNaN` check. Never assume dates parse successfully.

**Performance:**
- **Don't poll K8s** — use informers/watchers with event handlers. Polling the API server is forbidden.
- **Health checker runs on interval** — don't trigger ad-hoc health checks from the frontend. The checker owns the cadence.
- **SSE keepalive prevents proxy timeouts** — the 15s keepalive in `Broker` is critical for reverse proxy environments. Don't remove or increase it significantly.

---

## Project Structure

```
cmd/command-center/main.go    # Entrypoint — wiring only
embed.go                      # //go:embed all:web/build
internal/
  certs/                      # Self-signed TLS cert generation
  config/                     # YAML config loader + fsnotify watcher
  health/                     # HTTP health checker + composite status
  history/                    # Health history writer/reader/pruner
  k8s/                        # Ingress watcher + EndpointSlice watcher
  server/                     # Static file serving + dev proxy
  session/                    # Session management + middleware
  sse/                        # SSE broker + event formatting
  state/                      # Authoritative service state store
web/
  src/lib/                    # Stores, types, utils
  src/lib/components/         # Svelte components + co-located tests
  src/routes/                 # SvelteKit pages
```

---

## Usage Guidelines

**For AI Agents:**
- Read this file before implementing any code
- Follow ALL rules exactly as documented
- When in doubt, prefer the more restrictive option
- Update this file if new patterns emerge

**For Humans:**
- Keep this file lean and focused on agent needs
- Update when technology stack changes
- Review quarterly for outdated rules
- Remove rules that become obvious over time

Last Updated: 2026-02-27
