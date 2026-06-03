package hook

import (
	"context"
	"errors"
	"testing"
)

func TestRegistry_RegisterAndRun(t *testing.T) {
	r := NewRegistry()
	var called bool
	r.Register(&HookFunc{
		HookType:     BeforeToolCall,
		HookPriority: PriorityNormal,
		HookName:     "test-hook",
		Fn: func(ctx context.Context, ht Type, data any) error {
			called = true
			td, ok := data.(*ToolCallCtx)
			if !ok {
				t.Error("expected *ToolCallCtx")
			}
			if td.ToolName != "test_tool" {
				t.Errorf("expected test_tool, got %s", td.ToolName)
			}
			return nil
		},
	})
	err := r.Run(context.Background(), BeforeToolCall, &ToolCallCtx{ToolName: "test_tool"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("hook was not called")
	}
}

func TestRegistry_Abort(t *testing.T) {
	r := NewRegistry()
	r.Register(&HookFunc{
		HookType:     BeforeToolCall,
		HookPriority: PriorityHigh,
		HookName:     "gate",
		Fn: func(ctx context.Context, ht Type, data any) error {
			return Abort("not allowed")
		},
	})
	err := r.Run(context.Background(), BeforeToolCall, &ToolCallCtx{ToolName: "blocked"})
	if err == nil {
		t.Error("expected abort error")
	}
	var abortErr *ErrAbort
	if !errors.As(err, &abortErr) {
		t.Error("expected ErrAbort")
	}
}

func TestRegistry_PriorityOrder(t *testing.T) {
	r := NewRegistry()
	var order []string
	r.Register(&HookFunc{
		HookType:     BeforeToolCall,
		HookPriority: PriorityLow,
		HookName:     "c",
		Fn: func(ctx context.Context, ht Type, data any) error {
			order = append(order, "c")
			return nil
		},
	})
	r.Register(&HookFunc{
		HookType:     BeforeToolCall,
		HookPriority: PriorityHigh,
		HookName:     "a",
		Fn: func(ctx context.Context, ht Type, data any) error {
			order = append(order, "a")
			return nil
		},
	})
	r.Register(&HookFunc{
		HookType:     BeforeToolCall,
		HookPriority: PriorityNormal,
		HookName:     "b",
		Fn: func(ctx context.Context, ht Type, data any) error {
			order = append(order, "b")
			return nil
		},
	})

	_ = r.Run(context.Background(), BeforeToolCall, &ToolCallCtx{})
	if len(order) != 3 || order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Errorf("wrong order: %v", order)
	}
}