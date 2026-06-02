// Package llm — LLM adapter layer with circuit breakers and fallback chains.
//
// Supported providers: OpenAI-compatible, Ollama (local), Anthropic.
// Every adapter implements the same interface — agents are provider-agnostic.
//
// Circuit breaker pattern:
//   Closed → (N failures) → Open → (timeout) → Half-Open → (success) → Closed
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrStreamingNotSupported = errors.New("llm: streaming not supported")

// ── Types ────────────────────────────────────────────────────────────────────

// Message is a single turn in an LLM conversation.
type Message struct {
	Role    string `json:"role"`    // system, user, assistant, tool
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// Request wraps a chat completion request, provider-agnostic.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Tools       []ToolDef `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"` // "auto", "none", or tool name
}

// ToolDef describes a tool the LLM can call.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"` // JSON Schema
}

// Response is a chat completion response, provider-agnostic.
type Response struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice is one completion option.
type Choice struct {
	Index     int        `json:"index"`
	Message   Message    `json:"message"`
	Finish    string     `json:"finish_reason"` // stop, length, tool_calls
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall is a tool invocation requested by the LLM.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a single streaming response chunk from an LLM.
type StreamChunk struct {
	Model     string    `json:"model,omitempty"`
	Content   string    `json:"content,omitempty"`
	Done      bool      `json:"done"`
	Role      string    `json:"role,omitempty"`
	Finish    string    `json:"finish,omitempty"`
	Usage     Usage     `json:"usage,omitempty"`
	Error     string    `json:"error,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ── Adapter Interface ────────────────────────────────────────────────────────

// Adapter is the interface all LLM providers implement.
type Adapter interface {
	Provider() string
	ContextWindow() int
	Chat(ctx context.Context, req Request) (Response, error)
	StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error)
	HealthCheck(ctx context.Context) error
}

// ── Circuit Breaker ──────────────────────────────────────────────────────────

type breakerState int32

const (
	closed    breakerState = iota
	open
	halfOpen
)

// Breaker wraps an Adapter with a circuit breaker.
type Breaker struct {
	inner       Adapter
	failures    atomic.Int32
	lastFailure atomic.Int64 // Unix nano
	state       atomic.Int32

	maxFailures  int32
	resetTimeout time.Duration

	mu sync.Mutex
}

// NewBreaker wraps an adapter. After maxFailures consecutive failures,
// the breaker trips. After resetTimeout, it transitions to half-open.
func NewBreaker(inner Adapter, maxFailures int32, resetTimeout time.Duration) *Breaker {
	return &Breaker{
		inner:        inner,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
	}
}

func (b *Breaker) ContextWindow() int { return b.inner.ContextWindow() }
func (b *Breaker) Provider() string { return b.inner.Provider() }

func (b *Breaker) Chat(ctx context.Context, req Request) (Response, error) {
	if !b.allow() {
		return Response{}, fmt.Errorf("llm: circuit breaker open for %s", b.inner.Provider())
	}

	resp, err := b.inner.Chat(ctx, req)
	if err != nil {
		b.recordFailure()
		return resp, err
	}

	b.recordSuccess()
	return resp, nil
}

func (b *Breaker) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	if !b.allow() {
		return nil, fmt.Errorf("llm: circuit breaker open for %s", b.inner.Provider())
	}
	ch, err := b.inner.StreamChat(ctx, req)
	if err != nil {
		b.recordFailure()
		return ch, err
	}
	out := make(chan StreamChunk, 100)
	go func() {
		defer close(out)
		for chunk := range ch {
			out <- chunk
			if chunk.Done || chunk.Error != "" {
				if chunk.Error != "" {
					b.recordFailure()
				} else {
					b.recordSuccess()
				}
			}
		}
	}()
	return out, nil
}

func (b *Breaker) HealthCheck(ctx context.Context) error {
	if !b.allow() {
		return fmt.Errorf("llm: circuit breaker open")
	}
	err := b.inner.HealthCheck(ctx)
	if err != nil {
		b.recordFailure()
	} else {
		b.recordSuccess()
	}
	return err
}

func (b *Breaker) allow() bool {
	s := breakerState(b.state.Load())

	if s == closed {
		return true
	}

	if s == halfOpen {
		// Allow exactly one request through
		return true
	}

	// Open — check if timeout has elapsed
	last := b.lastFailure.Load()
	if time.Since(time.Unix(0, last)) > b.resetTimeout {
		b.state.Store(int32(halfOpen))
		return true
	}
	return false
}

func (b *Breaker) recordFailure() {
	f := b.failures.Add(1)
	b.lastFailure.Store(time.Now().UnixNano())
	if f >= b.maxFailures {
		b.state.Store(int32(open))
	}
}

func (b *Breaker) recordSuccess() {
	b.failures.Store(0)
	b.state.Store(int32(closed))
}

// ── Fallback Chain ───────────────────────────────────────────────────────────

// FallbackChain tries adapters in order. If the primary fails,
// it transparently falls through to the next adapter.
type FallbackChain struct {
	primary   Adapter
	fallbacks []Adapter
}

// NewFallbackChain creates an ordered adapter chain. Primary is tried first.
func NewFallbackChain(primary Adapter, fallbacks ...Adapter) *FallbackChain {
	return &FallbackChain{primary: primary, fallbacks: fallbacks}
}

func (fc *FallbackChain) ContextWindow() int {
	if fc.primary != nil { return fc.primary.ContextWindow() }
	return 128_000
}
func (fc *FallbackChain) Provider() string {
	var names []string
	names = append(names, fc.primary.Provider())
	for _, f := range fc.fallbacks {
		names = append(names, f.Provider())
	}
	return strings.Join(names, "→")
}

func (fc *FallbackChain) Chat(ctx context.Context, req Request) (Response, error) {
	var lastErr error

	resp, err := fc.primary.Chat(ctx, req)
	if err == nil {
		return resp, nil
	}
	lastErr = err

	for _, fb := range fc.fallbacks {
		resp, err = fb.Chat(ctx, req)
		if err == nil {
			// Log fallback usage
			fmt.Printf("llm: fell back from %s to %s\n", fc.primary.Provider(), fb.Provider())
			return resp, nil
		}
		lastErr = err
	}

	return Response{}, fmt.Errorf("llm: all adapters failed: %w", lastErr)
}

func (fc *FallbackChain) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	ch, err := fc.primary.StreamChat(ctx, req)
	if err == nil {
		return ch, nil
	}
	for _, fb := range fc.fallbacks {
		ch, err = fb.StreamChat(ctx, req)
		if err == nil {
			fmt.Printf("llm: streaming fell back from %s to %s\n", fc.primary.Provider(), fb.Provider())
			return ch, nil
		}
	}
	return nil, fmt.Errorf("llm: all streaming adapters failed: %w", err)
}

func (fc *FallbackChain) HealthCheck(ctx context.Context) error {
	for _, a := range fc.all() {
		if err := a.HealthCheck(ctx); err != nil {
			return fmt.Errorf("%s: %w", a.Provider(), err)
		}
	}
	return nil
}

func (fc *FallbackChain) all() []Adapter {
	return append([]Adapter{fc.primary}, fc.fallbacks...)
}

// ── OpenAI-Compatible Adapter ────────────────────────────────────────────────

// HTTPClient abstracts the HTTP layer for testability.
type HTTPClient interface {
	Do(req any) (resp any, err error)
}

// OpenAIConfig holds the adapter configuration.
type OpenAIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// OpenAIClient implements Adapter for OpenAI-compatible APIs.
// Works with OpenAI, Groq, Together, vLLM, Ollama (via /v1 endpoint), etc.
type OpenAIClient struct {
	cfg    OpenAIConfig
	client HTTPClient
}

// NewOpenAI creates an OpenAI-compatible adapter.
func NewOpenAI(cfg OpenAIConfig, client HTTPClient) *OpenAIClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &OpenAIClient{cfg: cfg, client: client}
}

func (c *OpenAIClient) ContextWindow() int {
	m := c.cfg.Model
	switch {
	case strings.Contains(m, "gpt-4.1") || strings.Contains(m, "gpt-4.5"):
		return 1_000_000
	case strings.Contains(m, "gpt-4o") || strings.Contains(m, "o4") || strings.Contains(m, "o3"):
		return 200_000
	case strings.Contains(m, "gpt-4"), strings.Contains(m, "gpt-3.5"):
		return 128_000
	case strings.Contains(m, "o1"):
		return 200_000
	default:
		return 128_000
	}
}
func (c *OpenAIClient) Provider() string { return "openai" }

func (c *OpenAIClient) Chat(ctx context.Context, req Request) (Response, error) {
	if req.Model == "" {
		req.Model = c.cfg.Model
	}

	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("llm: openai marshal: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("llm: openai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	httpClient := &http.Client{Timeout: c.cfg.Timeout}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("llm: openai http: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 10<<20))
	if err != nil {
		return Response{}, fmt.Errorf("llm: openai read: %w", err)
	}

	if httpResp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("llm: openai http %d: %s", httpResp.StatusCode, string(respBody))
	}

	// OpenAI returns tool_calls nested inside message; extract to top-level Choice
	var raw struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role      string     `json:"role"`
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return Response{}, fmt.Errorf("llm: openai parse: %w", err)
	}

	resp := Response{
		ID:    raw.ID,
		Model: raw.Model,
		Usage: raw.Usage,
	}
	for _, ch := range raw.Choices {
		resp.Choices = append(resp.Choices, Choice{
			Index: ch.Index,
			Message: Message{
				Role:    ch.Message.Role,
				Content: ch.Message.Content,
			},
			Finish:    ch.FinishReason,
			ToolCalls: ch.Message.ToolCalls,
		})
	}
	return resp, nil
}

// StreamChat streams an OpenAI-compatible chat response token-by-token.
func (c *OpenAIClient) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	if req.Model == "" {
		req.Model = c.cfg.Model
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("llm: openai stream marshal: %w", err)
	}
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: openai stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: openai stream http: %w", err)
	}
	out := make(chan StreamChunk, 256)
	go func() {
		defer close(out)
		defer httpResp.Body.Close()
		if httpResp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 10<<20))
			out <- StreamChunk{Error: fmt.Sprintf("openai http %d: %s", httpResp.StatusCode, string(respBody)), Done: true}
			return
		}
		reader := bufio.NewReader(httpResp.Body)
		var usage Usage
		var modelName string
		var finishReason string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if usage.TotalTokens > 0 || finishReason != "" {
					out <- StreamChunk{Done: true, Model: modelName, Usage: usage, Finish: finishReason}
				} else {
					out <- StreamChunk{Done: true}
				}
				return
			}
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") { continue }
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" || data == "[DONE]" {
				if data == "[DONE]" {
					out <- StreamChunk{Done: true, Model: modelName, Usage: usage, Finish: finishReason}
					return
				}
				continue
			}
			var chunk struct {
				Model string `json:"model"`
				Choices []struct {
					Delta struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *Usage `json:"usage"`
			}
			if json.Unmarshal([]byte(data), &chunk) != nil { continue }
			if chunk.Model != "" { modelName = chunk.Model }
			if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 { usage = *chunk.Usage }
			if len(chunk.Choices) == 0 { continue }
			delta := chunk.Choices[0].Delta
			if chunk.Choices[0].FinishReason != "" { finishReason = chunk.Choices[0].FinishReason }
			out <- StreamChunk{Content: delta.Content, Role: delta.Role, Done: false}
		}
	}()
	return out, nil
}

func (c *OpenAIClient) HealthCheck(ctx context.Context) error {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("llm: openai health: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("llm: openai health: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 300 {
		return fmt.Errorf("llm: openai health: status %d", httpResp.StatusCode)
	}
	return nil
}

// ── Ollama Adapter (Local) ───────────────────────────────────────────────────

// OllamaConfig holds the Ollama adapter configuration.
type OllamaConfig struct {
	BaseURL string        // http://localhost:11434
	Model   string
	Timeout time.Duration
}

// OllamaClient implements Adapter for locally-hosted Ollama.
type OllamaClient struct {
	cfg OpenAIConfig // Ollama now has /v1/chat/completions compatibility
}

// NewOllama creates an Ollama adapter.
func NewOllama(cfg OllamaConfig) *OllamaClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second // local models may be slower
	}
	return &OllamaClient{
		cfg: OpenAIConfig{
			BaseURL: cfg.BaseURL,
			APIKey:  "ollama",
			Model:   cfg.Model,
			Timeout: cfg.Timeout,
		},
	}
}

func (c *OllamaClient) ContextWindow() int { return 0 }
func (c *OllamaClient) Provider() string { return "ollama" }

// ollamaChatReq is the Ollama /api/chat request body.
type ollamaChatReq struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// ollamaChatResp is the Ollama /api/chat response body.
type ollamaChatResp struct {
	Model     string  `json:"model"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
	EvalCount int     `json:"eval_count"`
	PromptEvalCount int `json:"prompt_eval_count"`
}

// StreamChat streams an Ollama /api/chat response as NDJSON chunks.
func (c *OllamaClient) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	if req.Model == "" { req.Model = c.cfg.Model }
	ollamaReq := ollamaChatReq{Model: req.Model, Messages: req.Messages, Stream: true}
	body, _ := json.Marshal(ollamaReq)
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/api/chat"
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil { return nil, fmt.Errorf("llm: ollama stream http: %w", err) }
	out := make(chan StreamChunk, 256)
	go func() {
		defer close(out)
		defer httpResp.Body.Close()
		if httpResp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 10<<20))
			out <- StreamChunk{Error: fmt.Sprintf("ollama http %d: %s", httpResp.StatusCode, string(respBody)), Done: true}
			return
		}
		scanner := bufio.NewScanner(httpResp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		var usage Usage
		for scanner.Scan() {
			var chunk ollamaChatResp
			if json.Unmarshal(scanner.Bytes(), &chunk) != nil { continue }
			if chunk.Done {
				usage = Usage{PromptTokens: chunk.PromptEvalCount, CompletionTokens: chunk.EvalCount, TotalTokens: chunk.PromptEvalCount + chunk.EvalCount}
				out <- StreamChunk{Done: true, Model: chunk.Model, Usage: usage, Finish: "stop"}
				return
			}
			out <- StreamChunk{Content: chunk.Message.Content, Role: chunk.Message.Role}
		}
	}()
	return out, nil
}

