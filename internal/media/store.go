package media

import (
    "context"
    "database/sql"
    "encoding/json"
    "time"
)

// Store handles media library persistence
type Store struct {
    db *sql.DB
}

// NewStore creates a new media store
func NewStore(db *sql.DB) *Store {
    return &Store{db: db}
}

// SaveMediaItem saves or updates a media item
func (s *Store) SaveMediaItem(ctx context.Context, item *MediaItem) error {
    // Marshal streams to JSON
    videoJSON, _ := json.Marshal(item.VideoStream)
    audioJSON, _ := json.Marshal(item.AudioStreams)
    subtitleJSON, _ := json.Marshal(item.SubtitleStreams)

    query := `
    INSERT INTO media_items (
        id, library_id, path, title, year, overview, poster_path,
        video_stream_json, audio_streams_json, subtitle_streams_json,
        container, duration, file_size, file_modified_at, file_hash,
        created_at, updated_at
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(id) DO UPDATE SET
        title = excluded.title,
        year = excluded.year,
        overview = excluded.overview,
        poster_path = excluded.poster_path,
        video_stream_json = excluded.video_stream_json,
        audio_streams_json = excluded.audio_streams_json,
        subtitle_streams_json = excluded.subtitle_streams_json,
        container = excluded.container,
        duration = excluded.duration,
        file_size = excluded.file_size,
        file_modified_at = excluded.file_modified_at,
        file_hash = excluded.file_hash,
        updated_at = CURRENT_TIMESTAMP
    `

    now := time.Now()
    if item.CreatedAt.IsZero() {
        item.CreatedAt = now
    }
    item.UpdatedAt = now

    _, err := s.db.ExecContext(ctx, query,
        item.ID, item.LibraryID, item.Path, item.Title, item.Year, item.Overview, item.PosterPath,
        string(videoJSON), string(audioJSON), string(subtitleJSON),
        item.Container, item.Duration, item.FileSize, item.FileModifiedAt, item.FileHash,
        item.CreatedAt, item.UpdatedAt,
    )

    return err
}

// GetMediaItem retrieves a media item by ID
func (s *Store) GetMediaItem(ctx context.Context, id string) (*MediaItem, error) {
    query := `
    SELECT id, library_id, path, title, year, overview, poster_path,
           video_stream_json, audio_streams_json, subtitle_streams_json,
           container, duration, file_size, file_modified_at, file_hash,
           created_at, updated_at, metadata_fetched_at
    FROM media_items WHERE id = ?
    `

    var item MediaItem
    var videoJSON, audioJSON, subtitleJSON sql.NullString
    var year, fileSize sql.NullInt64
    var duration sql.NullFloat64
    var overview, posterPath sql.NullString
    var metadataFetchedAt sql.NullTime

    err := s.db.QueryRowContext(ctx, query, id).Scan(
        &item.ID, &item.LibraryID, &item.Path, &item.Title,
        &year, &overview, &posterPath,
        &videoJSON, &audioJSON, &subtitleJSON,
        &item.Container, &duration, &fileSize,
        &item.FileModifiedAt, &item.FileHash,
        &item.CreatedAt, &item.UpdatedAt, &metadataFetchedAt,
    )

    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    // Unmarshal optional fields
    if year.Valid {
        item.Year = int(year.Int64)
    }
    if overview.Valid {
        item.Overview = overview.String
    }
    if posterPath.Valid {
        item.PosterPath = posterPath.String
    }
    if duration.Valid {
        item.Duration = duration.Float64
    }
    if fileSize.Valid {
        item.FileSize = fileSize.Int64
    }
    if metadataFetchedAt.Valid {
        item.MetadataFetchedAt = &metadataFetchedAt.Time
    }

    // Unmarshal JSON fields
    if videoJSON.Valid && videoJSON.String != "" && videoJSON.String != "null" {
        json.Unmarshal([]byte(videoJSON.String), &item.VideoStream)
    }
    if audioJSON.Valid && audioJSON.String != "" && audioJSON.String != "null" {
        json.Unmarshal([]byte(audioJSON.String), &item.AudioStreams)
    }
    if subtitleJSON.Valid && subtitleJSON.String != "" && subtitleJSON.String != "null" {
        json.Unmarshal([]byte(subtitleJSON.String), &item.SubtitleStreams)
    }

    return &item, nil
}

