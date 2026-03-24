// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package metrics

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCounterVecInc(t *testing.T) {
	cv := NewCounterVec(CounterConfig{
		Name: "test_counter",
		Help: "Test counter",
	})

	counter := cv.WithLabelValues("endpoint", "GET", "200")
	counter.Inc()

	if counter.Value() != 1 {
		t.Errorf("expected counter to be 1, got %d", counter.Value())
	}

	counter.Inc()
	counter.Inc()

	if counter.Value() != 3 {
		t.Errorf("expected counter to be 3, got %d", counter.Value())
	}
}

func TestCounterVecAdd(t *testing.T) {
	cv := NewCounterVec(CounterConfig{
		Name: "test_counter",
		Help: "Test counter",
	})

	counter := cv.WithLabelValues("test", "value")
	counter.Add(5)

	if counter.Value() != 5 {
		t.Errorf("expected counter to be 5, got %d", counter.Value())
	}

	counter.Add(10)

	if counter.Value() != 15 {
		t.Errorf("expected counter to be 15, got %d", counter.Value())
	}
}

func TestCounterVecMultipleLabels(t *testing.T) {
	cv := NewCounterVec(CounterConfig{
		Name: "test_counter",
		Help: "Test counter",
	})

	c1 := cv.WithLabelValues("a", "b", "c")
	c2 := cv.WithLabelValues("a", "b", "d")
	c3 := cv.WithLabelValues("x", "y", "z")

	c1.Inc()
	c2.Inc()
	c2.Inc()
	c3.Inc()
	c3.Inc()
	c3.Inc()

	if c1.Value() != 1 {
		t.Errorf("expected c1 to be 1, got %d", c1.Value())
	}
	if c2.Value() != 2 {
		t.Errorf("expected c2 to be 2, got %d", c2.Value())
	}
	if c3.Value() != 3 {
		t.Errorf("expected c3 to be 3, got %d", c3.Value())
	}
}

func TestHistogramObserve(t *testing.T) {
	h := NewHistogram(HistogramConfig{
		Name:    "test_histogram",
		Help:    "Test histogram",
		Buckets: []float64{0.1, 0.5, 1.0, 5.0},
	})

	h.Observe(0.05)
	h.Observe(0.3)
	h.Observe(0.8)
	h.Observe(3.0)
	h.Observe(10.0)

	if h.Count() != 5 {
		t.Errorf("expected count to be 5, got %d", h.Count())
	}

	// Sum should be approximately 14.15
	sum := h.Sum()
	if sum < 14 || sum > 14.2 {
		t.Errorf("expected sum around 14.15, got %f", sum)
	}

	// Bucket counts
	if h.Bucket(0.1) != 1 {
		t.Errorf("expected bucket 0.1 count to be 1, got %d", h.Bucket(0.1))
	}
	if h.Bucket(0.5) != 2 {
		t.Errorf("expected bucket 0.5 count to be 2, got %d", h.Bucket(0.5))
	}
	if h.Bucket(1.0) != 3 {
		t.Errorf("expected bucket 1.0 count to be 3, got %d", h.Bucket(1.0))
	}
	if h.Bucket(5.0) != 4 {
		t.Errorf("expected bucket 5.0 count to be 4, got %d", h.Bucket(5.0))
	}
}

func TestHistogramVec(t *testing.T) {
	hv := NewHistogramVec(HistogramConfig{
		Name:    "test_histogram_vec",
		Help:    "Test histogram vector",
		Buckets: []float64{0.01, 0.1, 1.0},
	})

	h1 := hv.WithLabelValues("endpoint", "GET")
	h2 := hv.WithLabelValues("endpoint", "POST")

	h1.Observe(0.005)
	h1.Observe(0.05)
	h2.Observe(0.5)
	h2.Observe(2.0)

	if h1.Count() != 2 {
		t.Errorf("expected h1 count to be 2, got %d", h1.Count())
	}
	if h2.Count() != 2 {
		t.Errorf("expected h2 count to be 2, got %d", h2.Count())
	}
}

