# AgentForge — Web UI Architecture & Image Asset Registry v1.0

## Design Philosophy

**"Volcanic Glass"** — Dark basalt surfaces with glowing magma-orange accents, translucent glass panels, sharp geometric edges. Inspired by obsidian formation: natural, powerful, precise. NOT another blue/purple gradient dashboard. This is AgentForge — the UI should feel like forged volcanic glass: hot, sharp, unbreakable.

### Color System
- **Primary:** #FF6B2C (Magma Orange) — CTAs, active states, highlights
- **Primary Dark:** #E85D1A — hover states
- **Glass Surface:** rgba(20,20,28,0.85) with backdrop-blur: 20px
- **Glass Border:** rgba(255,107,44,0.12)
- **Background:** #0A0A0F (Deep Basalt)
- **Surface:** #14141C (Obsidian)
- **Text Primary:** #F0EDE8 (Warm White)
- **Text Secondary:** #8B8680 (Stone)
- **Success:** #4ADE80 (Emerald)
- **Warning:** #FBBF24 (Amber)
- **Error:** #F43F5E (Rose)
- **Info:** #38BDF8 (Sky)

### Typography
- **Headings:** Space Grotesk (variable weight, modern geometric)
- **Body:** Inter (clean, readable)
- **Code/Terminal:** JetBrains Mono

---

## PAGE ARCHITECTURE — Every Page, Every Component

### 1. LANDING / LOGIN PAGE
Route: `/`
Components:
- Full-viewport animated particle field (floating ember particles, like volcanic ash)
- Centered glass card with AgentForge logo
- Login form (API key or password)
- "Powered by AgentForge" subtle footer
- Image assets needed:
  - `logo-primary.png` — Full AgentForge logo with icon + wordmark (512x128)
  - `logo-icon.png` — Icon-only logo (128x128)
  - `bg-login.png` — Dark volcanic background texture (1920x1080)

### 2. DASHBOARD OVERVIEW
Route: `/dashboard`
Top Nav (glass bar, fixed):
  - Logo (small, top-left)
  - Nav items: Overview, Agents, Memory, Pipelines, Skills, Security, Settings
  - Right: Status indicator (online/offline), user avatar, notifications bell
  - Image assets needed:
    - `nav-logo.png` — Small logo mark for navbar (32x32)
    - `icon-dashboard.svg` — Dashboard/overview icon
    - `icon-agents.svg` — Agents icon
    - `icon-memory.svg` — Memory/knowledge icon
    - `icon-pipelines.svg` — Pipeline/workflow icon
    - `icon-skills.svg` — Skills/puzzle icon
    - `icon-security.svg` — Security/shield icon
    - `icon-settings.svg` — Settings/gear icon
    - `icon-bell.svg` — Notifications bell
    - `icon-status-online.svg` — Green pulse dot
    - `icon-user.svg` — User avatar placeholder

Main Content:
  - Hero stat cards (4 cards in a row): Active Agents, Memory Size, Skills Active, Pipelines Today
  - Quick action buttons: "Spawn Agent", "Run Pipeline", "Search Memory"
  - Recent activity feed (timeline with glass panels)
  - System health indicators (CPU, memory, disk — glass gauge cards)
  - Image assets needed:
    - `illust-hero-dashboard.svg` — Abstract volcanic glass geometric illustration for hero area
    - `icon-spawn.svg` — Spawn/add agent icon
    - `icon-pipeline.svg` — Pipeline icon
    - `icon-search.svg` — Search/magnifier icon
    - `icon-health.svg` — Heart rate/health monitor icon

### 3. AGENTS PAGE
Route: `/agents`
Layout: Split panel — left sidebar (agent list), right panel (agent detail)
Left Panel:
  - Search bar to filter agents
  - Agent list with status dots (green=running, amber=paused, red=stopped, grey=idle)
  - Department grouping with collapsible sections
  - "Spawn Agent" button at bottom
  - Image assets needed:
    - `icon-agent-status-running.svg` — Green glowing circle
    - `icon-agent-status-idle.svg` — Grey circle
    - `icon-agent-status-paused.svg` — Amber pause
    - `icon-agent-status-stopped.svg` — Red stop
    - `icon-agent-spawn.svg` — Plus/add with agent icon

