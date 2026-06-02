// Package cost — real cost tracking for LLM token usage.
//
// Accumulates token usage per call, session, day, provider, and model.
// Uses published pricing tables (per 1M tokens) for all major models.
// Thread-safe via sync.Mutex. Supports cached-token discounts and budget alerts.
package cost

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/agentforge/internal/llm"
)

// ── Pricing ──────────────────────────────────────────────────────────────────

// modelPrice holds input/output pricing per 1M tokens for a model.
type modelPrice struct {
	InputPrice  float64 // USD per 1M input tokens
	OutputPrice float64 // USD per 1M output tokens
	Provider    string
	Free        bool // e.g. Ollama — local, no API cost
}

// pricingTable maps model identifier patterns (prefix match) → price.
// MUST be ordered: more specific/longer patterns first to avoid collisions.
// Example: "gpt-4o-mini" before "gpt-4o" so "gpt-4o-mini" doesn't match the shorter prefix.
var pricingTable = []struct {
	pattern string
	price   modelPrice
}{
	// OpenAI — GPT-4o-mini: $0.15/$0.60 (must come before gpt-4o)
	{"gpt-4o-mini", modelPrice{0.15, 0.60, "openai", false}},
	// OpenAI — GPT-4.1: $2/$8 per 1M input/output
	{"gpt-4.1", modelPrice{2.0, 8.0, "openai", false}},
	// OpenAI — GPT-4o: $2.5/$10
	{"gpt-4o", modelPrice{2.5, 10.0, "openai", false}},
	// OpenAI — o3: $10/$40
	{"o3", modelPrice{10.0, 40.0, "openai", false}},
	// OpenAI — o4-mini: $1.1/$4.4
	{"o4-mini", modelPrice{1.1, 4.4, "openai", false}},
	// OpenAI — o1: $15/$60
	{"o1", modelPrice{15.0, 60.0, "openai", false}},
	// OpenAI — o1-mini: $3/$12
	{"o1-mini", modelPrice{3.0, 12.0, "openai", false}},
	// OpenAI — GPT-4: $30/$60
	{"gpt-4", modelPrice{30.0, 60.0, "openai", false}},
	// OpenAI — GPT-4-turbo: $10/$30
	{"gpt-4-turbo", modelPrice{10.0, 30.0, "openai", false}},
	// OpenAI — GPT-4-32k: $60/$120
	{"gpt-4-32k", modelPrice{60.0, 120.0, "openai", false}},
	// OpenAI — GPT-3.5-turbo: $0.50/$1.50
	{"gpt-3.5-turbo", modelPrice{0.5, 1.5, "openai", false}},
	// OpenAI — GPT-3.5-turbo-16k: $3/$4
	{"gpt-3.5-turbo-16k", modelPrice{3.0, 4.0, "openai", false}},

	// Anthropic — Claude Sonnet 4: $3/$15
	{"claude-sonnet-4", modelPrice{3.0, 15.0, "anthropic", false}},
	// Anthropic — Claude Opus 4: $15/$75
	{"claude-opus-4", modelPrice{15.0, 75.0, "anthropic", false}},
	// Anthropic — Claude 3.5 Sonnet: $3/$15
	{"claude-3.5-sonnet", modelPrice{3.0, 15.0, "anthropic", false}},
	// Anthropic — Claude 3.5 Sonnet v2 placeholder
	{"claude-3-5-sonnet", modelPrice{3.0, 15.0, "anthropic", false}},
	// Anthropic — Claude 3.5 Haiku: $1/$5
	{"claude-3.5-haiku", modelPrice{1.0, 5.0, "anthropic", false}},
	// Anthropic — Claude 3.5 Haiku v2
	{"claude-3-5-haiku", modelPrice{1.0, 5.0, "anthropic", false}},
	// Anthropic — Claude 3 Opus: $15/$75
	{"claude-3-opus", modelPrice{15.0, 75.0, "anthropic", false}},
	// Anthropic — Claude 3 Sonnet: $3/$15
	{"claude-3-sonnet", modelPrice{3.0, 15.0, "anthropic", false}},
	// Anthropic — Claude 3 Haiku: $0.25/$1.25
	{"claude-3-haiku", modelPrice{0.25, 1.25, "anthropic", false}},
	// Anthropic — Claude 4 Opus
	{"claude-opus-4", modelPrice{15.0, 75.0, "anthropic", false}},
	// Anthropic — Claude 4 Sonnet
	{"claude-sonnet-4", modelPrice{3.0, 15.0, "anthropic", false}},
	// Anthropic — Claude Opus 4.1
	{"claude-opus-4-1", modelPrice{15.0, 75.0, "anthropic", false}},
	// Anthropic — Claude Sonnet 4.5
	{"claude-sonnet-4-5", modelPrice{3.0, 15.0, "anthropic", false}},

	// DeepSeek — deepseek-chat (V3): $0.27/$1.10
	{"deepseek-chat", modelPrice{0.27, 1.10, "deepseek", false}},
	// DeepSeek — deepseek-reasoner (R1): $0.55/$2.19
	{"deepseek-reasoner", modelPrice{0.55, 2.19, "deepseek", false}},
	// DeepSeek — generic fallback
	{"deepseek", modelPrice{0.27, 1.10, "deepseek", false}},

	// Gemini — Gemini 2.5 Pro: $1.25/$5
	{"gemini-2.5-pro", modelPrice{1.25, 5.0, "google", false}},
	// Gemini — Gemini 2.5 Flash: $0.15/$0.60
	{"gemini-2.5-flash", modelPrice{0.15, 0.60, "google", false}},
	// Gemini — Gemini 2.0 Flash: $0.10/$0.40
	{"gemini-2.0-flash", modelPrice{0.10, 0.40, "google", false}},
	// Gemini — Gemini 1.5 Pro: $1.25/$5 (text >128K)
	{"gemini-1.5-pro", modelPrice{1.25, 5.0, "google", false}},
	// Gemini — Gemini 1.5 Flash: $0.075/$0.30
	{"gemini-1.5-flash", modelPrice{0.075, 0.30, "google", false}},
	// Gemini — generic fallback
	{"gemini", modelPrice{0.15, 0.60, "google", false}},
	// Google via Gemini prefix
	{"gemma", modelPrice{0.0, 0.0, "ollama", true}},

	// Ollama — local, free
	{"ollama", modelPrice{0.0, 0.0, "ollama", true}},
}

