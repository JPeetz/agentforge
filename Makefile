# Makefile — AgentForge

.GOOS ?= $(shell go env GOOS)
.GOARCH ?= $(shell go env GOARCH)
VERSION ?= 0.1.0
LDFLAGS := -s -w -X github.com/agentforge/agentforge/internal/config.version=$(VERSION)
BINARY := agentforge
DOCKER_IMAGE := agentforge/agentforge

.PHONY: all build test lint clean install run daemon docker docker-build docs fmt vet

all: build

## ──── Compilation ────────────────────────────────────────────

build: cmd/agentforge/main.go
	@echo "  BUILD  $(BINARY)"
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agentforge
	@echo "  DONE   $(BINARY) ($$(wc -c < $(BINARY)) bytes)"

agentctl: cmd/agentctl/main.go
	@echo "  BUILD  agentctl"
	go build -ldflags "$(LDFLAGS)" -o agentctl ./cmd/agentctl
	@echo "  DONE   agentctl"

## ──── Cross-compilation ──────────────────────────────────────

darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64 ./cmd/agentforge

darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64 ./cmd/agentforge

linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 ./cmd/agentforge

linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-arm64 ./cmd/agentforge

windows-amd64:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-windows-amd64.exe ./cmd/agentforge

## ──── Docker ─────────────────────────────────────────────────

docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		-t $(DOCKER_IMAGE):$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		-f deploy/docker/Dockerfile .

docker-run:
	docker run --rm -it \
		-v $$(pwd)/config.yaml:/etc/agentforge/config.yaml \
		-v agentforge-memory:/var/lib/agentforge \
		-p 8080:8080 \
		-p 9090:9090 \
		$(DOCKER_IMAGE):$(VERSION) daemon

## ──── Quality ────────────────────────────────────────────────

test:
	@echo "  TEST   all packages"
	go test -v -race -timeout 60s ./...

test-cover:
	@echo "  TEST   coverage"
	go test -v -race -timeout 60s -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...
	go vet ./...

check: fmt lint test

## ──── Docs ───────────────────────────────────────────────────

docs:
	@echo "  DOCS   generating..."
	@mkdir -p docs/generated
	# OpenAPI spec generation placeholder
	@echo "  DONE   docs"

## ──── Clean / Install ────────────────────────────────────────

clean:
	rm -f $(BINARY) agentctl
	rm -f $(BINARY)-darwin-* $(BINARY)-linux-* $(BINARY)-windows-*.exe
	rm -f coverage.out coverage.html

install: build
	@echo "  INST   /usr/local/bin/"
	install -Dm755 $(BINARY) /usr/local/bin/$(BINARY)

## ──── Run ─────────────────────────────────────────────────────

run: build
	./$(BINARY) run

daemon: build
	./$(BINARY) daemon --config config.yaml

shell: build
	./agentctl

## ──── Development helpers ─────────────────────────────────────

dev: build
	TUI=1 ./$(BINARY) run

watch:
	@# Requires: go install github.com/ EntropyBobby/swag@latest
	@echo "  LIVE   reloading not yet wired"
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agentforge && ./$(BINARY) run

deps:
	go mod download
	go mod tidy

## ──── Help ───────────────────────────────────────────────────

help:
	@echo "AgentForge Makefile  (v$(VERSION))"
	@echo ""
	@echo "  Targets:"
	@sed -n 's/^\([a-z-]*\):.*## \(.*\)/  \1  \2/p' $(MAKEFILE_LIST) | column -t -s "  " | expand -t 16
