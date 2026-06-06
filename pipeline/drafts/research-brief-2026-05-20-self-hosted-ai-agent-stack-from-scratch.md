# Research Brief: How to Build a Self-Hosted AI Agent Stack from Scratch

**Date:** 2026-05-20
**Target slug:** self-hosted-ai-agent-stack-from-scratch
**Primary keyword:** self-hosted AI agent stack
**Secondary keywords:** build AI agent from scratch, open source AI agent, self-hosted LLM agent, AI agent infrastructure

---

## 1. GitHub Repos (with star counts and last-updated dates)

| Repository | Stars | Language | Last Updated | Purpose |
|---|---|---|---|---|
| [openclaw/openclaw](https://github.com/openclaw/openclaw) | 372.8k | Python | Active May 2026 | Personal AI assistant framework; fastest-growing OSS AI agent project in history. Cross-platform (WhatsApp, Telegram, Discord, 12+ channels). |
| [langchain-ai/langchain](https://github.com/langchain-ai/langchain) | 123k | Python/TypeScript | Active May 2026 | Comprehensive framework for RAG, tool-augmented LLM apps. Agent workloads now directed to LangGraph. |
| [ollama/ollama](https://github.com/ollama/ollama) | 139k | Go | Active May 2026 | Easiest way to run open-source LLMs locally. Supports 100+ models. |
| [langgenius/dify](https://github.com/langgenius/dify) | 141.7k (Top 51 on GitHub) | TypeScript/Python | v1.14.1 (active May 2026) | Visual agent builder, RAG pipelines, observability. Self-hostable. |
| [crewAIInc/crewAI](https://github.com/crewAIInc/crewAI) | 48.3k | Python | Active May 2026 | Multi-agent orchestration. Role-playing autonomous agents. Independent of LangChain. |
| [langchain-ai/langgraph](https://github.com/langchain-ai/langgraph) | ~44.6k | Python | Active May 2026 | Low-level orchestration for stateful, long-running agents. Durable execution, streaming, human-in-the-loop. |
| [langflow-ai/langflow](https://github.com/langflow-ai/langflow) | 100k+ | Python/TypeScript | Active May 2026 | Visual canvas for LangChain. Handles multi-agent workflows, RAG pipelines. |
| [microsoft/autogen](https://github.com/microsoft/autogen) | 50.4k | Python | Maintenance mode early 2026 | Multi-agent framework. Microsoft moved to maintenance mode, points users to Agent Framework. |
| [letta-ai/letta](https://github.com/letta-ai/letta) | ~13k | Python | Active May 2026 | Stateful agents with persistent memory (formerly MemGPT). $10M seed from Felicis Ventures. |

**Star count sources:**
- OpenClaw: star-history.com/openclaw/openclaw — 372.8k stars, Global Rank #6 (May 2026)
- LangChain: analyticalinsider.ai — 123,000 stars (2026)
- Ollama: dev.to/web_dev-usman — 139k stars (2026)
- Dify: Github-Ranking Top100 — 141,708 stars (rank #51)
- CrewAI: ossinsight.io — 48,296 stars (April 2026)
- LangGraph: dev.to/linou518 — 44.6k stars (2026)
- AutoGen: microsoft/autogen discussion #7066 — 50.4k stars (2026)
- Letta: atlan.com — ~13k+ stars (2026)

---

## 2. Benchmark Numbers

### Local LLM Inference (Ollama)
- **M4 Max MacBook Pro + DeepSeek-R1 70B:** 12 tokens/sec in Q4_K_M quantization (source: dev.to/pooyagolchian, "Local AI in 2026")
- **M4 Pro + 70B models:** 10-25 tokens/sec depending on model size and quantization (source: sitepoint.com/best-local-llm-models-2026)
- **RTX 3060 + 13B models:** ~15-20 tokens/sec (source: LinkedIn benchmarking study by Dmitry Markov)
- **VRAM rule of thumb:** ~0.5 GB VRAM per billion parameters at 4-bit quantization (source: deployhq.com/self-hosting-ai-models)

### Hardware Minimum Requirements (Cloud LLM API route)
- CPU: Any modern processor (Intel i3+ / Apple M1+)
- RAM: 2 GB free
- Storage: 500 MB
- Network: Internet connection
(source: openclaw-ai.net/en/blog/self-hosted-ai-agent-guide)

### Hardware for Local 70B Models
- RAM: 64 GB+ (for Q4_K_M 70B)
- VRAM: 24 GB+ (RTX 4090 / A6000) for full GPU inference
- Alternative: CPU+GPU hybrid via Ollama's automatic layer splitting

---

## 3. Community Pain Points (HN, Reddit)

### From r/LocalLLaMA:
- **"I'm done with using local LLMs for coding"** (r/LocalLLaMA, 2026): User frustration with local LLM quality for coding tasks. "Local LLMs are not cookie cutter solutions yet." Highlights the gap between cloud API quality and local model capability.
- **"What's the Best Local AI Stack for a Complete ChatGPT Replacement?"** (r/LocalLLaMA, 2026): Users struggling to assemble a complete replacement. Pain points: model selection, hardware requirements, integration complexity.
- **"Building a self-hosted AI Knowledge System"** (r/LocalLLaMA, 2026): User built multi-agent system with Letta/MemGPT + Telegram + Neo4j. Complexity was overwhelming. "The space moved fast... got good pushback."
- **"Self-hosted AI coding that just works"** (r/LocalLLaMA, 2026): VSCode + RooCode + LM Studio + Devstral + snowflake-arctic-embed2 + docs-mcp-server. Users want a working stack recipe, not a menu of 50 options.

### From r/selfhosted:
- **"End of Year Self-Hosting Showcase 2025"** (r/selfhosted): Users sharing complete self-hosted setups. Common pain: integration complexity between components (AI models, frontends, automation tools, storage).
- **"What you gonna selfhost in 2025?"** (r/selfhosted): Users already running media stacks (Arr-Stack, Jellyfin) but AI agents are the new frontier. Lack of clear "getting started" guides.

### From Hacker News:
- **"Why we no longer use LangChain for building our AI agents"** (HN): "Most LLM applications require nothing more than string handling, API calls, loops, and maybe a vector DB." Developers frustrated with framework complexity.
- **"Don't trust AI agents"** (HN, 2026): "If you let them run too long, they become self-contradictory." Reliability concerns.
- **"AI agents are starting to eat SaaS"** (HN, 2026): Recognition that self-hosted agents are viable but setup friction remains high.
- **"NIST Seeking Public Comment on AI Agent Security"** (HN, 2026): Security concerns for autonomous agents. NIST involvement signals enterprise-grade concerns.

---

## 4. SaaS Pricing (Alternatives to Self-Hosting)

| Tool | Pricing | What It Replaces |
|---|---|---|
| Dify Cloud | Free tier (200 GPT-4 calls), then $159-590/workspace/month | LangChain + Langflow + custom dev |
| CrewAI Enterprise | $99/month (Team), $60,000/year (Enterprise) | Custom multi-agent development |
| OpenAI API (GPT-4o) | $2.50/1M input tokens, $10/1M output tokens | Local LLM inference |
| Anthropic API (Claude Sonnet 4) | $3/1M input tokens, $15/1M output tokens | Local LLM inference |
| LangSmith (LangChain observability) | $50-200+/month | Self-built monitoring |
| n8n Cloud | $20-55/month | Self-hosted n8n |

**Self-hosted VPS costs:**
- Hetzner CX23 (2 vCPU, 4GB RAM): €3.99/month — sufficient for API-based agents
- Hetzner CX33 (4 vCPU, 8GB RAM): €7.99/month — small local models (7B-13B)
- Hetzner GEX130 (dedicated GPU, Nvidia): €838/month — full local 70B+ inference
- DigitalOcean equivalent: ~2-3x Hetzner pricing

**Cost comparison:** A self-hosted stack using Ollama + OpenClaw + Dify on a Hetzner CX33 costs ~€8/month in infrastructure vs. $200-800/month in equivalent SaaS API costs for a team of 3-5 users.

---

## 5. Primary Sources

1. **OpenClaw GitHub:** github.com/openclaw/openclaw — 372.8k stars, MIT-licensed
2. **LangChain GitHub:** github.com/langchain-ai/langchain — 123k stars
3. **Ollama GitHub:** github.com/ollama/ollama — 139k stars
4. **Dify GitHub:** github.com/langgenius/dify — 141.7k stars, v1.14.1
5. **CrewAI GitHub:** github.com/crewAIInc/crewAI — 48.3k stars
6. **LangGraph GitHub:** github.com/langchain-ai/langgraph — 44.6k stars
7. **Langflow GitHub:** github.com/langflow-ai/langflow — 100k+ stars
8. **AutoGen GitHub:** github.com/microsoft/autogen — 50.4k stars
9. **Letta GitHub:** github.com/letta-ai/letta — ~13k stars
10. **Hetzner GPU Server GEX130:** hetzner.com/pressroom/gpu-server-gex130/ — €838/month
11. **Dify Pricing:** dify.ai/pricing — Free to $590/workspace/month
12. **CrewAI Pricing:** crewai.com/pricing — $99/month to $60,000/year
13. **Ollama benchmarks:** dev.to/pooyagolchian/local-ai-in-2026-running-production-llms
14. **Hardware requirements:** deployhq.com/self-hosting-ai-models-privacy-control-and-performance
15. **ByteByteGo 2026 repo rankings:** blog.bytebytego.com/p/top-ai-github-repositories-in-2026

---

## 6. FAQ Candidates (from real community discussions)

### Q1: "What's the minimum hardware I need to run a self-hosted AI agent?"
From r/LocalLLaMA "What's the Best Local AI Stack for a Complete ChatGPT Replacement?" — Users want to know if they need a GPU or can start with CPU-only.

### Q2: "Can I really replace ChatGPT/Claude with a self-hosted stack?"
From r/LocalLLaMA "Best Local LLMs 2025" and HN "AI agents are starting to eat SaaS" — The core question every builder asks.

### Q3: "Which framework should I use — LangGraph, CrewAI, Dify, or something else?"
From r/LocalLLaMA "AI Developer Tools Map 2026 Edition" and HN "Why we no longer use LangChain" — Framework fatigue is real.

### Q4: "How do I give my agent memory so it remembers past conversations?"
From r/LocalLLaMA "Building a self-hosted AI Knowledge System" — Memory is the #1 feature request for agents that do real work.

### Q5: "Is self-hosting actually cheaper than just paying for API access?"
From HN "AI agents are starting to eat SaaS" and r/selfhosted discussions — Cost is the primary motivator for self-hosting.

### Q6: "How long does it take to set up a working self-hosted agent stack?"
From r/LocalLLaMA "Self-hosted AI coding that just works" — Users want a realistic time estimate, not "5 minutes" marketing claims.

---

## 7. Stack Architecture Options

### Minimal Stack (API-based agents, ~€4/month)
- **LLM:** OpenAI/Anthropic API (pay per token)
- **Agent framework:** OpenClaw (372.8k stars)
- **Frontend:** OpenClaw's built-in channels (Telegram, Discord)
- **Hosting:** Hetzner CX23 (€3.99/month)
- **Setup time:** 2-4 hours

### Mid-tier Stack (local 7B-13B models, ~€8/month)
- **LLM:** Ollama + Qwen2.5-14B or Devstral
- **Agent framework:** LangGraph or CrewAI
- **Frontend:** Open WebUI or Dify
- **Memory:** Letta/MemGPT or simple RAG with ChromaDB
- **Hosting:** Hetzner CX33 (€7.99/month)
- **Setup time:** 4-8 hours

### Full Stack (local 70B models, ~€838/month)
- **LLM:** Ollama + DeepSeek-R1-70B (Q4_K_M)
- **Agent framework:** LangGraph + CrewAI
- **Frontend:** Dify (visual builder)
- **Memory:** Letta with persistent storage
- **Hosting:** Hetzner GEX130 (€838/month, dedicated GPU)
- **Setup time:** 8-16 hours

---

## 8. Key Trends (2026)

1. **OpenClaw's explosive growth:** 372.8k stars in ~4 months, making it the most-starred non-aggregator software project on GitHub (source: thenewstack.io/openclaw-github-stars-security/)
2. **AutoGen in maintenance mode:** Microsoft moved AutoGen to maintenance in early 2026, pointing users to the Microsoft Agent Framework (source: futureagi.com/blog/what-is-autogen-2026/)
3. **LangChain → LangGraph migration:** LangChain directs agent workloads to LangGraph while keeping LangChain for chains/RAG (source: analyticalinsider.ai)
4. **Visual builders gaining ground:** Dify (141.7k stars) and Langflow (100k+ stars) growing faster than code-first frameworks
5. **Memory as the differentiator:** Letta (formerly MemGPT) raised $10M to solve agent memory — the #1 unsolved problem in self-hosted agents
6. **Security concerns escalating:** NIST seeking public comment on AI agent security (source: HN item #47131689)
