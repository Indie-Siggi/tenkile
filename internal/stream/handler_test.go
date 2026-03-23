package stream

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

// MockCommandRunner is a mock implementation of CommandRunner for testing
type MockCommandRunner struct {
	Output      []byte
	Error       error
	Calls       [][]string // record of calls made
}

func (m *MockCommandRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, args)
	return m.Output, m.Error
}

func TestNewHandler(t *testing.T) {
	handler := NewHandler(nil)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.segmenter == nil {
		t.Error("expected non-nil segmenter")
	}
	if handler.sessions == nil {
		t.Error("expected non-nil sessions map")
	}
	if handler.sessionTTL != 4*time.Hour {
		t.Errorf("expected sessionTTL 4h, got %v", handler.sessionTTL)
	}
}

func TestStartSession(t *testing.T) {
	handler := NewHandler(nil)

	session, err := handler.StartSession(context.Background(), "media-1", "user-1", "device-1", "1080p", StreamTypeHLS)
	if err != nil {
		t.Fatalf("StartSession() returned error: %v", err)
	}

	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if session.MediaItemID != "media-1" {
		t.Errorf("expected MediaItemID 'media-1', got %q", session.MediaItemID)
	}
	if session.UserID != "user-1" {
		t.Errorf("expected UserID 'user-1', got %q", session.UserID)
	}
	if session.DeviceID != "device-1" {
		t.Errorf("expected DeviceID 'device-1', got %q", session.DeviceID)
	}
	if session.Variant != "1080p" {
		t.Errorf("expected Variant '1080p', got %q", session.Variant)
	}
	if session.StreamType != StreamTypeHLS {
		t.Errorf("expected StreamType HLS, got %v", session.StreamType)
	}
}

func TestGetSession(t *testing.T) {
	handler := NewHandler(nil)

	// Get non-existent session
	_, ok := handler.GetSession("non-existent")
	if ok {
		t.Error("expected not found for non-existent session")
	}

	// Create and get session
	created, _ := handler.StartSession(context.Background(), "media-1", "user-1", "device-1", "", StreamTypeHLS)
	retrieved, ok := handler.GetSession(created.ID)
	if !ok {
		t.Error("expected to find created session")
	}
	if retrieved.ID != created.ID {
		t.Errorf("expected session ID %q, got %q", created.ID, retrieved.ID)
	}
}

func TestEndSession(t *testing.T) {
	handler := NewHandler(nil)

	// Create session
	session, _ := handler.StartSession(context.Background(), "media-1", "user-1", "device-1", "", StreamTypeHLS)
	sessionID := session.ID

	// Verify session exists
	_, ok := handler.GetSession(sessionID)
	if !ok {
		t.Fatal("expected session to exist before EndSession")
	}

	// End session
	handler.EndSession(sessionID)

	// Verify session is gone
	_, ok = handler.GetSession(sessionID)
	if ok {
		t.Error("expected session to be removed after EndSession")
	}
}

