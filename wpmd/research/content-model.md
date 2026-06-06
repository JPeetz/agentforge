# WordPress Content Model — AgentForge

**Date:** 2026-05-16
**Purpose:** Define the custom post types, fields, and taxonomies needed in WordPress to represent all AgentForge content.

---

## Custom Post Types

### 1. `article` (SEO Articles + Tutorials)

This is the primary post type. Both standard SEO articles and tutorials share the same structure; tutorials are distinguished by a taxonomy term and the presence of code blocks.

**Post Type Key:** `article`
**Supports:** title, editor, thumbnail, excerpt, author, revisions, custom fields

#### Custom Fields (Post Meta)

| Field Key | Type | Required | Source | Notes |
|-----------|------|----------|--------|-------|
| `_af_slug` | string | Yes | frontmatter `slug` | URL slug; may match WP slug |
| `_af_seo_description` | string | Yes | frontmatter `description` | Meta description, ~150-160 chars |
| `_af_keywords` | array | Yes | frontmatter `keywords` | SEO keywords, stored as serialized array |
| `_af_word_count` | integer | No | frontmatter `word_count` | Populated after QA |
| `_af_status` | string | Yes | frontmatter `status` | `draft`, `qa-passed`, `published` |
| `_af_hero_image` | string (URL) | Yes | body line 13 image reference | Hero image path/URL |
| `_af_inline_images` | array (JSON) | No | `[IMAGE: ...]` placeholders | Each entry: `{description, position}` |
| `_af_has_code_blocks` | boolean | Auto | body content detection | True if fenced code blocks present |
| `_af_has_tables` | boolean | Auto | body content detection | True if markdown tables present |
| `_af_has_faq` | boolean | Auto | body content detection | True if FAQ section present (always true for articles) |
| `_af_faq_items` | array (JSON) | No | FAQ section parsed | Each entry: `{question, answer}` for FAQ schema |
| `_af_bottom_line` | string (long text) | No | "The Bottom Line" section | Summary/CTA for sidebar or footer display |
| `_af_sources` | array (JSON) | No | Inline citations parsed | Each entry: `{text, url}` for source list |

#### Taxonomies

- **`af_tag`** (non-hierarchical, like tags): Populated from frontmatter `tags`. Examples: `ai-saas`, `vendor-lock-in`, `crewai`, `multi-agent`, `ai-agent-security`, `self-hosted-ai`
- **`af_category`** (hierarchical): Inferred from content analysis. Initial categories:
  - `ai-tools` (tool reviews, comparisons)
  - `tutorials` (step-by-step guides with code)
  - `security` (security-focused articles)
  - `pricing-costs` (cost analysis, TCO)
  - `self-hosting` (self-hosted AI content)
  - `automation` (workflow automation)
- **`af_content_format`** (non-hierarchical): `standard-article`, `tutorial`, `tool-comparison`, `opinion-analysis`

---

### 2. `research_brief` (Research Briefs)

Research briefs are internal documents that may or may not be published on the site. If published, they appear as reference material linked from articles.

**Post Type Key:** `research_brief`
**Supports:** title, editor, author, revisions, custom fields

#### Custom Fields (Post Meta)

| Field Key | Type | Required | Source | Notes |
|-----------|------|----------|--------|-------|
| `_af_publish_date` | date | Yes | `**Target publish date:**` | When the associated article publishes |
| `_af_slug` | string | Yes | `**Slug:**` | Matches associated article slug |
| `_af_primary_keyword` | string | Yes | `**Primary keyword:**` | Main SEO target |
| `_af_secondary_keywords` | array | Yes | `**Secondary keywords:**` | Supporting keywords |
| `_af_key_stats` | array (JSON) | Yes | "Key Statistics" section | Each entry: `{stat, source_url, source_name}` |
| `_af_github_repos` | array (JSON) | Yes | "GitHub Repos" section | Each entry: `{name, stars, url, description}` |
| `_af_pricing_table` | array (JSON) | Yes | "Current SaaS Pricing" table | Each entry: `{tool, plan, price, notes}` |
| `_af_community_pain_points` | array (JSON) | Yes | "Community Pain Points" section | Each entry: `{quote, source, url}` |
| `_af_faq_candidates` | array | Yes | "FAQ Candidates" section | List of FAQ question strings |
| `_af_monetisation_angles` | array (JSON) | Yes | "Monetisation Angles" section | Each entry: `{type, description}` |
| `_af_article_angle` | string (long text) | Yes | "Article Angle" section | Core thesis, audience, tone |
| `_af_linked_article` | integer (post ID) | No | Derived | Post ID of the article this brief supports |

