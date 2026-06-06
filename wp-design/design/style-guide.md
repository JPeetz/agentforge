# Style Guide — AgentForge WordPress Theme

**Version:** 1.0
**Date:** 2026-05-15

---

## Design System Name: "The Forge"

Dark mode default. Amber accent. Editorial magazine aesthetic with terminal-inspired code elements. Premium, technical, distinctive.

---

## Colour Palette

### Dark Mode (Default)

| Token | Hex | Usage |
|-------|-----|-------|
| `--bg-primary` | `#0f0f1a` | Page background |
| `--bg-secondary` | `#1a1a2e` | Cards, surfaces |
| `--bg-tertiary` | `#252540` | Elevated surfaces, hover states |
| `--text-primary` | `#e8e8f0` | Body text, headings |
| `--text-secondary` | `#9090b0` | Meta text, captions |
| `--text-tertiary` | `#606080` | Disabled, placeholder |
| `--accent` | `#f59e0b` | Links, buttons, highlights |
| `--accent-hover` | `#fbbf24` | Hover state |
| `--accent-muted` | `#78350f` | Accent backgrounds |
| `--border` | `#2d2d44` | Borders, dividers |
| `--border-hover` | `#404060` | Border hover |
| `--success` | `#34d399` | Success states |
| `--error` | `#f87171` | Error states |

### Light Mode

| Token | Hex | Usage |
|-------|-----|-------|
| `--bg-primary` | `#fafafa` | Page background |
| `--bg-secondary` | `#ffffff` | Cards, surfaces |
| `--bg-tertiary` | `#f0f0f5` | Elevated surfaces |
| `--text-primary` | `#1a1a2e` | Body text, headings |
| `--text-secondary` | `#4a4a6a` | Meta text |
| `--text-tertiary` | `#8a8aa0` | Disabled |
| `--accent` | `#d97706` | Links, buttons |
| `--accent-hover` | `#b45309` | Hover |
| `--accent-muted` | `#fef3c7` | Accent backgrounds |
| `--border` | `#e2e2ec` | Borders |
| `--border-hover` | `#c0c0d0` | Border hover |

### Category Colours

| Category | Hex | Usage |
|----------|-----|-------|
| Articles | `#6366f1` (indigo) | Article cards, tags |
| Prompts | `#10b981` (emerald) | Prompt cards, tags |
| Research | `#8b5cf6` (violet) | Research cards, tags |
| Tools | `#f59e0b` (amber) | Tool cards, tags |

---

## Typography

### Font Stack

```css
--font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
--font-mono: "SF Mono", "Fira Code", "Fira Mono", "Roboto Mono", monospace;
--font-serif: "Georgia", "Times New Roman", serif;
```

**Primary:** System font stack (sans-serif for UI, headings, body)
**Monospace:** For code blocks, terminal elements, prompt text
**Optional web font:** Inter or Geist (load only if design needs it)

### Type Scale

```css
--text-xs: 0.75rem;    /* 12px */
--text-sm: 0.875rem;   /* 14px */
--text-base: 1.125rem; /* 18px — body text */
--text-lg: 1.25rem;    /* 20px */
--text-xl: 1.5rem;     /* 24px — h4 */
--text-2xl: 1.875rem;  /* 30px — h3 */
--text-3xl: 2.25rem;   /* 36px — h2 */
--text-4xl: 3rem;      /* 48px — h1 */
--text-5xl: 3.75rem;   /* 60px — display/hero */
```

### Line Heights

```css
--leading-tight: 1.2;    /* Headings */
--leading-snug: 1.4;     /* Subheadings */
--leading-normal: 1.7;   /* Body text */
--leading-relaxed: 1.8;  /* Long-form reading */
```

### Font Weights

```css
--font-normal: 400;
--font-medium: 500;
--font-semibold: 600;
--font-bold: 700;
```

### Body Text Rules

