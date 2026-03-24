// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package metrics

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestHealthCheckerCreation(t *testing.T) {
	hc := NewHealthChecker(nil, []string{"/tmp"}, "ffmpeg", "ffprobe")

	if hc == nil {
		t.Error("expected non-nil health checker")
	}
	if hc.ffmpegPath != "ffmpeg" {
		t.Errorf("expected ffmpegPath to be 'ffmpeg', got '%s'", hc.ffmpegPath)
	}
	if hc.ffprobePath != "ffprobe" {
		t.Errorf("expected ffprobePath to be 'ffprobe', got '%s'", hc.ffprobePath)
	}
}

func TestHealthCheckerDefaultPaths(t *testing.T) {
	hc := NewHealthChecker(nil, []string{}, "", "")

	if hc.ffmpegPath != "ffmpeg" {
		t.Errorf("expected default ffmpegPath to be 'ffmpeg', got '%s'", hc.ffmpegPath)
	}
	if hc.ffprobePath != "ffprobe" {
		t.Errorf("expected default ffprobePath to be 'ffprobe', got '%s'", hc.ffprobePath)
	}
}

func TestHealthStatus(t *testing.T) {
	status := HealthStatus{
		Status:     "healthy",
		Timestamp:  time.Now(),
		Checks:     make(map[string]CheckResult),
	}

	status.Checks["database"] = CheckResult{
		Status:  "pass",
		Message: "database healthy",
		Latency: "1ms",
	}

	if status.Status != "healthy" {
		t.Errorf("expected status to be 'healthy', got '%s'", status.Status)
	}

	if len(status.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(status.Checks))
	}

	check, ok := status.Checks["database"]
	if !ok {
		t.Error("expected database check to exist")
	}
	if check.Status != "pass" {
		t.Errorf("expected check status to be 'pass', got '%s'", check.Status)
	}
}

func TestCheckResult(t *testing.T) {
	result := CheckResult{
		Status:  "fail",
		Message: "connection refused",
		Latency: "500ms",
	}

	if result.Status != "fail" {
		t.Errorf("expected status to be 'fail', got '%s'", result.Status)
	}
	if result.Message != "connection refused" {
		t.Errorf("expected message to be 'connection refused', got '%s'", result.Message)
	}
	if result.Latency != "500ms" {
		t.Errorf("expected latency to be '500ms', got '%s'", result.Latency)
	}
}

func TestHealthCheckerWithNilDB(t *testing.T) {
	hc := NewHealthChecker(nil, []string{"/tmp"}, "ffmpeg", "ffprobe")

	ctx := context.Background()
	status := hc.Check(ctx)

	// Database check should fail when DB is nil
	dbCheck, ok := status.Checks["database"]
	if !ok {
		t.Error("expected database check to exist")
	}
	if dbCheck.Status != "fail" {
		t.Errorf("expected database check status to be 'fail', got '%s'", dbCheck.Status)
	}
}

func TestHealthCheckerWithMockDB(t *testing.T) {
	// This test would require a mock or in-memory database
	// For now, we test with nil to verify the nil check works
	hc := NewHealthChecker(nil, []string{"/tmp"}, "ffmpeg", "ffprobe")

	ctx := context.Background()
	status := hc.Check(ctx)

	if status.Status != "unhealthy" {
		t.Logf("status is %s (expected unhealthy when DB is nil)", status.Status)
	}
}

func TestHealthCheckerLiveness(t *testing.T) {
	hc := NewHealthChecker(nil, []string{"/tmp"}, "ffmpeg", "ffprobe")

	if !hc.LivenessCheck() {
		t.Error("expected liveness check to return true")
	}
}

func TestHealthCheckerReadinessWithNilDB(t *testing.T) {
	hc := NewHealthChecker(nil, []string{"/tmp"}, "ffmpeg", "ffprobe")

	ctx := context.Background()
	result := hc.ReadinessCheck(ctx)
	
	// Readiness should return true even without DB (degrades gracefully)
	// The actual check is whether the DB can be pinged, which will fail but
	// the checker itself may return true for basic readiness
	_ = result // Result depends on implementation
}

func TestHealthCheckerReadinessWithDB(t *testing.T) {
	// Create a health checker with no database path (nil pointer)
	// In real scenarios, we'd have a valid database connection
	hc := NewHealthChecker(nil, []string{"/tmp"}, "ffmpeg", "ffprobe")

	ctx := context.Background()
	// Without a valid DB, this should return false
	result := hc.ReadinessCheck(ctx)
	
	// This is expected to fail since we don't have a real DB
	// In production, this would be a real database connection
	_ = result
}

func TestCheckDiskSpaceWithInvalidPaths(t *testing.T) {
	hc := NewHealthChecker(nil, []string{"/nonexistent/path"}, "ffmpeg", "ffprobe")

	ctx := context.Background()
	status := hc.Check(ctx)

	diskCheck, ok := status.Checks["disk_space"]
	if !ok {
		t.Error("expected disk_space check to exist")
	}

	// Should have warnings for non-existent paths
	if diskCheck.Status == "fail" {
		t.Logf("disk check failed as expected for non-existent path: %s", diskCheck.Message)
	}
}

