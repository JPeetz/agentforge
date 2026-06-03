package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ── Envelope Tests ──────────────────────────────────────────────────────────

func TestEnvelope_MessageKinds(t *testing.T) {
	kinds := []MessageKind{KindCommand, KindEvent, KindQuery, KindResponse, KindHeartbeat}
	for _, k := range kinds {
		if k == "" {
			t.Errorf("MessageKind should not be empty")
		}
	}
}

// ── Basic Pub/Sub Tests ─────────────────────────────────────────────────────

func TestBus_Publish_Subscribe(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	ch, err := bus.Subscribe("test-topic", DefaultFilter)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	env := Envelope{
		Source: "publisher",
		Target: "subscriber",
		Kind:   KindEvent,
		Topic:  "test-topic",
	}

	go bus.Publish(context.Background(), env)

	select {
	case received := <-ch:
		if received.Source != "publisher" {
			t.Errorf("Expected source 'publisher', got %q", received.Source)
		}
		if received.Topic != "test-topic" {
			t.Errorf("Expected topic 'test-topic', got %q", received.Topic)
		}
		if received.ID == "" {
			t.Error("Expected auto-generated ID")
		}
		if received.Timestamp.IsZero() {
			t.Error("Expected auto-generated timestamp")
		}
	case <-time.After(time.Second):
		t.Fatal("Publish/Subscribe timeout")
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	// 5 subscribers on same topic
	channels := make([]<-chan Envelope, 5)
	for i := 0; i < 5; i++ {
		ch, err := bus.Subscribe("multi-topic", DefaultFilter)
		if err != nil {
			t.Fatalf("Subscribe %d failed: %v", i, err)
		}
		channels[i] = ch
	}

	env := Envelope{
		Source: "publisher",
		Kind:   KindEvent,
		Topic:  "multi-topic",
		Payload: json.RawMessage(`{"data":"test"}`),
	}

	bus.Publish(context.Background(), env)

	// All subscribers should receive
	for i, ch := range channels {
		select {
		case received := <-ch:
			if received.Topic != "multi-topic" {
				t.Errorf("Subscriber %d got wrong topic", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("Subscriber %d timeout", i)
		}
	}
}

func TestBus_FilteredSubscription(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	// Subscribe with filter: only accept "command" kind
	ch, err := bus.Subscribe("filtered-topic", func(e Envelope) bool {
		return e.Kind == KindCommand
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish command (should pass)
	commandEnv := Envelope{
		Kind:  KindCommand,
		Topic: "filtered-topic",
	}
	bus.Publish(context.Background(), commandEnv)

	// Publish event (should not pass)
	eventEnv := Envelope{
		Kind:  KindEvent,
		Topic: "filtered-topic",
	}
	bus.Publish(context.Background(), eventEnv)

	// Should receive command
	select {
	case received := <-ch:
		if received.Kind != KindCommand {
			t.Errorf("Expected KindCommand, got %v", received.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("Command timeout")
	}

	// Should not receive event (timeout expected)
	select {
	case <-ch:
		t.Error("Should not receive event")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestBus_TopicIsolation(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	ch1, _ := bus.Subscribe("topic-1", DefaultFilter)
	ch2, _ := bus.Subscribe("topic-2", DefaultFilter)

	env1 := Envelope{Topic: "topic-1"}
	env2 := Envelope{Topic: "topic-2"}

	bus.Publish(context.Background(), env1)
	bus.Publish(context.Background(), env2)

	// ch1 should receive only topic-1
	select {
	case received := <-ch1:
		if received.Topic != "topic-1" {
			t.Errorf("ch1 got wrong topic: %q", received.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("ch1 timeout")
	}

	// ch2 should receive only topic-2
	select {
	case received := <-ch2:
		if received.Topic != "topic-2" {
			t.Errorf("ch2 got wrong topic: %q", received.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("ch2 timeout")
	}
}

// ── Request/Response Tests ──────────────────────────────────────────────────

func TestBus_RequestResponse(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	// Responder: subscribe to query topic to receive requests
	queryTopic := "some.query"
	queryChForResponder, err := bus.Subscribe(queryTopic, DefaultFilter)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Simulate responder (must happen before/during request)
	go func() {
		select {
		case env := <-queryChForResponder:
			// Respond on the response topic with matching ID
			response := Envelope{
				ID:     env.ID,
				Source: "agent-1",
				Kind:   KindResponse,
				Topic:  fmt.Sprintf("agent.%s.response", env.Target),
			}
			bus.Publish(context.Background(), response)
		case <-time.After(2 * time.Second):
		}
	}()

	// Give responder time to subscribe
	time.Sleep(50 * time.Millisecond)

	// Requester makes a request
	reqEnv := Envelope{
		Source: "requester",
		Target: "agent-1",
		Kind:   KindQuery,
		Topic:  queryTopic,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := bus.Request(ctx, reqEnv, time.Second)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.Kind != KindResponse {
		t.Errorf("Expected KindResponse, got %v", resp.Kind)
	}
	if resp.Source != "agent-1" {
		t.Errorf("Expected source 'agent-1', got %q", resp.Source)
	}
}

func TestBus_RequestTimeout(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	reqEnv := Envelope{
		Source: "requester",
		Target: "nonexistent",
		Kind:   KindQuery,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := bus.Request(ctx, reqEnv, 50*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestBus_RequestContextCancellation(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	reqEnv := Envelope{
		Source: "requester",
		Target: "nonexistent",
		Kind:   KindQuery,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := bus.Request(ctx, reqEnv, time.Second)
	if err == nil {
		t.Error("Expected context cancellation error")
	}
}

// ── Broadcast Tests ─────────────────────────────────────────────────────────

func TestBus_Broadcast(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	ch, _ := bus.Subscribe("broadcast-topic", DefaultFilter)

	payload := map[string]string{"key": "value"}
	err := bus.Broadcast(context.Background(), "broadcast-topic", payload)
	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	select {
	case env := <-ch:
		if env.Topic != "broadcast-topic" {
			t.Errorf("Expected topic 'broadcast-topic', got %q", env.Topic)
		}
		if env.Source != "system" {
			t.Errorf("Expected source 'system', got %q", env.Source)
		}
		if env.Kind != KindEvent {
			t.Errorf("Expected KindEvent, got %v", env.Kind)
		}

		// Verify payload was marshaled
		var received map[string]string
		err := json.Unmarshal(env.Payload, &received)
		if err != nil {
			t.Fatalf("Failed to unmarshal payload: %v", err)
		}
		if received["key"] != "value" {
			t.Errorf("Expected key='value', got key=%q", received["key"])
		}
	case <-time.After(time.Second):
		t.Fatal("Broadcast timeout")
	}
}

func TestBus_BroadcastMultipleSubscribers(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	// 3 subscribers on different topics
	ch1, _ := bus.Subscribe("broadcast-topic", DefaultFilter)
	ch2, _ := bus.Subscribe("broadcast-topic", DefaultFilter)
	ch3, _ := bus.Subscribe("broadcast-topic", DefaultFilter)

	payload := map[string]int{"count": 3}
	bus.Broadcast(context.Background(), "broadcast-topic", payload)

	for i, ch := range []<-chan Envelope{ch1, ch2, ch3} {
		select {
		case env := <-ch:
			if env.Topic != "broadcast-topic" {
				t.Errorf("Subscriber %d got wrong topic", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("Subscriber %d timeout", i)
		}
	}
}

// ── Close and Cleanup Tests ─────────────────────────────────────────────────

func TestBus_CloseClosesChannels(t *testing.T) {
	bus := NewLocal()

	ch1, _ := bus.Subscribe("topic-1", DefaultFilter)
	ch2, _ := bus.Subscribe("topic-2", DefaultFilter)

	err := bus.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Channels should be closed
	select {
	case _, ok := <-ch1:
		if ok {
			t.Error("ch1 should be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch1 closure timeout")
	}

	select {
	case _, ok := <-ch2:
		if ok {
			t.Error("ch2 should be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch2 closure timeout")
	}
}

func TestBus_DoubleClose(t *testing.T) {
	bus := NewLocal()
	bus.Subscribe("topic", DefaultFilter)

	err1 := bus.Close()
	err2 := bus.Close()

	if err1 != nil {
		t.Errorf("First Close failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second Close failed: %v", err2)
	}
}

func TestBus_PublishAfterClose(t *testing.T) {
	bus := NewLocal()
	bus.Close()

	// Should not panic or error - just be a no-op
	env := Envelope{Topic: "test"}
	bus.Publish(context.Background(), env) // Should not panic
}

// ── Concurrent Tests ────────────────────────────────────────────────────────

func TestBus_ConcurrentPublishers(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	ch, _ := bus.Subscribe("concurrent-topic", DefaultFilter)

	numPublishers := 10
	messagesPerPublisher := 10

	// Start publishers
	for p := 0; p < numPublishers; p++ {
		go func(publisherID int) {
			for m := 0; m < messagesPerPublisher; m++ {
				env := Envelope{
					Source: "publisher",
					Topic:  "concurrent-topic",
				}
				bus.Publish(context.Background(), env)
			}
		}(p)
	}

	// Receive all messages
	expectedTotal := numPublishers * messagesPerPublisher
	received := 0

	timeout := time.After(5 * time.Second)
	for received < expectedTotal {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Fatalf("Timeout: received %d of %d messages", received, expectedTotal)
		}
	}

	if received != expectedTotal {
		t.Errorf("Expected %d messages, got %d", expectedTotal, received)
	}
}

func TestBus_ConcurrentSubscribers(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	numSubscribers := 10
	var channels []<-chan Envelope
	var mu sync.Mutex

	// Create subscribers concurrently
	done := make(chan bool, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		go func() {
			ch, err := bus.Subscribe("concurrent-subscribers", DefaultFilter)
			if err != nil {
				t.Errorf("Subscribe failed: %v", err)
			}
			mu.Lock()
			channels = append(channels, ch)
			mu.Unlock()
			done <- true
		}()
	}

	// Wait for all subscribers
	for i := 0; i < numSubscribers; i++ {
		<-done
	}

	// Publish to all
	for i := 0; i < 5; i++ {
		env := Envelope{Topic: "concurrent-subscribers"}
		bus.Publish(context.Background(), env)
	}

	// Verify all received
	mu.Lock()
	allChannels := channels
	mu.Unlock()

	for i, ch := range allChannels {
		received := 0
		timeout := time.After(time.Second)
		for received < 5 {
			select {
			case <-ch:
				received++
			case <-timeout:
				t.Fatalf("Subscriber %d timeout at %d/5 messages", i, received)
			}
		}
	}
}

func TestBus_ConcurrentPublishSubscribe(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	messageCount := atomic.Int64{}
	numWorkers := 5

	// Concurrent publishers and subscribers
	done := make(chan bool, numWorkers*2)

	for w := 0; w < numWorkers; w++ {
		// Publisher
		go func(workerID int) {
			for i := 0; i < 20; i++ {
				env := Envelope{
					Source: "publisher",
					Topic:  "mixed",
				}
				bus.Publish(context.Background(), env)
				time.Sleep(time.Millisecond)
			}
			done <- true
		}(w)

		// Subscriber
		go func(workerID int) {
			ch, _ := bus.Subscribe("mixed", DefaultFilter)
			for i := 0; i < 20; i++ {
				select {
				case <-ch:
					messageCount.Add(1)
				case <-time.After(2 * time.Second):
					break
				}
			}
			done <- true
		}(w)
	}

	// Wait for all workers
	for i := 0; i < numWorkers*2; i++ {
		<-done
	}

	// Note: not all published messages may be received due to timing,
	// but at least some should be
	if messageCount.Load() == 0 {
		t.Error("Expected to receive at least some messages")
	}
}

func TestBus_RaceConditionUnsubscribe(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	// This test doesn't verify the result, just ensures no data races
	ch, _ := bus.Subscribe("race-test", DefaultFilter)

	done := make(chan bool, 10)

	// Concurrent operations
	for i := 0; i < 10; i++ {
		go func() {
			env := Envelope{Topic: "race-test"}
			bus.Publish(context.Background(), env)
			done <- true
		}()

		go func() {
			select {
			case <-ch:
			case <-time.After(100 * time.Millisecond):
			}
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

// ── Integration Tests ───────────────────────────────────────────────────────

func TestBus_FullWorkflow(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	// Multiple topics and subscribers
	eventCh, _ := bus.Subscribe("events", DefaultFilter)
	commandCh, _ := bus.Subscribe("commands", DefaultFilter)
	responseCh, _ := bus.Subscribe("agent.responder.response", DefaultFilter)

	// Publish various message types
	go func() {
		bus.Publish(context.Background(), Envelope{Topic: "events", Kind: KindEvent})
		bus.Publish(context.Background(), Envelope{Topic: "commands", Kind: KindCommand})
		bus.Publish(context.Background(), Envelope{
			ID:    "req-123",
			Topic: "agent.responder.response",
			Kind:  KindResponse,
		})
	}()

	// Verify event
	select {
	case env := <-eventCh:
		if env.Kind != KindEvent {
			t.Error("Expected KindEvent")
		}
	case <-time.After(time.Second):
		t.Fatal("Event timeout")
	}

	// Verify command
	select {
	case env := <-commandCh:
		if env.Kind != KindCommand {
			t.Error("Expected KindCommand")
		}
	case <-time.After(time.Second):
		t.Fatal("Command timeout")
	}

	// Verify response
	select {
	case env := <-responseCh:
		if env.Kind != KindResponse {
			t.Error("Expected KindResponse")
		}
	case <-time.After(time.Second):
		t.Fatal("Response timeout")
	}
}

func TestBus_MessageIDUniqueness(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	ch, _ := bus.Subscribe("id-test", DefaultFilter)

	// Publish 100 messages without explicit ID
	for i := 0; i < 100; i++ {
		env := Envelope{Topic: "id-test"}
		bus.Publish(context.Background(), env)
	}

	// Collect IDs
	idMap := make(map[string]bool)
	timeout := time.After(5 * time.Second)
	for len(idMap) < 100 {
		select {
		case env := <-ch:
			if idMap[env.ID] {
				t.Fatalf("Duplicate ID: %s", env.ID)
			}
			idMap[env.ID] = true
		case <-timeout:
			t.Fatalf("Timeout: received %d IDs", len(idMap))
		}
	}
}

func TestBus_PayloadSerialization(t *testing.T) {
	bus := NewLocal()
	defer bus.Close()

	ch, _ := bus.Subscribe("payload-test", DefaultFilter)

	type TestPayload struct {
		Name  string
		Count int
		Data  []string
	}

	payload := TestPayload{
		Name:  "test",
		Count: 42,
		Data:  []string{"a", "b", "c"},
	}

	data, _ := json.Marshal(payload)
	env := Envelope{
		Topic:   "payload-test",
		Payload: data,
	}

	bus.Publish(context.Background(), env)

	select {
	case received := <-ch:
		var decoded TestPayload
		err := json.Unmarshal(received.Payload, &decoded)
		if err != nil {
			t.Fatalf("Failed to decode payload: %v", err)
		}
		if decoded.Name != "test" || decoded.Count != 42 {
			t.Errorf("Payload mismatch: %+v", decoded)
		}
	case <-time.After(time.Second):
		t.Fatal("Payload test timeout")
	}
}
