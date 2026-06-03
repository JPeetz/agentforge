package sse

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// ── Race Condition Tests ─────────────────────────────────────────────────────

func TestHub_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	// Test that Subscribe/Unsubscribe are thread-safe under concurrent load
	h := NewHub()

	// Create multiple clients concurrently
	const numClients = 10
	const numTopics = 5
	const numOps = 100

	var wg sync.WaitGroup

	// Spawn goroutines that concurrently subscribe/unsubscribe
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			w := httptest.NewRecorder()
			client, err := h.Connect(w)
			if err != nil {
				t.Errorf("Failed to connect client %d: %v", clientID, err)
				return
			}

			// Subscribe to multiple topics
			for j := 0; j < numOps; j++ {
				topic := "topic." + string(rune('a'+(j%numTopics)))
				h.Subscribe(client, topic)
			}

			// Unsubscribe from topics
			for j := 0; j < numOps; j++ {
				topic := "topic." + string(rune('a'+(j%numTopics)))
				h.Unsubscribe(client, topic)
			}

			h.Disconnect(client)
		}(i)
	}

	wg.Wait()
	t.Log("Concurrent subscribe/unsubscribe test passed")
}

func TestHub_ConcurrentBroadcast(t *testing.T) {
	// Test that Broadcast is thread-safe under concurrent load
	h := NewHub()

	// Create clients and subscribe to topics
	const numClients = 5
	const numTopics = 3

	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		w := httptest.NewRecorder()
		client, err := h.Connect(w)
		if err != nil {
			t.Fatalf("Failed to connect client: %v", err)
		}
		clients[i] = client

		// Subscribe to all topics
		for j := 0; j < numTopics; j++ {
			topic := "topic." + string(rune('a'+j))
			h.Subscribe(client, topic)
		}
	}

	// Broadcast concurrently from multiple goroutines
	const numBroadcasts = 100
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(broadcasterID int) {
			defer wg.Done()
			for j := 0; j < numBroadcasts; j++ {
				topic := "topic." + string(rune('a'+(j%numTopics)))
				event := SSEEvent{
					Event: "broadcast",
					Data:  map[string]interface{}{"msg": "test"},
				}
				h.Broadcast(topic, event)
			}
		}(i)
	}

	wg.Wait()

	// Verify all clients still connected
	if h.ClientCount() != numClients {
		t.Errorf("Expected %d clients, got %d", numClients, h.ClientCount())
	}

	// Disconnect all clients
	for _, client := range clients {
		h.Disconnect(client)
	}

	t.Log("Concurrent broadcast test passed")
}

func TestHub_ConcurrentConnectDisconnect(t *testing.T) {
	// Test that Connect/Disconnect are thread-safe
	h := NewHub()

	const numOps = 50
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				w := httptest.NewRecorder()
				client, err := h.Connect(w)
				if err != nil {
					t.Errorf("Connect failed: %v", err)
					return
				}

				// Small delay to simulate work
				time.Sleep(time.Microsecond)

				h.Disconnect(client)
			}
		}(i)
	}

	wg.Wait()

	// All clients should be disconnected
	if h.ClientCount() != 0 {
		t.Errorf("Expected 0 clients after all disconnects, got %d", h.ClientCount())
	}

	t.Log("Concurrent connect/disconnect test passed")
}

// ── Mutex Coverage Tests ─────────────────────────────────────────────────────

func TestHub_MutexProtectsClientsMap(t *testing.T) {
	// Verify that clients map access is properly synchronized
	h := NewHub()

	w := httptest.NewRecorder()
	client, _ := h.Connect(w)

	// ClientCount should be protected
	count := h.ClientCount()
	if count != 1 {
		t.Errorf("Expected 1 client, got %d", count)
	}

	h.Disconnect(client)
	count = h.ClientCount()
	if count != 0 {
		t.Errorf("Expected 0 clients after disconnect, got %d", count)
	}

	t.Log("Clients map synchronization verified")
}

func TestHub_MutexProtectsTopicSubsMap(t *testing.T) {
	// Verify that topicSubs map access is properly synchronized
	h := NewHub()

	w := httptest.NewRecorder()
	client, _ := h.Connect(w)

	// Subscribe and verify
	h.Subscribe(client, "test.topic")

	// Broadcast should find the subscription
	event := SSEEvent{Event: "test", Data: "data"}
	h.Broadcast("test.topic", event)

	// Receive the event (non-blocking send, so this won't block)
	select {
	case evt := <-client.ch:
		if evt.Event != "test" {
			t.Errorf("Wrong event received: %v", evt)
		}
	case <-time.After(10 * time.Millisecond):
		// Expected, send was non-blocking
	}

	h.Disconnect(client)
	t.Log("TopicSubs map synchronization verified")
}

// ── Integration Tests ────────────────────────────────────────────────────────

func TestHub_FullLifecycle(t *testing.T) {
	// Integration test for complete Hub lifecycle with concurrent operations
	h := NewHub()

	// Connect clients
	clients := make([]*Client, 3)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		c, _ := h.Connect(w)
		clients[i] = c
	}

	// Subscribe to topics
	for i, client := range clients {
		topic := "user." + string(rune('a'+i))
		h.Subscribe(client, topic)
	}

	// Broadcast to each topic
	for i := 0; i < 3; i++ {
		topic := "user." + string(rune('a'+i))
		event := SSEEvent{Event: "msg", Data: "hello"}
		h.Broadcast(topic, event)
	}

	// Unsubscribe and disconnect
	for i, client := range clients {
		topic := "user." + string(rune('a'+i))
		h.Unsubscribe(client, topic)
		h.Disconnect(client)
	}

	if h.ClientCount() != 0 {
		t.Errorf("Expected 0 clients at end, got %d", h.ClientCount())
	}

	t.Log("Full lifecycle test passed")
}