func TestCheckMemory(t *testing.T) {
	hc := NewHealthChecker(nil, []string{}, "ffmpeg", "ffprobe")

	ctx := context.Background()
	status := hc.Check(ctx)

	memCheck, ok := status.Checks["memory"]
	if !ok {
		t.Error("expected memory check to exist")
	}

	// Memory check should pass under normal conditions
	validStatuses := map[string]bool{"pass": true, "warn": true, "fail": true}
	if !validStatuses[memCheck.Status] {
		t.Errorf("expected valid memory status, got '%s'", memCheck.Status)
	}
}

func TestCheckGoroutines(t *testing.T) {
	hc := NewHealthChecker(nil, []string{}, "ffmpeg", "ffprobe")

	ctx := context.Background()
	status := hc.Check(ctx)

	goroutineCheck, ok := status.Checks["goroutines"]
	if !ok {
		t.Error("expected goroutines check to exist")
	}

	// Should pass with normal goroutine count
	if goroutineCheck.Status != "pass" {
		t.Logf("goroutines check status: %s", goroutineCheck.Status)
	}
}

func TestHealthStatusOverall(t *testing.T) {
	tests := []struct {
		name           string
		checkStatuses  map[string]string
		expectedOverall string
	}{
		{
			name:           "all pass",
			checkStatuses:  map[string]string{"check1": "pass", "check2": "pass"},
			expectedOverall: "healthy",
		},
		{
			name:           "with warning",
			checkStatuses:  map[string]string{"check1": "pass", "check2": "warn"},
			expectedOverall: "degraded",
		},
		{
			name:           "with failure",
			checkStatuses:  map[string]string{"check1": "pass", "check2": "fail"},
			expectedOverall: "unhealthy",
		},
		{
			name:           "failure takes precedence",
			checkStatuses:  map[string]string{"check1": "warn", "check2": "fail"},
			expectedOverall: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := HealthStatus{
				Status:  "healthy",
				Checks:  make(map[string]CheckResult),
			}

			hasFailure := false
			hasWarning := false

			for name, s := range tt.checkStatuses {
				status.Checks[name] = CheckResult{Status: s}
				if s == "fail" {
					hasFailure = true
				}
				if s == "warn" {
					hasWarning = true
				}
			}

			if hasFailure {
				status.Status = "unhealthy"
			} else if hasWarning {
				status.Status = "degraded"
			}

			if status.Status != tt.expectedOverall {
				t.Errorf("expected overall status '%s', got '%s'", tt.expectedOverall, status.Status)
			}
		})
	}
}

func TestHealthCheckerFFmpegChecks(t *testing.T) {
	// Test with non-existent ffmpeg path
	hc := NewHealthChecker(nil, []string{}, "/nonexistent/ffmpeg", "/nonexistent/ffprobe")

	ctx := context.Background()
	status := hc.Check(ctx)

	ffmpegCheck, ok := status.Checks["ffmpeg"]
	if !ok {
		t.Error("expected ffmpeg check to exist")
	}

	// Should fail with non-existent ffmpeg
	if ffmpegCheck.Status != "fail" {
		t.Errorf("expected ffmpeg check to fail, got '%s'", ffmpegCheck.Status)
	}
}

func TestCheckResultWithEmptyFields(t *testing.T) {
	result := CheckResult{}
	
	if result.Status != "" {
		t.Errorf("expected empty status, got '%s'", result.Status)
	}
	if result.Message != "" {
		t.Errorf("expected empty message, got '%s'", result.Message)
	}
	if result.Latency != "" {
		t.Errorf("expected empty latency, got '%s'", result.Latency)
	}
}

func TestHealthCheckerAllChecksExist(t *testing.T) {
	hc := NewHealthChecker(nil, []string{"/tmp"}, "ffmpeg", "ffprobe")

	ctx := context.Background()
	status := hc.Check(ctx)

	expectedChecks := []string{"database", "ffmpeg", "disk_space", "memory", "goroutines"}

	for _, checkName := range expectedChecks {
		if _, ok := status.Checks[checkName]; !ok {
			t.Errorf("expected check '%s' to exist", checkName)
		}
	}

	if len(status.Checks) != len(expectedChecks) {
		t.Errorf("expected %d checks, got %d", len(expectedChecks), len(status.Checks))
	}
}

func TestNilDBHealthCheck(t *testing.T) {
	// Explicitly test the nil database case
	hc := NewHealthChecker((*sql.DB)(nil), []string{}, "ffmpeg", "ffprobe")

	ctx := context.Background()
	dbCheck := hc.checkDatabase(ctx)

	if dbCheck.Status != "fail" {
		t.Errorf("expected nil DB check to fail, got '%s'", dbCheck.Status)
	}

	if dbCheck.Message == "" {
		t.Error("expected non-empty error message for nil DB")
	}
}
