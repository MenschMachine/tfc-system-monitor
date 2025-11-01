.PHONY: install-deps build clean help

help:
	@echo "TFC System Monitor - Makefile targets:"
	@echo ""
	@echo "  install-deps    Install system dependencies (RRD libraries)"
	@echo "  build           Build the binary"
	@echo "  run             Build and run in server mode"
	@echo "  cli             Build and run in CLI mode"
	@echo "  clean           Remove built binary and temporary files"
	@echo ""

install-deps:
	@echo "Installing RRD dependencies..."
	@if command -v apt-get &> /dev/null; then \
		echo "Detected Debian/Ubuntu system"; \
		sudo apt-get update && sudo apt-get install -y librrd-dev; \
	elif command -v brew &> /dev/null; then \
		echo "Detected macOS with Homebrew"; \
		brew install rrdtool; \
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
	@echo "Clean complete"
