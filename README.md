# Tenkile

[![Phase](https://img.shields.io/badge/Phase-3%20Complete-brightgreen)](AGENTS.md)
[![AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue)](LICENSE)

A modern, self-hosted media server with intelligent device probing, adaptive transcoding, and curated device support.

## What's New in Phase 3

Phase 3 introduces **Curated Device Database** and **Playback Feedback Loop** for improved trust-based capability resolution.

### Phase 3.1: Curated Smart TV Database
- **Pre-configured device profiles** for Samsung Tizen, LG WebOS, Roku, and Android TV
- **Fuzzy matching** to identify devices from user agent strings
- **Version-aware matching** for firmware-specific capabilities
- **Embedded device bundles** compiled into the binary for zero-config setup
- **Community voting** on device profile accuracy

### Phase 3.2: Playback Feedback Loop
- **Automatic trust adjustment** based on playback success/failure
- **Re-probe triggers** when devices consistently fail playback
- **Per-codec reliability tracking** to identify problematic codecs
- **Global playback metrics** with Prometheus export support

## Features

- **Device Probing**: Automatically detect and profile connected devices for optimal playback
- **Adaptive Transcoding**: Real-time video/audio transcoding based on device capabilities
- **Curated Device Database**: Pre-configured profiles for popular media devices
- **Multi-Format Support**: HLS, DASH, and direct play streaming
- **Authentication**: JWT-based auth with API key and OAuth support
- **Extensible**: Plugin architecture for custom codecs and device profiles

## Quick Start

### Prerequisites

- Go 1.24 or later
- SQLite 3 or PostgreSQL 14+
- Node.js 18+ (for web interface)

### Installation

```bash
# Clone the repository
git clone https://github.com/tenkile/tenkile.git
cd tenkile

# Install dependencies
go mod tidy

# Run migrations
make migrate-up

# Start the server
make dev
```

### Configuration

Create `configs/tenkile.yaml` with your settings:

```yaml
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  driver: sqlite
  dsn: ./data/tenkile.db

libraries:
  - name: Movies
    path: /media/movies
  - name: TV Shows
    path: /media/tv
```

## Project Structure

```
tenkile/
├── cmd/tenkile/          # Application entry point
├── internal/
│   ├── api/              # HTTP router and middleware
│   ├── clients/          # External service clients
│   ├── config/           # Configuration management
│   ├── database/         # Database layer and migrations
│   ├── probes/           # Device probing logic
│   ├── server/           # HTTP server implementation
│   └── transcode/        # Transcoding engine
├── pkg/
│   └── codec/            # Codec definitions and utilities
├── web/
│   └── probe/            # Client-side probing scripts
├── configs/              # Configuration files
├── api/                  # API specifications
└── data/                 # Runtime data storage
```

## API Documentation

OpenAPI specification is available at `/api/v1/openapi.yaml` or view the generated docs at `/api/v1/docs`.

### Curated Device Management (Phase 3.1)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/admin/curated/devices` | PUT | Create curated device |
| `/api/v1/admin/curated/devices/{id}` | PUT | Replace curated device |
| `/api/v1/admin/curated/devices/{id}` | DELETE | Delete curated device |
| `/api/v1/admin/curated/devices/{id}/vote` | POST | Vote on device accuracy |
| `/api/v1/admin/curated/search` | POST | Fuzzy search devices |
| `/api/v1/admin/curated/version-match` | POST | Version-aware matching |
| `/api/v1/admin/curated/embedded/stats` | GET | Embedded bundle statistics |
| `/api/v1/admin/curated/embedded/sync` | POST | Sync embedded to curated DB |
| `/api/v1/admin/curated/export` | GET | Export devices (JSON) |
| `/api/v1/admin/curated/import` | POST | Import devices (JSON) |

### Playback Feedback Loop (Phase 3.2)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/devices/{id}/feedback` | POST | Submit playback feedback |
| `/api/v1/devices/{id}/feedback/stats` | GET | Get playback statistics |
| `/api/v1/devices/{id}/reliable-codecs` | GET | Get reliable codecs for device |
| `/api/v1/devices/{id}/reprobe` | POST | Trigger re-probe |
| `/api/v1/devices/{id}/trust` | GET | Get trust report |
| `/api/v1/feedback/metrics` | GET | Get feedback metrics |

### Trust Adjustment (Phase 3.2)

Playback outcomes automatically adjust device trust scores:

| Outcome | Trust Delta | Severity |
|---------|-------------|----------|
| `success` | +0.01 | — |
| `codec_error` | -0.15 | Significant |
| `decoding_failed` | -0.25 | Major |
| `renderer_crash` | -0.30 | Severe |
| `network_error` | -0.05 | Minor |
| `buffering` | -0.025 | Minor |

Re-probe triggers: 3+ consecutive failures or >50% failure rate in last 10+ playbacks.

## License

AGPL-3.0-or-later - See [LICENSE](LICENSE) for details.
