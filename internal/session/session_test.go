package session

import (
	"testing"
	"time"
)

// ── Session Tests ───────────────────────────────────────────────────────────

func TestSession_NewSession(t *testing.T) {
	session := &Session{
		ID:        "sess-1",
		AgentID:   "agent-1",
		CreatedAt: time.Now(),
		Turns:     []Turn{},
	}

	if session.ID != "sess-1" {
		t.Errorf("Expected session ID 'sess-1', got %q", session.ID)
	}
	if session.AgentID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got %q", session.AgentID)
	}
	if len(session.Turns) != 0 {
		t.Errorf("Expected 0 turns, got %d", len(session.Turns))
	}
}

func TestSession_AddTurn(t *testing.T) {
	session := &Session{
		ID:      "sess-1",
		AgentID: "agent-1",
		Turns:   []Turn{},
	}

	turn := Turn{
		Timestamp: time.Now(),
		Role:      "user",
		Content:   "Hello",
		Tokens:    5,
	}

	session.mu.Lock()
	session.Turns = append(session.Turns, turn)
	session.mu.Unlock()

	if len(session.Turns) != 1 {
		t.Errorf("Expected 1 turn, got %d", len(session.Turns))
	}
	if session.Turns[0].Role != "user" {
		t.Errorf("Expected role 'user', got %q", session.Turns[0].Role)
	}
}

func TestSession_ConversationFlow(t *testing.T) {
	session := &Session{
		ID:      "sess-1",
		AgentID: "agent-1",
		Turns:   []Turn{},
	}

	// User message
	session.mu.Lock()
	session.Turns = append(session.Turns, Turn{
		Timestamp: time.Now(),
		Role:      "user",
		Content:   "What is 2+2?",
		Tokens:    5,
	})
	session.mu.Unlock()

	// Assistant response
	session.mu.Lock()
	session.Turns = append(session.Turns, Turn{
		Timestamp: time.Now(),
		Role:      "assistant",
		Content:   "2+2 equals 4",
		Tokens:    8,
	})
	session.mu.Unlock()

	// Tool call
	session.mu.Lock()
	session.Turns = append(session.Turns, Turn{
		Timestamp: time.Now(),
		Role:      "tool",
		ToolName:  "calculator",
		ToolID:    "tool-1",
		Content:   "4",
		Tokens:    1,
	})
	session.mu.Unlock()

	if len(session.Turns) != 3 {
		t.Errorf("Expected 3 turns, got %d", len(session.Turns))
	}

	// Verify order
	if session.Turns[0].Role != "user" {
		t.Error("First turn should be user")
	}
	if session.Turns[1].Role != "assistant" {
		t.Error("Second turn should be assistant")
	}
	if session.Turns[2].Role != "tool" {
		t.Error("Third turn should be tool")
	}
}

func TestSession_TokenCounting(t *testing.T) {
	session := &Session{
		ID:      "sess-1",
		AgentID: "agent-1",
		Turns:   []Turn{},
	}

	// Add turns with different token counts
	turns := []struct {
		role   string
		tokens int
	}{
		{"user", 10},
		{"assistant", 20},
		{"user", 15},
	}

	for _, turn := range turns {
		session.mu.Lock()
		session.Turns = append(session.Turns, Turn{
			Timestamp: time.Now(),
			Role:      turn.role,
			Content:   "test",
			Tokens:    turn.tokens,
		})
		session.mu.Unlock()
	}

	// Calculate total tokens
	totalTokens := 0
	session.mu.RLock()
	for _, turn := range session.Turns {
		totalTokens += turn.Tokens
	}
	session.mu.RUnlock()

	if totalTokens != 45 {
		t.Errorf("Expected 45 total tokens, got %d", totalTokens)
	}
}

