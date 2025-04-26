# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTIDY=$(GOCMD) mod tidy # Needed for the build dependency

# Project parameters
BINARY_NAME=genfile
BINARY_DIR=.
MAIN_PACKAGE=./cmd/cli/main.go

.PHONY: build tidy

# Default target
all: build

# Build the application binary
# Depends on 'tidy' to ensure dependencies are correct before building
build: tidy
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR) # Ensure the output directory exists
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "$(BINARY_NAME) built successfully in $(BINARY_DIR)/"

# Tidy go.mod and go.sum (Dependency for build)
tidy:
	@echo "Tidying dependencies..."
	$(GOTIDY)

# Clean build artifacts (Optional, but good practice to keep)
clean:
	@echo "Cleaning..."
	rm -rf $(BINARY_DIR)
	@echo "Clean complete."

