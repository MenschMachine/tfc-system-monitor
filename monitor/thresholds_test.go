package monitor

import (
	"testing"
)

// TestCheckDiskThresholds tests disk threshold checking
func TestCheckDiskThresholds(t *testing.T) {
	tests := []struct {
		name              string
		config            *Config
		stats             *SystemStats
		expectedViolations int
		expectedLevel     string
	}{
		{
			name: "disk disabled",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"disk": {
						Enabled: false,
					},
				},
			},
			stats: &SystemStats{
				DiskInfo: DiskInfo{
					Partitions: []PartitionInfo{
						{
							Device:     "/dev/sda1",
							Mountpoint: "/",
							Percentage: "85",
							FSType:     "ext4",
						},
					},
				},
			},
			expectedViolations: 0,
		},
		{
			name: "disk below warning threshold",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"disk": {
						Enabled: true,
						Thresholds: map[string]float64{
							"warning":  80,
							"critical": 90,
						},
					},
				},
			},
			stats: &SystemStats{
				DiskInfo: DiskInfo{
					Partitions: []PartitionInfo{
						{
							Device:     "/dev/sda1",
							Mountpoint: "/",
							Percentage: "70",
							FSType:     "ext4",
						},
					},
				},
			},
			expectedViolations: 0,
		},
		{
			name: "disk warning threshold",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"disk": {
						Enabled: true,
						Thresholds: map[string]float64{
							"warning":  80,
							"critical": 90,
						},
					},
				},
			},
			stats: &SystemStats{
				DiskInfo: DiskInfo{
					Partitions: []PartitionInfo{
						{
							Device:     "/dev/sda1",
							Mountpoint: "/",
							Percentage: "85",
							FSType:     "ext4",
						},
					},
				},
			},
			expectedViolations: 1,
			expectedLevel:      "warning",
		},
		{
			name: "disk critical threshold",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"disk": {
						Enabled: true,
						Thresholds: map[string]float64{
							"warning":  80,
							"critical": 90,
						},
					},
				},
			},
			stats: &SystemStats{
				DiskInfo: DiskInfo{
					Partitions: []PartitionInfo{
						{
							Device:     "/dev/sda1",
							Mountpoint: "/",
							Percentage: "95",
							FSType:     "ext4",
						},
					},
				},
			},
			expectedViolations: 1,
			expectedLevel:      "critical",
		},
		{
			name: "multiple disk partitions",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"disk": {
						Enabled: true,
						Thresholds: map[string]float64{
							"warning":  80,
							"critical": 90,
						},
					},
				},
			},
			stats: &SystemStats{
				DiskInfo: DiskInfo{
					Partitions: []PartitionInfo{
						{
							Device:     "/dev/sda1",
							Mountpoint: "/",
							Percentage: "85",
							FSType:     "ext4",
						},
						{
							Device:     "/dev/sda2",
							Mountpoint: "/home",
							Percentage: "95",
							FSType:     "ext4",
						},
						{
							Device:     "/dev/sdb1",
							Mountpoint: "/mnt/data",
							Percentage: "70",
							FSType:     "ext4",
						},
					},
				},
			},
			expectedViolations: 2,
			expectedLevel:      "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, err := checkDiskThresholds(tt.config, tt.stats)
			if err != nil {
				t.Errorf("checkDiskThresholds() error = %v", err)
			}
			if len(violations) != tt.expectedViolations {
				t.Errorf("checkDiskThresholds() got %d violations, expected %d", len(violations), tt.expectedViolations)
			}
			if tt.expectedViolations > 0 && violations[0].Level != tt.expectedLevel {
				t.Errorf("checkDiskThresholds() level = %s, expected %s", violations[0].Level, tt.expectedLevel)
			}
		})
	}
}

