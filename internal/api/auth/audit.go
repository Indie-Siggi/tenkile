// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	AuditEventLoginSuccess     AuditEventType = "login_success"
	AuditEventLoginFailure     AuditEventType = "login_failure"
	AuditEventLogout           AuditEventType = "logout"
	AuditEventTokenRefresh     AuditEventType = "token_refresh"
	AuditEventAccountLocked    AuditEventType = "account_locked"
	AuditEventAccountUnlocked  AuditEventType = "account_unlocked"
	AuditEventPasswordChange   AuditEventType = "password_change"
	AuditEventPasswordReset    AuditEventType = "password_reset"
	AuditEventBruteForceAttempt AuditEventType = "brute_force_attempt"
	AuditEventFirstRun         AuditEventType = "first_run"
)

// AuditEvent represents an authentication audit event
type AuditEvent struct {
	ID           int64          `json:"id"`
	EventType    AuditEventType `json:"event_type"`
	IPAddress    string         `json:"ip_address"`
	UserAgent    string         `json:"user_agent,omitempty"`
	Username     string         `json:"username,omitempty"`
	UserID       string         `json:"user_id,omitempty"`
	Success      bool           `json:"success"`
	FailureReason string        `json:"failure_reason,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
}

// AuditLogger handles authentication audit logging
type AuditLogger struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(db *sql.DB, logger *slog.Logger) *AuditLogger {
	if logger == nil {
		logger = slog.Default()
	}

	al := &AuditLogger{
		db:     db,
		logger: logger,
	}

	// Initialize database table
	al.initTable()

	return al
}

// initTable creates the audit_events table if it doesn't exist
func (a *AuditLogger) initTable() {
	if a.db == nil {
		return
	}

	_, err := a.db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type TEXT NOT NULL,
			ip_address TEXT,
			user_agent TEXT,
			username TEXT,
			user_id TEXT,
			success INTEGER NOT NULL DEFAULT 1,
			failure_reason TEXT,
			metadata TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_audit_timestamp (timestamp),
			INDEX idx_audit_username (username),
			INDEX idx_audit_event_type (event_type),
			INDEX idx_audit_ip (ip_address)
		)
	`)
	if err != nil {
		a.logger.Warn("Failed to create audit_events table", "error", err)
	}
}

// Log records an audit event
func (a *AuditLogger) Log(ctx context.Context, event AuditEvent) error {
	event.Timestamp = time.Now()

	// Always log to structured logger
	a.logToSlogger(event)

	// Store in database if available
	if a.db != nil {
		return a.storeInDB(ctx, event)
	}

	return nil
}

// logToSlogger logs the event to slog
func (a *AuditLogger) logToSlogger(event AuditEvent) {
	args := []any{
		"event_type", string(event.EventType),
		"ip", event.IPAddress,
		"success", event.Success,
	}

	if event.Username != "" {
		args = append(args, "username", event.Username)
	}
	if event.UserID != "" {
		args = append(args, "user_id", event.UserID)
	}
	if event.FailureReason != "" {
		args = append(args, "reason", event.FailureReason)
	}
	if event.UserAgent != "" {
		args = append(args, "user_agent", event.UserAgent)
	}

	switch event.EventType {
	case AuditEventLoginSuccess:
		a.logger.Info("Login successful", args...)
	case AuditEventLoginFailure:
		a.logger.Warn("Login failed", args...)
	case AuditEventAccountLocked:
		a.logger.Warn("Account locked", args...)
	case AuditEventAccountUnlocked:
		a.logger.Info("Account unlocked", args...)
	case AuditEventBruteForceAttempt:
		a.logger.Warn("Brute force attempt detected", args...)
	default:
		a.logger.Info("Auth event", args...)
	}
}

// storeInDB stores the event in the database
func (a *AuditLogger) storeInDB(ctx context.Context, event AuditEvent) error {
	metadataJSON := ""
	if len(event.Metadata) > 0 {
		data, err := json.Marshal(event.Metadata)
		if err == nil {
			metadataJSON = string(data)
		}
	}

	success := 0
	if event.Success {
		success = 1
	}

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO audit_events 
			(event_type, ip_address, user_agent, username, user_id, success, failure_reason, metadata, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, string(event.EventType), event.IPAddress, event.UserAgent, event.Username, event.UserID, success, event.FailureReason, metadataJSON, event.Timestamp)

	return err
}

