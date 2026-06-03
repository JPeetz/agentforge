package channel

import (
	"encoding/json"
	"fmt"
	"time"
)

// GatewayMessage is the normalized representation of a message from any channel.
// All channel adapters emit this format for unified downstream processing.
type GatewayMessage struct {
	ID         string          `json:"id"`
	Channel    string          `json:"channel"`
	SenderID   string          `json:"senderId"`
	SenderName string          `json:"senderName"`
	Text       string          `json:"text"`
	Command    string          `json:"command"`
	Args       []string        `json:"args"`
	ChatID     string          `json:"chatId"`
	IsCommand  bool            `json:"isCommand"`
	Raw        json.RawMessage `json:"raw,omitempty"`
	ReceivedAt time.Time       `json:"receivedAt"`
}

// Route is a routing rule that directs incoming messages to target agents.
type Route struct {
	Channel     string `json:"channel,omitempty"`
	SenderID    string `json:"senderId,omitempty"`
	Command     string `json:"command,omitempty"`
	TargetAgent string `json:"targetAgent"`
	Priority    int    `json:"priority"`
}

// RoutingTable matches incoming GatewayMessages to target agents.
type RoutingTable struct {
	routes []Route
}

// NewRoutingTable creates a routing table from config routes, sorted by priority descending.
func NewRoutingTable(routes []Route) *RoutingTable {
	sorted := make([]Route, len(routes))
	copy(sorted, routes)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Priority < sorted[j].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return &RoutingTable{routes: sorted}
}

// Resolve finds the best matching route for a message. Returns target agent ID or "".
func (rt *RoutingTable) Resolve(msg *GatewayMessage) string {
	for _, route := range rt.routes {
		if route.Channel != "" && route.Channel != msg.Channel {
			continue
		}
		if route.SenderID != "" && route.SenderID != msg.SenderID {
			continue
		}
		if route.Command != "" && route.Command != msg.Command {
			continue
		}
		return route.TargetAgent
	}
	return ""
}

// ResolveFallback returns the default agent (last route with no filters).
func (rt *RoutingTable) ResolveFallback() string {
	if len(rt.routes) > 0 {
		return rt.routes[len(rt.routes)-1].TargetAgent
	}
	return ""
}

// NormalizeMessage converts a raw channel message into a GatewayMessage.
func NormalizeMessage(channel, senderID, senderName, text, chatID string, rawPayload any) *GatewayMessage {
	msg := &GatewayMessage{
		Channel:    channel,
		SenderID:   senderID,
		SenderName: senderName,
		Text:       text,
		ChatID:     chatID,
		ReceivedAt: time.Now(),
	}

	if len(text) > 0 && text[0] == '/' {
		msg.IsCommand = true
		msg.Command = text
		for i, c := range text {
			if c == ' ' {
				msg.Command = text[:i]
				msg.Args = splitArgs(text[i+1:])
				break
			}
		}
	}

	if rawPayload != nil {
		data, _ := json.Marshal(rawPayload)
		msg.Raw = data
	}
	return msg
}

func splitArgs(s string) []string {
	var args []string
	var current string
	inQuote := false
	for _, c := range s {
		switch c {
		case '"':
			inQuote = !inQuote
		case ' ':
			if inQuote {
				current += string(c)
			} else {
				if current != "" {
					args = append(args, current)
					current = ""
				}
			}
		default:
			current += string(c)
		}
	}
	if current != "" {
		args = append(args, current)
	}
	return args
}

// GatewayStatus provides live status for all connected channels.
type GatewayStatus struct {
	Channels []Status `json:"channels"`
	Total    int      `json:"total"`
	Running  int      `json:"running"`
	Messages int64    `json:"messages"`
}

// Gateway wraps the channel Manager with routing and normalization.
type Gateway struct {
	manager *Manager
	routing *RoutingTable
}

// NewGateway creates a unified multi-channel gateway.
func NewGateway(mgr *Manager, routes []Route) *Gateway {
	if mgr == nil {
		mgr = &Manager{adapters: make(map[string]Adapter)}
	}
	return &Gateway{manager: mgr, routing: NewRoutingTable(routes)}
}

// Status returns aggregated status across all channels.
func (g *Gateway) Status() GatewayStatus {
	var totalMsgs int64
	statuses := g.manager.Status()
	running := 0
	for _, st := range statuses {
		totalMsgs += st.Messages
		if st.Running {
			running++
		}
	}
	return GatewayStatus{
		Channels: statuses,
		Total:    len(statuses),
		Running:  running,
		Messages: totalMsgs,
	}
}

// Manager returns the underlying channel manager.
func (g *Gateway) Manager() *Manager { return g.manager }

// Routing returns the routing table.
func (g *Gateway) Routing() *RoutingTable { return g.routing }

// String formats gateway status for TUI display.
func (gs GatewayStatus) String() string {
	var result string
	for _, ch := range gs.Channels {
		dot := "✗"
		if ch.Running {
			dot = "●"
		}
		result += fmt.Sprintf("%s %s: %d msgs\n", dot, ch.Name, ch.Messages)
	}
	result += fmt.Sprintf("\n%d/%d channels connected\n", gs.Running, gs.Total)
	return result
}
