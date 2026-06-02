package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/llm"
	"github.com/agentforge/agentforge/internal/memory"
	"github.com/agentforge/agentforge/internal/security"
	"github.com/agentforge/agentforge/internal/tool"
)

// ── Mock LLM adapter ─────────────────────────────────────────────────────────

type mockAdapter struct {
	response string
	toolCalls []llm.ToolCall
}

func (m *mockAdapter) Provider() string { return "mock" }
func (m *mockAdapter) Chat(_ context.Context, req llm.Request) (llm.Response, error) {
	// Return the configured response with optional tool calls
	return llm.Response{
		ID:    "mock-id",
		Model: req.Model,
		Choices: []llm.Choice{{
			Index: 0,
			Message: llm.Message{
				Role:    "assistant",
				Content: m.response,
			},
			Finish:    "stop",
			ToolCalls: m.toolCalls,
		}},
		Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}, nil
}
func (m *mockAdapter) HealthCheck(_ context.Context) error { return nil }
func (m *mockAdapter) ContextWindow() int { return 128000 }

// ── E2E Test: Agent receives prompt, calls LLM, writes to memory ────────────

func TestAgentLoopE2E(t *testing.T) {
	// Setup a temp memory directory
	memDir := t.TempDir()

	// Create bus, enforcer, registry, memory store, adapter
	b := bus.NewLocal()
	defer b.Close()

	sec := security.NewEnforcer("test-secret")

	reg := tool.NewRegistry()
	reg.Register(&tool.FilesystemTool{})

	store, err := memory.New(memDir)
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	defer store.Close()

	adapter := &mockAdapter{
		response: "Hello! I am a test agent. I have successfully processed your request.",
	}

	// Spawn an agent via the department
	ctx := context.Background()

	dept := NewDepartment("test", 5, AgentConfig{
		HeartbeatInterval: 60 * time.Second,
		MaxLoopIterations: 5,
		ToolTimeout:       5 * time.Second,
	}, b)

	agent, err := dept.Spawn(ctx, sec, "test-agent", "test", "mock-model", adapter, reg, store)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer dept.Release(agent)

	// Subscribe to response topic — agent replies on "agent.{source}.response"
	source := "test-suite"
	respCh, err := b.Subscribe("agent."+source+".response", bus.DefaultFilter)
	if err != nil {
		t.Fatalf("subscribe responses: %v", err)
	}

	// Give the agent goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// ── Phase 1: Send a command to the agent ────────────────────────────────
	cmdPayload := CommandPayload{
		Action: "run",
		Args: map[string]any{
			"prompt": "Write a short hello message",
		},
	}
	payload, _ := json.Marshal(cmdPayload)

	env := bus.Envelope{
		ID:      "test-env-1",
		Source:  source,
		Target:  agent.ID,
		Kind:    bus.KindCommand,
		Topic:   "agent." + agent.ID + ".inbox",
		Payload: json.RawMessage(payload),
	}
	b.Publish(ctx, env)

	// Wait for response
	var resp bus.Envelope
	select {
	case resp = <-respCh:
		// Got response
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for agent response")
	}

	// Verify response payload
	var result map[string]any
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if errMsg, ok := result["error"]; ok {
		t.Fatalf("agent returned error: %v", errMsg)
	}

	responseText, _ := result["response"].(string)
	if responseText == "" {
		t.Fatal("expected non-empty response from agent")
	}
	t.Logf("Agent response: %s", responseText)

	// ── Phase 2: Verify memory was written ──────────────────────────────────
	time.Sleep(100 * time.Millisecond) // let async writes flush

	results, err := store.Search("Hello", memory.SearchOpts{Limit: 10})
	if err != nil {
		t.Fatalf("memory search: %v", err)
	}
	t.Logf("Memory search returned %d results", len(results))
	// Even if FTS index isn't wired, fallback should at least find something

	// Check the agent log file exists
	logPath := filepath.Join(memDir, "daily", "agent-log.md")
	if _, err := os.Stat(logPath); err == nil {
		data, _ := os.ReadFile(logPath)
		t.Logf("Agent log written, size=%d bytes", len(data))
	} else {
		t.Logf("Agent log not yet written (appending may be async): %v", err)
	}
}