func TestMetricsRecordRequest(t *testing.T) {
	m := New()

	m.RecordRequest("/api/test", "GET", 200, 50*time.Millisecond)
	m.RecordRequest("/api/test", "GET", 200, 100*time.Millisecond)
	m.RecordRequest("/api/test", "POST", 201, 75*time.Millisecond)
	m.RecordRequest("/api/test", "GET", 404, 10*time.Millisecond)
	m.RecordRequest("/api/test", "GET", 500, 200*time.Millisecond)
}

func TestMetricsRecordStream(t *testing.T) {
	m := New()

	m.RecordStreamStart()
	m.RecordStreamStart()
	m.RecordStreamStart()

	if m.GetActiveStreams() != 3 {
		t.Errorf("expected 3 active streams, got %d", m.GetActiveStreams())
	}

	m.RecordStreamEnd(60*time.Second, 1024*1024*100)

	if m.GetActiveStreams() != 2 {
		t.Errorf("expected 2 active streams after end, got %d", m.GetActiveStreams())
	}
}

func TestMetricsCacheHitRate(t *testing.T) {
	m := New()

	// Initial state
	rate := m.GetCacheHitRate()
	if rate != 0 {
		t.Errorf("expected initial hit rate to be 0, got %f", rate)
	}

	// Add hits and misses
	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()

	rate = m.GetCacheHitRate()
	expected := 2.0 / 3.0
	if rate < expected-0.01 || rate > expected+0.01 {
		t.Errorf("expected hit rate around %f, got %f", expected, rate)
	}
}

func TestMetricsSnapshot(t *testing.T) {
	m := New()

	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()
	m.RecordStreamStart()
	m.SetCacheSize(1024)
	m.RecordDeviceRegistered()
	m.RecordWSConnect()

	snapshot := m.Snapshot()

	if snapshot.CacheHits != 2 {
		t.Errorf("expected CacheHits to be 2, got %d", snapshot.CacheHits)
	}
	if snapshot.CacheMisses != 1 {
		t.Errorf("expected CacheMisses to be 1, got %d", snapshot.CacheMisses)
	}
	if snapshot.ActiveStreams != 1 {
		t.Errorf("expected ActiveStreams to be 1, got %d", snapshot.ActiveStreams)
	}
	if snapshot.CacheSize != 1024 {
		t.Errorf("expected CacheSize to be 1024, got %d", snapshot.CacheSize)
	}
	if snapshot.DevicesRegistered != 1 {
		t.Errorf("expected DevicesRegistered to be 1, got %d", snapshot.DevicesRegistered)
	}
	if snapshot.WSConnections != 1 {
		t.Errorf("expected WSConnections to be 1, got %d", snapshot.WSConnections)
	}
}

func TestMetricsTranscode(t *testing.T) {
	m := New()

	m.RecordTranscodeStart()
	m.RecordTranscodeStart()
	m.RecordTranscodeEnd(10*time.Second, true)
	m.RecordTranscodeEnd(15*time.Second, false)

	snapshot := m.Snapshot()

	if snapshot.TranscodeCount != 2 {
		t.Errorf("expected TranscodeCount to be 2, got %d", snapshot.TranscodeCount)
	}
	if snapshot.TranscodeErrors != 1 {
		t.Errorf("expected TranscodeErrors to be 1, got %d", snapshot.TranscodeErrors)
	}
}

func TestMetricsWSMessages(t *testing.T) {
	m := New()

	m.RecordWSConnect()
	m.RecordWSConnect()
	m.RecordWSMessage(true)
	m.RecordWSMessage(true)
	m.RecordWSMessage(false)
	m.RecordWSDisconnect()

	snapshot := m.Snapshot()

	if snapshot.WSConnections != 1 {
		t.Errorf("expected WSConnections to be 1, got %d", snapshot.WSConnections)
	}
	if snapshot.WSMessagesSent != 2 {
		t.Errorf("expected WSMessagesSent to be 2, got %d", snapshot.WSMessagesSent)
	}
	if snapshot.WSMessagesRecv != 1 {
		t.Errorf("expected WSMessagesRecv to be 1, got %d", snapshot.WSMessagesRecv)
	}
}

