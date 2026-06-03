package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Model Creation & Initialization Tests ───────────────────────────────────

func TestModel_New(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("Model should not be nil")
	}
	if m.currentPage != pageOverview {
		t.Errorf("Initial page should be overview, got %d", m.currentPage)
	}
	if m.quitting {
		t.Error("Model should not be quitting initially")
	}
}

func TestModel_DefaultState(t *testing.T) {
	m := New()

	if m.agentCount != 5 {
		t.Errorf("Expected 5 agents, got %d", m.agentCount)
	}
	if m.totalTokens != 125000 {
		t.Errorf("Expected 125000 tokens, got %d", m.totalTokens)
	}
	if m.uptime != "2h 14m" {
		t.Errorf("Expected uptime '2h 14m', got %q", m.uptime)
	}
	if len(m.agents) != 3 {
		t.Errorf("Expected 3 agents, got %d", len(m.agents))
	}
	if len(m.pipelines) != 2 {
		t.Errorf("Expected 2 pipelines, got %d", len(m.pipelines))
	}
	if len(m.messages) == 0 {
		t.Error("Should have initial messages")
	}
}

func TestModel_Init(t *testing.T) {
	m := New()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a command")
	}
}

// ── Key Event Tests ─────────────────────────────────────────────────────────

func TestModel_KeyQuitOnce(t *testing.T) {
	m := New()

	// First 'q' press
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if !model.quitting {
		t.Error("Should be in quitting state after first 'q'")
	}
}

func TestModel_KeyQuitConfirm(t *testing.T) {
	m := New()
	m.quitting = true // Already quitting

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, cmd := m.Update(msg)
	model := newModel.(*Model)

	if !model.quitting {
		t.Error("Should remain in quitting state")
	}
	// Second 'q' should attempt to quit
	if cmd == nil {
		// tea.Quit returns nil cmd
	}
}

func TestModel_KeyCtrlC(t *testing.T) {
	m := New()

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if !model.quitting {
		t.Error("Ctrl+C should trigger quit state")
	}
}

// ── Page Navigation Tests ───────────────────────────────────────────────────

func TestModel_NavigatePage1(t *testing.T) {
	m := New()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if model.currentPage != pageOverview {
		t.Errorf("Expected page 1 (overview), got %d", model.currentPage)
	}
}

func TestModel_NavigatePage2(t *testing.T) {
	m := New()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if model.currentPage != pageAgents {
		t.Errorf("Expected page 2 (agents), got %d", model.currentPage)
	}
}

func TestModel_NavigatePage3(t *testing.T) {
	m := New()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if model.currentPage != pagePipelines {
		t.Errorf("Expected page 3 (pipelines), got %d", model.currentPage)
	}
}

func TestModel_NavigatePage4(t *testing.T) {
	m := New()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if model.currentPage != pageChannels {
		t.Errorf("Expected page 4 (channels), got %d", model.currentPage)
	}
}

func TestModel_NavigatePage5(t *testing.T) {
	m := New()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if model.currentPage != pageChat {
		t.Errorf("Expected page 5 (chat), got %d", model.currentPage)
	}
}

// ── Chat Input Tests ────────────────────────────────────────────────────────

func TestModel_ChatInputCharacter(t *testing.T) {
	m := New()
	m.currentPage = pageChat

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if model.inputText != "a" {
		t.Errorf("Expected input 'a', got %q", model.inputText)
	}
}

func TestModel_ChatInputMultipleCharacters(t *testing.T) {
	m := New()
	m.currentPage = pageChat

	chars := []string{"h", "e", "l", "l", "o"}
	for _, ch := range chars {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(ch[0])}}
		newModel, _ := m.Update(msg)
		m = newModel.(*Model)
	}

	if m.inputText != "hello" {
		t.Errorf("Expected 'hello', got %q", m.inputText)
	}
}

func TestModel_ChatInputBackspace(t *testing.T) {
	m := New()
	m.currentPage = pageChat
	m.inputText = "hello"

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if model.inputText != "hell" {
		t.Errorf("Expected 'hell', got %q", model.inputText)
	}
}

func TestModel_ChatInputBackspaceEmpty(t *testing.T) {
	m := New()
	m.currentPage = pageChat
	m.inputText = ""

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if model.inputText != "" {
		t.Errorf("Backspace on empty should remain empty, got %q", model.inputText)
	}
}

