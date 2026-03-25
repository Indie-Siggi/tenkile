// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tenkile/tenkile/internal/media"
)

// LibraryHandler handles library management requests
type LibraryHandler struct {
	store   *media.Store
	scanner *media.Scanner
}

// NewLibraryHandler creates a new library handler
func NewLibraryHandler(store *media.Store, scanner *media.Scanner) *LibraryHandler {
	return &LibraryHandler{
		store:   store,
		scanner: scanner,
	}
}

// List handles GET /libraries
func (h *LibraryHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	libs, err := h.store.GetAllLibraries(ctx)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to get libraries")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"libraries": libs,
		"count":    len(libs),
	})
}

// Create handles POST /libraries
func (h *LibraryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                   string                   `json:"name"`
		Path                   string                   `json:"path"`
		LibraryType            media.LibraryType        `json:"library_type"`
		Enabled                bool                     `json:"enabled"`
		RefreshIntervalMinutes int                      `json:"refresh_interval_minutes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" || req.Path == "" || req.LibraryType == "" {
		WriteError(w, http.StatusBadRequest, "name, path, and library_type are required")
		return
	}

	lib := &media.Library{
		ID:                     generateID(req.Path + time.Now().String()),
		Name:                   req.Name,
		Path:                   req.Path,
		LibraryType:            req.LibraryType,
		Enabled:                req.Enabled,
		RefreshIntervalMinutes: req.RefreshIntervalMinutes,
	}
	if lib.RefreshIntervalMinutes < 0 {
		WriteError(w, http.StatusBadRequest, "refresh_interval_minutes must be non-negative")
		return
	}
	if lib.RefreshIntervalMinutes == 0 {
		lib.RefreshIntervalMinutes = 60
	}

	if err := h.store.SaveLibrary(r.Context(), lib); err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to create library")
		return
	}

	WriteJSON(w, http.StatusCreated, lib)
}

// Get handles GET /libraries/{id}
func (h *LibraryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	lib, err := h.store.GetLibrary(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to get library")
		return
	}
	if lib == nil {
		WriteError(w, http.StatusNotFound, "Library not found")
		return
	}
	WriteJSON(w, http.StatusOK, lib)
}

// Update handles PUT /libraries/{id}
func (h *LibraryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get existing library
	existing, err := h.store.GetLibrary(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to get library")
		return
	}
	if existing == nil {
		WriteError(w, http.StatusNotFound, "Library not found")
		return
	}

	var req struct {
		Name                   string `json:"name"`
		Path                   string `json:"path"`
		LibraryType            string `json:"library_type"`
		Enabled                *bool  `json:"enabled"`
		RefreshIntervalMinutes int    `json:"refresh_interval_minutes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Update fields
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Path != "" {
		existing.Path = req.Path
	}
	if req.LibraryType != "" {
		existing.LibraryType = media.LibraryType(req.LibraryType)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.RefreshIntervalMinutes > 0 {
		existing.RefreshIntervalMinutes = req.RefreshIntervalMinutes
	}

	if err := h.store.SaveLibrary(r.Context(), existing); err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to update library")
		return
	}

	WriteJSON(w, http.StatusOK, existing)
}

// Delete handles DELETE /libraries/{id}
func (h *LibraryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteLibrary(r.Context(), id); err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to delete library")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Scan handles POST /libraries/{id}/scan
func (h *LibraryHandler) Scan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	lib, err := h.store.GetLibrary(r.Context(), id)
	if err != nil || lib == nil {
		WriteError(w, http.StatusNotFound, "Library not found")
		return
	}

	// Start scan in background with timeout
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()
		h.scanner.ScanLibrary(ctx, lib)
		h.store.UpdateLibraryScanTime(ctx, lib.ID, time.Now())
	}()

	WriteJSON(w, http.StatusAccepted, map[string]string{
		"message": "Scan started",
	})
}

// ScanStatus handles GET /libraries/{id}/scan/status
func (h *LibraryHandler) ScanStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status := h.scanner.GetStatus(id)
	if status == nil {
		WriteJSON(w, http.StatusOK, map[string]string{
			"status": "idle",
		})
		return
	}
	WriteJSON(w, http.StatusOK, status)
}

// ListItems handles GET /libraries/{libraryId}/items
func (h *LibraryHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "libraryId")

	// Pagination - parse from query params with validation
	offset := 0
	limit := 50

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		} else {
			WriteError(w, http.StatusBadRequest, "Invalid offset parameter: must be a non-negative integer")
			return
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 200 {
			limit = v
		} else {
			WriteError(w, http.StatusBadRequest, "Invalid limit parameter: must be between 1 and 200")
			return
		}
	}

	items, total, err := h.store.GetLibraryItems(r.Context(), libraryID, offset, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to get items")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"items":  items,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}
