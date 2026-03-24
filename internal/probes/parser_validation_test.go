// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"testing"
)

func TestValidateJSONSize(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		maxSize  int
		wantErr  bool
	}{
		{
			name:    "empty data",
			data:    []byte{},
			maxSize: 1024,
			wantErr: false,
		},
		{
			name:    "small data",
			data:    []byte(`{"test": "data"}`),
			maxSize: 1024,
			wantErr: false,
		},
		{
			name:    "exact limit",
			data:    make([]byte, 100),
			maxSize: 100,
			wantErr: false,
		},
		{
			name:    "over limit",
			data:    make([]byte, 101),
			maxSize: 100,
			wantErr: true,
		},
		{
			name:    "large data over limit",
			data:    make([]byte, MaxProbeSize+1),
			maxSize: MaxProbeSize,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONSize(tt.data, tt.maxSize, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJSONSize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateJSONSizeMaxProbeSize(t *testing.T) {
	// Test that MaxProbeSize is correctly defined
	if MaxProbeSize != 1<<20 {
		t.Errorf("MaxProbeSize = %d, want %d", MaxProbeSize, 1<<20)
	}

	// Test valid probe JSON
	validJSON := []byte(`{"device_id": "test", "platform": "android"}`)
	if err := ValidateJSONSize(validJSON, MaxProbeSize, "probe"); err != nil {
		t.Errorf("unexpected error for valid probe: %v", err)
	}

	// Test oversized probe JSON
	oversizedJSON := make([]byte, MaxProbeSize+1)
	if err := ValidateJSONSize(oversizedJSON, MaxProbeSize, "probe"); err == nil {
		t.Error("expected error for oversized probe JSON")
	}
}

func TestValidateJSONSizeErrorMessage(t *testing.T) {
	// Create data that exceeds limit
	data := make([]byte, 200)
	maxSize := 100
	
	err := ValidateJSONSize(data, maxSize, "test type")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	// Check error message contains useful info
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
}
