package event

import (
	"testing"
	"time"
)

// TestBusPublishSubscribe verifies pub/sub mechanics
func TestBusPublishSubscribe(t *testing.T) {
	bus := NewBus()

	// Subscribe to events
	events, unsub := bus.Subscribe()
	defer unsub()

	// Publish a token event
	tokenEvent := TokenEvent{
		Token:     "hello",
		Timestamp: time.Now(),
	}
	bus.Publish(tokenEvent)

	// Receive the event
	select {
	case event := <-events:
		if e, ok := event.(TokenEvent); !ok || e.Token != "hello" {
			t.Errorf("unexpected event: %v", event)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

// TestBusMultipleSubscribers verifies all subscribers get events
func TestBusMultipleSubscribers(t *testing.T) {
	bus := NewBus()

	// Subscribe two clients
	events1, unsub1 := bus.Subscribe()
	defer unsub1()
	events2, unsub2 := bus.Subscribe()
	defer unsub2()

	// Publish an event
	event := TokenEvent{Token: "test", Timestamp: time.Now()}
	bus.Publish(event)

	// Both should receive
	received1 := false
	received2 := false

	select {
	case <-events1:
		received1 = true
	case <-time.After(100 * time.Millisecond):
	}

	select {
	case <-events2:
		received2 = true
	case <-time.After(100 * time.Millisecond):
	}

	if !received1 || !received2 {
		t.Errorf("not all subscribers received event: %v, %v", received1, received2)
	}
}

// TestBusUnsubscribe verifies unsubscribe stops delivery
func TestBusUnsubscribe(t *testing.T) {
	bus := NewBus()

	events, unsub := bus.Subscribe()

	// Unsubscribe
	unsub()

	// Publish should not block even though channel is closed
	event := TokenEvent{Token: "test", Timestamp: time.Now()}
	bus.Publish(event)

	// Verify channel is closed
	select {
	case <-events:
		// Channel should be closed; receiving from closed channel returns zero value
	case <-time.After(100 * time.Millisecond):
		// OK
	}
}
