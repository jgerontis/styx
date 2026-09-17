package event

import (
	"sync"
)

// Bus publishes and subscribes to events.
type Bus interface {
	Publish(event Event)
	Subscribe() (<-chan Event, func())
}

// DefaultBus is an in-memory event bus using channels.
type DefaultBus struct {
	mu          sync.RWMutex
	subscribers []chan Event
}

// NewBus creates a new event bus.
func NewBus() *DefaultBus {
	return &DefaultBus{
		subscribers: make([]chan Event, 0),
	}
}

// Publish broadcasts an event to all subscribers.
func (b *DefaultBus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full; drop event to avoid blocking
		}
	}
}

// Subscribe returns a channel for receiving events and an unsubscribe function.
func (b *DefaultBus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Buffered channel to reduce likelihood of dropping events
	ch := make(chan Event, 100)
	b.subscribers = append(b.subscribers, ch)

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		for i, subscriber := range b.subscribers {
			if subscriber == ch {
				close(ch)
				b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
				return
			}
		}
	}

	return ch, unsubscribe
}
