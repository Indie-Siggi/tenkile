# Stage 1: Web builder
FROM node:20-alpine AS web-builder
WORKDIR /build
COPY web/package*.json ./
RUN npm ci 2>/dev/null || echo "No web dependencies"
COPY web/ ./
RUN npm run build 2>/dev/null || echo "Web build skipped"

# Stage 2: Go builder
FROM golang:1.24-alpine AS go-builder
WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT
RUN CGO_ENABLED=1 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o tenkile \
    ./cmd/tenkile

# Stage 3: Final scratch image
FROM scratch AS final

# Copy CA certificates for HTTPS
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary from builder
COPY --from=go-builder /build/tenkile /tenkile

# Copy web assets
COPY --from=web-builder /build/dist /web/dist

# Copy configs
COPY configs/ /etc/tenkile/

# Copy migrations
COPY internal/database/migrations/ /etc/tenkile/migrations/

# Create data directory
RUN mkdir -p /data

# Expose port
EXPOSE 8765

# Set working directory
WORKDIR /data

# Run the application
ENTRYPOINT ["/tenkile"]
CMD ["serve"]
