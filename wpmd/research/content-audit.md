# Content Audit — AgentForge WordPress Theme Design

**Date:** 2026-05-16
**Purpose:** Document every content type, its structure, fields, and visual elements to inform the WordPress content model and theme design.

---

## 1. SEO Articles (Content Pipeline)

These are the primary daily output of AgentForge. Three examples analyzed:
- `2026-05-16-hidden-costs-ai-saas-beyond-subscription.md` (123 lines, ~13KB)
- `2026-05-15-crewai-multi-agent-content-team-tutorial.md` (292 lines, ~16KB)
- `2026-05-14-ai-agent-security-self-hosting-privacy.md` (164 lines, ~18KB)

### Frontmatter Fields

| Field | Type | Example | Notes |
|-------|------|---------|-------|
| title | string | `"Hidden Costs AI SaaS: The Fees You Don't See Coming"` | Display title, H1 source |
| slug | string | `hidden-costs-ai-saas-beyond-subscription` | URL slug |
| date | ISO date | `2026-05-16` | Publish date |
| author | string | `AutoRanker` | Always "AutoRanker" currently |
| tags | string array | `[ai saas, vendor lock-in, tco, ai pricing, saas costs]` | 4-6 tags per article |
| description | string | Full meta description sentence | SEO meta description, ~150-160 chars |
| keywords | string array | `["hidden costs ai saas", "ai saas pricing", ...]` | 4-5 SEO keywords |
| status | enum | `draft` / `qa-passed` | Pipeline status |
| word_count | integer | `0` (draft) / `2572` (qa-passed) | Populated after QA |

### Body Structure

1. **Hero image** — Markdown image tag on line 13, referencing `images/{slug}-hero.png`
2. **H1 heading** — Matches title (sometimes slightly rephrased for readability)
3. **Opening paragraph** — 2-4 sentences, data-heavy, confrontational tone, cites sources inline
4. **H2 sections** — 5-8 sections per article, each with a bold, specific heading
5. **Body content within sections** — Mix of paragraphs, bold callouts, inline source citations
6. **FAQ section** — H2 "Frequently Asked Questions" with H3 questions and detailed answers (5-6 FAQs)
7. **Bottom Line section** — H2 "The Bottom Line" with summary and call-to-action

### Visual Elements

- **Hero image** — One per article, referenced as `images/{slug}-hero.png`. Placed at top, before H1.
- **Inline images** — 0-2 per article, placed mid-content using `[IMAGE: description]` placeholder syntax. Descriptions specify style, colors, content.
- **Tables** — Present in security article: OWASP vulnerability table with ID, Vulnerability, Self-Hosting Relevance columns. Also pricing comparison table in research brief.
- **Code blocks** — Present in tutorial article only: bash install commands, YAML agent/task configs, Python crew.py and main.py. Fenced with language identifiers.
- **Diagrams** — Referenced as image placeholders (e.g., TCO breakdown diagram, 3-agent pipeline diagram, layered isolation architecture). Not embedded as code.

### Taxonomies / Categories

- **Tags** (from frontmatter): `ai saas`, `vendor lock-in`, `tco`, `ai pricing`, `saas costs`, `crewai`, `multi-agent`, `python`, `ai agents`, `content automation`, `ai-agent-security`, `self-hosted-ai`, `openclaw-security`, `owasp-agentic`, `ai-privacy`
- **No explicit category field** in frontmatter — categories must be inferred from tags or content type.
- **Status** acts as a workflow taxonomy: `draft` -> `qa-passed` -> (presumably) `published`

### Typical Length

- **Short article:** ~120 lines / ~13KB / ~2,500 words (hidden costs)
- **Medium article:** ~160 lines / ~18KB / ~2,572 words (security)
- **Long article (tutorial):** ~290 lines / ~16KB / ~3,000+ words (CrewAI tutorial)
- **Target per master doc:** 1,200-1,800 words

### What Makes It Visually Distinct

- Data-heavy opening paragraphs with inline source citations
- Bold callout phrases within paragraphs (e.g., "**Token overage.**", "**Tool overlap.**")
- FAQ section at bottom with H3 questions — good for FAQ schema markup
- "The Bottom Line" closing section — consistent pattern across all articles
- Tutorial articles have code blocks with syntax highlighting needs
- Security/comparison articles have tables
- Image placeholders with detailed style descriptions (dark navy, teal accents, minimalist)

---

## 2. Research Briefs (Content Pipeline)

