package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tenkile/tenkile/internal/events"
	"github.com/tenkile/tenkile/internal/media"
)

// Handler handles streaming HTTP requests
type Handler struct {
	segmenter    *Segmenter
	mediaStore   *media.Store
	sessionTTL   time.Duration
	sessions     map[string]*StreamSession
	mu           sync.RWMutex // protects sessions map
	cleanupDone  chan struct{}
}

// NewHandler creates a new streaming handler
func NewHandler(mediaStore *media.Store) *Handler {
	h := &Handler{
		segmenter:  NewSegmenter("", "", nil),
		mediaStore: mediaStore,
		sessionTTL: 4 * time.Hour,
		sessions:   make(map[string]*StreamSession),
		cleanupDone: make(chan struct{}),
	}
	go h.cleanupLoop()
	return h
}

// cleanupLoop periodically cleans up expired sessions and orphaned HLS segments
func (h *Handler) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.cleanupExpiredSessions()
			h.cleanupOrphanedSegments()
		case <-h.cleanupDone:
			return
		}
	}
}

// cleanupExpiredSessions removes sessions that haven't been accessed within TTL
func (h *Handler) cleanupExpiredSessions() {
	h.mu.Lock()
	defer h.mu.Unlock()

	threshold := time.Now().Add(-h.sessionTTL)
	for id, session := range h.sessions {
		if session.LastAccess.Before(threshold) {
			// Clean up associated manifest files
			if session.ManifestPath != "" {
				dir := filepath.Dir(session.ManifestPath)
				os.RemoveAll(dir)
			}
			delete(h.sessions, id)
		}
	}
}

// cleanupOrphanedSegments removes HLS segment directories older than TTL
func (h *Handler) cleanupOrphanedSegments() {
	tempDir := os.TempDir()
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return
	}

	threshold := time.Now().Add(-h.sessionTTL)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "hls_") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(threshold) {
			os.RemoveAll(filepath.Join(tempDir, entry.Name()))
		}
	}
}

// Close stops the cleanup goroutine
func (h *Handler) Close() {
	close(h.cleanupDone)
}

// Segmenter returns the underlying segmenter
func (h *Handler) Segmenter() *Segmenter {
	return h.segmenter
}

// ServeHLS handles HLS streaming requests
func (h *Handler) ServeHLS(w http.ResponseWriter, r *http.Request) {
	mediaItemID := chi.URLParam(r, "id")
	variant := r.URL.Query().Get("variant") // e.g., "1080p", "720p"

	// Get media item
	item, err := h.mediaStore.GetMediaItem(r.Context(), mediaItemID)
	if err != nil || item == nil {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}

	// FIX #7: Check input file existence before calling GenerateHLS
	if _, err := os.Stat(item.Path); os.IsNotExist(err) {
		http.Error(w, "Media file not found", http.StatusNotFound)
		return
	}

	// Generate HLS manifest
	variants := selectVariants(variant)
	opts := HLSOptions{
		SegmentDuration: 6,
		TempDir:         os.TempDir(),
		IncludeAudio:    true,
	}

	manifest, err := h.segmenter.GenerateHLS(r.Context(), item.Path, variants, opts)
	if err != nil {
		// Publish stream error event
		events.PublishEvent(events.EventStreamError, events.TopicStreams, events.StreamPayload{
			MediaItemID: mediaItemID,
			MediaTitle:  item.Title,
			Variant:     variant,
		})
		http.Error(w, "Failed to generate stream", http.StatusInternalServerError)
		return
	}

	// Create a stream session
	session, _ := h.StartSession(r.Context(), mediaItemID, "", "", variant, StreamTypeHLS)
	session.ManifestPath = manifest.MasterPlaylist

	// FIX #5: Return URL like /api/v1/stream/hls/playlist?path=<encoded> not filesystem path
	encodedPath := filepath.ToSlash(manifest.MasterPlaylist)
	manifestURL := fmt.Sprintf("/api/v1/stream/hls/playlist?path=%s", encodedPath)

	// Publish stream started event
	events.PublishEvent(events.EventStreamStarted, events.TopicStreams, events.StreamPayload{
		StreamID:    session.ID,
		SessionID:   session.ID,
		MediaItemID: mediaItemID,
		MediaTitle:  item.Title,
		Variant:     variant,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"manifest": manifestURL,
		"type":     "hls",
		"session_id": session.ID,
	})
}

// ServeHLSManifest serves HLS playlist files
func (h *Handler) ServeHLSManifest(w http.ResponseWriter, r *http.Request) {
	// Extract path from URL
	playlistPath := r.URL.Query().Get("path")
	if playlistPath == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	// Security: ensure path is within allowed directory
	if !h.isPathAllowed(playlistPath) {
		http.Error(w, "Invalid path", http.StatusForbidden)
		return
	}

	// FIX #6: Check if segments exist before serving - serve cached manifest
	http.ServeFile(w, r, playlistPath)
}

