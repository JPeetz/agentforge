# Swarm Blueprint — AgentForge Department Swarms

*Mapping OpenSwarm's specialist-team pattern onto AgentForge's existing infrastructure.*

---

## Architecture

Every AgentForge swarm runs on the existing infrastructure:

```
internal/swarm/      ← NEW (composes existing modules)
  ├── swarm.go       Swarm runtime: spawn, route, collect
  ├── config.go      SwarmConfig: YAML/JSON definition
  ├── orchestrator.go Orchestrator agent: pure router
  ├── builder.go     SwarmBuilder: fromConfig / fromPrompt / fromDepartment
  └── swarm_test.go

Uses (already built):
  ├── internal/engine/    Agent pools, DAG pipelines, subagent trees, streaming
  ├── internal/bus/       CSP pub/sub for inter-agent routing
  ├── internal/security/  OCap tokens, derivation, glob scoping
  ├── internal/tool/      Registry, per-agent filtering
  ├── internal/learn/     Self-learning from swarm interactions
  └── internal/memory/    Git-tracked FTS5 shared context per swarm
```

**Key principle:** The swarm doesn't replace anything. It's a higher-level orchestration layer that composes existing primitives.

---

## Existing Departments → Blueprint Swarms

Each existing department becomes a specialist team:

### 1. Content Swarm
```
User: "Write an article about AI security"
        │
        ▼
┌─────────────────────────────────────────────┐
│  🧭 Orchestrator                            │
│  Routes to specialists, collects results    │
└─────────────────────────────────────────────┘
        │
        ├──→ 🔍 SEO Researcher
        │    Tools: web_search, web_fetch
        │    Output: keyword brief, competitor gaps
        │
        ├──→ ✍️ Content Writer
        │    Tools: filesystem, search
        │    Input: research brief
        │    Output: article draft
        │
        ├──→ ✏️ Editor
        │    Tools: filesystem
        │    Input: article draft
        │    Output: polished article
        │
        ├──→ 🖼️ Image Generator
        │    Tools: image_generate
        │    Input: article context
        │    Output: featured image
        │
        └──→ 📤 Publisher
             Tools: http, filesystem
             Input: article + image
             Output: published URL
```

### 2. SEO Swarm
```
Orchestrator → Keyword Researcher, Competitor Analyst,
               Technical Auditor, Content Optimizer, Rank Tracker
```

### 3. Social Swarm
```
Orchestrator → Copywriter, Image Creator, Scheduler,
               Engagement Monitor, Analytics Reporter
```

### 4. Prompts Swarm
```
Orchestrator → Prompt Researcher, Prompt Optimizer,
               Bundle Curator, Model Tester
```

### 5. Analytics Swarm
```
Orchestrator → Data Collector, Data Analyst (IPython kernel),
               Chart Generator, Report Writer
```

### 6. Hiring Swarm
```
Orchestrator → Agent Designer, Adversarial Reviewer,
               Agent Tester, Onboarder
```

### 7. PDF / Design Swarm
```
Orchestrator → Document Builder, Template Designer,
               Lead Magnet Generator, Export Handler
```

---

## New Swarms (OpenSwarm-Inspired)

### 8. Development Swarm
```
User: "Add user authentication to the API"
        │
┌─────────────────────────────────────────────┐
│  🧭 Orchestrator                            │
└─────────────────────────────────────────────┘
        │
        ├──→ 🏗️ Architect
        │    Analyzes existing codebase, designs approach
        │    Tools: filesystem, shell (git grep/diff)
        │    Output: design doc + file plan
        │
        ├──→ 👨‍💻 Coder
        │    Writes implementation
        │    Tools: filesystem, shell (go/rust/python)
        │    Input: design doc
        │    Output: code changes
        │
        ├──→ 🔍 Reviewer
        │    Security audit, code quality
        │    Tools: filesystem, shell (go vet, clippy, pylint)
        │    Output: review comments
        │
        ├──→ 🧪 Tester
        │    Writes and runs tests
        │    Tools: filesystem, shell (go test, pytest, cargo test)
        │    Output: test results
        │
        └──→ 🚀 DevOps
             CI/CD, deployment
             Tools: shell (docker, git push, helm)
             Output: deployed
```

