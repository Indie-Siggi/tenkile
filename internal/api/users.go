// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tenkile/tenkile/internal/api/auth"
	"golang.org/x/crypto/bcrypt"
)

// UserRole constants
const (
	RoleAdmin  = "admin"
	RoleUser   = "user"
	RoleViewer = "viewer"
)

// UserStatus constants
const (
	UserStatusActive   = "active"
	UserStatusLocked   = "locked"
	UserStatusDisabled = "disabled"
)

// UserProfile represents a user's profile (without sensitive data)
type UserProfile struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastLogin    time.Time `json:"last_login,omitempty"`
	PasswordHash string    `json:"-"` // Never expose password hash
}

// UserDetails represents detailed user information (admin only)
type UserDetails struct {
	UserProfile
	LoginAttempts    int       `json:"login_attempts,omitempty"`
	LockedUntil      time.Time `json:"locked_until,omitempty"`
	PasswordChangedAt time.Time `json:"password_changed_at,omitempty"`
}

// UserHandler handles user management requests
type UserHandler struct {
	db             *sql.DB
	authHandler    *AuthHandler
	passwordPolicy auth.PasswordPolicy
}

// NewUserHandler creates a new user handler
func NewUserHandler(db *sql.DB, authHandler *AuthHandler) *UserHandler {
	return &UserHandler{
		db:             db,
		authHandler:    authHandler,
		passwordPolicy: auth.DefaultPasswordPolicy(),
	}
}

// ListUsers handles GET /users (admin only)
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, username, email, role, status, created_at, updated_at, last_login
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to fetch users")
		return
	}
	defer rows.Close()

	var users []UserProfile
	for rows.Next() {
		var user UserProfile
		var email sql.NullString
		var lastLogin sql.NullTime
		if err := rows.Scan(&user.ID, &user.Username, &email, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt, &lastLogin); err != nil {
			continue
		}
		if email.Valid {
			user.Email = email.String
		}
		if lastLogin.Valid {
			user.LastLogin = lastLogin.Time
		}
		users = append(users, user)
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"users": users,
		"count": len(users),
	})
}