func (c *OllamaClient) Chat(ctx context.Context, req Request) (Response, error) {
	if req.Model == "" {
		req.Model = c.cfg.Model
	}

	ollamaReq := ollamaChatReq{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   false,
	}
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return Response{}, fmt.Errorf("llm: ollama marshal: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("llm: ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: c.cfg.Timeout}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("llm: ollama http: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 10<<20))
	if err != nil {
		return Response{}, fmt.Errorf("llm: ollama read: %w", err)
	}

	if httpResp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("llm: ollama http %d: %s", httpResp.StatusCode, string(respBody))
	}

	var raw ollamaChatResp
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return Response{}, fmt.Errorf("llm: ollama parse: %w", err)
	}

	return Response{
		Model: raw.Model,
		Choices: []Choice{{
			Index:   0,
			Message: raw.Message,
			Finish:  "stop",
		}},
		Usage: Usage{
			PromptTokens:     raw.PromptEvalCount,
			CompletionTokens: raw.EvalCount,
			TotalTokens:      raw.PromptEvalCount + raw.EvalCount,
		},
	}, nil
}

func (c *OllamaClient) HealthCheck(ctx context.Context) error {
	url := strings.TrimRight(c.cfg.BaseURL, "/")
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("llm: ollama health: %w", err)
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("llm: ollama health: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 300 {
		return fmt.Errorf("llm: ollama health: status %d", httpResp.StatusCode)
	}
	return nil
}

