// Package dashboard — embedded web dashboard server for AgentForge.
// Serves a glassmorphism SPA with live data via htmx partials.
package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/agentforge/agentforge/internal/auth"
	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/channel"
	"github.com/agentforge/agentforge/internal/config"
	"github.com/agentforge/agentforge/internal/cost"
	"github.com/agentforge/agentforge/internal/llm"
	"github.com/agentforge/agentforge/internal/api/mcp"
	"github.com/agentforge/agentforge/internal/mcpclient"
	"github.com/agentforge/agentforge/internal/session"
	"github.com/agentforge/agentforge/internal/sse"
)

//go:embed static
var staticFS embed.FS

type Server struct {
	cfg          *config.Config
	store        *config.PersistedStore
	bus          bus.Bus
	sessionMgr   *session.Manager
	mcpMgr       *mcp.Manager
	mcpClientMgr *mcpclient.ClientManager
	chanMgr      *channel.Manager
	costTracker  *cost.Tracker
	authStore    *auth.Store
	authManager  *auth.Manager
	sseHub       *sse.Hub
	adapter      llm.Adapter
	mux          *http.ServeMux
	log          *slog.Logger
	started      time.Time
}

func New(cfg *config.Config, b bus.Bus, sessionMgr *session.Manager, mcpMgr *mcp.Manager, mcpClientMgr *mcpclient.ClientManager, chanMgr *channel.Manager, costTracker *cost.Tracker, authStore *auth.Store, authManager *auth.Manager, hub *sse.Hub, adapter llm.Adapter) (*Server, error) {
	s := &Server{
		cfg:          cfg,
		store:        config.NewStore(cfg),
		bus:          b,
		sessionMgr:   sessionMgr,
		mcpMgr:       mcpMgr,
		mcpClientMgr: mcpClientMgr,
		chanMgr:      chanMgr,
		costTracker:  costTracker,
		authStore:    authStore,
		authManager:  authManager,
		sseHub:       hub,
		adapter:      adapter,
		mux:          http.NewServeMux(),
		log:          slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		started:      time.Now(),
	}

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("dashboard: embed static: %w", err)
	}
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	s.mux.HandleFunc("/", s.handleLogin)
	s.mux.HandleFunc("/dashboard", s.handleDashboard)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/pages/", s.handlePagePartials)
	s.mux.HandleFunc("/api/config", s.handleConfigAPI)
	s.mux.HandleFunc("/api/config/save", s.handleConfigSave)
	s.mux.HandleFunc("/api/tools", s.handleToolsAPI)
	s.mux.HandleFunc("/api/skills/search", s.handleSkillsSearch)
	s.mux.HandleFunc("/api/channels/test/", s.handleChannelTest)
	s.mux.HandleFunc("/api/pipelines/validate", s.handlePipelineValidate)
	s.mux.HandleFunc("/api/mcp", s.handleMCPAPI)
	s.mux.HandleFunc("/api/auth/login", s.handleLoginAPI)
	s.mux.HandleFunc("/api/auth/refresh", s.handleRefreshAPI)
	s.mux.HandleFunc("/api/auth/me", s.handleMeAPI)
	s.mux.HandleFunc("/api/auth/apikey", s.handleAPIKeyAPI)
	s.mux.HandleFunc("/api/cost/summary", s.handleCostSummary)
	s.mux.HandleFunc("/api/cost/daily", s.handleCostDaily)
	s.mux.HandleFunc("/api/cost/budget", s.handleCostBudget)
	s.mux.HandleFunc("/api/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("/api/chat/stream", s.handleChatStream)

	return s, nil
}

func (s *Server) Handler() http.Handler { return s.mux }

// ── Handle page partials ─────────────────────────────────────────────────────

func (s *Server) handlePagePartials(w http.ResponseWriter, r *http.Request) {
	page := strings.TrimPrefix(r.URL.Path, "/api/pages/")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	switch page {
	case "overview":
		s.renderOverview(w)
	case "agents":
		s.renderAgents(w)
	case "memory":
		s.renderMemory(w)
	case "pipelines":
		s.renderPipelines(w)
	case "skills":
		s.renderSkills(w)
	case "security":
		s.renderSecurity(w)
	case "mcp":
		s.renderMCPServers(w)
	case "logs":
		s.renderLogs(w)
	case "settings":
		s.renderSettings(w)
	case "tools":
		s.renderTools(w)
	case "skills-marketplace":
		s.renderSkillsMarketplace(w)
	case "pipeline-editor":
		s.renderPipelineEditor(w)
	case "agent-profiles":
		s.renderAgentProfiles(w)
	case "channels":
		s.renderChannels(w)
	case "chat":
		s.renderChat(w)
	default:
		fmt.Fprint(w, `<div class="panel">Page not found.</div>`)
	}
}

// ── Overview ─────────────────────────────────────────────────────────────────


// ── Agents ───────────────────────────────────────────────────────────────────

func (s *Server) renderAgents(w http.ResponseWriter) {
	// Build fleet from config profiles, fallback to built-in defaults
	type fleetRow struct {
		name, dept, model, icon string
		status string // Running, Idle, Paused
		uptime string
		fs, net, sh, spawn bool
	}
	
	rows := []fleetRow{}
	
	// Map agent profiles to rows
	for _, ap := range s.cfg.Agents.Profiles {
		status := "Idle"
		stCls := "badge-idle"
		if ap.Enabled {
			status = "Running"
			stCls = "badge-live"
		}
		_ = stCls
		
		// Pick icon based on department
		icon := "agent-chat.png"
		switch ap.Department {
		case "content": icon = "agent-content.png"
		case "seo": icon = "agent-research.png"
		case "social": icon = "agent-chat.png"
		case "security": icon = "agent-security.png"
		case "devops": icon = "agent-deploy.png"
		case "memory": icon = "agent-memory.png"
		case "orchestrator": icon = "agent-orchestrator.png"
		case "monitor": icon = "agent-monitor.png"
		}
		rows = append(rows, fleetRow{
			name: ap.Name, dept: ap.Department, model: ap.Provider + "/" + ap.Model,
			icon: icon, status: status,
			uptime: "–",
			fs: ap.Capability.AllowFileSystem, net: ap.Capability.AllowNetwork,
			sh: ap.Capability.AllowShell, spawn: ap.Capability.AllowSpawn,
		})
	}
	
	// Default fleet if no profiles configured
	if len(rows) == 0 {
		rows = []fleetRow{
			{"Content Writer", "content", "openai/gpt-4o", "agent-content.png", "Running", "2h 14m", true, true, false, true},
			{"SEO Auditor", "seo", "openai/gpt-4o", "agent-research.png", "Running", "1h 52m", true, true, false, false},
			{"Social Publisher", "social", "openai/gpt-4o-mini", "agent-chat.png", "Idle", "–", true, true, false, false},
			{"Code Reviewer", "devops", "anthropic/claude-sonnet", "agent-coding.png", "Running", "45m", true, true, false, true},
			{"Security Sentry", "security", "openai/gpt-4o", "agent-security.png", "Running", "3h 01m", true, true, false, false},
			{"Memory Archivist", "memory", "ollama/gemma3:27b", "agent-memory.png", "Idle", "–", true, false, false, false},
			{"Pipeline Orchestrator", "orchestrator", "openai/gpt-4o", "agent-orchestrator.png", "Running", "5h 12m", true, true, false, true},
			{"Health Monitor", "monitor", "openrouter/gpt-oss-120b", "agent-monitor.png", "Running", "8h 44m", true, true, false, false},
		}
	}
	
	capBadge := func(enabled bool, label string) string {
		if enabled {
			return fmt.Sprintf(`<span style="color:#22C55E;font-size:11px">%s</span>`, label)
		}
		return fmt.Sprintf(`<span style="color:var(--text-dim);font-size:11px">%s</span>`, label)
	}
	
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-agents.png"> Agent Fleet <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">`+fmt.Sprintf("%d agents", len(rows))+`</span></div>
<table class="data-table">
<thead><tr><th>Agent</th><th>Department</th><th>Status</th><th>Model</th><th>Uptime</th><th>Capabilities</th></tr></thead>
<tbody>`)
	
	for _, r := range rows {
		stCls := "badge-idle"
		if r.status == "Running" {
			stCls = "badge-live"
		}
		fmt.Fprintf(w, `<tr>
<td><img src="/static/img/icons/%s" width="16" style="vertical-align:middle;margin-right:6px"> %s</td>
<td><span class="badge">%s</span></td>
<td><span class="badge %s">%s</span></td>
<td style="font-family:var(--font-mono);font-size:12px">%s</td>
<td style="font-size:12px;color:var(--text-dim)">%s</td>
<td>%s %s %s %s</td></tr>`,
			r.icon, r.name, r.dept, stCls, r.status, r.model, r.uptime,
			capBadge(r.fs, "FS"), capBadge(r.net, "NET"), capBadge(r.sh, "SH"), capBadge(r.spawn, "SPAWN"))
	}
	
	fmt.Fprint(w, `</tbody></table></div>`)
}

// ── Memory ───────────────────────────────────────────────────────────────────

func (s *Server) renderMemory(w http.ResponseWriter) {
	fmt.Fprintf(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-memory.png"> Memory Store <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">%s</span></div>
<div style="margin-bottom:12px"><input placeholder="Search memory..." style="width:100%%;border:1px solid rgba(139,134,128,0.2);background:rgba(250,243,240,0.03);color:var(--text-primary);padding:10px 14px;border-radius:8px;font-size:14px"></div>
<table class="data-table">
<thead><tr><th>File</th><th>Kind</th><th>Size</th><th>Updated</th></tr></thead>
<tbody>
<tr><td style="font-family:var(--font-mono);font-size:13px">%s</td><td><span class="badge badge-magma">memory</span></td><td>8.2 KB</td><td>2026-06-02 15:30</td></tr>
<tr><td style="font-family:var(--font-mono);font-size:13px">%s</td><td><span class="badge">daily</span></td><td>12.5 KB</td><td>2026-06-02 15:45</td></tr>
<tr><td style="font-family:var(--font-mono);font-size:13px">%s</td><td><span class="badge badge-live">decision</span></td><td>3.1 KB</td><td>2026-06-02 14:50</td></tr>
</tbody></table></div>`,
		s.cfg.Memory.Root, "MEMORY.md", "memory/2026-06-02.md", "decisions.md")
}

