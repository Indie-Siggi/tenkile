// SPDX-License-Identifier: AGPL-3.0-or-later

package transcode

import (
	"fmt"
	"strings"

	"github.com/tenkile/tenkile/internal/server"
)

// FFmpegArgs holds the constructed FFmpeg command arguments.
type FFmpegArgs struct {
	Input         []string
	VideoFilters  []string
	VideoEncoder  string
	VideoArgs     []string
	AudioEncoder  string
	AudioArgs     []string
	OutputFormat  string
	OutputArgs    []string
	SubtitleArgs  []string
}

// Build returns the complete FFmpeg argument list.
func (a *FFmpegArgs) Build(inputPath, outputPath string) []string {
	args := make([]string, 0, 30)

	// Input
	args = append(args, a.Input...)
	args = append(args, "-i", inputPath)

	// Video
	if a.VideoEncoder != "" {
		args = append(args, "-c:v", a.VideoEncoder)
		if len(a.VideoFilters) > 0 {
			args = append(args, "-vf", strings.Join(a.VideoFilters, ","))
		}
		args = append(args, a.VideoArgs...)
	} else {
		args = append(args, "-c:v", "copy")
	}

	// Audio
	if a.AudioEncoder != "" {
		args = append(args, "-c:a", a.AudioEncoder)
		args = append(args, a.AudioArgs...)
	} else {
		args = append(args, "-c:a", "copy")
	}

	// Subtitles
	args = append(args, a.SubtitleArgs...)

	// Output format
	if a.OutputFormat != "" {
		args = append(args, "-f", a.OutputFormat)
	}
	args = append(args, a.OutputArgs...)
	args = append(args, outputPath)

	return args
}

// BuildFFmpegArgs constructs FFmpeg arguments from a PlaybackDecision.
func BuildFFmpegArgs(decision *PlaybackDecision, encoder *server.EncoderCapability, policy QualityPreservationPolicy) (*FFmpegArgs, error) {
	args := &FFmpegArgs{}

	if decision.Type == DecisionDirectPlay {
		return nil, nil // No FFmpeg needed
	}

	if decision.Type == DecisionRemux {
		// Remux: copy streams, change container only
		args.OutputFormat = decision.TargetContainer
		return args, nil
	}

	// Transcode
	buildVideoArgs(args, decision, encoder, policy)
	buildAudioArgs(args, decision)
	if err := buildSubtitleArgs(args, decision); err != nil {
		return nil, fmt.Errorf("build subtitle args: %w", err)
	}

	// Output format
	args.OutputFormat = mapContainerToFFmpegFormat(decision.TargetContainer)

	return args, nil
}

func buildVideoArgs(args *FFmpegArgs, decision *PlaybackDecision, encoder *server.EncoderCapability, policy QualityPreservationPolicy) {
	if !decision.NeedsVideoTranscode {
		return
	}

	if encoder == nil {
		args.VideoEncoder = "libx264" // Absolute fallback
	} else {
		args.VideoEncoder = encoder.EncoderName
	}

	// Handle legacy content (interlaced, anamorphic)
	legacyFlags := DetectLegacyContent(&MediaItem{
		Width:           decision.SourceWidth,
		Height:          decision.SourceHeight,
		Interlaced:      decision.SourceInterlaced,
		PixelAspectRatio: decision.SourcePixelAspectRatio,
	})
	legacyFilters := BuildLegacyFilterArgs(legacyFlags)
	args.VideoFilters = append(args.VideoFilters, legacyFilters...)

	// HDR handling
	if decision.SourceIsHDR {
		if decision.HDRPreserved {
			// HDR passthrough: preserve metadata and 10-bit
			buildHDRPassthroughArgs(args, encoder)
		} else if decision.ToneMapped {
			// HDR -> SDR tone mapping
			buildToneMappingArgs(args, policy)
		}
	}

	// Pixel format
	if decision.HDRPreserved {
		args.VideoArgs = append(args.VideoArgs, "-pix_fmt", "p010le")
	} else {
		args.VideoArgs = append(args.VideoArgs, "-pix_fmt", "yuv420p")
	}

	// Encoder-specific options
	addEncoderPreset(args, encoder)
}

func buildHDRPassthroughArgs(args *FFmpegArgs, encoder *server.EncoderCapability) {
	// Copy HDR metadata
	args.VideoArgs = append(args.VideoArgs,
		"-color_primaries", "bt2020",
		"-color_trc", "smpte2084",
		"-colorspace", "bt2020nc",
	)

	// Hardware encoder specific HDR flags
	if encoder != nil && encoder.IsHardware {
		switch {
		case strings.Contains(encoder.EncoderName, "nvenc"):
			args.VideoArgs = append(args.VideoArgs, "-rc", "constqp")
		case strings.Contains(encoder.EncoderName, "vaapi"):
			// VAAPI handles HDR metadata automatically
		case strings.Contains(encoder.EncoderName, "videotoolbox"):
			args.VideoArgs = append(args.VideoArgs, "-allow_sw", "1")
		}
	}
}

