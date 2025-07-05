# Makefile for the KiCad Watcher project

# Variables
BINARY_NAME=kicad-watcher
INSTALL_PATH=/usr/local/bin
SERVICE_NAME=kicad-watcher.service
SERVICE_PATH=/etc/systemd/system

# Default target executed when you run `make`
.DEFAULT_GOAL := help

# Phony targets are not files.
.PHONY: all build clean deps install service-install service-start service-stop service-status service-enable service-disable git-add git-commit git-push git-status help

# Build the Go application
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME)

# Update dependencies
deps:
	@echo "Tidying Go module dependencies..."
	@go mod tidy

# Install the binary to the system path
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo mv $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "$(BINARY_NAME) installed."

# Install the systemd service file
# Note: This requires a 'kicad-watcher.service' file in the project directory.
service-install:
	@echo "Installing systemd service..."
	@sudo cp $(SERVICE_NAME) $(SERVICE_PATH)/$(SERVICE_NAME)
	@sudo systemctl daemon-reload
	@echo "Service file installed. Run 'make service-enable' and 'make service-start'."

# Start the systemd service
service-start:
	@echo "Starting $(SERVICE_NAME)..."
	@sudo systemctl start $(SERVICE_NAME)
	@echo "Service started."

# Stop the systemd service
service-stop:
	@echo "Stopping $(SERVICE_NAME)..."
	@sudo systemctl stop $(SERVICE_NAME)
	@echo "Service stopped."

# Get the status of the systemd service
service-status:
	@sudo systemctl status $(SERVICE_NAME)

# Enable the service to start on boot
service-enable:
	@echo "Enabling $(SERVICE_NAME) to start on boot..."
	@sudo systemctl enable $(SERVICE_NAME)
	@echo "Service enabled."

# Disable the service from starting on boot
service-disable:
	@echo "Disabling $(SERVICE_NAME) from starting on boot..."
	@sudo systemctl disable $(SERVICE_NAME)
	@echo "Service disabled."

# Git Management
git-add:
	@echo "Adding all changes to git..."
	@git add .

git-commit:
	@git commit -m "$(m)"

git-push:
	@echo "Pushing changes to origin..."
	@git push origin main

git-status:
	@git status

# Remove the built binary
clean:
	@echo "Cleaning up..."
	@rm -f $(BINARY_NAME)

# Display help information
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build            Build the application binary."
	@echo "  deps             Update Go module dependencies."
	@echo "  install          Build and install the binary to $(INSTALL_PATH)."
	@echo "  clean            Remove the built binary."
	@echo ""
	@echo "Service Management (requires sudo):"
	@echo "  service-install  Install the systemd service file."
	@echo "  service-start    Start the systemd service."
	@echo "  service-stop     Stop the systemd service."
	@echo "  service-status   Check the status of the service."
	@echo "  service-enable   Enable the service to start on boot."
	@echo "  service-disable  Disable the service from starting on boot."
	@echo ""
	@echo "Git Management:"
	@echo "  git-add          Add all changes to staging."
	@echo "  git-commit       Commit staged changes. Use with m=\"Your message\"."
	@echo "  git-push         Push commits to the main branch on origin."
	@echo "  git-status       Show the working tree status."

