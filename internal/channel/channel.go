// Package channel — messaging channel adapters for AgentForge.
// Provides Telegram (long-poll) and Discord (WebSocket Gateway) adapters
// using only net/http and encoding/json — no external SDK dependencies.
package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/config"
)

// ── Interface ───────────────────────────────────────────────────────────────

// Adapter is a messaging channel that receives inbound messages
// and publishes them on the internal CSP bus.
type Adapter interface {
	Name() string
	Start(ctx context.Context, bus bus.Bus) error
	Stop() error
	Status() Status
}

// Status holds live telemetry for a channel adapter.
type Status struct {
	Name     string    `json:"name"`
	Running  bool      `json:"running"`
	Connects int       `json:"connects"`
	Messages int64     `json:"messages"`
	LastMsg  time.Time `json:"lastMsg"`
}

// ── Manager ─────────────────────────────────────────────────────────────────

// Manager owns the set of channel adapters and wires them to the bus.
type Manager struct {
	adapters map[string]Adapter
	mu       sync.RWMutex
	logger   *slog.Logger
}

// NewManager creates a Manager with adapters for every enabled channel.
func NewManager(cfg *config.ChannelsConfig) *Manager {
	m := &Manager{
		adapters: make(map[string]Adapter),
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" {
		m.adapters["telegram"] = NewTelegramAdapter(cfg.Telegram)
	}
	if cfg.Discord.Enabled && cfg.Discord.BotToken != "" {
		m.adapters["discord"] = NewDiscordAdapter(cfg.Discord)
	}
	if cfg.Slack.Enabled && cfg.Slack.BotToken != "" {
		m.adapters["slack"] = NewSlackAdapter(cfg.Slack)
	}
	if cfg.Signal.Enabled {
		// Signal adapter gracefully skips if signal-cli is not found
		m.adapters["signal"] = NewSignalAdapter(cfg.Signal)
	}

	return m
}

// Start starts every registered adapter and wires them to the bus.
func (m *Manager) Start(ctx context.Context, bus bus.Bus) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, a := range m.adapters {
		if err := a.Start(ctx, bus); err != nil {
			m.logger.Error("channel adapter start failed",
				slog.String("adapter", name),
				slog.Any("error", err),
			)
			return fmt.Errorf("channel %s: %w", name, err)
		}
		m.logger.Info("channel adapter started", slog.String("adapter", name))
	}
	return nil
}

// Stop gracefully stops every adapter.
func (m *Manager) Stop() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, a := range m.adapters {
		if err := a.Stop(); err != nil {
			m.logger.Warn("channel adapter stop", slog.String("adapter", name), slog.Any("error", err))
		}
	}
}

// Status returns live status for all adapters.
func (m *Manager) Status() []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Status, 0, len(m.adapters))
	for _, a := range m.adapters {
		out = append(out, a.Status())
	}
	return out
}

// Count returns the number of registered adapters.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.adapters)
}

// ── Telegram Types ──────────────────────────────────────────────────────────

// tgUpdate is a single Telegram Bot API update.
type tgUpdate struct {
	UpdateID int64     `json:"update_id"`
	Message  *tgMessage `json:"message,omitempty"`
}

// tgMessage is an incoming Telegram message.
type tgMessage struct {
	MessageID int64     `json:"message_id"`
	From      *tgUser   `json:"from,omitempty"`
	Chat      *tgChat   `json:"chat"`
	Text      string    `json:"text,omitempty"`
	Date      int64     `json:"date"`
}

type tgUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

type tgChat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Username string `json:"username,omitempty"`
}

type tgResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
}

type tgGetUpdatesResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

// ── Telegram Adapter ────────────────────────────────────────────────────────

// TelegramAdapter polls the Telegram Bot API with long-polling.
type TelegramAdapter struct {
	cfg        config.TelegramConfig
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	bus        bus.Bus
	client     *http.Client
	connects   atomic.Int64
	messages   atomic.Int64
	lastMsg    time.Time
	lastMu     sync.Mutex
	offset     int64
	logger     *slog.Logger
}

