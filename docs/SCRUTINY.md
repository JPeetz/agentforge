# AgentForge — Competitive Scrutiny Analysis
## Generated: 2026-06-02 | Analyst: Marvin, CEO AgentForge | Classification: BOARD CONFIDENTIAL

---

## PART 1: AGENTFORGE SYNOPSIS

### What It Is
AgentForge is a capability-secured, concurrent-native AI agent orchestration framework built in Go. Single 10MB static binary. 20 tools across 10 categories. 18 Go packages. Glassmorphism web dashboard with 15 htmx-powered pages. Session compaction with model-aware context windows. Self-learning engine (Observer→Extractor→Generator). Cron scheduler. Multi-MCP server support. Channel adapters for Telegram and Discord.

### Architecture
- **Runtime:** Go 1.24, goroutines as agents, channels as communication
- **Security:** HMAC-SHA256 capability tokens, per-agent allowlists (filesystem/network/shell/spawn), token budgets, timeout enforcement, audit logging
- **Memory:** MeMex Zero RAG — file-backed, git-versioned, SQLite FTS5 indexed
- **LLM:** Adapter pattern — OpenAI, Ollama, Anthropic with real HTTP + circuit breaker + fallback chain
- **Sessions:** Model-aware compaction (ContextWindow() from adapter), 90% threshold, memory flush to MeMex pre-compaction, pruning
- **Dashboard:** SPA with glassmorphism CSS, htmx partials, 15 pages including agent fleet editor modal, pipeline DAG editor, cost tracking
- **Channels:** Telegram (polling) + Discord (WebSocket Gateway v10), bare WebSocket implementation (RFC 6455)
- **MCP:** Multi-server config (name/port/transport/toolFilter), HTTP+stdio transports
- **Cron:** Native Go scheduler, cron-format + @every, pipeline trigger integration, cron_schedule tool
- **Self-Learning:** Observer watches bus events, Extractor uses Jaccard similarity clustering, Generator creates SKILL.md files when confidence > 0.8 and 5+ occurrences
- **Config:** 3-tier (YAML + ENV + CLI), 12 sections, 50+ settings, web-editable via dashboard
- **Deployment:** Static binary + Docker + docker-compose

### Binary Stats
- Binary: 10MB (static, Go 1.24)
- Runtime deps: None (single binary)
- Packages: 18
- Tools registered: 20
- Dashboard pages: 15
- Config settings: 50+
- Docs: 5 files, 2,264 total lines

---

## PART 2: COMPETITIVE LANDSCAPE — DETAILED COMPARISON

### 2.1 OpenClaw
**What it is:** Node.js-based, self-hosted AI agent gateway. Action-first agents for WhatsApp, Slack, Discord, Telegram, Signal. Large monorepo with plugin system. TaskFlow orchestration layer. WebSocket-based gateway server.

**Strengths:**
- TaskFlow: durable multi-step flows with state and revision tracking
- Hot config reload, session routing, auto-reply pipeline with buffering/deduplication
- Hooks system: before_tool_call, after_tool_call, tool_result_persist
- Streaming partial results (onUpdate callback) for long-running tools
- Provider manifest: multi-model routing, compaction model override
- Provision-rich memory with provenance labeling
- Plugin security scanner
- Secret redaction, scope-based access control
- Ed25519 device identity, timing-safe auth
- Large community, extensive docs, active development

**Weaknesses:**
- Node.js runtime — 200MB+ memory footprint
- Full host access for agents (no capability-based security model)
- No native self-learning / skill generation
- CVE-2026-25253 (CVSS 8.8)
- Electron desktop app (macOS only)
- No DAG pipeline editor
- No cost tracking built in
- Event loop concurrency model (not true parallel)

### 2.2 Hermes Agent (Nous Research)
**What it is:** Python-based self-improving agent framework. 64K+ GitHub stars. Closed learning loop (GEPA: Gather, Evaluate, Plan, Act). Skill auto-creation from complex interactions. 118 skills across 26 categories. Multi-platform gateway.