// CreateUser handles POST /users (admin only)
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		WriteError(w, http.StatusBadRequest, "Username is required")
		return
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		WriteError(w, http.StatusBadRequest, "Username must be 3-32 characters")
		return
	}

	// Validate role
	if req.Role == "" {
		req.Role = RoleUser
	}
	if req.Role != RoleAdmin && req.Role != RoleUser && req.Role != RoleViewer {
		WriteError(w, http.StatusBadRequest, "Invalid role")
		return
	}

	// Validate password
	valid, issues := h.passwordPolicy.Validate(req.Password)
	if !valid {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Password validation failed: %s", strings.Join(issues, "; ")))
		return
	}

	strength := auth.PasswordStrength(req.Password)
	if strength < 40 {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Password is too weak (strength: %s)", auth.StrengthDescription(strength)))
		return
	}

	// Check if username exists
	var exists int
	if err := h.db.QueryRowContext(r.Context(), "SELECT 1 FROM users WHERE username = ?", req.Username).Scan(&exists); err == nil {
		WriteError(w, http.StatusConflict, "Username already exists")
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Create user
	userID := generateID(req.Username + time.Now().String())
	now := time.Now()
	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, req.Username, nullString(req.Email), string(hash), req.Role, UserStatusActive, now, now)

	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Audit log
	h.authHandler.audit.Log(r.Context(), auth.AuditEvent{
		EventType: auth.AuditEventType("user_created"),
		IPAddress: auth.GetClientIP(r),
		UserAgent: r.UserAgent(),
		Username:  req.Username,
		UserID:   userID,
		Success:   true,
	})

	WriteJSON(w, http.StatusCreated, UserProfile{
		ID:        userID,
		Username:  req.Username,
		Email:     req.Email,
		Role:      req.Role,
		Status:    UserStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// GetUser handles GET /users/{id}
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		WriteError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	user, err := h.getUserByID(r.Context(), userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to fetch user")
		return
	}
	if user == nil {
		WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	// Non-admin users can only view their own profile
	currentUser := GetUserFromContext(r)
	if currentUser != nil && currentUser.Role != RoleAdmin && currentUser.ID != user.ID {
		WriteError(w, http.StatusForbidden, "Cannot view other users' profiles")
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

// UpdateUser handles PATCH /users/{id}
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		WriteError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	// Users can only update their own profile (except admins)
	currentUser := GetUserFromContext(r)
	if currentUser == nil {
		WriteError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	if currentUser.Role != RoleAdmin && currentUser.ID != userID {
		WriteError(w, http.StatusForbidden, "Cannot update other users' profiles")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"` // Admin only
	}
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Build update query
	updates := []string{}
	args := []interface{}{}

	if req.Email != "" {
		updates = append(updates, "email = ?")
		args = append(args, req.Email)
	}

	// Role can only be changed by admins
	if req.Role != "" && currentUser.Role == RoleAdmin {
		if req.Role != RoleAdmin && req.Role != RoleUser && req.Role != RoleViewer {
			WriteError(w, http.StatusBadRequest, "Invalid role")
			return
		}
		updates = append(updates, "role = ?")
		args = append(args, req.Role)
	}

	if len(updates) == 0 {
		WriteError(w, http.StatusBadRequest, "No fields to update")
		return
	}

	updates = append(updates, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, userID)

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = ?", strings.Join(updates, ", "))
	_, err := h.db.ExecContext(r.Context(), query, args...)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}

	// Audit log
	h.authHandler.audit.Log(r.Context(), auth.AuditEvent{
		EventType: auth.AuditEventType("user_updated"),
		IPAddress: auth.GetClientIP(r),
		UserAgent: r.UserAgent(),
		UserID:   userID,
		Success:   true,
	})

	// Return updated user
	user, _ := h.getUserByID(r.Context(), userID)
	WriteJSON(w, http.StatusOK, user)
}

// DeleteUser handles DELETE /users/{id} (admin only)
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		WriteError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	// Cannot delete yourself
	currentUser := GetUserFromContext(r)
	if currentUser != nil && currentUser.ID == userID {
		WriteError(w, http.StatusBadRequest, "Cannot delete your own account")
		return
	}

	// Check if user exists
	var username string
	if err := h.db.QueryRowContext(r.Context(), "SELECT username FROM users WHERE id = ?", userID).Scan(&username); err == sql.ErrNoRows {
		WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	// Delete user
	_, err := h.db.ExecContext(r.Context(), "DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	// Audit log
	h.authHandler.audit.Log(r.Context(), auth.AuditEvent{
		EventType: auth.AuditEventType("user_deleted"),
		IPAddress: auth.GetClientIP(r),
		UserAgent: r.UserAgent(),
		Username:  username,
		UserID:   userID,
		Success:   true,
	})

	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword handles POST /users/{id}/change-password
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		WriteError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	currentUser := GetUserFromContext(r)
	if currentUser == nil {
		WriteError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Users can only change their own password (admins can use admin reset)
	if currentUser.Role != RoleAdmin && currentUser.ID != userID {
		WriteError(w, http.StatusForbidden, "Cannot change other users' passwords")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword    string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get user for password verification
	user, err := h.getUserByID(r.Context(), userID)
	if err != nil || user == nil {
		WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	// Verify current password (admins can skip for other users)
	if currentUser.ID == userID || currentUser.Role != RoleAdmin {
		if req.CurrentPassword == "" {
			WriteError(w, http.StatusBadRequest, "Current password is required")
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
			WriteError(w, http.StatusUnauthorized, "Current password is incorrect")
			return
		}
	}

	// Validate new password
	valid, issues := h.passwordPolicy.Validate(req.NewPassword)
	if !valid {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Password validation failed: %s", strings.Join(issues, "; ")))
		return
	}

	strength := auth.PasswordStrength(req.NewPassword)
	if strength < 40 {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Password is too weak (strength: %s)", auth.StrengthDescription(strength)))
		return
	}

	// Hash and update password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	now := time.Now()
	_, err = h.db.ExecContext(r.Context(), `
		UPDATE users SET password_hash = ?, updated_at = ?, password_changed_at = ? WHERE id = ?
	`, string(hash), now, now, userID)

	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	// Audit log
	h.authHandler.audit.Log(r.Context(), auth.AuditEvent{
		EventType: auth.AuditEventPasswordChange,
		IPAddress: auth.GetClientIP(r),
		UserAgent: r.UserAgent(),
		UserID:   userID,
		Success:   true,
	})

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Password updated successfully"})
}

// LockUser handles POST /users/{id}/lock (admin only)
func (h *UserHandler) LockUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		WriteError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	// Cannot lock yourself
	currentUser := GetUserFromContext(r)
	if currentUser != nil && currentUser.ID == userID {
		WriteError(w, http.StatusBadRequest, "Cannot lock your own account")
		return
	}

	lockDuration := 24 * time.Hour // Default lock duration
	lockedUntil := time.Now().Add(lockDuration)

	result, err := h.db.ExecContext(r.Context(), `
		UPDATE users SET status = ?, locked_until = ?, updated_at = ? WHERE id = ? AND status != ?
	`, UserStatusLocked, lockedUntil, time.Now(), userID, UserStatusLocked)

	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to lock user")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		WriteError(w, http.StatusNotFound, "User not found or already locked")
		return
	}

	// Audit log
	h.authHandler.audit.Log(r.Context(), auth.AuditEvent{
		EventType: auth.AuditEventAccountLocked,
		IPAddress: auth.GetClientIP(r),
		UserAgent: r.UserAgent(),
		UserID:   userID,
		Success:   true,
		Metadata: map[string]interface{}{
			"reason": "admin_action",
		},
	})

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "User locked successfully",
		"locked_until": lockedUntil,
	})
}

// UnlockUser handles POST /users/{id}/unlock (admin only)
func (h *UserHandler) UnlockUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		WriteError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	result, err := h.db.ExecContext(r.Context(), `
		UPDATE users SET status = ?, locked_until = NULL, updated_at = ?, login_attempts = 0 WHERE id = ? AND status = ?
	`, UserStatusActive, time.Now(), userID, UserStatusLocked)

	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to unlock user")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		WriteError(w, http.StatusNotFound, "User not found or not locked")
		return
	}

	// Audit log
	h.authHandler.audit.Log(r.Context(), auth.AuditEvent{
		EventType: auth.AuditEventAccountUnlocked,
		IPAddress: auth.GetClientIP(r),
		UserAgent: r.UserAgent(),
		UserID:   userID,
		Success:   true,
	})

	WriteJSON(w, http.StatusOK, map[string]string{"message": "User unlocked successfully"})
}

// GetCurrentUserProfile handles GET /users/me
func (h *UserHandler) GetCurrentUserProfile(w http.ResponseWriter, r *http.Request) {
	currentUser := GetUserFromContext(r)
	if currentUser == nil {
		WriteError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	user, err := h.getUserByID(r.Context(), currentUser.ID)
	if err != nil || user == nil {
		WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

// getUserByID retrieves a user by ID
func (h *UserHandler) getUserByID(ctx context.Context, id string) (*UserProfile, error) {
	var user UserProfile
	var email sql.NullString
	var lastLogin, passwordChanged sql.NullTime
	var loginAttempts sql.NullInt64

	err := h.db.QueryRowContext(ctx, `
		SELECT id, username, email, role, status, created_at, updated_at, last_login, login_attempts, password_changed_at
		FROM users WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Username, &email, &user.Role, &user.Status,
		&user.CreatedAt, &user.UpdatedAt, &lastLogin, &loginAttempts, &passwordChanged,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if lastLogin.Valid {
		user.LastLogin = lastLogin.Time
	}

	return &user, nil
}

// nullString returns sql.NullString for optional string fields
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