// NewTelegramAdapter creates a Telegram polling adapter.
func NewTelegramAdapter(cfg config.TelegramConfig) *TelegramAdapter {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}
	return &TelegramAdapter{
		cfg:    cfg,
		done:   make(chan struct{}),
		client: &http.Client{Timeout: 35 * time.Second},
		offset: 0,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (tg *TelegramAdapter) Name() string { return "telegram" }

func (tg *TelegramAdapter) Status() Status {
	tg.lastMu.Lock()
	lm := tg.lastMsg
	tg.lastMu.Unlock()

	return Status{
		Name:     "telegram",
		Running:  tg.cancel != nil,
		Connects: int(tg.connects.Load()),
		Messages: tg.messages.Load(),
		LastMsg:  lm,
	}
}

func (tg *TelegramAdapter) Start(ctx context.Context, b bus.Bus) error {
	if tg.cancel != nil {
		return nil // already running
	}
	tg.bus = b
	tg.ctx, tg.cancel = context.WithCancel(ctx)
	tg.done = make(chan struct{})

	go tg.pollLoop()
	return nil
}

func (tg *TelegramAdapter) Stop() error {
	if tg.cancel == nil {
		return nil
	}
	tg.cancel()
	<-tg.done
	tg.cancel = nil
	return nil
}

func (tg *TelegramAdapter) pollLoop() {
	defer close(tg.done)

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-tg.ctx.Done():
			return
		default:
		}

		tg.connects.Add(1)
		updates, err := tg.getUpdates()
		if err != nil {
			tg.logger.Warn("telegram getUpdates failed, backing off",
				slog.String("error", err.Error()),
				slog.Duration("backoff", backoff),
			)
			select {
			case <-time.After(backoff):
			case <-tg.ctx.Done():
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = time.Second // reset on success

		for _, upd := range updates {
			tg.handleUpdate(upd)
			if upd.UpdateID >= tg.offset {
				tg.offset = upd.UpdateID + 1
			}
		}
	}
}

func (tg *TelegramAdapter) getUpdates() ([]tgUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", tg.cfg.BotToken)

	req, err := http.NewRequestWithContext(tg.ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("timeout", "30")
	q.Set("offset", strconv.FormatInt(tg.offset, 10))
	req.URL.RawQuery = q.Encode()

	resp, err := tg.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed tgGetUpdatesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse telegram response: %w", err)
	}
	if !parsed.OK {
		return nil, fmt.Errorf("telegram API error: %s", string(body))
	}

	return parsed.Result, nil
}

func (tg *TelegramAdapter) handleUpdate(upd tgUpdate) {
	if upd.Message == nil || upd.Message.Text == "" {
		return
	}

	// AllowFrom whitelist
	if len(tg.cfg.AllowFrom) > 0 && !tg.isAllowed(upd.Message) {
		tg.logger.Debug("telegram message from non-whitelisted sender, ignoring",
			slog.Int64("chat_id", upd.Message.Chat.ID),
		)
		return
	}

	// Handle slash commands inline
	switch upd.Message.Text {
	case "/start":
		tg.sendReply(upd.Message.Chat.ID, "👋 Hello! I'm AgentForge, an agentic orchestrator. Available commands: /help, /status")
		return
	case "/help":
		tg.sendReply(upd.Message.Chat.ID, "AgentForge Bot Commands:\n/start — welcome message\n/help — this help\n/status — system status")
		return
	case "/status":
		st := tg.Status()
		tg.sendReply(upd.Message.Chat.ID, fmt.Sprintf("📊 Channel Status\n• Connected: %v\n• Messages received: %d\n• Last message: %s",
			st.Running, st.Messages, st.LastMsg.Format("15:04:05")))
		return
	}

	// Publish to bus
	tg.messages.Add(1)
	tg.lastMu.Lock()
	tg.lastMsg = time.Now()
	tg.lastMu.Unlock()

	payload := map[string]any{
		"message_id": upd.Message.MessageID,
		"chat_id":    upd.Message.Chat.ID,
		"chat_type":  upd.Message.Chat.Type,
		"text":       upd.Message.Text,
		"date":       upd.Message.Date,
	}
	if upd.Message.From != nil {
		payload["from"] = map[string]any{
			"id":        upd.Message.From.ID,
			"username":  upd.Message.From.Username,
			"first_name": upd.Message.From.FirstName,
			"last_name":  upd.Message.From.LastName,
		}
	}

	data, _ := json.Marshal(payload)
	tg.bus.Publish(tg.ctx, bus.Envelope{
		Source:    "channel.telegram",
		Target:    "agentforge",
		Kind:      bus.KindEvent,
		Topic:     "channel.telegram.message",
		Payload:   data,
		Timestamp: time.Now(),
	})
}

func (tg *TelegramAdapter) isAllowed(msg *tgMessage) bool {
	if msg.From != nil {
		for _, allowed := range tg.cfg.AllowFrom {
			if msg.From.Username == allowed || strconv.FormatInt(msg.From.ID, 10) == allowed {
				return true
			}
		}
	}
	return false
}

func (tg *TelegramAdapter) sendReply(chatID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tg.cfg.BotToken)
	body := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	data, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(tg.ctx, 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tg.client.Do(req)
	if err != nil {
		tg.logger.Warn("telegram sendMessage failed", slog.Any("error", err))
		return
	}
	resp.Body.Close()
}

// ── Discord Types ───────────────────────────────────────────────────────────

// discordWSMsg is a Discord Gateway WebSocket message.
type discordWSMsg struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  *int            `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

// discordIdentify is the IDENTIFY payload sent on connect.
type discordIdentify struct {
	Token      string           `json:"token"`
	Properties discordProperties `json:"properties"`
	Intents    int              `json:"intents"`
}

type discordProperties struct {
	OS      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

// discordReady is the READY event (op 0, t=READY), used to detect connection success.
type discordReady struct {
	SessionID string `json:"session_id"`
}

// Gateway opcodes
const (
	discordOpDispatch  = 0
	discordOpHeartbeat = 1
	discordOpIdentify  = 2
	discordOpHello     = 10
	discordOpHeartbeatAck = 11
)

// Gateway events
const discordEventMessageCreate = "MESSAGE_CREATE"
const discordEventReady = "READY"

// discordMessageCreateEvent holds the MESSAGE_CREATE event data.
type discordMessageCreateEvent struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
	Content string `json:"content"`
}

// discordHello is the HELLO payload received on connect (op 10).
type discordHello struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

// ── Discord Adapter ─────────────────────────────────────────────────────────

// DiscordAdapter connects to the Discord Gateway using WebSocket and identifies as a bot.
type DiscordAdapter struct {
	cfg        config.DiscordConfig
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	bus        bus.Bus
	client     *http.Client
	connects   atomic.Int64
	messages   atomic.Int64
	lastMsg    time.Time
	lastMu     sync.Mutex
	logger     *slog.Logger
	wsConn     *discordWSConn
	mu         sync.Mutex
}

// NewDiscordAdapter creates a Discord WebSocket adapter.
func NewDiscordAdapter(cfg config.DiscordConfig) *DiscordAdapter {
	return &DiscordAdapter{
		cfg:    cfg,
		done:   make(chan struct{}),
		client: &http.Client{Timeout: 30 * time.Second},
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (dc *DiscordAdapter) Name() string { return "discord" }

func (dc *DiscordAdapter) Status() Status {
	dc.lastMu.Lock()
	lm := dc.lastMsg
	dc.lastMu.Unlock()

	dc.mu.Lock()
	running := dc.cancel != nil && dc.wsConn != nil
	dc.mu.Unlock()

	return Status{
		Name:     "discord",
		Running:  running,
		Connects: int(dc.connects.Load()),
		Messages: dc.messages.Load(),
		LastMsg:  lm,
	}
}

func (dc *DiscordAdapter) Start(ctx context.Context, b bus.Bus) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if dc.cancel != nil {
		return nil
	}
	dc.bus = b
	dc.ctx, dc.cancel = context.WithCancel(ctx)
	dc.done = make(chan struct{})

	go dc.gatewayLoop()
	return nil
}

func (dc *DiscordAdapter) Stop() error {
	dc.mu.Lock()
	if dc.cancel == nil {
		dc.mu.Unlock()
		return nil
	}
	dc.cancel()
	dc.mu.Unlock()
	<-dc.done

	dc.mu.Lock()
	dc.cancel = nil
	if dc.wsConn != nil {
		dc.wsConn.close()
		dc.wsConn = nil
	}
	dc.mu.Unlock()
	return nil
}

func (dc *DiscordAdapter) gatewayLoop() {
	defer close(dc.done)

	gatewayURL := "wss://gateway.discord.gg/?v=10&encoding=json"
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-dc.ctx.Done():
			return
		default:
		}

		dc.connects.Add(1)

		conn, err := dialWS(dc.ctx, gatewayURL)
		if err != nil {
			dc.logger.Warn("discord gateway dial failed, backing off",
				slog.String("error", err.Error()),
				slog.Duration("backoff", backoff),
			)
			select {
			case <-time.After(backoff):
			case <-dc.ctx.Done():
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		dc.mu.Lock()
		dc.wsConn = conn
		dc.mu.Unlock()

		// Read HELLO
		hello, err := conn.readHello(dc.ctx)
		if err != nil {
			dc.logger.Warn("discord hello read failed", slog.Any("error", err))
			conn.close()
			dc.safeClearConn()
			select {
			case <-time.After(backoff):
			case <-dc.ctx.Done():
				return
			}
			continue
		}

		backoff = time.Second // reset after connect

		// Send IDENTIFY
		identify := discordIdentify{
			Token: dc.cfg.BotToken,
			Properties: discordProperties{
				OS:      "linux",
				Browser: "agentforge",
				Device:  "agentforge",
			},
			Intents: 1 << 9, // GUILD_MESSAGES
		}
		identData, _ := json.Marshal(identify)
		if err := conn.writeMsg(discordOpIdentify, identData); err != nil {
			dc.logger.Warn("discord identify failed", slog.Any("error", err))
			conn.close()
			dc.safeClearConn()
			continue
		}

		// Start heartbeat goroutine
		heartbeatCtx, heartbeatCancel := context.WithCancel(dc.ctx)
		go dc.heartbeatLoop(heartbeatCtx, conn, time.Duration(hello.HeartbeatInterval)*time.Millisecond)

		// Read dispatch events
		dc.readEvents(conn)

		heartbeatCancel()
		conn.close()
		dc.safeClearConn()
	}
}

func (dc *DiscordAdapter) safeClearConn() {
	dc.mu.Lock()
	dc.wsConn = nil
	dc.mu.Unlock()
}

func (dc *DiscordAdapter) heartbeatLoop(ctx context.Context, conn *discordWSConn, interval time.Duration) {
	// Send initial heartbeat immediately
	if err := conn.writeMsg(discordOpHeartbeat, nil); err != nil {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Heartbeat uses last sequence number (null for initial)
			if err := conn.writeMsg(discordOpHeartbeat, nil); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (dc *DiscordAdapter) readEvents(conn *discordWSConn) {
	for {
		msg, err := conn.readMsg(dc.ctx)
		if err != nil {
			dc.logger.Warn("discord read event failed", slog.Any("error", err))
			return
		}

		switch msg.Op {
		case discordOpDispatch:
			switch msg.T {
			case discordEventMessageCreate:
				var payload discordMessageCreateEvent
				if err := json.Unmarshal(msg.D, &payload); err != nil {
					continue
				}
				dc.handleMessage(payload)
			}
		case discordOpHeartbeat:
			// Gateway requested a heartbeat; sent by heartbeatLoop independently
		case discordOpHeartbeatAck:
			// Heartbeat acknowledged
		}
	}
}

func (dc *DiscordAdapter) handleMessage(payload discordMessageCreateEvent) {
	if payload.Author.Bot {
		return // don't process bot messages
	}
	if payload.Content == "" {
		return
	}

	// AllowChannels whitelist
	if len(dc.cfg.AllowChannels) > 0 {
		allowed := false
		for _, chID := range dc.cfg.AllowChannels {
			if chID == payload.ChannelID {
				allowed = true
				break
			}
		}
		if !allowed {
			dc.logger.Debug("discord message from non-whitelisted channel, ignoring",
				slog.String("channel_id", payload.ChannelID),
			)
			return
		}
	}

	dc.messages.Add(1)
	dc.lastMu.Lock()
	dc.lastMsg = time.Now()
	dc.lastMu.Unlock()

	data, _ := json.Marshal(map[string]any{
		"message_id":  payload.ID,
		"channel_id":  payload.ChannelID,
		"author_id":   payload.Author.ID,
		"author_name": payload.Author.Username,
		"content":     payload.Content,
	})

	dc.bus.Publish(dc.ctx, bus.Envelope{
		Source:    "channel.discord",
		Target:    "agentforge",
		Kind:      bus.KindEvent,
		Topic:     "channel.discord.message",
		Payload:   data,
		Timestamp: time.Now(),
	})
}

// sendChannelMessage sends a text message to a Discord channel via REST API.
func (dc *DiscordAdapter) sendChannelMessage(channelID, text string) error {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)

	body := map[string]string{"content": text}
	data, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(dc.ctx, 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+dc.cfg.BotToken)

	resp, err := dc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord REST API error %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}