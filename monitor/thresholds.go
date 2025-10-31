package monitor

import (
	"fmt"
	"log"
	"strconv"
)

// ThresholdViolation represents a threshold violation for a metric
type ThresholdViolation struct {
	Metric  string  `json:"metric"`
	Level   string  `json:"level"`
	Message string  `json:"message"`
	Value   float64 `json:"value"`
}

// CheckAllThresholds checks all metrics against configured thresholds with throttling
func CheckAllThresholds(config *Config, stats *SystemStats, stateManager *StateManager) ([]ThresholdViolation, []ThresholdViolation) {
	var allViolations []ThresholdViolation

	// Check disk thresholds
	diskViolations := checkDiskThresholds(config, stats)
	allViolations = append(allViolations, diskViolations...)

	// Check CPU thresholds
	cpuUsage, err := strconv.ParseFloat(stats.CPUInfo.TotalCPUUsage, 64)
	if err == nil {
		cpuViolations := checkCPUThresholds(config, cpuUsage)
		allViolations = append(allViolations, cpuViolations...)
	} else {
		log.Printf("Error parsing CPU usage: %v", err)
	}

	// Check memory thresholds
	memUsed, err := strconv.ParseFloat(stats.MemoryInfo.VirtualMemory.Percentage, 64)
	if err == nil {
		memFree := 100 - memUsed
		memViolations := checkMemoryThresholds(config, memUsed, memFree)
		allViolations = append(allViolations, memViolations...)
	} else {
		log.Printf("Error parsing memory usage: %v", err)
	}

	// Apply throttling
	throttledViolations := applyThrottling(config, allViolations, stateManager)

	// Clear resolved violations
	clearResolvedViolations(allViolations, stateManager)

	// Separate by level
	var warningViolations, criticalViolations []ThresholdViolation
	for _, v := range throttledViolations {
		if v.Level == "warning" {
			warningViolations = append(warningViolations, v)
		} else if v.Level == "critical" {
			criticalViolations = append(criticalViolations, v)
		}
	}

	log.Printf("Threshold check: %d warnings, %d critical (throttled from %d total)",
		len(warningViolations), len(criticalViolations), len(allViolations))

	// Save state
	stateManager.Save()

	return warningViolations, criticalViolations
}

// checkDiskThresholds checks disk usage against configured thresholds
func checkDiskThresholds(config *Config, stats *SystemStats) []ThresholdViolation {
	var violations []ThresholdViolation

	metricConfig, ok := config.GetMetricConfig("disk")
	if !ok || !metricConfig.Enabled {
		return violations
	}

	thresholds := metricConfig.Thresholds
	warningThreshold := thresholds["warning"]
	criticalThreshold := thresholds["critical"]

	for _, partition := range stats.DiskInfo.Partitions {
		percentage, err := strconv.ParseFloat(partition.Percentage, 64)
		if err != nil {
			log.Printf("Error parsing disk percentage: %v", err)
			continue
		}

		// Check critical first (higher severity)
		if criticalThreshold > 0 && percentage > criticalThreshold {
			message := fmt.Sprintf("partition %s, mounted at %s is %.2f%% full (critical threshold: %.2f%%)",
				partition.Device, partition.Mountpoint, percentage, criticalThreshold)
			violations = append(violations, ThresholdViolation{
				Metric:  "disk",
				Level:   "critical",
				Message: message,
				Value:   percentage,
			})
		} else if warningThreshold > 0 && percentage > warningThreshold {
			message := fmt.Sprintf("partition %s, mounted at %s is %.2f%% full (warning threshold: %.2f%%)",
				partition.Device, partition.Mountpoint, percentage, warningThreshold)
			violations = append(violations, ThresholdViolation{
				Metric:  "disk",
				Level:   "warning",
				Message: message,
				Value:   percentage,
			})
		}
	}

	return violations
}

// checkCPUThresholds checks CPU usage against configured thresholds
func checkCPUThresholds(config *Config, cpuUsage float64) []ThresholdViolation {
	var violations []ThresholdViolation

	metricConfig, ok := config.GetMetricConfig("cpu")
	if !ok || !metricConfig.Enabled {
		return violations
	}

	thresholds := metricConfig.Thresholds
	warningThreshold := thresholds["warning"]
	criticalThreshold := thresholds["critical"]

	// Check critical first
	if criticalThreshold > 0 && cpuUsage > criticalThreshold {
		message := fmt.Sprintf("cpu usage: %.2f%% (critical threshold: %.2f%%)", cpuUsage, criticalThreshold)
		violations = append(violations, ThresholdViolation{
			Metric:  "cpu",
			Level:   "critical",
			Message: message,
			Value:   cpuUsage,
		})
	} else if warningThreshold > 0 && cpuUsage > warningThreshold {
		message := fmt.Sprintf("cpu usage: %.2f%% (warning threshold: %.2f%%)", cpuUsage, warningThreshold)
		violations = append(violations, ThresholdViolation{
			Metric:  "cpu",
			Level:   "warning",
			Message: message,
			Value:   cpuUsage,
		})
	}

	return violations
}

