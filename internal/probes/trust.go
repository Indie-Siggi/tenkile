// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// TrustLevel represents the trust level of a device
type TrustLevel int

const (
	TrustLevelUnknown TrustLevel = iota
	TrustLevelUntrusted
	TrustLevelLow
	TrustLevelMedium
	TrustLevelHigh
	TrustLevelTrusted
	TrustLevelVerified
)

// String returns the string representation of a trust level
func (t TrustLevel) String() string {
	switch t {
	case TrustLevelUnknown:
		return "unknown"
	case TrustLevelUntrusted:
		return "untrusted"
	case TrustLevelLow:
		return "low"
	case TrustLevelMedium:
		return "medium"
	case TrustLevelHigh:
		return "high"
	case TrustLevelTrusted:
		return "trusted"
	case TrustLevelVerified:
		return "verified"
	default:
		return "unknown"
	}
}

// TrustScore represents a calculated trust score for a device
type TrustScore struct {
	DeviceID      string    `json:"device_id"`
	Score         float64   `json:"score"`         // 0.0 to 1.0
	Level         TrustLevel `json:"level"`
	Components    []TrustComponent `json:"components"`
	LastCalculated time.Time `json:"last_calculated"`
	Reason        string    `json:"reason,omitempty"`
}

// TrustComponent represents a single component of the trust score
type TrustComponent struct {
	Name   string  `json:"name"`
	Score  float64 `json:"score"`  // 0.0 to 1.0
	Weight float64 `json:"weight"` // 0.0 to 1.0
	Reason string  `json:"reason,omitempty"`
}

// TrustResolver calculates and manages device trust scores
type TrustResolver struct {
	mu sync.RWMutex

	// Configuration
	MaxScore           float64
	MinScore           float64
	DecayRate          float64 // Per hour
	VerificationBonus  float64
	CurationBonus      float64
	ConsistencyBonus   float64

	// Cache for computed scores
	scoreCache map[string]*TrustScore
	cacheTTL   time.Duration

	// Known device fingerprints
	knownDevices map[string]bool
}

// NewTrustResolver creates a new TrustResolver instance
func NewTrustResolver() *TrustResolver {
	return &TrustResolver{
		MaxScore:          1.0,
		MinScore:          0.0,
		DecayRate:         0.01, // 1% per hour
		VerificationBonus: 0.2,
		CurationBonus:     0.15,
		ConsistencyBonus:  0.1,
		scoreCache:        make(map[string]*TrustScore),
		cacheTTL:          time.Hour * 24,
		knownDevices:      make(map[string]bool),
	}
}

// DeviceTrustData contains all data needed to calculate trust
type DeviceTrustData struct {
	DeviceID        string            `json:"device_id"`
	UserAgent       string            `json:"user_agent"`
	Platform        string            `json:"platform"`
	OSVersion       string            `json:"os_version"`
	AppVersion      string            `json:"app_version"`
	Model           string            `json:"model"`
	Manufacturer    string            `json:"manufacturer"`

	// Capability data
	VideoCodecs     []string          `json:"video_codecs"`
	AudioCodecs     []string          `json:"audio_codecs"`
	ContainerFormats []string         `json:"container_formats"`
	DRMSupport      *DRMSupported    `json:"drm_support"`

	// Behavioral data
	FirstSeen       time.Time         `json:"first_seen"`
	LastSeen        time.Time         `json:"last_seen"`
	ProbeCount      int               `json:"probe_count"`
	ConsistentProbes int              `json:"consistent_probes"`
	AnomalyCount    int               `json:"anomaly_count"`

	// Verification data
	IsVerified      bool              `json:"is_verified"`
	IsCurated       bool              `json:"is_curated"`
	Source          string            `json:"source"` // "community", "official", "curated"
	VotesUp         int               `json:"votes_up"`
	VotesDown       int               `json:"votes_down"`
}