### 9. Accounting & Tax Swarm
```
User: "Prepare Q2 tax report from my receipts and invoices"
        │
┌─────────────────────────────────────────────┐
│  🧭 Orchestrator                            │
└─────────────────────────────────────────────┘
        │
        ├──→ 📄 Receipt Parser
        │    OCR + categorize receipts/invoices
        │    Tools: filesystem, web_search (tax rules lookup)
        │
        ├──→ 📊 Transaction Categorizer
        │    Classify income/expenses by tax category
        │    Tools: filesystem, shell (csv parsing)
        │
        ├──→ 🧮 Tax Calculator
        │    Apply rules, compute liability
        │    Tools: shell (Python kernel for calculations)
        │
        ├──→ 📝 Report Generator
        │    Create formatted tax reports (PDF/DOCX)
        │    Tools: filesystem
        │
        └──→ ✅ Compliance Checker
             Verify against latest regulations
             Tools: web_search, web_fetch (gov portals)
```

### 10. Executive Assistant Swarm
```
User: "Sort my inbox, prep for tomorrow's meetings, and book a flight"
        │
┌─────────────────────────────────────────────┐
│  🧭 Orchestrator                            │
└─────────────────────────────────────────────┘
        │
        ├──→ 📧 Email Handler
        │    Triage, draft replies, flag urgent
        │    Tools: http (Gmail API via MCP), filesystem
        │
        ├──→ 📅 Calendar Manager
        │    Schedule, reschedule, conflict resolution
        │    Tools: http (Google Calendar via MCP)
        │
        ├──→ 🔬 Research Assistant
        │    Web research with citations
        │    Tools: web_search, web_fetch
        │
        ├──→ ✈️ Travel Planner
        │    Flight search, hotel booking, itinerary
        │    Tools: web_search, web_fetch
        │
        └──→ 📋 Daily Briefer
             Morning briefing: weather, calendar, news, tasks
             Tools: web_search, web_fetch, weather API
```

### 11. Marketing Swarm
```
Orchestrator → Campaign Strategist, Copywriter,
               Visual Designer, Analytics Tracker, A/B Tester
```

### 12. Legal & Compliance Swarm
```
Orchestrator → Contract Reviewer, GDPR Checker,
               Terms Generator, Policy Writer, Research Agent
```

### 13. Research Swarm (Deep Research)
```
Orchestrator → Literature Searcher, Paper Summarizer,
               Citation Tracker, Synthesis Writer, Fact Checker
```

---

## Cross-Department Chaining

Swarms can hook into each other through the CSP bus:

```
Content Pipeline:
  SEO Swarm → Content Swarm → Social Swarm → Analytics Swarm
  (keywords)  (articles)       (distribution)  (metrics)

Data-Driven Content:
  Analytics Swarm → Content Swarm
  (identify gaps)   (write to fill gaps)

Agent Creation Chain:
  Hiring Swarm → Any Department
  (builds agents)  (deploys into swarm)

Financial Intelligence:
  Accounting Swarm → Analytics Swarm → Executive Swarm
  (categorize)       (visualize)        (report to board)
```

---

## SwarmBuilder — Prompt to Swarm

The key innovation: users create swarms from natural language.

```
> agentforge swarm create "Build me an accounting team that can
  process receipts, categorize expenses, and prepare quarterly
  tax reports for an Irish sole trader"

→ SwarmBuilder calls LLM to generate swarm config
→ AgentForge validates the config
→ Spawns: Receipt Parser, Transaction Categorizer,
          Tax Calculator, Report Generator, Compliance Checker
→ Each gets capability-scoped tools via OCap
→ Registers on bus as "accounting" department
→ Available immediately via dashboard, chat, or API
```