// LogLoginSuccess logs a successful login
func (a *AuditLogger) LogLoginSuccess(ctx context.Context, ip, userAgent, username, userID string) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventLoginSuccess,
		IPAddress: ip,
		UserAgent: userAgent,
		Username:  username,
		UserID:   userID,
		Success:  true,
	})
}

// LogLoginFailure logs a failed login attempt
func (a *AuditLogger) LogLoginFailure(ctx context.Context, ip, userAgent, username, reason string) {
	a.Log(ctx, AuditEvent{
		EventType:     AuditEventLoginFailure,
		IPAddress:    ip,
		UserAgent:    userAgent,
		Username:     username,
		Success:      false,
		FailureReason: reason,
	})
}

// LogAccountLocked logs an account lockout
func (a *AuditLogger) LogAccountLocked(ctx context.Context, ip string, username string) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventAccountLocked,
		IPAddress: ip,
		Username: username,
		Success:  false,
		Metadata: map[string]interface{}{
			"reason": "brute_force_protection",
		},
	})
}

// LogAccountUnlocked logs an account unlock
func (a *AuditLogger) LogAccountUnlocked(ctx context.Context, ip string) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventAccountUnlocked,
		IPAddress: ip,
		Success:  true,
	})
}

// LogBruteForceAttempt logs a brute force attempt
func (a *AuditLogger) LogBruteForceAttempt(ctx context.Context, ip string, attempts int) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventBruteForceAttempt,
		IPAddress: ip,
		Success:   false,
		Metadata: map[string]interface{}{
			"attempts": attempts,
		},
	})
}

// LogLogout logs a logout event
func (a *AuditLogger) LogLogout(ctx context.Context, ip, userAgent, username, userID string) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventLogout,
		IPAddress: ip,
		UserAgent: userAgent,
		Username:  username,
		UserID:   userID,
		Success:  true,
	})
}

// LogTokenRefresh logs a token refresh
func (a *AuditLogger) LogTokenRefresh(ctx context.Context, ip, userAgent, username, userID string) {
	a.Log(ctx, AuditEvent{
		EventType: AuditEventTokenRefresh,
		IPAddress: ip,
		UserAgent: userAgent,
		Username:  username,
		UserID:   userID,
		Success:  true,
	})
}

// GetRecentEvents retrieves recent audit events
func (a *AuditLogger) GetRecentEvents(ctx context.Context, limit int) ([]AuditEvent, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT id, event_type, ip_address, user_agent, username, user_id, success, failure_reason, metadata, timestamp
		FROM audit_events
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		var success int
		var metadataJSON sql.NullString
		var userAgent sql.NullString
		var username sql.NullString
		var userID sql.NullString
		var failureReason sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.IPAddress,
			&userAgent,
			&username,
			&userID,
			&success,
			&failureReason,
			&metadataJSON,
			&event.Timestamp,
		)
		if err != nil {
			return nil, err
		}

		if userAgent.Valid {
			event.UserAgent = userAgent.String
		}
		if username.Valid {
			event.Username = username.String
		}
		if userID.Valid {
			event.UserID = userID.String
		}
		if failureReason.Valid {
			event.FailureReason = failureReason.String
		}
		event.Success = success == 1

		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &event.Metadata)
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// GetFailedAttemptsByIP retrieves failed login attempts for an IP
func (a *AuditLogger) GetFailedAttemptsByIP(ctx context.Context, ip string, since time.Time) (int, error) {
	if a.db == nil {
		return 0, fmt.Errorf("database not available")
	}

	var count int
	err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_events
		WHERE event_type = ? AND ip_address = ? AND success = 0 AND timestamp >= ?
	`, AuditEventLoginFailure, ip, since).Scan(&count)

	if err != nil {
		return 0, err
	}

	return count, nil
}
