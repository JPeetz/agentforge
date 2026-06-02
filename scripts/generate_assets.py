#!/usr/bin/env python3
"""
generate_all_assets.py — Batch generate ALL AgentForge UI assets via fal.ai FLUX.1 [schnell]

Generates 104 images: logo, icons, illustrations, backgrounds, UI elements.
Architecture: humane, sequential with progress. ~0.3s per image = ~31s total.
"""

import sys, os, urllib.request, time

ASSETS_DIR = os.path.expanduser("~/.openclaw/workspace/AgentForge/internal/dashboard/static/img")
API_KEY = "44ef2688-d7b6-447a-b525-dcb720431ca2:95849aeb7a1d918750438f243e1fa72e"

# ── Helper ──────────────────────────────────────────────────────────────────

def gen(prompt: str, path: str, size: str = "square_hd"):
    """Generate one image and save it."""
    from fal_client import SyncClient
    client = SyncClient(key=API_KEY)
    result = client.subscribe("fal-ai/flux/schnell", {
        "prompt": prompt,
        "num_images": 1,
        "image_size": size,
    })
    url = result["images"][0]["url"]
    os.makedirs(os.path.dirname(path), exist_ok=True)
    urllib.request.urlretrieve(url, path)
    t = result.get("timings", {}).get("inference", "?")
    print(f"  ✅ {os.path.basename(path)} ({t}s)")
    return url

# ── Asset Manifest ──────────────────────────────────────────────────────────

assets = []

def add(prompt, filename, size="square_hd"):
    assets.append((prompt, os.path.join(ASSETS_DIR, filename), size))

# ── 1. LOGO + BRAND IDENTITY ────────────────────────────────────────────────

add(
    "minimalist hexagonal volcano logo mark, obsidian black geometric crystal shape with glowing magma orange core, sharp faceted edges, dark volcanic glass material, centered on pure black background, vector style, ultra sharp, no text",
    "icons/logo-icon.png"
)
add(
    "minimalist hexagonal volcano logo mark with text 'AgentForge' in modern geometric sans-serif font, obsidian black crystal with glowing orange magma core, sharp faceted edges, wordmark to the right, dark background, corporate tech brand identity, ultra sharp, professional",
    "icons/logo-primary.png",
    "landscape_16x9"
)
add(
    "single geometric hexagonal volcano icon, obsidian black with bright magma orange glow, sharp crystal facets, pure black background, favicon style, centered, no text, ultra crisp",
    "icons/favicon.png"
)

# ── 2. BASE UI BACKGROUNDS ──────────────────────────────────────────────────

add(
    "dark volcanic glass texture, subtle geometric crystal facets, deep obsidian black with very faint orange amber undertones, seamless tileable pattern, abstract mineral surface, no text, dark and elegant, 4K quality",
    "backgrounds/bg-glass-texture.png"
)
add(
    "epic dark volcanic landscape, obsidian crystal formations floating in darkness, glowing orange magma veins running through black glass, subtle floating ember particles, mysterious and powerful atmosphere, tech brand hero background, no text, cinematic lighting, 8K, photorealistic",
    "backgrounds/bg-login.png",
    "landscape_16x9"
)
add(
    "single tiny glowing orange ember particle, floating spark, warm amber glow against transparent background, isolated, fire ember, photorealistic micro photography",
    "backgrounds/bg-particle.png"
)

# ── 3. LOADING + DIVIDER ────────────────────────────────────────────────────

add(
    "minimalist loading spinner animation frame, glowing magma orange circular arc, thin geometric ring partial, dark background, tech UI element, clean vector style, no text",
    "icons/loading-spinner.svg"
)
add(
    "ornamental horizontal divider line, geometric crystal faceted pattern, magma orange to obsidian black gradient, sharp angular design, UI decoration element, dark background, no text",
    "icons/divider.svg",
    "landscape_16x9"
)