// OpenRouter pricing: dynamic per model. Maintained as prefix-based overrides
// for commonly-used OpenRouter models. Others fall back to a default.
var openRouterPricing = []struct {
	prefix string
	price  modelPrice
}{
	{"openai/gpt-4.1", modelPrice{2.0, 8.0, "openrouter", false}},
	{"openai/gpt-4o", modelPrice{2.5, 10.0, "openrouter", false}},
	{"openai/gpt-4o-mini", modelPrice{0.15, 0.60, "openrouter", false}},
	{"openai/o3", modelPrice{10.0, 40.0, "openrouter", false}},
	{"openai/o1", modelPrice{15.0, 60.0, "openrouter", false}},
	{"openai/gpt-4", modelPrice{30.0, 60.0, "openrouter", false}},
	{"anthropic/claude-sonnet-4", modelPrice{3.0, 15.0, "openrouter", false}},
	{"anthropic/claude-opus-4", modelPrice{15.0, 75.0, "openrouter", false}},
	{"anthropic/claude-3.5-sonnet", modelPrice{3.0, 15.0, "openrouter", false}},
	{"anthropic/claude-3.5-haiku", modelPrice{1.0, 5.0, "openrouter", false}},
	{"anthropic/claude-3-haiku", modelPrice{0.25, 1.25, "openrouter", false}},
	{"anthropic/claude-3-sonnet", modelPrice{3.0, 15.0, "openrouter", false}},
	{"anthropic/claude-3-opus", modelPrice{15.0, 75.0, "openrouter", false}},
	{"google/gemini-2.5-pro", modelPrice{1.25, 5.0, "openrouter", false}},
	{"google/gemini-2.5-flash", modelPrice{0.15, 0.60, "openrouter", false}},
	{"google/gemini-2.0-flash", modelPrice{0.10, 0.40, "openrouter", false}},
	{"deepseek/deepseek-chat", modelPrice{0.27, 1.10, "openrouter", false}},
	{"deepseek/deepseek-reasoner", modelPrice{0.55, 2.19, "openrouter", false}},
	{"meta-llama/llama-4-maverick", modelPrice{0.20, 0.90, "openrouter", false}},
	{"meta-llama/llama-4-scout", modelPrice{0.15, 0.50, "openrouter", false}},
	{"qwen/qwen-max", modelPrice{2.4, 9.6, "openrouter", false}},
}