// ── Anthropic Adapter ────────────────────────────────────────────────────────

// AnthropicConfig holds the adapter configuration.
type AnthropicConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// AnthropicClient implements Adapter for Anthropic's Messages API.
type AnthropicClient struct {
	cfg AnthropicConfig
}

// NewAnthropic creates an Anthropic adapter.
func NewAnthropic(cfg AnthropicConfig) *AnthropicClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &AnthropicClient{cfg: cfg}
}

func (c *AnthropicClient) ContextWindow() int {
	m := c.cfg.Model
	switch {
	case strings.Contains(m, "claude-sonnet-4-20250514"):
		return 200_000
	case strings.Contains(m, "claude-3-opus"), strings.Contains(m, "claude-3.5"):
		return 200_000
	case strings.Contains(m, "claude-4"), strings.Contains(m, "claude-sonnet-4"):
		return 200_000
	case strings.Contains(m, "claude-3"):
		return 200_000
	default:
		return 200_000
	}
}
func (c *AnthropicClient) Provider() string { return "anthropic" }

// anthropicContent is a content block in Anthropic's Messages API.
type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// anthropicReq is the Anthropic Messages API request.
type anthropicReq struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string              `json:"role"`
	Content string              `json:"content"`
}

// anthropicResp is the Anthropic Messages API response.
type anthropicResp struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Role       string              `json:"role"`
	Content    []anthropicContent  `json:"content"`
	Model      string              `json:"model"`
	StopReason string              `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// StreamChat streams an Anthropic Messages API response via SSE.
func (c *AnthropicClient) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	if req.Model == "" { req.Model = c.cfg.Model }
	var sys string
	var aMsgs []anthropicMessage
	for _, m := range req.Messages {
		if m.Role == "system" { sys = m.Content; continue }
		aMsgs = append(aMsgs, anthropicMessage{Role: m.Role, Content: m.Content})
	}
	maxToks := req.MaxTokens
	if maxToks == 0 { maxToks = 4096 }
	body, _ := json.Marshal(anthropicReq{Model: req.Model, MaxTokens: maxToks, Messages: aMsgs, System: sys, Temperature: req.Temperature})
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/v1/messages"
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil { return nil, fmt.Errorf("llm: anthropic stream http: %w", err) }
	out := make(chan StreamChunk, 256)
	go func() {
		defer close(out)
		defer httpResp.Body.Close()
		if httpResp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 10<<20))
			out <- StreamChunk{Error: fmt.Sprintf("anthropic http %d: %s", httpResp.StatusCode, string(respBody)), Done: true}
			return
		}
		reader := bufio.NewReader(httpResp.Body)
		var usage Usage
		var mName string
		var finish string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if usage.TotalTokens > 0 || finish != "" {
					out <- StreamChunk{Done: true, Model: mName, Usage: usage, Finish: finish}
				} else { out <- StreamChunk{Done: true} }
				return
			}
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") { continue }
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" { continue }
			var chunk struct {
				Type string `json:"type"`
				Delta struct {
					Type       string `json:"type"`
					Text       string `json:"text"`
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage  struct { OutputTokens int `json:"output_tokens"` } `json:"usage"`
				Message struct { Model string `json:"model"` } `json:"message"`
			}
			if json.Unmarshal([]byte(data), &chunk) != nil { continue }
			switch chunk.Type {
			case "content_block_delta":
				if chunk.Delta.Type == "text_delta" { out <- StreamChunk{Content: chunk.Delta.Text} }
			case "message_start":
				mName = chunk.Message.Model
			case "message_delta":
				if chunk.Usage.OutputTokens > 0 { usage.CompletionTokens = chunk.Usage.OutputTokens }
				if chunk.Delta.StopReason != "" { finish = chunk.Delta.StopReason }
			case "message_stop":
				out <- StreamChunk{Done: true, Model: mName, Usage: usage, Finish: finish}
				return
			}
		}
	}()
	return out, nil
}

func (c *AnthropicClient) Chat(ctx context.Context, req Request) (Response, error) {
	if req.Model == "" {
		req.Model = c.cfg.Model
	}

	// Convert unified messages to Anthropic format
	var systemPrompt string
	var aMsgs []anthropicMessage
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}
		aMsgs = append(aMsgs, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	aReq := anthropicReq{
		Model:       req.Model,
		MaxTokens:   maxTokens,
		Messages:    aMsgs,
		System:      systemPrompt,
		Temperature: req.Temperature,
	}

	body, err := json.Marshal(aReq)
	if err != nil {
		return Response{}, fmt.Errorf("llm: anthropic marshal: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("llm: anthropic request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpClient := &http.Client{Timeout: c.cfg.Timeout}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("llm: anthropic http: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 10<<20))
	if err != nil {
		return Response{}, fmt.Errorf("llm: anthropic read: %w", err)
	}

	if httpResp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("llm: anthropic http %d: %s", httpResp.StatusCode, string(respBody))
	}

	var raw anthropicResp
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return Response{}, fmt.Errorf("llm: anthropic parse: %w", err)
	}

	// Extract text from content blocks
	var textParts []string
	for _, block := range raw.Content {
		if block.Type == "text" {
			textParts = append(textParts, block.Text)
		}
	}

	finishReason := "stop"
	switch raw.StopReason {
	case "end_turn":
		finishReason = "stop"
	case "max_tokens":
		finishReason = "length"
	case "tool_use":
		finishReason = "tool_calls"
	}

	return Response{
		ID:    raw.ID,
		Model: raw.Model,
		Choices: []Choice{{
			Index: 0,
			Message: Message{
				Role:    raw.Role,
				Content: strings.Join(textParts, "\n"),
			},
			Finish: finishReason,
		}},
		Usage: Usage{
			PromptTokens:     raw.Usage.InputTokens,
			CompletionTokens: raw.Usage.OutputTokens,
			TotalTokens:      raw.Usage.InputTokens + raw.Usage.OutputTokens,
		},
	}, nil
}

func (c *AnthropicClient) HealthCheck(ctx context.Context) error {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/v1/messages"
	// Use a minimal request to check connectivity
	body := []byte(`{"model":"claude-sonnet-4-20250514","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("llm: anthropic health: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("llm: anthropic health: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 300 {
		return fmt.Errorf("llm: anthropic health: status %d", httpResp.StatusCode)
	}
	return nil
}

// ── Compile-time checks ──────────────────────────────────────────────────────

var (
	_ Adapter = (*OpenAIClient)(nil)
	_ Adapter = (*OllamaClient)(nil)
	_ Adapter = (*AnthropicClient)(nil)
)