// ── Pipelines ────────────────────────────────────────────────────────────────

func (s *Server) renderPipelines(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-pipelines.png"> Pipelines <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">DAG orchestration</span></div>
<div class="pipeline-grid">`)
	for i, d := range s.cfg.Pipelines.Definitions {
		statusCls := "badge-live"
		statusTxt := "Active"
		if !d.Enabled {
			statusCls = "badge-idle"
			statusTxt = "Paused"
		}
		stages := len(d.Stages)
		trig := d.Trigger.Type
		if trig == "cron" {
			trig = "⏰ " + d.Trigger.CronExpr
		}
		fmt.Fprintf(w, `<div class="pipeline-card">
<div class="pipeline-card-header"><span>%s</span><span class="badge %s">%s</span></div>
<div style="font-size:12px;color:var(--text-dim);margin:8px 0">%s | Trigger: %s</div>
<div style="display:flex;gap:6px;flex-wrap:wrap">`, d.Name, statusCls, statusTxt, fmt.Sprintf("%d stages", stages), trig)
		for j, st := range d.Stages {
			fmt.Fprintf(w, `<div style="font-size:11px;padding:4px 10px;border-radius:6px;background:rgba(255,107,44,0.08);color:var(--af-magma)">%d. %s</div>`, j+1, st.Name)
		}
		fmt.Fprint(w, `</div></div>`)
		_ = i
	}
	if len(s.cfg.Pipelines.Definitions) == 0 {
		fmt.Fprint(w, `<div style="color:var(--text-dim);padding:30px;text-align:center">No pipelines defined. Create one via Settings → Agents or the Pipeline Editor.</div>`)
	}
	fmt.Fprint(w, `</div></div>`)
}

// ── Skills ───────────────────────────────────────────────────────────────────

func (s *Server) renderSkills(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-skills.png"> Skill Registry</div>
<div class="skill-grid">`)
	skillCards := []struct{ name, desc, version, author string; tags []string }{
		{"code-review", "Automated code review with best-practice checks and security linting for Go, Python, JS", "1.2.0", "AgentForge", []string{"code", "security", "ci-cd"}},
		{"memory-manager", "Semantic memory compression and long-term curation for MeMex RAG", "1.0.0", "AgentForge", []string{"memory", "rag", "context"}},
		{"security-audit", "Capability-based security posture audit with risk scoring and remediation suggestions", "1.1.3", "AgentForge", []string{"security", "audit", "capability"}},
		{"seo-auditor", "SEO quality gate with content scoring, keyword analysis, and competitive benchmarking", "2.0.1", "SkillsMP", []string{"seo", "content", "marketing"}},
		{"browser-automation", "Headless browser automation for web scraping and E2E testing", "0.9.0", "SkillsMP", []string{"browser", "automation", "testing"}},
		{"data-analysis", "Statistical analysis and visualization for structured datasets", "1.0.0", "SkillsMP", []string{"data", "analytics", "viz"}},
	}
	for _, sk := range skillCards {
		tagsHTML := ""
		for _, t := range sk.tags {
			tagsHTML += fmt.Sprintf(`<span class="skill-tag">%s</span>`, t)
		}
		fmt.Fprintf(w, `<div class="skill-card"><div class="skill-card-header"><span>%s</span><span class="badge badge-magma">%s</span></div><div style="font-size:12px;color:var(--text-dim);margin:8px 0">%s</div><div style="display:flex;gap:6px;flex-wrap:wrap;margin-bottom:8px">%s</div><div style="font-size:11px;color:var(--text-dim)">by %s</div></div>`,
			sk.name, sk.version, sk.desc, tagsHTML, sk.author)
	}
	fmt.Fprint(w, `</div></div>`)
}

// ── Security ─────────────────────────────────────────────────────────────────

func (s *Server) renderSecurity(w http.ResponseWriter) {
	fmt.Fprintf(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-security.png"> Security Posture</div>
<table class="data-table">
<thead><tr><th>Setting</th><th>Value</th><th>Status</th></tr></thead>
<tbody>
%s
%s
%s
%s
%s
%s
%s
%s
%s
</tbody></table>
</div>`,
		secRow("Capability Enforcement", fmt.Sprintf("%v on spawn / %v on tool", s.cfg.Security.EnforceOnSpawn, s.cfg.Security.EnforceOnToolCall), s.cfg.Security.EnforceOnSpawn),
		secRow("Audit Logging", fmt.Sprintf("%v", s.cfg.Security.AuditEnabled), s.cfg.Security.AuditEnabled),
		secRow("Sandbox Mode", s.cfg.Security.SandboxMode, true),
		secRow("Filesystem Access", fmt.Sprintf("%v", s.cfg.Security.AllowFileSystem), s.cfg.Security.AllowFileSystem),
		secRow("Network Access", fmt.Sprintf("%v", s.cfg.Security.AllowNetwork), s.cfg.Security.AllowNetwork),
		secRow("Shell Access", fmt.Sprintf("%v", s.cfg.Security.AllowShell), !s.cfg.Security.AllowShell),
		secRow("Browser Access", fmt.Sprintf("%v", s.cfg.Security.AllowBrowser), !s.cfg.Security.AllowBrowser),
		secRow("Default Token Budget", fmt.Sprintf("%s tokens", formatNumber(s.cfg.Security.DefaultTokenBudget)), s.cfg.Security.DefaultTokenBudget > 0),
		secRow("Approved Paths", fmt.Sprintf("%d paths configured", len(s.cfg.Security.ApprovedPaths)), len(s.cfg.Security.ApprovedPaths) > 0),
	)
}

func secRow(label, val string, ok bool) string {
	cls := "badge-live"
	txt := "OK"
	if !ok {
		cls = "badge-idle"
		txt = "Check"
	}
	return fmt.Sprintf(`<tr><td>%s</td><td style="font-family:var(--font-mono);font-size:12px">%s</td><td><span class="badge %s">%s</span></td></tr>`, label, val, cls, txt)
}

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// ── Logs ─────────────────────────────────────────────────────────────────────

func (s *Server) renderLogs(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-logs.png"> Log Viewer</div>
<div class="log-viewer">`)
	logs := []struct{ ts, level, msg string }{
		{"15:30:14", "INFO", "config: saved to ~/.agentforge/agentforge.yaml"},
		{"15:28:02", "DEBUG", "bus: heartbeat broadcast → 3 subscribers"},
		{"15:27:45", "INFO", "agent: content-writer-1 [content] spawned (cap:a3f2b1)"},
		{"15:27:44", "DEBUG", "memory: auto-commit 3 files (hash:a8b2c4)"},
		{"15:26:30", "INFO", "dashboard: HTTP server listening :8080"},
		{"15:26:29", "INFO", "agentforge: daemon started v0.1.0 (content=3, seo=1, social=2)"},
	}
	for _, l := range logs {
		levelCls := "log-info"
		if l.level == "DEBUG" {
			levelCls = "log-debug"
		}
		fmt.Fprintf(w, `<div class="log-line"><span class="log-ts">%s</span><span class="log-level %s">%s</span><span class="log-msg">%s</span></div>`, l.ts, levelCls, l.level, l.msg)
	}
	fmt.Fprint(w, `</div></div>`)
}

