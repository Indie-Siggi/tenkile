-- Migration: 20260323000004_create_streams_sessions
-- Description: Create active_streams and stream_sessions tables for playback tracking

CREATE TABLE IF NOT EXISTS active_streams (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    media_item_id TEXT NOT NULL,
    
    -- Playback position
    position_ms INTEGER NOT NULL DEFAULT 0,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Stream params
    stream_type TEXT NOT NULL CHECK (stream_type IN ('direct', 'remux', 'transcode', 'hls', 'dash')),
    selected_audio_index INTEGER,
    selected_subtitle_index INTEGER,
    
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (media_item_id) REFERENCES media_items(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS stream_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at DATETIME,
    total_bytes_served INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_streams_session ON active_streams(session_id);
CREATE INDEX idx_streams_media ON active_streams(media_item_id);
CREATE INDEX idx_streams_user ON active_streams(user_id);
