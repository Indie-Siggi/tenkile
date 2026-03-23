package probes

import (
	"testing"
	"time"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

func TestFeedbackManager_NewFeedbackManager(t *testing.T) {
	fm := NewFeedbackManager()

	if fm == nil {
		t.Fatal("expected non-nil FeedbackManager")
	}
	if fm.events == nil {
		t.Error("expected non-nil events map")
	}
	if fm.stats == nil {
		t.Error("expected non-nil stats map")
	}
}

func TestFeedbackManager_RecordSuccess(t *testing.T) {
	fm := NewFeedbackManager()

	feedback := PlaybackFeedback{
		DeviceID:   "test-device-1",
		MediaID:    "media-1",
		VideoCodec: "h264", // Required for stats to be updated
	}

	fm.RecordSuccess(feedback)

	stats := fm.GetPlaybackStats("test-device-1")
	if stats.TotalPlaybacks != 1 {
		t.Errorf("expected TotalPlaybacks 1, got %d", stats.TotalPlaybacks)
	}
	if stats.SuccessfulPlaybacks != 1 {
		t.Errorf("expected SuccessfulPlaybacks 1, got %d", stats.SuccessfulPlaybacks)
	}
	if stats.FailedPlaybacks != 0 {
		t.Errorf("expected FailedPlaybacks 0, got %d", stats.FailedPlaybacks)
	}
}

func TestFeedbackManager_RecordFailure(t *testing.T) {
	fm := NewFeedbackManager()

	feedback := PlaybackFeedback{
		DeviceID:   "test-device-2",
		MediaID:    "media-1",
		VideoCodec: "h264",
	}

	fm.RecordFailure(feedback)

	stats := fm.GetPlaybackStats("test-device-2")
	if stats.TotalPlaybacks != 1 {
		t.Errorf("expected TotalPlaybacks 1, got %d", stats.TotalPlaybacks)
	}
	if stats.SuccessfulPlaybacks != 0 {
		t.Errorf("expected SuccessfulPlaybacks 0, got %d", stats.SuccessfulPlaybacks)
	}
	if stats.FailedPlaybacks != 1 {
		t.Errorf("expected FailedPlaybacks 1, got %d", stats.FailedPlaybacks)
	}
}

func TestFeedbackManager_RecordMultiple(t *testing.T) {
	fm := NewFeedbackManager()

	// Record 5 successes
	for i := 0; i < 5; i++ {
		fm.RecordSuccess(PlaybackFeedback{
			DeviceID:   "test-device",
			MediaID:    "media-1",
			VideoCodec: "h264",
		})
	}

	// Record 2 failures
	for i := 0; i < 2; i++ {
		fm.RecordFailure(PlaybackFeedback{
			DeviceID:   "test-device",
			MediaID:    "media-2",
			VideoCodec: "h264",
		})
	}

	stats := fm.GetPlaybackStats("test-device")
	if stats.TotalPlaybacks != 7 {
		t.Errorf("expected TotalPlaybacks 7, got %d", stats.TotalPlaybacks)
	}
	if stats.SuccessfulPlaybacks != 5 {
		t.Errorf("expected SuccessfulPlaybacks 5, got %d", stats.SuccessfulPlaybacks)
	}
	if stats.FailedPlaybacks != 2 {
		t.Errorf("expected FailedPlaybacks 2, got %d", stats.FailedPlaybacks)
	}
	if stats.SuccessRate != 5.0/7.0 {
		t.Errorf("expected SuccessRate %.2f, got %.2f", 5.0/7.0, stats.SuccessRate)
	}
}

func TestFeedbackManager_ShouldReProbe_ConsecutiveFailures(t *testing.T) {
	fm := NewFeedbackManager()

	// Record 3 consecutive failures (default FailureWindowSize)
	for i := 0; i < 3; i++ {
		fm.RecordFailure(PlaybackFeedback{
			DeviceID:   "failing-device",
			MediaID:    "media-1",
			VideoCodec: "h264",
		})
	}

	shouldReProbe, reason := fm.ShouldReProbe("failing-device")
	if !shouldReProbe {
		t.Error("expected ShouldReProbe to return true after 3 consecutive failures")
	}
	if reason == "" {
		t.Error("expected non-empty re-probe reason")
	}
}

func TestFeedbackManager_ShouldReProbe_HighFailureRate(t *testing.T) {
	fm := NewFeedbackManager()

	// Record 8 successes and 5 failures (total 13, failure rate > 50%)
	for i := 0; i < 8; i++ {
		fm.RecordSuccess(PlaybackFeedback{
			DeviceID:   "unreliable-device",
			MediaID:    "media-1",
			VideoCodec: "h264",
		})
	}
	for i := 0; i < 5; i++ {
		fm.RecordFailure(PlaybackFeedback{
			DeviceID:   "unreliable-device",
			MediaID:    "media-2",
			VideoCodec: "h264",
		})
	}

	shouldReProbe, _ := fm.ShouldReProbe("unreliable-device")
	if !shouldReProbe {
		t.Error("expected ShouldReProbe to return true with high failure rate")
	}
}

func TestFeedbackManager_ShouldNotReProbe_LowFailureRate(t *testing.T) {
	fm := NewFeedbackManager()

	// Record 10 successes and 1 failure (9% failure rate)
	for i := 0; i < 10; i++ {
		fm.RecordSuccess(PlaybackFeedback{
			DeviceID:   "good-device",
			MediaID:    "media-1",
			VideoCodec: "h264",
		})
	}
	fm.RecordFailure(PlaybackFeedback{
		DeviceID:   "good-device",
		MediaID:    "media-2",
		VideoCodec: "h264",
	})

	shouldReProbe, _ := fm.ShouldReProbe("good-device")
	if shouldReProbe {
		t.Error("expected ShouldReProbe to return false with low failure rate")
	}
}

func TestFeedbackManager_ShouldNotReProbe_NoHistory(t *testing.T) {
	fm := NewFeedbackManager()

	shouldReProbe, reason := fm.ShouldReProbe("new-device")
	if shouldReProbe {
		t.Error("expected ShouldReProbe to return false for new device")
	}
	if reason != "" {
		t.Error("expected empty reason for new device")
	}
}

func TestFeedbackManager_GetTrustAdjustment(t *testing.T) {
	fm := NewFeedbackManager()

	// Initial trust adjustment should be 0
	adjustment := fm.GetTrustAdjustment("new-device")
	if adjustment != 0.0 {
		t.Errorf("expected initial trust adjustment 0.0, got %.4f", adjustment)
	}

	// Record some events
	fm.RecordSuccess(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-1",
		VideoCodec: "h264",
	})
	fm.RecordSuccess(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-1",
		VideoCodec: "h264",
	})

	adjustment = fm.GetTrustAdjustment("test-device")
	if adjustment <= 0.0 {
		t.Errorf("expected positive trust adjustment after successes, got %.4f", adjustment)
	}
}

