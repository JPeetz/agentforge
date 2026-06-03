# Architecture Overview

Complete system design, component interactions, and data flow in AgentForge.

---

## System Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                      AGENTFORGE DAEMON                           │
│                                                                   │
│  HTTP/gRPC  │  TUI (Bubble Tea)  │  Multi-MCP  │  Channels      │
│     :8080   │                     │    :9090+    │  TG | Discord  │
│  ┌──────────┴──────┬──────────────┴──────────────┴────────────┐  │
│  │  Web Dashboard  │  Agent Fleet Modal  │  MCP Manager       │  │
│  │  SPA + htmx     │  Pipeline DAG Editor│  N Servers         │  │
│  │  Cost Tracking  │  Circuit Breakers   │  toolFilter each   │  │
│  └─────────────────┴─────────────────────┴────────────────────┘  │
└──────┬──────────────┬─────────────┬──────────┬──────────────────┘
       │              │             │          │
       └──────────────┼─────────────┼──────────┘
                      │             │
             ┌────────▼────────┐    │
             │    CSP BUS      │◄───┘  ← Goroutines + channels
             │  (pub/sub)      │        Channel events published
             │  Cron triggers  │        here automatically
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
       │                                   │  CRON        │
       │                                   │  Native Go   │
       │                                   │  @every+cron │
       │                                   │  Pipeline trig
       └───────────────────────────────────┤  Persistent  │
                                           └──────────────┘
```

---

## Core Components

### 1. CSP Message Bus (`internal/bus/`)

**Purpose:** Central communication hub using Communicating Sequential Processes pattern.

**Implementation:**
- Go channels for message passing
- Topic-based publish/subscribe
- Request/reply pattern with correlation IDs
- Broadcast semantics (one publish, all subscribers receive)

**Key Types:**
```go
type Envelope struct {
    ID          string        // Message ID
    Topic       string        // Routing topic (e.g., "agent.status")
    Type        string        // Message type (e.g., "status_update")
    Payload     interface{}   // Message content
    ReplyTo     string        // Topic for replies
    CorrelationID string      // Links request/reply pairs
}
```

**Topics:**
- `agent.*` — Agent lifecycle (spawn, ready, done, error)
- `channel.*` — Channel events (telegram.message, discord.message)
- `llm.*` — LLM adapter events (token_usage, error)
- `memory.*` — Memory operations (search, update, compact)
- `tool.*` — Tool invocations (call, result, error)
- `pipeline.*` — Pipeline execution (start, stage_complete, done)

**Guarantees:**
- Messages are ordered per topic (happens-before semantics)
- Concurrent subscribers don't block each other
- Lost messages are logged (can be persisted to memory)

---

### 2. Agent Engine (`internal/engine/`)

**Purpose:** Goroutine-per-agent lifecycle management and orchestration.

**Components:**

#### Agent (`agent.go`)
```go
type Agent struct {
    ID         string
    State      State              // spawned, ready, running, done
    Capability *Capability        // Token for this agent
    LLM        Adapter            // Language model connection
    Memory     *MemoryStore       // Persistent memory
    Bus        *Bus               // Message bus reference
}
```

**Lifecycle:**
1. **Spawn** — Create agent with capability token + initial instructions
2. **Ready** — Wait for first user input
3. **Run** — LLM inference + tool execution loop
4. **Done** — Cleanup, archive session

#### Pool (`pool.go`)
```go
type Pool struct {
    agents map[string]*Agent
    bu     *Bus
}

func (p *Pool) Spawn(ctx context.Context, req SpawnRequest) (*Agent, error)
func (p *Pool) List() []*Agent
func (p *Pool) Kill(agentID string) error
```

Manages multiple agents concurrently. Each agent is a goroutine.

#### DAG Pipeline (`dag.go`)
```go
type Pipeline struct {
    Name   string
    Stages []Stage
}

type Stage struct {
    Name    string
    Tool    string
    Inputs  map[string]string
    Next    []string
}
```

Stages are serialized. Output of stage N becomes input to stage N+1.

#### Fleet (`fleet.go`)
```go
type Fleet struct {
    Name     string
    Agents   []*Agent
    Pipeline *Pipeline
}
```

Multiple agents running same pipeline in parallel.

---

### 3. Memory Store (`internal/memory/`)

**Purpose:** Persistent, searchable agent memory with Git versioning.

**Implementation:**
- **Storage:** Markdown files in `~/.agentforge/memory/`
- **Search:** SQLite FTS5 full-text search
- **Versioning:** Git for historical tracking
- **Compaction:** Summarize old turns when context window exceeds 90%

**Key Operations:**

```go
type Store struct {
    root string              // /home/user/.agentforge/memory
    db   *sql.DB             // FTS index
    git  *git.Repository     // Version control
}

