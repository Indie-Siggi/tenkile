# Tenkile

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0--or--later-blue.svg)](LICENSE)
[![Go 1.24+](https://img.shields.io/badge/Go-1.24+-00ADD8.svg)](https://go.dev)
[![Phase 7 Complete](https://img.shields.io/badge/Phase-7%20Complete-brightgreen)](AGENTS.md)

A standalone, self-hosted media server built from scratch in Go. Tenkile replaces Jellyfin with a cleaner architecture, trust-based device probing, and a quality-preserving transcode engine — all in a single binary under 15 MB.

## Why Tenkile?

Media servers guess what your devices can play. They rely on static XML profiles that are constantly stale, can't adapt to firmware updates, and treat all capability claims equally. When they guess wrong, you get unnecessary transcoding, stuttering playback, or silent quality loss.

**Tenkile takes a different approach:**

- **Trust-scored device probing** — Multiple probe sources (native APIs, CodecProbe, curated DB, playback feedback) each with a trust score. Playback success/failure adjusts trust over time. The system learns which devices actually support which codecs.
- **Server-aware transcoding** — Never picks a transcode target the server can't encode. Inventories FFmpeg capabilities and hardware acceleration at startup.
- **Quality preservation** — HDR stays HDR on HDR displays. High-quality tone-mapping to SDR on non-HDR displays. Surround audio channels are preserved, not collapsed to stereo.
- **Single binary deployment** — Web client, migrations, curated device DB, and default config all embedded via `go:embed`. Runs on a Raspberry Pi with <50 MB memory.

## Quick Start

### From Source

```bash
git clone https://github.com/Indie-Siggi/tenkile.git
cd tenkile

# Build and run (web client + Go binary)
make build
./tenkile serve

# Or use dev mode with hot reload
make dev
```

Open `http://localhost:8765` to access the web interface. First launch prompts you to create an admin account.

### Docker

```bash
docker run -d \
  --name tenkile \
  -p 8765:8765 \
  -v /path/to/media:/media \
  -v /path/to/data:/data \
  ghcr.io/indie-siggi/tenkile:latest
```

### Docker Compose

```yaml
services:
  tenkile:
    image: ghcr.io/indie-siggi/tenkile:latest
    ports:
      - "8765:8765"
    volumes:
      - /path/to/media:/media
      - tenkile-data:/data
    restart: unless-stopped

volumes:
  tenkile-data:
```

## Configuration

Tenkile uses YAML configuration with environment variable overrides (`TENKILE_` prefix).

```yaml
server:
  host: 0.0.0.0
  port: 8765

database:
  driver: sqlite           # sqlite or postgres
  dsn: ./data/tenkile.db

libraries:
  - name: Movies
    path: /media/movies
  - name: TV Shows
    path: /media/tv

transcode:
  ffmpeg_path: /usr/bin/ffmpeg
  hardware_accel: auto     # auto, nvenc, vaapi, videotoolbox, none
  max_concurrent: 3
```

All settings can be overridden with environment variables:
```bash
TENKILE_SERVER_PORT=9000
TENKILE_DATABASE_DRIVER=postgres
TENKILE_TRANSCODE_MAX_CONCURRENT=5
```

## Architecture

```
                 ┌─────────────┐
                 │  Web Client  │  Preact + Vite + TypeScript
                 │  (embedded)  │  CodecProbe integration
                 └──────┬──────┘
                        │
                 ┌──────▼──────┐
                 │   REST API   │  chi router, JWT auth, OpenAPI 3.1
                 │  WebSocket   │  Real-time events (scan, transcode, playback)
                 └──────┬──────┘
                        │
          ┌─────────────┼─────────────┐
          │             │             │
   ┌──────▼──────┐ ┌───▼────┐ ┌─────▼──────┐
   │   Probes    │ │ Media  │ │  Transcode  │
   │             │ │ Library│ │   Engine    │
   │ Trust scores│ │ Scanner│ │ Orchestrator│
   │ Curated DB  │ │ FFprobe│ │ FFmpeg      │
   │ Feedback    │ │ Store  │ │ HW accel    │
   │ Validation  │ │        │ │ HLS output  │
   └─────────────┘ └────────┘ └─────────────┘
          │             │             │
          └─────────────┼─────────────┘
                        │
                 ┌──────▼──────┐
                 │   Database   │  SQLite (default) / PostgreSQL
                 │  + Goose     │  Embedded migrations
                 └─────────────┘
```

### How Trust-Based Probing Works

When a device connects, Tenkile gathers capability data from multiple sources and weights them by trust:

| Source | Trust Score | Method |
|--------|-----------|--------|
| Playback feedback | 1.00 | Actual success/failure history |
| Curated device DB | 0.90 | Verified profiles for Smart TVs, set-top boxes |
| Native probes | 0.85 | Android MediaCodec, Apple AVFoundation |
| CodecProbe | 0.60 | Browser-based codec detection |
| Static profiles | 0.30 | Fallback for unknown devices |

Trust adjusts dynamically: successful playback increases trust (+0.01), codec errors decrease it (-0.15), decoder crashes decrease it severely (-0.30). After repeated failures, the system triggers a re-probe.

### Transcode Decision Flow

```
Direct Play? ──yes──▶ Stream as-is (zero CPU cost)
     │ no
     ▼
Remux only? ───yes──▶ Repackage container (near-zero CPU)
     │ no
     ▼
Transcode ──────────▶ Encode to device-compatible format
     │                 (HDR preserved or tone-mapped)
     │ server can't encode?
     ▼
Fallback ───────────▶ H.264 + AAC in MP4 (universal)
```

## Project Structure

```
tenkile/
├── cmd/tenkile/           # Entry point
├── internal/
│   ├── api/               # REST API handlers, auth, middleware, security headers
│   │   └── auth/          # Brute force protection, password policy, audit logging
│   ├── circuitbreaker/    # Circuit breaker for FFmpeg/FFprobe/transcode
│   ├── clients/           # Platform adapters (web, Android, Apple)
│   │   ├── android/       # MediaCodec probe adapter
│   │   └── apple/         # AVFoundation probe adapter (iOS + tvOS)
│   ├── config/            # YAML config + env var overrides
│   ├── database/          # SQLite/PostgreSQL + goose migrations
│   ├── events/            # Event bus + WebSocket broadcasting
│   ├── media/             # Library scanning, FFprobe metadata extraction
│   ├── metrics/           # Prometheus metrics, health checks, segment cache
│   ├── probes/            # Trust scoring, curated DB, feedback loop, validation
│   ├── server/            # Server encoding inventory, HW accel detection
│   ├── stream/            # HLS segmenter, segment delivery
│   └── transcode/         # Decision engine, FFmpeg command builder, quality policy
├── pkg/codec/             # Shared codec/container definitions
├── web/                   # Preact + Vite SPA (embedded in binary)
├── api/openapi.yaml       # OpenAPI 3.1 spec
├── configs/               # Default YAML config
├── data/curated/          # Curated Smart TV database
├── Dockerfile             # Multi-stage: node + go -> scratch (~15 MB)
└── AGENTS.md              # Development phases and agent prompts
```

## API

Full OpenAPI 3.1 spec at `api/openapi.yaml`. Key endpoints:

| Endpoint | Description |
|----------|-------------|
| `POST /api/v1/auth/login` | JWT authentication |
| `GET /api/v1/libraries` | List media libraries |
| `POST /api/v1/libraries/{id}/scan` | Trigger library scan |
| `GET /api/v1/libraries/{id}/items` | Browse media (paginated) |
| `GET /api/v1/stream/{id}/hls` | Start HLS stream |
| `POST /api/v1/probe/report` | Submit device probe results |
| `POST /api/v1/devices/{id}/feedback` | Report playback outcome |
| `GET /api/v1/devices/{id}/trust` | Get device trust report |
| `GET /health/ready` | Readiness probe (FFmpeg, DB) |
| `GET /health/detailed` | Full subsystem health + metrics |

## Development

```bash
make build        # Build web client + Go binary
make dev          # Dev mode: Vite HMR + Go hot reload (air)
make test         # Run all tests with race detector
make lint         # golangci-lint
make generate     # Regenerate code from OpenAPI spec
make docker       # Build Docker image
make migrate-up   # Run database migrations
make release      # Cross-compile for all platforms
```

Run a single test:
```bash
go test ./internal/transcode/ -run TestName
```

## Roadmap

| Phase | Status | Description |
|-------|--------|-------------|
| 1. Foundation | Done | Codec types, probe system, API scaffold |
| 2. Server Inventory | Done | FFmpeg discovery, transcode decision engine |
| 3. Curated DB | Done | Smart TV database, playback feedback loop |
| 4. Media Library | Done | Scanner, HLS streaming, auth, web client, WebSocket |
| 5. Client Adapters | Done | Android MediaCodec, Apple AVFoundation probes |
| 6. Production | Done | Performance, circuit breakers, Prometheus metrics, caching |
| 7. Security | Done | Auth hardening, input validation, security headers, audit |
| 8. Metadata | Planned | TMDB, TVDB, MusicBrainz integration |
| 9. Multi-User | Planned | User profiles, permissions, watch history, parental controls |
| 10. Subtitles | Planned | OpenSubtitles search, auto-match, sync |

See [AGENTS.md](AGENTS.md) for detailed phase specifications and agent prompts.

## Compared to Jellyfin

Tenkile is not a fork — it's a ground-up rewrite solving the same problem differently.

| Aspect | Jellyfin | Tenkile |
|--------|----------|---------|
| Language | C# / .NET 10 | Go 1.24 |
| Binary size | ~200 MB + runtime | <15 MB single binary |
| Memory usage | ~300 MB+ | <50 MB |
| Device profiles | Static XML files | Trust-scored multi-source probing |
| Transcode decision | 2,400-line monolith | Decomposed engine (8 focused files) |
| Feedback loop | None | Playback outcomes adjust trust scores |
| Codec lookup | O(n) linear scan | O(1) hash map indexes |
| Default port | 8096 | 8765 |

## License

[AGPL-3.0-or-later](LICENSE) — Required because Tenkile embeds [CodecProbe](https://github.com/nicknsy/CodecProbe) (AGPL-3.0 fork) for browser-based codec detection.

If you modify Tenkile and provide it as a network service, you must make your source code available under the same license (AGPL section 13).
