// Package channel — Signal adapter for AgentForge.
// Launches signal-cli as a JSON-RPC subprocess and relays messages
// to/from the internal CSP bus.

package channel

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/agentforge/agentforge/internal/bus"
	"github.com/agentforge/agentforge/internal/config"
)

// ── Signal JSON-RPC Types ───────────────────────────────────────────────────

// signalRPCRequest is a JSON-RPC 2.0 request sent to signal-cli.
type signalRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int64       `json:"id,omitempty"`
}

// signalRPCMessage is a JSON-RPC 2.0 message received from signal-cli.
// It may be a response (with id/result/error) or a notification (with method/params).
type signalRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// signalEnvelope is the params.receive notification envelope.
type signalEnvelope struct {
	Source       string                 `json:"source,omitempty"`
	SourceNumber string                 `json:"sourceNumber,omitempty"`
	SourceName   string                 `json:"sourceName,omitempty"`
	DataMessage  *signalDataMessage     `json:"dataMessage,omitempty"`
	TypingMessage *signalTypingMessage   `json:"typingMessage,omitempty"`
}

// signalDataMessage is the dataMessage inside a receive envelope.
type signalDataMessage struct {
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message,omitempty"`
	GroupInfo *struct {
		GroupID string `json:"groupId"`
		GroupName string `json:"groupName,omitempty"`
	} `json:"groupInfo,omitempty"`
}

// signalTypingMessage is the typingMessage inside a receive envelope (ignored).
type signalTypingMessage struct {
	Timestamp int64 `json:"timestamp"`
}

// signalSendParams is the params for the "send" method.
type signalSendParams struct {
	Recipient []string `json:"recipient"`
	Message   string   `json:"message"`
}

// signalRPCError is the error field in a JSON-RPC response.
type signalRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ── Signal Adapter ──────────────────────────────────────────────────────────

// SignalAdapter communicates with Signal via signal-cli JSON-RPC.
type SignalAdapter struct {
	cfg      config.SignalConfig
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	bus      bus.Bus
	connects atomic.Int64
	messages atomic.Int64
	lastMsg  time.Time
	lastMu   sync.Mutex
	logger   *slog.Logger
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Scanner
	cmdMu    sync.Mutex
	reqID    atomic.Int64
}

// NewSignalAdapter creates a Signal adapter using signal-cli subprocess.
func NewSignalAdapter(cfg config.SignalConfig) *SignalAdapter {
	if cfg.SignalCLIPath == "" {
		cfg.SignalCLIPath = "signal-cli"
	}
	return &SignalAdapter{
		cfg:    cfg,
		done:   make(chan struct{}),
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (sa *SignalAdapter) Name() string { return "signal" }

func (sa *SignalAdapter) Status() Status {
	sa.lastMu.Lock()
	lm := sa.lastMsg
	sa.lastMu.Unlock()

	sa.cmdMu.Lock()
	running := sa.cancel != nil && sa.cmd != nil
	sa.cmdMu.Unlock()

	return Status{
		Name:     "signal",
		Running:  running,
		Connects: int(sa.connects.Load()),
		Messages: sa.messages.Load(),
		LastMsg:  lm,
	}
}

func (sa *SignalAdapter) Start(ctx context.Context, b bus.Bus) error {
	sa.cmdMu.Lock()
	defer sa.cmdMu.Unlock()

	if sa.cancel != nil {
		return nil // already running
	}

	sa.bus = b
	sa.ctx, sa.cancel = context.WithCancel(ctx)
	sa.done = make(chan struct{})

	// Verify signal-cli exists
	cliPath := sa.cfg.SignalCLIPath
	if _, err := exec.LookPath(cliPath); err != nil {
		sa.logger.Warn("signal-cli not found in PATH, skipping adapter",
			slog.String("path", cliPath),
		)
		sa.cancel()
		sa.cancel = nil
		return nil // gracefully skip
	}

	// Launch signal-cli JSON-RPC daemon
	// signal-cli -a PHONE daemon --json
	cmd := exec.CommandContext(sa.ctx, cliPath,
		"-a", sa.cfg.PhoneNumber,
		"daemon",
		"--json",
	)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("signal stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("signal stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("signal-cli start: %w", err)
	}

	sa.cmd = cmd
	sa.stdin = stdinPipe
	sa.stdout = bufio.NewScanner(stdoutPipe)

	sa.logger.Info("signal-cli started",
		slog.String("phone", sa.cfg.PhoneNumber),
		slog.String("path", cliPath),
	)

	go sa.readLoop()
	go sa.waitProcess()

	return nil
}

func (sa *SignalAdapter) Stop() error {
	sa.cmdMu.Lock()
	if sa.cancel == nil {
		sa.cmdMu.Unlock()
		return nil
	}
	sa.cancel()

	cmd := sa.cmd
	sa.cmdMu.Unlock()

	// Graceful shutdown: signal-cli should receive the context cancel
	// Wait up to 5 seconds, then SIGTERM, then SIGKILL
	if cmd != nil && cmd.Process != nil {
		done := make(chan struct{})
		go func() {
			cmd.Wait()
			close(done)
		}()

		// Wait for graceful exit
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			// Send SIGTERM
			cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				// Last resort: SIGKILL
				cmd.Process.Signal(syscall.SIGKILL)
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					// Process is stuck; give up
				}
			}
		}
	}

	<-sa.done

	sa.cmdMu.Lock()
	sa.cancel = nil
	sa.cmd = nil
	sa.stdin = nil
	sa.stdout = nil
	sa.cmdMu.Unlock()

	return nil
}

// ── Read Loop ───────────────────────────────────────────────────────────────

