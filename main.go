package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/tfc/system-monitor/monitor"
)

// Status represents the overall system status response
type Status struct {
	Status string   `json:"status"`
	Info   []string `json:"info"`
}

// ToJSON converts Status to JSON string
func (s *Status) ToJSON() string {
	data, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return `{"status": "ERROR", "info": ["Failed to marshal status"]}`
	}
	return string(data)
}

// AddWarning adds a warning without changing status to CRITICAL
func (s *Status) AddWarning(category string, msg string) {
	if s.Status == "OK" {
		s.Status = "WARN"
	}
	s.Info = append(s.Info, fmt.Sprintf("%s: %s", category, msg))
}

// AddCritical adds a critical issue and changes status to CRITICAL
func (s *Status) AddCritical(category string, msg string) {
	s.Status = "CRITICAL"
	s.Info = append(s.Info, fmt.Sprintf("%s: %s", category, msg))
}

var (
	cliMode    = flag.Bool("cli", false, "Run in command line mode")
	configPath = flag.String("config", "config.yaml", "Path to config file")
	useAlerts  = flag.Bool("alert", false, "Execute configured alert actions")
	debugMode  = flag.Bool("debug", false, "Enable debug logging")
	port       = flag.Int("port", 12349, "Port for HTTP server")
)

func main() {
	flag.Parse()

	// Configure logging
	if *debugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		// Disable logging when not in debug mode
		log.SetOutput(io.Discard)
	}

	if *cliMode {
		// Run in CLI mode
		runCLI()
	} else {
		// Run in server mode
		runServer()
	}
}

// runCLI runs the monitor in command-line mode
func runCLI() {
	config, err := monitor.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	stateManager := monitor.NewStateManager()
	status := checkSystemStatus(config, stateManager)

	if *useAlerts {
		// Extract violations for alert processing
		stats, err := monitor.GetSystemStats()
		if err != nil {
			log.Fatalf("Failed to get system stats: %v", err)
		}
		warningViolations, criticalViolations := monitor.CheckAllThresholds(config, stats, stateManager)
		monitor.ProcessViolations(config, warningViolations, criticalViolations)
	}

	fmt.Println(status.ToJSON())
}

// runServer runs the monitor as an HTTP server
func runServer() {
	config, err := monitor.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	stateManager := monitor.NewStateManager()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("GET %s from %s", r.RequestURI, r.RemoteAddr)
		status := checkSystemStatus(config, stateManager)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(status.ToJSON()))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "OK"}`))
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting TFC System Monitor server on %s", addr)

	server := &http.Server{Addr: addr}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)
		if err := server.Close(); err != nil {
			log.Printf("Server close error: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped")
}

// checkSystemStatus checks system status and returns a Status object
func checkSystemStatus(config *monitor.Config, stateManager *monitor.StateManager) *Status {
	status := &Status{Status: "OK", Info: []string{}}

	// Get system statistics
	stats, err := monitor.GetSystemStats()
	if err != nil {
		status.AddCritical("system", fmt.Sprintf("Failed to get system stats: %v", err))
		return status
	}

	// Check thresholds
	warningViolations, criticalViolations := monitor.CheckAllThresholds(config, stats, stateManager)

	// Add violations to status
	for _, violation := range criticalViolations {
		status.AddCritical(violation.Metric, violation.Message)
	}

	for _, violation := range warningViolations {
		status.AddWarning(violation.Metric, violation.Message)
	}

	// Process alerts if enabled
	if *useAlerts {
		monitor.ProcessViolations(config, warningViolations, criticalViolations)
	}

	return status
}
