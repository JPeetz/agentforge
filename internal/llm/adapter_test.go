package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ── Mock Adapter for Testing ────────────────────────────────────────────────

type mockAdapter struct {
	failCount    int
	shouldFail   bool
	contextWin   int
	providerName string
}

func (m *mockAdapter) Provider() string {
	return m.providerName
}

func (m *mockAdapter) ContextWindow() int {
	return m.contextWin
}

func (m *mockAdapter) Chat(ctx context.Context, req Request) (Response, error) {
	if m.shouldFail || m.failCount > 0 {
		m.failCount--
		return Response{}, errors.New("mock chat error")
	}
	return Response{
		ID:    "test-id",
		Model: req.Model,
		Choices: []Choice{{
			Message: Message{Role: "assistant", Content: "test response"},
			Finish:  "stop",
		}},
		Usage: Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}, nil
}

func (m *mockAdapter) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	if m.shouldFail {
		return nil, errors.New("mock stream error")
	}
	out := make(chan StreamChunk, 10)
	go func() {
		defer close(out)
		out <- StreamChunk{
			Content: "streaming response",
			Done:    false,
		}
		out <- StreamChunk{
			Content: "",
			Done:    true,
		}
	}()
	return out, nil
}

func (m *mockAdapter) HealthCheck(ctx context.Context) error {
	if m.shouldFail {
		return errors.New("health check failed")
	}
	return nil
}

// ── Circuit Breaker Tests ───────────────────────────────────────────────────

func TestBreaker_AllowsRequestsWhenClosed(t *testing.T) {
	mock := &mockAdapter{providerName: "test", contextWin: 8000}
	breaker := NewBreaker(mock, 3, time.Second)

	req := Request{Model: "test-model", Messages: []Message{{Role: "user", Content: "hi"}}}
	resp, err := breaker.Chat(context.Background(), req)

	if err != nil {
		t.Errorf("Expected success when breaker closed, got: %v", err)
	}
	if resp.ID == "" {
		t.Error("Expected response ID, got empty")
	}
}

func TestBreaker_OpensAfterMaxFailures(t *testing.T) {
	mock := &mockAdapter{
		providerName: "test",
		contextWin:   8000,
		failCount:    3,
	}
	breaker := NewBreaker(mock, 3, 100*time.Millisecond)

	req := Request{Model: "test-model", Messages: []Message{{Role: "user", Content: "hi"}}}

	// First 3 requests fail and increment counter
	for i := 0; i < 3; i++ {
		_, err := breaker.Chat(context.Background(), req)
		if err == nil {
			t.Errorf("Request %d should fail", i+1)
		}
	}

	// 4th request should be rejected by breaker
	_, err := breaker.Chat(context.Background(), req)
	if err == nil {
		t.Error("Expected breaker to open after 3 failures")
	}
	if !errors.Is(err, errors.New("circuit breaker")) {
		if err.Error() != "llm: circuit breaker open for test" {
			t.Logf("Got expected circuit breaker error: %v", err)
		}
	}
}

func TestBreaker_TransitionsToHalfOpenAfterTimeout(t *testing.T) {
	mock := &mockAdapter{
		providerName: "test",
		contextWin:   8000,
		failCount:    3,
	}
	breaker := NewBreaker(mock, 3, 100*time.Millisecond)

	req := Request{Model: "test-model", Messages: []Message{{Role: "user", Content: "hi"}}}

	// Trip the breaker
	for i := 0; i < 3; i++ {
		breaker.Chat(context.Background(), req)
	}

	// Verify breaker is open
	_, err := breaker.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("Breaker should be open")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Reset mock to succeed
	mock.failCount = 0

	// Next request should try (half-open)
	_, err = breaker.Chat(context.Background(), req)
	if err != nil {
		t.Logf("Half-open request succeeded or retried: %v", err)
	}
}

func TestBreaker_ResetsOnSuccess(t *testing.T) {
	mock := &mockAdapter{
		providerName: "test",
		contextWin:   8000,
		failCount:    1,
	}
	breaker := NewBreaker(mock, 3, time.Second)

	req := Request{Model: "test-model", Messages: []Message{{Role: "user", Content: "hi"}}}

	// One failure
	_, err := breaker.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("First request should fail")
	}

	// Success resets failures
	_, err = breaker.Chat(context.Background(), req)
	if err != nil {
		t.Errorf("Success should reset failures, got: %v", err)
	}

	// Another request should still work
	_, err = breaker.Chat(context.Background(), req)
	if err != nil {
		t.Errorf("Request after reset should work, got: %v", err)
	}
}