// GetLibraryItems retrieves all items in a library with pagination
func (s *Store) GetLibraryItems(ctx context.Context, libraryID string, offset, limit int) ([]*MediaItem, int, error) {
    // Get total count
    var total int
    err := s.db.QueryRowContext(ctx,
        "SELECT COUNT(*) FROM media_items WHERE library_id = ?", libraryID,
    ).Scan(&total)
    if err != nil {
        return nil, 0, err
    }

    query := `
    SELECT id, library_id, path, title, year, overview, poster_path,
           video_stream_json, audio_streams_json, subtitle_streams_json,
           container, duration, file_size, file_modified_at, file_hash,
           created_at, updated_at
    FROM media_items WHERE library_id = ?
    ORDER BY title
    LIMIT ? OFFSET ?
    `

    rows, err := s.db.QueryContext(ctx, query, libraryID, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var items []*MediaItem
    for rows.Next() {
        var item MediaItem
        var videoJSON, audioJSON, subtitleJSON sql.NullString
        var year sql.NullInt64
        var duration sql.NullFloat64
        var overview, posterPath sql.NullString

        err := rows.Scan(
            &item.ID, &item.LibraryID, &item.Path, &item.Title,
            &year, &overview, &posterPath,
            &videoJSON, &audioJSON, &subtitleJSON,
            &item.Container, &duration, &item.FileSize,
            &item.FileModifiedAt, &item.FileHash,
            &item.CreatedAt, &item.UpdatedAt,
        )
        if err != nil {
            return nil, 0, err
        }

        if year.Valid {
            item.Year = int(year.Int64)
        }
        if overview.Valid {
            item.Overview = overview.String
        }
        if posterPath.Valid {
            item.PosterPath = posterPath.String
        }
        if duration.Valid {
            item.Duration = duration.Float64
        }

        if videoJSON.Valid && videoJSON.String != "" && videoJSON.String != "null" {
            json.Unmarshal([]byte(videoJSON.String), &item.VideoStream)
        }
        if audioJSON.Valid && audioJSON.String != "" && audioJSON.String != "null" {
            json.Unmarshal([]byte(audioJSON.String), &item.AudioStreams)
        }
        if subtitleJSON.Valid && subtitleJSON.String != "" && subtitleJSON.String != "null" {
            json.Unmarshal([]byte(subtitleJSON.String), &item.SubtitleStreams)
        }

        items = append(items, &item)
    }

    return items, total, nil
}

// SaveLibrary saves or updates a library
func (s *Store) SaveLibrary(ctx context.Context, lib *Library) error {
    now := time.Now()
    if lib.CreatedAt.IsZero() {
        lib.CreatedAt = now
    }
    lib.UpdatedAt = now

    query := `
    INSERT INTO libraries (id, name, path, library_type, enabled, refresh_interval_minutes, created_at, updated_at, last_scan_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(id) DO UPDATE SET
        name = excluded.name,
        path = excluded.path,
        library_type = excluded.library_type,
        enabled = excluded.enabled,
        refresh_interval_minutes = excluded.refresh_interval_minutes,
        updated_at = CURRENT_TIMESTAMP,
        last_scan_at = excluded.last_scan_at
    `

    _, err := s.db.ExecContext(ctx, query,
        lib.ID, lib.Name, lib.Path, lib.LibraryType, lib.Enabled, lib.RefreshIntervalMinutes,
        lib.CreatedAt, lib.UpdatedAt, lib.LastScanAt,
    )

    return err
}

// GetLibrary retrieves a library by ID
func (s *Store) GetLibrary(ctx context.Context, id string) (*Library, error) {
    query := `
    SELECT id, name, path, library_type, enabled, refresh_interval_minutes, created_at, updated_at, last_scan_at
    FROM libraries WHERE id = ?
    `

    var lib Library
    err := s.db.QueryRowContext(ctx, query, id).Scan(
        &lib.ID, &lib.Name, &lib.Path, &lib.LibraryType, &lib.Enabled, &lib.RefreshIntervalMinutes,
        &lib.CreatedAt, &lib.UpdatedAt, &lib.LastScanAt,
    )

    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    return &lib, nil
}

// GetAllLibraries retrieves all libraries
func (s *Store) GetAllLibraries(ctx context.Context) ([]*Library, error) {
    query := `
    SELECT id, name, path, library_type, enabled, refresh_interval_minutes, created_at, updated_at, last_scan_at
    FROM libraries ORDER BY name
    `

    rows, err := s.db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var libs []*Library
    for rows.Next() {
        var lib Library
        err := rows.Scan(
            &lib.ID, &lib.Name, &lib.Path, &lib.LibraryType, &lib.Enabled, &lib.RefreshIntervalMinutes,
            &lib.CreatedAt, &lib.UpdatedAt, &lib.LastScanAt,
        )
        if err != nil {
            return nil, err
        }
        libs = append(libs, &lib)
    }

    return libs, nil
}

// DeleteMediaItem removes a media item
func (s *Store) DeleteMediaItem(ctx context.Context, id string) error {
    _, err := s.db.ExecContext(ctx, "DELETE FROM media_items WHERE id = ?", id)
    return err
}

// DeleteLibrary removes a library and its items
func (s *Store) DeleteLibrary(ctx context.Context, id string) error {
    _, err := s.db.ExecContext(ctx, "DELETE FROM libraries WHERE id = ?", id)
    return err
}

// UpdateLibraryScanTime updates the last scan time for a library
func (s *Store) UpdateLibraryScanTime(ctx context.Context, id string, t time.Time) error {
    _, err := s.db.ExecContext(ctx, 
        "UPDATE libraries SET last_scan_at = ? WHERE id = ?", t, id)
    return err
}