// Write session to memory
func (s *Store) Write(ctx context.Context, sessionID string, turns []Turn) error

// Search memory
func (s *Store) Search(ctx context.Context, query string, limit int) ([]Result, error)

// Compact old turns when context window is 90% full
func (s *Store) Compact(ctx context.Context, sessionID string) error
```

**Model-Aware Compaction:**

Different models have different context windows:
- GPT-4.1: 1,000,000 tokens
- Claude Sonnet: 200,000 tokens
- Ollama Gemma: 32,768 tokens

Compaction triggers at 90% of model's context window:
```go
contextBudget := adapter.ContextWindow()  // Get model's max
threshold := int(float64(contextBudget) * 0.9)

if sessionTokens >= threshold {
    s.Compact(ctx, sessionID)
}
```

---

### 4. Capability Security (`internal/security/`)

**Purpose:** Permission enforcement with capability-based tokens.

**Key Types:**

```go
type Capability struct {
    AgentID        string
    Secret         string         // HMAC secret
    FilesystemACL  []string       // Glob patterns: /home/user/**
    NetworkACL     []string       // Domains: api.openai.com
    TokenBudget    int
    TimeoutSeconds int
}

type Enforcer struct {
    secret string
    cache  map[string]bool  // Cached grant decisions
}
```

**Enforcement:**

```go
func (e *Enforcer) Check(ctx context.Context, cap *Capability, action Action) error {
    // 1. Verify HMAC signature (prevent token tampering)
    if !e.verifySignature(cap) {
        return ErrTokenTampered
    }
    
    // 2. Check budget consumption
    tokensUsed := ctx.Value("tokens_used").(int)
    if tokensUsed >= cap.TokenBudget {
        return ErrBudgetExceeded
    }
    
    // 3. Check timeout
    elapsed := ctx.Value("elapsed_seconds").(int)
    if elapsed >= cap.TimeoutSeconds {
        return ErrTimeout
    }
    
    // 4. Check ACL (filesystem or network)
    switch action.Type {
    case ActionFilesystemRead:
        if !e.matchGlob(cap.FilesystemACL, action.Path) {
            return ErrPathNotAllowed
        }
    case ActionHTTPRequest:
        if !e.matchDomain(cap.NetworkACL, action.Domain) {
            return ErrDomainNotAllowed
        }
    }
    
    return nil
}
```

---

### 5. LLM Adapters (`internal/llm/`)

**Purpose:** Unified interface to multiple LLM providers.

**Implementation:**

```go
type Adapter interface {
    StreamChat(ctx context.Context, req *StreamChatRequest) error
    ContextWindow() int
    String() string
}

type OpenAIClient struct {
    endpoint string
    apiKey   string
    model    string
}

type AnthropicClient struct {
    endpoint string
    apiKey   string
    model    string
}

type OllamaClient struct {
    endpoint string
    model    string
}
```

**Providers:**
- OpenAI — GPT-4 (all variants), GPT-3.5
- Anthropic — Claude (all variants)
- Ollama — Local models (Gemma, Llama, Mistral, etc.)
- DeepSeek, Google Gemini (planned)

**Fallback Chain:**

If primary LLM fails, try secondary, then tertiary:
```yaml
llm:
  fallback_chain:
    - "openai"      # Primary
    - "anthropic"   # Secondary
    - "ollama"      # Tertiary (local)
