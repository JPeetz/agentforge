# AgentForge User Guide

Complete guide to using AgentForge for autonomous agent orchestration.

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Core Concepts](#core-concepts)
3. [Spawning Agents](#spawning-agents)
4. [Agent Capabilities](#agent-capabilities)
5. [Memory & Persistence](#memory--persistence)
6. [Pipelines & Workflows](#pipelines--workflows)
7. [Dashboard & Web UI](#dashboard--web-ui)
8. [Monitoring & Logging](#monitoring--logging)
9. [Troubleshooting](#troubleshooting)

---

## Quick Start

### Install AgentForge

```bash
# Homebrew (macOS)
brew install agentforge/tap/agentforge

# Go install
go install github.com/agentforge/agentforge/cmd/agentforge@latest

# Docker
docker run -p 8080:8080 agentforge/agentforge:latest
```

### Start the Daemon

```bash
agentforge daemon --config ~/.agentforge/config.yaml
```

Access the dashboard: http://localhost:8080

### Spawn Your First Agent

```bash
agentforge spawn my-first-agent \
  --fs-allow "$HOME/work/**" \
  --domain-allow "api.openai.com" \
  --token-budget 200000
```

### Chat with Your Agent

Open http://localhost:8080 → Chat tab → Type message → Send

---

## Core Concepts

### 1. Agents

**What:** Autonomous Go goroutines running LLM inference loops.

**Key Properties:**
- **ID:** Unique identifier (e.g., `content-writer-1`)
- **Model:** LLM provider + model (e.g., `openai:gpt-4.1`)
- **Capability Token:** HMAC-signed permissions (files, domains, budget, timeout)
- **Session:** Conversation history + token tracking
- **Department:** Logical grouping (content, seo, social, etc.)

**Lifecycle:**
```
spawned → ready → running (in loop) → done
  ↓
Every iteration:
1. Receive message from user/bus
2. Call LLM with context
3. Parse tool calls
4. Invoke tools (with capability check)
5. Update memory
6. Publish events to bus
7. Repeat until done
```

### 2. Capability Tokens

**Purpose:** Zero-trust security for agents.

**What You Specify:**
```bash
agentforge spawn agent-name \
  --fs-allow "/home/user/**"           # Can only read/write these paths
  --domain-allow "api.openai.com"      # Can only call these domains
  --token-budget 500000                # Max 500K tokens per session
  --timeout 1800                       # Max 30 minutes
```

**Enforcement:**
Every tool call passes through capability enforcer:
```
Tool Call Request
  ↓
Enforcer.Check(capability, action)
  ↓
Is action allowed? → YES → Execute tool
                   → NO  → Return error "Path not allowed"
```

### 3. Message Bus (CSP)

**Purpose:** Inter-agent communication and event broadcasting.

**Topics:**
- `agent.*` — Agent lifecycle (spawn, ready, done, error)
- `channel.*` — Incoming messages (telegram.message, discord.message)
- `llm.*` — LLM events (token_usage, error)
- `memory.*` — Memory operations
- `tool.*` — Tool invocations
- `pipeline.*` — Pipeline execution

**Example: Subscribing to Agent Events**

In your agent's instructions:
```
When you're done writing an article:
1. Save to memory
2. Publish to bus topic: "article.written"
3. Other agents can subscribe to "article.written" 
   and automatically format/publish it
```

### 4. Memory Store (MeMex RAG)

**What:** Persistent, searchable memory with Git versioning.

**Storage:**
```
~/.agentforge/memory/
├── articles/
│   ├── go-concurrency.md
│   ├── python-tips.md
│   └── ...
├── projects/
│   ├── project-a/
│   └── project-b/
└── .git/  (version control)
```

**Capabilities:**
- Full-text search (SQLite FTS5)
- Git history (retrieve old versions)
- Auto-commit on changes
- Cross-agent access (shared knowledge)

**Usage:**
```
In your agent's instructions:
"Before writing, search memory for similar articles using memory_search tool.
After writing, save to memory using memory_write tool.
This builds a searchable knowledge base over time."
```

---

## Spawning Agents

### Basic Spawn

```bash
agentforge spawn my-agent
```

Creates agent with defaults:
- Model: configured default (usually gpt-4.1)
- Filesystem: home directory only
- Network: openai.com and anthropic.com
- Budget: 1M tokens
- Timeout: 1 hour

### Custom Spawn with Full Permissions

```bash
agentforge spawn content-writer \
  --department content \
  --model openai:gpt-4.1 \
  --fs-allow "/home/user/articles/**" \
  --fs-allow "/home/user/media/**" \
  --domain-allow "api.openai.com" \
  --domain-allow "github.com" \
  --domain-allow "wikipedia.org" \
  --token-budget 500000 \
  --timeout 1800
```

### Spawn as YAML Config

```yaml
# config.yaml
agents:
  content-writer:
    model: "openai:gpt-4.1"
    department: "content"
    
    filesystem_acl:
      - "/home/user/articles/**"
      - "/home/user/media/**"
    
    network_acl:
      - "api.openai.com"
      - "github.com"
      - "wikipedia.org"
    
    token_budget: 500000
    timeout: 1800s
    
    instructions: |
      You are a professional content writer.
      - Research thoroughly using web_search
      - Save articles using file_write
      - Search memory for related articles
      - Cite sources in markdown format
```

Then:
```bash
agentforge daemon --config config.yaml
```

---

## Agent Capabilities

### What Tools Can Agents Use?

AgentForge includes **19 built-in tools**:

#### File I/O (Filesystem ACL Protected)
- `file_read` — Read files (JSON, markdown, text, code)
- `file_write` — Write/create files
- `file_list` — List directory contents
- `file_delete` — Delete files
- `file_mkdir` — Create directories

#### Network (Domain ACL Protected)
- `web_fetch` — GET/POST HTTP requests
- `web_search` — Google search integration

#### Memory
- `memory_search` — Full-text search with relevance scoring
- `memory_read` — Read specific memory file
- `memory_write` — Save to memory store
- `memory_list` — List memory files

#### LLM Integration
- `chat` — Call LLM directly
- `embedding` — Generate embeddings for semantic search

#### Content Generation
- `image_generate` — DALL-E, Stability AI
- `video_generate` — Runway, Synthesia
- `diagram_make` — Mermaid, PlantUML diagrams
- `music_generate` — AI music generation

#### Development
- `shell_exec` — Execute shell commands (with arg safety)
- `code_review` — Automated code analysis
- `api_design` — REST API specification
- `data_analysis` — Statistical analysis

#### Skills & Extensions
- `skill_search` — Find skills from marketplace
- `skill_install` — Install new skills
- `mcp_bridge` — Connect to external MCP servers

### Capability ACL Examples

#### Restrictive (Least Privilege)
```bash
# For a trusted worker agent
agentforge spawn worker \
  --fs-allow "/home/user/work/**" \
  --domain-allow "api.openai.com" \
  --token-budget 100000 \
  --timeout 300
```
Result: Can only:
- Read/write in `/home/user/work/`
- Call OpenAI API
- Use 100K tokens max
- Run for max 5 minutes

#### Moderate (Development)
```bash
# For content creation
agentforge spawn content-creator \
  --fs-allow "/home/user/content/**" \
  --fs-allow "/home/user/media/**" \
  --domain-allow "api.openai.com" \
  --domain-allow "github.com" \
  --domain-allow "*.wikipedia.org" \
  --token-budget 500000 \
  --timeout 1800
```
Result: Can:
- Read/write content and media
- Call OpenAI
- Research on GitHub and Wikipedia
- Use 500K tokens, 30 minutes max

#### Permissive (Research/Experimentation)
```bash
# For full-powered research agent (still safe)
agentforge spawn researcher \
  --fs-allow "/home/user/**" \
  --domain-allow "api.openai.com" \
  --domain-allow "api.anthropic.com" \
  --domain-allow "*.com" \
  --token-budget 1000000 \
  --timeout 3600
```
Result: Can:
- Access full home directory
- Call all LLM providers
- Research any website
- Use 1M tokens, 1 hour max
- Still: Cannot access `/etc/`, other users' files, or system commands

---

## Memory & Persistence

### Auto-Commit to Memory

Agents automatically save work:
```
Agent creates file: /home/user/articles/post.md
  ↓
Agent calls: memory_write("/articles/post.md")
  ↓
Memory store saves file
  ↓
Git auto-commits with message:
  "Article: post.md (by content-writer agent)"
  ↓
Searchable via memory_search
```

### Searching Memory

```
User asks agent: "Have you written about Go before?"

Agent calls: memory_search("Go concurrency")
  ↓
Returns: [
  {
    "file": "articles/go-patterns.md",
    "relevance": 0.95,
    "snippet": "Go's concurrency model..."
  },
  {
    "file": "articles/go-performance.md",
    "relevance": 0.87,
    "snippet": "Performance optimization in Go..."
  }
]
  ↓
Agent cites previous work + avoids duplication
```

### Session Compaction

**Automatic when:**
- Session reaches 90% of model's context window
- Example: GPT-4.1 has 1M tokens → compact at 900K tokens

**What happens:**
```
Old turns (1-100): Summarized by LLM
New turns (101-current): Kept full
  ↓
Summary saved to memory
Full history persisted
  ↓
Session continues with compressed context
```

---

## Pipelines & Workflows

### What is a Pipeline?

A **DAG (directed acyclic graph)** of stages that agents execute sequentially.

### Example: Blog Publishing Pipeline

```yaml
pipelines:
  publish-blog-post:
    description: "Research, write, review, publish blog post"
    
    stages:
      research:
        agent: "researcher"
        tool: "web_search"
        inputs:
          query: "{{ topic }}"
        outputs:
          - research_findings
      
      write:
        agent: "content-writer"
        depends_on:
          - research
        tool: "file_write"
        inputs:
          findings: "{{ research.findings }}"
          title: "{{ topic }}"
        outputs:
          - article_file
      
      review:
        agent: "editor"
        depends_on:
          - write
        tool: "code_review"
        inputs:
          file: "{{ write.article_file }}"
        outputs:
          - feedback
      
      publish:
        agent: "publisher"
        depends_on:
          - review
        tool: "shell_exec"
        inputs:
          command: "publish.sh {{ write.article_file }}"
        outputs:
          - published_url
```

### Run Pipeline

```bash
agentforge run-pipeline publish-blog-post \
  --topic "Go Concurrency Patterns"
```

**Execution Flow:**
```
Stage 1 (Research):   researcher → web_search → finds sources
Stage 2 (Write):      content-writer → file_write → draft article
Stage 3 (Review):     editor → code_review → finds issues
Stage 4 (Publish):    publisher → shell_exec → publish live
  ↓
All work saved to memory
Final result: published URL
```

### Parallel Execution

Stages without dependencies run in parallel:

```yaml
stages:
  research:
    outputs: [findings]
  
  write:
    depends_on: [research]
  
  design-graphics:      # No dependency = runs parallel with write
    agent: "designer"
    tool: "image_generate"
  
  publish:
    depends_on: [write, design-graphics]  # Waits for both
```

---

## Dashboard & Web UI

### Access Dashboard

```
http://localhost:8080
```

### Main Pages

#### 1. **Overview** (System Health)
- Active agents count
- Memory usage
- Token usage & cost
- Recent events
- Uptime

#### 2. **Agent Fleet** (Manage Agents)
- List all agents
- Create new agent
- Edit agent capabilities
- Monitor agent status
- View agent logs

#### 3. **Memory Browser** (Search & Manage)
- Full-text search
- Browse files
- View version history
- Search by agent/date
- Export memory

#### 4. **Pipelines** (Visual DAG Editor)
- Create pipelines visually
- Drag-drop stages
- Configure dependencies
- Run pipelines
- Monitor execution

#### 5. **Chat** (Real-time Chat with Agents)
- Chat with agents via SSE streaming
- Real-time tool call visualization
- File upload (📎 paperclip)
- Copy code blocks (💾)
- Markdown rendering

#### 6. **Security** (Audit & Compliance)
- Capability enforcement logs
- ACL violations
- Token budget tracking
- Timeline of all security events

#### 7. **Settings** (Configuration)
- LLM provider config
- API keys
- Agent defaults
- Channel settings (Telegram, Discord)
- Memory store location

---

## Monitoring & Logging

### View Agent Logs

```bash
# Last 20 log entries
agentforge logs my-agent --tail 20

# Follow in real-time
agentforge logs my-agent -f

# By time range
agentforge logs my-agent --since "2026-06-01" --until "2026-06-03"

# By level
agentforge logs --level error    # Only errors
agentforge logs --level debug    # Verbose
```

### Agent Status

```bash
# List all agents
agentforge agents list

# Get specific agent details
agentforge agents get my-agent

# Monitor live (updates every 5 seconds)
agentforge agents watch
```

### Metrics & Monitoring

Enable Prometheus metrics:
```yaml
daemon:
  metrics: true
```

Then scrape: `http://localhost:8080/metrics`

Key metrics:
- `agentforge_agents_active` — Running agents
- `agentforge_tokens_consumed` — Total tokens used
- `agentforge_cost_total_usd` — Total spend
- `agentforge_memory_size_bytes` — Memory store size
- `agentforge_tool_calls_total` — Tool invocations

---

## Troubleshooting

### Agent Won't Spawn

**Problem:** `error: "path not allowed"`

**Solution:** Update filesystem ACL
```bash
agentforge update my-agent \
  --fs-allow "/new/path/**"
```

### Tool Call Rejected

**Problem:** Agent tries to call tool, gets "capability denied"

**Cause:** Path/domain not in ACL

**Solution:** Check agent's capabilities
```bash
agentforge agents get my-agent | grep -A5 acl
```

Then update:
```bash
agentforge update my-agent \
  --domain-allow "api.new-service.com"
```

### Memory Search Slow

**Problem:** `memory_search` takes >5 seconds

**Solution:** Rebuild index
```bash
agentforge memory compact
```

Or check memory size:
```bash
du -sh ~/.agentforge/memory/
```

If >10GB, archive old sessions.

### Agent Stuck or Hanging

**Problem:** Agent doesn't respond, seems hung

**Solution:** Kill the agent (timeout will end it, but to force):
```bash
agentforge kill my-agent
```

Then re-spawn:
```bash
agentforge spawn my-agent ...
```

Check logs for what caused it:
```bash
agentforge logs my-agent --tail 50 | grep -i error
```

### High Cost / Token Usage

**Problem:** Agent consumed tokens unexpectedly

**Solution:**

1. Check which agent:
```bash
agentforge logs --level error | grep -i token
```

2. Lower budget:
```bash
agentforge update my-agent --token-budget 100000
```

3. Set shorter timeout:
```bash
agentforge update my-agent --timeout 300  # 5 minutes max
```

4. Monitor in real-time:
```bash
agentforge metrics | grep cost
```

---

## Best Practices

### 1. Use Minimal Capabilities
```bash
# BAD: Full access
agentforge spawn agent --fs-allow "/" --domain-allow "*"

# GOOD: Only what's needed
agentforge spawn agent \
  --fs-allow "/home/user/work/**" \
  --domain-allow "api.openai.com"
```

### 2. Save Work to Memory
```
Every agent should:
- Call memory_search before starting
- Call memory_write when done
- This builds institutional knowledge
```

### 3. Monitor Costs
```bash
# Check daily costs
agentforge cost --period day

# Set budget alerts
agentforge alerts add --type cost --threshold 50 --email me@example.com
```

### 4. Version Your Pipelines
```bash
# Export current pipeline
agentforge pipelines export my-pipeline > pipeline-v1.yaml

# Save in git for version control
git add pipelines/ && git commit -m "Pipeline v1.1: added review stage"
```

### 5. Rotate Agent Credentials
```bash
# Capability tokens should expire
# Rotate them monthly
agentforge rotate-token my-agent
```

---

## Getting Help

- **Documentation:** https://docs.agentforge.dev
- **Examples:** See `/examples/` directory
- **Discord:** https://discord.gg/agentforge
- **Issues:** https://github.com/agentforge/agentforge/issues

---

**AgentForge: Agents that work. Security you trust. Scale you need.**
