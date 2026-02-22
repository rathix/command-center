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
