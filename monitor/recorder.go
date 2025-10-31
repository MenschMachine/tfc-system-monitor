package monitor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ziutek/rrd"
)

// Recorder manages RRD files for metrics recording
type Recorder struct {
	RRDPath string
}

// NewRecorder creates a new Recorder instance
func NewRecorder(rrdPath string) *Recorder {
	return &Recorder{
		RRDPath: rrdPath,
	}
}

// Initialize creates RRD files if they don't exist
func (r *Recorder) Initialize() error {
	log.Printf("Initializing RRD recorder")

	// Create RRD directory if it doesn't exist
	if err := os.MkdirAll(r.RRDPath, 0755); err != nil {
		return fmt.Errorf("failed to create RRD directory: %w", err)
	}

	// Create RRD files
	if err := r.createRRDIfNotExists("cpu"); err != nil {
		return err
	}
	if err := r.createRRDIfNotExists("memory"); err != nil {
		return err
	}
	if err := r.createRRDIfNotExists("swap"); err != nil {
		return err
	}

	log.Printf("RRD recorder initialized")
	return nil
}

// createRRDIfNotExists creates an RRD file if it doesn't already exist
func (r *Recorder) createRRDIfNotExists(metric string) error {
	rrdFile := filepath.Join(r.RRDPath, metric+".rrd")

	// Check if file already exists
	if _, err := os.Stat(rrdFile); err == nil {
		log.Printf("RRD file already exists: %s", rrdFile)
		return nil
	}

	log.Printf("Creating RRD file: %s", rrdFile)

	// RRD configuration:
	// - Step: 60 seconds (matches monitoring frequency)
	// - Data source: GAUGE (absolute values, not counters)
	// - Archive: 5-min averages for 30 days
	// 30 days * 24 hours * 60 minutes / 5 minutes = 8640 data points

	now := time.Now()
	creator := rrd.NewCreator(rrdFile, now, 60)
	creator.RRA("AVERAGE", 0.5, 5, 8640) // 5-min averages, 8640 entries = 30 days

	// Add data source for the metric
	creator.DS(metric, "GAUGE", 120, 0, 100)

	if err := creator.Create(true); err != nil {
		return fmt.Errorf("failed to create RRD file %s: %w", rrdFile, err)
	}

	log.Printf("RRD file created: %s", rrdFile)
	return nil
}

// Record records system metrics to RRD files
func (r *Recorder) Record(stats *SystemStats) error {
	timestamp := time.Now().Unix()

	// Record CPU usage
	cpuUsage, err := strconv.ParseFloat(stats.CPUInfo.TotalCPUUsage, 64)
	if err != nil {
		log.Printf("Error parsing CPU usage: %v", err)
	} else {
		if err := r.recordMetric("cpu", cpuUsage, timestamp); err != nil {
			log.Printf("Error recording CPU metric: %v", err)
		}
	}

	// Record memory usage (percentage used)
	memUsed, err := strconv.ParseFloat(stats.MemoryInfo.VirtualMemory.Percentage, 64)
	if err != nil {
		log.Printf("Error parsing memory usage: %v", err)
	} else {
		if err := r.recordMetric("memory", memUsed, timestamp); err != nil {
			log.Printf("Error recording memory metric: %v", err)
		}
	}

	// Record swap usage (percentage used)
	swapUsed, err := strconv.ParseFloat(stats.MemoryInfo.SwapMemory.Percentage, 64)
	if err != nil {
		log.Printf("Error parsing swap usage: %v", err)
	} else {
		if err := r.recordMetric("swap", swapUsed, timestamp); err != nil {
			log.Printf("Error recording swap metric: %v", err)
		}
	}

	log.Printf("Metrics recorded at timestamp %d", timestamp)
	return nil
}

// recordMetric records a single metric value to RRD
func (r *Recorder) recordMetric(metric string, value float64, timestamp int64) error {
	rrdFile := filepath.Join(r.RRDPath, metric+".rrd")

	// Clamp value to valid range (0-100 for percentages)
	if value < 0 {
		value = 0
	} else if value > 100 {
		value = 100
	}

	// Update RRD file
	updater := rrd.NewUpdater(rrdFile)

	if err := updater.Update(timestamp, value); err != nil {
		return fmt.Errorf("failed to update RRD file %s: %w", rrdFile, err)
	}

	log.Printf("Recorded %s: %.2f at %d", metric, value, timestamp)
	return nil
}

// GetRRDPath returns the RRD file path for a metric
func (r *Recorder) GetRRDPath(metric string) string {
	return filepath.Join(r.RRDPath, metric+".rrd")
}
