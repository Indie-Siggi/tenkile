# Community Contribution Workflow

This document describes the process for contributing new device profiles to the Tenkile curated device database.

## Overview

Tenkile maintains a curated database of Smart TV and streaming device profiles with accurate codec support information. Community contributions are welcome and go through a verification process to ensure quality.

## Contribution Types

### 1. New Device Profile
Adding codec support information for a device not currently in the database.

### 2. Update Existing Profile
Correcting or improving codec information for an existing device.

### 3. Known Issues
Reporting playback issues, workarounds, or device-specific quirks.

### 4. Verification
Confirming that existing device profiles accurately represent real-world behavior.

## Submission Process

### Step 1: Gather Information

Collect the following information about your device:

```json
{
  "name": "Device Display Name (e.g., Samsung Q80T (2020))",
  "manufacturer": "Samsung",
  "model": "Q80T",
  "platform": "samsung_tizen"
}
```

**Capabilities to document:**
- Video codecs supported (h264, hevc, vp9, av1, etc.)
- Audio codecs supported (aac, ac3, eac3, truehd, dts, etc.)
- HDR support (HDR10, Dolby Vision, HLG)
- Maximum resolution and bitrate
- Container formats supported (mp4, mkv, ts, etc.)
- DRM support (Widevine, PlayReady)

### Step 2: Create Submission

Submit via one of these methods:

#### Method A: GitHub Pull Request
1. Fork the repository
2. Add or modify JSON file in `data/curated/`
3. Submit PR with:
   - Device name and model in title
   - Source of information (official specs, community testing, etc.)
   - Any known issues or caveats

#### Method B: API Submission
```bash
# Submit new device
curl -X PUT https://your-tenkile-instance/api/v1/admin/curated/devices \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_API_KEY" \
  -d '{
    "name": "My TV Model",
    "manufacturer": "Manufacturer",
    "model": "ModelNumber",
    "platform": "platform_name",
    "capabilities": {
      "video_codecs": ["h264", "hevc"],
      "max_width": 3840,
      "max_height": 2160
    }
  }'
```

### Step 3: Verification

All submissions go through verification:

| Submission Type | Auto-Verify | Manual Review |
|-----------------|-------------|---------------|
| Matches official specs | Yes | No |
| Community tested | No | Yes |
| Known issues reported | No | Yes |

**Verification criteria:**
- Source credibility (official docs vs. anecdotal)
- Consistency with similar devices
- Community voting/reports
- Testing evidence (probe reports)

## Device Profile Schema

```json
{
  "id": "unique-device-id",
  "device_hash": "platform|manufacturer|model|year",
  "name": "Full Display Name",
  "manufacturer": "Manufacturer Name",
  "model": "Model Number",
  "platform": "platform_type",
  "os_versions": ["OS versions supported"],
  "capabilities": {
    "video_codecs": ["h264", "hevc", "vp9", "av1"],
    "audio_codecs": ["aac", "ac3", "eac3", "truehd", "dts"],
    "subtitle_formats": ["srt", "vtt", "pgs"],
    "container_formats": ["mp4", "mkv", "ts"],
    "max_width": 3840,
    "max_height": 2160,
    "max_bitrate": 100000000,
    "supports_hdr": true,
    "supports_dolby_vision": false,
    "supports_dolby_atmos": true,
    "supports_dts": false,
    "drm_support": {
      "widevine": true,
      "playready": true,
      "clearkey": true
    }
  },
  "recommended_profile": "ultra_hd_hdr",
  "known_issues": [
    {
      "id": "unique-issue-id",
      "title": "Issue Title",
      "description": "Detailed description",
      "severity": "info|warning|error",
      "codecs": ["affected codecs"],
      "workaround": "How to work around",
      "resolved": false
    }
  ],
  "source": "community",
  "votes_up": 0,
  "votes_down": 0,
  "verified": false
}
```

## Platform Naming Conventions

| Platform | Example IDs |
|----------|-------------|
| Samsung Tizen | `samsung_tizen` |
| LG webOS | `lg_webos` |
| Roku | `roku` |
| Android TV | `android_tv` |
| Apple TV | `appletv` |
| Fire TV | `firetv` |
| Chromecast | `chromecast` |
| PlayStation | `playstation` |
| Xbox | `xbox` |
| Nintendo Switch | `nintendo_switch` |
| Smart TV (Generic) | `smart_tv` |

## Codec Naming

### Video Codecs
- `h264` - H.264/AVC
- `hevc` - H.265/HEVC
- `vp9` - VP9
- `av1` - AV1
- `mpeg2` - MPEG-2
- `vc1` - VC-1

### Audio Codecs
- `aac` - AAC
- `ac3` - Dolby Digital (AC-3)
- `eac3` - Dolby Digital Plus (E-AC-3)
- `truehd` - Dolby TrueHD
- `dts` - DTS
- `dtshd` - DTS-HD
- `flac` - FLAC
- `opus` - Opus
- `mp3` - MP3

## Testing Your Submission

### Probe Report
Test your device by running a probe and submitting the report:

```bash
# Via API
curl -X POST https://your-tenkile-instance/api/v1/probe/report \
  -H "Content-Type: application/json" \
  -d '{
    "capabilities": {
      "device_id": "test-device",
      "video_codecs": ["h264", "hevc"],
      "max_width": 3840,
      "max_height": 2160
    }
  }'
```

### Manual Testing Checklist
- [ ] Direct play H.264 1080p in MP4
- [ ] Direct play HEVC 4K in MP4
- [ ] Direct play HDR content
- [ ] Dolby Vision playback
- [ ] Dolby Atmos audio passthrough
- [ ] DTS audio passthrough
- [ ] MKV container support
- [ ] Subtitle formats (SRT, PGS)

## Vote Weighting

Community votes help prioritize verification:

| Vote Type | Weight | Description |
|-----------|--------|-------------|
| Upvote | +1 | Confirms profile accuracy |
| Downvote | -1 | Indicates inaccurate profile |
| Verified | +10 | Admin verified profile |

## Quality Guidelines

### Good Contributions
- Based on official specifications
- Include testing evidence
- Document known issues
- Follow schema format
- Use correct codec/platform names

### Poor Contributions
- Anecdotal ("I think it supports...")
- Missing essential codec information
- Incorrect manufacturer/model names
- Duplicate entries
- Vandalism or false information

## Recognition

Top contributors are acknowledged in:
- Release notes
- CONTRIBUTORS file
- GitHub contributor graph

## Support

For questions about contributions:
- GitHub Issues: Report bugs or request features
- GitHub Discussions: Ask questions
- Discord: Real-time community support

## License

By submitting content, you agree that your contributions will be licensed under AGPL-3.0-or-later.
