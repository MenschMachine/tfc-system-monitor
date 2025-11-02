package monitor

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Reporter generates HTML reports with embedded graphs
type Reporter struct {
	RRDPath    string
	Config     *Config
	OutputPath string
}

// NewReporter creates a new Reporter instance
func NewReporter(rrdPath string, config *Config, outputPath string) *Reporter {
	return &Reporter{
		RRDPath:    rrdPath,
		Config:     config,
		OutputPath: outputPath,
	}
}

// Generate creates the HTML report with embedded graphs
func (r *Reporter) Generate() error {
	log.Printf("Generating report")

	// Generate graphs
	if err := GenerateAllGraphs(r.RRDPath, r.Config); err != nil {
		return fmt.Errorf("failed to generate graphs: %w", err)
	}

	// Read generated graph images
	cpuGraphPath := filepath.Join(r.RRDPath, "cpu_graph.png")
	memGraphPath := filepath.Join(r.RRDPath, "memory_graph.png")
	swapGraphPath := filepath.Join(r.RRDPath, "swap_graph.png")

	cpuGraphData, err := encodeImageToBase64(cpuGraphPath)
	if err != nil {
		return fmt.Errorf("failed to read CPU graph: %w", err)
	}

	memGraphData, err := encodeImageToBase64(memGraphPath)
	if err != nil {
		return fmt.Errorf("failed to read memory graph: %w", err)
	}

	swapGraphData, err := encodeImageToBase64(swapGraphPath)
	if err != nil {
		return fmt.Errorf("failed to read swap graph: %w", err)
	}

	// Generate HTML
	html := r.generateHTML(cpuGraphData, memGraphData, swapGraphData)

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(r.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write HTML to file
	if err := os.WriteFile(r.OutputPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write report to %s: %w", r.OutputPath, err)
	}

	log.Printf("Report generated: %s", r.OutputPath)
	return nil
}

// encodeImageToBase64 reads an image file and returns base64-encoded data
func encodeImageToBase64(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file %s: %w", imagePath, err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// generateHTML creates the HTML report content
func (r *Reporter) generateHTML(cpuGraphData, memGraphData, swapGraphData string) string {
	now := time.Now()
	reportTitle := fmt.Sprintf("System Monitor Report - %s", now.Format("2006-01-02 15:04:05"))

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background-color: #f5f5f5;
            color: #333;
            line-height: 1.6;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 20px;
        }
        header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 40px 20px;
            border-radius: 8px;
            margin-bottom: 40px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        h1 {
            font-size: 32px;
            margin-bottom: 10px;
        }
        .timestamp {
            font-size: 14px;
            opacity: 0.9;
        }
        .content {
            display: grid;
            grid-template-columns: 1fr;
            gap: 30px;
        }
        .graph-section {
            background: white;
            padding: 25px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }
        .graph-section h2 {
            font-size: 24px;
            margin-bottom: 20px;
            color: #333;
            border-bottom: 3px solid #667eea;
            padding-bottom: 10px;
        }
        .graph-image {
            width: 100%%;
            height: auto;
            border-radius: 4px;
        }
        footer {
            text-align: center;
            margin-top: 40px;
            padding: 20px;
            color: #666;
            font-size: 12px;
            border-top: 1px solid #e0e0e0;
        }
        .threshold-legend {
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid #e0e0e0;
            font-size: 13px;
            color: #666;
        }
        .threshold-item {
            display: inline-block;
            margin-right: 20px;
            margin-top: 5px;
        }
        .color-indicator {
            display: inline-block;
            width: 12px;
            height: 12px;
            border-radius: 2px;
            margin-right: 5px;
            vertical-align: middle;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>%s</h1>
            <div class="timestamp">Generated: %s</div>
        </header>

        <div class="content">
            <div class="graph-section">
                <h2>CPU Usage (Last 30 Days)</h2>
                <img src="data:image/png;base64,%s" alt="CPU Usage Graph" class="graph-image">
                <div class="threshold-legend">
                    <div class="threshold-item">
                        <span class="color-indicator" style="background-color: #0000FF;"></span>
                        CPU Usage
                    </div>
                    <div class="threshold-item">
                        <span class="color-indicator" style="background-color: #FFFF00;"></span>
                        Warning Threshold
                    </div>
                    <div class="threshold-item">
                        <span class="color-indicator" style="background-color: #FF0000;"></span>
                        Critical Threshold
                    </div>
                </div>
            </div>

            <div class="graph-section">
                <h2>Memory Usage (Last 30 Days)</h2>
                <img src="data:image/png;base64,%s" alt="Memory Usage Graph" class="graph-image">
                <div class="threshold-legend">
                    <div class="threshold-item">
                        <span class="color-indicator" style="background-color: #0000FF;"></span>
                        Memory Usage
                    </div>
                    <div class="threshold-item">
                        <span class="color-indicator" style="background-color: #FFFF00;"></span>
                        Warning Threshold
                    </div>
                    <div class="threshold-item">
                        <span class="color-indicator" style="background-color: #FF0000;"></span>
                        Critical Threshold
                    </div>
                </div>
            </div>

            <div class="graph-section">
                <h2>Swap Usage (Last 30 Days)</h2>
                <img src="data:image/png;base64,%s" alt="Swap Usage Graph" class="graph-image">
                <div class="threshold-legend">
                    <div class="threshold-item">
                        <span class="color-indicator" style="background-color: #0000FF;"></span>
                        Swap Usage
                    </div>
                    <div class="threshold-item">
                        <span class="color-indicator" style="background-color: #FFFF00;"></span>
                        Warning Threshold
                    </div>
                    <div class="threshold-item">
                        <span class="color-indicator" style="background-color: #FF0000;"></span>
                        Critical Threshold
                    </div>
                </div>
            </div>
        </div>

        <footer>
            <p>TFC System Monitor Report • Generated on %s</p>
        </footer>
    </div>
</body>
</html>`,
		reportTitle,
		reportTitle,
		now.Format("2006-01-02 15:04:05 MST"),
		cpuGraphData,
		memGraphData,
		swapGraphData,
		now.Format("2006-01-02 15:04:05"),
	)

	return html
}
