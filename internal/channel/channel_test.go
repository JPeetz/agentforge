package channel

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/config"
)

// ── Status Tests ────────────────────────────────────────────────────────────

func TestStatus_Structure(t *testing.T) {
	status := Status{
		Name:    "test-channel",
		Running: true,
		Connects: 5,
		Messages: 42,
		LastMsg: time.Now(),
	}

	if status.Name != "test-channel" {
		t.Errorf("Expected name 'test-channel', got %q", status.Name)
	}
	if !status.Running {
		t.Error("Expected running to be true")
	}
	if status.Messages != 42 {
		t.Errorf("Expected 42 messages, got %d", status.Messages)
	}
}

// ── Manager Tests ───────────────────────────────────────────────────────────

func TestManager_NewManager(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{Enabled: false},
		Discord:  config.DiscordConfig{Enabled: false},
		Slack:    config.SlackConfig{Enabled: false},
		Signal:   config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
	if mgr.Count() != 0 {
		t.Errorf("Expected 0 adapters, got %d", mgr.Count())
	}
}

func TestManager_WithTelegramEnabled(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Discord: config.DiscordConfig{Enabled: false},
		Slack:   config.SlackConfig{Enabled: false},
		Signal:  config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	if mgr.Count() != 1 {
		t.Errorf("Expected 1 adapter (Telegram), got %d", mgr.Count())
	}
}

func TestManager_WithMultipleChannelsEnabled(t *testing.T) {
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
		Signal: config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	if mgr.Count() != 3 {
		t.Errorf("Expected 3 adapters, got %d", mgr.Count())
	}
}

func TestManager_DisabledChannelsNotAdded(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{Enabled: false},
		Discord:  config.DiscordConfig{Enabled: false},
		Slack:    config.SlackConfig{Enabled: false},
		Signal:   config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	if mgr.Count() != 0 {
		t.Errorf("Expected 0 adapters for disabled channels, got %d", mgr.Count())
	}
}

func TestManager_Status(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Discord: config.DiscordConfig{Enabled: false},
		Slack:   config.SlackConfig{Enabled: false},
		Signal:  config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	statuses := mgr.Status()

	if len(statuses) != 1 {
		t.Errorf("Expected 1 status, got %d", len(statuses))
	}

	if len(statuses) > 0 && statuses[0].Name != "telegram" {
		t.Errorf("Expected name 'telegram', got %q", statuses[0].Name)
	}
}

func TestManager_Count(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Discord: config.DiscordConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Slack:  config.SlackConfig{Enabled: false},
		Signal: config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	count := mgr.Count()

	if count != 2 {
		t.Errorf("Expected 2 adapters, got %d", count)
	}
}

// ── Adapter Interface Tests ─────────────────────────────────────────────────

func TestTelegramAdapter_Name(t *testing.T) {
	cfg := config.TelegramConfig{BotToken: "test-token"}
	adapter := NewTelegramAdapter(cfg)

	if adapter.Name() != "telegram" {
		t.Errorf("Expected name 'telegram', got %q", adapter.Name())
	}
}

func TestSlackAdapter_Name(t *testing.T) {
	cfg := config.SlackConfig{BotToken: "test-token"}
	adapter := NewSlackAdapter(cfg)

	if adapter.Name() != "slack" {
		t.Errorf("Expected name 'slack', got %q", adapter.Name())
	}
}

func TestSignalAdapter_Name(t *testing.T) {
	cfg := config.SignalConfig{}
	adapter := NewSignalAdapter(cfg)

	if adapter.Name() != "signal" {
		t.Errorf("Expected name 'signal', got %q", adapter.Name())
	}
}

func TestDiscordAdapter_Name(t *testing.T) {
	cfg := config.DiscordConfig{BotToken: "test-token"}
	adapter := NewDiscordAdapter(cfg)

	if adapter.Name() != "discord" {
		t.Errorf("Expected name 'discord', got %q", adapter.Name())
	}
}

// ── Adapter Status Tests ────────────────────────────────────────────────────

func TestTelegramAdapter_InitialStatus(t *testing.T) {
	cfg := config.TelegramConfig{BotToken: "test-token"}
	adapter := NewTelegramAdapter(cfg)

	status := adapter.Status()
	if status.Name != "telegram" {
		t.Errorf("Expected name 'telegram', got %q", status.Name)
	}
	if status.Running {
		t.Error("Expected Running to be false initially")
	}
}