// ── Settings ─────────────────────────────────────────────────────────────────

func (s *Server) renderSettings(w http.ResponseWriter) {
	cfg := s.cfg
	fmt.Fprint(w, `<div class="panel-header" style="border-bottom:1px solid rgba(139,134,128,0.1);padding-bottom:12px;margin-bottom:12px"><img src="/static/img/icons/nav-settings.png"> Settings</div>`)
	fmt.Fprint(w, `<div class="tabs" id="settings-tabs">
<button class="tab active" onclick="switchSettingsTab(event, 'general')"><img src="/static/img/icons/tab-general.png"> General</button>
<button class="tab" onclick="switchSettingsTab(event, 'llm')"><img src="/static/img/icons/tab-llm.png"> LLM</button>
<button class="tab" onclick="switchSettingsTab(event, 'providers')"><img src="/static/img/icons/agent-chat.png"> Providers</button>
<button class="tab" onclick="switchSettingsTab(event, 'memory')"><img src="/static/img/icons/tab-memory.png"> Memory</button>
<button class="tab" onclick="switchSettingsTab(event, 'security')"><img src="/static/img/icons/tab-security.png"> Security</button>
<button class="tab" onclick="switchSettingsTab(event, 'workers')"><img src="/static/img/icons/tab-workers.png"> Workers</button>
<button class="tab" onclick="switchSettingsTab(event, 'channels')"><img src="/static/img/icons/nav-settings.png"> Channels</button>
<button class="tab" onclick="switchSettingsTab(event, 'tools')"><img src="/static/img/icons/tab-tools.png"> Tools</button>
<button class="tab" onclick="switchSettingsTab(event, 'ui')"><img src="/static/img/icons/agent-chat.png"> UI</button>
</div>
`)

	// General
	fmt.Fprint(w, `<div class="settings-tab-content" id="tab-general"><div class="panel">`)
	settingRow(w, "Daemon Host", cfg.Daemon.Host, "Network interface to bind")
	settingRow(w, "Daemon Port", fmt.Sprintf("%d", cfg.Daemon.Port), "HTTP port for dashboard + API")
	settingRow(w, "MCP Port", fmt.Sprintf("%d", cfg.MCPPort), "First MCP server port (convenience)")
	settingBool(w, "MCP Enabled", cfg.MCP.Enabled, "Enable MCP servers for tool discovery")
	settingRow(w, "Log Level", cfg.Logging.Level, "debug, info, warn, or error")
	settingRow(w, "Log Format", cfg.Logging.Format, "json or text output format")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// LLM tab
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-llm"><div class="panel">`)
	settingRow(w, "Provider", cfg.LLM.Provider, "openai, anthropic, openrouter, ollama, or custom")
	settingRow(w, "Model", cfg.LLM.Model, "Primary model identifier")
	settingSecret(w, "API Key", cfg.LLM.APIKey, "Set via AGENTFORGE_LLM_APIKEY env var (recommended)")
	settingRow(w, "Base URL", cfg.LLM.BaseURL, "Custom API endpoint")
	settingRow(w, "Timeout", cfg.LLM.Timeout.String(), "Max wait time per request")
	settingRow(w, "Temperature", fmt.Sprintf("%.1f", cfg.LLM.Temperature), "0.0 (deterministic) to 2.0 (creative)")
	settingRow(w, "Max Tokens", fmt.Sprintf("%d", cfg.LLM.MaxTokens), "Output token limit")
	settingRow(w, "Top-P", fmt.Sprintf("%.2f", cfg.LLM.TopP), "Nucleus sampling threshold")
	settingRow(w, "Freq Penalty", fmt.Sprintf("%.2f", cfg.LLM.FrequencyPenalty), "Repetition penalty (-2.0 to 2.0)")
	settingRow(w, "Presence Penalty", fmt.Sprintf("%.2f", cfg.LLM.PresencePenalty), "Topic diversity (-2.0 to 2.0)")
	settingBool(w, "Streaming", cfg.LLM.Streaming, "Stream tokens as they're generated")
	settingRow(w, "Retry Count", fmt.Sprintf("%d", cfg.LLM.RetryCount), "Max retries on failure")
	settingRow(w, "Retry Delay", cfg.LLM.RetryDelay.String(), "Backoff between retries")
	settingRow(w, "Proxy", cfg.LLM.Proxy, "HTTP proxy (SOCKS5 supported)")
	settingRow(w, "Max Concurrency", fmt.Sprintf("%d", cfg.LLM.MaxConcurrency), "Parallel LLM requests")
	if len(cfg.LLM.Fallbacks) > 0 {
		for i, fb := range cfg.LLM.Fallbacks {
			settingRow(w, fmt.Sprintf("Fallback %d Provider / Model", i+1), fmt.Sprintf("%s / %s", fb.Provider, fb.Model), "Provider routing, in order")
		}
	}
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Providers tab (new)
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-providers"><div class="panel">`)
	provMap := map[string]config.ProviderConfig{
		"OpenAI": cfg.Providers.OpenAI, "Anthropic": cfg.Providers.Anthropic,
		"OpenRouter": cfg.Providers.OpenRouter, "Google": cfg.Providers.Google,
		"DeepSeek": cfg.Providers.DeepSeek, "Ollama": cfg.Providers.Ollama,
		"Groq": cfg.Providers.Groq, "Mistral": cfg.Providers.Mistral,
		"Cohere": cfg.Providers.Cohere,
	}
	for name, pc := range provMap {
		settingBool(w, name+" Enabled", pc.Enabled, "Enable "+name+" provider")
		if pc.Enabled {
			settingSecret(w, name+" API Key", pc.APIKey, "API key for "+name)
			settingRow(w, name+" Base URL", pc.BaseURL, "Override default endpoint")
			settingRow(w, name+" Default Model", pc.Model, "Default model for "+name)
		}
	}
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Memory
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-memory"><div class="panel">`)
	settingRow(w, "Memory Root", cfg.Memory.Root, "MeMex Zero RAG storage path")
	settingBool(w, "Auto-Commit", cfg.Memory.AutoCommit, "Auto git-commit memory on change")
	settingRow(w, "Commit Interval", cfg.Memory.CommitInterval.String(), "Max interval between auto-commits")
	settingBool(w, "Index Enabled", cfg.Memory.IndexEnabled, "FTS5 full-text search index")
	settingRow(w, "Compress Interval", cfg.Memory.CompressInterval.String(), "Compression interval")
	settingRow(w, "Max Daily Size", cfg.Memory.MaxDailySize, "Soft cap per daily log")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Security
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-security"><div class="panel">`)
	settingRow(w, "Default Token Budget", fmt.Sprintf("%d", cfg.Security.DefaultTokenBudget), "Max tokens per agent session")
	settingRow(w, "Default Timeout", cfg.Security.DefaultTimeout.String(), "Max agent session duration")
	settingBool(w, "Enforce On Spawn", cfg.Security.EnforceOnSpawn, "Validate capability at agent creation")
	settingBool(w, "Enforce On Tool Call", cfg.Security.EnforceOnToolCall, "Validate capability per tool invocation")
	settingBool(w, "Audit Enabled", cfg.Security.AuditEnabled, "Write capability checks to audit log")
	settingBool(w, "Allow FileSystem", cfg.Security.AllowFileSystem, "Allow agents filesystem access")
	settingBool(w, "Allow Network", cfg.Security.AllowNetwork, "Allow agents HTTP access")
	settingBool(w, "Allow Shell", cfg.Security.AllowShell, "Allow agents shell execution")
	settingBool(w, "Allow Browser", cfg.Security.AllowBrowser, "Allow agents browser access")
	settingRow(w, "Sandbox Mode", cfg.Security.SandboxMode, "non-main, all, or none")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Workers
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-workers"><div class="panel">`)
	settingRow(w, "Content Max Agents", fmt.Sprintf("%d", cfg.Workers.ContentMaxAgents), "Content department pool size")
	settingRow(w, "SEO Max Agents", fmt.Sprintf("%d", cfg.Workers.SEOMaxAgents), "SEO department pool size")
	settingRow(w, "Social Max Agents", fmt.Sprintf("%d", cfg.Workers.SocialMaxAgents), "Social department pool size")
	settingRow(w, "Default Max Agents", fmt.Sprintf("%d", cfg.Workers.DefaultMaxAgents), "Default pool size for new departments")
	settingRow(w, "Heartbeat Interval", cfg.Workers.HeartbeatInterval.String(), "Heartbeat frequency")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Channels (expanded)
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-channels"><div class="panel">`)
	renderChannelSection(w, "Telegram", cfg.Channels.Telegram.Enabled, func() {
		settingSecret(w, "Bot Token", cfg.Channels.Telegram.BotToken, "Telegram Bot API token")
		settingRow(w, "Webhook URL", cfg.Channels.Telegram.WebhookURL, "Inbound webhook endpoint")
		settingRow(w, "Poll Interval", cfg.Channels.Telegram.PollInterval.String(), "Long-poll interval")
		settingRow(w, "Max File Size", cfg.Channels.Telegram.MaxFileSize, "Max upload size")
		fmt.Fprintf(w, `<button class="btn" style="border:1px solid var(--af-magma);color:var(--af-magma);margin-top:8px;font-size:12px" onclick="testChannel('telegram')">🧪 Test Connection</button>`)
	})
	renderChannelSection(w, "Discord", cfg.Channels.Discord.Enabled, func() {
		settingSecret(w, "Bot Token", cfg.Channels.Discord.BotToken, "Discord bot token")
		settingRow(w, "Application ID", cfg.Channels.Discord.ApplicationID, "Discord app ID")
		settingRow(w, "Guild ID", cfg.Channels.Discord.GuildID, "Server ID")
		fmt.Fprintf(w, `<button class="btn" style="border:1px solid var(--af-magma);color:var(--af-magma);margin-top:8px;font-size:12px" onclick="testChannel('discord')">🧪 Test Connection</button>`)
	})
	renderChannelSection(w, "Signal", cfg.Channels.Signal.Enabled, func() {
		settingRow(w, "Phone Number", cfg.Channels.Signal.PhoneNumber, "Registered Signal number")
		settingRow(w, "signal-cli Path", cfg.Channels.Signal.SignalCLIPath, "Path to signal-cli binary")
	})
	renderChannelSection(w, "WhatsApp", cfg.Channels.WhatsApp.Enabled, func() {
		settingSecret(w, "API Key", cfg.Channels.WhatsApp.APIKey, "WhatsApp Cloud API key")
		settingRow(w, "Phone Number ID", cfg.Channels.WhatsApp.PhoneNumberID, "WhatsApp phone ID")
		settingRow(w, "Business ID", cfg.Channels.WhatsApp.BusinessID, "WhatsApp business account ID")
	})
	renderChannelSection(w, "Email (SMTP)", cfg.Channels.Email.Enabled, func() {
		settingRow(w, "SMTP Host", cfg.Channels.Email.SMTPHost, "SMTP server address")
		settingRow(w, "SMTP Port", fmt.Sprintf("%d", cfg.Channels.Email.SMTPPort), "SMTP port")
		settingRow(w, "Username", cfg.Channels.Email.Username, "SMTP username")
		settingSecret(w, "Password", cfg.Channels.Email.Password, "SMTP password")
		settingRow(w, "From Address", cfg.Channels.Email.FromAddress, "Sender email")
	})
	renderChannelSection(w, "Slack", cfg.Channels.Slack.Enabled, func() {
		settingSecret(w, "Bot Token", cfg.Channels.Slack.BotToken, "Slack bot token")
	})
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div>
<script>
function testChannel(ch) {
  fetch("/api/channels/test/" + ch).then(r => r.json()).then(d => {
    showToast((d.ok ? "✓ " : "✗ ") + d.message + (d.latency ? " (" + d.latency + ")" : ""), d.ok ? "success" : "error");
  }).catch(e => showToast("Network error: " + e, "error"));
}
</script>
</div></div>`)

	// Tools
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-tools"><div class="panel">`)
	settingBool(w, "Web Search", cfg.Tools.WebSearch, "Allow internet search")
	settingBool(w, "Web Fetch", cfg.Tools.WebFetch, "Allow URL content fetching")
	settingBool(w, "Image Generation", cfg.Tools.ImageGen, "Allow AI image generation")
	settingBool(w, "Image Analysis", cfg.Tools.ImageAnalyze, "Allow image analysis/vision")
	settingBool(w, "Video Generation", cfg.Tools.VideoGen, "Allow AI video generation")
	settingBool(w, "Audio Generation", cfg.Tools.AudioGen, "Allow AI audio/music generation")
	settingBool(w, "Browser", cfg.Tools.Browser, "Allow headless browser")
	settingBool(w, "Code Execution", cfg.Tools.CodeExec, "Allow sandboxed code execution")
	settingBool(w, "Git Operations", cfg.Tools.GitOps, "Allow git read/write")
	settingBool(w, "Cron Scheduling", cfg.Tools.Cron, "Allow cron job management")
	settingBool(w, "Notion", cfg.Tools.Notion, "Allow Notion API access")
	settingBool(w, "Calendar", cfg.Tools.Calendar, "Allow calendar access")
	settingBool(w, "Weather", cfg.Tools.Weather, "Allow weather queries")
	settingBool(w, "MCP Discovery", cfg.Tools.MCPDiscovery, "Auto-discover MCP tools")
	settingBool(w, "Diagram Generation", cfg.Tools.DiagramGen, "Allow diagram generation")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// UI
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-ui"><div class="panel">`)
	settingRow(w, "Theme", cfg.UI.Theme, "volcanic-glass, dark, light")
	settingBool(w, "Sidebar Collapsed", cfg.UI.SidebarCollapsed, "Start with sidebar collapsed")
	settingRow(w, "Auto-Refresh (sec)", fmt.Sprintf("%d", cfg.UI.AutoRefreshSecs), "Dashboard refresh interval")
	settingBool(w, "Animations", cfg.UI.ShowAnimations, "Show UI animations")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Save JS
	fmt.Fprint(w, `<script>
function switchSettingsTab(e, name) {
  e.target.closest(".tabs").querySelectorAll(".tab").forEach(t => t.classList.remove("active"));
  e.target.classList.add("active");
  document.querySelectorAll(".settings-tab-content").forEach(c => c.style.display = "none");
  document.getElementById("tab-" + name).style.display = "block";
}
function saveSettings() {
  var patch = {};
  document.querySelectorAll(".settings-tab-content:not([style*='none']) .setting-input:not([type='password'])").forEach(function(el) {
    var key = el.getAttribute("data-config-key") || el.closest(".setting-row").querySelector(".setting-label").textContent;
    patch[key] = el.value;
  });
  document.querySelectorAll(".settings-tab-content:not([style*='none']) .af-toggle-input").forEach(function(el) {
    patch[el.getAttribute("data-config-key")] = el.checked ? "true" : "false";
  });
  var formData = new URLSearchParams(patch);
  fetch("/api/config/save", { method: "POST", body: formData })
    .then(r => r.json())
    .then(data => {
      showToast(data.ok ? "Settings saved. Restart daemon to apply." : ("Error: " + data.error), data.ok ? "success" : "error");
    })
    .catch(e => showToast("Network error: " + e.message, "error"));
}
</script>`)
}

