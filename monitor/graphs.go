package monitor

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/ziutek/rrd"
)

// GraphConfig holds configuration for graph generation
type GraphConfig struct {
	Title          string
	Metric         string
	RRDPath        string
	WarningThresh  float64
	CriticalThresh float64
	OutputPath     string
	Width          uint
	Height         uint
}

// DefaultGraphConfig returns default graph configuration
func DefaultGraphConfig(metric string, rrdPath string) GraphConfig {
	return GraphConfig{
		Title:      fmt.Sprintf("%s Usage (Last 30 Days)", metric),
		Metric:     metric,
		RRDPath:    rrdPath,
		OutputPath: filepath.Join(rrdPath, metric+"_graph.png"),
		Width:      1200,
		Height:     400,
	}
}

// GenerateGraph generates a graph from RRD data with thresholds
func GenerateGraph(config *GraphConfig) error {
	log.Printf("Generating graph for metric: %s", config.Metric)

	rrdFile := filepath.Join(config.RRDPath, config.Metric+".rrd")

	// Create graph definition
	graphDef := rrd.NewGrapher()

	// Set graph parameters
	graphDef.SetTitle(config.Title)
	graphDef.SetSize(config.Width, config.Height)

	// Set Y-axis range (0-100 for percentages)
	graphDef.SetLowerLimit(0)
	graphDef.SetUpperLimit(100)
	graphDef.SetRigid()

	// Add data source from RRD
	graphDef.Def("metric", rrdFile, config.Metric, "AVERAGE")

	// Plot the metric line (blue)
	graphDef.Line(2, "metric", "0000FF", config.Metric)

	// Add warning threshold line (yellow)
	if config.WarningThresh > 0 {
		graphDef.HRule(fmt.Sprintf("%.2f", config.WarningThresh), "FFFF00", fmt.Sprintf("Warning (%.1f%%)", config.WarningThresh))
	}

	// Add critical threshold line (red)
	if config.CriticalThresh > 0 {
		graphDef.HRule(fmt.Sprintf("%.2f", config.CriticalThresh), "FF0000", fmt.Sprintf("Critical (%.1f%%)", config.CriticalThresh))
	}

	// Set time range (last 30 days)
	now := time.Now()
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)

	// Render the graph
	_, err := graphDef.SaveGraph(config.OutputPath, thirtyDaysAgo, now)
	if err != nil {
		return fmt.Errorf("failed to generate graph for %s: %w", config.Metric, err)
	}

	log.Printf("Graph generated: %s", config.OutputPath)
	return nil
}

// GenerateAllGraphs generates graphs for CPU, memory, and swap
func GenerateAllGraphs(rrdPath string, config *Config) error {
	metrics := []string{"cpu", "memory", "swap"}

	for _, metric := range metrics {
		graphConfig := DefaultGraphConfig(metric, rrdPath)

		// Get thresholds from config
		if metricConfig, ok := config.GetMetricConfig(metric); ok {
			graphConfig.WarningThresh = metricConfig.Thresholds["warning"]
			graphConfig.CriticalThresh = metricConfig.Thresholds["critical"]
		}

		if err := GenerateGraph(&graphConfig); err != nil {
			return err
		}
	}

	log.Printf("All graphs generated successfully")
	return nil
}