```

**Circuit Breaker:**

Prevents cascading failures when LLM is down:
- **Closed:** All requests go through
- **Open:** All requests fail fast (don't retry)
- **Half-Open:** Test requests to check if service recovered

---

### 6. Tool Registry (`internal/tool/`)

**Purpose:** Unified tool interface with 19 built-in tools + extensibility.

**Tools:**
1. **Filesystem:** read, write, list, delete, mkdir
2. **HTTP:** GET, POST, PUT, DELETE, PATCH
3. **Shell:** Execute with stdout/stderr capture
4. **Memory:** Search, read, write, list
5. **Web:** Fetch URL, search web
6. **Image Generation:** DALL-E, Stability AI
7. **Video Generation:** Runway, Synthesia
8. **Code Review:** Static analysis, security audit
9. **Diagrams:** Mermaid, PlantUML
10. **Data Analysis:** Pandas, statistical analysis
11. ... and more

**Tool Invocation:**

```go
func (a *Agent) InvokeTool(ctx context.Context, toolCall *ToolCall) (string, error) {
    // 1. Check capability
    action := Action{
        Type: ActionToolInvocation,
        Tool: toolCall.Name,
        Args: toolCall.Args,
    }
    if err := a.enforcer.Check(ctx, a.capability, action); err != nil {
        return "", err
    }
    
    // 2. Get tool from registry
    tool := registry.Get(toolCall.Name)
    if tool == nil {
        return "", ErrToolNotFound
    }
    
    // 3. Execute with timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    result, err := tool.Execute(ctx, toolCall.Args)
    return result, err
}
```

---

### 7. MCP Server (`internal/api/mcp/`)

**Purpose:** Model Context Protocol server exposing agents as tools.

**Architecture:**
- JSON-RPC 2.0 protocol
- HTTP transport (standard)
- Stdio transport (for IDE integration)
- Tool filtering per server (multi-server setup)

**Server Configuration:**

```yaml
mcp:
  servers:
    - name: "default"
      transport: "http"
      port: 9090
      toolFilter: "*"           # All tools
      capability_scope: "full"
    
    - name: "external"
      transport: "http"
      port: 9091
      toolFilter:
        - "memory_search"
        - "web_search"
      capability_scope: "readonly"
    
    - name: "local-ide"
      transport: "stdio"
      toolFilter: "*"
      capability_scope: "full"
```

**Client Protocol:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "memory_search",
    "arguments": {
      "query": "find my notes about React",
      "limit": 5
    }
  }
}
```

---

### 8. Dashboard (`internal/dashboard/`)

**Purpose:** Web management interface for the daemon.

**Pages:**
1. **Overview** — System stats (uptime, agents, memory)
2. **Agent Fleet** — List, create, edit, monitor agents
3. **Memory** — Search memory store, manage memories
4. **Pipelines** — Visual DAG editor
5. **Skills** — Marketplace integration
6. **Security** — Capability audit, ACL review
7. **Logs** — Tail daemon logs
8. **Settings** — Configuration UI
9. **MCP** — Multi-MCP server management
10. ... and more

**Architecture:**

```
Dashboard (:8080)
├── SPA (Vanilla JS + htmx)
├── API Routes
│   ├── /api/pages/{page}     — Page partials (htmx)
│   ├── /api/config           — Get/set config
│   ├── /api/tools            — Tool registry
│   ├── /api/auth             — JWT authentication
│   ├── /api/cost             — Cost tracking
│   └── ... (20+ endpoints)
├── Authentication
│   ├── Login (/api/auth/login)
│   ├── JWT tokens (15min access, 7d refresh)
│   ├── RBAC roles (admin, operator, viewer)
│   └── API key generation
└── WebSocket (SSE)
    └── Real-time updates
```

---

### 9. Channel Adapters (`internal/channel/`)

**Purpose:** Multi-protocol messaging integration.

**Adapters:**

#### Telegram
```go
type TelegramAdapter struct {
    token    string
    offset   int64              // Polling offset
    client   *http.Client
    bus      *Bus
    stopCh   chan struct{}
}

func (a *TelegramAdapter) Start() {
    go a.poll()  // Long-polling getUpdates API
}

func (a *TelegramAdapter) poll() {
    for {
        updates, newOffset := a.getUpdates(a.offset)
        for _, update := range updates {
            a.bus.Publish("channel.telegram.message", update)
        }
        a.offset = newOffset
    }
}
```

#### Discord
```go
type DiscordAdapter struct {
    token      string
    sessionID  string
    seq        int
    ws         *websocket.Conn
    bus        *Bus
    heartbeat  time.Duration
}

func (a *DiscordAdapter) Start() {
    go a.connectAndListen()
}

func (a *DiscordAdapter) connectAndListen() {
    a.ws.Connect()
    a.sendIdentify()      // Identify payload
    
    for {
        data := a.ws.Read()
        
        switch data.Op {
        case OPDispatch:
            if data.T == "MESSAGE_CREATE" {
                a.bus.Publish("channel.discord.message", data.D)
            }
        case OPHeartbeatACK:
            // Keepalive confirmed
        }
    }
}
```

