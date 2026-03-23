// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/tenkile/tenkile/pkg/codec"
)

// HWAccelType identifies the hardware acceleration backend.
type HWAccelType int

const (
	HWAccelNone         HWAccelType = iota
	HWAccelNVENC                    // NVIDIA
	HWAccelQSV                      // Intel Quick Sync Video
	HWAccelVAAPI                    // Video Acceleration API (Linux)
	HWAccelVideoToolbox             // macOS
	HWAccelAMF                      // AMD
	HWAccelV4L2                     // Raspberry Pi / embedded Linux
)

func (h HWAccelType) String() string {
	switch h {
	case HWAccelNVENC:
		return "nvenc"
	case HWAccelQSV:
		return "qsv"
	case HWAccelVAAPI:
		return "vaapi"
	case HWAccelVideoToolbox:
		return "videotoolbox"
	case HWAccelAMF:
		return "amf"
	case HWAccelV4L2:
		return "v4l2"
	default:
		return "none"
	}
}

// HardwareAcceleration describes a detected HW acceleration device.
type HardwareAcceleration struct {
	Type                  HWAccelType
	DeviceName            string
	SupportedCodecs       []string
	SupportsHDR           bool
	Supports10Bit         bool
	MaxConcurrentSessions int
}

// PerformanceEstimate describes the real-time encoding performance.
type PerformanceEstimate struct {
	RealtimeAt1080p bool
	RealtimeAt4K    bool
	SpeedRank       int // Higher = faster
}

// EncoderCapability describes what a specific encoder can do.
type EncoderCapability struct {
	Codec                  string // e.g. "h264", "hevc", "av1"
	EncoderName            string // e.g. "libx264", "hevc_nvenc"
	IsHardware             bool
	HWAccelType            HWAccelType
	Supports10Bit          bool
	SupportsHDRPassthrough bool
	SupportedProfiles      []string
	SupportedPixelFormats  []string
	Performance            PerformanceEstimate
}

// ServerCapabilities holds the complete server encoding/decoding inventory.
type ServerCapabilities struct {
	FFmpegVersion     string
	FFmpegPath        string
	AvailableEncoders []string
	AvailableDecoders []string
	HWAccel           []HardwareAcceleration
	EncoderDetails    []EncoderCapability
}

// CanEncode returns true if the server has any encoder for the given codec.
func (sc *ServerCapabilities) CanEncode(codecName string) bool {
	for _, enc := range sc.EncoderDetails {
		if codec.Equal(enc.Codec, codecName) {
			return true
		}
	}
	return false
}

// GetEncoders returns all encoders for a given codec, ordered by preference (HW first).
func (sc *ServerCapabilities) GetEncoders(codecName string) []EncoderCapability {
	var result []EncoderCapability
	for _, enc := range sc.EncoderDetails {
		if codec.Equal(enc.Codec, codecName) {
			result = append(result, enc)
		}
	}
	return result
}

// HasHWAccel checks if any hardware acceleration is available.
func (sc *ServerCapabilities) HasHWAccel() bool {
	return len(sc.HWAccel) > 0
}

// CommandRunner abstracts exec.Command for testing.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)
}

// ExecRunner is the real implementation using os/exec.
type ExecRunner struct{}

func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// Inventory discovers and tracks server encoding capabilities.
type Inventory struct {
	mu         sync.RWMutex
	caps       *ServerCapabilities
	runner     CommandRunner
	pathFinder FFmpegPathFinder
	logger     *slog.Logger
	goos       string // override runtime.GOOS for testing

	// deviceExists checks if a device file exists. Overridable for tests.
	deviceExistsFunc func(path string) bool
}

