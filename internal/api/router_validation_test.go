// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodySizeLimitMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		maxBytes       int64
		contentLength  int64
		body           string
		wantStatus     int
	}{
		{
			name:          "within limit",
			maxBytes:      1024,
			contentLength: 100,
			body:          strings.Repeat("x", 100),
			wantStatus:    http.StatusOK,
		},
		{
			name:          "exactly at limit",
			maxBytes:      100,
			contentLength: 100,
			body:          strings.Repeat("x", 100),
			wantStatus:    http.StatusOK,
		},
		{
			name:          "over limit",
			maxBytes:      100,
			contentLength: 101,
			body:          strings.Repeat("x", 101),
			wantStatus:    http.StatusRequestEntityTooLarge,
		},
		{
			name:          "well over limit",
			maxBytes:      100,
			contentLength: 10000,
			body:          strings.Repeat("x", 10000),
			wantStatus:    http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler that always returns OK
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with middleware
			wrapped := bodySizeLimitMiddleware(tt.maxBytes)(handler)

			// Create request
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.body))
			req.ContentLength = tt.contentLength
			rec := httptest.NewRecorder()

			// Execute
			wrapped.ServeHTTP(rec, req)

			// Check status
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestMaxConstants(t *testing.T) {
	// Verify constants are defined correctly
	if MaxRequestBodySize != 1<<20 {
		t.Errorf("MaxRequestBodySize = %d, want %d", MaxRequestBodySize, 1<<20)
	}
	if MaxProbeReportSize != 1<<20 {
		t.Errorf("MaxProbeReportSize = %d, want %d", MaxProbeReportSize, 1<<20)
	}
	if MaxFeedbackSize != 100<<10 {
		t.Errorf("MaxFeedbackSize = %d, want %d", MaxFeedbackSize, 100<<10)
	}
}
