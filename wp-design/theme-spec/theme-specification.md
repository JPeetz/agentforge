# Theme Specification — AgentForge WordPress Theme

**Version:** 1.0
**Date:** 2026-05-15
**Status:** Research complete. Ready for development.

---

## 1. Executive Summary

Build a custom WordPress theme called "AgentForge" from scratch. Classic PHP architecture (not block theme). Dark mode default with amber accent. Editorial magazine aesthetic. Content-first design. Optimised for Core Web Vitals. Ready to deploy via standard WP theme upload once login credentials are provided.

**Design system name:** "The Forge"

---

## 2. Site Overview

**Domain:** agent-forge.co
**Content types:**
- Articles (long-form, SEO-optimised, AI/agent topics)
- Prompts (prompt library entries, copyable, categorised)
- Research (research output, data, analysis)

**Target audience:** AI practitioners, prompt engineers, developers, researchers

**Tone:** Authoritative, technical, premium. Not corporate. Not a blog. A content platform.

---

## 3. Technical Architecture

### Classic PHP Theme

**Decision:** Classic PHP theme (not FSE/block theme). Rationale:
- Maximum performance control
- Precise HTML output
- No non-technical editors needed
- Better for custom content types
- Leaner markup

### Required Files

```
agentforge/
├── style.css              /* Theme metadata + CSS */
├── index.php              /* Fallback */
├── functions.php          /* Theme setup, enqueues, hooks */
├── header.php             /* <head> + site header */
├── footer.php             /* Site footer */
├── front-page.php         /* Homepage */
├── home.php               /* Blog posts index */
├── single.php             /* Single post (default) */
├── single-article.php     /* Single article */
├── single-prompt.php      /* Single prompt */
├── archive.php            /* Default archive */
├── archive-article.php    /* Article archive */
├── archive-prompt.php    /* Prompt archive */
├── category.php           /* Category archive */
├── search.php             /* Search results */
├── 404.php                /* Not found */
├── page.php               /* Static page */
├── template-parts/        /* Reusable parts */
│   ├── content.php
│   ├── content-single.php
│   ├── content-card.php
│   ├── content-none.php
│   └── meta.php
├── assets/
│   ├── css/
│   │   ├── critical.css   /* Inlined above-fold CSS */
│   │   └── main.css       /* Full stylesheet */
│   ├── js/
│   │   ├── main.js        /* Minimal JS */
│   │   └── dark-mode.js   /* Dark mode toggle */
│   └── img/               /* Theme images */
├── inc/
│   ├── setup.php          /* Theme setup */
│   ├── enqueue.php        /* Script/style registration */
│   ├── template-tags.php  /* Custom template tags */
│   ├── customizer.php     /* Customizer settings */
│   └── schema.php         /* Structured data */
└── screenshot.png         /* Theme screenshot */
```

### WordPress & PHP Requirements

- WordPress 6.4+
- PHP 8.0+
- No plugins required for theme functionality
- No page builders
- No jQuery dependency

---

## 4. Design Specification

### Colour Palette

**Dark Mode (Default):**
- Background: `#0f0f1a`
- Surface: `#1a1a2e`
- Text: `#e8e8f0`
- Accent: `#f59e0b` (amber)
- Border: `#2d2d44`

**Light Mode:**
- Background: `#fafafa`
- Surface: `#ffffff`
- Text: `#1a1a2e`
- Accent: `#d97706`
- Border: `#e2e2ec`

**Category Colours:**
- Articles: `#6366f1` (indigo)
- Prompts: `#10b981` (emerald)
- Research: `#8b5cf6` (violet)
- Tools: `#f59e0b` (amber)

### Typography

**Font Stack (System):**
- Sans: `-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`
- Mono: `"SF Mono", "Fira Code", "Fira Mono", "Roboto Mono", monospace`

**Type Scale:**
- Body: 18px / 1.7 line height
- H1: 48px / 1.2
- H2: 36px / 1.2
- H3: 30px / 1.3
- H4: 24px / 1.3
- Small: 14px / 1.5
- Caption: 12px / 1.5

**Max content width:** 700px

### Layout

**Breakpoints:**
- Mobile: < 640px (single column)
- Tablet: 640-1024px (two column)
- Desktop: > 1024px (three column + sidebar)

**Container:** Max 1200px, centred

**Grid gaps:** 24px (1.5rem)

### Key Templates

**Homepage (front-page.php):**
- Full-width hero image
- Latest articles (3-column card grid)
- Prompt library preview (3-column card grid)
- Research/Tools feature section

**Single Article (single-article.php):**
- Hero image
- Breadcrumb navigation
- Title, author, meta
- Sidebar: Table of contents (sticky)
- Content (700px max)
- Tags
- Related articles (3-column)

**Single Prompt (single-prompt.php):**
- Title, category, difficulty
- Prompt text (styled code block, copyable)
- Description/notes
- Metadata (model, tokens, category)
- Related prompts

**Archive (archive.php / archive-article.php / archive-prompt.php):**
- Category title + description
- Featured item (large card)
- Card grid (3-column)
- Pagination

---

## 5. Performance Requirements

### Core Web Vitals Targets

| Metric | Target |
|--------|--------|
| LCP | < 2.0s (targeting under 2.5s threshold) |
| INP | < 100ms (targeting under 200ms threshold) |
| CLS | < 0.05 (targeting under 0.1 threshold) |

### Performance Strategies

