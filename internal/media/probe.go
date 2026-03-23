package media

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
)

// CommandRunner interface for testing (Rule 10 pattern)
type CommandRunner interface {
    Run(ctx context.Context, args ...string) ([]byte, error)
}

// DefaultCommandRunner uses os/exec
type DefaultCommandRunner struct{}

func (r *DefaultCommandRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
    cmd := exec.CommandContext(ctx, args[0], args[1:]...)
    cmd.Stderr = os.Stderr
    return cmd.Output()
}

// FFprobe handles ffprobe metadata extraction
type FFprobe struct {
    path   string
    runner CommandRunner
}

// NewFFprobe creates a new FFprobe instance
func NewFFprobe(path string, runner CommandRunner) *FFprobe {
    if path == "" {
        path = "ffprobe"
    }
    return &FFprobe{
        path:   path,
        runner: runner,
    }
}

// ffprobeOutput represents ffprobe JSON output
type ffprobeOutput struct {
    Format  ffprobeFormat  `json:"format"`
    Streams []ffprobeStream `json:"streams"`
}

type ffprobeFormat struct {
    Filename string            `json:"filename"`
    Duration string            `json:"duration"`
    Size     string            `json:"size"`
    BitRate  string            `json:"bit_rate"`
    Tags     map[string]string `json:"tags"`
}

type ffprobeStream struct {
    Index              int               `json:"index"`
    CodecType          string            `json:"codec_type"`
    CodecName          string            `json:"codec_name"`
    Profile            string            `json:"profile"`
    Level              int               `json:"level"`
    Width              int               `json:"width"`
    Height             int               `json:"height"`
    CodedWidth         int               `json:"coded_width"`
    CodedHeight        int               `json:"coded_height"`
    FrameRate          string            `json:"r_frame_rate"`
    AvgFrameRate       string            `json:"avg_frame_rate"`
    TimeBase           string            `json:"time_base"`
    BitRate            string            `json:"bit_rate"`
    BitsPerRawSample   string            `json:"bits_per_raw_sample"`
    PixelFormat        string            `json:"pix_fmt"`
    ColorPrimaries     string            `json:"color_primaries"`
    ColorTransfer      string            `json:"color_transfer"`
    ColorSpace         string            `json:"color_space"`
    SampleRate         string            `json:"sample_rate"`
    Channels           int               `json:"channels"`
    ChannelLayout      string            `json:"channel_layout"`
    Language           string            `json:"language"`
    Title              string            `json:"title"`
    Tags               map[string]string `json:"tags"`
    ExtradataSize      int               `json:"extradata_size"`
    Disposition        int               `json:"disposition"`
    FieldOrder         string            `json:"field_order"`
}

// Probe extracts metadata from a media file using ffprobe
func (f *FFprobe) Probe(ctx context.Context, path string) (*MediaItem, error) {
    args := []string{
        "-v", "quiet",
        "-print_format", "json",
        "-show_format",
        "-show_streams",
        path,
    }

    out, err := f.runner.Run(ctx, append([]string{f.path}, args...)...)
    if err != nil {
        return nil, fmt.Errorf("ffprobe failed: %w", err)
    }

    var result ffprobeOutput
    if err := json.Unmarshal(out, &result); err != nil {
        return nil, fmt.Errorf("parse ffprobe output: %w", err)
    }

    return f.convertToMediaItem(path, &result)
}

func (f *FFprobe) convertToMediaItem(path string, result *ffprobeOutput) (*MediaItem, error) {
    item := &MediaItem{
        Path:             path,
        AudioStreams:     []AudioStream{},
        SubtitleStreams:  []SubtitleStream{},
    }

    // Parse file info
    if result.Format.Duration != "" {
        item.Duration, _ = strconv.ParseFloat(result.Format.Duration, 64)
    }

    if result.Format.Size != "" {
        item.FileSize, _ = strconv.ParseInt(result.Format.Size, 10, 64)
    }

    // Get file stats
    stat, err := os.Stat(path)
    if err == nil {
        item.FileModifiedAt = stat.ModTime()
    }

    // Parse streams
    for _, stream := range result.Streams {
        switch stream.CodecType {
        case "video":
            vs := f.parseVideoStream(stream)
            item.VideoStream = &vs
        case "audio":
            as := f.parseAudioStream(stream)
            item.AudioStreams = append(item.AudioStreams, as)
        case "subtitle":
            ss := f.parseSubtitleStream(stream)
            item.SubtitleStreams = append(item.SubtitleStreams, ss)
        }
    }

    // Set container from extension
    item.Container = strings.TrimPrefix(filepath.Ext(path), ".")

    // Generate title from filename
    item.Title = f.extractTitle(path)

    // Generate ID from path hash
    item.ID = generateID(path)

    return item, nil
}

