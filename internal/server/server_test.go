// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// MockPathFinder implements FFmpegPathFinder for testing.
type MockPathFinder struct {
	path string
	err  error
}

func (m *MockPathFinder) LookPath(_ string) (string, error) {
	return m.path, m.err
}

// MockRunner implements CommandRunner for testing.
type MockRunner struct {
	responses map[string]mockResponse
}

type mockResponse struct {
	stdout []byte
	stderr []byte
	err    error
}

func NewMockRunner() *MockRunner {
	return &MockRunner{responses: make(map[string]mockResponse)}
}

func (m *MockRunner) On(name string, args []string, stdout, stderr string, err error) {
	key := makeKey(name, args)
	m.responses[key] = mockResponse{
		stdout: []byte(stdout),
		stderr: []byte(stderr),
		err:    err,
	}
}

func (m *MockRunner) Run(_ context.Context, name string, args ...string) ([]byte, []byte, error) {
	key := makeKey(name, args)
	if resp, ok := m.responses[key]; ok {
		return resp.stdout, resp.stderr, resp.err
	}
	// Try prefix match for flexibility (e.g., ffmpeg with many args)
	for k, resp := range m.responses {
		if strings.HasPrefix(key, k) {
			return resp.stdout, resp.stderr, resp.err
		}
	}
	return nil, nil, fmt.Errorf("mock: no response for %s", key)
}

func makeKey(name string, args []string) string {
	return name + " " + strings.Join(args, " ")
}

const sampleEncoderOutput = ` Encoders:
 V..... = Video
 A..... = Audio
 S..... = Subtitle
 ------
 V..... libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (codec h264)
 V..... libx265              libx265 H.265 / HEVC (codec hevc)
 V..... libsvtav1            SVT-AV1 (codec av1)
 V..... libvpx-vp9           libvpx VP9 (codec vp9)
 A..... aac                  AAC (Advanced Audio Coding)
 A..... libopus              libopus Opus
`

const sampleDecoderOutput = ` Decoders:
 V..... = Video
 A..... = Audio
 S..... = Subtitle
 ------
 V..... h264                 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10
 V..... hevc                 HEVC (High Efficiency Video Coding)
 V..... av1                  Alliance for Open Media AV1
 V..... vp9                  Google VP9
 A..... aac                  AAC (Advanced Audio Coding)
 A..... flac                 FLAC (Free Lossless Audio Codec)
`

const sampleNVENCEncoderOutput = ` Encoders:
 V..... = Video
 A..... = Audio
 S..... = Subtitle
 ------
 V..... libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (codec h264)
 V..... libx265              libx265 H.265 / HEVC (codec hevc)
 V..... h264_nvenc           NVIDIA NVENC H.264 encoder (codec h264)
 V..... hevc_nvenc           NVIDIA NVENC hevc encoder (codec hevc)
 V..... av1_nvenc            NVIDIA NVENC av1 encoder (codec av1)
 A..... aac                  AAC (Advanced Audio Coding)
`

const sampleHevcNvencHelp = `Encoder hevc_nvenc [NVIDIA NVENC hevc encoder]:
    General capabilities: dr1 delay hardware
    Threading capabilities: none
    Supported pixel formats: yuv420p nv12 p010le yuv444p yuv444p16le bgr0 rgb0 cuda d3d11
`

func setupSWOnlyMock() (*MockRunner, *MockPathFinder) {
	mock := NewMockRunner()
	mock.On("/usr/bin/ffmpeg", []string{"-version"}, "ffmpeg version 7.0.1 Copyright (c) 2000-2024\n", "", nil)
	mock.On("/usr/bin/ffmpeg", []string{"-encoders"}, sampleEncoderOutput, "", nil)
	mock.On("/usr/bin/ffmpeg", []string{"-decoders"}, sampleDecoderOutput, "", nil)
	// All HW probes fail
	mock.On("nvidia-smi", []string{"--query-gpu=name", "--format=csv,noheader"}, "", "", fmt.Errorf("not found"))
	mock.On("/usr/bin/ffmpeg", []string{"-init_hw_device"}, "", "", fmt.Errorf("hw not available"))
	pf := &MockPathFinder{path: "/usr/bin/ffmpeg"}
	return mock, pf
}

func newTestInventory(mock *MockRunner, pf *MockPathFinder) *Inventory {
	inv := NewInventory(mock, nil)
	inv.SetPathFinder(pf)
	inv.SetDeviceExistsFunc(func(_ string) bool { return false })
	return inv
}

