# AgentForge — Architecture Specification v2.1

**Classification:** PUBLIC (open source)  
**Date:** 2026-06-02  
**Author:** AgentForge CEO (Marvin)  
**Status:** Active Development  
**Built Packages:** 16 core packages, 19 built-in tools, Multi-MCP server, Web dashboard, SkillsMP marketplace

---

## 1. System Overview

AgentForge is a Go-based, capability-secured, concurrent-native agent orchestration framework. It runs as a single static binary — CLI+daemon mode via `cmd/agentforge` using Cobra+Viper.

### 1.1 Core Design Principles

1. **Security First** — HMAC capability tokens at agent spawn. No ambient authority. No "full host access by default."
2. **Concurrency Native** — Goroutines = agents. Channels = communication. CSP = orchestration. Not bolted on — the language is the runtime.
3. **Memory as Files** — Markdown files + git + SQLite FTS5 index. Deterministic. Grep-able. Git-trackable. No vector soup.
4. **Single Binary** — Go static compilation. No runtime deps. Cross-compiles to everything from Raspberry Pi to Kubernetes.
5. **Offline-First** — Full functionality without internet. Local LLMs via Ollama. Syncs when connected.

### 1.2 Deployment Targets

| Target | Description | Priority |
|--------|-------------|----------|
| **Docker** | Single container, SQLite persistence | Week 1 |
| **Linux daemon** | systemd service, headless server | Week 1 |
| **macOS binary** | Homebrew formula, launchd | Week 2 |
| **Windows binary** | Windows Service, MSI installer | Week 2 |
| **TUI** | Bubble Tea terminal interface | Week 2 |
| **Web Dashboard** | htmx SPA, glassmorphism UI, 97+ icons, 15 partials | Week 3 |
| **Wails Desktop** | Native app (Windows/macOS/Linux) | Month 2 |

---

## 2. Package Map

```
agentforge/
├── cmd/
│   └── agentforge/              # CLI + daemon entry point (cobra, viper)
├── internal/
│   ├── bus/                     # CSP message bus
│   │   └── bus.go               # Pub/sub, request/reply, broadcast
│   ├── config/                  # 3-tier configuration
│   │   └── config.go            # YAML + ENV + CLI, 12 sections, 50+ settings
│   │                           # Persisted store with YAML node patching
│   ├── engine/                  # Core agent runtime
│   │   ├── agent.go             # Agent goroutine pool
│   │   ├── department.go        # Department worker pools
│   │   ├── pipeline.go          # DAG pipeline executor
│   │   └── tree.go              # Subagent tree manager
│   ├── memory/                  # MeMex Zero RAG memory store
│   │   ├── store.go             # Put/Get/Search/Append (git + SQLite + FTS5)
│   │   ├── index.go             # FTS5 full-text search index
│   │   └── watcher.go           # FileWatcher for change detection
│   ├── security/                # Capability-based security
│   │   ├── capability.go        # HMAC capability tokens (token budgets, timeouts)
│   │   │                       # Filesystem/network/shell/spawn permissions
│   │   └── enforcer.go          # Runtime permission enforcement
│   ├── llm/                     # LLM adapter layer
│   │   ├── adapter.go           # Provider interface + ContextWindow() + Chat + HealthCheck
│   │   ├── openai.go            # OpenAIClient (real HTTP)
│   │   ├── ollama.go            # OllamaClient (real HTTP)
│   │   ├── anthropic.go         # AnthropicClient (real HTTP)
│   │   ├── breaker.go           # Circuit breaker
│   │   └── fallback.go          # FallbackChain with retry
│   ├── tool/                    # Tool system
│   │   ├── registry.go          # Tool registry (Meta/Execute interface)
│   │   └── (19 builtins)        # filesystem, network, memory, vcs, agent,
│   │                           # automation, shell, media, sandbox, browser, mcp
│   ├── api/
│   │   └── mcp/                 # Multi-MCP server
│   │       ├── server.go        # JSON-RPC 2.0, HTTP + stdio, tool exposure
│   │       └── manager.go       # Multi-MCP Manager (N servers, dual transport)
│   ├── session/                 # Session management
│   │   └── session.go           # Session lifecycle, transcript persistence (JSON)
│   │                           # Auto-compaction, memory flush, ContextWindow detection
│   ├── skill/                   # SKILL.md system (Anthropic spec)
│   │   ├── loader.go            # SKILL.md parser + repository
│   │   ├── scorer.go            # Auto-activation scoring engine
│   │   └── marketplace.go       # SkillsMP API client + GitHub discovery
│   ├── dashboard/               # Web dashboard
│   │   ├── server.go            # Embedded HTTP server
│   │   ├── overview.go          # Stat cards, pipeline status, cost tracking, events
│   │   └── static/              # CSS (glassmorphism), HTML (SPA), 97+ icons
│   │       └── (15 page partials via htmx)
│   ├── cron/                    # Native cron scheduler
│   │   └── scheduler.go         # Cron-format + @every parser, Job lifecycle,
│   │                            # time.Ticker loop, pipeline trigger integration
│   ├── channel/                 # Channel adapters (Telegram, Discord)
│   │   ├── adapter.go           # Adapter interface (Name/Start/Stop/Status)
│   │   ├── manager.go           # Manager starts/stops enabled adapters
│   │   ├── telegram.go          # Telegram polling (getUpdates API)
│   │   ├── discord.go           # Discord Gateway v10 (IDENTIFY, heartbeat)
│   │   └── ws.go                # Native WebSocket (RFC 6455, no external deps)
│   └── learn/                   # Self-learning engine
│       ├── observer.go          # Watches bus events, logs Interactions
│       ├── extractor.go         # Jaccard similarity, time-window clustering
│       ├── generator.go         # SKILL.md file generator
│       └── manager.go           # Observe→Extract→Generate pipeline orchestrator
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 3. Package Diagram (ASCII)

```
┌─────────────────────────────────────────────────────────────────────┐
│                         c m d / a g e n t f o r g e                 │
│                    CLI + Daemon  (cobra, viper)                      │
└────────┬─────────────┬──────────────┬──────────────┬────────────────┘
         │             │              │              │
         ▼             ▼              ▼              ▼
