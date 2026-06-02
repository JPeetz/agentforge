// Package channel — Slack Socket Mode adapter for AgentForge.
// Connects via WebSocket to Slack Socket Mode, receives events_api messages,
// and publishes them on the internal CSP bus.

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
	"sync"
	"sync/atomic"
	"time"

	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/config"
)

// ── Slack Socket Mode Types ─────────────────────────────────────────────────

// slackSocketMsg is the top-level Socket Mode message.
type slackSocketMsg struct {
	Type           string               `json:"type"`
	EnvelopeID     string               `json:"envelope_id,omitempty"`
	Payload        *slackSocketPayload  `json:"payload,omitempty"`
	ConnectionInfo *slackConnectionInfo `json:"connection_info,omitempty"`
	NumConnections int                  `json:"num_connections,omitempty"`
	Reason         string               `json:"reason,omitempty"`
	// accepts_response_payload is present in hello/disconnect; ignore.
}

// slackSocketPayload wraps the nested event inside events_api.
type slackSocketPayload struct {
	Event json.RawMessage `json:"event,omitempty"`
}

// slackMessageEvent is the message event inside events_api → payload.event.
type slackMessageEvent struct {
	Type       string `json:"type"`
	User       string `json:"user,omitempty"`
	Channel    string `json:"channel,omitempty"`
	Text       string `json:"text,omitempty"`
	TS         string `json:"ts,omitempty"`
	BotID      string `json:"bot_id,omitempty"`
	Subtype    string `json:"subtype,omitempty"`
	BotProfile *struct {
		ID string `json:"id"`
	} `json:"bot_profile,omitempty"`
}

// slackConnectionInfo is the hello handshake payload.
type slackConnectionInfo struct {
	AppID string `json:"app_id"`
}

// slackConnOpenResponse is the REST response from apps.connections.open.
type slackConnOpenResponse struct {
	OK    bool   `json:"ok"`
	URL   string `json:"url"`
	Error string `json:"error,omitempty"`
}

// slackPostMessageResponse is the REST response from chat.postMessage.
type slackPostMessageResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ── Slack Adapter ───────────────────────────────────────────────────────────

// SlackAdapter connects to Slack using Socket Mode (WebSocket + app-level token).
type SlackAdapter struct {
	cfg      config.SlackConfig
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	bus      bus.Bus
	client   *http.Client
	connects atomic.Int64
	messages atomic.Int64
	lastMsg  time.Time
	lastMu   sync.Mutex
	logger   *slog.Logger
	wsConn   *discordWSConn
	mu       sync.Mutex // guards wsConn
}

