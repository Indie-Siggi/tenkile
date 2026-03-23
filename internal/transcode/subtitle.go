// SPDX-License-Identifier: AGPL-3.0-or-later

package transcode

import "github.com/tenkile/tenkile/pkg/codec"

// SubtitleType classifies subtitle formats by handling requirements.
type SubtitleType int

const (
	SubtitleText      SubtitleType = iota // SRT, VTT — serve externally as VTT
	SubtitleStyledText                    // ASS, SSA — configurable: burn-in or strip to VTT
	SubtitleGraphical                     // PGS, VOBSUB, DVB — must burn in if client can't render
	SubtitleNone                          // No subtitle track
)

// SubtitleDecision describes what to do with a subtitle track.
type SubtitleDecision struct {
	Type          SubtitleType
	SourceFormat  string
	Action        SubtitleAction
	OutputFormat  string // "vtt" for external delivery, "" for burn-in
	ForcesBurnIn  bool   // If true, this forces a full video transcode

	streamIndex int // FFmpeg stream index for burn-in filter (set by orchestrator)
}

// SubtitleAction describes the chosen handling.
type SubtitleAction int

const (
	SubtitlePassthrough  SubtitleAction = iota // Deliver as-is (or convert text to VTT)
	SubtitleConvertToVTT                       // Convert styled text to plain VTT
	SubtitleBurnIn                             // Overlay onto video (forces transcode)
	SubtitleDrop                               // No subtitle selected
)

func (a SubtitleAction) String() string {
	switch a {
	case SubtitlePassthrough:
		return "passthrough"
	case SubtitleConvertToVTT:
		return "convert_to_vtt"
	case SubtitleBurnIn:
		return "burn_in"
	case SubtitleDrop:
		return "drop"
	default:
		return "unknown"
	}
}

// SubtitleConfig controls styled text subtitle behavior.
type SubtitleConfig struct {
	BurnInStyledText bool // If true, ASS/SSA are burned in; if false, stripped to VTT
}

// ClassifySubtitle returns the subtitle type for a given format string.
func ClassifySubtitle(format string) SubtitleType {
	switch {
	case codec.Equal(format, "srt") || codec.Equal(format, "vtt") || codec.Equal(format, "subrip"):
		return SubtitleText
	case codec.Equal(format, "ass") || codec.Equal(format, "ssa"):
		return SubtitleStyledText
	case codec.Equal(format, "pgs") || codec.Equal(format, "hdmv_pgs_subtitle") ||
		codec.Equal(format, "vobsub") || codec.Equal(format, "dvbsub") ||
		codec.Equal(format, "dvb_subtitle") || codec.Equal(format, "dvdsub"):
		return SubtitleGraphical
	default:
		return SubtitleNone
	}
}

// DecideSubtitle determines the subtitle handling strategy.
// This MUST be called BEFORE the video transcode decision because
// burn-in forces a full video transcode.
func DecideSubtitle(subFormat string, deviceSupportsFormat bool, config SubtitleConfig) SubtitleDecision {
	if subFormat == "" {
		return SubtitleDecision{Type: SubtitleNone, Action: SubtitleDrop}
	}

	subType := ClassifySubtitle(subFormat)

	switch subType {
	case SubtitleText:
		// Text subs: serve externally as VTT. Zero transcode cost.
		return SubtitleDecision{
			Type:         SubtitleText,
			SourceFormat: subFormat,
			Action:       SubtitlePassthrough,
			OutputFormat: "vtt",
		}

	case SubtitleStyledText:
		if config.BurnInStyledText {
			return SubtitleDecision{
				Type:         SubtitleStyledText,
				SourceFormat: subFormat,
				Action:       SubtitleBurnIn,
				ForcesBurnIn: true,
			}
		}
		// Strip to VTT (loses styling)
		return SubtitleDecision{
			Type:         SubtitleStyledText,
			SourceFormat: subFormat,
			Action:       SubtitleConvertToVTT,
			OutputFormat: "vtt",
		}

	case SubtitleGraphical:
		// Graphical subs can't be converted to text.
		if deviceSupportsFormat {
			return SubtitleDecision{
				Type:         SubtitleGraphical,
				SourceFormat: subFormat,
				Action:       SubtitlePassthrough,
			}
		}
		// Must burn in — forces full video transcode
		return SubtitleDecision{
			Type:         SubtitleGraphical,
			SourceFormat: subFormat,
			Action:       SubtitleBurnIn,
			ForcesBurnIn: true,
		}

	default:
		return SubtitleDecision{Type: SubtitleNone, Action: SubtitleDrop}
	}
}
