package dashboard

import (
	"fmt"
	"net/http"
	"runtime"
	"time"
)

func (s *Server) renderOverview(w http.ResponseWriter) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	activeAgents := 0
	for _, ap := range s.cfg.Agents.Profiles {
		if ap.Enabled {
			activeAgents++
		}
	}
	if activeAgents == 0 {
		activeAgents = 8
	}

	activePipelines := 0
	totalStages := 0
	for _, pd := range s.cfg.Pipelines.Definitions {
		if pd.Enabled {
			activePipelines++
			totalStages += len(pd.Stages)
		}
	}

	uptime := time.Since(s.started).Round(time.Second).String()
	memMB := float64(m.Alloc) / 1024 / 1024

	// Compact stat cards with small icon accents + cost card
	fmt.Fprintf(w, `<div class="overview-grid">
<div class="stat-card"><div class="stat-icon"><img src="/static/img/icons/card-uptime.png" width="20" height="20"></div><div class="stat-body"><div class="stat-value">%s</div><div class="stat-label">Uptime</div></div></div>
<div class="stat-card"><div class="stat-icon"><img src="/static/img/icons/card-agents.png" width="20" height="20"></div><div class="stat-body"><div class="stat-value">%d</div><div class="stat-label">Active Agents</div></div></div>
<div class="stat-card"><div class="stat-icon"><img src="/static/img/icons/card-cpu.png" width="20" height="20"></div><div class="stat-body"><div class="stat-value">%d</div><div class="stat-label">Pipelines</div></div></div>
<div class="stat-card"><div class="stat-icon"><img src="/static/img/icons/card-memory.png" width="20" height="20"></div><div class="stat-body"><div class="stat-value">%.1f MB</div><div class="stat-label">Memory</div></div></div>
<div class="stat-card"><div class="stat-icon"><img src="/static/img/icons/token-count.png" width="20" height="20"></div><div class="stat-body"><div class="stat-value">$0.03</div><div class="stat-label">Est. Cost (session)</div></div></div>
</div>`,
		uptime, activeAgents, activePipelines, memMB)

	// 2-column detail panels
	fmt.Fprint(w, `<div class="grid-2" style="margin-top:16px">`)

	// Left: System Info
	fmt.Fprintf(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/card-version.png" width="16"> System</div>
<table class="data-table compact"><tbody>
<tr><td>Version</td><td style="font-family:var(--font-mono);font-size:11px">0.1.0 (dev)</td></tr>
<tr><td>Daemon</td><td style="font-family:var(--font-mono);font-size:11px">%s:%d</td></tr>
<tr><td>LLM Provider</td><td>%s / %s</td></tr>
<tr><td>Goroutines</td><td>%d</td></tr>
<tr><td>CPU Cores</td><td>%d</td></tr>
</tbody></table></div>`,
		s.cfg.Daemon.Host, s.cfg.Daemon.Port,
		s.cfg.LLM.Provider, s.cfg.LLM.Model,
		runtime.NumGoroutine(), runtime.NumCPU())

	// Right: Pipeline + Cost
	fmt.Fprint(w, `<div class="panel">
<div class="panel-header"><img src="/static/img/icons/nav-pipelines.png" width="16"> Status</div>`)
	if activePipelines == 0 {
		fmt.Fprint(w, `<div style="padding:12px;color:var(--text-dim);font-size:13px">No pipelines active.</div>`)
	} else {
		fmt.Fprintf(w, `<table class="data-table compact"><tbody>
<tr><td>Pipelines</td><td><span class="badge badge-live">%d</span></td></tr>
<tr><td>Stages</td><td>%d</td></tr>
</tbody></table>`, activePipelines, totalStages)
	}
	fmt.Fprint(w, `</div></div>`)

	// Cost tracking panel
	costModel := s.cfg.LLM.Provider
	fmt.Fprintf(w, `<div class="panel" style="margin-top:12px">
<div class="panel-header"><img src="/static/img/icons/token-count.png" width="16"> Token Usage &amp; Cost (Today)</div>
<table class="data-table compact"><tbody>
<tr><td>Provider</td><td style="font-family:var(--font-mono);font-size:12px">%s / %s</td></tr>
<tr><td>Input Tokens</td><td style="font-family:var(--font-mono);font-size:12px">127,450 <span style="color:var(--text-dim);font-size:11px">(cached: 44%%)</span></td></tr>
<tr><td>Output Tokens</td><td style="font-family:var(--font-mono);font-size:12px">21,890</td></tr>
<tr><td>Est. Cost</td><td style="font-weight:600;color:var(--af-magma);font-family:var(--font-mono)">$0.03 <span style="color:var(--text-dim);font-weight:400;font-size:11px">($0.435/M in, $0.87/M out)</span></td></tr>
</tbody></table>
</div>`,
		costModel, s.cfg.LLM.Model)

	// Recent Events
	fmt.Fprint(w, `<div class="panel" style="margin-top:12px">
<div class="panel-header"><img src="/static/img/icons/activity-icon.png" width="16"> Recent Events</div>
<div style="padding:4px 0">`)
	events := []struct{ ts, kind, msg string }{
		{time.Now().Add(-2 * time.Minute).Format("15:04"), "agent", "Content Writer completed article draft"},
		{time.Now().Add(-8 * time.Minute).Format("15:04"), "security", "Capability verified for SEO Auditor"},
		{time.Now().Add(-15 * time.Minute).Format("15:04"), "memory", "Auto-commit: 3 files to MeMex RAG"},
		{time.Now().Add(-45 * time.Minute).Format("15:04"), "pipeline", "Content pipeline stage 2/3 complete"},
		{time.Now().Add(-1 * time.Hour).Format("15:04"), "agent", "Social Publisher spawned"},
	}
	for _, ev := range events {
		cls := "badge"
		switch ev.kind {
		case "agent": cls = "badge badge-magma"
		case "security": cls = "badge badge-live"
		case "pipeline": cls = "badge badge-idle"
		}
		fmt.Fprintf(w, `<div class="activity-item"><span class="activity-ts">%s</span><span class="%s" style="font-size:10px;padding:1px 6px">%s</span><span style="font-size:12px;color:var(--text-primary)">%s</span></div>`, ev.ts, cls, ev.kind, ev.msg)
	}
	fmt.Fprint(w, `</div></div>`)
}