┌─────────────┐ ┌───────────┐ ┌───────────┐ ┌───────────────┐
│  dashboard  │ │   mcp     │ │  session  │ │    skill      │
│  (web UI)   │ │ (JSON-RPC)│ │ (compact) │ │ (SKILL.md)    │
└──────┬──────┘ └─────┬─────┘ └─────┬─────┘ └───────┬───────┘
       │              │             │               │
       └──────────────┼─────────────┼───────────────┘
                      │             │
              ┌───────▼─────┐       │
              │    bus      │◄──────┘
              │  (CSP pub/  │
              │   sub + req/│
              │   reply)    │
              └──┬───┬───┬──┘
                 │   │   │
       ┌─────────┼───┼───┼─────────────────┐
       │         │   │   │                 │
       ▼         ▼   │   │                 ▼
┌───────────┐ ┌──────┴──────┐     ┌───────────────┐
│  engine   │ │  channel    │     │    learn      │
│ ┌───────┐ │ │┌──────────┐ │     │ ┌───────────┐ │
│ │ agent  │ │ ││Telegram  │ │     │ │ observer  │ │
│ │ pool   │ │ ││(polling) │ │     │ ├───────────┤ │
│ ├───────┤ │ │├──────────┤ │     │ │ extractor │ │
│ │ dept   │ │ ││Discord   │ │     │ │(Jaccard)  │ │
│ │ pools  │ │ ││(Gateway) │ │     │ ├───────────┤ │
│ ├───────┤ │ │├──────────┤ │     │ │ generator │ │
│ │ DAG    │ │ ││Native WS │ │     │ │(SKILL.md) │ │
│ │pipeline│ │ ││(RFC6455) │ │     │ └─────┬─────┘ │
│ ├───────┤ │ │├──────────┤ │     │       │       │
│ │tree mgr│ │ ││ manager  │ │     │  ┌────▼────┐  │
│ └───────┘ │ │└──────────┘ │     │  │ memory  │  │
└─────┬─────┘ └─────────────┘     │  │(SKILL.md│  │
      │                            │  │ files)  │  │
      │  ┌───────────┐             │  └─────────┘  │
      │  │   cron    │             └───────────────┘
      │  │┌─────────┐│
      │  ││Scheduler││      ┌───────────┐
      │  ││Add/Remove││     │  memory   │
      │  ││Trigger  ││     │ ┌───────┐ │
      │  ││List     ││     │ │ store │ │
      │  │└────┬────┘│     │ │ (git+ │ │
      │  │     │     │     │ │ SQLite│ │
      │  │pipeline  │     │ │+FTS5) │ │
      │  │ triggers │     │ ├───────┤ │
      │  └──────────┘     │ │ index │ │
      │                    │ ├───────┤ │
      │                    │ │watch  │ │
      │                    │ └───────┘ │
      │                    └───────────┘
      │
      ▼
┌───────────┐     ┌───────────┐
│    llm    │     │   tool    │
│ ┌───────┐ │     │ ┌───────┐ │
│ │OpenAI │ │     │ │ 19    │ │
│ │Ollama │ │     │ │builtin│ │
│ │Anthrop│ │     │ │tools  │ │
│ ├───────┤ │     │ └───────┘ │
│ │breaker│ │     └───────────┘
│ ├───────┤ │
│ │fallbk │ │
│ └───────┘ │
└───────────┘
         ▲
         │
┌────────┴──────────┬───────────┬───────────┬────────────────────────┐
│                    │           │           │                        │
│  ┌────────────────▼─────┐ ┌───▼────────┐ ┌▼──────────────┐        │
│  │      config          │ │  channel   │ │    learn      │        │
│  │  YAML + ENV + CLI    │ │  config    │ │    config     │        │
│  │  12 → 15 sections    │ │ (adapters) │ │(similarity,   │        │
│  │  Persisted YAML      │ │            │ │ confidence,   │        │
│  │  node patching       │ │            │ │ windows)      │        │
│  └──────────────────────┘ └────────────┘ └───────────────┘        │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
                        config (3-tier)
```

---

## 4. Data Flow

### 4.1 Primary Request Path

```
HTTP Request (dashboard / MCP / API)
         │
         ▼
┌─────────────────┐
│   Dashboard /    │  htmx partials, JSON-RPC 2.0 over HTTP/stdio
│   MCP Server     │
└────────┬────────┘
         │ Publish(Envelope)
         ▼
┌─────────────────┐
│    CSP Bus       │  Topic routing, pub/sub dispatch
│                  │  Topics: agent.{id}.inbox, dept.{name}.broadcast
└────────┬────────┘
         │ Delivered to agent inbox channel
         ▼
┌─────────────────┐
│    Engine        │  Agent goroutine picks up envelope
│  ┌────────────┐  │
│  │ Agent Loop │  │  handleMessage(envelope):
│  └─────┬──────┘  │    1. security.Enforcer.Check(cap, action)
│        │         │    2. skill.Repository.AutoActivate(context)
│        │         │    3. Resolve which tool to invoke
│        ▼         │
│  ┌────────────┐  │
│  │ LLM Call   │──┼──► llm.Adapter.Chat(messages)
│  │ (if needed)│  │    ├─ circuit breaker check
│  └─────┬──────┘  │    ├─ ContextWindow() → model limits
│        │         │    └─ fallback chain on failure
│        ▼         │
│  ┌────────────┐  │
│  │Tool Exec   │──┼──► tool.Registry.Execute(name, args)
│  └─────┬──────┘  │    ├─ security.Enforcer.Check per invocation
│        │         │    ├─ Memory tools → memory.Store (MeMex)
│        │         │    ├─ Filesystem tools → OS
│        │         │    ├─ Network tools → HTTP
│        │         │    └─ Agent tools → bus.Publish (spawn/send)
│        ▼         │
│  ┌────────────┐  │
│  │ Result →   │──┼──► bus.Publish(response envelope)
│  │ Bus        │  │
│  └────────────┘  │
└─────────────────┘
         │
         ▼