- Font size: 18px (1.125rem) minimum
- Line height: 1.7
- Max width: 700px (for reading comfort)
- Paragraph spacing: 1em
- Word spacing: normal
- Letter spacing: normal (slight tracking on headings: -0.02em)

---

## Spacing Scale

```css
--space-1: 0.25rem;   /* 4px */
--space-2: 0.5rem;    /* 8px */
--space-3: 0.75rem;   /* 12px */
--space-4: 1rem;      /* 16px */
--space-5: 1.25rem;   /* 20px */
--space-6: 1.5rem;    /* 24px */
--space-8: 2rem;      /* 32px */
--space-10: 2.5rem;   /* 40px */
--space-12: 3rem;     /* 48px */
--space-16: 4rem;     /* 64px */
--space-20: 5rem;     /* 80px */
--space-24: 6rem;     /* 96px */
```

---

## Layout

### Breakpoints

```css
--bp-sm: 640px;    /* Mobile landscape */
--bp-md: 768px;    /* Tablet */
--bp-lg: 1024px;   /* Desktop */
--bp-xl: 1280px;   /* Wide desktop */
--bp-2xl: 1536px;  /* Ultra-wide */
```

### Container Widths

```css
--container-sm: 640px;
--container-md: 768px;
--container-lg: 1024px;
--container-xl: 1200px;
--content-max: 700px;   /* Max reading width */
```

### Grid

- Homepage: 3-column card grid on desktop, 2 on tablet, 1 on mobile
- Article layout: Single column (700px max) with optional sidebar on desktop
- Archive: 3-column card grid on desktop, 2 on tablet, 1 on mobile
- Gap: 1.5rem (24px) between cards

---

## Components

### Header

- Height: 64px (desktop), 56px (mobile)
- Background: `--bg-primary` with subtle bottom border
- Position: Sticky (stays at top on scroll)
- Logo: Left-aligned, max height 36px
- Navigation: Centre or right-aligned
- Search icon: Right side
- Dark mode toggle: Right side (sun/moon icon)
- Mobile: Hamburger menu

### Hero

- Full-width image area
- Aspect ratio: 16:9 (desktop), 4:3 (mobile)
- Max height: 60vh
- Overlay: Gradient from transparent to `--bg-primary` at bottom
- Text: Centred or left-aligned on top of image
- Category pages: Smaller hero (40vh max)

### Cards

- Background: `--bg-secondary`
- Border: 1px solid `--border`
- Border radius: 8px
- Padding: 1.5rem
- Image: Top of card, 16:9 aspect ratio
- Title: `--text-xl`, `--font-semibold`, `--leading-tight`
- Excerpt: `--text-sm`, `--text-secondary`, `--leading-normal`
- Meta: `--text-xs`, `--text-tertiary`
- Hover: Border colour changes to `--border-hover`, subtle lift (translateY -2px)

### Buttons

**Primary:**
- Background: `--accent`
- Text: `--bg-primary` (dark text on amber)
- Padding: 0.75rem 1.5rem
- Border radius: 6px
- Font weight: 600
- Hover: `--accent-hover`

**Secondary:**
- Background: transparent
- Border: 1px solid `--border`
- Text: `--text-primary`
- Hover: Border `--border-hover`, background `--bg-tertiary`

### Code Blocks

- Background: `--bg-tertiary`
- Border: 1px solid `--border`
- Border radius: 6px
- Font: `--font-mono`, `--text-sm`
- Line height: 1.6
- Padding: 1.5rem
- Overflow-x: auto
- Copy button: Top-right corner, appears on hover
- Syntax highlighting: Prism.js or highlight.js (lightweight)

### Pull Quotes

- Border-left: 3px solid `--accent`
- Padding-left: 1.5rem
- Font: `--text-lg`, `--font-serif` (italic)
- Colour: `--text-secondary`
- Margin: 2rem 0

### Table of Contents

- Position: Sidebar on desktop (sticky), inline on mobile
- Background: `--bg-secondary`
- Border: 1px solid `--border`
- Border radius: 8px
- Padding: 1rem
- Link colour: `--text-secondary`
- Active link: `--accent`
- Hover: `--text-primary`

