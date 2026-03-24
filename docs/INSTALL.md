# Installation Guide

This guide covers installing Tenkile on Linux, macOS, Windows, Docker, and building from source.

## System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 1 core (direct play only) | 4+ cores (transcoding) |
| RAM | 256 MB | 1 GB+ (concurrent transcoding) |
| Disk | 100 MB (binary + DB) | Depends on media library size |
| FFmpeg | 5.0+ | 7.0+ (AV1, HDR tone-mapping) |
| Go | 1.24+ (build from source only) | — |
| Node.js | 18+ (build from source only) | — |

**Supported platforms:** Linux (amd64, arm64), macOS (arm64, amd64), Windows (amd64)

## Option 1: Docker (Recommended)

Docker is the simplest way to run Tenkile. FFmpeg is included in the image.

### Quick Start

```bash
docker run -d \
  --name tenkile \
  -p 8765:8765 \
  -v /path/to/media:/media:ro \
  -v tenkile-data:/data \
  ghcr.io/indie-siggi/tenkile:latest
```

Open `http://localhost:8765` and create your admin account.

### Docker Compose

Create a `docker-compose.yml`:

```yaml
services:
  tenkile:
    image: ghcr.io/indie-siggi/tenkile:latest
    container_name: tenkile
    ports:
      - "8765:8765"
    volumes:
      - /path/to/movies:/media/movies:ro
      - /path/to/tv:/media/tv:ro
      - /path/to/music:/media/music:ro
      - tenkile-data:/data
      - ./tenkile.yaml:/etc/tenkile/tenkile.yaml:ro  # optional custom config
    environment:
      - TENKILE_AUTH_JWT_SECRET=change-me-to-a-random-string
    restart: unless-stopped

volumes:
  tenkile-data:
```

```bash
docker compose up -d
```

### Hardware Transcoding in Docker

**NVIDIA GPU:**
```yaml
services:
  tenkile:
    image: ghcr.io/indie-siggi/tenkile:latest
    runtime: nvidia
    environment:
      - NVIDIA_VISIBLE_DEVICES=all
      - TENKILE_TRANSCODE_HARDWARE_ACCEL=nvenc
    # ... rest of config
```

**Intel Quick Sync (VAAPI):**
```yaml
services:
  tenkile:
    image: ghcr.io/indie-siggi/tenkile:latest
    devices:
      - /dev/dri:/dev/dri
    environment:
      - TENKILE_TRANSCODE_HARDWARE_ACCEL=vaapi
    # ... rest of config
```

**Raspberry Pi (V4L2):**
```yaml
services:
  tenkile:
    image: ghcr.io/indie-siggi/tenkile:latest
    devices:
      - /dev/video10:/dev/video10
      - /dev/video11:/dev/video11
      - /dev/video12:/dev/video12
    # ... rest of config
```

## Option 2: Pre-built Binary

