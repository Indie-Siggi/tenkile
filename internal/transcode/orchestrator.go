// SPDX-License-Identifier: AGPL-3.0-or-later

package transcode

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tenkile/tenkile/internal/probes"
	"github.com/tenkile/tenkile/internal/server"
	"github.com/tenkile/tenkile/pkg/codec"
)

// DecisionType describes the playback strategy chosen.
type DecisionType int

const (
	DecisionDirectPlay DecisionType = iota
	DecisionRemux
	DecisionTranscode
	DecisionFallback // Absolute fallback: H.264/AAC/MP4
)

func (d DecisionType) String() string {
	switch d {
	case DecisionDirectPlay:
		return "direct_play"
	case DecisionRemux:
		return "remux"
	case DecisionTranscode:
		return "transcode"
	case DecisionFallback:
		return "fallback"
	default:
		return "unknown"
	}
}

// MediaItem describes the source media to be played.
type MediaItem struct {
	ID               string
	Title            string
	VideoCodec       string
	AudioCodec       string
	Container        string
	Width            int
	Height           int
	Bitrate          int64
	Framerate        float64
	AudioChannels    int
	AudioBitrate     int64
	BitDepth         int
	IsHDR            bool
	HDRType          string // "hdr10", "hdr10+", "dolby_vision", "hlg"
	IsDolbyVision    bool
	DolbyVisionProfile string // "5", "7", "8"
	IsDolbyAtmos     bool
	Interlaced       bool
	PixelAspectRatio float64
	SubtitleFormat   string
	SubtitleIndex    int
	Duration         time.Duration
}

// PlaybackDecision is the full result of the orchestrator's analysis.
type PlaybackDecision struct {
	Type               DecisionType
	Reasons            []string // Why this decision was made

	// Source info
	SourceVideoCodec   string
	SourceAudioCodec   string
	SourceContainer    string
	SourceWidth        int
	SourceHeight       int
	SourceBitrate      int64
	SourceIsHDR        bool
	SourceInterlaced   bool
	SourcePixelAspectRatio float64

	// Target info
	TargetVideoCodec   string
	TargetAudioCodec   string
	TargetContainer    string
	TargetAudioChannels int

	// What changed
	NeedsVideoTranscode bool
	NeedsAudioTranscode bool
	NeedsRemux          bool

	// Quality preservation
	HDRPreserved            bool
	ToneMapped              bool
	BitDepthPreserved       bool
	AudioChannelsPreserved  bool

	// Server-side details
	EncoderUsed        string
	HardwareAccelUsed  bool

	// Subtitle handling
	SubtitleDecision   SubtitleDecision

	// Trust
	DeviceCapabilityTrust float64
	CapabilitySources     []string

	// Timing
	DecisionDurationMs int64
}

// Orchestrator is the central transcode decision engine.
// It replaces Jellyfin's StreamBuilder with a quality-preserving design.
type Orchestrator struct {
	inventory *server.Inventory
	matcher   *DeviceMatcher
	policy    QualityPreservationPolicy
	subConfig SubtitleConfig
	logger    *slog.Logger
}

// NewOrchestrator creates a new transcode orchestrator.
func NewOrchestrator(inv *server.Inventory, policy QualityPreservationPolicy, subConfig SubtitleConfig, logger *slog.Logger) *Orchestrator {
	if logger == nil {
		logger = slog.Default()
	}

	return &Orchestrator{
		inventory: inv,
		matcher:   NewDeviceMatcher(policy),
		policy:    policy,
		subConfig: subConfig,
		logger:    logger,
	}
}