Right Panel (Agent Detail):
  - Agent header: name, ID, department badge, status
  - Capability overview: token budget gauge, permission badges, resource list
  - Tools: list of available tools with enable/disable toggles
  - Memory: memory usage preview, file count
  - Activity log: recent messages/actions
  - Action buttons: Chat, Pause, Terminate, Clone
  - Image assets needed:
    - `icon-agent-chat.svg` — Chat bubble icon
    - `icon-agent-pause.svg` — Pause icon
    - `icon-agent-terminate.svg` — Stop/terminate icon
    - `icon-agent-clone.svg` — Clone/copy icon
    - `icon-capability.svg` — Shield with key
    - `icon-tools.svg` — Wrench/screwdriver
    - `icon-activity.svg` — Activity/pulse

### 4. MEMORY PAGE
Route: `/memory`
Layout: Three-column — directory tree (left), file viewer (center), search (right)
Left — Directory Tree:
  - Glass file browser with collapsible folders
  - Color-coded: MEMORY.md (gold), daily logs (amber), agent states (blue), decisions (green)
  - Git status overlay (modified, staged, committed badges)
  - Image assets needed:
    - `icon-folder.svg` — Folder icon
    - `icon-file.svg` — Generic file
    - `icon-file-memory.svg` — Memory file with star
    - `icon-file-daily.svg` — Daily log file
    - `icon-file-decision.svg` — Decision file
    - `icon-git.svg` — Git branch icon
    - `icon-git-modified.svg` — Modified badge
    - `icon-git-committed.svg` — Committed check

Center — File Viewer:
  - Markdown renderer with syntax highlighting
  - File header with path breadcrumb
  - Edit/Preview toggle
  - "Summarize" and "Compress" buttons
  - Image assets needed:
    - `icon-edit.svg` — Edit/pencil
    - `icon-preview.svg` — Preview/eye
    - `icon-summarize.svg` — Compress/summarize
    - `icon-compress.svg` — Shrink/compact

Right — Search Panel:
  - Search input with filters (kind, agent, date range)
  - Results list with relevance score bars
  - Snippet preview with highlighted terms
  - Image assets needed:
    - `icon-filter.svg` — Filter/funnel
    - `icon-history.svg` — History/clock

### 5. PIPELINES PAGE
Route: `/pipelines`
Layout: Canvas-based DAG visualization with glass panels
Pipeline Canvas:
  - Visual DAG graph (nodes = stages, edges = dependencies)
  - Nodes show: stage name, agent icon, status color
  - Edges show animation for active pipelines
  - Zoom/pan controls
  - "Run Pipeline" and "New Pipeline" buttons
  - Image assets needed:
    - `icon-pipeline-node.svg` — Pipeline stage node
    - `icon-pipeline-edge.svg` — Edge/connection arrow
    - `icon-pipeline-running.svg` — Running pipeline animated indicator
    - `icon-pipeline-complete.svg` — Check/green circle
    - `icon-pipeline-failed.svg` — X/red circle
    - `icon-pipeline-add.svg` — New pipeline
    - `icon-zoom-in.svg` — Zoom in
    - `icon-zoom-out.svg` — Zoom out
    - `icon-fit.svg` — Fit to screen

Side Panel (Pipeline Detail):
  - Stage list with execution order
  - Stage detail: agent, tool, retry count, timeout
  - Run history with status
  - Image assets:
    - `icon-stage-retry.svg` — Retry/loop
    - `icon-stage-timeout.svg` — Timer/clock

### 6. SKILLS PAGE
Route: `/skills`
Layout: Grid of skill cards + marketplace sidebar
Skill Cards:
  - Glass card with skill icon, name, description snippet
  - Tags as mini badges
  - Author + version
  - Install/Enable toggle
  - Image assets needed:
    - `icon-skill-card.svg` — Skill card default icon
    - `icon-skill-install.svg` — Download/install
    - `icon-skill-enabled.svg` — Toggle on
    - `icon-skill-disabled.svg` — Toggle off

Marketplace Sidebar:
  - Search for new skills (SkillsMP/GitHub)
  - Category filters
  - Popular/trending list
  - Image assets:
    - `icon-marketplace.svg` — Store/marketplace
    - `icon-trending.svg` — Trending/fire
    - `icon-popular.svg` — Star/popular

### 7. SECURITY PAGE
Route: `/security`
Layout: Dashboard-style with glass audit panels
Security Overview:
  - Risk score gauge (circular, 0-100, color-coded green→amber→red)
  - Active capability count
  - Violations today
  - Image assets:
    - `icon-risk-low.svg` — Green shield
    - `icon-risk-medium.svg` — Amber shield
    - `icon-risk-high.svg` — Red shield
    - `icon-violation.svg` — Warning/alert triangle

