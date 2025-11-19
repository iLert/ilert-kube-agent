package memory

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// Memory thresholds as percentages
	WarningThreshold   = 70.0
	CriticalThreshold  = 85.0
	EmergencyThreshold = 95.0

	CheckInterval = 10 * time.Second
)

var (
	monitorOnce sync.Once
	monitor     *Monitor
)

type Monitor struct {
	ctx           context.Context
	cancel        context.CancelFunc
	enabled       bool
	memoryLimitMB uint64
	lastGC        time.Time
	mu            sync.RWMutex
	underPressure bool
	pressureLevel string
}

func NewMonitor(memoryLimitMB uint64) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Monitor{
		ctx:           ctx,
		cancel:        cancel,
		enabled:       true,
		memoryLimitMB: memoryLimitMB,
		lastGC:        time.Now(),
	}
}

func (m *Monitor) Start() {
	if !m.enabled {
		return
	}

	log.Info().
		Uint64("memory_limit_mb", m.memoryLimitMB).
		Msg("Starting memory monitor")

	go m.monitorLoop()
}

func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	log.Info().Msg("Memory monitor stopped")
}

func (m *Monitor) IsUnderPressure() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.underPressure
}

func (m *Monitor) GetPressureLevel() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pressureLevel
}

func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkMemory()
		}
	}
}

func (m *Monitor) checkMemory() {
	var mstats runtime.MemStats
	runtime.ReadMemStats(&mstats)

	allocatedMB := mstats.Alloc / 1024 / 1024
	sysMB := mstats.Sys / 1024 / 1024

	var usagePercent float64
	if m.memoryLimitMB > 0 {
		usagePercent = (float64(sysMB) / float64(m.memoryLimitMB)) * 100
	} else {
		usagePercent = (float64(sysMB) / float64(sysMB*125/100)) * 100
	}

	m.mu.Lock()
	oldPressure := m.underPressure
	oldLevel := m.pressureLevel

	if usagePercent >= EmergencyThreshold {
		m.underPressure = true
		m.pressureLevel = "emergency"
	} else if usagePercent >= CriticalThreshold {
		m.underPressure = true
		m.pressureLevel = "critical"
	} else if usagePercent >= WarningThreshold {
		m.underPressure = true
		m.pressureLevel = "warning"
	} else {
		m.underPressure = false
		m.pressureLevel = "normal"
	}
	m.mu.Unlock()

	if !oldPressure && m.underPressure || oldLevel != m.pressureLevel {
		log.Warn().
			Uint64("allocated_mb", allocatedMB).
			Uint64("sys_mb", sysMB).
			Uint64("limit_mb", m.memoryLimitMB).
			Float64("usage_percent", usagePercent).
			Str("pressure_level", m.pressureLevel).
			Msg("Memory pressure detected")
	} else if m.underPressure {
		log.Debug().
			Uint64("allocated_mb", allocatedMB).
			Uint64("sys_mb", sysMB).
			Float64("usage_percent", usagePercent).
			Str("pressure_level", m.pressureLevel).
			Msg("Memory pressure ongoing")
	}

	if m.underPressure {
		m.handleMemoryPressure(usagePercent, sysMB)
	}
}

func (m *Monitor) handleMemoryPressure(usagePercent float64, sysMB uint64) {
	switch m.pressureLevel {
	case "emergency":
		log.Error().
			Float64("usage_percent", usagePercent).
			Msg("Emergency memory pressure - taking aggressive action")
		m.emergencyCleanup()
	case "critical":
		log.Warn().
			Float64("usage_percent", usagePercent).
			Msg("Critical memory pressure - clearing caches")
		m.criticalCleanup()
	case "warning":
		log.Info().
			Float64("usage_percent", usagePercent).
			Msg("Warning memory pressure - clearing LRU cache")
		m.warningCleanup()
	}

	// Force GC if it's been more than 30 seconds since last GC
	if time.Since(m.lastGC) > 30*time.Second {
		log.Debug().Msg("Forcing garbage collection")
		runtime.GC()
		m.lastGC = time.Now()
	}
}

func (m *Monitor) warningCleanup() {
	// LRU cache will automatically prune when it reaches MaxSize
	// No manual cleanup needed as ccache handles this automatically
	log.Debug().Msg("Warning cleanup - LRU cache will auto-prune when needed")
}

func (m *Monitor) criticalCleanup() {
	// Clear LRU cache
	m.warningCleanup()

	// Try to clear informer cache if possible
	// Note: We can't directly clear the informer cache, but we can
	// trigger a resync by stopping and restarting (too aggressive)
	// Instead, we'll just log and rely on GC
	log.Info().Msg("Critical memory pressure - consider reducing informer cache size")
}

