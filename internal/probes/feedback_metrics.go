// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// PlaybackMetrics holds Prometheus-compatible metrics for playback feedback
type PlaybackMetrics struct {
	mu sync.RWMutex

	// Counters
	playbackTotal    map[string]map[string]int64 // deviceID -> outcome -> count
	reprobeTriggered map[string]map[string]int64 // deviceID -> reason -> count

	// Gauges
	trustScores map[string]float64 // deviceID -> trust score

	// Histograms
	feedbackLatencies []float64 // Recent feedback latencies in seconds

	// Global counters
	globalPlaybackTotal map[string]int64 // outcome -> count
	totalReProbes      int64
}

// NewPlaybackMetrics creates a new playback metrics instance
func NewPlaybackMetrics() *PlaybackMetrics {
	return &PlaybackMetrics{
		playbackTotal:      make(map[string]map[string]int64),
		reprobeTriggered:   make(map[string]map[string]int64),
		trustScores:        make(map[string]float64),
		feedbackLatencies: make([]float64, 0, 1000),
		globalPlaybackTotal: make(map[string]int64),
	}
}

// RecordPlayback records a playback event
func (m *PlaybackMetrics) RecordPlayback(deviceID string, outcome PlaybackOutcome) {
	m.mu.Lock()
	defer m.mu.Unlock()

	outcomeStr := outcome.String()

	// Initialize device maps if needed
	if _, ok := m.playbackTotal[deviceID]; !ok {
		m.playbackTotal[deviceID] = make(map[string]int64)
	}

	// Increment counters
	m.playbackTotal[deviceID][outcomeStr]++
	m.globalPlaybackTotal[outcomeStr]++
}

// RecordTrustScore updates the trust score gauge for a device
func (m *PlaybackMetrics) RecordTrustScore(deviceID string, score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.trustScores[deviceID] = score
}

// RecordReProbe records a re-probe trigger event
func (m *PlaybackMetrics) RecordReProbe(deviceID string, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize device map if needed
	if _, ok := m.reprobeTriggered[deviceID]; !ok {
		m.reprobeTriggered[deviceID] = make(map[string]int64)
	}

	m.reprobeTriggered[deviceID][reason]++
	m.totalReProbes++
}

// RecordFeedbackLatency records the latency of feedback processing
func (m *PlaybackMetrics) RecordFeedbackLatency(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	seconds := duration.Seconds()
	m.feedbackLatencies = append(m.feedbackLatencies, seconds)

	// Keep only recent latencies (last 1000)
	if len(m.feedbackLatencies) > 1000 {
		m.feedbackLatencies = m.feedbackLatencies[len(m.feedbackLatencies)-1000:]
	}
}

// PlaybackCounterValue represents the playback counter output
type PlaybackCounterValue struct {
	DeviceID string `json:"device_id"`
	Outcome  string `json:"outcome"`
	Count    int64  `json:"count"`
}

// TrustScoreValue represents the trust score gauge output
type TrustScoreValue struct {
	DeviceID string  `json:"device_id"`
	Score    float64 `json:"score"`
}

// ReProbeCounterValue represents the re-probe counter output
type ReProbeCounterValue struct {
	DeviceID string `json:"device_id"`
	Reason   string `json:"reason"`
	Count    int64  `json:"count"`
}

