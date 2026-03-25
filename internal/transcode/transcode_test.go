// SPDX-License-Identifier: AGPL-3.0-or-later

package transcode

import (
	"context"
	"testing"

	"github.com/tenkile/tenkile/internal/probes"
	"github.com/tenkile/tenkile/internal/server"
)

// --- Test helpers ---

func testOrchestrator(encoders []server.EncoderCapability) *Orchestrator {
	mock := &testMockRunner{encoders: encoders}
	inv := server.NewInventory(mock, nil)
	inv.SetPathFinder(&testPathFinder{})
	inv.SetDeviceExistsFunc(func(_ string) bool { return false })

	ctx := context.Background()
	_, _ = inv.Discover(ctx) // This will use our mock

	return NewOrchestrator(inv, DefaultQualityPolicy(), SubtitleConfig{}, nil)
}

type testPathFinder struct{}

func (t *testPathFinder) LookPath(_ string) (string, error) {
	return "/usr/bin/ffmpeg", nil
}

type testMockRunner struct {
	encoders []server.EncoderCapability
}

// audioCodecs is the set of codec names that should be flagged as audio in FFmpeg output.
var audioCodecs = map[string]bool{
	"aac": true, "eac3": true, "ac3": true, "opus": true, "flac": true,
	"mp3": true, "truehd": true, "dts": true, "vorbis": true, "alac": true,
}

func (m *testMockRunner) Run(_ context.Context, name string, args ...string) ([]byte, []byte, error) {
	// ffmpeg -version
	if len(args) > 0 && args[0] == "-version" {
		return []byte("ffmpeg version 7.0.1\n"), nil, nil
	}
	// ffmpeg -encoders — build from our encoder list
	if len(args) > 0 && args[0] == "-encoders" {
		output := " Encoders:\n V..... = Video\n A..... = Audio\n ------\n"
		for _, enc := range m.encoders {
			prefix := "V"
			if audioCodecs[enc.Codec] {
				prefix = "A"
			}
			output += " " + prefix + "..... " + enc.EncoderName + "              test\n"
		}
		return []byte(output), nil, nil
	}
	// ffmpeg -decoders
	if len(args) > 0 && args[0] == "-decoders" {
		return []byte(" Decoders:\n V..... = Video\n ------\n"), nil, nil
	}
	// All HW probes fail
	return nil, nil, &mockError{}
}

type mockError struct{}

func (e *mockError) Error() string { return "mock: not available" }

// Standard server encoder sets for tests
var (
	swEncoders = []server.EncoderCapability{
		{Codec: "h264", EncoderName: "libx264", Performance: server.PerformanceEstimate{RealtimeAt1080p: true, SpeedRank: 30}},
		{Codec: "hevc", EncoderName: "libx265", Supports10Bit: true, SupportsHDRPassthrough: true, Performance: server.PerformanceEstimate{RealtimeAt1080p: true, SpeedRank: 10}},
		{Codec: "av1", EncoderName: "libsvtav1", Supports10Bit: true, SupportsHDRPassthrough: true, Performance: server.PerformanceEstimate{SpeedRank: 5}},
		{Codec: "aac", EncoderName: "aac"},
		{Codec: "eac3", EncoderName: "eac3"},
		{Codec: "ac3", EncoderName: "ac3"},
		{Codec: "opus", EncoderName: "libopus"},
	}

	nvencEncoders = []server.EncoderCapability{
		{Codec: "h264", EncoderName: "libx264", Performance: server.PerformanceEstimate{RealtimeAt1080p: true, SpeedRank: 30}},
		{Codec: "hevc", EncoderName: "libx265", Supports10Bit: true, SupportsHDRPassthrough: true, Performance: server.PerformanceEstimate{RealtimeAt1080p: true, SpeedRank: 10}},
		{Codec: "h264", EncoderName: "h264_nvenc", IsHardware: true, Performance: server.PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 90}},
		{Codec: "hevc", EncoderName: "hevc_nvenc", IsHardware: true, Supports10Bit: true, SupportsHDRPassthrough: true, Performance: server.PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 85}},
		{Codec: "av1", EncoderName: "av1_nvenc", IsHardware: true, Supports10Bit: true, SupportsHDRPassthrough: true, Performance: server.PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 80}},
		{Codec: "aac", EncoderName: "aac"},
		{Codec: "eac3", EncoderName: "eac3"},
		{Codec: "ac3", EncoderName: "ac3"},
	}

	h264OnlyEncoders = []server.EncoderCapability{
		{Codec: "h264", EncoderName: "libx264", Performance: server.PerformanceEstimate{RealtimeAt1080p: true, SpeedRank: 30}},
		{Codec: "aac", EncoderName: "aac"},
	}
)