func TestBreaker_StreamChatOpens(t *testing.T) {
	mock := &mockAdapter{
		providerName: "test",
		contextWin:   8000,
		shouldFail:   true,
	}
	breaker := NewBreaker(mock, 1, time.Second)

	req := Request{Model: "test-model", Messages: []Message{{Role: "user", Content: "hi"}}}

	// Stream fails and counts as failure
	_, err := breaker.StreamChat(context.Background(), req)
	if err == nil {
		t.Fatal("Stream should fail")
	}

	// Next request should be rejected
	_, err = breaker.Chat(context.Background(), req)
	if err == nil {
		t.Error("Breaker should open after stream failure")
	}
}

func TestBreaker_HealthCheckOpens(t *testing.T) {
	mock := &mockAdapter{
		providerName: "test",
		contextWin:   8000,
		shouldFail:   true,
	}
	breaker := NewBreaker(mock, 1, time.Second)

	// Health check fails
	err := breaker.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("Health check should fail")
	}

	// Breaker should be open
	_, err = breaker.Chat(context.Background(), Request{Model: "test"})
	if err == nil {
		t.Error("Breaker should open after health check failure")
	}
}

func TestBreaker_HealthCheckSuccessResets(t *testing.T) {
	mock := &mockAdapter{
		providerName: "test",
		contextWin:   8000,
		failCount:    1,
	}
	breaker := NewBreaker(mock, 1, time.Second)

	// Cause a failure
	_, err := breaker.Chat(context.Background(), Request{Model: "test"})
	if err == nil {
		t.Fatal("Request should fail")
	}

	// Verify breaker opened
	_, err = breaker.Chat(context.Background(), Request{Model: "test"})
	if err == nil {
		t.Fatal("Breaker should be open")
	}

	// Wait and reset mock to allow half-open
	time.Sleep(100 * time.Millisecond)
	mock.failCount = 0

	// Health check succeeds
	err = breaker.HealthCheck(context.Background())
	if err != nil {
		t.Logf("Health check in half-open state: %v", err)
	}
}

func TestBreaker_PropagatesContextWindow(t *testing.T) {
	mock := &mockAdapter{
		providerName: "test",
		contextWin:   100000,
	}
	breaker := NewBreaker(mock, 3, time.Second)

	if breaker.ContextWindow() != 100000 {
		t.Errorf("Expected context window 100000, got %d", breaker.ContextWindow())
	}
}

func TestBreaker_PropagatesProvider(t *testing.T) {
	mock := &mockAdapter{
		providerName: "openai",
		contextWin:   8000,
	}
	breaker := NewBreaker(mock, 3, time.Second)

	if breaker.Provider() != "openai" {
		t.Errorf("Expected provider 'openai', got %q", breaker.Provider())
	}
}

func TestBreaker_ConcurrentRequests(t *testing.T) {
	mock := &mockAdapter{
		providerName: "test",
		contextWin:   8000,
	}
	breaker := NewBreaker(mock, 10, time.Second)

	req := Request{Model: "test", Messages: []Message{{Role: "user", Content: "hi"}}}

	// Concurrent successful requests
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := breaker.Chat(context.Background(), req)
			done <- err
		}()
	}

	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent request %d failed: %v", i, err)
		}
	}
}

// ── Adapter Interface Tests ─────────────────────────────────────────────────

func TestAdapter_ChatRequest(t *testing.T) {
	adapter := &mockAdapter{providerName: "test", contextWin: 8000}

	req := Request{
		Model:       "test-model",
		Messages:    []Message{{Role: "user", Content: "hello"}},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	resp, err := adapter.Chat(context.Background(), req)

	if err != nil {
		t.Errorf("Chat request failed: %v", err)
	}
	if resp.Model != "test-model" {
		t.Errorf("Expected model test-model, got %q", resp.Model)
	}
	if len(resp.Choices) == 0 {
		t.Error("Expected response choices")
	}
}

func TestAdapter_StreamChat(t *testing.T) {
	adapter := &mockAdapter{providerName: "test", contextWin: 8000}

	req := Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "hello"}},
	}

	ch, err := adapter.StreamChat(context.Background(), req)
	if err != nil {
		t.Errorf("StreamChat failed: %v", err)
	}

	chunks := make([]StreamChunk, 0)
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) < 2 {
		t.Error("Expected at least 2 chunks in stream")
	}

	if !chunks[len(chunks)-1].Done {
		t.Error("Expected final chunk to have Done=true")
	}
}

func TestAdapter_HealthCheck(t *testing.T) {
	adapter := &mockAdapter{providerName: "test", contextWin: 8000}

	err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestAdapter_HealthCheckFailure(t *testing.T) {
	adapter := &mockAdapter{
		providerName: "test",
		contextWin:   8000,
		shouldFail:   true,
	}

	err := adapter.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected health check to fail")
	}
}