Download the latest release for your platform from the [Releases page](https://github.com/Indie-Siggi/tenkile/releases).

### Linux (amd64)

```bash
# Download
curl -LO https://github.com/Indie-Siggi/tenkile/releases/latest/download/tenkile-linux-amd64
chmod +x tenkile-linux-amd64
sudo mv tenkile-linux-amd64 /usr/local/bin/tenkile

# Install FFmpeg (required for transcoding and media probing)
sudo apt install ffmpeg        # Debian/Ubuntu
sudo dnf install ffmpeg        # Fedora
sudo pacman -S ffmpeg          # Arch

# Create data directory
sudo mkdir -p /var/lib/tenkile
sudo mkdir -p /etc/tenkile

# Run
tenkile serve
```

### Linux (arm64 / Raspberry Pi)

```bash
curl -LO https://github.com/Indie-Siggi/tenkile/releases/latest/download/tenkile-linux-arm64
chmod +x tenkile-linux-arm64
sudo mv tenkile-linux-arm64 /usr/local/bin/tenkile

# FFmpeg on Raspberry Pi OS
sudo apt install ffmpeg

tenkile serve
```

### macOS (Apple Silicon)

```bash
curl -LO https://github.com/Indie-Siggi/tenkile/releases/latest/download/tenkile-darwin-arm64
chmod +x tenkile-darwin-arm64
mv tenkile-darwin-arm64 /usr/local/bin/tenkile

# FFmpeg via Homebrew
brew install ffmpeg

tenkile serve
```

### Windows

1. Download `tenkile-windows-amd64.exe` from the Releases page
2. Install FFmpeg: download from [gyan.dev](https://www.gyan.dev/ffmpeg/builds/) and add to PATH
3. Open a terminal and run:
   ```powershell
   .\tenkile-windows-amd64.exe serve
   ```

## Option 3: Build from Source

### Prerequisites

- Go 1.24+
- Node.js 18+ and npm (for web client)
- GCC (for SQLite CGo bindings)
- FFmpeg 5.0+ (runtime dependency)

### Build Steps

```bash
# Clone the repository
git clone https://github.com/Indie-Siggi/tenkile.git
cd tenkile

# Install Go dependencies
go mod download

# Build everything (web client + Go binary)
make build

# The binary is at dist/tenkile
./dist/tenkile serve
```

### Development Mode

```bash
# Install air for hot reload (one-time)
go install github.com/air-verse/air@latest

# Install web dependencies
cd web && npm install && cd ..

# Start dev server (Go hot reload + Vite HMR)
make dev
```

This starts the Go server with air (auto-restarts on .go file changes) and the Vite dev server with hot module replacement for the web client.

### Cross-Compile

```bash
# Build for all platforms
make release

# Output:
#   dist/tenkile-linux-amd64
#   dist/tenkile-darwin-arm64
#   dist/tenkile-windows-amd64.exe
```

## Post-Install Configuration

### First Run

1. Open `http://localhost:8765` in your browser
2. Create your admin account (the first-run wizard appears automatically)
3. Add media libraries by specifying paths to your movie/TV/music directories
4. Tenkile scans your libraries and extracts metadata via FFprobe

### Configuration File

Tenkile looks for configuration in this order:
1. `./tenkile.yaml` (current directory)
2. `./configs/tenkile.yaml`
3. `/etc/tenkile/tenkile.yaml`
4. `$HOME/.config/tenkile/tenkile.yaml`

Create a minimal config:

```yaml
server:
  host: 0.0.0.0
  port: 8765

database:
  driver: sqlite
  dsn: /var/lib/tenkile/tenkile.db

libraries:
  - name: Movies
    path: /media/movies
    type: movies
  - name: TV Shows
    path: /media/tv
    type: tv

auth:
  jwt_secret: "generate-a-random-string-here"
```

Generate a secure JWT secret:
```bash
openssl rand -base64 32
```

### Environment Variable Overrides

Any config value can be overridden with environment variables using the `TENKILE_` prefix, with underscores replacing dots and nested keys:

```bash
export TENKILE_SERVER_PORT=9000
export TENKILE_DATABASE_DRIVER=postgres
export TENKILE_DATABASE_DSN="postgres://user:pass@localhost:5432/tenkile?sslmode=disable"
export TENKILE_TRANSCODE_MAX_CONCURRENT=6
export TENKILE_AUTH_JWT_SECRET="your-secret-here"
export TENKILE_LOGGING_LEVEL=debug
```

### PostgreSQL (Multi-Instance)

SQLite is the default and works well for single-instance deployments. For multiple Tenkile instances sharing a database, use PostgreSQL:

```bash
# Create the database
createdb tenkile

# Configure Tenkile
export TENKILE_DATABASE_DRIVER=postgres
export TENKILE_DATABASE_DSN="postgres://user:pass@localhost:5432/tenkile?sslmode=disable"
```

Migrations run automatically on startup.

### TLS / HTTPS

For direct TLS termination (without a reverse proxy):

```yaml
server:
  tls:
    enabled: true
    cert_path: /etc/tenkile/cert.pem
    key_path: /etc/tenkile/key.pem
```

For most deployments, use a reverse proxy (nginx, Caddy, Traefik) instead and let it handle TLS.

### Reverse Proxy (nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name media.example.com;

    ssl_certificate     /etc/letsencrypt/live/media.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/media.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8765;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Large media uploads
        client_max_body_size 100M;
    }

    # HLS segments — allow range requests
    location /api/v1/stream/ {
        proxy_pass http://127.0.0.1:8765;
        proxy_buffering off;
        proxy_request_buffering off;
    }
}
```

### Reverse Proxy (Caddy)

```
media.example.com {
    reverse_proxy localhost:8765
}
```

Caddy handles TLS automatically via Let's Encrypt.

## Running as a System Service

### systemd (Linux)

Create `/etc/systemd/system/tenkile.service`:

```ini
[Unit]
Description=Tenkile Media Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=tenkile
Group=tenkile
ExecStart=/usr/local/bin/tenkile serve
WorkingDirectory=/var/lib/tenkile
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/tenkile
PrivateTmp=true

