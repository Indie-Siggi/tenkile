// SPDX-License-Identifier: AGPL-3.0-or-later

package transcode

import "math"

// LegacyContentFlags describes special handling required for legacy/SD content.
type LegacyContentFlags struct {
	IsInterlaced    bool
	IsAnamorphic    bool    // Non-square pixels (e.g., anamorphic DVD)
	IsSD            bool    // 480p/576p
	PixelAspectRatio float64 // DAR correction (e.g., 1.185 for 720x480 -> 853x480)
	ColorSpace      string  // "bt601" for SD, "bt709" for HD
}

// DetectLegacyContent analyzes a media item and returns flags for special handling.
func DetectLegacyContent(item *MediaItem) LegacyContentFlags {
	flags := LegacyContentFlags{}

	// SD detection: 480p (NTSC) or 576p (PAL)
	if item.Height <= 576 {
		flags.IsSD = true
		flags.ColorSpace = "bt601"
	} else {
		flags.ColorSpace = "bt709"
	}

	// Interlaced detection
	if item.Interlaced {
		flags.IsInterlaced = true
	}

	// Anamorphic detection (non-square pixels)
	if item.PixelAspectRatio > 0 && math.Abs(item.PixelAspectRatio-1.0) > 0.001 {
		flags.IsAnamorphic = true
		flags.PixelAspectRatio = item.PixelAspectRatio
	}

	return flags
}

// BuildLegacyFilterArgs returns FFmpeg video filter args for legacy content.
func BuildLegacyFilterArgs(flags LegacyContentFlags) []string {
	var filters []string

	// Deinterlace first (before any scaling)
	if flags.IsInterlaced {
		// bwdif is higher quality than yadif
		filters = append(filters, "bwdif=mode=send_frame:parity=auto:deint=all")
	}

	// Anamorphic correction: set correct display aspect ratio
	if flags.IsAnamorphic && flags.PixelAspectRatio > 0 {
		filters = append(filters, "setsar=1")
	}

	return filters
}
