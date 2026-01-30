# CloudSlash Enterprise Automation Makefile
# v2.1.1 - Infrastructure Optimization Platform

BINARY_NAME=cloudslash
GO_FILES=$(shell find . -name '*.go')
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# ANSI Colors
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

.PHONY: all build test e2e lint clean install help

all: lint test build

help:
	@echo "$(GREEN)CloudSlash Automation$(NC)"
	@echo "  make build    - Compile binary"
	@echo "  make test     - Run unit tests (fast)"
	@echo "  make e2e      - Run full verification suite (slow)"
	@echo "  make lint     - Static analysis"
	@echo "  make clean    - Remove artifacts and reports"

build:
	@echo "$(GREEN)-> Building $(BINARY_NAME) $(VERSION)...$(NC)"
	@go build -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)" -o $(BINARY_NAME) cmd/cloudslash-cli/main.go

test:
	@echo "$(GREEN)-> Running Unit Tests...$(NC)"
	@go test -v -short ./pkg/... ./cmd/...

e2e:
	@echo "$(YELLOW)-> Running END-TO-END Verification Suite (The Lazarus Protocol)...$(NC)"
	@echo "$(YELLOW)-> Note: This requires Docker (LocalStack) or valid AWS credentials.$(NC)"
	@go test -v ./test/e2e/...

lint:
	@echo "$(GREEN)-> Running Static Analysis...$(NC)"
	@golangci-lint run

clean:
	@echo "$(GREEN)-> Cleaning artifacts...$(NC)"
	@rm -f $(BINARY_NAME)
	@rm -rf cloudslash-out/
	@rm -rf .cloudslash/
	@rm -f waste.tf import.sh fix_terraform.sh safe_cleanup.sh undo_cleanup.sh restore.tf
	@rm -f coverage.out
	@echo "Done."

install: build
	@echo "$(GREEN)-> Installing to /usr/local/bin...$(NC)"
	@mv $(BINARY_NAME) /usr/local/bin/

