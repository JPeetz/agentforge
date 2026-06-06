# WordPress Theme Development Research
## AgentForge — WP Design Department

**Date:** 2026-05-15
**Researcher:** Marvin (direct research via web_search + web_extract)

---

## 1. Classic PHP Themes vs Block Themes (FSE): The Decision

This is the most important architectural decision. Here is the current state in 2025/2026:

### Block Themes (FSE)
- Templates stored as HTML files in `/templates/` (e.g., `single.html`, `archive.html`)
- Reusable parts in `/parts/` (header.html, footer.html)
- Global styles defined in `theme.json` + editable via Styles panel
- Visual editing via Site Editor — no PHP knowledge required
- WordPress is investing heavily in FSE; it is the platform's future
- **Performance**: Well-optimised block themes can be very fast (Nexter Theme <50KB, zero jQuery). Simple block themes outperform classic themes using page builders.
- **Limitation**: Complex custom functionality is harder. Some older plugins lack FSE support. Nested blocks can generate verbose HTML.

### Classic PHP Themes
- PHP template files (`header.php`, `single.php`, `archive.php`)
- `functions.php` for logic, `style.css` for presentation
- Customizer + widget areas for configuration
- 15+ years of battle-tested stability
- **Performance**: Granular control over queries, scripts, styles. Leaner markup. Better for high-traffic sites.
- **Limitation**: Requires PHP knowledge for layout changes. Customizer is indirect (one step removed from layout).

### Hybrid Themes
- Classic architecture with selective block editor support
- PHP templates for layout control + block editor for content areas
- `theme.json` for design tokens
- Best of both worlds but not a permanent solution (WordPress is moving toward full FSE)

### Recommendation for AgentForge

**Go with a Classic PHP theme.** Reasons:

1. **Content-focused site** = template hierarchy matters more than visual editing
2. **Performance control** = we need every millisecond for Core Web Vitals
3. **No non-technical editors** = the operator is technical; visual editing is not needed
4. **Custom content types** = articles + prompt libraries + research need custom templates
5. **Lean markup** = PHP gives us precise control over HTML output
6. **No page builders** = we are building from scratch anyway

We can still use `theme.json` for design tokens (colours, typography, spacing) to get some of the benefits of the block system without committing to FSE.

---

## 2. Theme Architecture: Required Files

### Minimum Required Files
```
style.css          /* Theme metadata + styles */
index.php          /* Fallback template */
```

### Recommended File Structure
```
agentforge/
├── style.css              /* Theme header + CSS */
├── index.php              /* Fallback */
├── functions.php          /* Theme setup, enqueues, hooks */
├── header.php             /* <head> + site header */
├── footer.php             /* Site footer + closing tags */
├── sidebar.php            /* Optional sidebar */
├── front-page.php         /* Homepage template */
├── home.php               /* Blog posts index */
├── single.php             /* Single post (default) */
├── single-article.php     /* Single article (custom) */
├── single-prompt.php      /* Single prompt library entry */
├── archive.php            /* Default archive */
├── archive-article.php    /* Article archive */
├── archive-prompt.php     /* Prompt library archive */
├── category.php           /* Category archive */
├── search.php             /* Search results */
├── 404.php                /* Not found */
├── page.php               /* Static page */
├── page-about.php         /* About page (custom) */
├── template-parts/        /* Reusable template parts */
│   ├── content.php        /* Post content */
│   ├── content-single.php /* Single post content */
│   ├── content-card.php   /* Post card for archives */
│   ├── content-none.php   /* No results */
│   └── meta.php           /* Post meta */
├── assets/
│   ├── css/
│   │   ├── critical.css   /* Above-the-fold CSS (inlined) */
│   │   └── main.css       /* Full stylesheet */
│   ├── js/
│   │   ├── main.js        /* Minimal JS */
│   │   └── dark-mode.js   /* Dark mode toggle */
│   └── img/
│       ├── hero-home.webp
│       ├── hero-content.webp
│       ├── hero-prompts.webp
│       ├── og-default.webp
│       ├── icons/
│       └── favicon/
├── inc/
│   ├── setup.php          /* Theme setup */
│   ├── enqueue.php        /* Script/style registration */
│   ├── template-tags.php  /* Custom template tags */
│   ├── customizer.php     /* Customizer settings */
│   └── schema.php         /* Structured data */
└── screenshot.png         /* Theme screenshot (1200x900) */
```

### Template Hierarchy (Key Templates)

For our content types:

**Homepage:**
1. `front-page.php` (if set to static page)
2. `home.php` (if set to latest posts)
3. `index.php` (fallback)

**Single Article:**
1. `single-article-{slug}.php`
2. `single-article.php`
3. `single.php`
4. `singular.php`
5. `index.php`

