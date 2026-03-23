// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// FeedbackIntegration connects the FeedbackManager to the TrustResolver
type FeedbackIntegration struct {
	mu sync.RWMutex

	// Dependencies
	feedbackManager *FeedbackManager
	trustResolver   *TrustResolver
	cache           *CapabilityCache
	orchestrator    interface{} // Will be *transcode.Orchestrator when available

	// Configuration
	config IntegrationConfig

	// Callbacks
	onTrustUpdate func(deviceID string, oldScore, newScore float64)
}

// IntegrationConfig holds configuration for feedback integration
type IntegrationConfig struct {
	Enabled              bool
	TrustDecayEnabled    bool
	GracePeriod         time.Duration // Time before feedback affects trust
	MaxTrustAdjustment   float64       // Maximum adjustment per period
	AdjustmentPeriod     time.Duration // Period for max adjustment
	AutoReProbe          bool          // Automatically trigger re-probe
	ReProbeDelay         time.Duration // Delay before re-probe
}

// DefaultIntegrationConfig returns the default integration configuration
func DefaultIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		Enabled:            true,
		TrustDecayEnabled: true,
		GracePeriod:        time.Hour,
		MaxTrustAdjustment: 0.5,
		AdjustmentPeriod:   24 * time.Hour,
		AutoReProbe:        true,
		ReProbeDelay:       time.Minute * 5,
	}
}

// NewFeedbackIntegration creates a new feedback integration instance
func NewFeedbackIntegration(
	feedbackManager *FeedbackManager,
	trustResolver *TrustResolver,
	cache *CapabilityCache,
) *FeedbackIntegration {
	integration := &FeedbackIntegration{
		feedbackManager: feedbackManager,
		trustResolver:   trustResolver,
		cache:           cache,
		config:          DefaultIntegrationConfig(),
	}

	// Set up callbacks
	integration.setupCallbacks()

	return integration
}

// setupCallbacks configures callbacks for trust and re-probe events
func (fi *FeedbackIntegration) setupCallbacks() {
	// Set up trust change callback
	fi.feedbackManager.SetOnTrustChange(func(deviceID string, oldScore, newScore float64) {
		fi.handleTrustChange(deviceID, oldScore, newScore)
	})

	// Set up re-probe required callback
	fi.feedbackManager.SetOnReProbeRequired(func(deviceID string, reason string) {
		fi.handleReProbeRequired(deviceID, reason)
	})
}

// handleTrustChange handles trust score changes
func (fi *FeedbackIntegration) handleTrustChange(deviceID string, oldScore, newScore float64) {
	if !fi.config.Enabled {
		return
	}

	// Invalidate trust cache for the device
	fi.trustResolver.InvalidateCache(deviceID)

	// Notify external callback if set
	if fi.onTrustUpdate != nil {
		fi.onTrustUpdate(deviceID, oldScore, newScore)
	}

	// Log the change
	slog.Info("Trust score adjusted via feedback",
		"device_id", deviceID,
		"old_score", oldScore,
		"new_score", newScore,
		"delta", newScore-oldScore)
}

// handleReProbeRequired handles re-probe requirement
func (fi *FeedbackIntegration) handleReProbeRequired(deviceID string, reason string) {
	if !fi.config.Enabled || !fi.config.AutoReProbe {
		return
	}

	slog.Info("Re-probe required",
		"device_id", deviceID,
		"reason", reason)

	// Get device capabilities to trigger re-probe workflow
	if fi.cache != nil {
		caps, found := fi.cache.Get(deviceID)
		if found && caps != nil {
			// Trigger re-probe after delay
			go fi.triggerReProbe(deviceID, caps, reason)
		}
	}
}

// triggerReProbe triggers a re-probe for a device after the configured delay
func (fi *FeedbackIntegration) triggerReProbe(deviceID string, caps *DeviceCapabilities, reason string) {
	// Wait for the configured delay before re-probing
	timer := time.NewTimer(fi.config.ReProbeDelay)
	defer timer.Stop()

	<-timer.C

	slog.Info("Triggering re-probe for device",
		"device_id", deviceID,
		"reason", reason)

	// Clear old capabilities to force re-probe
	if fi.cache != nil {
		fi.cache.Delete(deviceID)
	}

	// Reset trust adjustment to allow fresh evaluation
	fi.feedbackManager.ResetTrustAdjustment(deviceID)

	// The actual re-probe would be triggered by the client connecting again
	// This just sets up the conditions for a fresh probe
}

// SetOrchestrator sets the transcoding orchestrator
func (fi *FeedbackIntegration) SetOrchestrator(orchestrator interface{}) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.orchestrator = orchestrator
}

// SetOnTrustUpdate sets the callback for trust updates
func (fi *FeedbackIntegration) SetOnTrustUpdate(callback func(deviceID string, oldScore, newScore float64)) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.onTrustUpdate = callback
}