// Decide makes a playback decision for the given media item and device.
func (o *Orchestrator) Decide(ctx context.Context, item *MediaItem, deviceCaps *probes.DeviceCapabilities) *PlaybackDecision {
	start := time.Now()

	decision := &PlaybackDecision{
		SourceVideoCodec:       item.VideoCodec,
		SourceAudioCodec:       item.AudioCodec,
		SourceContainer:        item.Container,
		SourceWidth:            item.Width,
		SourceHeight:           item.Height,
		SourceBitrate:          item.Bitrate,
		SourceIsHDR:            item.IsHDR,
		SourceInterlaced:       item.Interlaced,
		SourcePixelAspectRatio: item.PixelAspectRatio,
		DeviceCapabilityTrust:  deviceCaps.TrustScore,
	}

	// Step 1: Resolve subtitle decision FIRST (may force transcode)
	deviceSupportsSub := containsCodec(deviceCaps.SubtitleFormats, item.SubtitleFormat)
	decision.SubtitleDecision = DecideSubtitle(item.SubtitleFormat, deviceSupportsSub, o.subConfig)
	decision.SubtitleDecision.streamIndex = item.SubtitleIndex

	// Step 2: Try direct play
	if !decision.SubtitleDecision.ForcesBurnIn && o.tryDirectPlay(item, deviceCaps, decision) {
		decision.DecisionDurationMs = time.Since(start).Milliseconds()
		return decision
	}

	// Step 3: Try remux (same codecs, different container)
	if !decision.SubtitleDecision.ForcesBurnIn && o.tryRemux(item, deviceCaps, decision) {
		decision.DecisionDurationMs = time.Since(start).Milliseconds()
		return decision
	}

	// Step 4: Transcode — walk codec ladders
	o.buildTranscode(item, deviceCaps, decision)
	decision.DecisionDurationMs = time.Since(start).Milliseconds()
	return decision
}

func (o *Orchestrator) tryDirectPlay(item *MediaItem, caps *probes.DeviceCapabilities, decision *PlaybackDecision) bool {
	if !o.matcher.CanDirectPlayVideo(caps, item) {
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("video codec %s not direct-playable (trust=%.2f)", item.VideoCodec, caps.TrustScore))
		return false
	}
	if !o.matcher.CanDirectPlayAudio(caps, item) {
		// Video is OK but audio isn't — this is a partial transcode, not direct play.
		// Still, we set video as copy and only transcode audio.
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("audio codec %s not supported", item.AudioCodec))
		return false
	}
	if !o.matcher.CanDirectPlayContainer(caps, item.Container) {
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("container %s not supported", item.Container))
		return false
	}

	// HDR check
	if item.IsHDR {
		switch o.policy.HdrPolicy {
		case HdrAlwaysToneMap:
			decision.Reasons = append(decision.Reasons, "HDR always tone-map policy")
			return false
		case HdrBestForDevice:
			if !caps.SupportsHDR {
				decision.Reasons = append(decision.Reasons, "device does not support HDR, need tone-map")
				return false
			}
		}
	}

	decision.Type = DecisionDirectPlay
	decision.TargetVideoCodec = item.VideoCodec
	decision.TargetAudioCodec = item.AudioCodec
	decision.TargetContainer = item.Container
	decision.TargetAudioChannels = item.AudioChannels
	decision.HDRPreserved = item.IsHDR
	decision.BitDepthPreserved = true
	decision.AudioChannelsPreserved = true
	decision.Reasons = append(decision.Reasons, "all codecs and container compatible")
	return true
}

func (o *Orchestrator) tryRemux(item *MediaItem, caps *probes.DeviceCapabilities, decision *PlaybackDecision) bool {
	if !o.matcher.CanDirectPlayVideo(caps, item) {
		return false
	}
	if !o.matcher.CanDirectPlayAudio(caps, item) {
		return false
	}

	// HDR check for remux
	if item.IsHDR && o.policy.HdrPolicy == HdrAlwaysToneMap {
		return false
	}
	if item.IsHDR && o.policy.HdrPolicy == HdrBestForDevice && !caps.SupportsHDR {
		return false
	}

	// Find a compatible container
	targetContainer := o.findCompatibleContainer(caps, item.VideoCodec, item.AudioCodec)
	if targetContainer == "" {
		return false
	}

	decision.Type = DecisionRemux
	decision.NeedsRemux = true
	decision.TargetVideoCodec = item.VideoCodec
	decision.TargetAudioCodec = item.AudioCodec
	decision.TargetContainer = targetContainer
	decision.TargetAudioChannels = item.AudioChannels
	decision.HDRPreserved = item.IsHDR
	decision.BitDepthPreserved = true
	decision.AudioChannelsPreserved = true
	decision.Reasons = append(decision.Reasons, fmt.Sprintf("remux from %s to %s", item.Container, targetContainer))
	return true
}