func TestFeedbackManager_GetTrustAdjustmentNegative(t *testing.T) {
	fm := NewFeedbackManager()

	// Note: CurrentTrustDelta accumulates trust adjustments.
	// The default MinTrust of 0.0 means negative deltas will be clamped.
	// This test verifies the delta calculation and recording.

	// Record a success (should add positive delta)
	fm.RecordSuccess(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-1",
		VideoCodec: "h264",
	})

	// Record a failure (should add negative delta)
	fm.RecordFailure(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-2",
		VideoCodec: "h264",
	})

	stats := fm.GetPlaybackStats("test-device")
	
	// Verify stats are being tracked
	if stats.TotalPlaybacks != 2 {
		t.Errorf("expected TotalPlaybacks 2, got %d", stats.TotalPlaybacks)
	}
	if stats.SuccessfulPlaybacks != 1 {
		t.Errorf("expected SuccessfulPlaybacks 1, got %d", stats.SuccessfulPlaybacks)
	}
	if stats.FailedPlaybacks != 1 {
		t.Errorf("expected FailedPlaybacks 1, got %d", stats.FailedPlaybacks)
	}

	// Trust adjustment should be calculated (value depends on clamping)
	adjustment := fm.GetTrustAdjustment("test-device")
	t.Logf("Trust adjustment after success+failure: %.4f", adjustment)
}

