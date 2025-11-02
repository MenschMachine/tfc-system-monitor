package monitor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewStateManager tests state manager creation
func TestNewStateManager(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "test_state.json")

	sm := &StateManager{
		StateFile: stateFile,
		States:    make(map[string]*ViolationState),
	}

	if sm.StateFile != stateFile {
		t.Errorf("StateManager.StateFile = %s, want %s", sm.StateFile, stateFile)
	}

	if len(sm.States) != 0 {
		t.Errorf("StateManager.States length = %d, want 0", len(sm.States))
	}
}

// TestGetOrCreate tests getting or creating violation state
func TestGetOrCreate(t *testing.T) {
	tmpDir := t.TempDir()
	sm := &StateManager{
		StateFile: filepath.Join(tmpDir, "state.json"),
		States:    make(map[string]*ViolationState),
	}

	// First call should create new state
	state1 := sm.GetOrCreate("cpu", "warning")
	if state1.Metric != "cpu" {
		t.Errorf("state.Metric = %s, want cpu", state1.Metric)
	}
	if state1.Level != "warning" {
		t.Errorf("state.Level = %s, want warning", state1.Level)
	}
	if state1.HasAlerted {
		t.Errorf("state.HasAlerted = true, want false")
	}

	// Second call should return same state
	state2 := sm.GetOrCreate("cpu", "warning")
	if state1 != state2 {
		t.Errorf("GetOrCreate returned different states on second call")
	}

	// Different metric should create new state
	state3 := sm.GetOrCreate("memory", "warning")
	if state3.Metric != "memory" {
		t.Errorf("state.Metric = %s, want memory", state3.Metric)
	}
	if state1 == state3 {
		t.Errorf("GetOrCreate returned same state for different metrics")
	}
}

// TestViolationStateMarkAlerted tests marking a state as alerted
func TestMarkAlerted(t *testing.T) {
	tmpDir := t.TempDir()
	sm := &StateManager{
		StateFile: filepath.Join(tmpDir, "state.json"),
		States:    make(map[string]*ViolationState),
	}

	state := sm.GetOrCreate("cpu", "warning")
	if state.HasAlerted {
		t.Errorf("initial HasAlerted = true, want false")
	}
	if state.LastAlertTime != nil {
		t.Errorf("initial LastAlertTime = %v, want nil", state.LastAlertTime)
	}

	state.MarkAlerted()
	if !state.HasAlerted {
		t.Errorf("after MarkAlerted, HasAlerted = false, want true")
	}
	if state.LastAlertTime == nil {
		t.Errorf("after MarkAlerted, LastAlertTime = nil, want set")
	}
}

// TestDurationMinutes tests duration calculation
func TestDurationMinutes(t *testing.T) {
	tests := []struct {
		name     string
		firstDet float64
		minDelta float64
	}{
		{
			name:     "just created",
			firstDet: float64(time.Now().Unix()),
			minDelta: 0,
		},
		{
			name:     "1 minute ago",
			firstDet: float64(time.Now().Unix() - 60),
			minDelta: 0.95,
		},
		{
			name:     "5 minutes ago",
			firstDet: float64(time.Now().Unix() - 300),
			minDelta: 4.95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &ViolationState{
				FirstDetectedTime: tt.firstDet,
			}

			duration := state.DurationMinutes()
			if duration < tt.minDelta {
				t.Errorf("DurationMinutes() = %f, want >= %f", duration, tt.minDelta)
			}
		})
	}
}

