// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
	db        *sql.DB
	jwtSecret []byte
	jwtExpiry time.Duration
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *sql.DB, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		db:        db,
		jwtSecret: []byte(jwtSecret),
		jwtExpiry: 24 * time.Hour,
	}
}

// Login handles POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
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

	user, err := h.getUserByUsername(r.Context(), req.Username)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		WriteError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

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

	if len(req.Password) < 8 {
		WriteError(w, http.StatusBadRequest, "Password must be at least 8 characters")
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

// generateID creates a unique ID from a string
func generateID(input string) string {
	h := uint64(0)
	for i, c := range input {
		h = h*31 + uint64(c) + uint64(i)
	}
	return fmt.Sprintf("%016x", h)
}