func (f *FFprobe) parseVideoStream(s ffprobeStream) VideoStream {
    vs := VideoStream{
        Index:     s.Index,
        Codec:     s.CodecName,
        Profile:   s.Profile,
        Width:     s.Width,
        Height:    s.Height,
        IsInterlaced: s.FieldOrder != "" && s.FieldOrder != "progressive",
    }

    // Parse level
    if s.Level > 0 {
        vs.Level = fmt.Sprintf("%.1f", float64(s.Level)/10.0)
    }

    // Parse framerate
    if parts := strings.Split(s.AvgFrameRate, "/"); len(parts) == 2 {
        var num, denom float64
        fmt.Sscanf(parts[0], "%f", &num)
        fmt.Sscanf(parts[1], "%f", &denom)
        if denom > 0 {
            vs.Framerate = num / denom
        }
    }

    // Parse bit depth
    if s.BitsPerRawSample != "" {
        vs.BitDepth, _ = strconv.Atoi(s.BitsPerRawSample)
    }

    // Detect HDR type
    vs.HDRType = f.detectHDR(s.ColorTransfer, s.PixelFormat)

    // Parse bitrate
    if s.BitRate != "" {
        vs.Bitrate, _ = strconv.ParseInt(s.BitRate, 10, 64)
    }

    return vs
}

func (f *FFprobe) parseAudioStream(s ffprobeStream) AudioStream {
    as := AudioStream{
        Index:    s.Index,
        Codec:    s.CodecName,
        Language: f.getLanguage(s.Tags),
        Title:    s.Tags["title"],
        IsDefault: s.Disposition&1 != 0,
    }

    if s.Channels > 0 {
        as.Channels = s.Channels
    } else if s.ChannelLayout != "" {
        as.Channels = channelLayoutToChannels(s.ChannelLayout)
    }

    if s.SampleRate != "" {
        as.SampleRate, _ = strconv.Atoi(s.SampleRate)
    }

    if s.BitRate != "" {
        as.Bitrate, _ = strconv.ParseInt(s.BitRate, 10, 64)
    }

    return as
}

func (f *FFprobe) parseSubtitleStream(s ffprobeStream) SubtitleStream {
    ss := SubtitleStream{
        Index:    s.Index,
        Language: f.getLanguage(s.Tags),
        Title:    s.Tags["title"],
        IsForced: s.Disposition&2 != 0,
    }

    // Map codec to format
    ss.Format = codecToSubtitleFormat(s.CodecName)

    return ss
}

func (f *FFprobe) detectHDR(colorTransfer, pixelFormat string) string {
    switch colorTransfer {
    case "smpte2084":
        if strings.Contains(pixelFormat, "dolby") {
            return "dolby_vision"
        }
        return "hdr10"
    case "smpte2086":
        return "hdr10+"
    case "hlg":
        return "hlg"
    }
    return ""
}

func (f *FFprobe) getLanguage(tags map[string]string) string {
    if lang, ok := tags["language"]; ok {
        return lang
    }
    return ""
}

func (f *FFprobe) extractTitle(path string) string {
    filename := filepath.Base(path)
    // Remove extension
    filename = strings.TrimSuffix(filename, filepath.Ext(filename))
    // Remove common release group patterns
    filename = cleanFilename(filename)
    // Replace dots and underscores with spaces
    filename = strings.ReplaceAll(filename, ".", " ")
    filename = strings.ReplaceAll(filename, "_", " ")
    return strings.TrimSpace(filename)
}

// Helper to generate ID from path
func generateID(path string) string {
    h := uint64(0)
    for i, c := range path {
        h = h*31 + uint64(c) + uint64(i)
    }
    return fmt.Sprintf("%016x", h)
}

// cleanFilename removes common release group patterns
func cleanFilename(filename string) string {
    // Remove common patterns at the end
    patterns := []string{
        "-GROUP", "_GROUP", "-RG", "_RG",
        "-SiMPLE", "_SiMPLE",
        "-SPARKS", "_SPARKS",
        "-GHOST", "_GHOST",
        "- flux", "_flux",
    }
    for _, p := range patterns {
        filename = strings.TrimSuffix(filename, p)
    }
    return filename
}

// channelLayoutToChannels converts ALSA channel layout to channel count
func channelLayoutToChannels(layout string) int {
    switch strings.ToLower(layout) {
    case "mono":
        return 1
    case "stereo":
        return 2
    case "5.1", "5.1(side)":
        return 6
    case "7.1", "7.1(side)":
        return 8
    default:
        return 2
    }
}

// codecToSubtitleFormat maps FFmpeg codec names to standard formats
func codecToSubtitleFormat(codec string) string {
    switch codec {
    case "subrip":
        return "srt"
    case "ass", "ssa":
        return codec
    case "hdmv_pgs_subtitle":
        return "pgs"
    case "dvd_subtitle":
        return "vobsub"
    case "webvtt":
        return "webvtt"
    case "mov_text":
        return "mov_text"
    default:
        return codec
    }
}