func TestMultipleSessions(t *testing.T) {
	handler := &Handler{
		segmenter:   NewSegmenter("", "", nil),
		sessionTTL:   4 * time.Hour,
		sessions:     make(map[string]*StreamSession),
		mu:           sync.RWMutex{},
		cleanupDone:  make(chan struct{}),
	}

	// Create first session
	session1 := &StreamSession{
		ID:          "session-1",
		MediaItemID: "media-1",
	}
	handler.mu.Lock()
	handler.sessions[session1.ID] = session1
	handler.mu.Unlock()

	// Small delay
	time.Sleep(time.Millisecond)

	// Create second session
	session2 := &StreamSession{
		ID:          "session-2",
		MediaItemID: "media-2",
	}
	handler.mu.Lock()
	handler.sessions[session2.ID] = session2
	handler.mu.Unlock()

	// Small delay
	time.Sleep(time.Millisecond)

	// Create third session
	session3 := &StreamSession{
		ID:          "session-3",
		MediaItemID: "media-3",
	}
	handler.mu.Lock()
	handler.sessions[session3.ID] = session3
	handler.mu.Unlock()

	// Verify all sessions exist
	handler.mu.RLock()
	defer handler.mu.RUnlock()
	if handler.sessions[session1.ID] == nil {
		t.Error("expected session1 to be in map")
	}
	if handler.sessions[session2.ID] == nil {
		t.Error("expected session2 to be in map")
	}
	if handler.sessions[session3.ID] == nil {
		t.Error("expected session3 to be in map")
	}
	count := len(handler.sessions)
	if count != 3 {
		t.Errorf("expected 3 sessions, got %d", count)
	}

	// Remove session2
	delete(handler.sessions, session2.ID)

	// Verify session2 is gone but session1 and session3 remain
	if handler.sessions[session2.ID] != nil {
		t.Error("expected session2 to be removed")
	}
	if handler.sessions[session1.ID] == nil {
		t.Error("expected session1 to still exist")
	}
	if handler.sessions[session3.ID] == nil {
		t.Error("expected session3 to still exist")
	}
}

func TestUpdateBytesServed(t *testing.T) {
	handler := NewHandler(nil)

	// Create session
	session, _ := handler.StartSession(context.Background(), "media-1", "user-1", "device-1", "", StreamTypeHLS)

	// Update bytes
	handler.UpdateBytesServed(session.ID, 1024)
	handler.UpdateBytesServed(session.ID, 2048)

	// Verify bytes were updated
	updated, _ := handler.GetSession(session.ID)
	if updated.BytesServed != 3072 {
		t.Errorf("expected BytesServed 3072, got %d", updated.BytesServed)
	}
}

func TestUpdateBytesServedNonExistent(t *testing.T) {
	handler := NewHandler(nil)

	// Should not panic when updating non-existent session
	handler.UpdateBytesServed("non-existent", 1000)
}

func TestEndSessionWithCleanup(t *testing.T) {
	// Create a temp directory for manifest
	tempDir, err := os.MkdirTemp("", "hls-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	handler := NewHandler(nil)

	// Create session with manifest path
	session, _ := handler.StartSession(context.Background(), "media-1", "user-1", "device-1", "", StreamTypeHLS)
	manifestPath := filepath.Join(tempDir, "master.m3u8")
	
	// Create the manifest file
	if err := os.WriteFile(manifestPath, []byte("#EXTM3U\n"), 0644); err != nil {
		t.Fatalf("failed to create manifest file: %v", err)
	}

	// Set manifest path via reflection or add a method
	handler.mu.Lock()
	handler.sessions[session.ID].ManifestPath = manifestPath
	handler.mu.Unlock()

	// End session
	handler.EndSession(session.ID)

	// Verify file was cleaned up
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Error("expected manifest file to be cleaned up after EndSession")
	}
}

func TestSetSessionManifest(t *testing.T) {
	handler := NewHandler(nil)

	// Create session
	session, _ := handler.StartSession(context.Background(), "media-1", "user-1", "device-1", "", StreamTypeHLS)

	// Set manifest path
	manifestPath := "/tmp/hls/master.m3u8"
	handler.SetSessionManifest(session.ID, manifestPath)

	// Verify
	updated, _ := handler.GetSession(session.ID)
	if updated.ManifestPath != manifestPath {
		t.Errorf("expected ManifestPath %q, got %q", manifestPath, updated.ManifestPath)
	}
}

func TestSetSessionManifestNonExistent(t *testing.T) {
	handler := NewHandler(nil)

	// Should not panic
	handler.SetSessionManifest("non-existent", "/tmp/path")
}

func TestHandlerClose(t *testing.T) {
	handler := NewHandler(nil)

	// Close should not panic
	handler.Close()
}

func TestIsPathAllowed(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"empty path", "", false},
		{"path traversal attempt", "/tmp/../../../etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.isPathAllowed(tt.path)
			if result != tt.expected {
				t.Errorf("isPathAllowed(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}
