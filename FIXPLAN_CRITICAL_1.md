# Fix Plan: Critical Issue #1 - Test Build Failure (agent_test.go)

**Status:** ✅ COMPLETE (Commit: b27f6b8)  
**Issue Type:** Test Build Failure / Missing Interface Implementation  
**File:** `internal/engine/agent_test.go`  
**Error Line:** 80, 203  
**Severity:** CRITICAL  

---

## Root Cause Analysis

### The Problem
```
internal/engine/agent_test.go:80:73: cannot use adapter (variable of type *mockAdapter) 
as llm.Adapter value: *mockAdapter does not implement llm.Adapter 
(missing method StreamChat)
```

### Why This Matters
1. **Build Failure:** The entire test suite cannot compile or run
2. **Untested Core Path:** The agent engine (heart of AgentForge) has ZERO test coverage
3. **Interface Mismatch:** The `llm.Adapter` interface requires 5 methods, but `mockAdapter` only implements 4
4. **Regression Risk:** Any changes to agent execution go unverified

### Root Cause Details

#### The Interface Requirement
File: `internal/llm/adapter.go:98-104`

```go
type Adapter interface {
    Provider() string
    ContextWindow() int
    Chat(ctx context.Context, req Request) (Response, error)
    StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error)  // ← MISSING
    HealthCheck(ctx context.Context) error
}
```

#### Current Mock Implementation
File: `internal/engine/agent_test.go:20-44`

```go
type mockAdapter struct {
    response  string
    toolCalls []llm.ToolCall
}

func (m *mockAdapter) Provider() string { return "mock" }
func (m *mockAdapter) Chat(_ context.Context, req llm.Request) (llm.Response, error) { ... }
func (m *mockAdapter) HealthCheck(_ context.Context) error { return nil }
func (m *mockAdapter) ContextWindow() int { return 128000 }
// ✗ StreamChat is NOT implemented
```

#### Why StreamChat Exists
File: `internal/engine/stream.go:84`

The agent engine supports token-by-token streaming via `RunStream()`:
```go
if streamSupported {
    ch, err := a.adapter.StreamChat(ctx, req)  // Called here
    // ... process channel of StreamChunk structs
    toolCalls, content, finish, u, serr := a.consumeStream(ctx, ch, cb)
}
```

**When added:** StreamChat was added to support streaming agent runs (real-time token emission for UI dashboards and long responses).

**Usage path:** `engine/stream.go` (line 84) calls `StreamChat` during `RunStream()` execution.

---

## What StreamChat Must Do

### Method Signature
```go
func (m *mockAdapter) StreamChat(ctx context.Context, req llm.Request) (<-chan llm.StreamChunk, error)
```

### Return Channel Contract
The returned channel must emit `llm.StreamChunk` structs that the agent's `consumeStream()` function expects:

```go
type StreamChunk struct {
    Model     string    `json:"model,omitempty"`
    Content   string    `json:"content,omitempty"`          // Text content
    Done      bool      `json:"done"`                       // Set true on final chunk
    Role      string    `json:"role,omitempty"`             // Message role
    Finish    string    `json:"finish,omitempty"`           // Finish reason (stop, tool_calls, etc)
    Usage     Usage     `json:"usage,omitempty"`            // Token metrics
    Error     string    `json:"error,omitempty"`            // Error message if failed
    ToolCalls []ToolCall `json:"tool_calls,omitempty"`      // Tool calls from LLM
}
```

### Consumption Pattern (from consumeStream, stream.go:184-220)
```go
for {
    select {
    case <-ctx.Done():
        return // context cancelled
    case chunk, ok := <-ch:
        if !ok {
            return  // channel closed — normal end
        }
        if chunk.Error != "" {
            return error  // stream error
        }
        // Process content, toolCalls, finish, usage
        if chunk.Done {
            return  // end of stream marker
        }
    }
}
```

---

## Test Coverage Context

### Test 1: TestAgentLoopE2E (lines 48-158)
**Purpose:** Full end-to-end agent loop with LLM response and memory persistence

**What it does:**
1. Creates temp memory directory
2. Spawns an agent via Department.Spawn()
3. Sends a command envelope to the agent
4. Waits for response on the agent's response topic
5. Verifies response payload is non-empty
6. Checks memory store was written

**Current Status:** BLOCKED — mockAdapter not valid

**What it tests:** Core agent command → LLM → response → memory pipeline

### Test 2: TestAgentToolCallE2E (lines 162-246)
**Purpose:** Agent handles tool calls returned by LLM

**What it does:**
1. Creates mockAdapter that returns a tool call in response
2. Sends "Read the test file" prompt
3. Agent should invoke the tool via registry
4. Tool execution result fed back to LLM

**Current Status:** BLOCKED — mockAdapter not valid

**What it tests:** Tool execution pipeline + agent loop iteration

### Test 3: TestDepartmentPoolLimits (lines 250-288)
**Purpose:** Department correctly manages agent pool size limits

**What it does:**
1. Creates Department with max 2 agents
2. Spawns 2 agents (should succeed)
3. Spawns 3rd agent (should fail with capacity error)

