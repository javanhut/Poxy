# Poxy Makefile
# Universal Package Manager

# Variables
BINARY_NAME=poxy
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build flags
LDFLAGS=-ldflags "-s -w -X poxy/internal/cli.Version=$(VERSION) -X poxy/internal/cli.Commit=$(COMMIT) -X poxy/internal/cli.BuildTime=$(BUILD_TIME)"

# Directories
BUILD_DIR=build
CMD_DIR=./cmd/poxy

# Platforms for cross-compilation
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Default target
.DEFAULT_GOAL := build

# Phony targets
.PHONY: all build build-all clean test test-verbose test-coverage lint fmt vet install uninstall run run-dev help completions release

## help: Show this help message
help:
	@echo "Poxy - Universal Package Manager"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

## build-all: Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-$${platform%/*}-$${platform#*/}$(if $(findstring windows,$${platform}),.exe,) $(CMD_DIR); \
		echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-$${platform%/*}-$${platform#*/}"; \
	done

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) ./... -count=1

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	$(GOTEST) -v ./... -count=1

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linters
lint: vet
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## install: Install to ~/.local/bin or GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	@if [ -n "$(GOPATH)" ]; then \
		mkdir -p $(GOPATH)/bin; \
		cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME); \
		echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"; \
	else \
		mkdir -p ~/.local/bin; \
		cp $(BUILD_DIR)/$(BINARY_NAME) ~/.local/bin/$(BINARY_NAME); \
		echo "Installed to ~/.local/bin/$(BINARY_NAME)"; \
		echo "Make sure ~/.local/bin is in your PATH"; \
	fi

## install-system: Install to /usr/local/bin (requires sudo)
install-system: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"

## uninstall: Remove from ~/.local/bin or GOPATH/bin
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@rm -f ~/.local/bin/$(BINARY_NAME) 2>/dev/null || true
	@if [ -n "$(GOPATH)" ]; then rm -f $(GOPATH)/bin/$(BINARY_NAME) 2>/dev/null || true; fi
	@rm -f /usr/local/bin/$(BINARY_NAME) 2>/dev/null || true
	@echo "Uninstalled"

## uninstall-system: Remove from /usr/local/bin (requires sudo)
uninstall-system:
	@echo "Uninstalling $(BINARY_NAME) from /usr/local/bin..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstalled"

## run: Build and run (use ARGS="command" to pass arguments)
run: build
	@$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

## run-dev: Run directly with go run (faster for development)
run-dev:
	@$(GOCMD) run $(CMD_DIR) $(ARGS)

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## completions: Generate shell completions
completions: build
	@echo "Generating shell completions..."
	@mkdir -p $(BUILD_DIR)/completions
	@$(BUILD_DIR)/$(BINARY_NAME) completion bash > $(BUILD_DIR)/completions/$(BINARY_NAME).bash
	@$(BUILD_DIR)/$(BINARY_NAME) completion zsh > $(BUILD_DIR)/completions/_$(BINARY_NAME)
	@$(BUILD_DIR)/$(BINARY_NAME) completion fish > $(BUILD_DIR)/completions/$(BINARY_NAME).fish
	@$(BUILD_DIR)/$(BINARY_NAME) completion powershell > $(BUILD_DIR)/completions/$(BINARY_NAME).ps1
	@echo "Completions generated in $(BUILD_DIR)/completions/"

## install-completions-bash: Install bash completions
install-completions-bash: completions
	@echo "Installing bash completions..."
	@sudo cp $(BUILD_DIR)/completions/$(BINARY_NAME).bash /etc/bash_completion.d/$(BINARY_NAME)
	@echo "Installed. Restart your shell or run: source /etc/bash_completion.d/$(BINARY_NAME)"

## install-completions-zsh: Install zsh completions
install-completions-zsh: completions
	@echo "Installing zsh completions..."
	@mkdir -p ~/.zsh/completions
	@cp $(BUILD_DIR)/completions/_$(BINARY_NAME) ~/.zsh/completions/
	@echo "Installed. Add 'fpath=(~/.zsh/completions $$fpath)' to your .zshrc"

## install-completions-fish: Install fish completions
install-completions-fish: completions
	@echo "Installing fish completions..."
	@mkdir -p ~/.config/fish/completions
	@cp $(BUILD_DIR)/completions/$(BINARY_NAME).fish ~/.config/fish/completions/
	@echo "Installed."

## release: Create release archives
release: build-all
	@echo "Creating release archives..."
	@mkdir -p $(BUILD_DIR)/release
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		binary=$(BUILD_DIR)/$(BINARY_NAME)-$$os-$$arch$$ext; \
		if [ -f "$$binary" ]; then \
			if [ "$$os" = "windows" ]; then \
				zip -j $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-$$os-$$arch.zip $$binary; \
			else \
				tar -czf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-$$os-$$arch.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-$$os-$$arch; \
			fi; \
		fi; \
	done
	@echo "Release archives created in $(BUILD_DIR)/release/"

## version: Show version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

## docker-build: Build docker image
docker-build:
	@echo "Building docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

## all: Clean, test, build
all: clean test build