#### Taxonomies

- **`af_tag`** (shared with articles): Inherited from associated article or parsed from keywords
- **`af_brief_type`** (non-hierarchical): `pre-publish` (brief written before article), `supporting` (reference material)

---

### 3. `prompt_bundle` (Prompt Collections / Bundles)

Sellable products. Each bundle is a collection of prompts organized by category. Published as both free preview content and paid downloadable products (WooCommerce integration).

**Post Type Key:** `prompt_bundle`
**Supports:** title, editor, thumbnail, excerpt, author, revisions, custom fields, comments

#### Custom Fields (Post Meta)

| Field Key | Type | Required | Source | Notes |
|-----------|------|----------|--------|-------|
| `_af_bundle_category` | string | Yes | PD dept category | e.g., `content-pipeline`, `hermes-bootstrap`, `keyword-research`, `wordpress-automation`, `seo-writing` |
| `_af_prompt_count` | integer | Yes | Count of prompts in bundle | e.g., 50+ |
| `_af_bundle_tools` | array | Yes | PD dept tool list | e.g., `["openclaw", "hermes", "claude-code"]` |
| `_af_bundle_use_cases` | array | Yes | PD dept use cases | e.g., `["youtube-automation", "social-media", "long-form-content"]` |
| `_af_bundle_version` | string | Yes | Semantic version | e.g., `1.0.0` |
| `_af_bundle_github_url` | string (URL) | No | GitHub repo URL | Link to open-source mirror |
| `_af_bundle_preview_prompts` | array (JSON) | Yes | First 3-5 prompts | Shown as free preview on site |
| `_af_bundle_full_download` | string (URL) | Yes | File URL | Downloadable .md or .zip (paid) |
| `_af_bundle_price` | string | No | WooCommerce price | e.g., `29.00` |
| `_af_bundle_woocommerce_id` | integer | No | WooCommerce product ID | Links bundle to WC product |

#### Taxonomies

- **`af_tool`** (non-hierarchical): Tool the prompts target. e.g., `openclaw`, `hermes`, `claude-code`, `codex-cli`
- **`af_use_case`** (non-hierarchical): Use case category. e.g., `youtube-automation`, `social-media`, `long-form-content`, `multi-agent-swarms`, `coding-assistant`, `research`, `trading`, `image-generation`, `voice-cloning`, `video-generation`, `customer-support`, `email-calendar`, `project-management`
- **`af_bundle_tier`** (non-hierarchical): `free`, `premium`, `enterprise`

---

### 4. `tool_comparison` (Tool Comparisons — Optional Separate Type)

Tool comparisons could be a separate post type or an `article` with `af_content_format = tool-comparison`. Recommended: keep as article subtype unless the site needs to query comparisons independently.

**Recommendation:** Use `article` post type with `af_content_format = tool-comparison`. Add comparison-specific meta only if needed.

#### Additional Fields (when `af_content_format = tool-comparison`)

| Field Key | Type | Notes |
|-----------|------|-------|
| `_af_comparison_tools` | array (JSON) | Each entry: `{name, version, url}` |
| `_af_comparison_table` | array (JSON) | Structured comparison data |
| `_af_comparison_winner` | string | Recommended tool (if any) |
| `_af_comparison_benchmarks` | array (JSON) | Each entry: `{metric, tool_a_score, tool_b_score, source}` |

---

## Custom Taxonomies (Registry)

### Shared Taxonomies

| Taxonomy Key | Type | Hierarchical | Applied To | Purpose |
|-------------|------|-------------|------------|---------|
| `af_tag` | Tag | No | article, research_brief, prompt_bundle | Cross-content tagging |
| `af_category` | Category | Yes | article, research_brief | Primary content categorization |
| `af_content_format` | Tag | No | article | Distinguishes article, tutorial, comparison, opinion |

### Article-Specific

| Taxonomy Key | Type | Hierarchical | Purpose |
|-------------|------|-------------|---------|
| `af_content_format` | Tag | No | `standard-article`, `tutorial`, `tool-comparison`, `opinion-analysis` |

### Prompt Bundle-Specific

| Taxonomy Key | Type | Hierarchical | Purpose |
|-------------|------|-------------|---------|
| `af_tool` | Tag | No | Tool targeting (openclaw, hermes, claude-code, codex-cli) |
| `af_use_case` | Tag | No | Use case category (14 categories from PD dept) |
| `af_bundle_tier` | Tag | No | `free`, `premium`, `enterprise` |

