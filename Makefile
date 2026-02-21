.PHONY: build test dev clean docker encrypt-secrets

CONTAINER_TOOL ?= $(shell if command -v docker >/dev/null 2>&1; then echo docker; elif command -v podman >/dev/null 2>&1; then echo podman; else echo docker; fi)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Ensure version is not empty
ifeq ($(VERSION),)
$(error VERSION is not set)
endif

LDFLAGS = -X main.Version=$(VERSION)

build:
	cd web && npm ci && npm run build
	go build -ldflags "$(LDFLAGS)" -o bin/command-center ./cmd/command-center

docker:
	$(CONTAINER_TOOL) build --build-arg VERSION="$(VERSION)" -t command-center .

test:
	@# Ensure web/build exists so go test -v ./... doesn't fail on embed.FS
	@mkdir -p web/build && touch web/build/index.html
	go test ./...
	cd web && npx vitest run

dev:
	@set -m; cd web && npm run dev & VITE_PID=$$!; \
	trap "kill -- -$$VITE_PID 2>/dev/null; wait" EXIT INT TERM; \
	go run -ldflags "$(LDFLAGS)" ./cmd/command-center --dev

encrypt-secrets:
	go build -o bin/encrypt-secrets ./cmd/encrypt-secrets

clean:
	rm -rf bin/ web/build/ web/.svelte-kit/
