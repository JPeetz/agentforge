# Changelog

All notable changes to AgentForge are documented here. This project adheres to
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) conventions with
**[Added]**, **[Changed]**, **[Fixed]**, and **[Removed]** sections.

---

## [0.3.0] — Unreleased

### Added

- **JWT+RBAC Authentication** — New `internal/auth/` package (auth.go, middleware.go, store.go, 870 lines total). HS256 JWT with access tokens (15min) + refresh tokens (7 days). RBAC with 3 roles (admin/operator/viewer) and 12 resources. HTTP middleware for AuthRequired, RequireRole, RequirePermission, CORS. In-memory user store seeded from config CapabilitySecret. API key generation (sha256-hashed, scoped per-agent). Dashboard endpoints: /api/auth/login (JWT pair), /api/auth/refresh, /api/auth/me, /api/auth/apikey. AuthConfig with AdminPassword, JWTExpiryMins, RefreshExpiryDays, AllowRegistration.
- **Real Cost Tracking** — New `internal/cost/cost.go` (877 lines). Per-model pricing table for 25+ models: OpenAI GPT-4 (all variants), Anthropic Claude, DeepSeek, Gemini, Ollama (free). Accumulator tracks: TotalCost, SessionCost, DailyCost, TokenCounts. Budget alerts at 80%/100% thresholds. Dashboard API: /api/cost/summary, /api/cost/daily, /api/cost/budget. CostConfig with BudgetLimit, AlertPercent, AlertEnabled.
- **MCP Client** — New `internal/mcpclient/` package (client.go 761 lines + tool.go 62 lines). HTTP+stdio transports. JSON-RPC 2.0 protocol. Auto-discover tools from connected MCP servers. MCPProxyTool wraps external tools as AgentForge tools (prefixed mcp_{server}_{tool}). Health checks with exponential backoff reconnect. Dashboard API for connect/disconnect. Wired into daemon — auto-connects and registers proxy tools.
- **SSE Streaming** — New `internal/sse/sse.go` (471 lines). SSEWriter with proper wire format (event/data/id/retry). KeepAlive ticker (15s). SSEHub with topic-based client management, Subscribe/Unsubscribe/Broadcast. ChatStreamRequest handler. Dashboard endpoint /api/chat/stream with real-time SSE output.
- **Slack + Signal Channels** — Slack adapter (508 lines) using Socket Mode with native RFC 6455 WebSocket. Signal adapter (466 lines) using signal-cli subprocess JSON-RPC. Both follow the existing Telegram/Discord adapter pattern with atomic counters, exponential backoff, and bus publishing.
- **Competitive Scrutiny Analysis** — `docs/SCRUTINY.md` (517 lines). Head-to-head comparison of 7 competitors: AgentForge, OpenClaw, Hermes, TinyClaw, BearClaw, PicoClaw, OpenHuman. 20 identified feature gaps with priorities. 22-row feature matrix across all competitors. Ranked #3 overall behind Hermes and OpenClaw.
- **Multi-MCP Server** — `MCPConfig` now supports a `servers: []` array with per-server fields: `name`, `port`, `transport` (one of `http` or `stdio`). The `stdio` transport spawns a command subprocess and communicates JSON-RPC over stdin/stdout, enabling integration with CLI-based MCP tools. Each `MCPServer` struct carries a `toolFilter` for per-server tool whitelisting. Server lifecycle is managed by the MCP Manager in `internal/api/mcp/manager.go` (start, stop, restart, health-check). The dashboard includes an **MCP Servers** page with status cards showing each server's name, port, transport, tool count, and health status. A default server on `:9090` is provisioned out of the box.
- **Channel Adapters** — New `internal/channel/` package with two initial adapters:
  - **Telegram** — Polling-based adapter using the `getUpdates` API with offset tracking, command parsing (`/`-prefixed messages), and reply routing.
  - **Discord** — WebSocket Gateway v10 adapter with heartbeat keep-alive, identify/ready handshake, and message-create event handling.
  - A **native WebSocket implementation** in `ws.go` (RFC 6455-compliant, zero external dependencies) powers the Discord connection. A **Channel Manager** starts and stops enabled adapters based on config. Inbound messages are published on bus topics `channel.telegram.message` and `channel.discord.message` for downstream agents and pipelines. The dashboard includes a **Channels** page with status cards per channel.
