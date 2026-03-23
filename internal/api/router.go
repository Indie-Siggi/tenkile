// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/tenkile/tenkile/internal/config"
	"github.com/tenkile/tenkile/internal/database"
	"github.com/tenkile/tenkile/internal/probes"
	"github.com/tenkile/tenkile/internal/transcode"
)

// deviceIDRegex validates device ID format (alphanumeric, hyphens, underscores, max 64 chars)
var deviceIDRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// isValidDeviceID validates a device ID string
func isValidDeviceID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	return deviceIDRegex.MatchString(id)
}

// isValidNumericField validates that a numeric field is positive and within reasonable bounds
func isValidNumericField(value int64, min, max int64) bool {
	return value >= min && value <= max
}

// Router wraps chi.Router and holds handler dependencies
type Router struct {
	chi.Router
	db            *database.SQLite
	cfg           *config.Config
	validator     *probes.Validator
	cache         *probes.CapabilityCache
	curatedDB     *probes.CuratedDatabase
	orchestrator  *transcode.Orchestrator
	decisionLog   *transcode.DecisionLogger
	rateLimiter   *rateLimiter
	feedbackManager *probes.FeedbackManager

	// Handlers
	devices  *DeviceHandlers
	playback *PlaybackHandlers
	admin    *AdminHandlers
}