func TestDiscoverSoftwareOnly(t *testing.T) {
	mock, pf := setupSWOnlyMock()
	inv := newTestInventory(mock, pf)

	caps, err := inv.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if caps.FFmpegVersion != "7.0.1" {
		t.Errorf("expected version 7.0.1, got %s", caps.FFmpegVersion)
	}
	if caps.FFmpegPath != "/usr/bin/ffmpeg" {
		t.Errorf("expected path /usr/bin/ffmpeg, got %s", caps.FFmpegPath)
	}
	if len(caps.HWAccel) != 0 {
		t.Errorf("expected no HW accel, got %d", len(caps.HWAccel))
	}

	// Should have SW encoders
	if !caps.CanEncode("h264") {
		t.Error("expected h264 encoding capability")
	}
	if !caps.CanEncode("hevc") {
		t.Error("expected hevc encoding capability")
	}
	if !caps.CanEncode("av1") {
		t.Error("expected av1 encoding capability")
	}
	if !caps.CanEncode("vp9") {
		t.Error("expected vp9 encoding capability")
	}

	// Check encoder details
	h264Encs := caps.GetEncoders("h264")
	if len(h264Encs) != 1 {
		t.Fatalf("expected 1 h264 encoder, got %d", len(h264Encs))
	}
	if h264Encs[0].EncoderName != "libx264" {
		t.Errorf("expected libx264, got %s", h264Encs[0].EncoderName)
	}
	if h264Encs[0].IsHardware {
		t.Error("libx264 should not be hardware")
	}
}

func setupNVIDIAMock() (*MockRunner, *MockPathFinder) {
	mock := NewMockRunner()
	mock.On("/usr/bin/ffmpeg", []string{"-version"}, "ffmpeg version 7.0.1 Copyright (c) 2000-2024\n", "", nil)
	mock.On("/usr/bin/ffmpeg", []string{"-encoders"}, sampleNVENCEncoderOutput, "", nil)
	mock.On("/usr/bin/ffmpeg", []string{"-decoders"}, sampleDecoderOutput, "", nil)
	mock.On("nvidia-smi", []string{"--query-gpu=name", "--format=csv,noheader"}, "NVIDIA GeForce RTX 3080\n", "", nil)
	mock.On("/usr/bin/ffmpeg", []string{"-init_hw_device", "cuda=cu", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=0.01", "-f", "null", "-"}, "", "", nil)
	mock.On("/usr/bin/ffmpeg", []string{"-h", "encoder=hevc_nvenc"}, sampleHevcNvencHelp, "", nil)
	// Other HW probes fail
	mock.On("/usr/bin/ffmpeg", []string{"-init_hw_device", "qsv=qs"}, "", "", fmt.Errorf("not available"))
	pf := &MockPathFinder{path: "/usr/bin/ffmpeg"}
	return mock, pf
}

func TestDiscoverNVIDIA(t *testing.T) {
	mock, pf := setupNVIDIAMock()
	inv := newTestInventory(mock, pf)
	inv.SetGOOS("linux")

	caps, err := inv.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(caps.HWAccel) != 1 {
		t.Fatalf("expected 1 HW accel, got %d", len(caps.HWAccel))
	}
	hw := caps.HWAccel[0]
	if hw.Type != HWAccelNVENC {
		t.Errorf("expected NVENC, got %s", hw.Type)
	}
	if hw.DeviceName != "NVIDIA GeForce RTX 3080" {
		t.Errorf("expected RTX 3080, got %s", hw.DeviceName)
	}
	if !hw.Supports10Bit {
		t.Error("expected 10-bit support from NVENC (hevc_nvenc has p010le)")
	}

	// Should have both SW and HW encoders for h264 and hevc
	h264Encs := caps.GetEncoders("h264")
	if len(h264Encs) != 2 {
		t.Fatalf("expected 2 h264 encoders (sw + nvenc), got %d", len(h264Encs))
	}

	hevcEncs := caps.GetEncoders("hevc")
	if len(hevcEncs) != 2 {
		t.Fatalf("expected 2 hevc encoders (sw + nvenc), got %d", len(hevcEncs))
	}

	// AV1 NVENC
	av1Encs := caps.GetEncoders("av1")
	if len(av1Encs) != 1 {
		t.Fatalf("expected 1 av1 encoder (nvenc), got %d", len(av1Encs))
	}
	if !av1Encs[0].IsHardware {
		t.Error("av1_nvenc should be hardware")
	}
}

func setupQSVMock() (*MockRunner, *MockPathFinder) {
	mock := NewMockRunner()
	mock.On("/usr/bin/ffmpeg", []string{"-version"}, "ffmpeg version 6.1.2 Copyright (c) 2000-2024\n", "", nil)
	qsvEncoders := ` Encoders:
 V..... = Video
 A..... = Audio
 S..... = Subtitle
 ------
 V..... libx264              libx264 H.264 / AVC
 V..... libx265              libx265 H.265 / HEVC
 V..... h264_qsv             H.264 / AVC / MPEG-4 AVC (Intel Quick Sync Video acceleration) (codec h264)
 V..... hevc_qsv             HEVC (Intel Quick Sync Video acceleration) (codec hevc)
 A..... aac                  AAC (Advanced Audio Coding)
`
	mock.On("/usr/bin/ffmpeg", []string{"-encoders"}, qsvEncoders, "", nil)
	mock.On("/usr/bin/ffmpeg", []string{"-decoders"}, sampleDecoderOutput, "", nil)
	mock.On("nvidia-smi", []string{"--query-gpu=name", "--format=csv,noheader"}, "", "", fmt.Errorf("not found"))
	mock.On("/usr/bin/ffmpeg", []string{"-init_hw_device", "cuda=cu", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=0.01", "-f", "null", "-"}, "", "", fmt.Errorf("no cuda"))
	mock.On("/usr/bin/ffmpeg", []string{"-init_hw_device", "qsv=qs", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=0.01", "-f", "null", "-"}, "", "", nil)
	pf := &MockPathFinder{path: "/usr/bin/ffmpeg"}
	return mock, pf
}

func TestDiscoverQSV(t *testing.T) {
	mock, pf := setupQSVMock()
	inv := newTestInventory(mock, pf)
	inv.SetGOOS("linux")

	caps, err := inv.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(caps.HWAccel) != 1 {
		t.Fatalf("expected 1 HW accel (QSV), got %d", len(caps.HWAccel))
	}
	if caps.HWAccel[0].Type != HWAccelQSV {
		t.Errorf("expected QSV, got %s", caps.HWAccel[0].Type)
	}

	h264Encs := caps.GetEncoders("h264")
	hasQSV := false
	for _, e := range h264Encs {
		if e.EncoderName == "h264_qsv" {
			hasQSV = true
			break
		}
	}
	if !hasQSV {
		t.Error("expected h264_qsv encoder")
	}
}

func TestDiscoverNoFFmpeg(t *testing.T) {
	mock := NewMockRunner()
	inv := NewInventory(mock, nil)
	inv.SetPathFinder(&MockPathFinder{err: fmt.Errorf("not found")})

	_, err := inv.Discover(context.Background())
	if err == nil {
		t.Fatal("expected error when ffmpeg not found")
	}
}

func TestParseFFmpegCodecList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "standard encoders",
			input:    sampleEncoderOutput,
			expected: []string{"libx264", "libx265", "libsvtav1", "libvpx-vp9", "aac", "libopus"},
		},
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "header only",
			input:    " Encoders:\n V..... = Video\n ------\n",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFFmpegCodecList(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d entries, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("entry %d: expected %q, got %q", i, exp, result[i])
				}
			}
		})
	}
}