func (sa *SignalAdapter) readLoop() {
	defer close(sa.done)

	for {
		sa.cmdMu.Lock()
		scanner := sa.stdout
		sa.cmdMu.Unlock()

		if scanner == nil {
			return
		}

		if !scanner.Scan() {
			// Scanner stopped (EOF or error)
			err := scanner.Err()
			if err != nil && err != io.EOF {
				sa.logger.Warn("signal-cli stdout read error", slog.Any("error", err))
			}
			return
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		sa.connects.Add(1)

		var msg signalRPCMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			sa.logger.Warn("signal parse message failed",
				slog.String("error", err.Error()),
				slog.String("line", string(line)),
			)
			continue
		}

		sa.handleMessage(&msg)
	}
}

func (sa *SignalAdapter) waitProcess() {
	sa.cmdMu.Lock()
	cmd := sa.cmd
	sa.cmdMu.Unlock()

	if cmd == nil {
		return
	}

	err := cmd.Wait()

	sa.logger.Warn("signal-cli process exited",
		slog.Int("exit_code", cmd.ProcessState.ExitCode()),
	)

	if err != nil {
		sa.logger.Warn("signal-cli wait error", slog.Any("error", err))
	}

	// Clean up state
	sa.cmdMu.Lock()
	sa.cmd = nil
	sa.stdin = nil
	sa.stdout = nil
	sa.cmdMu.Unlock()
}

// ── Message Handler ─────────────────────────────────────────────────────────

func (sa *SignalAdapter) handleMessage(msg *signalRPCMessage) {
	// Notifications have Method set, Responses have ID set
	if msg.Method == "" {
		// This is a response to one of our requests — ignore for now
		return
	}

	switch msg.Method {
	case "receive":
		sa.handleReceive(msg.Params)
	default:
		// Unknown notification — silently ignore
	}
}

func (sa *SignalAdapter) handleReceive(paramsRaw json.RawMessage) {
	var env signalEnvelope
	if err := json.Unmarshal(paramsRaw, &env); err != nil {
		sa.logger.Warn("signal parse envelope failed", slog.Any("error", err))
		return
	}

	// Only handle data messages (not typing indicators)
	if env.DataMessage == nil {
		return
	}

	dm := env.DataMessage
	if dm.Message == "" {
		return
	}

	// Determine sender identifier
	sender := env.Source
	if sender == "" {
		sender = env.SourceNumber
	}
	if sender == "" {
		sa.logger.Debug("signal message without sender, ignoring")
		return
	}

	// AllowFrom whitelist
	if len(sa.cfg.AllowFrom) > 0 {
		allowed := false
		for _, a := range sa.cfg.AllowFrom {
			if a == sender || a == env.SourceNumber || a == env.SourceName {
				allowed = true
				break
			}
		}
		if !allowed {
			sa.logger.Debug("signal message from non-whitelisted sender, ignoring",
				slog.String("sender", sender),
			)
			return
		}
	}

	// Admin commands
	switch dm.Message {
	case "/start", "start":
		sa.sendMessage(sender, "👋 Hello! I'm AgentForge, an agentic orchestrator. Available commands: /help, /status")
		return
	case "/help", "help":
		sa.sendMessage(sender, "AgentForge Bot Commands:\n/start — welcome message\n/help — this help\n/status — system status")
		return
	case "/status", "status":
		st := sa.Status()
		sa.sendMessage(sender, fmt.Sprintf("📊 Channel Status\n• Connected: %v\n• Messages received: %d\n• Last message: %s",
			st.Running, st.Messages, st.LastMsg.Format("15:04:05")))
		return
	}

	// Publish to bus
	sa.messages.Add(1)
	sa.lastMu.Lock()
	sa.lastMsg = time.Now()
	sa.lastMu.Unlock()

	payload := map[string]any{
		"sender":      sender,
		"source_name": env.SourceName,
		"message":     dm.Message,
		"timestamp":   dm.Timestamp,
	}

	if dm.GroupInfo != nil {
		payload["group_id"] = dm.GroupInfo.GroupID
		payload["group_name"] = dm.GroupInfo.GroupName
	}

	data, _ := json.Marshal(payload)
	sa.bus.Publish(sa.ctx, bus.Envelope{
		Source:    "channel.signal",
		Target:    "agentforge",
		Kind:      bus.KindEvent,
		Topic:     "channel.signal.message",
		Payload:   data,
		Timestamp: time.Now(),
	})
}

// ── Send Message ────────────────────────────────────────────────────────────

// sendMessage sends a text message to a Signal recipient via JSON-RPC.
func (sa *SignalAdapter) sendMessage(recipient, text string) error {
	sa.cmdMu.Lock()
	stdin := sa.stdin
	sa.cmdMu.Unlock()

	if stdin == nil {
		return fmt.Errorf("signal: not connected")
	}

	req := signalRPCRequest{
		JSONRPC: "2.0",
		Method:  "send",
		Params: signalSendParams{
			Recipient: []string{recipient},
			Message:   text,
		},
		ID: sa.reqID.Add(1),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("signal marshal send: %w", err)
	}
	data = append(data, '\n')

	// Write atomically (single line)
	if _, err := stdin.Write(data); err != nil {
		return fmt.Errorf("signal send write: %w", err)
	}

	return nil
}

// lookupSignalCLI is a helper to find the signal-cli binary.
// It checks the configured path, then falls back to PATH search.
func lookupSignalCLI(cfg config.SignalConfig) (string, error) {
	if cfg.SignalCLIPath != "" {
		return exec.LookPath(cfg.SignalCLIPath)
	}
	return exec.LookPath("signal-cli")
}