// Standard device capabilities for tests
func hdrDevice() *probes.DeviceCapabilities {
	return &probes.DeviceCapabilities{
		VideoCodecs:      []string{"h264", "hevc", "vp9"},
		AudioCodecs:      []string{"aac", "eac3", "ac3", "opus"},
		ContainerFormats: []string{"mp4", "mkv", "webm"},
		SubtitleFormats:  []string{"srt", "vtt"},
		MaxWidth:         3840,
		MaxHeight:        2160,
		MaxBitrate:       80000000,
		SupportsHDR:      true,
		TrustScore:       0.8,
		TrustLevel:       probes.TrustLevelHigh,
	}
}

func sdrDevice() *probes.DeviceCapabilities {
	return &probes.DeviceCapabilities{
		VideoCodecs:      []string{"h264", "hevc"},
		AudioCodecs:      []string{"aac", "eac3", "ac3"},
		ContainerFormats: []string{"mp4", "mkv"},
		SubtitleFormats:  []string{"srt", "vtt"},
		MaxWidth:         3840,
		MaxHeight:        2160,
		MaxBitrate:       40000000,
		SupportsHDR:      false,
		TrustScore:       0.8,
		TrustLevel:       probes.TrustLevelHigh,
	}
}

func h264OnlyDevice() *probes.DeviceCapabilities {
	return &probes.DeviceCapabilities{
		VideoCodecs:      []string{"h264"},
		AudioCodecs:      []string{"aac"},
		ContainerFormats: []string{"mp4"},
		SubtitleFormats:  []string{"srt", "vtt"},
		MaxWidth:         1920,
		MaxHeight:        1080,
		MaxBitrate:       20000000,
		SupportsHDR:      false,
		TrustScore:       0.8,
		TrustLevel:       probes.TrustLevelHigh,
	}
}

func dvCapableDevice() *probes.DeviceCapabilities {
	caps := hdrDevice()
	caps.SupportsDolbyVision = true
	return caps
}

// --- HDR Device Tests ---

func TestHDRDevice_AV1HDR_OpusMKV(t *testing.T) {
	// AV1 HDR10 / Opus / MKV -> device supports HEVC+HDR, not AV1
	// Expected: HEVC HDR10 / Opus / MP4
	orch := testOrchestrator(swEncoders)
	item := &MediaItem{
		VideoCodec: "av1", AudioCodec: "opus", Container: "mkv",
		Width: 3840, Height: 2160, Bitrate: 20000000,
		IsHDR: true, HDRType: "hdr10", AudioChannels: 2,
	}
	decision := orch.Decide(context.Background(), item, hdrDevice())

	if decision.Type != DecisionTranscode {
		t.Errorf("expected transcode, got %s", decision.Type)
	}
	if decision.TargetVideoCodec != "hevc" {
		t.Errorf("expected hevc target, got %s", decision.TargetVideoCodec)
	}
	if !decision.HDRPreserved {
		t.Error("expected HDR preserved on HDR device")
	}
	if decision.ToneMapped {
		t.Error("should NOT tone-map on HDR device")
	}
	if decision.NeedsAudioTranscode {
		t.Error("Opus should be directly playable, no audio transcode needed")
	}
}

func TestHDRDevice_HEVCDVP7_TrueHD71(t *testing.T) {
	// HEVC DV P7 / TrueHD 7.1 -> device supports HEVC+HDR, not DV
	// Expected: HEVC HDR10 base / E-AC-3 7.1
	orch := testOrchestrator(swEncoders)
	item := &MediaItem{
		VideoCodec: "hevc", AudioCodec: "truehd", Container: "mkv",
		Width: 3840, Height: 2160, Bitrate: 40000000,
		IsHDR: true, HDRType: "dolby_vision", IsDolbyVision: true,
		DolbyVisionProfile: "7", AudioChannels: 8,
	}
	decision := orch.Decide(context.Background(), item, hdrDevice())

	if decision.TargetVideoCodec != "hevc" {
		t.Errorf("expected hevc target, got %s", decision.TargetVideoCodec)
	}
	if decision.NeedsAudioTranscode && decision.TargetAudioCodec != "eac3" {
		t.Errorf("expected eac3 for audio, got %s", decision.TargetAudioCodec)
	}
}

