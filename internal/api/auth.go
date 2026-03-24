// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tenkile/tenkile/internal/api/auth"
	"golang.org/x/crypto/bcrypt"
)

// Claims represents JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// AuthHandler handles authentication requests
type AuthHandler struct {
	db                  *sql.DB
	jwtSecret           []byte
	jwtExpiry           time.Duration
	bruteForce          *auth.BruteForceProtector
	audit               *auth.AuditLogger
	passwordPolicy      auth.PasswordPolicy
	logger              *slog.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *sql.DB, jwtSecret string, logger *slog.Logger) *AuthHandler {
	h := &AuthHandler{
		db:             db,
		jwtSecret:      []byte(jwtSecret),
		jwtExpiry:      24 * time.Hour,
		passwordPolicy: auth.DefaultPasswordPolicy(),
		logger:         logger,
	}

	// Initialize brute force protection
	h.bruteForce = auth.NewBruteForceProtector(auth.DefaultBruteForceConfig())
	h.bruteForce.OnLockout(func(ip string, until time.Time) {
		h.logger.Warn("IP locked out due to brute force",
			"ip", ip,
			"until", until.Format(time.RFC3339),
		)
	})

	// Initialize audit logger
	h.audit = auth.NewAuditLogger(db, logger)

	return h
}

// Login handles POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	clientIP := auth.GetClientIP(r)
	userAgent := r.UserAgent()

	// SECURITY: Check if IP is locked out
	if h.bruteForce.IsLocked(clientIP) {
		remaining := h.bruteForce.GetLockoutRemaining(clientIP)
		h.audit.LogLoginFailure(r.Context(), clientIP, userAgent, "", "account_locked")
		
		WriteError(w, http.StatusTooManyRequests, fmt.Sprintf(
			"Too many failed login attempts. Please try again in %v.",
			remaining.Round(time.Second),
		))
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "Username and password required")
		return
	}

	// Normalize username
	req.Username = strings.TrimSpace(req.Username)

	user, err := h.getUserByUsername(r.Context(), req.Username)
	if err != nil {
		h.logger.Error("Database error during login", "error", err)
		WriteError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if user == nil {
		// Record failure but don't reveal if user exists
		locked, delay := h.bruteForce.RecordFailure(clientIP)
		h.audit.LogLoginFailure(r.Context(), clientIP, userAgent, req.Username, "user_not_found")
		
		if locked {
			h.audit.LogBruteForceAttempt(r.Context(), clientIP, h.bruteForce.GetAttempts(clientIP))
		}
		
		// Apply delay before returning error
		if delay > 0 {
			time.Sleep(delay)
		}
		
		WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		locked, delay := h.bruteForce.RecordFailure(clientIP)
		h.audit.LogLoginFailure(r.Context(), clientIP, userAgent, req.Username, "invalid_password")
		
		if locked {
			h.audit.LogAccountLocked(r.Context(), clientIP, req.Username)
			h.audit.LogBruteForceAttempt(r.Context(), clientIP, h.bruteForce.GetAttempts(clientIP))
			h.logger.Warn("IP locked out", "ip", clientIP, "username", req.Username)
		}
		
		// Apply delay before returning error
		if delay > 0 {
			time.Sleep(delay)
		}
		
		WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Login successful - clear failure record
	h.bruteForce.RecordSuccess(clientIP)
	h.audit.LogLoginSuccess(r.Context(), clientIP, userAgent, req.Username, user.ID)

	// Generate tokens
	accessToken, err := h.generateToken(user, h.jwtExpiry)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	refreshToken, err := h.generateToken(user, 7*24*time.Hour)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to generate refresh token")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(h.jwtExpiry.Seconds()),
		"user": map[string]string{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

// Refresh handles POST /auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	claims, err := h.validateToken(req.RefreshToken)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	user, err := h.getUserByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		WriteError(w, http.StatusUnauthorized, "User not found")
		return
	}

	// Log token refresh
	h.audit.LogTokenRefresh(r.Context(), auth.GetClientIP(r), r.UserAgent(), user.Username, user.ID)

	accessToken, err := h.generateToken(user, h.jwtExpiry)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": accessToken,
	})
}

// Logout handles POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user != nil {
		h.audit.LogLogout(r.Context(), auth.GetClientIP(r), r.UserAgent(), user.Username, user.ID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// FirstRun handles POST /auth/first-run
func (h *AuthHandler) FirstRun(w http.ResponseWriter, r *http.Request) {
	var count int
	err := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if count > 0 {
		WriteError(w, http.StatusBadRequest, "System already configured")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "Username and password required")
		return
	}

	// Validate password against policy
	valid, issues := h.passwordPolicy.Validate(req.Password)
	if !valid {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Password validation failed: %s", strings.Join(issues, "; ")))
		return
	}

	// Check password strength
	strength := auth.PasswordStrength(req.Password)
	if strength < 40 {
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Password is too weak (strength: %s). Please choose a stronger password.", auth.StrengthDescription(strength)))
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	userID := generateID(req.Username + time.Now().String())
	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
		VALUES (?, ?, ?, 'admin', ?, ?)
	`, userID, req.Username, string(hash), time.Now(), time.Now())

	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	h.audit.Log(r.Context(), auth.AuditEvent{
		EventType: auth.AuditEventFirstRun,
		IPAddress: auth.GetClientIP(r),
		UserAgent: r.UserAgent(),
		Username:  req.Username,
		UserID:    userID,
		Success:   true,
	})

	user := &User{ID: userID, Username: req.Username, Role: "admin"}
	accessToken, _ := h.generateToken(user, h.jwtExpiry)
	refreshToken, _ := h.generateToken(user, 7*24*time.Hour)

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": map[string]string{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

// ValidatePassword handles POST /auth/validate-password
func (h *AuthHandler) ValidatePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	valid, issues := h.passwordPolicy.Validate(req.Password)
	strength := auth.PasswordStrength(req.Password)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"valid":         valid,
		"issues":        issues,
		"strength":       strength,
		"strength_label": auth.StrengthDescription(strength),
	})
}

// GetCurrentUser handles GET /auth/me
func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

// GetBruteForceStatus handles GET /auth/brute-force-status
func (h *AuthHandler) GetBruteForceStatus(w http.ResponseWriter, r *http.Request) {
	clientIP := auth.GetClientIP(r)
	status := h.bruteForce.GetStatus(clientIP)

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ip":            status.IP,
		"attempts":      status.Attempts,
		"attempts_left": status.AttemptsLeft,
		"locked":        status.Locked,
		"locked_until":  status.LockedUntil,
	})
}

// SetPasswordPolicy updates the password policy
func (h *AuthHandler) SetPasswordPolicy(policy auth.PasswordPolicy) {
	h.passwordPolicy = policy
}

func (h *AuthHandler) getUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := h.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role FROM users WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *AuthHandler) getUserByID(ctx context.Context, id string) (*User, error) {
	var user User
	err := h.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *AuthHandler) generateToken(user *User, expiry time.Duration) (string, error) {
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}

func (h *AuthHandler) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return h.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

// generateID creates a unique ID from a String
func generateID(input string) string {
	h := uint64(0)
	for i, c := range input {
		h = h*31 + uint64(c) + uint64(i)
	}
	return fmt.Sprintf("%016x", h)
}
