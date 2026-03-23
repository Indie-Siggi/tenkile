// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tenkile/tenkile/internal/media"
	"github.com/tenkile/tenkile/internal/stream"
)

// MediaHandler handles media item requests
type MediaHandler struct {
	store         *media.Store
	streamHandler *stream.Handler
}

// NewMediaHandler creates a new media handler
func NewMediaHandler(store *media.Store, streamHandler *stream.Handler) *MediaHandler {
	return &MediaHandler{
		store:         store,
		streamHandler: streamHandler,
	}
}

// Get handles GET /media/{id}
func (h *MediaHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.store.GetMediaItem(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to get media item")
		return
	}
	if item == nil {
		WriteError(w, http.StatusNotFound, "Media not found")
		return
	}
	WriteJSON(w, http.StatusOK, item)
}

// StreamInfo handles GET /media/{id}/stream
func (h *MediaHandler) StreamInfo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.store.GetMediaItem(r.Context(), id)
	if err != nil || item == nil {
		WriteError(w, http.StatusNotFound, "Media not found")
		return
	}

	// Return stream info
	info := map[string]interface{}{
		"id":       item.ID,
		"title":    item.Title,
		"duration": item.Duration,
		"container": item.Container,
	}

	if item.VideoStream != nil {
		info["video"] = map[string]interface{}{
			"codec":   item.VideoStream.Codec,
			"width":   item.VideoStream.Width,
			"height":  item.VideoStream.Height,
			"bitrate": item.VideoStream.Bitrate,
			"hdr":     item.VideoStream.HDRType,
		}
	}

	info["audio_tracks"] = item.AudioStreams
	info["subtitle_tracks"] = item.SubtitleStreams

	WriteJSON(w, http.StatusOK, info)
}

// Play handles GET /media/{id}/play - returns streaming URL info
func (h *MediaHandler) Play(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.store.GetMediaItem(r.Context(), id)
	if err != nil || item == nil {
		WriteError(w, http.StatusNotFound, "Media not found")
		return
	}

	// Return playback info with stream URLs
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"id":           item.ID,
		"title":        item.Title,
		"duration":     item.Duration,
		"direct_play": map[string]string{
			"url":      "/api/v1/stream/hls/" + item.ID,
			"type":     "hls",
			"mime_type": "application/x-mpegURL",
		},
	})
}