func (o *Orchestrator) buildTranscode(item *MediaItem, caps *probes.DeviceCapabilities, decision *PlaybackDecision) {
	serverCaps := o.inventory.GetCurrent()
	if serverCaps == nil {
		o.absoluteFallback(decision, "server capabilities not available")
		return
	}
	selector := server.NewEncoderSelector(serverCaps)

	// Determine video target
	videoCodec, videoReasons := o.selectVideoTarget(item, caps, selector)
	decision.Reasons = append(decision.Reasons, videoReasons...)

	// Determine if we need HDR handling
	needsHDR := false
	needsToneMap := false
	if item.IsHDR {
		switch o.policy.HdrPolicy {
		case HdrBestForDevice:
			if caps.SupportsHDR {
				needsHDR = true
			} else {
				needsToneMap = true
			}
		case HdrAlwaysToneMap:
			needsToneMap = true
		case HdrNeverToneMap:
			needsHDR = true
		}
	}

	// Handle Dolby Vision -> HDR10 fallback
	if item.IsDolbyVision && !caps.SupportsDolbyVision {
		if item.DolbyVisionProfile == "7" || item.DolbyVisionProfile == "8" {
			// DV Profile 7/8 has HDR10 base layer — can extract without re-encode
			decision.Reasons = append(decision.Reasons, fmt.Sprintf("DolbyVision P%s -> HDR10 base layer fallback", item.DolbyVisionProfile))
			if !caps.SupportsHDR {
				needsToneMap = true
				needsHDR = false
			}
		}
	}

	// Check if source video codec is directly playable (only need audio transcode)
	if o.matcher.CanDirectPlayVideo(caps, item) && !needsToneMap && !decision.SubtitleDecision.ForcesBurnIn {
		decision.NeedsVideoTranscode = false
		decision.TargetVideoCodec = item.VideoCodec
		decision.HDRPreserved = item.IsHDR
		decision.BitDepthPreserved = true
	} else {
		decision.NeedsVideoTranscode = true
		decision.TargetVideoCodec = videoCodec

		// Select encoder
		encoder := selector.SelectEncoder(videoCodec, needsHDR, needsHDR)
		if encoder != nil {
			decision.EncoderUsed = encoder.EncoderName
			decision.HardwareAccelUsed = encoder.IsHardware

			// Verify the selected encoder actually supports HDR if we requested it.
			// SelectEncoder may have fallen back to a non-HDR encoder.
			if needsHDR && !encoder.SupportsHDRPassthrough {
				needsHDR = false
				needsToneMap = true
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("encoder %s lacks HDR passthrough, falling back to tone mapping", encoder.EncoderName))
			}
		}

		if needsHDR {
			decision.HDRPreserved = true
			decision.BitDepthPreserved = true
			decision.Reasons = append(decision.Reasons, "HDR preserved (device has HDR display)")
		} else if needsToneMap {
			decision.ToneMapped = true
			decision.HDRPreserved = false
			decision.BitDepthPreserved = false
			decision.Reasons = append(decision.Reasons, "HDR -> SDR tone mapping (no HDR display)")
		} else {
			decision.BitDepthPreserved = true
		}
	}

	// Determine audio target
	audioCodec, audioChannels, audioReasons := o.selectAudioTarget(item, caps, selector)
	decision.Reasons = append(decision.Reasons, audioReasons...)

	if codec.Equal(audioCodec, item.AudioCodec) && audioChannels >= item.AudioChannels {
		decision.NeedsAudioTranscode = false
		decision.TargetAudioCodec = item.AudioCodec
		decision.TargetAudioChannels = item.AudioChannels
		decision.AudioChannelsPreserved = true
	} else {
		decision.NeedsAudioTranscode = true
		decision.TargetAudioCodec = audioCodec
		decision.TargetAudioChannels = audioChannels
		decision.AudioChannelsPreserved = audioChannels >= item.AudioChannels
	}

	// Determine container
	decision.TargetContainer = o.selectContainer(decision.TargetVideoCodec, decision.TargetAudioCodec, caps)

	// Set decision type
	if decision.NeedsVideoTranscode || decision.NeedsAudioTranscode {
		decision.Type = DecisionTranscode
	} else {
		// Only container change needed
		decision.Type = DecisionRemux
		decision.NeedsRemux = true
	}
}

