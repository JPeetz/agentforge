# Research Brief: Temporal vs n8n — Durable Workflows for Mission-Critical Apps

**Date:** 2026-05-19
**Slug:** temporal-vs-n8n-durable-workflows-mission-critical
**Topic:** Temporal vs n8n comparison — when to use a code-first durable execution engine vs a visual workflow automation tool

---

## GitHub Repos

| Repo | Stars | Language | Last Updated | Notes |
|------|-------|----------|-------------|-------|
| [temporalio/temporal](https://github.com/temporalio/temporal) | 20.3k | Go (99.5%) | April 29, 2026 (v1.31.0) | Core Temporal server. 280 contributors, 166 releases. MIT license. |
| [n8n-io/n8n](https://github.com/n8n-io/n8n) | 188k | TypeScript (91.3%) | May 12, 2026 (v2.21.0) | Fair-code workflow automation. 681 contributors, 625 releases, 57.8k forks. |
| [inngest/inngest](https://github.com/inngest/inngest) | 4.2k | TypeScript/Go | March 2026 | Durable execution alternative. Lighter-weight than Temporal. |

## Benchmark Numbers & Performance Claims

- **Temporal at scale:** Netflix reduced transient deployment failures from 4% to 0.0001% using Temporal (source: byteiota.com, 2026; Medium analysis of Netflix engineering blog).
- **Temporal customers:** 3,000+ paying customers as of Replay 2026 conference (source: thenewstack.io, 2026).
- **n8n scale:** 400+ integrations, 900+ ready-to-use templates, 188k GitHub stars (source: n8n GitHub, May 2026).
- **Temporal v1.31.0:** Released April 29, 2026. Includes Elasticsearch v14 index templates, CVE-2026-5724 fix, CHASM signal backlinks, per-namespace rate limiting (source: GitHub releases).
- **n8n v2.21.0:** Released May 12, 2026. Includes MCP client improvements, Alpine 3.23 migration, Node 26 in CI, security dependency bumps (source: GitHub).

## Pricing Comparison

### Temporal Cloud
| Plan | Monthly Cost | Actions | Active Storage | Retained Storage |
|------|-------------|---------|---------------|-----------------|
| Essentials | $100/mo (or 5% of usage) | 1M included | 1 GB | 40 GB |
| Business | $500/mo (or 10% of usage) | 2.5M included | 2.5 GB | 100 GB |
| Enterprise | Custom (annual) | 10M included | 10 GB | 400 GB |

- **Overage pricing:** $50 per million actions (first 5M), scaling down to $25/M at 200M+.
- **Storage:** Active $0.042/GBh, Retained $0.00105/GBh.
- **Self-hosted:** Free (open source MIT license). Requires Cassandra/SQL cluster + Elasticsearch + Temporal service nodes.
- **Startup program:** $6,000 free credits for startups under $30M funding.
- **Source:** [temporal.io/pricing](https://temporal.io/pricing), [docs.temporal.io/cloud/pricing](https://docs.temporal.io/cloud/pricing)

### n8n Cloud
| Plan | Monthly Cost | Executions | Concurrent | Key Features |
|------|-------------|-----------|------------|-------------|
| Starter | $20/mo | 2.5K | 5 | 1 project, forum support |
| Pro | $50/mo | 10K | 20 | 3 projects, 7-day insights |
| Business | $800/mo | 40K | Custom | SSO/SAML, Git VC, multi-env |
| Enterprise | Custom | Custom | 200+ | Dedicated SLA, log streaming |

- **Self-hosted Community Edition:** Free. Server costs $3-7/mo on a VPS (source: instapods.com, 2026).
- **Execution model:** Pay per full workflow run, not per step. A 500-step workflow = 1 execution.
- **Source:** [n8n.io/pricing](https://n8n.io/pricing/)

## Community Pain Points

### n8n Pain Points (from Reddit r/selfhosted, r/n8n)
1. **Subpath hosting not supported** — Users report n8n cannot be hosted under a subpath (e.g., domain.com/n8n) without caveats (source: r/selfhosted, 2026).
2. **Documentation gaps** — Self-hosting newbies report lack of documentation for advanced configurations (source: r/selfhosted, 2026).
3. **Long-running workflow limitations** — n8n is not a true state machine; no deterministic replay of execution history (source: n8nlab.io comparison, 2026).
4. **Scaling costs** — At high execution volumes, n8n Cloud costs scale quickly ($800/mo for 40K executions on Business plan).
5. **"Prototype trap"** — Users report prototyping in n8n then needing to rewrite in Python for production (source: r/selfhosted, 2026).

### Temporal Pain Points (from HN, Reddit)
1. **High barrier to entry** — No drag-and-drop interface; requires weeks/months to learn vs. hours/days for n8n (source: n8nlab.io, 2026).
2. **Determinism constraints** — Developers must understand and code around Temporal's determinism requirements (source: HN discussion, 2026).
3. **Opaque to non-developers** — Business stakeholders cannot visually inspect workflows (source: n8nlab.io, 2026).
4. **Infrastructure complexity** — Self-hosting requires Cassandra/SQL cluster, Elasticsearch, and Temporal service nodes (source: n8nlab.io, 2026).
5. **"Over-engineering" perception** — For simple API integrations, Temporal adds unnecessary complexity (source: r/programming, 2026).

## Primary Sources

1. Temporal official pricing: [temporal.io/pricing](https://temporal.io/pricing)
2. Temporal Cloud pricing docs: [docs.temporal.io/cloud/pricing](https://docs.temporal.io/cloud/pricing)
3. n8n official pricing: [n8n.io/pricing](https://n8n.io/pricing/)
4. Temporal GitHub: [github.com/temporalio/temporal](https://github.com/temporalio/temporal) — 20.3k stars, v1.31.0 (April 29, 2026)
5. n8n GitHub: [github.com/n8n-io/n8n](https://github.com/n8n-io/n8n) — 188k stars, v2.21.0 (May 12, 2026)
6. n8n vs Temporal comparison (n8n agency perspective): [n8nlab.io/blog/n8n-vs-temporal-workflow-automation](https://n8nlab.io/blog/n8n-vs-temporal-workflow-automation)
7. Netflix Temporal case study: [byteiota.com/temporal-workflow-engine-netflixs-10x-speed-secret-2026](https://byteiota.com/temporal-workflow-engine-netflixs-10x-speed-secret-2026/)
8. Temporal 3,000+ customers: [thenewstack.io/temporal-durable-execution-ai-workflows](https://thenewstack.io/temporal-durable-execution-ai-workflows/)
9. HN durable execution discussions: [news.ycombinator.com/item?id=46245238](https://news.ycombinator.com/item?id=46245238)
10. AI Workflow Orchestration Tools 2026: [digitalapplied.com/blog/ai-workflow-orchestration-tools-2026-comparison](https://www.digitalapplied.com/blog/ai-workflow-orchestration-tools-2026-comparison)

## FAQ Candidates (from community discussions)

1. **Can n8n handle long-running workflows that span days or weeks?**
   n8n can handle long-running workflows with wait/delay nodes, but it lacks deterministic replay. If the server crashes mid-execution, the workflow state may be lost. Temporal guarantees completion through durable execution.

2. **Is Temporal worth the complexity for a small team?**
   For teams under 10 engineers running non-critical automations, Temporal is usually over-engineering. n8n or Inngest provide 80% of the value at 20% of the operational cost.

3. **What's the real cost difference between Temporal Cloud and self-hosted n8n?**
   Self-hosted n8n costs $3-7/mo in infrastructure. Temporal Cloud starts at $100/mo (Essentials). For high-volume workloads (10M+ actions/mo), Temporal Cloud costs $2,000-12,000/mo (source: checkthat.ai, 2026).

4. **Can I migrate from n8n to Temporal later?**
   Yes, but it requires rewriting workflows from visual nodes to code (Go/Java/TypeScript SDKs). There's no automated migration path. Plan your architecture before committing.

5. **How does Temporal compare to Inngest for durable execution?**
   Inngest is lighter-weight and developer-friendly (event-driven, works with existing HTTP handlers). Temporal is more mature, supports more languages, and handles larger-scale workloads. Inngest starts at $75/mo vs. Temporal's $100/mo.

6. **When should I choose Temporal over n8n?**
   Choose Temporal when: workflows must survive server crashes, execution history must be auditable, processes run for days/weeks, or you're orchestrating microservices at scale. Choose n8n for API integrations, AI agent pipelines, and business process automation.

## Key Differentiators Summary

| Dimension | Temporal | n8n |
|-----------|----------|-----|
| **Philosophy** | Code-first durable execution | Visual workflow automation |
| **Interface** | SDKs (Go, Java, TS, Python) | Drag-and-drop + JS/Python nodes |
| **Fault tolerance** | Deterministic replay, crash-proof | Retry logic, error triggers |
| **Long-running** | Months/years (workflow sleep) | Hours/days (wait nodes, no replay) |
| **Scale ceiling** | Millions of concurrent workflows | Thousands of executions/month |
| **Learning curve** | Weeks/months | Hours/days |
| **Best for** | Microservices, payments, subscriptions | API integrations, AI agents, business automation |
| **Self-host cost** | High (cluster + Elasticsearch) | Low ($3-7/mo VPS) |
| **Cloud cost** | $100-12,000/mo | $20-800/mo |
