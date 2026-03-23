package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

func TestAuthMiddleware_ValidToken(t *testing.T) {
	jwtSecret := []byte("test-secret-key")

	// Create a valid token
	claims := &Claims{
		UserID:   "user-123",
		Username: "testuser",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	// Create handler that checks auth
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r)
		if user == nil {
			t.Error("expected user in context")
			return
		}
		if user.ID != "user-123" {
			t.Errorf("expected user ID 'user-123', got %q", user.ID)
		}
		if user.Username != "testuser" {
			t.Errorf("expected username 'testuser', got %q", user.Username)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	middleware := AuthMiddleware(jwtSecret)
	wrapped := middleware(handler)

	// Create request with token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	jwtSecret := []byte("test-secret-key")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthMiddleware(jwtSecret)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	jwtSecret := []byte("test-secret-key")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthMiddleware(jwtSecret)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	jwtSecret := []byte("test-secret-key")

	// Create an expired token
	claims := &Claims{
		UserID:   "user-123",
		Username: "testuser",
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // Expired
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtSecret)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthMiddleware(jwtSecret)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for expired token, got %d", w.Code)
	}
}

func TestAuthMiddleware_WrongSecret(t *testing.T) {
	jwtSecret := []byte("test-secret-key")
	wrongSecret := []byte("wrong-secret-key")

	// Create token with wrong secret
	claims := &Claims{
		UserID:   "user-123",
		Username: "testuser",
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(wrongSecret)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthMiddleware(jwtSecret)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for wrong secret, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidAuthHeader(t *testing.T) {
	jwtSecret := []byte("test-secret-key")

	testCases := []struct {
		name   string
		header string
	}{
		{"no bearer", "token-only"},
		{"wrong scheme", "Basic token123"},
		{"empty bearer", "Bearer "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := AuthMiddleware(jwtSecret)
			wrapped := middleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", w.Code)
			}
		})
	}
}

func TestAdminMiddleware_Admin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AdminMiddleware()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)

	// Add admin user to context
	user := &User{ID: "admin-1", Username: "admin", Role: "admin"}
	ctx := WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for admin, got %d", w.Code)
	}
}

func TestAdminMiddleware_NonAdmin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AdminMiddleware()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)

	// Add non-admin user to context
	user := &User{ID: "user-1", Username: "user", Role: "user"}
	ctx := WithUser(req.Context(), user)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for non-admin, got %d", w.Code)
	}
}

func TestAdminMiddleware_NoUser(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AdminMiddleware()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	// Without user, returns 403 Forbidden (not 401 because user == nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 without user, got %d", w.Code)
	}
}

func TestOptionalAuthMiddleware_WithToken(t *testing.T) {
	jwtSecret := []byte("test-secret-key")

	// Create valid token
	claims := &Claims{
		UserID:   "user-123",
		Username: "testuser",
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtSecret)

	var capturedUser *User
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = GetUserFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	middleware := OptionalAuthMiddleware(jwtSecret)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if capturedUser == nil {
		t.Error("expected user in context")
	}
}

func TestOptionalAuthMiddleware_WithoutToken(t *testing.T) {
	jwtSecret := []byte("test-secret-key")

	var capturedUser *User
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = GetUserFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	middleware := OptionalAuthMiddleware(jwtSecret)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 without token, got %d", w.Code)
	}
	if capturedUser != nil {
		t.Error("expected no user in context without token")
	}
}

func TestOptionalAuthMiddleware_InvalidToken(t *testing.T) {
	jwtSecret := []byte("test-secret-key")

	var capturedUser *User
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = GetUserFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	middleware := OptionalAuthMiddleware(jwtSecret)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	// Should still pass through with no user
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 with invalid token, got %d", w.Code)
	}
	if capturedUser != nil {
		t.Error("expected no user in context with invalid token")
	}
}

// Test Claims struct
func TestClaims_Fields(t *testing.T) {
	claims := &Claims{
		UserID:   "user-123",
		Username: "testuser",
		Role:     "admin",
	}

	if claims.UserID != "user-123" {
		t.Errorf("expected UserID 'user-123', got %q", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("expected Username 'testuser', got %q", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("expected Role 'admin', got %q", claims.Role)
	}
}

// Test generateID function
func TestGenerateID(t *testing.T) {
	id1 := generateID("test-input")
	id2 := generateID("test-input")

	if id1 == "" {
		t.Error("expected non-empty ID")
	}

	// Same input should produce same output
	if id1 != id2 {
		t.Errorf("expected same ID for same input, got %q and %q", id1, id2)
	}

	id3 := generateID("different-input")
	if id1 == id3 {
		t.Error("expected different IDs for different inputs")
	}
}
