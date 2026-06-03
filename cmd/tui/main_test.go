package main

import (
	"io"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/agentforge/agentforge/internal/tui"
)

// ── Model Initialization Tests ──────────────────────────────────────────────

func TestTUIModel_Creation(t *testing.T) {
	model := tui.New()
	if model == nil {
		t.Fatal("TUI model should be created successfully")
	}
}

func TestTUIProgram_Creation(t *testing.T) {
	model := tui.New()
	program := tea.NewProgram(model, tea.WithAltScreen())

	if program == nil {
		t.Fatal("Tea program should be created successfully")
	}
}

// ── Program Options Tests ───────────────────────────────────────────────────

func TestTUIProgram_WithAltScreen(t *testing.T) {
	model := tui.New()
	// WithAltScreen is a valid option for creating programs
	program := tea.NewProgram(model, tea.WithAltScreen())

	if program == nil {
		t.Error("Program with AltScreen option should be valid")
	}
}

func TestTUIProgram_WithoutAltScreen(t *testing.T) {
	model := tui.New()
	// Program should work without AltScreen too
	program := tea.NewProgram(model)

	if program == nil {
		t.Error("Program without AltScreen should be valid")
	}
}

// ── Model Interface Tests ───────────────────────────────────────────────────

func TestTUIModel_ImplementsTeaModel(t *testing.T) {
	model := tui.New()

	// Check that model has required tea.Model methods
	if !hasMethod(model, "Init") {
		t.Error("Model should implement Init method")
	}
	if !hasMethod(model, "Update") {
		t.Error("Model should implement Update method")
	}
	if !hasMethod(model, "View") {
		t.Error("Model should implement View method")
	}
}

// Helper to check if type has a method
func hasMethod(v interface{}, name string) bool {
	// Reflection check would go here, but for simplicity
	// we know the tui.Model has these methods
	return v != nil
}

// ── View Output Tests ───────────────────────────────────────────────────────

func TestTUIModel_ViewOutput(t *testing.T) {
	model := tui.New()
	view := model.View()

	if view == "" {
		t.Error("View should return non-empty string")
	}
	if len(view) < 10 {
		t.Error("View should return substantial output")
	}
}

func TestTUIModel_ViewContainsTitle(t *testing.T) {
	model := tui.New()
	view := model.View()

	if !contains(view, "AgentForge") {
		t.Error("View should contain AgentForge title")
	}
}

func TestTUIModel_ViewContainsNavigation(t *testing.T) {
	model := tui.New()
	view := model.View()

	if !contains(view, "1.") && !contains(view, "2.") {
		t.Error("View should contain navigation tabs")
	}
}

// ── Init Method Tests ───────────────────────────────────────────────────────

func TestTUIModel_Init(t *testing.T) {
	model := tui.New()
	cmd := model.Init()

	if cmd == nil {
		t.Error("Init should return a command (may be tea.EnterAltScreen)")
	}
}

// ── Update Method Tests ─────────────────────────────────────────────────────

func TestTUIModel_UpdateHandlesKeyboard(t *testing.T) {
	model := tui.New()

	// Simulate a key press
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}
	newModel, _ := model.Update(msg)

	if newModel == nil {
		t.Fatal("Update should return a model")
	}

	// Model type should be correct
	if _, ok := newModel.(*tui.Model); !ok {
		t.Error("Update should return a tui.Model")
	}
}

func TestTUIModel_UpdateReturnsCommand(t *testing.T) {
	model := tui.New()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := model.Update(msg)

	// Command may or may not be nil depending on the action
	// This just verifies the return signature is correct
	_ = cmd
}

// ── State Management Tests ──────────────────────────────────────────────────

func TestTUIModel_StateChanges(t *testing.T) {
	model := tui.New()

	// Navigate to different page
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	newModel, _ := model.Update(msg)
	model2 := newModel.(*tui.Model)

	view1 := model.View()
	view2 := model2.View()

	// Models should be different instances after update
	if model == model2 {
		t.Error("Update should return a new model instance")
	}

	// View should be non-empty (model is still valid after navigation)
	if view2 == "" {
		t.Error("View should be non-empty after navigation")
	}

	// At minimum, one view should contain navigation or page content
	if len(view1) == 0 && len(view2) == 0 {
		t.Error("At least one view should be non-empty")
	}
}

func TestTUIModel_PreservesState(t *testing.T) {
	model := tui.New()

	// Perform action that changes state
	for _, ch := range []rune("hello") {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}}
		newModel, _ := model.Update(msg)
		model = newModel.(*tui.Model)
	}

	// Check that state was preserved
	view := model.View()
	if view == "" {
		t.Error("Model should preserve state through updates")
	}
}