// CalculateTrustScore calculates the trust score for a device
func (tr *TrustResolver) CalculateTrustScore(data *DeviceTrustData) *TrustScore {
	if data == nil {
		return &TrustScore{
			Score:  0.0,
			Level:  TrustLevelUntrusted,
			Reason: "No device data provided",
		}
	}

	components := []TrustComponent{}
	totalScore := 0.0
	totalWeight := 0.0

	// Component 1: Device fingerprint recognition (weight: 0.25)
	fingerprintScore, fingerprintReason := tr.evaluateFingerprint(data)
	fingerprintWeight := 0.25
	components = append(components, TrustComponent{
		Name:   "fingerprint",
		Score:  fingerprintScore,
		Weight: fingerprintWeight,
		Reason: fingerprintReason,
	})
	totalScore += fingerprintScore * fingerprintWeight
	totalWeight += fingerprintWeight

	// Component 2: Capability consistency (weight: 0.20)
	consistencyScore, consistencyReason := tr.evaluateConsistency(data)
	consistencyWeight := 0.20
	components = append(components, TrustComponent{
		Name:   "consistency",
		Score:  consistencyScore,
		Weight: consistencyWeight,
		Reason: consistencyReason,
	})
	totalScore += consistencyScore * consistencyWeight
	totalWeight += consistencyWeight

	// Component 3: DRM support validation (weight: 0.15)
	drmScore, drmReason := tr.evaluateDRMSupport(data)
	drmWeight := 0.15
	components = append(components, TrustComponent{
		Name:   "drm_support",
		Score:  drmScore,
		Weight: drmWeight,
		Reason: drmReason,
	})
	totalScore += drmScore * drmWeight
	totalWeight += drmWeight

	// Component 4: Platform validity (weight: 0.15)
	platformScore, platformReason := tr.evaluatePlatform(data)
	platformWeight := 0.15
	components = append(components, TrustComponent{
		Name:   "platform",
		Score:  platformScore,
		Weight: platformWeight,
		Reason: platformReason,
	})
	totalScore += platformScore * platformWeight
	totalWeight += platformWeight

	// Component 5: Behavioral analysis (weight: 0.15)
	behaviorScore, behaviorReason := tr.evaluateBehavior(data)
	behaviorWeight := 0.15
	components = append(components, TrustComponent{
		Name:   "behavior",
		Score:  behaviorScore,
		Weight: behaviorWeight,
		Reason: behaviorReason,
	})
	totalScore += behaviorScore * behaviorWeight
	totalWeight += behaviorWeight

	// Apply bonuses as weighted components before normalization
	reason := "Base trust score"
	if data.IsVerified {
		bonusWeight := 0.10
		totalScore += 1.0 * bonusWeight // Full score for verified bonus
		totalWeight += bonusWeight
		reason += "; verified device bonus"
	}
	if data.IsCurated {
		bonusWeight := 0.10
		totalScore += 1.0 * bonusWeight // Full score for curated bonus
		totalWeight += bonusWeight
		reason += "; curated device bonus"
	}

	// Normalize score to 0-1 range
	if totalWeight > 0 {
		totalScore = totalScore / totalWeight
	}

	// Clamp to valid range
	totalScore = clamp(totalScore, tr.MinScore, tr.MaxScore)

	// Determine trust level
	level := tr.scoreToLevel(totalScore)

	score := &TrustScore{
		DeviceID:      data.DeviceID,
		Score:         totalScore,
		Level:         level,
		Components:    components,
		LastCalculated: time.Now(),
		Reason:        reason,
	}

	// Cache the result
	tr.cacheScore(score)

	return score
}

// evaluateFingerprint checks if device fingerprint is recognized
func (tr *TrustResolver) evaluateFingerprint(data *DeviceTrustData) (float64, string) {
	fingerprint := tr.generateFingerprint(data)

	// Check if device is known
	tr.mu.RLock()
	known := tr.knownDevices[fingerprint]
	tr.mu.RUnlock()
	if known {
		return 0.8, "Recognized device fingerprint"
	}

	// Check if platform/manufacturer combo is valid
	validPlatforms := map[string]bool{
		"android": true, "ios": true, "tvos": true,
		"web": true, "chromecast": true, "roku": true,
		"firetv": true, "apple_tv": true, "xbox": true,
		"playstation": true, "nintendo": true, "windows": true,
		"macos": true, "linux": true,
	}

	if validPlatforms[data.Platform] {
		if data.Manufacturer != "" {
			return 0.6, "Valid platform with manufacturer"
		}
		return 0.5, "Valid platform"
	}

	return 0.3, "Unknown or invalid platform"
}

// evaluateConsistency checks capability report consistency
func (tr *TrustResolver) evaluateConsistency(data *DeviceTrustData) (float64, string) {
	if data.ProbeCount == 0 {
		return 0.5, "No probe history"
	}

	if data.AnomalyCount > 0 {
		anomalyRatio := float64(data.AnomalyCount) / float64(data.ProbeCount)
		if anomalyRatio > 0.5 {
			return 0.2, fmt.Sprintf("High anomaly rate: %.0f%%", anomalyRatio*100)
		}
		if anomalyRatio > 0.2 {
			return 0.5, fmt.Sprintf("Moderate anomaly rate: %.0f%%", anomalyRatio*100)
		}
	}

	if data.ConsistentProbes >= data.ProbeCount {
		return 1.0, "All probes consistent"
	}

	if data.ProbeCount > 10 && data.ConsistentProbes >= int(float64(data.ProbeCount)*0.8) {
		return 0.9, "High consistency across probes"
	}

	if data.ProbeCount > 5 && data.ConsistentProbes >= int(float64(data.ProbeCount)*0.6) {
		return 0.7, "Good consistency"
	}

	return 0.5, "Limited probe history"
}

