package stream

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CommandRunner interface for FFmpeg execution (Rule 10 pattern)
type CommandRunner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// DefaultCommandRunner executes real FFmpeg commands
type DefaultCommandRunner struct{}

// FIX #4: FFmpeg stderr handling using CombinedOutput() or capture stderr properly
func (r *DefaultCommandRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	return cmd.CombinedOutput()
}

// Segmenter handles HLS/DASH segment generation
type Segmenter struct {
	ffmpegPath  string
	ffprobePath string
	runner      CommandRunner
	tempDir     string
}

// NewSegmenter creates a new segmenter
func NewSegmenter(ffmpegPath, ffprobePath string, runner CommandRunner) *Segmenter {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	if runner == nil {
		runner = &DefaultCommandRunner{}
	}
	return &Segmenter{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		runner:      runner,
	}
}

// SetTempDir sets the temporary directory for segments
func (s *Segmenter) SetTempDir(dir string) {
	s.tempDir = dir
}

// GenerateHLS creates HLS segments from an input file
func (s *Segmenter) GenerateHLS(ctx context.Context, inputPath string, variants []Variant, opts HLSOptions) (*HLSManifest, error) {
	if len(variants) == 0 {
		variants = DefaultVariants()
	}
	if opts.SegmentDuration == 0 {
		opts.SegmentDuration = 6
	}
	if opts.TempDir == "" {
		opts.TempDir = s.tempDir
	}
	if opts.TempDir == "" {
		opts.TempDir = os.TempDir()
	}

	// Create temp directory for this session
	sessionID := fmt.Sprintf("hls_%d", time.Now().UnixNano())
	sessionDir := filepath.Join(opts.TempDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	manifest := &HLSManifest{
		MasterPlaylist: filepath.Join(sessionDir, "master.m3u8"),
		Variants:       make([]VariantPlaylist, 0, len(variants)),
		TempDir:        sessionDir,
		CreatedAt:      time.Now(),
	}

	// Process variants in parallel
	type variantResult struct {
		playlist VariantPlaylist
		err      error
	}

	resultChan := make(chan variantResult, len(variants))

	for _, variant := range variants {
		variant := variant // Capture loop variable
		go func() {
			variantDir := filepath.Join(sessionDir, variant.Name)
			if err := os.MkdirAll(variantDir, 0755); err != nil {
				resultChan <- variantResult{err: fmt.Errorf("create variant dir: %w", err)}
				return
			}

			playlist := filepath.Join(variantDir, "playlist.m3u8")

			// Build FFmpeg args for HLS
			args := s.buildHLSArgs(inputPath, variant, playlist, opts)

			output, err := s.runner.Run(ctx, args...)
			if err != nil {
				resultChan <- variantResult{err: fmt.Errorf("ffmpeg hls for %s: %w (output: %s)", variant.Name, err, string(output))}
				return
			}

			resultChan <- variantResult{playlist: VariantPlaylist{
				Name:        variant.Name,
				Playlist:    playlist,
				SegmentsDir: variantDir,
			}}
		}()
	}

	// Collect results
	for i := 0; i < len(variants); i++ {
		result := <-resultChan
		if result.err != nil {
			os.RemoveAll(sessionDir)
			return nil, result.err
		}
		manifest.Variants = append(manifest.Variants, result.playlist)
	}

	// Generate master playlist
	if err := s.writeMasterPlaylist(manifest); err != nil {
		os.RemoveAll(sessionDir)
		return nil, fmt.Errorf("write master playlist: %w", err)
	}

	return manifest, nil
}

// FIX #8: buildHLSArgs with explicit audio stream selection using -map 0:v -map 0:a:0
func (s *Segmenter) buildHLSArgs(inputPath string, variant Variant, outputPath string, opts HLSOptions) []string {
	args := []string{}

	// Input
	args = append(args, "-i", inputPath)

	// FIX #8: Explicit stream selection - first video and first audio track
	args = append(args, "-map", "0:v")
	if opts.IncludeAudio {
		args = append(args, "-map", "0:a:0")
	}

	// Video scaling
	if variant.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
			variant.Width, variant.Height, variant.Width, variant.Height))
	}

	// Video codec - use libx264 for compatibility
	args = append(args, "-c:v", "libx264")
	args = append(args, "-preset", "medium")
	args = append(args, "-b:v", fmt.Sprintf("%d", variant.Bitrate))
	args = append(args, "-maxrate", fmt.Sprintf("%d", int64(float64(variant.Bitrate)*1.5)))
	args = append(args, "-bufsize", fmt.Sprintf("%d", variant.Bitrate*2))

	// Audio codec
	if opts.IncludeAudio {
		args = append(args, "-c:a", "aac")
		args = append(args, "-b:a", fmt.Sprintf("%d", variant.AudioBitrate))
	}

	// HLS options
	args = append(args, "-f", "hls")
	args = append(args, "-hls_time", fmt.Sprintf("%d", opts.SegmentDuration))
	args = append(args, "-hls_list_size", fmt.Sprintf("%d", opts.PlaylistSize))
	args = append(args, "-hls_segment_filename", filepath.Join(filepath.Dir(outputPath), "segment_%03d.ts"))
	args = append(args, "-start_number", fmt.Sprintf("%d", opts.StartNumber))

	// Output
	args = append(args, outputPath)

	return append([]string{s.ffmpegPath}, args...)
}

// writeMasterPlaylist generates the master.m3u8 playlist
func (s *Segmenter) writeMasterPlaylist(manifest *HLSManifest) error {
	var sb strings.Builder

	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:3\n")

	for _, variant := range manifest.Variants {
		// Read variant playlist to get bandwidth
		bandwidth := s.estimateBandwidth(variant.Name)

		sb.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s\n",
			bandwidth, s.getResolution(variant.Name)))
		sb.WriteString(filepath.Join(variant.Name, "playlist.m3u8") + "\n")
	}

	return os.WriteFile(manifest.MasterPlaylist, []byte(sb.String()), 0644)
}

func (s *Segmenter) estimateBandwidth(name string) int64 {
	// Map variant names to approximate bandwidth
	switch name {
	case "4k":
		return 45_000_000
	case "1080p":
		return 8_000_000
	case "720p":
		return 4_000_000
	case "480p":
		return 2_500_000
	case "360p":
		return 1_000_000
	default:
		return 4_000_000
	}
}

func (s *Segmenter) getResolution(name string) string {
	switch name {
	case "4k":
		return "3840x2160"
	case "1080p":
		return "1920x1080"
	case "720p":
		return "1280x720"
	case "480p":
		return "854x480"
	case "360p":
		return "640x360"
	default:
		return "1280x720"
	}
}

// Cleanup removes session files
func (s *Segmenter) Cleanup(manifest *HLSManifest) error {
	if manifest != nil && manifest.TempDir != "" {
		return os.RemoveAll(manifest.TempDir)
	}
	return nil
}