func TestEncoderSelectorSWOnly(t *testing.T) {
	caps := &ServerCapabilities{
		EncoderDetails: []EncoderCapability{
			{Codec: "h264", EncoderName: "libx264", Performance: PerformanceEstimate{SpeedRank: 30}},
			{Codec: "hevc", EncoderName: "libx265", Supports10Bit: true, SupportsHDRPassthrough: true, Performance: PerformanceEstimate{SpeedRank: 10}},
			{Codec: "av1", EncoderName: "libsvtav1", Supports10Bit: true, SupportsHDRPassthrough: true, Performance: PerformanceEstimate{SpeedRank: 5}},
		},
	}

	sel := NewEncoderSelector(caps)

	// Basic selection
	enc := sel.SelectEncoderForCodec("h264")
	if enc == nil || enc.EncoderName != "libx264" {
		t.Errorf("expected libx264, got %v", enc)
	}

	enc = sel.SelectEncoderForCodec("hevc")
	if enc == nil || enc.EncoderName != "libx265" {
		t.Errorf("expected libx265, got %v", enc)
	}

	// No VP9 encoder available
	if sel.CanEncodeCodec("vp9") {
		t.Error("should not have vp9 encoder")
	}
}

func TestEncoderSelectorHWPreferred(t *testing.T) {
	caps := &ServerCapabilities{
		EncoderDetails: []EncoderCapability{
			{Codec: "hevc", EncoderName: "libx265", Supports10Bit: true, SupportsHDRPassthrough: true, Performance: PerformanceEstimate{SpeedRank: 10}},
			{Codec: "hevc", EncoderName: "hevc_nvenc", IsHardware: true, Supports10Bit: true, SupportsHDRPassthrough: true, Performance: PerformanceEstimate{SpeedRank: 85, RealtimeAt1080p: true, RealtimeAt4K: true}},
		},
	}

	sel := NewEncoderSelector(caps)

	// Should prefer HW
	enc := sel.SelectEncoderForCodec("hevc")
	if enc == nil || enc.EncoderName != "hevc_nvenc" {
		t.Errorf("expected hevc_nvenc (HW preferred), got %v", enc)
	}
}

