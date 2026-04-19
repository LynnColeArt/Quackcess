package mcp

import (
	"sync"
	"time"
)

const (
	eventCallStarted           = "mcp.call.started"
	eventCallSuccess           = "mcp.call.success"
	eventCallFailure           = "mcp.call.failure"
	eventCallDenied            = "mcp.call.denied"
	eventVectorRebuildProgress = "mcp.vector.rebuild.progress"
)

type Event struct {
	Timestamp time.Time      `json:"timestamp"`
	Type      string         `json:"type"`
	Tool      string         `json:"tool"`
	Principal string         `json:"principal,omitempty"`
	RequestID string         `json:"requestId,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

func (e Event) withTimestamp() Event {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	return e
}

type EventBus struct {
	mu       sync.RWMutex
	nextID   int
	channels map[int]chan Event
}

func NewEventBus() *EventBus {
	return &EventBus{
		channels: map[int]chan Event{},
	}
}

func (b *EventBus) Publish(event Event) {
	if b == nil {
		return
	}
	event = event.withTimestamp()

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.channels {
		select {
		case ch <- event:
		default:
			// best effort channel delivery
		}
	}
}

func (b *EventBus) Subscribe(bufferSize int) (<-chan Event, func()) {
	if b == nil {
		ch := make(chan Event)
		return ch, func() { close(ch) }
	}
	if bufferSize <= 0 {
		bufferSize = 16
	}

	ch := make(chan Event, bufferSize)
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.channels[id] = ch
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		channel, exists := b.channels[id]
		if exists {
			delete(b.channels, id)
			close(channel)
		}
		b.mu.Unlock()
	}
	return ch, cancel
}
