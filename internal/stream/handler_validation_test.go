// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package stream

import (
	"path/filepath"
	"testing"
)

func TestIsValidHLSName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOk  bool
	}{
		{
			name:   "valid HLS name",
			input:  "hls_abc123def",
			wantOk: true,
		},
		{
			name:   "valid HLS name with numbers",
			input:  "hls_1234567890abcdef",
			wantOk: true,
		},
		{
			name:   "missing prefix",
			input:  "abc_def",
			wantOk: false,
		},
		{
			name:   "path separator forward",
			input:  "hls_../etc",
			wantOk: false,
		},
		{
			name:   "path separator backslash",
			input:  "hls_..\\etc",
			wantOk: false,
		},
		{
			name:   "parent directory ref",
			input:  "hls_..",
			wantOk: false,
		},
		{
			name:   "null byte",
			input:  "hls_test\x00",
			wantOk: false,
		},
		{
			name:   "too short",
			input:  "hls_",
			wantOk: false,
		},
		{
			name:   "too long",
			input:  "hls_" + string(make([]byte, 60)),
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidHLSName(tt.input)
			if got != tt.wantOk {
				t.Errorf("isValidHLSName(%q) = %v, want %v", tt.input, got, tt.wantOk)
			}
		})
	}
}

func TestIsPathInDirectory(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		dir        string
		wantInDir  bool
	}{
		{
			name:      "path inside directory",
			path:      "/tmp/hls_abc123",
			dir:       "/tmp",
			wantInDir: true,
		},
		{
			name:      "path outside directory",
			path:      "/var/hls_abc123",
			dir:       "/tmp",
			wantInDir: false,
		},
		{
			name:      "path traversal attempt",
			path:      "/tmp/../etc/hls_abc",
			dir:       "/tmp",
			wantInDir: false,
		},
		{
			name:      "path is the directory itself",
			path:      "/tmp",
			dir:       "/tmp",
			wantInDir: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathInDirectory(tt.path, tt.dir)
			if got != tt.wantInDir {
				t.Errorf("isPathInDirectory(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.wantInDir)
			}
		})
	}
}

func TestIsPathInDirectoryRealPaths(t *testing.T) {
	// Test with actual filesystem paths
	tmpDir := t.TempDir()
	
	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "hls_test")
	
	got := isPathInDirectory(subDir, tmpDir)
	if !got {
		t.Errorf("expected %q to be in %q", subDir, tmpDir)
	}
	
	// Create a file in the subdirectory
	filePath := filepath.Join(subDir, "segment.ts")
	got = isPathInDirectory(filePath, tmpDir)
	if !got {
		t.Errorf("expected %q to be in %q", filePath, tmpDir)
	}
}
