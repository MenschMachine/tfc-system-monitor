package monitor

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

// Config represents the entire configuration structure
type Config struct {
	Metrics map[string]MetricConfig `yaml:"metrics"`
	Alerts  map[string]AlertLevel   `yaml:"alerts"`
}

// MetricConfig represents configuration for a single metric
type MetricConfig struct {
	Enabled    bool              `yaml:"enabled"`
	Thresholds map[string]float64 `yaml:"thresholds"`
	Throttle   ThrottleConfig    `yaml:"throttle"`
	Mode       string            `yaml:"mode"` // for memory metric
	Unit       string            `yaml:"unit"`
}

// ThrottleConfig represents throttle settings
type ThrottleConfig struct {
	MinDurationMinutes float64 `yaml:"min_duration_minutes"`
	Repeat             bool    `yaml:"repeat"`
	RepeatInterval     string  `yaml:"repeat_interval"`
}

// AlertLevel represents alert configuration for a severity level
type AlertLevel struct {
	Actions []map[string]interface{} `yaml:"actions"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Metrics: map[string]MetricConfig{
			"disk": {
				Enabled: true,
				Thresholds: map[string]float64{
					"warning":  80,
					"critical": 90,
				},
				Throttle: ThrottleConfig{
					MinDurationMinutes: 0,
					Repeat:             false,
				},
				Unit: "percentage",
			},
			"cpu": {
				Enabled: true,
				Thresholds: map[string]float64{
					"warning":  70,
					"critical": 90,
				},
				Throttle: ThrottleConfig{
					MinDurationMinutes: 0,
					Repeat:             false,
				},
				Unit: "percentage",
			},
			"memory": {
				Enabled: true,
				Thresholds: map[string]float64{
					"warning":  20,
					"critical": 5,
				},
				Throttle: ThrottleConfig{
					MinDurationMinutes: 0,
					Repeat:             false,
				},
				Mode: "min_free",
				Unit: "percentage",
			},
		},
		Alerts: map[string]AlertLevel{
			"warning": {
				Actions: []map[string]interface{}{
					{
						"type":  "logger",
						"level": "warning",
					},
				},
			},
			"critical": {
				Actions: []map[string]interface{}{
					{
						"type":  "logger",
						"level": "critical",
					},
				},
			},
		},
	}
}

// LoadConfig loads configuration from a YAML file, falling back to defaults
func LoadConfig(configPath string) (*Config, error) {
	log.Printf("Loading config from %s", configPath)

	// Start with default config
	config := DefaultConfig()

	// Try to load user config
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Config file %s not found, using defaults", configPath)
			printDefaults()
			return config, nil
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse user config
	userConfig := &Config{}
	if err := yaml.Unmarshal(data, userConfig); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Merge user config with defaults
	config = deepMergeConfig(config, userConfig)

	// Validate config
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	log.Println("Config loaded and validated successfully")
	return config, nil
}

// deepMergeConfig merges user config with defaults
func deepMergeConfig(defaults, overrides *Config) *Config {
	result := &Config{
		Metrics: make(map[string]MetricConfig),
		Alerts:  make(map[string]AlertLevel),
	}

	// Copy defaults
	for k, v := range defaults.Metrics {
		result.Metrics[k] = v
	}
	for k, v := range defaults.Alerts {
		result.Alerts[k] = v
	}

	// Override with user config
	if overrides != nil {
		for k, v := range overrides.Metrics {
			result.Metrics[k] = v
		}
		for k, v := range overrides.Alerts {
			result.Alerts[k] = v
		}
	}

	return result
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Metrics == nil {
		return fmt.Errorf("config missing 'metrics' section")
	}

	// Validate each metric
	for metricName, metricConfig := range config.Metrics {
		if err := validateMetricConfig(metricName, metricConfig); err != nil {
			return err
		}
	}

	// Validate alerts
	if config.Alerts != nil {
		for level, alertLevel := range config.Alerts {
			if err := validateAlertLevel(level, alertLevel); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateMetricConfig validates a single metric configuration
func validateMetricConfig(metricName string, config MetricConfig) error {
	if config.Thresholds == nil {
		return fmt.Errorf("metric %s missing 'thresholds' section", metricName)
	}

	// Validate threshold values
	for level, value := range config.Thresholds {
		if value < 0 {
			return fmt.Errorf("metric %s threshold %s must be >= 0", metricName, level)
		}
	}

	// Validate throttle
	if config.Throttle.MinDurationMinutes < 0 {
		return fmt.Errorf("metric %s 'min_duration_minutes' must be >= 0", metricName)
	}

	// Validate memory mode
	if metricName == "memory" && config.Mode != "" && config.Mode != "min_free" && config.Mode != "max_used" {
		return fmt.Errorf("memory metric 'mode' must be 'min_free' or 'max_used'")
	}

	return nil
}

// validateAlertLevel validates alert level configuration
func validateAlertLevel(level string, alertLevel AlertLevel) error {
	validLevels := map[string]bool{"warning": true, "critical": true}
	if !validLevels[level] {
		return fmt.Errorf("invalid alert level '%s'", level)
	}

	if alertLevel.Actions == nil {
		return fmt.Errorf("alert level '%s' missing 'actions' field", level)
	}

	// Validate each action
	for i, action := range alertLevel.Actions {
		if actionType, ok := action["type"]; !ok {
			return fmt.Errorf("alert action %d for level '%s' missing 'type' field", i, level)
		} else if actionTypeStr, ok := actionType.(string); ok {
			validTypes := map[string]bool{"logger": true, "syslog": true, "webhook": true, "script": true, "stdout": true}
			if !validTypes[actionTypeStr] {
				return fmt.Errorf("alert action type '%s' not supported", actionTypeStr)
			}

			// Validate required fields
			if actionTypeStr == "webhook" {
				if _, ok := action["url"]; !ok {
					return fmt.Errorf("alert action 'webhook' missing required 'url' field")
				}
			}
			if actionTypeStr == "script" {
				if _, ok := action["path"]; !ok {
					return fmt.Errorf("alert action 'script' missing required 'path' field")
				}
			}
		}
	}

	return nil
}

// GetMetricConfig gets configuration for a specific metric
func (c *Config) GetMetricConfig(metricName string) (MetricConfig, bool) {
	mc, ok := c.Metrics[metricName]
	return mc, ok
}

// IsMetricEnabled checks if a metric is enabled
func (c *Config) IsMetricEnabled(metricName string) bool {
	mc, ok := c.Metrics[metricName]
	return ok && mc.Enabled
}

// GetThrottleConfig gets throttle configuration for a metric
func (c *Config) GetThrottleConfig(metricName string) ThrottleConfig {
	if mc, ok := c.Metrics[metricName]; ok {
		return mc.Throttle
	}
	return ThrottleConfig{MinDurationMinutes: 0, Repeat: false}
}

// GetAlertActions gets alert actions for a specific level
func (c *Config) GetAlertActions(level string) []map[string]interface{} {
	if alertLevel, ok := c.Alerts[level]; ok {
		return alertLevel.Actions
	}
	return []map[string]interface{}{}
}

// printDefaults prints the default configuration
func printDefaults() {
	divider := "======================================================================"
	fmt.Println("\n" + divider)
	fmt.Println("DEFAULT METRICS CONFIGURATION")
	fmt.Println(divider)

	defaultConfig := DefaultConfig()
	for metricName, metricCfg := range defaultConfig.Metrics {
		fmt.Printf("\n%s:\n", metricName)
		fmt.Printf("  enabled: %v\n", metricCfg.Enabled)
		fmt.Println("  thresholds:")
		for level, value := range metricCfg.Thresholds {
			fmt.Printf("    %s: %v\n", level, value)
		}
		fmt.Println("  throttle:")
		fmt.Printf("    min_duration_minutes: %v\n", metricCfg.Throttle.MinDurationMinutes)
		fmt.Printf("    repeat: %v\n", metricCfg.Throttle.Repeat)
	}

	fmt.Println("\n" + divider)
	fmt.Println("DEFAULT ALERT ACTIONS")
	fmt.Println(divider)

	for level, levelCfg := range defaultConfig.Alerts {
		fmt.Printf("\n%s:\n", level)
		for _, action := range levelCfg.Actions {
			if actionType, ok := action["type"]; ok {
				fmt.Printf("  - type: %v\n", actionType)
				for key, value := range action {
					if key != "type" {
						fmt.Printf("    %s: %v\n", key, value)
					}
				}
			}
		}
	}

	fmt.Println("\n" + divider)
	fmt.Println("To override, create 'config.yaml' with your settings.")
	fmt.Println("See 'config-example.yaml' for a complete example.")
	fmt.Println(divider + "\n")
}