func TestSlackAdapter_InitialStatus(t *testing.T) {
	cfg := config.SlackConfig{BotToken: "test-token"}
	adapter := NewSlackAdapter(cfg)

	status := adapter.Status()
	if status.Name != "slack" {
		t.Errorf("Expected name 'slack', got %q", status.Name)
	}
	if status.Running {
		t.Error("Expected Running to be false initially")
	}
}

func TestSignalAdapter_InitialStatus(t *testing.T) {
	cfg := config.SignalConfig{}
	adapter := NewSignalAdapter(cfg)

	status := adapter.Status()
	if status.Name != "signal" {
		t.Errorf("Expected name 'signal', got %q", status.Name)
	}
	if status.Running {
		t.Error("Expected Running to be false initially")
	}
}

func TestDiscordAdapter_InitialStatus(t *testing.T) {
	cfg := config.DiscordConfig{BotToken: "test-token"}
	adapter := NewDiscordAdapter(cfg)

	status := adapter.Status()
	if status.Name != "discord" {
		t.Errorf("Expected name 'discord', got %q", status.Name)
	}
	if status.Running {
		t.Error("Expected Running to be false initially")
	}
}

// ── Manager Lifecycle Tests ─────────────────────────────────────────────────

func TestManager_StartStop(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Discord: config.DiscordConfig{Enabled: false},
		Slack:   config.SlackConfig{Enabled: false},
		Signal:  config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	mockBus := bus.NewLocal()
	defer mockBus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Note: Start may fail due to invalid bot token, which is expected in tests
	// We're just testing the manager logic, not actual bot connectivity
	_ = mgr.Start(ctx, mockBus)

	// Stop should not panic
	mgr.Stop()
}

// ── Message Type Tests ──────────────────────────────────────────────────────

func TestTelegramUpdate_Parsing(t *testing.T) {
	update := &tgUpdate{
		UpdateID: 12345,
		Message: &tgMessage{
			MessageID: 1,
			Text:      "hello",
			Date:      time.Now().Unix(),
			From: &tgUser{
				ID:        999,
				FirstName: "Test",
				Username:  "testuser",
			},
			Chat: &tgChat{
				ID:   -1001234567890,
				Type: "group",
			},
		},
	}

	if update.UpdateID != 12345 {
		t.Errorf("Expected UpdateID 12345, got %d", update.UpdateID)
	}
	if update.Message.Text != "hello" {
		t.Errorf("Expected message 'hello', got %q", update.Message.Text)
	}
	if update.Message.From.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %q", update.Message.From.Username)
	}
}

func TestSlackMessage_Parsing(t *testing.T) {
	msg := &slackMessageEvent{
		Type:    "message",
		User:    "U12345",
		Channel: "C67890",
		Text:    "hello slack",
		TS:      "1234567890.000001",
	}

	if msg.Type != "message" {
		t.Errorf("Expected type 'message', got %q", msg.Type)
	}
	if msg.Text != "hello slack" {
		t.Errorf("Expected text 'hello slack', got %q", msg.Text)
	}
}

func TestSignalMessage_Parsing(t *testing.T) {
	msg := &signalDataMessage{
		Timestamp: 1234567890000,
		Message:   "hello signal",
	}

	if msg.Message != "hello signal" {
		t.Errorf("Expected message 'hello signal', got %q", msg.Message)
	}
}

// ── JSON-RPC Message Tests ──────────────────────────────────────────────────

func TestSignalRPCRequest_Creation(t *testing.T) {
	req := &signalRPCRequest{
		JSONRPC: "2.0",
		Method:  "send",
		Params:  map[string]string{"message": "test"},
		ID:      1,
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %q", req.JSONRPC)
	}
	if req.Method != "send" {
		t.Errorf("Expected method 'send', got %q", req.Method)
	}
}

// ── Connection Info Tests ───────────────────────────────────────────────────

func TestSlackConnectionInfo_Parsing(t *testing.T) {
	info := &slackConnectionInfo{
		AppID: "A12345",
	}

	if info.AppID != "A12345" {
		t.Errorf("Expected AppID 'A12345', got %q", info.AppID)
	}
}

// ── Concurrent Manager Tests ────────────────────────────────────────────────

