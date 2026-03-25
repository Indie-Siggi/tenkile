package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tenkile/tenkile/internal/circuitbreaker"
	"github.com/tenkile/tenkile/internal/events"
)

// Media extensions to scan
var MediaExtensions = map[string]bool{
    ".mkv":  true,
    ".mp4":  true,
    ".m4v":  true,
    ".avi":  true,
    ".mov":  true,
    ".wmv":  true,
    ".webm": true,
    ".ts":   true,
    ".m2ts": true,
    ".flv":  true,
    ".mpg":  true,
    ".mpeg": true,
    ".3gp":  true,
}

// Patterns to skip (checked against individual path components)
var SkipPatterns = []string{
    "sample",
    "samples",
    "extras",
    "bonus",
    "trailers",
    ".AppleDouble",
    ".DS_Store",
    ".tmp",
    ".cache",
}

// isPathComponentSkipped checks if a path component should be skipped
// SECURITY FIX: Check individual path components instead of substring matching
// to prevent false positives like "/media/sampler/example.mkv"
func isPathComponentSkipped(path string) bool {
	// Split path into components
	parts := strings.Split(filepath.ToSlash(path), "/")
	
	for _, part := range parts {
		for _, pattern := range SkipPatterns {
			if strings.EqualFold(part, pattern) {
				return true
			}
		}
	}
	return false
}

// validatePathForScanning validates a path before adding to scan list
// Returns true if the path should be scanned
func validatePathForScanning(path string) bool {
	// Empty path is invalid
	if path == "" {
		return false
	}

	// Check for null bytes (common injection attempt)
	if strings.Contains(path, "\x00") {
		return false
	}

	// Resolve symlinks to get the real path before validation
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}

	// Check for path traversal attempts on the resolved path
	if strings.Contains(resolved, "..") {
		return false
	}

	// Path must exist and not be a directory
	if info, err := os.Stat(resolved); err != nil || info.IsDir() {
		return false
	}

	return true
}

// Scanner handles media library scanning
type Scanner struct {
    ffprobe      *FFprobe
    store        *Store
    mu           sync.RWMutex
    scanStatus   map[string]*LibraryScanStatus
    ffprobeBreaker *circuitbreaker.Breaker
}

// NewScanner creates a new media scanner
func NewScanner(ffprobe *FFprobe, store *Store) *Scanner {
    cb := circuitbreaker.New(circuitbreaker.DefaultConfig("ffprobe"))
    
    // Log circuit breaker state changes
    cb.StateChangeHandler(func(name string, from, to circuitbreaker.State) {
        events.PublishEvent(events.EventLibraryScanError, events.TopicLibraries, events.LibraryScanPayload{
            LibraryID:   "",
            LibraryName: "ffprobe",
            Status:      string(ScanStatusError),
            Error:       "circuit_breaker:" + name + ":" + from.String() + "->" + to.String(),
        })
    })

    return &Scanner{
        ffprobe:       ffprobe,
        store:        store,
        scanStatus:   make(map[string]*LibraryScanStatus),
        ffprobeBreaker: cb,
    }
}

// FFprobeBreaker returns the FFprobe circuit breaker
func (s *Scanner) FFprobeBreaker() *circuitbreaker.Breaker {
    return s.ffprobeBreaker
}

