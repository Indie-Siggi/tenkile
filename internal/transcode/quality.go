// SPDX-License-Identifier: AGPL-3.0-or-later

package transcode

// HdrPolicy controls HDR handling behavior.
type HdrPolicy int

const (
	HdrBestForDevice HdrPolicy = iota // Preserve on HDR display, tone-map on SDR
	HdrAlwaysToneMap                  // Always tone-map to SDR
	HdrNeverToneMap                   // Never tone-map (pass through as-is)
)

func (p HdrPolicy) String() string {
	switch p {
	case HdrBestForDevice:
		return "best_for_device"
	case HdrAlwaysToneMap:
		return "always_tonemap"
	case HdrNeverToneMap:
		return "never_tonemap"
	default:
		return "unknown"
	}
}

// ToneMappingQuality controls the tone mapping algorithm.
type ToneMappingQuality int

const (
	ToneMappingHigh   ToneMappingQuality = iota // libplacebo (best quality)
	ToneMappingMedium                            // zscale/hable
	ToneMappingFast                              // reinhard (fastest)
)

func (q ToneMappingQuality) String() string {
	switch q {
	case ToneMappingHigh:
		return "high"
	case ToneMappingMedium:
		return "medium"
	case ToneMappingFast:
		return "fast"
	default:
		return "unknown"
	}
}

// AudioChannelPolicy controls audio downmixing behavior.
type AudioChannelPolicy int

const (
	AudioPreserveChannels AudioChannelPolicy = iota // Keep original channel count
	AudioAllowDownmix                               // Allow downmix for bandwidth
)

// BitDepthPolicy controls bit depth handling.
type BitDepthPolicy int

const (
	BitDepthPreserveWhenPossible BitDepthPolicy = iota
	BitDepthAllow8Bit
)

// ResolutionPolicy controls resolution handling.
type ResolutionPolicy int

const (
	ResolutionMaintainOriginal   ResolutionPolicy = iota
	ResolutionAllowBandwidthReduce
)

// QualityPreservationPolicy configures quality preservation rules.
type QualityPreservationPolicy struct {
	HdrPolicy          HdrPolicy
	ToneMappingQuality ToneMappingQuality
	AudioChannelPolicy AudioChannelPolicy
	BitDepthPolicy     BitDepthPolicy
	ResolutionPolicy   ResolutionPolicy
	MinTrustForDirectPlay float64 // Default 0.6
}

// DefaultQualityPolicy returns a sensible default quality policy.
func DefaultQualityPolicy() QualityPreservationPolicy {
	return QualityPreservationPolicy{
		HdrPolicy:             HdrBestForDevice,
		ToneMappingQuality:    ToneMappingHigh,
		AudioChannelPolicy:    AudioPreserveChannels,
		BitDepthPolicy:        BitDepthPreserveWhenPossible,
		ResolutionPolicy:      ResolutionMaintainOriginal,
		MinTrustForDirectPlay: 0.6,
	}
}