# ── 4. NAVIGATION ICONS ─────────────────────────────────────────────────────

add("minimalist dashboard overview icon, four square grid layout, geometric, magma orange lines on obsidian black, clean UI icon, centered, no text", "icons/nav-dashboard.svg")
add("minimalist AI agent icon, stylized geometric head/shoulder silhouette made of crystal facets, magma orange on black, clean UI icon, centered, no text", "icons/nav-agents.svg")
add("minimalist memory storage icon, geometric brain shape made of hexagons, crystal facets, magma orange on black, clean UI icon, centered, no text", "icons/nav-memory.svg")
add("minimalist pipeline workflow icon, geometric connected nodes DAG graph, magma orange lines, crystal nodes, dark background, clean UI icon, centered, no text", "icons/nav-pipelines.svg")
add("minimalist skill puzzle piece icon, geometric interlocking shapes, crystal facets, magma orange on black, clean UI icon, centered, no text", "icons/nav-skills.svg")
add("minimalist security shield icon, geometric faceted shield shape, crystal glass material, magma orange accent on black, clean UI icon, centered, no text", "icons/nav-security.svg")
add("minimalist settings gear icon, geometric hexagonal gear shape, faceted crystal, magma orange on black, clean UI icon, centered, no text", "icons/nav-settings.svg")
add("minimalist notification bell icon, geometric crystal bell shape, magma orange accent dot, dark background, clean UI icon, centered, no text", "icons/nav-bell.svg")
add("minimalist online status dot, green emerald glowing circle with subtle pulse rings, dark background, clean UI element, centered, no text", "icons/nav-status-online.svg")

# ── 5. DASHBOARD ICONS ──────────────────────────────────────────────────────

add("abstract geometric volcanic crystal formation illustration, sharp faceted obsidian shapes with glowing magma orange core, floating crystal shards, dark background, modern tech illustration style, no text, epic and powerful", "illustrations/hero-dashboard.svg", "landscape_16x9")
add("minimalist spawn add icon, geometric plus symbol merging with crystal, magma orange glowing, dark background, clean UI icon, centered, no text", "icons/spawn-agent.svg")
add("minimalist pipeline run icon, geometric play triangle made of crystal facets, magma orange, dark background, clean UI icon, centered, no text", "icons/run-pipeline.svg")
add("minimalist search magnifier icon, geometric crystal lens shape, magma orange glow inside, dark background, clean UI icon, centered, no text", "icons/search-icon.svg")
add("minimalist health monitor icon, geometric heart rate line with crystal pulse dot, emerald green and magma orange, dark background, clean UI icon, centered, no text", "icons/health-icon.svg")

# ── 6. AGENT STATUS ICONS ───────────────────────────────────────────────────

add("glowing green emerald circle, solid filled, with subtle inner glow, status indicator, dark background, centered, no text, ultra clean", "icons/agent-running.svg")
add("grey circle, slightly transparent, status idle indicator, dark background, centered, no text, ultra clean", "icons/agent-idle.svg")
add("amber orange pause symbol, two vertical bars, glowing, status indicator, dark background, centered, no text, ultra clean", "icons/agent-paused.svg")
add("red stop symbol, square shape with subtle glow, error indicator, dark background, centered, no text, ultra clean", "icons/agent-stopped.svg")

print(f"\n{'='*60}")
print(f"🔥 AgentForge Asset Generator — {len(assets)} images total")
print(f"{'='*60}\n")

for i, (prompt, path, size) in enumerate(assets):
    name = os.path.basename(path)
    print(f"[{i+1}/{len(assets)}] {name}")
    try:
        gen(prompt, path, size)
    except Exception as e:
        print(f"  ❌ FAILED: {e}")
    time.sleep(0.15)  # rate limit buffer

print(f"\n{'='*60}")
print(f"✅ Generation complete. {len(assets)} images in {ASSETS_DIR}")
print(f"{'='*60}")