// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package auth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestBruteForceProtectorInitialState(t *testing.T) {
	p := NewBruteForceProtector(DefaultBruteForceConfig())

	if p.IsLocked("192.168.1.1") {
		t.Error("expected IP not to be locked initially")
	}
}

func TestBruteForceProtectorRecordFailure(t *testing.T) {
	p := NewBruteForceProtector(BruteForceConfig{
		MaxAttempts:     3,
		LockoutMinutes: 15,
		CleanupMinutes: 5,
	})

	ip := "192.168.1.1"

	// First two failures should not lock
	for i := 0; i < 2; i++ {
		locked, delay := p.RecordFailure(ip)
		if locked {
			t.Errorf("expected not locked after %d failures, got locked", i+1)
		}
		if delay == 0 {
			t.Error("expected delay on failure")
		}
	}

	// Third failure should lock
	locked, _ := p.RecordFailure(ip)
	if !locked {
		t.Error("expected locked after 3 failures")
	}

	// Should be locked now
	if !p.IsLocked(ip) {
		t.Error("expected IP to be locked")
	}
}

func TestBruteForceProtectorRecordSuccess(t *testing.T) {
	p := NewBruteForceProtector(DefaultBruteForceConfig())

	ip := "192.168.1.1"

	// Record some failures
	p.RecordFailure(ip)
	p.RecordFailure(ip)

	// Record success
	p.RecordSuccess(ip)

	// Should not be locked
	if p.IsLocked(ip) {
		t.Error("expected IP not to be locked after success")
	}

	// Attempts should be reset
	if p.GetAttempts(ip) != 0 {
		t.Errorf("expected 0 attempts after success, got %d", p.GetAttempts(ip))
	}
}

func TestBruteForceProtectorGetStatus(t *testing.T) {
	p := NewBruteForceProtector(BruteForceConfig{
		MaxAttempts:     5,
		LockoutMinutes: 15,
		CleanupMinutes: 5,
	})

	ip := "192.168.1.1"

	// Initial status
	status := p.GetStatus(ip)
	if status.Attempts != 0 {
		t.Errorf("expected 0 attempts, got %d", status.Attempts)
	}
	if status.AttemptsLeft != 5 {
		t.Errorf("expected 5 attempts left, got %d", status.AttemptsLeft)
	}

	// After failures
	p.RecordFailure(ip)
	p.RecordFailure(ip)

	status = p.GetStatus(ip)
	if status.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", status.Attempts)
	}
	if status.AttemptsLeft != 3 {
		t.Errorf("expected 3 attempts left, got %d", status.AttemptsLeft)
	}
}

func TestBruteForceProtectorProgressiveDelay(t *testing.T) {
	p := NewBruteForceProtector(BruteForceConfig{
		MaxAttempts:     10,
		LockoutMinutes: 15,
		CleanupMinutes: 5,
	})

	ip := "192.168.1.1"

	// First failure: 1s delay
	_, delay := p.RecordFailure(ip)
	if delay != 1*time.Second {
		t.Errorf("expected 1s delay, got %v", delay)
	}

	// Second failure: 2s delay
	_, delay = p.RecordFailure(ip)
	if delay != 2*time.Second {
		t.Errorf("expected 2s delay, got %v", delay)
	}

	// Third failure: 4s delay
	_, delay = p.RecordFailure(ip)
	if delay != 4*time.Second {
		t.Errorf("expected 4s delay, got %v", delay)
	}

	// Fourth failure: 8s delay
	_, delay = p.RecordFailure(ip)
	if delay != 8*time.Second {
		t.Errorf("expected 8s delay, got %v", delay)
	}

	// Fifth failure: 16s delay (capped)
	_, delay = p.RecordFailure(ip)
	if delay != 16*time.Second {
		t.Errorf("expected 16s delay, got %v", delay)
	}
}

func TestBruteForceProtectorLockoutDuration(t *testing.T) {
	p := NewBruteForceProtector(BruteForceConfig{
		MaxAttempts:     2,
		LockoutMinutes: 1, // 1 minute
		CleanupMinutes: 5,
	})

	ip := "192.168.1.1"

	// Lock the IP
	p.RecordFailure(ip)
	p.RecordFailure(ip)

	remaining := p.GetLockoutRemaining(ip)
	if remaining < 59*time.Second {
		t.Errorf("expected at least 59s remaining, got %v", remaining)
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name        string
		remoteAddr  string
		headers     map[string]string
		expectedIP  string
	}{
		{
			name:       "direct connection",
			remoteAddr: "192.168.1.100:12345",
			headers:    map[string]string{},
			expectedIP: "192.168.1.100",
		},
		{
			name:       "with X-Forwarded-For single",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100"},
			expectedIP: "192.168.1.100",
		},
		{
			name:       "with X-Forwarded-For multiple",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100, 10.0.0.2, 172.16.0.1"},
			expectedIP: "192.168.1.100",
		},
		{
			name:       "with X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Real-IP": "192.168.1.200"},
			expectedIP: "192.168.1.200",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100", "X-Real-IP": "192.168.1.200"},
			expectedIP: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := GetClientIP(req)
			if got != tt.expectedIP {
				t.Errorf("expected %s, got %s", tt.expectedIP, got)
			}
		})
	}
}

func TestBruteForceProtectorOnLockoutCallback(t *testing.T) {
	p := NewBruteForceProtector(BruteForceConfig{
		MaxAttempts:     1,
		LockoutMinutes: 15,
		CleanupMinutes: 5,
	})

	// Use channel to communicate callback result (avoids race condition)
	resultCh := make(chan struct {
		ip    string
		until time.Time
	}, 1)

	p.OnLockout(func(ip string, until time.Time) {
		resultCh <- struct {
			ip    string
			until time.Time
		}{ip: ip, until: until}
	})

	// First failure should lock (threshold is 1)
	locked, _ := p.RecordFailure("192.168.1.1")

	// Check if locked
	if !locked {
		t.Error("expected locked after 1 failure when threshold is 1")
	}

	// The IP should be locked
	if !p.IsLocked("192.168.1.1") {
		t.Error("expected IP to be locked")
	}

	// Wait for callback with timeout to avoid hanging
	select {
	case result := <-resultCh:
		if result.ip != "192.168.1.1" {
			t.Errorf("expected callback IP '192.168.1.1', got '%s'", result.ip)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for lockout callback")
	}
}
