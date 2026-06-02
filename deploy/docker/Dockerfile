FROM golang:1.24-alpine AS builder

WORKDIR /src

# Install Go dependencies first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
ARG VERSION=0.1.0
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w -X github.com/agentforge/agentforge/internal/config.version=${VERSION}" \
    -o agentforge \
    ./cmd/agentforge

# ── Runtime image ─────────────────────────────────────────────
FROM alpine:3.21

# CA certs + utilities
RUN apk add --no-cache ca-certificates git openssh-client sqlite-libs

WORKDIR /app

# Copy static binary
COPY --from=builder /src/agentforge /app/agentforge
COPY deploy/docker/config.example.yaml /app/config.yaml

# Linux home for memory, config, logs
ENV AGENTFORGE_HOME=/app/data
ENV AGENTFORGE_CONFIG=/app/config.yaml

RUN mkdir -p /app/data/memory /app/data/logs

# Non-root user
RUN adduser -D -g '' appuser
RUN chown -R appuser:appuser /app
USER appuser

EXPOSE 8080 9090

ENTRYPOINT ["/app/agentforge"]
CMD ["daemon"]