// NewInventory creates a new server capability inventory.
func NewInventory(runner CommandRunner, logger *slog.Logger) *Inventory {
	if runner == nil {
		runner = &ExecRunner{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Inventory{
		runner:     runner,
		pathFinder: &execPathFinder{},
		logger:     logger,
		goos:       runtime.GOOS,
		deviceExistsFunc: func(path string) bool {
			info, err := os.Stat(path)
			if err != nil {
				return false
			}
			return info.Mode()&os.ModeCharDevice != 0
		},
	}
}

// SetGOOS overrides the detected OS (for testing).
func (inv *Inventory) SetGOOS(goos string) {
	inv.goos = goos
}

// SetPathFinder overrides the FFmpeg path finder (for testing).
func (inv *Inventory) SetPathFinder(pf FFmpegPathFinder) {
	inv.pathFinder = pf
}

// SetDeviceExistsFunc overrides device existence check (for testing).
func (inv *Inventory) SetDeviceExistsFunc(fn func(string) bool) {
	inv.deviceExistsFunc = fn
}

func (inv *Inventory) deviceExists(path string) bool {
	return inv.deviceExistsFunc(path)
}

// Discover probes the system for FFmpeg and hardware acceleration capabilities.
func (inv *Inventory) Discover(ctx context.Context) (*ServerCapabilities, error) {
	caps := &ServerCapabilities{}

	ffmpegPath, err := inv.findFFmpeg(ctx)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found: %w", err)
	}
	caps.FFmpegPath = ffmpegPath

	caps.FFmpegVersion = inv.getFFmpegVersion(ctx, ffmpegPath)
	caps.AvailableEncoders = inv.getAvailableEncoders(ctx, ffmpegPath)
	caps.AvailableDecoders = inv.getAvailableDecoders(ctx, ffmpegPath)
	caps.HWAccel = inv.probeHardwareAcceleration(ctx, ffmpegPath, caps.AvailableEncoders)
	caps.EncoderDetails = inv.buildEncoderDetails(caps.AvailableEncoders, caps.HWAccel)

	inv.mu.Lock()
	inv.caps = caps
	inv.mu.Unlock()

	inv.logger.Info("server capability discovery complete",
		"ffmpeg_version", caps.FFmpegVersion,
		"encoders", len(caps.AvailableEncoders),
		"decoders", len(caps.AvailableDecoders),
		"hw_accel", len(caps.HWAccel),
		"encoder_details", len(caps.EncoderDetails),
	)

	return caps, nil
}

// GetCurrent returns the last discovered capabilities.
func (inv *Inventory) GetCurrent() *ServerCapabilities {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return inv.caps
}

// FFmpegPathFinder abstracts FFmpeg binary lookup for testing.
type FFmpegPathFinder interface {
	LookPath(file string) (string, error)
}

// execPathFinder uses exec.LookPath (cross-platform).
type execPathFinder struct{}

func (e *execPathFinder) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (inv *Inventory) findFFmpeg(_ context.Context) (string, error) {
	path, err := inv.pathFinder.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg binary not found in PATH: %w", err)
	}
	return path, nil
}

func (inv *Inventory) getFFmpegVersion(ctx context.Context, ffmpegPath string) string {
	stdout, _, err := inv.runner.Run(ctx, ffmpegPath, "-version")
	if err != nil {
		inv.logger.Warn("failed to get ffmpeg version", "error", err)
		return "unknown"
	}
	lines := strings.Split(string(stdout), "\n")
	if len(lines) > 0 {
		// First line: "ffmpeg version N.N.N ..."
		re := regexp.MustCompile(`ffmpeg version (\S+)`)
		if m := re.FindStringSubmatch(lines[0]); len(m) > 1 {
			return m[1]
		}
		return lines[0]
	}
	return "unknown"
}

func (inv *Inventory) getAvailableEncoders(ctx context.Context, ffmpegPath string) []string {
	stdout, _, err := inv.runner.Run(ctx, ffmpegPath, "-encoders")
	if err != nil {
		inv.logger.Warn("failed to list encoders", "error", err)
		return nil
	}
	return parseFFmpegCodecList(string(stdout))
}

