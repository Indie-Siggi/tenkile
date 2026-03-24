// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds all application metrics
type Metrics struct {
	// Request metrics
	requestDuration *HistogramVec
	requestCount    *CounterVec

	// Stream metrics
	activeStreams   atomic.Int64
	streamBytes    atomic.Int64
	streamDuration *Histogram

	// Cache metrics
	cacheHits   atomic.Int64
	cacheMisses atomic.Int64
	cacheSize   atomic.Int64

	// Transcode metrics
	transcodeDuration *Histogram
	transcodeCount    atomic.Int64
	transcodeErrors   atomic.Int64

	// Device metrics
	devicesRegistered atomic.Int64
	devicesActive     atomic.Int64

	// WebSocket metrics
	wsConnections   atomic.Int64
	wsMessagesSent  atomic.Int64
	wsMessagesRecv  atomic.Int64

	mu sync.RWMutex
}

// Global metrics instance
var global *Metrics
var initOnce sync.Once

// Get returns the global metrics instance
func Get() *Metrics {
	initOnce.Do(func() {
		global = New()
	})
	return global
}

// New creates a new Metrics instance
func New() *Metrics {
	return &Metrics{
		requestDuration: NewHistogramVec(HistogramConfig{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}),
		requestCount: NewCounterVec(CounterConfig{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		}),
		streamDuration: NewHistogram(HistogramConfig{
			Name:    "stream_duration_seconds",
			Help:    "Stream duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
		}),
		transcodeDuration: NewHistogram(HistogramConfig{
			Name:    "transcode_duration_seconds",
			Help:    "Transcode duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
		}),
	}
}

// RecordRequest records an HTTP request
func (m *Metrics) RecordRequest(endpoint string, method string, status int, duration time.Duration) {
	m.requestDuration.WithLabelValues(endpoint, method, statusCodeToString(status)).Observe(duration.Seconds())
	m.requestCount.WithLabelValues(endpoint, method, statusCodeToString(status)).Inc()
}

// RecordStreamStart records a stream starting
func (m *Metrics) RecordStreamStart() {
	m.activeStreams.Add(1)
}

// RecordStreamEnd records a stream ending
func (m *Metrics) RecordStreamEnd(duration time.Duration, bytes int64) {
	m.activeStreams.Add(-1)
	m.streamDuration.Observe(duration.Seconds())
	m.streamBytes.Add(bytes)
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	m.cacheHits.Add(1)
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	m.cacheMisses.Add(1)
}

// SetCacheSize sets the current cache size
func (m *Metrics) SetCacheSize(size int64) {
	m.cacheSize.Store(size)
}

// RecordTranscodeStart records a transcode starting
func (m *Metrics) RecordTranscodeStart() {
	m.transcodeCount.Add(1)
}

// RecordTranscodeEnd records a transcode ending
func (m *Metrics) RecordTranscodeEnd(duration time.Duration, success bool) {
	m.transcodeDuration.Observe(duration.Seconds())
	if !success {
		m.transcodeErrors.Add(1)
	}
}

// RecordDeviceRegistered records a new device registration
func (m *Metrics) RecordDeviceRegistered() {
	m.devicesRegistered.Add(1)
}

// RecordDeviceActive records a device becoming active
func (m *Metrics) RecordDeviceActive() {
	m.devicesActive.Add(1)
}

// RecordWSConnect records a WebSocket connection
func (m *Metrics) RecordWSConnect() {
	m.wsConnections.Add(1)
}

// RecordWSDisconnect records a WebSocket disconnection
func (m *Metrics) RecordWSDisconnect() {
	m.wsConnections.Add(-1)
}

// RecordWSMessage records a WebSocket message
func (m *Metrics) RecordWSMessage(sent bool) {
	if sent {
		m.wsMessagesSent.Add(1)
	} else {
		m.wsMessagesRecv.Add(1)
	}
}

// GetActiveStreams returns the number of active streams
func (m *Metrics) GetActiveStreams() int64 {
	return m.activeStreams.Load()
}

// GetCacheHitRate returns the cache hit rate (0-1)
func (m *Metrics) GetCacheHitRate() float64 {
	hits := m.cacheHits.Load()
	misses := m.cacheMisses.Load()
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// Snapshot returns a point-in-time snapshot of all metrics
func (m *Metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		ActiveStreams:   m.activeStreams.Load(),
		StreamBytes:    m.streamBytes.Load(),
		CacheHits:      m.cacheHits.Load(),
		CacheMisses:    m.cacheMisses.Load(),
		CacheSize:      m.cacheSize.Load(),
		CacheHitRate:   m.GetCacheHitRate(),
		TranscodeCount: m.transcodeCount.Load(),
		TranscodeErrors: m.transcodeErrors.Load(),
		DevicesRegistered: m.devicesRegistered.Load(),
		DevicesActive:   m.devicesActive.Load(),
		WSConnections:  m.wsConnections.Load(),
		WSMessagesSent:  m.wsMessagesSent.Load(),
		WSMessagesRecv:  m.wsMessagesRecv.Load(),
	}
}

