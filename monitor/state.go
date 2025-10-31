package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// parseDuration parses duration strings like "1h", "30m", "10s"
func parseDuration(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration '%s': %w", s, err)
	}
	return d, nil
}

const StateFile = "/tmp/tfc-monitor-state.json"

// ViolationState tracks state of a single metric violation
type ViolationState struct {
	Metric            string   `json:"metric"`
	Level             string   `json:"level"`
	FirstDetectedTime float64  `json:"first_detected_time"`
	LastAlertTime     *float64 `json:"last_alert_time"`
	HasAlerted        bool     `json:"has_alerted"`
}

// StateManager manages violation state persistence
type StateManager struct {
	StateFile string
	States    map[string]*ViolationState
}

// NewStateManager creates a new state manager
func NewStateManager() (*StateManager, error) {
	sm := &StateManager{
		StateFile: StateFile,
		States:    make(map[string]*ViolationState),
	}
	if err := sm.load(); err != nil {
		return nil, err
	}
	return sm, nil
}

// GetOrCreate gets existing state or creates new one
func (sm *StateManager) GetOrCreate(metric string, level string) *ViolationState {
	key := fmt.Sprintf("%s_%s", metric, level)
	if state, ok := sm.States[key]; ok {
		return state
	}

	now := time.Now().Unix()
	state := &ViolationState{
		Metric:            metric,
		Level:             level,
		FirstDetectedTime: float64(now),
		HasAlerted:        false,
	}
	sm.States[key] = state
	return state
}

// Clear clears state for a metric/level (violation resolved)
func (sm *StateManager) Clear(metric string, level string) error {
	key := fmt.Sprintf("%s_%s", metric, level)
	if _, ok := sm.States[key]; ok {
		delete(sm.States, key)
		if err := sm.save(); err != nil {
			return err
		}
	}
	return nil
}

// Save persists state to file
func (sm *StateManager) Save() error {
	return sm.save()
}

// save writes state to file
func (sm *StateManager) save() error {
	data := make(map[string]*ViolationState)
	for key, state := range sm.States {
		data[key] = state
	}

	// Create directory if needed
	dir := "/tmp"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal and write
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(sm.StateFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// load reads state from file
func (sm *StateManager) load() error {
	if _, err := os.Stat(sm.StateFile); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to stat state file: %w", err)
	}

	data, err := os.ReadFile(sm.StateFile)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var states map[string]*ViolationState
	if err := json.Unmarshal(data, &states); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	sm.States = states
	return nil
}

// DurationMinutes returns duration in minutes since first detection
func (vs *ViolationState) DurationMinutes() float64 {
	now := float64(time.Now().Unix())
	return (now - vs.FirstDetectedTime) / 60.0
}

// ShouldAlert determines if we should alert based on throttle settings
func (vs *ViolationState) ShouldAlert(minDurationMinutes float64, repeat bool, repeatInterval string) (bool, error) {
	duration := vs.DurationMinutes()

	// Not enough time has passed
	if duration < minDurationMinutes {
		return false, nil
	}

	// Already alerted
	if vs.HasAlerted {
		// If repeat is disabled, skip
		if !repeat {
			return false, nil
		}

		// If repeat is enabled, check repeat_interval
		if repeatInterval != "" {
			interval, err := parseDuration(repeatInterval)
			if err != nil {
				return false, err
			}
			if interval > 0 && vs.LastAlertTime != nil {
				timeSinceLastAlert := time.Since(time.Unix(int64(*vs.LastAlertTime), 0))
				if timeSinceLastAlert < interval {
					return false, nil
				}
			}
		}
	}

	// Allow alert
	return true, nil
}

// MarkAlerted marks that an alert was sent at this time
func (vs *ViolationState) MarkAlerted() {
	now := float64(time.Now().Unix())
	vs.LastAlertTime = &now
	vs.HasAlerted = true
}