// LatencyHistogram represents latency histogram output
type LatencyHistogram struct {
	Count   int     `json:"count"`
	Sum     float64 `json:"sum"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Avg     float64 `json:"avg"`
	P50     float64 `json:"p50"`
	P90     float64 `json:"p90"`
	P99     float64 `json:"p99"`
}

// ExportMetrics exports all metrics in Prometheus-compatible format
func (m *PlaybackMetrics) ExportMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Export playback counters
	var playbackCounters []PlaybackCounterValue
	for deviceID, outcomes := range m.playbackTotal {
		for outcome, count := range outcomes {
			playbackCounters = append(playbackCounters, PlaybackCounterValue{
				DeviceID: deviceID,
				Outcome:  outcome,
				Count:    count,
			})
		}
	}

	// Export trust scores
	var trustScoreValues []TrustScoreValue
	for deviceID, score := range m.trustScores {
		trustScoreValues = append(trustScoreValues, TrustScoreValue{
			DeviceID: deviceID,
			Score:    score,
		})
	}

	// Export re-probe counters
	var reProbeCounters []ReProbeCounterValue
	for deviceID, reasons := range m.reprobeTriggered {
		for reason, count := range reasons {
			reProbeCounters = append(reProbeCounters, ReProbeCounterValue{
				DeviceID: deviceID,
				Reason:   reason,
				Count:    count,
			})
		}
	}

	// Export latency histogram
	latencyHistogram := m.calculateLatencyHistogram()

	return map[string]interface{}{
		"playback_total": playbackCounters,
		"trust_scores":   trustScoreValues,
		"reprobe_total":  reProbeCounters,
		"latency_histogram": latencyHistogram,
	}
}

// calculateLatencyHistogram calculates latency histogram statistics
func (m *PlaybackMetrics) calculateLatencyHistogram() LatencyHistogram {
	if len(m.feedbackLatencies) == 0 {
		return LatencyHistogram{}
	}

	var sum, min, max float64
	min = m.feedbackLatencies[0]
	max = m.feedbackLatencies[0]

	for _, latency := range m.feedbackLatencies {
		sum += latency
		if latency < min {
			min = latency
		}
		if latency > max {
			max = latency
		}
	}

	avg := sum / float64(len(m.feedbackLatencies))

	// Sort for percentiles using O(n log n) sort
	sorted := make([]float64, len(m.feedbackLatencies))
	copy(sorted, m.feedbackLatencies)
	sort.Float64s(sorted)

	p50 := sorted[len(sorted)*50/100]
	p90 := sorted[len(sorted)*90/100]
	p99 := sorted[len(sorted)*99/100]

	return LatencyHistogram{
		Count: len(m.feedbackLatencies),
		Sum:   sum,
		Min:   min,
		Max:   max,
		Avg:   avg,
		P50:   p50,
		P90:   p90,
		P99:   p99,
	}
}

// ExportPrometheusFormat exports metrics in Prometheus text format
func (m *PlaybackMetrics) ExportPrometheusFormat() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lines []string

	// Playback total counters
	lines = append(lines, "# HELP tenkile_playback_total Total playback attempts by device and outcome")
	lines = append(lines, "# TYPE tenkile_playback_total counter")
	for deviceID, outcomes := range m.playbackTotal {
		for outcome, count := range outcomes {
			lines = append(lines, formatPrometheusLine("tenkile_playback_total", count, "device_id", deviceID, "outcome", outcome))
		}
	}

	// Trust score gauges
	lines = append(lines, "")
	lines = append(lines, "# HELP tenkile_playback_trust_score Current trust score adjustment for device")
	lines = append(lines, "# TYPE tenkile_playback_trust_score gauge")
	for deviceID, score := range m.trustScores {
		lines = append(lines, formatPrometheusLine("tenkile_playback_trust_score", score, "device_id", deviceID))
	}

	// Re-probe counters
	lines = append(lines, "")
	lines = append(lines, "# HELP tenkile_reprobe_triggered_total Total re-probe triggers by device and reason")
	lines = append(lines, "# TYPE tenkile_reprobe_triggered_total counter")
	for deviceID, reasons := range m.reprobeTriggered {
		for reason, count := range reasons {
			lines = append(lines, formatPrometheusLine("tenkile_reprobe_triggered_total", count, "device_id", deviceID, "reason", reason))
		}
	}

	// Feedback latency histogram
	lines = append(lines, "")
	lines = append(lines, "# HELP tenkile_feedback_latency_seconds Feedback processing latency in seconds")
	lines = append(lines, "# TYPE tenkile_feedback_latency_seconds histogram")
	if len(m.feedbackLatencies) > 0 {
		// Bucket boundaries (in seconds)
		buckets := []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}
		bucketCounts := make([]int64, len(buckets))
		for _, latency := range m.feedbackLatencies {
			for i, bucket := range buckets {
				if latency <= bucket {
					bucketCounts[i]++
				}
			}
		}

		// Cumulative counts
		var cumulative int64
		for i, bucket := range buckets {
			cumulative += bucketCounts[i]
			lines = append(lines, formatPrometheusLine("tenkile_feedback_latency_seconds_bucket", float64(cumulative), "le", formatFloat(bucket)))
		}

		// +Inf bucket
		lines = append(lines, formatPrometheusLine("tenkile_feedback_latency_seconds_bucket", float64(len(m.feedbackLatencies)), "le", "+Inf"))

		// Sum and count
		var sum float64
		for _, latency := range m.feedbackLatencies {
			sum += latency
		}
		lines = append(lines, formatPrometheusLine("tenkile_feedback_latency_seconds_sum", sum))
		lines = append(lines, formatPrometheusLine("tenkile_feedback_latency_seconds_count", float64(len(m.feedbackLatencies))))
	}

	return joinLines(lines)
}

// formatPrometheusLine formats a Prometheus metric line
func formatPrometheusLine(name string, value interface{}, labels ...string) string {
	labelStr := ""
	for i := 0; i < len(labels); i += 2 {
		if i+1 < len(labels) {
			if labelStr != "" {
				labelStr += ","
			}
			labelStr += labels[i] + `="` + labels[i+1] + `"`
		}
	}

	if labelStr != "" {
		return name + "{" + labelStr + "} " + formatValue(value)
	}
	return name + " " + formatValue(value)
}

// formatValue formats a metric value
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case float64:
		return formatFloat(val)
	case int64:
		return formatInt(val)
	default:
		return "0"
	}
}

// formatFloat formats a float for Prometheus
func formatFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}

// formatInt formats an int64 for Prometheus
func formatInt(i int64) string {
	return fmt.Sprintf("%d", i)
}

// joinLines joins lines with newlines
func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// ResetMetrics resets all metrics
func (m *PlaybackMetrics) ResetMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.playbackTotal = make(map[string]map[string]int64)
	m.reprobeTriggered = make(map[string]map[string]int64)
	m.trustScores = make(map[string]float64)
	m.feedbackLatencies = make([]float64, 0, 1000)
	m.globalPlaybackTotal = make(map[string]int64)
	m.totalReProbes = 0
}

// GetGlobalPlaybackTotals returns global playback totals by outcome
func (m *PlaybackMetrics) GetGlobalPlaybackTotals() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for outcome, count := range m.globalPlaybackTotal {
		result[outcome] = count
	}
	return result
}

// GetTotalReProbes returns total re-probe count
func (m *PlaybackMetrics) GetTotalReProbes() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalReProbes
}

// Global metrics instance
var globalPlaybackMetrics = NewPlaybackMetrics()

// GetGlobalPlaybackMetrics returns the global metrics instance
func GetGlobalPlaybackMetrics() *PlaybackMetrics {
	return globalPlaybackMetrics
}
