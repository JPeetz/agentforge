# AgentForge — WordPress Design & Content Department (WPMD)

> **Department name:** WordPress Design & Content (WPMD)
> **Department head:** wpmd-cmo
> **Reports to:** Marvin (CEO)
> **Cooperates with:** agentforge-cmo (content pipeline) + prompts-cmo (PD department)
> **Workspace:** /Users/joergpeetz/workspace/agentforge/wpmd/
> **Output:** Custom WordPress theme + content structure ready for agent-forge.co

---

## Department Purpose

WPMD exists to design and build a WordPress theme/template that visually represents what AgentForge actually produces. Not a generic blog theme. Not a corporate template. A design language that says "this is an AI-powered content operation" and makes the content — articles, prompt bundles, tutorials, research briefs — look like it belongs together.

The department has three phases:
1. **Research** — understand what content types exist and will exist
2. **Design** — create the visual language, layout system, and component library
3. **Development** — build the actual WordPress theme

When you provide WP login details, content posting begins immediately. No deployment blockers, no WooCommerce, no Stripe. Just design and content structure.

---

## What AgentForge Produces (Design Input)

The theme must accommodate these content types:

**From the Content Pipeline (agentforge-cmo):**
- Long-form SEO articles (1,500–3,000 words) on AI tools, self-hosting, automation
- Each article has: hero image, PDF download, structured sections, code blocks, comparisons
- Topics: self-hosted AI stacks, Ollama vs cloud APIs, n8n vs Zapier, agent security, CrewAI tutorials, hidden SaaS costs
- Published daily, one per day

**From the Prompt Development Department (prompts-cmo):**
- Curated prompt collections (bundled by use case)
- Tool-specific prompt guides (OpenClaw, Hermes, Claude Code, Codex)
- Use-case prompt packs (YouTube automation, social media, coding, research)
- Each prompt entry has: prompt text, use case, tool tags, rating

**Future content types (design for these now):**
- Prompt bundle landing pages (free + paid)
- Tutorial walkthroughs with embedded code
- Research briefs and market analysis
- Tool comparison tables

---

## Design Requirements

**Visual identity:**
- Dark-first design (this is a tech/AI audience)
- Monospace accents for code and technical content
- Clean typography hierarchy — articles are the product
- Subtle AI/forge motif (not cheesy, not literal — no robot logos)
- Fast, minimal, no bloat

**Layout system:**
- Article-focused homepage (latest posts grid/list)
- Category templates: Articles, Prompts, Tutorials, Research
- Single article template with: hero image, table of contents, content body, related posts, PDF download CTA
- Prompt collection template with: prompt cards, filter by tool/use case, copy-to-clipboard
- Responsive, mobile-first

**Technical requirements:**
- Standard WordPress theme (PHP + CSS + minimal JS)
- Custom post types: Article, Prompt Collection, Tutorial
- Custom taxonomies: Topic, Tool, Use Case
- Gutenberg-compatible (block editor support)
- SEO-ready schema markup
- Fast loading (no page builders, no bloated frameworks)

---

## Org Chart

```
MARVIN (CEO)
    │
    ▼
wpmd-cmo (Department Head)
    │
    ├── wpmd-researcher
    │   Audits content pipeline output, defines content models,
    │   researches competitor WP themes in AI/tech niche
    │
    ├── wpmd-designer
    │   Creates visual identity, layout concepts, component library,
    │   produces HTML/CSS mockups for all template types
    │
    └── wpmd-developer
        Builds the actual WordPress theme (PHP templates, CSS, JS),
        implements custom post types, taxonomies, Gutenberg blocks
```

---

## Phase 1: Research (Week 1)

**wpmd-researcher delivers:**
- Content audit: every content type from pipeline + PD, with metadata schema
- Competitor analysis: 5 WordPress themes used by AI/tech content sites
- Content model document: custom post types, taxonomies, fields needed
- Design brief: visual direction, layout requirements, component inventory

**Output location:** /workspace/agentforge/wpmd/research/

---

## Phase 2: Design (Week 2)

**wpmd-designer delivers:**
- Visual identity: color palette, typography, spacing system, icon style
- HTML/CSS mockups for: homepage, article single, article archive, prompt collection single, prompt archive, category pages
- Component library: cards, buttons, code blocks, tables, CTAs, navigation
- Mobile responsive variants for all templates

**Output location:** /workspace/agentforge/wpmd/design/

---

## Phase 3: Development (Week 3)

**wpmd-developer delivers:**
- Complete WordPress theme in /workspace/agentforge/wpmd/development/theme/
- style.css, functions.php, all template files
- Custom post types and taxonomies registered
- Gutenberg block styles matching the design
- Theme screenshot and readme
- Installation documentation

**Output location:** /workspace/agentforge/wpmd/development/

---

## Cooperation with Other Departments

**agentforge-cmo (content pipeline):**
- WPMD needs to know: what metadata accompanies each article (categories, tags, featured image, PDF link, hero image)
- Content pipeline needs to know: what fields the theme expects (so articles are structured correctly for WP)
- Shared document: content-model.md in /workspace/agentforge/wpmd/research/

**prompts-cmo (PD department):**
- WPMD needs to know: prompt data structure (prompt text, tags, tool, use case, rating)
- PD needs to know: how prompts will be displayed (so they're stored in compatible format)
- Shared document: prompt-schema.md in /workspace/agentforge/wpmd/research/

---

## What This Department Does NOT Do

- No WooCommerce
- No Stripe / payment integration
- No subscription gating
- No IONOS server management
- No deployment (that's ops, not design)
- No content writing (that's the content pipeline)
- No prompt research (that's PD)

When you provide WP login details, the developer phase output (the theme) gets installed and content posting begins.

---

## Status

- [x] Department spec created
- [ ] Agent profiles created
- [ ] Phase 1: Research
- [ ] Phase 2: Design
- [ ] Phase 3: Development
- [ ] Theme ready for installation
