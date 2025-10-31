package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"log/syslog"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// AlertAction is the interface for different alert types
type AlertAction interface {
	Execute(violation ThresholdViolation) error
}

// LoggerAction sends alerts using system logger command
type LoggerAction struct {
	Level string
	Tag   string
	ID    string
}

// Execute sends alert using logger command
func (la *LoggerAction) Execute(violation ThresholdViolation) error {
	message := fmt.Sprintf("[%s] %s: %s", strings.ToUpper(violation.Level), violation.Metric, violation.Message)

	cmd := exec.Command("logger", "-e", "-t", la.Tag, fmt.Sprintf("--id=%s", la.ID), "-s", message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send logger alert: %w", err)
	}

	log.Printf("Logger alert sent: %s", message)
	return nil
}

// SyslogAction sends alerts to syslog
type SyslogAction struct {
	Tag      string
	Facility syslog.Priority
	Priority syslog.Priority
}

// facilityMap maps string names to syslog facilities
var facilityMap = map[string]syslog.Priority{
	"user":   syslog.LOG_USER,
	"mail":   syslog.LOG_MAIL,
	"daemon": syslog.LOG_DAEMON,
	"auth":   syslog.LOG_AUTH,
	"syslog": syslog.LOG_SYSLOG,
	"lpr":    syslog.LOG_LPR,
	"news":   syslog.LOG_NEWS,
	"uucp":   syslog.LOG_UUCP,
	"cron":   syslog.LOG_CRON,
	"local0": syslog.LOG_LOCAL0,
	"local1": syslog.LOG_LOCAL1,
	"local2": syslog.LOG_LOCAL2,
	"local3": syslog.LOG_LOCAL3,
	"local4": syslog.LOG_LOCAL4,
	"local5": syslog.LOG_LOCAL5,
	"local6": syslog.LOG_LOCAL6,
	"local7": syslog.LOG_LOCAL7,
}

// priorityMap maps string names to syslog priorities
var priorityMap = map[string]syslog.Priority{
	"emergency": syslog.LOG_EMERG,
	"alert":     syslog.LOG_ALERT,
	"critical":  syslog.LOG_CRIT,
	"error":     syslog.LOG_ERR,
	"warning":   syslog.LOG_WARNING,
	"notice":    syslog.LOG_NOTICE,
	"info":      syslog.LOG_INFO,
	"debug":     syslog.LOG_DEBUG,
}

// NewSyslogAction creates a new syslog alert action
func NewSyslogAction(config map[string]interface{}) (*SyslogAction, error) {
	sa := &SyslogAction{
		Tag:      "tfc-monitor",
		Facility: syslog.LOG_LOCAL0,
		Priority: syslog.LOG_WARNING,
	}

	if tag, ok := config["tag"].(string); ok {
		sa.Tag = tag
	}

	if facilityName, ok := config["facility"].(string); ok {
		if facility, exists := facilityMap[facilityName]; exists {
			sa.Facility = facility
		} else {
			return nil, fmt.Errorf("invalid syslog facility '%s'", facilityName)
		}
	}

	if priorityName, ok := config["priority"].(string); ok {
		if priority, exists := priorityMap[priorityName]; exists {
			sa.Priority = priority
		} else {
			return nil, fmt.Errorf("invalid syslog priority '%s'", priorityName)
		}
	}

	return sa, nil
}

// Execute sends alert to syslog
func (sa *SyslogAction) Execute(violation ThresholdViolation) error {
	w, err := syslog.Dial("", "", sa.Facility|sa.Priority, sa.Tag)
	if err != nil {
		return fmt.Errorf("failed to connect to syslog: %w", err)
	}
	defer w.Close()

	message := fmt.Sprintf("[%s] %s: %s", strings.ToUpper(violation.Level), violation.Metric, violation.Message)
	if err := w.Warning(message); err != nil {
		return fmt.Errorf("failed to send syslog alert: %w", err)
	}

	log.Printf("Syslog alert sent: %s", message)
	return nil
}

// WebhookAction sends alerts via HTTP webhook
type WebhookAction struct {
	URL     string
	Timeout time.Duration
	Retry   int
}

// NewWebhookAction creates a new webhook alert action
func NewWebhookAction(config map[string]interface{}) (*WebhookAction, error) {
	wa := &WebhookAction{
		Timeout: 5 * time.Second,
		Retry:   1,
	}

	if url, ok := config["url"].(string); ok {
		wa.URL = url
	} else {
		return nil, fmt.Errorf("webhook action requires 'url' field")
	}

	if timeout, ok := config["timeout"].(float64); ok {
		wa.Timeout = time.Duration(timeout) * time.Second
	}

	if retry, ok := config["retry"].(float64); ok {
		wa.Retry = int(retry)
	}

	return wa, nil
}

