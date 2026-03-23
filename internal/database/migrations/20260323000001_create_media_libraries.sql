-- Migration: 20260323000001_create_media_libraries
-- Description: Create libraries table for media library management

CREATE TABLE IF NOT EXISTS libraries (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    library_type TEXT NOT NULL CHECK (library_type IN ('movie', 'tv', 'music')),
    enabled INTEGER NOT NULL DEFAULT 1,
    refresh_interval_minutes INTEGER NOT NULL DEFAULT 60,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_scan_at DATETIME
);

CREATE INDEX idx_libraries_type ON libraries(library_type);
CREATE INDEX idx_libraries_enabled ON libraries(enabled);
