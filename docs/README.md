# AgentForge Documentation

Welcome to the AgentForge documentation. AgentForge is a capability-secured, concurrent-native AI agent orchestration framework built in Go.

## Overview

| Document | Description |
|----------|-------------|
| [README](/README.md) | Project overview, quick start, competitor comparison |
| [Architecture](/ARCHITECTURE.md) | Full architecture spec — packages, data flow, security model |
| [Contributing](/CONTRIBUTING.md) | Dev setup, coding standards, how to add providers/tools/skills |
| [Changelog](/CHANGELOG.md) | Version history with Added/Changed/Fixed sections |
| [License](/LICENSE) | BUSL-1.1 (free for any use except production SaaS) |

## Getting Started

### Installation

```bash
# Build from source
git clone https://github.com/agentforge/agentforge.git
cd agentforge
make build  # → ./agentforge (10MB static binary)

# Docker
docker run -p 8080:8080 agentforge/agentforge:latest

# Download binary
curl -L https://github.com/agentforge/agentforge/releases/latest/download/agentforge-darwin-arm64 -o agentforge
chmod +x agentforge
```

### Quick Start

```bash
# Generate default config
agentforge config generate

# Start daemon
agentforge daemon

# Open dashboard
open http://localhost:8080/dashboard
```

## Core Concepts

### Capability-Based Security
Every agent receives an HMAC-signed capability token at spawn. Tokens declare:
- Filesystem allowlists/denylists
- Network domain allowlists
- Shell/spawn permissions
- Token budgets and timeouts
- Audit logging

No ambient authority. The principle of least privilege enforced at the runtime level.

### LLM Adapters
Pluggable provider system with:
- **OpenAI** — GPT-4o, GPT-4.1 (1M context), o-series
- **Anthropic** — Claude 3/4 (200K context)
- **Ollama** — Local models (Gemma, Llama, Mistral)
- **Circuit breaker** — automatic fail-open after threshold
- **Fallback chain** — ordered provider failover
- **Model-aware context windows** — auto-detected, not hardcoded

### Agent Engine
Goroutine-per-agent pool architecture:
- **Departments** — content, SEO, social, security, DevOps, memory, orchestrator, monitor
- **Worker pools** — configurable max agents per department
- **DAG pipelines** — multi-stage execution with dependency ordering
- **Subagent trees** — capability delegation with fan-out/fan-in

### Tool Registry
19 built-in tools across 10 categories:

| Category | Tools |
|----------|-------|
| Filesystem | read, write, edit |
| Network | HTTP, web_search, web_fetch |
| Memory | memory_search, memory_get |
| VCS | git_commit, git_push |
| Agent | session_spawn, session_send |
| Automation | cron_schedule |
| Shell | shell |
| Media | image_generate |
| Sandbox | code_exec |
| Browser | browser_navigate |
| MCP | mcp_invoke |

### Session Management
- **Transcript persistence** — JSON with timestamps, roles, token counts
- **Auto-compaction** — triggers at 90% of model's actual context window (1M for GPT-4.1, 200K for Claude/GPT-4o)
- **Memory flush** — full pre-compaction context written to MeMex before summarization — nothing lost
- **Pruning** — auto-trims tool outputs beyond configurable threshold (default 8K chars)
- **Tail preservation** — keeps recent conversation unsummarized for LLM continuity

### Memory: MeMex Zero RAG
Filesystem-first architecture:
- Markdown files as the canonical data format
- Git versioning for all changes
- SQLite FTS5 for full-text search
- File watcher for real-time indexing
- Deterministic, grep-able, git-trackable — no vector soup

### Skills System
- **SKILL.md** — full Anthropic spec support (name, description, when_to_use, tools, model, hooks)
- **Auto-activation** — scoring engine matches skills to task context
- **Marketplace** — SkillsMP.com API integration (bring your own API key)
- **Skill → Tool** — every skill auto-registers as a tool + MCP definition
- **Self-Learning** — Observer→Extractor→Generator pipeline auto-creates SKILL.md from user behavior

### Cron Scheduler
Native Go cron engine with no external dependencies:
- **Schedule formats** — cron-format (`30 8 * * *`) + `@every 5m` shorthand
- **Pipeline integration** — pipelines with `trigger.cronExpr` automatically register
- **cron_schedule tool** — agents can create cron jobs at runtime
- **Fire loop** — goroutine ticker checks due jobs every second, sorts by NextRun

