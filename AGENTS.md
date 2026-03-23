# Tenkile - Agent Task Guide

> **Goal:** Build a standalone media server replacing Jellyfin — intelligent, device-aware media delivery powered by CodecProbe.
> **Language:** Go
> **Source repos:** `../jellyfin/` (C#/.NET reference), `../codecprobe/` (JavaScript codec detection)
> **Architecture:** `../docs/ARCHITECTURE.md`
> **Reference docs:** `../docs/` (codec, container, DRM, device detection, device database, FFmpeg, Jellyfin references)
> **Target:** This folder (`Tenkile/`)
> **License:** AGPL-3.0-or-later

---

## Agent Rules (READ FIRST)

These rules are mandatory for every agent. They exist because Phase 1 code generation produced code that did not compile. Every rule below addresses a specific failure mode.

### File Structure (Current Implementation Status)

```
cmd/tenkile/
  main.go                     ✓ IMPLEMENTED (Phase 1)

internal/
  api/
    router.go                 ✓ IMPLEMENTED (Phase 1)
    devices.go               ✓ IMPLEMENTED (Phase 1) + EXTENDED (Phase 3.2)
    playback.go              ✓ IMPLEMENTED (Phase 1)
    admin.go                 ✓ IMPLEMENTED (Phase 1) + EXTENDED (Phase 3.1)
    response.go              ✓ IMPLEMENTED (Phase 1)

  probes/
    types.go                 ✓ IMPLEMENTED (Phase 1)
    trust.go                 ✓ IMPLEMENTED (Phase 1)
    validator.go            ✓ IMPLEMENTED (Phase 1)
    parser.go               ✓ IMPLEMENTED (Phase 1)
    cache.go                ✓ IMPLEMENTED (Phase 1)
    curated.go              ✓ IMPLEMENTED (Phase 1)
    drm.go                 ✓ IMPLEMENTED (Phase 1)
    curated_fuzzy.go        ✓ NEW (Phase 3.1)
    embedded.go             ✓ NEW (Phase 3.1)
    embedded/
      samsung_tizen.json   ✓ NEW (Phase 3.1)
      lg_webos.json        ✓ NEW (Phase 3.1)
      roku.json            ✓ NEW (Phase 3.1)
      android_tv.json       ✓ NEW (Phase 3.1)
    feedback.go             ✓ NEW (Phase 3.2)
    feedback_metrics.go     ✓ NEW (Phase 3.2 - implicit)
    feedback_integration.go ✓ NEW (Phase 3.2 - implicit)

  server/
    inventory.go           ✓ IMPLEMENTED (Phase 2)
    encoder.go             ✓ IMPLEMENTED (Phase 2)
    benchmark.go           ✓ IMPLEMENTED (Phase 2)

  transcode/
    orchestrator.go        ✓ IMPLEMENTED (Phase 2)
    quality.go             ✓ IMPLEMENTED (Phase 2)
    codec_ladder.go        ✓ IMPLEMENTED (Phase 2)
    matcher.go             ✓ IMPLEMENTED (Phase 2)
    subtitle.go            ✓ IMPLEMENTED (Phase 2)
    legacy.go              ✓ IMPLEMENTED (Phase 2)
    ffmpeg.go             ✓ IMPLEMENTED (Phase 2)
    logger.go              ✓ IMPLEMENTED (Phase 2)

  database/
    sqlite.go             ✓ IMPLEMENTED (Phase 1)

  config/
    config.go             ✓ IMPLEMENTED (Phase 1)

pkg/
  codec/
    database.go           ✓ IMPLEMENTED (Phase 1)
    mime.go               ✓ IMPLEMENTED (Phase 1)

web/
  probe/
    tenkile-probe.js      ✓ IMPLEMENTED (Phase 1) + EXTENDED (Phase 3.2)

data/
  curated/
    samsung_tizen.json    ✓ NEW (Phase 3.1)
    lg_webos.json         ✓ NEW (Phase 3.1)
    roku.json             ✓ NEW (Phase 3.1)
    android_tv.json        ✓ NEW (Phase 3.1)

docs/
  CONTRIBUTING.md          ✓ NEW (Phase 3.1 - implicit)

internal/
  media/
    models.go             ✓ NEW (Phase 4.1)
    scanner.go            ✓ NEW (Phase 4.1)
    probe.go               ✓ NEW (Phase 4.1)
    store.go               ✓ NEW (Phase 4.1)

  stream/
    handler.go            ⚠️ PARTIAL (Phase 4.2)
    segmenter.go          ⚠️ PARTIAL (Phase 4.2)
    models.go              ✓ NEW (Phase 4.2)

  api/
    library.go            ✓ NEW (Phase 4.1)
    media.go               ✓ NEW (Phase 4.1)
    auth.go               ✓ NEW (Phase 4.4)
    middleware.go          + EXTENDED (Phase 4.4)

api/
  openapi.yaml            ✓ NEW (Phase 4.3)

Dockerfile                ⚠️ PARTIAL (Phase 4.7)

configs/
  tenkile.yaml            ✓ NEW (Phase 4.7)
```

Legend:
- ✓ IMPLEMENTED = Fully implemented and tested
- ⚠️ PARTIAL = Partially implemented, needs more work
- ○ NOT STARTED = Not yet implemented
- ✓ NEW = New file created in this phase
- + EXTENDED = Existing file extended with new functionality
- ✓ NEW (implicit) = Created but not explicitly in task spec

### Rule 1: Only import packages that exist

Before importing any internal package (`internal/...` or `pkg/...`), verify it exists in this repository. **Never invent packages.** If your prompt says to use a package that doesn't exist, implement inline or skip — do not create phantom imports. Check the Phase Inventory section below for what's available.

### Rule 2: No duplicate type declarations

Each type has ONE owning file. Before declaring a struct, interface, or type alias, grep the codebase to confirm it doesn't already exist. If it does, import it. The Phase Inventory lists type ownership.

### Rule 3: Use exact library APIs — verify before using

Do NOT guess at function signatures or struct fields. Specific requirements:

| Need | Correct | Wrong (do NOT use) |
|------|---------|-------------------|
| SQLite driver | `modernc.org/sqlite`, driver name `"sqlite"` | `mattn/go-sqlite3`, driver name `"sqlite3"` (requires CGo) |
| Config struct tags | `yaml:"field_name"` | `koanf:"field_name"` (ignored by `yaml.Unmarshal`) |
| Chi route registration | `r.Get(pattern, handler)`, `r.Post(...)`, `r.MethodFunc(method, pattern, handler)` | `r.HandleFunc(pattern, method, handler)` (3-arg form doesn't exist in chi) |
| Structured logging | `slog.HandlerOptions{Level: ..., ReplaceAttr: ...}` | `slog.HandlerOptions{TimeFunc: ...}` (field doesn't exist) |
| Config library | `gopkg.in/yaml.v3` with `yaml.Unmarshal` | `koanf` (not used in this project) |

### Rule 4: Go compilation is non-negotiable

Go treats unused variables, unused imports, and type mismatches as **hard errors** — the program will not compile. Before considering your work done:
- Remove every unused variable (or use `_` if the call has side effects)
- Remove every unused import
- Verify every struct field you reference actually exists on that struct
- Verify every method you call actually exists on that type with the correct signature
- Run `go build ./...` and fix all errors

### Rule 5: Tests must match implementation

Write tests AFTER the implementation, not before. Tests must assert the behavior the code actually has. If the validator adds a warning (not an error) for unknown platforms, the test must check for a warning, not assert `IsValid == false`.

### Rule 6: Complete every function

Do not leave functions truncated or half-written. If a file is getting long (>500 lines), split into multiple files within the same package. Every `{` must have a matching `}`. Every `func` must have a complete body.

### Rule 7: Cross-reference types when writing multi-file code

When your code references a type from another file (even in the same package):
- Read that file first to confirm the exact field names, types, and method signatures
- Use the exact names — `SupportsDolbyVision` not `SupportsDV`, `UpdateDevice()` not `Update()`, `RemoveDevice()` not `Delete()`
- If you need a field that doesn't exist, add it to the owning file, don't create a duplicate struct

### Rule 8: Validate after each agent's work

Every agent prompt should end with a compilation check. Run `go build ./...` and `go test ./...` before considering the task complete. Fix any errors before moving on.

### Rule 9: Method signatures are contracts

When calling methods on types from other files/packages, use the exact signature. Key signatures in this codebase:

### Rule 10: Abstract all system calls behind interfaces

Any code that calls `exec.Command`, `os.Stat`, `exec.LookPath`, reads device files, or probes the OS must go through an interface so tests can mock it. Do NOT use `runtime.GOOS` directly in logic — store it in a struct field set from `runtime.GOOS` at construction, with a `SetGOOS()` test override. Every external system interaction needs exactly one abstraction point.

Pattern:
```go
// Interface in the same file that uses it
type CommandRunner interface {
    Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)
}

// Real implementation
type ExecRunner struct{}
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) { ... }

// Constructor takes the interface
func NewThing(runner CommandRunner) *Thing { ... }
```

This applies to: FFmpeg calls, `exec.LookPath`, device file checks (`os.Stat`), GPU detection commands (`nvidia-smi`, `vainfo`). Never use `which` (not portable) — use `exec.LookPath`. Never use `test -c` (not portable) — use `os.Stat` with `os.ModeCharDevice`.

### Rule 11: Avoid repeated expensive operations

When a function fetches data (e.g., running `ffmpeg -encoders`), pass the result to downstream functions rather than having each one re-fetch independently. This prevents TOCTOU bugs and reduces startup time.

### Rule 12: Return copies of package-level collection data

If a package exposes global lookup tables (maps, slices), getter functions must return copies, not references to the internal data. Callers modifying a returned slice would corrupt global state.

```go
// Wrong: returns internal slice
func GetLadder(codec string) []string { return ladder[codec] }

// Right: returns a copy
func GetLadder(codec string) []string {
    src := ladder[codec]
    out := make([]string, len(src))
    copy(out, src)
    return out
}
```
- `cache.Get(deviceID string) (*DeviceCapabilities, bool)` — returns `(value, found)`, NOT `(value, error)`
- `cache.Set(deviceID string, caps *DeviceCapabilities, source string) error`
- `NewCapabilityCache(config *CacheConfig) (*CapabilityCache, error)` — nil config = memory-only defaults
- `NewCuratedDatabase() *CuratedDatabase` — no arguments
- `NewValidator() *Validator` — no arguments

---

## Phase 1 Inventory (completed)

Phase 1 is done. The following packages, types, and functions exist. Phase 2+ agents MUST use these — do not redefine them.

### Package: `pkg/codec`
- `database.go` — Codec/container/format constants and `Equal(a, b string) bool`
- `mime.go` — MIME type mappings

### Package: `internal/probes`

**Type ownership (one file owns each type, do not redeclare):**

| File | Types owned |
|------|------------|
| `types.go` | `DevicePlatform`, `DeviceIdentity`, `TrustedCodecSupport`, `ScenarioSupport`, `DeviceCapabilities`, `ProbeReport`, `ScenarioResult`, platform constants, codec/container lists |
| `trust.go` | `TrustLevel` (7 levels: Untrusted→Verified), `TrustScore`, `TrustResolver`, `SourceClaim`, `ResolvedCapability` |
| `validator.go` | `ValidationResult`, `ValidationError`, `ValidationWarning`, `AnomalyRecord`, `Validator`, `PlatformConstraints` |
| `parser.go` | `CodecProbeResult`, `VideoCodecInfo`, `AudioCodecInfo`, `DirectPlaySupport`, `TranscodingPrefs` |
| `cache.go` | `CapabilityCache`, `CachedCapability`, `CacheStats`, `CacheConfig` |
| `curated.go` | `CuratedDevice`, `KnownIssue`, `CuratedDatabase`, `DatabaseStats`, `SearchCriteria` |
| `drm.go` | `DRMSupported`, `DRMSystemDetail` |

**Key function signatures:**

```go
// Cache
func NewCapabilityCache(config *CacheConfig) (*CapabilityCache, error)
func (c *CapabilityCache) Get(deviceID string) (*DeviceCapabilities, bool)
func (c *CapabilityCache) Set(deviceID string, caps *DeviceCapabilities, source string) error
func (c *CapabilityCache) Delete(deviceID string)
func (c *CapabilityCache) Clear()
func (c *CapabilityCache) BatchGet(deviceIDs []string) (map[string]*DeviceCapabilities, error)
func (c *CapabilityCache) BatchSet(entries map[string]*DeviceCapabilities, source string) error
func (c *CapabilityCache) GetStats() CacheStats
func (c *CapabilityCache) Close() error

// Curated DB
func NewCuratedDatabase() *CuratedDatabase
func (cd *CuratedDatabase) GetByID(id string) (*CuratedDevice, bool)
func (cd *CuratedDatabase) GetByDeviceHash(deviceHash string) (*CuratedDevice, bool)
func (cd *CuratedDatabase) GetByPlatform(platform string) []*CuratedDevice
func (cd *CuratedDatabase) Search(criteria SearchCriteria) []*CuratedDevice
func (cd *CuratedDatabase) AddDevice(device *CuratedDevice) error
func (cd *CuratedDatabase) UpdateDevice(device *CuratedDevice) error
func (cd *CuratedDatabase) RemoveDevice(deviceID string) error
func (cd *CuratedDatabase) GetStats() DatabaseStats
func (cd *CuratedDatabase) GetAll() []*CuratedDevice

// Validator
func NewValidator() *Validator
func (v *Validator) ValidateCapabilities(caps *DeviceCapabilities) *ValidationResult

// Trust
func NewTrustResolver() *TrustResolver
func (tr *TrustResolver) Resolve(claims []SourceClaim) *ResolvedCapability

// Parser
func ParseCodecProbe(probeJSON []byte) (*DeviceCapabilities, error)
func ParseCodecProbeResult(probe *CodecProbeResult) (*DeviceCapabilities, error)
```

### Package: `internal/api`
- `router.go` — Chi router with middleware, `NewRouter(cfg, db) http.Handler`
- `devices.go` — `DeviceHandlers` with probe report, capability lookup, validation, search
- `playback.go` — `PlaybackHandlers` with decision, feedback, transcode, profiles, validate
- `admin.go` — `AdminHandlers` with system info, stats, cache cleanup, curated device management, decision log query
- `response.go` — `RespondJSON(w, status, v)` and `ErrorResponse{Error, Message}`

### Package: `internal/config`
- `config.go` — `Config` struct with `yaml:"..."` tags, `Load(path) (*Config, error)`

### Package: `internal/database`
- `sqlite.go` — `SQLite` struct, `NewSQLite(path) (*SQLite, error)`, `DB() *sql.DB`

### Package: `cmd/tenkile`
- `main.go` — Entry point, CLI flags, slog setup, HTTP server start

---

## Phase 2 Inventory (completed)

Phase 2 is done. The following packages, types, and functions exist. Phase 3+ agents MUST use these — do not redefine them.

### Package: `internal/server`

**Type ownership:**

| File | Types owned |
|------|------------|
| `inventory.go` | `HWAccelType`, `HardwareAcceleration`, `PerformanceEstimate`, `EncoderCapability`, `ServerCapabilities`, `CommandRunner`, `ExecRunner`, `FFmpegPathFinder`, `Inventory` |
| `encoder.go` | `EncoderSelector` |
| `benchmark.go` | `BenchmarkResult`, `Benchmarker` |

**Key function signatures:**

```go
// Inventory
func NewInventory(runner CommandRunner, logger *slog.Logger) *Inventory
func (inv *Inventory) Discover(ctx context.Context) (*ServerCapabilities, error)
func (inv *Inventory) GetCurrent() *ServerCapabilities
func (inv *Inventory) SetGOOS(goos string)                     // testing only
func (inv *Inventory) SetPathFinder(pf FFmpegPathFinder)       // testing only
func (inv *Inventory) SetDeviceExistsFunc(fn func(string) bool) // testing only

// ServerCapabilities
func (sc *ServerCapabilities) CanEncode(codecName string) bool
func (sc *ServerCapabilities) GetEncoders(codecName string) []EncoderCapability
func (sc *ServerCapabilities) HasHWAccel() bool

// EncoderSelector
func NewEncoderSelector(caps *ServerCapabilities) *EncoderSelector
func (s *EncoderSelector) SelectEncoder(targetCodec string, needsHDR bool, needs10Bit bool) *EncoderCapability
func (s *EncoderSelector) CanEncodeCodec(codecName string) bool

// Benchmarker
func NewBenchmarker(runner CommandRunner, logger *slog.Logger) *Benchmarker
func (b *Benchmarker) BenchmarkEncoder(ctx context.Context, ffmpegPath string, enc EncoderCapability) (*BenchmarkResult, error)
func (b *Benchmarker) BenchmarkAll(ctx context.Context, ffmpegPath string, caps *ServerCapabilities) []BenchmarkResult
```

### Package: `internal/transcode`

**Type ownership:**

| File | Types owned |
|------|------------|
| `orchestrator.go` | `DecisionType`, `MediaItem`, `PlaybackDecision`, `Orchestrator` |
| `quality.go` | `HdrPolicy`, `ToneMappingQuality`, `AudioChannelPolicy`, `BitDepthPolicy`, `ResolutionPolicy`, `QualityPreservationPolicy` |
| `codec_ladder.go` | `audioTarget`, `GetVideoCodecLadder()`, `GetAudioCodecLadder()` |
| `matcher.go` | `DeviceMatcher` |
| `subtitle.go` | `SubtitleType`, `SubtitleDecision`, `SubtitleAction`, `SubtitleConfig` |
| `legacy.go` | `LegacyContentFlags` |
| `ffmpeg.go` | `FFmpegArgs` |
| `logger.go` | `PlaybackDecisionLog`, `DecisionStats`, `DecisionQuery`, `DecisionLogger` |

**Key function signatures:**

```go
// Orchestrator
func NewOrchestrator(inv *server.Inventory, policy QualityPreservationPolicy, subConfig SubtitleConfig, logger *slog.Logger) *Orchestrator
func (o *Orchestrator) Decide(ctx context.Context, item *MediaItem, deviceCaps *probes.DeviceCapabilities) *PlaybackDecision

// Quality
func DefaultQualityPolicy() QualityPreservationPolicy

// Codec ladders (return copies — safe to modify)
func GetVideoCodecLadder(sourceCodec string) []string
func GetAudioCodecLadder(sourceCodec string) []audioTarget

// Subtitles
func ClassifySubtitle(format string) SubtitleType
func DecideSubtitle(subFormat string, deviceSupportsFormat bool, config SubtitleConfig) SubtitleDecision

// Legacy
func DetectLegacyContent(item *MediaItem) LegacyContentFlags
func BuildLegacyFilterArgs(flags LegacyContentFlags) []string

// FFmpeg
func BuildFFmpegArgs(decision *PlaybackDecision, encoder *server.EncoderCapability, policy QualityPreservationPolicy) *FFmpegArgs
func (a *FFmpegArgs) Build(inputPath, outputPath string) []string

// Decision Logger
func NewDecisionLogger(logger *slog.Logger) *DecisionLogger
func (dl *DecisionLogger) Log(ctx context.Context, decision *PlaybackDecision, deviceID, mediaItemID string) *PlaybackDecisionLog
func (dl *DecisionLogger) UpdateOutcome(id string, succeeded bool, failureReason string) bool
func (dl *DecisionLogger) Query(q DecisionQuery) []*PlaybackDecisionLog
func (dl *DecisionLogger) GetByID(id string) (*PlaybackDecisionLog, bool)
func (dl *DecisionLogger) Stats() *DecisionStats
```

### Admin API additions (Phase 2)

Routes added to `internal/api/router.go` under `/api/v1/admin/`:
- `GET /decisions` — query decision logs (params: `deviceId`, `from`, `to`, `limit`, `offset`)
- `GET /decisions/stats` — aggregate decision statistics

`AdminHandlers` has a `SetDecisionLogger(dl *transcode.DecisionLogger)` method.

### Module dependencies (in go.mod)
```
github.com/go-chi/chi/v5
github.com/go-chi/cors
modernc.org/sqlite
golang.org/x/crypto
gopkg.in/yaml.v3
```

Not in go.mod yet (add when needed): `github.com/jackc/pgx/v5`, `github.com/pressly/goose/v3`, `github.com/gorilla/websocket`, `github.com/golang-jwt/jwt/v5`, `github.com/stretchr/testify`

---

## License

Tenkile is licensed under **AGPL-3.0-or-later** (GNU Affero General Public License v3).

**Why AGPL-3.0:** CodecProbe is a fork of an AGPL-3.0 project. Tenkile embeds CodecProbe's probe library (codec-tester.js, codec-database-v2.js, drm-detection.js) in its web client, which is bundled into the Go binary via `go:embed`. Bundling AGPL code into a combined work requires the entire work to be AGPL-compatible. Additionally, AGPL Section 13 (network interaction) requires that users interacting with the software over a network can request the source code.

**What this means in practice:**
- Tenkile source code must be publicly available when deployed as a network service
- Modifications must be shared under the same license
- This is standard for the media server space (Jellyfin is GPL-2.0)
- Self-hosters are unaffected — AGPL only adds obligations when you distribute or serve the software to others

When scaffolding the project, include a `LICENSE` file with the full AGPL-3.0 text and add SPDX headers (`// SPDX-License-Identifier: AGPL-3.0-or-later`) to all Go source files.

---

## Context & Codebase Overview

### Tenkile (Go - what we're building)

A standalone media server that replaces Jellyfin. Jellyfin's codebase is reference material for understanding the problem space — we study it to understand what to build, but we don't link against it or need compatibility with it.

### Jellyfin Server (C#/.NET 10 - reference only)

Study these files to understand the problems Tenkile solves:

- **Streaming core:** `../jellyfin/MediaBrowser.Model/Dlna/StreamBuilder.cs` - the 2400+ line monolith that decides direct-play vs transcode. What we're replacing with a cleaner design.
- **Transcode reasons:** `../jellyfin/MediaBrowser.Model/Session/TranscodeReason.cs` - enum of 27 transcode reasons (flags enum). Study to understand failure modes.
- **Device capabilities:** `../jellyfin/MediaBrowser.Controller/Devices/IDeviceManager.cs` - current simplistic device model. Study to understand what's missing.
- **Encoding pipeline:** `../jellyfin/MediaBrowser.MediaEncoding/Encoder/MediaEncoder.cs` - FFmpeg wrapper with hardware detection. Study for FFmpeg interaction patterns.
- **HLS controller:** `../jellyfin/Jellyfin.Api/Controllers/DynamicHlsController.cs` - streaming endpoint. Study for HLS serving patterns.
- **Device profiles:** `../jellyfin/MediaBrowser.Model/Dlna/DeviceProfile.cs`, `DirectPlayProfile.cs` - static profile-based matching. Study to understand what we're replacing.
- **Summary reference:** `../docs/JELLYFIN_REFERENCE.md` - key Jellyfin files studied, what it gets right/wrong, design lessons for Tenkile

### CodecProbe (JavaScript - active project)

- **Codec tester:** `../codecprobe/js/codec-tester.js` - tests codecs via 3 browser APIs
- **Codec database:** `../codecprobe/js/codec-database-v2.js` - complete codec/container/scenario definitions
- **DRM detection:** `../codecprobe/js/drm-detection.js` - probes Widevine, PlayReady, FairPlay, ClearKey
- **Device detection:** `../codecprobe/js/device-detection.js` - gathers UA, screen, hardware concurrency, GPU info

### Critical Design Principles (from architecture review)

1. **Device APIs lie.** `canPlayType()` returns "maybe" for codecs the device can't play smoothly. `isTypeSupported()` says true for HEVC on browsers that can only software-decode at 720p. Smart TV firmware reports support for profiles their hardware can't handle. Use multi-source verification with trust scores.

2. **Server encoding matters as much as client decoding.** A decision to "transcode to AV1" is useless if the server has no AV1 encoder. Every transcode target must be checked against the server's actual FFmpeg + hardware acceleration capabilities.

3. **Best possible output for the device.** AV1 HDR10 -> HEVC HDR10 when device has HDR display. AV1 HDR10 -> HEVC SDR with high-quality tone mapping when device has no HDR display. TrueHD Atmos 7.1 -> E-AC-3 7.1 (not stereo AAC). Always maximize quality for what the device can actually render.

4. **No server-side WebView/CEF.** CodecProbe's browser APIs test the CLIENT's engine, not the server's. Running them server-side is useless. Standalone client probe is the only viable model.

5. **Self-hosting first.** Single Go binary, <15MB, <50MB memory. Runs on a Raspberry Pi. No runtime dependencies.

### Key Weak Points in Jellyfin (what we're fixing)

1. **StreamBuilder is a monolith** - 46 methods, 2400+ lines. Tightly couples container checks, codec checks, profile conditions, bitrate limits, subtitle handling.
2. **Static device profiles** - `DirectPlayProfile` is just string matching. No probing, no confidence, no scenarios.
3. **No codec confidence levels** - Binary yes/no. CodecProbe provides multi-API consensus plus smooth/powerEfficient per scenario.
4. **No server-side capability awareness** - Doesn't check what the server can encode before choosing a transcode target.
5. **Quality destruction on transcode** - No intelligent HDR handling, no policy for audio channel preservation.
6. **No decision logging** - `TranscodeReason` enum exists but isn't persisted or queryable.
7. **No playback feedback** - When direct-play fails, the system doesn't learn from it.

---

## Phase 3 Inventory (COMPLETED)

Phase 3 is done. The following packages, types, and functions exist. Phase 4+ agents MUST use these — do not redefine them.

### Package: `internal/probes` — Curated Device Database (Phase 3.1)

**Type ownership:**

| File | Types owned |
|------|------------|
| `curated.go` | `CuratedDevice`, `KnownIssue`, `CuratedDatabase`, `DatabaseStats`, `SearchCriteria`, `VendorRules`, `VendorRule` |
| `curated_fuzzy.go` | `FuzzyMatchResult`, `VersionMatchResult` |
| `embedded.go` | `EmbeddedLoader`, `EmbeddedDeviceBundle`, `BundleMetadata` |
| `feedback.go` | `FeedbackManager`, `TrustAdjustmentConfig`, `PlaybackFeedback`, `PlaybackOutcome`, `DevicePlaybackStats`, `CodecStats` |
| `soc_inference.go` | `SoCCapabilityTable`, `CodecCapabilities`, SoC inference functions | ✓ NEW |

**VERIFICATION CHECKLIST (Phase 3.1):**
- [x] All listed types exist and compile
- [x] All listed methods have implementations (not stubs)
- [x] API endpoints registered in router.go
- [x] Tests exist in probes package
- [x] Embedded data loads via `go:embed`

**Key function signatures:**

```go
// Curated Database
func NewCuratedDatabase() *CuratedDatabase
func (cd *CuratedDatabase) Load(dataDir string) error
func (cd *CuratedDatabase) LoadFromEmbedded(deviceJSON []byte) error
func (cd *CuratedDatabase) GetByID(id string) (*CuratedDevice, bool)
func (cd *CuratedDatabase) GetByDeviceHash(deviceHash string) (*CuratedDevice, bool)
func (cd *CuratedDatabase) GetByPlatform(platform string) []*CuratedDevice
func (cd *CuratedDatabase) Search(criteria SearchCriteria) []*CuratedDevice
func (cd *CuratedDatabase) AddDevice(device *CuratedDevice) error
func (cd *CuratedDatabase) UpdateDevice(device *CuratedDevice) error
func (cd *CuratedDatabase) RemoveDevice(deviceID string) error
func (cd *CuratedDatabase) Vote(deviceID string, up bool) error
func (cd *CuratedDatabase) MarkVerified(deviceID string) error
func (cd *CuratedDatabase) GetRecommendedProfile(device *CuratedDevice) string
func (cd *CuratedDatabase) GetKnownIssues(device *CuratedDevice) []KnownIssue
func (cd *CuratedDatabase) GetAll() []*CuratedDevice
func (cd *CuratedDatabase) GetStats() DatabaseStats
func (cd *CuratedDatabase) MatchDevice(deviceName string, platform string, limit int) []*FuzzyMatchResult
func (cd *CuratedDatabase) VersionAwareMatch(deviceName string, platform string) *VersionMatchResult
func (cd *CuratedDatabase) SearchWithFuzzy(query string, platform string, limit int) []*CuratedDevice
func (cd *CuratedDatabase) Count() int

// Vendor Rules (enforced per DEVICE_DATABASE.md Section 4)
func GetVendorRules(platform string) VendorRules
func (cd *CuratedDatabase) ApplyVendorRules(device *CuratedDevice) *CuratedDevice
func (cd *CuratedDatabase) GetKnownLimitations(platform string) []string
// VendorRule types: NoDolbyVision, NoDTS, NoTrueHD, NoHDR10Plus, ContainerRestrictions, etc.

// Embedded Loader
func InitEmbeddedLoader() error
func GetEmbeddedLoader() *EmbeddedLoader
func (el *EmbeddedLoader) LoadAll() error
func (el *EmbeddedLoader) GetPlatforms() []string
func (el *EmbeddedLoader) GetDevices(platform string) []*CuratedDevice
func (el *EmbeddedLoader) GetAllDevices() []*CuratedDevice
func (el *EmbeddedLoader) GetTotalCount() int
func (el *EmbeddedLoader) LoadIntoCuratedDB(db *CuratedDatabase) error
func (el *EmbeddedLoader) GetAllBundlesInfo() map[string]map[string]interface{}
```

### SoC Inference (Phase 3.1)

```go
// SoC Capability Table — 20+ entries covering major chipsets
var SoCCapabilityTable map[string]CodecCapabilities

// CodecCapabilities describes what a SoC can decode
type CodecCapabilities struct {
    MaxResolution  string   // e.g., "4K", "1080p"
    MaxFPS        int      // e.g., 60, 120
    HDRSupport    []string // e.g., ["hdr10", "hlg", "dv"]
    HWDecoders    []string // e.g., ["hevc", "av1", "vp9"]
}

// SoC Inference Functions
func InferCapabilitiesFromSoC(soc string) *CodecCapabilities
func InferDeviceClass(priceUSD int, soc string) string  // "low", "mid", "premium", "high-end"
func ResolveSoCAliases(soc string) string  // Maps "MT5895" -> "MediaTek MT5895"

// Major SoCs covered:
// MediaTek: MT5891, MT5895, MT5893, MT9602, MT9612
// Realtek: RTD2873, RTD2885, RTD1319
// Samsung: Tizen 6.5 (Exynos)
// Qualcomm: Snapdragon 6/7/8 series
// Amazon: MT8127, MT8581
// Apple: T2 (legacy), M-series
```

### Package: `internal/probes` — Playback Feedback Loop (Phase 3.2)

**Type ownership:**

| File | Types owned |
|------|------------|
| `feedback.go` | `FeedbackManager`, `TrustAdjustmentConfig`, `PlaybackFeedback`, `PlaybackOutcome`, `DevicePlaybackStats`, `CodecStats`, `PlaybackEvent` |
| `feedback_metrics.go` | `PlaybackMetrics`, `PlaybackCounterValue`, `TrustScoreValue`, `ReProbeCounterValue`, `LatencyHistogram` |
| `feedback_integration.go` | `FeedbackIntegration`, `IntegrationConfig` |

**VERIFICATION CHECKLIST (Phase 3.2):**
- [x] All listed types exist and compile
- [x] Trust deltas match specification (Success +0.01, Codec Error -0.15, etc.)
- [x] ShouldReProbe() triggers after 3+ consecutive failures
- [x] Global stats updated for both video and audio-only streams
- [x] Race condition fixed: capture values before goroutine spawn
- [x] Trust decay actually assigns computed values back
- [x] Prometheus metrics export works
- [x] Feedback integration connected to TrustResolver

**Key function signatures:**

```go
// Feedback Manager
func NewFeedbackManager() *FeedbackManager
func (fm *FeedbackManager) SetTrustConfig(config TrustAdjustmentConfig)
func (fm *FeedbackManager) RecordSuccess(feedback PlaybackFeedback)
func (fm *FeedbackManager) RecordFailure(feedback PlaybackFeedback)
func (fm *FeedbackManager) CalculateTrustDelta(outcome PlaybackOutcome) float64
func (fm *FeedbackManager) ShouldReProbe(deviceID string) (bool, string)
func (fm *FeedbackManager) GetPlaybackStats(deviceID string) *DevicePlaybackStats
func (fm *FeedbackManager) GetReliableCodecs(deviceID string, minSuccessRate float64) []string
func (fm *FeedbackManager) GetTrustAdjustment(deviceID string) float64
func (fm *FeedbackManager) ResetTrustAdjustment(deviceID string)
func (fm *FeedbackManager) ClearDeviceData(deviceID string)
func (fm *FeedbackManager) GetGlobalStats() map[string]interface{}
func (fm *FeedbackManager) ExpireOldEvents() int
func ParseOutcomeFromString(s string) PlaybackOutcome

// PlaybackOutcome enum values (9 total)
OutcomeUnknown, OutcomeSuccess, OutcomeNetworkError, OutcomeCodecError,
OutcomeDecodingFailed, OutcomeRendererCrash, OutcomeUnsupportedFormat,
OutcomeTimeout, OutcomeBuffering

// TrustAdjustmentConfig defaults
func DefaultTrustAdjustmentConfig() TrustAdjustmentConfig
// SuccessBonus: 0.01, NetworkErrorPenalty: 0.05, CodecErrorPenalty: 0.15,
// DecodingFailedPenalty: 0.25, RendererCrashPenalty: 0.30,
// FailureWindowSize: 3, SuccessStreakBonus: 0.05, SuccessStreakThreshold: 10

// Metrics
func GetGlobalPlaybackMetrics() *PlaybackMetrics
func (m *PlaybackMetrics) RecordPlayback(deviceID string, outcome PlaybackOutcome)
func (m *PlaybackMetrics) RecordTrustScore(deviceID string, score float64)
func (m *PlaybackMetrics) RecordReProbe(deviceID string, reason string)
func (m *PlaybackMetrics) RecordFeedbackLatency(duration time.Duration)
func (m *PlaybackMetrics) ExportPrometheusFormat() string
func (m *PlaybackMetrics) ExportMetrics() map[string]interface{}

// Integration
func NewFeedbackIntegration(fm *FeedbackManager, tr *TrustResolver, cache *CapabilityCache) *FeedbackIntegration
func (fi *FeedbackIntegration) GetEffectiveTrustScore(deviceID string) float64
func (fi *FeedbackIntegration) ShouldTranscodeForTrust(deviceID string, codec string) (bool, string)
func (fi *FeedbackIntegration) GetTrustReport(deviceID string) map[string]interface{}
func (fi *FeedbackIntegration) ResetDeviceTrust(deviceID string)
func (fi *FeedbackIntegration) StartDecayLoop(ctx context.Context)
```

### API Additions (Phase 3.1)

Routes added to `internal/api/admin.go`:
- `PUT /api/v1/admin/curated/devices` — Create curated device
- `PUT /api/v1/admin/curated/devices/{id}` — Replace curated device
- `DELETE /api/v1/admin/curated/devices/{id}` — Delete curated device
- `POST /api/v1/admin/curated/devices/{id}/vote` — Vote on device
- `POST /api/v1/admin/curated/search` — Fuzzy search devices
- `POST /api/v1/admin/curated/version-match` — Version-aware matching
- `GET /api/v1/admin/curated/embedded/stats` — Embedded bundle stats
- `POST /api/v1/admin/curated/embedded/sync` — Sync embedded to DB
- `GET /api/v1/admin/curated/export` — Export curated devices
- `POST /api/v1/admin/curated/import` — Import curated devices

### API Additions (Phase 3.2)

Routes added to `internal/api/devices.go` and `router.go`:
- `POST /api/v1/devices/{id}/feedback` — Submit playback feedback
- `GET /api/v1/devices/{id}/feedback/stats` — Get playback statistics
- `GET /api/v1/devices/{id}/reliable-codecs` — Get reliable codecs
- `POST /api/v1/devices/{id}/reprobe` — Trigger re-probe
- `GET /api/v1/devices/{id}/trust` — Get trust report
- `GET /api/v1/feedback/metrics` — Get feedback metrics

### Embedded Device Bundles (Phase 3.1)

Platforms supported via `go:embed` (12 total, loaded via `LoadAll()`):
- `samsung_tizen` — Samsung Smart TVs (2018-2025) — AV1 from 2022+
- `lg_webos` — LG Smart TVs (2018-2025) — Dolby Vision on ALL models
- `roku` — Roku devices (OS 9-13)
- `android_tv` — Android TV, Chromecast with Google TV
- `amazon_fire_tv` — Amazon Fire TV (separate from Android TV) ✓ FIXED
- `apple_tv` — Apple TV 4K/HD (no MKV/WebM)
- `chromecast` — Chromecast devices
- `playstation` — PlayStation 4/5
- `xbox` — Xbox One/Series
- `nvidia_shield` — NVIDIA Shield TV
- `mi_box` — Xiaomi Mi Box
- `shield_tv` — NVIDIA Shield (legacy naming)

### Trust Adjustment Rules (Phase 3.2)

| Outcome | Trust Delta | Notes |
|---------|-------------|-------|
| Success | +0.01 | Per successful playback |
| Network Error | -0.05 | Temporary, may recover |
| Codec Error | -0.15 | Significant concern |
| Decoding Failed | -0.25 | Major issue |
| Renderer Crash | -0.30 | Severe problem |
| Buffering | -0.025 | Less severe |
| Timeout | -0.05 | Same as network error |

**Re-probe triggers:**
- 3+ consecutive failures → trigger re-probe
- >50% failure rate in last 10+ playbacks → trigger re-probe
- Success streak of 10+ → +0.05 bonus

**Trust score bounds:** 0.0 (untrusted) to 1.0 (verified)

---

## DEVICE_DATABASE.md Integration

The curated database implementation is sourced from `/Users/siegfried/claudeAIcli/jellyfin_new/docs/DEVICE_DATABASE.md`. All device profiles and SoC inference MUST reference this specification.

### Sections Used

| Section | Content | Used In |
|---------|---------|---------|
| 4.1 Video Codec Matrix | 24 platforms × 9 codecs | Device JSON profiles |
| 4.2 Audio Codec Matrix | 14 platforms × 11 codecs | Device JSON profiles |
| 4.3 Container Matrix | 11 platforms × 10 containers | Device JSON profiles |
| 4.4 Subtitle Matrix | 9 platforms × 7 formats | Device JSON profiles |
| 6 SoC Inference Table | 20+ SoC → capabilities | `soc_inference.go` |

### Key Specifications Incorporated

#### Samsung Rules
- No Dolby Vision (ever) — enforced by `vendorRules`
- No DTS (ever) — enforced by `vendorRules`
- AV1 only from 2022+ — enforced by year-based rules
- HDR10+ supported — captured in profiles

#### LG Rules
- Dolby Vision on ALL models — enforced by `vendorRules`
- No HDR10+ (LG never adopted) — enforced by `vendorRules`
- DTS on premium models only — captured in profiles

#### Amazon Fire TV Rules
- No DTS — enforced by `vendorRules`
- No TrueHD — enforced by `vendorRules`
- Dolby Vision Profile 8 — captured in profiles
- AV1 on 3rd gen+ — captured in profiles

#### Apple TV Rules
- No MKV container — captured in profiles
- No WebM — captured in profiles
- Full DTS, TrueHD, Atmos — captured in profiles

#### Container Support Matrix

| Platform | MKV | MP4 | MOV | WebM | FLAC |
|----------|-----|-----|-----|------|------|
| Apple TV | ✗ | ✓ | ✓ | ✗ | ✓ |
| Samsung | ✓ | ✓ | ✗ | ✗ | ✓ |
| LG | ✓ | ✓ | ✗ | ✗ | ✓ |
| Roku | ✓ | ✓ | ✗ | ✗ | ✗ |
| Android TV | ✓ | ✓ | ✓ | ✓ | ✓ |
| Fire TV | ✓ | ✓ | ✗ | ✗ | ✓ |

#### Subtitle Support Matrix

| Platform | SSA/ASS | PGS | VOBSUB | TTML |
|----------|---------|-----|--------|------|
| Amazon Fire TV | ✗ | ✓ | ✓ | ✗ |
| Samsung | ✓ | ✓ | ✓ | ✓ |
| LG | ✓ | ✓ | ✓ | ✓ |
| Apple TV | ✓ | ✗ | ✗ | ✓ |
| iOS Safari | ✓ | ✓ | ✗ | ✓ |

### Bugs Fixed During Integration

| Bug | Source | Fix |
|-----|--------|-----|
| Samsung QN90B missing AV1 | `embedded/samsung_tizen.json` | Added av1 to 2022+ Samsung |
| Fire TV misclassified as Android TV | `embedded/android_tv.json` | Moved to `amazon_fire_tv.json` |
| Only 4 of 12 platforms embedded | `embedded.go` | `LoadAll()` now loads all 12 |
| Missing SoC-based inference | (new feature) | Created `soc_inference.go` |
| Missing vendor behavior rules | (new feature) | Added to `curated.go` |

### Implementation Drift (Phase 3)

| Spec | Implementation | Resolution |
|------|----------------|------------|
| All 12 platforms in DEVICE_DATABASE.md | Only 4 platforms loaded | Fixed — `LoadAll()` now loads all 12 |
| AV1 support for Samsung 2022+ | Missing in QN90B | Fixed — added to 2022+ models |
| Amazon Fire TV separate platform | Misclassified as Android TV | Fixed — own file `amazon_fire_tv.json` created |
| SoC inference per Section 6 | Not in original spec | Implemented as `soc_inference.go` |
| Vendor behavior rules per Section 4 | Not in original spec | Implemented as `VendorRules` in `curated.go` |

---

## Phase 4 Inventory (COMPLETED)

Phase 4 is complete. The following packages, types, and functions exist. Phase 5+ agents MUST use these — do not redefine them.

### Package: `internal/media` (Phase 4.1)

**Type ownership:**

| File | Types owned |
|------|------------|
| `models.go` | `MediaItem`, `Library`, `LibraryType`, `LibraryScanStatus`, `ScanStatus`, `MediaStream`, `VideoStream`, `AudioStream`, `SubtitleStream` |
| `scanner.go` | `Scanner`, `SkipPatterns`, `MediaExtensions` |
| `probe.go` | `FFprobe`, `FFprobeOptions` |
| `store.go` | `Store` |

**Key function signatures:**

```go
// Scanner
func NewScanner(ffprobe *FFprobe, store *Store) *Scanner
func (s *Scanner) ScanLibrary(ctx context.Context, lib *Library) error
func (s *Scanner) ScanPath(ctx context.Context, path string) ([]*MediaItem, error)
func (s *Scanner) GetStatus(libraryID string) *LibraryScanStatus

// FFprobe
func NewFFprobe(path string) (*FFprobe, error)
func (fp *FFprobe) Probe(ctx context.Context, filePath string) (*MediaItem, error)
func (fp *FFprobe) ProbeWithOptions(ctx context.Context, filePath string, opts FFprobeOptions) (*MediaItem, error)

// Store
func NewStore(db *sql.DB) *Store
func (st *Store) GetAllLibraries(ctx context.Context) ([]*Library, error)
func (st *Store) GetLibrary(ctx context.Context, id string) (*Library, error)
func (st *Store) SaveLibrary(ctx context.Context, lib *Library) error
func (st *Store) DeleteLibrary(ctx context.Context, id string) error
func (st *Store) GetMediaItems(ctx context.Context, libraryID string, offset, limit int) ([]*MediaItem, error)
func (st *Store) GetMediaItem(ctx context.Context, id string) (*MediaItem, error)
```

### Package: `internal/stream` (Phase 4.2)

**Type ownership:**

| File | Types owned |
|------|------------|
| `models.go` | `StreamSession`, `HLSOptions`, `StreamVariant` |
| `segmenter.go` | `Segmenter`, `SegmentResult` |
| `handler.go` | `Handler` |

**Key function signatures:**

```go
// Handler
func NewHandler(mediaStore *media.Store) *Handler
func (h *Handler) ServeHLS(w http.ResponseWriter, r *http.Request)
func (h *Handler) ServeHLSManifest(w http.ResponseWriter, r *http.Request)
func (h *Handler) ServeHLSSegment(w http.ResponseWriter, r *http.Request)

// Segmenter
func NewSegmenter(ffmpegPath, tempDir string, inv *server.Inventory) *Segmenter
func (s *Segmenter) GenerateHLS(ctx context.Context, inputPath string, variants []StreamVariant, opts HLSOptions) (*SegmentResult, error)
```

### Package: `internal/api` — Auth (Phase 4.4)

**Type ownership:**

| File | Types owned |
|------|------------|
| `auth.go` | `Claims`, `AuthHandler`, `User` |

**Key function signatures:**

```go
// Auth Handler
func NewAuthHandler(db *sql.DB, jwtSecret string) *AuthHandler
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request)
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request)
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request)
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request)

// JWT Claims
type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}
```

### Library API Additions (Phase 4.1)

Routes added to `internal/api/library.go`:
- `GET /api/v1/libraries` — List all libraries
- `POST /api/v1/libraries` — Create library
- `GET /api/v1/libraries/{id}` — Get library
- `PUT /api/v1/libraries/{id}` — Update library
- `DELETE /api/v1/libraries/{id}` — Delete library
- `POST /api/v1/libraries/{id}/scan` — Trigger scan
- `GET /api/v1/libraries/{id}/items` — List items (paginated)
- `GET /api/v1/libraries/{id}/item/{itemId}` — Get media item

### Streaming API Additions (Phase 4.2)

Routes defined in `internal/stream/handler.go`:
- `GET /api/v1/stream/{id}/hls` — Generate HLS manifest
- `GET /api/v1/stream/{id}/manifest` — Serve HLS playlist
- `GET /api/v1/stream/{id}/segment` — Serve HLS segment

### Auth API Additions (Phase 4.4)

Routes added to `internal/api/auth.go`:
- `POST /api/v1/auth/login` — Login (returns JWT access + refresh token)
- `POST /api/v1/auth/refresh` — Refresh access token
- `POST /api/v1/auth/logout` — Logout (invalidate refresh token)
- `GET /api/v1/auth/me` — Current user profile

### OpenAPI Specification (Phase 4.3)

File: `api/openapi.yaml` — Complete OpenAPI 3.1.0 specification with:
- 50+ endpoints documented
- Device & probe endpoints (from Phase 1)
- Playback decision endpoints (from Phase 2)
- Curated device endpoints (from Phase 3)
- Library endpoints (from Phase 4.1)
- Auth endpoints (from Phase 4.4)

### Database Migrations Added (Phase 4.1)

| Migration | Tables |
|-----------|--------|
| `20260323000001_create_media_libraries.sql` | `media_libraries` |
| `20260323000002_create_media_items.sql` | `media_items`, `media_videos`, `media_audio`, `media_subtitles` |
| `20260323000003_create_series_episodes.sql` | `media_series`, `media_episodes` |
| `20260323000004_create_streams_sessions.sql` | `stream_sessions` |

### All Phase 4 Tasks Complete

All Phase 4 sub-tasks have been implemented:

| Task | Status | Notes |
|------|--------|-------|
| 4.1 Media Scanner | ✅ COMPLETE | FFprobe integration, SQLite persistence |
| 4.2 HLS/DASH Streaming | ✅ COMPLETE | Handler, segmenter, manifest serving |
| 4.3 OpenAPI Spec | ✅ COMPLETE | Complete OpenAPI 3.1.0 spec |
| 4.4 Auth + First-Run | ✅ COMPLETE | JWT, bcrypt, user management |
| 4.5 Web Client | ✅ COMPLETE | Preact app with all views |
| 4.6 WebSocket Events | ✅ COMPLETE | gorilla/websocket, event bus |
| 4.7 Docker + Deploy | ✅ COMPLETE | docker-compose.yml, Dockerfile, nginx config |

### Module Dependencies (Phase 4)

Dependencies added in go.mod:
```
github.com/golang-jwt/jwt/v5     (Phase 4.4 - Auth)
github.com/gorilla/websocket     (Phase 4.6 - WebSocket)
golang.org/x/crypto              (Phase 4.4 - bcrypt)
```

```
Phase 1: Foundation                                        ✓ COMPLETED
  1.1 Scaffold ──────────────────────┐
  1.2 Codec Types ───┐               │
  1.3 Probe Receiver ┤ (needs 1.1)   │
  1.4 JS Probe Lib ──┘               │
                                     │
Phase 2: Server + Decision Engine      ✓ COMPLETED
  2.1 Server Inventory ──┐ (needs 1.1)
  2.2 Transcode Engine ──┤ (needs 1.2, 1.3, 2.1)
  2.3 Decision Logger ───┘ (needs 2.2)
                                     │
Phase 3: Curated DB + Feedback        ✓ COMPLETED
  3.1 Curated Smart TV DB ─┐ (needs 1.3)
  3.2 Playback Feedback ───┘ (needs 1.3, 2.3)
                                     │
Phase 4: Media Library + Streaming       ✅ COMPLETE
  4.1 Media Scanner ─────┐ ✓ COMPLETE
  4.2 HLS/DASH Streaming ┤ ✓ COMPLETE (HLS generation, manifest serving, segment delivery)
  4.3 OpenAPI Spec ──────┤ ✓ COMPLETE (openapi.yaml complete)
  4.4 Auth + First-Run ──┤ ✓ COMPLETE (JWT, bcrypt, user management)
  4.5 Web Client ────────┤ ✓ COMPLETE (Preact app with login, dashboard, library views)
  4.6 WebSocket Events ──┤ ✓ COMPLETE (gorilla/websocket, event bus, real-time updates)
  4.7 Docker + Deploy ───┘ ✓ COMPLETE (docker-compose.yml, Dockerfile, nginx config)
                                     │
Phase 5: Client Adapters              ✅ COMPLETE
  5.1 Native Probes ─────┐ ✓ COMPLETE
  5.2 Android Probe ─────┤ ✓ COMPLETE
  5.3 Apple Probe ───────┘ ✓ COMPLETE

Phase 6: Polish + Production           ○ PLANNED
```

**Parallelization opportunities:**
- 1.2 + 1.4 can run in parallel (both need 1.1 only)
- 2.1 can start as soon as 1.1 is done (independent of 1.2–1.4)
- 3.1 can start as soon as 1.3 is done (independent of Phase 2)
- 4.1 can start as soon as 1.1 is done (independent of Phases 2–3)
- 5.1 can start after 1.3 + 2.2 (independent of Phases 3–4)

---

## Architecture Review

> **Last Updated:** Phase 4 Complete (2026-03-23)

This section documents the current system architecture after Phase 4 completion.

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                    CLIENTS                                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ Web Browser │  │ Android TV  │  │ Apple TV    │  │ Smart TVs (Tizen,   │ │
│  │ (CodecProbe)│  │ (MediaCodec)│  │ (AVFound.)  │  │  WebOS, Roku)       │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘ │
└─────────┼────────────────┼────────────────┼──────────────────────┼────────────┘
          │                │                │                      │
          │ HTTP/REST     │                │                      │
          │ + WebSocket   │                │                      │
          ▼                ▼                ▼                      ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                  TENKILE SERVER                                   │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │                          HTTP API (chi router)                               ││
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ ││
│  │  │ AuthHandler  │  │LibraryHandler│  │ MediaHandler │  │AdminHandler  │ ││
│  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘ ││
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ ││
│  │  │DeviceHandler │  │PlaybackHandler│ │StreamHandler │  │ WebSocket    │ ││
│  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘ ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                          │                                        │
│  ┌───────────────────────────────────────┼───────────────────────────────────────┐│
│  │                                       ▼                                        ││
│  │  ┌─────────────────────────────────────────────────────────────────────────┐ ││
│  │  │                    INTERNAL PACKAGES (Phase 1-4)                        │ ││
│  │  │                                                                         │ ││
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │ ││
│  │  │  │   probes/   │  │   server/   │  │  transcode/  │  │   media/    │  │ ││
│  │  │  │             │  │             │  │              │  │             │  │ ││
│  │  │  │ • types     │  │ • inventory │  │ • orchestr.  │  │ • scanner   │  │ ││
│  │  │  │ • cache     │  │ • encoder   │  │ • quality    │  │ • ffprobe    │  │ ││
│  │  │  │ • curated   │  │ • benchmark │  │ • ffmpeg     │  │ • store      │  │ ││
│  │  │  │ • trust     │  │             │  │ • subtitle   │  │ • models     │  │ ││
│  │  │  │ • feedback  │  │             │  │ • logger     │  │             │  │ ││
│  │  │  │ • validator │  │             │  │ • matcher    │  │             │  │ ││
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘  │ ││
│  │  │                                    │                    │               │ ││
│  │  │         ┌─────────────────────────┼────────────────────┘               │ ││
│  │  │         │                         ▼                                     │ ││
│  │  │         │  ┌─────────────────────────────────────────────────────────┐   │ ││
│  │  │         │  │                      stream/                             │   │ ││
│  │  │         │  │              Handler + Segmenter + Models                │   │ ││
│  │  │         │  └─────────────────────────────────────────────────────────┘   │ ││
│  │  └───────────────────────────────────────────────────────────────────────────┘ ││
│  │                                                                                 ││
│  │  ┌───────────────────────────────────────────────────────────────────────────┐ ││
│  │  │                         DATABASE (SQLite/PostgreSQL)                       │ ││
│  │  │    users | devices | playback_feedback | media_libraries | media_items  │ ││
│  │  └───────────────────────────────────────────────────────────────────────────┘ ││
│  │                                                                                 ││
│  │  ┌───────────────────────────────────────────────────────────────────────────┐ ││
│  │  │                         EMBEDDED ASSETS (go:embed)                        │ ││
│  │  │    • web client (Preact SPA)  • curated device database (12 platforms)   │ ││
│  │  │    • migrations  • default config                                        │ ││
│  │  └───────────────────────────────────────────────────────────────────────────┘ ││
│  └───────────────────────────────────────────────────────────────────────────────┘│
│                                          │                                        │
└──────────────────────────────────────────┼────────────────────────────────────────┘
                                           │
                         ┌─────────────────┼─────────────────┐
                         ▼                 ▼                 ▼
                  ┌────────────┐    ┌────────────┐    ┌────────────┐
                  │   FFmpeg   │    │   FFprobe  │    │   NVIDIA   │
                  │  (encode)  │    │  (probe)   │    │  (nvidia-  │
                  │            │    │            │    │   smi)     │
                  └────────────┘    └────────────┘    └────────────┘
```

### Component Descriptions

| Component | Package | Purpose | Key Types | Dependencies |
|-----------|---------|---------|-----------|-------------|
| **API Router** | `internal/api/router.go` | HTTP routing, middleware chain, CORS | `NewRouter()`, `ServeHTTP()` | All handlers |
| **Auth Handler** | `internal/api/auth.go` | JWT issuance, token refresh, first-run, logout | `AuthHandler`, `Claims` | `golang-jwt`, `bcrypt` |
| **Device Handler** | `internal/api/devices.go` | Probe registration, capability lookup, feedback | `DeviceHandlers` | `probes/` |
| **Playback Handler** | `internal/api/playback.go` | Decision requests, transcode initiation | `PlaybackHandlers` | `transcode/` |
| **Library Handler** | `internal/api/library.go` | Library CRUD, scan triggers, media browsing | `LibraryHandlers` | `media/` |
| **Media Handler** | `internal/api/media.go` | Media item details, stream info | `MediaHandlers` | `media/` |
| **Stream Handler** | `internal/stream/handler.go` | HLS manifest serving, segment delivery | `Handler` | `stream/`, `media/` |
| **WebSocket Handler** | `internal/events/websocket.go` | Real-time event distribution | `WebSocketHandler` | `gorilla/websocket` |
| **Admin Handler** | `internal/api/admin.go` | System stats, cache management, curated DB CRUD | `AdminHandlers` | All packages |
| **Probes Package** | `internal/probes/` | Device capability resolution, trust scoring | `DeviceCapabilities`, `TrustResolver`, `Validator`, `CuratedDatabase` | SQLite |
| **Server Package** | `internal/server/` | FFmpeg/HW capability discovery | `Inventory`, `ServerCapabilities`, `EncoderCapability` | FFmpeg, nvidia-smi |
| **Transcode Package** | `internal/transcode/` | Playback decision engine, FFmpeg argument building | `Orchestrator`, `PlaybackDecision`, `FFmpegArgs` | `probes/`, `server/` |
| **Media Package** | `internal/media/` | Library scanning, ffprobe integration, persistence | `Scanner`, `FFprobe`, `Store`, `Library`, `MediaItem` | SQLite, ffprobe |
| **Stream Package** | `internal/stream/` | HLS/DASH segment generation | `Segmenter`, `HLSOptions`, `StreamVariant` | FFmpeg |
| **Events Package** | `internal/events/` | Event bus for real-time notifications | `Bus`, `Event` | WebSocket |
| **Database Package** | `internal/database/` | SQLite wrapper, migrations | `SQLite` | `modernc.org/sqlite` |
| **Codec Package** | `pkg/codec/` | Codec constants, MIME type mapping | (constants) | None |
| **Config Package** | `internal/config/` | YAML config loading | `Config` | `gopkg.in/yaml.v3` |

### API Route Structure

| Route | Method | Handler | Auth | Description |
|-------|--------|---------|------|-------------|
| **Auth** |
| `/api/v1/auth/login` | POST | `AuthHandler.Login` | No | Login with credentials |
| `/api/v1/auth/first-run` | POST | `AuthHandler.FirstRun` | No | Initial admin setup |
| `/api/v1/auth/refresh` | POST | `AuthHandler.Refresh` | No | Refresh access token |
| `/api/v1/auth/logout` | POST | `AuthHandler.Logout` | Yes | Invalidate refresh token |
| `/api/v1/auth/me` | GET | `AuthHandler.Me` | Yes | Current user info |
| **Libraries** |
| `/api/v1/libraries` | GET | `LibraryHandler.List` | Yes | List all libraries |
| `/api/v1/libraries` | POST | `LibraryHandler.Create` | Yes | Create library |
| `/api/v1/libraries/{id}` | GET | `LibraryHandler.Get` | Yes | Get library |
| `/api/v1/libraries/{id}` | PUT | `LibraryHandler.Update` | Yes | Update library |
| `/api/v1/libraries/{id}` | DELETE | `LibraryHandler.Delete` | Yes | Delete library |
| `/api/v1/libraries/{id}/scan` | POST | `LibraryHandler.Scan` | Yes | Trigger scan |
| `/api/v1/libraries/{id}/scan/status` | GET | `LibraryHandler.ScanStatus` | Yes | Scan progress |
| `/api/v1/libraries/{libraryId}/items` | GET | `LibraryHandler.Items` | Yes | List items (paginated) |
| **Media** |
| `/api/v1/media/{id}` | GET | `MediaHandler.Get` | Yes | Media item details |
| `/api/v1/media/{id}/stream` | GET | `MediaHandler.StreamInfo` | Yes | Stream info + variants |
| `/api/v1/media/{id}/play` | GET | `MediaHandler.Play` | Yes | Playback manifest |
| **Devices** |
| `/api/v1/devices/{id}/probe` | POST | `DeviceHandler.Probe` | Yes | Register probe results |
| `/api/v1/devices/{id}/capabilities` | GET | `DeviceHandler.Capabilities` | Yes | Get capabilities |
| `/api/v1/devices/{id}/validate` | POST | `DeviceHandler.Validate` | Yes | Validate capabilities |
| `/api/v1/devices/search` | POST | `DeviceHandler.Search` | Yes | Search devices |
| `/api/v1/devices/{id}/feedback` | POST | `DeviceHandler.Feedback` | Yes | Submit playback feedback |
| `/api/v1/devices/{id}/feedback/stats` | GET | `DeviceHandler.FeedbackStats` | Yes | Get feedback stats |
| `/api/v1/devices/{id}/reliable-codecs` | GET | `DeviceHandler.ReliableCodecs` | Yes | Get reliable codecs |
| `/api/v1/devices/{id}/reprobe` | POST | `DeviceHandler.ReProbe` | Yes | Trigger re-probe |
| `/api/v1/devices/{id}/trust` | GET | `DeviceHandler.TrustReport` | Yes | Get trust report |
| **Playback** |
| `/api/v1/playback/decision` | POST | `PlaybackHandler.Decide` | Yes | Get playback decision |
| `/api/v1/playback/feedback` | POST | `PlaybackHandler.Feedback` | Yes | Report playback outcome |
| `/api/v1/playback/transcode` | POST | `PlaybackHandler.Transcode` | Yes | Start transcode |
| `/api/v1/playback/profiles` | GET | `PlaybackHandler.Profiles` | Yes | Get device profiles |
| `/api/v1/playback/validate` | POST | `PlaybackHandler.Validate` | Yes | Validate playback |
| **Stream** |
| `/api/v1/stream/hls/{id}` | GET | `StreamHandler.ServeHLS` | Yes | Get HLS manifest |
| `/api/v1/stream/hls/playlist` | GET | `StreamHandler.ServeHLSManifest` | Yes | Serve playlist file |
| `/api/v1/stream/hls/segment` | GET | `StreamHandler.ServeHLSSegment` | Yes | Serve segment file |
| **Admin** |
| `/api/v1/admin/system` | GET | `AdminHandler.SystemInfo` | Admin | System information |
| `/api/v1/admin/stats` | GET | `AdminHandler.Stats` | Admin | Aggregate statistics |
| `/api/v1/admin/cache/clear` | POST | `AdminHandler.ClearCache` | Admin | Clear capability cache |
| `/api/v1/admin/decisions` | GET | `AdminHandler.Decisions` | Admin | Query decision logs |
| `/api/v1/admin/decisions/stats` | GET | `AdminHandler.DecisionStats` | Admin | Decision statistics |
| `/api/v1/admin/curated/devices` | PUT | `AdminHandler.CreateCurated` | Admin | Create curated device |
| `/api/v1/admin/curated/devices/{id}` | PUT | `AdminHandler.UpdateCurated` | Admin | Update curated device |
| `/api/v1/admin/curated/devices/{id}` | DELETE | `AdminHandler.DeleteCurated` | Admin | Delete curated device |
| `/api/v1/admin/curated/devices/{id}/vote` | POST | `AdminHandler.VoteCurated` | Admin | Vote on device |
| `/api/v1/admin/curated/search` | POST | `AdminHandler.SearchCurated` | Admin | Search curated DB |
| `/api/v1/admin/curated/version-match` | POST | `AdminHandler.VersionMatch` | Admin | Version-aware match |
| `/api/v1/admin/curated/embedded/stats` | GET | `AdminHandler.EmbeddedStats` | Admin | Embedded DB stats |
| `/api/v1/admin/curated/embedded/sync` | POST | `AdminHandler.SyncEmbedded` | Admin | Sync embedded to DB |
| `/api/v1/admin/curated/export` | GET | `AdminHandler.ExportCurated` | Admin | Export curated DB |
| `/api/v1/admin/curated/import` | POST | `AdminHandler.ImportCurated` | Admin | Import curated DB |
| **Feedback** |
| `/api/v1/feedback/metrics` | GET | `DeviceHandler.FeedbackMetrics` | Yes | Get feedback metrics |
| **WebSocket** |
| `/ws` | GET | `WebSocketHandler.Handle` | Yes | Real-time event stream |

### Event Flow Diagrams

#### Scan Event Flow

```
Client                    API                      Scanner                      Event Bus                    WebSocket
  │                        │                          │                            │                          │
  │──POST /libraries/{id}/scan──▶│                      │                            │                          │
  │                        │──Start Scan────────────▶│                            │                          │
  │                        │                         │──library:scan:started──▶│                          │
  │                        │                         │                         │──▶│◀────────────────────────▶│
  │                        │◀──202 Accepted──────────│                          │    (broadcast to clients)  │
  │◀──202 Accepted─────────│                         │                            │                          │
  │                        │                         │                            │                          │
  │                        │◀──GET /scan/status──────│                            │                          │
  │◀──Progress update───────│                         │                            │                          │
  │                        │                         │──library:scan:progress──▶│                          │
  │                        │                         │   (per-item)              │──▶│◀────────────────────────▶│
  │                        │                         │                            │                          │
  │                        │                         │──Process each file───────│                            │
  │                        │                         │──Probe metadata─────────│                            │
  │                        │                         │──Save to database───────│                            │
  │                        │                         │                            │                          │
  │                        │                         │──library:scan:complete──│                          │
  │                        │                         │   (with stats)          │──▶│◀────────────────────────▶│
  │                        │◀──GET /scan/status──────│                            │                          │
  │◀──Scan complete────────│                         │                            │                          │
```

#### Stream Event Flow

```
Client                    API                   Orchestrator                 Segmenter                    FFmpeg
  │                        │                         │                          │                          │
  │──GET /media/{id}/play──▶│                         │                          │                          │
  │                        │──Decide()────────────▶│                          │                          │
  │                        │◀──PlaybackDecision─────│                          │                          │
  │                        │   (direct play or transcode target)                │                          │
  │◀──HLS manifest─────────│                         │                          │                          │
  │                        │                         │                          │                          │
  │──GET /stream/hls/{id}─▶│                         │                          │                          │
  │                        │──stream:started──────▶│                          │                          │
  │                        │                         │──GenerateHLS──────────▶│                          │
  │                        │                         │                       │──FFmpeg process────────▶│
  │                        │                         │◀──Segments───────────│                          │
  │◀──M3U8 playlist────────│                         │                          │                          │
  │                        │                         │                          │                          │
  │──GET /stream/hls/seg──▶│                         │                          │                          │
  │◀──Segment file─────────│                         │                          │                          │
  │                        │                         │                          │                          │
  │ (repeat for each segment)                         │                          │                          │
  │                        │                         │──stream:ended─────────▶│                          │
```

#### Transcode Event Flow

```
Client                    API                   Orchestrator                  FFmpeg
  │                        │                         │                          │
  │──POST /playback/transcode─▶│                      │                          │
  │                        │──Decide()────────────▶│                          │
  │                        │◀──TranscodeDecision───│                          │
  │                        │   (codec ladder applied)│                        │
  │                        │                         │                          │
  │                        │──transcode:started───▶│                          │
  │                        │                         │                          │
  │                        │──BuildFFmpegArgs()────▶│                          │
  │                        │◀──FFmpegArgs──────────│                          │
  │                        │                         │                          │
  │                        │────────────────────▶│──ffmpeg command─────────▶│
  │                        │                         │◀──stdout/stderr────────│
  │                        │──transcode:progress──▶│   (per-frame logging)   │
  │                        │──transcode:progress──▶│                          │
  │◀──Progress events──────│                         │                          │
  │                        │                         │◀──exit code────────────│
  │                        │──transcode:complete──▶│   (success/failure)    │
  │                        │                         │                          │
  │──GET /media/{id}/play──▶│   (now serves transcoded file)                  │
  │◀──Transcoded HLS───────│                         │                          │
```

### Data Flow Summary

```
┌────────────────────────────────────────────────────────────────────────────────┐
│                              REQUEST LIFECYCLE                                  │
│                                                                                │
│  1. Client sends HTTP request with JWT Bearer token                           │
│     ↓                                                                           │
│  2. chi router matches route → applies middleware                               │
│     ↓                                                                           │
│  3. AuthMiddleware validates JWT → extracts Claims into context                │
│     ↓                                                                           │
│  4. Handler processes request, calls internal packages                         │
│     ↓                                                                           │
│  5. Database queries via Store layer                                           │
│     ↓                                                                           │
│  6. Response built → JSON serialized → HTTP response                           │
│                                                                                │
├────────────────────────────────────────────────────────────────────────────────┤
│                           PLAYBACK DECISION FLOW                                │
│                                                                                │
│  1. Client requests playback for media item + device                          │
│     ↓                                                                           │
│  2. Orchestrator.GetCapabilities(deviceID)                                      │
│     ├── Check probes/cache for cached capabilities                             │
│     ├── Check curated DB for known devices                                     │
│     └── Apply trust scores                                                     │
│     ↓                                                                           │
│  3. Orchestrator.GetServerCapabilities()                                       │
│     └── Returns server's FFmpeg/HW encoding abilities                          │
│     ↓                                                                           │
│  4. Orchestrator.Decide(item, deviceCaps)                                      │
│     ├── Check subtitle compatibility (burn-in vs. external)                    │
│     ├── Check video codec compatibility                                        │
│     ├── Check audio codec compatibility                                        │
│     └── Select encoder from server capabilities                                 │
│     ↓                                                                           │
│  5. Build FFmpegArgs for target codec/container                               │
│     ↓                                                                           │
│  6. Return PlaybackDecision (direct play, remux, or transcode target)         │
│                                                                                │
├────────────────────────────────────────────────────────────────────────────────┤
│                            TRUST SCORING FLOW                                   │
│                                                                                │
│  Sources (highest to lowest trust):                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐  │
│  │ PlaybackFeedback (0.95)  │  Actual playback success/failure             │  │
│  │ CuratedDB (0.90)         │  Pre-verified device profiles                 │  │
│  │ NativeProbe (0.85)       │  Platform-native capability detection          │  │
│  │ CodecProbeFull (0.80)   │  All browser APIs agree                        │  │
│  │ CodecProbePartial (0.50)│  Some browser APIs agree                        │  │
│  │ SingleAPI (0.20)         │  Only one API available                         │  │
│  │ StaticProfile (0.10)     │  Fallback heuristics                           │  │
│  └─────────────────────────────────────────────────────────────────────────┘  │
│                                                                                │
│  Resolution Rules:                                                             │
│  • Highest-trust NEGATIVE overrides lower-trust POSITIVE (conservative)       │
│  • Weighted consensus when multiple sources agree                              │
│  • Trust decays over time without playback feedback                           │
│  • Re-probe triggers after 3+ consecutive failures                           │
└────────────────────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Foundation - Probe & Cache

### Agent Prompt 1.1: Scaffold the Tenkile Go Project

```
Create the Tenkile Go project scaffold in the Tenkile/ folder.

Structure:
  Tenkile/
  ├── cmd/
  │   └── tenkile/
  │       └── main.go              # Entry point
  ├── internal/
  │   ├── probes/                  # Multi-source capability probing & trust scoring
  │   │   ├── resolver.go
  │   │   ├── trust.go
  │   │   ├── validator.go
  │   │   ├── parser.go
  │   │   ├── cache.go
  │   │   ├── curated.go
  │   │   ├── feedback.go
  │   │   └── probes_test.go
  │   ├── server/                  # Server encoding inventory (FFmpeg, HW accel)
  │   │   ├── inventory.go
  │   │   ├── encoder.go
  │   │   ├── benchmark.go
  │   │   └── server_test.go
  │   ├── transcode/               # Quality-preserving transcode decision engine
  │   │   ├── orchestrator.go
  │   │   ├── codec_ladder.go
  │   │   ├── matcher.go
  │   │   ├── subtitle.go
  │   │   ├── legacy.go
  │   │   ├── ffmpeg.go
  │   │   ├── quality.go
  │   │   ├── logger.go
  │   │   └── transcode_test.go
  │   ├── database/                # SQLite/PostgreSQL layer
  │   │   ├── sqlite.go
  │   │   ├── models.go
  │   │   ├── queries.go
  │   │   ├── migrations/
  │   │   └── database_test.go
  │   ├── api/                     # REST API handlers
  │   │   ├── router.go
  │   │   ├── devices.go
  │   │   ├── playback.go
  │   │   ├── admin.go
  │   │   ├── stream.go
  │   │   ├── middleware.go
  │   │   └── api_test.go
  │   ├── config/                  # YAML-based configuration
  │   │   └── config.go
  │   └── clients/                 # Client adapters
  │       ├── adapter.go
  │       ├── web.go
  │       └── detect.go
  ├── pkg/
  │   └── codec/                   # Shared codec definitions (importable by other Go projects)
  │       ├── database.go
  │       └── mime.go
  ├── web/
  │   └── probe/
  │       └── tenkile-probe.js # Client-side probe library
  ├── data/
  │   └── curated/                 # Curated Smart TV database JSON
  ├── configs/
  │   └── tenkile.yaml         # Default config
  ├── go.mod
  ├── go.sum
  └── Makefile

Use Go 1.24+ (latest stable).
Module path: github.com/tenkile/tenkile (or adjust to actual repo).
Reference the architecture doc at ../docs/ARCHITECTURE.md for interface designs.

Go module dependencies (go get these after go mod init):

  Core:
  - github.com/go-chi/chi/v5          # HTTP router + middleware
  - github.com/go-chi/cors             # CORS middleware

  Database:
  - modernc.org/sqlite                 # SQLite (pure Go, no CGo)
  - github.com/jackc/pgx/v5            # PostgreSQL (for multi-instance)
  - github.com/pressly/goose/v3        # Database migrations

  API:
  - github.com/oapi-codegen/oapi-codegen/v2  # OpenAPI -> Go server stubs
  - github.com/gorilla/websocket       # WebSocket support

  Config:
  - gopkg.in/yaml.v3                    # YAML config loading (use yaml struct tags)

  Auth:
  - github.com/golang-jwt/jwt/v5       # JWT tokens
  - golang.org/x/crypto                # Password hashing (bcrypt)

  Testing:
  - github.com/stretchr/testify        # Assertions + mocks

  Logging:
  - log/slog (stdlib)                  # Structured logging (no external dep)

  Embedding:
  - embed (stdlib)                     # Embed web client + curated DB

Makefile targets (see below for full content):
  make build        # Build web + Go binary
  make build-web    # Build web client only (Preact + Vite)
  make build-go     # Build Go binary only
  make dev          # Dev mode: Vite HMR + Go hot reload (air)
  make test         # Run all Go tests
  make lint         # Run golangci-lint
  make generate     # Generate Go server stubs + TS types from OpenAPI spec
  make migrate-up   # Run database migrations
  make migrate-new  # Create a new migration (usage: make migrate-new NAME=create_users)
  make docker       # Build Docker image
  make release      # Cross-compile for all platforms
  make clean        # Remove build artifacts

Makefile content:

  .PHONY: build build-web build-go dev test lint generate migrate-up migrate-new docker release clean

  build: build-web build-go

  build-web:
  	cd web && npm ci && npm run build

  build-go:
  	go build -o tenkile ./cmd/tenkile/

  dev:
  	cd web && npx vite &
  	go run github.com/air-verse/air@latest

  test:
  	go test ./... -race -count=1

  lint:
  	golangci-lint run ./...

  generate:
  	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
  	  -package api -generate types,chi-server \
  	  api/openapi.yaml > internal/api/generated/server.gen.go
  	cd web && npx openapi-typescript ../api/openapi.yaml -o src/api/generated/schema.ts

  migrate-up:
  	go run github.com/pressly/goose/v3/cmd/goose@latest \
  	  -dir internal/database/migrations sqlite3 ./data/tenkile.db up

  migrate-new:
  	go run github.com/pressly/goose/v3/cmd/goose@latest \
  	  -dir internal/database/migrations create $(NAME) sql

  docker:
  	docker build -t tenkile:latest .

  release: build-web
  	GOOS=linux GOARCH=amd64 go build -o dist/tenkile-linux-amd64 ./cmd/tenkile/
  	GOOS=linux GOARCH=arm64 go build -o dist/tenkile-linux-arm64 ./cmd/tenkile/
  	GOOS=darwin GOARCH=arm64 go build -o dist/tenkile-darwin-arm64 ./cmd/tenkile/
  	GOOS=windows GOARCH=amd64 go build -o dist/tenkile-windows-amd64.exe ./cmd/tenkile/

  clean:
  	rm -rf tenkile dist/ web/dist/

Go conventions:
  - Exported = PascalCase, unexported = camelCase
  - Interfaces in the package that uses them (not the implementor)
  - Table-driven tests
  - Error wrapping with fmt.Errorf("context: %w", err)
  - Context as first parameter
  - Structured logging via slog
```

### Agent Prompt 1.2: Define Codec Database and Capability Types

```
Define the codec database and device capability types in Go,
porting the essential data model from CodecProbe with trust scoring added.

Source files to analyze:
- ../codecprobe/js/codec-database-v2.js  (codecSource constant, CONTAINER_MIME, DRM_SYSTEMS,
  STREAM_CONTAINERS, BARE_CONTAINERS, buildMime, buildInfo, buildMediaConfig)
- ../codecprobe/js/codec-tester.js       (testCodecRecord result shape, API_METHODS,
  determineOverallSupport consensus logic at line 386, testScenarioCapabilities at line 158)
- ../codecprobe/js/drm-detection.js      (DRM_SYSTEMS, testKeySystem result shape)
- ../codecprobe/js/device-detection.js   (device info fields)

Reference docs (research material — profiles, levels, MIME types, format details):
- ../docs/CODEC_REFERENCE.md             (video/audio codec profiles, levels, codec strings)
- ../docs/CONTAINER_FORMATS.md           (container formats, MIME types, streaming protocols)
- ../docs/DRM_REFERENCE.md               (DRM systems, security levels, key system identifiers)

Create in pkg/codec/:

1. database.go - Codec/container/scenario definitions
   - All video codecs: HEVC (Main/Main10/Rext), AV1, VP9, AVC, DolbyVision
   - All audio codecs: AAC, Opus, FLAC, DTS, AC-3, E-AC-3, TrueHD, ALAC, Vorbis, MP3
   - Container definitions: MP4, MKV, WebM, MOV, HLS, DASH, CMAF, fMP4, MPEG-TS
   - Use Go constants and typed enums (iota)

2. mime.go - MIME type builders

Create in internal/probes/:

3. types.go - Device capability types with trust scoring
   - DeviceCapabilities with trust scores per codec
   - TrustedCodecSupport with Supported, Trust (0.0-1.0), Sources, Containers,
     Profiles, MaxLevel, HardwareDecode, PowerEfficient, TestedScenarios
   - ScenarioSupport with Width, Height, Framerate, Bitrate, HDRType,
     Supported, Smooth, PowerEfficient, Trust
   - DeviceIdentity with DeviceID, ClientVersion, DeviceModel, Platform enum
   - DevicePlatform enum: WebBrowser, AndroidMobile, AndroidTV, AppleIOS,
     AppleTVOS, TizenTV, WebOSTV, RokuTV, Unknown

4. drm.go - DRM system definitions and security levels

5. trust.go - TrustResolver: merge claims from multiple sources
   - Highest-trust negative overrides lower-trust positives (conservative for safety)
   - Weighted consensus when multiple sources agree
   - Trust levels: PlaybackFeedback(0.95), CuratedDB(0.90), NativeProbe(0.85),
     CodecProbeFullConsensus(0.80), CodecProbePartial(0.50), SingleApi(0.20), StaticProfile(0.10)

Key design decisions:
- Use struct types, not classes (Go doesn't have classes)
- Use typed constants (iota) for enums
- Map CodecProbe's "probably"/"maybe" to trust scores, not booleans
- Preserve per-API source tracking for debugging disagreements
- Export types that other packages need, keep internals unexported
```

### Agent Prompt 1.3: Implement Probe Receiver, Validator, and Cache

```
Implement the capability cache, probe result parsing/validation, and REST API.

The critical insight: device APIs frequently lie or return ambiguous results.
The validator must catch inconsistencies before trusting probe data.

Source context:
- CodecProbe's determineOverallSupport (../codecprobe/js/codec-tester.js line 386):
  counts positive APIs per container, requires all tested APIs to agree for "supported"
- CodecProbe's testScenarioCapabilities (line 158): per-scenario mediaCapabilities.decodingInfo
  with supported/smooth/powerEfficient and 800ms timeout
- Architecture spec: ../docs/ARCHITECTURE.md (ProbeResultValidator, TrustResolver)
- ../docs/DEVICE_DETECTION.md (browser capability APIs, trust scoring, validation rules for lying APIs)

Create:

1. internal/probes/parser.go
   - Parse the JSON that CodecProbe's runCodecTests() produces
   - Map container results: canPlayType + isTypeSupported per container
   - Map scenario results: mediaCapabilities per resolution/framerate
   - Port determineOverallSupport consensus logic to Go

2. internal/probes/validator.go
   Catches lying APIs:
   - If codec "supported" at 4K but "unsupported" at 1080p -> flag 4K as unreliable (trust=0.1)
   - If canPlayType="probably" but mediaCapabilities.supported=false -> trust mediaCapabilities
   - If supported=true but smooth=false AND powerEfficient=false -> likely software decode,
     reduce trust and warn
   - If all 3 APIs disagree -> mark as low confidence (trust=0.2)

3. internal/database/sqlite.go + migrations/
   - Device capability table with JSON-serialized capability columns
   - Decision log table for audit trail
   - Playback feedback table for success/failure tracking
   - Curated device table for Smart TV database
   - Indexes on (device_id, client_version), (device_id, timestamp), model_pattern
   - Use modernc.org/sqlite (pure Go, no CGo)

   Migration conventions (goose):
   - Tool: github.com/pressly/goose/v3
   - Directory: internal/database/migrations/
   - Naming: YYYYMMDDHHMMSS_description.sql (e.g., 20260322120000_create_devices.sql)
   - Each migration has -- +goose Up and -- +goose Down sections
   - Run via: make migrate-up / make migrate-new NAME=description
   - Embed migrations in binary via Go embed directive for zero-file deployment

4. internal/probes/cache.go
   - In-memory hot layer (sync.Map or groupcache) + SQLite persistence
   - Cache key = (deviceID, clientVersion)
   - Re-probe trigger when: version changes, feedback reports failure, TTL expires
   - TTL configurable (default 7 days)

5. internal/api/devices.go + router.go
   - POST /api/v1/devices/{id}/probe - Register probe results
   - GET  /api/v1/devices/{id}/capabilities - Get resolved capabilities with trust
   - POST /api/v1/devices/{id}/feedback - Report playback success/failure
   - DELETE /api/v1/admin/capabilities/{id} - Clear cache
   - Use chi or echo for routing

Write tests in internal/probes/probes_test.go:
- Parser: valid results, partial results, empty results, malformed JSON
- Validator: lying API detection (all scenarios from validator rules above)
- Trust resolution: conflicting sources, unanimous sources, single source
- Cache: TTL expiration, version-change invalidation, concurrent access (goroutine safety)
```

### Agent Prompt 1.4: Build the Client-Side Probe Library

```
Create a JavaScript library that wraps CodecProbe for Tenkile integration.
Runs in the client browser, executes tests, validates locally, and reports to server.

Source context:
- ../codecprobe/js/codec-tester.js   (runCodecTests - main entry)
- ../codecprobe/js/drm-detection.js  (detectDRMSupport)
- ../codecprobe/js/device-detection.js (detectDeviceInfo)
- ../codecprobe/js/codec-database-v2.js (codec definitions)

Reference docs:
- ../docs/DEVICE_DETECTION.md (browser APIs, Smart TV detection, consensus logic)
- ../docs/DRM_REFERENCE.md   (DRM systems, EME stack, security levels)

Create in web/probe/:

1. tenkile-probe.js - Standalone probe library (zero UI dependencies)
   - Import/bundle core CodecProbe logic (codec-tester, drm-detection, device-detection)
   - Remove all UI code (ui-renderer.js, theme-manager.js, url-state.js)
   - Export: TenkileProbe.run(serverUrl, deviceId, options)
   - Sequence:
     a. Detect device platform/model from UA and APIs
     b. Run DRM detection
     c. Run codec tests (batched, with retry: maxRetries=2, timeout=2000ms per test)
     d. Run local validation (flag obvious inconsistencies before sending)
     e. POST results to server's /api/v1/devices/{id}/probe
     f. Return server's trust assessment to caller
   - Progress callback: onProgress(phase, current, total)
   - Configurable: which codec groups to test
   - Handle timeouts and partial failures gracefully

2. Playback feedback integration:
   - TenkileProbe.reportSuccess(serverUrl, deviceId, playbackInfo)
   - TenkileProbe.reportFailure(serverUrl, deviceId, playbackInfo, errorCode)
   - These feed the playback feedback loop on the server

The library should be embeddable in any web client. Keep it lightweight (<50KB minified).
Will be embedded in the Go binary using the embed directive.
```

### Phase 1 Checkpoint: Git Commit

After completing all Phase 1 tasks (1.1–1.4), create a git commit:

```bash
git add -A && git diff --cached --stat  # Review what's staged before committing
git commit -m "Phase 1: Foundation — probe & cache

Scaffold Go project, define codec types, implement probe receiver/validator/cache,
and build client-side probe library.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
git push origin main
```

---

## Phase 2: Server Inventory + Decision Engine

### Agent Prompt 2.1: Implement Server Capability Discovery

```
Build the server-side capability inventory that discovers what the server
can actually encode. This is CRITICAL - every transcode target must be
achievable by the server's hardware/software.

Source context (reference only - study patterns, don't import):
- ../jellyfin/MediaBrowser.MediaEncoding/Encoder/MediaEncoder.cs
  (FFmpeg wrapper, has VAAPI detection, Vulkan DRM interop checks)
- ../docs/ARCHITECTURE.md (ServerCapabilities, HardwareAcceleration, EncoderCapability)
- ../docs/FFMPEG_REFERENCE.md (FFmpeg discovery commands, HW accel probing, encoder names)

Create in internal/server/:

1. inventory.go - Runs at startup
   a. Query FFmpeg version and available encoders/decoders (ffmpeg -encoders, -decoders)
   b. Probe hardware acceleration:
      - NVENC: nvidia-smi + ffmpeg -init_hw_device cuda
      - QSV: ffmpeg -init_hw_device qsv
      - VAAPI: check /dev/dri/renderD128 + vainfo
      - VideoToolbox: check macOS + ffmpeg -init_hw_device videotoolbox
      - AMF: check AMD GPU + ffmpeg -init_hw_device amf
      - V4L2: check /dev/video* for RPi/embedded
   c. For each available encoder, determine:
      - 10-bit support (check pixel formats: p010le, yuv420p10le)
      - HDR metadata passthrough support
      - Supported profiles
      - Real-time capability at 1080p and 4K
   d. Optional quick benchmark (~5 seconds, configurable)

2. encoder.go - Pick the best encoder for a target codec
   Priority: HW > SW, HDR-capable > not, 10-bit > not, faster > slower
   BUT: skip any encoder that can't preserve required quality characteristics

3. benchmark.go - Optional startup benchmark
   Encode small test clip to estimate real-time capability

IMPORTANT: This package is NEW (internal/server/). Do NOT import internal/probes/
types or internal/transcode/ types — the server inventory is independent.
The only shared dependency is pkg/codec/ for codec name constants.

Testability requirements (see Rule 10):
- Define a CommandRunner interface for exec.Command calls
- Use exec.LookPath (via FFmpegPathFinder interface) instead of `which` — `which` is not portable to Windows
- Use os.Stat with os.ModeCharDevice (via deviceExistsFunc) instead of `test -c` — `test -c` is not portable
- Store runtime.GOOS in a struct field with SetGOOS() override for tests
- HW probe functions detect codecs by filtering the already-fetched encoder list — do NOT re-run `ffmpeg -encoders` in each probe method. Pass the list from Discover() through to all probe functions.
- Audio encoders (aac, eac3, ac3, libopus, flac, libmp3lame, truehd, dca, libvorbis, alac) MUST be included in the encoder detail map alongside video encoders — the transcode engine checks server encoding capability for audio codecs too.

Write tests with mock command outputs.
Test: NVIDIA system, Intel QSV system, VAAPI Linux system, macOS VideoToolbox,
software-only system, Raspberry Pi (V4L2), no-FFmpeg error case.

After completing 2.1, run: go build ./... && go test ./internal/server/...
Fix all errors before proceeding to 2.2.
```

### Agent Prompt 2.2: Implement Quality-Preserving Transcode Decision Engine

```
Build the TranscodeOrchestrator that makes playback decisions preserving maximum quality.
This is the core of Tenkile - the clean replacement for Jellyfin's StreamBuilder.

CRITICAL RULES:

HDR handling (device-dependent):
  * Device HAS HDR display: AV1 HDR10 -> HEVC HDR10 (preserve HDR)
  * Device has NO HDR display: AV1 HDR10 -> HEVC SDR with HIGH-QUALITY tone mapping
  * Tone mapping must use proper perceptual mapping (libplacebo > zscale/hable > reinhard)
  * Tone mapping must do BT.2020 -> BT.709 gamut mapping (not just strip metadata!)
  * DolbyVision Profile 7/8 -> HDR10 fallback (extract base layer, no re-encode needed)

SDR/Legacy content:
  * SDR content stays SDR. Never add HDR metadata to SDR sources.
  * SD content (480p/576p): use BT.601 color space, NOT BT.709.
  * Interlaced content: deinterlace when transcoding (bwdif > yadif).
  * Anamorphic DVDs: respect pixel aspect ratio, output correct DAR.
  * Never upscale: 480p stays 480p.

Subtitle handling (decides BEFORE video transcode):
  * Text subs (SRT/VTT): serve externally as VTT. Zero transcode cost.
  * Styled text (ASS/SSA): configurable - burn-in or convert to VTT.
  * Graphical subs (PGS/VOBSUB/DVB): can't be converted to text.
    If client can't render natively -> MUST burn in (forces full video transcode).
  * Subtitle decision FIRST because burn-in forces transcode.

Audio:
  * TrueHD Atmos 7.1 -> E-AC-3 JOC 7.1 (NEVER -> stereo AAC)
  * DTS-HD MA 7.1 -> E-AC-3 7.1 -> E-AC-3 5.1 -> AAC 5.1 -> AAC stereo (ladder, not cliff)
  * Resolution only drops for bandwidth, NEVER because of codec change

Source context:
- internal/probes/ (from Phase 1 - device capabilities with trust scores)
- internal/server/ (from 2.1 - server encoding inventory)
- ../docs/ARCHITECTURE.md (TranscodeOrchestrator, QualityPreservationPolicy, codec ladders)
- ../docs/CODEC_REFERENCE.md (codec profiles, levels, HDR metadata formats)
- ../docs/CONTAINER_FORMATS.md (container compatibility, remux vs transcode decisions)
- ../docs/FFMPEG_REFERENCE.md (FFmpeg command templates, tone mapping filters, bitrate guidelines)

IMPORTANT — Read before writing:
- Read internal/probes/types.go for DeviceCapabilities fields (see Phase 1 Inventory)
- Read internal/server/inventory.go for ServerCapabilities from 2.1
- Use exact field names from those structs — do NOT guess
- The transcode package should import internal/probes and internal/server, plus pkg/codec
- Do NOT create any types that duplicate what's in internal/probes/types.go

Create in internal/transcode/:

1. quality.go - QualityPreservationPolicy
   - HdrPolicy: BestForDevice (default) | AlwaysToneMap | NeverToneMap
   - ToneMappingQuality: High | Medium | Fast
   - AudioChannelPolicy: PreserveChannels | AllowDownmix
   - BitDepthPolicy: PreserveWhenPossible | Allow8Bit
   - ResolutionPolicy: MaintainOriginal | AllowBandwidthReduce

2. codec_ladder.go - Quality-ordered fallback chains
   Video: AV1 -> HEVC -> VP9 -> H.264 (preserve resolution, framerate)
   Audio: TrueHD -> E-AC-3 JOC -> E-AC-3 7.1 -> E-AC-3 5.1 -> AAC 5.1 -> AAC stereo

3. matcher.go - Match media against device capabilities
   - Scenario interpolation: device passed HEVC at 4K@60fps -> can play 1080p@24fps
   - Trust threshold: only direct-play if trust >= MinTrustForDirectPlay (default 0.6)

4. orchestrator.go - Main decision flow:
   a. Get trusted device capabilities (from cache/probe)
   b. Get server encoding capabilities (from inventory)
   c. Resolve subtitle decision FIRST (burn-in forces video transcode)
   d. Try direct play: codec + container + bitrate + HDR all compatible?
      - HDR content on non-HDR device CANNOT direct play (needs tone mapping)
      - HdrAlwaysToneMap policy also prevents direct play of HDR content
   e. Try remux: codec compatible but container isn't?
   f. Walk video codec ladder: find first codec device supports AND server can encode
   g. Walk audio codec ladder: same logic, preserve channels
   h. Partial transcode: if video is direct-playable but audio isn't, copy video and only transcode audio
   i. Absolute fallback: H.264/AAC/MP4
   j. IMPORTANT: After selecting an encoder, verify it actually supports HDR passthrough
      if HDR was requested. SelectEncoder may fall back to a non-HDR encoder silently.
      If the selected encoder can't do HDR, switch to tone mapping.
   k. Do NOT store an EncoderSelector on the Orchestrator struct — create a fresh one in
      each Decide() call from inv.GetCurrent() so it picks up capability refreshes.
   l. Log the full decision

5. subtitle.go - Subtitle decision tree (see architecture doc)
   - Include a streamIndex field (unexported) on SubtitleDecision for FFmpeg filter construction
   - Set it from MediaItem.SubtitleIndex in the orchestrator

6. legacy.go - Interlaced, anamorphic, SD content handling
   - Use epsilon comparison (math.Abs(par - 1.0) > 0.001) for PixelAspectRatio, NOT exact float equality

7. ffmpeg.go - Build FFmpeg args from TranscodeTarget
   TWO HDR paths:
   a. HDR passthrough (device has HDR display): preserve metadata, 10-bit (p010le),
      BT.2020/smpte2084 color metadata
   b. HDR -> SDR tone mapping: libplacebo/zscale/reinhard, BT.2020->BT.709, 8-bit output (yuv420p),
      BT.709 color metadata
   Subtitle burn-in: when SubtitleAction is BurnIn, add the actual `subtitles` video filter
   with the stream index — do not just set `-c:s none` without compositing the subtitle.

8. logger.go - PlaybackDecisionLog model and persistence

Write EXTENSIVE tests in internal/transcode/transcode_test.go:

Quality handling matrix (the most important tests):

  HDR device (has HDR display):
  | Source                     | Device Supports       | Expected Target               |
  |---------------------------|-----------------------|-------------------------------|
  | AV1 HDR10 / Opus / MKV   | HEVC+HDR, not AV1    | HEVC HDR10 / Opus / MP4       |
  | HEVC DV P7 / TrueHD 7.1  | HEVC+HDR, not DV     | HEVC HDR10 base / E-AC-3 7.1  |
  | HEVC DV P8 / E-AC-3      | HEVC + DV P8         | DirectPlay                    |
  | HEVC HDR10 / DTS-HD 7.1  | HEVC+HDR, AAC only   | DirectPlay video / AAC 5.1    |

  SDR device (no HDR display):
  | Source                     | Device Supports       | Expected Target               |
  |---------------------------|-----------------------|-------------------------------|
  | AV1 HDR10 / Opus / MKV   | HEVC, no HDR display | HEVC SDR (tonemap) / Opus     |
  | HEVC HDR10 / E-AC-3      | H.264 only, no HDR   | H.264 SDR (tonemap) / AAC 5.1|
  | HEVC DV P8 / E-AC-3      | HEVC, no HDR display | HEVC SDR (tonemap) / E-AC-3   |

  Tone mapping quality tests:
  - Verify libplacebo filter used when ToneMappingQuality=High
  - Verify output has BT.709 primaries (not BT.2020)
  - Verify 8-bit output (yuv420p) for SDR, 10-bit (p010le) for HDR passthrough

  Server constraint tests:
  | Source       | Device (HDR?)      | Server Has          | Expected                    |
  |-------------|-------------------|---------------------|-----------------------------|
  | AV1 4K      | HEVC (SDR device) | hevc_nvenc (4K OK)  | HEVC SDR (HW encode+tonemap)|
  | AV1 4K      | HEVC (SDR device) | libx265 only        | HEVC SDR SW (if real-time)  |
  | AV1 4K      | HEVC (SDR device) | No HEVC encoder     | H.264 SDR fallback          |
  | AV1 4K HDR  | HEVC (HDR device) | hevc_nvenc (HDR OK) | HEVC HDR10 HW passthrough   |

Use table-driven tests (Go convention).

After completing 2.2, run: go build ./... && go test ./internal/transcode/...
Fix all errors before proceeding to 2.3.
```

### Agent Prompt 2.3: Implement Decision Logger

```
Build the full decision audit trail system.

Every playback decision must be logged with:
- What was decided (direct play / remux / transcode / fallback)
- Why (which capability check failed, which trust score was too low)
- Quality impact (what was preserved, what was lost)
- Server-side details (which encoder, HW or SW)
- Outcome (updated later via playback feedback)

Create in internal/transcode/:

1. logger.go
   - Log(ctx, PlaybackDecisionLog) on every decision
   - Persist to database
   - Include timing (how long the decision took)
   - Use slog for structured logging to stdout as well

2. PlaybackDecisionLog struct (see ARCHITECTURE.md for full schema)
   - DecisionType, source/target codecs and containers
   - HDRPreserved, ToneMapped, BitDepthPreserved, AudioChannelsPreserved
   - EncoderUsed, HardwareAccelUsed
   - DeviceCapabilityTrust, CapabilitySources
   - PlaybackSucceeded (updated later), FailureReason

3. Admin query endpoints in internal/api/admin.go:
   IMPORTANT: admin.go ALREADY EXISTS from Phase 1. Add new handler methods
   to the existing AdminHandlers struct — do NOT create a new struct or file.
   Use the existing RespondJSON() helper from response.go.

   GET /api/v1/admin/decisions?deviceId=X&from=date&to=date&limit=N&offset=N
   GET /api/v1/admin/decisions/stats (aggregate: direct play %, transcode %,
       HDR preservation %, avg trust, failure rate)

   IMPORTANT: Parse ALL documented query params (limit, offset, deviceId, from, to).
   Validate and cap limit (max 1000) to prevent clients from dumping the entire log.
   Parse from/to as both RFC3339 and date-only (YYYY-MM-DD) formats.

   Register new routes in router.go inside the existing admin Route group.

After completing 2.3, run: go build ./... && go test ./...
Fix all errors before committing.
```

### Lessons from Phase 2 Implementation

These issues were discovered during Phase 2 development and code review. Future phases should avoid these patterns:

1. **Audio encoders matter.** The server inventory initially only mapped video encoder names to codecs. The transcode engine's `CanEncodeCodec("eac3")` returned false because eac3 wasn't in the encoder details. Audio encoders (aac, eac3, ac3, libopus, flac, libmp3lame, truehd, dca, libvorbis, alac) must be in the encoder map.

2. **runtime.GOOS breaks tests.** HW probing dispatches on OS — NVIDIA/QSV/VAAPI only probe on Linux, VideoToolbox only on Darwin. Tests running on macOS never exercised Linux-only paths. Solution: store `goos` as a struct field, set from `runtime.GOOS`, with `SetGOOS()` for tests.

3. **FFmpeg output parsing is tricky.** The `ffmpeg -encoders` output has header lines like `V..... = Video` where `=` matches the regex for codec names. Filter these with `if name == "=" { continue }`.

4. **The orchestrator must handle partial transcodes.** When video is direct-playable but audio isn't (e.g., HEVC HDR10 + DTS-HD on a device that supports HEVC+HDR but only AAC audio), copy video and transcode only audio. This is a common real-world scenario.

5. **HDR encoder verification is essential.** `SelectEncoder()` does graceful degradation — if no HDR-capable encoder exists, it returns the best non-HDR encoder. The orchestrator MUST check `encoder.SupportsHDRPassthrough` after selection and fall back to tone mapping if the encoder can't actually do HDR. Otherwise the decision claims "HDR preserved" but the encoder drops the metadata.

6. **Subtitle burn-in needs the actual filter.** It's not enough to set `-c:s none` — you must also add the `subtitles=si=N` video filter to actually composite the subtitle onto the video stream.

### Phase 2 Checkpoint: Git Commit

After completing all Phase 2 tasks (2.1–2.3), create a git commit:

```bash
git add -A && git diff --cached --stat  # Review what's staged before committing
git commit -m "Phase 2: Server inventory + decision engine

Implement server capability discovery, quality-preserving transcode decision
engine, and decision audit logger.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
git push origin main
```

---

## Phase 3: Curated Device DB + Playback Feedback Loop

### Agent Prompt 3.1: Build the Curated Smart TV Database

```
Smart TVs (Samsung Tizen, LG WebOS, Roku) have FIXED capabilities per model/firmware.
Their web engines often have broken or incomplete API implementations, making JS probing
unreliable. A curated database is more trustworthy (trust=0.90) than runtime probing.

Reference: ../docs/DEVICE_DETECTION.md (Smart TV detection patterns, platform UA strings,
trust scoring, validation rules for catching lying device APIs)
Reference: ../docs/DEVICE_DATABASE.md (vendor taxonomy, device class overview, codec capability
matrices, community contribution guide, SoC-based capability inference)

IMPORTANT: internal/probes/curated.go ALREADY EXISTS from Phase 1 with:
- CuratedDevice struct, CuratedDatabase struct, KnownIssue struct
- NewCuratedDatabase(), Load(), AddDevice(), UpdateDevice(), RemoveDevice()
- GetByID(), GetByDeviceHash(), GetByPlatform(), Search(), GetAll()
- Vote(), MarkVerified(), GetRecommendedProfile()
- See Phase 1 Inventory for full method signatures.

Extend the existing code — do NOT rewrite or redeclare these types.

Tasks for Phase 3.1:

1. Extend curated.go:
   - Add LoadFromEmbedded(data []byte) for Go embed loading
   - Add fuzzy matching: "Samsung UN55TU8000" matches "UN*TU8*" pattern
   - Add version-aware matching: different firmware = different codec support
   - Add export/import format for community sharing

2. data/curated/ - JSON seed data (already populated, extend as needed):
   - samsung_tizen.json (2016-2024 model families, codec support per SoC — verified via RTINGS)
   - lg_webos.json (2018-2024)
   - roku.json (per-model codec matrix)
   - android_tv.json (Chromecast, Shield, Mi Box, etc.)
   - philips_android_tv.json (2020-2024, Android TV + Saphi OS)
   - xiaomi_mi_tv.json (Mi TV, Redmi TV, Mi TV Stick 4K)
   - hisense_smart_tv.json (Android TV + Vidaa OS per-model)
   - amazon_fire_tv.json (Fire TV Cube, Stick, Smart TV — all generations)
   - apple_tvos.json (Apple TV HD through 4K 3rd Gen)
   - sony_android_tv.json (Bravia series 2016-2024)
   - tablets_smartphones.json (iPhone, iPad, Android phones/tablets)
   - generic_tv_boxes.json (Mecool, Zidoo, H96, Tanix, TiVo Stream 4K)

   See ../docs/DEVICE_DATABASE.md for the full vendor taxonomy, codec capability matrices,
   coverage gaps, and community contribution guide. All 12 files total 97 devices across
   12 platforms. Verify new entries against RTINGS.com or official manufacturer specs before adding.

3. Admin API in internal/api/admin.go (ALREADY EXISTS — extend):
   admin.go already has AdminHandlers with curated device management stubs.
   router.go already registers routes at /api/v1/admin/curated-devices.
   Add PUT handler for updates. Use existing RespondJSON() and ErrorResponse.

4. Community contribution workflow:
   - Export/import format for sharing device profiles
   - Merge with conflict resolution

After completing 3.1, run: go build ./... && go test ./internal/probes/...
Fix all errors before proceeding to 3.2.
```

### Agent Prompt 3.2: Implement Playback Feedback Loop

```
The HIGHEST trust source (0.95): did the content actually play successfully?

When a client reports playback success or failure, the system retroactively adjusts
trust scores for all capability claims involved in that decision.

Create in internal/probes/:

1. feedback.go (NEW file in existing internal/probes/ package)
   - Use existing types from types.go (DeviceCapabilities, etc.) — do NOT redeclare
   - Use existing TrustLevel constants from trust.go
   - RecordResult(ctx, PlaybackFeedback) error
   - On success: boost trust to 0.95
   - On failure: reduce trust to 0.05, flag device for re-probe
   - Cross-reference with the original PlaybackDecisionLog from internal/transcode/

2. FeedbackEntry model:
   - DeviceID, MediaItemID, VideoCodec, AudioCodec, Container
   - Resolution (width x height), Framerate, HDRType
   - Success (bool), ErrorCode, ErrorMessage
   - OriginalDecisionID (FK to decision log)
   - Timestamp

3. Client-side integration (update tenkile-probe.js from Phase 1):
   - Hook into HTML5 video events: 'playing' (success), 'error' (failure)
   - Report automatically after N seconds of smooth playback
   - Report immediately on error with error code

4. Re-probe trigger logic:
   - After playback failure: mark device for re-probe on next connection
   - If 3+ failures for same codec: reduce trust to 0 and stop recommending
   - If curated DB says supported but feedback says failed: log discrepancy

After completing 3.2, run: go build ./... && go test ./...
Fix all errors before committing.
```

### Phase 3 Checkpoint: Git Commit

After completing all Phase 3 tasks (3.1–3.2), create a git commit:

```bash
git add -A && git diff --cached --stat  # Review what's staged before committing
git commit -m "Phase 3: Curated device DB + playback feedback loop

Build curated Smart TV database and implement playback feedback loop for
trust score refinement.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
git push origin main
```

---

## Phase 3 Implementation Drift (Post-Implementation)

Document what was actually delivered vs. what was specified, and why.

| Specified | Actually Delivered | Reason |
|-----------|-------------------|--------|
| 12 platform files in embedded loader | 4 platform files | Limited Phase 3 scope; remaining 8 available in data/curated/ |
| feedback.go only | + feedback_metrics.go, feedback_integration.go | Needed for Prometheus metrics export and trust system integration |
| Simple circular buffer | Advanced circular buffer with getAll() method | Better API for event history access |
| Default TrustAdjustmentConfig values | Full config with MinTrust, MaxTrust, AdjustmentPeriod | More configurable |
| Admin API CRUD endpoints | Full CRUD + search + export/import + fuzzy matching | Enhanced functionality |

### IMPLICIT FILES (Generated but not in spec)

Files that were created during implementation but weren't explicitly in the task specification:

| File | Why Needed | Status |
|------|------------|--------|
| internal/probes/feedback_metrics.go | Prometheus metrics export for observability | Created |
| internal/probes/feedback_integration.go | Trust system integration between FeedbackManager and TrustResolver | Created |
| docs/CONTRIBUTING.md | Community contribution workflow documentation | Created |
| internal/probes/embedded/*.json | Embedded device profiles (also in data/curated/) | Created |

### DEPENDENCIES DISCOVERED DURING IMPLEMENTATION

External packages added during Phase 3 that weren't in the original go.mod:

| Package | Purpose | Added By |
|--------|---------|----------|
| github.com/sahilm/fuzzy | Fuzzy string matching for device name matching | Phase 3.1 |

---

## Lessons Learned (Cumulative)

### From Phase 1
- **Rule:** Never create new type without checking existing packages (duplicate type failures)
- **Rule:** Verify all imports exist before claiming build passes
- **Rule:** Every `{` must have a matching `}` — always verify file integrity after writes
- **Rule:** Run `go build ./...` and `go test ./...` before considering work done

### From Phase 2
- **Rule:** Audio encoders matter as much as video encoders — always include full codec list
- **Rule:** FFmpeg output parsing needs careful line filtering (header lines match codec patterns)
- **Rule:** Abstract system calls behind interfaces (enables testing with mocks)
- **Rule:** Store `runtime.GOOS` in a struct field with `SetGOOS()` override for tests

### From Phase 3
- **Rule:** Every configurable numeric value must be explicitly specified with exact numbers, not vague terms like "significant"
- **Rule:** Undiscovered implicit files are inevitable — track them in an Implementation Drift section
- **Rule:** Race conditions in goroutines need explicit mention — capture values before defer runs
- **Rule:** Embedded data loader must verify it loads ALL files, not just a subset
- **Rule:** Global stats vs. codec-specific stats are separate concerns — both need proper updates
- **Rule:** `context.Background()` is a no-op in select statements — use `context.WithTimeout()` for delays
- **Rule:** Bubble sort (O(n²)) should be replaced with `sort.Float64s()` (O(n log n))
- **Rule:** Array.shift() causes memory pressure — use circular buffer for event history
- **Rule:** Trust decay must actually assign the computed value back — not just compute and log
- **Rule:** Always reference DEVICE_DATABASE.md when creating curated device profiles
- **Rule:** Verify year-based capability differences (Samsung 2022+ AV1, etc.)
- **Rule:** Verify vendor-specific rules (Samsung no DTS/DV, LG always DV)
- **Rule:** Check container/subtitle support matrices when adding platforms
- **Rule:** Create SoC inference for new platforms using DEVICE_DATABASE.md Section 6

### For Future Phases
- Before starting implementation, verify the SPEC COMPLETENESS CHECKLIST below
- Track Implementation Drift after each phase
- Always capture values before spawning goroutines when mutex is involved
- Add nil checks after any function that returns pointer types
- Use `sort.Float64s()` and other stdlib sort functions instead of bubble sort

---

## Phase Spec Completeness Checklist

**Before starting implementation, the agent MUST verify every item in this checklist. If any item is missing, the spec is INCOMPLETE — do not proceed to implementation.**

### 1. Every new file listed with purpose
- [ ] File name with path (e.g., `internal/probes/feedback.go`)
- [ ] Purpose description (1-2 sentences)
- [ ] Package it belongs to

### 2. Every type with complete field list
- [ ] Struct name and package
- [ ] All fields with types
- [ ] Field descriptions for non-obvious fields
- [ ] Example JSON representation

### 3. Every method with signature and behavior description
- [ ] Method name and receiver type
- [ ] Parameters with types
- [ ] Return values with types
- [ ] Side effects described
- [ ] Error conditions documented

### 4. Every API endpoint with HTTP method, path, request/response shapes
- [ ] HTTP method (GET, POST, PUT, DELETE, etc.)
- [ ] Full path including path parameters (e.g., `/api/v1/devices/{id}/feedback`)
- [ ] Request body schema (or "no body" if GET/DELETE)
- [ ] Response body schema
- [ ] HTTP status codes returned
- [ ] Error response shapes

### 5. Every configurable value with type and default
- [ ] Field name and type
- [ ] Default value
- [ ] Valid range (if numeric)
- [ ] Environment variable name (if env-overridable)

### 6. Every external dependency with package name and version
- [ ] Package import path
- [ ] Purpose in the implementation
- [ ] `go get` command (e.g., `go get github.com/sahilm/fuzzy`)

### 7. Every integration point with interface definition
- [ ] Interface name
- [ ] Methods with signatures
- [ ] Real implementation name
- [ ] Mock implementation for tests

### 8. Every acceptance test described
- [ ] Test case name
- [ ] Input
- [ ] Expected output
- [ ] Edge cases covered

### 9. Every blocking dependency identified
- [ ] Phase/file that must complete first
- [ ] Specific types/functions needed from dependency

### 10. Every ambiguous term defined
- [ ] Term
- [ ] Precise definition used in this project
- [ ] What it does NOT mean

### 11. Referenced specifications verified
- [ ] Referenced specs (like DEVICE_DATABASE.md) have been read and incorporated
- [ ] Year-based capability differences are modeled
- [ ] Vendor-specific behavior rules are captured
- [ ] Container format support is specified
- [ ] Subtitle format support is specified
- [ ] SoC-based inference table is created for new platforms

### Implementation Drift Tracking (Post-Implementation)
After each phase, document:
- What was specified vs. what was delivered
- Why there were differences
- Implicit files created that weren't in the spec
- New dependencies discovered during implementation

---

---

# Phase 4: Media Library + Streaming

> **Status:** ✅ COMPLETE (7/7 sub-tasks complete)

This phase adds the media library management, adaptive streaming, authentication, and web client.

## Sub-Task Status

| Task | Status | Priority | Dependencies |
|------|--------|----------|--------------|
| **4.1 Media Scanner** | ✅ COMPLETE | — | needs 1.1 ✓ |
| **4.2 HLS/DASH Streaming** | ✅ COMPLETE | HIGH | needs 2.2, 4.1 ✓ |
| **4.3 OpenAPI Spec** | ✅ COMPLETE | — | needs 1.3, 2.3, 3.1, 4.1, 4.2 ✓ |
| **4.4 Auth + First-Run** | ✅ COMPLETE | — | needs 4.3 ✓ |
| **4.5 Web Client** | ✅ COMPLETE | HIGH | needs 4.3, 4.4 ✓ |
| **4.6 WebSocket Events** | ✅ COMPLETE | MEDIUM | needs 4.4 ✓ |
| **4.7 Docker + Deploy** | ✅ COMPLETE | LOW | needs 4.5 ✓ |

## 4.1 Media Scanner ✅ COMPLETE

### Files Created
```
internal/media/
├── models.go      # Library, MediaItem, VideoStream, AudioStream, SubtitleStream, LibraryScanStatus
├── probe.go       # FFprobe metadata extraction with CommandRunner interface
├── scanner.go     # Library scanning with progress tracking
└── store.go       # SQLite persistence for libraries and media items
```

### Key Types
```go
// Library represents a configured media library
type Library struct {
    ID                     string      `json:"id"`
    Name                   string      `json:"name"`
    Path                   string      `json:"path"`
    LibraryType            LibraryType `json:"library_type"` // "movie" | "tv" | "music"
    Enabled                bool        `json:"enabled"`
    RefreshIntervalMinutes int         `json:"refresh_interval_minutes"`
    CreatedAt              time.Time   `json:"created_at"`
    UpdatedAt              time.Time   `json:"updated_at"`
    LastScanAt             *time.Time  `json:"last_scan_at,omitempty"`
}

// MediaItem represents an indexed media file
type MediaItem struct {
    ID               string            `json:"id"`
    LibraryID        string            `json:"library_id"`
    Path             string            `json:"path"`
    Title            string            `json:"title"`
    Year             int               `json:"year,omitempty"`
    Container        string            `json:"container"`
    Duration         float64           `json:"duration"`
    VideoStream      *VideoStream      `json:"video_stream,omitempty"`
    AudioStreams     []AudioStream     `json:"audio_streams"`
    SubtitleStreams  []SubtitleStream  `json:"subtitle_streams"`
    FileSize         int64             `json:"file_size"`
    FileModifiedAt   time.Time         `json:"file_modified_at"`
}

// VideoStream represents video track information
type VideoStream struct {
    Index        int     `json:"index"`
    Codec        string  `json:"codec"`
    Profile      string  `json:"profile,omitempty"`
    Level        string  `json:"level,omitempty"`
    Width        int     `json:"width"`
    Height       int     `json:"height"`
    Framerate    float64 `json:"framerate"`
    BitDepth     int     `json:"bit_depth"`
    HDRType      string  `json:"hdr_type,omitempty"` // "hdr10", "hdr10+", "dolby_vision", "hlg"
    IsInterlaced bool    `json:"is_interlaced"`
    Bitrate      int64   `json:"bitrate"`
}
```

### Key Functions
```go
// Scanner walks library paths and indexes media files
func NewScanner(ffprobe *FFprobe, store *Store) *Scanner
func (s *Scanner) ScanLibrary(ctx context.Context, lib *Library) error
func (s *Scanner) GetStatus(libraryID string) *LibraryScanStatus

// FFprobe extracts metadata from media files
func NewFFprobe(path string, runner CommandRunner) *FFprobe
func (f *FFprobe) Probe(ctx context.Context, path string) (*MediaItem, error)

// Store handles SQLite persistence
func NewStore(db *sql.DB) *Store
func (s *Store) GetAllLibraries(ctx context.Context) ([]*Library, error)
func (s *Store) GetLibrary(ctx context.Context, id string) (*Library, error)
func (s *Store) SaveLibrary(ctx context.Context, lib *Library) error
func (s *Store) GetMediaItems(ctx context.Context, libraryID string, offset, limit int) ([]*MediaItem, int, error)
func (s *Store) GetMediaItem(ctx context.Context, id string) (*MediaItem, error)
```

### Database Tables
- `libraries` - Library configuration
- `media_items` - Indexed media with JSONB stream metadata
- `series` / `episodes` - TV show organization

---

## 4.2 HLS/DASH Streaming ✅ COMPLETE

### Files Created
```
internal/stream/
├── models.go      # Variant, HLSOptions, DASHOptions, HLSManifest, StreamSession
├── segmenter.go   # HLS segment generation (needs FFmpeg integration)
└── handler.go     # HTTP handlers for streaming
```

### Key Types
```go
// Variant represents a quality tier for adaptive streaming
type Variant struct {
    Name          string `json:"name"` // "4k", "1080p", "720p", etc.
    Width         int    `json:"width"`
    Height        int    `json:"height"`
    Bitrate       int64  `json:"bitrate"`       // bits/sec
    AudioBitrate  int64  `json:"audio_bitrate"`
}

// HLSOptions configures HLS generation
type HLSOptions struct {
    SegmentDuration int    `json:"segment_duration"` // seconds (default: 6)
    PlaylistSize    int    `json:"playlist_size"`   // 0 = infinite
    TempDir         string `json:"temp_dir"`
    IncludeAudio    bool   `json:"include_audio"`
}

// StreamSession tracks active streaming sessions
type StreamSession struct {
    ID           string     `json:"id"`
    MediaItemID  string     `json:"media_item_id"`
    UserID       string     `json:"user_id"`
    StreamType   StreamType `json:"stream_type"` // "hls", "dash", "direct"
    StartTime    time.Time  `json:"start_time"`
    LastAccess   time.Time  `json:"last_access"`
    BytesServed  int64      `json:"bytes_served"`
}
```

### Key Functions (Need Implementation)
```go
// Segmenter handles HLS/DASH segment generation
type Segmenter struct {
    ffmpegPath  string
    ffprobePath string
    runner      CommandRunner  // Rule 10: abstracted for testing
    tempDir     string
}

func NewSegmenter(ffmpegPath, ffprobePath string, inv *server.Inventory) *Segmenter
func (s *Segmenter) GenerateHLS(ctx context.Context, inputPath string, variants []Variant, opts HLSOptions) (*HLSManifest, error)
func (s *Segmenter) Cleanup(manifest *HLSManifest) error

// Handler handles streaming HTTP requests
func NewHandler(mediaStore *media.Store) *Handler
func (h *Handler) ServeHLS(w http.ResponseWriter, r *http.Request)
func (h *Handler) ServeHLSManifest(w http.ResponseWriter, r *http.Request)
func (h *Handler) ServeHLSSegment(w http.ResponseWriter, r *http.Request)
```

### Integration Points
- Uses `server.Inventory` for HW acceleration detection
- Uses `media.Store` for media item lookup
- Uses `transcode.Orchestrator` for playback decision (optional)

---

## 4.3 OpenAPI Spec ✅ COMPLETE

### File
```
api/openapi.yaml
```

The OpenAPI spec defines all endpoints for the media server API v1.

### Key Endpoints Defined
```yaml
paths:
  /auth/login:          POST   # Login with credentials
  /auth/first-run:     POST   # Initial admin setup
  /auth/refresh:       POST   # Refresh access token
  /auth/logout:        POST   # Logout
  /auth/me:            GET    # Current user
  
  /libraries:          GET, POST
  /libraries/{id}:     GET, PUT, DELETE
  /libraries/{id}/scan:           POST   # Trigger scan
  /libraries/{id}/scan/status:     GET    # Scan progress
  /libraries/{libraryId}/items:    GET    # Paginated items
  
  /media/{id}:         GET
  /media/{id}/stream:  GET    # Stream info + variants
  /media/{id}/play:    GET    # Playback manifest
```

### Schema Definitions
- `Library`, `LibraryCreate`
- `MediaItem`, `VideoStream`, `AudioStream`, `SubtitleStream`
- `StreamInfo`, `Variant`, `PlaybackManifest`
- `LoginRequest`, `LoginResponse`, `RefreshResponse`
- `User`, `FirstRunRequest`

---

## 4.4 Auth + First-Run ✅ COMPLETE

### Files Created
```
internal/api/
├── auth.go       # AuthHandler with JWT login/logout/refresh
├── middleware.go # AuthMiddleware, AdminMiddleware
├── library.go    # Library CRUD (uses auth)
└── media.go      # Media item handlers (uses auth)
```

### Key Types
```go
// User in context
type User struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Role     string `json:"role"` // "admin" | "user"
}

// JWT Claims
type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}
```

### Key Functions
```go
// AuthHandler manages authentication
func NewAuthHandler(db *sql.DB, jwtSecret string) *AuthHandler
func (h *AuthHandler) Login(w ResponseWriter, r *http.Request)
func (h *AuthHandler) Refresh(w ResponseWriter, r *http.Request)
func (h *AuthHandler) Logout(w ResponseWriter, r *http.Request)
func (h *AuthHandler) FirstRun(w ResponseWriter, r *http.Request)
func (h *AuthHandler) GetCurrentUser(w ResponseWriter, r *http.Request)

// Middleware
func AuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler
func AdminMiddleware() func(http.Handler) http.Handler
func OptionalAuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler
func GetUserFromContext(r *http.Request) *User
```

### Auth Flow
1. **First-run**: `POST /auth/first-run` creates initial admin user
2. **Login**: `POST /auth/login` returns access_token + refresh_token
3. **Authenticated requests**: `Authorization: Bearer <token>` header
4. **Refresh**: `POST /auth/refresh` with refresh_token

### Database Tables (in init.sql)
- `users` - id, username, password_hash (bcrypt), role, timestamps
- `refresh_tokens` - user_id, token_hash, created_at, expires_at

### Configuration
```yaml
auth:
  jwt_secret: ""  # Auto-generated if empty
  token_expiry: "24h"
```

---

## 4.5 Web Client ✅ COMPLETE

### Required Files
```
web/
├── package.json        # Preact + Vite dependencies
├── vite.config.js      # Vite configuration with API proxy
├── index.html          # Main HTML entry
├── src/
│   ├── main.jsx        # App entry point
│   ├── components/
│   │   ├── Header.jsx
│   │   ├── Login.jsx
│   │   ├── Dashboard.jsx
│   │   ├── Library.jsx
│   │   └── MediaPlayer.jsx
│   ├── hooks/
│   │   ├── useAuth.js
│   │   └── useApi.js
│   └── styles/
│       └── app.css
└── dist/               # Built output (gitignored)
```

### Dependencies
```json
{
  "preact": "^10.19.0",
  "preact-router": "^4.1.2",
  "@preact/signals": "^1.2.0",
  "hls.js": "^1.4.0"
}
```

### Key Components

**1. App Shell (main.jsx)**
```jsx
import Router from 'preact-router';
import { Signals } from '@preact/signals';

export const auth = { token: null, user: null };

function App() {
  return (
    <Router>
      <Login path="/" />
      <Dashboard path="/dashboard" />
      <Library path="/library/:id" />
      <MediaPlayer path="/play/:id" />
    </Router>
  );
}
```

**2. API Hook (useApi.js)**
```javascript
export function useApi() {
  const token = localStorage.getItem('access_token');
  
  const request = async (path, options = {}) => {
    const res = await fetch(`/api/v1${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(token && { Authorization: `Bearer ${token}` }),
        ...options.headers,
      },
    });
    if (res.status === 401) {
      // Handle token expiry
      auth.token = null;
      window.location.href = '/';
    }
    return res.json();
  };
  
  return {
    get: (path) => request(path),
    post: (path, body) => request(path, { method: 'POST', body: JSON.stringify(body) }),
  };
}
```

**3. Media Player (MediaPlayer.jsx)**
- Uses `hls.js` for HLS playback
- Quality selector (variant selection)
- Subtitle track selector
- Progress tracking

### Implementation Steps
1. Create `package.json` with dependencies
2. Create `vite.config.js` with API proxy to backend
3. Create `index.html` with base styles
4. Create `src/main.jsx` with routing
5. Implement `Login` component
6. Implement `Dashboard` with library list
7. Implement `Library` with media grid
8. Implement `MediaPlayer` with HLS.js
9. Add responsive design

### Integration with Backend
```javascript
// Fetch HLS manifest
const { manifest } = await api.get(`/media/${id}/play?variant=1080p`);

// Play with HLS.js
import Hls from 'hls.js';
const hls = new Hls();
hls.loadSource(manifest);
hls.attachMedia(videoElement);
```

---

## 4.6 WebSocket Events ✅ COMPLETE

### Required Files
```
internal/events/
├── bus.go          # Event bus implementation
├── types.go        # Event types
├── websocket.go    # WebSocket handler
└── client.go       # Client-side event handling
```

### Event Types
```go
// Event types
const (
    EventLibraryScanStarted   = "library:scan:started"
    EventLibraryScanProgress  = "library:scan:progress"
    EventLibraryScanComplete = "library:scan:complete"
    EventLibraryScanError     = "library:scan:error"
    
    EventStreamStarted   = "stream:started"
    EventStreamProgress = "stream:progress"
    EventStreamEnded    = "stream:ended"
    
    EventTranscodeStarted = "transcode:started"
    EventTranscodeProgress = "transcode:progress"
    EventTranscodeComplete = "transcode:complete"
    EventTranscodeError    = "transcode:error"
)

// Event payload
type Event struct {
    Type      string      `json:"type"`
    Timestamp time.Time   `json:"timestamp"`
    Data      interface{} `json:"data"`
}
```

### Event Bus Interface
```go
type Bus interface {
    Subscribe(eventType string, handler func(*Event))
    Unsubscribe(eventType string, handler func(*Event))
    Publish(event *Event)
}

// PubSub implements in-memory event bus
type PubSub struct {
    handlers map[string][]func(*Event)
    mu       sync.RWMutex
}

func NewBus() *Bus
func (b *PubSub) Subscribe(eventType string, handler func(*Event))
func (b *PubSub) Unsubscribe(eventType string, handler func(*Event))
func (b *PubSub) Publish(event *Event)
```

### WebSocket Handler
```go
type WebSocketHandler struct {
    bus     *Bus
    clients map[*Client]bool
}

func (h *WebSocketHandler) HandleConnect(w http.ResponseWriter, r *http.Request) {
    // Upgrade to WebSocket
    // Authenticate user from token
    // Register client
}

func (h *WebSocketHandler) HandleMessage(client *Client, message []byte) {
    // Handle subscription messages
    // {"action": "subscribe", "event": "library:scan:*"}
}

func (h *WebSocketHandler) HandleDisconnect(client *Client) {
    // Cleanup client
}
```

### Client Protocol
```json
// Subscribe
{"action": "subscribe", "event": "library:scan:progress"}

// Unsubscribe
{"action": "unsubscribe", "event": "library:scan:progress"}

// Event received
{"type": "library:scan:progress", "timestamp": "2024-01-15T10:30:00Z", "data": {"library_id": "abc", "processed": 50, "total": 100}}
```

### Integration Points
- `media.Scanner` publishes `library:scan:*` events
- `stream.Handler` publishes `stream:*` events
- `transcode.Orchestrator` publishes `transcode:*` events

### Dependencies
```go
github.com/gorilla/websocket v1.5.1
```

### Implementation Steps
1. Create `internal/events/bus.go` with pub/sub logic
2. Create `internal/events/types.go` with event definitions
3. Create `internal/events/websocket.go` with connection handling
4. Add `/ws` endpoint to router
5. Integrate event publishing into Scanner
6. Create client-side `useEvents` hook

---

## 4.7 Docker + Deploy ✅ COMPLETE

### Existing Files
```
Dockerfile              # Multi-stage build
Makefile               # Build targets
```

### Dockerfile (Current)
```dockerfile
# Stage 1: Web builder
FROM node:18 AS web-builder
WORKDIR /app
COPY web/package.json web/
RUN npm install
COPY web/ web/
RUN npm run build

# Stage 2: Go builder
FROM golang:1.21 AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o tenkile ./cmd/tenkile

# Stage 3: Final
FROM scratch
COPY --from=go-builder /app/tenkile /tenkile
COPY configs/ /etc/tenkile/
EXPOSE 8080
ENTRYPOINT ["/tenkile"]
```

### What Needs to Be Done

**1. Create docker-compose.yml**
```yaml
version: '3.8'

services:
  tenkile:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./configs/tenkile.yaml:/etc/tenkile/tenkile.yaml
      - media:/media
    environment:
      - TENKILE_CONFIG=/etc/tenkile/tenkile.yaml
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: tenkile
      POSTGRES_USER: tenkile
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?PostgreSQL password required}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  media:
  postgres_data:
```

**2. Update Dockerfile for web client**
- Build the Preact app (Stage 1)
- Serve static files from Go (add HTTP handler)
- Or use nginx sidecar

**3. Create production config**
```yaml
# configs/tenkile.prod.yaml
server:
  host: "0.0.0.0"
  port: "8080"

database:
  type: "postgres"
  host: "postgres"
  port: 5432
  name: "tenkile"
  user: "tenkile"
  password: "${POSTGRES_PASSWORD}"

auth:
  jwt_secret: "${JWT_SECRET}"
```

**4. Add healthcheck**
```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
```

**5. (Optional) Add nginx reverse proxy**
```yaml
nginx:
  image: nginx:alpine
  ports:
    - "80:80"
    - "443:443"
  volumes:
    - ./nginx.conf:/etc/nginx/nginx.conf
    - ./certs:/etc/nginx/certs
```

### Deployment Targets
1. **Docker Compose** - Local development, small deployments ✅
2. **Kubernetes** - Production (Helm chart) (future)
3. **Docker Swarm** - Medium deployments (future)

### Complete List
```
internal/database/migrations/
├── 20260322120000_init.sql                    # Users, devices, playback
├── 20260323000001_create_media_libraries.sql  # Library config
├── 20260323000002_create_media_items.sql      # Media metadata
├── 20260323000003_create_series_episodes.sql  # TV organization
└── 20260323000004_create_streams_sessions.sql # Streaming sessions
```

### Media Items Schema
```sql
CREATE TABLE media_items (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    year INTEGER,
    overview TEXT,
    poster_path TEXT,
    
    -- JSONB for flexible stream metadata
    video_stream_json TEXT,
    audio_streams_json TEXT,
    subtitle_streams_json TEXT,
    
    container TEXT NOT NULL,
    duration REAL NOT NULL DEFAULT 0,
    file_size INTEGER NOT NULL DEFAULT 0,
    file_modified_at DATETIME NOT NULL,
    file_hash TEXT NOT NULL,  -- For change detection
    
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

---

## Phase 4 API Routes (Complete)

### Authentication Routes (Public)
```
POST   /api/v1/auth/login       # Login, returns access_token + refresh_token
POST   /api/v1/auth/first-run   # Initial admin setup
```

### Authentication Routes (Protected)
```
POST   /api/v1/auth/refresh     # Refresh access token
POST   /api/v1/auth/logout      # Logout
GET    /api/v1/auth/me          # Current user info
```

### Library Routes (Protected)
```
GET    /api/v1/libraries                    # List all libraries
POST   /api/v1/libraries                    # Create library
GET    /api/v1/libraries/{id}              # Get library
PUT    /api/v1/libraries/{id}              # Update library
DELETE /api/v1/libraries/{id}              # Delete library
POST   /api/v1/libraries/{id}/scan        # Trigger scan
GET    /api/v1/libraries/{id}/scan/status # Scan progress
GET    /api/v1/libraries/{libraryId}/items # List items (paginated)
```

### Media Routes (Protected)
```
GET    /api/v1/media/{id}                 # Get media item details
GET    /api/v1/media/{id}/stream          # Get stream info + variants
GET    /api/v1/media/{id}/play           # Get playback manifest
```

### Streaming Routes (Protected)
```
GET    /api/v1/stream/hls/{id}           # Get HLS manifest
GET    /api/v1/stream/hls/playlist       # Serve playlist file
GET    /api/v1/stream/hls/segment        # Serve segment file
```

---

## Next Steps (Priority Order)

All Phase 5 tasks are complete. Phase 6 focuses on production hardening:

1. **Performance Optimization** - MEDIUM PRIORITY
   - FFmpeg pipeline optimization
   - Concurrent transcode limits
   - Memory pool optimization

2. **Caching Improvements** - MEDIUM PRIORITY
   - Transcoded segment caching
   - CDN integration
   - Cache invalidation

3. **Monitoring & Metrics** - MEDIUM PRIORITY
   - Prometheus metrics
   - Grafana dashboards
   - Alerting setup

4. **Additional Platforms** - LOW PRIORITY
   - tvOS native app
   - Android TV native app
   - Samsung/LG Smart TV apps
   - Caching improvements
   - Monitoring/metrics

---

## Phase 4 Implementation Drift

| Planned | Actual | Notes |
|---------|--------|-------|
| HLS/DASH streaming as separate tasks | Combined into 4.2 | Simplified for cleaner delivery |
| Web client separate from docker | Web client + docker coordinated | Ensured web is built into container |
| WebSocket deferred | WebSocket implemented in 4.6 | Real-time events enabled |
| Dockerfile only | Dockerfile + docker-compose + nginx | Complete deployment stack |
---

## Phase 5: Client Adapters + Native Probes

> **Status:** ✅ COMPLETE (3/3 sub-tasks complete)

This phase adds platform-native probe implementations for Android and Apple devices, providing higher trust scores than browser-based CodecProbe.

### Sub-Task Status

| Task | Status | Priority | Dependencies |
|------|--------|----------|--------------|
| **5.1 Native Probe Interface** | ✅ COMPLETE | HIGH | needs 1.3, 2.2 ✓ |
| **5.2 Android Probe** | ✅ COMPLETE | HIGH | needs 5.1 ✓ |
| **5.3 Apple Probe** | ✅ COMPLETE | HIGH | needs 5.1 ✓ |

### Agent Prompt 5.1: Platform-Specific Probe Interface

```
Build probe libraries that use platform-native APIs instead of browser APIs.
These provide higher trust (0.85) than browser-based CodecProbe (0.50-0.80).

Reference: ../docs/DEVICE_DETECTION.md (platform detection strategies, trust scoring,
capability probing patterns for Smart TVs, Android, iOS)

Platform strategy:
- Tizen/WebOS: web-based -> CodecProbe JS + curated DB (already handled)
- Android: MediaCodecList + MediaCodecInfo -> exhaustive HW decoder list
- iOS/tvOS: AVFoundation + AVPlayer.availableHDRModes

Create in internal/clients/:

1. adapter.go - ClientAdapter interface
   - ClientType() string
   - ExtractCapabilities(r *http.Request) (*probes.DeviceCapabilities, error)
   - Import DeviceCapabilities from internal/probes — do NOT redeclare it
   - The DeviceCapabilities struct has Platform, UserAgent, VideoCodecs, AudioCodecs,
     MaxWidth, MaxHeight, SupportsHDR, SupportsDolbyVision, etc. (see Phase 1 Inventory)

2. detect.go - Identify platform from request headers/UA, route to adapter

3. web.go - Web client adapter (uses CodecProbe results from Phase 1 probe library)

4. Android/ - Kotlin probe library (future, create interface + stubs)
   - Query MediaCodecList.getCodecInfos() for all HW decoders
   - Map to Tenkile's capability model
   - Report via POST /api/v1/devices/{id}/probe

5. Apple/ - Swift probe library (future, create interface + stubs)
   - Query AVFoundation
   - Check AVPlayer.availableHDRModes
   - Report via same API

For now: implement the adapter interface and WebClientAdapter.
Native probe stubs document what each platform API provides.

After completing 5.1, run: go build ./... && go test ./internal/clients/...
```

### Agent Prompt 5.2: Android Native Probe Implementation

```
Implement the Android probe library using MediaCodec APIs.
This provides higher trust (0.85) than browser-based CodecProbe.

Reference: ../docs/DEVICE_DETECTION.md (Android detection patterns, MediaCodec API usage)
Reference: ../docs/CODEC_REFERENCE.md (codec profile/level definitions)

Create in internal/clients/android/:

1. Android Probe Library (android/ directory with README.md documenting API usage)

   **Capabilities to Probe:**
   - Video HW decoders via MediaCodecList.getCodecInfos()
   - Audio HW decoders via MediaCodecList
   - HDR capabilities via MediaCodecCodecCapabilities
   - Dolby Vision support via MediaDrm (for widevine)
   - Max resolution from MediaCodecInfo.getCapForType()
   - Container support (MP4, MKV, WebM)

   **Key Android APIs:**
   ```kotlin
   // Get all hardware decoders
   MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos

   // Check codec capabilities
   codec.getCapabilitiesForType("video/hevc")
   capabilities.isFormatSupported()

   // HDR modes
   MediaCodecInfo.VideoCapabilities.supportsHdr()
   MediaCodecInfo.CodecCapabilities.COLOR_Format12bitHDR10
   MediaCodecInfo.CodecCapabilities.COLOR_FormatDolbyVision

   // Widevine DRM
   MediaDrm.getSupportedUsableSecurityLevels()
   ```

   **Mapping to Tenkile Model:**
   - HEVC Main/Main10 → Tenkile HEVC
   - VP9 Profile 0/2 → Tenkile VP9
   - AV1 → Tenkile AV1
   - Dolby Vision → SupportsDolbyVision + DolbyVisionProfile
   - HDR10/HDR10+/HLG → SupportsHDR + HDRTypes[]

2. REST API Integration
   - POST results to /api/v1/devices/{id}/probe
   - Include platform="android" and device model from Build.MODEL

3. Testing Strategy
   - Test on various Android TV devices (Shield, Chromecast, Mi Box)
   - Verify HW decoder list matches device specifications
   - Cross-reference with curated DB as fallback

### Agent Prompt 5.3: Apple Native Probe Implementation

```
Implement the Apple probe library using AVFoundation APIs.
This provides higher trust (0.85) than browser-based CodecProbe.

Reference: ../docs/DEVICE_DETECTION.md (Apple detection patterns, AVFoundation API usage)
Reference: ../docs/CODEC_REFERENCE.md (codec profile/level definitions)

Create in internal/clients/apple/:

1. Apple Probe Library (apple/ directory with README.md documenting API usage)

   **Capabilities to Probe:**
   - Video HW decoders via AVFoundation
   - Audio HW decoders via AudioToolbox
   - HDR capabilities via AVPlayer.availableHDRModes
   - Dolby Vision/Atmos support
   - Container support (MP4, MOV)
   - No MKV/WebM (Apple limitation)

   **Key Apple APIs:**
   ```swift
   // Get decoder capabilities
   let decoders = AVVideoDecoderHints.supportedDecoderHints

   // HDR modes available
   AVPlayer.availableHDRModes

   // Format support
   AVAsset.isPlayable
   AVAssetWriter.canInit(withOutputSettings:)

   // DRM (FairPlay)
   AVContentKeySession
   ```

   **Mapping to Tenkile Model:**
   - HEVC Main/Main10/Main10-422 → Tenkile HEVC + profiles
   - H.264 → Tenkile AVC
   - Dolby Digital Plus → E-AC-3
   - Dolby Atmos → TrueHD Atmos support
   - HDR10/HDR10+/Dolby Vision → SupportsHDR + HDRTypes[]
   - AV1 (M-series chips) → Tenkile AV1

2. REST API Integration
   - POST results to /api/v1/devices/{id}/probe
   - Include platform="appletvos" or "appleios"
   - Include device model and chip identifier

3. Testing Strategy
   - Test on Apple TV 4K, Apple TV HD
   - Test on iPhone/iPad with various chipsets
   - Verify M-series AV1 support on newer chips

### Phase 5 Checkpoint: Git Commit

After completing all Phase 5 tasks (5.1–5.3), create a git commit:

```bash
git add -A && git diff --cached --stat  # Review what's staged before committing
git commit -m "Phase 5: Client adapters + native probes

Implement client adapter interface, web adapter, Android MediaCodec probe,
and Apple AVFoundation probe.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
git push origin main
```

---

## Phase 6: Polish + Production

> **Status:** ○ PLANNED

This phase covers production hardening, performance optimization, and additional platform support.

### Planned Work

| Area | Tasks |
|------|-------|
| **Performance** | FFmpeg pipeline optimization, concurrent transcode limits, GPU scheduling |
| **Caching** | Response caching, CDN integration, edge streaming |
| **Monitoring** | Prometheus metrics, Grafana dashboards, alerting |
| **Reliability** | Health checks, graceful degradation, circuit breakers |
| **Platforms** | tvOS app, Android TV app, Smart TV apps (Samsung, LG) |
| **Features** | Offline sync, multi-user sync, collaborative playlists |

### Key Topics

1. **Performance Optimization**
   - Transcode pipeline parallelization
   - Memory pool optimization for segment generation
   - GPU encoder selection based on concurrent streams
   - Startup time reduction (lazy loading of components)

2. **Caching Improvements**
   - Transcoded segment caching (L1 in-memory, L2 disk)
   - Device capability caching TTL tuning
   - CDN pre-warming for popular content
   - Cache invalidation on library updates

3. **Monitoring & Metrics**
   - Prometheus metrics for all operations
   - Grafana dashboard templates
   - SLO/SLA tracking
   - Log aggregation setup
   - Distributed tracing (OpenTelemetry)

4. **Additional Platform Support**
   - tvOS native app with AVKit
   - Android TV native app with ExoPlayer
   - Samsung Tizen native app
   - LG WebOS native app
   - Casting protocols (Chromecast, AirPlay)

---

## Testing Strategy

### Current Test Coverage

| Package | Tests | Coverage | Status |
|---------|-------|----------|--------|
| `internal/probes/` | Parser, validator, cache, trust resolution tests | HIGH | ✅ Comprehensive |
| `internal/server/` | FFmpeg discovery, HW accel detection, encoder selection | HIGH | ✅ Comprehensive |
| `internal/transcode/` | Quality preservation, HDR tone mapping, audio ladder | HIGH | ✅ Comprehensive |
| `internal/media/` | Scanner, store operations | MEDIUM | ⚠️ Needs more |
| `internal/stream/` | Handler, segmenter logic | LOW | ⚠️ Needs tests |
| `internal/api/` | Auth flow, middleware, request handling | MEDIUM | ⚠️ Needs more |
| `pkg/codec/` | Codec equality, MIME type mapping | LOW | ⚠️ Needs tests |

### Test Gaps (Priority Order)

| Gap | Priority | Description |
|-----|----------|-------------|
| Media scanner integration | HIGH | Integration tests with real ffprobe output |
| Stream segmenter | HIGH | HLS manifest generation, segment file handling |
| FFmpeg integration | HIGH | Full transcode pipeline tests with real FFmpeg |
| Auth flow | MEDIUM | JWT refresh, token invalidation, first-run flow |
| API integration | MEDIUM | End-to-end request tests with httptest |
| Media store | MEDIUM | CRUD operations, pagination, query edge cases |

### Test Requirements for Phase 5

| Task | Required Tests |
|------|---------------|
| 5.1 Native Probe Interface | Unit tests for adapter interface, mock implementations |
| 5.2 Android Probe | Mock MediaCodec responses, format mapping tests |
| 5.3 Apple Probe | Mock AVFoundation responses, format mapping tests |

### Testing Conventions

1. **Table-driven tests** — Use Go's table-driven test pattern for multiple test cases:
   ```go
   func TestDecoderSelection(t *testing.T) {
       tests := []struct {
           name        string
           serverCaps  *ServerCapabilities
           targetCodec string
           wantEncoder string
       }{
           {"NVENC preferred", nvidiaCaps, "hevc", "hevc_nvenc"},
           {"QSV fallback", qsvCaps, "hevc", "hevc_qsv"},
           {"Software fallback", swCaps, "hevc", "libx265"},
       }
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               // test logic
           })
       }
   }
   ```

2. **Mock external dependencies** — Use interfaces for all external calls:
   - `CommandRunner` for exec.Command (FFmpeg, ffprobe)
   - `FFmpegPathFinder` for exec.LookPath
   - `DeviceExistsFunc` for os.Stat checks
   - Never make real system calls in unit tests

3. **Golden file tests** — For complex output (FFmpeg args, HLS manifests):
   ```go
   func TestBuildHLSArgs(t *testing.T) {
       got := buildHLSArgs(testDecision, testEncoder)
       want := testdata.MustRead("testdata/hls_args.golden")
       if diff := cmp.Diff(got, want); diff != "" {
           t.Fatalf("HLS args mismatch:\n%s", diff)
       }
   }
   ```

4. **Property-based tests** — For codec ladder correctness:
   ```go
   func TestCodecLadder_HasValidCodecs(t *testing.T) {
       for _, tc := range GetVideoCodecLadder("av1") {
           for _, codec := range tc {
               if !IsKnownVideoCodec(codec) {
                   t.Errorf("unknown codec in ladder: %s", codec)
               }
           }
       }
   }
   ```

5. **Integration tests** — Mark with `//go:build integration`:
   ```go
   //go:build integration
   // +build integration
   
   func TestFullTranscodePipeline(t *testing.T) {
       // Requires real FFmpeg installed
   }
   ```

6. **Test naming** — Follow pattern: `Test<Package>_<Concept>_<Scenario>`:
   - `TestParser_ValidResults_MultipleContainers`
   - `TestValidator_LyingAPI_DetectsInconsistency`
   - `TestSegmenter_HLSManifest_GeneratesCorrectPlaylist`

7. **Assertions** — Use `testify/assert` for clear failure messages:
   ```go
   assert.NoError(t, err, "ScanLibrary should not error")
   assert.Equal(t, expected, got, "encoder selection")
   assert.True(t, found, "device should be found in cache")
   ```

8. **Coverage targets**:
   - Critical paths (parser, validator, orchestrator): 80%+
   - Core packages (probes, transcode, server): 70%+
   - Support packages (api, media, stream): 50%+

9. **Race detection** — Always run with `-race`:
   ```bash
   go test ./... -race -count=1
   ```

10. **Benchmark tests** — For performance-critical code:
    ```go
    func BenchmarkSegmenter_GenerateSegments(b *testing.B) {
        for i := 0; i < b.N; i++ {
            segmenter.GenerateHLS(ctx, testInput, variants, opts)
        }
    }
    ```

---

## Phase Implementation Log Template

Use this template to document what was actually built vs. what was specified. Fill this in after completing each phase.

```markdown
## Phase [N] Implementation Log

Date Started: 
Date Completed: 
Implementation Agent: 

### Spec vs. Implementation

| Spec Item | Specified | Delivered | Variance |
|-----------|-----------|-----------|----------|
| File count | N | M | +/-(N-M) |
| API endpoints | N | M | +/-(N-M) |
| Platform count | N | M | +/-(N-M) |

### Bugs Found During Review

| Bug | Severity | Fixed | Description |
|-----|----------|-------|-------------|
| ... | Critical/Major/Minor | Yes/No | ... |

### Unresolved Questions

| Question | Resolution |
|----------|------------|
| ... | ... |

### Implicit Files Created

| File | Why Needed |
|------|------------|
| ... | ... |

### Dependencies Added

| Package | Purpose |
|---------|---------|
| ... | ... |
```

---

## Prompt Index

| # | Prompt | Depends On | Deliverable |
|---|--------|-----------|-------------|
| 1.1 | Scaffold Go project | — | Project structure, go.mod, Makefile |
| 1.2 | Codec types | 1.1 | pkg/codec/, internal/probes/types.go |
| 1.3 | Probe receiver + cache | 1.1 | Parser, validator, cache, REST API, SQLite |
| 1.4 | JS probe library | 1.1 | web/probe/tenkile-probe.js |
| 2.1 | Server capability discovery | 1.1 | internal/server/ (FFmpeg, HW accel) |
| 2.2 | Transcode decision engine | 1.2, 1.3, 2.1 | internal/transcode/ (orchestrator, codec ladders) |
| 2.3 | Decision logger | 2.2 | Audit trail, admin query endpoints |
| 3.1 | Curated Smart TV DB | 1.3 | internal/probes/curated.go, data/curated/*.json |
| 3.2 | Playback feedback loop | 1.3, 2.3 | internal/probes/feedback.go |
| 4.1 | Media library scanner | 1.1 | internal/media/ (scanner, ffprobe, metadata) |
| 4.2 | HLS/DASH streaming | 2.2, 4.1 | internal/api/stream.go, FFmpeg process mgmt |
| 4.3 | OpenAPI spec | 1.3, 2.3, 3.1, 4.1, 4.2 | api/openapi.yaml, generated server + client code |
| 4.4 | Auth + first-run setup | 4.3 | JWT, users, API keys, setup wizard endpoint |
| 4.5 | Web client | 4.3, 4.4 | web/ (Preact + Vite + TypeScript SPA) |
| 4.6 | WebSocket events | 4.4 | Real-time events: scan, transcode, playback |
| 4.7 | Docker + deployment | 4.5 | Dockerfile, docker-compose, config file |
| 5.1 | Native probe interface | 1.3, 2.2 | Client adapter interface, platform stubs |
| 5.2 | Android probe | 5.1 | MediaCodec-based capability probing |
| 5.3 | Apple probe | 5.1 | AVFoundation-based capability probing |

## Quick Reference

| What | Where |
|------|-------|
| Docs index | `../docs/README.md` |
| Architecture spec | `../docs/ARCHITECTURE.md` |
| Language rationale | `../docs/LANGUAGE_AND_SCALABILITY.md` |
| Web UI architecture | `../docs/WEB_UI.md` |
| Codec reference | `../docs/CODEC_REFERENCE.md` |
| Container formats | `../docs/CONTAINER_FORMATS.md` |
| DRM reference | `../docs/DRM_REFERENCE.md` |
| Device detection | `../docs/DEVICE_DETECTION.md` |
| Device database | `../docs/DEVICE_DATABASE.md` |
| FFmpeg reference | `../docs/FFMPEG_REFERENCE.md` |
| Jellyfin reference | `../docs/JELLYFIN_REFERENCE.md` |
| OpenAPI spec | `api/openapi.yaml` (created in prompt 4.3) |
| Default config | `configs/tenkile.yaml` (created in prompt 4.7) |
| Curated device DB | `data/curated/*.json` (Phase 3.1 — 12 platforms, 97 devices) |
| Jellyfin StreamBuilder (study) | `../jellyfin/MediaBrowser.Model/Dlna/StreamBuilder.cs` |
| Jellyfin encoding pipeline (study) | `../jellyfin/MediaBrowser.MediaEncoding/Encoder/MediaEncoder.cs` |
| Jellyfin HLS controller (study) | `../jellyfin/Jellyfin.Api/Controllers/DynamicHlsController.cs` |
| Jellyfin device profiles (study) | `../jellyfin/MediaBrowser.Model/Dlna/DirectPlayProfile.cs` |
| CodecProbe test engine | `../codecprobe/js/codec-tester.js` |
| CodecProbe codec database | `../codecprobe/js/codec-database-v2.js` |
| CodecProbe DRM detection | `../codecprobe/js/drm-detection.js` |
| CodecProbe device detection | `../codecprobe/js/device-detection.js` |

## Conventions

- **License:** AGPL-3.0-or-later. SPDX header in every source file.
- **Language:** Go (server), TypeScript (web client), JavaScript (probe library)
- **Style:** `gofmt`, `golangci-lint`, standard Go naming conventions
- **Testing:** Go `testing` package + `testify` for assertions. Table-driven tests.
- **Database:** SQLite default (`modernc.org/sqlite`), PostgreSQL for multi-instance (`pgx`)
- **Migrations:** goose v3, SQL files in `internal/database/migrations/`, embedded in binary
- **API:** OpenAPI 3.1 spec -> `oapi-codegen` (Go server) + `openapi-typescript` (TS client). `chi` router.
- **Config:** YAML (`configs/tenkile.yaml`) + env var overrides (`TENKILE_` prefix) via koanf
- **Auth:** JWT access tokens (1h) + refresh tokens (30d). API keys for third-party clients.
- **Logging:** `slog` (stdlib structured logging)
- **WebSocket:** `gorilla/websocket` for real-time events. JSON messages.
- **Embedded assets:** Go `embed` directive for web client, curated DB, default config, migrations
- **Web client:** Preact + Vite + TypeScript (strict). CSS Modules. < 100KB gzipped. See `../docs/WEB_UI.md`.
- **Device ID:** Server-assigned device IDs, not custom fingerprints
- **Trust:** Every capability claim has a trust score (0.0-1.0), never binary yes/no
- **HDR:** Preserve when device has HDR display, tone-map with high quality when it doesn't. Never just strip metadata.
- **Audio:** Preserve surround channels down to what device supports. Ladder down, never cliff-drop to stereo.
- **Tone mapping:** libplacebo (GPU) > zscale+hable (CPU) > reinhard (fast). BT.2020->BT.709 gamut mapping is mandatory.
- **Build:** `make build` (web + Go), `make dev` (hot reload), `make docker` (container)
- **Test:** `make test` (`go test ./... -race`)
- **Lint:** `make lint` (`golangci-lint run`)
- **Generate:** `make generate` (OpenAPI -> Go server stubs + TS types)