┌─────────────────┐
│    Session       │  Transcript logging (JSON persistence)
│                  │  Token counting against ContextWindow()
└─────────────────┘
```

### 4.2 Complete Data Flow Chain

```
HTTP → dashboard → bus → engine → LLM adapters → tool registry → memory store
                                                      │
                                                      ├── filesystem tools
                                                      ├── network tools
                                                      ├── memory tools → MeMex Zero RAG
                                                      ├── agent tools → bus (spawn/send)
                                                      └── MCP tools → external MCP servers

Channel → bus → learn pipeline:
  Telegram/Discord  →  channel.Adapter.Recv()  →  bus.Publish(channel.{name}.message)
                                                         │
                                                         ▼
                                                  learn.Observer
                                                       │
                                            ┌──────────▼──────────┐
                                            │  InteractionLogger   │
                                            │  (topic, payload,    │
                                            │   timestamp, agent)  │
                                            └──────────┬──────────┘
                                                       │
                                                       ▼
                                                  learn.Extractor
                                                       │
                                            ┌──────────▼──────────┐
                                            │ Jaccard word-overlap │
                                            │ similarity clustering │
                                            │ 5-min time windows    │
                                            └──────────┬──────────┘
                                                       │
                                                       ▼
                                                  learn.Generator
                                                       │
                                            ┌──────────▼──────────┐
                                            │ confidence > 0.8      │
                                            │ count ≥ 5            │
                                            │ → memory/SKILL.md    │
                                            └──────────────────────┘
```

### 4.3 Cron-Driven Pipeline Flow

```
cron.Scheduler (time.Ticker loop)
         │
         │ Due job detected
         ▼
┌─────────────────┐
│  Job.Trigger()  │  Evaluate schedule, update LastRun/NextRun
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  bus.Publish()  │  Publish pipeline trigger envelope
│  topic:         │  e.g., pipeline.{name}.trigger
│  cron.{job}.    │
│  trigger        │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  engine.Pipeline│  DAG executor picks up trigger
│  .Execute()     │  Runs pipeline stages
└─────────────────┘
```

---

## 5. Core Components

### 5.1 CSP Message Bus (`internal/bus/`)

The bus is the central nervous system. Every component communicates through typed channels.

**Operations:** Pub/sub, request/reply with timeout, broadcast to all subscribers.

**Topology:** Topics are hierarchical — `agent.{id}.inbox`, `agent.{id}.events`, `dept.{name}.broadcast`, `system.heartbeat`, `channel.{name}.message`, `cron.{job}.trigger`, `learn.interaction`.

### 5.2 Configuration (`internal/config/`)

Three-tier configuration system with 15 sections and 60+ settings:

```
Priority (lowest to highest):
  1. YAML config file (defaults)
  2. Environment variables (AGENTFORGE_ prefix)
  3. CLI flags (cobra, highest priority)

15 Config Sections:
  server, llm, memory, security, engine, tool,
  dashboard, mcp, session, skill, logging, telemetry,
  cron, channel, learn

Persistence: YAML node patching — writes back only changed
keys to the YAML file, preserving comments and formatting.
```

### 5.3 Engine (`internal/engine/`)

Four subsystems:

| Subsystem | File | Role |
|-----------|------|------|
| **Agent Pool** | `agent.go` | Goroutine pool managing agent lifecycles. Spawn with capability validation, run event loop, terminate with cleanup. |
| **Department Pools** | `department.go` | Bounded worker pools per department. Semaphore-limited concurrency. Named pools like "content", "seo", "social". |
| **DAG Pipeline** | `pipeline.go` | Directed acyclic graph executor. Topological sort, parallel stage execution, channel-based data passing between dependent stages. Retry + timeout per stage. |
| **Subagent Tree** | `tree.go` | Hierarchical delegation manager. Max depth enforced. Child capabilities are strict subsets of parent. Timeout cascades. Results aggregate upward. |

### 5.4 MeMex Zero RAG Memory (`internal/memory/`)

```go
// Store is the deterministic, file-backed memory system
type Store interface {
    Put(path string, data []byte, meta Metadata) error
    Get(path string) ([]byte, error)
    Search(query string) ([]Result, error)
    Append(path string, data []byte) error
}
```

**Architecture:**
- **Markdown files** as canonical storage (MEMORY.md, YYYY-MM-DD.md, agent state, project context)
- **Git** for versioning (history, diff, rollback)
- **SQLite FTS5** for full-text search with porter stemming / unicode61 tokenizer
- **FileWatcher** (`fsnotify`) for real-time change detection

**Schema:**
```
memory/
├── MEMORY.md              # Long-term memory (curated)
├── YYYY-MM-DD.md          # Daily logs (raw)
├── decisions.md           # Decision register
├── agents/{id}/
│   ├── state.md           # Current agent state
│   └── learnings.md       # Accumulated learnings
├── projects/{name}/
│   └── context.md         # Project context
├── skills/                # Auto-generated SKILL.md files
│   └── learned/           # From learn.Generator
└── .git/                  # Git repository
```

**FTS5 Index:**
```sql
CREATE TABLE documents (
    path TEXT PRIMARY KEY,
    content TEXT,
    kind TEXT,
    agent_id TEXT,
    updated_at INTEGER
);

CREATE VIRTUAL TABLE documents_fts USING fts5(
    path, content, tokenize='porter unicode61'
);
```

### 5.5 Capability-Based Security (`internal/security/`)

```go
type Capability struct {
    ID          string          // UUID
    Issuer      string          // "agentforge" or parent agent ID
    Subject     string          // agent ID this was issued to
    Permissions []Permission    // allowed operations
    Resources   []ResourceRule  // scoped resource access
    TokenBudget int64           // max tokens per session
    Timeout     time.Duration   // max session duration
    ExpiresAt   time.Time       // absolute expiry
    Signature   []byte          // HMAC-SHA256 signature (prevents forgery)
    ParentID    string          // delegation chain (for audit)
}

