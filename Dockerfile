# Stage 1: Web builder
FROM node:20-alpine AS web-builder
WORKDIR /build
COPY web/package*.json ./
RUN npm ci || echo "No web dependencies"
COPY web/ ./
RUN npm run build || echo "Web build skipped"

# Stage 2: Go builder
FROM golang:1.25-alpine AS go-builder
WORKDIR /build

# Install build dependencies (git only — modernc.org/sqlite is pure Go, no CGO needed)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Create data directory for scratch stage (scratch has no shell)
RUN mkdir -p /data

# Build the application (CGO disabled — all deps are pure Go)
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT
RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o tenkile \
    ./cmd/tenkile

# Stage 3: Test (use with: docker build --target test .)
FROM go-builder AS test
RUN CGO_ENABLED=0 go test -v -race ./...

# Stage 4: Final scratch image
FROM scratch AS final

# Copy CA certificates for HTTPS
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary from builder
COPY --from=go-builder /build/tenkile /tenkile

# Copy web assets
COPY --from=web-builder /build/dist /web/dist

# Copy configs from builder stage (not build context)
COPY --from=go-builder /build/configs/ /etc/tenkile/

# Copy migrations from builder stage
COPY --from=go-builder /build/internal/database/migrations/ /etc/tenkile/migrations/

# Copy pre-created data directory from builder
COPY --from=go-builder /data /data

# Expose port
EXPOSE 8765

# Set working directory
WORKDIR /data

# Run the application with config path matching Docker layout
ENTRYPOINT ["/tenkile"]
CMD ["serve", "-config", "/etc/tenkile/tenkile.yaml"]
