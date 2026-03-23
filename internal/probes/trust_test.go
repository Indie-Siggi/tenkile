package probes

import (
	"testing"
	"time"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

func TestTrustResolver_NewTrustResolver(t *testing.T) {
	tr := NewTrustResolver()

	if tr == nil {
		t.Fatal("expected non-nil TrustResolver")
	}
	if tr.scoreCache == nil {
		t.Error("expected non-nil scoreCache")
	}
	if tr.knownDevices == nil {
		t.Error("expected non-nil knownDevices")
	}
}

func TestTrustResolver_CalculateTrustScore_Basic(t *testing.T) {
	tr := NewTrustResolver()

	data := &DeviceTrustData{
		DeviceID:   "test-device-1",
		Platform:   "android",
		Manufacturer: "Samsung",
		Model:      "Galaxy S21",
		VideoCodecs: []string{"h264", "hevc"},
		AudioCodecs: []string{"aac", "ac3"},
		FirstSeen:  time.Now().Add(-24 * time.Hour),
		LastSeen:   time.Now(),
		ProbeCount: 5,
	}

	score := tr.CalculateTrustScore(data)

	if score == nil {
		t.Fatal("expected non-nil TrustScore")
	}
	if score.DeviceID != "test-device-1" {
		t.Errorf("expected DeviceID 'test-device-1', got %q", score.DeviceID)
	}
	if score.Score < 0 || score.Score > 1 {
		t.Errorf("expected Score between 0 and 1, got %.4f", score.Score)
	}
	if len(score.Components) == 0 {
		t.Error("expected at least one component")
	}
}

func TestTrustResolver_CalculateTrustScore_NilData(t *testing.T) {
	tr := NewTrustResolver()

	score := tr.CalculateTrustScore(nil)

	if score == nil {
		t.Fatal("expected non-nil TrustScore")
	}
	if score.Score != 0.0 {
		t.Errorf("expected Score 0.0 for nil data, got %.4f", score.Score)
	}
	if score.Level != TrustLevelUntrusted {
		t.Errorf("expected TrustLevel Untrusted, got %v", score.Level)
	}
}

func TestTrustResolver_CalculateTrustScore_Verified(t *testing.T) {
	tr := NewTrustResolver()

	unverified := &DeviceTrustData{
		DeviceID:   "test-device",
		Platform:   "android",
		FirstSeen:  time.Now().Add(-24 * time.Hour),
		LastSeen:   time.Now(),
		IsVerified: false,
	}
	verified := &DeviceTrustData{
		DeviceID:   "test-device-2",
		Platform:   "android",
		FirstSeen:  time.Now().Add(-24 * time.Hour),
		LastSeen:   time.Now(),
		IsVerified: true,
	}

	score1 := tr.CalculateTrustScore(unverified)
	score2 := tr.CalculateTrustScore(verified)

	if score2.Score <= score1.Score {
		t.Errorf("expected verified device to have higher score: unverified=%.4f, verified=%.4f",
			score1.Score, score2.Score)
	}
}

func TestTrustResolver_CalculateTrustScore_Curated(t *testing.T) {
	tr := NewTrustResolver()

	uncurated := &DeviceTrustData{
		DeviceID:   "test-device-1",
		Platform:   "android",
		FirstSeen:  time.Now().Add(-24 * time.Hour),
		LastSeen:   time.Now(),
		IsCurated:  false,
	}
	curated := &DeviceTrustData{
		DeviceID:   "test-device-2",
		Platform:   "android",
		FirstSeen:  time.Now().Add(-24 * time.Hour),
		LastSeen:   time.Now(),
		IsCurated:  true,
	}

	score1 := tr.CalculateTrustScore(uncurated)
	score2 := tr.CalculateTrustScore(curated)

	if score2.Score <= score1.Score {
		t.Errorf("expected curated device to have higher score: uncurated=%.4f, curated=%.4f",
			score1.Score, score2.Score)
	}
}

func TestTrustResolver_TrustLevel(t *testing.T) {
	tr := NewTrustResolver()

	tests := []struct {
		score   float64
		level   TrustLevel
	}{
		{0.95, TrustLevelVerified},
		{0.85, TrustLevelTrusted},
		{0.75, TrustLevelHigh},
		{0.55, TrustLevelMedium},
		{0.35, TrustLevelLow},
		{0.15, TrustLevelUntrusted},
		{0.05, TrustLevelUnknown},
	}

	for _, tt := range tests {
		level := tr.scoreToLevel(tt.score)
		if level != tt.level {
			t.Errorf("scoreToLevel(%.2f) = %v, expected %v", tt.score, level, tt.level)
		}
	}
}

func TestTrustLevel_String(t *testing.T) {
	tests := []struct {
		level    TrustLevel
		expected string
	}{
		{TrustLevelUnknown, "unknown"},
		{TrustLevelUntrusted, "untrusted"},
		{TrustLevelLow, "low"},
		{TrustLevelMedium, "medium"},
		{TrustLevelHigh, "high"},
		{TrustLevelTrusted, "trusted"},
		{TrustLevelVerified, "verified"},
	}

	for _, tt := range tests {
		result := tt.level.String()
		if result != tt.expected {
			t.Errorf("TrustLevel(%d).String() = %q, expected %q", tt.level, result, tt.expected)
		}
	}
}

func TestTrustResolver_GetCachedScore(t *testing.T) {
	tr := NewTrustResolver()

	data := &DeviceTrustData{
		DeviceID: "test-device",
		Platform: "android",
	}

	// Calculate and cache score
	score := tr.CalculateTrustScore(data)

	// Get cached score
	cached, found := tr.GetCachedScore("test-device")
	if !found {
		t.Error("expected to find cached score")
	}
	if cached.Score != score.Score {
		t.Errorf("expected cached score %.4f, got %.4f", score.Score, cached.Score)
	}
}

func TestTrustResolver_GetCachedScore_NotFound(t *testing.T) {
	tr := NewTrustResolver()

	_, found := tr.GetCachedScore("non-existent")
	if found {
		t.Error("expected not to find cached score")
	}
}

func TestTrustResolver_InvalidateCache(t *testing.T) {
	tr := NewTrustResolver()

	// Calculate score
	tr.CalculateTrustScore(&DeviceTrustData{
		DeviceID: "test-device",
		Platform: "android",
	})

	// Verify it's cached
	_, found := tr.GetCachedScore("test-device")
	if !found {
		t.Error("expected to find cached score before invalidation")
	}

	// Invalidate
	tr.InvalidateCache("test-device")

	// Verify it's gone
	_, found = tr.GetCachedScore("test-device")
	if found {
		t.Error("expected not to find cached score after invalidation")
	}
}

func TestTrustResolver_RegisterKnownDevice(t *testing.T) {
	tr := NewTrustResolver()

	// Register a device
	tr.RegisterKnownDevice("known-fingerprint")

	// Check if registered
	if !tr.knownDevices["known-fingerprint"] {
		t.Error("expected device to be registered")
	}
}

func TestTrustResolver_GetTrustLevelForDevice(t *testing.T) {
	tr := NewTrustResolver()

	// No score should return Unknown
	level := tr.GetTrustLevelForDevice("non-existent")
	if level != TrustLevelUnknown {
		t.Errorf("expected TrustLevel Unknown for non-existent device, got %v", level)
	}

	// Calculate score
	tr.CalculateTrustScore(&DeviceTrustData{
		DeviceID: "test-device",
		Platform: "android",
		FirstSeen: time.Now().Add(-24 * time.Hour),
		LastSeen: time.Now(),
	})

	level = tr.GetTrustLevelForDevice("test-device")
	if level == TrustLevelUnknown {
		t.Error("expected TrustLevel not Unknown after score calculation")
	}
}

func TestTrustResolver_ValidateCapabilities(t *testing.T) {
	// Valid capabilities
	valid, warnings := ValidateCapabilities(
		[]string{"h264", "hevc", "vp9"},
		[]string{"aac", "mp3", "ac3"},
		[]string{"mp4", "mkv", "webm"},
	)

	if !valid {
		t.Error("expected capabilities to be valid")
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

func TestTrustResolver_ValidateCapabilities_TooManyCodecs(t *testing.T) {
	// Create a list with more than 10 video codecs
	codecs := make([]string, 15)
	for i := range codecs {
		codecs[i] = "codec" + string(rune('A'+i))
	}

	_, warnings := ValidateCapabilities(codecs, []string{"aac"}, []string{"mp4"})

	foundWarning := false
	for _, w := range warnings {
		if len(w) > 0 {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected warning for unusual number of codecs")
	}
}

func TestTrustResolver_MarshalUnmarshal(t *testing.T) {
	tr := NewTrustResolver()

	data := &DeviceTrustData{
		DeviceID: "test-device",
		Platform: "android",
	}
	score := tr.CalculateTrustScore(data)

	// Marshal
	bytes, err := MarshalTrustScore(score)
	if err != nil {
		t.Fatalf("MarshalTrustScore failed: %v", err)
	}

	// Unmarshal
	restored, err := UnmarshalTrustScore(bytes)
	if err != nil {
		t.Fatalf("UnmarshalTrustScore failed: %v", err)
	}

	if restored.DeviceID != score.DeviceID {
		t.Errorf("expected DeviceID %q, got %q", score.DeviceID, restored.DeviceID)
	}
	if restored.Score != score.Score {
		t.Errorf("expected Score %.4f, got %.4f", score.Score, restored.Score)
	}
}

func TestTrustScore_SortComponents(t *testing.T) {
	score := &TrustScore{
		Components: []TrustComponent{
			{Name: "low", Score: 0.3},
			{Name: "high", Score: 0.9},
			{Name: "mid", Score: 0.6},
		},
	}

	score.SortComponents()

	if score.Components[0].Score != 0.9 {
		t.Errorf("expected first component score 0.9, got %.2f", score.Components[0].Score)
	}
	if score.Components[2].Score != 0.3 {
		t.Errorf("expected last component score 0.3, got %.2f", score.Components[2].Score)
	}
}

func TestTrustResolver_EvaluateFingerprint(t *testing.T) {
	tr := NewTrustResolver()

	// Test with valid platform
	data := &DeviceTrustData{
		Platform:    "android",
		Manufacturer: "Samsung",
	}

	score, reason := tr.evaluateFingerprint(data)
	if score <= 0 {
		t.Errorf("expected positive score for valid platform, got %.4f", score)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestTrustResolver_EvaluateFingerprint_KnownDevice(t *testing.T) {
	tr := NewTrustResolver()

	// Register the device first
	data := &DeviceTrustData{
		Platform:    "android",
		Manufacturer: "Samsung",
		Model:       "Galaxy",
	}
	fingerprint := tr.generateFingerprint(data)
	tr.RegisterKnownDevice(fingerprint)

	score, reason := tr.evaluateFingerprint(data)
	if score < 0.7 {
		t.Errorf("expected high score for known device, got %.4f", score)
	}
	if reason != "Recognized device fingerprint" {
		t.Errorf("expected 'Recognized device fingerprint', got %q", reason)
	}
}

func TestTrustResolver_EvaluateConsistency(t *testing.T) {
	tr := NewTrustResolver()

	tests := []struct {
		name       string
		data       *DeviceTrustData
		minScore   float64
	}{
		{
			"no probes",
			&DeviceTrustData{ProbeCount: 0},
			0.4,
		},
		{
			"consistent probes",
			&DeviceTrustData{ProbeCount: 10, ConsistentProbes: 10},
			0.9,
		},
		{
			"high anomaly rate",
			&DeviceTrustData{ProbeCount: 10, AnomalyCount: 6},
			0.0, // Should be low
		},
	}

	for _, tt := range tests {
		score, _ := tr.evaluateConsistency(tt.data)
		if tt.name == "high anomaly rate" {
			if score >= 0.3 {
				t.Errorf("%s: expected low score for high anomaly rate, got %.4f", tt.name, score)
			}
		} else if score < tt.minScore {
			t.Errorf("%s: expected score >= %.2f, got %.4f", tt.name, tt.minScore, score)
		}
	}
}

func TestTrustResolver_EvaluatePlatform(t *testing.T) {
	tr := NewTrustResolver()

	tests := []struct {
		platform  string
		minScore  float64
	}{
		{"android", 0.8},
		{"ios", 0.8},
		{"tvos", 0.8},
		{"web", 0.8},
		{"unknown_platform", 0.0},
	}

	for _, tt := range tests {
		data := &DeviceTrustData{Platform: tt.platform}
		score, _ := tr.evaluatePlatform(data)
		if tt.platform == "unknown_platform" {
			if score >= 0.3 {
				t.Errorf("expected low score for unknown platform, got %.4f", score)
			}
		} else if score < tt.minScore {
			t.Errorf("platform %s: expected score >= %.2f, got %.4f", tt.platform, tt.minScore, score)
		}
	}
}

func TestTrustResolver_EvaluateBehavior(t *testing.T) {
	tr := NewTrustResolver()

	tests := []struct {
		name       string
		data       *DeviceTrustData
		minScore   float64
	}{
		{
			"new device",
			&DeviceTrustData{
				FirstSeen: time.Now(),
				LastSeen:  time.Now(),
			},
			0.0, // Less than 1 hour
		},
		{
			"established device",
			&DeviceTrustData{
				FirstSeen: time.Now().Add(-48 * time.Hour),
				LastSeen:  time.Now(),
			},
			0.6,
		},
		{
			"inactive device",
			&DeviceTrustData{
				FirstSeen: time.Now().Add(-30 * 24 * time.Hour),
				LastSeen:  time.Now().Add(-60 * 24 * time.Hour),
			},
			0.4,
		},
	}

	for _, tt := range tests {
		score, _ := tr.evaluateBehavior(tt.data)
		if score < tt.minScore {
			t.Errorf("%s: expected score >= %.2f, got %.4f", tt.name, tt.minScore, score)
		}
	}
}

func TestCalculateTrustDeltaFromConfig(t *testing.T) {
	config := DefaultTrustAdjustmentConfig()

	tests := []struct {
		outcome  PlaybackOutcome
		expected float64
	}{
		{OutcomeSuccess, config.SuccessBonus},
		{OutcomeNetworkError, -config.NetworkErrorPenalty},
		{OutcomeCodecError, -config.CodecErrorPenalty},
		{OutcomeDecodingFailed, -config.DecodingFailedPenalty},
		{OutcomeRendererCrash, -config.RendererCrashPenalty},
		{OutcomeTimeout, -config.NetworkErrorPenalty},
		{OutcomeBuffering, -config.NetworkErrorPenalty * 0.5},
		{OutcomeUnknown, 0.0},
	}

	for _, tt := range tests {
		delta := calculateTrustDeltaFromConfig(config, tt.outcome)
		if delta != tt.expected {
			t.Errorf("outcome %v: expected %.4f, got %.4f", tt.outcome, tt.expected, delta)
		}
	}
}
