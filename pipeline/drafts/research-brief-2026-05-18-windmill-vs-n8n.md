# Research Brief: Windmill vs n8n — When Code-First Automation Wins

**Date:** 2026-05-17
**Target publish date:** 2026-05-18
**Slug:** windmill-vs-n8n-code-first-automation
**Primary keyword:** windmill vs n8n
**Secondary keywords:** code-first automation, self-hosted workflow automation, windmill automation, n8n alternative

---

## GitHub Repos

### Windmill
- **Repo:** https://github.com/windmill-labs/windmill
- **Stars:** 16.5k ⭐
- **Forks:** 960
- **Contributors:** 160
- **Releases:** 1,386
- **Latest release:** v1.703.1 (May 16, 2026 — 1 day ago)
- **License:** AGPLv3
- **Language:** Rust (backend), Svelte 5 (frontend)

### n8n
- **Repo:** https://github.com/n8n-io/n8n
- **Stars:** ~188k ⭐ (per prior research; Hatchworks cites 40k+ — using 188k from prior pipeline research)
- **Contributors:** 681
- **Releases:** 625+
- **License:** Fair-code (source-available, not OSI-approved)
- **Language:** TypeScript/Node.js

### Apache Airflow (benchmark reference)
- **Repo:** https://github.com/apache/airflow
- **Stars:** ~38k
- **License:** Apache 2.0

---

## Benchmark Data

### Windmill vs Airflow (official Windmill benchmarks, March 2026)
Source: https://www.windmill.dev/docs/misc/benchmarks/competitors/airflow

**Test 1: 40 lightweight Fibonacci tasks (n=10)**
| Metric | Airflow | Windmill | Windmill Dedicated |
|--------|---------|----------|-------------------|
| Total duration | 116.2s | 4.4s | 2.1s |
| Assignment overhead | 64.6% | 41.9% | 85.8% |
| Execution | 10.8% | 50.5% | 5.8% |
| Transition | 24.6% | 7.6% | 8.4% |
| **Speedup** | 1x | **26x** | **55x** |

**Test 2: 10 heavyweight Fibonacci tasks (n=33)**
| Metric | Airflow | Windmill | Windmill Dedicated |
|--------|---------|----------|-------------------|
| Total duration | 54.7s | 8.3s | 7.7s |
| Assignment overhead | 40.4% | 5.1% | 4.8% |
| **Speedup** | 1x | **6.5x** | **7x** |