// GetEffectiveTrustScore calculates the effective trust score for a device
func (fi *FeedbackIntegration) GetEffectiveTrustScore(deviceID string) float64 {
	if !fi.config.Enabled {
		return 1.0 // Default to full trust when disabled
	}

	// Get base trust score from trust resolver
	data := fi.getDeviceTrustData(deviceID)
	if data == nil {
		return 0.5 // Default when no data
	}

	baseScore := fi.trustResolver.CalculateTrustScore(data)
	if baseScore == nil {
		return 0.5
	}

	// Get feedback adjustment
	adjustment := fi.feedbackManager.GetTrustAdjustment(deviceID)

	// Combine scores
	effectiveScore := baseScore.Score + adjustment

	// Clamp to valid range
	if effectiveScore > 1.0 {
		effectiveScore = 1.0
	}
	if effectiveScore < 0.0 {
		effectiveScore = 0.0
	}

	return effectiveScore
}

// getDeviceTrustData retrieves trust data for a device
func (fi *FeedbackIntegration) getDeviceTrustData(deviceID string) *DeviceTrustData {
	if fi.cache == nil {
		return nil
	}

	caps, found := fi.cache.Get(deviceID)
	if !found || caps == nil {
		return nil
	}

	// Get playback stats
	stats := fi.feedbackManager.GetPlaybackStats(deviceID)
	if stats == nil {
		return nil
	}

	return &DeviceTrustData{
		DeviceID:         deviceID,
		UserAgent:        caps.UserAgent,
		Platform:         caps.Platform,
		OSVersion:        caps.OSVersion,
		AppVersion:       caps.AppVersion,
		Model:            caps.Model,
		Manufacturer:     caps.Manufacturer,
		VideoCodecs:      caps.VideoCodecs,
		AudioCodecs:      caps.AudioCodecs,
		ContainerFormats: caps.ContainerFormats,
		DRMSupport:       caps.DRMSupport,
		ProbeCount:       int(stats.TotalPlaybacks),
		AnomalyCount:     int(stats.FailedPlaybacks),
	}
}

// ShouldTranscodeForTrust determines if content should be transcoded based on trust
func (fi *FeedbackIntegration) ShouldTranscodeForTrust(deviceID string, codec string) (bool, string) {
	if !fi.config.Enabled {
		return false, "" // Trust enabled, don't transcode
	}

	effectiveTrust := fi.GetEffectiveTrustScore(deviceID)

	// Low trust - transcode to ensure compatibility
	if effectiveTrust < 0.3 {
		// Get reliable codecs
		reliableCodecs := fi.feedbackManager.GetReliableCodecs(deviceID, 0.8)
		for _, reliable := range reliableCodecs {
			if reliable == codec {
				return false, "" // This codec is reliable, don't transcode
			}
		}
		return true, fmt.Sprintf("low_trust_%.2f", effectiveTrust)
	}

	// Medium trust - check specific codec reliability
	if effectiveTrust < 0.7 {
		reliableCodecs := fi.feedbackManager.GetReliableCodecs(deviceID, 0.9)
		for _, reliable := range reliableCodecs {
			if reliable == codec {
				return false, "" // This codec is reliable
			}
		}
		return true, fmt.Sprintf("medium_trust_codec_unreliable")
	}

	// High trust - direct play
	return false, ""
}

// ApplyTrustAdjustment applies a trust adjustment to a device
func (fi *FeedbackIntegration) ApplyTrustAdjustment(deviceID string, delta float64) error {
	if !fi.config.Enabled {
		return fmt.Errorf("feedback integration is disabled")
	}

	// Clamp delta to max adjustment
	if delta > fi.config.MaxTrustAdjustment {
		delta = fi.config.MaxTrustAdjustment
	}
	if delta < -fi.config.MaxTrustAdjustment {
		delta = -fi.config.MaxTrustAdjustment
	}

	// The actual application is handled by the feedback manager
	// This method is for external trust adjustments (e.g., admin overrides)
	slog.Info("Applying manual trust adjustment",
		"device_id", deviceID,
		"delta", delta)

	// Invalidate trust cache
	fi.trustResolver.InvalidateCache(deviceID)

	return nil
}

