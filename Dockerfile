# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.26-alpine AS backend
WORKDIR /app
COPY go.mod go.su[m] ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/build ./web/build
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /command-center ./cmd/command-center

# Stage 3: Final distroless image
FROM gcr.io/distroless/static-debian12
COPY --from=backend /command-center /command-center
EXPOSE 8443
USER 65532:65532
ENTRYPOINT ["/command-center"]
