# TFC System Monitor - Go Port

A complete Go port of the Python tfc-system-monitor application. Monitors system metrics (CPU, memory, disk) and triggers configurable alerts when thresholds are exceeded.

## Features

- **System Monitoring**: Real-time CPU, memory, and disk usage monitoring
- **Configurable Thresholds**: Set warning and critical thresholds for each metric
- **Alert Throttling**: Prevent alert spam with configurable throttle settings
  - One-time alerts with `repeat: false`
  - Repeated alerts with configurable intervals via `repeat_interval` (e.g., "1h", "30m")
- **Multiple Alert Modes**:
  - System logger (via `logger` command)
  - Syslog (with facility and priority control)
  - HTTP Webhooks
  - Custom scripts
  - **Stdout** (for direct output or piping)
- **Persistent State**: Track alert history to prevent duplicate alerts
- **CLI and Server Modes**:
  - CLI mode for one-time checks or cron jobs
  - HTTP server mode for continuous monitoring
- **Flexible Output**: Status only printed to stdout with `--debug` flag or via configured alerts

## Installation

### Requirements
- Go 1.21 or later (for building from source)
- On Linux: standard system utilities (logger, syslog support)

### Build from Source

```bash
cd tfc-system-monitor-go
go mod tidy
./build.sh
```

Or manually:
```bash
go build -o tfc-system-monitor
```

### Linux System Installation

Follow these steps to install on a Linux system for production use:

#### 1. Build the Binary

```bash
# On your build machine
./build.sh
```

#### 2. Install Binary

```bash
# Copy to system path
sudo cp tfc-system-monitor /usr/local/bin/
sudo chmod +x /usr/local/bin/tfc-system-monitor
```

#### 3. Create Config Directory

```bash
sudo mkdir -p /etc/tfc-monitor
sudo cp config-example.yaml /etc/tfc-monitor/config.yaml
sudo chmod 644 /etc/tfc-monitor/config.yaml
```

#### 4. Configure Monitoring

Edit `/etc/tfc-monitor/config.yaml` with your alert thresholds and actions.

#### 5. Set Up as Cron Job (Recommended for CLI Mode)

For periodic checks every 5 minutes:

```bash
# Edit crontab
sudo crontab -e

# Add this line
*/5 * * * * /usr/local/bin/tfc-system-monitor -cli -config /etc/tfc-monitor/config.yaml
```

Or for continuous monitoring, create a systemd service:

```bash
sudo tee /etc/systemd/system/tfc-monitor.service > /dev/null <<EOF
[Unit]
Description=TFC System Monitor
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/tfc-system-monitor -config /etc/tfc-monitor/config.yaml -port 12349
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable tfc-monitor
sudo systemctl start tfc-monitor

# Check status
sudo systemctl status tfc-monitor
```

#### 6. Verify Installation

```bash
# Test CLI mode
/usr/local/bin/tfc-system-monitor -cli -config /etc/tfc-monitor/config.yaml -debug

# Test server mode
curl http://localhost:12349/
```

## Usage

### CLI Mode (one-time check)

```bash
# Check system status (with alerts enabled from config)
./tfc-system-monitor -cli -config /etc/tfc-monitor/config.yaml

# Enable debug logging to see status output
./tfc-system-monitor -cli -config /etc/tfc-monitor/config.yaml -debug

# Use default config
./tfc-system-monitor -cli
```

Note: Status is not printed to stdout by default. Use `-debug` flag to see it, or configure stdout alerts in the config file.

### Server Mode (continuous monitoring)

```bash
# Start server on default port 12349
./tfc-system-monitor

# Start on custom port
./tfc-system-monitor -port 8080

# With custom config
./tfc-system-monitor -config /etc/tfc-monitor/config.yaml
```

## Configuration

### Example Config File

See `config-example.yaml` for a complete example.

### Default Configuration

If no config file is provided, these defaults are used:

**Metrics:**
- **Disk**: Warning at 80%, Critical at 90%
- **CPU**: Warning at 70%, Critical at 90%
- **Memory**: Warning when free < 20%, Critical when free < 5%

**Alerts:**
- Warning violations trigger system logger
- Critical violations trigger system logger

### Configuration Options

#### Metric Configuration

```yaml
metrics:
  disk:
    enabled: true                  # Enable/disable metric
    thresholds:
      warning: 80                  # Warning threshold
      critical: 90                 # Critical threshold
    throttle:
      min_duration_minutes: 0       # Minimum duration before alerting (in minutes)
      repeat: false                # Allow repeated alerts
      repeat_interval: ""          # Interval between repeated alerts (e.g., "1h", "30m", "10s")
    unit: percentage               # Unit (for documentation)
```

#### Throttle Settings

- **min_duration_minutes**: Only alert after violation has existed for this duration (default: 0, immediate)
- **repeat**: If `true`, allow alerts to repeat. If `false`, only alert once (default: false)
- **repeat_interval**: When `repeat: true`, alert again after this interval. Supports Go duration format:
  - `"1h"` - 1 hour
  - `"30m"` - 30 minutes
  - `"10s"` - 10 seconds
  - Leave empty to repeat continuously

#### Disk Exclusions