// NewRouter creates a new Chi router with configured middleware and handlers
func NewRouter(cfg *config.Config, db *database.SQLite, orchestrator *transcode.Orchestrator, decisionLog *transcode.DecisionLogger) http.Handler {
	r := &Router{
		Router:       chi.NewRouter(),
		db:           db,
		cfg:          cfg,
		orchestrator: orchestrator,
		decisionLog:  decisionLog,
	}

	// Initialize dependencies
	r.validator = probes.NewValidator()

	cacheConfig := &probes.CacheConfig{
		MemoryTTL:      24 * time.Hour,
		SQLiteTTL:      30 * 24 * time.Hour,
		EnableSQLite:   false, // Phase 1: memory-only cache
		MaxMemoryItems: 10000,
	}
	var err error
	r.cache, err = probes.NewCapabilityCache(cacheConfig)
	if err != nil {
		slog.Error("Failed to initialize capability cache", "error", err)
		// Fall back to memory-only cache
		r.cache, _ = probes.NewCapabilityCache(&probes.CacheConfig{
			MemoryTTL:      24 * time.Hour,
			EnableSQLite:   false,
			MaxMemoryItems: 10000,
		})
	}

	r.curatedDB = probes.NewCuratedDatabase()

	// Load curated devices from data directory
	curatedPath := "./data/curated"
	if err := r.curatedDB.Load(curatedPath); err != nil {
		slog.Warn("Failed to load curated devices", "error", err)
	}

	// Initialize feedback manager (Phase 3.2)
	r.feedbackManager = probes.NewFeedbackManager()

	// Initialize handlers
	r.devices = NewDeviceHandlers(r.validator, r.cache, r.curatedDB)
	r.devices.SetFeedbackManager(r.feedbackManager)
	r.playback = NewPlaybackHandlers(r.validator, r.cache, r.curatedDB, r.orchestrator)
	r.admin = NewAdminHandlers(r.validator, r.cache, r.curatedDB)
	if r.decisionLog != nil {
		r.admin.SetDecisionLogger(r.decisionLog)
	}

	// Initialize rate limiter (100 requests per minute for public endpoints)
	r.rateLimiter = newRateLimiter(100, time.Minute)

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.RequestSize(10 << 20))

	// CORS configuration
	allowedOrigins := cfg.Auth.AllowedOrigins
	if len(allowedOrigins) == 0 {
		if cfg.Auth.ProductionMode {
			allowedOrigins = []string{}
		} else {
			allowedOrigins = []string{"http://localhost:*", "http://127.0.0.1:*"}
		}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint
	r.Get("/health", r.healthCheckHandler)

	// API v1 routes — use different variable names to avoid shadowing Router receiver
	r.Route("/api/v1", func(api chi.Router) {
		// Public endpoints (no auth required)
		api.Group(func(pub chi.Router) {
			pub.Get("/openapi.yaml", r.openAPIHandler)

			// Device probe endpoints with rate limiting
			pub.Group(func(rl chi.Router) {
				rl.Use(rateLimitMiddleware(r.rateLimiter))
				rl.Post("/probe/report", r.devices.handleProbeReport)
				rl.Get("/probe/scenarios", r.probeScenariosHandler)
			})

			// Capabilities endpoints
			pub.Get("/capabilities", r.devices.handleGetCapabilities)
			pub.Post("/capabilities", r.devices.handleValidateCapabilities)
		})

		// Protected endpoints (auth required)
		api.Group(func(auth chi.Router) {
			auth.Use(r.authMiddleware)

			// Device endpoints
			auth.Get("/devices", r.devices.handleSearchDevices)
			auth.Get("/devices/curated", r.devices.handleGetCuratedDevices)
			auth.Get("/devices/platform/{platform}", r.devices.handleGetDevicesByPlatform)
			auth.Get("/devices/codecs/{codec}", r.devices.handleGetDevicesByCodec)
			auth.Get("/device/{id}/profile", r.devices.handleGetDeviceProfile)

			// Phase 3.2: Playback Feedback endpoints
			auth.Post("/devices/{id}/feedback", r.devices.handlePlaybackFeedback)
			auth.Get("/devices/{id}/feedback/stats", r.devices.handleGetPlaybackStats)
			auth.Get("/devices/{id}/reliable-codecs", r.devices.handleGetReliableCodecs)
			auth.Post("/devices/{id}/reprobe", r.devices.handleReProbeDevice)
			auth.Get("/devices/{id}/trust", r.devices.handleGetTrustReport)
			auth.Get("/feedback/metrics", r.devices.handleGetFeedbackMetrics)

			// Playback endpoints
			auth.Post("/playback/decision", r.playback.handlePlaybackDecision)
			auth.Post("/playback/feedback", r.playback.handlePlaybackFeedback)
			auth.Post("/playback/transcode", r.playback.handleTranscodeRecommendation)
			auth.Get("/playback/profiles", r.playback.handleGetProfiles)
			auth.Post("/playback/validate", r.playback.handleValidateStream)
			auth.Get("/playback/history", r.playbackHistoryHandler)

			// Library endpoints (stubs for future phases)
			auth.Route("/library", func(lib chi.Router) {
				lib.Get("/", r.listLibrariesHandler)
				lib.Get("/{libraryID}", r.getLibraryHandler)
				lib.Get("/{libraryID}/items", r.listLibraryItemsHandler)
				lib.Get("/{libraryID}/items/{itemID}", r.getLibraryItemHandler)
			})

			// Legacy device endpoints
			auth.Route("/devices", func(dev chi.Router) {
				dev.Get("/", r.listDevicesHandler)
				dev.Get("/{deviceID}", r.getDeviceHandler)
				dev.Get("/{deviceID}/capabilities", r.getDeviceCapabilitiesHandler)
				dev.Patch("/{deviceID}", r.updateDeviceHandler)
			})

			// User endpoints (stubs for future phases)
			auth.Route("/users", func(usr chi.Router) {
				usr.Get("/", r.listUsersHandler)
				usr.Post("/", r.createUserHandler)
				usr.Get("/{userID}", r.getUserHandler)
				usr.Put("/{userID}", r.updateUserHandler)
				usr.Delete("/{userID}", r.deleteUserHandler)
			})

			// Admin endpoints
			auth.Route("/admin", func(adm chi.Router) {
				adm.Use(r.adminOnlyMiddleware)

				adm.Get("/system", r.admin.handleGetSystemInfo)
				adm.Get("/health", r.admin.handleHealthCheck)

				adm.Get("/stats/cache", r.admin.handleGetCacheStats)
				adm.Get("/stats/database", r.admin.handleGetDatabaseStats)
				adm.Get("/stats/validator", r.admin.handleGetValidatorStats)

				adm.Post("/cache/cleanup", r.admin.handleCacheCleanup)
				adm.Get("/cache/memory", r.admin.handleGetMemoryCache)

				adm.Get("/anomalies", r.admin.handleGetAnomalies)

				// Decision log endpoints (Phase 2.3)
				adm.Get("/decisions", r.admin.handleGetDecisions)
				adm.Get("/decisions/stats", r.admin.handleGetDecisionStats)

				adm.Post("/curated/device", r.admin.handleUpdateCuratedDevice)
				adm.Post("/curated/device/{id}/verify", r.admin.handleVerifyDevice)
				adm.Delete("/curated/device/{id}", r.admin.handleRemoveCuratedDevice)

				// Phase 3.1: Extended curated device management
				adm.Put("/curated/devices", r.admin.handleCreateCuratedDevice)
				adm.Put("/curated/devices/{id}", r.admin.handlePutCuratedDevice)
				adm.Delete("/curated/devices/{id}", r.admin.handleDeleteCuratedDevice)
				adm.Post("/curated/devices/{id}/vote", r.admin.handleVoteCuratedDevice)
				adm.Post("/curated/search", r.admin.handleFuzzySearchCuratedDevices)
				adm.Post("/curated/version-match", r.admin.handleVersionMatch)
				adm.Get("/curated/embedded/stats", r.admin.handleGetEmbeddedStats)
				adm.Post("/curated/embedded/sync", r.admin.handleSyncEmbeddedToCuratedDB)
				adm.Get("/curated/export", r.admin.handleExportCuratedDevices)
				adm.Post("/curated/import", r.admin.handleImportCuratedDevices)

				adm.Get("/stats", r.adminStatsHandler)
				adm.Post("/migrations", r.runMigrationsHandler)
				adm.Get("/curated-devices", r.listCuratedDevicesHandler)
				adm.Post("/curated-devices", r.createCuratedDeviceHandler)
			})
		})
	})

	return r
}

