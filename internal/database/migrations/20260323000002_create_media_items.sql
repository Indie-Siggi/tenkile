-- Migration: 20260323000002_create_media_items
-- Description: Create media_items table for storing media file metadata

CREATE TABLE IF NOT EXISTS media_items (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    year INTEGER,
    overview TEXT,
    poster_path TEXT,
    
    -- Video stream info (JSONB for flexibility)
    video_stream_json TEXT,
    
    -- Audio streams (JSONB array)
    audio_streams_json TEXT,
    
    -- Subtitle streams (JSONB array)
    subtitle_streams_json TEXT,
    
    -- Container
    container TEXT NOT NULL,
    duration REAL NOT NULL DEFAULT 0,
    
    -- File metadata
    file_size INTEGER NOT NULL DEFAULT 0,
    file_modified_at DATETIME NOT NULL,
    file_hash TEXT NOT NULL, -- xxhash for change detection
    
    -- Metadata timestamps
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata_fetched_at DATETIME,
    
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_items_library ON media_items(library_id);
CREATE INDEX idx_media_items_path ON media_items(path);
CREATE INDEX idx_media_items_title ON media_items(title);
CREATE INDEX idx_media_items_year ON media_items(year);
CREATE INDEX idx_media_items_hash ON media_items(file_hash);