# Environment
Environment=TENKILE_AUTH_JWT_SECRET=your-secret-here

[Install]
WantedBy=multi-user.target
```

```bash
# Create service user
sudo useradd -r -s /usr/sbin/nologin -d /var/lib/tenkile tenkile
sudo mkdir -p /var/lib/tenkile
sudo chown tenkile:tenkile /var/lib/tenkile

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable tenkile
sudo systemctl start tenkile

# Check status
sudo systemctl status tenkile
sudo journalctl -u tenkile -f
```

### launchd (macOS)

Create `~/Library/LaunchAgents/com.tenkile.server.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.tenkile.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/tenkile</string>
        <string>serve</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>WorkingDirectory</key>
    <string>/usr/local/var/tenkile</string>
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/tenkile.log</string>
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/tenkile.error.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>TENKILE_AUTH_JWT_SECRET</key>
        <string>your-secret-here</string>
    </dict>
</dict>
</plist>
```

```bash
mkdir -p /usr/local/var/tenkile
launchctl load ~/Library/LaunchAgents/com.tenkile.server.plist
```

## Firewall

Tenkile uses a single port for all traffic (HTTP, WebSocket, HLS streaming):

| Port | Protocol | Purpose |
|------|----------|---------|
| 8765 | TCP | Web UI, REST API, WebSocket, HLS streams |

If you're behind a firewall:
```bash
# ufw (Ubuntu)
sudo ufw allow 8765/tcp

# firewalld (Fedora/RHEL)
sudo firewall-cmd --permanent --add-port=8765/tcp
sudo firewall-cmd --reload

# iptables
sudo iptables -A INPUT -p tcp --dport 8765 -j ACCEPT
```

## Health Checks

Tenkile exposes health endpoints for monitoring and orchestration:

| Endpoint | Purpose | Use For |
|----------|---------|---------|
| `GET /health/live` | Is the process running? | Kubernetes liveness probe |
| `GET /health/ready` | Are FFmpeg, FFprobe, and DB available? | Kubernetes readiness probe |
| `GET /health/detailed` | Full subsystem status + metrics | Monitoring dashboards |

Example Kubernetes probes:
```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 8765
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health/ready
    port: 8765
  initialDelaySeconds: 10
  periodSeconds: 15
```

## Updating

### Docker

```bash
docker compose pull
docker compose up -d
```

### Binary

Download the new release binary and replace the old one:
```bash
sudo systemctl stop tenkile
sudo cp tenkile-linux-amd64 /usr/local/bin/tenkile
sudo systemctl start tenkile
```

Database migrations run automatically on startup. No manual migration step needed.

## Uninstalling

### Docker

```bash
docker compose down -v   # -v removes the data volume too
```

### Binary (Linux)

```bash
sudo systemctl stop tenkile
sudo systemctl disable tenkile
sudo rm /etc/systemd/system/tenkile.service
sudo systemctl daemon-reload
sudo rm /usr/local/bin/tenkile
sudo rm -rf /var/lib/tenkile    # removes database and transcoding cache
sudo rm -rf /etc/tenkile        # removes configuration
sudo userdel tenkile
```

## Troubleshooting

### Tenkile won't start

```bash
# Check if the port is in use
lsof -i :8765

# Run with debug logging
TENKILE_LOGGING_LEVEL=debug tenkile serve

# Check systemd logs
journalctl -u tenkile -n 50 --no-pager
```

### FFmpeg not found

Tenkile needs FFmpeg and FFprobe in PATH. Verify:
```bash
ffmpeg -version
ffprobe -version
```

If installed in a non-standard location:
```bash
export PATH="/path/to/ffmpeg/bin:$PATH"
```

### Permission denied on media files

The user running Tenkile must have read access to media directories:
```bash
# Check permissions
ls -la /media/movies/

# If using systemd, add the tenkile user to the media group
sudo usermod -aG media tenkile
sudo systemctl restart tenkile
```

### Database locked (SQLite)

If you see "database is locked" errors, ensure only one Tenkile instance is running against the same SQLite database. For multiple instances, switch to PostgreSQL.

### WebSocket connection failed

If using a reverse proxy, ensure WebSocket upgrade headers are forwarded. See the nginx and Caddy examples above.