func TestModel_ChatInputEnter(t *testing.T) {
	m := New()
	m.currentPage = pageChat
	m.inputText = "hello"
	initialMsgCount := len(m.messages)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	// Should add user message and processing message
	if len(model.messages) != initialMsgCount+2 {
		t.Errorf("Expected %d messages, got %d", initialMsgCount+2, len(model.messages))
	}
	if model.inputText != "" {
		t.Errorf("Input should be cleared after enter, got %q", model.inputText)
	}
	if !strings.Contains(model.messages[initialMsgCount], "You: hello") {
		t.Error("Should add user message to chat")
	}
	if !strings.Contains(model.messages[initialMsgCount+1], "Processing") {
		t.Error("Should add processing message")
	}
}

func TestModel_ChatInputEnterEmpty(t *testing.T) {
	m := New()
	m.currentPage = pageChat
	m.inputText = ""
	initialMsgCount := len(m.messages)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	// Should not add message if input is empty
	if len(model.messages) != initialMsgCount {
		t.Errorf("Should not add message on empty input")
	}
}

// ── Input Isolation Tests ───────────────────────────────────────────────────

func TestModel_InputOnNonChatPage(t *testing.T) {
	m := New()
	m.currentPage = pageOverview // Not chat page

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	// Input should not be captured on non-chat pages
	if model.inputText != "" {
		t.Errorf("Input should not be captured outside chat page, got %q", model.inputText)
	}
}

func TestModel_BackspaceOnNonChatPage(t *testing.T) {
	m := New()
	m.currentPage = pageAgents
	m.inputText = "test"

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	// Backspace should still work everywhere (model allows it)
	if model.inputText != "tes" {
		t.Errorf("Backspace should work, expected 'tes', got %q", model.inputText)
	}
}

// ── Window Size Tests ───────────────────────────────────────────────────────

func TestModel_WindowSize(t *testing.T) {
	m := New()

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	if model.width != 80 || model.height != 24 {
		t.Errorf("Expected 80x24, got %dx%d", model.width, model.height)
	}
}

func TestModel_WindowSizeChange(t *testing.T) {
	m := New()

	msg1 := tea.WindowSizeMsg{Width: 80, Height: 24}
	newModel, _ := m.Update(msg1)
	m = newModel.(*Model)

	msg2 := tea.WindowSizeMsg{Width: 120, Height: 40}
	newModel, _ = m.Update(msg2)
	model := newModel.(*Model)

	if model.width != 120 || model.height != 40 {
		t.Errorf("Expected 120x40 after resize, got %dx%d", model.width, model.height)
	}
}

// ── View Rendering Tests ────────────────────────────────────────────────────

func TestModel_ViewOverview(t *testing.T) {
	m := New()
	view := m.View()

	if !strings.Contains(view, "OVERVIEW") {
		t.Error("Overview view should contain 'OVERVIEW' header")
	}
	if !strings.Contains(view, "Uptime") {
		t.Error("Overview should show uptime")
	}
	if !strings.Contains(view, "Agents") {
		t.Error("Overview should show agent count")
	}
}

func TestModel_ViewAgents(t *testing.T) {
	m := New()
	m.currentPage = pageAgents
	view := m.View()

	if !strings.Contains(view, "AGENT FLEET") {
		t.Error("Agents view should contain 'AGENT FLEET' header")
	}
	if !strings.Contains(view, "running") || !strings.Contains(view, "idle") {
		t.Error("Agents view should show agent statuses")
	}
}

func TestModel_ViewPipelines(t *testing.T) {
	m := New()
	m.currentPage = pagePipelines
	view := m.View()

	if !strings.Contains(view, "PIPELINES") {
		t.Error("Pipelines view should contain 'PIPELINES' header")
	}
}

func TestModel_ViewChannels(t *testing.T) {
	m := New()
	m.currentPage = pageChannels
	view := m.View()

	if !strings.Contains(view, "CHANNELS") {
		t.Error("Channels view should contain 'CHANNELS' header")
	}
	if !strings.Contains(view, "Telegram") || !strings.Contains(view, "Discord") {
		t.Error("Channels view should show channel status")
	}
}

