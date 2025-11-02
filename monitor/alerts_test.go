package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoggerAction tests the logger alert action structure
func TestLoggerAction(t *testing.T) {
	// Test that LoggerAction implements AlertAction interface
	var _ AlertAction = &LoggerAction{
		Level: "warning",
		Tag:   "TEST",
		ID:    "123",
	}

	// Test structure
	action := &LoggerAction{
		Level: "warning",
		Tag:   "TESTTAG",
		ID:    "999",
	}

	if action.Level != "warning" {
		t.Errorf("LoggerAction.Level = %s, want warning", action.Level)
	}
	if action.Tag != "TESTTAG" {
		t.Errorf("LoggerAction.Tag = %s, want TESTTAG", action.Tag)
	}
	if action.ID != "999" {
		t.Errorf("LoggerAction.ID = %s, want 999", action.ID)
	}
}

// TestStdoutAction tests stdout alert action
func TestStdoutAction(t *testing.T) {
	violation := ThresholdViolation{
		Metric:  "disk",
		Level:   "critical",
		Message: "Disk /dev/sda1 is 95% full",
		Value:   95.0,
	}

	action := &StdoutAction{}
	err := action.Execute(violation)
	if err != nil {
		t.Errorf("StdoutAction.Execute() error = %v", err)
	}
}

// TestSyslogActionCreation tests syslog action creation with valid config
func TestSyslogActionCreation(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		wantTag string
	}{
		{
			name: "default syslog config",
			config: map[string]interface{}{
				"type": "syslog",
			},
			wantErr: false,
			wantTag: "tfc-system-monitor",
		},
		{
			name: "custom tag",
			config: map[string]interface{}{
				"type": "syslog",
				"tag":  "custom-alert",
			},
			wantErr: false,
			wantTag: "custom-alert",
		},
		{
			name: "invalid facility",
			config: map[string]interface{}{
				"type":     "syslog",
				"facility": "invalid_facility",
			},
			wantErr: true,
		},
		{
			name: "invalid priority",
			config: map[string]interface{}{
				"type":     "syslog",
				"priority": "invalid_priority",
			},
			wantErr: true,
		},
		{
			name: "valid facility and priority",
			config: map[string]interface{}{
				"type":      "syslog",
				"facility":  "local0",
				"priority":  "warning",
			},
			wantErr: false,
			wantTag: "tfc-system-monitor",
		},
		{
			name: "all facilities",
			config: map[string]interface{}{
				"type":      "syslog",
				"facility":  "local7",
				"priority":  "emergency",
			},
			wantErr: false,
			wantTag: "tfc-system-monitor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := NewSyslogAction(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSyslogAction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && action != nil && action.Tag != tt.wantTag {
				t.Errorf("NewSyslogAction() tag = %s, want %s", action.Tag, tt.wantTag)
			}
		})
	}
}

// TestWebhookActionCreation tests webhook action creation
func TestWebhookActionCreation(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		wantURL string
	}{
		{
			name: "webhook with URL only",
			config: map[string]interface{}{
				"type": "webhook",
				"url":  "https://example.com/webhook",
			},
			wantErr: false,
			wantURL: "https://example.com/webhook",
		},
		{
			name: "webhook without URL",
			config: map[string]interface{}{
				"type": "webhook",
			},
			wantErr: true,
		},
		{
			name: "webhook with custom timeout",
			config: map[string]interface{}{
				"type":    "webhook",
				"url":     "https://example.com/webhook",
				"timeout": 10.0,
			},
			wantErr: false,
			wantURL: "https://example.com/webhook",
		},
		{
			name: "webhook with custom retry",
			config: map[string]interface{}{
				"type":  "webhook",
				"url":   "https://example.com/webhook",
				"retry": 3.0,
			},
			wantErr: false,
			wantURL: "https://example.com/webhook",
		},
		{
			name: "webhook with all options",
			config: map[string]interface{}{
				"type":    "webhook",
				"url":     "https://example.com/webhook",
				"timeout": 15.0,
				"retry":   5.0,
			},
			wantErr: false,
			wantURL: "https://example.com/webhook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := NewWebhookAction(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWebhookAction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && action != nil && action.URL != tt.wantURL {
				t.Errorf("NewWebhookAction() URL = %s, want %s", action.URL, tt.wantURL)
			}
		})
	}
}

