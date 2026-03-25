// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package circuitbreaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Breaker implements the circuit breaker pattern
type Breaker struct {
	name             string
	failureThreshold int32
	successThreshold int32
	timeout          time.Duration
	halfOpenMax      int32

	state           int32 // 0=closed, 1=open, 2=half-open
	failures        int32
	successes       int32
	lastFailureTime int64

	mu sync.RWMutex
	onStateChange func(name string, from, to State)
}

// LoadState atomically loads the state
func (b *Breaker) LoadState() int32 {
	return atomic.LoadInt32(&b.state)
}

// StoreState atomically stores the state
func (b *Breaker) StoreState(s int32) {
	atomic.StoreInt32(&b.state, s)
}

// Config holds circuit breaker configuration
type Config struct {
	Name             string        // Circuit name for logging/metrics
	FailureThreshold int           // Failures to open circuit (default: 5)
	SuccessThreshold int           // Successes in half-open to close (default: 3)
	Timeout          time.Duration // Time before half-open (default: 30s)
	HalfOpenMax      int           // Max calls in half-open (default: 1)
}

// DefaultConfig returns default configuration
func DefaultConfig(name string) Config {
	return Config{
		Name:             name,
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		HalfOpenMax:      1,
	}
}

// New creates a new circuit breaker
func New(config Config) *Breaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 3
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.HalfOpenMax <= 0 {
		config.HalfOpenMax = 1
	}

	return &Breaker{
		name:             config.Name,
		failureThreshold: int32(config.FailureThreshold),
		successThreshold: int32(config.SuccessThreshold),
		timeout:          config.Timeout,
		halfOpenMax:      int32(config.HalfOpenMax),
		state:           int32(StateClosed),
		onStateChange:    nil,
	}
}

// StateChangeHandler sets a callback for state changes
func (b *Breaker) StateChangeHandler(fn func(name string, from, to State)) {
	b.mu.Lock()
	b.onStateChange = fn
	b.mu.Unlock()
}

// State returns the current state
func (b *Breaker) State() State {
	return State(b.LoadState())
}

// Allow checks if a request should be allowed through
func (b *Breaker) Allow() error {
	state := b.State()

	switch state {
	case StateClosed:
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		lastFailure := time.Unix(0, atomic.LoadInt64(&b.lastFailureTime))
		if time.Since(lastFailure) > b.timeout {
			b.toState(StateHalfOpen)
			// Re-check state after transition — another goroutine may have changed it
			if State(b.LoadState()) != StateHalfOpen {
				return ErrCircuitOpen
			}
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		// Allow limited requests in half-open
		successes := atomic.LoadInt32(&b.successes)
		if successes < b.halfOpenMax {
			return nil
		}
		return ErrCircuitOpen

	default:
		return ErrCircuitOpen
	}
}

// RecordSuccess records a successful call
func (b *Breaker) RecordSuccess() {
	state := b.State()

	switch state {
	case StateClosed:
		// Reset failure count on success
		atomic.StoreInt32(&b.failures, 0)

	case StateHalfOpen:
		successes := atomic.AddInt32(&b.successes, 1)
		if successes >= b.successThreshold {
			atomic.StoreInt32(&b.successes, 0)
			b.toState(StateClosed)
		}

	case StateOpen:
		// Shouldn't happen but handle gracefully
	}
}

// RecordFailure records a failed call
func (b *Breaker) RecordFailure() {
	state := b.State()

	switch state {
	case StateClosed:
		failures := atomic.AddInt32(&b.failures, 1)
		if failures >= b.failureThreshold {
			atomic.StoreInt64(&b.lastFailureTime, time.Now().UnixNano())
			b.toState(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in half-open opens the circuit
		atomic.StoreInt64(&b.lastFailureTime, time.Now().UnixNano())
		b.toState(StateOpen)

	case StateOpen:
		// Update last failure time
		atomic.StoreInt64(&b.lastFailureTime, time.Now().UnixNano())
	}
}

// Do executes the function if circuit allows
func (b *Breaker) Do(fn func() error) error {
	if err := b.Allow(); err != nil {
		return err
	}

	err := fn()
	if err != nil {
		b.RecordFailure()
	} else {
		b.RecordSuccess()
	}
	return err
}

// DoWithResult executes the function and returns result if circuit allows
func (b *Breaker) DoWithResult(fn func() (interface{}, error)) (interface{}, error) {
	if err := b.Allow(); err != nil {
		return nil, err
	}

	result, err := fn()
	if err != nil {
		b.RecordFailure()
	} else {
		b.RecordSuccess()
	}
	return result, err
}

// toState transitions to a new state
func (b *Breaker) toState(newState State) {
	oldState := State(b.LoadState())
	b.StoreState(int32(newState))
	
	if oldState != newState {
		b.onStateChangeUnsafe(oldState, newState)
	}
}

// onStateChangeUnsafe calls the state change handler (must hold lock)
func (b *Breaker) onStateChangeUnsafe(from, to State) {
	b.mu.RLock()
	fn := b.onStateChange
	b.mu.RUnlock()

	if fn != nil {
		fn(b.name, from, to)
	}
}

// Stats returns current circuit breaker statistics
func (b *Breaker) Stats() Stats {
	return Stats{
		Name:        b.name,
		State:       b.State().String(),
		Failures:    atomic.LoadInt32(&b.failures),
		Successes:   atomic.LoadInt32(&b.successes),
		Threshold:   int(b.failureThreshold),
		LastFailure: time.Unix(0, atomic.LoadInt64(&b.lastFailureTime)),
	}
}

// Stats holds circuit breaker statistics
type Stats struct {
	Name        string
	State       string
	Failures    int32
	Successes   int32
	Threshold   int
	LastFailure time.Time
}

// MultiBreaker manages multiple circuit breakers
type MultiBreaker struct {
	breakers map[string]*Breaker
	mu       sync.RWMutex
	defaults Config
}

// NewMulti creates a new multi-breaker manager
func NewMulti(defaults Config) *MultiBreaker {
	return &MultiBreaker{
		breakers: make(map[string]*Breaker),
		defaults: defaults,
	}
}

// Get returns a circuit breaker by name, creating if needed
func (m *MultiBreaker) Get(name string) *Breaker {
	m.mu.RLock()
	b, ok := m.breakers[name]
	m.mu.RUnlock()

	if ok {
		return b
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if b, ok = m.breakers[name]; ok {
		return b
	}

	config := m.defaults
	config.Name = name
	b = New(config)
	m.breakers[name] = b
	return b
}

// Stats returns stats for all breakers
func (m *MultiBreaker) Stats() map[string]Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]Stats, len(m.breakers))
	for name, b := range m.breakers {
		result[name] = b.Stats()
	}
	return result
}
