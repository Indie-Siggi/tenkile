// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package circuitbreaker

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBreakerInitialState(t *testing.T) {
	cb := New(DefaultConfig("test"))
	if cb.State() != StateClosed {
		t.Errorf("expected initial state to be closed, got %s", cb.State())
	}
}

func TestBreakerAllowsCallsWhenClosed(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
	})

	for i := 0; i < 10; i++ {
		if err := cb.Allow(); err != nil {
			t.Errorf("expected call to be allowed, got error: %v", err)
		}
	}
}

func TestBreakerOpensAfterFailures(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
	})

	// Record failures (threshold is 3)
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.State() != StateOpen {
		t.Errorf("expected state to be open after 3 failures, got %s", cb.State())
	}

	// Should be rejected
	if err := cb.Allow(); err != ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestBreakerHalfOpenAfterTimeout(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Open the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("expected state to be open, got %s", cb.State())
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Allow() call should trigger transition to half-open
	if err := cb.Allow(); err != nil {
		t.Errorf("expected call to be allowed (transition to half-open), got error: %v", err)
	}

	// Now state should be half-open
	if cb.State() != StateHalfOpen {
		t.Errorf("expected state to be half-open after timeout, got %s", cb.State())
	}
}

func TestBreakerClosesAfterSuccesses(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Open and transition to half-open
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // Enter half-open

	// Record successes
	cb.RecordSuccess()
	if cb.State() != StateHalfOpen {
		t.Errorf("expected state to still be half-open after 1 success, got %s", cb.State())
	}

	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("expected state to be closed after 2 successes, got %s", cb.State())
	}
}

func TestBreakerReopensOnFailureInHalfOpen(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Open and transition to half-open
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // Enter half-open

	// Failure in half-open
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Errorf("expected state to be open after failure in half-open, got %s", cb.State())
	}
}

func TestBreakerDoExecutesFunction(t *testing.T) {
	cb := New(DefaultConfig("test"))

	called := false
	err := cb.Do(func() error {
		called = true
		return nil
	})

	if !called {
		t.Error("expected function to be called")
	}
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestBreakerDoRecordsFailure(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
	})

	expectedErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		cb.Do(func() error {
			return expectedErr
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("expected circuit to be open, got %s", cb.State())
	}
}

func TestBreakerDoRejectsWhenOpen(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 1,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
	})

	// Open the circuit
	cb.Do(func() error {
		return errors.New("fail")
	})

	// Should be rejected
	err := cb.Do(func() error {
		t.Error("function should not be called when circuit is open")
		return nil
	})

	if err != ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestBreakerResetsFailuresOnSuccess(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
	})

	// Record some failures
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	stats := cb.Stats()
	if stats.Failures != 3 {
		t.Errorf("expected 3 failures, got %d", stats.Failures)
	}

	// Record success
	cb.RecordSuccess()

	stats = cb.Stats()
	if stats.Failures != 0 {
		t.Errorf("expected failures to be reset to 0, got %d", stats.Failures)
	}
}

func TestBreakerStateChangeCallback(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 3,
		Timeout:          50 * time.Millisecond,
	})

	var changes []struct {
		from, to State
	}

	cb.StateChangeHandler(func(name string, from, to State) {
		changes = append(changes, struct{ from, to State }{from, to})
	})

	// Open circuit (threshold is 3)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	if len(changes) != 1 || changes[0].to != StateOpen {
		t.Errorf("expected state change to open, got %v", changes)
	}

	// Transition to half-open (wait for timeout then call Allow)
	time.Sleep(60 * time.Millisecond)
	cb.Allow()

	if len(changes) != 2 || changes[1].to != StateHalfOpen {
		t.Errorf("expected state change to half-open, got %v", changes)
	}
}

func TestBreakerConcurrentAccess(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 100,
		SuccessThreshold: 3,
		Timeout:          50 * time.Millisecond,
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cb.RecordSuccess()
				cb.RecordFailure()
				cb.Allow()
				cb.State()
				cb.Stats()
			}
		}()
	}

	wg.Wait()

	// Should not panic and should have reasonable state
	stats := cb.Stats()
	if stats.Name != "test" {
		t.Errorf("expected name to be 'test', got %s", stats.Name)
	}
}

func TestMultiBreaker(t *testing.T) {
	mb := NewMulti(Config{
		Name:             "default",
		FailureThreshold: 2,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
	})

	// Get two different breakers
	b1 := mb.Get("service-a")
	b2 := mb.Get("service-b")
	b1Again := mb.Get("service-a")

	// Should be the same instance
	if b1 != b1Again {
		t.Error("expected same breaker instance for same name")
	}

	// Should be different instances
	if b1 == b2 {
		t.Error("expected different breaker instances for different names")
	}

	// Operate on one (threshold is 2)
	b1.RecordFailure()
	b1.RecordFailure()

	if b1.State() != StateOpen {
		t.Errorf("expected b1 to be open, got %s", b1.State())
	}

	// b2 should still be closed
	if b2.State() != StateClosed {
		t.Errorf("expected b2 to be closed, got %s", b2.State())
	}
}

func TestMultiBreakerStats(t *testing.T) {
	mb := NewMulti(DefaultConfig("default"))

	mb.Get("service-a")
	mb.Get("service-b")

	stats := mb.Stats()
	if len(stats) != 2 {
		t.Errorf("expected 2 breakers, got %d", len(stats))
	}
}

func TestBreakerDoWithResult(t *testing.T) {
	cb := New(DefaultConfig("test"))

	result, err := cb.DoWithResult(func() (interface{}, error) {
		return "success", nil
	})

	if result != "success" {
		t.Errorf("expected 'success', got %v", result)
	}
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestBreakerDoWithResultRejectsWhenOpen(t *testing.T) {
	cb := New(Config{
		Name:             "test",
		FailureThreshold: 1,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
	})

	// Open the circuit
	cb.Do(func() error {
		return errors.New("fail")
	})

	// Should be rejected
	_, err := cb.DoWithResult(func() (interface{}, error) {
		t.Error("function should not be called when circuit is open")
		return nil, nil
	})

	if err != ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestBreakerStats(t *testing.T) {
	cb := New(Config{
		Name:             "my-breaker",
		FailureThreshold: 10,
		SuccessThreshold: 5,
		Timeout:          1 * time.Second,
	})

	stats := cb.Stats()
	if stats.Name != "my-breaker" {
		t.Errorf("expected name 'my-breaker', got %s", stats.Name)
	}
	if stats.State != "closed" {
		t.Errorf("expected state 'closed', got %s", stats.State)
	}
	if stats.Threshold != 10 {
		t.Errorf("expected threshold 10, got %d", stats.Threshold)
	}
}

func TestBreakerDefaultConfig(t *testing.T) {
	config := DefaultConfig("test")
	if config.Name != "test" {
		t.Errorf("expected name 'test', got %s", config.Name)
	}
	if config.FailureThreshold != 5 {
		t.Errorf("expected FailureThreshold 5, got %d", config.FailureThreshold)
	}
	if config.SuccessThreshold != 3 {
		t.Errorf("expected SuccessThreshold 3, got %d", config.SuccessThreshold)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", config.Timeout)
	}
}