One example analyzed:
- `research-brief-2026-05-16-hidden-costs-ai-saas.md` (93 lines, ~7.8KB)

### Frontmatter Fields

Research briefs use **no YAML frontmatter**. The metadata is in the Markdown body itself:

| Field | Location | Example |
|-------|----------|---------|
| Title | H1 heading | `Research Brief: The Hidden Costs of AI SaaS (Beyond the Subscription)` |
| Date | Bold line | `**Date:** 2026-05-15` |
| Target publish date | Bold line | `**Target publish date:** 2026-05-16` |
| Slug | Bold line | `**Slug:** hidden-costs-ai-saas-beyond-subscription` |
| Primary keyword | Bold line | `**Primary keyword:** hidden costs AI SaaS` |
| Secondary keywords | Bold line | `**Secondary keywords:** AI SaaS pricing, vendor lock-in AI, ...` |

### Body Structure

1. **Key Statistics** — Numbered list of 10 findings, each with inline source citation
2. **GitHub Repos (Open-Source Alternatives)** — Numbered list of 5 repos with star counts, descriptions, source links
3. **Current SaaS Pricing** — Markdown table with columns: Tool, Plan, Price, Notes (11 rows)
4. **Community Pain Points** — Numbered list of 7 items from HN/Reddit with quotes and sources
5. **FAQ Candidates** — Numbered list of 6 questions derived from community discussions
6. **Monetisation Angles** — Bullet list of affiliate, course, follow-up, partnership opportunities
7. **Article Angle** — Core thesis, target audience, tone description

### Visual Elements

- **Tables** — One pricing comparison table (Tool | Plan | Price | Notes)
- **Numbered lists** — Primary content format throughout
- **No images** — Research briefs are text-only research documents
- **No code blocks**

### Taxonomies / Categories

- No explicit taxonomy. Implicitly categorized by the slug/topic of the associated article.
- Keywords serve as de facto taxonomy.

### Typical Length

- ~90 lines / ~8KB / ~1,500 words of research content

### What Makes It Visually Distinct

- Pure research document — no narrative, no hero image, no FAQ
- Structured as reference material for article writers
- Heavy use of numbered lists and source citations
- Pricing table is the only tabular data
- Monetisation angles section is unique to this content type

---

## 3. Prompt Collections / Bundles (PD Department)

Source: `PROMPT-DEV-DEPT.md` (first 100 lines of 1,570)

### Structure

Prompt bundles are **not yet created as standalone files** in the analyzed materials. They are a planned product type described in the master doc and PD department spec.

From the master doc (lines 170-189), planned bundle categories:
- The AgentForge Content Pipeline Bundle (all content agent prompts and skills)
- The Hermes Company Bootstrap Bundle (org structure, HM/HMA templates, governance)
- The Keyword Research Automation Bundle
- The WordPress Automation Bundle
- The SEO Article Writing Prompt Bundle

From the PD department spec, prompts flow through stages:
- Raw prompts (discovered from Reddit, GitHub, X, forums, Discord, YouTube)
- Curated prompts (deduplicated, categorized, tagged, stored in Memex)
- Optimized prompts (A/B tested, improved)
- Published prompts (stored in Memex + Obsidian + MD, published to GitHub)

### Fields (Inferred from PD Department Spec)

| Field | Source | Notes |
|-------|--------|-------|
| Prompt text | Core content | The actual prompt |
| Category | PD dept taxonomy | Tool-specific or use-case |
| Tags | PD dept tagging | For searchability |
| Source URL | Researcher | Where it was found |
| Optimization log | Optimizer | A/B test results |
| Use case | Analyst | What it's for |
| Tool compatibility | Analyst | OpenClaw, Hermes, Claude Code, etc. |

### Visual Elements

- Prompts are text/code — likely displayed as code blocks in WordPress
- Bundles may include YAML/JSON configs
- No images expected in prompt content

### Taxonomies

PD department focus areas (from the spec):
- **Tool-specific:** OpenClaw, Hermes, Claude Code, Codex CLI
- **Use-case:** YouTube automation, social media, long-form content, multi-agent swarms, coding, research, trading, image generation, voice cloning, video, customer support, email/calendar, project management

### Typical Length

- Individual prompts: 50-500 words
- Bundles: 50+ prompts per category (per master doc)

### What Makes It Visually Distinct

- Code-heavy content requiring syntax highlighting
- Categorized by tool and use case
- Battle-tested positioning ("these run a live content operation")
- Sold as downloadable products (WooCommerce integration)