func TestHDRDevice_HEVCDVP8_EAC3_DirectPlay(t *testing.T) {
	// HEVC DV P8 / E-AC-3 -> device supports HEVC + DV P8
	// Expected: DirectPlay
	orch := testOrchestrator(swEncoders)
	device := dvCapableDevice()
	item := &MediaItem{
		VideoCodec: "hevc", AudioCodec: "eac3", Container: "mp4",
		Width: 3840, Height: 2160, Bitrate: 30000000,
		IsHDR: true, HDRType: "dolby_vision", IsDolbyVision: true,
		DolbyVisionProfile: "8", AudioChannels: 6,
	}
	decision := orch.Decide(context.Background(), item, device)

	if decision.Type != DecisionDirectPlay {
		t.Errorf("expected direct_play, got %s (reasons: %v)", decision.Type, decision.Reasons)
	}
}

func TestHDRDevice_HEVCHDR10_DTSHD71_AACOnly(t *testing.T) {
	// HEVC HDR10 / DTS-HD 7.1 -> device supports HEVC+HDR, AAC only audio
	// Expected: DirectPlay video / AAC 5.1 audio
	orch := testOrchestrator(swEncoders)
	device := hdrDevice()
	device.AudioCodecs = []string{"aac"} // AAC only
	device.SupportsDTS = false

	item := &MediaItem{
		VideoCodec: "hevc", AudioCodec: "dts", Container: "mkv",
		Width: 3840, Height: 2160, Bitrate: 40000000,
		IsHDR: true, HDRType: "hdr10", AudioChannels: 8,
	}
	decision := orch.Decide(context.Background(), item, device)

	if decision.Type != DecisionTranscode {
		t.Errorf("expected transcode (audio only), got %s", decision.Type)
	}
	if decision.NeedsVideoTranscode {
		t.Error("video should be direct-play (HEVC HDR10 on HDR device)")
	}
	if !decision.NeedsAudioTranscode {
		t.Error("audio should be transcoded (DTS-HD -> AAC)")
	}
	if decision.TargetAudioCodec != "aac" {
		t.Errorf("expected aac audio, got %s", decision.TargetAudioCodec)
	}
	if !decision.HDRPreserved {
		t.Error("HDR should be preserved (video is direct-play)")
	}
}

// --- SDR Device Tests ---

func TestSDRDevice_AV1HDR_OpusMKV(t *testing.T) {
	// AV1 HDR10 / Opus / MKV -> device supports HEVC, no HDR display
	// Expected: HEVC SDR (tonemap) / Opus
	orch := testOrchestrator(swEncoders)
	item := &MediaItem{
		VideoCodec: "av1", AudioCodec: "opus", Container: "mkv",
		Width: 3840, Height: 2160, Bitrate: 20000000,
		IsHDR: true, HDRType: "hdr10", AudioChannels: 2,
	}
	decision := orch.Decide(context.Background(), item, sdrDevice())

	if decision.TargetVideoCodec != "hevc" {
		t.Errorf("expected hevc target, got %s", decision.TargetVideoCodec)
	}
	if !decision.ToneMapped {
		t.Error("expected tone mapping on SDR device")
	}
	if decision.HDRPreserved {
		t.Error("HDR should NOT be preserved on SDR device")
	}
}

func TestSDRDevice_HEVCHDR10_EAC3(t *testing.T) {
	// HEVC HDR10 / E-AC-3 -> device supports H.264 only, no HDR
	// Expected: H.264 SDR (tonemap) / AAC 5.1
	orch := testOrchestrator(swEncoders)
	device := h264OnlyDevice()
	item := &MediaItem{
		VideoCodec: "hevc", AudioCodec: "eac3", Container: "mkv",
		Width: 3840, Height: 2160, Bitrate: 30000000,
		IsHDR: true, HDRType: "hdr10", AudioChannels: 6,
	}
	decision := orch.Decide(context.Background(), item, device)

	if decision.TargetVideoCodec != "h264" {
		t.Errorf("expected h264 target, got %s", decision.TargetVideoCodec)
	}
	if !decision.ToneMapped {
		t.Error("expected tone mapping on SDR device")
	}
	if decision.TargetAudioCodec != "aac" {
		t.Errorf("expected aac audio fallback, got %s", decision.TargetAudioCodec)
	}
}