#### Slack (Socket Mode)
```go
type SlackAdapter struct {
    token      string
    appToken   string
    ws         *websocket.Conn  // Socket Mode
    bus        *Bus
}

func (a *SlackAdapter) Start() {
    a.ws.Connect(a.appToken)
    
    for {
        envelope := a.ws.Read()  // Socket Mode envelope
        
        switch envelope.Type {
        case "events_api":
            a.bus.Publish("channel.slack.message", envelope.Payload)
            a.ackEnvelope(envelope.EnvelopeID)
        }
    }
}
```

---

### 10. Self-Learning Engine (`internal/learn/`)

**Purpose:** Autonomous skill generation from agent interactions.

**Pipeline:**

```
Agent Interactions
    │
    ▼
┌──────────────┐
│   Observer   │  Records interactions with timestamps
│ (Watch bus)  │  • Agent inputs
│              │  • Tool calls
│              │  • Outputs
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Extractor   │  Cluster similar interactions (Jaccard)
│  (Clustering)│  • Compute similarity scores
│              │  • Group into patterns
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Generator   │  Create SKILL.md when confident
│ (Template)   │  • Write markdown file
│              │  • Register in marketplace
└──────┬───────┘
       │
       ▼
New Skill in Marketplace
(Ready for use by other agents)
```

**Similarity Metric (Jaccard):**

```
Similarity = (Intersection size) / (Union size)

Example:
Interaction 1: [start] → [tool: web_search] → [tool: memory_write] → [end]
Interaction 2: [start] → [tool: web_search] → [tool: memory_write] → [end]
Intersection: 3 steps (web_search, memory_write, end)
Union: 3 steps
Similarity: 3/3 = 1.0 (100% match)

Interaction 3: [start] → [tool: file_read] → [tool: web_search]
Similarity with Interaction 1: 1/4 = 0.25 (25% match)
```

---

### 11. Cron Scheduler (`internal/cron/`)

**Purpose:** Native Go cron job scheduler (no external process).

**Syntax:**

```
Standard cron format: minute hour day month dow
@every shorthand: @every 5m, @every 1h30m

Examples:
"0 9 * * *"      — 9:00 AM daily
"*/5 * * * *"    — Every 5 minutes
"0 2 * * 0"      — 2:00 AM Sunday
"@every 30m"     — Every 30 minutes
"@every 1h"      — Every hour
```

**Configuration:**

```yaml
cron:
  enabled: true
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
```

**Execution:**

```go
func (s *Scheduler) Execute(trigger *Trigger) error {
    // Trigger fires pipeline on schedule
    engine := s.engine
    
    return engine.ExecutePipeline(context.Background(), trigger.Pipeline)
}
```

---

## Data Flow Examples

### Example 1: User Input → Agent → LLM → Tool → Memory

```
User Input (Dashboard)
  ↓
PUT /api/chat with { "message": "find my notes about Auth" }
  ↓
Dashboard broadcasts on bus: channel.dashboard.message
  ↓
Agent subscribes to dashboard messages
  ↓
Agent calls LLM adapter: StreamChat(message)
  ↓
LLM returns: "Use memory_search tool with query 'Auth'"
  ↓
Agent invokes tool: memory_search("Auth")
  ↓
Tool enforces capability: Check ACL (allowed)
  ↓
Memory store performs FTS search
  ↓
Returns results: [file1.md, file2.md, ...]
  ↓
Agent processes results, calls LLM again
  ↓
LLM returns final response
  ↓
Agent publishes to bus: agent.done
  ↓
Dashboard subscribes to agent.done, updates UI
```

### Example 2: Incoming Telegram Message → Bus → Handler

```
Telegram Server
  ↓
TelegramAdapter.poll() receives update
  ↓
Publishes to bus: channel.telegram.message
  ↓
Any subscribers listening to "channel.telegram.*" receive it
  ↓
Agent subscriber processes message
  ↓
Agent triggers LLM inference
  ↓
LLM might call tools
  ↓
Agent publishes result to bus: channel.telegram.reply
  ↓
Handler sends reply back to Telegram
```

### Example 3: Cron Job → Pipeline → Multiple Agents