**Single Prompt:**
1. `single-prompt-{slug}.php`
2. `single-prompt.php`
3. `single.php`
4. `singular.php`
5. `index.php`

**Article Archive:**
1. `archive-article.php`
2. `archive.php`
3. `index.php`

**Category Archive:**
1. `category-{slug}.php`
2. `category-{id}.php`
3. `category.php`
4. `archive.php`
5. `index.php`

---

## 3. Performance: Core Web Vitals

### The Three Metrics

| Metric | Target | Measures |
|--------|--------|----------|
| LCP | < 2.5s | Largest contentful paint (hero image, text block) |
| INP | < 200ms | Interaction to next paint (replaced FID in 2024) |
| CLS | < 0.1 | Visual stability (elements jumping during load) |

### LCP Optimisation

- **Images**: Convert to WebP/AVIF (25-35% better than JPEG). Use `srcset` for responsive images. Never lazy-load above-the-fold images.
- **Server**: Quality managed hosting. Page caching (Redis/Memcached). CDN essential (Cloudflare, BunnyCDN).
- **Render-blocking**: Inline critical CSS in `<head>`. Defer non-essential JS.
- **Target**: Hero image should be preloaded: `<link rel="preload" as="image" href="hero.webp">`

### INP Optimisation

- **Audit JS**: Chrome DevTools Performance panel to find long tasks (>50ms)
- **Break up long tasks**: Code splitting, progressive enhancement
- **Event handlers**: Debounce/throttle scroll and resize. Use event delegation.
- **Third-party scripts**: Load async. Use facades for embeds.
- **Rule**: Our theme should load ZERO JavaScript for basic content viewing. JS only for dark mode toggle and optional enhancements.

### CLS Optimisation

- **Image dimensions**: Always define `width` and `height` attributes. Use CSS `aspect-ratio`.
- **Fonts**: `font-display: swap`. Preload critical fonts. Host fonts locally (not Google Fonts CDN).
- **Dynamic content**: Reserve space for widgets, comments, related posts. Use skeleton screens.
- **Animations**: Use CSS transforms only (not layout-triggering properties).

### Critical CSS Strategy

Inline the minimum CSS needed for above-the-fold content directly in `<head>`:
```php
<style>
  /* Critical CSS inlined */
  <?php echo file_get_contents( get_template_directory() . '/assets/css/critical.css' ); ?>
</style>
<link rel="stylesheet" href="<?php echo get_stylesheet_uri(); ?>" media="print" onload="this.media='all'">
```

### Font Strategy

**Recommended: System font stack.** Zero network requests, instant render.

```css
--font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
--font-mono: "SF Mono", "Fira Code", "Fira Mono", "Roboto Mono", monospace;
--font-serif: "Georgia", "Times New Roman", serif;
```

**If we want one web font:** Use a variable font (single file, multiple weights). Host locally. Preload:
```html
<link rel="preload" href="/assets/fonts/inter-var.woff2" as="font" type="font/woff2" crossorigin>
```

### Image Strategy

- Hero images: WebP, max 1920px wide, quality 80
- Content images: WebP with `srcset` for responsive
- Icons: SVG (inline for critical, sprite for others)
- Lazy loading: Native `loading="lazy"` on all below-fold images
- Dimensions: Always specify width/height to prevent CLS

---

## 4. Typography for Content Sites

### Readability Research

- **Body font size**: Minimum 16px. Recommended 18px for long-form content.
- **Line height**: 1.6 to 1.8 for body text. Tighter (1.2-1.4) for headings.
- **Line length**: 65-75 characters optimal. Max 80 characters.
- **Paragraph spacing**: 1em between paragraphs. More whitespace = better comprehension.
- **Font pairing**: Maximum 2 fonts. One for headings, one for body. Or a single variable font.

### Recommended Typography Scale

```css
--text-xs: 0.75rem;    /* 12px - captions, labels */
--text-sm: 0.875rem;   /* 14px - meta, secondary */
--text-base: 1.125rem; /* 18px - body text */
--text-lg: 1.25rem;    /* 20px - large body */
--text-xl: 1.5rem;     /* 24px - h4 */
--text-2xl: 1.875rem;  /* 30px - h3 */
--text-3xl: 2.25rem;   /* 36px - h2 */
--text-4xl: 3rem;      /* 48px - h1 */
--text-5xl: 3.75rem;   /* 60px - display */
```

### Font Pairing Options

1. **System stack only** (fastest): `-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto` for sans + `SF Mono, Fira Code` for mono
2. **Inter + Source Serif**: Inter (sans) for UI + Source Serif 4 (serif) for article body. Classic content site combination.
3. **IBM Plex**: IBM Plex Sans + IBM Plex Mono. Tech-forward, distinctive, open source.
4. **Geist + Geist Mono**: Vercel's font. Modern, clean, tech-audience familiarity.

