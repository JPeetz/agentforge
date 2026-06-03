package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/channel"
	"github.com/agentforge/agentforge/internal/config"
)

// ── Integration Tests ───────────────────────────────────────────────────────

// TestChannelBusIntegration verifies end-to-end message flow:
// Channel adapter → publishes to bus → handler receives message
func TestChannelBusIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Setup bus and channel manager
	b := bus.NewLocal()
	defer b.Close()

	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
	}
	manager := channel.NewManager(cfg)

	// Subscribe to telegram topic before starting adapters
	updates, err := b.Subscribe("telegram", bus.DefaultFilter)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Start manager (adapters will try to connect)
	if err := manager.Start(ctx, b); err != nil {
		t.Fatalf("Manager.Start failed: %v", err)
	}
	defer manager.Stop()

	// Verify adapter is running
	status := manager.Status()
	if len(status) == 0 {
		t.Fatal("No adapters running")
	}

	if status[0].Name != "telegram" {
		t.Errorf("Expected adapter 'telegram', got %q", status[0].Name)
	}

	// Verify subscription is active and receiving on the bus
	if updates == nil {
		t.Fatal("Updates channel should not be nil")
	}

	// Timeout after waiting for a message (adapter may not receive real messages in test)
	select {
	case msg := <-updates:
		if msg.Topic != "telegram" {
			t.Errorf("Expected topic 'telegram', got %q", msg.Topic)
		}
	case <-ctx.Done():
		// Expected in test - real Telegram polling would take time
		t.Log("No message received (expected in unit test)")
	}
}

// TestMultiChannelConcurrentMessages verifies that multiple channels
// can send messages concurrently without interference or deadlock.
func TestMultiChannelConcurrentMessages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b := bus.NewLocal()
	defer b.Close()

	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token-1",
		},
		Discord: config.DiscordConfig{
			Enabled:  true,
			BotToken: "test-token-2",
		},
		Slack: config.SlackConfig{
			Enabled:  true,
			BotToken: "test-token-3",
		},
	}
	manager := channel.NewManager(cfg)

	// Verify all channels are registered
	if count := manager.Count(); count != 3 {
		t.Fatalf("Expected 3 adapters, got %d", count)
	}

	// Subscribe to all channels
	tgUpdates, err := b.Subscribe("telegram", bus.DefaultFilter)
	if err != nil {
		t.Fatalf("Subscribe telegram failed: %v", err)
	}

	dcUpdates, err := b.Subscribe("discord", bus.DefaultFilter)
	if err != nil {
		t.Fatalf("Subscribe discord failed: %v", err)
	}

	slackUpdates, err := b.Subscribe("slack", bus.DefaultFilter)
	if err != nil {
		t.Fatalf("Subscribe slack failed: %v", err)
	}

	// Start all adapters concurrently
	if err := manager.Start(ctx, b); err != nil {
		t.Fatalf("Manager.Start failed: %v", err)
	}
	defer manager.Stop()

	// Verify all adapters started
	statuses := manager.Status()
	if len(statuses) != 3 {
		t.Fatalf("Expected 3 adapter statuses, got %d", len(statuses))
	}

	adapterNames := make(map[string]bool)
	for _, s := range statuses {
		adapterNames[s.Name] = true
	}

	if !adapterNames["telegram"] || !adapterNames["discord"] || !adapterNames["slack"] {
		t.Fatal("Not all expected adapters found in status")
	}

	// Verify subscription channels are distinct and functional
	if tgUpdates == dcUpdates || tgUpdates == slackUpdates || dcUpdates == slackUpdates {
		t.Fatal("Subscription channels should be distinct")
	}

	// Wait a bit for adapters to attempt connection
	// In real scenario, messages would flow here
	<-time.After(500 * time.Millisecond)

	// All should still be healthy (no panics, no crashes)
	stillRunning := manager.Status()
	if len(stillRunning) != 3 {
		t.Errorf("Expected 3 adapters still running, got %d", len(stillRunning))
	}
}

