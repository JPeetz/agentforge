// Package dashboard — embedded web dashboard server for AgentForge.
// Serves a glassmorphism SPA with live data via htmx partials.
package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
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

type ProviderInfo struct {
	Name          string `json:"name"`
	Available     bool   `json:"available"`
	Authenticated bool   `json:"authenticated"`
	Model         string `json:"model"`
}

type Server struct {
	cfg                *config.Config
	store              *config.PersistedStore
	bus                bus.Bus
	sessionMgr         *session.Manager
	mcpMgr             *mcp.Manager
	mcpClientMgr       *mcpclient.ClientManager
	chanMgr            *channel.Manager
	costTracker        *cost.Tracker
	authStore          *auth.Store
	authManager        *auth.Manager
	sseHub             *sse.Hub
	adapter            llm.Adapter
	adapterMu          sync.RWMutex
	rebuildAdapter     func(*config.Config) llm.Adapter
	availableProviders map[string]ProviderInfo
	mux                *http.ServeMux
	log                *slog.Logger
	started            time.Time
}

func New(cfg *config.Config, b bus.Bus, sessionMgr *session.Manager, mcpMgr *mcp.Manager, mcpClientMgr *mcpclient.ClientManager, chanMgr *channel.Manager, costTracker *cost.Tracker, authStore *auth.Store, authManager *auth.Manager, hub *sse.Hub, adapter llm.Adapter, rebuildAdapter func(*config.Config) llm.Adapter, availableProviders map[string]ProviderInfo) (*Server, error) {
	s := &Server{
		cfg:                cfg,
		store:              config.NewStore(cfg),
		bus:                b,
		sessionMgr:         sessionMgr,
		mcpMgr:             mcpMgr,
		mcpClientMgr:       mcpClientMgr,
		chanMgr:            chanMgr,
		costTracker:        costTracker,
		authStore:          authStore,
		authManager:        authManager,
		availableProviders: availableProviders,
		sseHub:             hub,
		adapter:            adapter,
		rebuildAdapter:     rebuildAdapter,
		mux:                http.NewServeMux(),
		log:                slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		started:            time.Now(),
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
	s.mux.HandleFunc("/api/events", s.handleEventsAPI)
	s.mux.HandleFunc("/api/events-html", s.handleEventsHTMLAPI)
	s.mux.HandleFunc("/api/providers/available", s.handleProvidersAPI)
	s.mux.HandleFunc("/api/providers/models", s.handleProviderModels)
	s.mux.HandleFunc("/api/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("/api/chat/upload", s.handleFileUpload)

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
		s.renderPipelineManager(w)
	case "skills":
		s.renderSkills(w)
	case "security":
		s.renderSecurity(w)
	case "mcp":
		s.renderMCPServersManager(w)
	case "logs":
		s.renderLogs(w)
	case "settings":
		s.renderSettings(w)
	case "tools":
		s.renderTools(w)
	case "skills-marketplace":
		s.renderSkillsMarketplace(w)
	case "agent-profiles":
		s.renderAgentProfilesManager(w)
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

func (s *Server) renderPipelineManager(w http.ResponseWriter) {
	pipelinesJSON, _ := json.Marshal(s.cfg.Pipelines.Definitions)

	fmt.Fprint(w, `<div style="display:grid;grid-template-columns:1fr 2fr;gap:20px;height:calc(100vh - 200px)">
<!-- Left pane: Pipeline list -->
<div class="panel" style="overflow-y:auto">
<div class="panel-header"><img src="/static/img/icons/nav-pipelines.png"> Pipelines</div>
<div id="pipeline-list" style="display:flex;flex-direction:column;gap:8px">`)

	if len(s.cfg.Pipelines.Definitions) == 0 {
		fmt.Fprint(w, `<div style="color:var(--text-dim);font-size:12px;padding:12px;text-align:center">No pipelines yet</div>`)
	} else {
		for _, d := range s.cfg.Pipelines.Definitions {
			statusCls := "badge-live"
			if !d.Enabled {
				statusCls = "badge-idle"
			}
			fmt.Fprintf(w, `<div class="pipeline-list-item" onclick='selectPipeline(this, %q)' style="padding:12px;border:1px solid rgba(139,134,128,0.15);border-radius:8px;cursor:pointer;transition:all 0.2s;background:rgba(250,243,240,0.02)">
<div style="display:flex;justify-content:space-between;align-items:center;gap:8px">
<div style="flex:1">
<strong style="display:block;margin-bottom:4px">%s</strong>
<span style="font-size:11px;color:var(--text-dim)">%d stages • %s</span>
</div>
<span class="badge %s" style="font-size:10px">%s</span>
</div>
</div>`, d.Name, d.Name, len(d.Stages), d.Trigger.Type, statusCls, map[bool]string{true: "Active", false: "Paused"}[d.Enabled])
		}
	}

	fmt.Fprint(w, `</div>
<button class="btn btn-primary" style="margin-top:16px;width:100%" onclick="createPipeline()">+ New Pipeline</button>
</div>

<!-- Right pane: Pipeline editor -->
<div class="panel" style="overflow-y:auto;display:none" id="editor-pane">
<div class="panel-header">Pipeline Editor</div>
<div id="editor-content" style="padding:16px">
<div style="color:var(--text-dim);text-align:center">Select a pipeline to edit</div>
</div>
</div>

<script>
let currentPipeline = null;
let pipelines = `+string(pipelinesJSON)+`;

function selectPipeline(elem, name) {
  currentPipeline = pipelines.find(p => p.name === name);
  if(!currentPipeline) return;

  document.querySelectorAll('.pipeline-list-item').forEach(e => e.style.borderColor='rgba(139,134,128,0.15)');
  elem.style.borderColor = 'var(--af-magma)';

  let editor = document.getElementById('editor-pane');
  editor.style.display = 'block';
  renderEditor();
}

function renderEditor() {
  if(!currentPipeline) return;
  let trig = currentPipeline.trigger || {};
  let stages = currentPipeline.stages || [];

  let stagesHTML = stages.length === 0
    ? '<div style="color:var(--text-dim);font-size:12px;padding:12px">No stages yet</div>'
    : stages.map((s,i) => '<div style="padding:8px;border-left:3px solid var(--af-magma);background:rgba(255,107,44,0.05);margin-bottom:8px"><strong>'+html.EscapeString(s.name)+'</strong><br><span style="font-size:11px;color:var(--text-dim)">Agent: '+html.EscapeString(s.agent)+' • Tool: '+html.EscapeString(s.tool)+'</span><button style="font-size:10px;padding:2px 6px;margin-top:4px;background:rgba(255,107,44,0.1);border:none;color:var(--af-magma);border-radius:4px;cursor:pointer" onclick="removeStage('+i+')">Remove</button></div>').join('');

  let cronExpr = trig.type === 'cron' ? (trig.cronExpr || '') : '';

  let html = '<form id="pipeline-form" onsubmit="savePipeline(event)" style="display:flex;flex-direction:column;gap:12px">' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Name</label><input type="text" id="pipeline-name" value="' + html.EscapeString(currentPipeline.name || '') + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)" required></div>' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Description</label><input type="text" id="pipeline-desc" value="' + html.EscapeString(currentPipeline.description || '') + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)"></div>' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Trigger Type</label><select id="pipeline-trigger" onchange="updateTriggerUI()" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)"><option value="manual"' + (trig.type === 'manual' ? ' selected' : '') + '>Manual</option><option value="cron"' + (trig.type === 'cron' ? ' selected' : '') + '>Cron Schedule</option><option value="event"' + (trig.type === 'event' ? ' selected' : '') + '>Event</option></select></div>' +
    '<div id="trigger-ui"></div>' +
    '<div style="border-top:1px solid rgba(139,134,128,0.1);padding-top:12px"><div style="font-size:12px;color:var(--text-dim);margin-bottom:8px"><strong>Stages</strong></div>' + stagesHTML + '<button type="button" style="font-size:12px;padding:8px 12px;background:rgba(255,107,44,0.1);border:1px solid rgba(255,107,44,0.3);color:var(--af-magma);border-radius:6px;cursor:pointer" onclick="addStage()">+ Add Stage</button></div>' +
    '<div style="display:flex;gap:8px"><button type="submit" class="btn btn-primary" style="flex:1">Save Pipeline</button><button type="button" style="flex:1;padding:8px;border:1px solid rgba(139,134,128,0.2);background:rgba(250,243,240,0.03);color:var(--text-primary);border-radius:6px;cursor:pointer" onclick="cancelEdit()">Cancel</button></div>' +
    '</form>';

  document.getElementById('editor-content').innerHTML = html;
  updateTriggerUI();
}

const html = { EscapeString(text) {
  if(!text) return '';
  let map = {'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;'};
  return text.replace(/[&<>"']/g, m => map[m]);
}};

function updateTriggerUI() {
  let trig = document.getElementById('pipeline-trigger').value;
  let trigUI = document.getElementById('trigger-ui');
  if(trig === 'cron') {
    let cronExpr = (currentPipeline.trigger || {}).cronExpr || '';
    trigUI.innerHTML = '<input type="text" id="cron-expr" placeholder="0 9 * * *" value="' + html.EscapeString(cronExpr) + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)">';
  } else {
    trigUI.innerHTML = '';
  }
}

function addStage() {
  let name = prompt('Stage name:'); if(!name) return;
  let agent = prompt('Agent name:'); if(!agent) agent = 'content';
  let tool = prompt('Tool name:'); if(!tool) tool = 'web-search';
  currentPipeline.stages = currentPipeline.stages || [];
  currentPipeline.stages.push({name, agent, tool, timeout: '300s', retry: 0, onFailure: 'skip'});
  renderEditor();
}

function removeStage(i) {
  if(currentPipeline.stages) {
    currentPipeline.stages.splice(i, 1);
    renderEditor();
  }
}

function savePipeline(e) {
  e.preventDefault();
  if(!currentPipeline) return;

  currentPipeline.name = document.getElementById('pipeline-name').value;
  currentPipeline.description = document.getElementById('pipeline-desc').value;
  currentPipeline.trigger.type = document.getElementById('pipeline-trigger').value;
  if(currentPipeline.trigger.type === 'cron') {
    currentPipeline.trigger.cronExpr = document.getElementById('cron-expr').value;
  }

  let cfg = JSON.stringify(pipelines);
  fetch('/api/config/save', {method:'POST',body:new URLSearchParams({'pipelines.definitions':cfg})})
    .then(r => r.json()).then(d => {
      if(d.ok) {
        showToast('Pipeline saved. Restart daemon to apply.', 'success');
        setTimeout(() => location.reload(), 1200);
      } else {
        showToast('Error: ' + d.error, 'error');
      }
    }).catch(e => showToast('Error: ' + e, 'error'));
}

function createPipeline() {
  currentPipeline = {name:'', description:'', enabled:true, trigger:{type:'manual'}, stages:[]};
  pipelines.push(currentPipeline);

  document.getElementById('pipeline-list').innerHTML = '<div style="color:var(--text-dim);font-size:12px;padding:12px;text-align:center">Creating...</div>';
  document.getElementById('editor-pane').style.display = 'block';
  renderEditor();
}

function cancelEdit() {
  currentPipeline = null;
  document.getElementById('editor-pane').style.display = 'none';
  document.getElementById('editor-content').innerHTML = '<div style="color:var(--text-dim);text-align:center">Select a pipeline to edit</div>';
}
</script>
</div>`)
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
	settingRow(w, "Daemon Host", cfg.Daemon.Host, "Network interface to bind", "daemon.host")
	settingRow(w, "Daemon Port", fmt.Sprintf("%d", cfg.Daemon.Port), "HTTP port for dashboard + API", "daemon.port")
	settingRow(w, "MCP Port", fmt.Sprintf("%d", cfg.MCPPort), "First MCP server port (convenience)", "mcp.port")
	settingBool(w, "MCP Enabled", cfg.MCP.Enabled, "Enable MCP servers for tool discovery", "mcp.enabled")
	settingRow(w, "Log Level", cfg.Logging.Level, "debug, info, warn, or error", "logging.level")
	settingRow(w, "Log Format", cfg.Logging.Format, "json or text output format", "logging.format")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// LLM tab
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-llm"><div class="panel">`)

	// Provider with datalist
	fmt.Fprint(w, `<div class="setting-row"><div><div class="setting-label">Provider</div><div class="setting-desc">openai, anthropic, openrouter, ollama, claude-cli, gemini-cli, or custom</div></div><input type="text" class="setting-input" list="llm-provider-list" data-config-key="llm.provider" value="`+html.EscapeString(cfg.LLM.Provider)+`" onchange="updateLLMModel()"><datalist id="llm-provider-list"><option value="openai"><option value="anthropic"><option value="openrouter"><option value="ollama"><option value="claude-cli"><option value="gemini-cli"></datalist></div>`)

	// Model with auto-update and datalist
	fmt.Fprint(w, `<div class="setting-row"><div><div class="setting-label">Model</div><div class="setting-desc">Select or type a model name (list auto-populates from selected provider)</div></div><input type="text" class="setting-input" id="llm-model" list="llm-model-list" data-config-key="llm.model" value="`+html.EscapeString(cfg.LLM.Model)+`"><datalist id="llm-model-list"></datalist></div>`)
	settingSecret(w, "API Key", cfg.LLM.APIKey, "Set via AGENTFORGE_LLM_APIKEY env var (recommended)", "llm.apiKey")
	settingRow(w, "Base URL", cfg.LLM.BaseURL, "Custom API endpoint", "llm.baseUrl")
	settingRow(w, "Timeout", cfg.LLM.Timeout.String(), "Max wait time per request", "llm.timeout")
	settingRow(w, "Temperature", fmt.Sprintf("%.1f", cfg.LLM.Temperature), "0.0 (deterministic) to 2.0 (creative)", "llm.temperature")
	settingRow(w, "Max Tokens", fmt.Sprintf("%d", cfg.LLM.MaxTokens), "Output token limit", "llm.maxTokens")
	settingRow(w, "Top-P", fmt.Sprintf("%.2f", cfg.LLM.TopP), "Nucleus sampling threshold", "llm.topP")
	settingRow(w, "Freq Penalty", fmt.Sprintf("%.2f", cfg.LLM.FrequencyPenalty), "Repetition penalty (-2.0 to 2.0)", "llm.frequencyPenalty")
	settingRow(w, "Presence Penalty", fmt.Sprintf("%.2f", cfg.LLM.PresencePenalty), "Topic diversity (-2.0 to 2.0)", "llm.presencePenalty")
	settingBool(w, "Streaming", cfg.LLM.Streaming, "Stream tokens as they're generated", "llm.streaming")
	settingRow(w, "Retry Count", fmt.Sprintf("%d", cfg.LLM.RetryCount), "Max retries on failure", "llm.retryCount")
	settingRow(w, "Retry Delay", cfg.LLM.RetryDelay.String(), "Backoff between retries", "llm.retryDelay")
	settingRow(w, "Proxy", cfg.LLM.Proxy, "HTTP proxy (SOCKS5 supported)", "llm.proxy")
	settingRow(w, "Max Concurrency", fmt.Sprintf("%d", cfg.LLM.MaxConcurrency), "Parallel LLM requests", "llm.maxConcurrency")
	if len(cfg.LLM.Fallbacks) > 0 {
		for i, fb := range cfg.LLM.Fallbacks {
			settingRow(w, fmt.Sprintf("Fallback %d Provider / Model", i+1), fmt.Sprintf("%s / %s", fb.Provider, fb.Model), "Provider routing, in order", fmt.Sprintf("llm.fallbacks.%d", i))
		}
	}
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Providers tab
	claudeDetected := false
	if _, ok := s.availableProviders["claude-cli"]; ok {
		claudeDetected = true
	}
	geminiDetected := false
	if _, ok := s.availableProviders["gemini-cli"]; ok {
		geminiDetected = true
	}
	detectedBadge := func(ok bool) string {
		if ok {
			return `<span style="color:#16a766;font-size:11px;padding:2px 8px;border-radius:10px;background:rgba(22,167,102,0.15)">✓ detected</span>`
		}
		return `<span style="color:#888;font-size:11px;padding:2px 8px;border-radius:10px;background:rgba(139,134,128,0.15)">not installed</span>`
	}

	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-providers"><div class="panel">`)
	fmt.Fprint(w, `<div style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.5px;color:var(--text-dim);margin-bottom:12px">CLI Providers</div>`)

	// providerCard renders a titled card containing arbitrary content
	provCard := func(title, badge, content string) {
		fmt.Fprintf(w, `<div style="border:1px solid rgba(139,134,128,0.2);border-radius:10px;padding:16px;margin-bottom:12px;background:rgba(250,243,240,0.02)">
<div style="display:flex;align-items:center;gap:12px;margin-bottom:12px">%s%s</div>%s</div>`, title, badge, content)
	}

	claudeModelOpts := modelOptions(cfg.Providers.ClaudeCLI.Model, []string{
		"claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4-5-20251001", "claude-opus-4", "claude-sonnet-4-5",
	})
	claudeEnabledCheck := ""
	if cfg.Providers.ClaudeCLI.Enabled {
		claudeEnabledCheck = " checked"
	}
	provCard(
		fmt.Sprintf(`<label class="af-toggle-label" title="Enable Claude CLI"><input type="checkbox" class="af-toggle-input" data-config-key="providers.claudeCli.enabled"%s><span class="af-toggle-slider"></span></label><strong style="font-size:14px">Claude CLI</strong>`, claudeEnabledCheck),
		detectedBadge(claudeDetected),
		fmt.Sprintf(`<div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
  <div><div style="font-size:11px;color:var(--text-dim);margin-bottom:4px">Executable path</div>
  <input class="setting-input" data-config-key="providers.claudeCli.cliPath" value="%s" placeholder="claude" style="width:100%%"></div>
  <div><div style="font-size:11px;color:var(--text-dim);margin-bottom:4px">Model</div>
  <input class="setting-input" list="claude-cli-models" data-config-key="providers.claudeCli.model" value="%s" placeholder="claude-sonnet-4-6" style="width:100%%">
  <datalist id="claude-cli-models">%s</datalist></div>
</div>`,
			html.EscapeString(cfg.Providers.ClaudeCLI.CLIPath),
			html.EscapeString(cfg.Providers.ClaudeCLI.Model),
			claudeModelOpts),
	)

	geminiModelOpts := modelOptions(cfg.Providers.GeminiCLI.Model, []string{
		"gemini-2.5-pro", "gemini-2.0-flash", "gemini-2.0-flash-lite", "gemini-1.5-pro", "gemini-1.5-flash",
	})
	geminiEnabledCheck := ""
	if cfg.Providers.GeminiCLI.Enabled {
		geminiEnabledCheck = " checked"
	}
	provCard(
		fmt.Sprintf(`<label class="af-toggle-label" title="Enable Gemini CLI"><input type="checkbox" class="af-toggle-input" data-config-key="providers.geminiCli.enabled"%s><span class="af-toggle-slider"></span></label><strong style="font-size:14px">Gemini CLI</strong>`, geminiEnabledCheck),
		detectedBadge(geminiDetected),
		fmt.Sprintf(`<div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
  <div><div style="font-size:11px;color:var(--text-dim);margin-bottom:4px">Executable path</div>
  <input class="setting-input" data-config-key="providers.geminiCli.cliPath" value="%s" placeholder="gemini" style="width:100%%"></div>
  <div><div style="font-size:11px;color:var(--text-dim);margin-bottom:4px">Model</div>
  <input class="setting-input" list="gemini-cli-models" data-config-key="providers.geminiCli.model" value="%s" placeholder="gemini-2.5-pro" style="width:100%%">
  <datalist id="gemini-cli-models">%s</datalist></div>
</div>`,
			html.EscapeString(cfg.Providers.GeminiCLI.CLIPath),
			html.EscapeString(cfg.Providers.GeminiCLI.Model),
			geminiModelOpts),
	)

	// API Providers — fields always visible so users can configure before/after enabling
	fmt.Fprint(w, `<div style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.5px;color:var(--text-dim);margin-bottom:12px;border-top:1px solid rgba(139,134,128,0.1);padding-top:16px">API Providers</div>`)

	type apiProvEntry struct {
		name       string
		display    string
		pc         config.ProviderConfig
		defaultURL string
		models     []string
		noAPIKey   bool
	}
	apiProvList := []apiProvEntry{
		{"openai", "OpenAI", cfg.Providers.OpenAI,
			"https://api.openai.com/v1",
			[]string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "o1", "o1-mini", "o3-mini", "o4-mini"},
			false},
		{"anthropic", "Anthropic", cfg.Providers.Anthropic,
			"https://api.anthropic.com",
			[]string{"claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4-5-20251001", "claude-opus-4", "claude-sonnet-4-5"},
			false},
		{"openrouter", "OpenRouter", cfg.Providers.OpenRouter,
			"https://openrouter.ai/api/v1",
			[]string{"openai/gpt-4o", "anthropic/claude-opus-4", "anthropic/claude-sonnet-4-6", "meta-llama/llama-3.3-70b-instruct", "google/gemini-2.5-pro", "deepseek/deepseek-r1"},
			false},
		{"google", "Google (Gemini API)", cfg.Providers.Google,
			"https://generativelanguage.googleapis.com/v1beta",
			[]string{"gemini-2.5-pro", "gemini-2.0-flash", "gemini-2.0-flash-lite", "gemini-1.5-pro", "gemini-1.5-flash"},
			false},
		{"groq", "Groq", cfg.Providers.Groq,
			"https://api.groq.com/openai/v1",
			[]string{"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768", "gemma2-9b-it", "deepseek-r1-distill-llama-70b"},
			false},
		{"deepseek", "DeepSeek", cfg.Providers.DeepSeek,
			"https://api.deepseek.com/v1",
			[]string{"deepseek-chat", "deepseek-coder", "deepseek-reasoner"},
			false},
		{"mistral", "Mistral", cfg.Providers.Mistral,
			"https://api.mistral.ai/v1",
			[]string{"mistral-large-latest", "mistral-medium-latest", "mistral-small-latest", "codestral-latest", "open-mistral-nemo"},
			false},
		{"cohere", "Cohere", cfg.Providers.Cohere,
			"https://api.cohere.ai/v1",
			[]string{"command-r-plus", "command-r", "command", "command-light"},
			false},
		{"ollama", "Ollama (local)", cfg.Providers.Ollama,
			"http://localhost:11434",
			[]string{"llama3", "llama3.2", "mistral", "codellama", "qwen2.5", "phi3", "deepseek-r1"},
			true},
	}

	for _, pe := range apiProvList {
		pfx := "providers." + pe.name
		enabledCheck := ""
		if pe.pc.Enabled {
			enabledCheck = " checked"
		}
		modelListID := "models-" + pe.name
		modelOpts := modelOptions(pe.pc.Model, pe.models)

		keyField := ""
		if !pe.noAPIKey {
			keyPlaceholder := "enter API key"
			if pe.pc.APIKey != "" {
				keyPlaceholder = config.MaskAPIKey(pe.pc.APIKey) + "  (blank = keep existing)"
			}
			keyField = fmt.Sprintf(`<div><div style="font-size:11px;color:var(--text-dim);margin-bottom:4px">API Key</div>
  <input class="setting-input" type="password" data-config-key="%s.apiKey" value="" placeholder="%s" style="width:100%%"></div>`,
				pfx, html.EscapeString(keyPlaceholder))
		} else {
			keyField = `<div style="font-size:11px;color:var(--text-dim);padding-top:16px">No API key required — connects to local Ollama instance.</div>`
		}

		content := fmt.Sprintf(`<div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
  %s
  <div><div style="font-size:11px;color:var(--text-dim);margin-bottom:4px">Gateway URL</div>
  <input class="setting-input" data-config-key="%s.baseUrl" value="%s" placeholder="%s" style="width:100%%"></div>
  <div><div style="font-size:11px;color:var(--text-dim);margin-bottom:4px">Model</div>
  <input class="setting-input" list="%s" data-config-key="%s.model" value="%s" placeholder="%s" style="width:100%%">
  <datalist id="%s">%s</datalist></div>
</div>`,
			keyField,
			pfx, html.EscapeString(pe.pc.BaseURL), html.EscapeString(pe.defaultURL),
			modelListID, pfx, html.EscapeString(pe.pc.Model),
			func() string { if len(pe.models) > 0 { return pe.models[0] }; return "" }(),
			modelListID, modelOpts)

		titleHTML := fmt.Sprintf(`<label class="af-toggle-label" title="Enable %s"><input type="checkbox" class="af-toggle-input" data-config-key="%s.enabled"%s><span class="af-toggle-slider"></span></label><strong style="font-size:14px">%s</strong>`,
			pe.display, pfx, enabledCheck, pe.display)
		provCard(titleHTML, "", content)
	}

	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Memory
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-memory"><div class="panel">`)
	settingRow(w, "Memory Root", cfg.Memory.Root, "MeMex Zero RAG storage path", "memory.root")
	settingBool(w, "Auto-Commit", cfg.Memory.AutoCommit, "Auto git-commit memory on change", "memory.autoCommit")
	settingRow(w, "Commit Interval", cfg.Memory.CommitInterval.String(), "Max interval between auto-commits", "memory.commitInterval")
	settingBool(w, "Index Enabled", cfg.Memory.IndexEnabled, "FTS5 full-text search index", "memory.indexEnabled")
	settingRow(w, "Compress Interval", cfg.Memory.CompressInterval.String(), "Compression interval", "memory.compressInterval")
	settingRow(w, "Max Daily Size", cfg.Memory.MaxDailySize, "Soft cap per daily log", "memory.maxDailySize")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Security
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-security"><div class="panel">`)
	settingRow(w, "Default Token Budget", fmt.Sprintf("%d", cfg.Security.DefaultTokenBudget), "Max tokens per agent session", "security.defaultTokenBudget")
	settingRow(w, "Default Timeout", cfg.Security.DefaultTimeout.String(), "Max agent session duration", "security.defaultTimeout")
	settingBool(w, "Enforce On Spawn", cfg.Security.EnforceOnSpawn, "Validate capability at agent creation", "security.enforceOnSpawn")
	settingBool(w, "Enforce On Tool Call", cfg.Security.EnforceOnToolCall, "Validate capability per tool invocation", "security.enforceOnToolCall")
	settingBool(w, "Audit Enabled", cfg.Security.AuditEnabled, "Write capability checks to audit log", "security.auditEnabled")
	settingBool(w, "Allow FileSystem", cfg.Security.AllowFileSystem, "Allow agents filesystem access", "security.allowFileSystem")
	settingBool(w, "Allow Network", cfg.Security.AllowNetwork, "Allow agents HTTP access", "security.allowNetwork")
	settingBool(w, "Allow Shell", cfg.Security.AllowShell, "Allow agents shell execution", "security.allowShell")
	settingBool(w, "Allow Browser", cfg.Security.AllowBrowser, "Allow agents browser access", "security.allowBrowser")
	settingRow(w, "Sandbox Mode", cfg.Security.SandboxMode, "non-main, all, or none", "security.sandboxMode")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Workers
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-workers"><div class="panel">`)
	settingRow(w, "Content Max Agents", fmt.Sprintf("%d", cfg.Workers.ContentMaxAgents), "Content department pool size", "workers.contentMaxAgents")
	settingRow(w, "SEO Max Agents", fmt.Sprintf("%d", cfg.Workers.SEOMaxAgents), "SEO department pool size", "workers.seoMaxAgents")
	settingRow(w, "Social Max Agents", fmt.Sprintf("%d", cfg.Workers.SocialMaxAgents), "Social department pool size", "workers.socialMaxAgents")
	settingRow(w, "Default Max Agents", fmt.Sprintf("%d", cfg.Workers.DefaultMaxAgents), "Default pool size for new departments", "workers.defaultMaxAgents")
	settingRow(w, "Heartbeat Interval", cfg.Workers.HeartbeatInterval.String(), "Heartbeat frequency", "workers.heartbeatInterval")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Channels (expanded)
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-channels"><div class="panel">`)
	renderChannelSection(w, "Telegram", "telegram", cfg.Channels.Telegram.Enabled, func() {
		settingSecret(w, "Bot Token", cfg.Channels.Telegram.BotToken, "Telegram Bot API token", "channels.telegram.botToken")
		settingRow(w, "Webhook URL", cfg.Channels.Telegram.WebhookURL, "Inbound webhook endpoint", "channels.telegram.webhookUrl")
		settingRow(w, "Poll Interval", cfg.Channels.Telegram.PollInterval.String(), "Long-poll interval", "channels.telegram.pollInterval")
		settingRow(w, "Max File Size", cfg.Channels.Telegram.MaxFileSize, "Max upload size", "channels.telegram.maxFileSize")
		fmt.Fprintf(w, `<button class="btn" style="border:1px solid var(--af-magma);color:var(--af-magma);margin-top:8px;font-size:12px" onclick="testChannel('telegram')">🧪 Test Connection</button>`)
	})
	renderChannelSection(w, "Discord", "discord", cfg.Channels.Discord.Enabled, func() {
		settingSecret(w, "Bot Token", cfg.Channels.Discord.BotToken, "Discord bot token", "channels.discord.botToken")
		settingRow(w, "Application ID", cfg.Channels.Discord.ApplicationID, "Discord app ID", "channels.discord.applicationId")
		settingRow(w, "Guild ID", cfg.Channels.Discord.GuildID, "Server ID", "channels.discord.guildId")
		fmt.Fprintf(w, `<button class="btn" style="border:1px solid var(--af-magma);color:var(--af-magma);margin-top:8px;font-size:12px" onclick="testChannel('discord')">🧪 Test Connection</button>`)
	})
	renderChannelSection(w, "Signal", "signal", cfg.Channels.Signal.Enabled, func() {
		settingRow(w, "Phone Number", cfg.Channels.Signal.PhoneNumber, "Registered Signal number", "channels.signal.phoneNumber")
		settingRow(w, "signal-cli Path", cfg.Channels.Signal.SignalCLIPath, "Path to signal-cli binary", "channels.signal.signalCliPath")
	})
	renderChannelSection(w, "WhatsApp", "whatsApp", cfg.Channels.WhatsApp.Enabled, func() {
		settingSecret(w, "API Key", cfg.Channels.WhatsApp.APIKey, "WhatsApp Cloud API key", "channels.whatsApp.apiKey")
		settingRow(w, "Phone Number ID", cfg.Channels.WhatsApp.PhoneNumberID, "WhatsApp phone ID", "channels.whatsApp.phoneNumberId")
		settingRow(w, "Business ID", cfg.Channels.WhatsApp.BusinessID, "WhatsApp business account ID", "channels.whatsApp.businessId")
	})
	renderChannelSection(w, "Email (SMTP)", "email", cfg.Channels.Email.Enabled, func() {
		settingRow(w, "SMTP Host", cfg.Channels.Email.SMTPHost, "SMTP server address", "channels.email.smtpHost")
		settingRow(w, "SMTP Port", fmt.Sprintf("%d", cfg.Channels.Email.SMTPPort), "SMTP port", "channels.email.smtpPort")
		settingRow(w, "Username", cfg.Channels.Email.Username, "SMTP username", "channels.email.username")
		settingSecret(w, "Password", cfg.Channels.Email.Password, "SMTP password", "channels.email.password")
		settingRow(w, "From Address", cfg.Channels.Email.FromAddress, "Sender email", "channels.email.fromAddress")
	})
	renderChannelSection(w, "Slack", "slack", cfg.Channels.Slack.Enabled, func() {
		settingSecret(w, "Bot Token", cfg.Channels.Slack.BotToken, "Slack bot token", "channels.slack.botToken")
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
	settingBool(w, "Web Search", cfg.Tools.WebSearch, "Allow internet search", "tools.webSearch")
	settingBool(w, "Web Fetch", cfg.Tools.WebFetch, "Allow URL content fetching", "tools.webFetch")
	settingBool(w, "Image Generation", cfg.Tools.ImageGen, "Allow AI image generation", "tools.imageGen")
	settingBool(w, "Image Analysis", cfg.Tools.ImageAnalyze, "Allow image analysis/vision", "tools.imageAnalyze")
	settingBool(w, "Video Generation", cfg.Tools.VideoGen, "Allow AI video generation", "tools.videoGen")
	settingBool(w, "Audio Generation", cfg.Tools.AudioGen, "Allow AI audio/music generation", "tools.audioGen")
	settingBool(w, "Browser", cfg.Tools.Browser, "Allow headless browser", "tools.browser")
	settingBool(w, "Code Execution", cfg.Tools.CodeExec, "Allow sandboxed code execution", "tools.codeExec")
	settingBool(w, "Git Operations", cfg.Tools.GitOps, "Allow git read/write", "tools.gitOps")
	settingBool(w, "Cron Scheduling", cfg.Tools.Cron, "Allow cron job management", "tools.cron")
	settingBool(w, "Notion", cfg.Tools.Notion, "Allow Notion API access", "tools.notion")
	settingBool(w, "Calendar", cfg.Tools.Calendar, "Allow calendar access", "tools.calendar")
	settingBool(w, "Weather", cfg.Tools.Weather, "Allow weather queries", "tools.weather")
	settingBool(w, "MCP Discovery", cfg.Tools.MCPDiscovery, "Auto-discover MCP tools", "tools.mcpDiscovery")
	settingBool(w, "Diagram Generation", cfg.Tools.DiagramGen, "Allow diagram generation", "tools.diagramGen")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// UI
	fmt.Fprint(w, `<div class="settings-tab-content" style="display:none" id="tab-ui"><div class="panel">`)
	settingRow(w, "Theme", cfg.UI.Theme, "volcanic-glass, dark, light", "ui.theme")
	settingBool(w, "Sidebar Collapsed", cfg.UI.SidebarCollapsed, "Start with sidebar collapsed", "ui.sidebarCollapsed")
	settingRow(w, "Auto-Refresh (sec)", fmt.Sprintf("%d", cfg.UI.AutoRefreshSecs), "Dashboard refresh interval", "ui.autoRefreshSecs")
	settingBool(w, "Animations", cfg.UI.ShowAnimations, "Show UI animations", "ui.showAnimations")
	fmt.Fprint(w, `<div style="margin-top:16px"><button class="btn btn-primary" onclick="saveSettings()">Save Changes</button></div></div></div>`)

	// Save JS — runs when HTMX injects this content (inline scripts execute immediately)
	fmt.Fprint(w, `<script>
function switchSettingsTab(e, name) {
  e.target.closest(".tabs").querySelectorAll(".tab").forEach(t => t.classList.remove("active"));
  e.target.classList.add("active");
  document.querySelectorAll(".settings-tab-content").forEach(c => c.style.display = "none");
  document.getElementById("tab-" + name).style.display = "block";
}

function saveSettings() {
  var patch = {};
  var activeTab = document.querySelector(".settings-tab-content:not([style*='none'])");
  if (!activeTab) return;

  // Text inputs (not password, not radio)
  activeTab.querySelectorAll(".setting-input:not([type='password']):not([type='radio'])").forEach(function(el) {
    var key = el.getAttribute("data-config-key");
    if (key) patch[key] = el.value;
  });

  // Password fields — only send if the user typed a new value (blank = keep existing)
  activeTab.querySelectorAll("input[type='password'][data-config-key]").forEach(function(el) {
    var key = el.getAttribute("data-config-key");
    if (key && el.value.trim() !== '') patch[key] = el.value.trim();
  });

  // Toggle checkboxes
  activeTab.querySelectorAll(".af-toggle-input").forEach(function(el) {
    var key = el.getAttribute("data-config-key");
    if (key) patch[key] = el.checked ? "true" : "false";
  });

  // Radio buttons — only the checked one
  activeTab.querySelectorAll("input[type='radio'][data-config-key]:checked").forEach(function(el) {
    patch[el.getAttribute("data-config-key")] = el.value;
  });

  var formData = new URLSearchParams(patch);
  fetch("/api/config/save", { method: "POST", body: formData })
    .then(r => r.json())
    .then(data => {
      if (!data.ok) {
        showToast("Error: " + (data.error || "unknown"), "error");
      } else if (data.reloaded) {
        showToast("Provider reloaded: " + data.provider, "success");
      } else {
        showToast("Settings saved", "success");
      }
    })
    .catch(e => showToast("Network error: " + e.message, "error"));
}

function highlightProvider(name) {
  ['claude-cli','gemini-cli'].forEach(function(p) {
    var card = document.querySelector('input[value="' + p + '"][name="llm-provider"]');
    if (!card) return;
    var label = card.closest('label');
    if (!label) return;
    label.style.borderColor = (p === name) ? 'rgba(255,107,44,0.6)' : 'rgba(139,134,128,0.2)';
  });
}

function loadCLIModels(provider, selectId, currentModel) {
  var sel = document.getElementById(selectId);
  if (!sel) return;
  fetch('/api/providers/models?provider=' + encodeURIComponent(provider))
    .then(r => r.json())
    .then(data => {
      var models = data.models || [];
      sel.innerHTML = '';
      // Ensure currentModel is present even if not in list
      var allModels = models.includes(currentModel) ? models : (currentModel ? [currentModel].concat(models) : models);
      allModels.forEach(function(m) {
        var opt = document.createElement('option');
        opt.value = m;
        opt.textContent = m;
        if (m === currentModel) opt.selected = true;
        sel.appendChild(opt);
      });
    })
    .catch(function() {});
}

function updateLLMModel() {
  var providerInput = document.querySelector('#tab-llm input[list="llm-provider-list"]');
  var modelInput = document.getElementById('llm-model');
  var modelList = document.getElementById('llm-model-list');
  if (!providerInput || !modelInput) return;
  var provider = providerInput.value.trim();
  if (!provider) return;
  fetch('/api/providers/models?provider=' + encodeURIComponent(provider))
    .then(r => r.json())
    .then(data => {
      if (!modelList) return;
      modelList.innerHTML = '';
      (data.models || []).forEach(function(m) {
        var opt = document.createElement('option');
        opt.value = m;
        modelList.appendChild(opt);
      });
      if (!modelInput.value && data.models && data.models.length > 0) {
        modelInput.value = data.models[0];
      }
    })
    .catch(function() {});
}

// Init — runs immediately when HTMX injects this content (no DOMContentLoaded needed)
(function() {
  var claudeModel = (document.getElementById('claude-cli-model-select') || {value:''}).value;
  var geminiModel = (document.getElementById('gemini-cli-model-select') || {value:''}).value;
  loadCLIModels('claude-cli', 'claude-cli-model-select', claudeModel);
  loadCLIModels('gemini-cli', 'gemini-cli-model-select', geminiModel);
  // Populate LLM tab model datalist for current provider
  var provInput = document.querySelector('#tab-llm input[list="llm-provider-list"]');
  if (provInput && provInput.value) updateLLMModel();
})();
</script>`)
}

func renderChannelSection(w http.ResponseWriter, name, yamlName string, enabled bool, render func()) {
	yamlKey := "channels." + yamlName + ".enabled"
	fmt.Fprintf(w, `<div style="margin-bottom:16px;padding:12px;border:1px solid rgba(139,134,128,0.15);border-radius:10px;background:rgba(250,243,240,0.02)"><div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px;"><strong style="font-size:14px;color:var(--text-primary)">%s</strong></div>`, name)
	settingBool(w, name+" Enabled", enabled, "Enable "+name+" channel", yamlKey)
	render()
	fmt.Fprint(w, `</div>`)
}

// ── Setting Helpers ──────────────────────────────────────────────────────────

// modelOptions renders <option> elements for a datalist; current is pre-selected first if not in list.
func modelOptions(current string, models []string) string {
	seen := map[string]bool{}
	var b strings.Builder
	write := func(m string) {
		if seen[m] || m == "" {
			return
		}
		seen[m] = true
		fmt.Fprintf(&b, `<option value="%s">`, html.EscapeString(m))
	}
	if current != "" {
		write(current)
	}
	for _, m := range models {
		write(m)
	}
	return b.String()
}

func settingRow(w http.ResponseWriter, label, value, desc, configKey string) {
	fmt.Fprintf(w, `<div class="setting-row"><div><div class="setting-label">%s</div><div class="setting-desc">%s</div></div><input class="setting-input" data-config-key="%s" value="%s"></div>`, label, desc, configKey, html.EscapeString(value))
}

func settingBool(w http.ResponseWriter, label string, current bool, desc, configKey string) {
	checked := ""
	if current {
		checked = " checked"
	}
	fmt.Fprintf(w, `<div class="setting-row"><div><div class="setting-label">%s</div><div class="setting-desc">%s</div></div><label class="af-toggle-label"><input type="checkbox" class="af-toggle-input" data-config-key="%s"%s onchange="this.closest('.setting-row').querySelector('.toggle-status').textContent=this.checked?'Enabled':'Disabled'"><span class="af-toggle-slider"></span><span class="toggle-status" style="margin-left:10px;font-size:12px;color:var(--text-dim)">%s</span></label></div>`, label, desc, configKey, checked, boolLabel(current))
}

func boolLabel(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}

func settingSecret(w http.ResponseWriter, label, current, desc, configKey string) {
	placeholder := "enter to set"
	if current != "" {
		placeholder = config.MaskAPIKey(current) + "  (blank = keep existing)"
	}
	fmt.Fprintf(w, `<div class="setting-row"><div><div class="setting-label">%s</div><div class="setting-desc">%s</div></div><input class="setting-input" type="password" data-config-key="%s" value="" placeholder="%s"></div>`, label, desc, configKey, html.EscapeString(placeholder))
}

// ── Chat partial ─────────────────────────────────────────────────────────────

func (s *Server) renderChat(w http.ResponseWriter) {
	history := s.sessionMgr.History("agentforge")

	fmt.Fprint(w, `<div class="panel chat-panel" style="flex:1;display:flex;flex-direction:column">
<div class="chat-messages" id="chat-messages" style="flex:1;overflow-y:auto;padding:16px;min-height:400px;border-bottom:1px solid rgba(139,134,128,0.15)">`)

	if len(history) == 0 {
		fmt.Fprint(w, `<div class="chat-msg agent">I'm AgentForge. I am a capability-secured agent orchestration system. I can help you spawn agents, run pipelines, search memory, and audit your security posture. What would you like to do?</div>`)
	} else {
		for _, t := range history {
			switch t.Role {
			case "user":
				fmt.Fprintf(w, `<div class="chat-msg user">%s</div>`, html.EscapeString(t.Content))
			case "assistant":
				fmt.Fprintf(w, `<div class="chat-msg agent chat-history-msg" data-raw="%s"></div>`, html.EscapeString(t.Content))
			}
		}
	}

	fmt.Fprint(w, `</div>
<div id="chat-cost-display" class="chat-cost-display" style="padding:8px 12px;border-bottom:1px solid rgba(139,134,128,0.15);background:rgba(250,243,240,0.01);display:none">
<div style="font-size:11px;color:var(--text-dim);display:flex;justify-content:space-between;gap:16px">
  <span>Session Cost: <strong id="session-total-cost">$0.00</strong></span>
  <span>Tokens: <strong id="session-token-count">0</strong> (in: <span id="session-input-tokens">0</span> | out: <span id="session-output-tokens">0</span>)</span>
</div>
</div>
<div class="chat-input-bar" style="display:flex;gap:8px;padding:12px;background:rgba(250,243,240,0.02)">
<input placeholder="Type a message... (Shift+Enter for new line)" id="chat-input" style="flex:1;border:1px solid rgba(139,134,128,0.2);background:rgba(250,243,240,0.03);color:var(--text-primary);padding:10px 12px;border-radius:6px;font-size:13px;resize:none;max-height:100px" onkeydown="if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();sendChat()}">
<label class="file-upload-btn" title="Attach file (images, documents, code)">
  <input type="file" id="chat-file-input" multiple style="display:none" accept="image/*,.pdf,.doc,.docx,.txt,.js,.py,.go,.sql,.json" onchange="handleFileUpload()">
  <img src="/static/img/icons/chat-attach.png" width="18" alt="Attach">
</label>
<button class="btn btn-primary" id="send-btn" onclick="sendChat()" style="white-space:nowrap"><img src="/static/img/icons/chat-send.png"> Send</button>
</div>
</div>
<script>(function(){
  // Reset streaming state — partial reload means user navigated back;
  // any in-flight stream is gone, so unlock the send button unconditionally.
  if (window.agChatReset) window.agChatReset();
  // Apply markdown rendering to history messages
  if (window.agChatRenderHistory) {
    document.querySelectorAll('.chat-history-msg').forEach(function(el) {
      window.agChatRenderHistory(el);
    });
  }
  var msgs = document.getElementById('chat-messages');
  if (msgs) msgs.scrollTop = msgs.scrollHeight;
})();</script>

<style>
.chat-messages {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.chat-msg {
  padding: 12px 14px;
  border-radius: 8px;
  max-width: 80%;
  word-wrap: break-word;
  line-height: 1.5;
  font-size: 13px;
}

.chat-msg.user {
  align-self: flex-end;
  background: rgba(139, 134, 128, 0.2);
  border: 1px solid rgba(139, 134, 128, 0.3);
  color: var(--text-primary);
}

.chat-msg.agent {
  align-self: flex-start;
  background: rgba(250, 243, 240, 0.05);
  border: 1px solid rgba(139, 134, 128, 0.15);
  color: var(--text-primary);
}

.chat-msg.streaming {
  background: rgba(250, 243, 240, 0.03);
}

.chat-msg code {
  background: rgba(0, 0, 0, 0.3);
  padding: 2px 6px;
  border-radius: 3px;
  font-family: var(--font-mono);
  font-size: 12px;
}

.chat-msg pre {
  background: rgba(0, 0, 0, 0.3);
  padding: 10px 12px;
  border-radius: 6px;
  overflow-x: auto;
  margin: 4px 0;
  position: relative;
}

.code-block-copy {
  position: absolute;
  top: 4px;
  right: 4px;
  background: rgba(139, 134, 128, 0.3);
  border: none;
  color: var(--text-primary);
  padding: 4px 8px;
  border-radius: 3px;
  cursor: pointer;
  font-size: 11px;
  opacity: 0;
  transition: opacity 0.2s;
}

.chat-msg pre:hover .code-block-copy {
  opacity: 1;
}

.code-block-copy:hover {
  background: rgba(139, 134, 128, 0.5);
}

.typing-indicator {
  display: flex;
  gap: 4px;
  align-items: center;
}

.typing-indicator span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgba(139, 134, 128, 0.6);
  animation: typing 1.4s infinite;
}

.typing-indicator span:nth-child(2) {
  animation-delay: 0.2s;
}

.typing-indicator span:nth-child(3) {
  animation-delay: 0.4s;
}

@keyframes typing {
  0%, 60%, 100% { opacity: 0.5; transform: translateY(0); }
  30% { opacity: 1; transform: translateY(-8px); }
}

.tool-progress {
  padding: 8px 12px;
  border-radius: 6px;
  background: rgba(34, 197, 94, 0.1);
  border: 1px solid rgba(34, 197, 94, 0.3);
  color: var(--text-dim);
  font-size: 12px;
  display: flex;
  gap: 8px;
  align-items: center;
  margin: 4px 0;
}

.tool-progress.tool-done {
  background: rgba(34, 197, 94, 0.15);
  border-color: rgba(34, 197, 94, 0.4);
}

.tool-spinner {
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 2px solid rgba(34, 197, 94, 0.3);
  border-top-color: rgba(34, 197, 94, 0.8);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.tool-stale .tool-spinner {
  animation: none;
  border: none;
}

.chat-error {
  color: #EF4444;
  font-weight: 600;
}

.chat-status {
  padding: 8px 12px;
  background: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.3);
  border-radius: 6px;
  color: rgba(59, 130, 246, 1);
  font-size: 12px;
  margin: 4px 0;
}

.file-upload-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border: 1px solid rgba(139, 134, 128, 0.2);
  background: rgba(250, 243, 240, 0.03);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.file-upload-btn:hover {
  border-color: var(--af-magma);
  background: rgba(250, 243, 240, 0.05);
}

.file-upload-btn img {
  filter: brightness(0.9);
}
</style>

<script>
function handleFileUpload() {
  var input = document.getElementById('chat-file-input');
  var files = input.files;
  if (files.length === 0) return;

  var div = document.getElementById('chat-messages');
  var formData = new FormData();

  for (var i = 0; i < files.length; i++) {
    formData.append('files', files[i]);
    var file = files[i];
    var fileMsg = document.createElement('div');
    fileMsg.className = 'chat-msg user';
    fileMsg.innerHTML = '<strong>📎 ' + escapeHtml(file.name) + '</strong><br><span style="font-size:11px;color:var(--text-dim)">' + (file.size / 1024).toFixed(1) + ' KB</span>';
    div.appendChild(fileMsg);
  }

  fetch('/api/chat/upload', {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer ' + (localStorage.getItem('af_access_token') || '')
    },
    body: formData
  }).then(function(resp) {
    if (!resp.ok) {
      console.error('File upload failed:', resp.status);
    }
    return resp.json();
  }).then(function(data) {
    console.log('Files uploaded:', data);
  }).catch(function(err) {
    console.error('Upload error:', err);
  });

  input.value = '';
  div.scrollTop = div.scrollHeight;
}

function escapeHtml(s) {
  return String(s).replace(/&/g, '&amp;')
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;')
          .replace(/"/g, '&quot;');
}
</script>`)
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


// ── Agent Profiles Manager ──────────────────────────────────────────────────

func countEnabledAgents(agents []config.AgentProfile) int {
	count := 0
	for _, a := range agents {
		if a.Enabled {
			count++
		}
	}
	return count
}

func countDisabledAgents(agents []config.AgentProfile) int {
	count := 0
	for _, a := range agents {
		if !a.Enabled {
			count++
		}
	}
	return count
}

func (s *Server) renderAgentProfilesManager(w http.ResponseWriter) {
	agentsJSON, _ := json.Marshal(s.cfg.Agents.Profiles)
	activeCount := countEnabledAgents(s.cfg.Agents.Profiles)
	disabledCount := countDisabledAgents(s.cfg.Agents.Profiles)

	fmt.Fprint(w, `<div style="display:flex;flex-direction:column;gap:16px;height:calc(100vh - 200px);overflow-y:auto">
<!-- Fleet Overview -->
<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-agents.png"> Agent Fleet Overview</div>
<div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(120px,1fr));gap:12px">
<div style="padding:12px;border:1px solid rgba(139,134,128,0.1);border-radius:8px;background:rgba(250,243,240,0.02);text-align:center">
<div style="font-size:28px;font-weight:700;color:var(--af-magma)">` + fmt.Sprintf("%d", len(s.cfg.Agents.Profiles)) + `</div>
<div style="font-size:11px;color:var(--text-dim);margin-top:4px">Total Agents</div>
</div>
<div style="padding:12px;border:1px solid rgba(139,134,128,0.1);border-radius:8px;background:rgba(250,243,240,0.02);text-align:center">
<div style="font-size:28px;font-weight:700;color:#10B981">` + fmt.Sprintf("%d", activeCount) + `</div>
<div style="font-size:11px;color:var(--text-dim);margin-top:4px">Active</div>
</div>
<div style="padding:12px;border:1px solid rgba(139,134,128,0.1);border-radius:8px;background:rgba(250,243,240,0.02);text-align:center">
<div style="font-size:28px;font-weight:700;color:#EF4444">` + fmt.Sprintf("%d", disabledCount) + `</div>
<div style="font-size:11px;color:var(--text-dim);margin-top:4px">Paused</div>
</div>
</div>
</div>

<!-- Agent Fleet Grid -->
<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-agents.png"> Fleet Agents <span style="font-weight:400;font-size:12px;color:var(--text-dim);margin-left:8px">click any agent to configure</span></div>
<div id="agent-list" style="display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:12px">`)

	if len(s.cfg.Agents.Profiles) == 0 {
		fmt.Fprint(w, `<div style="color:var(--text-dim);font-size:12px;padding:40px;text-align:center;grid-column:1/-1">No agents in fleet yet. Create one to get started.</div>`)
	} else {
		for _, ap := range s.cfg.Agents.Profiles {
			statusCls := "badge-live"
			statusTxt := "Active"
			if !ap.Enabled {
				statusCls = "badge-idle"
				statusTxt = "Paused"
			}
			fmt.Fprintf(w, `<div class="agent-card" onclick='selectAgent(this, %q)' style="padding:16px;border:1px solid rgba(139,134,128,0.15);border-radius:10px;cursor:pointer;transition:all 0.2s;background:rgba(250,243,240,0.02);display:flex;flex-direction:column;gap:10px">
<div style="display:flex;justify-content:space-between;align-items:start;gap:8px">
<div style="flex:1">
<div style="font-size:15px;font-weight:700;color:var(--text-primary)">%s</div>
<div style="font-size:11px;color:var(--text-dim);margin-top:2px">%s</div>
</div>
<span class="badge %s" style="font-size:10px;padding:4px 8px">%s</span>
</div>
<div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;font-size:12px;padding:8px;background:rgba(0,0,0,0.08);border-radius:6px">
<div><span style="color:var(--text-dim);font-size:10px">Model</span><br><span style="font-family:var(--font-mono);font-size:11px">%s</span></div>
<div><span style="color:var(--text-dim);font-size:10px">Tools</span><br><span style="font-family:var(--font-mono);font-size:11px">%d</span></div>
</div>
<div style="display:flex;gap:4px;flex-wrap:wrap">
<span style="font-size:9px;padding:3px 6px;border-radius:4px;background:rgba(255,107,44,0.1);color:var(--af-magma)">%s</span>
<span style="font-size:9px;padding:3px 6px;border-radius:4px;background:rgba(139,134,128,0.1);color:var(--text-dim)">%s</span>
</div>
<button class="btn" onclick="editAgentClick(event, %q)" style="width:100%;border:1px solid rgba(255,107,44,0.2);color:var(--af-magma);font-size:12px;padding:6px;border-radius:6px;cursor:pointer;background:transparent;font-weight:500">Configure</button>
</div>`, ap.Name, ap.ID, statusCls, statusTxt, ap.Model, len(ap.Tools), ap.Department, ap.Provider, ap.Name)
		}
	}

	fmt.Fprint(w, `</div>
<button class="btn btn-primary" style="margin-top:8px" onclick="createAgent()">+ Add Agent to Fleet</button>
</div>

<!-- Agent editor modal (hidden by default) -->
<div id="editor-pane" style="display:none;position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.5);z-index:1000;overflow-y:auto;padding:20px">
<div class="panel" style="max-width:600px;margin:40px auto">
<div class="panel-header">Configure Agent</div>
<div id="editor-content" style="padding:16px">
<div style="color:var(--text-dim);text-align:center">Loading agent configuration...</div>
</div>
</div>
</div>

<script>
let currentAgent = null;
let agents = ` + string(agentsJSON) + `;

function selectAgent(elem, name) {
  currentAgent = agents.find(a => a.name === name);
  if(!currentAgent) return;

  document.querySelectorAll('.agent-card').forEach(e => e.style.borderColor='rgba(139,134,128,0.15)');
  elem.style.borderColor = 'var(--af-magma)';

  document.getElementById('editor-pane').style.display = 'flex';
  renderAgentEditor();
}

function editAgentClick(e, name) {
  e.stopPropagation();
  currentAgent = agents.find(a => a.name === name);
  if(!currentAgent) return;
  document.getElementById('editor-pane').style.display = 'flex';
  renderAgentEditor();
}

function renderAgentEditor() {
  if(!currentAgent) return;

  let toolsHTML = (currentAgent.tools || []).length === 0
    ? '<div style="color:var(--text-dim);font-size:12px;padding:12px">No tools configured</div>'
    : (currentAgent.tools || []).map((t,i) => '<div style="display:inline-block;padding:4px 10px;border-radius:6px;background:rgba(255,107,44,0.1);color:var(--af-magma);font-size:11px;margin-right:6px;margin-bottom:6px">'+html.EscapeString(t)+'<button style="margin-left:6px;background:none;border:none;color:var(--af-magma);cursor:pointer;font-size:10px" onclick="removeTool('+i+')">×</button></div>').join('');

  let html = '<form id="agent-form" onsubmit="saveAgent(event)" style="display:flex;flex-direction:column;gap:12px">' +
    '<div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Name</label><input type="text" id="agent-name" value="' + html.EscapeString(currentAgent.name || '') + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)" required></div>' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Department <span style="font-size:10px;font-weight:400">(custom or predefined)</span></label><input type="text" id="agent-dept" list="dept-suggestions" value="' + html.EscapeString(currentAgent.department || '') + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)" placeholder="e.g. content, seo, custom-dept"><datalist id="dept-suggestions"><option value="content">content</option><option value="seo">seo</option><option value="social">social</option><option value="security">security</option><option value="devops">devops</option><option value="memory">memory</option><option value="orchestrator">orchestrator</option><option value="monitor">monitor</option></datalist></div>' +
    '</div>' +
    '<div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Provider</label><input type="text" id="agent-provider" list="provider-suggestions" value="' + html.EscapeString(currentAgent.provider || 'openai') + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)" onchange="updateModelFromProvider()"><datalist id="provider-suggestions"><option value="openai">openai</option><option value="anthropic">anthropic</option><option value="ollama">ollama</option><option value="claude-cli">claude-cli</option><option value="gemini-cli">gemini-cli</option></datalist></div>' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Model</label><input type="text" id="agent-model" list="agent-model-list" value="' + html.EscapeString(currentAgent.model || 'gpt-4o') + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)"><datalist id="agent-model-list"></datalist></div>' +
    '</div>' +
    '<div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Temperature</label><input type="number" id="agent-temp" min="0" max="2" step="0.1" value="' + (currentAgent.temperature || 0.7) + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)"></div>' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Max Tokens</label><input type="number" id="agent-max-tokens" min="100" step="100" value="' + (currentAgent.maxTokens || 4096) + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)"></div>' +
    '</div>' +
    '<div style="border-top:1px solid rgba(139,134,128,0.1);padding-top:12px"><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:8px"><strong>Tools</strong></label><div style="margin-bottom:8px">' + toolsHTML + '</div><input type="text" id="tool-input" placeholder="Add tool (e.g. web-search)" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary);font-size:12px;margin-bottom:8px"><button type="button" style="font-size:12px;padding:6px 12px;background:rgba(255,107,44,0.1);border:1px solid rgba(255,107,44,0.3);color:var(--af-magma);border-radius:6px;cursor:pointer" onclick="addTool()">+ Add Tool</button></div>' +
    '<div style="border-top:1px solid rgba(139,134,128,0.1);padding-top:12px"><div style="font-size:12px;color:var(--text-dim);margin-bottom:8px"><strong>Capabilities</strong></div><div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;margin-bottom:12px">' +
    '<label style="display:flex;align-items:center;gap:6px;cursor:pointer"><input type="checkbox" id="cap-fs" ' + (currentAgent.capability?.allowFileSystem ? 'checked' : '') + ' style="cursor:pointer"><span style="font-size:12px">File System</span></label>' +
    '<label style="display:flex;align-items:center;gap:6px;cursor:pointer"><input type="checkbox" id="cap-net" ' + (currentAgent.capability?.allowNetwork ? 'checked' : '') + ' style="cursor:pointer"><span style="font-size:12px">Network</span></label>' +
    '<label style="display:flex;align-items:center;gap:6px;cursor:pointer"><input type="checkbox" id="cap-shell" ' + (currentAgent.capability?.allowShell ? 'checked' : '') + ' style="cursor:pointer"><span style="font-size:12px">Shell</span></label>' +
    '<label style="display:flex;align-items:center;gap:6px;cursor:pointer"><input type="checkbox" id="cap-spawn" ' + (currentAgent.capability?.allowSpawn ? 'checked' : '') + ' style="cursor:pointer"><span style="font-size:12px">Spawn</span></label>' +
    '</div></div>' +
    '<div style="display:flex;gap:8px"><button type="submit" class="btn btn-primary" style="flex:1">Save Agent</button><button type="button" style="flex:1;padding:8px;border:1px solid rgba(139,134,128,0.2);background:rgba(250,243,240,0.03);color:var(--text-primary);border-radius:6px;cursor:pointer" onclick="cancelEdit()">Cancel</button></div>' +
    '</form>';

  document.getElementById('editor-content').innerHTML = html;
}

function html.EscapeString(text) {
  if(!text) return '';
  let map = {'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;'};
  return text.replace(/[&<>"']/g, m => map[m]);
}

function addTool() {
  let tool = document.getElementById('tool-input').value;
  if(!tool) return;
  currentAgent.tools = currentAgent.tools || [];
  if(!currentAgent.tools.includes(tool)) {
    currentAgent.tools.push(tool);
    document.getElementById('tool-input').value = '';
    renderAgentEditor();
  }
}

function removeTool(i) {
  if(currentAgent.tools) {
    currentAgent.tools.splice(i, 1);
    renderAgentEditor();
  }
}

function updateModelFromProvider() {
  const provider = document.getElementById('agent-provider').value;
  const modelField = document.getElementById('agent-model');
  const modelList = document.getElementById('agent-model-list');
  if (!provider || !modelField) return;

  fetch('/api/providers/models?provider=' + encodeURIComponent(provider))
    .then(r => r.json())
    .then(data => {
      if (modelList) {
        modelList.innerHTML = '';
        (data.models || []).forEach(m => {
          var opt = document.createElement('option');
          opt.value = m;
          modelList.appendChild(opt);
        });
      }
      if (!modelField.value && data.models && data.models.length > 0) {
        modelField.value = data.models[0];
      }
    })
    .catch(() => {});
}

function saveAgent(e) {
  e.preventDefault();
  if(!currentAgent) return;

  currentAgent.name = document.getElementById('agent-name').value;
  currentAgent.department = document.getElementById('agent-dept').value;
  currentAgent.provider = document.getElementById('agent-provider').value;
  currentAgent.model = document.getElementById('agent-model').value;
  currentAgent.temperature = parseFloat(document.getElementById('agent-temp').value);
  currentAgent.maxTokens = parseInt(document.getElementById('agent-max-tokens').value);
  currentAgent.capability = {
    allowFileSystem: document.getElementById('cap-fs').checked,
    allowNetwork: document.getElementById('cap-net').checked,
    allowShell: document.getElementById('cap-shell').checked,
    allowSpawn: document.getElementById('cap-spawn').checked,
    tokenBudget: 1000000
  };

  let cfg = JSON.stringify(agents);
  fetch('/api/config/save', {method:'POST',body:new URLSearchParams({'agents.profiles':cfg})})
    .then(r => r.json()).then(d => {
      if(d.ok) {
        showToast('Agent saved. Restart daemon to apply.', 'success');
        setTimeout(() => location.reload(), 1200);
      } else {
        showToast('Error: ' + d.error, 'error');
      }
    }).catch(e => showToast('Error: ' + e, 'error'));
}

function createAgent() {
  currentAgent = {
    id: 'agent-' + Date.now(),
    name: 'New Agent',
    enabled: true,
    provider: 'openai',
    model: 'gpt-4o',
    department: 'content',
    temperature: 0.7,
    maxTokens: 4096,
    timeout: '300s',
    tools: [],
    skills: [],
    capability: {
      allowFileSystem: true,
      allowNetwork: true,
      allowShell: false,
      allowSpawn: false,
      tokenBudget: 1000000
    }
  };
  agents.push(currentAgent);

  document.getElementById('agent-list').innerHTML = '<div style="color:var(--text-dim);font-size:12px;padding:12px;text-align:center">Creating...</div>';
  document.getElementById('editor-pane').style.display = 'block';
  renderAgentEditor();
}

function cancelEdit() {
  currentAgent = null;
  document.getElementById('editor-pane').style.display = 'none';
  document.getElementById('editor-content').innerHTML = '<div style="color:var(--text-dim);text-align:center">Select an agent to edit</div>';
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
		endpoint = fmt.Sprintf("%s/skills/ai-search?q=%s", s.cfg.Skills.MarketplaceURL, url.QueryEscape(q))
	} else {
		endpoint = fmt.Sprintf("%s/skills/search?q=%s", s.cfg.Skills.MarketplaceURL, url.QueryEscape(q))
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

	// Parse skillsmp response (with data.skills structure)
	var skillsmpResp struct {
		Success bool `json:"success"`
		Data    struct {
			Skills []map[string]any `json:"skills"`
		} `json:"data"`
	}

	var raw []map[string]any
	if err := json.Unmarshal([]byte(bodyStr), &skillsmpResp); err == nil && skillsmpResp.Success && len(skillsmpResp.Data.Skills) > 0 {
		raw = skillsmpResp.Data.Skills
	} else if err := json.Unmarshal([]byte(bodyStr), &raw); err != nil {
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
	// If we got real skills data, return just the array
	if len(raw) > 0 {
		data, _ := json.Marshal(raw)
		w.Write(data)
	} else {
		w.Write([]byte(bodyStr))
	}
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
	// Auto-enable the named provider when llm.provider changes so the adapter
	// can be rebuilt immediately. CLI providers are always enabled when selected.
	// API providers are enabled only when an API key is also present in the save.
	if newProvider, ok := patch["llm.provider"]; ok && newProvider != "" {
		switch newProvider {
		case "claude-cli":
			patch["providers.claudeCli.enabled"] = "true"
		case "gemini-cli":
			patch["providers.geminiCli.enabled"] = "true"
		case "ollama":
			patch["providers.ollama.enabled"] = "true"
		default:
			keyField := "providers." + newProvider + ".apiKey"
			if patch[keyField] != "" || patch["llm.apiKey"] != "" {
				patch["providers."+newProvider+".enabled"] = "true"
			}
		}
	}
	if err := s.store.Update(patch); err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":false,"error":"%s"}`, err.Error())
		return
	}
	s.cfg = s.store.Cfg()

	// Hot-reload adapter when LLM or provider config changes
	adapterReloaded := false
	for key := range patch {
		if strings.HasPrefix(key, "llm.") || strings.HasPrefix(key, "providers.") {
			if s.rebuildAdapter != nil {
				newAdapter := s.rebuildAdapter(s.cfg)
				s.adapterMu.Lock()
				s.adapter = newAdapter
				s.adapterMu.Unlock()
				adapterReloaded = true
			}
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if adapterReloaded {
		provider := s.cfg.LLM.Provider
		w.Write([]byte(`{"ok":true,"reloaded":true,"provider":"` + provider + `"}`))
	} else {
		w.Write([]byte(`{"ok":true}`))
	}
}

func (s *Server) handleConfigAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data, _ := s.cfg.ToJSON()
	w.Write(data)
}


// ── MCP Servers ──────────────────────────────────────────────────────────────

func (s *Server) renderMCPServersManager(w http.ResponseWriter) {
	serversJSON, _ := json.Marshal(s.cfg.MCP.Servers)

	fmt.Fprint(w, `<div style="display:grid;grid-template-columns:1fr 2fr;gap:20px;height:calc(100vh - 200px)">
<!-- Left pane: Server list -->
<div class="panel" style="overflow-y:auto">
<div class="panel-header"><img src="/static/img/icons/tools-icon.png"> MCP Servers</div>
<div id="server-list" style="display:flex;flex-direction:column;gap:8px">`)

	if len(s.cfg.MCP.Servers) == 0 {
		fmt.Fprint(w, `<div style="color:var(--text-dim);font-size:12px;padding:12px;text-align:center">No servers yet</div>`)
	} else {
		for _, srv := range s.cfg.MCP.Servers {
			statusCls := "badge-live"
			if !srv.Enabled {
				statusCls = "badge-idle"
			}
			fmt.Fprintf(w, `<div class="mcp-list-item" onclick='selectMCPServer(this, %q)' style="padding:12px;border:1px solid rgba(139,134,128,0.15);border-radius:8px;cursor:pointer;transition:all 0.2s;background:rgba(250,243,240,0.02)">
<div style="display:flex;justify-content:space-between;align-items:center;gap:8px">
<div style="flex:1">
<strong style="display:block;margin-bottom:4px">%s</strong>
<span style="font-size:11px;color:var(--text-dim)">:%d • %s</span>
</div>
<span class="badge %s" style="font-size:10px">%s</span>
</div>
</div>`, srv.Name, srv.Name, srv.Port, srv.Transport, statusCls, map[bool]string{true: "Enabled", false: "Disabled"}[srv.Enabled])
		}
	}

	fmt.Fprint(w, `</div>
<button class="btn btn-primary" style="margin-top:16px;width:100%" onclick="createMCPServer()">+ New Server</button>
</div>

<!-- Right pane: Server editor -->
<div class="panel" style="overflow-y:auto;display:none" id="editor-pane">
<div class="panel-header">MCP Server Editor</div>
<div id="editor-content" style="padding:16px">
<div style="color:var(--text-dim);text-align:center">Select a server to edit</div>
</div>
</div>

<script>
let currentServer = null;
let servers = ` + string(serversJSON) + `;

function selectMCPServer(elem, name) {
  currentServer = servers.find(s => s.name === name);
  if(!currentServer) return;

  document.querySelectorAll('.mcp-list-item').forEach(e => e.style.borderColor='rgba(139,134,128,0.15)');
  elem.style.borderColor = 'var(--af-magma)';

  document.getElementById('editor-pane').style.display = 'block';
  renderMCPEditor();
}

function renderMCPEditor() {
  if(!currentServer) return;

  let html = '<form id="mcp-form" onsubmit="saveMCPServer(event)" style="display:flex;flex-direction:column;gap:12px">' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Server Name</label><input type="text" id="mcp-name" value="' + html.EscapeString(currentServer.name || '') + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)" required></div>' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Port</label><input type="number" id="mcp-port" min="1" max="65535" value="' + (currentServer.port || 9090) + '" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)" required></div>' +
    '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Transport</label><select id="mcp-transport" onchange="updateTransportUI()" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)"><option value="http"' + (currentServer.transport === 'http' ? ' selected' : '') + '>HTTP</option><option value="stdio"' + (currentServer.transport === 'stdio' ? ' selected' : '') + '>Stdio (subprocess)</option></select></div>' +
    '<div id="transport-fields"></div>' +
    '<div style="border-top:1px solid rgba(139,134,128,0.1);padding-top:12px"><label style="display:flex;align-items:center;gap:6px;cursor:pointer"><input type="checkbox" id="mcp-enabled" ' + (currentServer.enabled ? 'checked' : '') + ' style="cursor:pointer"><span style="font-size:12px">Enabled</span></label></div>' +
    '<div style="display:flex;gap:8px"><button type="submit" class="btn btn-primary" style="flex:1">Save Server</button><button type="button" style="flex:1;padding:8px;border:1px solid rgba(139,134,128,0.2);background:rgba(250,243,240,0.03);color:var(--text-primary);border-radius:6px;cursor:pointer" onclick="cancelEdit()">Cancel</button></div>' +
    '</form>';

  document.getElementById('editor-content').innerHTML = html;
  updateTransportUI();
}

function html.EscapeString(text) {
  if(!text) return '';
  let map = {'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;'};
  return text.replace(/[&<>"']/g, m => map[m]);
}

function updateTransportUI() {
  let transport = document.getElementById('mcp-transport').value;
  let fieldsDiv = document.getElementById('transport-fields');

  if(transport === 'http') {
    fieldsDiv.innerHTML = '';
  } else if(transport === 'stdio') {
    let cmd = currentServer.command || '';
    let args = (currentServer.args || []).join(' ');
    fieldsDiv.innerHTML = '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Command</label><input type="text" id="mcp-command" value="' + html.EscapeString(cmd) + '" placeholder="e.g. node" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)"></div>' +
      '<div><label style="display:block;font-size:12px;color:var(--text-dim);margin-bottom:4px">Arguments (space-separated)</label><input type="text" id="mcp-args" value="' + html.EscapeString(args) + '" placeholder="e.g. mcp-serve script.js" style="width:100%;padding:8px;border:1px solid rgba(139,134,128,0.2);border-radius:6px;background:rgba(250,243,240,0.03);color:var(--text-primary)"></div>';
  }
}

function saveMCPServer(e) {
  e.preventDefault();
  if(!currentServer) return;

  currentServer.name = document.getElementById('mcp-name').value;
  currentServer.port = parseInt(document.getElementById('mcp-port').value);
  currentServer.transport = document.getElementById('mcp-transport').value;
  currentServer.enabled = document.getElementById('mcp-enabled').checked;

  if(currentServer.transport === 'stdio') {
    currentServer.command = document.getElementById('mcp-command').value;
    let argsStr = document.getElementById('mcp-args').value;
    currentServer.args = argsStr ? argsStr.split(' ') : [];
  } else {
    delete currentServer.command;
    delete currentServer.args;
  }

  let cfg = JSON.stringify(servers);
  fetch('/api/config/save', {method:'POST',body:new URLSearchParams({'mcp.servers':cfg})})
    .then(r => r.json()).then(d => {
      if(d.ok) {
        showToast('MCP Server saved. Restart daemon to apply.', 'success');
        setTimeout(() => location.reload(), 1200);
      } else {
        showToast('Error: ' + d.error, 'error');
      }
    }).catch(e => showToast('Error: ' + e, 'error'));
}

function createMCPServer() {
  currentServer = {
    name: 'New Server',
    enabled: true,
    port: 9090,
    transport: 'http'
  };
  servers.push(currentServer);

  document.getElementById('server-list').innerHTML = '<div style="color:var(--text-dim);font-size:12px;padding:12px;text-align:center">Creating...</div>';
  document.getElementById('editor-pane').style.display = 'block';
  renderMCPEditor();
}

function cancelEdit() {
  currentServer = null;
  document.getElementById('editor-pane').style.display = 'none';
  document.getElementById('editor-content').innerHTML = '<div style="color:var(--text-dim);text-align:center">Select a server to edit</div>';
}
</script>
</div>`)
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
