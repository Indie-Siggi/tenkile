# Tenkile

A modern, self-hosted media server with intelligent device probing, adaptive transcoding, and curated device support.

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

## License

AGPL-3.0-or-later - See [LICENSE](LICENSE) for details.