// ── Types ────────────────────────────────────────────────────────────────────

// TokenCounts tracks input/output token tallies.
type TokenCounts struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	CachedTokens     int64 `json:"cached_tokens"` // tokens served from cache (discounted)
	CallCount        int64 `json:"call_count"`
}

// SessionCostSummary is cost data scoped to one session.
type SessionCostSummary struct {
	SessionID    string      `json:"session_id"`
	TotalCost    float64     `json:"total_cost"`
	TokenCounts  TokenCounts `json:"token_counts"`
	ModelCosts   []ModelCostSummary `json:"model_costs"`
}

// DailyCostSummary is cost data for a single day.
type DailyCostSummary struct {
	Date        string      `json:"date"`
	TotalCost   float64     `json:"total_cost"`
	TokenCounts TokenCounts `json:"token_counts"`
}

// ProviderCostSummary breaks down costs by provider.
type ProviderCostSummary struct {
	Provider   string      `json:"provider"`
	TotalCost  float64     `json:"total_cost"`
	TokenCounts TokenCounts `json:"token_counts"`
}

// ModelCostSummary breaks down costs by specific model.
type ModelCostSummary struct {
	Model      string      `json:"model"`
	Provider   string      `json:"provider"`
	TotalCost  float64     `json:"total_cost"`
	InputCost  float64     `json:"input_cost"`
	OutputCost float64     `json:"output_cost"`
	TokenCounts TokenCounts `json:"token_counts"`
}