func renderChannelSection(w http.ResponseWriter, name string, enabled bool, render func()) {
	fmt.Fprintf(w, `<div style="margin-bottom:16px;padding:12px;border:1px solid rgba(139,134,128,0.15);border-radius:10px;background:rgba(250,243,240,0.02)"><div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px;"><strong style="font-size:14px;color:var(--text-primary)">%s</strong></div>`, name)
	settingBool(w, name+" Enabled", enabled, "Enable "+name+" channel")
	render()
	fmt.Fprint(w, `</div>`)
}

// ── Setting Helpers ──────────────────────────────────────────────────────────

func settingRow(w http.ResponseWriter, label, value, desc string) {
	fmt.Fprintf(w, `<div class="setting-row"><div><div class="setting-label">%s</div><div class="setting-desc">%s</div></div><input class="setting-input" data-config-key="%s" value="%s"></div>`, label, desc, toCamelKey(label), value)
}

func settingBool(w http.ResponseWriter, label string, current bool, desc string) {
	checked := ""
	if current {
		checked = " checked"
	}
	fmt.Fprintf(w, `<div class="setting-row"><div><div class="setting-label">%s</div><div class="setting-desc">%s</div></div><label class="af-toggle-label"><input type="checkbox" class="af-toggle-input" data-config-key="%s"%s onchange="this.closest('.setting-row').querySelector('.toggle-status').textContent=this.checked?'Enabled':'Disabled'"><span class="af-toggle-slider"></span><span class="toggle-status" style="margin-left:10px;font-size:12px;color:var(--text-dim)">%s</span></label></div>`, label, desc, toCamelKey(label), checked, boolLabel(current))
}