type Permission string
const (
    PermRead     Permission = "read"
    PermWrite    Permission = "write"
    PermExec     Permission = "exec"      // shell
    PermNet      Permission = "net"
    PermSpawn    Permission = "spawn"     // create sub-agents
    PermDelegate Permission = "delegate"  // pass capabilities down
)

type ResourceRule struct {
    Path       string       // filesystem path (glob allowed)
    Domain     string       // network domain (glob allowed)
    Operations []Permission // allowed ops on this resource
}
```

**Enforcer** (`enforcer.go`): Called before every tool invocation. Checks capability validity (HMAC signature verification), resource scoping, token budget, and timeout. Can derive child capabilities (strict subset of parent).

**Enforcement points:**
1. Agent spawn — capability validated, signed, bound to agent ID
2. Tool invocation — every tool call checks capability
3. Filesystem access — path must match resource rules
4. Network access — domain must match resource rules
5. Subagent spawn — derived capability (subset of parent)
6. Token budget — every LLM call deducts from budget
7. Timeout — absolute clock; agent terminated if exceeded

**Signature scheme:** HMAC-SHA256(permissions + resources + subject + expires, serverSecret). Server verifies on every check. Agent cannot forge.

### 5.6 LLM Adapter Layer (`internal/llm/`)

```go
type Provider interface {
    Chat(ctx context.Context, messages []Message) (*Response, error)
    HealthCheck(ctx context.Context) error
    ContextWindow() int  // Returns model-specific token limit
}
```

**Built adapters (all with real HTTP implementations):**
| Adapter | File | Notes |
|---------|------|-------|
| `OpenAIClient` | `openai.go` | GPT-4, GPT-4o, compatible endpoints |
| `OllamaClient` | `ollama.go` | Local models, no API key required |
| `AnthropicClient` | `anthropic.go` | Claude models |

**Resilience:**
| Component | File | Function |
|-----------|------|----------|
| **Circuit Breaker** | `breaker.go` | Prevents cascading failures. Opens after threshold errors, half-open probe, auto-reset. |
| **Fallback Chain** | `fallback.go` | Ordered provider list. Tries each in sequence on failure. Retry with exponential backoff. |

**ContextWindow()** returns the model-specific context window size (e.g., 128K for GPT-4o, 200K for Claude). Used by the session manager for auto-compaction thresholds.

### 5.7 Tool System (`internal/tool/`)

```go
type Tool interface {
    Meta() ToolMeta       // Name, description, parameter schema
    Execute(ctx context.Context, args map[string]interface{}) (Result, error)
}
```

**Registry** manages tool discovery, registration, and dispatch. 19 built-in tools:

| Category | Tools |
|----------|-------|
| **Filesystem** | `read`, `write`, `edit` |
| **Network** | `http`, `search`, `fetch` |
| **Memory** | `memory_search`, `memory_get` |
| **VCS** | `git` |
| **Agent** | `spawn` (create sub-agent), `send` (message agent) |
| **Automation** | `cron` |
| **Shell** | `shell` |
| **Media** | `image_gen` |
| **Sandbox** | `sandbox` |
| **Browser** | `browser` |
| **MCP** | `mcp` (connect to external MCP servers) |

### 5.8 MCP Server (`internal/api/mcp/`)

Implements JSON-RPC 2.0 over HTTP and stdio transports. Exposes AgentForge tools as MCP tools for external MCP clients.

```
Transport options:
  - HTTP (REST endpoint at /mcp)
  - stdio (for subprocess integration)

Protocol: JSON-RPC 2.0
  - tools/list     → 19 built-in tools
  - tools/call     → Execute tool with capability check
  - resources/*    → Memory store as MCP resources
```

### 5.9 Session Manager (`internal/session/`)

Manages agent session lifecycles with transcript persistence and automatic compaction.

**Features:**
- Transcript logging in JSON format
- Auto-compaction at 90% of model context window
- Memory flush to MeMex on compaction
- Pruning of large tool outputs (`maxToolOutputChars`)
- `ContextWindow()` auto-detection from the LLM adapter

### 5.10 Skill System (`internal/skill/`)

**SKILL.md Parser:** Implements the Anthropic SKILL.md specification. Parses skill metadata, tool declarations, and activation rules from markdown files.

**Repository:** Loads skills from filesystem directories. Manages skill lifecycle (load, activate, deactivate, unload).

**Auto-Activation Scoring Engine:** Scores skills against the current context to determine which skills should auto-activate. Uses keyword matching, semantic relevance, and explicit trigger rules.

**Marketplace** (`marketplace.go`):
- **SkillsMP API client** — search and download skills from skillsmp.com
- **GitHub discovery** — find skills in community repositories

### 5.11 Web Dashboard (`internal/dashboard/`)

Single-page application served by the embedded daemon. Built with:
- **htmx** for partial page updates (15 page partials)
- **CSS** with glassmorphism design system
- **97+ icons** for visual navigation
- **Stat cards** on overview page (agent count, pipeline status, token usage)
- **Cost tracking** with per-model pricing
- **Recent events feed** for real-time monitoring
- **Pipeline status** visualization (DAG progress, stage states)
- **Embedded static assets** via `embed.FS` — no external dependencies

### 5.12 Cron Scheduler (`internal/cron/`)

Native cron scheduler with no external dependencies. Parses standard crontab format and `@every` shorthand syntax for recurring job execution.

#### Schedule Formats

```
Standard cron:  "min hour dom month dow"
  Examples:     "0 9 * * 1"      — every Monday at 9:00 AM
                "*/5 * * * *"    — every 5 minutes
                "0 0 1 * *"      — midnight on the 1st of each month
                "30 14 15 6 3"   — 2:30 PM on June 15 if it's a Wednesday