1. **Critical CSS inlined** in `<head>` (above-fold only)
2. **Main stylesheet deferred** (`media="print"` swap trick)
3. **Zero JavaScript for content viewing** — JS only for dark mode toggle
4. **System font stack** — zero font network requests
5. **WebP images** — all images in WebP format
6. **Native lazy loading** — `loading="lazy"` on all below-fold images
7. **Image dimensions** — explicit width/height on all images
8. **Hero image preloaded** — `<link rel="preload">`
9. **No jQuery** — vanilla JS only
10. **Minimal DOM** — lean PHP templates, no unnecessary wrappers

### Budget

- Total page weight: < 500KB (excluding hero image)
- Hero image: < 150KB (WebP, 1920px)
- CSS: < 30KB (minified)
- JS: < 5KB (minified)
- Fonts: 0KB (system stack)

---

## 6. Graphics Assets

### Generated (via fal.ai)

| File | Location | Usage |
|------|----------|-------|
| hero-home.png | graphics/ | Homepage hero banner |
| hero-content.png | graphics/ | Article archive hero |
| hero-prompts.png | graphics/ | Prompt library hero |
| og-default.png | graphics/ | Open Graph default image |
| icon-articles.png | graphics/icons/ | Articles category icon |
| icon-prompts.png | graphics/icons/ | Prompts category icon |
| icon-research.png | graphics/icons/ | Research category icon |

### Still Needed

- Favicon set (16x16, 32x32, 180x180 Apple touch)
- Social media icons (Twitter/X, GitHub, LinkedIn)
- Logo (SVG, light + dark variants)
- 404 page illustration
- Search empty state illustration

---

## 7. SEO Requirements

### Structured Data
- Article schema on all article pages
- BreadcrumbList schema on all pages
- WebSite schema on homepage
- Organization schema in footer

### Meta Tags
- Open Graph (og:title, og:description, og:image, og:url, og:type)
- Twitter Card (summary_large_image)
- Canonical URLs
- Meta description (auto-generated from excerpt)

### Semantic HTML
- Single `<h1>` per page
- Proper heading hierarchy (no skipping)
- `<article>`, `<section>`, `<nav>`, `<aside>` elements
- `<time datetime="...">` for dates
- `<figure>` + `<figcaption>` for images

---

## 8. Accessibility Requirements

- WCAG 2.1 AA compliance
- All text meets 4.5:1 contrast ratio
- Keyboard navigation for all interactive elements
- Visible focus indicators (2px solid accent)
- Skip navigation link
- ARIA labels on interactive elements
- Alt text on all images
- `prefers-reduced-motion` respected

---

## 9. Dark Mode

### Default: Dark

The site defaults to dark mode. Users can toggle to light. Preference saved in `localStorage`.

### Implementation

- CSS custom properties for all colours
- `data-theme="dark"` / `data-theme="light"` on `<html>`
- Toggle button in header (sun/moon icon)
- Respects `prefers-color-scheme` on first visit
- Toggle JS: < 1KB, inline in `<head>`

---

## 10. Development Phases

### Phase 1: Research [COMPLETE]
- Theme architecture research
- Design direction research
- Competitor analysis
- Graphics generation

### Phase 2: Core Theme Development [NEXT]
- Set up theme skeleton (style.css, functions.php, index.php)
- Build header.php and footer.php
- Build front-page.php (homepage)
- Build single.php and single-article.php
- Build archive.php and category.php
- Build search.php and 404.php
- Implement dark mode toggle
- Implement critical CSS strategy

### Phase 3: Styling & Components
- Complete CSS (all components from style-guide.md)
- Responsive breakpoints
- Card layouts
- Code block styling
- TOC component
- Tag styling

### Phase 4: Content & Graphics
- Generate remaining graphics (favicon, logo, social icons)
- Convert all images to WebP
- Set up image sizes in functions.php
- Create default content templates

### Phase 5: Testing & Optimisation
- Core Web Vitals testing (PageSpeed Insights)
- Cross-browser testing (Chrome, Firefox, Safari, Edge)
- Mobile responsive testing
- Accessibility audit (WAVE, axe)
- Performance budget validation

### Phase 6: Deployment
- Package theme as .zip
- Deploy to WordPress (method TBD once credentials available)
- Verify all templates render correctly
- Test dark mode toggle
- Verify structured data (Google Rich Results Test)

---

## 11. File Reference

| File | Description |
|------|-------------|
| DEPARTMENT.md | Department organisation and scope |
| research/wp-theme-development.md | Technical research on WP theme building |
| research/design-direction.md | Design research and visual identity |
| research/competitor-analysis.md | Competitor analysis (in design-direction.md) |
| design/style-guide.md | Complete design system specification |
| theme-spec/theme-specification.md | This document |
| graphics/ | All generated images and assets |

---

## 12. Open Questions

1. **Logo:** Need to design the AgentForge logo (SVG, light + dark variants)
2. **Favicon:** Need to generate favicon set from logo
3. **Custom post types:** Should prompts be a custom post type or a category/tag taxonomy?
4. **Comments:** Enable comments on articles? (Recommend: no, for performance)
5. **Newsletter:** Email signup form? (Recommend: add later, not in v1)
6. **Analytics:** Which analytics? (Recommend: Plausible or Umami, privacy-focused, lightweight)
7. **Caching:** Server-side caching plugin? (Recommend: decide after deployment)

---

## 13. Success Criteria

The theme is complete when:

- [ ] All templates render correctly with sample content
- [ ] Core Web Vitals: LCP < 2.0s, INP < 100ms, CLS < 0.05
- [ ] Page weight < 500KB (excluding hero)
- [ ] Dark mode works and defaults correctly
- [ ] Responsive on mobile, tablet, desktop
- [ ] WCAG 2.1 AA compliant
- [ ] Structured data validates in Google Rich Results Test
- [ ] No JavaScript errors in console
- [ ] No PHP errors or warnings
- [ ] Ready to deploy via WP Admin theme upload