---

## 4. Tutorials

Tutorials are a **variant of SEO articles** with additional structural elements. The CrewAI article is the primary example.

### How Tutorials Differ from Standard SEO Articles

- **Step-by-step structure** — Numbered steps (Step 1, Step 2, etc.) with H2 headings
- **Code blocks** — Bash, YAML, Python with language-specific syntax highlighting
- **Project structure** — File paths and directory trees shown in code blocks
- **Configuration files** — YAML configs displayed as code (agents.yaml, tasks.yaml)
- **Execution instructions** — Bash commands to run the code
- **Deployment plan** — Specific timeline (e.g., "Saturday morning: Install CrewAI...")
- **"What You Are Building" section** — Sets expectations before the tutorial begins
- **"Honest Tradeoffs" section** — Compares the tool to alternatives with benchmark data
- **Longer length** — ~290 lines vs ~120-160 for standard articles

### Visual Elements (Additional to Standard Articles)

- **Code blocks** — 6 code blocks in the CrewAI tutorial (bash, yaml, python x3, bash)
- **File tree references** — `src/content_team/config/agents.yaml`, `crew.py`, `main.py`
- **Diagram image** — Pipeline diagram placeholder with detailed style description
- **Inline code** — Backtick-wrapped references to class names, commands, file paths

### Taxonomies

- Same frontmatter as SEO articles (title, slug, date, author, tags, description, keywords, status, word_count)
- Tags indicate tutorial nature: `crewai`, `multi-agent`, `python`, `ai agents`, `content automation`

### What Makes It Visually Distinct

- Code blocks are the primary differentiator — need proper syntax highlighting
- Step-by-step H2 structure creates a scannable tutorial format
- "Honest Tradeoffs" section with comparison data (benchmarks, percentages)
- Deployment plan with specific timeline

---

## 5. Tool Comparisons

Tool comparisons are a **pattern within existing articles**, not a separate content type. They appear as sections within SEO articles and tutorials.

### Where Comparisons Appear

- **Hidden costs article:** Compares self-hosted vs SaaS TCO (Ollama + n8n + open-source vs ChatGPT Team + Claude Team)
- **CrewAI tutorial:** "CrewAI vs LangGraph or AutoGen" section with benchmark percentages (79% vs 62% on complex tasks, 54% vs 62%)
- **Security article:** Self-hosting vs cloud comparison throughout; OWASP table with "Self-Hosting Relevance" column; guardrails comparison (free vs enterprise)

### Comparison Structure

- **Benchmark data tables** — Percentage scores, success rates
- **Feature comparison tables** — Tool | Plan | Price | Notes (from research brief)
- **Pros/cons sections** — "Honest Tradeoffs" heading with bullet lists
- **Inline comparisons** — "X does Y better than Z because..." within paragraphs
- **Pricing comparisons** — Side-by-side pricing tables

### Visual Elements

- **Tables** — Primary comparison format (OWASP table, pricing table)
- **Bold callouts** — Key differentiators called out inline
- **Percentage/stat callouts** — "54% vs 62%", "79% of simple tasks"
- **Image placeholders** — Diagram suggestions for visual comparisons

### Taxonomies

- Inherited from parent article tags
- Implicit "comparison" nature from section headings

### What Makes It Visually Distinct

- Tables are the dominant visual element
- Side-by-side data presentation
- Benchmark percentages called out prominently
- "Honest Tradeoffs" framing — not just feature lists but real-world reliability data

---

## Summary: Content Type Comparison

| Attribute | SEO Article | Research Brief | Tutorial | Tool Comparison | Prompt Bundle |
|-----------|-------------|----------------|----------|-----------------|---------------|
| Frontmatter | Full YAML | None (inline MD) | Full YAML | Inherited | Planned |
| Hero image | Yes | No | Yes | Inherited | No |
| Code blocks | Rare | No | Heavy | No | Heavy |
| Tables | Occasional | Yes (pricing) | No | Yes (primary) | No |
| FAQs | Yes (5-6) | No | Yes (5-6) | No | No |
| Images | 1-2 | 0 | 1-2 | 0-1 | 0 |
| Length | 120-160 lines | ~90 lines | ~290 lines | Section | 50+ items |
| Tone | Data-driven, confrontational | Neutral, reference | Instructional, practical | Analytical | Technical |
| Status field | draft/qa-passed | N/A | draft/qa-passed | Inherited | N/A |