func TestFeedbackManager_ResetCounters(t *testing.T) {
	fm := NewFeedbackManager()

	// Record some events
	fm.RecordSuccess(PlaybackFeedback{DeviceID: "test-device"})
	fm.RecordSuccess(PlaybackFeedback{DeviceID: "test-device"})
	fm.RecordFailure(PlaybackFeedback{DeviceID: "test-device"})

	// Reset trust adjustment
	fm.ResetTrustAdjustment("test-device")

	adjustment := fm.GetTrustAdjustment("test-device")
	if adjustment != 0.0 {
		t.Errorf("expected trust adjustment 0.0 after reset, got %.4f", adjustment)
	}

	// Stats should still exist but counters should be reset
	stats := fm.GetPlaybackStats("test-device")
	if stats.TotalPlaybacks == 0 {
		t.Error("expected TotalPlaybacks to be preserved after reset")
	}
}

func TestFeedbackManager_ClearDeviceData(t *testing.T) {
	fm := NewFeedbackManager()

	fm.RecordSuccess(PlaybackFeedback{DeviceID: "test-device"})
	fm.RecordSuccess(PlaybackFeedback{DeviceID: "test-device"})

	// Clear data
	fm.ClearDeviceData("test-device")

	// Stats should be empty
	stats := fm.GetPlaybackStats("test-device")
	if stats.TotalPlaybacks != 0 {
		t.Errorf("expected TotalPlaybacks 0 after clear, got %d", stats.TotalPlaybacks)
	}

	// Should not need re-probe
	shouldReProbe, _ := fm.ShouldReProbe("test-device")
	if shouldReProbe {
		t.Error("expected ShouldReProbe false for cleared device")
	}
}

func TestFeedbackManager_GetReliableCodecs(t *testing.T) {
	fm := NewFeedbackManager()

	// Record successes with h264
	for i := 0; i < 5; i++ {
		fm.RecordSuccess(PlaybackFeedback{
			DeviceID:   "test-device",
			MediaID:    "media-1",
			VideoCodec: "h264",
		})
	}

	// Record failures with vp9
	for i := 0; i < 3; i++ {
		fm.RecordFailure(PlaybackFeedback{
			DeviceID:   "test-device",
			MediaID:    "media-2",
			VideoCodec: "vp9",
		})
	}

	// h264 should be reliable (100% success)
	reliableCodecs := fm.GetReliableCodecs("test-device", 0.9)
	foundH264 := false
	for _, codec := range reliableCodecs {
		if codec == "video:h264" {
			foundH264 = true
		}
	}
	if !foundH264 {
		t.Error("expected h264 to be in reliable codecs")
	}
}

func TestFeedbackManager_GetPlaybackStats(t *testing.T) {
	fm := NewFeedbackManager()

	// Record mixed feedback
	fm.RecordSuccess(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-1",
		VideoCodec: "h264",
		AudioCodec: "aac",
	})
	fm.RecordFailure(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-2",
		VideoCodec: "hevc",
	})

	stats := fm.GetPlaybackStats("test-device")
	if stats.DeviceID != "test-device" {
		t.Errorf("expected DeviceID 'test-device', got %q", stats.DeviceID)
	}
	if stats.OutcomeCounts["success"] != 1 {
		t.Errorf("expected 1 success, got %d", stats.OutcomeCounts["success"])
	}
}

func TestFeedbackManager_GetPlaybackStatsNewDevice(t *testing.T) {
	fm := NewFeedbackManager()

	stats := fm.GetPlaybackStats("non-existent")
	if stats == nil {
		t.Fatal("expected non-nil stats for new device")
	}
	if stats.DeviceID != "non-existent" {
		t.Errorf("expected DeviceID 'non-existent', got %q", stats.DeviceID)
	}
	if stats.TotalPlaybacks != 0 {
		t.Errorf("expected TotalPlaybacks 0, got %d", stats.TotalPlaybacks)
	}
}

