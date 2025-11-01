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
	"strings"
	"syscall"
	"time"

	"github.com/MenschMachine/tfc-system-monitor/monitor"
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
	cliMode    = flag.Bool("cli", false, "")
	configPath = flag.String("config", "config.yaml", "")
	debugMode  = flag.Bool("debug", false, "")
	port       = flag.Int("port", 12349, "")
	reportMode = flag.Bool("report", false, "")
	rrdPath    = flag.String("rrd-path", "./rrd-data", "")
	versionFlag = flag.Bool("version", false, "")
)

// Set at build time with -ldflags
var Version = "dev"

func printHelp() {
	fmt.Fprintf(flag.CommandLine.Output(), `TFC System Monitor - Monitor system resources and generate alerts

USAGE:
  tfc-system-monitor [OPTIONS]

FLAGS:
  -cli
      Run in command-line mode. Checks system status once and exits.
      Useful for cron jobs or one-time checks.

  -config string
      Path to YAML configuration file (default: "config.yaml")
      Defines thresholds, alert actions, and monitoring settings.
      See example: https://github.com/MenschMachine/tfc-system-monitor/blob/main/config-example.yaml

  -debug
      Enable debug logging. Shows detailed log output including file names and line numbers.
      Useful for troubleshooting issues.

  -port int
      Port for HTTP server (default: 12349)
      Only used when running in server mode (default).
      The server exposes endpoints: / (status) and /health

  -report
      Generate an HTML report from collected RRD data and exit.
      Creates a timestamped report file in the ./reports directory.

  -rrd-path string
      Path to RRD (Round-Robin Database) data directory (default: "./rrd-data")
      Where historical metrics are stored. Directory will be created if it doesn't exist.
      Can also be set in config file via 'rrd_path' key. Flag overrides config file.

  -h, -help
      Show this help message

MODES:
  Server Mode (default)
    Runs as an HTTP server on the specified port.
    Continuously monitors system metrics and responds to HTTP requests.
    Use for long-running monitoring with external polling.

  CLI Mode (-cli flag)
    Single check mode. Useful for integration with cron, alerting systems, or scripts.

  Report Mode (-report flag)
    Generates an HTML report from historical RRD data.
    Requires prior data collection in server or CLI mode.

DOCUMENTATION:
  README:        https://github.com/MenschMachine/tfc-system-monitor/blob/main/README.md
  Config Example: https://github.com/MenschMachine/tfc-system-monitor/blob/main/config-example.yaml

INSTALLATION:
  go install github.com/MenschMachine/tfc-system-monitor@latest

EXAMPLES:
  # Start server on default port (12349)
  tfc-system-monitor

  # Start server on custom port
  tfc-system-monitor -port 8080

  # Check system status once and exit
  tfc-system-monitor -cli

  # Enable debug logging
  tfc-system-monitor -debug

  # Use custom config file
  tfc-system-monitor -config /etc/monitor/config.yaml

  # Generate report from collected data
  tfc-system-monitor -report
`)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Usage = printHelp

	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "-help" || arg == "--help" {
			printHelp()
			return nil
		}
		if arg == "-version" || arg == "--version" {
			fmt.Println(Version)
			return nil
		}
	}

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	if flag.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flag.Args(), " "))
	}

	// Configure logging
	if *debugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		// Disable logging when not in debug mode
		log.SetOutput(io.Discard)
	}

	switch {
	case *reportMode:
		return runReport()
	case *cliMode:
		return runCLI()
	default:
		return runServer()
	}
}

// runReport generates a report from RRD data and exits
func runReport() error {
	config, err := monitor.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Use --rrd-path flag if provided, otherwise use config value
	rrdPathToUse := *rrdPath
	if config.RRDPath != "" && *rrdPath == "./rrd-data" {
		rrdPathToUse = config.RRDPath
	}

	reporter := monitor.NewReporter(rrdPathToUse, config, fmt.Sprintf("./reports/report-%s.html", time.Now().Format("2006-01-02")))
	if err := reporter.Generate(); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	fmt.Printf("Report generated successfully: %s\n", reporter.OutputPath)
	return nil
}

// runCLI runs the monitor in command-line mode
func runCLI() error {
	config, err := monitor.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Use --rrd-path flag if provided, otherwise use config value
	rrdPathToUse := *rrdPath
	if config.RRDPath != "" && *rrdPath == "./rrd-data" {
		rrdPathToUse = config.RRDPath
	}

	recorder := monitor.NewRecorder(rrdPathToUse)
	if err := recorder.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize recorder: %w", err)
	}

	stateManager, err := monitor.NewStateManager()
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}
	status, err := checkSystemStatus(config, stateManager, recorder)
	if err != nil {
		return err
	}

	if *debugMode {
		fmt.Println(status.ToJSON())
	}
	return nil
}

// runServer runs the monitor as an HTTP server
func runServer() error {
	config, err := monitor.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Use --rrd-path flag if provided, otherwise use config value
	rrdPathToUse := *rrdPath
	if config.RRDPath != "" && *rrdPath == "./rrd-data" {
		rrdPathToUse = config.RRDPath
	}

	recorder := monitor.NewRecorder(rrdPathToUse)
	if err := recorder.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize recorder: %w", err)
	}

	stateManager, err := monitor.NewStateManager()
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("GET %s from %s", r.RequestURI, r.RemoteAddr)
		status, err := checkSystemStatus(config, stateManager, recorder)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			http.Error(w, `{"status":"ERROR","info":["internal server error"]}`, http.StatusInternalServerError)
			return
		}
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
		return fmt.Errorf("server error: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

// checkSystemStatus checks system status and returns a Status object
func checkSystemStatus(config *monitor.Config, stateManager *monitor.StateManager, recorder *monitor.Recorder) (*Status, error) {
	status := &Status{Status: "OK", Info: []string{}}

	// Get system statistics
	stats, err := monitor.GetSystemStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get system stats: %w", err)
	}

	// Record metrics to RRD
	if recorder != nil {
		if err := recorder.Record(stats); err != nil {
			return nil, fmt.Errorf("failed to record metrics: %w", err)
		}
	}

	// Check thresholds
	warningViolations, criticalViolations, err := monitor.CheckAllThresholds(config, stats, stateManager)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate thresholds: %w", err)
	}

	// Add violations to status
	for _, violation := range criticalViolations {
		status.AddCritical(violation.Metric, violation.Message)
	}

	for _, violation := range warningViolations {
		status.AddWarning(violation.Metric, violation.Message)
	}

	// Process violations (alerts)
	if err := monitor.ProcessViolations(config, warningViolations, criticalViolations); err != nil {
		return nil, fmt.Errorf("failed to process violations: %w", err)
	}

	return status, nil
}
