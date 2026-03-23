# Phase 3: Curated Device Database + Playback Feedback Loop

> **Status:** ✅ COMPLETED  
> **Trust Sources:** Playback Feedback (0.95) > Curated DB (0.90) > Native Probe (0.85)

Phase 3 introduces two complementary systems:

1. **Curated Device Database** — Pre-configured capability profiles for known devices (Smart TVs, streaming boxes)
2. **Playback Feedback Loop** — Real-world trust adjustment based on actual playback outcomes

## Curated Device Database (Phase 3.1)

### Overview

Smart TVs (Samsung Tizen, LG WebOS, Roku) have **fixed capabilities per model/firmware version**. Browser-based probing is unreliable because:
- Web engines often have broken or incomplete API implementations
- Device manufacturers misreport codec support
- Firmware updates can change capabilities without user notification

The curated database provides **trust=0.90** — higher than runtime probing (0.50–0.85) for known devices.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CuratedDatabase                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  devices    │  │  hashIndex  │  │   platformIndex     │ │
│  │  map[id]    │  │  hash→ids   │  │   platform→ids      │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│                                                              │
│  Thread-safe via sync.RWMutex                                │
└─────────────────────────────────────────────────────────────┘
          │                    │
          ▼                    ▼
┌─────────────────┐  ┌─────────────────────────────────────┐
│  Embedded FS   │  │         Data Directory              │
│  (go:embed)     │  │    data/curated/*.json              │
│                 │  │                                     │
│  samsung_tizen  │  │  community-submitted profiles       │
│  lg_webos       │  │  with voting and verification       │
│  roku           │  │                                     │
│  android_tv     │  │                                     │
└─────────────────┘  └─────────────────────────────────────┘
```

### Supported Platforms

| Platform | Manufacturer | SoC Families | Firmware Range |
|----------|-------------|--------------|----------------|
| `samsung_tizen` | Samsung | Tizen 4.0-6.5 | 2018-2025 |
| `lg_webos` | LG | WebOS 4.0-6.0 | 2018-2025 |
| `roku` | Roku | All models | OS 9-13 |
| `android_tv` | Various | Android TV, Google TV | API 9+ |

### Database Schema

#### CuratedDevice

```go
type CuratedDevice struct {
    ID                   string              `json:"id"`
    DeviceHash           string              `json:"device_hash"` // Consistent identifier
    Name                 string              `json:"name"`
    Manufacturer         string              `json:"manufacturer"`
    Model                string              `json:"model"`
    Platform             string              `json:"platform"`
    OSVersions          []string            `json:"os_versions,omitempty"`
    Capabilities        *DeviceCapabilities `json:"capabilities"`
    RecommendedProfile   string              `json:"recommended_profile,omitempty"`
    KnownIssues         []KnownIssue        `json:"known_issues,omitempty"`
    Source               string              `json:"source"` // "community", "official", "curated"
    VotesUp              int                 `json:"votes_up"`
    VotesDown            int                 `json:"votes_down"`
    Verified             bool                `json:"verified"`
    Notes                string              `json:"notes,omitempty"`
    LastUpdated          time.Time           `json:"last_updated"`
    CreatedAt            time.Time           `json:"created_at"`
}
```

#### KnownIssue

```go
type KnownIssue struct {
    ID          string   `json:"id"`
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Severity    string   `json:"severity"` // "info", "warning", "error"
    Codecs      []string `json:"codecs,omitempty"`
    Containers  []string `json:"containers,omitempty"`
    Workaround  string   `json:"workaround,omitempty"`
    Resolved    bool     `json:"resolved"`
}
```

#### DatabaseStats

```go
type DatabaseStats struct {
    TotalDevices     int64 `json:"total_devices"`
    VerifiedDevices  int64 `json:"verified_devices"`
    CommunityDevices int64 `json:"community_devices"`
    OfficialDevices  int64 `json:"official_devices"`
    CuratedDevices   int64 `json:"curated_devices"`
    PlatformsCount   int   `json:"platforms_count"`
}
```

### API Reference

#### Create Curated Device

```
PUT /api/v1/admin/curated/devices
```

**Request:**
```json
{
    "name": "Samsung QN65Q80T",
    "manufacturer": "Samsung",
    "model": "QN65Q80T",
    "platform": "samsung_tizen",
    "capabilities": {
        "platform": "samsung_tizen",
        "video_codecs": ["hevc", "av1", "vp9", "avc"],
        "audio_codecs": ["aac", "ac3", "eac3", "flac", "opus"],
        "max_width": 3840,
        "max_height": 2160,
        "supports_hdr": true,
        "supports_dolby_vision": false
    },
    "known_issues": [
        {
            "title": "HDR10+ not supported",
            "description": "Only HDR10 is supported, not HDR10+",
            "severity": "warning",
            "codecs": ["hdr10plus"]
        }
    ],
    "source": "community",
    "notes": "Tested with firmware 1005.4"
}
```

**Response:** `201 Created`
```json
{
    "success": true,
    "device_id": "samsung-qn65q80t-...",
    "message": "Device created successfully",
    "device": { ... }
}
```

#### Replace Curated Device

```
PUT /api/v1/admin/curated/devices/{id}
```

Full device replacement. Preserves `verified`, `created_at`, `votes_up`, `votes_down`.

#### Delete Curated Device

```
DELETE /api/v1/admin/curated/devices/{id}
```

Cannot delete verified official devices without unverifying first.

#### Vote on Device

```
POST /api/v1/admin/curated/devices/{id}/vote
```

**Request:**
```json
{
    "up": true
}
```

#### Fuzzy Search Devices

```
POST /api/v1/admin/curated/search
```

**Request:**
```json
{
    "device_name": "Samsung Q80T",
    "platform": "samsung_tizen",
    "limit": 10
}
```

**Response:**
```json
{
    "query": "Samsung Q80T",
    "results": [
        {
            "device": { ... },
            "score": 95.0,
            "matched_on": ["name", "base_model"],
            "match_type": "version",
            "year_detected": "2020"
        }
    ],
    "total_found": 3
}
```

#### Version-Aware Matching

```
POST /api/v1/admin/curated/version-match
```

**Request:**
```json
{
    "device_name": "Samsung UN55TU8000FXZA",
    "platform": "samsung_tizen"
}
```

**Response:**
```json
{
    "original": "Samsung UN55TU8000FXZA",
    "best_match": { ... },
    "confidence": "high",
    "base_model": "TU8000",
    "year": "2020",
    "variant": "FXZA"
}
```

#### Export Devices

```
GET /api/v1/admin/curated/export?platform=samsung_tizen&format=json
```

Exports devices as JSON with metadata.

#### Import Devices

```
POST /api/v1/admin/curated/import
```

**Request:**
```json
{
    "devices": [ ... ],
    "merge_strategy": "upsert"  // "skip", "overwrite", "upsert"
}
```

### Fuzzy Matching Algorithm

The fuzzy matching system handles:
1. **Model number variations**: "Q80T" matches "QN65Q80T"
2. **Firmware year extraction**: Detects years from patterns like "(2020)", "Gen 2021"
3. **Variant codes**: Handles regional suffixes like "FXZA", "UK", "ZA"
4. **Manufacturer normalization**: "SS" → "samsung"

**Scoring:**
| Match Type | Score | Description |
|-----------|-------|-------------|
| Exact | 100 | Full model match |
| Base Model | 90 | Matches model family |
| Version | 80+ | Year/variant match |
| Fuzzy | 50-79 | Partial similarity |
| Partial | 30-49 | Low confidence |

Verified devices get +1.0 bonus score.

---

## Playback Feedback Loop (Phase 3.2)

### Overview

Playback feedback provides the **highest trust source (0.95)** because it represents real-world validation:

```
┌─────────────┐     ┌─────────────┐     ┌──────────────┐
│  Playback   │────▶│  Feedback    │────▶│ Trust Score  │
│  Attempt    │     │  Manager     │     │ Adjustment   │
└─────────────┘     └─────────────┘     └──────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │ Re-probe     │
                    │ Trigger      │
                    └──────────────┘
```

### Trust Adjustment Rules

| Outcome | Delta | Cumulative Cap | Description |
|---------|-------|----------------|-------------|
| `success` | +0.01 | 1.0 | Per successful playback |
| `codec_error` | -0.15 | 0.0 | Codec not supported |
| `decoding_failed` | -0.25 | 0.0 | Decoder failed |
| `renderer_crash` | -0.30 | 0.0 | Client crashed |
| `network_error` | -0.05 | 0.0 | Network issue |
| `buffering` | -0.025 | 0.0 | Buffering issues |
| `timeout` | -0.05 | 0.0 | Playback timeout |
| `unsupported_format` | -0.15 | 0.0 | Format not supported |

### Re-probe Triggers

A device is flagged for re-probing when:
- **3+ consecutive failures** → Automatic re-probe
- **>50% failure rate** in last 10+ playbacks → Re-probe
- **Success streak bonus**: 10+ consecutive successes → +0.05 bonus

### API Reference

#### Submit Playback Feedback

```
POST /api/v1/devices/{id}/feedback
```

**Request:**
```json
{
    "media_id": "movie-123",
    "outcome": "success",
    "duration_seconds": 7200,
    "buffer_duration_seconds": 5,
    "network_quality": "excellent",
    "video_codec": "hevc",
    "audio_codec": "eac3",
    "container": "mp4",
    "resolution": "3840x2160",
    "bitrate": 25000000
}
```

**Response:**
```json
{
    "success": true,
    "recorded": true,
    "trust_delta": 0.01,
    "needs_reprobe": false,
    "reprobe_reason": "",
    "message": "Feedback recorded successfully"
}
```

#### Get Playback Statistics

```
GET /api/v1/devices/{id}/feedback/stats
```

**Response:**
```json
{
    "device_id": "device-abc",
    "stats": {
        "total_playbacks": 150,
        "successful_playbacks": 142,
        "failed_playbacks": 8,
        "success_rate": 0.946,
        "consecutive_successes": 12,
        "consecutive_failures": 0,
        "outcome_counts": {
            "success": 142,
            "codec_error": 5,
            "buffering": 3
        },
        "codec_stats": {
            "video:hevc": { "attempts": 100, "successes": 98, "failures": 2, "success_rate": 0.98 },
            "audio:eac3": { "attempts": 80, "successes": 79, "failures": 1, "success_rate": 0.99 }
        },
        "last_playback": "2024-03-15T10:30:00Z",
        "last_success": "2024-03-15T10:30:00Z",
        "last_failure": "2024-03-14T22:15:00Z",
        "current_trust_delta": 0.12,
        "needs_reprobe": false
    }
}
```

#### Get Reliable Codecs

```
GET /api/v1/devices/{id}/reliable-codecs?min_rate=0.8
```

**Response:**
```json
{
    "device_id": "device-abc",
    "reliable_codecs": ["video:hevc", "audio:eac3", "video:av1"],
    "min_success_rate": 0.8,
    "codec_stats": { ... }
}
```

#### Trigger Re-probe

```
POST /api/v1/devices/{id}/reprobe
```

Clears device from cache and resets trust adjustment.

#### Get Trust Report

```
GET /api/v1/devices/{id}/trust
```

**Response:**
```json
{
    "device_id": "device-abc",
    "effective_trust": 0.92,
    "trust_level": "very_high",
    "feedback_adjustment": 0.12,
    "needs_reprobe": false,
    "reprobe_reason": "",
    "reliable_codecs": ["video:hevc", "audio:eac3"],
    "playback_stats": { ... }
}
```

#### Get Feedback Metrics

```
GET /api/v1/feedback/metrics
GET /api/v1/feedback/metrics?format=prometheus
```

Prometheus format for monitoring dashboards:
```
# HELP tenkile_playback_total Total playback attempts
# TYPE tenkile_playback_total counter
tenkile_playback_total{outcome="success"} 142
tenkile_playback_total{outcome="failure"} 8

# HELP tenkile_trust_score_current Current trust score
# TYPE tenkile_trust_score_current gauge
tenkile_trust_score_current{device_id="abc"} 0.92

# HELP tenkile_reprobe_triggered Re-probe triggers
# TYPE tenkile_reprobe_triggered counter
tenkile_reprobe_triggered{reason="consecutive_failures"} 2
```

### Data Structures

#### PlaybackFeedback

```go
type PlaybackFeedback struct {
    DeviceID       string           `json:"device_id"`
    MediaID        string           `json:"media_id"`
    Outcome        PlaybackOutcome `json:"outcome"`
    ErrorCode      string           `json:"error_code,omitempty"`
    ErrorMessage   string           `json:"error_message,omitempty"`
    Timestamp      time.Time        `json:"timestamp"`
    Duration       time.Duration    `json:"duration"`
    BufferDuration time.Duration    `json:"buffer_duration,omitempty"`
    NetworkQuality string           `json:"network_quality,omitempty"`
    VideoCodec     string           `json:"video_codec,omitempty"`
    AudioCodec     string           `json:"audio_codec,omitempty"`
    Container      string           `json:"container,omitempty"`
    Resolution     string           `json:"resolution,omitempty"`
    Bitrate        int64            `json:"bitrate,omitempty"`
}
```

#### PlaybackOutcome Enum

```go
const (
    OutcomeUnknown        PlaybackOutcome = iota
    OutcomeSuccess
    OutcomeNetworkError
    OutcomeCodecError
    OutcomeDecodingFailed
    OutcomeRendererCrash
    OutcomeUnsupportedFormat
    OutcomeTimeout
    OutcomeBuffering
)
```

#### TrustAdjustmentConfig

```go
type TrustAdjustmentConfig struct {
    SuccessBonus           float64  // 0.01
    NetworkErrorPenalty    float64  // 0.05
    CodecErrorPenalty      float64  // 0.15
    DecodingFailedPenalty  float64  // 0.25
    RendererCrashPenalty  float64  // 0.30
    MaxTrust              float64  // 1.0
    MinTrust              float64  // 0.0
    FailureWindowSize     int      // 3
    SuccessStreakBonus    float64  // 0.05
    SuccessStreakThreshold int     // 10
}
```

#### DevicePlaybackStats

```go
type DevicePlaybackStats struct {
    DeviceID              string              `json:"device_id"`
    TotalPlaybacks       int64               `json:"total_playbacks"`
    SuccessfulPlaybacks  int64               `json:"successful_playbacks"`
    FailedPlaybacks      int64               `json:"failed_playbacks"`
    SuccessRate          float64             `json:"success_rate"`
    ConsecutiveSuccesses int64               `json:"consecutive_successes"`
    ConsecutiveFailures  int64               `json:"consecutive_failures"`
    OutcomeCounts        map[string]int64    `json:"outcome_counts"`
    CodecStats           map[string]CodecStats `json:"codec_stats"`
    LastPlayback         time.Time           `json:"last_playback"`
    LastSuccess          time.Time           `json:"last_success"`
    LastFailure          time.Time           `json:"last_failure"`
    CurrentTrustDelta    float64             `json:"current_trust_delta"`
    NeedsReProbe         bool                `json:"needs_reprobe"`
    ReProbeReason        string              `json:"reprobe_reason,omitempty"`
}
```

---

## Usage Examples

### Curated Device Lookup

```go
// Initialize curated database
db := probes.NewCuratedDatabase()
db.Load("./data/curated")

// Search by platform
samsungTvs := db.GetByPlatform("samsung_tizen")

// Fuzzy search
results := db.MatchDevice("Samsung Q80T", "samsung_tizen", 5)
if len(results) > 0 {
    best := results[0].Device
    fmt.Printf("Best match: %s (score: %.0f)\n", best.Name, results[0].Score)
}

// Version-aware matching
match := db.VersionAwareMatch("Samsung UN55TU8000FXZA", "samsung_tizen")
fmt.Printf("Confidence: %s\n", match.Confidence)
```

### Playback Feedback Integration

```go
// Initialize feedback manager
fm := probes.NewFeedbackManager()

// Record successful playback
feedback := probes.PlaybackFeedback{
    DeviceID:    "device-123",
    MediaID:     "movie-456",
    Outcome:     probes.OutcomeSuccess,
    Duration:    2 * time.Hour,
    VideoCodec:  "hevc",
    AudioCodec: "eac3",
}
fm.RecordSuccess(feedback)

// Check if re-probe needed
needsReProbe, reason := fm.ShouldReProbe("device-123")
if needsReProbe {
    // Trigger re-probe flow
}

// Get reliable codecs for device
reliable := fm.GetReliableCodecs("device-123", 0.8)
fmt.Printf("Reliable codecs: %v\n", reliable)
```

### Customizing Trust Adjustment

```go
// Customize trust configuration
config := probes.DefaultTrustAdjustmentConfig()
config.SuccessBonus = 0.02          // Larger success bonus
config.CodecErrorPenalty = 0.20    // Stricter codec penalties
config.FailureWindowSize = 5       // More failures before re-probe
config.SuccessStreakThreshold = 15 // Longer streak for bonus

fm.SetTrustConfig(config)
```

---

## Client Integration

### Submitting Feedback (JavaScript)

```javascript
// Hook into video player
video.addEventListener('playing', () => {
    // Report success after 5 seconds of smooth playback
    setTimeout(() => {
        fetch(`/api/v1/devices/${deviceId}/feedback`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                media_id: currentMediaId,
                outcome: 'success',
                duration_seconds: video.duration,
                video_codec: currentVideoCodec,
                audio_codec: currentAudioCodec
            })
        });
    }, 5000);
});

video.addEventListener('error', () => {
    // Report failure immediately
    fetch(`/api/v1/devices/${deviceId}/feedback`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            media_id: currentMediaId,
            outcome: 'codec_error',
            error_code: video.error.code,
            error_message: video.error.message
        })
    });
});
```

---

## Files Reference

| File | Purpose |
|------|---------|
| `internal/probes/curated.go` | CuratedDatabase implementation |
| `internal/probes/curated_fuzzy.go` | Fuzzy matching algorithm |
| `internal/probes/embedded.go` | Embedded device bundle loader |
| `internal/probes/feedback.go` | FeedbackManager implementation |
| `internal/api/admin.go` | Curated device API handlers |
| `internal/api/devices.go` | Playback feedback API handlers |
| `internal/api/router.go` | Route registration |
| `data/curated/*.json` | Community device profiles |
| `internal/probes/embedded/*.json` | Embedded device bundles |

---

## See Also

- [AGENTS.md](../AGENTS.md) — Agent task guide with Phase 3 implementation details
- [ARCHITECTURE.md](./ARCHITECTURE.md) — System architecture overview
- [DEVICE_DETECTION.md](./DEVICE_DETECTION.md) — Device detection strategies