// TestShouldAlert tests the throttling logic
func TestShouldAlert(t *testing.T) {
	tests := []struct {
		name           string
		firstDetected  time.Time
		lastAlert      *time.Time
		hasAlerted     bool
		minDuration    float64
		repeat         bool
		repeatInterval string
		want           bool
		wantErr        bool
	}{
		{
			name:          "first alert allowed",
			firstDetected: time.Now(),
			minDuration:   0,
			repeat:        false,
			want:          true,
			wantErr:       false,
		},
		{
			name:          "not enough duration elapsed",
			firstDetected: time.Now().Add(-30 * time.Second),
			minDuration:   1,
			repeat:        false,
			want:          false,
			wantErr:       false,
		},
		{
			name:          "enough duration elapsed",
			firstDetected: time.Now().Add(-2 * time.Minute),
			minDuration:   1,
			repeat:        false,
			want:          true,
			wantErr:       false,
		},
		{
			name:          "already alerted, repeat disabled",
			firstDetected: time.Now().Add(-5 * time.Minute),
			lastAlert:     ptrTime(time.Now().Add(-1 * time.Minute)),
			hasAlerted:    true,
			minDuration:   0,
			repeat:        false,
			want:          false,
			wantErr:       false,
		},
		{
			name:           "already alerted, repeat enabled, interval not passed",
			firstDetected:  time.Now().Add(-5 * time.Minute),
			lastAlert:      ptrTime(time.Now().Add(-30 * time.Second)),
			hasAlerted:     true,
			minDuration:    0,
			repeat:         true,
			repeatInterval: "1m",
			want:           false,
			wantErr:        false,
		},
		{
			name:           "already alerted, repeat enabled, interval passed",
			firstDetected:  time.Now().Add(-5 * time.Minute),
			lastAlert:      ptrTime(time.Now().Add(-2 * time.Minute)),
			hasAlerted:     true,
			minDuration:    0,
			repeat:         true,
			repeatInterval: "1m",
			want:           true,
			wantErr:        false,
		},
		{
			name:           "invalid repeat interval format",
			firstDetected:  time.Now(),
			hasAlerted:     true,
			minDuration:    0,
			repeat:         true,
			repeatInterval: "invalid",
			want:           false,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &ViolationState{
				FirstDetectedTime: float64(tt.firstDetected.Unix()),
				HasAlerted:        tt.hasAlerted,
			}
			if tt.lastAlert != nil {
				lastAlertFloat := float64(tt.lastAlert.Unix())
				state.LastAlertTime = &lastAlertFloat
			}

			got, err := state.ShouldAlert(tt.minDuration, tt.repeat, tt.repeatInterval)
			if (err != nil) != tt.wantErr {
				t.Errorf("ShouldAlert() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ShouldAlert() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStatePersistence tests saving and loading state
func TestStatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	// Create and save state
	sm1 := &StateManager{
		StateFile: stateFile,
		States:    make(map[string]*ViolationState),
	}

	state := sm1.GetOrCreate("cpu", "warning")
	state.MarkAlerted()

	if err := sm1.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Fatalf("state file not created")
	}

	// Load state in new manager
	sm2 := &StateManager{
		StateFile: stateFile,
		States:    make(map[string]*ViolationState),
	}

	if err := sm2.load(); err != nil {
		t.Fatalf("load() error = %v", err)
	}

	if len(sm2.States) != 1 {
		t.Errorf("loaded states count = %d, want 1", len(sm2.States))
	}

	key := "cpu_warning"
	if _, ok := sm2.States[key]; !ok {
		t.Errorf("state key %s not found", key)
	}

	if loaded := sm2.States[key]; !loaded.HasAlerted {
		t.Errorf("loaded state HasAlerted = false, want true")
	}
}

// TestClearState tests clearing state
func TestClearState(t *testing.T) {
	tmpDir := t.TempDir()
	sm := &StateManager{
		StateFile: filepath.Join(tmpDir, "state.json"),
		States:    make(map[string]*ViolationState),
	}

	state := sm.GetOrCreate("cpu", "warning")
	state.MarkAlerted()

	if err := sm.Clear("cpu", "warning"); err != nil {
		t.Errorf("Clear() error = %v", err)
	}

	key := "cpu_warning"
	if _, ok := sm.States[key]; ok {
		t.Errorf("state key %s still exists after clear", key)
	}
}

// TestClearResolvedViolations tests clearing states for resolved violations
func TestClearResolvedViolations(t *testing.T) {
	tmpDir := t.TempDir()
	sm := &StateManager{
		StateFile: filepath.Join(tmpDir, "state.json"),
		States:    make(map[string]*ViolationState),
	}

	// Create some states
	sm.GetOrCreate("cpu", "warning").MarkAlerted()
	sm.GetOrCreate("memory", "critical").MarkAlerted()
	sm.GetOrCreate("disk", "warning").MarkAlerted()

	// Only provide current violations for some metrics
	currentViolations := []ThresholdViolation{
		{Metric: "cpu", Level: "warning"},
	}

	if err := clearResolvedViolations(currentViolations, sm); err != nil {
		t.Errorf("clearResolvedViolations() error = %v", err)
	}

	// CPU warning should still exist
	if _, ok := sm.States["cpu_warning"]; !ok {
		t.Errorf("cpu_warning state was cleared, should exist")
	}

	// Memory critical should be cleared
	if _, ok := sm.States["memory_critical"]; ok {
		t.Errorf("memory_critical state still exists, should be cleared")
	}

	// Disk warning should be cleared
	if _, ok := sm.States["disk_warning"]; ok {
		t.Errorf("disk_warning state still exists, should be cleared")
	}
}

// TestApplyThrottling tests the complete throttling flow
func TestApplyThrottling(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		Metrics: map[string]MetricConfig{
			"cpu": {
				Throttle: ThrottleConfig{
					MinDurationMinutes: 0,
					Repeat:             false,
				},
			},
			"memory": {
				Throttle: ThrottleConfig{
					MinDurationMinutes: 0,
					Repeat:             true,
					RepeatInterval:     "30s",
				},
			},
		},
	}

	sm := &StateManager{
		StateFile: filepath.Join(tmpDir, "state.json"),
		States:    make(map[string]*ViolationState),
	}

	violations := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU high"},
		{Metric: "memory", Level: "critical", Message: "Memory low"},
	}

	throttled, err := applyThrottling(config, violations, sm)
	if err != nil {
		t.Errorf("applyThrottling() error = %v", err)
	}

	// Both should be throttled on first pass (since min duration is 0 and repeat enabled)
	if len(throttled) != 2 {
		t.Errorf("applyThrottling() returned %d violations, want 2", len(throttled))
	}

	// Check that states were marked as alerted
	cpuState := sm.GetOrCreate("cpu", "warning")
	if !cpuState.HasAlerted {
		t.Errorf("CPU warning state not marked as alerted")
	}

	memState := sm.GetOrCreate("memory", "critical")
	if !memState.HasAlerted {
		t.Errorf("Memory critical state not marked as alerted")
	}
}

// TestThrottleMinDuration tests minimum duration throttling
func TestThrottleMinDuration(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		Metrics: map[string]MetricConfig{
			"cpu": {
				Throttle: ThrottleConfig{
					MinDurationMinutes: 5,
					Repeat:             false,
				},
			},
		},
	}

	sm := &StateManager{
		StateFile: filepath.Join(tmpDir, "state.json"),
		States:    make(map[string]*ViolationState),
	}

	// Create a violation that just appeared
	now := time.Now().Unix()
	sm.States["cpu_warning"] = &ViolationState{
		Metric:            "cpu",
		Level:             "warning",
		FirstDetectedTime: float64(now),
		HasAlerted:        false,
	}

	violations := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU high"},
	}

	throttled, err := applyThrottling(config, violations, sm)
	if err != nil {
		t.Errorf("applyThrottling() error = %v", err)
	}

	// Should be throttled due to min duration
	if len(throttled) != 0 {
		t.Errorf("applyThrottling() returned %d violations, want 0 (throttled)", len(throttled))
	}
}

// Helper function to create a pointer to time.Time
func ptrTime(t time.Time) *time.Time {
	return &t
}