**Strengths:**
- Industry-leading self-learning: auto-generates skills after 5+ tool calls, patches during use
- 118 bundled skills, 687 community skills via Skills Hub
- 40+ built-in tools
- 8 pluggable memory providers
- FTS5 session search with LLM summarization
- Honcho dialectic user modeling (personality profiling)
- Built-in cron scheduler with natural language
- 200+ model support via OpenRouter, Nous Portal
- Multi-platform: Telegram, Discord, Slack, WhatsApp, Signal, Email, Matrix, Mattermost
- Six terminal backends (local, Docker, SSH, Singularity, Modal, Daytona)
- Serverless persistence (Modal, Daytona)
- TUI with multiline editing, slash-command autocomplete, streaming tool output
- Voice memo transcription
- Atropos RL training system for model fine-tuning
- Research-ready batch trajectory generation
- Cross-platform conversation continuity
- Mass migration from OpenClaw (hermes claw migrate)

**Weaknesses:**
- Python runtime — 20+ pip packages, venv management
- No capability-based security model (no token budgets, HMAC enforcement)
- 100MB+ RAM at idle
- No native Go binary (Python venv/pip install)
- No DAG pipeline editor
- Skills are Markdown-based — no tool-level capability enforcement

### 2.3 TinyClaw
**What it is:** Bash + TypeScript personal AI companion. ~20K LOC. File-based message queue. Delegates tool execution to Claude/Codex CLI. Discord-like web UI.

**Strengths:**
- Self-configuring: no manual config files needed
- Heartware personality engine (SOUL.md + IDENTITY.md)
- 4-layer context compaction (rule-based pre-compression, shingle deduplication, LLM summarization, tiered summaries)
- Smart routing: 8-dimension query classifier, tiers cheap vs expensive models
- SHIELD.md anti-malware enforcement
- 5-layer security (path sandbox, content validation, audit log, auto-backup, rate limit)
- Discord-like web experience with real-time SSE streaming
- 3-layer adaptive memory (episodic, semantic FTS5, temporal decay)
- Self-improving behavioral pattern detection
- Delegation system with self-improving role templates and blackboard collaboration
- Inter-agent pub/sub event bus
- SQLite persistence, Ollama Cloud built-in
- Bun-native, single binary

**Weaknesses:**
- Under active development, not yet released
- No tool registry — delegates all execution to CLI (zero visibility into agent actions)
- Bash dependency limits cross-platform portability
- ~50MB+ RAM (Node.js + tmux)
- No capability security model
- 3 channels only (CLI, Telegram, HTTP)
- No cron scheduler
- No MCP support

### 2.4 PicoClaw
**What it is:** Go-based complete agent on minimal hardware. ~4,600 lines. Single binary, <10MB RAM. 2 runtime dependencies.

**Strengths:**
- Best architecture of group: parallel tool execution, defense-in-depth security
- Dual-channel tool results (ForLLM/ForUser split)
- Subagent delegation
- Full agentic loop
- Extremely lean (<10MB RAM)
- Go binary (single file)

**Weaknesses:**
- ~4,600 lines total — feature scope is tiny
- No dashboard / web UI
- Security has critical holes (symlink bypass, bypassable regex deny-list, no SSRF)
- No channel adapters (3: CLI, Telegram, HTTP)
- No cron scheduler
- No session compaction
- No self-learning
- No MCP
- No DAG pipeline
- No cost tracking

### 2.5 OpenHuman
**What it is:** Rust (Tauri) desktop AI agent. SQLite memory. OAuth sprawl security model.

**Strengths:**
- Native desktop app (Tauri)
- Full offline capability
- SQLite memory

**Weaknesses:**
- Desktop-only (no server/cloud deployment)
- OAuth-based security (not capability tokens)
- No multi-agent orchestration
- No channel adapters
- No web dashboard
- No cron
- No self-learning
- No DAG pipeline
- No session compaction
- Limited tool set

### 2.6 ClaudeClaw (Deprecated)
**What it is:** Python-based multi-agent system. Best multi-agent team collaboration. Full ReAct loop with parallel execution.

