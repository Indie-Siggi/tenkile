// SPDX-License-Identifier: AGPL-3.0-or-later

package transcode

// videoCodecLadder defines quality-ordered fallback chains for video codecs.
// Each source codec has an ordered list of transcode targets, best first.
var videoCodecLadder = map[string][]string{
	"av1":  {"hevc", "vp9", "h264"},
	"hevc": {"hevc", "vp9", "h264"}, // HEVC remux first, then fallback
	"vp9":  {"hevc", "vp9", "h264"},
	"h264": {"h264"},                 // H.264 is already the universal fallback
	"mpeg2": {"h264"},
	"mpeg4": {"h264"},
	"vc1":  {"h264"},
	"vp8":  {"vp9", "h264"},
}

// GetVideoCodecLadder returns the quality-ordered fallback chain for the given source codec.
// The first entry is preferred (highest quality); the last is the absolute fallback.
// Returns a copy so callers cannot mutate the global ladder.
func GetVideoCodecLadder(sourceCodec string) []string {
	if ladder, ok := videoCodecLadder[sourceCodec]; ok {
		out := make([]string, len(ladder))
		copy(out, ladder)
		return out
	}
	// Unknown codec: fallback to H.264
	return []string{"h264"}
}

// audioCodecLadder defines quality-ordered fallback chains for audio codecs.
// Key principle: preserve channel count as long as possible.
var audioCodecLadder = map[string][]audioTarget{
	"truehd": {
		{Codec: "truehd", MaxChannels: 8},
		{Codec: "eac3", MaxChannels: 8},  // E-AC-3 JOC (Atmos) 7.1
		{Codec: "eac3", MaxChannels: 6},  // E-AC-3 5.1
		{Codec: "ac3", MaxChannels: 6},   // AC-3 5.1
		{Codec: "aac", MaxChannels: 6},   // AAC 5.1
		{Codec: "aac", MaxChannels: 2},   // AAC stereo (last resort)
	},
	"dts": {
		{Codec: "dts", MaxChannels: 8},
		{Codec: "eac3", MaxChannels: 8},
		{Codec: "eac3", MaxChannels: 6},
		{Codec: "ac3", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 2},
	},
	"eac3": {
		{Codec: "eac3", MaxChannels: 8},
		{Codec: "eac3", MaxChannels: 6},
		{Codec: "ac3", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 2},
	},
	"ac3": {
		{Codec: "ac3", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 2},
	},
	"flac": {
		{Codec: "flac", MaxChannels: 8},
		{Codec: "aac", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 2},
	},
	"alac": {
		{Codec: "alac", MaxChannels: 8},
		{Codec: "aac", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 2},
	},
	"opus": {
		{Codec: "opus", MaxChannels: 8},
		{Codec: "aac", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 2},
	},
	"aac": {
		{Codec: "aac", MaxChannels: 6},
		{Codec: "aac", MaxChannels: 2},
	},
	"mp3": {
		{Codec: "mp3", MaxChannels: 2},
		{Codec: "aac", MaxChannels: 2},
	},
}

// audioTarget represents a candidate audio codec with channel count.
type audioTarget struct {
	Codec       string
	MaxChannels int
}

// GetAudioCodecLadder returns the quality-ordered audio fallback chain.
// Returns a copy so callers cannot mutate the global ladder.
func GetAudioCodecLadder(sourceCodec string) []audioTarget {
	if ladder, ok := audioCodecLadder[sourceCodec]; ok {
		out := make([]audioTarget, len(ladder))
		copy(out, ladder)
		return out
	}
	// Unknown: fallback to AAC stereo
	return []audioTarget{
		{Codec: "aac", MaxChannels: 2},
	}
}