func (inv *Inventory) getAvailableDecoders(ctx context.Context, ffmpegPath string) []string {
	stdout, _, err := inv.runner.Run(ctx, ffmpegPath, "-decoders")
	if err != nil {
		inv.logger.Warn("failed to list decoders", "error", err)
		return nil
	}
	return parseFFmpegCodecList(string(stdout))
}

// parseFFmpegCodecList parses output of ffmpeg -encoders or -decoders.
// Lines look like: " V..... libx264              ..."
// Header lines like " V..... = Video" are excluded.
func parseFFmpegCodecList(output string) []string {
	var result []string
	re := regexp.MustCompile(`^\s+[VAS][F.][S.][X.][B.][D.]\s+(\S+)`)
	for _, line := range strings.Split(output, "\n") {
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			name := m[1]
			if name == "=" {
				continue // Skip header legend lines
			}
			result = append(result, name)
		}
	}
	return result
}

func (inv *Inventory) probeHardwareAcceleration(ctx context.Context, ffmpegPath string, availableEncoders []string) []HardwareAcceleration {
	var accels []HardwareAcceleration

	switch inv.goos {
	case "linux":
		if accel := inv.probeNVENC(ctx, ffmpegPath, availableEncoders); accel != nil {
			accels = append(accels, *accel)
		}
		if accel := inv.probeVAAPI(ctx, ffmpegPath, availableEncoders); accel != nil {
			accels = append(accels, *accel)
		}
		if accel := inv.probeQSV(ctx, ffmpegPath, availableEncoders); accel != nil {
			accels = append(accels, *accel)
		}
		if accel := inv.probeV4L2(ctx); accel != nil {
			accels = append(accels, *accel)
		}
	case "darwin":
		if accel := inv.probeVideoToolbox(ctx, ffmpegPath, availableEncoders); accel != nil {
			accels = append(accels, *accel)
		}
	case "windows":
		if accel := inv.probeNVENC(ctx, ffmpegPath, availableEncoders); accel != nil {
			accels = append(accels, *accel)
		}
		if accel := inv.probeQSV(ctx, ffmpegPath, availableEncoders); accel != nil {
			accels = append(accels, *accel)
		}
		if accel := inv.probeAMF(ctx, ffmpegPath, availableEncoders); accel != nil {
			accels = append(accels, *accel)
		}
	}

	return accels
}

