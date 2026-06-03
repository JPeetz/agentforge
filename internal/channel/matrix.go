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
)

// matrixConfig holds Matrix connection settings.
type matrixConfig struct {
	Enabled     bool
	HomeServer  string
	UserID      string
	AccessToken string
	RoomID      string
}

// MatrixAdapter connects to a Matrix homeserver using the Client-Server API.
type MatrixAdapter struct {
	cfg        matrixConfig
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
	since      string
}

// NewMatrixAdapter creates a Matrix adapter for the given homeserver.
func NewMatrixAdapter(homeserver, userID, accessToken, roomID string) *MatrixAdapter {
	return &MatrixAdapter{
		cfg: matrixConfig{
			HomeServer:  homeserver,
			UserID:      userID,
			AccessToken: accessToken,
			RoomID:      roomID,
		},
		done:   make(chan struct{}),
		client: &http.Client{Timeout: 60 * time.Second},
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (mx *MatrixAdapter) Name() string { return "matrix" }

func (mx *MatrixAdapter) Status() Status {
	mx.lastMu.Lock()
	lm := mx.lastMsg
	mx.lastMu.Unlock()
	return Status{
		Name:     "matrix",
		Running:  mx.cancel != nil,
		Connects: int(mx.connects.Load()),
		Messages: mx.messages.Load(),
		LastMsg:  lm,
	}
}

func (mx *MatrixAdapter) Start(ctx context.Context, b bus.Bus) error {
	if mx.cancel != nil {
		return nil
	}
	mx.bus = b
	mx.ctx, mx.cancel = context.WithCancel(ctx)
	mx.done = make(chan struct{})

	mx.since = mx.initialSync()

	go mx.syncLoop()
	return nil
}

func (mx *MatrixAdapter) Stop() error {
	if mx.cancel == nil {
		return nil
	}
	mx.cancel()
	<-mx.done
	mx.cancel = nil
	return nil
}

func (mx *MatrixAdapter) initialSync() string {
	url := fmt.Sprintf("%s/_matrix/client/v3/sync?timeout=0", mx.cfg.HomeServer)
	req, _ := http.NewRequestWithContext(mx.ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+mx.cfg.AccessToken)
	resp, err := mx.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var syncResp struct {
		NextBatch string `json:"next_batch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return ""
	}
	return syncResp.NextBatch
}

func (mx *MatrixAdapter) syncLoop() {
	defer close(mx.done)

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-mx.ctx.Done():
			return
		default:
		}

		mx.connects.Add(1)

		url := fmt.Sprintf("%s/_matrix/client/v3/sync?timeout=30000&since=%s",
			mx.cfg.HomeServer, mx.since)
		req, _ := http.NewRequestWithContext(mx.ctx, "GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+mx.cfg.AccessToken)

		resp, err := mx.client.Do(req)
		if err != nil {
			mx.logger.Warn("matrix sync failed, backing off",
				slog.Any("error", err), slog.Duration("backoff", backoff))
			select {
			case <-time.After(backoff):
			case <-mx.ctx.Done():
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		var syncResp struct {
			NextBatch string `json:"next_batch"`
			Rooms     struct {
				Join map[string]struct {
					Timeline struct {
						Events []struct {
							Type    string `json:"type"`
							Sender  string `json:"sender"`
							Content struct {
								Body    string `json:"body"`
								MsgType string `json:"msgtype"`
							} `json:"content"`
							EventID string `json:"event_id"`
						} `json:"events"`
					} `json:"timeline"`
				} `json:"join"`
			} `json:"rooms"`
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			mx.logger.Warn("matrix sync HTTP error", slog.Int("status", resp.StatusCode))
			backoff = maxBackoff
			select {
			case <-time.After(backoff):
			case <-mx.ctx.Done():
				return
			}
			continue
		}

		json.Unmarshal(body, &syncResp)
		backoff = time.Second

		if syncResp.NextBatch != "" {
			mx.since = syncResp.NextBatch
		}

		if mx.cfg.RoomID == "" {
			continue
		}

		if room, ok := syncResp.Rooms.Join[mx.cfg.RoomID]; ok {
			for _, ev := range room.Timeline.Events {
				if ev.Type != "m.room.message" || ev.Content.MsgType != "m.text" {
					continue
				}
				if ev.Sender == mx.cfg.UserID {
					continue
				}

				mx.messages.Add(1)
				mx.lastMu.Lock()
				mx.lastMsg = time.Now()
				mx.lastMu.Unlock()

				data, _ := json.Marshal(map[string]any{
					"event_id": ev.EventID,
					"sender":   ev.Sender,
					"text":     ev.Content.Body,
					"room_id":  mx.cfg.RoomID,
					"channel":  "matrix",
				})

				mx.bus.Publish(mx.ctx, bus.Envelope{
					Source:    "channel.matrix",
					Target:    "agentforge",
					Kind:      bus.KindEvent,
					Topic:     "channel.matrix.message",
					Payload:   data,
					Timestamp: time.Now(),
				})
			}
		}
	}
}

// SendMessage sends a text message to the configured Matrix room.
func (mx *MatrixAdapter) SendMessage(text string) error {
	txnID := fmt.Sprintf("agentforge-%d", time.Now().UnixNano())
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		mx.cfg.HomeServer, mx.cfg.RoomID, txnID)

	body := map[string]any{
		"msgtype": "m.text",
		"body":    text,
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(mx.ctx, "PUT", url, bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+mx.cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := mx.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("matrix: HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}