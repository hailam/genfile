# Makefile for genfile CLI tool

# Name of the output binary
BINARY := genfile

# Go build command and flags
GO      := go
GOFLAGS :=

.PHONY: all build clean

# Default target: build the binary
all: build

# Build genfile
build:
	@echo "→ Building $(BINARY)..."
	$(GO) build $(GOFLAGS) -o $(BINARY) .

# Remove the binary
clean:
	@echo "→ Cleaning up..."
	rm -f $(BINARY)
