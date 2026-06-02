# Contributing to AgentForge

We welcome contributions. AgentForge is built in the open — every agent, tool, and pipeline is a community asset.

## Architecture Philosophy

Before you contribute, understand the five pillars:

1. **Capability-based security** — no ambient authority, ever. Every agent gets an HMAC-signed token.
2. **CSP concurrency** — goroutines are agents, channels are communication.
3. **MeMex Zero RAG** — filesystem is the database. Deterministic, grep-able, git-tracked.
4. **WASM sandboxes** — third-party tools run isolated, content-addressed.
5. **Single binary** — one static Go binary. No runtime, no venv, no docker required.

## Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- [Ollama](https://ollama.com) (optional, for local LLM)
- [Docker](https://docker.com) (optional, for containerized dev)

## Getting Started

```bash
git clone https://github.com/agentforge/agentforge.git
cd agentforge

# Build
make build          # → ./agentforge (10MB static binary)

# Run tests
make test           # go test ./...

# Lint
make lint           # golangci-lint run

# Start daemon
make daemon         # agentforge daemon --config deploy/docker/config.example.yaml

# All checks
make all            # build + test + lint + vet
```

## Project Structure

```
agentforge/
├── cmd/agentforge/       # CLI + daemon (cobra, viper)
├── internal/
│   ├── engine/           # Agent pool, departments, DAG, subagent trees
│   ├── bus/              # CSP message bus
│   ├── memory/           # MeMex Zero RAG (git + SQLite + FTS5)
│   ├── security/         # Capability tokens, enforcement
│   ├── llm/              # LLM adapters (OpenAI, Ollama, Anthropic)
│   ├── tool/             # Tool registry (19 builtins)
│   ├── api/mcp/          # MCP server (JSON-RPC 2.0)
│   ├── session/          # Session management, compaction, pruning
│   ├── skill/            # SKILL.md loader, marketplace
│   ├── dashboard/        # Web dashboard (SPA, htmx)
│   └── config/           # Configuration management
├── deploy/               # Docker, docker-compose, K8s manifests
├── docs/                 # Documentation
└── skills/               # Example skills
│   └── auto/             # Auto-generated skills (self-learning)
```

## Development Workflow

### 1. Pick an Issue

Check the [issues](https://github.com/agentforge/agentforge/issues) for `good-first-issue` or `help-wanted` labels.

### 2. Create a Branch

```bash
git checkout -b feat/my-feature
# or: fix/my-bug, docs/my-docs, chore/my-chore
```

### 3. Make Changes

Follow Go conventions:
- `gofmt` all code
- Write tests for new functionality
- Document exported types and functions
- Keep packages focused — one responsibility per package

### 4. Run Tests

```bash
go test ./... -race -count=1
go vet ./...
golangci-lint run
```

### 5. Submit a PR

- Describe what changed and why
- Link related issues
- Include screenshots for UI changes
- Ensure CI passes (GitHub Actions)

## Adding a New LLM Provider

1. Create a client struct in `internal/llm/adapter.go`
2. Implement the `Adapter` interface: `Provider()`, `ContextWindow()`, `Chat()`, `HealthCheck()`
3. Register the provider in the daemon's provider detection
4. Add config defaults in `internal/config/config.go`
5. Add a provider config block in the example config

```go
type MyProviderClient struct {
    cfg MyProviderConfig
}

func (c *MyProviderClient) Provider() string       { return "myprovider" }
func (c *MyProviderClient) ContextWindow() int     { return 200_000 }
func (c *MyProviderClient) Chat(ctx context.Context, req Request) (Response, error) { ... }
func (c *MyProviderClient) HealthCheck(ctx context.Context) error { ... }
```

## Adding a New Tool

1. Implement the `Tool` interface in `internal/tool/`:

```go
type MyTool struct{}

func (t *MyTool) Meta() ToolMeta {
    return ToolMeta{
        Name:        "my_tool",
        Description: "What this tool does",
        Category:    "custom",
        Parameters:  []ParamDef{{Name: "input", Type: "string", Required: true}},
    }
}

func (t *MyTool) Execute(ctx context.Context, args map[string]any, cap *Capability) (string, error) {
    // Enforce capability first
    // Execute the tool
    return "result", nil
}
```

2. Register it in the daemon's tool seeding
3. Add to the tool catalog in the dashboard

## Adding a Skill

Create a `SKILL.md` file following the [Anthropic spec](https://docs.anthropic.com/en/docs/agents-and-tools/agent-skills):

```markdown
---
name: my-skill
description: What this skill does and when to use it
allowed-tools: [read_file, web_search]
---

# My Skill

Instructions for the agent...
```

Place it in `skills/my-skill/SKILL.md`. The loader auto-discovers it at startup.

## Security Considerations

- Never bypass capability enforcement in tool implementations
- Always validate tool inputs — treat them as untrusted
- Don't log API keys or capability secrets
- Use `security.DeriveCapability()` for subagent delegation
- Respect `cap.TokenBudget` and `cap.ExpiresAt`

## Documentation

- Architecture: `ARCHITECTURE.md`
- User docs: `docs/README.md`
- API docs: generated from Go doc comments
- Dashboard: `internal/dashboard/UI-DESIGN.md`

## Adding a Channel Adapter

1. Create your adapter in `internal/channel/` implementing the `Adapter` interface:
   - `Name() string` — unique adapter identifier
   - `Start(ctx context.Context, bus bus.Bus) error` — connect, start polling/WS
   - `Stop() error` — graceful disconnect
   - `Status() Status` — runtime state snapshot
2. Register it in the Channel Manager's `NewManager()`
3. Add config struct in `internal/config/config.go` under `ChannelsConfig`
4. Add channel settings to the dashboard Settings page
5. Publish inbound messages on `channel.{name}.message` bus topic

### Adding a Learning Heuristic

Extend `internal/learn/learn.go`:
- Add similarity functions to the Extractor (`wordOverlap`, `intentMatch`, etc.)
- Tune `Confidence()` scoring weights in the Pattern struct
- Add trigger extraction logic in `extractTriggers()`
- Auto-generated skills write to `skills/auto/` — ensure the directory exists

## Community

- [GitHub Discussions](https://github.com/agentforge/agentforge/discussions)
- [Discord](https://discord.gg/agentforge)
- [Blog](https://agentforge.dev/blog)

## License

By contributing, you agree that your contributions will be licensed under the [BUSL-1.1](LICENSE).