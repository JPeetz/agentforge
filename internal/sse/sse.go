// Package sse — Server-Sent Events streaming infrastructure for AgentForge.
//
// Implements a lightweight, spec-compliant SSE hub with client tracking,
// per-topic subscriptions, and keep-alive pings. Zero external dependencies.
//
// SSE wire format (RFC 8895):
//   event: <type>\n
//   id: <id>\n
//   data: <json>\n
//   retry: <ms>\n
//   \n
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

// SSEEvent is a single Server-Sent Event.
type SSEEvent struct {
	Event string      `json:"event,omitempty"` // event type: chunk, tool_call, done, error, status
	Data  interface{} `json:"data"`            // arbitrary JSON-serialisable payload
	ID    string      `json:"id,omitempty"`    // event ID for EventSource auto-reconnect
	Retry int         `json:"retry,omitempty"` // reconnect delay in ms (0 = omit)
}

// StreamChunk represents a single chunk from a streaming LLM response.
type StreamChunk struct {
	Content string `json:"content,omitempty"`
	Done    bool   `json:"done"`
	Model   string `json:"model,omitempty"`
	Usage   *Usage `json:"usage,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ToolCallChunk represents a tool call being executed during streaming.
type ToolCallChunk struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
}

// StatusEvent carries agent status changes (idle/streaming/tool_exec).
type StatusEvent struct {
	Agent   string `json:"agent"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Model   string `json:"model,omitempty"`
}

// Usage carries token usage info from the final chunk.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ── SSE Writer ───────────────────────────────────────────────────────────────

// SSEWriter wraps an http.ResponseWriter with Flusher for SSE output.
// Conforms to the SSE wire format.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
}

// NewSSEWriter creates an SSE writer and writes the required headers.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("sse: ResponseWriter does not support flushing")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	return &SSEWriter{
		w:       w,
		flusher: flusher,
		done:    make(chan struct{}),
	}, nil
}

// Write formats and sends an SSE event.
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

// Comment sends an SSE comment line (used for keep-alive).
func (s *SSEWriter) Comment(text string) {
	fmt.Fprintf(s.w, ": %s\n\n", text)
	s.flusher.Flush()
}

// Done closes the writer signalling completion.
func (s *SSEWriter) Done() {
	close(s.done)
}

// Closed returns a channel that closes when the writer is done (client disconnects).
func (s *SSEWriter) Closed() <-chan struct{} {
	return s.done
}

// ── Client ───────────────────────────────────────────────────────────────────

// Client represents a connected SSE consumer subscribed to one or more topics.
type Client struct {
	ID     string
	Writer *SSEWriter
	Topics map[string]struct{}
	ch     chan SSEEvent
	quit   chan struct{}
}

// Send pushes an event to this client's output channel.
func (c *Client) Send(event SSEEvent) {
	select {
	case c.ch <- event:
	default:
		// client too slow, drop
	}
}

// Quit returns a channel that closes when the client is done.
func (c *Client) Quit() <-chan struct{} {
	return c.quit
}

// ── Hub ──────────────────────────────────────────────────────────────────────

// Hub manages connected SSE clients with topic-based subscriptions.
// Thread-safe; all public methods are safe for concurrent use.
type Hub struct {
	mu         sync.RWMutex
	clients    map[string]*Client
	topicSubs  map[string]map[string]struct{} // topic -> clientID set
	clientSeq  atomic.Int64
	log        func(format string, args ...interface{})
}

// NewHub creates a new SSE hub.
func NewHub() *Hub {
	return &Hub{
		clients:   make(map[string]*Client),
		topicSubs: make(map[string]map[string]struct{}),
		log:       log.Printf,
	}
}

// ClientCount returns the number of connected SSE clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// TopicCount returns the number of active topic subscriptions.
func (h *Hub) TopicCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topicSubs)
}

// Connect registers a new SSE client. Returns the Client struct.
// Caller is responsible for running client.listen() in a goroutine.
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

	h.log("sse: client connected %s (%d total)", id, h.ClientCount())

	// Start the client's write loop
	go client.listen(h)

	return client, nil
}