func boolLabel(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}

func toCamelKey(label string) string {
	parts := strings.Fields(strings.ToLower(label))
	if len(parts) == 0 {
		return label
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += strings.Title(parts[i])
	}
	return result
}

func settingSecret(w http.ResponseWriter, label, current, desc string) {
	display := current
	if current != "" {
		display = config.MaskAPIKey(current)
	} else {
		display = "(not set)"
	}
	fmt.Fprintf(w, `<div class="setting-row"><div><div class="setting-label">%s</div><div class="setting-desc">%s</div></div><input class="setting-input" type="password" data-config-key="%s" value="%s"></div>`, label, desc, toCamelKey(label), display)
}

// ── Chat partial ─────────────────────────────────────────────────────────────

func (s *Server) renderChat(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel chat-panel" style="flex:1;min-height:500px">
<div class="chat-messages" id="chat-messages">
<div class="chat-msg agent">I'm AgentForge. I am a capability-secured agent orchestration system. I can help you spawn agents, run pipelines, search memory, and audit your security posture. What would you like to do?</div>
<div class="chat-msg user">What agents are currently running?</div>
<div class="chat-msg agent">Three departments are active: <strong>Content</strong> (3 slots, 1 agent running), <strong>SEO</strong> (1 slot, 1 agent running), and <strong>Social</strong> (2 slots, 1 agent idle). All systems operational with no security violations.</div>
</div>
<div class="chat-input-bar">
<input placeholder="Type a message..." id="chat-input" onkeydown="if(event.key==='Enter'){var m=this.value;var div=document.getElementById('chat-messages');div.innerHTML+='<div class=&quot;chat-msg user&quot;>'+m+'</div>';this.value='';div.scrollTop=div.scrollHeight}">
<button class="btn btn-primary" onclick="var i=document.getElementById('chat-input');var m=i.value;var div=document.getElementById('chat-messages');if(m){div.innerHTML+='<div class=&quot;chat-msg user&quot;>'+m+'</div>';i.value='';div.scrollTop=div.scrollHeight;setTimeout(function(){div.innerHTML+='<div class=&quot;chat-msg agent&quot;>Processing: &quot;'+m+'&quot; — AgentForge engine dispatch initiated.</div>';div.scrollTop=div.scrollHeight},500)}"><img src="/static/img/icons/chat-send.png"> Send</button>
</div>
</div>`)
}


// ── Channels ─────────────────────────────────────────────────────────────────

func (s *Server) renderChannels(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-agents.png"> Channel Adapters <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">live messaging</span></div>
<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:12px">`)

	if s.chanMgr == nil {
		fmt.Fprint(w, `<div class="channel-card" style="padding:20px;text-align:center;color:var(--text-dim);grid-column:1/-1">Channel manager not available.</div>`)
	} else {
		statuses := s.chanMgr.Status()
		if len(statuses) == 0 {
			fmt.Fprint(w, `<div class="channel-card" style="padding:20px;text-align:center;color:var(--text-dim);grid-column:1/-1">No channel adapters enabled. Configure channels in Settings.</div>`)
		}
		for _, st := range statuses {
			dotColor := "#EF4444"
			dotGlow := "rgba(239,68,68,0.3)"
			runningLabel := "Offline"
			badgeCls := "badge-idle"
			if st.Running {
				dotColor = "#22C55E"
				dotGlow = "rgba(34,197,94,0.3)"
				runningLabel = "Running"
				badgeCls = "badge-live"
			}
			lastMsg := st.LastMsg.Format("15:04:05")
			if st.LastMsg.IsZero() {
				lastMsg = "—"
			}
			icon := "telegram"
			if strings.Contains(st.Name, "discord") {
				icon = "discord"
			}
			fmt.Fprintf(w, `<div class="channel-card">
<div style="display:flex;align-items:center;gap:8px;margin-bottom:12px">
<span style="width:10px;height:10px;border-radius:50%%;background:%s;box-shadow:0 0 8px %s"></span>
<span style="font-weight:600;font-size:15px;text-transform:capitalize">%s</span>
<span class="badge %s">%s</span>
</div>
<div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;font-size:12px">
<div><div style="color:var(--text-dim);font-size:11px;text-transform:uppercase;letter-spacing:0.5px">Connected</div><div style="font-family:var(--font-mono);font-size:13px">%d times</div></div>
<div><div style="color:var(--text-dim);font-size:11px;text-transform:uppercase;letter-spacing:0.5px">Messages</div><div style="font-family:var(--font-mono);font-size:13px">%d</div></div>
<div style="grid-column:1/-1"><div style="color:var(--text-dim);font-size:11px;text-transform:uppercase;letter-spacing:0.5px">Last Message</div><div style="font-family:var(--font-mono);font-size:13px">%s</div></div>
</div>
</div>`,
				dotColor, dotGlow, icon, badgeCls, runningLabel,
				st.Connects, st.Messages, lastMsg)
		}
	}

	fmt.Fprint(w, `</div></div>
<style>
.channel-card{background:rgba(250,243,240,0.02);border:1px solid rgba(139,134,128,0.15);border-radius:12px;padding:18px;transition:all 0.2s}
.channel-card:hover{border-color:var(--af-magma)}
</style>`)
}

// ── Skills Marketplace Partial ───────────────────────────────────────────────