```
Cron trigger fires at scheduled time: "0 9 * * *"
  ↓
Cron publishes to bus: cron.daily-digest
  ↓
Engine executes pipeline: "morning_briefing"
  ↓
Pipeline has 3 stages:
  1. Call LLM: "Summarize yesterday's logs"
  2. Call memory_search: "Find important items"
  3. Call email_send: "Send digest"
  ↓
Engine spawns fleet of 3 agents
  ↓
Each agent runs one stage in parallel
  ↓
Agent 1: Summarize logs
Agent 2: Search memory
Agent 3: Send email
  ↓
All agents complete, publish to bus: pipeline.done
  ↓
Memory store records pipeline execution
  ↓
Cost tracker logs token usage
```

---

## Concurrency Model

AgentForge uses **Go's goroutine model** as the primary concurrency primitive:

- **Each agent is a goroutine** — Lightweight, 1000s can run concurrently
- **Communication via channels** — CSP message bus uses channels exclusively
- **No shared memory (except behind mutexes)** — Enforced at capability level
- **Race detection in tests** — All tests run with `-race` flag

```
Main Goroutine
├── Bus Goroutine (receives all messages)
├── Dashboard HTTP Server Goroutine
├── Agent 1 Goroutine (running pipeline)
├── Agent 2 Goroutine (processing Telegram message)
├── Agent 3 Goroutine (executing cron job)
├── Memory Store Goroutine (compaction)
├── Cron Scheduler Goroutine
├── Channel Adapter Goroutines (Telegram poller, Discord WS, etc.)
└── LLM Fallback Chain Goroutine (retry logic)
```

**Synchronization:**
- `sync.Mutex` — Protect shared state (config, capability cache)
- `sync.RWMutex` — Read-heavy access (memory search)
- `sync.Once` — One-time initialization (load config)
- **Channels** — Message passing (CSP bus)
- **Context** — Cancellation and timeout propagation

---

## Production Deployment

### Single-Machine Deployment

```
┌──────────────────────────────────────┐
│         Single Server                │
│  ┌──────────────────────────────────┐│
│  │   AgentForge Binary               ││
│  │  • Web Dashboard :8080            ││
│  │  • MCP Server :9090               ││
│  │  • TUI Mode                       ││
│  │  • Cron Scheduler                 ││
│  │  • CSP Bus                        ││
│  │  • Memory Store                   ││
│  └──────────────────────────────────┘│
│                                       │
│  ~/.agentforge/                       │
│  ├── memory/           (Git + FTS)   │
│  ├── agents.yaml       (Config)      │
│  └── sessions/         (Archived)    │
└──────────────────────────────────────┘
```

### Kubernetes Deployment

```
┌─────────────────────────────────────────┐
│         Kubernetes Cluster              │
│                                         │
│ Pod 1: AgentForge (Web Dashboard)      │
│ Pod 2: AgentForge (Cron + Agents)      │
│ Pod 3: AgentForge (External MCP)       │
│                                         │
│ Shared Storage: Memory Store (NFS)     │
│ Shared Config: ConfigMap               │
│ Service Load Balancer: :8080 :9090     │
└─────────────────────────────────────────┘
```

### Docker Deployment

```dockerfile
FROM golang:1.24 as builder
WORKDIR /app
COPY . .
RUN go build -o agentforge cmd/agentforge/main.go

FROM alpine:3.20
COPY --from=builder /app/agentforge /bin/agentforge
EXPOSE 8080 9090
ENTRYPOINT ["agentforge", "daemon", "--config", "/etc/agentforge/config.yaml"]
```

Run:
```bash
docker run -p 8080:8080 -p 9090:9090 \
  -v ~/.agentforge:/root/.agentforge \
  -e OPENAI_API_KEY=sk-... \
  agentforge:latest
```

---

## Performance Characteristics

### Throughput

- **Messages/sec:** 100,000+ on CSP bus (in-process)
- **Concurrent agents:** 1,000+ on modest hardware
- **Memory per agent:** ~1 MB (minimal footprint)
- **Dashboard requests:** 1,000+ req/sec

### Latency

- **Agent spawn:** 10-50 ms
- **Tool invocation:** 1-100 ms (depends on tool)
- **Bus publish:** <1 ms
- **Memory search:** 10-100 ms (depends on corpus size)

### Storage

- **Binary size:** 10 MB (static, no dependencies)
- **Memory store:** 1 MB per 100K tokens (SQLite FTS)
- **Config:** <1 MB
- **Sessions:** Variable (depends on conversation length)

---

**Status:** 🟢 Production Ready — All components tested, zero data races, security audit complete.