// Execute sends alert via webhook
func (wa *WebhookAction) Execute(violation ThresholdViolation) error {
	payload := map[string]interface{}{
		"metric":  violation.Metric,
		"level":   violation.Level,
		"message": violation.Message,
		"value":   violation.Value,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	var lastError error
	for attempt := 0; attempt < wa.Retry; attempt++ {
		client := &http.Client{Timeout: wa.Timeout}
		resp, err := client.Post(wa.URL, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			lastError = err
			log.Printf("Webhook alert failed (attempt %d/%d): %v", attempt+1, wa.Retry, err)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Webhook alert sent to %s: %v", wa.URL, payload)
			resp.Body.Close()
			return nil
		}

		resp.Body.Close()
		lastError = fmt.Errorf("webhook returned status %d", resp.StatusCode)
		log.Printf("Webhook alert failed (attempt %d/%d): %v", attempt+1, wa.Retry, lastError)
	}

	return fmt.Errorf("failed to send webhook alert after %d attempts: %w", wa.Retry, lastError)
}

// ScriptAction executes external script for alert
type ScriptAction struct {
	Path    string
	Args    []string
	Timeout time.Duration
}

// NewScriptAction creates a new script alert action
func NewScriptAction(config map[string]interface{}) (*ScriptAction, error) {
	sa := &ScriptAction{
		Timeout: 30 * time.Second,
	}

	if path, ok := config["path"].(string); ok {
		sa.Path = path
	} else {
		return nil, fmt.Errorf("script action requires 'path' field")
	}

	if args, ok := config["args"].([]interface{}); ok {
		for _, arg := range args {
			if argStr, ok := arg.(string); ok {
				sa.Args = append(sa.Args, argStr)
			}
		}
	}

	if timeout, ok := config["timeout"].(float64); ok {
		sa.Timeout = time.Duration(timeout) * time.Second
	}

	return sa, nil
}

// Execute executes alert script
func (sa *ScriptAction) Execute(violation ThresholdViolation) error {
	args := sa.Args
	args = append(args, violation.Metric, violation.Level, violation.Message)

	cmd := exec.Command(sa.Path, args...)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("script alert failed: %w", err)
		}
		log.Printf("Script alert executed: %s", sa.Path)
		return nil
	case <-time.After(sa.Timeout):
		cmd.Process.Kill()
		return fmt.Errorf("script alert timed out after %v", sa.Timeout)
	}
}

// CreateAction creates appropriate alert action based on config
func CreateAction(config map[string]interface{}) (AlertAction, error) {
	actionType, ok := config["type"].(string)
	if !ok {
		return nil, fmt.Errorf("alert action missing 'type' field")
	}

	switch actionType {
	case "logger":
		level := "warning"
		if l, ok := config["level"].(string); ok {
			level = l
		}
		return &LoggerAction{
			Level: level,
			Tag:   "ALERT",
			ID:    "451",
		}, nil
	case "syslog":
		return NewSyslogAction(config)
	case "webhook":
		return NewWebhookAction(config)
	case "script":
		return NewScriptAction(config)
	default:
		return nil, fmt.Errorf("unknown alert action type: %s", actionType)
	}
}

// ProcessViolations executes configured alert actions for violations
func ProcessViolations(config *Config, warningViolations []ThresholdViolation, criticalViolations []ThresholdViolation) {
	// Process critical violations
	if len(criticalViolations) > 0 {
		log.Printf("Processing %d critical violations", len(criticalViolations))
		criticalActions := config.GetAlertActions("critical")
		for _, actionConfig := range criticalActions {
			action, err := CreateAction(actionConfig)
			if err != nil {
				log.Printf("Failed to create alert action: %v", err)
				continue
			}
			for _, violation := range criticalViolations {
				if err := action.Execute(violation); err != nil {
					log.Printf("Failed to execute alert: %v", err)
				}
			}
		}
	}

	// Process warning violations
	if len(warningViolations) > 0 {
		log.Printf("Processing %d warning violations", len(warningViolations))
		warningActions := config.GetAlertActions("warning")
		for _, actionConfig := range warningActions {
			action, err := CreateAction(actionConfig)
			if err != nil {
				log.Printf("Failed to create alert action: %v", err)
				continue
			}
			for _, violation := range warningViolations {
				if err := action.Execute(violation); err != nil {
					log.Printf("Failed to execute alert: %v", err)
				}
			}
		}
	}
}