- **Self-Learning Engine** — New `internal/learn/learn.go` (817 lines) implementing an autonomous skill generation pipeline:
  - **Observer** watches bus events matching `channel.*`, `agent.*`, and `dashboard.action` topics, recording interaction traces with timestamps.
  - **Extractor** clusters traces using Jaccard similarity on 5-minute time windows and assigns confidence scores based on cluster size and repetition stability.
  - **Generator** creates `SKILL.md` files in `skills/auto/` when confidence exceeds `0.8` and at least 5 occurrences form a stable pattern.
  - **Manager** orchestrates the full `Observe → Extract → Generate` pipeline on a configurable interval, with stats tracking: `interactions_observed`, `patterns_extracted`, and `skills_generated`.
- **TUI** — Bubble Tea terminal interface for headless operation
- **WASM Plugin SDK** — Rust-based SDK for third-party tool plugins running in WASI sandboxes
- **Community Site** — Public website with skill marketplace, documentation, and tutorials
- **Wails Desktop App** — Native Windows/macOS/Linux GUI via Wails

---

## [0.2.2] — 2026-06-02

### Added

- **Session Management** — Full session lifecycle: create, track, compact, and archive per-agent conversation transcripts
- **Auto-Compaction** — Triggers at 90% of the model context window, summarizes older turns into a system-level summary
- **1M Token Context Window Support** — Recognizes GPT-4.1/GPT-4.5 1M-context models and adjusts threshold accordingly
- **Memory Flush to MeMex** — Pre-compaction durable writes persist full conversation detail to MeMex RAG before summarization
- **Pruning** — Tool output truncation at configurable character limit (`maxToolOutputChars`, default 8,000)
- **Session Persistence** — JSON-serialized transcripts saved to `~/.agentforge/sessions/` on compaction and shutdown
- **Edit Agent Modal** — Per-agent editing UI in the dashboard (Agent Profiles page)
- **Session Stats** — Track compaction count, total token usage, and turn count per agent session

### Changed

- Dashboard overview now shows live session token usage and cost estimates
- LLM adapter `ContextWindow()` informs session compaction thresholds dynamically

---

## [0.2.1] — 2026-06-02

### Added

- **Engine Wiring** — Full agent→LLM→tools→memory loop wired end-to-end; agents can now receive user input, invoke the LLM, call tools, and persist results
- **Config Persistence Layer** — `PersistedStore` with `Update()` supporting dot-notation YAML patching, file reload, and thread-safe reads
- **Dashboard Config Save** — Interactive settings pages (General, LLM, Providers, Memory, Security, Workers, Channels, Tools, UI) persist to `~/.agentforge/agentforge.yaml` via AJAX
- **Pipeline Save** — Pipeline definitions (name, description, trigger, stages) saved to config YAML through the Pipeline Editor
- **Agent Profile Save** — Agent fleet definitions persisted to config YAML through the Agent Profiles page

### Changed

- **Overview Redesign** — Compact 5-card stat layout (Uptime, Active Agents, Pipelines, Memory, Est. Cost) + 2-column detail panels (System Info, Pipeline Status, Token Usage & Cost)
- Settings API key fields masked in the UI (`MaskAPIKey` with `sk_***...abc` format)
- Dashboard cost-tracking panel now shows estimated session spend calculated from token usage × provider pricing

---

## [0.2.0] — 2026-06-02

### Added

