# Design Brief: agent-forge.co WordPress Theme

> Author: wpmd-researcher (based on content-audit.md + competitor-analysis.md)
> Date: 2026-05-15
> Audience: wpmd-designer (next phase)

---

## What This Theme Is For

AgentForge publishes one high-quality SEO article per day about AI tools, self-hosting, automation, and prompt engineering. It also produces curated prompt bundles. That's it. The theme must make this content look exceptional and be immediately ready for posting when WP login details are provided.

This is not a magazine. Not a portfolio. Not an agency site. It is a content publishing platform with a very specific, very focused content model.

---

## Content Types and Template Requirements

### 1. SEO Article (Custom Post Type: `article`)

**Frequency:** 1 per day
**Length:** 1,500-3,000 words
**Template:** single-article.php

Required elements:
- Hero image (full-width, top of article)
- Article metadata: date, author ("AutoRanker"), categories, tags, reading time
- Table of contents (auto-generated from h2/h3 headings)
- Body content with: headings, paragraphs, code blocks (syntax highlighted), tables, blockquotes, images with captions, comparison boxes, FAQ section
- PDF download button (each article has an associated PDF)
- Related posts (3 articles, same category/topic)
- "The Bottom Line" closing section (consistent pattern across all articles)
- Schema markup: Article, FAQPage

**Design notes:**
- Code blocks must be syntax-highlighted (dark theme, monospace font)
- Tables need alternating row colors and clear borders
- Blockquotes need visual distinction (left border, subtle background)
- FAQ section should be collapsible (accordion) for long FAQ lists
- Hero image should have a subtle gradient overlay for text readability if needed

### 2. Prompt Bundle (Custom Post Type: `prompt_bundle`)

**Frequency:** 1-2 per week (from PD department)
**Length:** Variable (10-50 prompts per bundle)
**Template:** single-prompt.php

Required elements:
- Bundle title, description, use case tags, tool tags
- Prompt cards: each card has prompt text, use case, tool tags, copy-to-clipboard button
- Filter by: tool, use case (AJAX or page reload)
- Bundle metadata: number of prompts, last updated, difficulty level
- Related bundles section

**Design notes:**
- Prompt cards are the core UI element — they must be clean and scannable
- Copy-to-clipboard on every prompt card (one-click)
- Filter UI should be prominent (this is how users find what they need)
- Dark theme works well for code-like content

### 3. Article Archive (archive-article.php)

**Template for:** /articles/ and category/tag archives
**Layout options:** Grid (default) and list view (toggle)
**Elements per card:** Hero thumbnail, title, excerpt (150 chars), date, categories, reading time
**Pagination:** Load more button (AJAX) or traditional pagination
**Sidebar (optional):** Topic cloud, tool filter, recent articles

### 4. Homepage (front-page.php)

**Layout:**
- Hero section: site title, tagline, latest article featured
- Latest articles grid (6 most recent, 3-column)
- Prompt bundles section (latest 3 bundles)
- Topic categories section (visual grid of topics)
- Footer: about, social links, newsletter signup (future)

**Design notes:**
- The homepage should immediately communicate "this is where AI content lives"
- Latest article should be prominently featured (not buried in a grid)
- Keep it clean — no sliders, no carousels, no pop-ups

### 5. Category/Topic Archive (taxonomy-af_topic.php)

**Layout:** Same as article archive but filtered by topic
**Elements:** Topic title, description, article count, article grid
**Subtopics:** If topic has child terms, show subtopic navigation

---

## Visual Identity

### Color Palette (Dark-First)

```
Background:        #0d1117 (GitHub-dark-like, not pure black)
Surface/cards:     #161b22 (slightly lighter than background)
Border:            #30363d (subtle, visible on dark)
Text primary:      #e6edf3 (near-white, high contrast)
Text secondary:    #8b949e (muted, for metadata)
Accent:            #58a6ff (blue, for links and CTAs)
Accent hover:      #79c0ff (lighter blue)
Code background:    #161b22
Code text:         #e6edf3
Code accent:       #ff7b72 (red/orange for syntax highlighting)
Success:           #3fb950 (green, for copy confirmation)
Warning:           #d29922 (yellow, for notes/warnings)
```

**Rationale:** This palette is inspired by GitHub's dark theme — familiar to the developer/AI audience, excellent contrast ratios, and proven at scale. WCAG AA compliant.

### Typography

```
Headings:     Inter (or system sans-serif fallback)
Body:         Inter (or system sans-serif fallback)
Code:         JetBrains Mono (or system monospace fallback)
Base size:    16px (1rem)
Line height:  1.7 (body), 1.3 (headings)
Max width:    720px (article body), 1200px (page container)
```