**SwarmConfig (generated by builder):**
```yaml
swarms:
  accounting:
    orchestrator:
      model: openrouter/deepseek-v4-pro
      system_prompt: |
        You are the Accounting Department Orchestrator.
        Route financial requests to specialists.
        Never perform calculations or categorize transactions yourself.
        Always delegate to the appropriate specialist.
    specialists:
      - id: receipt-parser
        name: Receipt Parser
        model: openrouter/anthropic/claude-sonnet-4.5
        tools: [filesystem]
        capability: [read, write]
        prompt: "Extract amounts, dates, vendors, and categories from receipts and invoices."

      - id: transaction-categorizer
        name: Transaction Categorizer
        model: openrouter/anthropic/claude-sonnet-4.5
        tools: [filesystem, shell]
        capability: [read, write, exec]
        prompt: "Classify transactions into Irish tax categories (IT, CT, VAT applicable)."

      - id: tax-calculator
        name: Tax Calculator
        model: openrouter/anthropic/claude-sonnet-4.5
        tools: [shell, filesystem]
        capability: [read, write, exec]
        prompt: "Calculate tax obligations using Irish Revenue rules. Use Python for computations."

      - id: report-generator
        name: Report Generator
        model: openrouter/anthropic/claude-sonnet-4.5
        tools: [filesystem]
        capability: [read, write]
        prompt: "Format financial data into professional reports suitable for tax filing."

      - id: compliance-checker
        name: Compliance Checker
        model: openrouter/anthropic/claude-sonnet-4.5
        tools: [web_search, web_fetch]
        capability: [read, net]
        prompt: "Verify tax computations against current Irish Revenue guidelines."

    routes:
      - intent: "process receipt"
        target: receipt-parser
      - intent: "categorize expenses"
        target: transaction-categorizer
      - intent: "calculate tax"
        target: tax-calculator
      - intent: "generate report"
        target: report-generator
      - intent: "check compliance"
        target: compliance-checker
      - intent: "tax report"
        pipeline: [receipt-parser, transaction-categorizer, tax-calculator, compliance-checker, report-generator]
```

---

## Dashboard Integration

New pages:
- **Swarm Builder** — Visual swarm designer (drag agents → connect → configure tools)
- **Swarm Marketplace** — Share & discover swarm templates
- **Swarm Activity** — Live view of agent coordination (who routed to whom)

New APIs:
- `POST /api/swarms/build` — Prompt → swarm config (LLM-generated)
- `POST /api/swarms/deploy` — Deploy a swarm
- `GET /api/swarms/:name/status` — Live swarm status
- `POST /api/swarms/:name/route` — Route a request to a swarm

---

## Strategic Positioning

| Capability | OpenSwarm | AgentForge + Swarms |
|---|---|---|
| Multi-agent orchestration | ✅ | ✅ |
| Prompt-to-swarm creation | ✅ Agent Builder | ✅ SwarmBuilder |
| Specialist agents | 8 hardcoded | Unlimited, configurable |
| Security model | ❌ Shared process | ✅ OCap per agent |
| Memory | ❌ Session only | ✅ Git FTS5 + dedup |
| Self-learning | ❌ | ✅ Auto-generates skills |
| Self-hosted | ✅ | ✅ Single binary |
| UI | Terminal only | ✅ Dashboard + Terminal |
| Cross-swarm chaining | ❌ | ✅ CSP bus routing |
| Capability scoping | ❌ | ✅ Glob-scoped resources |
| Channels | None | ✅ Slack/Signal/WhatsApp/Matrix |
| Deployment | npm + Python | ✅ `./agentforge` — one file |

---

## Implementation Order

1. `internal/swarm/config.go` — SwarmConfig struct + YAML loader + validation
2. `internal/swarm/orchestrator.go` — Orchestrator agent (pure router, CSP-based)
3. `internal/swarm/swarm.go` — Swarm runtime: spawn agents from config, connect to bus
4. `internal/swarm/builder.go` — SwarmBuilder: fromPrompt → LLM → config → deploy
5. Dashboard page: Swarm Builder UI + API endpoints
6. Swarm Marketplace: shareable swarm templates
7. Swarm-to-swarm chaining via DAG pipelines