// Package tui — Bubble Tea terminal interface for AgentForge.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6B2C")).MarginLeft(1)
	tabStyle    = lipgloss.NewStyle().Padding(0, 3).Margin(1, 0)
	activeTab   = tabStyle.Foreground(lipgloss.Color("#FF6B2C")).Background(lipgloss.Color("#2A2A2A"))
	inactiveTab = tabStyle.Foreground(lipgloss.Color("#888888"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
)

type page int

const (
	pageOverview  page = iota
	pageAgents
	pagePipelines
	pageChannels
	pageChat
)

// Model holds the TUI state.
type Model struct {
	width, height int
	currentPage   page
	quitting      bool

	// Live data feeds
	agentCount  int
	totalTokens int
	uptime      string
	agents      []string
	pipelines   []string
	messages    []string
	inputText   string
}

// New creates a new TUI model with default state.
func New() *Model {
	return &Model{
		currentPage: pageOverview,
		agentCount:  5,
		totalTokens: 125000,
		uptime:      "2h 14m",
		agents:      []string{"content-writer (running)", "seo-auditor (running)", "social-publisher (idle)"},
		pipelines:   []string{"daily-content ✓", "seo-gate ✓"},
		messages:    []string{"AgentForge: I am a capability-secured agent orchestrator. Ask me anything."},
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.quitting {
				return m, tea.Quit
			}
			m.quitting = true
			return m, nil
		case "1":
			m.currentPage = pageOverview
		case "2":
			m.currentPage = pageAgents
		case "3":
			m.currentPage = pagePipelines
		case "4":
			m.currentPage = pageChannels
		case "5":
			m.currentPage = pageChat
		case "enter":
			if m.currentPage == pageChat && m.inputText != "" {
				m.messages = append(m.messages, "You: "+m.inputText)
				m.messages = append(m.messages, "AgentForge: Processing...")
				m.inputText = ""
			}
		case "backspace":
			if len(m.inputText) > 0 {
				m.inputText = m.inputText[:len(m.inputText)-1]
			}
		default:
			if m.currentPage == pageChat && len(msg.String()) == 1 {
				m.inputText += msg.String()
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *Model) View() string {
	if m.quitting {
		return "Goodbye from AgentForge TUI.\n"
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("🧠 AgentForge TUI"))
	b.WriteString("\n\n")

	// Navigation tabs
	tabs := []string{"1.Overview", "2.Agents", "3.Pipelines", "4.Channels", "5.Chat"}
	for i, tab := range tabs {
		if i == int(m.currentPage) {
			b.WriteString(activeTab.Render(tab))
		} else {
			b.WriteString(inactiveTab.Render(tab))
		}
	}
	b.WriteString("\n\n")

	// Page content
	switch m.currentPage {
	case pageOverview:
		b.WriteString(m.viewOverview())
	case pageAgents:
		b.WriteString(m.viewAgents())
	case pagePipelines:
		b.WriteString(m.viewPipelines())
	case pageChannels:
		b.WriteString(m.viewChannels())
	case pageChat:
		b.WriteString(m.viewChat())
	}

	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("q quit • 1-5 tabs • enter send"))
	return b.String()
}

func (m *Model) viewOverview() string {
	var b strings.Builder
	b.WriteString("═══ OVERVIEW ═══\n\n")
	b.WriteString(fmt.Sprintf("Uptime:      %s\n", m.uptime))
	b.WriteString(fmt.Sprintf("Agents:      %d active\n", m.agentCount))
	b.WriteString(fmt.Sprintf("Tokens:      %d used today\n", m.totalTokens))
	b.WriteString(fmt.Sprintf("Pipelines:   %d running\n", len(m.pipelines)))
	b.WriteString("\nPipelines:\n")
	for _, p := range m.pipelines {
		b.WriteString(fmt.Sprintf("  %s\n", p))
	}
	return b.String()
}

func (m *Model) viewAgents() string {
	var b strings.Builder
	b.WriteString("═══ AGENT FLEET ═══\n\n")
	for _, a := range m.agents {
		switch {
		case strings.Contains(a, "running"):
			b.WriteString(greenStyle.Render(fmt.Sprintf("  ● %s\n", a)))
		case strings.Contains(a, "idle"):
			b.WriteString(dimStyle.Render(fmt.Sprintf("  ○ %s\n", a)))
		default:
			b.WriteString(redStyle.Render(fmt.Sprintf("  ✗ %s\n", a)))
		}
	}
	return b.String()
}

func (m *Model) viewPipelines() string {
	var b strings.Builder
	b.WriteString("═══ PIPELINES ═══\n\n")
	for _, p := range m.pipelines {
		b.WriteString(fmt.Sprintf("  %s\n", p))
	}
	if m.pipelines == nil {
		b.WriteString("  No pipelines configured.\n")
	}
	return b.String()
}

func (m *Model) viewChannels() string {
	var b strings.Builder
	b.WriteString("═══ CHANNELS ═══\n\n")
	b.WriteString(greenStyle.Render("  ● Telegram: connected\n"))
	b.WriteString(greenStyle.Render("  ● Discord: connected\n"))
	b.WriteString(redStyle.Render("  ✗ Slack: offline\n"))
	b.WriteString(redStyle.Render("  ✗ Signal: offline\n"))
	b.WriteString(dimStyle.Render("  ○ WhatsApp: not configured\n"))
	b.WriteString(dimStyle.Render("  ○ Matrix: not configured\n"))
	return b.String()
}

func (m *Model) viewChat() string {
	var b strings.Builder
	b.WriteString("═══ CHAT ═══\n\n")
	for _, msg := range m.messages {
		b.WriteString(fmt.Sprintf("%s\n", msg))
	}
	b.WriteString(fmt.Sprintf("\n> %s", m.inputText))
	return b.String()
}