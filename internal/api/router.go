// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/tenkile/tenkile/internal/config"
)

// NewRouter creates a new Chi router with configured middleware
func NewRouter(cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint
	r.Get("/health", healthCheck)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints (no auth required)
		r.Group(func(r chi.Router) {
			r.Get("/openapi.yaml", openAPIHandler)
			r.Post("/probe/report", probeReportHandler)
			r.Get("/probe/scenarios", probeScenariosHandler)
		})

		// Protected endpoints (auth required)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware(cfg))

			// Playback endpoints
			r.Route("/playback", func(r chi.Router) {
				r.Post("/decision", playbackDecisionHandler)
				r.Post("/feedback", playbackFeedbackHandler)
				r.Get("/history", playbackHistoryHandler)
			})

			// Library endpoints
			r.Route("/library", func(r chi.Router) {
				r.Get("/", listLibrariesHandler)
				r.Get("/{libraryID}", getLibraryHandler)
				r.Get("/{libraryID}/items", listLibraryItemsHandler)
				r.Get("/{libraryID}/items/{itemID}", getLibraryItemHandler)
			})

			// Device endpoints
			r.Route("/devices", func(r chi.Router) {
				r.Get("/", listDevicesHandler)
				r.Get("/{deviceID}", getDeviceHandler)
				r.Get("/{deviceID}/capabilities", getDeviceCapabilitiesHandler)
				r.Patch("/{deviceID}", updateDeviceHandler)
			})

			// User endpoints
			r.Route("/users", func(r chi.Router) {
				r.Get("/", listUsersHandler)
				r.Post("/", createUserHandler)
				r.Get("/{userID}", getUserHandler)
				r.Put("/{userID}", updateUserHandler)
				r.Delete("/{userID}", deleteUserHandler)
			})

			// Admin endpoints
			r.Route("/admin", func(r chi.Router) {
				r.Use(adminOnlyMiddleware)
				r.Get("/stats", adminStatsHandler)
				r.Post("/migrations", runMigrationsHandler)
				r.Get("/curated-devices", listCuratedDevicesHandler)
				r.Post("/curated-devices", createCuratedDeviceHandler)
			})
		})
	})

	return r
}

// healthCheck handles the health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// openAPIHandler serves the OpenAPI specification
func openAPIHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Serve actual OpenAPI spec
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# OpenAPI specification placeholder\n"))
}

// probeReportHandler handles device probe reports
func probeReportHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// probeScenariosHandler returns available probe scenarios
func probeScenariosHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// playbackDecisionHandler handles playback decisions
func playbackDecisionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// playbackFeedbackHandler handles playback feedback
func playbackFeedbackHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// playbackHistoryHandler returns playback history
func playbackHistoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listLibrariesHandler lists all media libraries
func listLibrariesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getLibraryHandler gets a specific library
func getLibraryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listLibraryItemsHandler lists items in a library
func listLibraryItemsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getLibraryItemHandler gets a specific library item
func getLibraryItemHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listDevicesHandler lists all registered devices
func listDevicesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getDeviceHandler gets a specific device
func getDeviceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getDeviceCapabilitiesHandler gets device capabilities
func getDeviceCapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// updateDeviceHandler updates device information
func updateDeviceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listUsersHandler lists all users
func listUsersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// createUserHandler creates a new user
func createUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getUserHandler gets a specific user
func getUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// updateUserHandler updates a user
func updateUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// deleteUserHandler deletes a user
func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// adminStatsHandler returns admin statistics
func adminStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// runMigrationsHandler runs database migrations
func runMigrationsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listCuratedDevicesHandler lists curated device profiles
func listCuratedDevicesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// createCuratedDeviceHandler creates a curated device profile
func createCuratedDeviceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// authMiddleware handles authentication
func authMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement JWT/API key authentication
			next.ServeHTTP(w, r)
		})
	}
}

// adminOnlyMiddleware ensures user has admin role
func adminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Check admin role
		next.ServeHTTP(w, r)
	})
}
