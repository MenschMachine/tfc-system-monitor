package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

const StateFile = "/tmp/tfc-monitor-state.json"

// ViolationState tracks state of a single metric violation
type ViolationState struct {
	Metric           string     `json:"metric"`
	Level            string     `json:"level"`
	FirstDetectedTime float64    `json:"first_detected_time"`
	LastAlertTime    *float64   `json:"last_alert_time"`
	HasAlerted       bool       `json:"has_alerted"`
}

// StateManager manages violation state persistence
type StateManager struct {
	StateFile string
	States    map[string]*ViolationState
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	sm := &StateManager{
		StateFile: StateFile,
		States:    make(map[string]*ViolationState),
	}
	sm.load()
	return sm
}

// GetOrCreate gets existing state or creates new one
func (sm *StateManager) GetOrCreate(metric string, level string) *ViolationState {
	key := fmt.Sprintf("%s_%s", metric, level)
	if state, ok := sm.States[key]; ok {
		return state
	}

	now := time.Now().Unix()
	state := &ViolationState{
		Metric:           metric,
		Level:            level,
		FirstDetectedTime: float64(now),
		HasAlerted:       false,
	}
	sm.States[key] = state
	return state
}

// Clear clears state for a metric/level (violation resolved)
func (sm *StateManager) Clear(metric string, level string) {
	key := fmt.Sprintf("%s_%s", metric, level)
	if _, ok := sm.States[key]; ok {
		log.Printf("Clearing state for %s/%s", metric, level)
		delete(sm.States, key)
		sm.save()
	}
}

// Save persists state to file
func (sm *StateManager) Save() {
	sm.save()
}

// save writes state to file
func (sm *StateManager) save() {
	try := func() error {
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

		log.Printf("State saved to %s", sm.StateFile)
		return nil
	}()

	if try != nil {
		log.Printf("Failed to save state: %v", try)
	}
}

// load reads state from file
func (sm *StateManager) load() {
	if _, err := os.Stat(sm.StateFile); os.IsNotExist(err) {
		log.Printf("State file not found: %s", sm.StateFile)
		return
	}

	data, err := os.ReadFile(sm.StateFile)
	if err != nil {
		log.Printf("Failed to read state file: %v", err)
		return
	}

	var states map[string]*ViolationState
	if err := json.Unmarshal(data, &states); err != nil {
		log.Printf("Failed to unmarshal state: %v", err)
		return
	}

	sm.States = states
	log.Printf("State loaded from %s: %d entries", sm.StateFile, len(sm.States))
}

// DurationMinutes returns duration in minutes since first detection
func (vs *ViolationState) DurationMinutes() float64 {
	now := float64(time.Now().Unix())
	return (now - vs.FirstDetectedTime) / 60.0
}

// ShouldAlert determines if we should alert based on throttle settings
func (vs *ViolationState) ShouldAlert(minDurationMinutes float64, repeat bool) bool {
	duration := vs.DurationMinutes()

	// Not enough time has passed
	if duration < minDurationMinutes {
		log.Printf("Throttle: %s/%s duration %.1fm < min %.1fm, skipping alert",
			vs.Metric, vs.Level, duration, minDurationMinutes)
		return false
	}

	// Already alerted and not repeating
	if vs.HasAlerted && !repeat {
		log.Printf("Throttle: %s/%s already alerted and repeat=false, skipping",
			vs.Metric, vs.Level)
		return false
	}

	// Allow alert
	return true
}

// MarkAlerted marks that an alert was sent at this time
func (vs *ViolationState) MarkAlerted() {
	now := float64(time.Now().Unix())
	vs.LastAlertTime = &now
	vs.HasAlerted = true
}