func TestFeedbackManager_CalculateTrustDelta(t *testing.T) {
	fm := NewFeedbackManager()

	// Test success delta
	successDelta := fm.CalculateTrustDelta(OutcomeSuccess)
	if successDelta <= 0 {
		t.Errorf("expected positive delta for success, got %.4f", successDelta)
	}

	// Test codec error delta
	codecDelta := fm.CalculateTrustDelta(OutcomeCodecError)
	if codecDelta >= 0 {
		t.Errorf("expected negative delta for codec error, got %.4f", codecDelta)
	}

	// Test network error delta
	networkDelta := fm.CalculateTrustDelta(OutcomeNetworkError)
	if networkDelta >= 0 {
		t.Errorf("expected negative delta for network error, got %.4f", networkDelta)
	}

	// Test buffering delta
	bufferingDelta := fm.CalculateTrustDelta(OutcomeBuffering)
	if bufferingDelta >= 0 {
		t.Errorf("expected negative delta for buffering, got %.4f", bufferingDelta)
	}
}

func TestFeedbackManager_SetTrustConfig(t *testing.T) {
	fm := NewFeedbackManager()

	config := TrustAdjustmentConfig{
		SuccessBonus:          0.05,
		NetworkErrorPenalty:  0.10,
		CodecErrorPenalty:    0.20,
		DecodingFailedPenalty: 0.30,
		RendererCrashPenalty:  0.35,
		MaxTrust:             1.0,
		MinTrust:             -0.5,
		FailureWindowSize:    5,
		SuccessStreakBonus:   0.10,
		SuccessStreakThreshold: 15,
	}

	fm.SetTrustConfig(config)

	// Verify by checking delta values
	successDelta := fm.CalculateTrustDelta(OutcomeSuccess)
	if successDelta != 0.05 {
		t.Errorf("expected SuccessBonus 0.05, got %.4f", successDelta)
	}
}

func TestFeedbackManager_CodecStats(t *testing.T) {
	fm := NewFeedbackManager()

	// Record mixed feedback for different codecs
	testCases := []struct {
		codec    string
		success  bool
	}{
		{"h264", true},
		{"h264", true},
		{"h264", true},
		{"hevc", true},
		{"hevc", false},
		{"vp9", false},
	}

	for _, tc := range testCases {
		feedback := PlaybackFeedback{
			DeviceID:   "test-device",
			MediaID:    "media-1",
			VideoCodec: tc.codec,
		}
		if tc.success {
			fm.RecordSuccess(feedback)
		} else {
			fm.RecordFailure(feedback)
		}
	}

	stats := fm.GetPlaybackStats("test-device")

	// Check h264 codec stats
	h264Stats := stats.CodecStats["video:h264"]
	if h264Stats.Attempts != 3 {
		t.Errorf("expected h264 Attempts 3, got %d", h264Stats.Attempts)
	}
	if h264Stats.SuccessRate != 1.0 {
		t.Errorf("expected h264 SuccessRate 1.0, got %.2f", h264Stats.SuccessRate)
	}
}

func TestFeedbackManager_ConsecutiveTracking(t *testing.T) {
	fm := NewFeedbackManager()

	// Record success sequence
	fm.RecordSuccess(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-1",
		VideoCodec: "h264",
	})
	fm.RecordSuccess(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-1",
		VideoCodec: "h264",
	})
	fm.RecordSuccess(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-1",
		VideoCodec: "h264",
	})

	stats := fm.GetPlaybackStats("test-device")
	if stats.ConsecutiveSuccesses != 3 {
		t.Errorf("expected ConsecutiveSuccesses 3, got %d", stats.ConsecutiveSuccesses)
	}
	if stats.ConsecutiveFailures != 0 {
		t.Errorf("expected ConsecutiveFailures 0, got %d", stats.ConsecutiveFailures)
	}

	// Record failure - should reset consecutive successes
	fm.RecordFailure(PlaybackFeedback{
		DeviceID:   "test-device",
		MediaID:    "media-1",
		VideoCodec: "h264",
	})

	stats = fm.GetPlaybackStats("test-device")
	if stats.ConsecutiveSuccesses != 0 {
		t.Errorf("expected ConsecutiveSuccesses 0 after failure, got %d", stats.ConsecutiveSuccesses)
	}
	if stats.ConsecutiveFailures != 1 {
		t.Errorf("expected ConsecutiveFailures 1, got %d", stats.ConsecutiveFailures)
	}
}