func TestSDRDevice_HEVCDVP8_EAC3(t *testing.T) {
	// HEVC DV P8 / E-AC-3 -> device supports HEVC, no HDR display
	// Expected: HEVC SDR (tonemap) / E-AC-3
	orch := testOrchestrator(swEncoders)
	item := &MediaItem{
		VideoCodec: "hevc", AudioCodec: "eac3", Container: "mp4",
		Width: 3840, Height: 2160, Bitrate: 30000000,
		IsHDR: true, HDRType: "dolby_vision", IsDolbyVision: true,
		DolbyVisionProfile: "8", AudioChannels: 6,
	}
	decision := orch.Decide(context.Background(), item, sdrDevice())

	if decision.TargetVideoCodec != "hevc" {
		t.Errorf("expected hevc target, got %s", decision.TargetVideoCodec)
	}
	if !decision.ToneMapped {
		t.Error("expected tone mapping on SDR device")
	}
}

// --- Server Constraint Tests ---

func TestServerConstraint_NVENC_HEVC(t *testing.T) {
	// AV1 4K -> device supports HEVC (SDR device) -> server has hevc_nvenc (4K OK)
	// Expected: HEVC SDR HW encode + tonemap
	orch := testOrchestrator(nvencEncoders)
	item := &MediaItem{
		VideoCodec: "av1", AudioCodec: "aac", Container: "mkv",
		Width: 3840, Height: 2160, Bitrate: 20000000,
		IsHDR: true, HDRType: "hdr10", AudioChannels: 2,
	}
	decision := orch.Decide(context.Background(), item, sdrDevice())

	if decision.TargetVideoCodec != "hevc" {
		t.Errorf("expected hevc target, got %s", decision.TargetVideoCodec)
	}
	if decision.EncoderUsed != "hevc_nvenc" && decision.EncoderUsed != "libx265" {
		t.Errorf("expected HW or SW hevc encoder, got %s", decision.EncoderUsed)
	}
}

func TestServerConstraint_SWOnly_HEVC(t *testing.T) {
	// AV1 4K -> device supports HEVC (SDR device) -> server has libx265 only
	// Expected: HEVC SDR SW
	orch := testOrchestrator(swEncoders)
	item := &MediaItem{
		VideoCodec: "av1", AudioCodec: "aac", Container: "mkv",
		Width: 3840, Height: 2160, Bitrate: 20000000,
		IsHDR: true, HDRType: "hdr10", AudioChannels: 2,
	}
	decision := orch.Decide(context.Background(), item, sdrDevice())

	if decision.TargetVideoCodec != "hevc" {
		t.Errorf("expected hevc, got %s", decision.TargetVideoCodec)
	}
	if decision.EncoderUsed != "libx265" {
		t.Errorf("expected libx265 SW encoder, got %s", decision.EncoderUsed)
	}
}

func TestServerConstraint_NoHEVC_Fallback(t *testing.T) {
	// AV1 4K -> device supports HEVC (SDR device) -> server has NO HEVC encoder
	// Expected: H.264 SDR fallback
	orch := testOrchestrator(h264OnlyEncoders)
	item := &MediaItem{
		VideoCodec: "av1", AudioCodec: "aac", Container: "mkv",
		Width: 3840, Height: 2160, Bitrate: 20000000,
		IsHDR: true, HDRType: "hdr10", AudioChannels: 2,
	}
	device := sdrDevice()
	device.VideoCodecs = []string{"h264", "hevc"} // Device supports HEVC but server can't encode it
	decision := orch.Decide(context.Background(), item, device)

	if decision.TargetVideoCodec != "h264" {
		t.Errorf("expected h264 fallback, got %s", decision.TargetVideoCodec)
	}
}

func TestServerConstraint_NVENCHDRPassthrough(t *testing.T) {
	// AV1 4K HDR -> device supports HEVC (HDR device) -> server has hevc_nvenc (HDR OK)
	// Expected: HEVC HDR10 HW passthrough
	orch := testOrchestrator(nvencEncoders)
	item := &MediaItem{
		VideoCodec: "av1", AudioCodec: "aac", Container: "mkv",
		Width: 3840, Height: 2160, Bitrate: 30000000,
		IsHDR: true, HDRType: "hdr10", AudioChannels: 2,
	}
	decision := orch.Decide(context.Background(), item, hdrDevice())

	if decision.TargetVideoCodec != "hevc" {
		t.Errorf("expected hevc target, got %s", decision.TargetVideoCodec)
	}
	if !decision.HDRPreserved {
		t.Error("expected HDR preserved on HDR device with capable encoder")
	}
	if decision.ToneMapped {
		t.Error("should NOT tone-map on HDR device")
	}
}