### Multi-MCP Server
Run any number of MCP servers from a single daemon:
- **Per-server config** — name, port, transport (HTTP or stdio), tool filter
- **Stdio transport** — spawn command subprocess with JSON-RPC on stdin/stdout
- **HTTP transport** — dedicated `http.ServeMux` per server
- **Dashboard management** — view all servers with status, port, transport, tool count

### Channel Adapters
Connect AgentForge natively to messaging platforms:
- **Telegram** — polling `getUpdates` API, command handling (/start, /help, /status)
- **Discord** — WebSocket Gateway v10, IDENTIFY handshake, heartbeat loop
- **Native WebSocket** — RFC 6455 implementation, zero external dependencies
- **Bus integration** — messages published on `channel.{name}.message` topics
- **Reconnect** — exponential backoff on connection failures
- **Dashboard** — status cards with connection state, message count, last activity

### Web Dashboard
Glassmorphism SPA with 15 pages:
- **Overview** — stat cards, pipeline status, cost tracking, token usage, recent events
- **Agent Fleet** — full CRUD modal editor, capability toggles, department assignment
- **Memory** — file browser, search, MeMex status
- **Pipelines** — visual DAG editor, stage management, dependency ordering
- **Skills** — local skill browser + marketplace search (keyword/AI)
- **Settings** — 9 tabbed sections, 50+ interactive toggles, save to YAML
- **Security** — capability viewer, audit log
- **Tools** — tool catalog with descriptions and parameters
- **Chat** — live agent chat interface
- All pages loaded via htmx partials with real data from runtime config

## Configuration Reference

```yaml
daemon:
  host: 127.0.0.1
  port: 8080

mcp:
  port: 9090
  enabled: true

memory:
  root: ~/.agentforge/memory
  autoCommit: true
  commitInterval: 30s
  indexEnabled: true

llm:
  provider: openai          # openai | anthropic | ollama | openrouter
  model: gpt-4o
  temperature: 0.7
  maxTokens: 4096
  retryCount: 3
  maxConcurrency: 10

security:
  defaultTokenBudget: 1000000
  defaultTimeout: 3600s
  enforceOnSpawn: true
  allowFileSystem: true
  allowShell: false
  sandboxMode: non-main

session:
  maxContextTokens: 0       # 0 = auto-detect from LLM adapter
  autoCompactAt: 90         # compact at 90% of context window
  keepRecentTokens: 8000
  notifyUser: true
  pruneToolOutputs: true
  maxToolOutputChars: 8000

workers:
  contentMaxAgents: 3
  seoMaxAgents: 1
  defaultMaxAgents: 2

tools:
  webSearch: true
  webFetch: true
  gitOps: true
  cron: true

skills:
  autoInstall: true
  marketplace: skillsmp
  marketplaceUrl: https://skillsmp.com/api/v1

agents:
  profiles:
    - name: Content Writer
      department: content
      model: openai/gpt-4o
      temperature: 0.7
      maxTokens: 4096
      capability:
        allowFileSystem: true
        allowNetwork: true
        allowShell: false
        tokenBudget: 500000

channels:
  telegram:
    enabled: true
    botToken: "${TELEGRAM_TOKEN}"
  discord:
    enabled: false
```

## Deployment

### Docker
```bash
docker compose -f deploy/docker/docker-compose.yaml up -d
```

### Kubernetes
```bash
kubectl apply -f deploy/k8s/
```

### Systemd
```bash
sudo cp deploy/systemd/agentforge.service /etc/systemd/system/
sudo systemctl enable --now agentforge
```

### Air-Gapped
```bash
# Copy the 10MB binary + Ollama
scp agentforge airgap-server:/usr/local/bin/
scp ~/.ollama/models airgap-server:~/.ollama/
```

## Development

```bash
git clone https://github.com/agentforge/agentforge.git
cd agentforge
make deps        # go mod download
make build       # compile
make test        # run tests
make daemon      # start daemon
make all         # build + test + lint + vet
```

## Community

- [GitHub Discussions](https://github.com/agentforge/agentforge/discussions)
- [Discord](https://discord.gg/agentforge)
- [Blog](https://agentforge.dev/blog)
- [Website](https://agentforge.dev)

## License

AgentForge is [BUSL-1.1](/LICENSE) — free for any use except production SaaS hosting. Converts to Apache 2.0 after 4 years.