**Strengths:**
- Best team collaboration patterns (fan-out, sequential handoffs)
- Full ReAct loop with Promise.all parallel execution
- Hook pipeline: before-hooks (PolicyEngine), JSON Schema validation, after-hooks
- ToolContext DI container (AbortSignal, SecurityPolicy, PolicyEngine, rate limiter, tool registry, provider factory)
- ForLLM/ForUser tool result split
- Both team collaboration and subagent spawning

**Weaknesses:**
- Deprecated/superseded by BearClaw/OpenClaw
- Python runtime
- Complex dependency tree
- Not actively maintained

---

## PART 3: GAP ANALYSIS — WHAT AGENTFORGE MUST BUILD TO WIN

### CRITICAL (Ship within 2 weeks)

#### GAP-1: Real Authentication System
**Status:** Dashboard accepts "any string" as login token. No JWT, no session management, no RBAC.
**Competitors:** OpenClaw has Ed25519 device identity + timing-safe auth. Hermes has platform-level auth per channel. TinyClaw has pairing auth with CSPRNG codes. ZeroClaw has CSPRNG codes + 5-attempt lockout.
**What to build:**
- JWT-based authentication with refresh tokens
- Role-based access control (admin, operator, viewer)
- Per-agent API key generation
- Session token rotation
- Rate limiting per-user
- Audit log of all auth events

#### GAP-2: Platform Channel Adapters (Beyond Telegram/Discord)
**Status:** Only Telegram (polling) and Discord (Gateway WS) implemented. Config has stubs for Signal, WhatsApp, Email, Slack but zero implementations.
**Competitors:** Hermes has 7+ channels (Telegram, Discord, Slack, WhatsApp, Signal, Email, Matrix, Mattermost) all through a single gateway. OpenClaw has WhatsApp, Slack, Discord, Telegram, Signal. TinyClaw has 3 (CLI, Telegram, HTTP).
**What to build:**
- Slack adapter (Events API + WebSocket)
- Signal adapter (signal-cli integration)
- WhatsApp adapter (Baileys or WhatsApp Business API)
- Matrix adapter (Synapse federation)
- Unified gateway: single process for all channels (like Hermes)
- Cross-platform conversation continuity (start on Telegram, continue on Discord)

#### GAP-3: Streaming Tool Output / SSE
**Status:** Config has streaming fields but they're not wired. Chat page exists but doesn't stream.
**Competitors:** OpenClaw has onUpdate streaming callbacks. TinyClaw has SSE streaming with typing indicators. Hermes has streaming tool output in TUI.
**What to build:**
- Server-Sent Events (SSE) endpoint for agent responses
- Streaming LLM output via adapter (OpenAI streaming already has code, just not exposed)
- Streaming tool progress updates
- Typing indicators in chat UI
- Partial result display for long-running tools (web_search, web_fetch)

#### GAP-4: Real Cost Tracking (Not Mock Data)
**Status:** Overview shows "$0.03" hardcoded. Token counts are fake (127,450 / 21,890).
**Competitors:** TinyClaw has smart routing that tiers queries to cut costs. No competitor has proper per-agent per-model cost breakdowns.
**What to build:**
- Usage struct from LLM adapter already tracks tokens — expose to cost tracker
- Per-model pricing table (OpenAI, Anthropic, Ollama pricing per 1M tokens)
- Session cost accumulator, daily/weekly/monthly rollups
- Per-agent cost breakdown
- Budget alerts (warn at 80%, block at 100%)
- Cost optimization suggestions (model routing)

#### GAP-5: Real MCP Client (Not Just Server)
**Status:** Multi-MCP server support done. But AgentForge agents cannot *use* external MCP servers — can only *be* one.
**Competitors:** Hermes supports MCP out of the box — agents can connect to external tool servers. OpenClaw has MCP client support.
**What to build:**
- MCP client that connects to external MCP servers
- Auto-discovery of tools from connected MCP servers
- Per-agent MCP server allowlists
- Dashboard page to manage connected MCP servers
- Health check / reconnect for MCP connections

### HIGH (Ship within 4 weeks)