// ScanLibrary scans a library path and indexes all media files
func (s *Scanner) ScanLibrary(ctx context.Context, lib *Library) error {
    s.updateStatus(lib.ID, &LibraryScanStatus{
        LibraryID: lib.ID,
        Status:    ScanStatusScanning,
        StartedAt:  timePtr(time.Now()),
    })

    // Publish scan started event
    events.PublishEvent(events.EventLibraryScanStarted, events.TopicLibraries, events.LibraryScanPayload{
        LibraryID:   lib.ID,
        LibraryName: lib.Name,
        TotalFiles:  0,
        Status:      string(ScanStatusScanning),
    })

    // Walk directory
    var files []string
    err := filepath.Walk(lib.Path, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil // Skip errors
        }

        // Skip directories
        if info.IsDir() {
            return nil
        }

        // SECURITY FIX: Check if any path component should be skipped
        if isPathComponentSkipped(path) {
            return nil
        }

        // SECURITY FIX: Validate path before processing
        if !validatePathForScanning(path) {
            return nil
        }

        // Check extension
        ext := strings.ToLower(filepath.Ext(path))
        if !MediaExtensions[ext] {
            return nil
        }

        files = append(files, path)
        return nil
    })

    if err != nil {
        s.updateStatus(lib.ID, &LibraryScanStatus{
            LibraryID: lib.ID,
            Status:    ScanStatusError,
            Error:     err.Error(),
        })
        // Publish scan error event
        events.PublishEvent(events.EventLibraryScanError, events.TopicLibraries, events.LibraryScanPayload{
            LibraryID: lib.ID,
            LibraryName: lib.Name,
            Status:    string(ScanStatusError),
            Error:    err.Error(),
        })
        return err
    }

    // Update total
    s.updateStatus(lib.ID, &LibraryScanStatus{
        LibraryID:  lib.ID,
        Status:     ScanStatusScanning,
        TotalFiles: len(files),
        Processed:  0,
    })

    // Process files
    for i, path := range files {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        // Use circuit breaker for FFprobe calls
        var item *MediaItem
        var probeErr error = errors.New("not attempted")
        
        _ = s.ffprobeBreaker.Do(func() error {
            item, probeErr = s.ffprobe.Probe(ctx, path)
            if probeErr != nil {
                return probeErr
            }
            // Record success
            return nil
        })
        
        if probeErr != nil {
            s.ffprobeBreaker.RecordFailure()
            // Create minimal media item from file info
            item = &MediaItem{
                ID:        generateID(path),
                Path:      path,
                LibraryID: lib.ID,
                Title:     filepath.Base(path),
            }
        } else {
            s.ffprobeBreaker.RecordSuccess()
        }

        item.LibraryID = lib.ID

        // Save to store
        if err := s.store.SaveMediaItem(ctx, item); err != nil {
            // Log but continue
            continue
        }

        // Update progress
        s.updateStatus(lib.ID, &LibraryScanStatus{
            LibraryID:   lib.ID,
            Status:       ScanStatusScanning,
            TotalFiles:   len(files),
            Processed:    i + 1,
            CurrentFile:  path,
        })

        // Publish progress event (every 10 files or on last file)
        if (i+1)%10 == 0 || i+1 == len(files) {
            events.PublishEvent(events.EventLibraryScanProgress, events.TopicLibraries, events.LibraryScanPayload{
                LibraryID:   lib.ID,
                LibraryName: lib.Name,
                TotalFiles:  len(files),
                Processed:   i + 1,
                CurrentFile: path,
                Status:      string(ScanStatusScanning),
            })
        }
    }

    // Mark complete
    s.updateStatus(lib.ID, &LibraryScanStatus{
        LibraryID:  lib.ID,
        Status:     ScanStatusCompleted,
        TotalFiles: len(files),
        Processed:  len(files),
    })

    // Publish scan complete event
    events.PublishEvent(events.EventLibraryScanComplete, events.TopicLibraries, events.LibraryScanPayload{
        LibraryID:   lib.ID,
        LibraryName: lib.Name,
        TotalFiles:  len(files),
        Processed:   len(files),
        Status:      string(ScanStatusCompleted),
    })

    return nil
}

// GetStatus returns the current scan status for a library
func (s *Scanner) GetStatus(libraryID string) *LibraryScanStatus {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.scanStatus[libraryID]
}

func (s *Scanner) updateStatus(libraryID string, status *LibraryScanStatus) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.scanStatus[libraryID] = status
}

func timePtr(t time.Time) *time.Time {
    return &t
}
