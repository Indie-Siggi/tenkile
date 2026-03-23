// SPDX-License-Identifier: AGPL-3.0-or-later

package transcode

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// PlaybackDecisionLog is the full audit record for a playback decision.
type PlaybackDecisionLog struct {
	ID                    string        `json:"id"`
	Timestamp             time.Time     `json:"timestamp"`
	DeviceID              string        `json:"device_id"`
	MediaItemID           string        `json:"media_item_id"`

	// Decision
	DecisionType          string        `json:"decision_type"`
	Reasons               []string      `json:"reasons"`

	// Source
	SourceVideoCodec      string        `json:"source_video_codec"`
	SourceAudioCodec      string        `json:"source_audio_codec"`
	SourceContainer       string        `json:"source_container"`
	SourceWidth           int           `json:"source_width"`
	SourceHeight          int           `json:"source_height"`
	SourceBitrate         int64         `json:"source_bitrate"`

	// Target
	TargetVideoCodec      string        `json:"target_video_codec"`
	TargetAudioCodec      string        `json:"target_audio_codec"`
	TargetContainer       string        `json:"target_container"`

	// Quality
	HDRPreserved          bool          `json:"hdr_preserved"`
	ToneMapped            bool          `json:"tone_mapped"`
	BitDepthPreserved     bool          `json:"bit_depth_preserved"`
	AudioChannelsPreserved bool         `json:"audio_channels_preserved"`

	// Server
	EncoderUsed           string        `json:"encoder_used"`
	HardwareAccelUsed     bool          `json:"hardware_accel_used"`

	// Trust
	DeviceCapabilityTrust float64       `json:"device_capability_trust"`
	CapabilitySources     []string      `json:"capability_sources,omitempty"`

	// Timing
	DecisionDurationMs    int64         `json:"decision_duration_ms"`

	// Outcome (updated later via playback feedback)
	PlaybackSucceeded     *bool         `json:"playback_succeeded,omitempty"`
	FailureReason         string        `json:"failure_reason,omitempty"`
}

// DecisionStats holds aggregate statistics about playback decisions.
type DecisionStats struct {
	TotalDecisions      int     `json:"total_decisions"`
	DirectPlayCount     int     `json:"direct_play_count"`
	DirectPlayPercent   float64 `json:"direct_play_percent"`
	RemuxCount          int     `json:"remux_count"`
	RemuxPercent        float64 `json:"remux_percent"`
	TranscodeCount      int     `json:"transcode_count"`
	TranscodePercent    float64 `json:"transcode_percent"`
	FallbackCount       int     `json:"fallback_count"`
	FallbackPercent     float64 `json:"fallback_percent"`
	HDRPreservedPercent float64 `json:"hdr_preserved_percent"`
	ToneMappedPercent   float64 `json:"tone_mapped_percent"`
	AvgTrust            float64 `json:"avg_trust"`
	AvgDecisionMs       float64 `json:"avg_decision_ms"`
	HWAccelPercent      float64 `json:"hw_accel_percent"`
	SuccessRate         float64 `json:"success_rate"`
	FailureRate         float64 `json:"failure_rate"`
}

// DecisionQuery filters for querying decision logs.
type DecisionQuery struct {
	DeviceID string
	From     time.Time
	To       time.Time
	Limit    int
	Offset   int
}

// DecisionLogger logs playback decisions and maintains an in-memory store.
type DecisionLogger struct {
	mu      sync.RWMutex
	entries []*PlaybackDecisionLog
	maxSize int
	idSeq   atomic.Int64
	logger  *slog.Logger
}

// NewDecisionLogger creates a decision logger.
func NewDecisionLogger(logger *slog.Logger) *DecisionLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &DecisionLogger{
		entries: make([]*PlaybackDecisionLog, 0, 1000),
		maxSize: 10000, // Keep last 10K decisions in memory
		logger:  logger,
	}
}

// Log records a playback decision and stores it.
func (dl *DecisionLogger) Log(_ context.Context, decision *PlaybackDecision, deviceID, mediaItemID string) *PlaybackDecisionLog {
	logEntry := &PlaybackDecisionLog{
		ID:                     dl.nextID(),
		Timestamp:              time.Now(),
		DeviceID:               deviceID,
		MediaItemID:            mediaItemID,
		DecisionType:           decision.Type.String(),
		Reasons:                decision.Reasons,
		SourceVideoCodec:       decision.SourceVideoCodec,
		SourceAudioCodec:       decision.SourceAudioCodec,
		SourceContainer:        decision.SourceContainer,
		SourceWidth:            decision.SourceWidth,
		SourceHeight:           decision.SourceHeight,
		SourceBitrate:          decision.SourceBitrate,
		TargetVideoCodec:       decision.TargetVideoCodec,
		TargetAudioCodec:       decision.TargetAudioCodec,
		TargetContainer:        decision.TargetContainer,
		HDRPreserved:           decision.HDRPreserved,
		ToneMapped:             decision.ToneMapped,
		BitDepthPreserved:      decision.BitDepthPreserved,
		AudioChannelsPreserved: decision.AudioChannelsPreserved,
		EncoderUsed:            decision.EncoderUsed,
		HardwareAccelUsed:      decision.HardwareAccelUsed,
		DeviceCapabilityTrust:  decision.DeviceCapabilityTrust,
		CapabilitySources:      decision.CapabilitySources,
		DecisionDurationMs:     decision.DecisionDurationMs,
	}

	dl.store(logEntry)

	dl.logger.Info("playback decision",
		"id", logEntry.ID,
		"type", logEntry.DecisionType,
		"device", deviceID,
		"media", mediaItemID,
		"source_video", logEntry.SourceVideoCodec,
		"target_video", logEntry.TargetVideoCodec,
		"source_audio", logEntry.SourceAudioCodec,
		"target_audio", logEntry.TargetAudioCodec,
		"hdr_preserved", logEntry.HDRPreserved,
		"tone_mapped", logEntry.ToneMapped,
		"encoder", logEntry.EncoderUsed,
		"hw_accel", logEntry.HardwareAccelUsed,
		"trust", logEntry.DeviceCapabilityTrust,
		"duration_ms", logEntry.DecisionDurationMs,
	)

	return logEntry
}