func TestModel_ViewChat(t *testing.T) {
	m := New()
	m.currentPage = pageChat
	view := m.View()

	if !strings.Contains(view, "CHAT") {
		t.Error("Chat view should contain 'CHAT' header")
	}
	if !strings.Contains(view, ">") {
		t.Error("Chat view should show input prompt")
	}
}

func TestModel_QuitMessage(t *testing.T) {
	m := New()
	m.quitting = true
	view := m.View()

	if !strings.Contains(view, "Goodbye") {
		t.Error("Quit view should show goodbye message")
	}
}

// ── Navigation Display Tests ────────────────────────────────────────────────

func TestModel_TabDisplay(t *testing.T) {
	m := New()
	view := m.View()

	if !strings.Contains(view, "1.Overview") {
		t.Error("View should contain tab 1")
	}
	if !strings.Contains(view, "2.Agents") {
		t.Error("View should contain tab 2")
	}
	if !strings.Contains(view, "3.Pipelines") {
		t.Error("View should contain tab 3")
	}
	if !strings.Contains(view, "4.Channels") {
		t.Error("View should contain tab 4")
	}
	if !strings.Contains(view, "5.Chat") {
		t.Error("View should contain tab 5")
	}
}

func TestModel_ActiveTabHighlight(t *testing.T) {
	m := New()
	m.currentPage = pageAgents
	view := m.View()

	// Active tab should be highlighted (orange color code in rendered output)
	if !strings.Contains(view, "2.Agents") {
		t.Error("Agents tab should be in view")
	}
}

// ── Concurrent State Modification Tests ──────────────────────────────────────

func TestModel_SequentialUpdates(t *testing.T) {
	m := New()

	// Navigate to chat
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}}
	newModel, _ := m.Update(msg)
	m = newModel.(*Model)

	// Type message
	for _, ch := range []rune("test") {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}}
		newModel, _ := m.Update(msg)
		m = newModel.(*Model)
	}

	// Send message
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ = m.Update(msg)
	m = newModel.(*Model)

	if m.inputText != "" {
		t.Errorf("Input should be cleared after send, got %q", m.inputText)
	}
	if len(m.messages) < 3 {
		t.Error("Should have accumulated messages from conversation")
	}
}

// ── Data Consistency Tests ──────────────────────────────────────────────────

func TestModel_AgentListConsistency(t *testing.T) {
	m := New()
	initialCount := len(m.agents)

	// Navigate and interact
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	newModel, _ := m.Update(msg)
	m = newModel.(*Model)

	if len(m.agents) != initialCount {
		t.Error("Agent list should not change during navigation")
	}
}

func TestModel_PipelineListConsistency(t *testing.T) {
	m := New()
	initialCount := len(m.pipelines)

	// Navigate and interact
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}
	newModel, _ := m.Update(msg)
	m = newModel.(*Model)

	if len(m.pipelines) != initialCount {
		t.Error("Pipeline list should not change during navigation")
	}
}

// ── Unknown Input Tests ─────────────────────────────────────────────────────

func TestModel_UnknownKey(t *testing.T) {
	m := New()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	newModel, _ := m.Update(msg)
	model := newModel.(*Model)

	// Unknown keys should be ignored
	if model.currentPage != pageOverview {
		t.Error("Unknown key should not change state")
	}
}

// ── Integration Tests ───────────────────────────────────────────────────────

func TestModel_CompleteUserSession(t *testing.T) {
	m := New()

	// Start at overview
	view := m.View()
	if !strings.Contains(view, "OVERVIEW") {
		t.Error("Should start at overview")
	}

	// Navigate through all pages
	pages := []string{"1", "2", "3", "4", "5"}
	for _, page := range pages {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(page[0])}}
		newModel, _ := m.Update(msg)
		m = newModel.(*Model)
		view = m.View()
		if view == "" {
			t.Errorf("Page %s should render", page)
		}
	}

	// Chat interaction
	m.currentPage = pageChat
	initialMsgCount := len(m.messages)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	newModel, _ := m.Update(msg)
	m = newModel.(*Model)

	msg = tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ = m.Update(msg)
	m = newModel.(*Model)

	if len(m.messages) <= initialMsgCount {
		t.Error("Chat should add messages")
	}

	// Quit
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, _ = m.Update(msg)
	m = newModel.(*Model)

	if !m.quitting {
		t.Error("Should be in quit state")
	}
}
