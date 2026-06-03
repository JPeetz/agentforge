// Package hook — lifecycle hook system for AgentForge.
// Hooks fire around tool calls, LLM calls, and session events.
// They are pluggable — any agent/system can register hooks that
// observe, modify, or short-circuit pipeline stages.
package hook

import (
	"context"
	"fmt"
	"sync"
)

// Type identifies which lifecycle event a hook fires on.
type Type string

const (
	BeforeToolCall   Type = "before_tool_call"
	AfterToolCall    Type = "after_tool_call"
	OnToolResult     Type = "on_tool_result"
	BeforeLLMCall    Type = "before_llm_call"
	AfterLLMCall     Type = "after_llm_call"
	OnSessionCompact Type = "on_session_compact"
	OnAgentSpawn     Type = "on_agent_spawn"
	OnAgentShutdown  Type = "on_agent_shutdown"
)

// Priority defines hook execution order. Lower values run first.
type Priority int

const (
	PriorityHigh   Priority = 0
	PriorityNormal Priority = 50
	PriorityLow    Priority = 100
)

// ToolCallCtx carries data for tool-call hook events.
type ToolCallCtx struct {
	AgentID    string         `json:"agentId"`
	AgentName  string         `json:"agentName"`
	ToolName   string         `json:"toolName"`
	ToolArgs   map[string]any `json:"toolArgs"`
	ToolResult map[string]any `json:"toolResult,omitempty"`
	ForLLM     string         `json:"forLLM,omitempty"`
	ForUser    string         `json:"forUser,omitempty"`
	Duration   string         `json:"duration,omitempty"`
	Err        string         `json:"err,omitempty"`
}

// LLMCallCtx carries data for LLM-call hook events.
type LLMCallCtx struct {
	AgentID      string  `json:"agentId"`
	AgentName    string  `json:"agentName"`
	Model        string  `json:"model"`
	Messages     int     `json:"messages"`
	MaxTokens    int     `json:"maxTokens"`
	Temperature  float64 `json:"temperature"`
	Response     string  `json:"response,omitempty"`
	PromptTokens int     `json:"promptTokens,omitempty"`
	CompTokens   int     `json:"compTokens,omitempty"`
	Duration     string  `json:"duration,omitempty"`
	FinishReason string  `json:"finishReason,omitempty"`
	Err          string  `json:"err,omitempty"`
}

// SessionCtx carries data for session-compaction hook events.
type SessionCtx struct {
	AgentID         string `json:"agentId"`
	AgentName       string `json:"agentName"`
	TokensBefore    int    `json:"tokensBefore"`
	TokensAfter     int    `json:"tokensAfter"`
	CompactionCount int    `json:"compactionCount"`
	FlushPath       string `json:"flushPath,omitempty"`
}

// AgentSpawnCtx carries data for agent spawn/shutdown events.
type AgentSpawnCtx struct {
	AgentID    string `json:"agentId"`
	AgentName  string `json:"agentName"`
	Department string `json:"department"`
	Model      string `json:"model"`
}

// Hook is a function that receives typed context and can observe, log, modify, or abort.
// Returning an error aborts the current operation.
type Hook interface {
	Type() Type
	Priority() Priority
	Name() string
	Run(ctx context.Context, t Type, data any) error
}

// HookFunc is an adapter that allows plain functions to implement the Hook interface.
type HookFunc struct {
	HookType     Type
	HookPriority Priority
	HookName     string
	Fn           func(ctx context.Context, t Type, data any) error
}

func (h *HookFunc) Type() Type           { return h.HookType }
func (h *HookFunc) Priority() Priority    { return h.HookPriority }
func (h *HookFunc) Name() string         { return h.HookName }
func (h *HookFunc) Run(ctx context.Context, t Type, data any) error {
	return h.Fn(ctx, t, data)
}

// Registry stores and runs hooks grouped by type.
type Registry struct {
	mu    sync.RWMutex
	hooks map[Type][]Hook
	stats map[Type]*HookStats
}

// HookStats tracks runtime metrics for a hook type.
type HookStats struct {
	Total  int64
	Errors int64
	Aborts int64
}

// NewRegistry creates an empty hook registry.
func NewRegistry() *Registry {
	return &Registry{
		hooks: make(map[Type][]Hook),
		stats: make(map[Type]*HookStats),
	}
}

// Register adds a hook. Hooks are sorted by priority within each type.
func (r *Registry) Register(h Hook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := h.Type()
	r.hooks[t] = append(r.hooks[t], h)
	sortByPriority(r.hooks[t])
	if _, ok := r.stats[t]; !ok {
		r.stats[t] = &HookStats{}
	}
}

// Run executes all hooks for the given type in priority order.
// Returns the first abort error that should stop the operation, or nil.
func (r *Registry) Run(ctx context.Context, t Type, data any) error {
	r.mu.RLock()
	hooks := r.hooks[t]
	r.mu.RUnlock()

	for _, h := range hooks {
		r.recordStart(t)
		err := h.Run(ctx, t, data)
		if err != nil {
			r.recordError(t)
			if isAbort(err) {
				r.recordAbort(t)
				return err
			}
			fmt.Printf("hook %s/%s: error (non-abort): %v\n", t, h.Name(), err)
		}
	}
	return nil
}

// Stats returns cumulative hook statistics.
func (r *Registry) Stats() map[Type]HookStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[Type]HookStats)
	for t, s := range r.stats {
		out[t] = *s
	}
	return out
}

func (r *Registry) recordStart(t Type) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.stats[t]; ok {
		s.Total++
	}
}

func (r *Registry) recordError(t Type) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.stats[t]; ok {
		s.Errors++
	}
}

func (r *Registry) recordAbort(t Type) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.stats[t]; ok {
		s.Aborts++
	}
}

func sortByPriority(hooks []Hook) {
	for i := 0; i < len(hooks); i++ {
		for j := i + 1; j < len(hooks); j++ {
			if hooks[i].Priority() > hooks[j].Priority() {
				hooks[i], hooks[j] = hooks[j], hooks[i]
			}
		}
	}
}

// ErrAbort is returned by hooks that want to stop the current operation.
type ErrAbort struct {
	Reason string
}

func (e *ErrAbort) Error() string { return fmt.Sprintf("aborted: %s", e.Reason) }

func isAbort(err error) bool {
	_, ok := err.(*ErrAbort)
	return ok
}

// Abort returns an error that stops the current operation.
func Abort(reason string) error {
	return &ErrAbort{Reason: reason}
}