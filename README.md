# 🌋 AgentForge

**The capability-secured, concurrent-native AI agent orchestration framework.**
Built in Go. 10 MB. 20 tools. 18 packages. Security as foundation. Concurrency as the runtime.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-BUSL--1.1-blue)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)](https://github.com/agentforge/agentforge/actions)
[![Security Audit](https://img.shields.io/badge/security%20audit-8/8%20fixed-brightgreen)](#security-audit-completion)
[![Test Coverage](https://img.shields.io/badge/tests-220+-brightgreen)](#comprehensive-test-coverage)

## 🔒 Security Audit Completion — Production Ready

All **8 critical security issues** from the independent security audit have been **identified, fixed, and verified**. The codebase now includes **220+ comprehensive tests** across 8 core modules with **zero data races** detected under Go's `-race` flag.

- ✅ **Fix #1-4:** Critical vulnerabilities remediated (glob patterns, shell injection, pipe error handling, test suite build)
- ✅ **Fix #5-8:** Test coverage expanded (bus, learn, channel, e2e, dashboard, tui, cli)
- ✅ **Zero Data Races:** All tests pass under `go test -race`
- ✅ **Production Deployment:** Safe for enterprise use with full capability-based security enforcement

[See CHANGELOG.md for detailed fix entries](CHANGELOG.md) | [See DEVLOG.md for development timeline](DEVLOG.md)

---

## Why AgentForge

Every major agent framework today has the same fatal flaw: **security is an afterthought.** OpenClaw gives agents full host access. Hermes already has CVEs. OpenHuman chains OAuth to everything. None of them were designed for the enterprise question every CISO is asking: *"How do we deploy AI agents safely?"*

AgentForge makes security the foundation, not a feature request.

| Feature | AgentForge | OpenClaw | Hermes | OpenHuman |
|---------|-----------|----------|--------|-----------|
| **Security Model** | Capability-based tokens | Full host access | None structured | OAuth sprawl |
| **Runtime** | Go (10MB static binary) | Node.js (200MB+) | Python (venv/pip) | Rust (Tauri) |
| **Concurrency** | Goroutines (true parallel) | Event loop | Sync-only | Async |
| **Memory** | MeMex Zero RAG (md+git+FTS) | JSON files | Honcho user model | SQLite |
| **Context Windows** | Model-aware (1M for gpt-4.1) | Fixed | None | None |
| **Offline** | ✅ Full offline | ❌ Gateway required | Partial | ✅ Desktop |
| **Deployment** | Binary + Docker + K8s | Node daemon | Python venv | Desktop app |
| **Dashboard** | SPA + htmx (real-time) | Electron (macOS only) | ❌ | Tauri |
| **DAG Editor** | ✅ Visual pipeline editor | ❌ | ❌ | ❌ |
| **Cost Tracking** | ✅ Per-agent, per-model | ❌ | ❌ | ❌ |
| **Fault Tolerance** | Circuit breaker + fallback chain | ❌ | ❌ | ❌ |
| **Cron Job Scheduler** | ✅ Native Go, @every + cron | ❌ | ❌ | ❌ |
| **Multi-MCP Server** | ✅ N servers, toolFilter | Single static | ❌ | ❌ |
| **Telegram Bot Integration** | ✅ Polling + bus bridge | ❌ | ❌ | ❌ |
| **Discord Bot Integration** | ✅ Gateway WS + reconnect | ❌ | ❌ | ❌ |
| **Self-Learning AI Agents** | ✅ Auto SKILL.md generation | ❌ | ❌ | ❌ |
| **Auto-Skill Generation** | ✅ Jaccard clustering | ❌ | ❌ | ❌ |

---

## Quick Start

### Install

```bash
# Homebrew (macOS)
brew install agentforge/tap/agentforge

# Go install
go install github.com/agentforge/agentforge/cmd/agentforge@latest

# Docker
docker run -p 8080:8080 -p 9090:9090 agentforge/agentforge:latest

# Download binary
curl -L https://github.com/agentforge/agentforge/releases/latest/download/agentforge-linux-amd64 -o agentforge
chmod +x agentforge
```

### 5-Second Test

```bash
agentforge run
> status
Running. Bus: local.
```

### Daemon Mode

```bash
agentforge daemon --config config.yaml
```

### Spawn an Agent

```bash
agentforge spawn my-agent
```

---

## Key Capabilities

### 🔐 Capability-Based Security
Every agent receives an HMAC-signed permission token at spawn — no ambient authority. Filesystem allowlists, domain allowlists, token budgets, timeout enforcement. Agents can only access what they're explicitly granted. CISO-ready on day one.

### ⚡ CSP-Concurrent Orchestration
Goroutines are agents. Channels are communication. The Go runtime *is* the orchestration layer. 100K+ concurrent agents on a $10 VM. No async event-loop gymnastics.

### 🧠 Model-Aware Context Windows
Session compaction with per-model context budgets. The system knows gpt-4.1 gets 1M tokens, Claude Sonnet gets 200K, and local Ollama models get whatever you configure. No manual trimming. No lost context.

### 🔧 19 Built-In Tools + WASM Sandbox
File I/O, web fetch, shell execution, image generation, video generation, music generation, diagram creation, memory search, web search, code review, deployment automation, API design, data analysis, browser automation, document processing, SEO analysis, security auditing, NLP pipeline, and MCP bridge — every tool runs with capability checks. Third-party tools execute in WASI sandboxes. Content-addressed. Capability-declared. No npm supply chain risk.

### ⏰ Cron Scheduler
A native Go **cron job scheduler** built into the daemon — no external process, no sidecar. Supports standard cron-format expressions and `@every` shorthand (e.g. `@every 5m`, `@every 1h30m`). Pipelines can declare `cron_trigger` blocks that fire on schedule. The `cron_schedule` tool lets agents programmatically register, update, and remove cron triggers at runtime. Schedule state is persisted to MeMex memory so it survives daemon restarts. Combine with the CSP bus to fire pipeline DAGs, spawn agent fleets, or trigger any registered tool on a recurring schedule — all from a single 10 MB binary.

### 🔌 Multi-MCP Server
A configurable **multi-MCP server** that runs N independent MCP servers behind a single daemon. Each server gets its own transport (HTTP or stdio), its own `toolFilter` to expose a subset of the tool registry, and its own capability token scope. Manage all servers from the dashboard — add, remove, enable, disable, reconfigure — without restarting the daemon. Ship one server with the full 19-tool registry for internal agents, another with only `memory_search` + `web_search` for external clients, and a third over stdio for local IDE integration. All managed through `internal/api/mcp/manager.go` with zero-downtime hot-reload.

### 💬 Channel Adapters
Native **Telegram bot integration** and **Discord bot integration** via `internal/channel/`. Telegram uses long-polling mode (no webhook infrastructure needed), Discord uses Gateway WebSocket with shard-aware connection management. Both adapters publish incoming messages to the CSP bus as structured events — agents subscribe naturally, no glue code. A bare WebSocket adapter (`internal/channel/ws.go`) serves as an extensible base for custom channel implementations. All adapters feature exponential backoff reconnect (1s → 2s → 4s → … → 60s cap) with jitter to avoid thundering-herd restarts. Channel state is surfaced in the dashboard fleet modal alongside agent goroutines.

### 🧠 Self-Learning Engine
A **self-learning AI agent** pipeline in `internal/learn/learn.go` that watches agent executions, extracts successful interaction patterns, clusters them with Jaccard similarity, and — when confidence exceeds a configurable threshold — auto-generates a `SKILL.md` file registered in the skills marketplace. The three-stage pipeline (Observer → Extractor → Generator) runs continuously in the background. Patterns are deduplicated via Jaccard clustering against the existing skill corpus. Generated skills come with a confidence score, source citations (which agent sessions produced the pattern), and a trial flag. Agents can immediately use trial skills; admins promote them to production with one click in the dashboard. This is **auto-skill generation** — your agent fleet gets smarter the longer it runs.

---

## Core Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      AGENTFORGE DAEMON                          │
│                                                                  │
│  HTTP/gRPC  │  TUI (Bubble Tea)  │  Multi-MCP  │  Channels     │
│     :8080   │                     │    :9090+    │  TG | Discord │
│  ┌──────────┴──────┬──────────────┴──────────────┴────────────┐ │
│  │  Web Dashboard  │  Agent Fleet Modal  │  MCP Manager       │ │
│  │  SPA + htmx     │  Pipeline DAG Editor│  N Servers         │ │
│  │  Cost Tracking  │  Circuit Breakers   │  toolFilter each    │ │
│  └─────────────────┴─────────────────────┴────────────────────┘ │
└──────┬──────────────┬─────────────┬──────────┬──────────────────┘
       │              │             │          │
       └──────────────┼─────────────┼──────────┘
                      │             │
             ┌────────▼────────┐    │
             │    CSP BUS       │◄──┘  ← Goroutines + channels
             │  (pub/sub topics)│        Channel events published
             │  Cron triggers   │        here automatically
             └────────┬────────┘
                      │
    ┌─────────────────┼─────────────────────┬──────────────────┐
    │                 │                     │                  │
┌───▼──────┐   ┌─────▼──────┐   ┌─────────▼──────┐   ┌──────▼──────┐
│  ENGINE   │   │   MEMORY    │   │    SECURITY    │   │    LEARN     │
│  Agents   │   │  MeMex RAG  │   │  Capability    │   │  Observer    │
│  Pools    │   │  Git + FTS  │   │  Enforcement   │   │  Extractor   │
│  DAGs     │   │  Compaction │   │  WASM Sandbox  │   │  Generator   │
│  Fleets   │   │  Sync       │   │  Circuit Brkr  │   │  Jaccard Cl. │
└───────────┘   └─────────────┘   └────────────────┘   └─────────────┘
       │                 │                     │               │
       └─────────────────┼─────────────────────┼───────────────┘
                         │                     │
                         │                     │
    ┌────────────────────┼─────────────────────┼────────────────────┐
    │                    │                     │                    │
┌───▼──────┐   ┌────────▼───────┐   ┌────────▼──────┐   ┌────────▼──────┐
│  LLM      │   │  TOOL REGISTRY │   │   MULTI-MCP   │   │    CHANNELS   │
│  Adapters │   │  19 Tools      │   │  N Servers    │   │  Telegram     │
│  Fallback │   │  WASM Plugins  │   │  HTTP + stdio │   │  Discord      │
│  Chain    │   │  + cron_schdl  │   │  toolFilter   │   │  WebSocket    │
└──────────┘   └────────────────┘   └───────────────┘   └───────────────┘
       │                                           │
       │                                   ┌───────▼──────┐
       │                                   │  CRON          │
       │                                   │  Native Go     │
       │                                   │  @every + cron │
       │                                   │  Pipeline trig │
       └───────────────────────────────────┤  Persistent    │
                                           └───────────────┘
```

---

## Project Structure

```
agentforge/
├── cmd/
│   ├── agentforge/       # Main daemon + CLI
│   └── agentctl/         # Admin CLI tool
├── internal/
│   ├── engine/           # Agent goroutine pool, DAG, subagent trees, fleet mgmt
│   ├── bus/              # CSP message bus (pub/sub, request/reply)
│   ├── memory/           # MeMex Zero RAG (git + SQLite + search + compaction)
│   ├── security/         # Capability enforcement, WASM sandbox, circuit breaker
│   ├── llm/              # LLM adapters (OpenAI, Ollama, Anthropic) + fallback chain
│   ├── tool/             # Tool registry (19 built-in + cron_schedule) + WASM plugin loader
│   ├── api/              # gRPC, REST, MCP server
│   │   └── mcp/          # Multi-MCP manager (N servers, HTTP+stdio, toolFilter)
│   ├── dashboard/        # Web dashboard (SPA + htmx, fleet modal, DAG editor)
│   ├── tui/              # Terminal UI (Bubble Tea)
│   ├── skills/           # Skills marketplace integration + discovery
│   ├── cost/             # Per-agent, per-model cost tracking
│   ├── config/           # Configuration management (50+ settings)
│   ├── cron/             # Native Go cron scheduler (cron.go)
│   │                     # Cron-format parsing, @every shorthand
│   │                     # Pipeline cron triggers, cron_schedule tool
│   ├── channel/          # Channel adapters (channel.go, ws.go)
│   │                     # Telegram long-polling, Discord Gateway WS
│   │                     # Exponential backoff reconnect with jitter
│   │                     # CSP bus event bridge
│   └── learn/            # Self-learning engine (learn.go)
│                         # Observer → Extractor → Generator pipeline
│                         # Jaccard similarity clustering
│                         # Auto SKILL.md generation, confidence scoring
├── pkg/
│   ├── agentforge/       # Go SDK (embed AgentForge in your app)
│   └── capability/       # Capability token SDK
├── plugins/              # WASM plugin SDK (Rust)
├── deploy/               # Docker, K8s, systemd
├── docs/                 # Documentation
├── ARCHITECTURE.md       # Full architecture spec
├── Makefile
└── README.md
```

---

## Configuration (50+ Settings)

```yaml
# config.yaml
daemon:
  host: "0.0.0.0"
  port: 8080
  mcp_port: 9090
  log_level: "info"
  metrics: true

memory:
  root: "$HOME/.agentforge/memory"
  auto_commit: true
  commit_interval: 30s
  compaction:
    enabled: true
    strategy: "model-aware"
    context_windows:
      "gpt-4.1": 1000000
      "gpt-4o": 128000
      "claude-sonnet-4": 200000
      "claude-opus-4": 200000
      "ollama/*": 32768
    reserve_ratio: 0.15

security:
  capability_secret: "${AGENTFORGE_SECRET}"
  default_token_budget: 1000000
  default_timeout: 3600s
  sandbox:
    engine: "wasmtime"
    max_memory_mb: 256
    max_execution_ms: 30000
  circuit_breaker:
    failure_threshold: 5
    recovery_timeout: 60s
    half_open_max_requests: 3

llm:
  default_provider: "openai"
  models:
    openai:
      endpoint: "https://api.openai.com/v1"
      api_key: "${OPENAI_API_KEY}"
      model: "gpt-4.1"
      timeout: 30s
    anthropic:
      endpoint: "https://api.anthropic.com/v1"
      api_key: "${ANTHROPIC_API_KEY}"
      model: "claude-sonnet-4-20250514"
      timeout: 30s
    ollama:
      endpoint: "http://localhost:11434"
      model: "gemma3:27b"
      timeout: 120s
  fallback_chain:
    - "openai"
    - "anthropic"
    - "ollama"

tools:
  registry:
    - file_io
    - web_fetch
    - shell_exec
    - image_generate
    - video_generate
    - music_generate
    - diagram_maker
    - memory_search
    - web_search
    - code_review
    - deployment_automation
    - api_design
    - data_analysis
    - browser_automation
    - document_processing
    - seo_analysis
    - security_auditing
    - nlp_pipeline
    - mcp_bridge
    - cron_schedule
  marketplace:
    enabled: true
    skills_hub_url: "https://skillsmp.com"

cron:
  enabled: true
  location: "UTC"
  persistence:
    store: "memory"       # Cron state persisted to MeMex store
  triggers:
    - name: "daily-digest"
      schedule: "0 9 * * *"
      pipeline: "morning_briefing"
    - name: "heartbeat"
      schedule: "@every 30m"
      pipeline: "agent_heartbeat"
    - name: "weekly-cleanup"
      schedule: "0 2 * * 0"
      pipeline: "memory_compaction"

mcp:
  enabled: true
  servers:
    - name: "default"
      transport: "http"
      port: 9090
      toolFilter: "*"              # All tools exposed
      capability_scope: "full"
    - name: "external"
      transport: "http"
      port: 9091
      toolFilter:                  # Restricted subset
        - "memory_search"
        - "web_search"
        - "diagram_maker"
      capability_scope: "readonly"
    - name: "local-ide"
      transport: "stdio"
      toolFilter: "*"
      capability_scope: "full"

channels:
  telegram:
    enabled: false
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    mode: "polling"                # Long-polling, no webhook needed
    poll_interval: 2s
  discord:
    enabled: false
    bot_token: "${DISCORD_BOT_TOKEN}"
    mode: "gateway"                # Gateway WebSocket with shard awareness
    intents:                       # Gateway intents bitmask
      - "guild_messages"
      - "direct_messages"
    shard_count: 1
  reconnect:
    strategy: "exponential_backoff"
    initial: 1s
    max: 60s
    multiplier: 2.0
    jitter: true

learning:
  enabled: true
  pipeline:
    observer:
      sample_rate: 1.0             # Observe all agent sessions
      min_steps: 3                 # Minimum steps to qualify as a pattern
    extractor:
      cluster_algorithm: "jaccard"
      similarity_threshold: 0.7    # Jaccard threshold for clustering
      max_clusters: 100
    generator:
      confidence_threshold: 0.8    # Auto-publish above this score
      output_dir: "$HOME/.agentforge/skills/auto"
      trial_mode: true             # Generated skills start in trial mode
      max_auto_skills: 50          # Cap on auto-generated skills

workers:
  content_max_agents: 3
  seo_max_agents: 1
  social_max_agents: 2

cost_tracking:
  enabled: true
  alert_threshold_usd: 50.00
  alert_interval: "24h"
  export_csv: false
```

---

## Development

```bash
git clone https://github.com/agentforge/agentforge.git
cd agentforge
make deps        # go mod download
make build       # compile
make test        # run tests
make daemon      # start daemon
```

---

## Roadmap

- [x] **Phase 1:** Core daemon, agent goroutine pool, CSP bus, MeMex store, capability enforcement
- [x] **Phase 2:** Departments, pipeline DAG, LLM adapters (OpenAI/Anthropic/Ollama), subagent trees, MCP server, Docker
- [x] **Phase 3:** Web dashboard (SPA + htmx), fleet modal, DAG editor, circuit breaker, fallback chain, cost tracking
- [x] **Phase 4:** WASM plugin SDK, 19-tool registry, skills marketplace integration, session compaction (model-aware context windows)
- [x] **Phase 4.5:** Native cron job scheduler (cron-format + @every, pipeline triggers, cron_schedule tool), multi-MCP server (N configurable servers with HTTP/stdio transport and per-server toolFilter), channel adapters (Telegram bot integration via polling, Discord bot integration via Gateway WebSocket, bare WebSocket adapter with exponential backoff reconnect), self-learning engine (Observer→Extractor→Generator pipeline, Jaccard similarity clustering, auto-skill generation with confidence scoring)
- [ ] **Phase 5:** Launch (Show HN), community site, enterprise page, tutorials, SSO/RBAC, v1.0

---

## License

AgentForge is [BUSL-1.1](LICENSE) licensed — free for any use except production SaaS hosting. Converts to Apache 2.0 after 4 years.

---

**Built with Go. Secured by design. Deployed anywhere.**

[Website](https://agentforge.dev) · [Docs](https://docs.agentforge.dev) · [Discord](https://discord.gg/agentforge) · [Blog](https://agentforge.dev/blog)