// evaluateDRMSupport validates DRM support claims
func (tr *TrustResolver) evaluateDRMSupport(data *DeviceTrustData) (float64, string) {
	if data.DRMSupport == nil {
		return 0.5, "No DRM information available"
	}

	if !data.DRMSupport.Supported {
		// Some platforms legitimately don't support DRM
		if data.Platform == "web" || data.Platform == "linux" {
			return 0.7, "No DRM (expected for platform)"
		}
		return 0.3, "No DRM support"
	}

	// Check for valid DRM systems
	validSystems := map[string]bool{
		"widevine": true, "playready": true, "fairplay": true,
	}

	validCount := 0
	for _, detail := range data.DRMSupport.Details {
		if detail.Supported && validSystems[strings.ToLower(detail.System)] {
			validCount++
		}
	}

	if validCount >= 2 {
		return 1.0, fmt.Sprintf("Multiple valid DRM systems: %d", validCount)
	}

	if validCount == 1 {
		return 0.8, "Single valid DRM system"
	}

	return 0.5, "Limited DRM support"
}

// evaluatePlatform checks platform validity
func (tr *TrustResolver) evaluatePlatform(data *DeviceTrustData) (float64, string) {
	validPlatforms := map[string]bool{
		"android": true, "ios": true, "tvos": true,
		"web": true, "chromecast": true, "roku": true,
		"firetv": true, "apple_tv": true, "xbox": true,
		"playstation": true, "nintendo": true, "windows": true,
		"macos": true, "linux": true,
	}

	if !validPlatforms[data.Platform] {
		return 0.2, fmt.Sprintf("Unknown platform: %s", data.Platform)
	}

	// Check for reasonable version format
	if data.OSVersion != "" && len(data.OSVersion) < 3 {
		return 0.6, "Platform valid but version suspicious"
	}

	if data.AppVersion != "" && len(data.AppVersion) < 3 {
		return 0.7, "Platform valid but app version suspicious"
	}

	return 0.9, "Valid platform with versions"
}

// evaluateBehavior analyzes behavioral patterns
func (tr *TrustResolver) evaluateBehavior(data *DeviceTrustData) (float64, string) {
	now := time.Now()

	// Check device age
	age := now.Sub(data.FirstSeen)
	if age < time.Hour {
		return 0.3, "New device (less than 1 hour old)"
	}
	if age < 24*time.Hour {
		return 0.5, "New device (less than 1 day old)"
	}
	if age < 7*24*time.Hour {
		return 0.7, "Established device"
	}

	// Check recent activity
	lastActivity := now.Sub(data.LastSeen)
	if lastActivity > 30*24*time.Hour {
		return 0.4, "Inactive device (30+ days)"
	}
	if lastActivity > 7*24*time.Hour {
		return 0.6, "Moderately active device"
	}

	// Check probe frequency
	if data.ProbeCount > 100 {
		return 0.9, "Highly active device"
	}
	if data.ProbeCount > 20 {
		return 0.8, "Active device"
	}
	if data.ProbeCount > 5 {
		return 0.7, "Moderately active device"
	}

	return 0.5, "Limited activity"
}

// scoreToLevel converts a numeric score to a trust level
func (tr *TrustResolver) scoreToLevel(score float64) TrustLevel {
	return TrustLevelFromScore(score)
}

// TrustLevelFromScore converts a numeric trust score to a TrustLevel
func TrustLevelFromScore(score float64) TrustLevel {
	switch {
	case score >= 0.9:
		return TrustLevelVerified
	case score >= 0.8:
		return TrustLevelTrusted
	case score >= 0.7:
		return TrustLevelHigh
	case score >= 0.5:
		return TrustLevelMedium
	case score >= 0.3:
		return TrustLevelLow
	case score >= 0.1:
		return TrustLevelUntrusted
	default:
		return TrustLevelUnknown
	}
}

// generateFingerprint creates a unique fingerprint for a device
func (tr *TrustResolver) generateFingerprint(data *DeviceTrustData) string {
	h := sha256.New()
	h.Write([]byte(data.Platform))
	h.Write([]byte(data.Manufacturer))
	h.Write([]byte(data.Model))
	h.Write([]byte(data.UserAgent))
	return hex.EncodeToString(h.Sum(nil))
}