func (m *Monitor) emergencyCleanup() {
	// Most aggressive cleanup
	m.criticalCleanup()

	// Force multiple GC cycles
	log.Warn().Msg("Emergency cleanup - forcing multiple GC cycles")
	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}
	m.lastGC = time.Now()

	// Log that we're in emergency mode
	log.Error().
		Msg("Emergency memory cleanup completed - if memory continues to grow, consider reducing cluster monitoring scope")
}

// GetGlobalMonitor returns the global memory monitor instance
func GetGlobalMonitor() *Monitor {
	return monitor
}

// GetMemoryLimitMB reads memory limit from cgroup or environment
// Returns 0 if limit cannot be determined
func GetMemoryLimitMB() uint64 {
	// Try environment variable first (for testing or manual override)
	if envLimit := os.Getenv("MEMORY_LIMIT_MB"); envLimit != "" {
		if limit, err := strconv.ParseUint(envLimit, 10, 64); err == nil {
			log.Info().Uint64("limit_mb", limit).Msg("Using memory limit from MEMORY_LIMIT_MB environment variable")
			return limit
		}
	}

	// Try cgroup v2 memory.max
	if limit := readCgroupV2MemoryLimit(); limit > 0 {
		log.Info().Uint64("limit_mb", limit).Msg("Using memory limit from cgroup v2")
		return limit
	}

	// Try cgroup v1 memory.limit_in_bytes
	if limit := readCgroupV1MemoryLimit(); limit > 0 {
		log.Info().Uint64("limit_mb", limit).Msg("Using memory limit from cgroup v1")
		return limit
	}

	// If no limit found, use a conservative default based on system memory
	// or return 0 to use percentage-based heuristics
	var mstats runtime.MemStats
	runtime.ReadMemStats(&mstats)
	sysMB := mstats.Sys / 1024 / 1024

	// Default to 80% of current system memory as a safety limit
	defaultLimit := sysMB * 125 / 100
	log.Info().
		Uint64("default_limit_mb", defaultLimit).
		Uint64("sys_mb", sysMB).
		Msg("No memory limit found, using default based on system memory")

	return defaultLimit
}

func readCgroupV2MemoryLimit() uint64 {
	// Try /sys/fs/cgroup/memory.max (cgroup v2)
	file, err := os.Open("/sys/fs/cgroup/memory.max")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0
	}

	line := strings.TrimSpace(scanner.Text())
	// "max" means no limit
	if line == "max" {
		return 0
	}

	limit, err := strconv.ParseUint(line, 10, 64)
	if err != nil {
		return 0
	}

	// Convert bytes to MB
	return limit / 1024 / 1024
}

func readCgroupV1MemoryLimit() uint64 {
	// Try /sys/fs/cgroup/memory/memory.limit_in_bytes (cgroup v1)
	file, err := os.Open("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		// Also try the pod-specific path in Kubernetes
		file, err = os.Open("/sys/fs/cgroup/memory/memory.limit_in_bytes")
		if err != nil {
			return 0
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0
	}

	line := strings.TrimSpace(scanner.Text())
	// Very large number (like 9223372036854771712) means no limit
	if strings.HasPrefix(line, "922337203685477") {
		return 0
	}

	limit, err := strconv.ParseUint(line, 10, 64)
	if err != nil {
		return 0
	}

	// Convert bytes to MB
	return limit / 1024 / 1024
}

// StartGlobalMonitor starts the global memory monitor
func StartGlobalMonitor(memoryLimitMB uint64) {
	monitorOnce.Do(func() {
		monitor = NewMonitor(memoryLimitMB)
		monitor.Start()
	})
}

// StopGlobalMonitor stops the global memory monitor
func StopGlobalMonitor() {
	if monitor != nil {
		monitor.Stop()
	}
}

// RecoverPanic recovers from a panic and logs it
func RecoverPanic(component string) {
	if r := recover(); r != nil {
		log.Error().
			Str("component", component).
			Interface("panic", r).
			Msg("Recovered from panic - application continues running")

		// Log stack trace
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		log.Error().
			Str("component", component).
			Str("stack", string(buf[:n])).
			Msg("Panic stack trace")
	}
}

// SafeGo runs a function in a goroutine with panic recovery
func SafeGo(component string, fn func()) {
	go func() {
		defer RecoverPanic(component)
		fn()
	}()
}
