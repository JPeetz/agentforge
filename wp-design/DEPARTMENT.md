# WP Design Department — AgentForge

## Mission

Research, design, and build a custom WordPress theme for agent-forge.co that reflects the output of the Content Department and Prompt Department (PD). The theme must be ready to deploy once WP login credentials are provided.

No e-commerce. No subscriptions. No payment gating. Pure content presentation.

## Scope

### IN SCOPE
- WordPress theme development from scratch (not a child theme, not a page builder)
- Template hierarchy: home, single post, archive, category, search, 404, page
- Responsive design (mobile-first)
- Typography, colour scheme, layout grids
- Hero images, icons, and visual identity via fal.ai
- Performance (Core Web Vitals, lazy loading, minimal JS)
- SEO-friendly markup (schema.org, Open Graph, clean HTML)
- Content-focused design that showcases:
  - Content pipeline articles (long-form, SEO-optimised)
  - PD research output (prompt libraries, prompt engineering guides)
  - Mixed media: text, images, code blocks, data tables

### OUT OF SCOPE (for now)
- WooCommerce / e-commerce
- Stripe / payment processing
- User registration / login / memberships
- IONOS deployment specifics
- Plugin recommendations beyond theme functionality
- Multilingual

## Organisation

### Department Head: Marvin (OWL)
- Coordinates research agents
- Consolidates findings into theme specification
- Reviews all output before it goes to the spec document
- Reports progress to operator

### Research Agent 1: WP Theme Developer
- Researches WordPress theme development from scratch
- Template hierarchy, required files, best practices
- Modern WP theme standards (block theme vs classic)
- Performance patterns
- Outputs to research/wp-theme-development.md

### Research Agent 2: Content Site Designer
- Researches design patterns for content-heavy sites
- What makes content sites work (typography, whitespace, readability)
- Competitor analysis: top content sites in AI/tech niche
- Colour theory for tech/content brands
- Outputs to research/design-direction.md

### Graphics Agent: Visual Identity Designer
- Uses fal.ai to create:
  - Hero images for homepage and section pages
  - Category icons (content, prompts, research, tools)
  - Background textures / patterns
  - Open Graph default image
  - Favicon set
- Outputs to graphics/

## Workspace

```
workspace/agentforge/wp-design/
├── research/
│   ├── wp-theme-development.md
│   ├── design-direction.md
│   └── competitor-analysis.md
├── design/
│   ├── style-guide.md
│   ├── wireframes.md
│   └── component-library.md
├── graphics/
│   ├── hero-home.png
│   ├── hero-content.png
│   ├── hero-prompts.png
│   ├── og-default.png
│   ├── icons/
│   └── favicon/
├── theme-spec/
│   ├── theme-specification.md    <-- THE deliverable
│   └── technical-requirements.md
└── assets/
    └── (downloaded references, screenshots, etc.)
```

## Deliverables

### Phase 1: Research (NOW)
1. `research/wp-theme-development.md` — how to build a WP theme from scratch
2. `research/design-direction.md` — what the theme should look like and why
3. `research/competitor-analysis.md` — what other content sites do well

### Phase 2: Design
4. `design/style-guide.md` — colours, typography, spacing, components
5. `design/wireframes.md` — layout for key templates
6. `design/component-library.md` — reusable design elements

### Phase 3: Graphics
7. Hero images, icons, favicon in `graphics/`

### Phase 4: Specification
8. `theme-spec/theme-specification.md` — complete theme spec ready for development
9. `theme-spec/technical-requirements.md` — file structure, dependencies, build process

## Design Principles

1. **Content first** — the design serves the content, not the other way around
2. **Fast** — every design decision is filtered through performance
3. **Readable** — typography and spacing optimised for long-form reading
4. **Distinctive** — doesn't look like every other AI blog
5. **Scalable** — works for 10 articles and 10,000
6. **Dark mode** — default or option, this is a tech audience

## Technical Constraints

- WordPress 6.4+ (latest stable)
- PHP 8.0+
- No page builders (Elementor, Divi, etc.)
- Minimal JavaScript (vanilla, no frameworks unless essential)
- CSS: custom properties, mobile-first, no heavy frameworks
- Images: WebP with PNG fallbacks, lazy loaded
- Fonts: system font stack preferred, max 1 web font

## Timeline

Research agents start immediately. Graphics agent starts once design direction is established. Theme spec is the final deliverable before actual theme development begins.

## Operator Notes

- WP login credentials not yet available — theme must be deployable via standard WP theme upload or FTP once credentials are provided
- Content pipeline and PD research are the two primary content types
- The site is agent-forge.co (domain registered, hosting TBD)
- All text in GB-UK English
- All currency in EUR (for future use)