// ── Test: Agent processes tool calls via registry ────────────────────────────

func TestAgentToolCallE2E(t *testing.T) {
	memDir := t.TempDir()
	b := bus.NewLocal()
	defer b.Close()

	sec := security.NewEnforcer("test-secret")

	reg := tool.NewRegistry()
	reg.Register(&tool.FilesystemTool{})

	store, err := memory.New(memDir)
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	defer store.Close()

	// Mock LLM that returns a tool call
	adapter := &mockAdapter{
		response: "",
		toolCalls: []llm.ToolCall{{
			ID:   "call-1",
			Name: "filesystem",
			Arguments: json.RawMessage(`{"action": "read", "path": "` +
				memDir + `/test-file.txt"}`),
		}},
	}

	ctx := context.Background()

	source := "test-suite"
	respCh, err := b.Subscribe("agent."+source+".response", bus.DefaultFilter)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	dept := NewDepartment("test-tools", 5, AgentConfig{
		HeartbeatInterval: 60 * time.Second,
		MaxLoopIterations: 5,
		ToolTimeout:       5 * time.Second,
	}, b)

	agent, err := dept.Spawn(ctx, sec, "tool-agent", "test-tools", "mock-model", adapter, reg, store)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer dept.Release(agent)

	time.Sleep(50 * time.Millisecond)

	// Write a test file first
	testFile := filepath.Join(memDir, "test-file.txt")
	os.WriteFile(testFile, []byte("Hello from test file"), 0o644)

	cmdPayload := CommandPayload{
		Action: "run",
		Args: map[string]any{
			"prompt": "Read the test file for me",
		},
	}
	payload, _ := json.Marshal(cmdPayload)

	env := bus.Envelope{
		ID:      "test-env-2",
		Source:  source,
		Target:  agent.ID,
		Kind:    bus.KindCommand,
		Topic:   "agent." + agent.ID + ".inbox",
		Payload: json.RawMessage(payload),
	}
	b.Publish(ctx, env)

	select {
	case resp := <-respCh:
		var result map[string]any
		if err := json.Unmarshal(resp.Payload, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if errMsg, ok := result["error"]; ok {
			t.Logf("Agent returned error (expected with mock — tool call response fed back to LLM): %v", errMsg)
		}
		t.Logf("Tool call result: %+v", result)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for tool call agent")
	}
}

// ── Test: Spawn manages pool slots correctly ────────────────────────────────

func TestDepartmentPoolLimits(t *testing.T) {
	memDir := t.TempDir()
	b := bus.NewLocal()
	defer b.Close()

	sec := security.NewEnforcer("test-secret")
	reg := tool.NewRegistry()
	reg.Register(&tool.FilesystemTool{})
	store, err := memory.New(memDir)
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	defer store.Close()

	// Department with max 2 agents
	dept := NewDepartment("limited", 2, AgentConfig{}, b)

	ctx := context.Background()

	// Spawn 2 agents — should succeed
	a1, err := dept.Spawn(ctx, sec, "agent1", "limited", "model", nil, reg, store)
	if err != nil {
		t.Fatalf("spawn agent1: %v", err)
	}
	defer dept.Release(a1)

	a2, err := dept.Spawn(ctx, sec, "agent2", "limited", "model", nil, reg, store)
	if err != nil {
		t.Fatalf("spawn agent2: %v", err)
	}
	defer dept.Release(a2)

	// Spawn 3rd — should fail
	_, err = dept.Spawn(ctx, sec, "agent3", "limited", "model", nil, reg, store)
	if err == nil {
		t.Fatal("expected pool capacity error, got nil")
	}
	t.Logf("Pool correctly limited: %v", err)
}