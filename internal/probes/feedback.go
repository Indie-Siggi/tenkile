// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"
)

// PlaybackOutcome represents the outcome of a playback attempt
type PlaybackOutcome int

const (
	OutcomeUnknown PlaybackOutcome = iota
	OutcomeSuccess
	OutcomeNetworkError
	OutcomeCodecError
	OutcomeDecodingFailed
	OutcomeRendererCrash
	OutcomeUnsupportedFormat
	OutcomeTimeout
	OutcomeBuffering
)

func (o PlaybackOutcome) String() string {
	switch o {
	case OutcomeUnknown:
		return "unknown"
	case OutcomeSuccess:
		return "success"
	case OutcomeNetworkError:
		return "network_error"
	case OutcomeCodecError:
		return "codec_error"
	case OutcomeDecodingFailed:
		return "decoding_failed"
	case OutcomeRendererCrash:
		return "renderer_crash"
	case OutcomeUnsupportedFormat:
		return "unsupported_format"
	case OutcomeTimeout:
		return "timeout"
	case OutcomeBuffering:
		return "buffering"
	default:
		return "unknown"
	}
}

// PlaybackFeedback represents feedback from a playback attempt
type PlaybackFeedback struct {
	DeviceID      string         `json:"device_id"`
	MediaID       string         `json:"media_id"`
	Outcome       PlaybackOutcome `json:"outcome"`
	ErrorCode     string         `json:"error_code,omitempty"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
	Duration      time.Duration  `json:"duration"`
	BufferDuration time.Duration `json:"buffer_duration,omitempty"`
	NetworkQuality string        `json:"network_quality,omitempty"`
	VideoCodec    string         `json:"video_codec,omitempty"`
	AudioCodec    string         `json:"audio_codec,omitempty"`
	Container     string         `json:"container,omitempty"`
	Resolution    string         `json:"resolution,omitempty"`
	Bitrate       int64          `json:"bitrate,omitempty"`
}

// PlaybackEvent represents a single playback event for tracking
type PlaybackEvent struct {
	Feedback    PlaybackFeedback
	TrustDelta  float64
	Timestamp   time.Time
	IsRecent    bool
}

// DevicePlaybackStats holds playback statistics for a device
type DevicePlaybackStats struct {
	DeviceID             string               `json:"device_id"`
	TotalPlaybacks       int64                `json:"total_playbacks"`
	SuccessfulPlaybacks  int64                `json:"successful_playbacks"`
	FailedPlaybacks      int64                `json:"failed_playbacks"`
	SuccessRate          float64              `json:"success_rate"`
	ConsecutiveSuccesses  int64                `json:"consecutive_successes"`
	ConsecutiveFailures   int64                `json:"consecutive_failures"`
	OutcomeCounts        map[string]int64     `json:"outcome_counts"`
	CodecStats           map[string]CodecStats `json:"codec_stats"`
	LastPlayback         time.Time            `json:"last_playback"`
	LastSuccess          time.Time            `json:"last_success"`
	LastFailure          time.Time            `json:"last_failure"`
	CurrentTrustDelta    float64              `json:"current_trust_delta"`
	NeedsReProbe         bool                 `json:"needs_reprobe"`
	ReProbeReason        string               `json:"reprobe_reason,omitempty"`
}

// CodecStats holds statistics for a specific codec
type CodecStats struct {
	Attempts   int64   `json:"attempts"`
	Successes  int64   `json:"successes"`
	Failures   int64   `json:"failures"`
	SuccessRate float64 `json:"success_rate"`
}

// FeedbackManager manages playback feedback and trust adjustments
type FeedbackManager struct {
	mu sync.RWMutex

	// Event storage
	events     map[string][]PlaybackEvent // deviceID -> events
	maxEvents  int                        // Max events per device

	// Rolling window
	windowDuration time.Duration

	// Statistics
	stats map[string]*DevicePlaybackStats // deviceID -> stats

	// Trust configuration
	trustConfig TrustAdjustmentConfig

	// Callbacks
	onTrustChange     func(deviceID string, oldScore, newScore float64)
	onReProbeRequired func(deviceID string, reason string)
}

// TrustAdjustmentConfig holds trust adjustment parameters
type TrustAdjustmentConfig struct {
	SuccessBonus          float64
	NetworkErrorPenalty   float64
	CodecErrorPenalty     float64
	DecodingFailedPenalty float64
	RendererCrashPenalty  float64
	MaxTrust              float64
	MinTrust              float64
	FailureWindowSize     int
	SuccessStreakBonus    float64
	SuccessStreakThreshold int
}

// DefaultTrustAdjustmentConfig returns the default trust adjustment configuration
func DefaultTrustAdjustmentConfig() TrustAdjustmentConfig {
	return TrustAdjustmentConfig{
		SuccessBonus:           0.01,
		NetworkErrorPenalty:   0.05,
		CodecErrorPenalty:     0.15,
		DecodingFailedPenalty: 0.25,
		RendererCrashPenalty:   0.30,
		MaxTrust:               1.0,
		MinTrust:               0.0,
		FailureWindowSize:      3,
		SuccessStreakBonus:     0.05,
		SuccessStreakThreshold: 10,
	}
}

// NewFeedbackManager creates a new feedback manager
func NewFeedbackManager() *FeedbackManager {
	return &FeedbackManager{
		events:        make(map[string][]PlaybackEvent),
		maxEvents:     1000,
		windowDuration: 24 * time.Hour,
		stats:         make(map[string]*DevicePlaybackStats),
		trustConfig:   DefaultTrustAdjustmentConfig(),
	}
}

// SetTrustConfig sets the trust adjustment configuration
func (fm *FeedbackManager) SetTrustConfig(config TrustAdjustmentConfig) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.trustConfig = config
}

// SetOnTrustChange sets the callback for trust changes
func (fm *FeedbackManager) SetOnTrustChange(callback func(deviceID string, oldScore, newScore float64)) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.onTrustChange = callback
}

// SetOnReProbeRequired sets the callback for re-probe requirements
func (fm *FeedbackManager) SetOnReProbeRequired(callback func(deviceID string, reason string)) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.onReProbeRequired = callback
}

// RecordSuccess records a successful playback
func (fm *FeedbackManager) RecordSuccess(feedback PlaybackFeedback) {
	feedback.Outcome = OutcomeSuccess
	feedback.Timestamp = time.Now()
	fm.recordFeedback(feedback)
}

// RecordFailure records a failed playback
func (fm *FeedbackManager) RecordFailure(feedback PlaybackFeedback) {
	feedback.Timestamp = time.Now()
	fm.recordFeedback(feedback)
}

// recordFeedback records a playback feedback event
func (fm *FeedbackManager) recordFeedback(feedback PlaybackFeedback) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Initialize device if not exists
	if _, ok := fm.events[feedback.DeviceID]; !ok {
		fm.events[feedback.DeviceID] = []PlaybackEvent{}
		fm.stats[feedback.DeviceID] = &DevicePlaybackStats{
			DeviceID:      feedback.DeviceID,
			OutcomeCounts: make(map[string]int64),
			CodecStats:    make(map[string]CodecStats),
		}
	}

	// Calculate trust delta
	trustDelta := fm.CalculateTrustDelta(feedback.Outcome)

	// Create event
	event := PlaybackEvent{
		Feedback:   feedback,
		TrustDelta: trustDelta,
		Timestamp:  time.Now(),
		IsRecent:   true,
	}

	// Add event
	events := fm.events[feedback.DeviceID]
	events = append(events, event)

	// Trim old events if necessary
	if len(events) > fm.maxEvents {
		events = events[len(events)-fm.maxEvents:]
	}
	fm.events[feedback.DeviceID] = events

	// Update statistics
	fm.updateStats(feedback, trustDelta)

	// Check if re-probe is needed (must be done while mutex is held)
	stats := fm.stats[feedback.DeviceID]
	needsReProbe := false
	reProbeReason := ""
	if stats != nil {
		needsReProbe = fm.shouldReProbe(stats)
		if needsReProbe {
			reProbeReason = fm.getReProbeReason(stats)
			stats.NeedsReProbe = true
			stats.ReProbeReason = reProbeReason
		}
	}

	// Capture values before unlocking to avoid race conditions in goroutines
	capturedReProbe := needsReProbe
	capturedReProbeReason := reProbeReason
	capturedTrustDelta := stats.CurrentTrustDelta + trustDelta
	stats.CurrentTrustDelta = capturedTrustDelta

	// Check if re-probe is needed
	if capturedReProbe {
		if fm.onReProbeRequired != nil {
			go fm.onReProbeRequired(feedback.DeviceID, capturedReProbeReason)
		}
	}

	// Notify trust change callback
	if fm.onTrustChange != nil {
		go fm.onTrustChange(feedback.DeviceID, stats.CurrentTrustDelta, capturedTrustDelta)
	}

	slog.Debug("Recorded playback feedback",
		"device_id", feedback.DeviceID,
		"outcome", feedback.Outcome.String(),
		"trust_delta", trustDelta)
}

// CalculateTrustDelta calculates the trust adjustment for an outcome
func (fm *FeedbackManager) CalculateTrustDelta(outcome PlaybackOutcome) float64 {
	fm.mu.RLock()
	config := fm.trustConfig
	fm.mu.RUnlock()

	var delta float64
	switch outcome {
	case OutcomeSuccess:
		delta = config.SuccessBonus
	case OutcomeNetworkError:
		delta = -config.NetworkErrorPenalty
	case OutcomeCodecError:
		delta = -config.CodecErrorPenalty
	case OutcomeDecodingFailed:
		delta = -config.DecodingFailedPenalty
	case OutcomeRendererCrash:
		delta = -config.RendererCrashPenalty
	case OutcomeUnsupportedFormat:
		delta = -config.CodecErrorPenalty // Treat as codec error
	case OutcomeTimeout:
		delta = -config.NetworkErrorPenalty
	case OutcomeBuffering:
		delta = -config.NetworkErrorPenalty * 0.5 // Less severe
	default:
		delta = 0
	}

	return delta
}

// updateStats updates playback statistics for a device
func (fm *FeedbackManager) updateStats(feedback PlaybackFeedback, trustDelta float64) {
	stats := fm.stats[feedback.DeviceID]
	if stats == nil {
		return
	}

	stats.TotalPlaybacks++
	stats.LastPlayback = time.Now()

	// Update outcome counts
	outcomeStr := feedback.Outcome.String()
	stats.OutcomeCounts[outcomeStr]++

	// Update codec stats if available
	if feedback.VideoCodec != "" {
		codecKey := "video:" + feedback.VideoCodec
		codecStats := stats.CodecStats[codecKey]
		codecStats.Attempts++
		if feedback.Outcome == OutcomeSuccess {
			codecStats.Successes++
			stats.SuccessfulPlaybacks++
			stats.LastSuccess = time.Now()
			stats.ConsecutiveSuccesses++
			stats.ConsecutiveFailures = 0
		} else {
			codecStats.Failures++
			stats.FailedPlaybacks++
			stats.LastFailure = time.Now()
			stats.ConsecutiveFailures++
			stats.ConsecutiveSuccesses = 0
		}
		codecStats.SuccessRate = float64(codecStats.Successes) / float64(codecStats.Attempts)
		stats.CodecStats[codecKey] = codecStats
	}

	if feedback.AudioCodec != "" {
		codecKey := "audio:" + feedback.AudioCodec
		codecStats := stats.CodecStats[codecKey]
		codecStats.Attempts++
		if feedback.Outcome == OutcomeSuccess {
			codecStats.Successes++
			// Update global success counters for audio-only streams
			if feedback.VideoCodec == "" {
				stats.SuccessfulPlaybacks++
				stats.LastSuccess = time.Now()
				stats.ConsecutiveSuccesses++
				stats.ConsecutiveFailures = 0
			}
		} else {
			codecStats.Failures++
			// Update global failure counters for audio-only streams
			if feedback.VideoCodec == "" {
				stats.FailedPlaybacks++
				stats.LastFailure = time.Now()
				stats.ConsecutiveFailures++
				stats.ConsecutiveSuccesses = 0
			}
		}
		codecStats.SuccessRate = float64(codecStats.Successes) / float64(codecStats.Attempts)
		stats.CodecStats[codecKey] = codecStats
	}

	// Update success rate
	if stats.TotalPlaybacks > 0 {
		stats.SuccessRate = float64(stats.SuccessfulPlaybacks) / float64(stats.TotalPlaybacks)
	}

	// Update trust delta
	stats.CurrentTrustDelta += trustDelta

	// Clamp trust delta
	if stats.CurrentTrustDelta > fm.trustConfig.MaxTrust {
		stats.CurrentTrustDelta = fm.trustConfig.MaxTrust
	}
	if stats.CurrentTrustDelta < fm.trustConfig.MinTrust {
		stats.CurrentTrustDelta = fm.trustConfig.MinTrust
	}

	// Check for success streak bonus
	if stats.ConsecutiveSuccesses >= int64(fm.trustConfig.SuccessStreakThreshold) {
		stats.CurrentTrustDelta += fm.trustConfig.SuccessStreakBonus
		if stats.CurrentTrustDelta > fm.trustConfig.MaxTrust {
			stats.CurrentTrustDelta = fm.trustConfig.MaxTrust
		}
	}
}

// shouldReProbe determines if a device needs re-probing
func (fm *FeedbackManager) shouldReProbe(stats *DevicePlaybackStats) bool {
	if stats == nil {
		return false
	}

	// Check for 3+ consecutive failures
	if stats.ConsecutiveFailures >= int64(fm.trustConfig.FailureWindowSize) {
		return true
	}

	// Check for high failure rate in recent events
	if stats.TotalPlaybacks >= 10 {
		failureRate := float64(stats.FailedPlaybacks) / float64(stats.TotalPlaybacks)
		if failureRate > 0.5 {
			return true
		}
	}

	return false
}

// getReProbeReason returns the reason for re-probing
func (fm *FeedbackManager) getReProbeReason(stats *DevicePlaybackStats) string {
	if stats.ConsecutiveFailures >= int64(fm.trustConfig.FailureWindowSize) {
		return fmt.Sprintf("%d consecutive failures", stats.ConsecutiveFailures)
	}

	if stats.TotalPlaybacks >= 10 {
		failureRate := float64(stats.FailedPlaybacks) / float64(stats.TotalPlaybacks)
		if failureRate > 0.5 {
			return fmt.Sprintf("high failure rate: %.1f%%", failureRate*100)
		}
	}

	return "unknown"
}

// ShouldReProbe checks if a specific device needs re-probing
func (fm *FeedbackManager) ShouldReProbe(deviceID string) (bool, string) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	stats, ok := fm.stats[deviceID]
	if !ok {
		return false, ""
	}

	if fm.shouldReProbe(stats) {
		return true, stats.ReProbeReason
	}

	return false, ""
}

// GetPlaybackStats returns playback statistics for a device
func (fm *FeedbackManager) GetPlaybackStats(deviceID string) *DevicePlaybackStats {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	stats, ok := fm.stats[deviceID]
	if !ok {
		return &DevicePlaybackStats{
			DeviceID:      deviceID,
			OutcomeCounts: make(map[string]int64),
			CodecStats:    make(map[string]CodecStats),
		}
	}

	// Return a copy
	result := *stats
	result.OutcomeCounts = make(map[string]int64)
	for k, v := range stats.OutcomeCounts {
		result.OutcomeCounts[k] = v
	}
	result.CodecStats = make(map[string]CodecStats)
	for k, v := range stats.CodecStats {
		result.CodecStats[k] = v
	}

	return &result
}

// GetReliableCodecs returns codecs that work reliably on a device
func (fm *FeedbackManager) GetReliableCodecs(deviceID string, minSuccessRate float64) []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	stats, ok := fm.stats[deviceID]
	if !ok {
		return []string{}
	}

	var reliableCodecs []string
	for codec, codecStats := range stats.CodecStats {
		if codecStats.Attempts >= 3 && codecStats.SuccessRate >= minSuccessRate {
			reliableCodecs = append(reliableCodecs, codec)
		}
	}

	return reliableCodecs
}

// GetAllStats returns statistics for all devices
func (fm *FeedbackManager) GetAllStats() map[string]*DevicePlaybackStats {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make(map[string]*DevicePlaybackStats)
	for deviceID, stats := range fm.stats {
		// Return copies
		statsCopy := *stats
		statsCopy.OutcomeCounts = make(map[string]int64)
		for k, v := range stats.OutcomeCounts {
			statsCopy.OutcomeCounts[k] = v
		}
		statsCopy.CodecStats = make(map[string]CodecStats)
		for k, v := range stats.CodecStats {
			statsCopy.CodecStats[k] = v
		}
		result[deviceID] = &statsCopy
	}

	return result
}

// ExpireOldEvents removes events older than the rolling window
func (fm *FeedbackManager) ExpireOldEvents() int {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	cutoff := time.Now().Add(-fm.windowDuration)
	totalExpired := 0

	for deviceID, events := range fm.events {
		var recentEvents []PlaybackEvent
		for _, event := range events {
			if event.Timestamp.After(cutoff) {
				recentEvents = append(recentEvents, event)
			}
		}

		if len(recentEvents) < len(events) {
			totalExpired += len(events) - len(recentEvents)
			fm.events[deviceID] = recentEvents
		}
	}

	return totalExpired
}

// GetRecentEvents returns recent playback events for a device
func (fm *FeedbackManager) GetRecentEvents(deviceID string, limit int) []PlaybackFeedback {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	events, ok := fm.events[deviceID]
	if !ok {
		return []PlaybackFeedback{}
	}

	// Get most recent events
	start := len(events) - limit
	if start < 0 {
		start = 0
	}

	feedbacks := make([]PlaybackFeedback, 0, limit)
	for i := start; i < len(events); i++ {
		feedbacks = append(feedbacks, events[i].Feedback)
	}

	return feedbacks
}

// GetTrustAdjustment returns the current trust adjustment for a device
func (fm *FeedbackManager) GetTrustAdjustment(deviceID string) float64 {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	stats, ok := fm.stats[deviceID]
	if !ok {
		return 0.0
	}

	return stats.CurrentTrustDelta
}

// ResetTrustAdjustment resets the trust adjustment for a device
func (fm *FeedbackManager) ResetTrustAdjustment(deviceID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if stats, ok := fm.stats[deviceID]; ok {
		stats.CurrentTrustDelta = 0.0
		stats.ConsecutiveSuccesses = 0
		stats.ConsecutiveFailures = 0
		stats.NeedsReProbe = false
		stats.ReProbeReason = ""
	}
}

// ClearDeviceData clears all data for a device
func (fm *FeedbackManager) ClearDeviceData(deviceID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	delete(fm.events, deviceID)
	delete(fm.stats, deviceID)
}

// GetGlobalStats returns aggregate statistics across all devices
func (fm *FeedbackManager) GetGlobalStats() map[string]interface{} {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	totalPlaybacks := int64(0)
	totalSuccesses := int64(0)
	totalFailures := int64(0)
	devicesNeedingReProbe := 0

	outcomeTotals := make(map[string]int64)

	for _, stats := range fm.stats {
		totalPlaybacks += stats.TotalPlaybacks
		totalSuccesses += stats.SuccessfulPlaybacks
		totalFailures += stats.FailedPlaybacks
		if stats.NeedsReProbe {
			devicesNeedingReProbe++
		}

		for outcome, count := range stats.OutcomeCounts {
			outcomeTotals[outcome] += count
		}
	}

	return map[string]interface{}{
		"total_devices":          len(fm.stats),
		"total_playbacks":        totalPlaybacks,
		"total_successes":       totalSuccesses,
		"total_failures":        totalFailures,
		"global_success_rate":    float64(totalSuccesses) / float64(totalPlaybacks),
		"devices_needing_reprobe": devicesNeedingReProbe,
		"outcome_totals":        outcomeTotals,
	}
}

// ParseOutcomeFromString parses a playback outcome from string
func ParseOutcomeFromString(s string) PlaybackOutcome {
	switch s {
	case "success":
		return OutcomeSuccess
	case "network_error":
		return OutcomeNetworkError
	case "codec_error":
		return OutcomeCodecError
	case "decoding_failed":
		return OutcomeDecodingFailed
	case "renderer_crash":
		return OutcomeRendererCrash
	case "unsupported_format":
		return OutcomeUnsupportedFormat
	case "timeout":
		return OutcomeTimeout
	case "buffering":
		return OutcomeBuffering
	default:
		return OutcomeUnknown
	}
}

// SortCodecStatsBySuccessRate sorts codec stats by success rate
func SortCodecStatsBySuccessRate(stats map[string]CodecStats) []struct {
	Codec  string
	Stats  CodecStats
} {
	type codecWithStats struct {
		Codec string
		Stats CodecStats
	}

	var sorted []codecWithStats
	for codec, cs := range stats {
		sorted = append(sorted, codecWithStats{Codec: codec, Stats: cs})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Stats.SuccessRate > sorted[j].Stats.SuccessRate
	})

	result := make([]struct {
		Codec string
		Stats CodecStats
	}, len(sorted))
	for i, cs := range sorted {
		result[i] = struct {
			Codec string
			Stats CodecStats
		}{cs.Codec, cs.Stats}
	}

	return result
}

// CalculateBufferHealth calculates buffer health from events
func CalculateBufferHealth(bufferDuration, totalDuration time.Duration) float64 {
	if totalDuration == 0 {
		return 0.0
	}
	ratio := float64(bufferDuration) / float64(totalDuration)
	return math.Min(1.0, math.Max(0.0, ratio))
}
