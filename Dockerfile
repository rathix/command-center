# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.26-alpine AS backend
ARG VERSION="dev"
WORKDIR /app
COPY go.mod go.su[m] ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/build ./web/build
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=${VERSION}" -o /command-center ./cmd/command-center

# Stage 3: Final distroless image
FROM gcr.io/distroless/static-debian12:nonroot
ARG IMAGE_SOURCE="https://github.com/rathix/command-center"
LABEL org.opencontainers.image.source=$IMAGE_SOURCE
LABEL org.opencontainers.image.description="Kubernetes service dashboard"
LABEL org.opencontainers.image.licenses="MIT"
COPY --from=backend /command-center /command-center
COPY --from=backend --chown=nonroot:nonroot /dev/null /data/.keep
EXPOSE 8443
USER nonroot:nonroot
ENTRYPOINT ["/command-center"]
