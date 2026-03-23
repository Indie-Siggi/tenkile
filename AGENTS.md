# Tenkile - Agent Task Guide

> **Goal:** Build a standalone media server replacing Jellyfin — intelligent, device-aware media delivery powered by CodecProbe.
> **Language:** Go
> **Source repos:** `../jellyfin/` (C#/.NET reference), `../codecprobe/` (JavaScript codec detection)
> **Architecture:** `../docs/ARCHITECTURE.md`
> **Target:** This folder (`Tenkile/`)
> **License:** AGPL-3.0-or-later

---

## Agent Rules (READ FIRST)

These rules are mandatory for every agent. They exist because Phase 1 code generation produced code that did not compile. Every rule below addresses a specific failure mode.

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

## Phase Dependencies

```
Phase 1: Foundation
  1.1 Scaffold ──────────────────────┐
  1.2 Codec Types ───┐               │
  1.3 Probe Receiver ┤ (needs 1.1)   │
  1.4 JS Probe Lib ──┘               │
                                     │
Phase 2: Server + Decision Engine    │
  2.1 Server Inventory ──┐ (needs 1.1)
  2.2 Transcode Engine ──┤ (needs 1.2, 1.3, 2.1)
  2.3 Decision Logger ───┘ (needs 2.2)
                                     │
Phase 3: Curated DB + Feedback       │
  3.1 Curated Smart TV DB ─┐ (needs 1.3)
  3.2 Playback Feedback ───┘ (needs 1.3, 2.3)
                                     │
Phase 4: Media Library + Streaming   │
  4.1 Media Scanner ─────┐ (needs 1.1)
  4.2 HLS/DASH Streaming ┤ (needs 2.2, 4.1)
  4.3 OpenAPI Spec ──────┤ (needs 1.3, 2.3, 3.1, 4.1, 4.2)
  4.4 Auth + First-Run ──┤ (needs 4.3)
  4.5 Web Client ────────┤ (needs 4.3, 4.4)
  4.6 WebSocket Events ──┤ (needs 4.4)
  4.7 Docker + Deploy ───┘ (needs 4.5)
                                     │
Phase 5: Client Adapters             │
  5.1 Native Probes ─────── (needs 1.3, 2.2)
```

**Parallelization opportunities:**
- 1.2 + 1.4 can run in parallel (both need 1.1 only)
- 2.1 can start as soon as 1.1 is done (independent of 1.2–1.4)
- 3.1 can start as soon as 1.3 is done (independent of Phase 2)
- 4.1 can start as soon as 1.1 is done (independent of Phases 2–3)
- 5.1 can start after 1.3 + 2.2 (independent of Phases 3–4)

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

2. data/curated/ - JSON seed data:
   - samsung_tizen.json (2018-2025 model families, codec support per SoC)
   - lg_webos.json (2018-2025)
   - roku.json (per-model codec matrix)
   - android_tv.json (Chromecast, Shield, Mi Box, etc.)

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

## Phase 4: Media Library, Streaming, Web UI & Deployment

### Agent Prompt 4.1: Implement Media Library Scanner

```
Build the media library scanner that discovers and indexes media files.

Create in internal/media/:

1. scanner.go
   - Scan configured library paths recursively
   - Detect media files by extension (.mkv, .mp4, .avi, .mov, .ts, .m2ts, etc.)
   - Watch for filesystem changes (fsnotify)
   - Incremental scanning (only process new/changed files)
   - Configurable scan interval

2. probe.go
   - Use ffprobe to extract media metadata:
     Video: codec, profile, level, bit depth, HDR type, resolution, framerate,
            mastering display metadata, max content light level
     Audio: codec, channels, sample rate, bit depth
     Subtitles: format (SRT/ASS/PGS/VOBSUB), language, forced flag
     Container: format, duration, overall bitrate
   - Parse ffprobe JSON output into MediaItem struct
   - Detect: interlaced, anamorphic, HDR10/DV/HLG

3. metadata.go
   - TMDb API integration for movie metadata (poster, title, year, overview)
   - TVDB/TMDb for TV show metadata (series, season, episode)
   - Match filenames to metadata using naming conventions
   - Cache metadata in database

4. MediaItem struct:
   - Path, Library, Title, Year, Overview, Poster
   - VideoCodec, VideoProfile, VideoLevel, BitDepth, HDRType, Width, Height, Framerate
   - AudioStreams (multiple), SubtitleStreams (multiple)
   - Container, Duration, Bitrate
   - IsInterlaced, HasAnamorphicPixels, PixelAspectRatio
   - MasteringDisplayMetadata, MaxContentLightLevel, DolbyVisionProfile

This is a NEW package (internal/media/). Use pkg/codec for codec name constants.
Do NOT import internal/probes/ or internal/transcode/ — the media scanner is
independent. The transcode engine will import media types, not the other way around.

After completing 4.1, run: go build ./... && go test ./internal/media/...
```

### Agent Prompt 4.2: Implement HLS/DASH Streaming

```
Build the streaming endpoints that serve media to clients.

Create in internal/api/:

1. stream.go
   - GET /api/v1/stream/{itemId}/master.m3u8 - HLS master playlist
   - GET /api/v1/stream/{itemId}/{quality}/{segId}.ts - HLS segment
   - GET /api/v1/stream/{itemId}/direct - Direct file serving (no transcode)
   - GET /api/v1/stream/{itemId}/subs/{trackId}.vtt - External subtitle track

2. FFmpeg process management:
   - Start transcode process based on TranscodeOrchestrator decision
   - Monitor process health, restart on failure
   - Cancel when client disconnects
   - Limit concurrent transcodes (configurable)
   - Segment-based seeking for HLS

3. Direct play:
   - Serve file directly with range request support
   - Content-Type from container MIME type
   - Seek support via byte ranges

4. Bandwidth estimation:
   - Estimate connection speed from segment download times
   - Adjust quality if bandwidth is insufficient for current stream

Create stream.go as a NEW file in internal/api/. The existing router.go already
has stub handlers (listLibrariesHandler, etc.) — replace those stubs with real
implementations that call the new media and streaming packages.

Register new routes in router.go inside the existing auth Route group.
Use existing RespondJSON() and ErrorResponse from response.go.

After completing 4.2, run: go build ./... && go test ./...
```

### Agent Prompt 4.3: Create OpenAPI Specification

```
Consolidate ALL API endpoints from Phases 1-4 into a single OpenAPI 3.1 spec.
This is the contract between the Go server and all clients (web, mobile, third-party).

Create api/openapi.yaml with all endpoints:

  Devices & Probing (from Phase 1):
    POST   /api/v1/devices/{id}/probe        - Submit CodecProbe results
    GET    /api/v1/devices/{id}/capabilities  - Get resolved capabilities with trust
    POST   /api/v1/devices/{id}/feedback      - Report playback success/failure
    DELETE /api/v1/admin/capabilities/{id}    - Clear capability cache

  Playback Decisions (from Phase 2):
    POST   /api/v1/playback/decide            - Request playback decision for media item

  Decision Audit (from Phase 2):
    GET    /api/v1/admin/decisions             - Query decision log (filterable)
    GET    /api/v1/admin/decisions/stats       - Aggregate statistics

  Curated Devices (from Phase 3):
    GET    /api/v1/admin/curated-devices       - List curated device profiles
    POST   /api/v1/admin/curated-devices       - Add curated device
    PUT    /api/v1/admin/curated-devices/{id}  - Update curated device

  Library (from Phase 4.1):
    GET    /api/v1/library                     - Browse library (paginated, filterable)
    GET    /api/v1/library/{id}                - Media item detail
    POST   /api/v1/library/scan                - Trigger library scan

  Streaming (from Phase 4.2):
    GET    /api/v1/stream/{itemId}/master.m3u8 - HLS master playlist
    GET    /api/v1/stream/{itemId}/{quality}/{segId}.ts - HLS segment
    GET    /api/v1/stream/{itemId}/direct       - Direct play stream
    GET    /api/v1/stream/{itemId}/subs/{trackId}.vtt - Subtitle track

  Auth (from Phase 4.4):
    POST   /api/v1/auth/login                  - Login (returns JWT + refresh token)
    POST   /api/v1/auth/refresh                - Refresh access token
    DELETE /api/v1/auth                        - Logout
    GET    /api/v1/auth/me                     - Current user info

  Users:
    GET    /api/v1/users                       - List users (admin)
    POST   /api/v1/users                       - Create user
    PUT    /api/v1/users/{id}                  - Update user
    DELETE /api/v1/users/{id}                  - Delete user

  System:
    GET    /api/v1/system                      - Server info + capabilities
    GET    /api/v1/system/setup                - Setup status (has admin been created?)
    POST   /api/v1/system/setup                - Complete first-run setup

  WebSocket:
    GET    /api/v1/ws                          - Real-time event stream

For each endpoint define:
  - Request/response schemas with JSON examples
  - Error responses (400, 401, 403, 404, 500)
  - Authentication requirements (which endpoints need JWT, which are public)
  - Pagination schema (offset/limit with total count)

After creating the spec, generate code:
  - Go server: oapi-codegen -> internal/api/generated/server.gen.go
  - TypeScript client: openapi-typescript -> web/src/api/generated/schema.ts
  - Run: make generate

The generated Go interface becomes the contract — handlers must implement it.

NOTE: Many of these endpoints already exist in router.go from Phase 1.
The OpenAPI spec should document the ACTUAL routes and request/response shapes
already implemented, plus new ones from Phases 2-4. Do not change existing
handler signatures to match a spec — update the spec to match the code,
then evolve both together.

After generating code, run: go build ./... && go test ./...
```

### Agent Prompt 4.4: Implement Auth and First-Run Setup

```
Build authentication, user management, and the first-run setup flow.

Prerequisites: api/openapi.yaml (4.3)

Create:

1. internal/api/middleware.go (NEW file, but NOTE: router.go already has)
   - authMiddleware (stub) and adminOnlyMiddleware (stub) — replace stubs with real JWT validation
   - rateLimitMiddleware with rateLimiter struct — already implemented, extend if needed
   - CORS is already configured in router.go via chi/cors — do NOT duplicate
   - Request logging already uses chi/middleware.Logger — do NOT duplicate
   Add to middleware.go:
   - JWT token validation logic
   - API key authentication (for third-party clients, X-API-Key header)

2. internal/api/auth.go (implement generated server interfaces)
   - POST /api/v1/auth/login
     - Validate credentials against bcrypt hash
     - Return JWT access token (1 hour TTL) + refresh token (30 day, httpOnly cookie)
   - POST /api/v1/auth/refresh
     - Validate refresh token, issue new access token
   - DELETE /api/v1/auth
     - Invalidate refresh token
   - GET /api/v1/auth/me
     - Return current user profile

3. internal/database/ - User schema + migrations
   - users table: id, username, password_hash, is_admin, created_at
   - api_keys table: id, user_id, key_hash, name, created_at, last_used_at
   - refresh_tokens table: id, user_id, token_hash, expires_at

4. First-run setup flow:
   - GET /api/v1/system/setup returns { setupRequired: true } if no users exist
   - POST /api/v1/system/setup creates admin user + sets initial config
     Input: { username, password, libraryPaths: [...] }
   - This endpoint is ONLY accessible when no users exist (returns 403 otherwise)
   - After setup completes, triggers initial library scan
   - Web client checks /system/setup on load and redirects to setup wizard if needed

Tests:
  - Login with valid/invalid credentials
  - JWT token validation and expiry
  - Refresh token rotation
  - API key auth
  - First-run setup: works when no users, blocked when users exist
  - CORS: allowed and disallowed origins

Register auth routes in router.go — add a new public group for /auth endpoints.
Use existing RespondJSON() and ErrorResponse from response.go.
After completing 4.4, run: go build ./... && go test ./...
```

### Agent Prompt 4.5: Build Web Client

```
Build the full web client per ../docs/WEB_UI.md architecture.

Prerequisites: OpenAPI spec (4.3), Auth (4.4)

Create web/ directory:

1. Scaffold:
   web/
   ├── package.json           # Preact + Vite + TypeScript
   ├── vite.config.ts         # Proxy /api to Go server in dev mode
   ├── tsconfig.json          # Strict mode
   ├── index.html             # Minimal shell, loads main.tsx
   └── src/
       ├── main.tsx           # App bootstrap
       ├── app.tsx            # Root component + router

2. API layer (generated + thin wrapper):
   web/src/api/
   ├── generated/schema.ts   # From openapi-typescript (make generate)
   ├── client.ts             # Fetch wrapper: base URL, JWT from localStorage,
   │                           auto-refresh on 401, error handling
   └── websocket.ts          # WebSocket connection manager with auto-reconnect

3. Pages:
   web/src/pages/
   ├── setup/Setup.tsx        # First-run wizard (create admin, set library paths)
   ├── login/Login.tsx        # Login form
   ├── home/Home.tsx          # Library browser (poster grid, search, filters)
   ├── detail/Detail.tsx      # Media item detail (metadata, streams, play button)
   ├── player/
   │   ├── Player.tsx         # Player shell + controls (play/pause, seek, volume,
   │   │                        audio track selector, subtitle selector)
   │   ├── HlsPlayer.tsx      # hls.js wrapper for transcoded/adaptive content
   │   ├── DirectPlayer.tsx   # Native <video> for direct play
   │   └── Player.module.css
   └── settings/Settings.tsx  # Server config (libraries, users, transcoding)

4. Components:
   web/src/components/
   ├── PosterGrid.tsx         # Responsive poster grid (CSS Grid, auto-fit)
   ├── MediaCard.tsx          # Single poster card with title overlay
   ├── Nav.tsx                # Top navigation bar
   └── Layout.tsx             # Page layout shell (nav + content area)

5. CodecProbe integration:
   web/src/probe/
   ├── index.ts               # Import tenkile-probe.js from web/probe/
   │                            Run on first visit, POST results to /api/v1/devices/{id}/probe
   └── feedback.ts            # Hook into <video> events:
                                'playing' -> report success after 5s smooth playback
                                'error' -> report failure immediately with error code

6. State (Preact Signals):
   web/src/state/
   ├── auth.ts                # JWT token, user profile, login/logout actions
   ├── player.ts              # Current playback state, active stream info
   └── library.ts             # Current library view, filters, search query

7. Embed in Go binary:
   cmd/tenkile/main.go ALREADY EXISTS — extend it, do NOT rewrite.
   Add: //go:embed web/dist/*
   Serve at /web/* with SPA fallback (index.html for unmatched routes).
   Root (/) redirects to /web/.
   --no-web flag disables web client serving (API-only mode).
   The existing main.go uses slog, flag, and starts an http.Server — follow the same patterns.

Technology constraints:
  - Preact (not React) — 3KB vs 40KB
  - preact-router for client-side routing
  - CSS Modules for scoped styling (no CSS framework)
  - hls.js for adaptive streaming
  - 100% TypeScript, strict mode
  - Target: < 100KB gzipped initial load (excluding lazy-loaded hls.js and probe)
  - Code-split: hls.js and probe loaded only when needed

Build:
  cd web && npm run build -> outputs to web/dist/
  make build-web runs this step
  make dev runs Vite dev server with proxy to Go server on port 8096

NOTE: web/probe/tenkile-probe.js ALREADY EXISTS from Phase 1.
Import it from the probe integration code, do not recreate it.

After completing 4.5, run: cd web && npm run build && cd .. && go build ./...
```

### Agent Prompt 4.6: Implement WebSocket Real-Time Events

```
Build WebSocket support for real-time server-to-client events.

Prerequisites: Auth (4.4)

Create:

1. internal/api/websocket.go
   - GET /api/v1/ws - Upgrade to WebSocket (requires valid JWT)
   - Use github.com/gorilla/websocket
   - Connection manager: track active connections per user
   - Broadcast to all connections, or target specific user/session
   - Ping/pong heartbeat (30s interval) to detect stale connections
   - Graceful shutdown: send close frame on server stop

2. Event types (JSON messages, server -> client):

   Library scanning:
   { "type": "library.scan.started", "libraryId": "...", "libraryName": "..." }
   { "type": "library.scan.progress", "libraryId": "...", "percent": 42,
     "filesScanned": 1200, "filesTotal": 2850 }
   { "type": "library.scan.complete", "libraryId": "...", "itemsAdded": 350,
     "itemsUpdated": 12, "duration": "2m34s" }

   Playback:
   { "type": "playback.started", "sessionId": "...", "itemId": "...",
     "userId": "...", "deviceName": "..." }
   { "type": "playback.stopped", "sessionId": "..." }
   { "type": "playback.progress", "sessionId": "...", "position": 3600,
     "duration": 7200 }

   Transcode:
   { "type": "transcode.started", "sessionId": "...", "encoder": "hevc_nvenc" }
   { "type": "transcode.progress", "sessionId": "...", "percent": 65,
     "fps": 120, "speed": "5.0x" }
   { "type": "transcode.error", "sessionId": "...", "error": "..." }

   System:
   { "type": "system.shutdown", "reason": "restart", "countdown": 30 }

3. internal/events/bus.go - Simple event bus
   - Publish(event Event) - called by scanner, transcode manager, etc.
   - Subscribe(filter EventFilter) <-chan Event
   - WebSocket handler subscribes and forwards matching events to client

4. Client-side (update web/src/api/websocket.ts):
   - Auto-connect after login
   - Auto-reconnect with exponential backoff (1s, 2s, 4s, max 30s)
   - Parse event types, dispatch to Preact Signals
   - Show scan progress bar on home page
   - Show transcode progress in player
   - Show active sessions in admin/settings page

Register the WebSocket endpoint in router.go inside the existing auth group.
Use github.com/gorilla/websocket — add to go.mod with go get.

After completing 4.6, run: go build ./... && go test ./...
```

### Agent Prompt 4.7: Docker and Deployment

```
Build Docker support and deployment configuration.

Prerequisites: Web client (4.5)

Create:

1. Dockerfile (multi-stage):

   # Stage 1: Build web client
   FROM node:22-alpine AS web-builder
   WORKDIR /build/web
   COPY web/package*.json ./
   RUN npm ci
   COPY web/ ./
   RUN npm run build

   # Stage 2: Build Go binary
   FROM golang:1.24-alpine AS go-builder
   WORKDIR /build
   COPY go.mod go.sum ./
   RUN go mod download
   COPY . .
   COPY --from=web-builder /build/web/dist ./web/dist
   RUN CGO_ENABLED=0 go build -o tenkile ./cmd/tenkile/

   # Stage 3: Final image
   FROM scratch
   COPY --from=go-builder /build/tenkile /tenkile
   COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
   EXPOSE 8096
   VOLUME ["/config", "/media"]
   ENTRYPOINT ["/tenkile"]
   CMD ["--config", "/config/tenkile.yaml"]

   Target image size: < 20MB (scratch base + static Go binary)

2. docker-compose.yml (for users who want easy setup):

   services:
     tenkile:
       image: tenkile:latest
       build: .
       ports:
         - "8096:8096"
       volumes:
         - ./config:/config
         - /path/to/media:/media:ro
       environment:
         - TENKILE_DATABASE_PATH=/config/tenkile.db
       restart: unless-stopped

   Optional: add PostgreSQL + Redis services for multi-instance deployment.

3. configs/tenkile.yaml - Default configuration file:

   server:
     port: 8096
     host: "0.0.0.0"

   database:
     driver: sqlite               # sqlite or postgres
     path: "./data/tenkile.db"  # SQLite path
     # postgres: "postgres://user:pass@host:5432/tenkile"  # PostgreSQL DSN

   libraries:
     - name: "Movies"
       path: "/media/movies"
       type: movie
     - name: "TV Shows"
       path: "/media/tv"
       type: tvshow

   transcoding:
     ffmpeg_path: "ffmpeg"         # Auto-detect if on PATH
     max_concurrent: 2             # Limit simultaneous transcodes
     temp_dir: "/tmp/tenkile"  # Transcode temp directory
     hw_accel: auto                # auto | nvenc | qsv | vaapi | videotoolbox | none

   probe:
     cache_ttl: "168h"             # 7 days
     min_trust_direct_play: 0.6
     re_probe_on_failure: true

   quality:
     hdr_policy: best_for_device   # best_for_device | always_tonemap | never_tonemap
     tonemap_quality: high          # high (libplacebo) | medium (zscale) | fast (reinhard)
     audio_channel_policy: preserve # preserve | allow_downmix
     max_bitrate: 0                # 0 = unlimited

   auth:
     jwt_secret: ""                # Auto-generated on first run if empty
     access_token_ttl: "1h"
     refresh_token_ttl: "720h"     # 30 days

   logging:
     level: info                   # debug | info | warn | error
     format: text                  # text | json

4. internal/config/config.go (ALREADY EXISTS from Phase 1 — extend, don't rewrite)
   - Uses gopkg.in/yaml.v3 with yaml struct tags (NOT koanf)
   - Add environment variable overrides: TENKILE_SERVER_PORT=9090, TENKILE_DATABASE_DRIVER=postgres, etc.
   - Add validation on startup (check paths exist, FFmpeg available, etc.)
   - Auto-generate JWT secret on first run, persist to config file

5. .dockerignore:
   .git
   jellyfin/
   codecprobe/
   docs/
   *.md
   dist/
   node_modules/

Tests:
  - Config loading: YAML file, env overrides, defaults
  - Config validation: missing library paths, invalid driver, bad port
  - Docker build: verify image builds and binary starts (CI integration test)

After completing 4.7, run: go build ./... && go test ./... && docker build -t tenkile:latest .
```

### Phase 4 Checkpoint: Git Commit

After completing all Phase 4 tasks (4.1–4.7), create a git commit:

```bash
git add -A && git diff --cached --stat  # Review what's staged before committing
git commit -m "Phase 4: Media library, streaming, web UI & deployment

Implement media scanner, HLS streaming, OpenAPI spec, auth/first-run setup,
Preact web client, WebSocket events, and Docker deployment.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
git push origin main
```

---

## Phase 5: Client Adapters + Native Probes

### Agent Prompt 5.1: Platform-Specific Probe Libraries

```
Build probe libraries that use platform-native APIs instead of browser APIs.
These provide higher trust (0.85) than browser-based CodecProbe (0.50-0.80).

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

### Phase 5 Checkpoint: Git Commit

After completing all Phase 5 tasks (5.1), create a git commit:

```bash
git add -A && git diff --cached --stat  # Review what's staged before committing
git commit -m "Phase 5: Client adapters + native probe stubs

Implement client adapter interface, web adapter, and platform-specific
probe library stubs (Android, iOS/tvOS).

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
git push origin main
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
| 5.1 | Native probe stubs | 1.3, 2.2 | Client adapter interface, platform stubs |

## Quick Reference

| What | Where |
|------|-------|
| Architecture spec | `../docs/ARCHITECTURE.md` |
| Language rationale | `../docs/LANGUAGE_AND_SCALABILITY.md` |
| Web UI architecture | `../docs/WEB_UI.md` |
| OpenAPI spec | `api/openapi.yaml` (created in prompt 4.3) |
| Default config | `configs/tenkile.yaml` (created in prompt 4.7) |
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