**Rationale:** Inter is the modern standard for UI/content sites. JetBrains Mono is the best monospace font for code readability. System font fallback ensures fast loading.

### Spacing System

```
Base unit: 8px
Scale: 8, 16, 24, 32, 48, 64, 96, 128
Article padding: 24px (mobile), 32px (desktop)
Section spacing: 48px (between major sections)
Component spacing: 16px (between cards, between elements)
```

---

## Component Inventory

Components needed (designer must produce HTML/CSS for all):

1. **Navigation** — Logo, menu (Articles, Prompts, Topics, About), search icon, dark/light toggle (optional)
2. **Article card** — Thumbnail, title, excerpt, metadata (date, categories, reading time)
3. **Prompt card** — Prompt text, use case tag, tool tags, copy button
4. **Hero section** — Full-width image or gradient, title, subtitle, CTA
5. **Table of contents** — Vertical list of links, active state highlighting, sticky on scroll
6. **Code block** — Syntax highlighted, language label, copy button, line numbers (optional)
7. **Blockquote** — Left border accent, italic text, citation
8. **Table** — Alternating row colors, header row distinct, responsive (horizontal scroll on mobile)
9. **FAQ accordion** — Question (clickable), answer (expandable), smooth animation
10. **Related posts** — 3 cards in a row, thumbnail + title + date
11. **Category/tag pills** — Rounded rectangles, accent color border, hover fill
12. **Filter bar** — Dropdown or pill-based filter for prompt archive
13. **Pagination** — Page numbers or load more button
14. **Footer** — 3 columns (about, topics, social), copyright
15. **PDF download button** — Icon + text, prominent placement in article template
16. **Reading time badge** — Small, subtle, near article title
17. **Author byline** — Avatar (optional), name, date
18. **Newsletter signup** — Email input, submit button (for future use)

---

## Responsive Breakpoints

```
Mobile:      320px - 767px (single column, stacked)
Tablet:      768px - 1023px (2-column grid)
Desktop:     1024px - 1279px (3-column grid, sidebar optional)
Wide:        1280px+ (max-width container, centered)
```

**Mobile priorities:**
- Article body must be readable without zooming (16px base, 1.7 line height)
- Code blocks must scroll horizontally (no wrapping)
- Navigation collapses to hamburger menu
- Prompt cards stack vertically
- Tables scroll horizontally

---

## Special Features

### Code Syntax Highlighting
- Use Prism.js or highlight.js (lightweight, dark theme)
- Support: Python, Bash, JavaScript, YAML, JSON, Markdown, PHP
- Copy-to-clipboard button on every code block
- Language label in top-right corner of code block

### Table of Contents
- Auto-generated from h2 and h3 headings in article
- Sticky on desktop (follows scroll)
- Active section highlighting
- Collapsible on mobile

### Copy-to-Clipboard (Prompt Cards)
- One-click copy
- Visual feedback (button changes to "Copied!" for 2 seconds)
- No library dependency (use navigator.clipboard API)

### FAQ Schema Markup
- JSON-LD structured data for FAQ sections
- Improves SEO (Google FAQ rich results)

### Article Schema Markup
- JSON-LD for Article, NewsArticle
- Author, datePublished, dateModified, image, publisher

---

## Performance Budget

- Total CSS: < 50KB (minified)
- Total JS: < 30KB (minified, excluding syntax highlighter)
- Fonts: System fonts only (or self-hosted, < 100KB total)
- Images: WebP format, lazy loading for below-fold images
- Target: 95+ PageSpeed score, < 2s LCP

---

## SEO Requirements

- Clean permalink structure: /articles/slug for articles, /prompts/slug for bundles
- Meta title and description fields (via Yoast SEO or similar)
- Open Graph tags for social sharing
- Twitter Card tags
- Schema markup: Article, FAQPage, BreadcrumbList
- XML sitemap (via plugin)
- Breadcrumb navigation on all pages

---

## What This Theme Does NOT Need

- WooCommerce (no e-commerce)
- User registration/login (not yet)
- Comments (not yet — can be added later)
- Forum/bbPress
- Multi-language (English only)
- Page builder compatibility (Gutenberg only)
- Widget-ready sidebars (optional, not priority)
- Customizer options (hardcode the design)
- RTL support (English only)

---

## Design Inspiration (Not Copy)

- **GitHub** — Dark theme, code readability, monospace usage
- **Linear** — Clean UI, subtle animations, dark mode
- **Vercel Blog** — Typography, spacing, article layout
- **Tailwind CSS docs** — Code blocks, navigation, search
- **Hacker News** — Content-first, no-nonsense layout (but modernized)

The theme should feel like it belongs alongside these sites — technically credible, content-focused, no visual noise.
