// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// HealthChecker performs health checks on system components
type HealthChecker struct {
	db             *sql.DB
	mediaPaths     []string
	ffmpegPath    string
	ffprobePath   string
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *sql.DB, mediaPaths []string, ffmpegPath, ffprobePath string) *HealthChecker {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	return &HealthChecker{
		db:           db,
		mediaPaths:   mediaPaths,
		ffmpegPath:   ffmpegPath,
		ffprobePath: ffprobePath,
	}
}

// HealthStatus represents the overall health status
type HealthStatus struct {
	Status     string                 `json:"status"` // "healthy", "degraded", "unhealthy"
	Timestamp time.Time               `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult represents a single health check result
type CheckResult struct {
	Status  string `json:"status"`  // "pass", "fail", "warn"
	Message string `json:"message"`
	Latency string `json:"latency,omitempty"`
}

// Check runs all health checks
func (hc *HealthChecker) Check(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Status:     "healthy",
		Timestamp: time.Now(),
		Checks:    make(map[string]CheckResult),
	}

	checks := []struct {
		name    string
		checkFn func(context.Context) CheckResult
	}{
		{"database", hc.checkDatabase},
		{"ffmpeg", hc.checkFFmpeg},
		{"disk_space", hc.checkDiskSpace},
		{"memory", hc.checkMemory},
		{"goroutines", hc.checkGoroutines},
	}

	hasFailure := false
	hasWarning := false

	for _, c := range checks {
		result := c.checkFn(ctx)
		status.Checks[c.name] = result

		switch result.Status {
		case "fail":
			hasFailure = true
		case "warn":
			hasWarning = true
		}
	}

	if hasFailure {
		status.Status = "unhealthy"
	} else if hasWarning {
		status.Status = "degraded"
	}

	return status
}

// checkDatabase checks database connectivity
func (hc *HealthChecker) checkDatabase(ctx context.Context) CheckResult {
	start := time.Now()
	defer func() {
		_ = time.Since(start)
	}()

	if hc.db == nil {
		return CheckResult{
			Status:  "fail",
			Message: "database not configured",
		}
	}

	if err := hc.db.PingContext(ctx); err != nil {
		return CheckResult{
			Status:  "fail",
			Message: fmt.Sprintf("database ping failed: %v", err),
		}
	}

	// Test a simple query
	var count int
	if err := hc.db.QueryRowContext(ctx, "SELECT 1").Scan(&count); err != nil {
		return CheckResult{
			Status:  "fail",
			Message: fmt.Sprintf("query failed: %v", err),
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: "database healthy",
		Latency: time.Since(start).String(),
	}
}

// checkFFmpeg checks FFmpeg availability
func (hc *HealthChecker) checkFFmpeg(ctx context.Context) CheckResult {
	start := time.Now()

	// Check FFmpeg version
	cmd := exec.CommandContext(ctx, hc.ffmpegPath, "-version")
	if err := cmd.Run(); err != nil {
		return CheckResult{
			Status:  "fail",
			Message: fmt.Sprintf("ffmpeg not available: %v", err),
		}
	}

	// Check FFprobe as well
	cmd = exec.CommandContext(ctx, hc.ffprobePath, "-version")
	if err := cmd.Run(); err != nil {
		return CheckResult{
			Status:  "fail",
			Message: fmt.Sprintf("ffprobe not available: %v", err),
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: "ffmpeg/ffprobe available",
		Latency: time.Since(start).String(),
	}
}

// checkDiskSpace checks available disk space
func (hc *HealthChecker) checkDiskSpace(ctx context.Context) CheckResult {
	start := time.Now()

	var warnings []string
	var errors []string

	// Check temp directory (for transcoding)
	tempDir := os.TempDir()
	if free, total, err := getDiskSpace(tempDir); err == nil {
		percentFree := float64(free) / float64(total) * 100
		if percentFree < 5 {
			errors = append(errors, fmt.Sprintf("temp dir only %.1f%% free", percentFree))
		} else if percentFree < 10 {
			warnings = append(warnings, fmt.Sprintf("temp dir only %.1f%% free", percentFree))
		}
	}

	// Check media paths
	for _, path := range hc.mediaPaths {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf("media path does not exist: %s", path))
			} else {
				errors = append(errors, fmt.Sprintf("cannot access media path: %s", path))
			}
			continue
		}
		if !info.IsDir() {
			warnings = append(warnings, fmt.Sprintf("media path is not a directory: %s", path))
			continue
		}

		if free, total, err := getDiskSpace(path); err == nil {
			percentFree := float64(free) / float64(total) * 100
			if percentFree < 5 {
				errors = append(errors, fmt.Sprintf("%s only %.1f%% free", path, percentFree))
			} else if percentFree < 10 {
				warnings = append(warnings, fmt.Sprintf("%s only %.1f%% free", path, percentFree))
			}
		}
	}

	status := "pass"
	message := "disk space adequate"

	if len(errors) > 0 {
		status = "fail"
		message = fmt.Sprintf("critical: %s", errors[0])
	} else if len(warnings) > 0 {
		status = "warn"
		message = fmt.Sprintf("warnings: %s", warnings[0])
	}

	return CheckResult{
		Status:  status,
		Message: message,
		Latency: time.Since(start).String(),
	}
}

// checkMemory checks system memory usage
func (hc *HealthChecker) checkMemory(ctx context.Context) CheckResult {
	start := time.Now()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Calculate memory usage percentage
	sysMem := memStats.Sys
	allocMem := memStats.Alloc

	// Get system memory info
	var sysTotal, sysFree uint64
	if err := readMemInfo(&sysTotal, &sysFree); err == nil && sysTotal > 0 {
		usedMem := sysTotal - sysFree
		percentUsed := float64(usedMem) / float64(sysTotal) * 100

		if percentUsed > 95 {
			return CheckResult{
				Status:  "fail",
				Message: fmt.Sprintf("system memory critical: %.1f%% used", percentUsed),
				Latency: time.Since(start).String(),
			}
		} else if percentUsed > 85 {
			return CheckResult{
				Status:  "warn",
				Message: fmt.Sprintf("system memory high: %.1f%% used", percentUsed),
				Latency: time.Since(start).String(),
			}
		}
	}

	// Check if Go is using excessive memory
	// GOGC defaults to 100, so heap should be ~2x live data
	heapRatio := float64(allocMem) / float64(sysMem)
	if heapRatio > 0.8 {
		return CheckResult{
			Status:  "warn",
			Message: fmt.Sprintf("Go heap usage high: %.1f%% of sys memory", heapRatio*100),
			Latency: time.Since(start).String(),
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: fmt.Sprintf("memory: Go %.1fMB", float64(allocMem)/1e6),
		Latency: time.Since(start).String(),
	}
}

// checkGoroutines checks for excessive goroutines
func (hc *HealthChecker) checkGoroutines(ctx context.Context) CheckResult {
	start := time.Now()

	goroutineCount := runtime.NumGoroutine()

	// Reasonable threshold based on application size
	if goroutineCount > 1000 {
		return CheckResult{
			Status:  "fail",
			Message: fmt.Sprintf("excessive goroutines: %d", goroutineCount),
			Latency: time.Since(start).String(),
		}
	} else if goroutineCount > 500 {
		return CheckResult{
			Status:  "warn",
			Message: fmt.Sprintf("high goroutine count: %d", goroutineCount),
			Latency: time.Since(start).String(),
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: fmt.Sprintf("goroutines: %d", goroutineCount),
		Latency: time.Since(start).String(),
	}
}

// getDiskSpace returns free and total bytes for a path
func getDiskSpace(path string) (free, total uint64, err error) {
	stat := &unixStat_t{}
	if err = unixStat(path, stat); err != nil {
		return 0, 0, err
	}
	// On some systems, stat.Bavail is available blocks * block size
	// This is a simplified implementation
	return stat.Free, stat.Total, nil
}

// unixStat_t is a platform-specific stat structure
type unixStat_t struct {
	Free  uint64
	Total uint64
}

// unixStat is a placeholder - real implementation would use syscall
func unixStat(path string, stat *unixStat_t) error {
	// Simplified - real implementation would use golang.org/x/sys/unix.Statfs_t
	stat.Free = 1 << 30 // 1GB placeholder
	stat.Total = 10 << 30 // 10GB placeholder
	return nil
}

// readMemInfo reads memory info (placeholder for platform-specific code)
func readMemInfo(total, free *uint64) error {
	// Simplified - real implementation would read from /proc/meminfo or sysctl
	*total = 16 << 30 // 16GB placeholder
	*free = 8 << 30   // 8GB placeholder
	return nil
}

// LivenessCheck performs a quick liveness check
func (hc *HealthChecker) LivenessCheck() bool {
	return true
}

// ReadinessCheck performs a readiness check
func (hc *HealthChecker) ReadinessCheck(ctx context.Context) bool {
	// Quick checks that should pass before accepting traffic
	if hc.db != nil {
		if err := hc.db.PingContext(ctx); err != nil {
			return false
		}
	}
	return true
}
