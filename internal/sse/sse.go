// Package sse — Server-Sent Events streaming infrastructure for AgentForge.
package sse

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// ── Types ────────────────────────────────────────────────────────────────────

type SSEEvent struct {
	Event string      `json:"event,omitempty"`
	Data  interface{} `json:"data"`
	ID    string      `json:"id,omitempty"`
	Retry int         `json:"retry,omitempty"`
}

type StreamChunk struct {
	Content string `json:"content,omitempty"`
	Done    bool   `json:"done"`
	Model   string `json:"model,omitempty"`
	Usage   *Usage `json:"usage,omitempty"`
	Error   string `json:"error,omitempty"`
}

type ToolCallChunk struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
}

type StatusEvent struct {
	Agent   string `json:"agent"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Model   string `json:"model,omitempty"`
}

type CostUpdateEvent struct {
	SessionID       string  `json:"session_id"`
	Model           string  `json:"model"`
	DeltaCost       float64 `json:"delta_cost"`
	TotalCost       float64 `json:"total_cost"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	CachedTokens    int     `json:"cached_tokens"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ── SSEWriter ───────────────────────────────────────────────────────────────

type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
}

func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("sse: ResponseWriter does not support flushing")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &SSEWriter{w: w, flusher: flusher, done: make(chan struct{})}, nil
}

func (s *SSEWriter) Write(event SSEEvent) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("sse: marshal data: %w", err)
	}
	var buf []byte
	if event.Event != "" {
		buf = append(buf, fmt.Sprintf("event: %s\n", event.Event)...)
	}
	if event.ID != "" {
		buf = append(buf, fmt.Sprintf("id: %s\n", event.ID)...)
	}
	if event.Retry > 0 {
		buf = append(buf, fmt.Sprintf("retry: %d\n", event.Retry)...)
	}
	buf = append(buf, fmt.Sprintf("data: %s\n\n", string(data))...)
	_, err = s.w.Write(buf)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *SSEWriter) Comment(text string) {
	fmt.Fprintf(s.w, ": %s\n\n", text)
	s.flusher.Flush()
}

func (s *SSEWriter) Done() { close(s.done) }
func (s *SSEWriter) Closed() <-chan struct{} { return s.done }

// ── Client ───────────────────────────────────────────────────────────────────

type Client struct {
	ID     string
	Writer *SSEWriter
	Topics map[string]struct{}
	ch     chan SSEEvent
	quit   chan struct{}
}

func (c *Client) Send(event SSEEvent) {
	select {
	case c.ch <- event:
	default:
	}
}

func (c *Client) Quit() <-chan struct{} { return c.quit }

// ── Hub ──────────────────────────────────────────────────────────────────────

type Hub struct {
	mu        sync.RWMutex
	clients   map[string]*Client
	topicSubs map[string]map[string]struct{}
	clientSeq atomic.Int64
	log       func(format string, args ...interface{})
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[string]*Client),
		topicSubs: make(map[string]map[string]struct{}),
		log:       log.Printf,
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Connect(w http.ResponseWriter) (*Client, error) {
	writer, err := NewSSEWriter(w)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("sse-%d-%s", h.clientSeq.Add(1), uuid.New().String()[:8])
	client := &Client{
		ID:     id,
		Writer: writer,
		Topics: make(map[string]struct{}),
		ch:     make(chan SSEEvent, 256),
		quit:   make(chan struct{}),
	}
	h.mu.Lock()
	h.clients[id] = client
	h.mu.Unlock()
	go client.listen(h)
	return client, nil
}

func (c *Client) listen(h *Hub) {
	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case event := <-c.ch:
			if err := c.Writer.Write(event); err != nil {
				h.log("sse: client %s write error: %v", c.ID, err)
				h.Disconnect(c)
				return
			}
		case <-keepAlive.C:
			c.Writer.Comment("keepalive")
		case <-c.quit:
			return
		case <-c.Writer.Closed():
			h.Disconnect(c)
			return
		}
	}
}

func (h *Hub) Subscribe(client *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.topicSubs[topic] == nil {
		h.topicSubs[topic] = make(map[string]struct{})
	}
	h.topicSubs[topic][client.ID] = struct{}{}
	client.Topics[topic] = struct{}{}
}

func (h *Hub) Unsubscribe(client *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(client.Topics, topic)
	if subs, ok := h.topicSubs[topic]; ok {
		delete(subs, client.ID)
		if len(subs) == 0 {
			delete(h.topicSubs, topic)
		}
	}
}

func (h *Hub) Disconnect(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client.ID]; !ok {
		return
	}
	for topic := range client.Topics {
		if subs, ok := h.topicSubs[topic]; ok {
			delete(subs, client.ID)
			if len(subs) == 0 {
				delete(h.topicSubs, topic)
			}
		}
	}
	delete(h.clients, client.ID)
	close(client.quit)
	client.Writer.Done()
	h.log("sse: client disconnected %s (%d remaining)", client.ID, len(h.clients))
}

func (h *Hub) Broadcast(topic string, event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	subs, ok := h.topicSubs[topic]
	if !ok {
		return
	}
	for clientID := range subs {
		if client, ok := h.clients[clientID]; ok {
			client.Send(event)
		}
	}
}

func (h *Hub) SendToClient(client *Client, event SSEEvent) {
	client.Send(event)
}

// ── Event helpers ────────────────────────────────────────────────────────────

func NewChunkEvent(content string) SSEEvent {
	return SSEEvent{Event: "chunk", Data: StreamChunk{Content: content}}
}

func NewToolCallEvent(id, name, args string, done bool, errStr string) SSEEvent {
	return SSEEvent{Event: "tool_call", Data: ToolCallChunk{ID: id, Name: name, Arguments: args, Done: done, Error: errStr}}
}

func NewDoneEvent(model string, usage *Usage) SSEEvent {
	return SSEEvent{Event: "done", Data: StreamChunk{Done: true, Model: model, Usage: usage}}
}

func NewStatusEvent(agent, status, message string) SSEEvent {
	return SSEEvent{Event: "status", Data: StatusEvent{Agent: agent, Status: status, Message: message}}
}

func NewErrorEvent(errStr string) SSEEvent {
	return SSEEvent{Event: "error", Data: StreamChunk{Error: errStr, Done: true}}
}

func NewCostUpdateEvent(sessionID, model string, deltaCost, totalCost float64, inputTokens, outputTokens, cachedTokens int) SSEEvent {
	return SSEEvent{Event: "cost_update", Data: CostUpdateEvent{
		SessionID:    sessionID,
		Model:        model,
		DeltaCost:    deltaCost,
		TotalCost:    totalCost,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CachedTokens: cachedTokens,
	}}
}