func (inv *Inventory) probeNVENC(ctx context.Context, ffmpegPath string, availableEncoders []string) *HardwareAcceleration {
	// Check nvidia-smi first
	stdout, _, err := inv.runner.Run(ctx, "nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	if err != nil {
		return nil
	}
	gpuName := strings.TrimSpace(string(stdout))
	if gpuName == "" {
		return nil
	}

	// Verify FFmpeg can init CUDA
	_, _, err = inv.runner.Run(ctx, ffmpegPath, "-init_hw_device", "cuda=cu", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=0.01", "-f", "null", "-")
	if err != nil {
		inv.logger.Debug("nvidia-smi found GPU but ffmpeg cuda init failed", "gpu", gpuName, "error", err)
		return nil
	}

	codecs := detectNVENCCodecs(availableEncoders)
	supports10Bit := inv.checkNVENC10Bit(ctx, ffmpegPath)

	return &HardwareAcceleration{
		Type:                  HWAccelNVENC,
		DeviceName:            gpuName,
		SupportedCodecs:       codecs,
		SupportsHDR:           supports10Bit, // HDR requires 10-bit
		Supports10Bit:         supports10Bit,
		MaxConcurrentSessions: 5, // Default NVENC limit (consumer GPUs)
	}
}

func detectNVENCCodecs(availableEncoders []string) []string {
	nvencMap := map[string]string{
		"h264_nvenc": "h264",
		"hevc_nvenc": "hevc",
		"av1_nvenc":  "av1",
	}
	var codecs []string
	for _, enc := range availableEncoders {
		if codecName, ok := nvencMap[enc]; ok {
			codecs = append(codecs, codecName)
		}
	}
	return codecs
}

func (inv *Inventory) checkNVENC10Bit(ctx context.Context, ffmpegPath string) bool {
	stdout, _, err := inv.runner.Run(ctx, ffmpegPath, "-h", "encoder=hevc_nvenc")
	if err != nil {
		return false
	}
	return strings.Contains(string(stdout), "p010le")
}

func (inv *Inventory) probeVAAPI(ctx context.Context, ffmpegPath string, availableEncoders []string) *HardwareAcceleration {
	// Check for render device
	if !inv.deviceExists("/dev/dri/renderD128") {
		return nil
	}

	// Verify FFmpeg can init VAAPI
	_, _, err := inv.runner.Run(ctx, ffmpegPath, "-init_hw_device", "vaapi=va:/dev/dri/renderD128", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=0.01", "-f", "null", "-")
	if err != nil {
		return nil
	}

	// Try vainfo for details
	var deviceName string
	stdout, _, vainfoErr := inv.runner.Run(ctx, "vainfo", "--display", "drm", "--device", "/dev/dri/renderD128")
	if vainfoErr == nil {
		deviceName = parseVAAPIDeviceName(string(stdout))
	}
	if deviceName == "" {
		deviceName = "VAAPI"
	}

	codecs := detectVAAPICodecs(availableEncoders)

	return &HardwareAcceleration{
		Type:            HWAccelVAAPI,
		DeviceName:      deviceName,
		SupportedCodecs: codecs,
		SupportsHDR:     true,
		Supports10Bit:   true,
	}
}

func parseVAAPIDeviceName(vainfoOutput string) string {
	re := regexp.MustCompile(`(?i)vainfo:\s+Driver version:\s+(.+)`)
	if m := re.FindStringSubmatch(vainfoOutput); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func detectVAAPICodecs(availableEncoders []string) []string {
	vaapiMap := map[string]string{
		"h264_vaapi": "h264",
		"hevc_vaapi": "hevc",
		"vp9_vaapi":  "vp9",
		"av1_vaapi":  "av1",
	}
	var codecs []string
	for _, enc := range availableEncoders {
		if codecName, ok := vaapiMap[enc]; ok {
			codecs = append(codecs, codecName)
		}
	}
	return codecs
}

func (inv *Inventory) probeQSV(ctx context.Context, ffmpegPath string, availableEncoders []string) *HardwareAcceleration {
	_, _, err := inv.runner.Run(ctx, ffmpegPath, "-init_hw_device", "qsv=qs", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=0.01", "-f", "null", "-")
	if err != nil {
		return nil
	}

	codecs := detectQSVCodecs(availableEncoders)
	if len(codecs) == 0 {
		return nil
	}

	return &HardwareAcceleration{
		Type:            HWAccelQSV,
		DeviceName:      "Intel Quick Sync Video",
		SupportedCodecs: codecs,
		SupportsHDR:     true,
		Supports10Bit:   true,
	}
}

func detectQSVCodecs(availableEncoders []string) []string {
	qsvMap := map[string]string{
		"h264_qsv": "h264",
		"hevc_qsv": "hevc",
		"vp9_qsv":  "vp9",
		"av1_qsv":  "av1",
	}
	var codecs []string
	for _, enc := range availableEncoders {
		if codecName, ok := qsvMap[enc]; ok {
			codecs = append(codecs, codecName)
		}
	}
	return codecs
}

func (inv *Inventory) probeVideoToolbox(ctx context.Context, ffmpegPath string, availableEncoders []string) *HardwareAcceleration {
	_, _, err := inv.runner.Run(ctx, ffmpegPath, "-init_hw_device", "videotoolbox=vt", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=0.01", "-f", "null", "-")
	if err != nil {
		return nil
	}

	codecs := detectVideoToolboxCodecs(availableEncoders)
	if len(codecs) == 0 {
		return nil
	}

	return &HardwareAcceleration{
		Type:            HWAccelVideoToolbox,
		DeviceName:      "Apple VideoToolbox",
		SupportedCodecs: codecs,
		SupportsHDR:     true,
		Supports10Bit:   true,
	}
}

func detectVideoToolboxCodecs(availableEncoders []string) []string {
	vtMap := map[string]string{
		"h264_videotoolbox": "h264",
		"hevc_videotoolbox": "hevc",
	}
	var codecs []string
	for _, enc := range availableEncoders {
		if codecName, ok := vtMap[enc]; ok {
			codecs = append(codecs, codecName)
		}
	}
	return codecs
}

func (inv *Inventory) probeAMF(ctx context.Context, ffmpegPath string, availableEncoders []string) *HardwareAcceleration {
	_, _, err := inv.runner.Run(ctx, ffmpegPath, "-init_hw_device", "amf=amf", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=0.01", "-f", "null", "-")
	if err != nil {
		return nil
	}

	codecs := detectAMFCodecs(availableEncoders)
	if len(codecs) == 0 {
		return nil
	}

	return &HardwareAcceleration{
		Type:            HWAccelAMF,
		DeviceName:      "AMD AMF",
		SupportedCodecs: codecs,
		SupportsHDR:     true,
		Supports10Bit:   true,
	}
}

func detectAMFCodecs(availableEncoders []string) []string {
	amfMap := map[string]string{
		"h264_amf": "h264",
		"hevc_amf": "hevc",
		"av1_amf":  "av1",
	}
	var codecs []string
	for _, enc := range availableEncoders {
		if codecName, ok := amfMap[enc]; ok {
			codecs = append(codecs, codecName)
		}
	}
	return codecs
}

func (inv *Inventory) probeV4L2(_ context.Context) *HardwareAcceleration {
	// Check for V4L2 M2M devices (Raspberry Pi)
	if !inv.deviceExists("/dev/video10") && !inv.deviceExists("/dev/video11") {
		return nil
	}

	return &HardwareAcceleration{
		Type:            HWAccelV4L2,
		DeviceName:      "V4L2 M2M",
		SupportedCodecs: []string{"h264"},
		SupportsHDR:     false,
		Supports10Bit:   false,
	}
}

// buildEncoderDetails builds detailed encoder capability entries
// by combining available encoders with detected HW acceleration.
func (inv *Inventory) buildEncoderDetails(availableEncoders []string, hwAccels []HardwareAcceleration) []EncoderCapability {
	var details []EncoderCapability

	// Map of encoder name -> (codec, hwType)
	encoderInfo := map[string]struct {
		codec   string
		hwType  HWAccelType
		isHW    bool
	}{
		// Software encoders
		"libx264":    {codec: "h264", hwType: HWAccelNone, isHW: false},
		"libx265":    {codec: "hevc", hwType: HWAccelNone, isHW: false},
		"libsvtav1":  {codec: "av1", hwType: HWAccelNone, isHW: false},
		"libvpx-vp9": {codec: "vp9", hwType: HWAccelNone, isHW: false},
		"libvpx":     {codec: "vp8", hwType: HWAccelNone, isHW: false},
		// NVENC
		"h264_nvenc": {codec: "h264", hwType: HWAccelNVENC, isHW: true},
		"hevc_nvenc": {codec: "hevc", hwType: HWAccelNVENC, isHW: true},
		"av1_nvenc":  {codec: "av1", hwType: HWAccelNVENC, isHW: true},
		// QSV
		"h264_qsv": {codec: "h264", hwType: HWAccelQSV, isHW: true},
		"hevc_qsv": {codec: "hevc", hwType: HWAccelQSV, isHW: true},
		"vp9_qsv":  {codec: "vp9", hwType: HWAccelQSV, isHW: true},
		"av1_qsv":  {codec: "av1", hwType: HWAccelQSV, isHW: true},
		// VAAPI
		"h264_vaapi": {codec: "h264", hwType: HWAccelVAAPI, isHW: true},
		"hevc_vaapi": {codec: "hevc", hwType: HWAccelVAAPI, isHW: true},
		"vp9_vaapi":  {codec: "vp9", hwType: HWAccelVAAPI, isHW: true},
		"av1_vaapi":  {codec: "av1", hwType: HWAccelVAAPI, isHW: true},
		// VideoToolbox
		"h264_videotoolbox": {codec: "h264", hwType: HWAccelVideoToolbox, isHW: true},
		"hevc_videotoolbox": {codec: "hevc", hwType: HWAccelVideoToolbox, isHW: true},
		// AMF
		"h264_amf": {codec: "h264", hwType: HWAccelAMF, isHW: true},
		"hevc_amf": {codec: "hevc", hwType: HWAccelAMF, isHW: true},
		"av1_amf":  {codec: "av1", hwType: HWAccelAMF, isHW: true},
		// Audio encoders
		"aac":        {codec: "aac", hwType: HWAccelNone, isHW: false},
		"libfdk_aac": {codec: "aac", hwType: HWAccelNone, isHW: false},
		"eac3":       {codec: "eac3", hwType: HWAccelNone, isHW: false},
		"ac3":        {codec: "ac3", hwType: HWAccelNone, isHW: false},
		"libopus":    {codec: "opus", hwType: HWAccelNone, isHW: false},
		"flac":       {codec: "flac", hwType: HWAccelNone, isHW: false},
		"libmp3lame": {codec: "mp3", hwType: HWAccelNone, isHW: false},
		"truehd":     {codec: "truehd", hwType: HWAccelNone, isHW: false},
		"dca":        {codec: "dts", hwType: HWAccelNone, isHW: false},
		"libvorbis":  {codec: "vorbis", hwType: HWAccelNone, isHW: false},
		"alac":       {codec: "alac", hwType: HWAccelNone, isHW: false},
	}

	for _, encName := range availableEncoders {
		info, known := encoderInfo[encName]
		if !known {
			continue
		}

		// If it's a HW encoder, verify we actually detected that HW
		if info.isHW {
			hwFound := false
			for _, hw := range hwAccels {
				if hw.Type == info.hwType {
					hwFound = true
					break
				}
			}
			if !hwFound {
				continue
			}
		}

		cap := EncoderCapability{
			Codec:       info.codec,
			EncoderName: encName,
			IsHardware:  info.isHW,
			HWAccelType: info.hwType,
		}

		// Set capabilities based on known encoder properties
		inv.populateEncoderCapability(&cap, hwAccels)
		details = append(details, cap)
	}

	return details
}

func (inv *Inventory) populateEncoderCapability(cap *EncoderCapability, hwAccels []HardwareAcceleration) {
	switch cap.EncoderName {
	// Software encoders
	case "libx264":
		cap.SupportedProfiles = []string{"Baseline", "Main", "High"}
		cap.SupportedPixelFormats = []string{"yuv420p", "yuv422p", "yuv444p"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: false, SpeedRank: 30}
	case "libx265":
		cap.Supports10Bit = true
		cap.SupportsHDRPassthrough = true
		cap.SupportedProfiles = []string{"Main", "Main 10"}
		cap.SupportedPixelFormats = []string{"yuv420p", "yuv420p10le"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: false, SpeedRank: 10}
	case "libsvtav1":
		cap.Supports10Bit = true
		cap.SupportsHDRPassthrough = true
		cap.SupportedProfiles = []string{"Main"}
		cap.SupportedPixelFormats = []string{"yuv420p", "yuv420p10le"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: false, RealtimeAt4K: false, SpeedRank: 5}
	case "libvpx-vp9":
		cap.Supports10Bit = true
		cap.SupportedProfiles = []string{"0", "2"}
		cap.SupportedPixelFormats = []string{"yuv420p", "yuv420p10le"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: false, RealtimeAt4K: false, SpeedRank: 8}

	// NVENC
	case "h264_nvenc":
		cap.SupportedProfiles = []string{"Baseline", "Main", "High"}
		cap.SupportedPixelFormats = []string{"yuv420p", "nv12"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 90}
	case "hevc_nvenc":
		cap.SupportedProfiles = []string{"Main", "Main 10"}
		cap.SupportedPixelFormats = []string{"yuv420p", "p010le", "nv12"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 85}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelNVENC)
	case "av1_nvenc":
		cap.SupportedProfiles = []string{"Main"}
		cap.SupportedPixelFormats = []string{"yuv420p", "p010le", "nv12"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 80}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelNVENC)

	// QSV
	case "h264_qsv":
		cap.SupportedProfiles = []string{"Baseline", "Main", "High"}
		cap.SupportedPixelFormats = []string{"nv12", "qsv"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 80}
	case "hevc_qsv":
		cap.SupportedProfiles = []string{"Main", "Main 10"}
		cap.SupportedPixelFormats = []string{"nv12", "p010le", "qsv"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 75}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelQSV)
	case "vp9_qsv":
		cap.SupportedProfiles = []string{"0", "2"}
		cap.SupportedPixelFormats = []string{"nv12", "p010le", "qsv"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: false, SpeedRank: 65}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelQSV)
	case "av1_qsv":
		cap.SupportedProfiles = []string{"Main"}
		cap.SupportedPixelFormats = []string{"nv12", "p010le", "qsv"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: false, SpeedRank: 60}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelQSV)

	// VAAPI
	case "h264_vaapi":
		cap.SupportedProfiles = []string{"Main", "High"}
		cap.SupportedPixelFormats = []string{"vaapi"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 75}
	case "hevc_vaapi":
		cap.SupportedProfiles = []string{"Main", "Main 10"}
		cap.SupportedPixelFormats = []string{"vaapi"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 70}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelVAAPI)
	case "vp9_vaapi":
		cap.SupportedProfiles = []string{"0", "2"}
		cap.SupportedPixelFormats = []string{"vaapi"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: false, SpeedRank: 60}
	case "av1_vaapi":
		cap.SupportedProfiles = []string{"Main"}
		cap.SupportedPixelFormats = []string{"vaapi"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: false, SpeedRank: 55}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelVAAPI)

	// VideoToolbox
	case "h264_videotoolbox":
		cap.SupportedProfiles = []string{"Baseline", "Main", "High"}
		cap.SupportedPixelFormats = []string{"nv12", "videotoolbox_vld"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 85}
	case "hevc_videotoolbox":
		cap.SupportedProfiles = []string{"Main", "Main 10"}
		cap.SupportedPixelFormats = []string{"nv12", "p010le", "videotoolbox_vld"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 80}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelVideoToolbox)

	// AMF
	case "h264_amf":
		cap.SupportedProfiles = []string{"Main", "High"}
		cap.SupportedPixelFormats = []string{"nv12", "d3d11"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 75}
	case "hevc_amf":
		cap.SupportedProfiles = []string{"Main", "Main 10"}
		cap.SupportedPixelFormats = []string{"nv12", "p010le", "d3d11"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: true, SpeedRank: 70}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelAMF)
	case "av1_amf":
		cap.SupportedProfiles = []string{"Main"}
		cap.SupportedPixelFormats = []string{"nv12", "p010le", "d3d11"}
		cap.Performance = PerformanceEstimate{RealtimeAt1080p: true, RealtimeAt4K: false, SpeedRank: 60}
		inv.applyHWCapsFromAccel(cap, hwAccels, HWAccelAMF)
	}
}

func (inv *Inventory) applyHWCapsFromAccel(cap *EncoderCapability, hwAccels []HardwareAcceleration, hwType HWAccelType) {
	for _, hw := range hwAccels {
		if hw.Type == hwType {
			cap.Supports10Bit = hw.Supports10Bit
			cap.SupportsHDRPassthrough = hw.SupportsHDR && hw.Supports10Bit
			return
		}
	}
}