// TestCheckCPUThresholds tests CPU threshold checking
func TestCheckCPUThresholds(t *testing.T) {
	tests := []struct {
		name              string
		config            *Config
		cpuUsage          float64
		expectedViolations int
		expectedLevel     string
	}{
		{
			name: "cpu disabled",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"cpu": {
						Enabled: false,
					},
				},
			},
			cpuUsage:          85.5,
			expectedViolations: 0,
		},
		{
			name: "cpu below warning",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"cpu": {
						Enabled: true,
						Thresholds: map[string]float64{
							"warning":  70,
							"critical": 90,
						},
					},
				},
			},
			cpuUsage:          50.0,
			expectedViolations: 0,
		},
		{
			name: "cpu warning threshold",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"cpu": {
						Enabled: true,
						Thresholds: map[string]float64{
							"warning":  70,
							"critical": 90,
						},
					},
				},
			},
			cpuUsage:          75.5,
			expectedViolations: 1,
			expectedLevel:      "warning",
		},
		{
			name: "cpu critical threshold",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"cpu": {
						Enabled: true,
						Thresholds: map[string]float64{
							"warning":  70,
							"critical": 90,
						},
					},
				},
			},
			cpuUsage:          95.5,
			expectedViolations: 1,
			expectedLevel:      "critical",
		},
		{
			name: "cpu at exact threshold",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"cpu": {
						Enabled: true,
						Thresholds: map[string]float64{
							"warning":  70,
							"critical": 90,
						},
					},
				},
			},
			cpuUsage:          70.0,
			expectedViolations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := checkCPUThresholds(tt.config, tt.cpuUsage)
			if len(violations) != tt.expectedViolations {
				t.Errorf("checkCPUThresholds() got %d violations, expected %d", len(violations), tt.expectedViolations)
			}
			if tt.expectedViolations > 0 && violations[0].Level != tt.expectedLevel {
				t.Errorf("checkCPUThresholds() level = %s, expected %s", violations[0].Level, tt.expectedLevel)
			}
		})
	}
}

// TestCheckMemoryThresholds tests memory threshold checking
func TestCheckMemoryThresholds(t *testing.T) {
	tests := []struct {
		name              string
		config            *Config
		memUsed           float64
		memFree           float64
		expectedViolations int
		expectedLevel     string
	}{
		{
			name: "memory disabled",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"memory": {
						Enabled: false,
					},
				},
			},
			memUsed:            80,
			memFree:            20,
			expectedViolations: 0,
		},
		{
			name: "memory min_free mode above threshold",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"memory": {
						Enabled: true,
						Mode:    "min_free",
						Thresholds: map[string]float64{
							"warning":  20,
							"critical": 5,
						},
					},
				},
			},
			memUsed:            60,
			memFree:            40,
			expectedViolations: 0,
		},
		{
			name: "memory min_free warning",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"memory": {
						Enabled: true,
						Mode:    "min_free",
						Thresholds: map[string]float64{
							"warning":  20,
							"critical": 5,
						},
					},
				},
			},
			memUsed:            85,
			memFree:            15,
			expectedViolations: 1,
			expectedLevel:      "warning",
		},
		{
			name: "memory min_free critical",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"memory": {
						Enabled: true,
						Mode:    "min_free",
						Thresholds: map[string]float64{
							"warning":  20,
							"critical": 5,
						},
					},
				},
			},
			memUsed:            97,
			memFree:            3,
			expectedViolations: 1,
			expectedLevel:      "critical",
		},
		{
			name: "memory max_used mode below threshold",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"memory": {
						Enabled: true,
						Mode:    "max_used",
						Thresholds: map[string]float64{
							"warning":  80,
							"critical": 95,
						},
					},
				},
			},
			memUsed:            70,
			memFree:            30,
			expectedViolations: 0,
		},
		{
			name: "memory max_used warning",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"memory": {
						Enabled: true,
						Mode:    "max_used",
						Thresholds: map[string]float64{
							"warning":  80,
							"critical": 95,
						},
					},
				},
			},
			memUsed:            85,
			memFree:            15,
			expectedViolations: 1,
			expectedLevel:      "warning",
		},
		{
			name: "memory max_used critical",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"memory": {
						Enabled: true,
						Mode:    "max_used",
						Thresholds: map[string]float64{
							"warning":  80,
							"critical": 95,
						},
					},
				},
			},
			memUsed:            97,
			memFree:            3,
			expectedViolations: 1,
			expectedLevel:      "critical",
		},
		{
			name: "memory default mode (min_free)",
			config: &Config{
				Metrics: map[string]MetricConfig{
					"memory": {
						Enabled: true,
						Thresholds: map[string]float64{
							"warning":  20,
							"critical": 5,
						},
					},
				},
			},
			memUsed:            85,
			memFree:            15,
			expectedViolations: 1,
			expectedLevel:      "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := checkMemoryThresholds(tt.config, tt.memUsed, tt.memFree)
			if len(violations) != tt.expectedViolations {
				t.Errorf("checkMemoryThresholds() got %d violations, expected %d", len(violations), tt.expectedViolations)
			}
			if tt.expectedViolations > 0 && violations[0].Level != tt.expectedLevel {
				t.Errorf("checkMemoryThresholds() level = %s, expected %s", violations[0].Level, tt.expectedLevel)
			}
		})
	}
}