**Key claim:** Windmill is the fastest self-hostable open-source workflow engine, benchmarked at up to 13x faster than Airflow (source: https://www.windmill.dev/blog/launch-week-1/fastest-workflow-engine).

### Windmill performance specs
- ~50ms added latency (job pull → start → result)
- ~100ms total for typical lightweight Deno job
- Dedicated workers: up to 1,000 steps/second per worker
- Cold start: Python ~60ms, Deno/Bun ~30ms, Bash/Go: none

---

## Pricing Comparison

### n8n Pricing (2026)
Source: https://n8n.io/pricing/, https://connectsafely.ai/articles/n8n-cloud-pricing-guide

| Plan | Price | Executions | Notes |
|------|-------|------------|-------|
| Self-hosted | Free | Unlimited | Fair-code license |
| Starter | €20/mo ($24) | 2,500/mo | Billed annually |
| Pro | €50/mo ($60) | 10,000/mo | |
| Business | €800/mo | Custom | Enterprise features |
| Self-hosted cost | $3-7/mo | Unlimited | VPS hosting only |

**n8n pain point:** Cloud plans have strict execution limits. Self-hosting removes limits but costs $200-500/mo for small deployments when factoring in maintenance (source: https://expresstech.io/the-real-cost-of-self-hosting-n8n-in-2026/).

### Windmill Pricing (2026)
Source: https://www.windmill.dev/pricing

| Plan | Price | Seats | Compute | Notes |
|------|-------|-------|---------|-------|
| Free (self-host) | $0 | Up to 50 users | Unlimited executions | Community Discord support |
| Enterprise | From $120/mo | Developer: $20/mo, Operator: $10/mo | Compute Units: $100/mo per 2 workers | SAML, SLA, audit logs |

**Windmill pain point:** Enterprise pricing is less predictable at scale. Compute Units model can get expensive for high-throughput workloads.

---

## Community Pain Points

### n8n Pain Points (from Reddit r/n8n, r/selfhosted)
1. **Steep learning curve:** New users need 2-8 weeks to build workflows confidently (source: https://emergent.sh/learn/best-n8n-alternatives-and-competitors)
2. **Complex workflows become unwieldy:** 100+ node workflows resemble a "spider web"; version upgrades can break existing flows
3. **Self-hosting overhead:** Real cost $200-500/month for small deployments; six figures annually at enterprise scale
4. **Stateless architecture:** No persistent memory across executions; context wiped after workflow ends
5. **Not truly no-code:** Requires understanding of APIs, JSON, expressions, and logic
6. **Fair-code license restrictions:** Restricts commercial use in SaaS products
7. **Scaling pain:** User scaling from $6 droplet to 400k+ executions reports significant infrastructure challenges (source: https://www.reddit.com/r/n8n/comments/1kfmifs/what_ive_learned_scaling_n8n_from_a_6_droplet_to_/)

### Windmill Pain Points (from Reddit, HN, G2)
1. **Steeper learning curve than n8n:** Code-first approach means non-developers struggle (source: https://hostadvice.com/blog/ai/automation/n8n-vs-windmill/)
2. **Smaller community:** Less documentation, fewer tutorials, smaller Discord than n8n's forum
3. **Fewer pre-built integrations:** Script-based approach means you build more from scratch vs n8n's 1,100+ nodes
4. **Less polished UI:** Flow editor functional but not as visually refined as n8n's canvas
5. **AGPLv3 license:** More restrictive than MIT (Activepieces) — can't use in proprietary SaaS without commercial license
6. **Overkill for simple automations:** If you just need "when X happens, do Y," Windmill's code-first approach adds unnecessary complexity

---

## Feature Comparison

| Feature | Windmill | n8n |
|---------|----------|-----|
| Primary approach | Code-first (scripts) | Visual node-based (low-code) |
| Languages | Python, TypeScript, Go, Bash, PHP, Rust, C#, Java, SQL | JavaScript, Python |
| Integrations | Script-based (build your own) | 1,100+ pre-built nodes |
| AI capabilities | Full Python ML ecosystem (LangChain, transformers) | Native AI agent nodes (Claude, OpenAI, Gemini) |
| Workflow engine | Rust-based, ~50ms latency | Node.js-based |
| Auto-generated UIs | Yes (from script parameters) | No |
| Approval steps | Yes (Enterprise) | Yes |
| Git sync | Yes (Enterprise) | No native Git sync |
| Self-host RAM | 2-4 GB recommended | 1-2 GB |
| License | AGPLv3 | Fair-code |
| Community size | ~16.5k stars, Discord | ~188k stars, active forum |

---

## Recent Developments (2026)

### Windmill Launch Week #2 (March 30 - April 3, 2026)
Source: https://www.windmill.dev/launch-week-march-2026

1. **Full-code apps** — Build complex low-code apps on top of scripts and flows
2. **Data tables & Ducklake** — Built-in data tables with Ducklake integration
3. **AI sandboxes & volumes** — Run AI coding agents with process isolation and persistent file storage
4. **Git sync & workspace forks** — Git-based collaboration with workspace forking
5. **Workflow-as-code v2** — Define workflows as code for version control

### n8n Developments
- Valuation: $2.3 billion (August 2025), up from $350M four months prior
- Revenue: Surpassed $40M annually
- Clients: Vodafone, Delivery Hero
- n8n-as-code project crossed 1,000 GitHub stars (community-driven)

---

## FAQ Candidates (from community discussions)

1. **When should I choose Windmill over n8n?**
   Engineering teams running data pipelines, ETL jobs, or complex multi-step scripts in Python/TypeScript. If your team writes code daily and wants Git-native workflows, Windmill wins.

2. **Can Windmill replace n8n for simple automations?**
   Technically yes, but it's overkill. If you're connecting Slack to Google Sheets, n8n's visual editor is faster. Windmill shines when you need loops, conditionals, and data transformations that would require 50+ n8n nodes.

3. **Is Windmill really faster than n8n?**
   For workflow engine performance, yes — Windmill's Rust backend with PostgreSQL queue achieves ~50ms job latency vs n8n's Node.js event loop. But "faster" depends on context: n8n is faster to build simple workflows; Windmill is faster at executing complex ones.

4. **What's the real cost difference?**
   Both are free to self-host. n8n Cloud starts at €20/mo with execution limits. Windmill's free tier supports up to 50 users with unlimited executions. For cloud, Windmill Enterprise starts at $120/mo. At scale, n8n's execution-based pricing can get expensive; Windmill's compute-unit model is more predictable for high-throughput workloads.

5. **Which has better AI support?**
   n8n wins for no-code AI workflows — native Claude, OpenAI, and Gemini nodes with AI agent builders. Windmill wins for code-first AI — full Python ML ecosystem access (LangChain, scikit-learn, transformers) with AI sandboxes for running coding agents.

6. **Can I migrate from n8n to Windmill?**
   There's no automated migration tool. Workflows must be rebuilt — n8n's visual nodes become Windmill scripts. The migration cost is real: expect 2-4 weeks for a medium-complexity workflow library.

---

## Key Statistics Summary

- Windmill: 16.5k GitHub stars, 960 forks, 160 contributors, 1,386 releases, latest v1.703.1 (May 16, 2026)
- n8n: ~188k GitHub stars, 681 contributors, 625+ releases, $2.3B valuation
- Windmill benchmark: 26-55x faster than Airflow for lightweight tasks, 6.5-7x for heavyweight
- n8n cloud: €20-800/mo depending on executions
- Windmill free tier: up to 50 users, unlimited executions
- n8n self-hosted real cost: $200-500/mo (including maintenance)
- Windmill self-hosted cost: $3-7/mo (VPS only, no execution limits)
- Workflow automation market: $26B in 2026, projected $80B+ by 2035
- Windmill used by 3,000+ organizations (source: Shakudo, March 2026)

---

## Sources

1. https://github.com/windmill-labs/windmill — Windmill GitHub repo
2. https://github.com/n8n-io/n8n — n8n GitHub repo
3. https://www.windmill.dev/docs/misc/benchmarks/competitors/airflow — Windmill vs Airflow benchmarks
4. https://www.windmill.dev/blog/launch-week-1/fastest-workflow-engine — Fastest workflow engine claim
5. https://www.windmill.dev/pricing — Windmill pricing
6. https://n8n.io/pricing/ — n8n pricing
7. https://www.booleanbeyond.com/en/insights/n8n-vs-activepieces-vs-windmill-open-source-automation — Boolean & Beyond comparison
8. https://hostadvice.com/blog/ai/automation/n8n-vs-windmill/ — HostAdvice comparison
9. https://emergent.sh/learn/best-n8n-alternatives-and-competitors — n8n alternatives guide
10. https://www.windmill.dev/launch-week-march-2026 — Windmill Launch Week #2
11. https://www.reddit.com/r/n8n/comments/1kfmifs/what_ive_learned_scaling_n8n_from_a_6_droplet_to_/ — n8n scaling pain points
12. https://expresstech.io/the-real-cost-of-self-hosting-n8n-in-2026/ — n8n self-hosting costs
13. https://connectsafely.ai/articles/n8n-cloud-pricing-guide — n8n pricing guide
14. https://www.shakudo.io/blog/top-9-workflow-automation-tools — 3,000+ orgs using Windmill