@every shorthand:
  "@every 5m"                    — every 5 minutes
  "@every 1h"                    — every hour
  "@every 30s"                   — every 30 seconds
  "@every 90m"                   — every 90 minutes
```

#### Job Struct

```go
type Job struct {
    ID       string        // UUID
    Name     string        // Human-readable label
    Schedule string        // Cron expression or @every shorthand
    Command  string        // What to execute (pipeline name, tool, etc.)
    Args     []string      // Arguments to the command
    LastRun  time.Time     // Timestamp of last execution
    NextRun  time.Time     // Timestamp of next scheduled execution
}
```

#### Scheduler

```go
type Scheduler interface {
    Start() error                           // Begin the time.Ticker loop
    Stop() error                            // Graceful shutdown
    Add(job Job) (string, error)            // Register a new job, returns ID
    Remove(id string) error                 // Remove by ID
    Trigger(id string) error                // Force immediate execution
    List() []Job                            // Snapshot of all registered jobs
}
```

**Execution Loop:** A dedicated goroutine runs a `time.Ticker` at 1-second resolution. On each tick, it evaluates all registered jobs against their schedule. Any job whose `NextRun` has passed fires immediately — the scheduler publishes a trigger envelope to the bus on topic `cron.{job}.trigger`, updates `LastRun` to now, and recalculates `NextRun`.

**Pipeline Integration:** Pipeline cron triggers are auto-registered from the DAG pipeline configuration. When a pipeline stage specifies a `cron:` trigger in its definition, the scheduler automatically creates a corresponding Job on startup. This means any DAG pipeline can be scheduled without manual cron registration — declare the cron expression in your pipeline YAML and it runs on schedule.

**Lifecycle:** The scheduler is started during daemon initialization (`cmd/agentforge` → `cron.Start()`) and stopped during graceful shutdown. Jobs persist as part of the application config, not in a separate database — they're defined declaratively alongside pipelines.

### 5.13 Multi-MCP Server (`internal/api/mcp/manager.go`)

The Multi-MCP Manager (`manager.go`) extends the single-server MCP implementation to support running N MCP servers simultaneously from a configuration array. Each server is independently configured with its own transport, tool filter, and lifecycle.

#### Server Config

```go
type MCPServerConfig struct {
    Name       string         // Unique server identifier
    Enabled    bool           // Whether to start on boot
    Transport  string         // "http" or "stdio"
    Port       int            // HTTP port (for "http" transport)
    Command    string         // Subprocess command (for "stdio" transport)
    Args       []string       // Subprocess arguments
    ToolFilter []string       // Whitelist of tool names to expose
}
```

#### Transport: HTTP

When `transport` is `"http"`, the manager creates a dedicated `http.ServeMux` per server. Each server gets its own port and isolated routing. The manager binds the mux to the configured port and starts an HTTP listener. All standard MCP JSON-RPC 2.0 endpoints are routed through this mux:

- `POST /` — JSON-RPC request handler (`tools/list`, `tools/call`, `resources/read`)
- `GET /health` — Health check endpoint

Multiple HTTP MCP servers can run simultaneously on different ports — e.g., one on `:9090` exposing all tools, another on `:9091` exposing only memory tools.

#### Transport: stdio

When `transport` is `"stdio"`, the manager spawns the configured command as a subprocess using `os/exec`. JSON-RPC 2.0 messages are exchanged over the subprocess's `stdin` (write) and `stdout` (read). `stderr` is captured and logged for diagnostics. The manager manages the subprocess lifecycle — start on server init, terminate on shutdown, restart on unexpected exit.

#### ToolFilter

The `ToolFilter` field is a whitelist of tool names. When non-empty, the server only exposes the listed tools in its `tools/list` response. When empty (default), all 19 built-in tools are exposed. This enables least-privilege MCP server configurations — a read-only server might expose only `[read, memory_search, memory_get]` while an admin server exposes everything.

#### Manager Lifecycle

```go
type MCPManager interface {
    StartAll() error                    // Start all enabled servers
    StopAll() error                     // Graceful shutdown of all servers
    Start(name string) error           // Start a specific server
    Stop(name string) error            // Stop a specific server
    ServerInfo(name string) (ServerInfo, error)   // Config + runtime info
    ServerState(name string) (ServerState, error)  // Running/stopped, uptime, connections
}
```

**ServerInfo** returns the full configuration for a named server plus metadata (uptime, tool count, connection status). **ServerState** returns a lightweight snapshot of whether the server is running, how long it's been up, and how many active connections it has. Both are exposed through the dashboard for real-time MCP server monitoring.

### 5.14 Channel Adapters (`internal/channel/`)

Channel adapters bridge external messaging platforms (Telegram, Discord) into the AgentForge CSP bus. Each adapter implements the `Adapter` interface and runs as a managed background goroutine.

#### Adapter Interface

```go
type Adapter interface {
    Name() string                  // "telegram", "discord"
    Start(ctx context.Context) error
    Stop() error
    Status() AdapterStatus         // Running state + connection metrics
}
```

#### Manager

```go
type Manager struct {
    adapters map[string]Adapter    // name → adapter
    bus      *bus.Bus              // CSP bus reference
}

func (m *Manager) StartAll() error     // Start enabled adapters
func (m *Manager) StopAll() error      // Graceful shutdown
func (m *Manager) Status() map[string]AdapterStatus
```

The manager reads the `channel` config section to determine which adapters are enabled. On startup, it initializes and starts each enabled adapter. All adapters publish incoming messages to the bus on a consistent topic pattern.

#### Telegram Adapter (`telegram.go`)

Uses the **polling getUpdates API** (not webhooks). A dedicated goroutine runs a poll loop at a configurable interval (default: 1 second).

```
Polling Flow:
  1. GET /bot{token}/getUpdates?offset={offset}&timeout={long_poll_timeout}
  2. Parse incoming updates (messages, commands, callbacks)
  3. For each update:
     a. Normalize to internal Message struct
     b. bus.Publish("channel.telegram.message", message)
  4. Update offset to last_seen update_id + 1
  5. Sleep for poll interval (respects rate limits)
