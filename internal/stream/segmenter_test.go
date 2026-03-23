package stream

import (
	"context"
	"testing"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

func TestNewSegmenter(t *testing.T) {
	segmenter := NewSegmenter("", "", nil)

	if segmenter == nil {
		t.Fatal("expected non-nil segmenter")
	}
	if segmenter.ffmpegPath != "ffmpeg" {
		t.Errorf("expected default ffmpegPath 'ffmpeg', got %q", segmenter.ffmpegPath)
	}
	if segmenter.ffprobePath != "ffprobe" {
		t.Errorf("expected default ffprobePath 'ffprobe', got %q", segmenter.ffprobePath)
	}
	if segmenter.runner == nil {
		t.Error("expected non-nil runner")
	}
}

func TestNewSegmenterWithCustomPaths(t *testing.T) {
	segmenter := NewSegmenter("/custom/ffmpeg", "/custom/ffprobe", nil)

	if segmenter.ffmpegPath != "/custom/ffmpeg" {
		t.Errorf("expected ffmpegPath '/custom/ffmpeg', got %q", segmenter.ffmpegPath)
	}
	if segmenter.ffprobePath != "/custom/ffprobe" {
		t.Errorf("expected ffprobePath '/custom/ffprobe', got %q", segmenter.ffprobePath)
	}
}

func TestNewSegmenterWithCustomRunner(t *testing.T) {
	mockRunner := &MockCommandRunner{}
	segmenter := NewSegmenter("", "", mockRunner)

	if segmenter.runner != mockRunner {
		t.Error("expected runner to be the mock runner")
	}
}

func TestSetTempDir(t *testing.T) {
	segmenter := NewSegmenter("", "", nil)
	
	segmenter.SetTempDir("/custom/temp")
	if segmenter.tempDir != "/custom/temp" {
		t.Errorf("expected tempDir '/custom/temp', got %q", segmenter.tempDir)
	}
}

func TestBuildHLSArgsBasic(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	variant := Variant{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 8_000_000, AudioBitrate: 192_000}
	opts := HLSOptions{
		SegmentDuration: 6,
		IncludeAudio:    true,
	}

	args := segmenter.buildHLSArgs("/input/video.mkv", variant, "/output/playlist.m3u8", opts)

	// Check basic structure
	if len(args) < 10 {
		t.Fatalf("expected at least 10 args, got %d: %v", len(args), args)
	}

	// Check input
	if args[0] != "ffmpeg" {
		t.Errorf("expected first arg 'ffmpeg', got %q", args[0])
	}
	if args[1] != "-i" {
		t.Error("expected '-i' for input")
	}
	if args[2] != "/input/video.mkv" {
		t.Errorf("expected input '/input/video.mkv', got %q", args[2])
	}

	// Check output
	lastArg := args[len(args)-1]
	if lastArg != "/output/playlist.m3u8" {
		t.Errorf("expected output '/output/playlist.m3u8', got %q", lastArg)
	}
}

func TestBuildHLSArgsWithAudioMapping(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	variant := Variant{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 8_000_000, AudioBitrate: 192_000}
	opts := HLSOptions{
		SegmentDuration: 6,
		IncludeAudio:    true,
	}

	args := segmenter.buildHLSArgs("/input/video.mkv", variant, "/output/playlist.m3u8", opts)

	// Check for explicit audio mapping
	foundMapV := false
	foundMapA := false
	for i, arg := range args {
		if arg == "-map" && i+1 < len(args) {
			if args[i+1] == "0:v" {
				foundMapV = true
			}
			if args[i+1] == "0:a:0" {
				foundMapA = true
			}
		}
	}

	if !foundMapV {
		t.Error("expected -map 0:v for video")
	}
	if !foundMapA {
		t.Error("expected -map 0:a:0 for first audio track")
	}
}

func TestBuildHLSArgsWithoutAudio(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	variant := Variant{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 8_000_000, AudioBitrate: 192_000}
	opts := HLSOptions{
		SegmentDuration: 6,
		IncludeAudio:    false,
	}

	args := segmenter.buildHLSArgs("/input/video.mkv", variant, "/output/playlist.m3u8", opts)

	// Should not have audio mapping
	for i, arg := range args {
		if arg == "-map" && i+1 < len(args) && args[i+1] == "0:a:0" {
			t.Error("should not have audio mapping when IncludeAudio is false")
		}
	}

	// Should still have video mapping
	foundMapV := false
	for i, arg := range args {
		if arg == "-map" && i+1 < len(args) && args[i+1] == "0:v" {
			foundMapV = true
		}
	}
	if !foundMapV {
		t.Error("expected video mapping even without audio")
	}
}

func TestBuildHLSArgsCodecSettings(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	variant := Variant{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 8_000_000, AudioBitrate: 192_000}
	opts := HLSOptions{
		SegmentDuration: 6,
		IncludeAudio:    true,
	}

	args := segmenter.buildHLSArgs("/input/video.mkv", variant, "/output/playlist.m3u8", opts)

	// Check video codec
	foundVcodec := false
	for i, arg := range args {
		if arg == "-c:v" && i+1 < len(args) {
			if args[i+1] == "libx264" {
				foundVcodec = true
			}
		}
	}
	if !foundVcodec {
		t.Error("expected video codec libx264")
	}

	// Check audio codec
	foundAcodec := false
	for i, arg := range args {
		if arg == "-c:a" && i+1 < len(args) {
			if args[i+1] == "aac" {
				foundAcodec = true
			}
		}
	}
	if !foundAcodec {
		t.Error("expected audio codec aac")
	}
}

func TestBuildHLSArgsBitrate(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	variant := Variant{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 8_000_000, AudioBitrate: 192_000}
	opts := HLSOptions{
		SegmentDuration: 6,
		IncludeAudio:    true,
	}

	args := segmenter.buildHLSArgs("/input/video.mkv", variant, "/output/playlist.m3u8", opts)

	// Check bitrate settings
	foundBitrate := false
	for i, arg := range args {
		if arg == "-b:v" && i+1 < len(args) {
			if args[i+1] == "8000000" {
				foundBitrate = true
			}
		}
	}
	if !foundBitrate {
		t.Error("expected video bitrate -b:v 8000000")
	}
}

func TestBuildHLSArgsHLSOptions(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	variant := Variant{Name: "720p", Width: 1280, Height: 720, Bitrate: 4_000_000, AudioBitrate: 128_000}
	opts := HLSOptions{
		SegmentDuration: 10,
		PlaylistSize:    100,
		StartNumber:    0,
		IncludeAudio:    true,
	}

	args := segmenter.buildHLSArgs("/input/video.mkv", variant, "/output/playlist.m3u8", opts)

	// Check HLS time
	foundHLSTime := false
	for i, arg := range args {
		if arg == "-hls_time" && i+1 < len(args) {
			if args[i+1] == "10" {
				foundHLSTime = true
			}
		}
	}
	if !foundHLSTime {
		t.Error("expected -hls_time 10")
	}

	// Check HLS list size
	foundHLSList := false
	for i, arg := range args {
		if arg == "-hls_list_size" && i+1 < len(args) {
			if args[i+1] == "100" {
				foundHLSList = true
			}
		}
	}
	if !foundHLSList {
		t.Error("expected -hls_list_size 100")
	}

	// Check start number
	foundStartNumber := false
	for i, arg := range args {
		if arg == "-start_number" && i+1 < len(args) {
			if args[i+1] == "0" {
				foundStartNumber = true
			}
		}
	}
	if !foundStartNumber {
		t.Error("expected -start_number 0")
	}
}

func TestBuildHLSArgsScaleFilter(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	variant := Variant{Name: "720p", Width: 1280, Height: 720, Bitrate: 4_000_000, AudioBitrate: 128_000}
	opts := HLSOptions{SegmentDuration: 6}

	args := segmenter.buildHLSArgs("/input/video.mkv", variant, "/output/playlist.m3u8", opts)

	// Check for scale filter
	foundScale := false
	for i, arg := range args {
		if arg == "-vf" && i+1 < len(args) {
			if len(args[i+1]) > 5 && args[i+1][:5] == "scale" {
				foundScale = true
			}
		}
	}
	if !foundScale {
		t.Error("expected video filter with scale")
	}
}

func TestBuildHLSArgsPreset(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	variant := Variant{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 8_000_000, AudioBitrate: 192_000}
	opts := HLSOptions{SegmentDuration: 6}

	args := segmenter.buildHLSArgs("/input/video.mkv", variant, "/output/playlist.m3u8", opts)

	// Check for preset
	foundPreset := false
	for i, arg := range args {
		if arg == "-preset" && i+1 < len(args) {
			if args[i+1] == "medium" {
				foundPreset = true
			}
		}
	}
	if !foundPreset {
		t.Error("expected -preset medium")
	}
}

func TestBuildHLSArgsSegmentFilename(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	variant := Variant{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 8_000_000, AudioBitrate: 192_000}
	opts := HLSOptions{SegmentDuration: 6}

	outputPath := "/output/1080p/playlist.m3u8"
	args := segmenter.buildHLSArgs("/input/video.mkv", variant, outputPath, opts)

	// Check for segment filename pattern (should contain -hls_segment_filename)
	foundSegmentFile := false
	for _, arg := range args {
		if arg == "-hls_segment_filename" {
			foundSegmentFile = true
			break
		}
	}
	if !foundSegmentFile {
		t.Error("expected -hls_segment_filename in args")
	}
}

func TestGenerateHLSValidation(t *testing.T) {
	segmenter := NewSegmenter("ffmpeg", "", nil)

	// Test with empty variants (should use defaults)
	variants := []Variant{}
	opts := HLSOptions{TempDir: "/tmp"}

	// This would fail because the temp dir doesn't exist, but it tests the defaults
	// We can't actually run FFmpeg without a real file
	_, err := segmenter.GenerateHLS(context.Background(), "/nonexistent/video.mkv", variants, opts)
	if err == nil {
		t.Error("expected error for non-existent input file")
	}
}

func TestMockCommandRunner(t *testing.T) {
	mock := &MockCommandRunner{
		Output: []byte("success"),
		Error:  nil,
	}

	ctx := context.Background()
	output, err := mock.Run(ctx, "ffmpeg", "-version")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(output) != "success" {
		t.Errorf("expected output 'success', got %q", string(output))
	}

	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 call recorded, got %d", len(mock.Calls))
	}
	if len(mock.Calls[0]) != 2 || mock.Calls[0][0] != "ffmpeg" || mock.Calls[0][1] != "-version" {
		t.Errorf("unexpected call recorded: %v", mock.Calls[0])
	}
}

func TestMockCommandRunnerError(t *testing.T) {
	mock := &MockCommandRunner{
		Output: []byte(""),
		Error:  context.DeadlineExceeded,
	}

	ctx := context.Background()
	_, err := mock.Run(ctx, "ffmpeg", "-invalid")

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestDefaultCommandRunner(t *testing.T) {
	runner := &DefaultCommandRunner{}

	// This test would actually run ffmpeg if available
	// Just verify the interface is satisfied
	_, err := runner.Run(context.Background(), "ffmpeg", "-version")
	// FFmpeg might not be installed, so we just check it doesn't panic
	_ = err
}
