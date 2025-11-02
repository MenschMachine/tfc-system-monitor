package monitor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestProcessViolationsNoViolations tests processing when there are no violations
func TestProcessViolationsNoViolations(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
		},
	}

	err := ProcessViolations(config, []ThresholdViolation{}, []ThresholdViolation{})
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}

// TestProcessViolationsWarningsOnly tests processing warnings only
func TestProcessViolationsWarningsOnly(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{},
			},
		},
	}

	warnings := []ThresholdViolation{
		{
			Metric:  "cpu",
			Level:   "warning",
			Message: "CPU usage high",
			Value:   75.5,
		},
	}

	err := ProcessViolations(config, warnings, []ThresholdViolation{})
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}

// TestProcessViolationsCriticalOnly tests processing critical violations only
func TestProcessViolationsCriticalOnly(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{},
			},
			"critical": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
		},
	}

	criticals := []ThresholdViolation{
		{
			Metric:  "memory",
			Level:   "critical",
			Message: "Free memory critical",
			Value:   2.5,
		},
	}

	err := ProcessViolations(config, []ThresholdViolation{}, criticals)
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}

// TestProcessViolationsMixed tests processing both warnings and critical violations
func TestProcessViolationsMixed(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
		{Metric: "disk", Level: "warning", Message: "Disk warning", Value: 85.0},
	}

	criticals := []ThresholdViolation{
		{Metric: "memory", Level: "critical", Message: "Memory critical", Value: 2.5},
	}

	err := ProcessViolations(config, warnings, criticals)
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}

// TestProcessViolationsMultipleActions tests multiple actions per level
func TestProcessViolationsMultipleActions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
					{"type": "webhook", "url": server.URL},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
	}

	err := ProcessViolations(config, warnings, []ThresholdViolation{})
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}

// TestProcessViolationsInvalidAction tests error handling for invalid action
func TestProcessViolationsInvalidAction(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "invalid_type"},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
	}

	err := ProcessViolations(config, warnings, []ThresholdViolation{})
	if err == nil {
		t.Errorf("ProcessViolations() expected error for invalid action type, got nil")
	}
	if err.Error() != fmt.Sprintf("failed to create warning alert action: unknown alert action type: invalid_type") {
		t.Errorf("ProcessViolations() unexpected error message: %v", err)
	}
}

// TestProcessViolationsWebhookError tests error handling when webhook fails
func TestProcessViolationsWebhookError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "webhook", "url": server.URL, "retry": 1.0},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
	}

	err := ProcessViolations(config, warnings, []ThresholdViolation{})
	if err == nil {
		t.Errorf("ProcessViolations() expected error for webhook failure, got nil")
	}
}

// TestProcessViolationsMissingURLField tests error handling when webhook URL is missing
func TestProcessViolationsMissingURLField(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "webhook"},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
	}

	err := ProcessViolations(config, warnings, []ThresholdViolation{})
	if err == nil {
		t.Errorf("ProcessViolations() expected error for missing webhook URL, got nil")
	}
}

// TestProcessViolationsScriptError tests script action error handling
func TestProcessViolationsScriptError(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "script", "path": "/nonexistent/script.sh"},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
	}

	err := ProcessViolations(config, warnings, []ThresholdViolation{})
	if err == nil {
		t.Errorf("ProcessViolations() expected error for nonexistent script, got nil")
	}
}

// TestProcessViolationsMultipleViolationsSingleAction tests processing multiple violations with single action
func TestProcessViolationsMultipleViolationsSingleAction(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
		{Metric: "disk", Level: "warning", Message: "Disk warning", Value: 85.0},
		{Metric: "memory", Level: "warning", Message: "Memory warning", Value: 15.0},
	}

	criticals := []ThresholdViolation{
		{Metric: "disk", Level: "critical", Message: "Disk critical", Value: 95.0},
	}

	err := ProcessViolations(config, warnings, criticals)
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}

// TestProcessViolationsEmptyActions tests when action lists are empty
func TestProcessViolationsEmptyActions(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{},
			},
			"critical": {
				Actions: []map[string]interface{}{},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
	}

	criticals := []ThresholdViolation{
		{Metric: "memory", Level: "critical", Message: "Memory critical", Value: 2.5},
	}

	err := ProcessViolations(config, warnings, criticals)
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}

// TestProcessViolationsWebhookSuccess tests successful webhook calls
func TestProcessViolationsWebhookSuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "webhook", "url": server.URL},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
		{Metric: "disk", Level: "warning", Message: "Disk warning", Value: 85.0},
	}

	err := ProcessViolations(config, warnings, []ThresholdViolation{})
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}

	if callCount != 2 {
		t.Errorf("webhook called %d times, expected 2", callCount)
	}
}

// TestProcessViolationsStdoutAction tests stdout action processing
func TestProcessViolationsStdoutAction(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
	}

	criticals := []ThresholdViolation{
		{Metric: "memory", Level: "critical", Message: "Memory critical", Value: 2.5},
	}

	err := ProcessViolations(config, warnings, criticals)
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}

// TestProcessViolationsSyslogAction tests syslog action processing
func TestProcessViolationsSyslogAction(t *testing.T) {
	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{
						"type":      "syslog",
						"tag":       "test-monitor",
						"facility":  "local0",
						"priority":  "warning",
					},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
	}

	err := ProcessViolations(config, warnings, []ThresholdViolation{})
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}

// TestProcessViolationsMixedActions tests mixing different action types
func TestProcessViolationsMixedActions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &Config{
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
					{"type": "webhook", "url": server.URL},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{
					{"type": "stdout"},
				},
			},
		},
	}

	warnings := []ThresholdViolation{
		{Metric: "cpu", Level: "warning", Message: "CPU warning", Value: 75.0},
	}

	criticals := []ThresholdViolation{
		{Metric: "memory", Level: "critical", Message: "Memory critical", Value: 2.5},
	}

	err := ProcessViolations(config, warnings, criticals)
	if err != nil {
		t.Errorf("ProcessViolations() error = %v", err)
	}
}