Capability List:
  - Table of all issued capabilities
  - Each row: agent, permissions, budget remaining, expires, status
  - Image assets:
    - `icon-capability-token.svg` — Token/key icon
    - `icon-capability-expired.svg` — Expired/clock red

Audit Log:
  - Live streaming log of all capability checks
  - Filter by agent, action, result (allow/deny)
  - Image assets:
    - `icon-audit-log.svg` — Log/list
    - `icon-audit-allow.svg` — Allow/check green
    - `icon-audit-deny.svg` — Deny/x red

### 8. SETTINGS PAGE
Route: `/settings`
Layout: Tabbed panels with glass cards
Tabs: General, LLM, Memory, Security, Workers, Tools, MCP, About
Each tab: glass card with labeled form fields, toggles, sliders
Image assets needed:
  - `icon-tab-general.svg` — General/cog
  - `icon-tab-llm.svg` — AI/brain
  - `icon-tab-memory.svg` — Memory/database
  - `icon-tab-security.svg` — Security/lock
  - `icon-tab-workers.svg` — Workers/people
  - `icon-tab-tools.svg` — Tools/wrench
  - `icon-tab-mcp.svg` — MCP/plug
  - `icon-tab-about.svg` — Info/circle

### 9. CHAT PAGE (per agent)
Route: `/chat/{agentId}`
Layout: Streaming chat interface with glass message bubbles
Header: Agent name, model badge, token counter
Messages: Glass bubbles, markdown rendering, code blocks
Input: Glass text area with tool attachment buttons
Image assets:
  - `icon-chat-send.svg` — Send arrow
  - `icon-chat-attach.svg` — Paperclip/attach
  - `icon-chat-code.svg` — Code/tag
  - `icon-chat-voice.svg` — Microphone
  - `icon-token-count.svg` — Token counter

### 10. COMMON / SHARED
Image assets needed everywhere:
  - `bg-glass-texture.png` — Subtle glass texture pattern (tileable, 256x256)
  - `bg-particle.png` — Single ember particle (16x16)
  - `divider.svg` — Ornamental section divider, magma-colored
  - `empty-state.svg` — Generic empty state illustration
  - `loading-spinner.svg` — Animated loading spinner (magma orange)
  - `favicon.png` — Browser favicon (32x32)
  - `og-image.png` — Open Graph / social preview image (1200x630)
  - `icon-close.svg` — Close/X
  - `icon-expand.svg` — Expand/maximize
  - `icon-collapse.svg` — Collapse/minimize
  - `icon-refresh.svg` — Refresh/reload
  - `icon-copy.svg` — Copy to clipboard
  - `icon-external.svg` — External link
  - `icon-chevron-down.svg` — Chevron/expand
  - `icon-chevron-right.svg` — Chevron/navigate
  - `icon-plus.svg` — Plus/add
  - `icon-minus.svg` — Minus/remove
  - `icon-check.svg` — Checkmark
  - `icon-x.svg` — X mark
  - `icon-undo.svg` — Undo
  - `icon-redo.svg` — Redo
  - `icon-trash.svg` — Delete/trash
  - `icon-download.svg` — Download
  - `icon-upload.svg` — Upload
  - `icon-lock.svg` — Lock
  - `icon-unlock.svg` — Unlock
  - `icon-light.svg` — Light/sun
  - `icon-dark.svg` — Dark/moon

---

## TOTAL IMAGE ASSETS: 104
### Priority Order (generate in this sequence):
1. **Logo + Brand** (3): logo-primary.png, logo-icon.png, favicon.png
2. **Base UI** (5): bg-glass-texture.png, bg-login.png, bg-particle.png, loading-spinner.svg, divider.svg
3. **Navigation** (10): nav-logo.png, all 9 section icons
4. **Dashboard** (5): illust-hero-dashboard.svg, spawn/pipeline/search/health icons
5. **Agents** (14): status icons, spawn, detail panel icons
6. **Memory** (14): file type icons, git icons, viewer icons, search icons
7. **Pipelines** (9): node, edge, status, zoom icons
8. **Skills** (6): skill card, install, toggle, marketplace icons
9. **Security** (8): risk, violation, capability, audit icons
10. **Settings** (8): tab icons
11. **Chat** (5): send, attach, code, voice, token icons
12. **Common** (20): generic UI icons (chevrons, close, copy, etc.)
13. **Social** (1): og-image.png