-- Migration: 20260322120000_init
-- Description: Initial schema for Tenkile media server

-- Device registration and tracking
CREATE TABLE IF NOT EXISTS devices (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    model TEXT,
    manufacturer TEXT,
    platform TEXT NOT NULL,
    os_version TEXT,
    app_version TEXT,
    first_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip_address TEXT,
    user_agent TEXT,
    is_trusted BOOLEAN DEFAULT FALSE,
    trust_score REAL DEFAULT 0.0,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Device codec and format capabilities
CREATE TABLE IF NOT EXISTS device_capabilities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    video_codecs JSONB NOT NULL DEFAULT '[]',
    audio_codecs JSONB NOT NULL DEFAULT '[]',
    subtitle_formats JSONB NOT NULL DEFAULT '[]',
    container_formats JSONB NOT NULL DEFAULT '[]',
    max_resolution TEXT,
    max_bitrate INTEGER,
    supports_hdr BOOLEAN DEFAULT FALSE,
    supports_dolby_vision BOOLEAN DEFAULT FALSE,
    supports_dolby_atmos BOOLEAN DEFAULT FALSE,
    supports_dts BOOLEAN DEFAULT FALSE,
    direct_play_support JSONB,
    transcoding_preferences JSONB,
    detected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(device_id, detected_at)
);

-- Playback decision history
CREATE TABLE IF NOT EXISTS playback_decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    media_id TEXT NOT NULL,
    media_type TEXT NOT NULL CHECK (media_type IN ('video', 'audio', 'subtitle')),
    decision TEXT NOT NULL CHECK (decision IN ('direct_play', 'transcode_video', 'transcode_audio', 'transcode_container', 'transcode_all')),
    selected_profile TEXT,
    video_codec TEXT,
    audio_codec TEXT,
    container TEXT,
    bitrate INTEGER,
    resolution TEXT,
    reason TEXT,
    duration_ms INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- User feedback on playback quality
CREATE TABLE IF NOT EXISTS playback_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    playback_id INTEGER NOT NULL REFERENCES playback_decisions(id) ON DELETE CASCADE,
    media_id TEXT NOT NULL,
    quality_rating INTEGER CHECK (quality_rating >= 1 AND quality_rating <= 5),
    had_buffering BOOLEAN DEFAULT FALSE,
    buffering_count INTEGER DEFAULT 0,
    total_buffering_time_ms INTEGER DEFAULT 0,
    had_errors BOOLEAN DEFAULT FALSE,
    error_type TEXT,
    had_dropouts BOOLEAN DEFAULT FALSE,
    submitted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Curated device profiles from community/trusted sources
CREATE TABLE IF NOT EXISTS curated_devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    manufacturer TEXT,
    model TEXT,
    platform TEXT NOT NULL,
    capabilities JSONB NOT NULL,
    recommended_profile TEXT,
    known_issues JSONB,
    source TEXT NOT NULL CHECK (source IN ('community', 'official', 'curated')),
    votes_up INTEGER DEFAULT 0,
    votes_down INTEGER DEFAULT 0,
    verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- User accounts
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT,
    avatar_url TEXT,
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user', 'viewer')),
    is_active BOOLEAN DEFAULT TRUE,
    last_login TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- API keys for programmatic access
CREATE TABLE IF NOT EXISTS api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    permissions JSONB NOT NULL DEFAULT '[]',
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- OAuth refresh tokens
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    device_id TEXT REFERENCES devices(id) ON DELETE SET NULL,
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_devices_platform ON devices(platform);
CREATE INDEX IF NOT EXISTS idx_devices_trusted ON devices(is_trusted);
CREATE INDEX IF NOT EXISTS idx_device_capabilities_device ON device_capabilities(device_id);
CREATE INDEX IF NOT EXISTS idx_playback_decisions_device ON playback_decisions(device_id);
CREATE INDEX IF NOT EXISTS idx_playback_decisions_media ON playback_decisions(media_id);
CREATE INDEX IF NOT EXISTS idx_playback_feedback_device ON playback_feedback(device_id);
CREATE INDEX IF NOT EXISTS idx_playback_feedback_playback ON playback_feedback(playback_id);
CREATE INDEX IF NOT EXISTS idx_curated_devices_hash ON curated_devices(device_hash);
CREATE INDEX IF NOT EXISTS idx_curated_devices_platform ON curated_devices(platform);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