```

**Command Handling:** The Telegram adapter recognizes bot commands (`/start`, `/help`, `/status`). When a command is received, it publishes to topic `channel.telegram.command.{name}`. Pre-registered command handlers in the engine can subscribe and respond. Built-in commands:

| Command | Topic | Action |
|---------|-------|--------|
| `/start` | `channel.telegram.command.start` | Welcome message, agent introduction |
| `/help` | `channel.telegram.command.help` | Available commands + agent capabilities |
| `/status` | `channel.telegram.command.status` | System health, agent count, uptime |

Responses are sent via `sendMessage` API call back to the chat.

#### Discord Adapter (`discord.go`)

Uses the **WebSocket Gateway v10** protocol. Maintains a persistent WebSocket connection to Discord's gateway.

```
Gateway Flow:
  1. GET /gateway/bot → obtain gateway URL with ?v=10&encoding=json
  2. Connect WebSocket to gateway URL
  3. Send IDENTIFY payload:
     {
       "token": "{bot_token}",
       "intents": 513,        // GUILD_MESSAGES + DIRECT_MESSAGES
       "properties": { ... }
     }
  4. Receive READY event → gateway connection established
  5. Heartbeat loop: send opcode 1 every heartbeat_interval ms
  6. Message loop: process incoming DISPATCH events
     a. Filter: MESSAGE_CREATE events only
     b. Normalize to internal Message struct
     c. bus.Publish("channel.discord.message", message)
  7. On disconnect: exponential backoff reconnect
```

**Heartbeat:** The Discord Gateway requires regular heartbeats. The adapter maintains a dedicated goroutine that sends heartbeat payloads (opcode 1) at the interval specified in the `HELLO` event. Missed heartbeats trigger automatic reconnection.

#### Native WebSocket (`ws.go`)

The Discord adapter uses a **fully native WebSocket implementation** (RFC 6455) with zero external dependencies. No gorilla/websocket, no nhooyr.io/websocket — just Go's `net/http`, `crypto/sha1`, `crypto/rand`, and `encoding/binary`.

```
RFC 6455 Implementation:
  - Upgrade: HTTP/1.1 101 Switching Protocols handshake
  - Framing: Parse/extract opcode, payload length, mask, payload data
  - Masking: Client→Server frames XOR-masked with 4-byte key (crypto/rand)
  - Opcodes: Text(0x1), Binary(0x2), Close(0x8), Ping(0x9), Pong(0xA)
  - Control frames: Ping/Pong/Close handled inline during read loop
  - Close handshake: Send Close frame, wait for echo, terminate connection
```

**Why native?** Eliminates external dependency risk for a critical-path component. The WebSocket protocol is well-defined and the implementation surface is small (~300 lines). No supply-chain vulnerability surface from third-party WebSocket libraries.

#### Bus Integration

All adapters publish normalized messages to the bus using a consistent topic pattern:

```
channel.{adapter_name}.message       — all incoming messages
channel.{adapter_name}.command.{cmd} — recognized commands
channel.{adapter_name}.status        — adapter status updates
```

The bus topic `channel.{name}.message` is the primary integration point. The self-learning engine's Observer subscribes to all `channel.*.message` topics to log interactions. Engine agents can subscribe to specific channel topics to handle incoming messages.

### 5.15 Self-Learning Engine (`internal/learn/`)

The self-learning engine observes bus traffic, extracts behavioral patterns from agent interactions, and automatically generates SKILL.md files for discovered patterns. It implements a three-stage pipeline: **Observe → Extract → Generate**.

#### Pipeline Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                    Self-Learning Engine Pipeline                      │
│                                                                      │
│  bus.Publish(channel.*.message)                                      │
│         │                                                            │
│         ▼                                                            │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐          │
│  │              │     │              │     │              │          │
│  │  OBSERVER    │────►│  EXTRACTOR   │────►│  GENERATOR   │          │
│  │              │     │              │     │              │          │
│  │ Watches bus  │     │ Jaccard      │     │ SKILL.md     │          │
│  │ events       │     │ similarity   │     │ file writer  │          │
│  │              │     │ clustering   │     │              │          │
│  │ Logs         │     │              │     │ confidence   │          │
│  │ Interactions │     │ Time windows │     │ > 0.8 &      │          │
│  │              │     │              │     │ count ≥ 5    │          │
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘          │
│         │                    │                    │                  │
│         ▼                    ▼                    ▼                  │
│  interaction_log     pattern_clusters     memory/skills/             │
│  (in-memory          (in-memory           learned/                   │
│   ring buffer)        similarity graph)   {pattern}.SKILL.md         │
│                                                                      │
│  MANAGER orchestrates the pipeline:                                  │
│    manager.Run() → observer.Start() → extractor.Process() →         │
│    generator.Generate()                                              │
└──────────────────────────────────────────────────────────────────────┘
```

#### Observer (`observer.go`)

The Observer subscribes to bus topics matching `channel.*.message` and `agent.*.events`. It logs every interaction as an `Interaction` struct:

```go
type Interaction struct {
    ID        string            // UUID
    Timestamp time.Time         // When it occurred
    Topic     string            // Bus topic (channel.telegram.message, etc.)
    AgentID   string            // Agent that handled it
    Input     string            // Incoming message/query text
    Actions   []string          // Tools/skills activated in response
    Output    string            // Final response text
    Duration  time.Duration     // Processing time
}

type Observer struct {
    bus      *bus.Bus
    buffer   []Interaction      // Ring buffer, configurable size
    mu       sync.RWMutex
}

func (o *Observer) Start()      // Subscribe to bus, begin logging
func (o *Observer) Recent(d time.Duration) []Interaction  // Window query
```

