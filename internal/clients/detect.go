// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package clients

import (
	"net/http"
	"regexp"
	"strings"
)

// DetectionPattern represents a pattern for platform detection
type DetectionPattern struct {
	Platform   string // Platform identifier (e.g., "android", "appletvos")
	Headers    map[string]string // Header -> Pattern to match
	UserAgent  *regexp.Regexp   // User-Agent pattern
	Priority   int              // Higher priority = checked first
}

// Detector identifies platform-specific adapters based on request characteristics
type Detector struct {
	adapters map[string]ClientAdapter
	patterns []DetectionPattern
}

// NewDetector creates a new platform detector
func NewDetector() *Detector {
	d := &Detector{
		adapters: make(map[string]ClientAdapter),
		patterns: make([]DetectionPattern, 0),
	}

	// Register default detection patterns (order matters for priority)
	d.registerPatterns()
	return d
}

// registerPatterns registers default platform detection patterns
func (d *Detector) registerPatterns() {
	// TV platforms (higher priority - more specific)
	d.addPattern(DetectionPattern{
		Platform: "appletvos",
		Headers:  map[string]string{"X-Platform": "appletvos"},
		Priority: 100,
	})
	d.addPattern(DetectionPattern{
		Platform: "tizen",
		Headers:  map[string]string{"X-Platform": "tizen"},
		Priority: 100,
	})
	d.addPattern(DetectionPattern{
		Platform: "webos",
		Headers:  map[string]string{"X-Platform": "webos"},
		Priority: 100,
	})
	d.addPattern(DetectionPattern{
		Platform: "roku",
		Headers:  map[string]string{"X-Platform": "roku"},
		Priority: 100,
	})
	d.addPattern(DetectionPattern{
		Platform: "firetv",
		Headers:  map[string]string{"X-Platform": "firetv"},
		Priority: 100,
	})

	// Mobile platforms
	d.addPattern(DetectionPattern{
		Platform: "android",
		Headers:  map[string]string{"X-Platform": "android"},
		Priority: 90,
	})
	d.addPattern(DetectionPattern{
		Platform: "ios",
		Headers:  map[string]string{"X-Platform": "ios"},
		Priority: 90,
	})

	// TV platforms UA patterns (higher priority than mobile)
	d.addPattern(DetectionPattern{
		Platform:    "appletvos",
		UserAgent:   regexp.MustCompile(`(?i)AppleTV|tvOS|atvxb`),
		Priority:    80,
	})
	d.addPattern(DetectionPattern{
		Platform:    "tizen",
		UserAgent:   regexp.MustCompile(`(?i)Tizen|Samsung|SAMSUNG`),
		Priority:    80,
	})
	d.addPattern(DetectionPattern{
		Platform:    "webos",
		UserAgent:   regexp.MustCompile(`(?i)WebOS|webOS|LG`),
		Priority:    80,
	})
	d.addPattern(DetectionPattern{
		Platform:    "roku",
		UserAgent:   regexp.MustCompile(`(?i)Roku|DVP|RokuOS`),
		Priority:    80,
	})
	d.addPattern(DetectionPattern{
		Platform:    "firetv",
		UserAgent:   regexp.MustCompile(`(?i)FireTV|Amazon Fire|AFT[A-Z]`),
		Priority:    80,
	})
	d.addPattern(DetectionPattern{
		Platform:    "chromecast",
		UserAgent:   regexp.MustCompile(`(?i)CrKey|Chromecast`),
		Priority:    80,
	})
	d.addPattern(DetectionPattern{
		Platform:    "playstation",
		UserAgent:   regexp.MustCompile(`(?i)PlayStation|PS[3-5]|PlayStation [3-5]`),
		Priority:    80,
	})
	d.addPattern(DetectionPattern{
		Platform:    "xbox",
		UserAgent:   regexp.MustCompile(`(?i)Xbox|NXPE|XboxOne`),
		Priority:    80,
	})

	// Mobile platforms (lower priority - checked after TV platforms)
	d.addPattern(DetectionPattern{
		Platform:    "android",
		UserAgent:   regexp.MustCompile(`(?i)Android|Mobile; Android`),
		Priority:    70,
	})
	d.addPattern(DetectionPattern{
		Platform:    "ios",
		UserAgent:   regexp.MustCompile(`(?i)iPhone|iPad|iPod.*OS|iOS`),
		Priority:    70,
	})

	// Web browsers (higher priority than generic web)
	d.addPattern(DetectionPattern{
		Platform:    "edge",
		UserAgent:   regexp.MustCompile(`(?i)Edg/`),
		Priority:    65,
	})
	d.addPattern(DetectionPattern{
		Platform:    "chrome",
		UserAgent:   regexp.MustCompile(`(?i)Chrome/`),
		Priority:    60,
	})
	d.addPattern(DetectionPattern{
		Platform:    "firefox",
		UserAgent:   regexp.MustCompile(`(?i)Firefox/`),
		Priority:    60,
	})
	d.addPattern(DetectionPattern{
		Platform:    "safari",
		UserAgent:   regexp.MustCompile(`(?i)Version/.*Safari/`),
		Priority:    60,
	})

	// Generic web browsers (lowest priority)
	d.addPattern(DetectionPattern{
		Platform:    "web",
		UserAgent:   regexp.MustCompile(`(?i)Chrome/|Firefox/|Safari/|Edge/|OPR/|Edg/`),
		Priority:    50,
	})
}

// addPattern adds a detection pattern
func (d *Detector) addPattern(pattern DetectionPattern) {
	d.patterns = append(d.patterns, pattern)
}

// Detect identifies the appropriate adapter for the given request
func (d *Detector) Detect(r *http.Request) ClientAdapter {
	platform := d.detectPlatform(r)
	if adapter, ok := d.adapters[platform]; ok {
		return adapter
	}

	// Fall back to web adapter
	if adapter, ok := d.adapters["web"]; ok {
		return adapter
	}

	return nil
}

// detectPlatform identifies the platform from request headers/user-agent
func (d *Detector) detectPlatform(r *http.Request) string {
	// Check header-based patterns first (higher priority)
	for _, pattern := range d.patterns {
		if len(pattern.Headers) == 0 {
			continue // Skip UA-only patterns for header check
		}

		for header, value := range pattern.Headers {
			if h := r.Header.Get(header); h != "" {
				if strings.EqualFold(h, value) {
					return pattern.Platform
				}
			}
		}
	}

	// Check User-Agent patterns - find best match by priority
	ua := r.UserAgent()
	if ua == "" {
		return "unknown"
	}

	var bestMatch string
	var bestPriority int

	for _, pattern := range d.patterns {
		if pattern.UserAgent == nil {
			continue
		}
		if pattern.UserAgent.MatchString(ua) {
			if pattern.Priority > bestPriority {
				bestMatch = pattern.Platform
				bestPriority = pattern.Priority
			}
		}
	}

	if bestMatch != "" {
		return bestMatch
	}

	return "web" // Default to web
}

// RegisterAdapter registers a platform adapter
func (d *Detector) RegisterAdapter(adapter ClientAdapter) {
	d.adapters[adapter.ClientType()] = adapter
}

// GetAdapter returns the adapter for a specific platform
func (d *Detector) GetAdapter(platform string) ClientAdapter {
	return d.adapters[platform]
}
