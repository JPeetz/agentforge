package cost

import (
	"math"
	"sync"
	"testing"

	"github.com/agentforge/agentforge/internal/llm"
)

func TestLookupPrice_OpenAIGPT4o(t *testing.T) {
	p := lookupPrice("gpt-4o")
	if p.Provider != "openai" {
		t.Errorf("expected openai, got %s", p.Provider)
	}
	if p.InputPrice != 2.5 {
		t.Errorf("expected $2.5/M input, got $%.2f/M", p.InputPrice)
	}
	if p.OutputPrice != 10.0 {
		t.Errorf("expected $10.0/M output, got $%.2f/M", p.OutputPrice)
	}
	if p.Free {
		t.Error("gpt-4o should not be free")
	}
}

func TestLookupPrice_GPT4oMini(t *testing.T) {
	p := lookupPrice("gpt-4o-mini")
	if p.InputPrice != 0.15 || p.OutputPrice != 0.60 {
		t.Errorf("gpt-4o-mini: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_ClaudeSonnet4(t *testing.T) {
	p := lookupPrice("claude-sonnet-4-20250514")
	if p.Provider != "anthropic" {
		t.Errorf("expected anthropic, got %s", p.Provider)
	}
	if p.InputPrice != 3.0 || p.OutputPrice != 15.0 {
		t.Errorf("claude-sonnet-4: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_ClaudeOpus4(t *testing.T) {
	p := lookupPrice("claude-opus-4-20250514")
	if p.InputPrice != 15.0 || p.OutputPrice != 75.0 {
		t.Errorf("claude-opus-4: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_O3(t *testing.T) {
	p := lookupPrice("o3")
	if p.InputPrice != 10.0 || p.OutputPrice != 40.0 {
		t.Errorf("o3: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_O4Mini(t *testing.T) {
	p := lookupPrice("o4-mini")
	if p.InputPrice != 1.1 || p.OutputPrice != 4.4 {
		t.Errorf("o4-mini: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_O1(t *testing.T) {
	p := lookupPrice("o1")
	if p.InputPrice != 15.0 || p.OutputPrice != 60.0 {
		t.Errorf("o1: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_DeepSeekChat(t *testing.T) {
	p := lookupPrice("deepseek-chat")
	if p.Provider != "deepseek" {
		t.Errorf("expected deepseek, got %s", p.Provider)
	}
	if p.InputPrice != 0.27 || p.OutputPrice != 1.10 {
		t.Errorf("deepseek-chat: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_DeepSeekReasoner(t *testing.T) {
	p := lookupPrice("deepseek-reasoner")
	if p.InputPrice != 0.55 || p.OutputPrice != 2.19 {
		t.Errorf("deepseek-reasoner: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_Gemini25Pro(t *testing.T) {
	p := lookupPrice("gemini-2.5-pro")
	if p.Provider != "google" {
		t.Errorf("expected google, got %s", p.Provider)
	}
	if p.InputPrice != 1.25 || p.OutputPrice != 5.0 {
		t.Errorf("gemini-2.5-pro: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_Gemini25Flash(t *testing.T) {
	p := lookupPrice("gemini-2.5-flash")
	if p.InputPrice != 0.15 || p.OutputPrice != 0.60 {
		t.Errorf("gemini-2.5-flash: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_Ollama(t *testing.T) {
	p := lookupPrice("ollama/llama3:latest")
	if !p.Free {
		t.Error("ollama should be free")
	}
	if p.Provider != "ollama" {
		t.Errorf("expected ollama, got %s", p.Provider)
	}
}

func TestLookupPrice_Gemma(t *testing.T) {
	p := lookupPrice("gemma3:27b")
	if !p.Free {
		t.Error("gemma should be free")
	}
}

func TestLookupPrice_OpenRouter(t *testing.T) {
	p := lookupPrice("openai/gpt-4o")
	if p.Provider != "openrouter" {
		t.Errorf("expected openrouter, got %s", p.Provider)
	}
	if p.InputPrice != 2.5 || p.OutputPrice != 10.0 {
		t.Errorf("openrouter openai/gpt-4o: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_OpenRouterAnthropic(t *testing.T) {
	p := lookupPrice("anthropic/claude-sonnet-4")
	if p.Provider != "openrouter" {
		t.Errorf("expected openrouter, got %s", p.Provider)
	}
	if p.InputPrice != 3.0 || p.OutputPrice != 15.0 {
		t.Errorf("openrouter anthropic/claude-sonnet-4: got $%.2f/$%.2f", p.InputPrice, p.OutputPrice)
	}
}

func TestLookupPrice_OpenRouterUnknownModel(t *testing.T) {
	// An unknown model under openrouter/ should fall back to reasonable pricing
	p := lookupPrice("openrouter/some-unknown-model-v2")
	if p.Provider != "openrouter" {
		t.Errorf("expected openrouter, got %s", p.Provider)
	}
	// Should fall back to a reasonable default
	if p.InputPrice <= 0 || p.OutputPrice <= 0 {
		t.Error("unknown openrouter model should have non-zero fallback pricing")
	}
}

func TestLookupPrice_UnknownModel_Fallback(t *testing.T) {
	p := lookupPrice("some-random-model-name")
	if p.Provider != "unknown" {
		t.Errorf("expected unknown, got %s", p.Provider)
	}
	// Conservative default should be non-zero
	if p.InputPrice <= 0 {
		t.Error("unknown model fallback should have non-zero input price")
	}
}

func TestRecord_BasicCost(t *testing.T) {
	tracker := NewTracker()

	// gpt-4o: $2.5/M input, $10/M output
	// 1000 input + 500 output = $0.0025 + $0.005 = $0.0075
	usage := llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}
	cost := tracker.Record("gpt-4o", usage)

	expected := float64(1000)*2.5/1_000_000 + float64(500)*10.0/1_000_000 // 0.0025 + 0.005
	if math.Abs(cost-expected) > 0.0001 {
		t.Errorf("cost: got $%.6f, want $%.6f", cost, expected)
	}

	total := tracker.GetTotalCost()
	if math.Abs(total-expected) > 0.0001 {
		t.Errorf("total cost: got $%.6f, want $%.6f", total, expected)
	}
}

func TestRecord_CachedTokens_OpenAI(t *testing.T) {
	tracker := NewTracker()

	// gpt-4o with cached input: 50% discount on cached tokens
	// 1000 input (500 cached) + 500 output
	// = 500 * 2.5/1M + 500 * 2.5/1M * 0.5 + 500 * 10/1M
	// = 0.00125 + 0.000625 + 0.005 = 0.006875
	usage := llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}
	cost := tracker.RecordWithCached("gpt-4o", usage, 500, "test-session")

	expected := 500*2.5/1_000_000 + 500*2.5*0.5/1_000_000 + 500*10.0/1_000_000
	if math.Abs(cost-expected) > 0.0001 {
		t.Errorf("cached openai cost: got $%.6f, want $%.6f", cost, expected)
	}

	// Verify session tracking
	session, ok := tracker.GetSessionCost("test-session")
	if !ok {
		t.Fatal("session not found")
	}
	if session.SessionID != "test-session" {
		t.Errorf("session ID: got %s", session.SessionID)
	}
	if math.Abs(session.TotalCost-expected) > 0.0001 {
		t.Errorf("session cost: got $%.6f, want $%.6f", session.TotalCost, expected)
	}
}

func TestRecord_CachedTokens_Anthropic(t *testing.T) {
	tracker := NewTracker()

	// claude-sonnet-4: $3/M input, $15/M output, 10% cached discount
	// 1000 input (500 cached) + 500 output
	// = 500 * 3/1M + 500 * 3/1M * 0.1 + 500 * 15/1M
	// = 0.0015 + 0.00015 + 0.0075 = 0.00915
	usage := llm.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}
	cost := tracker.RecordWithCached("claude-sonnet-4", usage, 500, "")

	expected := 500*3.0/1_000_000 + 500*3.0*0.1/1_000_000 + 500*15.0/1_000_000
	if math.Abs(cost-expected) > 0.0001 {
		t.Errorf("cached anthropic cost: got $%.6f, want $%.6f", cost, expected)
	}
}

func TestRecord_Ollama_Free(t *testing.T) {
	tracker := NewTracker()

	usage := llm.Usage{
		PromptTokens:     10000,
		CompletionTokens: 5000,
		TotalTokens:      15000,
	}
	cost := tracker.Record("ollama/llama3:latest", usage)
	if cost != 0 {
		t.Errorf("ollama cost should be 0, got $%.6f", cost)
	}

	total := tracker.GetTotalCost()
	if total != 0 {
		t.Errorf("total cost with ollama should be 0, got $%.6f", total)
	}
}

func TestRecord_ZeroUsage(t *testing.T) {
	tracker := NewTracker()
	cost := tracker.Record("gpt-4o", llm.Usage{})
	if cost != 0 {
		t.Errorf("zero usage should cost 0, got $%.6f", cost)
	}
}

func TestGetTotalTokens(t *testing.T) {
	tracker := NewTracker()

	tracker.Record("gpt-4o", llm.Usage{
		PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150,
	})
	tracker.Record("gpt-4o-mini", llm.Usage{
		PromptTokens: 200, CompletionTokens: 100, TotalTokens: 300,
	})

	tokens := tracker.GetTotalTokens()
	if tokens.PromptTokens != 300 {
		t.Errorf("prompt tokens: got %d, want 300", tokens.PromptTokens)
	}
	if tokens.CompletionTokens != 150 {
		t.Errorf("completion tokens: got %d, want 150", tokens.CompletionTokens)
	}
	if tokens.TotalTokens != 450 {
		t.Errorf("total tokens: got %d, want 450", tokens.TotalTokens)
	}
	if tokens.CallCount != 2 {
		t.Errorf("call count: got %d, want 2", tokens.CallCount)
	}
}

func TestGetDailyCosts(t *testing.T) {
	tracker := NewTracker()

	tracker.Record("gpt-4o", llm.Usage{
		PromptTokens: 1_000_000, CompletionTokens: 500_000, TotalTokens: 1_500_000,
	})

	daily := tracker.GetDailyCosts(7)
	if len(daily) != 1 {
		t.Fatalf("expected 1 day, got %d", len(daily))
	}
	if daily[0].TotalCost <= 0 {
		t.Errorf("expected non-zero daily cost")
	}
	if daily[0].TokenCounts.CallCount != 1 {
		t.Errorf("daily call count: got %d", daily[0].TokenCounts.CallCount)
	}
}

func TestGetProviderCosts(t *testing.T) {
	tracker := NewTracker()

	tracker.Record("gpt-4o", llm.Usage{
		PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500,
	})
	tracker.Record("claude-sonnet-4", llm.Usage{
		PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500,
	})

	providers := tracker.GetProviderCosts()
	if len(providers) < 2 {
		t.Fatalf("expected at least 2 providers, got %d", len(providers))
	}

	// Should be sorted by cost descending (most expensive first)
	if providers[0].TotalCost < providers[len(providers)-1].TotalCost {
		t.Error("provider costs not sorted descending")
	}
}

func TestGetModelCosts(t *testing.T) {
	tracker := NewTracker()

	tracker.Record("gpt-4o", llm.Usage{
		PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500,
	})
	tracker.Record("gpt-4o-mini", llm.Usage{
		PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500,
	})

	models := tracker.GetModelCosts()
	if len(models) < 2 {
		t.Fatalf("expected at least 2 models, got %d", len(models))
	}

	// Verify both models exist and gpt-4o costs more than mini for same usage
	var cost4o, costMini float64
	for _, m := range models {
		switch m.Model {
		case "gpt-4o":
			cost4o = m.TotalCost
		case "gpt-4o-mini":
			costMini = m.TotalCost
		}
	}
	if cost4o <= 0 {
		t.Error("gpt-4o cost should be > 0")
	}
	if costMini <= 0 {
		t.Error("gpt-4o-mini cost should be > 0")
	}
	if cost4o <= costMini {
		t.Errorf("gpt-4o ($%.6f) should cost more than gpt-4o-mini ($%.6f)", cost4o, costMini)
	}
}

func TestBudgetStatus_Wildcard(t *testing.T) {
	tracker := NewTracker()

	tracker.Record("gpt-4o", llm.Usage{
		PromptTokens: 1_000_000, CompletionTokens: 1_000_000, TotalTokens: 2_000_000,
	})

	// Set a budget of $20
	tracker.SetBudget("*", 20.0)

	spent, limit, percent, level := tracker.BudgetStatus("*")
	if limit != 20.0 {
		t.Errorf("budget limit: got $%.2f", limit)
	}

	// gpt-4o: $2.5/M in + $10/M out = $12.50
	// percent = 12.5/20 * 100 = 62.5%
	if spent <= 0 {
		t.Error("spent should be > 0")
	}
	t.Logf("Budget: spent=$%.4f, limit=$%.2f, pct=%.1f%%, level=%s", spent, limit, percent, level)
}

func TestBudgetStatus_SpecificModel(t *testing.T) {
	tracker := NewTracker()

	tracker.Record("gpt-4o", llm.Usage{
		PromptTokens: 10_000_000, CompletionTokens: 1_000_000, TotalTokens: 11_000_000,
	})

	tracker.SetBudget("gpt-4o", 10.0)
	spent, limit, percent, level := tracker.BudgetStatus("gpt-4o")

	// 10M input * $2.5/M + 1M output * $10/M = $25 + $10 = $35
	// 35/10 = 350% → block
	if level != "block" {
		t.Errorf("expected block, got %s (pct=%.1f%%)", level, percent)
	}
	t.Logf("Model budget: spent=$%.4f, limit=$%.2f, pct=%.1f%%, level=%s", spent, limit, percent, level)
}

func TestBudgetStatus_NoBudget(t *testing.T) {
	tracker := NewTracker()

	tracker.Record("gpt-4o", llm.Usage{
		PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500,
	})

	_, _, _, level := tracker.BudgetStatus("gpt-4o")
	if level != "ok" {
		t.Errorf("no budget should be ok, got %s", level)
	}
}

func TestGetRecentCalls(t *testing.T) {
	tracker := NewTracker()

	for i := 0; i < 5; i++ {
		tracker.Record("gpt-4o-mini", llm.Usage{
			PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150,
		})
	}

	calls := tracker.GetRecentCalls(3)
	if len(calls) != 3 {
		t.Fatalf("expected 3 recent calls, got %d", len(calls))
	}
	if calls[0].CallCost <= 0 {
		t.Error("recent call should have non-zero cost")
	}
}

func TestGetRecentCalls_Empty(t *testing.T) {
	tracker := NewTracker()
	calls := tracker.GetRecentCalls(10)
	if len(calls) != 0 {
		t.Errorf("empty tracker should return 0 calls, got %d", len(calls))
	}
}

func TestOverview(t *testing.T) {
	tracker := NewTracker()

	tracker.Record("gpt-4o", llm.Usage{
		PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500,
	})
	tracker.Record("gpt-4o-mini", llm.Usage{
		PromptTokens: 2000, CompletionTokens: 1000, TotalTokens: 3000,
	})

	ov := tracker.Overview()

	if ov.TotalCost <= 0 {
		t.Error("overview total cost should be > 0")
	}
	if ov.TotalTokens.CallCount != 2 {
		t.Errorf("overview call count: got %d", ov.TotalTokens.CallCount)
	}
	if len(ov.Models) < 2 {
		t.Errorf("overview should have at least 2 models, got %d", len(ov.Models))
	}
	if len(ov.Providers) < 1 {
		t.Error("overview should have at least 1 provider")
	}
	if len(ov.Daily) != 1 {
		t.Errorf("overview should have 1 daily entry, got %d", len(ov.Daily))
	}

	// Verify JSON serialization
	json := tracker.ToOverviewJSON()
	if len(json) == 0 {
		t.Error("ToOverviewJSON returned empty")
	}

	fullJSON := tracker.ToJSON()
	if len(fullJSON) == 0 {
		t.Error("ToJSON returned empty")
	}
}

func TestSessionCost(t *testing.T) {
	tracker := NewTracker()

	// Record calls for different sessions
	tracker.RecordWithCached("gpt-4o", llm.Usage{
		PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500,
	}, 0, "session-1")

	tracker.RecordWithCached("gpt-4o-mini", llm.Usage{
		PromptTokens: 2000, CompletionTokens: 1000, TotalTokens: 3000,
	}, 0, "session-1")

	tracker.RecordWithCached("claude-sonnet-4", llm.Usage{
		PromptTokens: 500, CompletionTokens: 200, TotalTokens: 700,
	}, 0, "session-2")

	// Session 1 should have 2 models
	s1, ok := tracker.GetSessionCost("session-1")
	if !ok {
		t.Fatal("session-1 not found")
	}
	if s1.TokenCounts.CallCount != 2 {
		t.Errorf("session-1 call count: got %d", s1.TokenCounts.CallCount)
	}
	if len(s1.ModelCosts) != 2 {
		t.Errorf("session-1 model count: got %d", len(s1.ModelCosts))
	}

	// Session 2
	s2, ok := tracker.GetSessionCost("session-2")
	if !ok {
		t.Fatal("session-2 not found")
	}
	if s2.TokenCounts.CallCount != 1 {
		t.Errorf("session-2 call count: got %d", s2.TokenCounts.CallCount)
	}

	// Non-existent session
	_, ok = tracker.GetSessionCost("nonexistent")
	if ok {
		t.Error("nonexistent session should not be found")
	}
}

func TestFormatUSD(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{0, "$0.00"},
		{0.001, "$0.0010"},
		{0.01, "$0.010"},
		{0.09, "$0.090"},
		{0.10, "$0.10"},
		{0.123, "$0.12"},
		{1.0, "$1.00"},
		{10.5, "$10.50"},
		{100.0, "$100.00"},
	}

	for _, tt := range tests {
		result := FormatUSD(tt.cost)
		if result != tt.expected {
			t.Errorf("FormatUSD(%.4f): got %s, want %s", tt.cost, result, tt.expected)
		}
	}
}

func TestCacheDiscountPct(t *testing.T) {
	tests := []struct {
		provider string
		expected int
	}{
		{"openai", 50},
		{"OpenAI", 50},
		{"openrouter", 50},
		{"OpenRouter", 50},
		{"anthropic", 10},
		{"Anthropic", 10},
		{"google", 0},
		{"ollama", 0},
		{"unknown", 0},
	}

	for _, tt := range tests {
		result := CacheDiscountPct(tt.provider)
		if result != tt.expected {
			t.Errorf("CacheDiscountPct(%s): got %d, want %d", tt.provider, result, tt.expected)
		}
	}
}

// TestConcurrentRecording ensures the tracker is goroutine-safe.
func TestConcurrentRecording(t *testing.T) {
	tracker := NewTracker()

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			model := "gpt-4o"
			if i%2 == 0 {
				model = "gpt-4o-mini"
			}
			tracker.Record(model, llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			})
			tracker.RecordWithCached("claude-sonnet-4", llm.Usage{
				PromptTokens:     200,
				CompletionTokens: 100,
				TotalTokens:      300,
			}, 50, "shared-session")
		}(i)
	}
	wg.Wait()

	tokens := tracker.GetTotalTokens()
	if tokens.CallCount != int64(iterations*2) {
		t.Errorf("concurrent call count: got %d, want %d", tokens.CallCount, iterations*2)
	}

	total := tracker.GetTotalCost()
	if total <= 0 {
		t.Error("concurrent total cost should be > 0")
	}

	// Verify session tracking survived concurrency
	session, ok := tracker.GetSessionCost("shared-session")
	if !ok {
		t.Fatal("shared session not found after concurrent recording")
	}
	if session.TokenCounts.CallCount != int64(iterations) {
		t.Errorf("concurrent session call count: got %d, want %d", session.TokenCounts.CallCount, iterations)
	}

	// Verify daily tracking
	daily := tracker.GetDailyCosts(1)
	if len(daily) != 1 || daily[0].TokenCounts.CallCount != int64(iterations*2) {
		t.Errorf("concurrent daily: len=%d callCount=%d", len(daily), daily[0].TokenCounts.CallCount)
	}
}

func TestLookupPrice_CaseInsensitive(t *testing.T) {
	p1 := lookupPrice("GPT-4O")
	p2 := lookupPrice("gpt-4o")
	if p1.InputPrice != p2.InputPrice || p1.OutputPrice != p2.OutputPrice {
		t.Error("price lookup should be case-insensitive")
	}
}

func TestLookupPrice_GPT41(t *testing.T) {
	p := lookupPrice("gpt-4.1")
	if p.InputPrice != 2.0 || p.OutputPrice != 8.0 {
		t.Errorf("gpt-4.1: got $%.2f/$%.2f, want $2.00/$8.00", p.InputPrice, p.OutputPrice)
	}
}