Interactions are stored in an in-memory ring buffer (configurable size, default 10,000 entries). The Observer runs continuously, draining interactions into the extractor on a configurable tick interval.

#### Extractor (`extractor.go`)

The Extractor runs periodically over the interaction buffer, applying **Jaccard similarity** to cluster related interactions.

**Jaccard Word-Overlap Similarity:**

```
              |Words(A) ∩ Words(B)|
J(A,B) = ──────────────────────────────
              |Words(A) ∪ Words(B)|

Where Words(X) = set of tokens after:
  - Lowercasing
  - Removing punctuation
  - Filtering stop words
  - Splitting on whitespace

Score range: 0.0 (completely different) to 1.0 (identical word sets)
```

**Time Windows:** The extractor uses a 5-minute sliding window. Only interactions within the same 5-minute window are compared for clustering. This keeps the clustering computation bounded and prevents false matches across unrelated conversations that happen to share vocabulary.

**Confidence Scoring:**

```go
type Cluster struct {
    ID           string          // UUID
    Interactions []string        // Interaction IDs in this cluster
    Keywords     []string        // Common words across interactions
    Size         int             // Number of interactions
    Confidence   float64         // 0.0 to 1.0
}

// Confidence formula:
// confidence = avg_pairwise_similarity × min(1.0, size/10)
//
// avg_pairwise_similarity = mean of all J(A,B) within the cluster
// size_factor = min(1.0, count/10) — caps at 1.0 once cluster has 10+ interactions
```

The confidence formula rewards both high internal similarity (interactions that look very similar) and sufficient volume (clusters with more data are more trustworthy).

#### Generator (`generator.go`)

The Generator takes clusters from the Extractor and produces SKILL.md files when a cluster meets the production threshold:

```
Threshold: confidence > 0.8 AND count ≥ 5
```

```go
type Generator struct {
    outputDir string  // memory/skills/learned/
    skillRepo *skill.Repository
}

func (g *Generator) Generate(cluster Cluster) (*skill.Skill, error)
```

**Generated SKILL.md Structure:**

```markdown
# {Pattern Name} — Auto-Generated Skill

**Generated:** {timestamp}
**Source:** {interaction_count} interactions
**Confidence:** {confidence_score}
**Status:** draft

## Activation Triggers
{keywords from cluster}

## Description
{Summary generated from common action patterns}

## Tools Used
{List of tools most commonly activated in this cluster}

## Example Interactions
{Top 2-3 representative interactions}
```

Generated skills are written to `memory/skills/learned/{pattern-name}.SKILL.md`. On generation, the skill is automatically registered with the skill repository so it becomes available for auto-activation. A `Status: draft` flag distinguishes learned skills from hand-crafted ones, allowing human review and promotion.

#### Manager (`manager.go`)

The Manager orchestrates the full Observe→Extract→Generate pipeline:

```go
type Manager struct {
    observer  *Observer
    extractor *Extractor
    generator *Generator
}

func (m *Manager) Run(ctx context.Context) error  // Start pipeline loop
func (m *Manager) Stats() LearnStats              // Pipeline metrics

type LearnStats struct {
    InteractionsObserved int64
    ClustersFound        int64
    SkillsGenerated      int64
    LastExtraction       time.Time
    LastGeneration       time.Time
}
```

The pipeline runs on a configurable tick (default: every 5 minutes, matching the extractor window). On each tick: (1) Observer drains recent interactions into extractor, (2) Extractor computes Jaccard similarity and forms clusters, (3) For each cluster meeting the threshold, Generator creates/replaces the corresponding SKILL.md file.

---

## 6. Session Lifecycle

### 6.1 Session States

```
CREATED → ACTIVE → COMPACTING → COMPACTED → TERMINATED
                                 │
                                 └── memory flush to MeMex
```

### 6.2 Compaction Pipeline

The session manager monitors token usage in real time and triggers compaction when the session transcript approaches the model's context window limit.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Compaction Pipeline                           │
│                                                                 │
│  1. MONITOR                                                     │
│     Count tokens in transcript after each LLM exchange          │
│                                                                 │
│  2. THRESHOLD CHECK                                             │
│     if used_tokens >= 90% of adapter.ContextWindow()            │
│                                                                 │
│  3. BUILD SUMMARY                                               │
│     buildSummary():                                             │
│       - Feed oldest conversation turns to LLM                   │
│       - Generate concise summary (decisions, state, context)    │
│       - Preserve tool call results above significance threshold │
│                                                                 │
│  4. PURGE OLD TURNS                                             │
│       - Remove summarized conversation turns from transcript    │
│       - Keep last N turns as "tail" for immediate context       │
│       - Prepend summary as system context                       │
│                                                                 │
│  5. FLUSH TO MEMORY                                             │
│     flushToMemory():                                            │
│       - Write summary to memory/agent-{id}-session.md           │
│       - memory.Store.Put(path, summary_json, metadata)          │
│       - Git commit the memory file                              │
│                                                                 │
│  6. RESUME                                                      │
│     Agent continues with compacted context + fresh token budget │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 6.3 Compaction Details

```
Trigger:       usedTokens >= 0.90 × ContextWindow()
ContextWindow: auto-detected from llm.Adapter.ContextWindow()
               (model-specific: GPT-4o=128K, Claude=200K, Ollama=varies)

buildSummary():
  Input:  Transcript entries from [oldest ... (oldest+N)]
  Output: Structured summary string containing:
    - Key decisions made
    - Current task state
    - Files created/modified
    - Pending actions
    - User preferences expressed

Tail Preservation:
  Latest M conversation turns are preserved verbatim
  M = min(5, total_turns * 0.2)  — at least 5, at most 20% of turns

flushToMemory():
  Destination: memory/agents/{agentID}/sessions/{timestamp}.json
  Includes:   Summary, compaction timestamp, token counts before/after
  Storage:    memory.Store.Put() → SQLite FTS5 indexed → git committed

Pruning:
  maxToolOutputChars: Truncates tool outputs exceeding this limit
  Default: 8000 chars per tool result
  Applied: During transcript write, not during compaction
```

