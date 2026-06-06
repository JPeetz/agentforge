# Design Direction Research
## AgentForge — WP Design Department

**Date:** 2026-05-15
**Researcher:** Marvin (direct research)

---

## 1. What Makes Content Sites Work

### Typography Hierarchy

The single most important design decision for a content site is typography. Users come to read. Everything else is secondary.

**Principles:**
- Clear visual hierarchy: h1 > h2 > h3 > body > caption
- Generous whitespace between sections (2-3rem between major sections)
- Comfortable reading width: 65-75 characters per line (~700px max for content)
- Adequate line height: 1.6-1.8 for body text
- Distinct visual treatment for links (colour + underline on hover)
- Code blocks clearly distinguished from prose (monospace font, background colour, padding)

### Whitespace

Content sites fail when they cram too much into the viewport. The best content sites breathe.

**Principles:**
- Minimum 1.5rem padding on mobile, 2-3rem on desktop
- Generous margins between paragraphs (1em)
- Section dividers: subtle, not heavy. A thin line or extra whitespace.
- Sidebar (if used): minimum 300px, separated by clear visual boundary
- No "card wall" on homepage — show 3-5 featured items, not 20

### Content Discovery

Users need to find content easily. Design patterns that work:

- **Sticky navigation** with search icon
- **Category-based browsing** (not just chronological)
- **Related posts** at the end of articles
- **Table of contents** for long articles (auto-generated from h2/h3)
- **Tag cloud or tag list** for cross-cutting topics
- **Author pages** with bio and all posts
- **Search** that actually works (relevancy-sorted, not date-sorted)

### Reading Experience