// listen pumps events from the client channel to the SSE writer.
func (c *Client) listen(h *Hub) {
	// Keep-alive ticker
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
			if err := c.Writer.Write(SSEEvent{
				Event: "ping",
				Data:  map[string]string{"ts": time.Now().Format(time.RFC3339)},
			}); err != nil {
				// Client probably disconnected
				h.Disconnect(c)
				return
			}

		case <-c.quit:
			return

		case <-c.Writer.Closed():
			h.Disconnect(c)
			return
		}
	}
}

// Subscribe adds the client to a topic.
func (h *Hub) Subscribe(client *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.topicSubs[topic] == nil {
		h.topicSubs[topic] = make(map[string]struct{})
	}
	h.topicSubs[topic][client.ID] = struct{}{}
	client.Topics[topic] = struct{}{}
}

// Unsubscribe removes the client from a topic.
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

// Disconnect removes the client and cleans up all subscriptions.
func (h *Hub) Disconnect(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.ID]; !ok {
		return
	}

	// Clean up topic subscriptions
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

// Broadcast sends an event to all clients subscribed to a topic.
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

// SendToClient sends an event directly to a specific client.
func (h *Hub) SendToClient(client *Client, event SSEEvent) {
	client.Send(event)
}

// ── Stream Handler ──────────────────────────────────────────────────────────

// StreamHandler returns an http.HandlerFunc that upgrades connections to SSE.
// The handler calls connectFn with the client for custom setup (subscriptions, etc.).
type StreamConnectFunc func(client *Client) error

// HandleSSE returns a handler that sets up an SSE connection.
// On connect, it calls the connectFn to allow subscription setup.
func (h *Hub) HandleSSE(connectFn StreamConnectFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client, err := h.Connect(w)
		if err != nil {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		if connectFn != nil {
			if err := connectFn(client); err != nil {
				h.Disconnect(client)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		// Wait for client to disconnect
		<-client.quit
	}
}

// ── Chat stream ──────────────────────────────────────────────────────────────

// ChatStreamRequest is the POST body for initiating a streaming chat.
type ChatStreamRequest struct {
	Prompt      string `json:"prompt"`
	Agent       string `json:"agent"`
	Model       string `json:"model"`
	Temperature float64 `json:"temperature"`
}

// ChatHandler returns an HTTP handler that accepts a POST with ChatStreamRequest
// and streams the LLM response back via SSE.
func (h *Hub) ChatHandler(streamFn func(client *Client, req ChatStreamRequest) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatStreamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		if req.Prompt == "" {
			http.Error(w, "prompt is required", http.StatusBadRequest)
			return
		}

		client, err := h.Connect(w)
		if err != nil {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}
		h.Subscribe(client, "chat."+client.ID)

		// Run the stream function in a goroutine
		go func() {
			if err := streamFn(client, req); err != nil {
				h.SendToClient(client, SSEEvent{
					Event: "error",
					Data:  StreamChunk{Error: err.Error(), Done: true},
				})
			}
			h.Disconnect(client)
		}()

		<-client.quit
	}
}

// ── Event helpers ────────────────────────────────────────────────────────────

// NewChunkEvent creates a streaming content chunk event.
func NewChunkEvent(content string) SSEEvent {
	return SSEEvent{
		Event: "chunk",
		Data:  StreamChunk{Content: content},
	}
}

// NewToolCallEvent creates a tool-call-progress event.
func NewToolCallEvent(id, name, args string, done bool, errStr string) SSEEvent {
	return SSEEvent{
		Event: "tool_call",
		Data: ToolCallChunk{
			ID:    id,
			Name:  name,
			Arguments: args,
			Done:  done,
			Error: errStr,
		},
	}
}

// NewDoneEvent creates a terminal event with usage info.
func NewDoneEvent(model string, usage *Usage) SSEEvent {
	return SSEEvent{
		Event: "done",
		Data: StreamChunk{
			Done:  true,
			Model: model,
			Usage: usage,
		},
	}
}

// NewStatusEvent creates an agent status update event.
func NewStatusEvent(agent, status, message string) SSEEvent {
	return SSEEvent{
		Event: "status",
		Data: StatusEvent{
			Agent:   agent,
			Status:  status,
			Message: message,
		},
	}
}

// NewErrorEvent creates an error event.
func NewErrorEvent(errStr string) SSEEvent {
	return SSEEvent{
		Event: "error",
		Data:  StreamChunk{Error: errStr, Done: true},
	}
}