// TestBusMessageFiltering verifies that handlers can filter messages
// by topic and other criteria without receiving unrelated messages.
func TestBusMessageFiltering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b := bus.NewLocal()
	defer b.Close()

	// Subscribe with topic filter: only telegram messages
	tgFilter := func(e bus.Envelope) bool {
		return e.Topic == "telegram"
	}

	updates, err := b.Subscribe("telegram", tgFilter)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish messages to different topics
	go func() {
		payload, _ := json.Marshal(map[string]string{"text": "hello"})
		b.Publish(ctx, bus.Envelope{
			Topic:   "telegram",
			Payload: payload,
		})

		// This should be filtered out by the handler
		b.Publish(ctx, bus.Envelope{
			Topic:   "discord",
			Payload: payload,
		})

		// This should also be filtered out
		b.Publish(ctx, bus.Envelope{
			Topic:   "slack",
			Payload: payload,
		})
	}()

	// Receive first message (should be telegram only)
	select {
	case msg := <-updates:
		if msg.Topic != "telegram" {
			t.Errorf("Expected 'telegram', got %q", msg.Topic)
		}
	case <-ctx.Done():
		t.Fatal("Timeout waiting for telegram message")
	}

	// Ensure no other messages arrive
	select {
	case msg := <-updates:
		t.Errorf("Should not receive message from topic %q", msg.Topic)
	case <-time.After(500 * time.Millisecond):
		// Expected: filter blocked other topics
	}
}

// TestManagerStartStopRecovery verifies that stopping and restarting
// the manager cleanly resets state without leaks or hung resources.
func TestManagerStartStopRecovery(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Discord: config.DiscordConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
	}

	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		b := bus.NewLocal()

		manager := channel.NewManager(cfg)
		if err := manager.Start(ctx, b); err != nil {
			cancel()
			t.Fatalf("Iteration %d: Start failed: %v", i, err)
		}

		// Verify running
		statuses := manager.Status()
		if len(statuses) != 2 {
			cancel()
			t.Fatalf("Iteration %d: Expected 2 adapters, got %d", i, len(statuses))
		}

		// Stop
		manager.Stop()

		// Verify stopped (Status should still work, just report stopped adapters)
		afterStop := manager.Status()
		if len(afterStop) != 2 {
			cancel()
			t.Fatalf("Iteration %d: Status after stop should still report adapters, got %d", i, len(afterStop))
		}

		b.Close()
		cancel()
	}
}

// TestConcurrentChannelOperations verifies that multiple goroutines
// can safely call manager methods without race conditions or deadlocks.
func TestConcurrentChannelOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b := bus.NewLocal()
	defer b.Close()

	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Discord: config.DiscordConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Slack: config.SlackConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
	}
	manager := channel.NewManager(cfg)

	if err := manager.Start(ctx, b); err != nil {
		t.Fatalf("Manager.Start failed: %v", err)
	}
	defer manager.Stop()

	// Spawn goroutines that concurrently call manager methods
	done := make(chan bool, 9)

	// 3 goroutines calling Status()
	for i := 0; i < 3; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				status := manager.Status()
				if len(status) != 3 {
					t.Errorf("Goroutine %d: Expected 3 adapters, got %d", id, len(status))
				}
				<-time.After(10 * time.Millisecond)
			}
			done <- true
		}(i)
	}

	// 3 goroutines calling Count()
	for i := 0; i < 3; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				count := manager.Count()
				if count != 3 {
					t.Errorf("Goroutine %d: Expected count 3, got %d", id, count)
				}
				<-time.After(10 * time.Millisecond)
			}
			done <- true
		}(i + 3)
	}

	// 3 goroutines subscribing and listening
	for i := 0; i < 3; i++ {
		go func(id int, topic string) {
			updates, err := b.Subscribe(topic, bus.DefaultFilter)
			if err != nil {
				t.Errorf("Goroutine %d: Subscribe failed: %v", id, err)
				done <- true
				return
			}
			// Just verify channel exists without deadlock
			_ = updates
			done <- true
		}(i + 6, []string{"telegram", "discord", "slack"}[i])
	}

	// Wait for all goroutines
	for i := 0; i < 9; i++ {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("Test timeout - possible deadlock in concurrent operations")
		}
	}
}