// --- Direct Play Tests ---

func TestDirectPlay_FullyCompatible(t *testing.T) {
	orch := testOrchestrator(swEncoders)
	item := &MediaItem{
		VideoCodec: "h264", AudioCodec: "aac", Container: "mp4",
		Width: 1920, Height: 1080, Bitrate: 8000000,
		AudioChannels: 2,
	}
	decision := orch.Decide(context.Background(), item, sdrDevice())

	if decision.Type != DecisionDirectPlay {
		t.Errorf("expected direct_play, got %s (reasons: %v)", decision.Type, decision.Reasons)
	}
}

func TestDirectPlay_LowTrust(t *testing.T) {
	orch := testOrchestrator(swEncoders)
	device := sdrDevice()
	device.TrustScore = 0.3 // Below MinTrustForDirectPlay (0.6)
	item := &MediaItem{
		VideoCodec: "h264", AudioCodec: "aac", Container: "mp4",
		Width: 1920, Height: 1080, Bitrate: 8000000,
		AudioChannels: 2,
	}
	decision := orch.Decide(context.Background(), item, device)

	if decision.Type == DecisionDirectPlay {
		t.Error("should NOT direct-play with low trust score")
	}
}

// --- Remux Tests ---

func TestRemux_ContainerIncompat(t *testing.T) {
	orch := testOrchestrator(swEncoders)
	item := &MediaItem{
		VideoCodec: "h264", AudioCodec: "aac", Container: "avi",
		Width: 1920, Height: 1080, Bitrate: 8000000,
		AudioChannels: 2,
	}
	decision := orch.Decide(context.Background(), item, sdrDevice())

	if decision.Type != DecisionRemux {
		t.Errorf("expected remux (container change), got %s (reasons: %v)", decision.Type, decision.Reasons)
	}
	if decision.TargetContainer == "avi" {
		t.Error("should have changed container from avi")
	}
}

// --- Subtitle Tests ---

func TestSubtitle_TextSRT(t *testing.T) {
	sub := DecideSubtitle("srt", false, SubtitleConfig{})
	if sub.Action != SubtitlePassthrough {
		t.Errorf("expected passthrough for SRT, got %s", sub.Action)
	}
	if sub.OutputFormat != "vtt" {
		t.Errorf("expected vtt output, got %s", sub.OutputFormat)
	}
	if sub.ForcesBurnIn {
		t.Error("SRT should never force burn-in")
	}
}

func TestSubtitle_StyledASS_NoBurnIn(t *testing.T) {
	sub := DecideSubtitle("ass", false, SubtitleConfig{BurnInStyledText: false})
	if sub.Action != SubtitleConvertToVTT {
		t.Errorf("expected convert_to_vtt, got %s", sub.Action)
	}
}

func TestSubtitle_StyledASS_BurnIn(t *testing.T) {
	sub := DecideSubtitle("ass", false, SubtitleConfig{BurnInStyledText: true})
	if sub.Action != SubtitleBurnIn {
		t.Errorf("expected burn_in, got %s", sub.Action)
	}
	if !sub.ForcesBurnIn {
		t.Error("ASS burn-in should force video transcode")
	}
}

func TestSubtitle_GraphicalPGS_DeviceCant(t *testing.T) {
	sub := DecideSubtitle("pgs", false, SubtitleConfig{})
	if sub.Action != SubtitleBurnIn {
		t.Errorf("expected burn_in for PGS, got %s", sub.Action)
	}
	if !sub.ForcesBurnIn {
		t.Error("PGS burn-in should force video transcode")
	}
}

func TestSubtitle_GraphicalPGS_DeviceCan(t *testing.T) {
	sub := DecideSubtitle("pgs", true, SubtitleConfig{})
	if sub.Action != SubtitlePassthrough {
		t.Errorf("expected passthrough for PGS on capable device, got %s", sub.Action)
	}
}