### Tags

- Display: Inline-block
- Background: `--bg-tertiary`
- Border: 1px solid `--border`
- Border radius: 9999px (pill)
- Padding: 0.25rem 0.75rem
- Font: `--text-xs`
- Hover: Border `--accent`, text `--accent`

### Footer

- Background: `--bg-secondary`
- Border-top: 1px solid `--border`
- Padding: 3rem 0
- Columns: 3 on desktop (About, Categories, Social), 1 on mobile
- Text: `--text-sm`, `--text-secondary`
- Links: `--text-secondary` → `--accent` on hover

---

## Images

### File Locations

```
graphics/
├── hero-home.png         → Homepage hero
├── hero-content.png      → Article archive hero
├── hero-prompts.png      → Prompt library hero
├── og-default.png        → Open Graph default
├── icons/
│   ├── icon-articles.png → Articles category
│   ├── icon-prompts.png  → Prompts category
│   └── icon-research.png → Research category
└── favicon/              → (to be generated)
```

### Image Specifications

| Type | Format | Max Width | Quality |
|------|--------|-----------|---------|
| Hero | WebP | 1920px | 80% |
| Card thumbnail | WebP | 600px | 80% |
| Content image | WebP | 1200px | 85% |
| OG image | PNG | 1200x630 | 90% |
| Icons | SVG or PNG | 64x64 | - |
| Favicon | PNG | 32x32, 16x16 | - |

### Image Treatment

- All images: `loading="lazy"` (except hero)
- All images: Explicit `width` and `height` attributes
- All images: `decoding="async"`
- Hero image: `<link rel="preload">` in `<head>`
- Responsive images: `srcset` with 1x, 2x variants
- Alt text: Descriptive, not decorative

---

## Motion & Transitions

### Principles

- Minimal motion. This is a content site, not a portfolio.
- Transitions should be fast (150-200ms) and subtle.
- Respect `prefers-reduced-motion`.

### Transitions

```css
--transition-fast: 150ms ease;
--transition-normal: 200ms ease;
--transition-slow: 300ms ease;
```

### Animated Elements

- Card hover: `transform: translateY(-2px)`, `border-color` change
- Link hover: `color` change
- Button hover: `background-color` change
- Dark mode toggle: Icon rotation (150ms)
- Page transitions: None (instant)
- Scroll progress: Thin bar at top, width 0→100%

### Reduced Motion

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## Dark Mode Toggle

### Default Behaviour

1. Check `localStorage` for saved preference
2. If saved, apply saved theme
3. If not saved, check `prefers-color-scheme` media query
4. Default to dark if no preference detected

### Toggle Button

- Position: Header, right side
- Icon: Sun (light mode) / Moon (dark mode)
- Accessible: `aria-label="Toggle dark mode"`
- Keyboard: Focusable, Enter/Space to toggle

---

## Accessibility

### Requirements

- WCAG 2.1 AA minimum
- All text meets 4.5:1 contrast ratio (normal text) or 3:1 (large text)
- All interactive elements are keyboard accessible
- Focus indicators visible (2px solid `--accent`)
- Skip navigation link
- ARIA labels on all interactive elements
- Semantic HTML throughout

### Focus Styles

```css
:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
```

---

## Responsive Behaviour

### Mobile (< 640px)

- Single column layout
- Header: Hamburger menu
- Cards: Full width, stacked
- Hero: 40vh max
- TOC: Collapsible, below title
- Font size: 16px base
- Padding: 1rem

### Tablet (640px - 1024px)

- Two-column card grid
- Header: Visible nav (condensed)
- Hero: 50vh max
- TOC: Sidebar (if space permits)
- Font size: 17px base
- Padding: 1.5rem

### Desktop (1024px+)

- Three-column card grid
- Full header navigation
- Hero: 60vh max
- TOC: Sticky sidebar
- Article: Content + sidebar layout
- Font size: 18px base
- Padding: 2rem