### Research Brief-Specific

| Taxonomy Key | Type | Hierarchical | Purpose |
|-------------|------|-------------|---------|
| `af_brief_type` | Tag | No | `pre-publish`, `supporting` |

---

## Content Mapping: Pipeline to WordPress

### SEO Article Pipeline

```
Pipeline Draft (MD file with YAML frontmatter)
    │
    ├── Frontmatter fields → WP post meta (_af_slug, _af_seo_description, etc.)
    ├── tags → af_tag taxonomy terms
    ├── H1 → WP post title
    ├── Body → WP post content (processed from Markdown to HTML)
    │   ├── Hero image → WP featured image + _af_hero_image meta
    │   ├── [IMAGE: ...] placeholders → _af_inline_images meta (for theme rendering)
    │   ├── Code blocks → Preserved as <pre><code> with language class
    │   ├── Tables → Rendered as HTML tables
    │   ├── FAQ section → _af_faq_items meta (for FAQ schema markup)
    │   └── "The Bottom Line" → _af_bottom_line meta
    ├── status → _af_status meta (draft → qa-passed → published)
    └── Content analysis → af_category, af_content_format terms
```

### Research Brief Pipeline

```
Research Brief (MD file, no YAML frontmatter)
    │
    ├── H1 title → WP post title
    ├── Bold metadata lines → WP post meta (_af_publish_date, _af_slug, _af_primary_keyword, etc.)
    ├── Key Statistics section → _af_key_stats meta
    ├── GitHub Repos section → _af_github_repos meta
    ├── Pricing table → _af_pricing_table meta
    ├── Community Pain Points → _af_community_pain_points meta
    ├── FAQ Candidates → _af_faq_candidates meta
    ├── Monetisation Angles → _af_monetisation_angles meta
    ├── Article Angle → _af_article_angle meta
    └── Linked article → _af_linked_article meta (post ID reference)
```

---

## Content Mapping: PD Department to WordPress

### Prompt Bundle Pipeline

```
PD Department (prompts stored in Memex + Obsidian + MD files)
    │
    ├── Bundle category → _af_bundle_category meta + af_use_case taxonomy
    ├── Prompt count → _af_prompt_count meta
    ├── Tool list → _af_bundle_tools meta + af_tool taxonomy
    ├── Use cases → _af_bundle_use_cases meta + af_use_case taxonomy
    ├── Version → _af_bundle_version meta
    ├── GitHub URL → _af_bundle_github_url meta
    ├── Preview prompts (3-5) → _af_bundle_preview_prompts meta (displayed on site)
    ├── Full download → _af_bundle_full_download meta (WooCommerce product file)
    ├── Price → _af_bundle_price meta + WooCommerce product
    └── Bundle tier → af_bundle_tier taxonomy
```

### Prompt Display on Article Pages

When an article references a prompt bundle:
- Article body contains inline prompts as code blocks
- "Related Bundle" section at bottom links to `prompt_bundle` post
- Bundle preview shows first 3-5 prompts with "Get Full Bundle" CTA

---

## WordPress Options / Settings Needed

| Option | Purpose |
|--------|---------|
| Ad network configuration | AdSense, Ezoic, Mediavine ad slot definitions |
| FAQ schema enable/disable | Toggle FAQ structured data output |
| Code syntax highlighting | Prism.js or Highlight.js language bundle config |
| Bundle WooCommerce integration | Product type for downloadable prompt bundles |
| Internal linking rules | Auto-link articles targeting same keywords |
| Image placeholder fallback | Default image for articles without hero image |

---

## Post Type Relationships

```
research_brief ──(_af_linked_article)──► article
article ──(af_tag shared)──► prompt_bundle
prompt_bundle ──(af_tool/af_use_case)──► article (related articles)
article ──(af_content_format = tool-comparison)──► article (compared tools)
```

---

## Recommended Plugin Stack

| Plugin | Purpose |
|--------|---------|
| ACF (Advanced Custom Fields) | All custom field groups for post types |
| Custom Post Type UI | Register post types and taxonomies (or code in theme) |
| WooCommerce | Prompt bundle sales |
| Yoast SEO / Rank Math | SEO meta, FAQ schema, breadcrumbs |
| Prism.js for WP | Syntax highlighting for code blocks |
| TablePress or native | Table rendering (prefer native Gutenberg tables) |
| WP All Import / custom importer | Markdown-to-WP import pipeline |