func TestSubtitle_ForcesTranscode(t *testing.T) {
	// PGS subtitle on a device that can't render it should force video transcode
	orch := testOrchestrator(swEncoders)
	device := sdrDevice()
	item := &MediaItem{
		VideoCodec: "h264", AudioCodec: "aac", Container: "mp4",
		Width: 1920, Height: 1080, Bitrate: 8000000,
		AudioChannels: 2, SubtitleFormat: "pgs",
	}
	decision := orch.Decide(context.Background(), item, device)

	// Should NOT direct play because PGS forces burn-in
	if decision.Type == DecisionDirectPlay {
		t.Error("PGS should prevent direct play when device can't render it")
	}
}

// --- Audio Ladder Tests ---

func TestAudioLadder_TrueHD_to_EAC3(t *testing.T) {
	// TrueHD Atmos 7.1 -> device supports E-AC-3 -> E-AC-3 7.1 (NOT AAC stereo)
	orch := testOrchestrator(swEncoders)
	device := sdrDevice()
	item := &MediaItem{
		VideoCodec: "h264", AudioCodec: "truehd", Container: "mkv",
		Width: 1920, Height: 1080, Bitrate: 8000000,
		AudioChannels: 8, IsDolbyAtmos: true,
	}
	decision := orch.Decide(context.Background(), item, device)

	if decision.TargetAudioCodec != "eac3" {
		t.Errorf("expected eac3 (preserve channels), got %s", decision.TargetAudioCodec)
	}
	if decision.TargetAudioChannels < 6 {
		t.Errorf("expected at least 6 channels, got %d", decision.TargetAudioChannels)
	}
}

func TestAudioLadder_DTS_to_AAC(t *testing.T) {
	// DTS-HD 7.1 -> device supports only AAC -> AAC 5.1 (not stereo)
	orch := testOrchestrator(swEncoders)
	device := sdrDevice()
	device.AudioCodecs = []string{"aac"} // Only AAC
	item := &MediaItem{
		VideoCodec: "h264", AudioCodec: "dts", Container: "mkv",
		Width: 1920, Height: 1080, Bitrate: 8000000,
		AudioChannels: 8,
	}
	decision := orch.Decide(context.Background(), item, device)

	if decision.TargetAudioCodec != "aac" {
		t.Errorf("expected aac, got %s", decision.TargetAudioCodec)
	}
	// Should preserve channels as much as possible
	if decision.TargetAudioChannels < 6 {
		t.Errorf("expected at least 6 channels (AAC 5.1), got %d", decision.TargetAudioChannels)
	}
}

// --- Codec Ladder Tests ---

func TestVideoCodecLadder(t *testing.T) {
	tests := []struct {
		source   string
		expected []string
	}{
		{"av1", []string{"hevc", "vp9", "h264"}},
		{"hevc", []string{"hevc", "vp9", "h264"}},
		{"vp9", []string{"hevc", "vp9", "h264"}},
		{"h264", []string{"h264"}},
		{"mpeg2", []string{"h264"}},
		{"unknown", []string{"h264"}},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			ladder := GetVideoCodecLadder(tt.source)
			if len(ladder) != len(tt.expected) {
				t.Fatalf("expected %d entries, got %d", len(tt.expected), len(ladder))
			}
			for i, exp := range tt.expected {
				if ladder[i] != exp {
					t.Errorf("index %d: expected %s, got %s", i, exp, ladder[i])
				}
			}
		})
	}
}

// --- Legacy Content Tests ---

func TestLegacy_InterlacedContent(t *testing.T) {
	item := &MediaItem{Width: 1920, Height: 1080, Interlaced: true}
	flags := DetectLegacyContent(item)

	if !flags.IsInterlaced {
		t.Error("should detect interlaced content")
	}
	filters := BuildLegacyFilterArgs(flags)
	found := false
	for _, f := range filters {
		if f == "bwdif=mode=send_frame:parity=auto:deint=all" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected bwdif deinterlace filter, got %v", filters)
	}
}

func TestLegacy_SDContent(t *testing.T) {
	item := &MediaItem{Width: 720, Height: 480}
	flags := DetectLegacyContent(item)

	if !flags.IsSD {
		t.Error("should detect SD content")
	}
	if flags.ColorSpace != "bt601" {
		t.Errorf("SD content should use bt601, got %s", flags.ColorSpace)
	}
}

func TestLegacy_AnamorphicDVD(t *testing.T) {
	item := &MediaItem{Width: 720, Height: 480, PixelAspectRatio: 1.185}
	flags := DetectLegacyContent(item)

	if !flags.IsAnamorphic {
		t.Error("should detect anamorphic content")
	}
}

// --- FFmpeg Args Tests ---