// clamp restricts a value to a range
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// cacheScore stores a trust score in the cache
func (tr *TrustResolver) cacheScore(score *TrustScore) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.scoreCache[score.DeviceID] = score
}

// GetCachedScore retrieves a cached trust score
func (tr *TrustResolver) GetCachedScore(deviceID string) (*TrustScore, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	score, ok := tr.scoreCache[deviceID]
	if !ok {
		return nil, false
	}

	// Check if cache has expired
	if time.Since(score.LastCalculated) > tr.cacheTTL {
		delete(tr.scoreCache, deviceID)
		return nil, false
	}

	return score, true
}

// InvalidateCache removes a device from the cache
func (tr *TrustResolver) InvalidateCache(deviceID string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	delete(tr.scoreCache, deviceID)
}

// CleanCache removes expired entries
func (tr *TrustResolver) CleanCache() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	now := time.Now()
	for deviceID, score := range tr.scoreCache {
		if now.Sub(score.LastCalculated) > tr.cacheTTL {
			delete(tr.scoreCache, deviceID)
		}
	}
}

// RegisterKnownDevice marks a device fingerprint as known
func (tr *TrustResolver) RegisterKnownDevice(fingerprint string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.knownDevices[fingerprint] = true
}

// GetTrustLevelForDevice returns the current trust level for a device
func (tr *TrustResolver) GetTrustLevelForDevice(deviceID string) TrustLevel {
	score, _ := tr.GetCachedScore(deviceID)
	if score == nil {
		return TrustLevelUnknown
	}
	return score.Level
}

// ValidateCapabilities checks if reported capabilities are plausible
func ValidateCapabilities(videoCodecs, audioCodecs, containers []string) (bool, []string) {
	warnings := []string{}

	// Check for impossible codec combinations
	videoCodecSet := make(map[string]bool)
	for _, c := range videoCodecs {
		videoCodecSet[strings.ToLower(c)] = true
	}

	// Some devices shouldn't have certain codecs
	// e.g., mobile devices typically don't have VC-1
	if len(videoCodecs) > 10 {
		warnings = append(warnings, "Unusually high number of video codecs reported")
	}

	// Check for common impossible combinations
	hasAV1 := videoCodecSet["av1"]
	hasVP9 := videoCodecSet["vp9"]
	_ = videoCodecSet["hevc"]
	_ = videoCodecSet["h264"]

	// AV1 and VP9 together is common, but if device claims no VP9 and has AV1, might be suspicious
	if hasAV1 && !hasVP9 && len(videoCodecs) > 2 {
		warnings = append(warnings, "AV1 without VP9 is unusual for this platform")
	}

	// Check audio codecs
	audioCodecSet := make(map[string]bool)
	for _, c := range audioCodecs {
		audioCodecSet[strings.ToLower(c)] = true
	}

	// Most devices should have at least AAC or MP3
	if !audioCodecSet["aac"] && !audioCodecSet["mp3"] {
		warnings = append(warnings, "Missing common audio codecs (AAC/MP3)")
	}

	// Check containers
	containerSet := make(map[string]bool)
	for _, c := range containers {
		containerSet[strings.ToLower(c)] = true
	}

	// Most devices should support MP4
	if !containerSet["mp4"] && !containerSet["mkv"] {
		warnings = append(warnings, "Missing common container formats")
	}

	isValid := len(warnings) == 0
	return isValid, warnings
}

// LogTrustAnalysis logs detailed trust analysis for debugging
func LogTrustAnalysis(ctx context.Context, deviceID string, score *TrustScore) {
	logger := slog.Default()
	if logger == nil {
		return
	}

	logger.InfoContext(ctx, "Trust analysis completed",
		"device_id", deviceID,
		"score", score.Score,
		"level", score.Level.String(),
		"reason", score.Reason,
	)

	for _, component := range score.Components {
		logger.DebugContext(ctx, "Trust component",
			"component", component.Name,
			"score", component.Score,
			"weight", component.Weight,
			"reason", component.Reason,
		)
	}
}

// MarshalTrustScore serializes a trust score to JSON
func MarshalTrustScore(score *TrustScore) ([]byte, error) {
	return json.Marshal(score)
}

// UnmarshalTrustScore deserializes a trust score from JSON
func UnmarshalTrustScore(data []byte) (*TrustScore, error) {
	var score TrustScore
	if err := json.Unmarshal(data, &score); err != nil {
		return nil, err
	}
	return &score, nil
}

// SortComponents sorts trust components by score descending
func (t *TrustScore) SortComponents() {
	sort.Slice(t.Components, func(i, j int) bool {
		return t.Components[i].Score > t.Components[j].Score
	})
}
