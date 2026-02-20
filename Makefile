.PHONY: build test dev clean docker

build:
	cd web && npm ci && npm run build
	go build -o bin/command-center ./cmd/command-center

docker:
	podman build -t command-center .

test:
	go test ./...
	cd web && npx vitest run

dev:
	@set -m; cd web && npm run dev & VITE_PID=$$!; \
	trap "kill -- -$$VITE_PID 2>/dev/null; wait" EXIT INT TERM; \
	go run ./cmd/command-center -dev

clean:
	rm -rf bin/ web/build/ web/.svelte-kit/
