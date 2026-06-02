// Package bus — CSP message bus for agent communication.

package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// MessageKind classifies a bus message.
type MessageKind string

const (
	KindCommand   MessageKind = "command"
	KindEvent     MessageKind = "event"
	KindQuery     MessageKind = "query"
	KindResponse  MessageKind = "response"
	KindHeartbeat MessageKind = "heartbeat"
)

// Envelope is the universal message type on the CSP bus.
type Envelope struct {
	ID        string          `json:"id"`
	Source    string          `json:"source"`
	Target    string          `json:"target"`
	Kind      MessageKind     `json:"kind"`
	Topic     string          `json:"topic"`
	Payload   json.RawMessage `json:"payload"`
	TraceID   string          `json:"traceId"`
	Timestamp time.Time       `json:"timestamp"`
	TTL       time.Duration   `json:"ttl"`
}

// Filter is used to select messages from a subscription.
type Filter func(Envelope) bool

// DefaultFilter passes all messages.
var DefaultFilter Filter = func(e Envelope) bool { return true }

// Bus is the CSP pub/sub/message-passing bus.
type Bus interface {
	Publish(ctx context.Context, env Envelope)
	Subscribe(topic string, filter Filter) (<-chan Envelope, error)
	Request(ctx context.Context, env Envelope, timeout time.Duration) (Envelope, error)
	Broadcast(ctx context.Context, topic string, payload any) error
	Close() error
}

type topicSub struct {
	ch     chan Envelope
	filter Filter
}

// NewLocal creates a new in-process CSP bus.
func NewLocal() Bus {
	return newLocalBus()
}

type localBus struct {
	mu      sync.RWMutex
	topics  map[string]map[*topicSub]struct{}
	seq     atomic.Int64
	closed  atomic.Bool
	timeout time.Duration
}

func newLocalBus() *localBus {
	return &localBus{
		topics:  make(map[string]map[*topicSub]struct{}),
		timeout: 30 * time.Second,
	}
}

func (b *localBus) Publish(_ context.Context, env Envelope) {
	if env.ID == "" {
		env.ID = uuid.New().String()
	}
	if env.Timestamp.IsZero() {
		env.Timestamp = time.Now()
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	subs, ok := b.topics[env.Topic]
	if !ok {
		return
	}

	for sub := range subs {
		if sub.filter(env) {
			select {
			case sub.ch <- env:
			default:
			}
		}
	}
}

func (b *localBus) Subscribe(topic string, filter Filter) (<-chan Envelope, error) {
	if filter == nil {
		filter = DefaultFilter
	}

	ch := make(chan Envelope, 100)
	sub := &topicSub{ch: ch, filter: filter}

	b.mu.Lock()
	if b.topics[topic] == nil {
		b.topics[topic] = make(map[*topicSub]struct{})
	}
	b.topics[topic][sub] = struct{}{}
	b.mu.Unlock()

	return ch, nil
}

func (b *localBus) Request(ctx context.Context, env Envelope, timeout time.Duration) (Envelope, error) {
	if timeout == 0 {
		timeout = b.timeout
	}
	if env.ID == "" {
		env.ID = uuid.New().String()
	}

	topic := fmt.Sprintf("agent.%s.response", env.Target)
	subCh, err := b.Subscribe(topic, func(e Envelope) bool {
		return e.ID == env.ID
	})
	if err != nil {
		return Envelope{}, err
	}

	b.Publish(ctx, env)

	select {
	case resp := <-subCh:
		return resp, nil
	case <-ctx.Done():
		return Envelope{}, ctx.Err()
	case <-time.After(timeout):
		return Envelope{}, fmt.Errorf("request timeout after %s", timeout)
	}
}

func (b *localBus) Broadcast(ctx context.Context, topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	b.Publish(ctx, Envelope{
		ID:        uuid.New().String(),
		Source:    "system",
		Target:    "broadcast",
		Kind:      KindEvent,
		Topic:     topic,
		Payload:   data,
		Timestamp: time.Now(),
	})
	return nil
}

func (b *localBus) Close() error {
	if b.closed.Swap(true) {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, subs := range b.topics {
		for sub := range subs {
			close(sub.ch)
		}
	}
	return nil
}