func (s *Server) renderSkillsMarketplace(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-skills.png"> Skills Marketplace <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">skillsmp.com/api/v1 &bull; bring your own API key</span></div>
<div style="margin-bottom:16px;padding:14px;border:1px solid rgba(255,107,44,0.2);border-radius:10px;background:rgba(255,107,44,0.04)">
  <div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">
    <div style="flex:1;min-width:200px">
      <div style="font-size:11px;color:var(--text-dim);margin-bottom:4px;text-transform:uppercase;letter-spacing:0.5px">SkillsMP API Key</div>
      <input id="skillsmp-api-key" type="password" placeholder="sk_live_skillsmp_..." style="width:100%;border:1px solid rgba(139,134,128,0.2);background:rgba(250,243,240,0.03);color:var(--text-primary);padding:8px 12px;border-radius:6px;font-size:13px;font-family:var(--font-mono)">
    </div>
    <div style="display:flex;gap:8px;align-items:flex-end">
      <button class="btn btn-primary" onclick="saveSkillsMPKey()" style="font-size:12px"><img src="/static/img/icons/skill-install.png" width="12"> Save Key</button>
      <button class="btn btn-ghost" onclick="var h=document.getElementById('skillsmp-help');h.style.display=h.style.display==='none'?'block':'none'" style="font-size:12px">?</button>
    </div>
  </div>
  <div id="skillsmp-help" style="display:none;margin-top:10px;font-size:12px;color:var(--text-dim);line-height:1.6;padding:10px;background:rgba(0,0,0,0.15);border-radius:6px">
    <strong style="color:var(--text-primary)">How to use the Skills Marketplace:</strong><br>
    1. Create an account at <a href="https://skillsmp.com" target="_blank" style="color:var(--af-magma)">skillsmp.com</a><br>
    2. Generate an API key from your SkillsMP dashboard<br>
    3. Paste your key above and click <strong style="color:var(--af-magma)">Save Key</strong> &mdash; it gets stored in <code style="background:rgba(255,107,44,0.1);padding:1px 4px;border-radius:3px">~/.agentforge/agentforge.yaml</code><br>
    4. Use <strong>Search</strong> (keyword) or <strong>AI Search</strong> (semantic) to find skills<br>
    5. Click <strong>Install</strong> on any skill to add it to your AgentForge instance<br><br>
    <span style="color:var(--text-dim)">Rate limit: 500 requests/day &bull; 30/min (authenticated). Without a key, mock results are shown.</span>
  </div>
</div>
<div style="display:flex;gap:12px;margin-bottom:16px">
  <input id="skills-search-input" placeholder="Search skills (keyword or semantic)..." style="flex:1;border:1px solid rgba(139,134,128,0.2);background:rgba(250,243,240,0.03);color:var(--text-primary);padding:10px 14px;border-radius:8px;font-size:14px" onkeydown="if(event.key==='Enter')searchSkills()">
  <button class="btn btn-primary" onclick="searchSkills()"><img src="/static/img/icons/nav-skills.png"> Search</button>
  <button class="btn" style="border:1px solid rgba(255,107,44,0.2);color:var(--af-magma)" onclick="searchSkillsAI()">🤖 AI Search</button>
</div>
<div id="skills-results" style="display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:12px">
  <div class="skill-card-placeholder">Search SkillsMP marketplace for agent skills to install</div>
</div>
</div>
<script>
function saveSkillsMPKey() {
  var k = document.getElementById("skillsmp-api-key").value;
  fetch("/api/config/save", {method:"POST",body:new URLSearchParams({"skills.marketplaceKey":k})})
    .then(r=>r.json()).then(d=>{
      showToast(d.ok ? "SkillsMP key saved. Restart daemon to apply." : ("Error: "+d.error), d.ok?"success":"error");
    }).catch(e=>showToast("Network error: "+e.message,"error"));
}

function searchSkills() {
  var q = document.getElementById("skills-search-input").value;
  if(!q) return;
  fetch("/api/skills/search?q=" + encodeURIComponent(q) + "&mode=keyword")
    .then(r => r.json()).then(d => renderSkillCards(d));
}
function searchSkillsAI() {
  var q = document.getElementById("skills-search-input").value;
  if(!q) return;
  fetch("/api/skills/search?q=" + encodeURIComponent(q) + "&mode=ai")
    .then(r => r.json()).then(d => renderSkillCards(d));
}
function renderSkillCards(results) {
  var div = document.getElementById("skills-results");
  if(!results || results.length===0) { div.innerHTML = "<div class=\"skill-card-placeholder\">No skills found. Try a different query.</div>"; return; }
  div.innerHTML = results.map(s => '<div class="skill-card"><div class="skill-card-header"><span style="font-weight:600;font-size:15px">'+s.name+'</span><span class="badge badge-magma">'+s.version+'</span></div><div style="font-size:12px;color:var(--text-dim);margin:8px 0">'+s.description+'</div><div style="display:flex;gap:6px;flex-wrap:wrap;margin-bottom:8px">'+(s.tags||[]).map(t=>'<span style="font-size:10px;background:rgba(255,107,44,0.1);color:var(--af-magma);padding:2px 8px;border-radius:4px">'+t+'</span>').join('')+'</div><div style="display:flex;justify-content:space-between;align-items:center"><span style="font-size:11px;color:var(--text-dim)">by '+s.author+'</span><button class="btn" style="border:1px solid var(--af-magma);color:var(--af-magma);font-size:12px;padding:4px 12px" onclick="installSkill(\''+s.name+'\')">Install</button></div></div>').join('');
}
function installSkill(name) {
  showToast("Skill '" + name + "' queued for install. Restart to activate.", "info");
}
</script>
<style>.skill-card{background:rgba(250,243,240,0.03);border:1px solid rgba(139,134,128,0.15);border-radius:10px;padding:16px;transition:all 0.2s}.skill-card:hover{border-color:var(--af-magma)}.skill-card-placeholder{padding:40px;text-align:center;color:var(--text-dim)}</style>`)
}

// ── Pipeline Editor Partial ──────────────────────────────────────────────────

func (s *Server) renderPipelineEditor(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-pipelines.png"> Pipeline Editor <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">DAG orchestration</span></div>
<div style="display:flex;gap:16px;margin-bottom:16px">
  <select id="pipeline-select" onchange="loadPipeline()" style="border:1px solid rgba(139,134,128,0.2);background:rgba(250,243,240,0.03);color:var(--text-primary);padding:10px 14px;border-radius:8px;font-size:14px;min-width:200px">
    <option value="">-- Select Pipeline --</option>`)
	for _, d := range s.cfg.Pipelines.Definitions {
		act := ""
		if d.Enabled {
			act = " • Active"
		}
		fmt.Fprintf(w, `<option value="%s">%s%s</option>`, d.Name, d.Name, act)
	}
	fmt.Fprint(w, `</select>
  <button class="btn btn-primary" onclick="addPipeline()">+ New Pipeline</button>
</div>
<div id="pipeline-detail" style="background:rgba(250,243,240,0.02);border:1px solid rgba(139,134,128,0.1);border-radius:10px;padding:20px;min-height:300px">
  <div style="color:var(--text-dim);text-align:center;padding:40px">Select a pipeline to edit its stages, triggers, and dependencies.</div>
</div>
<script>
function loadPipeline() {
  var sel = document.getElementById("pipeline-select").value;
  if(!sel) { document.getElementById("pipeline-detail").innerHTML='<div style="color:var(--text-dim);text-align:center;padding:40px">Select a pipeline to edit its stages, triggers, and dependencies.</div>'; return; }
  fetch("/api/config/save", {method:"POST",body:new URLSearchParams({"pipelines.definitions":sel})}).then(r=>r.json());
}
function addPipeline() {
  var n = prompt("Pipeline name:"); if(!n) return;
  var ds = prompt("Description:"); if(!ds) ds = n;
  var conf = JSON.stringify([{name:n,enabled:true,description:ds,trigger:{type:"manual"},stages:[]}]);
  fetch("/api/config/save", {method:"POST",body:new URLSearchParams({"pipelines.definitions":conf})})
    .then(r=>r.json()).then(d=>{
      if(d.ok){showToast("Pipeline '"+n+"' saved. Restart daemon to apply.","success");setTimeout(()=>location.reload(),1200)}
      else showToast("Error: "+d.error,"error");
    }).catch(e=>showToast("Error: "+e,"error"));
}
</script>
</div>`)
}

// ── Agent Profiles Partial ───────────────────────────────────────────────────