// CallRecord is a single LLM call as recorded in the tracker.
type CallRecord struct {
	Time             time.Time `json:"time"`
	Model            string    `json:"model"`
	SessionID        string    `json:"session_id,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	CachedTokens     int       `json:"cached_tokens"`
	InputCost        float64   `json:"input_cost"`
	OutputCost       float64   `json:"output_cost"`
	CallCost         float64   `json:"call_cost"`
}

// BudgetAlert is emitted when budget thresholds are crossed.
type BudgetAlert struct {
	Model     string    `json:"model"`
	Limit     float64   `json:"limit"`
	Current   float64   `json:"current_spend"`
	Percent   float64   `json:"percent"`
	Level     string    `json:"level"` // "warn" or "block"
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
}

// budget tracks spend against a limit for one model.
type budget struct {
	Model      string
	Limit      float64
	Spent      float64
	Blocked    bool // blocked once 100% hit
}

// Tracker accumulates cost data with thread safety.
type Tracker struct {
	mu sync.RWMutex

	// Running totals
	totalCost   float64
	totalTokens TokenCounts

	// Per-session tracking: sessionID → SessionCostSummary
	sessions map[string]*sessionAccum

	// Per-day tracking: date string → token/cost
	daily map[string]*dailyAccum

	// Per-provider: provider name → accum
	providers map[string]*providerAccum

	// Per-model: full model name → accum  
	models map[string]*modelAccum

	// Recent call records (ring buffer)
	recentCalls []CallRecord
	maxCalls    int
	callIdx     int

	// Budget tracking
	budgets map[string]*budget
}

type sessionAccum struct {
	ID       string
	Cost     float64
	Tokens   TokenCounts
	Models   map[string]*modelSubAccum
}

type dailyAccum struct {
	Cost   float64
	Tokens TokenCounts
}

type providerAccum struct {
	Cost   float64
	Tokens TokenCounts
}

type modelAccum struct {
	Model      string
	Provider   string
	InputCost  float64
	OutputCost float64
	Tokens     TokenCounts
}

type modelSubAccum struct {
	InputCost  float64
	OutputCost float64
	Tokens     TokenCounts
}

// ── Constructor ──────────────────────────────────────────────────────────────

// NewTracker creates a cost tracker.
func NewTracker() *Tracker {
	return &Tracker{
		sessions:    make(map[string]*sessionAccum),
		daily:       make(map[string]*dailyAccum),
		providers:   make(map[string]*providerAccum),
		models:      make(map[string]*modelAccum),
		budgets:     make(map[string]*budget),
		recentCalls: make([]CallRecord, 200),
		maxCalls:    200,
	}
}

// ── Lookup ───────────────────────────────────────────────────────────────────

// lookupPrice finds the modelPrice for a given model string.
// Matches by prefix: more specific patterns tested first.
func lookupPrice(model string) modelPrice {
	lower := strings.ToLower(strings.TrimSpace(model))

	// Ollama local models — check before OpenRouter slash detection
	if strings.HasPrefix(lower, "ollama") || strings.HasPrefix(lower, "ollama/") {
		return modelPrice{0.0, 0.0, "ollama", true}
	}
	// Gemma (local Ollama models)
	if strings.Contains(lower, "gemma") && !strings.Contains(lower, "openai") && !strings.Contains(lower, "anthropic") {
		return modelPrice{0.0, 0.0, "ollama", true}
	}

	// Check OpenRouter-specific pricing first (models prefixed with provider/)
	if strings.Contains(lower, "/") {
		for _, or := range openRouterPricing {
			if strings.HasPrefix(lower, strings.ToLower(or.prefix)) {
				return or.price
			}
		}
		// Generic OpenRouter with unknown model: parse the model part after /
		parts := strings.SplitN(lower, "/", 2)
		if len(parts) == 2 {
			inner := lookupPrice(parts[1])
			inner.Provider = "openrouter"
			return inner
		}
		return modelPrice{1.0, 4.0, "openrouter", false}
	}

	// Standard pricing table — ordered more-specific-first
	for _, entry := range pricingTable {
		if strings.Contains(lower, strings.ToLower(entry.pattern)) {
			return entry.price
		}
	}

	// Unknown model — use reasonable default by checking provider
	if strings.Contains(lower, "openai") || strings.Contains(lower, "gpt") {
		return modelPrice{2.5, 10.0, "openai", false}
	}
	if strings.Contains(lower, "claude") {
		return modelPrice{3.0, 15.0, "anthropic", false}
	}
	if strings.Contains(lower, "gemini") || strings.Contains(lower, "google") {
		return modelPrice{0.15, 0.60, "google", false}
	}
	if strings.Contains(lower, "deepseek") {
		return modelPrice{0.27, 1.10, "deepseek", false}
	}

	// Conservative default: assume mid-tier pricing
	return modelPrice{2.5, 10.0, "unknown", false}
}

// ── Recording ────────────────────────────────────────────────────────────────

// Record records an LLM call's usage and calculates cost.
func (t *Tracker) Record(model string, usage llm.Usage) float64 {
	return t.RecordWithCached(model, usage, 0, "")
}

// RecordWithCached records usage with cached token discounts.
// cachedTokens: tokens served from cache (Anthropic 10%, OpenAI 50% discount on input).
// sessionID: optional session scope for per-session aggregation.
func (t *Tracker) RecordWithCached(model string, usage llm.Usage, cachedTokens int, sessionID string) float64 {
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return 0
	}

	price := lookupPrice(model)
	if price.Free {
		return 0
	}

	// Calculate cached discount
	cachedDiscountFactor := 1.0
	effectivePrompt := usage.PromptTokens
	if cachedTokens > 0 && effectivePrompt > 0 {
		// OpenAI: 50% discount on cached input tokens
		// Anthropic: 10% discount on cached input tokens
		switch price.Provider {
		case "openai":
			cachedDiscountFactor = 0.5
		case "anthropic":
			cachedDiscountFactor = 0.10
		default:
			cachedDiscountFactor = 0.0 // no cached discount
		}

		if cachedTokens > effectivePrompt {
			cachedTokens = effectivePrompt
		}
	}

	// Calculate cost
	regularPrompt := usage.PromptTokens - cachedTokens
	inputCost := float64(regularPrompt)*price.InputPrice/1_000_000 +
		float64(cachedTokens)*price.InputPrice*cachedDiscountFactor/1_000_000
	outputCost := float64(usage.CompletionTokens) * price.OutputPrice / 1_000_000
	callCost := inputCost + outputCost

	// Round to 6 decimal places to avoid float noise
	callCost = math.Round(callCost*1_000_000) / 1_000_000

	todayKey := time.Now().Format("2006-01-02")

	t.mu.Lock()
	defer t.mu.Unlock()

	// ── Running totals ────────────────────────────────────────────────
	t.totalCost += callCost
	t.totalTokens.PromptTokens += int64(usage.PromptTokens)
	t.totalTokens.CompletionTokens += int64(usage.CompletionTokens)
	t.totalTokens.TotalTokens += int64(usage.TotalTokens)
	if cachedTokens > 0 {
		t.totalTokens.CachedTokens += int64(cachedTokens)
	}
	t.totalTokens.CallCount++

	// ── Daily ───────────────────────────────────────────────────────
	if t.daily[todayKey] == nil {
		t.daily[todayKey] = &dailyAccum{}
	}
	da := t.daily[todayKey]
	da.Cost += callCost
	da.Tokens.PromptTokens += int64(usage.PromptTokens)
	da.Tokens.CompletionTokens += int64(usage.CompletionTokens)
	da.Tokens.TotalTokens += int64(usage.TotalTokens)
	if cachedTokens > 0 {
		da.Tokens.CachedTokens += int64(cachedTokens)
	}
	da.Tokens.CallCount++

	// ── Provider ─────────────────────────────────────────────────────
	provider := price.Provider
	if t.providers[provider] == nil {
		t.providers[provider] = &providerAccum{}
	}
	pa := t.providers[provider]
	pa.Cost += callCost
	pa.Tokens.PromptTokens += int64(usage.PromptTokens)
	pa.Tokens.CompletionTokens += int64(usage.CompletionTokens)
	pa.Tokens.TotalTokens += int64(usage.TotalTokens)
	if cachedTokens > 0 {
		pa.Tokens.CachedTokens += int64(cachedTokens)
	}
	pa.Tokens.CallCount++

	// ── Model ────────────────────────────────────────────────────────
	modelKey := model
	if t.models[modelKey] == nil {
		t.models[modelKey] = &modelAccum{
			Model:    model,
			Provider: provider,
		}
	}
	ma := t.models[modelKey]
	ma.InputCost += inputCost
	ma.OutputCost += outputCost
	ma.Tokens.PromptTokens += int64(usage.PromptTokens)
	ma.Tokens.CompletionTokens += int64(usage.CompletionTokens)
	ma.Tokens.TotalTokens += int64(usage.TotalTokens)
	if cachedTokens > 0 {
		ma.Tokens.CachedTokens += int64(cachedTokens)
	}
	ma.Tokens.CallCount++

	// ── Session ──────────────────────────────────────────────────────
	if sessionID != "" {
		if t.sessions[sessionID] == nil {
			t.sessions[sessionID] = &sessionAccum{
				ID:     sessionID,
				Models: make(map[string]*modelSubAccum),
			}
		}
		sa := t.sessions[sessionID]
		sa.Cost += callCost
		sa.Tokens.PromptTokens += int64(usage.PromptTokens)
		sa.Tokens.CompletionTokens += int64(usage.CompletionTokens)
		sa.Tokens.TotalTokens += int64(usage.TotalTokens)
		if cachedTokens > 0 {
			sa.Tokens.CachedTokens += int64(cachedTokens)
		}
		sa.Tokens.CallCount++

		if sa.Models[modelKey] == nil {
			sa.Models[modelKey] = &modelSubAccum{}
		}
		msa := sa.Models[modelKey]
		msa.InputCost += inputCost
		msa.OutputCost += outputCost
		msa.Tokens.PromptTokens += int64(usage.PromptTokens)
		msa.Tokens.CompletionTokens += int64(usage.CompletionTokens)
		msa.Tokens.TotalTokens += int64(usage.TotalTokens)
		if cachedTokens > 0 {
			msa.Tokens.CachedTokens += int64(cachedTokens)
		}
		msa.Tokens.CallCount++
	}

	// ── Recent calls ring buffer ─────────────────────────────────────
	t.recentCalls[t.callIdx%t.maxCalls] = CallRecord{
		Time:             time.Now(),
		Model:            model,
		SessionID:        sessionID,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CachedTokens:     cachedTokens,
		InputCost:        inputCost,
		OutputCost:       outputCost,
		CallCost:         callCost,
	}
	t.callIdx++

	// ── Budget check ─────────────────────────────────────────────────
	t.checkBudget(model)

	return callCost
}

// checkBudget checks if the model budget has been exceeded.
func (t *Tracker) checkBudget(model string) {
	b, ok := t.budgets[model]
	if !ok || b.Blocked || b.Limit <= 0 {
		return
	}

	// Sum up spend for this model
	ma, ok := t.models[model]
	if !ok {
		return
	}
	b.Spent = ma.InputCost + ma.OutputCost

	// No alerts built-in — caller can poll BudgetStatus()
}

// ── Queries ──────────────────────────────────────────────────────────────────

// GetTotalCost returns the total accumulated cost.
func (t *Tracker) GetTotalCost() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return math.Round(t.totalCost*1_000_000) / 1_000_000
}

// GetTotalTokens returns total token counts.
func (t *Tracker) GetTotalTokens() TokenCounts {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalTokens
}

// GetSessionCost returns cost summary for a specific session.
func (t *Tracker) GetSessionCost(sessionID string) (SessionCostSummary, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	sa, ok := t.sessions[sessionID]
	if !ok {
		return SessionCostSummary{}, false
	}

	var modelCosts []ModelCostSummary
	for modelKey, msa := range sa.Models {
		price := lookupPrice(modelKey)
		modelCosts = append(modelCosts, ModelCostSummary{
			Model:       modelKey,
			Provider:    price.Provider,
			TotalCost:   msa.InputCost + msa.OutputCost,
			InputCost:   msa.InputCost,
			OutputCost:  msa.OutputCost,
			TokenCounts: msa.Tokens,
		})
	}
	sort.Slice(modelCosts, func(i, j int) bool {
		return modelCosts[i].TotalCost > modelCosts[j].TotalCost
	})

	return SessionCostSummary{
		SessionID:   sessionID,
		TotalCost:   sa.Cost,
		TokenCounts: sa.Tokens,
		ModelCosts:  modelCosts,
	}, true
}

// GetDailyCosts returns cost summaries for the last N days.
func (t *Tracker) GetDailyCosts(days int) []DailyCostSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	type kv struct {
		date string
		acc  *dailyAccum
	}
	var sorted []kv
	for date, acc := range t.daily {
		sorted = append(sorted, kv{date, acc})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].date > sorted[j].date
	})

	result := make([]DailyCostSummary, 0, days)
	for i, kv := range sorted {
		if i >= days {
			break
		}
		result = append(result, DailyCostSummary{
			Date:        kv.date,
			TotalCost:   kv.acc.Cost,
			TokenCounts: kv.acc.Tokens,
		})
	}
	return result
}

// GetProviderCosts returns cost breakdown by provider.
func (t *Tracker) GetProviderCosts() []ProviderCostSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []ProviderCostSummary
	for provider, acc := range t.providers {
		result = append(result, ProviderCostSummary{
			Provider:    provider,
			TotalCost:   acc.Cost,
			TokenCounts: acc.Tokens,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCost > result[j].TotalCost
	})
	return result
}

// GetModelCosts returns cost breakdown by model.
func (t *Tracker) GetModelCosts() []ModelCostSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []ModelCostSummary
	for _, acc := range t.models {
		result = append(result, ModelCostSummary{
			Model:       acc.Model,
			Provider:    acc.Provider,
			TotalCost:   acc.InputCost + acc.OutputCost,
			InputCost:   acc.InputCost,
			OutputCost:  acc.OutputCost,
			TokenCounts: acc.Tokens,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalCost > result[j].TotalCost
	})
	return result
}

// GetRecentCalls returns the most recent N call records.
func (t *Tracker) GetRecentCalls(n int) []CallRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	total := t.callIdx
	if total > t.maxCalls {
		total = t.maxCalls
	}
	if n > total {
		n = total
	}

	result := make([]CallRecord, n)
	for i := 0; i < n; i++ {
		idx := (t.callIdx - 1 - i + t.maxCalls) % t.maxCalls
		result[n-1-i] = t.recentCalls[idx]
	}
	return result
}

// ── Budget ───────────────────────────────────────────────────────────────────

// SetBudget configures a spending limit for a model.
func (t *Tracker) SetBudget(model string, limit float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.budgets[model] = &budget{
		Model: model,
		Limit: limit,
	}
}

// BudgetStatus returns the current budget status for a model.
// Use "*" to check overall budget across all models.
// Returns: spent, limit, percent, level ("ok", "warn", "block").
func (t *Tracker) BudgetStatus(model string) (spent, limit, percent float64, level string) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Wildcard: aggregate all budgets
	if model == "*" {
		// Sum all model costs as total spent
		for _, acc := range t.models {
			spent += acc.InputCost + acc.OutputCost
		}
		// Find the most-constrained budget as the effective limit
		for _, b := range t.budgets {
			if b.Limit <= 0 {
				continue
			}
			if limit == 0 || b.Limit < limit {
				limit = b.Limit
			}
		}
		if limit == 0 && cfgBudgetLimit() > 0 {
			limit = cfgBudgetLimit()
		}
		if limit > 0 {
			percent = math.Round(spent/limit*10000) / 100
		}
		level = "ok"
		if percent >= 100 {
			level = "block"
		} else if percent >= 80 {
			level = "warn"
		}
		return
	}

	b, ok := t.budgets[model]
	if !ok || b.Limit <= 0 {
		return 0, 0, 0, "ok"
	}

	// Refresh spent from model accum
	spent = b.Spent
	if ma, ok := t.models[model]; ok {
		spent = ma.InputCost + ma.OutputCost
	}

	limit = b.Limit
	if limit > 0 {
		percent = math.Round(spent/limit*10000) / 100
	}

	level = "ok"
	if percent >= 100 {
		level = "block"
	} else if percent >= 80 {
		level = "warn"
	}

	return spent, limit, percent, level
}

// cfgBudgetLimit returns the default budget limit (50 USD).
func cfgBudgetLimit() float64 { return 50.0 }

// ── Serialization ────────────────────────────────────────────────────────────

// OverviewSummary is the top-level cost summary for the dashboard overview.
type OverviewSummary struct {
	TotalCost     float64              `json:"total_cost"`
	TodayCost     float64              `json:"today_cost"`
	TotalTokens   TokenCounts          `json:"total_tokens"`
	Providers     []ProviderCostSummary `json:"providers"`
	Models        []ModelCostSummary    `json:"models"`
	Daily         []DailyCostSummary    `json:"daily"`
	BudgetAlerts  []BudgetAlert         `json:"budget_alerts,omitempty"`
}

// ToJSON returns the full cost state as JSON for the dashboard API.
func (t *Tracker) ToJSON() []byte {
	summary := t.getOverviewSummary()
	data, _ := json.MarshalIndent(summary, "", "  ")
	return data
}

// ToOverviewJSON returns a compact overview suitable for the dashboard.
func (t *Tracker) Overview() OverviewSummary {
	return t.getOverviewSummary()
}

func (t *Tracker) ToOverviewJSON() []byte {
	summary := t.getOverviewSummary()
	data, _ := json.Marshal(summary)
	return data
}

func (t *Tracker) getOverviewSummary() OverviewSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	todayKey := time.Now().Format("2006-01-02")
	todayCost := 0.0
	if da, ok := t.daily[todayKey]; ok {
		todayCost = da.Cost
	}

	// Providers
	var providers []ProviderCostSummary
	for provider, acc := range t.providers {
		providers = append(providers, ProviderCostSummary{
			Provider:    provider,
			TotalCost:   acc.Cost,
			TokenCounts: acc.Tokens,
		})
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].TotalCost > providers[j].TotalCost
	})

	// Models
	var models []ModelCostSummary
	for _, acc := range t.models {
		models = append(models, ModelCostSummary{
			Model:       acc.Model,
			Provider:    acc.Provider,
			TotalCost:   acc.InputCost + acc.OutputCost,
			InputCost:   acc.InputCost,
			OutputCost:  acc.OutputCost,
			TokenCounts: acc.Tokens,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].TotalCost > models[j].TotalCost
	})

	// Daily (last 7 days)
	var daily []DailyCostSummary
	type dkv struct {
		date string
		acc  *dailyAccum
	}
	var sorted []dkv
	for date, acc := range t.daily {
		sorted = append(sorted, dkv{date, acc})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].date > sorted[j].date })

	for i := 0; i < len(sorted) && i < 7; i++ {
		daily = append(daily, DailyCostSummary{
			Date:        sorted[i].date,
			TotalCost:   sorted[i].acc.Cost,
			TokenCounts: sorted[i].acc.Tokens,
		})
	}

	// Budget alerts
	var alerts []BudgetAlert
	for _, b := range t.budgets {
		spent := b.Spent
		if ma, ok := t.models[b.Model]; ok {
			spent = ma.InputCost + ma.OutputCost
		}
		if b.Limit > 0 {
			pct := math.Round(spent / b.Limit * 10000) / 100
			level := "ok"
			msg := "Within budget"
			if pct >= 100 {
				level = "block"
				msg = fmt.Sprintf("Budget limit of $%.2f exceeded for %s", b.Limit, b.Model)
			} else if pct >= 80 {
				level = "warn"
				msg = fmt.Sprintf("%s is at %.1f%% of budget ($%.2f/$%.2f)", b.Model, pct, spent, b.Limit)
			}
			if level != "ok" {
				alerts = append(alerts, BudgetAlert{
					Model:   b.Model,
					Limit:   b.Limit,
					Current: spent,
					Percent: pct,
					Level:   level,
					Message: msg,
					Time:    time.Now(),
				})
			}
		}
	}

	return OverviewSummary{
		TotalCost:    math.Round(t.totalCost*1_000_000) / 1_000_000,
		TodayCost:    math.Round(todayCost*1_000_000) / 1_000_000,
		TotalTokens:  t.totalTokens,
		Providers:    providers,
		Models:       models,
		Daily:        daily,
		BudgetAlerts: alerts,
	}
}

// ── Utility ───────────────────────────────────────────────────────────────────

// FormatUSD returns a readable cost string with appropriate precision.
func FormatUSD(cost float64) string {
	if cost == 0 {
		return "$0.00"
	}
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	if cost < 0.10 {
		return fmt.Sprintf("$%.3f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

// CacheDiscountPct returns the cached-input discount percentage for a provider.
func CacheDiscountPct(provider string) int {
	switch strings.ToLower(provider) {
	case "openai", "openrouter":
		return 50
	case "anthropic":
		return 10
	default:
		return 0
	}
}