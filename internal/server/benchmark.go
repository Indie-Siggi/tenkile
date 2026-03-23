// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BenchmarkResult holds the result of an encoder benchmark.
type BenchmarkResult struct {
	EncoderName    string
	Codec          string
	Width          int
	Height         int
	FPS            float64
	SpeedFactor    float64 // >1.0 means faster than real-time
	Duration       time.Duration
	RealtimeCapable bool
}

// Benchmarker runs optional startup benchmarks to estimate encoding performance.
type Benchmarker struct {
	runner CommandRunner
	logger *slog.Logger
}

// NewBenchmarker creates a new benchmarker.
func NewBenchmarker(runner CommandRunner, logger *slog.Logger) *Benchmarker {
	if runner == nil {
		runner = &ExecRunner{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Benchmarker{runner: runner, logger: logger}
}

// BenchmarkEncoder runs a quick encode test to estimate real-time capability.
// Uses a synthetic source (lavfi testsrc2) for ~5 seconds of encoding.
func (b *Benchmarker) BenchmarkEncoder(ctx context.Context, ffmpegPath string, enc EncoderCapability) (*BenchmarkResult, error) {
	width := 1920
	height := 1080
	fps := 30.0
	testDuration := 5 // seconds of test content

	// Build FFmpeg args for benchmark
	args := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc2=duration=%d:size=%dx%d:rate=%d", testDuration, width, height, int(fps)),
		"-c:v", enc.EncoderName,
	}

	// Use yuv420p for benchmarks (most compatible). If the encoder doesn't
	// support it, use its first listed pixel format as a fallback.
	if len(enc.SupportedPixelFormats) > 0 {
		pixFmt := enc.SupportedPixelFormats[0]
		for _, pf := range enc.SupportedPixelFormats {
			if pf == "yuv420p" {
				pixFmt = "yuv420p"
				break
			}
		}
		args = append(args, "-pix_fmt", pixFmt)
	}

	// Use fastest preset for benchmark
	switch {
	case strings.Contains(enc.EncoderName, "nvenc"):
		args = append(args, "-preset", "p1")
	case strings.Contains(enc.EncoderName, "qsv"):
		args = append(args, "-preset", "veryfast")
	case enc.EncoderName == "libx264":
		args = append(args, "-preset", "ultrafast")
	case enc.EncoderName == "libx265":
		args = append(args, "-preset", "ultrafast")
	case enc.EncoderName == "libsvtav1":
		args = append(args, "-preset", "12")
	}

	args = append(args, "-f", "null", "-")

	start := time.Now()
	_, stderr, err := b.runner.Run(ctx, ffmpegPath, args...)
	elapsed := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("benchmark failed for %s: %w", enc.EncoderName, err)
	}

	// Parse speed from FFmpeg stderr output
	speedFactor := parseFFmpegSpeed(string(stderr))
	if speedFactor <= 0 {
		// Fallback: estimate from wall time vs content duration
		speedFactor = float64(testDuration) / elapsed.Seconds()
	}

	result := &BenchmarkResult{
		EncoderName:     enc.EncoderName,
		Codec:           enc.Codec,
		Width:           width,
		Height:          height,
		FPS:             fps,
		SpeedFactor:     speedFactor,
		Duration:        elapsed,
		RealtimeCapable: speedFactor >= 1.0,
	}

	b.logger.Info("benchmark complete",
		"encoder", enc.EncoderName,
		"speed", fmt.Sprintf("%.2fx", speedFactor),
		"realtime", result.RealtimeCapable,
		"duration", elapsed.Round(time.Millisecond),
	)

	return result, nil
}

// BenchmarkAll runs benchmarks for all available encoders and updates
// their performance estimates in the capabilities.
func (b *Benchmarker) BenchmarkAll(ctx context.Context, ffmpegPath string, caps *ServerCapabilities) []BenchmarkResult {
	var results []BenchmarkResult

	for i := range caps.EncoderDetails {
		enc := &caps.EncoderDetails[i]

		result, err := b.BenchmarkEncoder(ctx, ffmpegPath, *enc)
		if err != nil {
			b.logger.Warn("benchmark failed", "encoder", enc.EncoderName, "error", err)
			continue
		}

		// Update the encoder's performance estimate from actual benchmark
		enc.Performance.RealtimeAt1080p = result.RealtimeCapable
		// Extrapolate 4K: assume ~4x slower than 1080p
		enc.Performance.RealtimeAt4K = result.SpeedFactor >= 4.0

		results = append(results, *result)
	}

	return results
}

// parseFFmpegSpeed extracts the speed multiplier from FFmpeg stderr.
// FFmpeg outputs lines like "speed=2.5x" or "speed= 1.23x"
func parseFFmpegSpeed(stderr string) float64 {
	re := regexp.MustCompile(`speed=\s*([\d.]+)x`)
	matches := re.FindAllStringSubmatch(stderr, -1)
	if len(matches) == 0 {
		return 0
	}
	// Take the last speed report (final average)
	last := matches[len(matches)-1]
	if len(last) > 1 {
		speed, err := strconv.ParseFloat(last[1], 64)
		if err == nil {
			return speed
		}
	}
	return 0
}