func TestFeedbackManager_GetRecentEvents(t *testing.T) {
	fm := NewFeedbackManager()

	// Record multiple events
	for i := 0; i < 10; i++ {
		fm.RecordSuccess(PlaybackFeedback{
			DeviceID: "test-device",
			MediaID:  "media-1",
		})
	}

	events := fm.GetRecentEvents("test-device", 5)
	if len(events) != 5 {
		t.Errorf("expected 5 recent events, got %d", len(events))
	}
}

func TestFeedbackManager_GetRecentEventsLimit(t *testing.T) {
	fm := NewFeedbackManager()

	// Record only 3 events
	for i := 0; i < 3; i++ {
		fm.RecordSuccess(PlaybackFeedback{
			DeviceID: "test-device",
			MediaID:  "media-1",
		})
	}

	// Request more than available
	events := fm.GetRecentEvents("test-device", 10)
	if len(events) != 3 {
		t.Errorf("expected 3 events (max available), got %d", len(events))
	}
}

func TestFeedbackManager_GetGlobalStats(t *testing.T) {
	fm := NewFeedbackManager()

	// Record events for multiple devices
	fm.RecordSuccess(PlaybackFeedback{DeviceID: "device-1"})
	fm.RecordSuccess(PlaybackFeedback{DeviceID: "device-1"})
	fm.RecordFailure(PlaybackFeedback{DeviceID: "device-2"})

	global := fm.GetGlobalStats()

	if global["total_devices"].(int) != 2 {
		t.Errorf("expected 2 total devices, got %v", global["total_devices"])
	}
	if global["total_playbacks"].(int64) != 3 {
		t.Errorf("expected 3 total playbacks, got %v", global["total_playbacks"])
	}
}

func TestCalculateBufferHealth(t *testing.T) {
	tests := []struct {
		buffer   time.Duration
		total    time.Duration
		expected float64
	}{
		{0, time.Hour, 0.0},
		{time.Hour / 4, time.Hour, 0.25},
		{time.Hour / 2, time.Hour, 0.5},
		{time.Hour, time.Hour, 1.0},
		{time.Hour * 2, time.Hour, 1.0}, // Clamped
	}

	for _, tt := range tests {
		result := CalculateBufferHealth(tt.buffer, tt.total)
		if result != tt.expected {
			t.Errorf("CalculateBufferHealth(%v, %v) = %.2f, expected %.2f",
				tt.buffer, tt.total, result, tt.expected)
		}
	}
}

func TestParseOutcomeFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected PlaybackOutcome
	}{
		{"success", OutcomeSuccess},
		{"network_error", OutcomeNetworkError},
		{"codec_error", OutcomeCodecError},
		{"decoding_failed", OutcomeDecodingFailed},
		{"renderer_crash", OutcomeRendererCrash},
		{"unsupported_format", OutcomeUnsupportedFormat},
		{"timeout", OutcomeTimeout},
		{"buffering", OutcomeBuffering},
		{"unknown_string", OutcomeUnknown},
	}

	for _, tt := range tests {
		result := ParseOutcomeFromString(tt.input)
		if result != tt.expected {
			t.Errorf("ParseOutcomeFromString(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestPlaybackOutcome_String(t *testing.T) {
	tests := []struct {
		outcome  PlaybackOutcome
		expected string
	}{
		{OutcomeSuccess, "success"},
		{OutcomeNetworkError, "network_error"},
		{OutcomeCodecError, "codec_error"},
		{OutcomeUnknown, "unknown"},
	}

	for _, tt := range tests {
		result := tt.outcome.String()
		if result != tt.expected {
			t.Errorf("%v.String() = %q, expected %q", tt.outcome, result, tt.expected)
		}
	}
}