func TestManager_ConcurrentStatus(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Discord: config.DiscordConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Slack:  config.SlackConfig{Enabled: false},
		Signal: config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)

	done := make(chan bool, 5)

	// Concurrent status reads
	for i := 0; i < 5; i++ {
		go func() {
			_ = mgr.Status()
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestManager_ConcurrentCount(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Discord: config.DiscordConfig{Enabled: false},
		Slack:   config.SlackConfig{Enabled: false},
		Signal:  config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)

	done := make(chan bool, 10)

	// Concurrent count reads
	for i := 0; i < 10; i++ {
		go func() {
			count := mgr.Count()
			if count != 1 {
				t.Errorf("Expected 1 adapter, got %d", count)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ── Configuration Tests ─────────────────────────────────────────────────────

func TestManager_TelegramDisabledWithoutToken(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{
			Enabled:  true,
			BotToken: "", // Empty token
		},
		Discord: config.DiscordConfig{Enabled: false},
		Slack:   config.SlackConfig{Enabled: false},
		Signal:  config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	if mgr.Count() != 0 {
		t.Error("Telegram adapter should not be added without token")
	}
}

func TestManager_SlackDisabledWithoutToken(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{Enabled: false},
		Discord:  config.DiscordConfig{Enabled: false},
		Slack: config.SlackConfig{
			Enabled:  true,
			BotToken: "", // Empty token
		},
		Signal: config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	if mgr.Count() != 0 {
		t.Error("Slack adapter should not be added without token")
	}
}

func TestManager_DiscordDisabledWithoutToken(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{Enabled: false},
		Discord: config.DiscordConfig{
			Enabled:  true,
			BotToken: "", // Empty token
		},
		Slack:  config.SlackConfig{Enabled: false},
		Signal: config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)
	if mgr.Count() != 0 {
		t.Error("Discord adapter should not be added without token")
	}
}

// ── Adapter Instantiation Tests ─────────────────────────────────────────────

func TestAdapterInstantiation_Telegram(t *testing.T) {
	cfg := config.TelegramConfig{BotToken: "test-token"}
	adapter := NewTelegramAdapter(cfg)

	if adapter == nil {
		t.Fatal("Telegram adapter should not be nil")
	}

	status := adapter.Status()
	if status.Name != "telegram" {
		t.Error("Telegram adapter should have correct name")
	}
}

func TestAdapterInstantiation_Slack(t *testing.T) {
	cfg := config.SlackConfig{BotToken: "test-token"}
	adapter := NewSlackAdapter(cfg)

	if adapter == nil {
		t.Fatal("Slack adapter should not be nil")
	}

	status := adapter.Status()
	if status.Name != "slack" {
		t.Error("Slack adapter should have correct name")
	}
}

func TestAdapterInstantiation_Signal(t *testing.T) {
	cfg := config.SignalConfig{}
	adapter := NewSignalAdapter(cfg)

	if adapter == nil {
		t.Fatal("Signal adapter should not be nil")
	}

	status := adapter.Status()
	if status.Name != "signal" {
		t.Error("Signal adapter should have correct name")
	}
}

func TestAdapterInstantiation_Discord(t *testing.T) {
	cfg := config.DiscordConfig{BotToken: "test-token"}
	adapter := NewDiscordAdapter(cfg)

	if adapter == nil {
		t.Fatal("Discord adapter should not be nil")
	}

	status := adapter.Status()
	if status.Name != "discord" {
		t.Error("Discord adapter should have correct name")
	}
}

// ── Integration Tests ───────────────────────────────────────────────────────

func TestChannelManager_FullLifecycle(t *testing.T) {
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
		Signal: config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)

	// Verify all adapters initialized
	if mgr.Count() != 3 {
		t.Errorf("Expected 3 adapters, got %d", mgr.Count())
	}

	// Verify status for all adapters
	statuses := mgr.Status()
	if len(statuses) != 3 {
		t.Errorf("Expected 3 statuses, got %d", len(statuses))
	}

	// Verify adapter names
	names := make(map[string]bool)
	for _, s := range statuses {
		names[s.Name] = true
	}

	if !names["telegram"] {
		t.Error("Missing telegram adapter")
	}
	if !names["discord"] {
		t.Error("Missing discord adapter")
	}
	if !names["slack"] {
		t.Error("Missing slack adapter")
	}
}

func TestChannelManager_SelectiveEnable(t *testing.T) {
	cfg := &config.ChannelsConfig{
		Telegram: config.TelegramConfig{Enabled: false},
		Discord:  config.DiscordConfig{Enabled: false},
		Slack: config.SlackConfig{
			Enabled:  true,
			BotToken: "test-token",
		},
		Signal: config.SignalConfig{Enabled: false},
	}

	mgr := NewManager(cfg)

	if mgr.Count() != 1 {
		t.Errorf("Expected 1 adapter, got %d", mgr.Count())
	}

	statuses := mgr.Status()
	if len(statuses) > 0 && statuses[0].Name != "slack" {
		t.Error("Expected only slack adapter")
	}
}