**Recommendation**: Start with system font stack for speed. Add one web font (Inter or Geist) later if the design needs it.

---

## 5. Dark Mode Implementation

### Approach: CSS Custom Properties + Class Toggle

```css
:root {
  --bg-primary: #ffffff;
  --bg-secondary: #f8f9fa;
  --text-primary: #1a1a2e;
  --text-secondary: #4a4a6a;
  --accent: #6366f1;
  --border: #e2e8f0;
}

[data-theme="dark"] {
  --bg-primary: #0f0f1a;
  --bg-secondary: #1a1a2e;
  --text-primary: #e2e8f0;
  --text-secondary: #94a3b8;
  --accent: #818cf8;
  --border: #2d2d44;
}

@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --bg-primary: #0f0f1a;
    --bg-secondary: #1a1a2e;
    --text-primary: #e2e8f0;
    --text-secondary: #94a3b8;
    --accent: #818cf8;
    --border: #2d2d44;
  }
}
```

### Toggle JavaScript (minimal)

```javascript
// dark-mode.js
const toggle = document.querySelector('.dark-mode-toggle');
const saved = localStorage.getItem('theme');
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)');

if (saved === 'dark' || (!saved && prefersDark.matches)) {
  document.documentElement.setAttribute('data-theme', 'dark');
}

toggle?.addEventListener('click', () => {
  const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
  document.documentElement.setAttribute('data-theme', isDark ? 'light' : 'dark');
  localStorage.setItem('theme', isDark ? 'light' : 'dark');
});
```

### Colour Palette Recommendations

**Option A: Indigo Dark (recommended)**
- Background: `#0f0f1a` (near-black with blue tint)
- Surface: `#1a1a2e` (dark navy)
- Text: `#e2e8f0` (cool white)
- Accent: `#818cf8` (indigo-400)
- Border: `#2d2d44`

**Option B: Slate Dark**
- Background: `#0f172a` (slate-900)
- Surface: `#1e293b` (slate-800)
- Text: `#f1f5f9` (slate-100)
- Accent: `#38bdf8` (sky-400)
- Border: `#334155`

**Option C: Warm Dark**
- Background: `#1a1612` (warm near-black)
- Surface: `#2a2420` (warm dark)
- Text: `#f5f0eb` (warm white)
- Accent: `#f59e0b` (amber-500)
- Border: `#3d3530`

**Recommendation**: Option A (Indigo Dark). It feels technical without being cold. The indigo accent works well for links, buttons, and highlights. Good contrast ratios for accessibility.

---

## 6. SEO Markup

### Schema.org Structured Data

```php
// For articles
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Article",
  "headline": "<?php the_title(); ?>",
  "author": {
    "@type": "Person",
    "name": "<?php the_author(); ?>"
  },
  "datePublished": "<?php echo get_the_date('c'); ?>",
  "dateModified": "<?php echo the_modified_date('c'); ?>",
  "image": "<?php echo get_the_post_thumbnail_url(); ?>"
}
</script>
```

### Open Graph (in header.php)

```php
<meta property="og:title" content="<?php the_title(); ?>">
<meta property="og:description" content="<?php echo get_the_excerpt(); ?>">
<meta property="og:image" content="<?php echo get_the_post_thumbnail_url(null, 'large'); ?>">
<meta property="og:url" content="<?php the_permalink(); ?>">
<meta property="og:type" content="article">
<meta property="og:site_name" content="AgentForge">
```

### Semantic HTML

- Use `<article>`, `<section>`, `<nav>`, `<aside>`, `<header>`, `<footer>` appropriately
- Single `<h1>` per page (post title)
- Heading hierarchy: h1 > h2 > h3 (no skipping)
- `<time datetime="...">` for dates
- `<figure>` + `<figcaption>` for images with captions
- `<code>`, `<pre>`, `<samp>` for code blocks

---

## 7. Key functions.php Patterns

### Theme Setup