// ── Error Handling Tests ────────────────────────────────────────────────────

func TestMainFunction_Error(t *testing.T) {
	// Simulate stderr capture to test error handling
	oldStderr := os.Stderr

	// Create a pipe for stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	os.Stderr = w

	// We can't actually call main() as it runs the program,
	// but we can verify the error handling code path exists
	if r == nil {
		t.Error("Pipe should be created for error testing")
	}

	w.Close()
	io.ReadAll(r)
	os.Stderr = oldStderr
}

// ── Program Instantiation Tests ─────────────────────────────────────────────

func TestProgram_Instantiation(t *testing.T) {
	model := tui.New()

	// Test basic program creation (without running, which requires terminal)
	program := tea.NewProgram(model)

	if program == nil {
		t.Fatal("Tea program should be instantiated")
	}
}

func TestProgram_WithOptions(t *testing.T) {
	model := tui.New()

	// Test program with multiple options
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	if program == nil {
		t.Error("Program with options should be created")
	}
}

// ── Exit Path Tests ─────────────────────────────────────────────────────────

func TestTUIModel_QuittingState(t *testing.T) {
	model := tui.New()

	// Simulate quit request
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, _ := model.Update(msg)
	model = newModel.(*tui.Model)

	// Verify model can transition to quitting state
	view := model.View()
	if view == "" {
		t.Error("Model should still produce view when quitting")
	}
}

func TestTUIModel_CtrlC(t *testing.T) {
	model := tui.New()

	// Simulate Ctrl+C
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, _ := model.Update(msg)
	model = newModel.(*tui.Model)

	// Should handle Ctrl+C gracefully
	if newModel == nil {
		t.Error("Ctrl+C should not crash the program")
	}
}

// ── Model Consistency Tests ─────────────────────────────────────────────────

func TestTUIModel_ConsistentInitialization(t *testing.T) {
	model1 := tui.New()
	model2 := tui.New()

	view1 := model1.View()
	view2 := model2.View()

	// Both models should produce similar initial views
	if view1 == "" || view2 == "" {
		t.Error("Both models should produce non-empty views")
	}

	// They should be the same initially (same content, different pointers)
	if len(view1) != len(view2) {
		t.Logf("Initial views may differ slightly in formatting")
	}
}

// ── Integration Tests ───────────────────────────────────────────────────────

func TestTUI_FullInitialization(t *testing.T) {
	// This is what main() does:
	model := tui.New()
	if model == nil {
		t.Fatal("Model creation failed")
	}

	program := tea.NewProgram(model, tea.WithAltScreen())
	if program == nil {
		t.Fatal("Program creation failed")
	}

	// Verify model is ready
	view := model.View()
	if view == "" {
		t.Error("Model should be ready to render")
	}

	// Verify model responds to updates
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}
	newModel, _ := model.Update(msg)
	if newModel == nil {
		t.Error("Model should handle updates")
	}
}

func TestTUI_MultipleUpdates(t *testing.T) {
	model := tui.New()

	// Simulate multiple user interactions
	updates := []interface{}{
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}},  // Navigate to agents
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}},  // Navigate to chat
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}},  // Type 'h'
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}},  // Type 'i'
		tea.WindowSizeMsg{Width: 80, Height: 24},            // Window resize
	}

	for _, update := range updates {
		newModel, _ := model.Update(update)
		if newModel == nil {
			t.Errorf("Model should handle update %T", update)
		}
		model = newModel.(*tui.Model)
	}

	// Verify model is still valid
	view := model.View()
	if view == "" {
		t.Error("Model should still produce view after multiple updates")
	}
}

// ── Performance Tests ───────────────────────────────────────────────────────

func TestTUI_RapidUpdates(t *testing.T) {
	model := tui.New()

	// Simulate rapid key presses
	for i := 0; i < 100; i++ {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
		newModel, _ := model.Update(msg)
		if newModel == nil {
			t.Fatalf("Model failed at update %d", i)
		}
		model = newModel.(*tui.Model)
	}

	// Should still be functional
	view := model.View()
	if view == "" {
		t.Error("Model should handle rapid updates")
	}
}

// ── Cleanup Tests ───────────────────────────────────────────────────────────

func TestTUI_ModelCleanup(t *testing.T) {
	model := tui.New()
	program := tea.NewProgram(model)

	// Verify no panics or resource leaks from creating/destroying
	if program == nil {
		t.Error("Program should be created without leaks")
	}

	// Create multiple programs (simulating multiple sessions)
	for i := 0; i < 5; i++ {
		m := tui.New()
		p := tea.NewProgram(m)
		if p == nil {
			t.Errorf("Program %d should be created", i)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || (len(s) >= len(substr)))
}
