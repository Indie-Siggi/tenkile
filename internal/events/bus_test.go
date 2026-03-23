package events

import (
	"sync"
	"testing"
	"time"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

func TestNewEventBus(t *testing.T) {
	bus := NewEventBus()

	if bus == nil {
		t.Fatal("expected non-nil event bus")
	}
	if bus.subscribers == nil {
		t.Error("expected non-nil subscribers map")
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	bus := NewEventBus()
	ch := make(chan *Event, 10)

	// Subscribe
	unsubscribe := bus.Subscribe("test-topic", ch)
	if unsubscribe == nil {
		t.Fatal("expected non-nil unsubscribe function")
	}

	// Verify subscription
	if bus.SubscriberCount("test-topic") != 1 {
		t.Errorf("expected 1 subscriber, got %d", bus.SubscriberCount("test-topic"))
	}

	// Unsubscribe
	unsubscribe()

	// Verify unsubscribed
	if bus.SubscriberCount("test-topic") != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", bus.SubscriberCount("test-topic"))
	}
}

func TestPublish(t *testing.T) {
	bus := NewEventBus()
	ch := make(chan *Event, 10)
	bus.Subscribe("test-topic", ch)

	event := &Event{
		Type:    "test.event",
		Topic:   "test-topic",
		Payload: map[string]string{"key": "value"},
	}

	bus.Publish(event)

	// Receive event
	select {
	case received := <-ch:
		if received.Type != "test.event" {
			t.Errorf("expected type 'test.event', got %q", received.Type)
		}
		if received.Topic != "test-topic" {
			t.Errorf("expected topic 'test-topic', got %q", received.Topic)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestPublishMultipleSubscribers(t *testing.T) {
	bus := NewEventBus()
	ch1 := make(chan *Event, 10)
	ch2 := make(chan *Event, 10)

	bus.Subscribe("test-topic", ch1)
	bus.Subscribe("test-topic", ch2)

	event := &Event{Type: "test", Topic: "test-topic"}
	bus.Publish(event)

	// Both should receive
	for i, ch := range []chan *Event{ch1, ch2} {
		select {
		case <-ch:
			// OK
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d did not receive event", i+1)
		}
	}
}

func TestUnsubscribeRemovesSubscriber(t *testing.T) {
	bus := NewEventBus()
	ch1 := make(chan *Event, 10)
	ch2 := make(chan *Event, 10)

	unsubscribe1 := bus.Subscribe("test-topic", ch1)
	bus.Subscribe("test-topic", ch2)

	if bus.SubscriberCount("test-topic") != 2 {
		t.Fatalf("expected 2 subscribers, got %d", bus.SubscriberCount("test-topic"))
	}

	// Unsubscribe first
	unsubscribe1()

	if bus.SubscriberCount("test-topic") != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", bus.SubscriberCount("test-topic"))
	}

	// Publish should only reach ch2
	event := &Event{Type: "test", Topic: "test-topic"}
	bus.Publish(event)

	select {
	case <-ch1:
		t.Error("ch1 should not receive event after unsubscribe")
	case <-ch2:
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event on ch2")
	}
}

func TestTopicWildcard(t *testing.T) {
	bus := NewEventBus()
	allCh := make(chan *Event, 10)
	topicCh := make(chan *Event, 10)

	bus.Subscribe("all", allCh)
	bus.Subscribe("specific-topic", topicCh)

	event := &Event{Type: "test", Topic: "specific-topic"}
	bus.Publish(event)

	// Both subscribers should receive (wildcard + exact match)
	for i, ch := range []chan *Event{allCh, topicCh} {
		select {
		case <-ch:
			// OK
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d did not receive event", i+1)
		}
	}
}

func TestClose(t *testing.T) {
	bus := NewEventBus()
	ch := make(chan *Event, 10)
	bus.Subscribe("test-topic", ch)

	// Close should clean up
	bus.Close()

	// Verify no subscribers
	if bus.SubscriberCount("test-topic") != 0 {
		t.Errorf("expected 0 subscribers after close, got %d", bus.SubscriberCount("test-topic"))
	}
}

func TestConcurrentSubscribe(t *testing.T) {
	bus := NewEventBus()
	const subscriberCount = 10

	var wg sync.WaitGroup
	wg.Add(subscriberCount)

	// Subscribe concurrently
	for i := 0; i < subscriberCount; i++ {
		go func() {
			defer wg.Done()
			ch := make(chan *Event, 10)
			bus.Subscribe("concurrent-topic", ch)
		}()
	}

	wg.Wait()

	if bus.SubscriberCount("concurrent-topic") != subscriberCount {
		t.Errorf("expected %d subscribers, got %d", subscriberCount, bus.SubscriberCount("concurrent-topic"))
	}
}

func TestConcurrentPublish(t *testing.T) {
	// Use a fresh bus to avoid global bus interference
	bus := NewEventBus()
	
	// Use a buffered channel large enough for all events
	ch := make(chan *Event, 200)
	
	// Subscribe to both topic and "all"
	bus.Subscribe("concurrent-topic", ch)
	bus.Subscribe("all", ch)

	const publishCount = 100

	var wg sync.WaitGroup
	wg.Add(publishCount)

	// Publish concurrently
	for i := 0; i < publishCount; i++ {
		go func(n int) {
			defer wg.Done()
			event := &Event{Type: "test", Topic: "concurrent-topic"}
			bus.Publish(event)
		}(i)
	}

	wg.Wait()

	// Give goroutines time to deliver events
	time.Sleep(100 * time.Millisecond)

	// Due to wildcard subscription, we get 2x events (one for topic, one for "all")
	expected := publishCount * 2
	
	// Drain the channel
	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			goto done
		}
	}
done:

	t.Logf("Received %d events for %d published (expected %d)", received, publishCount, expected)
	
	if received != expected {
		t.Errorf("expected %d events, received %d", expected, received)
	}
}

func TestGlobalEventBus(t *testing.T) {
	// Test that global bus is initialized
	bus := GetBus()
	if bus == nil {
		t.Fatal("expected non-nil global bus")
	}

	// Subscribe to global bus
	ch := make(chan *Event, 10)
	unsubscribe := Subscribe("global-test", ch)
	defer unsubscribe()

	// Publish to global bus
	event := &Event{Type: "global.test", Topic: "global-test"}
	Publish(event)

	select {
	case received := <-ch:
		if received.Type != "global.test" {
			t.Errorf("expected type 'global.test', got %q", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event on global bus")
	}
}

func TestPublishEvent(t *testing.T) {
	// Reset global bus for test isolation
	bus := NewEventBus()
	SetBus(bus)

	ch := make(chan *Event, 10)
	Subscribe("test-topic", ch)

	// Use PublishEvent helper
	PublishEvent(EventType("custom.event"), "test-topic", map[string]int{"value": 42})

	select {
	case received := <-ch:
		if received.Type != "custom.event" {
			t.Errorf("expected type 'custom.event', got %q", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestSetBus(t *testing.T) {
	customBus := NewEventBus()
	SetBus(customBus)

	if GetBus() != customBus {
		t.Error("GetBus should return the custom bus")
	}

	// Reset to original
	SetBus(NewEventBus())
}
