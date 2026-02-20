.PHONY: build test dev clean docker

CONTAINER_TOOL ?= $(shell if command -v docker >/dev/null 2>&1; then echo docker; elif command -v podman >/dev/null 2>&1; then echo podman; else echo docker; fi)

build:
	cd web && npm ci && npm run build
	go build -o bin/command-center ./cmd/command-center

docker:
	$(CONTAINER_TOOL) build -t command-center .

test:
	@# Ensure web/build exists so go test -v ./... doesn't fail on embed.FS
	@mkdir -p web/build && touch web/build/index.html
	go test ./...
	cd web && npx vitest run

dev:
	@set -m; cd web && npm run dev & VITE_PID=$$!; \
	trap "kill -- -$$VITE_PID 2>/dev/null; wait" EXIT INT TERM; \
	go run ./cmd/command-center --dev

clean:
	rm -rf bin/ web/build/ web/.svelte-kit/