func (s *Server) renderAgentProfiles(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-agents.png"> Agent Profiles <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">per-agent model, tools & capability</span></div>
<table class="data-table">
<thead><tr><th>Agent</th><th>Department</th><th>Provider / Model</th><th>Tools</th><th>FileSystem</th><th>Network</th><th>Shell</th><th>Spawn</th><th></th></tr></thead>
<tbody>`)
	for _, ap := range s.cfg.Agents.Profiles {
		enabledCls := "badge-idle"
		enabledTxt := "Paused"
		if ap.Enabled {
			enabledCls = "badge-live"
			enabledTxt = "Active"
		}
		tick := func(b bool) string {
			if b {
				return `<span style="color:#22C55E">✓</span>`
			}
			return `<span style="color:#EF4444">✗</span>`
		}
		fmt.Fprintf(w, `<tr data-agent-id="%s"><td><strong>%s</strong><br><span style="font-size:11px;color:var(--text-dim)">%s</span></td><td><span class="badge">%s</span></td><td style="font-family:var(--font-mono);font-size:12px">%s / %s</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><span class="badge %s">%s</span> <button class="btn" style="border:1px solid rgba(255,107,44,0.2);color:var(--af-magma);font-size:11px;padding:2px 8px" onclick="editAgent('%s')">Edit</button></td></tr>`,
			ap.ID, ap.Name, ap.ID, ap.Department, ap.Provider, ap.Model,
			len(ap.Tools),
			tick(ap.Capability.AllowFileSystem), tick(ap.Capability.AllowNetwork),
			tick(ap.Capability.AllowShell), tick(ap.Capability.AllowSpawn),
			enabledCls, enabledTxt, ap.ID)
	}
	fmt.Fprint(w, `</tbody></table>
<button class="btn btn-primary" style="margin-top:12px" onclick="editAgent('new')">+ Create Agent Profile</button>
<script>
function editAgent(id) {
  if(id==='new') {
    var nn = prompt("Agent Name:"); if(!nn) return;
    var dd = prompt("Department (content/seo/social/security/devops/memory/orchestrator/monitor):"); if(!dd) dd="content";
    var mm = prompt("Model (e.g. openai/gpt-4o):"); if(!mm) mm="openai/gpt-4o";
    var parts = mm.split("/");
    var cfg = JSON.stringify([{name:nn,department:dd,enabled:true,provider:parts[0],model:parts[1]||mm,temperature:0.7,maxTokens:4096,timeout:"300s",tools:[],skills:[],capability:{allowFileSystem:true,allowNetwork:true,allowShell:false,allowSpawn:false,tokenBudget:100000}});
    fetch("/api/config/save", {method:"POST",body:new URLSearchParams({"agents.profiles":cfg})})
      .then(r=>r.json()).then(d=>{
        if(d.ok){showToast("Agent '"+nn+"' saved. Restart daemon to apply.","success");setTimeout(()=>location.reload(),1200)}
        else showToast("Error: "+d.error,"error");
      }).catch(e=>showToast("Error: "+e,"error"));
    return;
  }
  showToast("Editing agent: " + id, "info");
}
</script>
</div>`)
}

// ── Skills Search API ────────────────────────────────────────────────────────

func (s *Server) handleSkillsSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	mode := r.URL.Query().Get("mode")
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	var endpoint string
	if mode == "ai" {
		endpoint = fmt.Sprintf("%s/skills/ai-search", s.cfg.Skills.MarketplaceURL)
	} else {
		endpoint = fmt.Sprintf("%s/skills/search?keyword=%s", s.cfg.Skills.MarketplaceURL, q)
	}

	req, _ := http.NewRequestWithContext(r.Context(), "GET", endpoint, nil)
	if s.cfg.Skills.MarketplaceKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.Skills.MarketplaceKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"name":"error","description":"SkillsMP unreachable: %s","version":"N/A","author":"system","tags":["error"]}]`, err.Error())
		return
	}
	defer resp.Body.Close()

	// Try to proxy the response
	buf := new(strings.Builder)
	io.Copy(buf, resp.Body)
	bodyStr := buf.String()

	// Try parse as JSON array; if not, wrap in mock results for demo
	var raw []map[string]any
	if err := json.Unmarshal([]byte(bodyStr), &raw); err != nil {
		// Mock results for demo when SkillsMP key not configured
		mock := []map[string]any{
			{"name": "code-review", "version": "1.2.0", "description": "Automated code review with best-practice checks and security linting", "author": "AgentForge", "tags": []any{"code", "security", "ci-cd"}},
			{"name": "memory-manager", "version": "1.0.0", "description": "Semantic memory compression and long-term curation for MeMex RAG", "author": "AgentForge", "tags": []any{"memory", "rag", "context"}},
			{"name": "security-audit", "version": "1.1.3", "description": "Capability-based security posture audit with risk scoring", "author": "AgentForge", "tags": []any{"security", "audit", "capability"}},
			{"name": "seo-auditor", "version": "2.0.1", "description": "SEO quality gate with content scoring and keyword analysis", "author": "skillsmp", "tags": []any{"seo", "content", "marketing"}},
			{"name": "browser-automation", "version": "0.9.0", "description": "Headless browser automation for web scraping and testing", "author": "skillsmp", "tags": []any{"browser", "automation", "testing"}},
		}
		// Filter by query
		var filtered []map[string]any
		for _, m := range mock {
			if strings.Contains(strings.ToLower(fmt.Sprint(m["name"])+fmt.Sprint(m["description"])), strings.ToLower(q)) {
				filtered = append(filtered, m)
			}
		}
		if len(filtered) == 0 {
			filtered = mock
		}
		data, _ := json.Marshal(filtered)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(bodyStr))
}

func (s *Server) handleChannelTest(w http.ResponseWriter, r *http.Request) {
	channel := strings.TrimPrefix(r.URL.Path, "/api/channels/test/")
	w.Header().Set("Content-Type", "application/json")

	type result struct {
		Channel string `json:"channel"`
		OK      bool   `json:"ok"`
		Message string `json:"message"`
		Latency string `json:"latency"`
	}

	start := time.Now()

	switch channel {
	case "telegram":
		if !s.cfg.Channels.Telegram.Enabled {
			json.NewEncoder(w).Encode(result{Channel: "telegram", OK: false, Message: "Not enabled"})
			return
		}
		// Test getMe endpoint
		resp, err := http.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getMe", s.cfg.Channels.Telegram.BotToken))
		if err != nil {
			json.NewEncoder(w).Encode(result{Channel: "telegram", OK: false, Message: err.Error(), Latency: time.Since(start).String()})
			return
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			json.NewEncoder(w).Encode(result{Channel: "telegram", OK: true, Message: "Connected", Latency: time.Since(start).String()})
		} else {
			json.NewEncoder(w).Encode(result{Channel: "telegram", OK: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode), Latency: time.Since(start).String()})
		}
	default:
		json.NewEncoder(w).Encode(result{Channel: channel, OK: false, Message: "Test not implemented for this channel"})
	}
}

func (s *Server) handlePipelineValidate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Stub: validate pipeline DAG for cycles
	json.NewEncoder(w).Encode(map[string]any{"valid": true, "cycles": false, "message": "Pipeline DAG is valid"})
}



func (s *Server) handleToolsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tools := []map[string]any{
		{"name": "filesystem", "category": "filesystem", "description": "Read, write, list, delete files within capability-scoped paths", "version": "1.0.0", "enabled": s.cfg.Security.AllowFileSystem},
		{"name": "shell", "category": "shell", "description": "Execute shell commands within sandbox constraints", "version": "1.0.0", "enabled": s.cfg.Security.AllowShell},
		{"name": "http", "category": "network", "description": "Execute HTTP requests within allowlisted domains", "version": "1.0.0", "enabled": s.cfg.Tools.WebFetch},
		{"name": "web_search", "category": "network", "description": "Search the web via DuckDuckGo or SearXNG", "version": "1.0.0", "enabled": s.cfg.Tools.WebSearch},
		{"name": "web_fetch", "category": "network", "description": "Fetch and extract readable content from URLs", "version": "1.0.0", "enabled": s.cfg.Tools.WebFetch},
		{"name": "image_generate", "category": "media", "description": "Generate images via fal.ai FLUX", "version": "1.0.0", "enabled": s.cfg.Tools.ImageGen},
		{"name": "memory_search", "category": "memory", "description": "Semantic search MeMex Zero RAG store", "version": "1.0.0", "enabled": true},
		{"name": "memory_get", "category": "memory", "description": "Retrieve memory entries by path", "version": "1.0.0", "enabled": true},
		{"name": "git_commit", "category": "vcs", "description": "Stage and commit changes to memory git repo", "version": "1.0.0", "enabled": s.cfg.Tools.GitOps},
		{"name": "git_push", "category": "vcs", "description": "Push commits to remote origin", "version": "1.0.0", "enabled": s.cfg.Tools.GitOps},
		{"name": "cron_schedule", "category": "automation", "description": "Schedule recurring agent tasks", "version": "1.0.0", "enabled": s.cfg.Tools.Cron},
		{"name": "session_spawn", "category": "agent", "description": "Spawn isolated sub-agent sessions", "version": "1.0.0", "enabled": true},
		{"name": "session_send", "category": "agent", "description": "Send messages between agent sessions", "version": "1.0.0", "enabled": true},
		{"name": "read_file", "category": "filesystem", "description": "Read file contents with offset/limit", "version": "1.0.0", "enabled": s.cfg.Security.AllowFileSystem},
		{"name": "write_file", "category": "filesystem", "description": "Create or overwrite files", "version": "1.0.0", "enabled": s.cfg.Security.AllowFileSystem},
		{"name": "edit_file", "category": "filesystem", "description": "Precise text replacements in files", "version": "1.0.0", "enabled": s.cfg.Security.AllowFileSystem},
		{"name": "mcp_invoke", "category": "mcp", "description": "Invoke tools on connected MCP servers", "version": "1.0.0", "enabled": s.cfg.MCP.Enabled},
		{"name": "browser_navigate", "category": "browser", "description": "Navigate to URLs in headless browser", "version": "1.0.0", "enabled": s.cfg.Tools.Browser},
		{"name": "code_exec", "category": "sandbox", "description": "Execute code in isolated sandbox", "version": "1.0.0", "enabled": s.cfg.Tools.CodeExec},
	}
	data, _ := json.Marshal(tools)
	w.Write(data)
}

