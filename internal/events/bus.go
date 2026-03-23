package events

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// globalEventBus is the package-level event bus instance
var globalEventBus *EventBus

// init initializes the global event bus
func init() {
	globalEventBus = NewEventBus()
}

// EventBus manages event subscriptions and publishing
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan *Event]bool // topic -> channels
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string]map[chan *Event]bool),
	}
}

// Subscribe adds a channel to receive events for the given topic.
// Returns an unsubscribe function that should be called when done.
func (b *EventBus) Subscribe(topic string, ch chan *Event) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subscribers[topic] == nil {
		b.subscribers[topic] = make(map[chan *Event]bool)
	}
	b.subscribers[topic][ch] = true

	// Also subscribe to "all" for wildcard listeners
	if topic != TopicAll {
		if b.subscribers[TopicAll] == nil {
			b.subscribers[TopicAll] = make(map[chan *Event]bool)
		}
		b.subscribers[TopicAll][ch] = true
	}

	return func() {
		b.Unsubscribe(topic, ch)
	}
}

// Unsubscribe removes a channel from receiving events for the given topic
func (b *EventBus) Unsubscribe(topic string, ch chan *Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs := b.subscribers[topic]; subs != nil {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(b.subscribers, topic)
		}
	}

	// Also remove from "all"
	if subs := b.subscribers[TopicAll]; subs != nil {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(b.subscribers, TopicAll)
		}
	}
}

// Publish sends an event to all subscribers of the event's topic
func (b *EventBus) Publish(event *Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Send to topic subscribers
	if subs := b.subscribers[event.Topic]; subs != nil {
		for ch := range subs {
			select {
			case ch <- event:
			default:
				// Channel full, skip
			}
		}
	}

	// Send to "all" topic subscribers
	if event.Topic != TopicAll {
		if subs := b.subscribers[TopicAll]; subs != nil {
			for ch := range subs {
				select {
				case ch <- event:
				default:
					// Channel full, skip
				}
			}
		}
	}
}

// PublishToTopic publishes an event to a specific topic
func (b *EventBus) PublishToTopic(topic string, payload interface{}) {
	event := &Event{
		Type:      EventType("event." + topic),
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
		ID:        generateEventID(),
	}
	b.Publish(event)
}

// SubscriberCount returns the number of subscribers for a topic
func (b *EventBus) SubscriberCount(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := 0
	if subs := b.subscribers[topic]; subs != nil {
		count = len(subs)
	}
	return count
}

// Close cleans up all subscriptions
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Clear all subscriber maps
	for topic := range b.subscribers {
		delete(b.subscribers, topic)
	}
}

// Package-level helpers

// Subscribe creates a subscription to the global event bus
func Subscribe(topic string, ch chan *Event) func() {
	return globalEventBus.Subscribe(topic, ch)
}

// Publish publishes an event to the global event bus
func Publish(event *Event) {
	globalEventBus.Publish(event)
}

// PublishEvent creates and publishes an event with the given type, topic, and payload
func PublishEvent(eventType EventType, topic string, payload interface{}) {
	event := NewEvent(eventType, topic, payload)
	Publish(event)
}

// GetBus returns the global event bus instance
func GetBus() *EventBus {
	return globalEventBus
}

// SetBus sets the global event bus instance (useful for testing)
func SetBus(bus *EventBus) {
	globalEventBus = bus
}

// randomString generates a random hex string of the given length
func randomString(n int) string {
	bytes := make([]byte, n/2+1)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:n]
}

// generateEventID creates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}