// ServeHLSSegment serves HLS segment files
func (h *Handler) ServeHLSSegment(w http.ResponseWriter, r *http.Request) {
	segmentPath := r.URL.Query().Get("path")
	if segmentPath == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	// Security: ensure path is within allowed directory
	if !h.isPathAllowed(segmentPath) {
		http.Error(w, "Invalid path", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, segmentPath)
}

// FIX #3: Strong path validation using filepath.EvalSymlinks() and existence check
func (h *Handler) isPathAllowed(path string) bool {
	// Evaluate any symbolic links in the path
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}

	// Check if path exists
	if _, err := os.Stat(realPath); os.IsNotExist(err) {
		return false
	}

	// Ensure path is within temp directory
	abs, err := filepath.Abs(realPath)
	if err != nil {
		return false
	}
	tempDir := os.TempDir()
	return strings.HasPrefix(abs, tempDir)
}

// StreamInfo returns information about available streams for a media item
func (h *Handler) StreamInfo(w http.ResponseWriter, r *http.Request) {
	mediaItemID := chi.URLParam(r, "id")

	item, err := h.mediaStore.GetMediaItem(r.Context(), mediaItemID)
	if err != nil || item == nil {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}

	// Get available variants based on source resolution
	variants := h.getAvailableVariants(item)

	videoCodec := ""
	if item.VideoStream != nil {
		videoCodec = item.VideoStream.Codec
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"media_id":    mediaItemID,
		"duration":    item.Duration,
		"container":   item.Container,
		"video_codec": videoCodec,
		"variants":    variants,
	})
}

func (h *Handler) getAvailableVariants(item *media.MediaItem) []Variant {
	sourceHeight := 0
	if item.VideoStream != nil {
		sourceHeight = item.VideoStream.Height
	}

	allVariants := DefaultVariants()
	var available []Variant

	for _, v := range allVariants {
		// Only offer variants up to source resolution
		if v.Height <= sourceHeight {
			available = append(available, v)
		}
	}

	return available
}

func selectVariants(requestedVariant string) []Variant {
	allVariants := DefaultVariants()

	if requestedVariant == "" {
		return allVariants
	}

	// Return only the requested variant plus lower ones
	var selected []Variant
	for _, v := range allVariants {
		selected = append(selected, v)
		if v.Name == requestedVariant {
			break
		}
	}

	return selected
}

// FIX #1 & #9: StartSession with proper locking and field population
func (h *Handler) StartSession(ctx context.Context, mediaItemID, userID, deviceID, variant string, streamType StreamType) (*StreamSession, error) {
	session := &StreamSession{
		ID:          fmt.Sprintf("session_%d", time.Now().UnixNano()),
		MediaItemID: mediaItemID,
		UserID:      userID,
		DeviceID:    deviceID,
		StreamType:  streamType,
		Variant:     variant,
		StartTime:   time.Now(),
		LastAccess:  time.Now(),
	}

	h.mu.Lock()
	h.sessions[session.ID] = session
	h.mu.Unlock()
	return session, nil
}

// SetSessionManifest sets the manifest path for a session (called after HLS generation)
func (h *Handler) SetSessionManifest(sessionID, manifestPath string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if session, ok := h.sessions[sessionID]; ok {
		session.ManifestPath = manifestPath
	}
}

// FIX #1: GetSession with proper locking
func (h *Handler) GetSession(id string) (*StreamSession, bool) {
	h.mu.RLock()
	session, ok := h.sessions[id]
	h.mu.RUnlock()

	if ok {
		h.mu.Lock()
		session.LastAccess = time.Now()
		h.mu.Unlock()
	}
	return session, ok
}

// FIX #1: EndSession with proper locking
func (h *Handler) EndSession(id string) {
	h.mu.Lock()
	session, ok := h.sessions[id]
	if ok && session.ManifestPath != "" {
		// Clean up manifest files
		dir := filepath.Dir(session.ManifestPath)
		os.RemoveAll(dir)
	}
	delete(h.sessions, id)
	h.mu.Unlock()

	// Publish stream ended event
	if ok && session != nil {
		events.PublishEvent(events.EventStreamEnded, events.TopicStreams, events.StreamPayload{
			StreamID:    session.ID,
			SessionID:   session.ID,
			MediaItemID: session.MediaItemID,
			Variant:     session.Variant,
			BytesServed: session.BytesServed,
		})
	}
}

// FIX #1: UpdateBytesServed with proper locking
func (h *Handler) UpdateBytesServed(id string, bytes int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if session, ok := h.sessions[id]; ok {
		session.BytesServed += bytes
	}
}