func buildToneMappingArgs(args *FFmpegArgs, policy QualityPreservationPolicy) {
	switch policy.ToneMappingQuality {
	case ToneMappingHigh:
		// libplacebo: highest quality perceptual mapping
		args.VideoFilters = append(args.VideoFilters,
			"libplacebo=tonemapping=bt.2390:colorspace=bt709:color_primaries=bt709:color_trc=bt709:format=yuv420p",
		)
	case ToneMappingMedium:
		// zscale: good quality, CPU-based
		args.VideoFilters = append(args.VideoFilters,
			"zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p",
		)
	case ToneMappingFast:
		// Simple reinhard tone mapping
		args.VideoFilters = append(args.VideoFilters,
			"zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=reinhard:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p",
		)
	}

	// Output must be BT.709 SDR
	args.VideoArgs = append(args.VideoArgs,
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-colorspace", "bt709",
	)
}

func buildAudioArgs(args *FFmpegArgs, decision *PlaybackDecision) {
	if !decision.NeedsAudioTranscode {
		return
	}

	switch decision.TargetAudioCodec {
	case "aac":
		args.AudioEncoder = "aac"
		args.AudioArgs = append(args.AudioArgs, "-b:a", audioBitrateForChannels(decision.TargetAudioChannels))
	case "eac3":
		args.AudioEncoder = "eac3"
		args.AudioArgs = append(args.AudioArgs, "-b:a", audioBitrateForEAC3(decision.TargetAudioChannels))
	case "ac3":
		args.AudioEncoder = "ac3"
		args.AudioArgs = append(args.AudioArgs, "-b:a", "640k")
	case "opus":
		args.AudioEncoder = "libopus"
		args.AudioArgs = append(args.AudioArgs, "-b:a", "256k")
	case "flac":
		args.AudioEncoder = "flac"
	case "mp3":
		args.AudioEncoder = "libmp3lame"
		args.AudioArgs = append(args.AudioArgs, "-b:a", "320k")
	default:
		args.AudioEncoder = "aac"
		args.AudioArgs = append(args.AudioArgs, "-b:a", "256k")
	}

	if decision.TargetAudioChannels > 0 {
		args.AudioArgs = append(args.AudioArgs, "-ac", fmt.Sprintf("%d", decision.TargetAudioChannels))
	}
}

func buildSubtitleArgs(args *FFmpegArgs, decision *PlaybackDecision) error {
	if decision.SubtitleDecision.Action == SubtitleBurnIn {
		// Validate streamIndex is a non-negative integer before interpolation
		// to prevent command injection via crafted stream index values.
		if decision.SubtitleDecision.streamIndex < 0 {
			return fmt.Errorf("invalid subtitle stream index: %d (must be non-negative)", decision.SubtitleDecision.streamIndex)
		}

		// Add subtitle overlay filter to burn subtitles into video.
		// For graphical subs (PGS/VOBSUB), use overlay filter via filter_complex.
		// For text/styled subs (ASS/SRT), use the subtitles filter.
		switch decision.SubtitleDecision.Type {
		case SubtitleGraphical:
			// Graphical: extract subtitle stream and overlay
			args.VideoFilters = append(args.VideoFilters,
				fmt.Sprintf("subtitles=si=%d", decision.SubtitleDecision.streamIndex))
		case SubtitleStyledText:
			// ASS/SSA: render with full styling via subtitles filter
			args.VideoFilters = append(args.VideoFilters,
				fmt.Sprintf("subtitles=si=%d", decision.SubtitleDecision.streamIndex))
		}
		args.SubtitleArgs = append(args.SubtitleArgs, "-sn") // Don't copy subtitle stream
	} else {
		args.SubtitleArgs = append(args.SubtitleArgs, "-sn") // Strip subtitles from output
	}
	return nil
}

func addEncoderPreset(args *FFmpegArgs, encoder *server.EncoderCapability) {
	if encoder == nil {
		args.VideoArgs = append(args.VideoArgs, "-preset", "fast")
		return
	}

	switch {
	case strings.Contains(encoder.EncoderName, "nvenc"):
		args.VideoArgs = append(args.VideoArgs, "-preset", "p4", "-tune", "hq")
	case strings.Contains(encoder.EncoderName, "qsv"):
		args.VideoArgs = append(args.VideoArgs, "-preset", "faster")
	case strings.Contains(encoder.EncoderName, "vaapi"):
		// VAAPI doesn't have preset options
	case strings.Contains(encoder.EncoderName, "videotoolbox"):
		args.VideoArgs = append(args.VideoArgs, "-realtime", "1")
	case encoder.EncoderName == "libx264":
		args.VideoArgs = append(args.VideoArgs, "-preset", "fast", "-crf", "23")
	case encoder.EncoderName == "libx265":
		args.VideoArgs = append(args.VideoArgs, "-preset", "fast", "-crf", "28")
	case encoder.EncoderName == "libsvtav1":
		args.VideoArgs = append(args.VideoArgs, "-preset", "8", "-crf", "30")
	}
}

func audioBitrateForChannels(channels int) string {
	switch {
	case channels >= 8:
		return "512k"
	case channels >= 6:
		return "384k"
	default:
		return "256k"
	}
}

func audioBitrateForEAC3(channels int) string {
	switch {
	case channels >= 8:
		return "1024k"
	case channels >= 6:
		return "640k"
	default:
		return "384k"
	}
}

func mapContainerToFFmpegFormat(container string) string {
	switch container {
	case "mp4":
		return "mp4"
	case "mkv":
		return "matroska"
	case "ts":
		return "mpegts"
	case "webm":
		return "webm"
	case "mov":
		return "mov"
	default:
		return "mp4"
	}
}
