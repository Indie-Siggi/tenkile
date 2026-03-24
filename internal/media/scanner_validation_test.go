// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package media

import (
	"testing"
)

func TestIsPathComponentSkipped(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantSkip bool
	}{
		{
			name:     "sample directory",
			path:     "/media/sample/video.mkv",
			wantSkip: true,
		},
		{
			name:     "samples directory",
			path:     "/media/samples/video.mkv",
			wantSkip: true,
		},
		{
			name:     "extras directory",
			path:     "/media/extras/video.mkv",
			wantSkip: true,
		},
		{
			name:     "bonus directory",
			path:     "/media/bonus/video.mkv",
			wantSkip: true,
		},
		{
			name:     "trailers directory",
			path:     "/media/trailers/video.mkv",
			wantSkip: true,
		},
		{
			name:     "AppleDouble file",
			path:     "/media/.AppleDouble/video.mkv",
			wantSkip: true,
		},
		{
			name:     "DS_Store file",
			path:     "/media/.DS_Store",
			wantSkip: true,
		},
		{
			name:     "normal media path",
			path:     "/media/movies/video.mkv",
			wantSkip: false,
		},
		{
			name:     "sampler should not skip",
			path:     "/media/sampler/video.mkv",
			wantSkip: false, // "sampler" is not the same as "sample"
		},
		{
			name:     "case insensitive sample",
			path:     "/media/SAMPLE/video.mkv",
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathComponentSkipped(tt.path)
			if got != tt.wantSkip {
				t.Errorf("isPathComponentSkipped(%q) = %v, want %v", tt.path, got, tt.wantSkip)
			}
		})
	}
}

func TestValidatePathForScanning(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantOk  bool
	}{
		{
			name:   "empty path",
			path:   "",
			wantOk: false,
		},
		{
			name:   "path traversal attempt",
			path:   "/media/../etc/passwd",
			wantOk: false,
		},
		{
			name:   "null byte injection",
			path:   "/media/video\x00.mkv",
			wantOk: false,
		},
		{
			name:   "slash path traversal",
			path:   "/media/../../etc",
			wantOk: false,
		},
		{
			name:   "double dot path",
			path:   "/media/..",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validatePathForScanning(tt.path)
			if got != tt.wantOk {
				t.Errorf("validatePathForScanning(%q) = %v, want %v", tt.path, got, tt.wantOk)
			}
		})
	}
}
