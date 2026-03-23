package events

import (
	"testing"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected string
	}{
		{EventLibraryScanStarted, "library.scan.started"},
		{EventLibraryScanProgress, "library.scan.progress"},
		{EventLibraryScanComplete, "library.scan.complete"},
		{EventLibraryScanError, "library.scan.error"},
		{EventStreamStarted, "stream.started"},
		{EventStreamEnded, "stream.ended"},
		{EventStreamError, "stream.error"},
		{EventTranscodeStarted, "transcode.started"},
		{EventTranscodeProgress, "transcode.progress"},
		{EventTranscodeComplete, "transcode.complete"},
		{EventTranscodeError, "transcode.error"},
		{EventDeviceConnected, "device.connected"},
		{EventDeviceDisconnected, "device.disconnected"},
	}

	for _, tt := range tests {
		if string(tt.eventType) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, string(tt.eventType))
		}
	}
}

func TestTopicConstants(t *testing.T) {
	if TopicLibraries != "libraries" {
		t.Errorf("expected TopicLibraries 'libraries', got %q", TopicLibraries)
	}
	if TopicStreams != "streams" {
		t.Errorf("expected TopicStreams 'streams', got %q", TopicStreams)
	}
	if TopicTranscodes != "transcodes" {
		t.Errorf("expected TopicTranscodes 'transcodes', got %q", TopicTranscodes)
	}
	if TopicDevices != "devices" {
		t.Errorf("expected TopicDevices 'devices', got %q", TopicDevices)
	}
	if TopicAll != "all" {
		t.Errorf("expected TopicAll 'all', got %q", TopicAll)
	}
}

func TestNewEvent(t *testing.T) {
	event := NewEvent(EventStreamStarted, TopicStreams, StreamPayload{
		StreamID:    "stream-123",
		MediaItemID: "media-456",
	})

	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.Type != EventStreamStarted {
		t.Errorf("expected type %q, got %q", EventStreamStarted, event.Type)
	}
	if event.Topic != TopicStreams {
		t.Errorf("expected topic %q, got %q", TopicStreams, event.Topic)
	}
	if event.ID == "" {
		t.Error("expected non-empty ID")
	}
	if event.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestNewEventWithPayload(t *testing.T) {
	payload := LibraryScanPayload{
		LibraryID:   "lib-123",
		LibraryName: "Test Library",
		TotalFiles:  100,
		Processed:   50,
	}

	event := NewEvent(EventLibraryScanProgress, TopicLibraries, payload)

	if event.Payload == nil {
		t.Fatal("expected non-nil payload")
	}

	// Type assert to check payload
	scanPayload, ok := event.Payload.(LibraryScanPayload)
	if !ok {
		t.Fatal("expected payload to be LibraryScanPayload")
	}
	if scanPayload.LibraryID != "lib-123" {
		t.Errorf("expected LibraryID 'lib-123', got %q", scanPayload.LibraryID)
	}
	if scanPayload.TotalFiles != 100 {
		t.Errorf("expected TotalFiles 100, got %d", scanPayload.TotalFiles)
	}
}

func TestEventPayload(t *testing.T) {
	payload := EventPayload{
		Data:    map[string]string{"key": "value"},
		Message: "test message",
		Error:   "",
	}

	if payload.Message != "test message" {
		t.Errorf("expected Message 'test message', got %q", payload.Message)
	}
	if payload.Data == nil {
		t.Error("expected non-nil Data")
	}
}

func TestLibraryScanPayload(t *testing.T) {
	payload := LibraryScanPayload{
		LibraryID:   "lib-abc",
		LibraryName: "Movies",
		TotalFiles:  250,
		Processed:   100,
		CurrentFile: "/movies/film.mkv",
		Status:      "scanning",
	}

	if payload.LibraryID != "lib-abc" {
		t.Errorf("expected LibraryID 'lib-abc', got %q", payload.LibraryID)
	}
	if payload.TotalFiles != 250 {
		t.Errorf("expected TotalFiles 250, got %d", payload.TotalFiles)
	}
	if payload.Processed != 100 {
		t.Errorf("expected Processed 100, got %d", payload.Processed)
	}
	if payload.CurrentFile != "/movies/film.mkv" {
		t.Errorf("expected CurrentFile '/movies/film.mkv', got %q", payload.CurrentFile)
	}
}

func TestStreamPayload(t *testing.T) {
	payload := StreamPayload{
		StreamID:    "stream-xyz",
		SessionID:   "session-123",
		MediaItemID: "media-789",
		MediaTitle:  "Test Movie",
		Variant:     "1080p",
		BytesServed: 1024000,
		UserID:      "user-001",
		DeviceID:    "device-002",
	}

	if payload.StreamID != "stream-xyz" {
		t.Errorf("expected StreamID 'stream-xyz', got %q", payload.StreamID)
	}
	if payload.MediaTitle != "Test Movie" {
		t.Errorf("expected MediaTitle 'Test Movie', got %q", payload.MediaTitle)
	}
	if payload.Variant != "1080p" {
		t.Errorf("expected Variant '1080p', got %q", payload.Variant)
	}
	if payload.BytesServed != 1024000 {
		t.Errorf("expected BytesServed 1024000, got %d", payload.BytesServed)
	}
}

func TestTranscodePayload(t *testing.T) {
	payload := TranscodePayload{
		TranscodeID:    "trans-abc",
		SessionID:      "session-123",
		MediaItemID:    "media-456",
		MediaTitle:     "HDR Movie",
		SourceCodec:    "hevc",
		TargetCodec:    "h264",
		Progress:       45.5,
		Bitrate:        8000000,
		FrameRate:      24.0,
		Duration:       7200.0,
		ProcessedTime:  3240.0,
		Status:         "processing",
	}

	if payload.TranscodeID != "trans-abc" {
		t.Errorf("expected TranscodeID 'trans-abc', got %q", payload.TranscodeID)
	}
	if payload.SourceCodec != "hevc" {
		t.Errorf("expected SourceCodec 'hevc', got %q", payload.SourceCodec)
	}
	if payload.TargetCodec != "h264" {
		t.Errorf("expected TargetCodec 'h264', got %q", payload.TargetCodec)
	}
	if payload.Progress != 45.5 {
		t.Errorf("expected Progress 45.5, got %f", payload.Progress)
	}
}

func TestDevicePayload(t *testing.T) {
	payload := DevicePayload{
		DeviceID:     "device-001",
		DeviceName:   "Apple TV 4K",
		Platform:     "tvOS",
		Capabilities: map[string]interface{}{"hdr": true},
		TrustScore:   0.95,
	}

	if payload.DeviceID != "device-001" {
		t.Errorf("expected DeviceID 'device-001', got %q", payload.DeviceID)
	}
	if payload.DeviceName != "Apple TV 4K" {
		t.Errorf("expected DeviceName 'Apple TV 4K', got %q", payload.DeviceName)
	}
	if payload.Platform != "tvOS" {
		t.Errorf("expected Platform 'tvOS', got %q", payload.Platform)
	}
	if payload.TrustScore != 0.95 {
		t.Errorf("expected TrustScore 0.95, got %f", payload.TrustScore)
	}
}