- **Web Dashboard** — Embedded HTTP server serving a glassmorphism SPA via htmx partials
- **15 Dashboard Pages**: Overview, Agent Fleet, Memory Store, Pipelines, Skills, Security, Logs, Settings, Tools, Skills Marketplace, Pipeline Editor, Agent Profiles, Chat, Agent Fleet (modal), Skills Marketplace (modal)
- **Glassmorphism UI** — Translucent card design with `rgba(250,243,240,0.03)` backgrounds, `rgba(139,134,128,0.15)` borders, and smooth hover transitions
- **Interactive Settings** — 50+ toggles across 9 tabbed panels (General, LLM, Providers, Memory, Security, Workers, Channels, Tools, UI)
- **Provider Configuration** — Per-provider enable/disable toggles: OpenAI, Anthropic, OpenRouter, Google, DeepSeek, Ollama, Groq, Mistral, Cohere
- **Channel Configuration** — Telegram, Discord, Signal, WhatsApp, Email (SMTP), Slack with connection test buttons
- **Pipeline Editor** — Create, select, and save pipeline definitions with DAG stage configuration
- **Agent Fleet Editor** — Per-agent model, provider, department, tools, and capability toggles with create/edit modals
- **Skills Marketplace** — SkillsMP API key UI with save, search (keyword + AI/semantic), install flow, and mock fallback when no key configured
- **Skill Registry** — 5 built-in skill cards: code-review, memory-manager, security-audit, seo-auditor, browser-automation, data-analysis
- **Cost Tracking** — Token usage panel with estimated cost calculated from provider pricing
- **Chat Panel** — Basic chat interface with simulated agent responses
- **Security Posture** — Read-only audit view showing capability enforcement, sandbox mode, and per-scope access status
- **Log Viewer** — Last 6 log entries with timestamp, level, and message
- **Memory Browser** — MeMex RAG root display with search bar and file listing
- **Static Asset Embedding** — Go `embed.FS` for bundled icons and assets inside the 10MB binary
- **Runtime Stats** — Goroutine count, CPU cores, memory allocation in dashboard overview

---

## [0.1.2] — 2026-05-24

### Added

- **Skills System** — Full `SKILL.md` parser implementing the Anthropic standard with AgentForge extensions
- **SKILL.md Loader** — YAML frontmatter parser for `name`, `description`, `when_to_use`, `allowed-tools`, `context`, `model`, `effort`, `hooks`, `tags`, `capability-required`, `version`, `author`, and `license` fields
- **Auto-Activation** — Skills auto-register as tools in the tool registry; `allowed-tools` maps to capability enforcement
- **Skills Marketplace** — SkillsMP API integration for skill discovery and installation (`marketplace.go`)
- **3 Example Skills** —
  - `code-review` (v1.2.0): Automated code review with best-practice checks and security linting
  - `memory-manager` (v1.0.0): Semantic memory compression and long-term curation for MeMex RAG
  - `security-audit` (v1.1.3): Capability-based security posture audit with risk scoring
- **Skill Lifecycle** — WASM-sandboxable execution model, MeMex memory integration, manifest-based capability scoping

### Changed

- Tool registry now dynamically includes skills registered from SKILL.md manifests

---

## [0.1.1] — 2026-05-23

### Added

- **LLM Adapters** — Provider-agnostic `Adapter` interface with three implementations:
  - **OpenAI** (`OpenAIClient`): Full `/v1/chat/completions` HTTP client with auto model detection and context window reporting
  - **Ollama** (`OllamaClient`): Native `/api/chat` endpoint support for local models
  - **Anthropic** (`AnthropicClient`): Messages API with system prompt extraction, content block parsing, and `anthropic-version` header
- **Circuit Breaker** — Wraps any adapter with closed→open→half-open state machine; consecutive failures trip the breaker, timeout resets to half-open
- **Fallback Chain** — Ordered adapter chain: primary→secondary→tertiary with automatic failover on error
- **Tool Registry** — Thread-safe registry with 4 built-in tools (19 total tool definitions):
  - **Filesystem**: `read`, `write`, `list`, `delete`, `mkdir` within capability-scoped paths
  - **Shell**: Command execution with configurable timeout, working directory, stdout/stderr capture
  - **HTTP**: GET/POST/PUT/DELETE/PATCH with domain allowlists, header forwarding, 1MB response limit
  - **MCP Client**: JSON-RPC 2.0 over HTTP and stdio for connecting to external MCP servers