func TestFFmpegArgs_DirectPlay(t *testing.T) {
	decision := &PlaybackDecision{Type: DecisionDirectPlay}
	args, err := BuildFFmpegArgs(decision, nil, DefaultQualityPolicy())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args != nil {
		t.Error("direct play should not produce FFmpeg args")
	}
}

func TestFFmpegArgs_ToneMapping_High(t *testing.T) {
	decision := &PlaybackDecision{
		Type:               DecisionTranscode,
		NeedsVideoTranscode: true,
		SourceIsHDR:        true,
		ToneMapped:         true,
		TargetVideoCodec:   "hevc",
		TargetContainer:    "mp4",
	}
	encoder := &server.EncoderCapability{
		EncoderName: "libx265",
		Supports10Bit: true,
	}
	policy := DefaultQualityPolicy()
	policy.ToneMappingQuality = ToneMappingHigh

	args, err := BuildFFmpegArgs(decision, encoder, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args == nil {
		t.Fatal("expected FFmpeg args for transcode")
	}
	if args.VideoEncoder != "libx265" {
		t.Errorf("expected libx265, got %s", args.VideoEncoder)
	}

	// Should have libplacebo tone mapping filter
	foundToneMap := false
	for _, f := range args.VideoFilters {
		if len(f) > 10 && f[:10] == "libplacebo" {
			foundToneMap = true
		}
	}
	if !foundToneMap {
		t.Errorf("expected libplacebo filter for high-quality tone mapping, got filters: %v", args.VideoFilters)
	}

	// Output should be BT.709
	foundBT709 := false
	for i, a := range args.VideoArgs {
		if a == "-color_primaries" && i+1 < len(args.VideoArgs) && args.VideoArgs[i+1] == "bt709" {
			foundBT709 = true
		}
	}
	if !foundBT709 {
		t.Error("tone-mapped output should have BT.709 color primaries")
	}
}

func TestFFmpegArgs_HDRPassthrough(t *testing.T) {
	decision := &PlaybackDecision{
		Type:                DecisionTranscode,
		NeedsVideoTranscode: true,
		SourceIsHDR:         true,
		HDRPreserved:        true,
		TargetVideoCodec:    "hevc",
		TargetContainer:     "mp4",
	}
	encoder := &server.EncoderCapability{
		EncoderName:            "hevc_nvenc",
		IsHardware:             true,
		Supports10Bit:          true,
		SupportsHDRPassthrough: true,
	}

	args, err := BuildFFmpegArgs(decision, encoder, DefaultQualityPolicy())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args == nil {
		t.Fatal("expected FFmpeg args")
	}

	// Should have BT.2020 metadata
	foundBT2020 := false
	for i, a := range args.VideoArgs {
		if a == "-color_primaries" && i+1 < len(args.VideoArgs) && args.VideoArgs[i+1] == "bt2020" {
			foundBT2020 = true
		}
	}
	if !foundBT2020 {
		t.Error("HDR passthrough should have BT.2020 color primaries")
	}

	// Should use 10-bit pixel format
	foundP010 := false
	for i, a := range args.VideoArgs {
		if a == "-pix_fmt" && i+1 < len(args.VideoArgs) && args.VideoArgs[i+1] == "p010le" {
			foundP010 = true
		}
	}
	if !foundP010 {
		t.Error("HDR passthrough should use p010le pixel format")
	}
}

// --- Decision Logger Tests ---

func TestDecisionLogger(t *testing.T) {
	dl := NewDecisionLogger(nil)
	decision := &PlaybackDecision{
		Type:             DecisionTranscode,
		SourceVideoCodec: "av1",
		TargetVideoCodec: "hevc",
		ToneMapped:       true,
	}
	logEntry := dl.Log(context.Background(), decision, "device-123", "media-456")

	if logEntry.DecisionType != "transcode" {
		t.Errorf("expected transcode, got %s", logEntry.DecisionType)
	}
	if logEntry.DeviceID != "device-123" {
		t.Errorf("expected device-123, got %s", logEntry.DeviceID)
	}
	if logEntry.MediaItemID != "media-456" {
		t.Errorf("expected media-456, got %s", logEntry.MediaItemID)
	}
	if !logEntry.ToneMapped {
		t.Error("expected tone_mapped in log")
	}
	if logEntry.ID == "" {
		t.Error("expected non-empty log ID")
	}
}

func TestDecisionLoggerQuery(t *testing.T) {
	dl := NewDecisionLogger(nil)
	ctx := context.Background()

	// Log several decisions
	dl.Log(ctx, &PlaybackDecision{Type: DecisionDirectPlay, DeviceCapabilityTrust: 0.8}, "dev-1", "media-1")
	dl.Log(ctx, &PlaybackDecision{Type: DecisionTranscode, ToneMapped: true, DeviceCapabilityTrust: 0.7}, "dev-2", "media-2")
	dl.Log(ctx, &PlaybackDecision{Type: DecisionTranscode, HDRPreserved: true, HardwareAccelUsed: true, DeviceCapabilityTrust: 0.9}, "dev-1", "media-3")

	// Query all
	all := dl.Query(DecisionQuery{})
	if len(all) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(all))
	}

	// Query by device
	dev1 := dl.Query(DecisionQuery{DeviceID: "dev-1"})
	if len(dev1) != 2 {
		t.Fatalf("expected 2 decisions for dev-1, got %d", len(dev1))
	}

	// Query with limit
	limited := dl.Query(DecisionQuery{Limit: 1})
	if len(limited) != 1 {
		t.Fatalf("expected 1 decision with limit=1, got %d", len(limited))
	}
}

