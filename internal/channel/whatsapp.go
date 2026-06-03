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

// WhatsAppAdapter connects to the WhatsApp Cloud API (Meta Business Platform).
type WhatsAppAdapter struct {
	cfg      config.WhatsAppConfig
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
}

// NewWhatsAppAdapter creates a WhatsApp Cloud API adapter.
func NewWhatsAppAdapter(cfg config.WhatsAppConfig) *WhatsAppAdapter {
	return &WhatsAppAdapter{
		cfg:    cfg,
		done:   make(chan struct{}),
		client: &http.Client{Timeout: 30 * time.Second},
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (wa *WhatsAppAdapter) Name() string { return "whatsapp" }

func (wa *WhatsAppAdapter) Status() Status {
	wa.lastMu.Lock()
	lm := wa.lastMsg
	wa.lastMu.Unlock()
	return Status{
		Name:     "whatsapp",
		Running:  wa.cancel != nil,
		Connects: int(wa.connects.Load()),
		Messages: wa.messages.Load(),
		LastMsg:  lm,
	}
}

func (wa *WhatsAppAdapter) Start(ctx context.Context, b bus.Bus) error {
	if wa.cancel != nil {
		return nil
	}
	wa.bus = b
	wa.ctx, wa.cancel = context.WithCancel(ctx)
	wa.done = make(chan struct{})
	go wa.pollLoop()
	return nil
}

func (wa *WhatsAppAdapter) Stop() error {
	if wa.cancel == nil {
		return nil
	}
	wa.cancel()
	<-wa.done
	wa.cancel = nil
	return nil
}

func (wa *WhatsAppAdapter) pollLoop() {
	defer close(wa.done)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wa.connects.Add(1)
			wa.checkHealth()
		case <-wa.ctx.Done():
			return
		}
	}
}

func (wa *WhatsAppAdapter) checkHealth() {
	if wa.cfg.PhoneNumberID == "" || wa.cfg.APIKey == "" {
		return
	}
	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s", wa.cfg.PhoneNumberID)
	req, _ := http.NewRequestWithContext(wa.ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+wa.cfg.APIKey)
	resp, err := wa.client.Do(req)
	if err != nil {
		wa.logger.Warn("whatsapp health check failed", slog.Any("error", err))
		return
	}
	resp.Body.Close()
}

// SendMessage sends a WhatsApp text message via the Cloud API.
func (wa *WhatsAppAdapter) SendMessage(to, text string) error {
	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", wa.cfg.PhoneNumberID)
	body := map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text":              map[string]string{"body": text},
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(wa.ctx, "POST", url, bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+wa.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := wa.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp: HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// HandleWebhook processes an incoming WhatsApp webhook event and publishes to the bus.
func (wa *WhatsAppAdapter) HandleWebhook(payload []byte) {
	if wa.bus == nil {
		return
	}

	var wh struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Messages []struct {
						From string `json:"from"`
						ID   string `json:"id"`
						Text struct {
							Body string `json:"body"`
						} `json:"text"`
					} `json:"messages"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(payload, &wh); err != nil {
		wa.logger.Warn("whatsapp webhook parse failed", slog.Any("error", err))
		return
	}

	for _, entry := range wh.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
				if len(wa.cfg.AllowFrom) > 0 {
					allowed := false
					for _, a := range wa.cfg.AllowFrom {
						if a == msg.From {
							allowed = true
							break
						}
					}
					if !allowed {
						continue
					}
				}

				wa.messages.Add(1)
				wa.lastMu.Lock()
				wa.lastMsg = time.Now()
				wa.lastMu.Unlock()

				data, _ := json.Marshal(map[string]any{
					"message_id": msg.ID,
					"from":       msg.From,
					"text":       msg.Text.Body,
					"channel":    "whatsapp",
				})

				wa.bus.Publish(wa.ctx, bus.Envelope{
					Source:    "channel.whatsapp",
					Target:    "agentforge",
					Kind:      bus.KindEvent,
					Topic:     "channel.whatsapp.message",
					Payload:   data,
					Timestamp: time.Now(),
				})
			}
		}
	}
}