func (o *Orchestrator) selectVideoTarget(item *MediaItem, caps *probes.DeviceCapabilities, selector *server.EncoderSelector) (string, []string) {
	ladder := GetVideoCodecLadder(item.VideoCodec)
	var reasons []string

	for _, candidateCodec := range ladder {
		if !o.matcher.SupportsCodec(caps, candidateCodec) {
			reasons = append(reasons, fmt.Sprintf("device doesn't support %s", candidateCodec))
			continue
		}
		if !selector.CanEncodeCodec(candidateCodec) {
			reasons = append(reasons, fmt.Sprintf("server can't encode %s", candidateCodec))
			continue
		}
		reasons = append(reasons, fmt.Sprintf("selected video codec %s", candidateCodec))
		return candidateCodec, reasons
	}

	// Absolute fallback: H.264 (if server can encode it)
	if selector.CanEncodeCodec("h264") {
		reasons = append(reasons, "fallback to h264")
		return "h264", reasons
	}

	reasons = append(reasons, "no usable video encoder found")
	return "h264", reasons
}

func (o *Orchestrator) selectAudioTarget(item *MediaItem, caps *probes.DeviceCapabilities, selector *server.EncoderSelector) (string, int, []string) {
	ladder := GetAudioCodecLadder(item.AudioCodec)
	var reasons []string

	for _, candidate := range ladder {
		if !o.matcher.SupportsAudioCodec(caps, candidate.Codec) {
			continue
		}
		if !selector.CanEncodeCodec(candidate.Codec) && !codec.Equal(candidate.Codec, item.AudioCodec) {
			continue
		}

		channels := candidate.MaxChannels
		if channels > item.AudioChannels {
			channels = item.AudioChannels
		}

		reasons = append(reasons, fmt.Sprintf("selected audio %s %dch", candidate.Codec, channels))
		return candidate.Codec, channels, reasons
	}

	// Absolute fallback
	reasons = append(reasons, "fallback to aac stereo")
	return "aac", 2, reasons
}

func (o *Orchestrator) selectContainer(videoCodec, audioCodec string, caps *probes.DeviceCapabilities) string {
	// Prefer MP4 (widest compatibility), then MKV, then TS
	preferred := []string{"mp4", "mkv", "ts", "webm"}

	for _, c := range preferred {
		if containsCodec(caps.ContainerFormats, c) &&
			codec.CanContainerContain(c, videoCodec) &&
			codec.CanContainerContain(c, audioCodec) {
			return c
		}
	}
	return "mp4" // Universal fallback
}

func (o *Orchestrator) findCompatibleContainer(caps *probes.DeviceCapabilities, videoCodec, audioCodec string) string {
	for _, c := range caps.ContainerFormats {
		if codec.CanContainerContain(c, videoCodec) && codec.CanContainerContain(c, audioCodec) {
			return c
		}
	}
	return ""
}

func (o *Orchestrator) absoluteFallback(decision *PlaybackDecision, reason string) {
	decision.Type = DecisionFallback
	decision.NeedsVideoTranscode = true
	decision.NeedsAudioTranscode = true
	decision.TargetVideoCodec = "h264"
	decision.TargetAudioCodec = "aac"
	decision.TargetContainer = "mp4"
	decision.TargetAudioChannels = 2
	decision.Reasons = append(decision.Reasons, "absolute fallback: "+reason)
}