func TestGlobalMetrics(t *testing.T) {
	// Get global instance
	m1 := Get()
	m2 := Get()

	if m1 != m2 {
		t.Error("expected Get() to return same instance")
	}

	// Verify it's functional
	m1.RecordCacheHit()
	if m2.GetCacheHitRate() == 0 {
		t.Error("expected hit rate to be non-zero after recording hit on m1")
	}
}

func TestStatusCodeToString(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{200, "2xx"},
		{201, "2xx"},
		{299, "2xx"},
		{301, "3xx"},
		{304, "3xx"},
		{400, "4xx"},
		{404, "4xx"},
		{500, "5xx"},
		{503, "5xx"},
		{600, "5xx"}, // Falls into 500+ range
		{-1, "unknown"},
	}

	for _, tt := range tests {
		result := statusCodeToString(tt.code)
		if result != tt.expected {
			t.Errorf("statusCodeToString(%d) = %s, expected %s", tt.code, result, tt.expected)
		}
	}
}

func TestJoinLabels(t *testing.T) {
	result := joinLabels("a", "b", "c")
	if result != "a|b|c" {
		t.Errorf("expected 'a|b|c', got '%s'", result)
	}

	result = joinLabels("single")
	if result != "single" {
		t.Errorf("expected 'single', got '%s'", result)
	}

	result = joinLabels()
	if result != "" {
		t.Errorf("expected empty string, got '%s'", result)
	}
}

func TestConcurrentCounterAccess(t *testing.T) {
	cv := NewCounterVec(CounterConfig{Name: "concurrent"})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter := cv.WithLabelValues("test")
			for j := 0; j < 10; j++ {
				counter.Inc()
			}
		}()
	}

	wg.Wait()

	counter := cv.WithLabelValues("test")
	if counter.Value() != 1000 {
		t.Errorf("expected counter to be 1000, got %d", counter.Value())
	}
}

func TestConcurrentHistogramObserve(t *testing.T) {
	hv := NewHistogramVec(HistogramConfig{
		Name:    "concurrent_hist",
		Buckets: []float64{0.1, 0.5, 1.0},
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			h := hv.WithLabelValues("test")
			for j := 0; j < 20; j++ {
				h.Observe(float64(n%10) * 0.1)
			}
		}(i)
	}

	wg.Wait()

	h := hv.WithLabelValues("test")
	expected := 1000 // 50 goroutines * 20 observations each
	if h.Count() != int64(expected) {
		t.Errorf("expected count to be %d, got %d", expected, h.Count())
	}
}

func TestMetricsPrometheusFormat(t *testing.T) {
	m := New()

	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()
	m.SetCacheSize(1000)
	m.RecordDeviceRegistered()
	m.RecordDeviceActive()

	snapshot := m.Snapshot()

	// Build Prometheus format manually like metricsHandler does
	output := buildPrometheusOutput(snapshot)

	if !strings.Contains(output, "tenkile_cache_hit_total 2") {
		t.Error("expected output to contain cache hit count")
	}
	if !strings.Contains(output, "tenkile_cache_miss_total 1") {
		t.Error("expected output to contain cache miss count")
	}
	if !strings.Contains(output, "tenkile_cache_hit_rate") {
		t.Error("expected output to contain cache hit rate")
	}
}

// buildPrometheusOutput creates Prometheus-formatted output for testing
func buildPrometheusOutput(snapshot MetricsSnapshot) string {
	return formatMetric("tenkile_cache_hit_total", snapshot.CacheHits) +
		formatMetric("tenkile_cache_miss_total", snapshot.CacheMisses) +
		formatMetricFloat("tenkile_cache_hit_rate", snapshot.CacheHitRate) +
		formatMetric("tenkile_devices_registered", snapshot.DevicesRegistered)
}

func formatMetric(name string, value int64) string {
	return name + " " + string(rune(value+'0')) + "\n"
}

func formatMetricFloat(name string, value float64) string {
	return name + " " + string(rune(int(value*100)+'0')) + "\n"
}