// NewSlackAdapter creates a Slack Socket Mode adapter.
func NewSlackAdapter(cfg config.SlackConfig) *SlackAdapter {
	return &SlackAdapter{
		cfg:    cfg,
		done:   make(chan struct{}),
		client: &http.Client{Timeout: 30 * time.Second},
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (s *SlackAdapter) Name() string { return "slack" }

func (s *SlackAdapter) Status() Status {
	s.lastMu.Lock()
	lm := s.lastMsg
	s.lastMu.Unlock()

	s.mu.Lock()
	running := s.cancel != nil && s.wsConn != nil
	s.mu.Unlock()

	return Status{
		Name:     "slack",
		Running:  running,
		Connects: int(s.connects.Load()),
		Messages: s.messages.Load(),
		LastMsg:  lm,
	}
}

func (s *SlackAdapter) Start(ctx context.Context, b bus.Bus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		return nil // already running
	}
	s.bus = b
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.done = make(chan struct{})

	go s.gatewayLoop()
	return nil
}

func (s *SlackAdapter) Stop() error {
	s.mu.Lock()
	if s.cancel == nil {
		s.mu.Unlock()
		return nil
	}
	s.cancel()
	s.mu.Unlock()
	<-s.done

	s.mu.Lock()
	s.cancel = nil
	if s.wsConn != nil {
		s.wsConn.writeFrame(wsOpClose, nil)
		s.wsConn.conn.Close()
		s.wsConn = nil
	}
	s.mu.Unlock()
	return nil
}

// ── Gateway Loop ────────────────────────────────────────────────────────────

func (s *SlackAdapter) gatewayLoop() {
	defer close(s.done)

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		s.connects.Add(1)

		// Step 1: Get a WebSocket URL from apps.connections.open
		wsURL, err := s.fetchSocketURL()
		if err != nil {
			s.logger.Warn("slack fetch socket URL failed, backing off",
				slog.String("error", err.Error()),
				slog.Duration("backoff", backoff),
			)
			select {
			case <-time.After(backoff):
			case <-s.ctx.Done():
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Step 2: Dial WebSocket
		conn, err := dialWS(s.ctx, wsURL)
		if err != nil {
			s.logger.Warn("slack socket dial failed, backing off",
				slog.String("error", err.Error()),
				slog.Duration("backoff", backoff),
			)
			select {
			case <-time.After(backoff):
			case <-s.ctx.Done():
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		s.mu.Lock()
		s.wsConn = conn
		s.mu.Unlock()

		backoff = time.Second // reset on connect

		// Step 3: Read hello message
		if err := s.readHello(conn); err != nil {
			s.logger.Warn("slack hello failed", slog.Any("error", err))
			conn.writeFrame(wsOpClose, nil)
			conn.conn.Close()
			s.safeClearConn()
			select {
			case <-time.After(backoff):
			case <-s.ctx.Done():
				return
			}
			continue
		}

		s.logger.Info("slack socket mode connected")

		// Step 4: Event loop — read messages until disconnect or error
		s.readEvents(conn)

		conn.writeFrame(wsOpClose, nil)
		conn.conn.Close()
		s.safeClearConn()

		s.logger.Info("slack socket mode disconnected, will reconnect")
	}
}

func (s *SlackAdapter) safeClearConn() {
	s.mu.Lock()
	s.wsConn = nil
	s.mu.Unlock()
}

// ── REST Helpers ────────────────────────────────────────────────────────────

// fetchSocketURL calls apps.connections.open to get a WebSocket URL.
func (s *SlackAdapter) fetchSocketURL() (string, error) {
	url := "https://slack.com/api/apps.connections.open"

	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.AppToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed slackConnOpenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("slack parse connections.open: %w", err)
	}
	if !parsed.OK {
		return "", fmt.Errorf("slack apps.connections.open error: %s (body: %s)", parsed.Error, string(body))
	}
	if parsed.URL == "" {
		return "", fmt.Errorf("slack apps.connections.open returned empty URL")
	}

	return parsed.URL, nil
}

// sendChannelMessage sends a text message to a Slack channel via the Web API.
// Uses the Bot token (xoxb-), not the app-level token (xapp-).
func (s *SlackAdapter) sendChannelMessage(channelID, text string) error {
	url := "https://slack.com/api/chat.postMessage"

	body := map[string]string{
		"channel": channelID,
		"text":    text,
	}
	data, _ := json.Marshal(body)

	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+s.cfg.BotToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var parsed slackPostMessageResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return fmt.Errorf("slack parse chat.postMessage: %w", err)
	}
	if !parsed.OK {
		return fmt.Errorf("slack chat.postMessage error: %s", parsed.Error)
	}
	return nil
}

// ── WebSocket Read Helpers ─────────────────────────────────────────────────

// readHello waits for the first Socket Mode message which must be type "hello".
func (s *SlackAdapter) readHello(conn *discordWSConn) error {
	for {
		msg, err := s.readSlackMessage(conn)
		if err != nil {
			return err
		}
		if msg.Type == "hello" {
			return nil
		}
		// Ignore any other message type before hello (should not happen)
		s.logger.Warn("slack unexpected message before hello", slog.String("type", msg.Type))
	}
}

// readSlackMessage reads the next WebSocket text frame and parses it as a Slack Socket Mode message.
func (s *SlackAdapter) readSlackMessage(conn *discordWSConn) (*slackSocketMsg, error) {
	for {
		frame, err := conn.readFrame(s.ctx)
		if err != nil {
			return nil, err
		}

		switch frame.opcode {
		case wsOpText:
			var msg slackSocketMsg
			if err := json.Unmarshal(frame.payload, &msg); err != nil {
				return nil, fmt.Errorf("slack parse ws message: %w", err)
			}
			return &msg, nil
		case wsOpClose:
			return nil, io.EOF
		case 9: // Ping — respond with Pong
			if err := conn.writeFrame(0xA, frame.payload); err != nil {
				return nil, err
			}
			continue
		default:
			continue
		}
	}
}

// ── Event Loop ─────────────────────────────────────────────────────────────

func (s *SlackAdapter) readEvents(conn *discordWSConn) {
	for {
		msg, err := s.readSlackMessage(conn)
		if err != nil {
			if err != io.EOF {
				s.logger.Warn("slack read event failed", slog.Any("error", err))
			}
			return
		}

		switch msg.Type {
		case "events_api":
			// Acknowledge the event by echoing the envelope_id
			if msg.EnvelopeID != "" {
				ack, _ := json.Marshal(map[string]string{"envelope_id": msg.EnvelopeID})
				conn.writeFrame(wsOpText, ack)
			}

			if msg.Payload != nil && msg.Payload.Event != nil {
				s.handleEvent(msg.Payload.Event)
			}

		case "disconnect":
			s.logger.Info("slack socket received disconnect",
				slog.String("reason", msg.Reason),
			)
			return

		case "hello":
			// Duplicate hello — ignore
		default:
			s.logger.Debug("slack unknown message type", slog.String("type", msg.Type))
		}
	}
}

// ── Event Handler ───────────────────────────────────────────────────────────

func (s *SlackAdapter) handleEvent(raw json.RawMessage) {
	var evt slackMessageEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		s.logger.Warn("slack parse event failed", slog.Any("error", err))
		return
	}

	// Only handle message events
	if evt.Type != "message" {
		return
	}

	// Skip bot messages
	if evt.BotID != "" || (evt.BotProfile != nil && evt.BotProfile.ID != "") {
		return
	}

	// Skip message subtypes (channel_join, etc.)
	if evt.Subtype != "" {
		return
	}

	if evt.Text == "" {
		return
	}

	// AllowChannels whitelist
	if len(s.cfg.AllowChannels) > 0 {
		allowed := false
		for _, chID := range s.cfg.AllowChannels {
			if chID == evt.Channel {
				allowed = true
				break
			}
		}
		if !allowed {
			s.logger.Debug("slack message from non-whitelisted channel, ignoring",
				slog.String("channel_id", evt.Channel),
			)
			return
		}
	}

	// Admin commands
	switch evt.Text {
	case "/start", "start":
		s.sendChannelMessage(evt.Channel, "👋 Hello! I'm AgentForge, an agentic orchestrator. Available commands: /help, /status")
		return
	case "/help", "help":
		s.sendChannelMessage(evt.Channel, "AgentForge Bot Commands:\n/start — welcome message\n/help — this help\n/status — system status")
		return
	case "/status", "status":
		st := s.Status()
		s.sendChannelMessage(evt.Channel, fmt.Sprintf("📊 Channel Status\n• Connected: %v\n• Messages received: %d\n• Last message: %s",
			st.Running, st.Messages, st.LastMsg.Format("15:04:05")))
		return
	}

	// Publish to bus
	s.messages.Add(1)
	s.lastMu.Lock()
	s.lastMsg = time.Now()
	s.lastMu.Unlock()

	payload := map[string]any{
		"user":       evt.User,
		"channel_id": evt.Channel,
		"text":       evt.Text,
		"ts":         evt.TS,
	}

	data, _ := json.Marshal(payload)
	s.bus.Publish(s.ctx, bus.Envelope{
		Source:    "channel.slack",
		Target:    "agentforge",
		Kind:      bus.KindEvent,
		Topic:     "channel.slack.message",
		Payload:   data,
		Timestamp: time.Now(),
	})
}

// writeWSMessage sends a raw text frame over the WebSocket.
// Used for ACK and other Socket Mode protocol messages.
func (s *SlackAdapter) writeWSMessage(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wsConn == nil {
		return fmt.Errorf("slack: not connected")
	}
	return s.wsConn.writeFrame(wsOpText, data)
}