#### GAP-6: ForLLM/ForUser Tool Result Split
**Status:** Tool results return a single string. No separation between what the LLM sees and what the user sees.
**Competitors:** PicoClaw and BearClaw independently arrived at this pattern. It's the #1 pattern worth stealing according to the competitive analysis.
**What to build:**
- Structured ToolResult with ForLLM and ForUser fields
- Silent flag (tool executes but doesn't show in chat)
- Async flag (fire-and-forget tool execution)
- onUpdate callback for streaming progress

#### GAP-7: Parallel Tool Execution
**Status:** Tools execute sequentially. When LLM calls 3 tools, they run one at a time.
**Competitors:** BearClaw executes tools via Promise.all. PicoClaw has parallel execution via goroutines. Both append results in request order.
**What to build:**
- Goroutine-based parallel tool execution
- Configurable max parallelism per agent
- Result ordering preserved (append in request order, not completion order)
- Timeout per parallel batch

#### GAP-8: TUI (Terminal UI)
**Status:** Empty `internal/tui/` directory. No terminal interface exists.
**Competitors:** Hermes has the best TUI: multiline editing, slash-command autocomplete, conversation history, interrupt-and-redirect, streaming tool output. OpenClaw has Control UI.
**What to build:**
- Bubble Tea TUI with full feature parity to Hermes
- Slash-command autocomplete (/spawn, /model, /status, /compact)
- Conversation history with search
- Streaming tool output in terminal
- Interrupt-and-redirect (send new message while agent is thinking)
- Model switching at runtime (/model openai/gpt-4o)

#### GAP-9: Hook System
**Status:** No hooks exist in tool execution pipeline.
**Competitors:** OpenClaw has before_tool_call, after_tool_call, tool_result_persist. BearClaw has before-hook pipeline with PolicyEngine + JSON Schema validation + after-hooks.
**What to build:**
- before_tool_execution hook (can block or modify args)
- after_tool_execution hook (fire-and-forget or result mutator)
- tool_result_persist hook (can redact/censor before saving to memory)
- Plugin system for custom hooks
- Hook ordering (priority-based execution)

#### GAP-10: Tests
**Status:** 1 test file. No test suite for LLM adapters, session management, cron, channels, config, engine, tools.
**Competitors:** ZeroClaw has 1,017 tests. AgentForge needs at minimum: table-driven engine tests, LLM adapter mock tests, session compaction unit tests, config round-trip tests, tool execution tests.
**What to build:**
- Engine: agent lifecycle, pipeline DAG execution, department spawn/limit
- Session: compaction trigger, memory flush, pruning, ContextWindow detection
- Config: YAML round-trip, ENV override, JSON serialization with secrets masked
- LLM: mock HTTP server tests for OpenAI/Ollama/Anthropic, circuit breaker, fallback chain
- Tools: execution with capability enforcement, parameter validation
- Cron: schedule parsing, next-run calculation, fire loop
- Channels: mock Telegram/Discord servers for adapter tests
- Integration: agent receives message → LLM → tool → memory end-to-end

### MEDIUM (Ship within 8 weeks)

#### GAP-11: WASM Plugin Sandbox
**Status:** Architecture spec calls for it. No implementation.
**Competitors:** ZeroClaw has filesystem sandboxing. OpenClaw has plugin security scanner.
**What to build:**
- WASM runtime (wazero) for third-party tool execution
- Capability declaration per plugin (filesystem, network, compute)
- Content-addressed plugin loading
- Plugin marketplace / registry integration

#### GAP-12: Desktop Application
**Status:** Not started.
**Competitors:** OpenClaw has Electron (macOS). OpenHuman has Tauri desktop.
**What to build:**
- Wails-based native desktop app
- System tray integration
- Local LLM integration (Ollama)
- Offline-first architecture

#### GAP-13: Helm Chart / K8s Operator
**Status:** No K8s deployment manifests.
**What to build:**
- Helm chart with configurable replicas, ingress, PVC for memory
- K8s operator for AgentForge CRDs (Agent, Pipeline, Department)
- Auto-scaling based on queue depth

#### GAP-14: agentctl Admin CLI
**Status:** No admin CLI exists.
**What to build:**
- agentctl agents list|spawn|kill|logs
- agentctl pipelines trigger|status|history
- agentctl config get|set|validate
- agentctl sessions list|compact|prune
- agentctl mcp list|add|remove
- agentctl cron list|add|remove|trigger

#### GAP-15: Model Router / Smart Routing
**Status:** Hardcoded to single provider. No tiered routing.
**Competitors:** TinyClaw has 8-dimension query classifier. OpenClaw has provider manifest with multi-model routing.
**What to build:**
- Query complexity classifier
- Tiered model routing (simple→cheap, complex→powerful)
- Per-task model override
- Cost-aware routing (budget-constrained mode)
- Fallback chain with model-aware escalation

#### GAP-16: Voice / Audio Support
**Status:** Not implemented.
**Competitors:** Hermes has voice memo transcription. OpenClaw has TTS support.
**What to build:**
- Voice memo transcription (Whisper API or local)
- Text-to-speech for agent responses
- Audio file handling in channel adapters

#### GAP-17: Plugin / Extension Marketplace
**Status:** Skills marketplace exists (SkillsMP API key UI). No provider/tool/channel marketplace.
**Competitors:** Hermes has Skills Hub (687 skills, 18 categories). TinyClaw has plugin architecture.
**What to build:**
- Provider plugin registry
- Tool plugin registry
- Channel plugin registry
- One-click install from dashboard
- Plugin versioning and compatibility checking

#### GAP-18: gRPC API
**Status:** Config has gRPC port field. No implementation.
**What to build:**
- gRPC service definitions (AgentService, PipelineService, ConfigService)
- Streaming RPC for agent output
- Client libraries (Go, Python, TypeScript)
- gRPC gateway for REST compatibility

#### GAP-19: Observability / Monitoring
**Status:** No metrics, no tracing, no alerting.
**What to build:**
- Prometheus metrics endpoint
- OpenTelemetry tracing
- Agent-level metrics (tokens used, tools called, latency, error rate)
- Pipeline metrics (success rate, stage duration, bottleneck detection)
- Alerting rules (agent failure, pipeline stall, budget exceeded)
- Grafana dashboard template

#### GAP-20: AgentForge Go SDK (pkg/)
**Status:** Directory doesn't exist.
**What to build:**
- Embeddable Go SDK: `import "github.com/JPeetz/agentforge/pkg/agentforge"`
- AgentForge as a library in other Go applications
- Programmatic agent creation and management
- Custom tool registration API
- Memory store integration

---

## PART 4: FEATURE MATRIX — ALL COMPETITORS

| Capability | AF | OpenClaw | Hermes | TinyClaw | PicoClaw | OpenHuman | BearClaw |
|---|---|---|---|---|---|---|---|
| **Security** | | | | | | | |
| Capability tokens | ✅ | ❌ | ❌ | ❌ | ⚠️ | ❌ | ⚠️ |
| HMAC enforcement | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Token budgets | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| SSRF protection | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| Secret encryption | ❌ | ✅ | ❌ | ⚠️ | ❌ | ❌ | ✅ |
| **Runtime** | | | | | | | |
| Language | Go | TypeScript | Python | Bash+TS | Go | Rust | TypeScript |
| Binary size | 10MB | 200MB+ | 100MB+ | 50MB+ | <10MB | Tauri | Node.js |
| True parallelism | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ |
| Static binary | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ |
| **LLM** | | | | | | | |
| OpenAI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Ollama | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Anthropic | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| Circuit breaker | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| Fallback chain | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| Model-aware ctx window | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Smart routing | ❌ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| **Tools** | | | | | | | |
| Tool count | 20 | 15+ | 40+ | 0 (CLI) | 10+ | 5+ | 12+ |
| Parallel execution | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ |
| ForLLM/ForUser split | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ |
| Streaming progress | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Pre-execution hooks | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| JSON Schema validation | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| **Sessions** | | | | | | | |
| Auto-compaction | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Model-aware threshold | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Memory flush pre-compact | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Pruning | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Dashboard** | | | | | | | |
| Web dashboard | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ |
| SPA with live data | ✅ | ⚠️ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Pipeline editor | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Agent fleet editor | ✅ | ❌ | ⚠️ | ❌ | ❌ | ❌ | ❌ |
| Cost tracking | ⚠️ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Settings editor | ✅ | ✅ | ✅ | ⚠️ | ❌ | ❌ | ❌ |
| **Channels** | | | | | | | |
| Telegram | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| Discord | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| Slack | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| WhatsApp | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Signal | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Email | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Matrix | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Unified gateway | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Self-Learning** | | | | | | | |
| Pattern extraction | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Auto-skill generation | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Skill self-improvement | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| User modeling | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| RL training | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| **Infrastructure** | | | | | | | |
| Cron scheduler | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| MCP server | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| MCP client | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Multi-MCP | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Docker | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
| K8s/Helm | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| TUI | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Desktop app | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ |
| gRPC API | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Go SDK | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Tests | ⚠️ (1) | ✅ | ✅ | ⚠️ | ✅ | ✅ | ✅ |
| **Auth** | | | | | | | |
| JWT/RBAC | ❌ | ✅ | ⚠️ | ✅ | ✅ | ❌ | ✅ |
| API keys | ❌ | ✅ | ⚠️ | ❌ | ❌ | ❌ | ✅ |
| Pairing auth | ❌ | ✅ | ❌ | ✅ | ✅ | ❌ | ✅ |
| **Ops** | | | | | | | |
| Metrics/Prometheus | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| OpenTelemetry | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Audit logging | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ |
| Rate limiting | ❌ | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ |

**Legend:** ✅ Complete | ⚠️ Partial/Mock | ❌ Missing

---

## PART 5: AGENTFORGE'S UNIQUE ADVANTAGES (Moats)

1. **Capability-Based Security** — No other framework has HMAC-signed permission tokens with token budgets, timeout enforcement, filesystem/network/shell/spawn allowlists. This is the only framework a CISO would approve for production.

2. **Go Runtime** — Single 10MB static binary. True parallelism via goroutines. No Node.js event loop. No Python GIL. No venv management. Runs on a Raspberry Pi. Deploy with `scp`.

3. **Model-Aware Compaction** — ContextWindow() queries the actual LLM adapter. GPT-4.1 gets 1M context (compact at 900K). Claude gets 200K. No other framework does this — they all use hardcoded limits.

4. **Pipeline DAG Editor** — Visual pipeline orchestration in the dashboard. No competitor has this. Hermes has cron but no visual pipeline builder.

5. **Self-Learning + Capability Security** — The only framework that combines autonomous skill generation with security enforcement. Hermes has better self-learning but no security model.

6. **MeMex Zero RAG** — File-backed, git-versioned, SQLite FTS5, deterministic. No vector soup. Grep-able. Production-proven architecture pattern.

---

## PART 6: PRIORITIZED BUILD ROADMAP

### Phase 1: Critical (Week 1-2) — "Ship the Foundation"
1. GAP-1: Real Auth (JWT + RBAC)
2. GAP-4: Real Cost Tracking (not mock data)
3. GAP-5: MCP Client (connect to external servers)
4. GAP-3: SSE Streaming (agent output, tool progress)
5. GAP-2: Slack + Signal channel adapters
6. GAP-10: Tests (engine, session, config, LLM, tools)

### Phase 2: High (Week 3-4) — "Close the UX Gap"
7. GAP-6: ForLLM/ForUser Tool Results
8. GAP-7: Parallel Tool Execution
9. GAP-8: TUI (Bubble Tea)
10. GAP-9: Hook System
11. GAP-2: WhatsApp + Matrix channels + unified gateway

### Phase 3: Medium (Week 5-8) — "Enterprise Ready"
12. GAP-11: WASM Plugin Sandbox
13. GAP-13: Helm Chart / K8s Operator
14. GAP-14: agentctl Admin CLI
15. GAP-15: Smart Model Router
16. GAP-18: gRPC API
17. GAP-19: Observability (Prometheus + Grafana)
18. GAP-20: Go SDK (pkg/)

### Phase 4: Ecosystem (Week 9-12) — "Developer Adoption"
19. GAP-12: Desktop App (Wails)
20. GAP-16: Voice/Audio Support
21. GAP-17: Plugin Marketplace
22. Community site, tutorials, Show HN launch