// MetricsSnapshot holds a point-in-time copy of metrics
type MetricsSnapshot struct {
	ActiveStreams     int64   `json:"active_streams"`
	StreamBytes      int64   `json:"stream_bytes"`
	CacheHits        int64   `json:"cache_hits"`
	CacheMisses      int64   `json:"cache_misses"`
	CacheSize        int64   `json:"cache_size"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	TranscodeCount   int64   `json:"transcode_count"`
	TranscodeErrors  int64   `json:"transcode_errors"`
	DevicesRegistered int64  `json:"devices_registered"`
	DevicesActive    int64   `json:"devices_active"`
	WSConnections    int64   `json:"ws_connections"`
	WSMessagesSent   int64   `json:"ws_messages_sent"`
	WSMessagesRecv   int64   `json:"ws_messages_recv"`
}

// Counter is a simple counter
type Counter struct {
	value atomic.Int64
}

// CounterVec is a counter with labels
type CounterVec struct {
	counters map[string]*Counter
	mu       sync.RWMutex
}

// CounterConfig holds counter configuration
type CounterConfig struct {
	Name string
	Help string
}

// NewCounterVec creates a new counter vector
func NewCounterVec(config CounterConfig) *CounterVec {
	return &CounterVec{
		counters: make(map[string]*Counter),
	}
}

// WithLabelValues returns the counter for the given labels
func (cv *CounterVec) WithLabelValues(labels ...string) *Counter {
	key := joinLabels(labels...)
	cv.mu.RLock()
	c, ok := cv.counters[key]
	cv.mu.RUnlock()
	if ok {
		return c
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	if c, ok = cv.counters[key]; ok {
		return c
	}
	c = &Counter{}
	cv.counters[key] = c
	return c
}

// Inc increments the counter
func (c *Counter) Inc() {
	c.value.Add(1)
}

// Add adds a value to the counter
func (c *Counter) Add(v int64) {
	c.value.Add(v)
}

// Value returns the current value
func (c *Counter) Value() int64 {
	return c.value.Load()
}

// Histogram tracks value distributions
type Histogram struct {
	count   atomic.Int64
	sum     atomic.Value // float64
	buckets map[float64]*atomic.Int64
	config  HistogramConfig
	mu      sync.RWMutex
}

// HistogramVec is a histogram with labels
type HistogramVec struct {
	histograms map[string]*Histogram
	mu         sync.RWMutex
	config     HistogramConfig
}

// HistogramConfig holds histogram configuration
type HistogramConfig struct {
	Name    string
	Help    string
	Buckets []float64
}

// NewHistogramVec creates a new histogram vector
func NewHistogramVec(config HistogramConfig) *HistogramVec {
	return &HistogramVec{
		histograms: make(map[string]*Histogram),
		config:     config,
	}
}

// WithLabelValues returns the histogram for the given labels
func (hv *HistogramVec) WithLabelValues(labels ...string) *Histogram {
	key := joinLabels(labels...)
	hv.mu.RLock()
	h, ok := hv.histograms[key]
	hv.mu.RUnlock()
	if ok {
		return h
	}
	hv.mu.Lock()
	defer hv.mu.Unlock()
	if h, ok = hv.histograms[key]; ok {
		return h
	}
	h = NewHistogram(hv.config)
	hv.histograms[key] = h
	return h
}

// NewHistogram creates a new histogram
func NewHistogram(config HistogramConfig) *Histogram {
	buckets := make(map[float64]*atomic.Int64)
	for _, b := range config.Buckets {
		buckets[b] = &atomic.Int64{}
	}
	return &Histogram{
		buckets: buckets,
		config:  config,
	}
}

// Observe records an observation
func (h *Histogram) Observe(v float64) {
	h.count.Add(1)
	
	// Use CAS for float64 sum
	for {
		oldPtr := h.sum.Load()
		old := 0.0
		if oldPtr != nil {
			old = oldPtr.(float64)
		}
		newPtr := old + v
		if h.sum.CompareAndSwap(oldPtr, newPtr) {
			break
		}
	}
	
	h.mu.RLock()
	defer h.mu.RUnlock()
	for bucketBound, bucketCount := range h.buckets {
		if v <= bucketBound {
			bucketCount.Add(1)
		}
	}
}

// Count returns the total count
func (h *Histogram) Count() int64 {
	return h.count.Load()
}

// Sum returns the total sum
func (h *Histogram) Sum() float64 {
	ptr := h.sum.Load()
	if ptr == nil {
		return 0
	}
	return ptr.(float64)
}

// Bucket returns the count for a bucket
func (h *Histogram) Bucket(le float64) int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if count, ok := h.buckets[le]; ok {
		return count.Load()
	}
	return 0
}

// statusCodeToString converts status code to string bucket
func statusCodeToString(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

// joinLabels joins label values into a key
func joinLabels(labels ...string) string {
	result := ""
	for i, l := range labels {
		if i > 0 {
			result += "|"
		}
		result += l
	}
	return result
}