func TestEncoderSelectorHDRRequirement(t *testing.T) {
	caps := &ServerCapabilities{
		EncoderDetails: []EncoderCapability{
			{Codec: "hevc", EncoderName: "libx265", Supports10Bit: true, SupportsHDRPassthrough: true, Performance: PerformanceEstimate{SpeedRank: 10}},
			{Codec: "hevc", EncoderName: "hevc_nvenc", IsHardware: true, Supports10Bit: false, SupportsHDRPassthrough: false, Performance: PerformanceEstimate{SpeedRank: 85}},
		},
	}

	sel := NewEncoderSelector(caps)

	// Needs HDR — should pick libx265 since nvenc doesn't support it here
	enc := sel.SelectEncoder("hevc", true, true)
	if enc == nil || enc.EncoderName != "libx265" {
		t.Errorf("expected libx265 for HDR, got %v", enc)
	}

	// No HDR requirement — should pick nvenc (faster)
	enc = sel.SelectEncoder("hevc", false, false)
	if enc == nil || enc.EncoderName != "hevc_nvenc" {
		t.Errorf("expected hevc_nvenc without HDR, got %v", enc)
	}
}

func TestGetCurrent(t *testing.T) {
	mock, pf := setupSWOnlyMock()
	inv := newTestInventory(mock, pf)

	// Before discover
	if inv.GetCurrent() != nil {
		t.Error("expected nil before discover")
	}

	_, err := inv.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// After discover
	caps := inv.GetCurrent()
	if caps == nil {
		t.Fatal("expected non-nil after discover")
	}
	if caps.FFmpegVersion != "7.0.1" {
		t.Errorf("unexpected version: %s", caps.FFmpegVersion)
	}
}

func TestParseFFmpegSpeed(t *testing.T) {
	tests := []struct {
		name     string
		stderr   string
		expected float64
	}{
		{
			name:     "normal speed",
			stderr:   "frame=  150 fps= 75 q=28.0 Lsize=     256kB time=00:00:05.00 speed=2.50x\n",
			expected: 2.5,
		},
		{
			name:     "multiple speed lines (take last)",
			stderr:   "speed=1.20x\nspeed=1.50x\nspeed=1.80x\n",
			expected: 1.8,
		},
		{
			name:     "no speed",
			stderr:   "frame=  150 fps= 75 q=28.0 Lsize=     256kB\n",
			expected: 0,
		},
		{
			name:     "speed with space",
			stderr:   "speed= 3.10x\n",
			expected: 3.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFFmpegSpeed(tt.stderr)
			if result != tt.expected {
				t.Errorf("expected %.2f, got %.2f", tt.expected, result)
			}
		})
	}
}

func TestServerCapabilitiesCanEncode(t *testing.T) {
	caps := &ServerCapabilities{
		EncoderDetails: []EncoderCapability{
			{Codec: "h264", EncoderName: "libx264"},
			{Codec: "hevc", EncoderName: "libx265"},
		},
	}

	if !caps.CanEncode("h264") {
		t.Error("should be able to encode h264")
	}
	if !caps.CanEncode("hevc") {
		t.Error("should be able to encode hevc")
	}
	if caps.CanEncode("av1") {
		t.Error("should NOT be able to encode av1")
	}
	// Case insensitive via codec.Equal
	if !caps.CanEncode("H264") {
		t.Error("should handle case insensitive")
	}
}

func TestHasHWAccel(t *testing.T) {
	caps := &ServerCapabilities{}
	if caps.HasHWAccel() {
		t.Error("no HW accel expected")
	}
	caps.HWAccel = []HardwareAcceleration{{Type: HWAccelNVENC}}
	if !caps.HasHWAccel() {
		t.Error("expected HW accel")
	}
}

func TestParseVAAPIDeviceName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "intel driver",
			input:    "vainfo: Driver version: Intel iHD driver for Intel(R) Gen Graphics - 23.3.2\n",
			expected: "Intel iHD driver for Intel(R) Gen Graphics - 23.3.2",
		},
		{
			name:     "no match",
			input:    "libva info: VA-API version 1.18.0\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVAAPIDeviceName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
