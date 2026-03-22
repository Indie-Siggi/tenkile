.PHONY: build build-web build-go dev test lint generate migrate-up migrate-new docker release clean

# Application name
APP_NAME := tenkile

# Go parameters
GO := go
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Default target
all: build

# Build the application
build: build-go build-web

build-go:
	@echo "Building Go application..."
	$(GO) build $(LDFLAGS) -o dist/$(APP_NAME) ./cmd/tenkile

build-web:
	@echo "Building web assets..."
	@if [ -d "web" ]; then \
		cd web && npm run build 2>/dev/null || echo "Web build skipped (no package.json or npm)"; \
	fi

dev:
	@echo "Starting development server with hot reload..."
	air

test:
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

generate:
	@echo "Running code generation..."
	$(GO) generate ./...

migrate-up:
	@echo "Running all pending migrations..."
	$(GO) run ./cmd/tenkile migrate up

migrate-new:
	@echo "Usage: make migrate-new NAME=your_migration_name"
	@if [ -z "$(NAME)" ]; then \
		echo "Please provide a migration name: make migrate-new NAME=your_migration_name"; \
		exit 1; \
	fi
	goose -dir internal/database/migrations create $(NAME) sql

docker:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) .

release:
	@echo "Creating release build..."
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o dist/$(APP_NAME)-linux-amd64 ./cmd/tenkile
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o dist/$(APP_NAME)-darwin-arm64 ./cmd/tenkile
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o dist/$(APP_NAME)-windows-amd64.exe ./cmd/tenkile

clean:
	@echo "Cleaning build artifacts..."
	rm -rf dist/
	rm -rf data/*.db
	rm -rf *.log
	$(GO) clean