The disk metric supports excluding specific devices, filesystem types, or mountpoints:

```yaml
metrics:
  disk:
    enabled: true
    thresholds:
      warning: 80
      critical: 90
    exclude:
      devices:       # Device patterns (glob patterns)
        - "/dev/loop*"
        - "/dev/cd*"
      filesystems:   # Filesystem types
        - "tmpfs"
        - "devfs"
        - "iso9660"
      mountpoints:   # Mountpoint patterns (glob patterns)
        - "/dev*"
        - "/sys/*"
        - "/proc/*"
```

Common exclusions:
- **Devices**: `/dev/loop*` (loop devices), `/dev/cd*` (CD/DVD drives)
- **Filesystems**: `tmpfs` (temporary), `devfs` (device filesystem), `iso9660` (CD/DVD)
- **Mountpoints**: `/dev*`, `/sys/*`, `/proc/*` (virtual filesystems)

#### Memory Mode

The memory metric supports two modes:

- **min_free** (default): Threshold represents minimum free memory percentage. Alert when free memory drops below threshold.
- **max_used**: Threshold represents maximum used memory percentage. Alert when used memory exceeds threshold.

#### Alert Actions

Supported alert types:

**Stdout** (print to standard output):
```yaml
- type: stdout
```

Prints violations directly to stdout in format: `[LEVEL] metric: message`
Useful for CLI mode, cron jobs, or piping to other tools.

**Logger** (via `logger` command):
```yaml
- type: logger
  level: warning
```

**Syslog** (direct syslog):
```yaml
- type: syslog
  tag: tfc-monitor           # Syslog tag
  facility: local0           # Syslog facility
  priority: warning          # Syslog priority
```

Facilities: user, mail, daemon, auth, syslog, lpr, news, uucp, cron, local0-7
Priorities: emergency, alert, critical, error, warning, notice, info, debug

**Webhook** (HTTP POST):
```yaml
- type: webhook
  url: https://example.com/alerts
  timeout: 5                 # Timeout in seconds
  retry: 3                   # Number of retry attempts
```

Payload: `{"metric": "...", "level": "...", "message": "...", "value": ...}`

**Script** (execute command):
```yaml
- type: script
  path: /usr/local/bin/alert.sh
  args:                      # Optional arguments
    - "--notify"
  timeout: 30                # Timeout in seconds
```

Script receives: `script_path arg1 arg2 metric level message`

## HTTP Endpoints

### GET /

Returns system status as JSON:

```json
{
    "status": "OK",
    "info": []
}
```

Status values: `OK`, `WARN`, `CRITICAL`
Info contains details about any violations.

### GET /health

Returns simple health check:

```json
{"status": "OK"}
```

## State Management

Alert state is persisted to `/tmp/tfc-monitor-state.json` to:
- Track when violations started
- Prevent duplicate alerts
- Support throttling logic

The state file is automatically managed and requires no configuration.

## Examples

### Basic Monitoring

```bash
# Start server
./tfc-system-monitor --port 12349

# In another terminal, check status
curl http://localhost:12349/
```

### With Stdout Alerts

Create `config.yaml`:
```yaml
metrics:
  disk:
    enabled: true
    thresholds:
      warning: 75
      critical: 85
    throttle:
      min_duration_minutes: 0
      repeat: true
      repeat_interval: "1h"

alerts:
  warning:
    actions:
      - type: stdout
  critical:
    actions:
      - type: stdout
```

Run to see violations on stdout:
```bash
./tfc-system-monitor -cli -config config.yaml
```

### Cron Job with Alerts

```bash
# Add to crontab for every 5 minutes
*/5 * * * * /usr/local/bin/tfc-system-monitor -cli -config /etc/tfc-monitor/config.yaml

# Or with email notification
*/5 * * * * /usr/local/bin/tfc-system-monitor -cli -config /etc/tfc-monitor/config.yaml 2>&1 | mail -s "System Monitor Alerts" admin@example.com
```

### Multiple Alert Channels

```yaml
alerts:
  warning:
    actions:
      - type: stdout
      - type: logger
        level: warning
  critical:
    actions:
      - type: stdout
      - type: webhook
        url: https://alerts.example.com/critical
        timeout: 5
        retry: 3
```

## Logging

Logs are written to stdout/stderr. Log level can be controlled via:
- `--debug` flag for verbose logging
- Default level is INFO

## Comparison with Python Version

### Advantages of Go Version
- **Single Binary**: No dependencies or virtualenv needed
- **Performance**: ~10x faster system startup time
- **Lower Memory**: Minimal memory footprint
- **Cross-platform**: Easy deployment to different architectures

### Compatibility
The Go port maintains 100% compatibility with the Python version's:
- Configuration format (YAML)
- Alert actions and payloads
- HTTP API responses
- State management format
- Logging output

## Troubleshooting

### Config file not found
The monitor will use default configuration. To verify defaults, run with `--cli` flag.

### Permission denied for script alert
Ensure the script has execute permissions:
```bash
chmod +x /path/to/alert-script.sh
```

### Syslog not working
On macOS, syslog may require special configuration. Use logger alert type instead.

### Debug logging
Enable with `--debug` flag to see detailed operation logs.

## License

Same as original tfc-system-monitor