---

## 7. Configuration Reference

### 7.1 Config Sections

| Section | Settings | Description |
|---------|----------|-------------|
| `server` | host, port, tls, cors | Daemon HTTP server |
| `llm` | provider, model, api_key, temperature, max_tokens, timeout | LLM provider configuration |
| `memory` | root_path, git_enabled, sync_remote | MeMex memory store |
| `security` | server_secret, max_depth, default_timeout | Capability security |
| `engine` | max_agents, department_pools, pipeline_timeout | Agent runtime |
| `tool` | enabled_tools, tool_timeouts | Tool system |
| `dashboard` | enabled, port, theme | Web dashboard |
| `mcp` | enabled, transport, port, servers[] | MCP server / Multi-MCP manager |
| `session` | compaction_threshold, max_tool_output_chars, transcript_dir | Session management |
| `skill` | skill_dirs, auto_activate, marketplace_enabled | Skill system |
| `logging` | level, format, output | Logging |
| `telemetry` | enabled, endpoint, interval | Metrics & tracing |
| `cron` | enabled, jobs[], tick_interval | Native cron scheduler |
| `channel` | adapters{} (telegram, discord), poll_interval, reconnect_backoff | Channel adapter configuration |
| `learn` | enabled, extraction_interval, similarity_threshold, min_cluster_size, confidence_threshold, buffer_size | Self-learning engine |

### 7.2 Config Loading

```
Priority (highest wins):
  CLI flags  ─────────────────────────►  applied last
  Environment variables (AGENTFORGE_*) ►  applied second
  YAML config file                     ►  applied first

Persistence:
  Config writes use YAML node patching —
  only changed keys are modified in the file,
  preserving comments, whitespace, and formatting.
```

---

## 8. Tool Interactions

### 8.1 Tool Execution Flow

```
Agent decides to invoke tool
         │
         ▼
┌─────────────────────┐
│ security.Enforcer.  │  Check capability allows this tool + resource
│ Check(action)       │
└────────┬────────────┘
         │ ✓ allowed
         ▼
┌─────────────────────┐
│ tool.Registry.      │  Look up tool by name
│ Execute(name, args) │
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│ Tool.Execute(ctx,   │  Actual tool logic
│ args)               │
└────────┬────────────┘
         │
    ┌────┴────┬──────────┬──────────┬──────────┐
    ▼         ▼          ▼          ▼          ▼
 filesystem  network   memory     agent       MCP
 tools       tools     tools      tools       tools
 (r/w/e)    (http/     (MeMex)   (spawn/     (external)
            search/              send)
            fetch)
         │
         ▼
┌─────────────────────┐
│ Session log         │  Record tool invocation + result in transcript
│ (JSON persistence)  │  Apply maxToolOutputChars pruning
└─────────────────────┘
```

### 8.2 Tool→Bus Integration

```
Agent Tool: spawn(name, capability, department)
  → engine creates new agent goroutine in department pool
  → registers agent inbox channel on bus
  → capability token signed and bound
  → returns agent ID to caller

Agent Tool: send(target_agent_id, message)
  → bus.Publish(Envelope{Target: target_agent_id, ...})
  → target agent picks up from inbox channel
  → processes and responds via bus.Publish back
```

---

## 9. Build & Development

### 9.1 Build

```bash
make build        # go build -o bin/agentforge ./cmd/agentforge
make test         # go test ./...
make lint         # golangci-lint run
make docker       # docker build -t agentforge .
```

### 9.2 Run

```bash
# Daemon mode
./bin/agentforge daemon --config agentforge.yaml

# Dashboard available at http://localhost:8080
# MCP servers at configured ports (default http://localhost:9090, stdio available)
```

### 9.3 Dependencies

| Dependency | Purpose |
|-----------|---------|
| `cobra` | CLI framework |
| `viper` | Configuration management |
| `go-sqlite3` | SQLite driver (CGo) |
| `go-git` | Git operations (pure Go, no libgit2) |
| `fsnotify` | File system watcher |
| `htmx` | Dashboard partial updates (embeddable JS) |

---

## 10. Appendix: Architecture Decisions

| Decision | Rationale | Date |
|----------|-----------|------|
| Go (not Python/Rust) | Goroutines for agent model, static binary, fast compile, good ecosystem | 2026-05 |
| MeMex Zero RAG (not vector DB) | Deterministic, grep-able, git-trackable, no embedding drift | 2026-05 |
| HMAC capability tokens (not JWT) | Keyed hash prevents offline forgery, no key distribution needed | 2026-05 |
| CSP bus (not gRPC streaming) | In-process channels are zero-copy, Go-native, no serialization overhead | 2026-05 |
| SQLite (not Postgres) | Embedded, zero ops, single-file persistence, FTS5 built-in | 2026-05 |
| htmx (not React/Vue) | Minimal JS, server-rendered, works without build step, 15 partials | 2026-05 |
| Cobra+Viper (not flaggy) | Industry standard, composable commands, multi-source config | 2026-05 |
| SkillsMP API integration | Community skill marketplace, discoverability, no reinventing package management | 2026-06 |
| Glassmorphism dashboard | Modern aesthetic, distinct visual identity, works on all backgrounds | 2026-06 |
| Native WebSocket (not gorilla) | No external deps for critical-path component, RFC 6455 is well-defined, ~300 LOC | 2026-06 |
| Jaccard similarity (not cosine) | Interpretable, no embeddings needed, fast on strings, good for short-text clustering | 2026-06 |
| Cron scheduler in-process (not system cron) | Portable, no OS dependency, declarative YAML config, pipeline-aware | 2026-06 |
| Telegram polling (not webhooks) | No public endpoint required, simpler deployment, works behind NAT/firewall | 2026-06 |
| Auto-generated SKILL.md with draft flag | Human-in-the-loop review path, learned skills are distinguishable from hand-crafted | 2026-06 |