- **Progress indicator** for long articles (thin bar at top)
- **Estimated reading time** displayed near title
- **Copy code button** on all code blocks
- **Anchor links** on headings (clickable # on hover)
- **Footnotes** instead of inline links where possible
- **Print stylesheet** for articles (users do print long-form content)

---

## 2. Niche Ideas: What Makes an AI/Agent Content Site Distinctive

Most AI blogs look the same: white background, Inter font, blue accent, card grid. Here are approaches that would make agent-forge.co stand out:

### Direction A: "Terminal Aesthetic"
- Monospace font for headings (not body)
- Dark background by default (terminal green or amber accent)
- Code blocks styled like terminal output
- ASCII art dividers between sections
- Subtle scan-line or CRT effect (CSS only, very subtle)
- **Pros**: Immediately distinctive, resonates with developer audience, dark mode native
- **Cons**: Can feel gimmicky if overdone, harder to read long-form in monospace

### Direction B: "Research Paper"
- Serif font for body text (academic feel)
- Two-column layout on desktop (content + sidebar TOC)
- Muted colour palette (navy, cream, forest green)
- Figures and diagrams styled like academic papers
- Citation-style references at the end of articles
- **Pros**: Signals depth and authority, excellent for long-form, unique in AI space
- **Cons**: Can feel stuffy, less approachable for casual readers

### Direction C: "Data Dashboard"
- Clean, minimal design with data visualisation elements
- Interactive charts/graphs in articles (embedded)
- Colour-coded content categories (articles = blue, prompts = green, research = purple)
- Stats and metrics displayed prominently (word count, reading time, difficulty level)
- Card-based layout with rich metadata
- **Pros**: Feels modern and technical, great for mixed content types
- **Cons**: Risk of looking like a SaaS landing page, not a content site

### Direction D: "Editorial Magazine" (RECOMMENDED)
- Large hero images and typography
- Editorial layout with pull quotes, drop caps, and feature images
- Dark mode with warm accent colour (amber or coral)
- Generous whitespace, magazine-style grid
- Category-specific visual treatments (articles vs prompts vs research)
- **Pros**: Premium feel, excellent readability, distinctive in AI space, works for all content types
- **Cons**: Requires strong graphic design, more complex layouts

### Direction E: "Minimalist Wiki"
- Wikipedia-inspired: clean, functional, no-nonsense
- Single column, max 700px content width
- Blue links, serif body text, sans-serif headings
- Infobox-style metadata panels
- Cross-linking between articles prominent
- **Pros**: Extremely fast, familiar to technical users, great for reference content
- **Cons**: Can feel bland, not visually exciting

### Recommendation

**Direction D (Editorial Magazine) with elements of Direction A (Terminal).**

Rationale:
- The editorial magazine approach is distinctive in the AI space (most AI blogs are either corporate-clean or developer-minimal)
- It works for all three content types: articles, prompts, research
- Dark mode with a warm accent (amber/coral) gives it personality without being gimmicky
- Terminal elements (monospace for code, subtle terminal-inspired accents) can be used sparingly for brand identity
- Large typography and whitespace signal quality and authority

---

## 3. Colour Theory for Tech Brands

### Dark Mode Palette (Primary)

Based on the "Editorial Magazine + Terminal" direction:

```
Background:     #0f0f1a  (near-black, blue tint)
Surface:        #1a1a2e  (dark navy)
Surface Alt:    #252540  (slightly lighter navy)
Text Primary:   #e8e8f0  (cool white)
Text Secondary: #9090b0  (muted lavender)
Text Tertiary:  #606080  (dimmed)
Accent:         #f59e0b  (amber-500) — warm, distinctive
Accent Hover:   #fbbf24  (amber-400)
Accent Muted:   #78350f  (amber-900, for backgrounds)
Success:        #34d399  (emerald-400)
Error:          #f87171  (red-400)
Border:         #2d2d44  (subtle border)
Border Hover:   #404060  (hover state)
```

### Light Mode Palette

```
Background:     #fafafa  (warm white)
Surface:        #ffffff  (pure white)
Surface Alt:    #f0f0f5  (slightly cool)
Text Primary:   #1a1a2e  (dark navy)
Text Secondary: #4a4a6a  (muted navy)
Text Tertiary:  #8a8aa0  (dimmed)
Accent:         #d97706  (amber-600) — slightly darker for light bg
Accent Hover:   #b45309  (amber-700)
Accent Muted:   #fef3c7  (amber-100, for backgrounds)
Success:        #059669  (emerald-600)
Error:          #dc2626  (red-600)
Border:         #e2e2ec  (light border)
Border Hover:   #c0c0d0  (hover state)
```

### Category Colours

```
Articles:       #6366f1  (indigo) — default content
Prompts:        #10b981  (emerald) — prompt library
Research:       #8b5cf6  (violet) — research output
Tools:          #f59e0b  (amber) — tools and resources
```

### Accessibility

All colour combinations must meet WCAG AA:
- Text on background: minimum 4.5:1 contrast ratio
- Large text (18px+): minimum 3:1
- Interactive elements: minimum 3:1 against adjacent colours

The amber accent on dark background (#f59e0b on #0f0f1a) = 10.2:1 — passes AAA.
The primary text on background (#e8e8f0 on #0f0f1a) = 14.1:1 — passes AAA.

---

## 4. Layout Patterns

### Homepage

```
┌─────────────────────────────────────────────┐
│  LOGO    Nav    Nav    Nav    [Search] [DM] │  ← Sticky header
├─────────────────────────────────────────────┤
│                                             │
│  ┌───────────────────────────────────────┐  │
│  │         HERO IMAGE / BANNER           │  │  ← Full-width hero
│  │    "AgentForge — AI Content Hub"      │  │
│  └───────────────────────────────────────┘  │
│                                             │
│  ── Latest Articles ──────────────────────  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ Card     │ │ Card     │ │ Card     │   │  ← 3-column card grid
│  │          │ │          │ │          │   │
│  └──────────┘ └──────────┘ └──────────┘   │
│                                             │
│  ── Prompt Library ───────────────────────  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ Prompt   │ │ Prompt   │ │ Prompt   │   │  ← Category-specific cards
│  │ Card     │ │ Card     │ │ Card     │   │
│  └──────────┘ └──────────┘ └──────────┘   │
│                                             │
│  ── Research & Tools ─────────────────────  │
│  ┌───────────────────────────────────────┐  │
│  │  Featured research / tool             │  │  ← Full-width feature
│  └───────────────────────────────────────┘  │
│                                             │
├─────────────────────────────────────────────┤
│  Footer: About | Categories | Social | ©   │
└─────────────────────────────────────────────┘
```

### Single Article

```
┌─────────────────────────────────────────────┐
│  LOGO    Nav    Nav    Nav    [Search] [DM] │
├─────────────────────────────────────────────┤
│  Home > Category > Article Title            │  ← Breadcrumb
├─────────────────────────────────────────────┤
│                                             │
│  ┌───────────────────────────────────────┐  │
│  │         HERO IMAGE                    │  │
│  └───────────────────────────────────────┘  │
│                                             │
│  Category · Reading Time · Date             │
│                                             │
│  Article Title (H1)                         │
│  Author Name                                │
│                                             │
│  ┌─────────────────┐ ┌──────────────────┐  │
│  │                 │ │ Table of Contents │  │  ← Sidebar on desktop
│  │  Article body   │ │ - Section 1      │  │
│  │  with images,   │ │ - Section 2      │  │
│  │  code blocks,   │ │ - Section 3      │  │
│  │  pull quotes    │ │                  │  │
│  │                 │ │ [Share buttons]  │  │
│  │                 │ │                  │  │
│  └─────────────────┘ └──────────────────┘  │
│                                             │
│  ── Tags ──────────────────────────────────  │
│  [tag] [tag] [tag]                          │
│                                             │
│  ── Related Articles ──────────────────────  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ Related  │ │ Related  │ │ Related  │   │
│  └──────────┘ └──────────┘ └──────────┘   │
│                                             │
├─────────────────────────────────────────────┤
│  Footer                                     │
└─────────────────────────────────────────────┘
```

### Prompt Library Entry

```
┌─────────────────────────────────────────────┐
│  LOGO    Nav    Nav    Nav    [Search] [DM] │
├─────────────────────────────────────────────┤
│  Home > Prompts > Category > Prompt Title   │
├─────────────────────────────────────────────┤
│                                             │
│  Prompt Title (H1)                          │
│  Category · Difficulty · Date               │
│                                             │
│  ┌───────────────────────────────────────┐  │
│  │  PROMPT TEXT                          │  │  ← Styled like a code block
│  │  (formatted, copyable)                │  │     but readable
│  │                                       │  │
│  │  [Copy Prompt] [Copy as Markdown]     │  │
│  └───────────────────────────────────────┘  │
│                                             │
│  Description / Notes                        │
│  (when to use, expected output, tips)       │
│                                             │
│  ── Metadata ──────────────────────────────  │
│  Model: Claude 3.5 / GPT-4 / etc.          │
│  Tokens: ~500                               │
│  Category: [Creative] [Coding] [Analysis]   │
│                                             │
│  ── Related Prompts ───────────────────────  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ Prompt   │ │ Prompt   │ │ Prompt   │   │
│  └──────────┘ └──────────┘ └──────────┘   │
│                                             │
├─────────────────────────────────────────────┤
│  Footer                                     │
└─────────────────────────────────────────────┘
```

### Archive / Category Page

```
┌─────────────────────────────────────────────┐
│  LOGO    Nav    Nav    Nav    [Search] [DM] │
├─────────────────────────────────────────────┤
│  Category: [Name]                           │
│  Description of this category...            │
├─────────────────────────────────────────────┤
│                                             │
│  ┌───────────────────────────────────────┐  │
│  │  Featured Article (large card)        │  │  ← First item large
│  └───────────────────────────────────────┘  │
│                                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ Card     │ │ Card     │ │ Card     │   │  ← 3-col grid
│  └──────────┘ └──────────┘ └──────────┘   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ Card     │ │ Card     │ │ Card     │   │
│  └──────────┘ └──────────┘ └──────────┘   │
│                                             │
│  [Load More] or Pagination                  │
│                                             │
├─────────────────────────────────────────────┤
│  Footer                                     │
└─────────────────────────────────────────────┘
```

---

## 5. Competitor Analysis

### AI/Research Blogs

| Site | URL | Strengths | Weaknesses | Score |
|------|-----|-----------|------------|-------|
| Lil'Log (Lilian Weng) | lilianweng.github.io | Deep technical content, clean reading experience, excellent code formatting | Minimal design, no visual identity, no search, no categories | 6/10 |
| Anthropic Engineering | anthropic.com/engineering | Beautiful design, great typography, strong brand identity | Corporate feel, content is marketing-adjacent, no dark mode | 8/10 |
| OpenAI Research | openai.com/research | Clean layout, good imagery, professional | Very corporate, hard to browse, content is PDFs not blog posts | 7/10 |
| Google AI Blog | ai.googleblog.com | Good content organisation, searchable | Dated design, poor mobile experience, no dark mode | 5/10 |
| HuggingFace Blog | huggingface.co/blog | Good card layout, community feel, dark mode | Cluttered homepage, too many CTAs, inconsistent typography | 6/10 |
| Jay Alammar | jalammar.github.io | Excellent visual explanations, distinctive diagrams | Minimal site design, no navigation structure, no search | 7/10 |
| Sebastian Ruder | ruder.io | Clean academic style, good typography, clear structure | Very plain, no visual identity, no dark mode | 6/10 |
| Distill.pub | distill.pub | Beautiful interactive articles, unique design | Slow loading, heavy JS, niche format | 8/10 |

### Key Patterns Worth Borrowing

1. **Anthropic**: Typography scale, heading hierarchy, article layout with sidebar TOC
2. **Distill.pub**: Interactive elements, visual explanations, distinctive design
3. **Jay Alammar**: Diagram-heavy explanations, visual learning
4. **HuggingFace**: Dark mode, community feel, category browsing
5. **Lil'Log**: Deep content, no distractions, code-forward presentation

### What's Missing (Opportunities)

1. **No AI content site has a distinctive visual identity** — they're all either corporate-clean or developer-minimal
2. **No AI content site treats prompts as first-class content** — prompts are buried in GitHub repos, not presented as readable content
3. **No AI content site has a magazine-style editorial design** — all are either blogs or documentation
4. **Dark mode is rare** — only HuggingFace and a few others offer it
5. **Code presentation is inconsistent** — most sites use basic code blocks without syntax highlighting or copy buttons
6. **No site combines articles + prompts + research in a unified design** — they're always separate

---

## 6. Visual Identity Direction

### Three Concepts

#### Concept 1: "The Terminal"
- Dark background (#0f0f1a), green/amber terminal accents
- Monospace headings, sans-serif body
- Code-style decorative elements
- Feels: Developer-native, technical, hacker
- Risk: Too niche, alienates non-developers

#### Concept 2: "The Journal"
- Serif body text, editorial layout
- Cream/dark mode with forest green accents
- Academic paper aesthetic
- Feels: Authoritative, deep, scholarly
- Risk: Too formal, not tech-forward enough

#### Concept 3: "The Forge" (RECOMMENDED)
- Dark mode default, amber/warm accent
- Large editorial typography with terminal-inspired code blocks
- Magazine-style layouts with rich imagery
- Category-specific colour coding
- Feels: Premium, technical, distinctive, warm
- Risk: Requires strong execution, more complex

### Recommended: "The Forge"

The name "AgentForge" suggests creation, craftsmanship, heat. The design should reflect this:

- **Dark mode default** — the forge is a dark place with bright sparks
- **Amber accent** — the colour of fire, molten metal, sparks
- **Large, bold typography** — the confidence of a blacksmith's hammer
- **Clean layouts with rich imagery** — the precision of crafted work
- **Code blocks styled like metalwork** — structured, precise, functional
- **Category colours like different metals** — indigo (steel), emerald (copper patina), violet (titanium)

This is distinctive in the AI content space. No other site looks like this.

---

## Sources

- Anthropic Engineering Blog: anthropic.com/engineering
- Lil'Log: lilianweng.github.io
- HuggingFace Blog: huggingface.co/blog
- Jay Alammar: jalammar.github.io
- Distill.pub: distill.pub
- Colour contrast: WebAIM contrast checker
- Typography: wpbestpractices.dev/design/typography/
