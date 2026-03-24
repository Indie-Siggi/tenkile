// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tenkile/tenkile/internal/config"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.SecurityConfig
		expectedHeader string
		expectedValue  string
	}{
		{
			name: "default headers",
			cfg: &config.SecurityConfig{
				EnableHeaders:        true,
				CSP:                  "default-src 'self'",
				XFrameOptions:        "DENY",
				XContentTypeOptions:  "nosniff",
				XXSSProtection:       "1; mode=block",
				ReferrerPolicy:       "strict-origin-when-cross-origin",
				PermissionsPolicy:     "geolocation=()",
				HSTSEnabled:          false,
			},
			expectedHeader: "X-Frame-Options",
			expectedValue:  "DENY",
		},
		{
			name: "custom CSP",
			cfg: &config.SecurityConfig{
				EnableHeaders:        true,
				CSP:                  "script-src 'self'",
				XFrameOptions:        "SAMEORIGIN",
				XContentTypeOptions:  "nosniff",
				XXSSProtection:       "1; mode=block",
				ReferrerPolicy:       "no-referrer",
				PermissionsPolicy:    "",
				HSTSEnabled:          true,
				HSTSMaxAge:           31536000,
			},
			expectedHeader: "Content-Security-Policy",
			expectedValue:  "script-src 'self'",
		},
		{
			name: "HSTS enabled",
			cfg: &config.SecurityConfig{
				EnableHeaders:        true,
				CSP:                  "",
				XFrameOptions:        "DENY",
				XContentTypeOptions:  "nosniff",
				XXSSProtection:       "",
				ReferrerPolicy:       "",
				PermissionsPolicy:    "",
				HSTSEnabled:          true,
				HSTSMaxAge:           31536000,
			},
			expectedHeader: "Strict-Transport-Security",
			expectedValue:  "max-age=31536000; includeSubDomains",
		},
		{
			name: "HSTS enabled without includeSubDomains",
			cfg: &config.SecurityConfig{
				EnableHeaders:        true,
				CSP:                  "",
				XFrameOptions:        "DENY",
				XContentTypeOptions:  "nosniff",
				XXSSProtection:       "",
				ReferrerPolicy:       "",
				PermissionsPolicy:    "",
				HSTSEnabled:          true,
				HSTSMaxAge:           0, // Zero means no includeSubDomains
			},
			expectedHeader: "Strict-Transport-Security",
			expectedValue:  "max-age=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			wrapped := securityHeadersMiddleware(tt.cfg)(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			// Check the expected header is set
			if got := rec.Header().Get(tt.expectedHeader); got != tt.expectedValue {
				t.Errorf("expected %s=%s, got %s", tt.expectedHeader, tt.expectedValue, got)
			}
		})
	}
}

func TestSecurityHeadersMiddlewareDisabled(t *testing.T) {
	cfg := &config.SecurityConfig{
		EnableHeaders: false,
		CSP:           "default-src 'none'",
		XFrameOptions: "DENY",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := securityHeadersMiddleware(cfg)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// When disabled, headers should not be set
	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("expected empty CSP when disabled, got %s", got)
	}
}

func TestSecurityHeadersAllPresent(t *testing.T) {
	cfg := &config.SecurityConfig{
		EnableHeaders:        true,
		CSP:                  "default-src 'self'",
		XFrameOptions:        "DENY",
		XContentTypeOptions:   "nosniff",
		XXSSProtection:       "1; mode=block",
		ReferrerPolicy:       "strict-origin-when-cross-origin",
		PermissionsPolicy:    "geolocation=()",
		HSTSEnabled:          true,
		HSTSMaxAge:           86400,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := securityHeadersMiddleware(cfg)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Check all expected headers are present
	expectedHeaders := map[string]string{
		"Content-Security-Policy":    cfg.CSP,
		"X-Frame-Options":           cfg.XFrameOptions,
		"X-Content-Type-Options":     cfg.XContentTypeOptions,
		"X-XSS-Protection":           cfg.XXSSProtection,
		"Referrer-Policy":            cfg.ReferrerPolicy,
		"Permissions-Policy":         cfg.PermissionsPolicy,
		"Strict-Transport-Security":  "max-age=86400; includeSubDomains",
		"X-Permitted-Cross-Domain-Policies": "none",
	}

	for header, expected := range expectedHeaders {
		if got := rec.Header().Get(header); got != expected {
			t.Errorf("expected %s=%s, got %s", header, expected, got)
		}
	}
}

func TestSecurityHeadersAdditionalHeaders(t *testing.T) {
	cfg := &config.SecurityConfig{
		EnableHeaders: true,
		CSP:           "",
		XFrameOptions: "DENY",
		AdditionalHeaders: map[string]string{
			"X-Custom-Header":    "custom-value",
			"X-Another-Header":   "another-value",
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := securityHeadersMiddleware(cfg)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Custom-Header"); got != "custom-value" {
		t.Errorf("expected X-Custom-Header=custom-value, got %s", got)
	}

	if got := rec.Header().Get("X-Another-Header"); got != "another-value" {
		t.Errorf("expected X-Another-Header=another-value, got %s", got)
	}
}

func TestSecurityHeadersNilConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := securityHeadersMiddleware(nil)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// With nil config, default values should be used
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("expected X-Frame-Options=DENY, got %s", got)
	}

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected X-Content-Type-Options=nosniff, got %s", got)
	}
}