// healthCheckHandler handles the health check endpoint
func (r *Router) healthCheckHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := r.db.DB().Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy","error":"database connection failed"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// openAPIHandler serves the OpenAPI specification
func (r *Router) openAPIHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# OpenAPI specification placeholder\n"))
}

// probeScenariosHandler returns available probe scenarios
func (r *Router) probeScenariosHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// playbackHistoryHandler returns playback history
func (r *Router) playbackHistoryHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listLibrariesHandler lists all media libraries
func (r *Router) listLibrariesHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getLibraryHandler gets a specific library
func (r *Router) getLibraryHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listLibraryItemsHandler lists items in a library
func (r *Router) listLibraryItemsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getLibraryItemHandler gets a specific library item
func (r *Router) getLibraryItemHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listDevicesHandler lists all registered devices
func (r *Router) listDevicesHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getDeviceHandler gets a specific device
func (r *Router) getDeviceHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getDeviceCapabilitiesHandler gets device capabilities
func (r *Router) getDeviceCapabilitiesHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// updateDeviceHandler updates device information
func (r *Router) updateDeviceHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listUsersHandler lists all users
func (r *Router) listUsersHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// createUserHandler creates a new user
func (r *Router) createUserHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// getUserHandler gets a specific user
func (r *Router) getUserHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// updateUserHandler updates a user
func (r *Router) updateUserHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// deleteUserHandler deletes a user
func (r *Router) deleteUserHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// adminStatsHandler returns admin statistics
func (r *Router) adminStatsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// runMigrationsHandler runs database migrations
func (r *Router) runMigrationsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// listCuratedDevicesHandler lists curated device profiles
func (r *Router) listCuratedDevicesHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// createCuratedDeviceHandler creates a curated device profile
func (r *Router) createCuratedDeviceHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