// TestMatchesPattern tests partition pattern matching
func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		text    string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "/dev/sda1",
			text:    "/dev/sda1",
			want:    true,
		},
		{
			name:    "wildcard suffix",
			pattern: "/dev/loop*",
			text:    "/dev/loop0",
			want:    true,
		},
		{
			name:    "question mark wildcard",
			pattern: "/dev/sda?",
			text:    "/dev/sda1",
			want:    true,
		},
		{
			name:    "no match",
			pattern: "/dev/sda1",
			text:    "/dev/sda2",
			want:    false,
		},
		{
			name:    "glob with character class",
			pattern: "/dev/[sl]*",
			text:    "/dev/sda1",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPattern(tt.pattern, tt.text)
			if got != tt.want {
				t.Errorf("matchesPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsPartitionExcludedByConfig tests partition exclusion
func TestIsPartitionExcludedByConfig(t *testing.T) {
	tests := []struct {
		name    string
		exclude ExcludeConfig
		part    PartitionInfo
		want    bool
	}{
		{
			name:    "no exclusions",
			exclude: ExcludeConfig{},
			part: PartitionInfo{
				Device:     "/dev/sda1",
				Mountpoint: "/",
				FSType:     "ext4",
			},
			want: false,
		},
		{
			name: "exclude by device pattern",
			exclude: ExcludeConfig{
				Devices: []string{"/dev/loop*"},
			},
			part: PartitionInfo{
				Device:     "/dev/loop0",
				Mountpoint: "/mnt/iso",
				FSType:     "iso9660",
			},
			want: true,
		},
		{
			name: "exclude by filesystem type",
			exclude: ExcludeConfig{
				Filesystems: []string{"tmpfs", "devfs"},
			},
			part: PartitionInfo{
				Device:     "/dev/shm",
				Mountpoint: "/dev/shm",
				FSType:     "tmpfs",
			},
			want: true,
		},
		{
			name: "exclude by mountpoint pattern",
			exclude: ExcludeConfig{
				Mountpoints: []string{"/sys", "/proc"},
			},
			part: PartitionInfo{
				Device:     "sysfs",
				Mountpoint: "/sys",
				FSType:     "sysfs",
			},
			want: true,
		},
		{
			name: "not excluded - no match",
			exclude: ExcludeConfig{
				Devices: []string{"/dev/loop*"},
			},
			part: PartitionInfo{
				Device:     "/dev/sda1",
				Mountpoint: "/",
				FSType:     "ext4",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPartitionExcludedByConfig(tt.part, tt.exclude)
			if got != tt.want {
				t.Errorf("isPartitionExcludedByConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCheckAllThresholds tests the complete threshold checking flow
func TestCheckAllThresholds(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := tmpDir + "/state.json"

	config := &Config{
		Metrics: map[string]MetricConfig{
			"disk": {
				Enabled: true,
				Thresholds: map[string]float64{
					"warning":  80,
					"critical": 90,
				},
			},
			"cpu": {
				Enabled: true,
				Thresholds: map[string]float64{
					"warning":  70,
					"critical": 90,
				},
			},
			"memory": {
				Enabled: true,
				Mode:    "min_free",
				Thresholds: map[string]float64{
					"warning":  20,
					"critical": 5,
				},
			},
		},
	}

	stats := &SystemStats{
		DiskInfo: DiskInfo{
			Partitions: []PartitionInfo{
				{
					Device:     "/dev/sda1",
					Mountpoint: "/",
					Percentage: "85",
					FSType:     "ext4",
				},
			},
		},
		CPUInfo: CPUInfo{
			TotalCPUUsage: "75.5",
		},
		MemoryInfo: MemoryInfo{
			VirtualMemory: VirtualMemory{
				Percentage: "85",
			},
		},
	}

	sm := &StateManager{
		StateFile: stateFile,
		States:    make(map[string]*ViolationState),
	}

	warnings, criticals, err := CheckAllThresholds(config, stats, sm)
	if err != nil {
		t.Errorf("CheckAllThresholds() error = %v", err)
	}

	if len(warnings) == 0 && len(criticals) == 0 {
		t.Errorf("CheckAllThresholds() returned no violations, expected some")
	}

	// Verify warnings and criticals are separated
	for _, w := range warnings {
		if w.Level != "warning" {
			t.Errorf("expected warning level, got %s", w.Level)
		}
	}

	for _, c := range criticals {
		if c.Level != "critical" {
			t.Errorf("expected critical level, got %s", c.Level)
		}
	}
}
