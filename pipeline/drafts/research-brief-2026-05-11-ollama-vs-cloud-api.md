# Research Brief: Ollama vs Cloud APIs — When Local LLMs Make Financial Sense

**Date:** 2026-05-11
**Slug:** ollama-vs-cloud-api-local-llm-cost-analysis
**Topic Angle:** Cost-benefit analysis with real benchmarks — when does self-hosting beat paying per-token?

---

## 1. GitHub Repos (with star counts, last-updated)

| Repo | Stars | Forks | Last Release | Notes |
|------|-------|-------|-------------|-------|
| [ollama/ollama](https://github.com/ollama/ollama) | 171k | 16.1k | v0.23.2 (May 7, 2026) | Go-based, wraps llama.cpp. 602 contributors. MIT license. |
| [ggml-org/llama.cpp](https://github.com/ggml-org/llama.cpp) | 109k | 18.1k | Active (May 2026) | C/C++ inference engine. 1,661 contributors. Foundation for Ollama. |
| [vllm-project/vllm](https://github.com/vllm-project/vllm) | 79.6k | 16.6k | v0.20.2 (May 10, 2026) | Python, PagedAttention. 2,572+ contributors. Apache-2.0. Used by 8k+ projects. |
| [mudler/LocalAI](https://github.com/mudler/LocalAI) | 46.2k | 4.1k | Active (May 2026) | OpenAI-compatible API drop-in. 36+ backends. MIT license. |

---

## 2. Benchmark Numbers (Specific Performance Claims)

### Throughput: Ollama vs vLLM vs llama.cpp

| Tool | Hardware | Model | Throughput | Source |
|------|----------|-------|------------|--------|
| Ollama | RTX 4090 | Llama 3.1 8B Q4_K_M | ~62 tok/s (single-user) | SitePoint, March 2026 |
| vLLM | RTX 4090 | Llama 3.1 8B FP16 | ~71 tok/s (single-user) | SitePoint, March 2026 |
| Ollama | RTX 4090 | Llama 3.1 8B | ~155 tok/s aggregate (50 users) | SitePoint, March 2026 |
| vLLM | RTX 4090 | Llama 3.1 8B | ~920 tok/s aggregate (50 users) | SitePoint, March 2026 |
| vLLM | RTX 5090 | Qwen2.5-Coder-7B | **5,841 tok/s** (batch=8, 1024 ctx) | DecodesFuture, Jan 2026 |
| llama.cpp | M4 Ultra | Llama 3.1 8B | ~150 tok/s | DecodesFuture, Jan 2026 |
| Ollama | M4 Ultra | Llama 3.1 8B | ~35-41 tok/s | DecodesFuture, Jan 2026 |
| llama.cpp | M4 Pro 48GB | Qwen 2.5 (Q4) | 32-38 tok/s | Contracollective, 2026 |
| Ollama | M3 Max 128GB | 70B Q4_K_M | ~8 tok/s | SitePoint, March 2026 |
| EXO Labs | M4 Pro cluster | Nemotron 70B | 4-8 tok/s | Julien Simon, April 2026 |
| EXO Labs | M4 Pro cluster | Qwen2.5-Coder-32B | 18 tok/s | Julien Simon, April 2026 |

### Latency at Scale (50 concurrent users, RTX 4090)

| Metric | Ollama | vLLM |
|--------|--------|------|
| p99 latency | ~24.7s | ~2.8s |
| TTFT stability | Degrades under load | Stable <100ms |

### Memory Requirements (VRAM)

| Model Size | Q4 Quantization | FP16 | Example GPU |
|------------|-----------------|------|-------------|
| 7-8B | 4-6GB | 16GB | RTX 3060/4060 |
| 13B | 8-10GB | 26GB | RTX 4060 Ti 16GB |
| 32-34B | 16-20GB | 64GB | RTX 4090 |
| 70B | 35-40GB | 140GB | Dual RTX 4090 / A100 |

---

## 3. Cloud API Pricing (Per 1M Tokens, May 2026)

| Model | Provider | Input | Output | Context |
|-------|----------|-------|--------|---------|
| GPT-5.2 | OpenAI | $1.75 | $14.00 | 128K |
| GPT-5 Mini | OpenAI | $0.25 | $2.00 | 128K |
| o4-mini | OpenAI | $1.10 | $4.40 | 200K |
| Claude Opus 4.7 | Anthropic | $5.00 | $25.00 | 1M |
| Claude Haiku 4.5 | Anthropic | $1.00 | $5.00 | 200K |
| Gemini 3.1 Pro | Google | $2.00 | $12.00 | 2M |
| Gemini 3 Flash | Google | $0.50 | $3.00 | 1M |
| DeepSeek V3.2 | DeepSeek | $0.28 | $0.42 | 128K |
| Grok 4.1 Fast | x.ai | $0.20 | $0.50 | 2M |
| Llama 4 Scout | Groq | $0.11 | $0.34 | 128K |

### Cost at Scale (3:1 input:output ratio)

| Model | 1M tokens | 10M tokens | 100M tokens |
|-------|-----------|------------|-------------|
| GPT-5.2 | $4.81 | $48.13 | $481.25 |
| Claude Opus 4.7 | $10.00 | $100.00 | $1,000.00 |
| GPT-5 Mini | $0.69 | $6.88 | $68.75 |
| DeepSeek V3.2 | $0.32 | $3.15 | $31.50 |
| Llama 4 Scout (Groq) | $0.17 | $1.68 | $16.75 |

---

## 4. Self-Hosted Hardware Costs

| Hardware | Approx. Cost | VRAM | Amortized/mo (3yr) |
|----------|-------------|------|---------------------|
| RTX 4090 | $1,599-2,000 | 24GB | ~$45-55/mo |
| RTX 5090 | ~$2,000+ | 32GB | ~$55-65/mo |
| MacBook M4 Pro 48GB | ~$3,000 | 48GB unified | ~$83/mo |
| MacBook M4 Ultra | ~$5,000+ | 192GB unified | ~$140/mo |
| Dual RTX 4090 server | ~$5,000-8,000 | 48GB | ~$140-220/mo |
| Cloud RTX 4090 (hourly) | $0.44-1.61/hr | 24GB | ~$320-1,160/mo (24/7) |

### Break-Even Analysis

- **RTX 4090** at 100M tokens/mo: ~$50/mo hardware vs $481/mo GPT-5.2 → **break-even at ~10M tokens/mo**
- **RTX 4090** at 100M tokens/mo: ~$50/mo hardware vs $31.50/mo DeepSeek V3.2 → **cloud is cheaper at this tier**
- **MacBook M4 Pro 48GB** at 50M tokens/mo: ~$83/mo hardware vs $240/mo GPT-5.2 → **break-even at ~17M tokens/mo**

---

## 5. Community Pain Points (HN, Reddit r/LocalLLaMA, r/selfhosted)

### From Reddit r/LocalLLaMA:
- **"API pricing is in freefall"** — Users note API costs dropping 50% every few months, making local hosting less compelling for cost alone. The case for local is now privacy, latency, and control, not just savings.
- **"Replacing $200/mo Cursor subscription"** — Users replacing expensive AI coding tools with local Ollama + selective API calls for complex tasks.
- **"Ollama Cloud $100/mo limits are insane"** — Power users hitting cloud subscription limits, considering self-hosting instead.
- **"Best Claude Code / OpenCode alternatives"** — Users seeking free, self-hosted alternatives to paid AI coding assistants.

### From Hacker News:
- **"Self-hosting an AI with your own hardware is probably just as cost-prohibitive"** — HN commenters noting that when you factor in time, power, and hardware depreciation, self-hosting isn't automatically cheaper.
- **"LLMs are cheap"** — Debate about whether cloud APIs are loss-leaders, making true cost comparison difficult.
- **"Running a remote cloud LLM costs about nothing"** — Counter-argument that for low-volume users, API costs are negligible.

### Key Pain Points Summary:
1. **API pricing volatility** — Prices dropping unpredictably, making long-term cost planning hard
2. **Data privacy** — Healthcare, finance, legal teams can't send data to third-party APIs
3. **Latency** — Round-trip to cloud adds 200-500ms per call; local is <50ms
4. **Vendor lock-in** — Switching costs when API providers change pricing or deprecate models
5. **Rate limits** — Production workloads hitting API concurrency limits
6. **Hardware complexity** — Setting up and maintaining GPU servers is non-trivial
7. **Power costs** — RTX 4090 draws ~450W under load; at $0.12/kWh, that's ~$40/mo in electricity

---

## 6. FAQ Candidates (from community discussions)

1. **At what usage volume does self-hosting become cheaper than cloud APIs?**
   — Depends on model and hardware, but roughly 10-20M tokens/month for an RTX 4090 vs GPT-5.2.

2. **Can I run vLLM on a Mac?**
   — Not optimally. vLLM is CUDA/NVIDIA-focused. Use llama.cpp or Ollama for Apple Silicon.

3. **What's the best GPU for local LLMs in 2026?**
   — RTX 4090 for value (24GB VRAM, ~$1,600). RTX 5090 for performance (32GB, 2.6x A100). M4 Ultra for unified memory (up to 192GB).

4. **How much does it cost to run a 70B model locally?**
   — Needs 35-40GB VRAM (Q4). Dual RTX 4090 or single A100/H100. Hardware cost: $3,000-8,000. Amortized: $80-220/mo.

5. **Is Ollama good enough for production?**
   — No. Ollama caps at ~41 TPS and degrades under concurrent load. Use vLLM for production serving.

6. **What about data privacy with cloud APIs?**
   — OpenAI, Anthropic, and Google all claim they don't train on API data. But data still touches their servers. Self-hosting is the only way to guarantee data never leaves your network.

---

## 7. Key Takeaways for Article

- **The break-even point is ~10-20M tokens/month** for self-hosting vs premium cloud APIs
- **For low-volume users (<5M tokens/mo), cloud APIs are cheaper** — especially DeepSeek V3.2 at $0.28/$0.42 per 1M
- **vLLM is 19x faster than Ollama** under concurrent load (920 vs 155 tok/s at 50 users)
- **Ollama is the best starting point** — 1-minute setup, but not production-grade
- **Apple Silicon is viable** for single-user workloads — M4 Ultra can run 70B models in 192GB unified memory
- **The real case for self-hosting is privacy, latency, and control** — not just cost
- **Hybrid approach wins** — Ollama for dev, vLLM for production, cloud APIs for burst/overflow

---

## Sources

1. GitHub: ollama/ollama — 171k stars, v0.23.2 (May 7, 2026)
2. GitHub: ggml-org/llama.cpp — 109k stars
3. GitHub: vllm-project/vllm — 79.6k stars, v0.20.2 (May 10, 2026)
4. GitHub: mudler/LocalAI — 46.2k stars
5. SitePoint: "Ollama vs vLLM Performance Benchmark 2026" (March 5, 2026)
6. DecodesFuture: "llama.cpp vs Ollama vs vLLM: 2026 Comparison" (January 31, 2026)
7. BuildMVPFast: "LLM API Pricing Comparison 2026" (April 2026)
8. Prem AI: "Self-Hosted LLM Guide: Setup, Tools & Cost Comparison 2026"
9. Ollama.com/pricing — Free/Pro $20/mo/Max $100/mo
10. Reddit r/LocalLLaMA: "API pricing is in freefall" (2026)
11. Reddit r/LocalLLaMA: "Replacing $200/mo Cursor subscription" (2026)
12. Hacker News: "Self-hosting an AI with your own hardware" (2026)
13. Julien Simon (Medium): "What to Buy for Local LLMs (April 2026)"
14. Contracollective: "M4 Pro vs M5 Pro: Local AI Inference Benchmarks 2026"