**Current Status:** BLOCKED by agent_test.go not compiling (cascading failure)

**What it tests:** Agent pool resource management

---

## Design of StreamChat Mock

### Strategy
Implement a **simple, non-blocking** mock that:
1. Does NOT use actual goroutines for streaming
2. Returns a pre-populated channel with mock chunks
3. Follows the same pattern as real adapters but with test data

### Implementation Approach

```go
func (m *mockAdapter) StreamChat(ctx context.Context, req llm.Request) (<-chan llm.StreamChunk, error) {
    out := make(chan llm.StreamChunk, 10)  // Buffered to avoid blocking
    
    // Simulate response as chunks (could be single chunk or multiple)
    go func() {
        defer close(out)
        
        // Emit content chunk
        if m.response != "" {
            out <- llm.StreamChunk{
                Content: m.response,
                Role:    "assistant",
            }
        }
        
        // Emit tool calls if configured
        if len(m.toolCalls) > 0 {
            out <- llm.StreamChunk{
                ToolCalls: m.toolCalls,
            }
        }
        
        // Final chunk with metadata
        out <- llm.StreamChunk{
            Done:   true,
            Model:  req.Model,
            Finish: "stop",
            Usage: llm.Usage{
                PromptTokens:     10,
                CompletionTokens: 20,
                TotalTokens:      30,
            },
        }
    }()
    
    return out, nil
}
```

### Why This Design
1. **Matches Chat() behavior:** Returns same response data as `Chat()` method
2. **Proper channel cleanup:** `defer close(out)` ensures no panic on receive after close
3. **Buffered channel:** Prevents goroutine deadlock if consumer is slow
4. **Goroutine safety:** Spawned goroutine exits cleanly when channel drains
5. **Context-aware:** Uses request context (though not actively cancelling for simplicity in mock)
6. **Matches real adapters:** Pattern similar to OpenAI/Ollama/Anthropic StreamChat implementations

### Potential Refinement: Context Handling
Could add context cancellation support:
```go
go func() {
    defer close(out)
    select {
    case <-ctx.Done():
        out <- llm.StreamChunk{Error: "cancelled", Done: true}
        return
    default:
    }
    // ... emit chunks
}()
```

But for initial fix, simple version is sufficient for tests.

---

## Before & After Comparison

### Before (Current - Won't Compile)
```go
type mockAdapter struct {
    response  string
    toolCalls []llm.ToolCall
}

func (m *mockAdapter) Provider() string { return "mock" }
func (m *mockAdapter) Chat(...) (llm.Response, error) { ... }
func (m *mockAdapter) HealthCheck(...) error { return nil }
func (m *mockAdapter) ContextWindow() int { return 128000 }
// Missing StreamChat → COMPILE ERROR
```

### After (Will Compile)
```go
type mockAdapter struct {
    response  string
    toolCalls []llm.ToolCall
}

func (m *mockAdapter) Provider() string { return "mock" }
func (m *mockAdapter) Chat(...) (llm.Response, error) { ... }
func (m *mockAdapter) HealthCheck(...) error { return nil }
func (m *mockAdapter) ContextWindow() int { return 128000 }
func (m *mockAdapter) StreamChat(ctx context.Context, req llm.Request) (<-chan llm.StreamChunk, error) {
    // Implementation here
}
```

---

## Verification Plan

After implementing, verify:

1. **Compilation:** `go test ./internal/engine -compile` passes
2. **All 3 tests compile:** No type errors
3. **Test execution:** `go test ./internal/engine -v` runs all 3 tests
4. **Test pass rate:** At minimum, TestDepartmentPoolLimits (independent of StreamChat) should pass
5. **No panics:** E2E tests should not panic on channel operations
6. **Channel cleanup:** No goroutine leaks (channel properly closed)

---

## Integration with Remaining Fixes

This fix **unblocks**:
- Test coverage in engine package
- Ability to write regression tests for Fixes #2-4
- CI/CD pipeline (current build fails)

This fix is **independent of**:
- Fix #2 (shell injection) - different module
- Fix #3 (pipe error handling) - different module  
- Fix #4 (glob pattern matching) - different module

But tests written for Fixes #2-4 will rely on this being completed first.

---

## Files to Modify

| File | Lines | Change | Type |
|------|-------|--------|------|
| `internal/engine/agent_test.go` | After line 44 | Add StreamChat method | New method |

---

## Risk Assessment

**Risk Level:** VERY LOW

- **Non-breaking:** Adding method doesn't change existing behavior
- **Well-defined contract:** llm.Adapter interface is stable
- **No dependencies:** Mock is internal to tests only
- **Reversible:** Can always simplify/expand implementation
- **Isolated:** Changes only to test file

---

## Commit Message

```
Fix: Implement StreamChat in mockAdapter for agent_test.go

Add missing StreamChat method to mockAdapter to implement the full
llm.Adapter interface. This unblocks the agent engine test suite which
currently fails to compile.

Fixes: agent_test.go:80,203 build failure
Test coverage: Enables TestAgentLoopE2E, TestAgentToolCallE2E tests
```

