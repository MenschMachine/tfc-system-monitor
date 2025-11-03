.PHONY: install-deps build clean help test test-verbose test-coverage test-short test-race

export GOPATH := $(HOME)/.go

help:
	@echo "TFC System Monitor - Makefile targets:"
	@echo ""
	@echo "Build and Run:"
	@echo "  install-deps    Install system dependencies (RRD libraries)"
	@echo "  build           Build the binary"
	@echo "  run             Build and run in server mode"
	@echo "  cli             Build and run in CLI mode"
	@echo "  clean           Remove built binary and temporary files"
	@echo ""
	@echo "Testing:"
	@echo "  test            Run all unit tests"
	@echo "  test-verbose    Run tests with verbose output and all details"
	@echo "  test-short      Run tests in short mode (skip long-running tests)"
	@echo "  test-race       Run tests with race detector enabled"
	@echo "  test-coverage   Run tests with coverage report and generate HTML report"
	@echo ""

install-deps:
	@echo "Installing RRD dependencies..."
	@if command -v apt-get &> /dev/null; then \
		echo "Detected Debian/Ubuntu system"; \
		sudo apt-get update && sudo apt-get install -y librrd-dev; \
	elif command -v brew &> /dev/null; then \
		echo "Detected macOS with Homebrew"; \
		HOMEBREW_NO_AUTO_UPDATE=1 brew install rrdtool; \
	elif command -v dnf &> /dev/null; then \
		echo "Detected Fedora/RHEL system"; \
		sudo dnf install -y rrdtool-devel; \
	elif command -v pacman &> /dev/null; then \
		echo "Detected Arch Linux system"; \
		sudo pacman -S rrdtool; \
	else \
		echo "ERROR: Could not detect package manager"; \
		echo "Please install RRD libraries manually:"; \
		echo "  Ubuntu/Debian: sudo apt-get install librrd-dev"; \
		echo "  macOS: brew install rrdtool"; \
		echo "  Fedora/RHEL: sudo dnf install rrdtool-devel"; \
		echo "  Arch: sudo pacman -S rrdtool"; \
		exit 1; \
	fi
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy

build: install-deps
	@echo "Building tfc-system-monitor..."
	@COMMIT_HASH=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	go build -ldflags "-X main.Version=$$COMMIT_HASH" -o tfc-system-monitor .
	@echo "Build complete: ./tfc-system-monitor"

run: build
	@echo "Running in server mode..."
	./tfc-system-monitor

cli: build
	@echo "Running in CLI mode..."
	./tfc-system-monitor -cli

clean:
	@echo "Cleaning up..."
	rm -f tfc-system-monitor
	go clean
	rm -rf coverage.out coverage.html logs rrd-data reports
	@echo "Clean complete"

test:
	@echo "Running unit tests..."
	go test ./... -v

test-verbose:
	@echo "Running unit tests with verbose output..."
	go test ./... -v -count=1

test-short:
	@echo "Running unit tests (short mode)..."
	go test ./... -short -v

test-race:
	@echo "Running unit tests with race detector..."
	go test ./... -race -v

test-coverage:
	@echo "Running unit tests with coverage..."
	@go test ./... -v -coverprofile=coverage.out -covermode=atomic
	@echo ""
	@echo "Coverage Summary:"
	@go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report generated: coverage.html"
	@echo "Open with: open coverage.html (macOS) or xdg-open coverage.html (Linux)"