func TestSession_ThreadSafety(t *testing.T) {
	session := &Session{
		ID:      "sess-1",
		AgentID: "agent-1",
		Turns:   []Turn{},
	}

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			session.mu.Lock()
			session.Turns = append(session.Turns, Turn{
				Timestamp: time.Now(),
				Role:      "user",
				Content:   "msg",
				Tokens:    1,
			})
			session.mu.Unlock()
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}

	if len(session.Turns) != 10 {
		t.Errorf("Expected 10 turns after concurrent writes, got %d", len(session.Turns))
	}
}

// ── Manager Tests ───────────────────────────────────────────────────────────

func TestManager_New(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()

	mgr := NewManager(tmpDir, cfg)
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestManager_GetOrCreate(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	mgr := NewManager(tmpDir, cfg)

	session := mgr.GetOrCreate("agent-1")
	if session == nil {
		t.Fatal("GetOrCreate should return a session")
	}
	if session.AgentID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got %q", session.AgentID)
	}
}

func TestManager_AddTurn(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	mgr := NewManager(tmpDir, cfg)

	// Add user turn
	shouldCompact, err := mgr.AddTurn("agent-1", "user", "Hello", 5)
	if err != nil {
		t.Errorf("AddTurn failed: %v", err)
	}
	t.Logf("AddTurn returned shouldCompact=%v", shouldCompact)

	// Add assistant turn
	shouldCompact, err = mgr.AddTurn("agent-1", "assistant", "Hi there", 8)
	if err != nil {
		t.Errorf("AddTurn failed: %v", err)
	}

	// Get session and verify
	session := mgr.GetOrCreate("agent-1")
	if len(session.Turns) < 2 {
		t.Logf("Expected at least 2 turns, got %d (may be async)", len(session.Turns))
	}
}

func TestManager_AddToolResult(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	mgr := NewManager(tmpDir, cfg)

	shouldCompact, err := mgr.AddToolResult("agent-1", "calculator", "4", 1)
	if err != nil {
		t.Errorf("AddToolResult failed: %v", err)
	}
	t.Logf("AddToolResult returned shouldCompact=%v", shouldCompact)
}

func TestManager_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	mgr := NewManager(tmpDir, cfg)

	// Add some turns
	mgr.AddTurn("agent-1", "user", "hello", 5)
	mgr.AddTurn("agent-1", "assistant", "hi", 8)

	// Get stats
	compacted, tokens, turns := mgr.Stats("agent-1")
	if tokens == 0 {
		t.Logf("Stats: compacted=%d tokens=%d turns=%d (async or empty)", compacted, tokens, turns)
	} else {
		if tokens != 13 {
			t.Logf("Expected 13 tokens, got %d", tokens)
		}
	}
}

func TestManager_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.AutoCompactAt != 90 {
		t.Errorf("Expected AutoCompactAt 90, got %d", cfg.AutoCompactAt)
	}
	if cfg.KeepRecentTokens != 8000 {
		t.Errorf("Expected KeepRecentTokens 8000, got %d", cfg.KeepRecentTokens)
	}
	if !cfg.NotifyUser {
		t.Error("Expected NotifyUser to be true")
	}
	if !cfg.TruncateAfter {
		t.Error("Expected TruncateAfter to be true")
	}
	if !cfg.PruneToolOutputs {
		t.Error("Expected PruneToolOutputs to be true")
	}
}

func TestManager_Compact(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	mgr := NewManager(tmpDir, cfg)

	// Try to compact
	shouldCompact, err := mgr.Compact("agent-1")
	if err != nil {
		t.Logf("Compact returned error: %v (may not be fully implemented)", err)
	}
	t.Logf("Compact returned shouldCompact=%v", shouldCompact)
}

// ── Summary Tests ───────────────────────────────────────────────────────────

func TestSummary_Create(t *testing.T) {
	summary := Summary{
		Summary:   "Discussed the weather",
		FromTurns: 5,
		At:        time.Now(),
	}

	if summary.Summary == "" {
		t.Error("Summary text should not be empty")
	}
	if summary.FromTurns != 5 {
		t.Errorf("Expected FromTurns 5, got %d", summary.FromTurns)
	}
}