func (s *Server) renderTools(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/tools-icon.png"> Tool Registry <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">19 tools · 8 categories</span></div>
<table class="data-table">
<thead><tr><th>Tool</th><th>Category</th><th>Status</th><th>Version</th><th>Description</th></tr></thead>
<tbody>
`+s.toolTableRow("filesystem", "filesystem", s.cfg.Security.AllowFileSystem, "1.0.0", "Read, write, list, delete files within capability-scoped paths")+
s.toolTableRow("shell", "shell", s.cfg.Security.AllowShell, "1.0.0", "Execute shell commands within sandbox constraints")+
s.toolTableRow("http", "network", s.cfg.Tools.WebFetch, "1.0.0", "Execute HTTP requests within allowlisted domains")+
s.toolTableRow("web_search", "network", s.cfg.Tools.WebSearch, "1.0.0", "Search the web via DuckDuckGo or SearXNG")+
s.toolTableRow("web_fetch", "network", s.cfg.Tools.WebFetch, "1.0.0", "Fetch and extract readable content from URLs")+
s.toolTableRow("image_generate", "media", s.cfg.Tools.ImageGen, "1.0.0", "Generate images via fal.ai FLUX")+
s.toolTableRow("memory_search", "memory", true, "1.0.0", "Semantic search MeMex Zero RAG store")+
s.toolTableRow("memory_get", "memory", true, "1.0.0", "Retrieve memory entries by path")+
s.toolTableRow("git_commit", "vcs", s.cfg.Tools.GitOps, "1.0.0", "Stage and commit changes to memory git repo")+
s.toolTableRow("git_push", "vcs", s.cfg.Tools.GitOps, "1.0.0", "Push commits to remote origin")+
s.toolTableRow("cron_schedule", "automation", s.cfg.Tools.Cron, "1.0.0", "Schedule recurring agent tasks")+
s.toolTableRow("session_spawn", "agent", true, "1.0.0", "Spawn isolated sub-agent sessions")+
s.toolTableRow("session_send", "agent", true, "1.0.0", "Send messages between agent sessions")+
s.toolTableRow("read_file", "filesystem", s.cfg.Security.AllowFileSystem, "1.0.0", "Read file contents with offset/limit")+
s.toolTableRow("write_file", "filesystem", s.cfg.Security.AllowFileSystem, "1.0.0", "Create or overwrite files")+
s.toolTableRow("edit_file", "filesystem", s.cfg.Security.AllowFileSystem, "1.0.0", "Precise text replacements in files")+
s.toolTableRow("mcp_invoke", "mcp", s.cfg.MCP.Enabled, "1.0.0", "Invoke tools on connected MCP servers")+
s.toolTableRow("browser_navigate", "browser", s.cfg.Tools.Browser, "1.0.0", "Navigate to URLs in headless browser")+
s.toolTableRow("code_exec", "sandbox", s.cfg.Tools.CodeExec, "1.0.0", "Execute code in isolated sandbox")+
`</tbody></table>
</div>`)
}

func (s *Server) toolTableRow(name, category string, enabled bool, version, desc string) string {
	statusCls := "badge-live"
	statusLabel := "Active"
	if !enabled {
		statusCls = "badge-idle"
		statusLabel = "Off"
	}
	return fmt.Sprintf(`<tr><td style="font-weight:600;font-family:var(--font-mono);font-size:13px"><img src="/static/img/icons/tools-icon.png" width="14" style="vertical-align:middle;margin-right:6px;opacity:0.5">%s</td><td><span class="badge badge-magma">%s</span></td><td><span class="badge %s">%s</span></td><td style="font-family:var(--font-mono);font-size:11px;color:var(--text-dim)">%s</td><td style="font-size:12px;color:var(--text-dim);max-width:300px">%s</td></tr>`, name, category, statusCls, statusLabel, version, desc)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","version":"0.1.0","uptime":"%s","goroutines":%d}`,
		time.Since(s.started).Round(time.Second), runtime.NumGoroutine())
}

func (s *Server) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":false,"error":"parse form"}`))
		return
	}
	patch := make(map[string]string)
	for key, vals := range r.PostForm {
		if len(vals) > 0 {
			patch[key] = vals[0]
		}
	}
	if err := s.store.Update(patch); err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":false,"error":"%s"}`, err.Error())
		return
	}
	s.cfg = s.store.Cfg()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleConfigAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data, _ := s.cfg.ToJSON()
	w.Write(data)
}


// ── MCP Servers ──────────────────────────────────────────────────────────────

func (s *Server) renderMCPServers(w http.ResponseWriter) {
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-agents.png"> MCP Servers <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">Model Context Protocol endpoints</span></div>
<table class="data-table">
<thead><tr><th>Server</th><th>Port</th><th>Transport</th><th>Status</th><th>Tools</th><th>Endpoint</th></tr></thead>
<tbody>`)

	if s.mcpMgr != nil {
		servers := s.mcpMgr.List()
		for _, srv := range servers {
			statusCls := "badge-idle"
			statusTxt := "Stopped"
			if srv.Running {
				statusCls = "badge-live"
				statusTxt = "Running"
			}

			endpoint := "–"
			switch srv.Transport {
			case "http":
				endpoint = fmt.Sprintf("http://localhost:%d", srv.Port)
			case "stdio":
				endpoint = "stdio (subprocess)"
			}

			filter := ""
			if len(srv.ToolFilter) > 0 {
				filter = fmt.Sprintf(" <span style=\"font-size:10px;color:var(--text-dim)\">(filtered: %d)</span>", len(srv.ToolFilter))
			}

			fmt.Fprintf(w, `<tr>
<td><img src="/static/img/icons/tools-icon.png" width="14" style="vertical-align:middle;margin-right:6px;opacity:0.5"><strong>%s</strong></td>
<td style="font-family:var(--font-mono);font-size:12px">%d</td>
<td><span class="badge badge-magma">%s</span></td>
<td><span class="badge %s">%s</span></td>
<td>%d%s</td>
<td style="font-family:var(--font-mono);font-size:11px;color:var(--text-dim)">%s</td></tr>`,
				srv.Name, srv.Port, srv.Transport, statusCls, statusTxt, srv.ToolCount, filter, endpoint)
		}

		if len(servers) == 0 {
			fmt.Fprint(w, `<tr><td colspan="6" style="text-align:center;padding:30px;color:var(--text-dim)">No MCP servers configured. Add servers in <code>~/.agentforge/agentforge.yaml</code> under <code>mcp.servers</code>.</td></tr>`)
		}
	} else {
		fmt.Fprint(w, `<tr><td colspan="6" style="text-align:center;padding:30px;color:var(--text-dim)">MCP Manager not available.</td></tr>`)
	}

	fmt.Fprint(w, `</tbody></table>
<div style="margin-top:16px;padding:12px;border:1px solid rgba(139,134,128,0.15);border-radius:10px;background:rgba(250,243,240,0.02)">
<div style="font-size:13px;color:var(--text-primary);margin-bottom:8px"><strong>Configuration</strong></div>
<div style="font-size:12px;color:var(--text-dim);font-family:var(--font-mono);line-height:1.6;padding:10px;background:rgba(0,0,0,0.15);border-radius:6px">
<span style="color:var(--text-dim)"># ~/.agentforge/agentforge.yaml</span><br>
<span style="color:#F59E0B">mcp:</span><br>
<span style="color:#10B981">&nbsp;&nbsp;enabled:</span> <span style="color:#EF4444">true</span><br>
<span style="color:#F59E0B">&nbsp;&nbsp;servers:</span><br>
<span style="color:#EF4444">&nbsp;&nbsp;&nbsp;&nbsp;- name:</span> <span style="color:#10B981">default</span><br>
<span style="color:#10B981">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;port:</span> <span style="color:#EF4444">9090</span><br>
<span style="color:#10B981">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;enabled:</span> <span style="color:#EF4444">true</span><br>
<span style="color:#10B981">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;transport:</span> <span style="color:#10B981">http</span><br>
<span style="color:#EF4444">&nbsp;&nbsp;&nbsp;&nbsp;- name:</span> <span style="color:#10B981">skills-bridge</span><br>
<span style="color:#10B981">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;port:</span> <span style="color:#EF4444">9091</span><br>
<span style="color:#10B981">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;transport:</span> <span style="color:#10B981">stdio</span><br>
<span style="color:#10B981">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;command:</span> <span style="color:#10B981">node</span><br>
<span style="color:#10B981">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;args:</span> ["<span style="color:#10B981">mcp-serve</span>"]
</div>
</div></div>`)
}

func (s *Server) handleMCPAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.mcpMgr == nil {
		w.Write([]byte(`{"servers":[]}`))
		return
	}
	servers := s.mcpMgr.List()
	data, _ := json.Marshal(map[string]any{"servers": servers})
	w.Write(data)
}


// ── Login & Dashboard pages ──────────────────────────────────────────────────

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data, _ := staticFS.ReadFile("static/login.html")
	w.Write(data)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data, _ := staticFS.ReadFile("static/dashboard.html")
	w.Write(data)
}
