# GEMINI.md

## Project Overview

Command Center — a Kubernetes service dashboard. Go backend serving a SvelteKit SPA via embed.FS as a single binary.

## Security — PUBLIC REPOSITORY

This project is hosted on a **public Git repository**. All commits, branches, and history are visible to anyone. Follow these rules without exception:

- **Never commit secrets** — no API keys, tokens, passwords, certificates, or private keys
- **No hardcoded credentials** — use environment variables or config files excluded from git
- **No `.env` files in git** — only commit `.env.example` with placeholder values
- **Audit before committing** — review staged changes for accidental secret inclusion
- **Dependencies** — only add well-known, maintained packages; avoid typosquat risks
- **No sensitive data in logs or comments** — no internal URLs, IPs, or infrastructure details in code

## Tech Stack

- **Go 1.26** — standard library `net/http`, no third-party routers
- **SvelteKit** (adapter-static) in `web/` — Svelte 5, TypeScript, Tailwind v4, Vitest
- **Build** — `make build` produces single binary at `bin/command-center`
- **Tests** — `go test ./...` and `cd web && npx vitest run`

## Versioning

Semantic versioning with git tags tied to the story/epic lifecycle:

- **Story complete** — patch bump (v0.2.1, v0.2.2, ...)
- **Epic complete** — minor bump (v0.3.0, v0.4.0, ...)
- **Feature-complete** — major bump to v1.0.0 when all epics are done

After pushing a story or epic commit, create an annotated tag and push it:

```
git tag -a vX.Y.Z -m "description" && git push origin vX.Y.Z
```

Current: v0.2.0 (Epic 2 complete). Next story tag: v0.2.1.