// TestWebhookActionExecution tests webhook action execution with mock server
func TestWebhookActionExecution(t *testing.T) {
	tests := []struct {
		name       string
		serverResp int
		retry      int
		wantErr    bool
	}{
		{
			name:       "successful webhook",
			serverResp: http.StatusOK,
			retry:      1,
			wantErr:    false,
		},
		{
			name:       "webhook with 201 response",
			serverResp: http.StatusCreated,
			retry:      1,
			wantErr:    false,
		},
		{
			name:       "webhook with 500 error",
			serverResp: http.StatusInternalServerError,
			retry:      1,
			wantErr:    true,
		},
		{
			name:       "webhook with 404 error",
			serverResp: http.StatusNotFound,
			retry:      1,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST request, got %s", r.Method)
				}
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("expected Content-Type application/json, got %s", ct)
				}

				var payload map[string]interface{}
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &payload)

				if payload["metric"] != "cpu" {
					t.Errorf("expected metric 'cpu', got %v", payload["metric"])
				}

				w.WriteHeader(tt.serverResp)
			}))
			defer server.Close()

			action := &WebhookAction{
				URL:     server.URL,
				Timeout: 5 * time.Second,
				Retry:   tt.retry,
			}

			violation := ThresholdViolation{
				Metric:  "cpu",
				Level:   "warning",
				Message: "High CPU usage",
				Value:   85.5,
			}

			err := action.Execute(violation)
			if (err != nil) != tt.wantErr {
				t.Errorf("WebhookAction.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestWebhookPayload tests webhook payload structure
func TestWebhookPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &payload)

		if payload["metric"] != "memory" {
			t.Errorf("expected metric 'memory', got %v", payload["metric"])
		}
		if payload["level"] != "critical" {
			t.Errorf("expected level 'critical', got %v", payload["level"])
		}
		if payload["message"] != "Low free memory" {
			t.Errorf("expected message 'Low free memory', got %v", payload["message"])
		}
		if payload["value"] != 5.5 {
			t.Errorf("expected value 5.5, got %v", payload["value"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	action := &WebhookAction{
		URL:     server.URL,
		Timeout: 5 * time.Second,
		Retry:   1,
	}

	violation := ThresholdViolation{
		Metric:  "memory",
		Level:   "critical",
		Message: "Low free memory",
		Value:   5.5,
	}

	err := action.Execute(violation)
	if err != nil {
		t.Errorf("WebhookAction.Execute() error = %v", err)
	}
}

// TestScriptActionCreation tests script action creation
func TestScriptActionCreation(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		wantPath string
	}{
		{
			name: "script with path only",
			config: map[string]interface{}{
				"type": "script",
				"path": "/usr/local/bin/alert.sh",
			},
			wantErr: false,
			wantPath: "/usr/local/bin/alert.sh",
		},
		{
			name: "script without path",
			config: map[string]interface{}{
				"type": "script",
			},
			wantErr: true,
		},
		{
			name: "script with args",
			config: map[string]interface{}{
				"type": "script",
				"path": "/usr/local/bin/alert.sh",
				"args": []interface{}{"--email", "admin@example.com"},
			},
			wantErr:  false,
			wantPath: "/usr/local/bin/alert.sh",
		},
		{
			name: "script with timeout",
			config: map[string]interface{}{
				"type":    "script",
				"path":    "/usr/local/bin/alert.sh",
				"timeout": 60.0,
			},
			wantErr:  false,
			wantPath: "/usr/local/bin/alert.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := NewScriptAction(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewScriptAction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && action != nil && action.Path != tt.wantPath {
				t.Errorf("NewScriptAction() path = %s, want %s", action.Path, tt.wantPath)
			}
		})
	}
}

// TestScriptActionExecution tests script execution
func TestScriptActionExecution(t *testing.T) {
	// Create a temporary script
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test_alert.sh")

	// Write a simple script that exits successfully
	script := `#!/bin/bash
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	action := &ScriptAction{
		Path:    scriptPath,
		Args:    []string{"--test"},
		Timeout: 5 * time.Second,
	}

	violation := ThresholdViolation{
		Metric:  "disk",
		Level:   "warning",
		Message: "Disk usage high",
		Value:   85.0,
	}

	err := action.Execute(violation)
	if err != nil {
		t.Errorf("ScriptAction.Execute() error = %v", err)
	}
}

// TestCreateAction tests the action factory function
func TestCreateAction(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]interface{}
		wantType   string
		wantErr    bool
	}{
		{
			name: "create logger action",
			config: map[string]interface{}{
				"type":  "logger",
				"level": "warning",
			},
			wantType: "*monitor.LoggerAction",
			wantErr:  false,
		},
		{
			name: "create stdout action",
			config: map[string]interface{}{
				"type": "stdout",
			},
			wantType: "*monitor.StdoutAction",
			wantErr:  false,
		},
		{
			name: "create webhook action",
			config: map[string]interface{}{
				"type": "webhook",
				"url":  "https://example.com/webhook",
			},
			wantType: "*monitor.WebhookAction",
			wantErr:  false,
		},
		{
			name: "create script action",
			config: map[string]interface{}{
				"type": "script",
				"path": "/usr/local/bin/alert.sh",
			},
			wantType: "*monitor.ScriptAction",
			wantErr:  false,
		},
		{
			name: "create syslog action",
			config: map[string]interface{}{
				"type": "syslog",
			},
			wantType: "*monitor.SyslogAction",
			wantErr:  false,
		},
		{
			name: "unknown action type",
			config: map[string]interface{}{
				"type": "unknown",
			},
			wantErr: true,
		},
		{
			name: "missing type field",
			config: map[string]interface{}{
				"level": "warning",
			},
			wantErr: true,
		},
		{
			name: "webhook missing URL",
			config: map[string]interface{}{
				"type": "webhook",
			},
			wantErr: true,
		},
		{
			name: "script missing path",
			config: map[string]interface{}{
				"type": "script",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := CreateAction(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateAction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && action != nil {
				gotType := fmt.Sprintf("%T", action)
				if gotType != tt.wantType {
					t.Errorf("CreateAction() returned %s, want %s", gotType, tt.wantType)
				}
			}
		})
	}
}

// TestViolationFormatting tests violation message formatting
func TestViolationFormatting(t *testing.T) {
	tests := []struct {
		name      string
		violation ThresholdViolation
		expectMsg string
	}{
		{
			name: "cpu violation",
			violation: ThresholdViolation{
				Metric:  "cpu",
				Level:   "warning",
				Message: "CPU usage high",
				Value:   85.5,
			},
			expectMsg: "[WARNING] cpu: CPU usage high",
		},
		{
			name: "disk violation",
			violation: ThresholdViolation{
				Metric:  "disk",
				Level:   "critical",
				Message: "Disk 95% full",
				Value:   95.0,
			},
			expectMsg: "[CRITICAL] disk: Disk 95% full",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := &StdoutAction{}

			// Capture output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			action.Execute(tt.violation)

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			if !bytes.Contains(buf.Bytes(), []byte(tt.expectMsg)) {
				t.Errorf("expected output to contain %q, got %q", tt.expectMsg, output)
			}
		})
	}
}