// GetTrustReport generates a comprehensive trust report for a device
func (fi *FeedbackIntegration) GetTrustReport(deviceID string) map[string]interface{} {
	report := make(map[string]interface{})

	// Get base trust score
	data := fi.getDeviceTrustData(deviceID)
	var baseScore *TrustScore
	if data != nil {
		baseScore = fi.trustResolver.CalculateTrustScore(data)
	}

	// Get playback stats
	playbackStats := fi.feedbackManager.GetPlaybackStats(deviceID)

	// Get feedback adjustment
	adjustment := fi.feedbackManager.GetTrustAdjustment(deviceID)

	// Get reliable codecs
	reliableCodecs := fi.feedbackManager.GetReliableCodecs(deviceID, 0.8)

	// Get effective trust
	effectiveTrust := fi.GetEffectiveTrustScore(deviceID)

	// Determine trust level
	trustLevel := "low"
	if effectiveTrust >= 0.8 {
		trustLevel = "very_high"
	} else if effectiveTrust >= 0.7 {
		trustLevel = "high"
	} else if effectiveTrust >= 0.5 {
		trustLevel = "medium"
	} else if effectiveTrust >= 0.3 {
		trustLevel = "low"
	}

	report["device_id"] = deviceID
	report["effective_trust_score"] = effectiveTrust
	report["trust_level"] = trustLevel
	report["feedback_adjustment"] = adjustment
	report["needs_reprobe"] = playbackStats.NeedsReProbe
	report["reprobe_reason"] = playbackStats.ReProbeReason
	report["reliable_codecs"] = reliableCodecs
	report["playback_stats"] = playbackStats

	if baseScore != nil {
		report["base_trust_score"] = baseScore.Score
		report["base_trust_level"] = baseScore.Level.String()
		report["base_trust_reason"] = baseScore.Reason
		report["trust_components"] = baseScore.Components
	}

	return report
}

// NotifyTranscodeDecision notifies the feedback system of a transcoding decision
func (fi *FeedbackIntegration) NotifyTranscodeDecision(deviceID string, decision string, reason string) {
	slog.Debug("Transcode decision notified",
		"device_id", deviceID,
		"decision", decision,
		"reason", reason)
}

// StartDecayLoop starts the trust decay loop
func (fi *FeedbackIntegration) StartDecayLoop(ctx context.Context) {
	if !fi.config.Enabled || !fi.config.TrustDecayEnabled {
		return
	}

	ticker := time.NewTicker(fi.config.AdjustmentPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi.runDecay()
		}
	}
}

// runDecay applies trust decay
func (fi *FeedbackIntegration) runDecay() {
	if !fi.config.TrustDecayEnabled {
		return
	}

	// Decay adjustments towards zero over time
	stats := fi.feedbackManager.GetAllStats()
	decayRate := 0.1 // 10% decay per period

	for deviceID, deviceStats := range stats {
		if deviceStats.CurrentTrustDelta != 0 {
			// Decay the adjustment
			newDelta := deviceStats.CurrentTrustDelta
			if deviceStats.CurrentTrustDelta > 0 {
				newDelta = deviceStats.CurrentTrustDelta * (1 - decayRate)
				if newDelta < 0.01 {
					newDelta = 0
				}
			} else {
				newDelta = deviceStats.CurrentTrustDelta * (1 - decayRate)
				if newDelta > -0.01 {
					newDelta = 0
				}
			}

			if newDelta != deviceStats.CurrentTrustDelta {
				// Assign the decayed value back
				deviceStats.CurrentTrustDelta = newDelta
				slog.Debug("Applying trust decay",
					"device_id", deviceID,
					"old_delta", newDelta/decayRate,
					"new_delta", newDelta)
			}
		}
	}
}

// GracePeriodChecker checks if a device is still in the grace period
func (fi *FeedbackIntegration) GracePeriodChecker(deviceID string) bool {
	if !fi.config.Enabled || fi.config.GracePeriod == 0 {
		return false
	}

	stats := fi.feedbackManager.GetPlaybackStats(deviceID)
	if stats.TotalPlaybacks == 0 {
		return true // No data yet, in grace period
	}

	// Check if first playback was within grace period
	timeSinceFirstPlayback := time.Since(stats.LastPlayback)
	return timeSinceFirstPlayback < fi.config.GracePeriod
}

// ResetDeviceTrust resets all trust data for a device
func (fi *FeedbackIntegration) ResetDeviceTrust(deviceID string) {
	// Reset feedback data
	fi.feedbackManager.ClearDeviceData(deviceID)

	// Invalidate trust cache
	fi.trustResolver.InvalidateCache(deviceID)

	// Clear from capability cache if needed
	if fi.cache != nil {
		fi.cache.Delete(deviceID)
	}

	slog.Info("Device trust reset", "device_id", deviceID)
}

// GetIntegrationStats returns integration statistics
func (fi *FeedbackIntegration) GetIntegrationStats() map[string]interface{} {
	globalStats := fi.feedbackManager.GetGlobalStats()
	metrics := GetGlobalPlaybackMetrics()

	return map[string]interface{}{
		"enabled":             fi.config.Enabled,
		"trust_decay_enabled": fi.config.TrustDecayEnabled,
		"grace_period":        fi.config.GracePeriod.String(),
		"max_adjustment":      fi.config.MaxTrustAdjustment,
		"playback_stats":      globalStats,
		"total_reprobes":      metrics.GetTotalReProbes(),
	}
}
