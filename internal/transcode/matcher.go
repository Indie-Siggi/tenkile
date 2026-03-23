// SPDX-License-Identifier: AGPL-3.0-or-later

package transcode

import (
	"github.com/tenkile/tenkile/internal/probes"
	"github.com/tenkile/tenkile/pkg/codec"
)

// DeviceMatcher evaluates whether a device can handle specific media.
type DeviceMatcher struct {
	policy QualityPreservationPolicy
}

// NewDeviceMatcher creates a new matcher with the given quality policy.
func NewDeviceMatcher(policy QualityPreservationPolicy) *DeviceMatcher {
	return &DeviceMatcher{policy: policy}
}

// CanDirectPlayVideo checks if the device can directly play the video codec.
func (m *DeviceMatcher) CanDirectPlayVideo(caps *probes.DeviceCapabilities, item *MediaItem) bool {
	if caps.TrustScore < m.policy.MinTrustForDirectPlay {
		return false
	}

	// Check video codec support
	if !containsCodec(caps.VideoCodecs, item.VideoCodec) {
		return false
	}

	// Check resolution
	if caps.MaxWidth > 0 && item.Width > caps.MaxWidth {
		return false
	}
	if caps.MaxHeight > 0 && item.Height > caps.MaxHeight {
		return false
	}

	// Check bitrate
	if caps.MaxBitrate > 0 && item.Bitrate > caps.MaxBitrate {
		return false
	}

	// Check HDR compatibility
	if item.IsHDR && !caps.SupportsHDR {
		return false
	}

	// Check Dolby Vision
	if item.IsDolbyVision && !caps.SupportsDolbyVision {
		return false
	}

	return true
}

// CanDirectPlayAudio checks if the device can directly play the audio codec.
func (m *DeviceMatcher) CanDirectPlayAudio(caps *probes.DeviceCapabilities, item *MediaItem) bool {
	if !containsCodec(caps.AudioCodecs, item.AudioCodec) {
		return false
	}

	// Check DTS support
	if (codec.Equal(item.AudioCodec, "dts") || codec.Equal(item.AudioCodec, "dts-hd")) && !caps.SupportsDTS {
		return false
	}

	// Check Dolby Atmos
	if item.IsDolbyAtmos && !caps.SupportsDolbyAtmos {
		return false
	}

	return true
}

// CanDirectPlayContainer checks container format compatibility.
func (m *DeviceMatcher) CanDirectPlayContainer(caps *probes.DeviceCapabilities, container string) bool {
	return containsCodec(caps.ContainerFormats, container)
}

// SupportsCodec checks if the device supports a given video codec.
func (m *DeviceMatcher) SupportsCodec(caps *probes.DeviceCapabilities, codecName string) bool {
	return containsCodec(caps.VideoCodecs, codecName)
}

// SupportsAudioCodec checks if the device supports a given audio codec.
func (m *DeviceMatcher) SupportsAudioCodec(caps *probes.DeviceCapabilities, codecName string) bool {
	return containsCodec(caps.AudioCodecs, codecName)
}

// containsCodec checks if a codec name is in a list (case-insensitive).
func containsCodec(list []string, name string) bool {
	for _, item := range list {
		if codec.Equal(item, name) {
			return true
		}
	}
	return false
}