// checkMemoryThresholds checks memory usage against configured thresholds
func checkMemoryThresholds(config *Config, memUsed float64, memFree float64) []ThresholdViolation {
	var violations []ThresholdViolation

	metricConfig, ok := config.GetMetricConfig("memory")
	if !ok || !metricConfig.Enabled {
		return violations
	}

	mode := metricConfig.Mode
	if mode == "" {
		mode = "min_free"
	}

	thresholds := metricConfig.Thresholds
	warningThreshold := thresholds["warning"]
	criticalThreshold := thresholds["critical"]

	if mode == "min_free" {
		// Thresholds represent minimum free memory percentage
		// Alert if free memory DROPS BELOW threshold
		freePercent := memFree

		// Check critical first (lower is worse for free memory)
		if criticalThreshold > 0 && freePercent < criticalThreshold {
			message := fmt.Sprintf("free memory: %.2f%% (critical threshold: below %.2f%%)",
				freePercent, criticalThreshold)
			violations = append(violations, ThresholdViolation{
				Metric:  "memory",
				Level:   "critical",
				Message: message,
				Value:   freePercent,
			})
		} else if warningThreshold > 0 && freePercent < warningThreshold {
			message := fmt.Sprintf("free memory: %.2f%% (warning threshold: below %.2f%%)",
				freePercent, warningThreshold)
			violations = append(violations, ThresholdViolation{
				Metric:  "memory",
				Level:   "warning",
				Message: message,
				Value:   freePercent,
			})
		}
	} else {
		// mode == "max_used"
		// Thresholds represent maximum used memory percentage
		// Alert if used memory EXCEEDS threshold
		usedPercent := memUsed

		if criticalThreshold > 0 && usedPercent > criticalThreshold {
			message := fmt.Sprintf("memory used: %.2f%% (critical threshold: %.2f%%)",
				usedPercent, criticalThreshold)
			violations = append(violations, ThresholdViolation{
				Metric:  "memory",
				Level:   "critical",
				Message: message,
				Value:   usedPercent,
			})
		} else if warningThreshold > 0 && usedPercent > warningThreshold {
			message := fmt.Sprintf("memory used: %.2f%% (warning threshold: %.2f%%)",
				usedPercent, warningThreshold)
			violations = append(violations, ThresholdViolation{
				Metric:  "memory",
				Level:   "warning",
				Message: message,
				Value:   usedPercent,
			})
		}
	}

	return violations
}

// applyThrottling applies throttling rules to violations
func applyThrottling(config *Config, violations []ThresholdViolation, stateManager *StateManager) []ThresholdViolation {
	var throttled []ThresholdViolation

	for _, violation := range violations {
		throttleConfig := config.GetThrottleConfig(violation.Metric)
		minDuration := throttleConfig.MinDurationMinutes
		repeat := throttleConfig.Repeat

		// Get or create state
		state := stateManager.GetOrCreate(violation.Metric, violation.Level)

		// Check if we should alert
		if state.ShouldAlert(minDuration, repeat) {
			throttled = append(throttled, violation)
			state.MarkAlerted()
			log.Printf("Throttle: %s/%s will alert (duration %.1fm >= %.1fm)",
				violation.Metric, violation.Level, state.DurationMinutes(), minDuration)
		} else {
			log.Printf("Throttle: %s/%s suppressed (duration %.1fm < %.1fm or repeat=false)",
				violation.Metric, violation.Level, state.DurationMinutes(), minDuration)
		}
	}

	return throttled
}

// clearResolvedViolations clears state for metrics that are no longer violating
func clearResolvedViolations(currentViolations []ThresholdViolation, stateManager *StateManager) {
	// Get currently violating metric/level combinations
	currentKeys := make(map[string]bool)
	for _, v := range currentViolations {
		key := fmt.Sprintf("%s_%s", v.Metric, v.Level)
		currentKeys[key] = true
	}

	// Get all state keys and check which ones are no longer violating
	var keysToClear []string
	for key := range stateManager.States {
		if !currentKeys[key] {
			keysToClear = append(keysToClear, key)
		}
	}

	// Clear non-violating states
	for _, key := range keysToClear {
		if state, ok := stateManager.States[key]; ok {
			stateManager.Clear(state.Metric, state.Level)
		}
	}
}