// authMiddleware handles authentication
func (r *Router) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		apiKeyHeader := r.cfg.Auth.APIKeyHeader
		if apiKeyHeader == "" {
			apiKeyHeader = "X-API-Key"
		}
		apiKey := req.Header.Get(apiKeyHeader)

		authHeader := req.Header.Get("Authorization")
		hasJWT := len(authHeader) > 7 && authHeader[:7] == "Bearer "

		if apiKey == "" && !hasJWT {
			RespondJSON(w, http.StatusUnauthorized, ErrorResponse{
				Error:   "authentication_required",
				Message: "Authentication required",
			})
			return
		}

		// Validate API key
		if apiKey != "" {
			if r.cfg.Auth.APIKey == "" {
				RespondJSON(w, http.StatusUnauthorized, ErrorResponse{
					Error:   "api_key_not_configured",
					Message: "API key authentication is not configured",
				})
				return
			}
			if apiKey != r.cfg.Auth.APIKey {
				RespondJSON(w, http.StatusUnauthorized, ErrorResponse{
					Error:   "invalid_api_key",
					Message: "Invalid API key",
				})
				return
			}
			user := &UserInfo{
				ID:      "api-key-user",
				Role:    "admin",
				IsAdmin: true,
				APIKey:  true,
			}
			ctx := context.WithValue(req.Context(), userContextKey{}, user)
			next.ServeHTTP(w, req.WithContext(ctx))
			return
		}

		// Validate JWT token (simplified — proper JWT validation is Phase 2)
		if hasJWT && r.cfg.Auth.JWTSecret != "" {
			user := &UserInfo{
				ID:      "jwt-user",
				Role:    "user",
				IsAdmin: false,
			}
			ctx := context.WithValue(req.Context(), userContextKey{}, user)
			next.ServeHTTP(w, req.WithContext(ctx))
			return
		}

		RespondJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "authentication_failed",
			Message: "Authentication failed",
		})
	})
}

// userContextKey is the context key for user information
type userContextKey struct{}

// UserInfo holds authenticated user information
type UserInfo struct {
	ID      string
	Email   string
	Role    string
	IsAdmin bool
	APIKey  bool
}

// adminOnlyMiddleware ensures user has admin role
func (r *Router) adminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		user, ok := req.Context().Value(userContextKey{}).(*UserInfo)
		if !ok || user == nil {
			RespondJSON(w, http.StatusUnauthorized, ErrorResponse{
				Error:   "authentication_required",
				Message: "Authentication required",
			})
			return
		}
		if !user.IsAdmin {
			RespondJSON(w, http.StatusForbidden, ErrorResponse{
				Error:   "forbidden",
				Message: "Admin access required",
			})
			return
		}
		next.ServeHTTP(w, req)
	})
}

// GetRouter returns the underlying chi.Router
func (r *Router) GetRouter() chi.Router {
	return r.Router
}

// GetDatabase returns the database connection
func (r *Router) GetDatabase() *database.SQLite {
	return r.db
}

// GetCache returns the capability cache
func (r *Router) GetCache() *probes.CapabilityCache {
	return r.cache
}

// GetCuratedDB returns the curated database
func (r *Router) GetCuratedDB() *probes.CuratedDatabase {
	return r.curatedDB
}

// GetValidator returns the validator
func (r *Router) GetValidator() *probes.Validator {
	return r.validator
}

// rateLimiter provides simple rate limiting
type rateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request is allowed
func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	requests := rl.requests[key]

	var validRequests []time.Time
	for _, t := range requests {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}

	if len(validRequests) >= rl.limit {
		rl.requests[key] = validRequests
		return false
	}

	validRequests = append(validRequests, now)
	rl.requests[key] = validRequests
	return true
}

// rateLimitMiddleware creates a rate limiting middleware
func rateLimitMiddleware(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			clientIP := req.RemoteAddr
			if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
				clientIP = forwarded
			}

			if !rl.Allow(clientIP) {
				RespondJSON(w, http.StatusTooManyRequests, ErrorResponse{
					Error:   "rate_limit_exceeded",
					Message: "Too many requests, please try again later",
				})
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}