```php
<?php
// inc/setup.php
function agentforge_setup() {
    // Title tag support
    add_theme_support('title-tag');

    // Post thumbnails
    add_theme_support('post-thumbnails');
    set_post_thumbnail_size(1200, 630, true);

    // Custom image sizes
    add_image_size('card', 600, 315, true);
    add_image_size('hero', 1920, 600, true);

    // HTML5 markup
    add_theme_support('html5', array(
        'search-form', 'comment-form', 'comment-list', 'gallery', 'caption', 'script', 'style'
    ));

    // Responsive embeds
    add_theme_support('responsive-embeds');

    // Block editor styles
    add_theme_support('editor-styles');
    add_editor_style('assets/css/editor.css');

    // Wide alignment
    add_theme_support('align-wide');

    // Custom logo
    add_theme_support('custom-logo', array(
        'height' => 60,
        'width' => 200,
        'flex-height' => true,
        'flex-width' => true,
    ));

    // Navigation menus
    register_nav_menus(array(
        'primary' => 'Primary Menu',
        'footer'  => 'Footer Menu',
    ));

    // Feed links
    add_theme_support('automatic-feed-links');
}
add_action('after_setup_theme', 'agentforge_setup');
```

### Script/Style Enqueueing

```php
<?php
// inc/enqueue.php
function agentforge_scripts() {
    // Main stylesheet
    wp_enqueue_style('agentforge-style', get_stylesheet_uri(), array(), '1.0.0');

    // Main JS (deferred)
    wp_enqueue_script('agentforge-main', get_template_directory_uri() . '/assets/js/main.js', array(), '1.0.0', true);

    // Dark mode JS (inline, small)
    wp_add_inline_script('agentforge-main', '
        const s = localStorage.getItem("theme");
        const p = window.matchMedia("(prefers-color-scheme: dark)");
        if (s === "dark" || (!s && p.matches)) {
            document.documentElement.setAttribute("data-theme", "dark");
        }
    ');

    // Comment reply script on single posts
    if (is_singular() && comments_open() && get_option('thread_comments')) {
        wp_enqueue_script('comment-reply');
    }
}
add_action('wp_enqueue_scripts', 'agentforge_scripts');
```

### Security & Cleanup

```php
<?php
// Remove WordPress version from head
remove_action('wp_head', 'wp_generator');

// Remove emoji scripts
remove_action('wp_head', 'print_emoji_detection_script', 7);
remove_action('wp_print_styles', 'print_emoji_styles');

// Remove RSD link
remove_action('wp_head', 'rsd_link');

// Remove Windows Live Writer link
remove_action('wp_head', 'wlwmanifest_link');

// Remove shortlink
remove_action('wp_head', 'wp_shortlink_wp_head');

// Disable XML-RPC
add_filter('xmlrpc_enabled', '__return_false');

// Limit post revisions
define('WP_POST_REVISIONS', 3);

// Disable file editing in admin
define('DISALLOW_FILE_EDIT', true);
```

---

## 8. Development Workflow

### Local Development
- **LocalWP** (localwp.com) — free, fast, easiest for local WordPress
- Alternative: **wp-env** (official WordPress Docker environment)
- Alternative: **Laravel Valet** + WordPress

### Version Control
- Theme directory in git
- Ignore `node_modules/`, `*.log`, `.DS_Store`
- Track all theme files, assets, inc/

### Build Process
- No build step needed for CSS (write CSS directly, use PostCSS if desired)
- No build step needed for JS (vanilla JS, no frameworks)
- Optional: PostCSS for autoprefixing and custom properties fallback

### Deployment
- Once WP credentials available: upload via WP Admin > Appearance > Themes > Upload
- Or FTP/SFTP to `/wp-content/themes/agentforge/`
- Or git push to server if SSH access available

---

## 9. Key Takeaways

1. **Classic PHP theme** — best performance control, no non-technical editors needed
2. **System font stack** — zero network requests for fonts, add one web font later if needed
3. **Dark mode default** — use CSS custom properties, respect `prefers-color-scheme`
4. **Critical CSS inline** — above-the-fold CSS in `<head>`, defer main stylesheet
5. **Zero JS for content** — JS only for dark mode toggle and optional enhancements
6. **WebP images** — all images in WebP, lazy load below-fold, preload hero
7. **Semantic HTML** — proper heading hierarchy, schema.org, Open Graph
8. **Minimal plugin dependency** — theme should work without plugins
9. **Mobile-first CSS** — single-column base, enhance for larger screens
10. **Template parts** — reusable components via `get_template_part()`

---

## Sources

- WordPress Developer Resources: developer.wordpress.org/themes/
- Template Hierarchy: developer.wordpress.org/themes/classic-themes/basics/template-hierarchy/
- Block Themes vs Classic: nexterwp.com/blog/wordpress-fse-block-themes-vs-classic-themes/
- FSE vs Classic: wprobo.com/fse-vs-classic-wordpress-theme/
- Core Web Vitals: oddjar.com/wordpress-core-web-vitals-optimization-guide-2025/
- Dark Mode in Block Themes: developer.wordpress.org/news/2024/12/mastering-light-and-dark-mode-styling-in-block-themes/
- Typography Best Practices: wpbestpractices.dev/design/typography/