// UpdateOutcome updates a decision log with playback outcome.
func (dl *DecisionLogger) UpdateOutcome(id string, succeeded bool, failureReason string) bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	for _, entry := range dl.entries {
		if entry.ID == id {
			entry.PlaybackSucceeded = &succeeded
			entry.FailureReason = failureReason
			return true
		}
	}
	return false
}

// Query returns decision logs matching the given filter.
func (dl *DecisionLogger) Query(q DecisionQuery) []*PlaybackDecisionLog {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	var results []*PlaybackDecisionLog

	for i := len(dl.entries) - 1; i >= 0; i-- {
		entry := dl.entries[i]

		if q.DeviceID != "" && entry.DeviceID != q.DeviceID {
			continue
		}
		if !q.From.IsZero() && entry.Timestamp.Before(q.From) {
			continue
		}
		if !q.To.IsZero() && entry.Timestamp.After(q.To) {
			continue
		}

		results = append(results, entry)
	}

	// Apply offset and limit
	if q.Offset > 0 && q.Offset < len(results) {
		results = results[q.Offset:]
	} else if q.Offset >= len(results) {
		return nil
	}

	if q.Limit > 0 && q.Limit < len(results) {
		results = results[:q.Limit]
	}

	return results
}

// GetByID returns a single decision log by ID.
func (dl *DecisionLogger) GetByID(id string) (*PlaybackDecisionLog, bool) {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	for _, entry := range dl.entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return nil, false
}

// Stats computes aggregate statistics over all stored decisions.
func (dl *DecisionLogger) Stats() *DecisionStats {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	stats := &DecisionStats{}
	total := len(dl.entries)
	if total == 0 {
		return stats
	}

	stats.TotalDecisions = total

	var (
		trustSum     float64
		durationSum  int64
		hdrCount     int
		toneMapCount int
		hwAccelCount int
		successCount int
		failCount    int
		feedbackCount int
	)

	for _, e := range dl.entries {
		switch e.DecisionType {
		case "direct_play":
			stats.DirectPlayCount++
		case "remux":
			stats.RemuxCount++
		case "transcode":
			stats.TranscodeCount++
		case "fallback":
			stats.FallbackCount++
		}

		trustSum += e.DeviceCapabilityTrust
		durationSum += e.DecisionDurationMs

		if e.HDRPreserved {
			hdrCount++
		}
		if e.ToneMapped {
			toneMapCount++
		}
		if e.HardwareAccelUsed {
			hwAccelCount++
		}
		if e.PlaybackSucceeded != nil {
			feedbackCount++
			if *e.PlaybackSucceeded {
				successCount++
			} else {
				failCount++
			}
		}
	}

	ft := float64(total)
	stats.DirectPlayPercent = float64(stats.DirectPlayCount) / ft * 100
	stats.RemuxPercent = float64(stats.RemuxCount) / ft * 100
	stats.TranscodePercent = float64(stats.TranscodeCount) / ft * 100
	stats.FallbackPercent = float64(stats.FallbackCount) / ft * 100
	stats.HDRPreservedPercent = float64(hdrCount) / ft * 100
	stats.ToneMappedPercent = float64(toneMapCount) / ft * 100
	stats.AvgTrust = trustSum / ft
	stats.AvgDecisionMs = float64(durationSum) / ft
	stats.HWAccelPercent = float64(hwAccelCount) / ft * 100

	if feedbackCount > 0 {
		stats.SuccessRate = float64(successCount) / float64(feedbackCount) * 100
		stats.FailureRate = float64(failCount) / float64(feedbackCount) * 100
	}

	return stats
}

func (dl *DecisionLogger) store(entry *PlaybackDecisionLog) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	dl.entries = append(dl.entries, entry)

	// Evict oldest entries if over capacity
	if len(dl.entries) > dl.maxSize {
		dl.entries = dl.entries[len(dl.entries)-dl.maxSize:]
	}
}

func (dl *DecisionLogger) nextID() string {
	seq := dl.idSeq.Add(1)
	return fmt.Sprintf("dec-%d-%d", time.Now().UnixMilli(), seq)
}
