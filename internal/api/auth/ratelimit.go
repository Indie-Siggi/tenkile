// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// BruteForceProtector implements login brute force protection
type BruteForceProtector struct {
	// Track failed attempts per IP
	failures map[string]*failureRecord
	mu       sync.RWMutex

	// Configuration
	maxAttempts    int
	lockoutDuration time.Duration
	cleanupInterval time.Duration

	// Callbacks
	onLockout func(ip string, until time.Time)
	onUnlock  func(ip string)
}

// failureRecord tracks failed login attempts
type failureRecord struct {
	IP          string
	Attempts    int
	FirstAttempt time.Time
	LastAttempt  time.Time
	LockedUntil  time.Time
}

// BruteForceConfig holds brute force protection configuration
type BruteForceConfig struct {
	MaxAttempts     int           // Max failed attempts before lockout (default: 10)
	LockoutMinutes int           // Lockout duration in minutes (default: 15)
	CleanupMinutes int           // Cleanup interval in minutes (default: 5)
}

// DefaultBruteForceConfig returns default brute force configuration
func DefaultBruteForceConfig() BruteForceConfig {
	return BruteForceConfig{
		MaxAttempts:     10,
		LockoutMinutes: 15,
		CleanupMinutes: 5,
	}
}

// NewBruteForceProtector creates a new brute force protector
func NewBruteForceProtector(cfg BruteForceConfig) *BruteForceProtector {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 10
	}
	if cfg.LockoutMinutes <= 0 {
		cfg.LockoutMinutes = 15
	}
	if cfg.CleanupMinutes <= 0 {
		cfg.CleanupMinutes = 5
	}

	p := &BruteForceProtector{
		failures:        make(map[string]*failureRecord),
		maxAttempts:     cfg.MaxAttempts,
		lockoutDuration: time.Duration(cfg.LockoutMinutes) * time.Minute,
		cleanupInterval: time.Duration(cfg.CleanupMinutes) * time.Minute,
	}

	// Start cleanup goroutine
	go p.cleanupLoop()

	return p
}

// IsLocked checks if an IP is currently locked out
func (p *BruteForceProtector) IsLocked(ip string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	record, exists := p.failures[ip]
	if !exists {
		return false
	}

	// Check if locked and lockout hasn't expired
	if record.LockedUntil.After(time.Now()) {
		return true
	}

	return false
}

// GetLockoutRemaining returns the remaining lockout time for an IP
func (p *BruteForceProtector) GetLockoutRemaining(ip string) time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	record, exists := p.failures[ip]
	if !exists || record.LockedUntil.IsZero() {
		return 0
	}

	remaining := time.Until(record.LockedUntil)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RecordFailure records a failed login attempt for an IP
func (p *BruteForceProtector) RecordFailure(ip string) (locked bool, delay time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	record, exists := p.failures[ip]

	if !exists {
		record = &failureRecord{
			IP:          ip,
			Attempts:    0,
			FirstAttempt: now,
		}
		p.failures[ip] = record
	}

	// Check if already locked
	if record.LockedUntil.After(now) {
		return true, time.Until(record.LockedUntil)
	}

	record.Attempts++
	record.LastAttempt = now

	// Check if should be locked
	if record.Attempts >= p.maxAttempts {
		record.LockedUntil = now.Add(p.lockoutDuration)
		if p.onLockout != nil {
			go p.onLockout(ip, record.LockedUntil)
		}
		return true, p.lockoutDuration
	}

	// Calculate progressive delay: 2^(attempts-1) seconds, capped at 16s
	delay = time.Duration(1<<uint(record.Attempts-1)) * time.Second
	if delay > 16*time.Second {
		delay = 16 * time.Second
	}

	return false, delay
}

// RecordSuccess clears the failure record for an IP on successful login
func (p *BruteForceProtector) RecordSuccess(ip string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.failures, ip)
}

// GetAttempts returns the number of failed attempts for an IP
func (p *BruteForceProtector) GetAttempts(ip string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	record, exists := p.failures[ip]
	if !exists {
		return 0
	}
	return record.Attempts
}

// GetStatus returns the current status for an IP
func (p *BruteForceProtector) GetStatus(ip string) BruteForceStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	record, exists := p.failures[ip]
	if !exists {
		return BruteForceStatus{
			IP:           ip,
			Attempts:     0,
			Locked:       false,
			AttemptsLeft: p.maxAttempts,
		}
	}

	locked := record.LockedUntil.After(time.Now())
	attemptsLeft := p.maxAttempts - record.Attempts
	if attemptsLeft < 0 {
		attemptsLeft = 0
	}

	return BruteForceStatus{
		IP:           ip,
		Attempts:     record.Attempts,
		FirstAttempt: record.FirstAttempt,
		LastAttempt:  record.LastAttempt,
		Locked:       locked,
		LockedUntil:  record.LockedUntil,
		AttemptsLeft: attemptsLeft,
	}
}

// BruteForceStatus holds the status for an IP
type BruteForceStatus struct {
	IP           string
	Attempts     int
	FirstAttempt time.Time
	LastAttempt  time.Time
	Locked       bool
	LockedUntil  time.Time
	AttemptsLeft int
}

// OnLockout sets a callback for lockout events
func (p *BruteForceProtector) OnLockout(fn func(ip string, until time.Time)) {
	p.onLockout = fn
}

// OnUnlock sets a callback for unlock events
func (p *BruteForceProtector) OnUnlock(fn func(ip string)) {
	p.onUnlock = fn
}

// cleanupLoop periodically removes expired entries
func (p *BruteForceProtector) cleanupLoop() {
	ticker := time.NewTicker(p.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		p.cleanup()
	}
}

// cleanup removes expired entries
func (p *BruteForceProtector) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for ip, record := range p.failures {
		// Remove if no lockout and old
		if record.LockedUntil.IsZero() && now.Sub(record.LastAttempt) > 24*time.Hour {
			delete(p.failures, ip)
			continue
		}

		// Unlock and remove if lockout expired
		if !record.LockedUntil.IsZero() && now.After(record.LockedUntil) {
			oldLocked := record.LockedUntil
			delete(p.failures, ip)
			if p.onUnlock != nil {
				go p.onUnlock(ip)
			}
			_ = oldLocked // silence unused variable warning
		}
	}
}

// GetClientIP extracts the client IP from an HTTP request
// Handles X-Forwarded-For and X-Real-IP headers
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain multiple IPs)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Get the first IP (client IP)
		if idx := strings.Index(xff, ","); idx != -1 {
			xff = xff[:idx]
		}
		xff = strings.TrimSpace(xff)
		if xff != "" {
			return normalizeIP(xff)
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return normalizeIP(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return normalizeIP(ip)
}

// normalizeIP normalizes an IP address
func normalizeIP(ip string) string {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ip
	}
	return parsedIP.String()
}