func TestDecisionLoggerStats(t *testing.T) {
	dl := NewDecisionLogger(nil)
	ctx := context.Background()

	dl.Log(ctx, &PlaybackDecision{Type: DecisionDirectPlay, DeviceCapabilityTrust: 0.8}, "dev-1", "m-1")
	dl.Log(ctx, &PlaybackDecision{Type: DecisionDirectPlay, DeviceCapabilityTrust: 0.9}, "dev-1", "m-2")
	dl.Log(ctx, &PlaybackDecision{Type: DecisionTranscode, ToneMapped: true, HardwareAccelUsed: true, DeviceCapabilityTrust: 0.7}, "dev-2", "m-3")
	dl.Log(ctx, &PlaybackDecision{Type: DecisionFallback, DeviceCapabilityTrust: 0.3}, "dev-3", "m-4")

	stats := dl.Stats()

	if stats.TotalDecisions != 4 {
		t.Errorf("expected 4 total, got %d", stats.TotalDecisions)
	}
	if stats.DirectPlayCount != 2 {
		t.Errorf("expected 2 direct play, got %d", stats.DirectPlayCount)
	}
	if stats.DirectPlayPercent != 50 {
		t.Errorf("expected 50%% direct play, got %.1f%%", stats.DirectPlayPercent)
	}
	if stats.TranscodeCount != 1 {
		t.Errorf("expected 1 transcode, got %d", stats.TranscodeCount)
	}
	if stats.FallbackCount != 1 {
		t.Errorf("expected 1 fallback, got %d", stats.FallbackCount)
	}
	if stats.HWAccelPercent != 25 {
		t.Errorf("expected 25%% HW accel, got %.1f%%", stats.HWAccelPercent)
	}
}

func TestDecisionLoggerUpdateOutcome(t *testing.T) {
	dl := NewDecisionLogger(nil)
	ctx := context.Background()

	entry := dl.Log(ctx, &PlaybackDecision{Type: DecisionDirectPlay}, "dev-1", "m-1")

	// Update outcome
	found := dl.UpdateOutcome(entry.ID, true, "")
	if !found {
		t.Fatal("should find entry to update")
	}

	// Verify update
	updated, ok := dl.GetByID(entry.ID)
	if !ok {
		t.Fatal("should find entry by ID")
	}
	if updated.PlaybackSucceeded == nil || !*updated.PlaybackSucceeded {
		t.Error("expected playback succeeded = true")
	}

	// Update with failure
	entry2 := dl.Log(ctx, &PlaybackDecision{Type: DecisionTranscode}, "dev-1", "m-2")
	dl.UpdateOutcome(entry2.ID, false, "codec_unsupported")

	updated2, _ := dl.GetByID(entry2.ID)
	if updated2.PlaybackSucceeded == nil || *updated2.PlaybackSucceeded {
		t.Error("expected playback failed")
	}
	if updated2.FailureReason != "codec_unsupported" {
		t.Errorf("expected failure reason, got %s", updated2.FailureReason)
	}

	// Stats should reflect outcomes
	stats := dl.Stats()
	if stats.SuccessRate != 50 {
		t.Errorf("expected 50%% success rate, got %.1f%%", stats.SuccessRate)
	}
}