- **Tool→LLM Bridge** — `ToLLMToolDef()` converts tool metadata to OpenAI-compatible function definitions
- **MCP Server** — Full JSON-RPC 2.0 server (`server.go`) exposing AgentForge agents as tools via the Model Context Protocol
- **Subagent Trees** — `internal/engine/tree.go` for hierarchical agent delegation with capability scoping
- **Capability Enforcement per Tool** — Every tool invocation passes through `Enforcer.Check(ctx, cap, action)` before execution
- **Context Window Awareness** — Each adapter reports its model's max context window for session management

### Fixed

- Ollama adapter path corrected to use `/api/chat` (not `/v1/chat/completions`) for native performance
- Anthropic adapter correctly handles multi-block content responses and maps `stop_reason` to standard finish reasons
- HTTP tool respects capability-scoped domain allowlists via the enforcer

---

## [0.1.0] — 2026-05-21

### Added

- **Core Daemon** — CLI entry point via `cmd/agentforge/main.go` with subcommands: `run`, `daemon`, `version`, `spawn`
- **CSP Message Bus** — In-process publish/subscribe system (`internal/bus/bus.go`) with topic routing, request/reply, and broadcast patterns built on Go channels
- **Agent Engine** — Goroutine-per-agent lifecycle (`internal/engine/agent.go`), department worker pools, and DAG pipeline executor for multi-stage orchestration
- **Capability-Based Security** — HMAC-SHA256 signed permission tokens (`internal/security/capability.go`) with:
  - Resource allowlists (filesystem paths, network domains)
  - Token budgets (max tokens per agent session)
  - Timeout enforcement (max agent session duration)
  - Audit logging (`internal/security/audit.go`)
  - Runtime permission checks via `Enforcer`
  - No ambient authority — every agent granted only what its capability token allows
- **MeMex Zero RAG Memory** — Deterministic file-backed memory (`internal/memory/store.go`) with:
  - Markdown files as the canonical storage format
  - SQLite FTS5 full-text search index
  - Git versioning layer (`internal/memory/git.go`)
  - File watching for auto-index
  - Multi-instance sync (CRDT-based)
- **Configuration System** — YAML + ENV + CLI flag loading (`internal/config/config.go`) with sensible defaults
- **Docker Support** — 6MB static binary container, `docker-compose.yaml` for local development
- **CI/CD** — GitHub Actions workflow (`.github/workflows/ci.yaml`): build matrix (linux/macOS/windows), lint, tests, Docker image build
- **Documentation** — `README.md` (feature matrix vs OpenClaw, Hermes, OpenHuman), `ARCHITECTURE.md` (full system spec), `CONTRIBUTING.md`, `LICENSE` (BUSL-1.1 → Apache 2.0 after 4 years)
- **Project Scaffold** — Go module (`go.mod`), Makefile, `.gitignore`, `pkg/` public API package, `plugins/` WASM SDK directory

---

## Symbolic Links

- **[Unreleased]:** https://github.com/agentforge/agentforge/compare/v0.2.2...HEAD
- **[0.2.2]:** https://github.com/agentforge/agentforge/releases/tag/v0.2.2
- **[0.2.1]:** https://github.com/agentforge/agentforge/releases/tag/v0.2.1
- **[0.2.0]:** https://github.com/agentforge/agentforge/releases/tag/v0.2.0
- **[0.1.2]:** https://github.com/agentforge/agentforge/releases/tag/v0.1.2
- **[0.1.1]:** https://github.com/agentforge/agentforge/releases/tag/v0.1.1
- **[0.1.0]:** https://github.com/agentforge/agentforge/releases/tag/v0.1.0