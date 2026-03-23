// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"sort"

	"github.com/tenkile/tenkile/pkg/codec"
)

// EncoderSelector picks the best encoder for a given target codec,
// considering quality requirements and hardware capabilities.
type EncoderSelector struct {
	caps *ServerCapabilities
}

// NewEncoderSelector creates an encoder selector from current server capabilities.
func NewEncoderSelector(caps *ServerCapabilities) *EncoderSelector {
	return &EncoderSelector{caps: caps}
}

// SelectEncoder picks the best available encoder for the target codec.
// Priority: HW > SW, HDR-capable > not, 10-bit > not, faster > slower.
// If needsHDR is true, only encoders supporting HDR passthrough are considered.
// If needs10Bit is true, only 10-bit capable encoders are considered.
func (s *EncoderSelector) SelectEncoder(targetCodec string, needsHDR bool, needs10Bit bool) *EncoderCapability {
	candidates := s.caps.GetEncoders(targetCodec)
	if len(candidates) == 0 {
		return nil
	}

	// Filter by hard requirements
	var filtered []EncoderCapability
	for _, enc := range candidates {
		if needsHDR && !enc.SupportsHDRPassthrough {
			continue
		}
		if needs10Bit && !enc.Supports10Bit {
			continue
		}
		filtered = append(filtered, enc)
	}

	// If hard requirements eliminate everything, try without HDR requirement
	// (caller can fall back to tone mapping)
	if len(filtered) == 0 && needsHDR {
		for _, enc := range candidates {
			if needs10Bit && !enc.Supports10Bit {
				continue
			}
			filtered = append(filtered, enc)
		}
	}

	// Still nothing? Return the best available ignoring all quality constraints
	if len(filtered) == 0 {
		filtered = candidates
	}

	// Sort by preference
	sort.Slice(filtered, func(i, j int) bool {
		return encoderScore(&filtered[i]) > encoderScore(&filtered[j])
	})

	result := filtered[0]
	return &result
}

// SelectEncoderForCodec returns the best encoder for a codec without quality constraints.
func (s *EncoderSelector) SelectEncoderForCodec(codecName string) *EncoderCapability {
	return s.SelectEncoder(codecName, false, false)
}

// CanEncodeCodec checks if any encoder is available for the given codec name.
func (s *EncoderSelector) CanEncodeCodec(codecName string) bool {
	for _, enc := range s.caps.EncoderDetails {
		if codec.Equal(enc.Codec, codecName) {
			return true
		}
	}
	return false
}

// encoderScore computes a preference score for sorting. Higher = better.
func encoderScore(enc *EncoderCapability) int {
	score := enc.Performance.SpeedRank

	if enc.IsHardware {
		score += 100 // Strong HW preference
	}
	if enc.SupportsHDRPassthrough {
		score += 20
	}
	if enc.Supports10Bit {
		score += 10
	}
	if enc.Performance.RealtimeAt4K {
		score += 15
	}
	if enc.Performance.RealtimeAt1080p {
		score += 5
	}

	return score
}
