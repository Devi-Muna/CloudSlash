# CloudSlash Automation Makefile
# v1.3.6 - Infrastructure Analysis Platform

BINARY_NAME=cloudslash
GO_FILES=$(shell find . -name '*.go')

.PHONY: all build test lint clean scan-mock

all: lint test build

build:
	@echo " -> Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) cmd/cloudslash/main.go

test:
	@echo " -> Running Unit & Integration Tests (with Race Detection)..."
	go test -race -v ./...

lint:
	@echo " -> Running Static Analysis (golangci-lint)..."
	golangci-lint run

clean:
	@echo " -> Cleaning up..."
	go clean
	rm -f $(BINARY_NAME)
	rm -f cloudslash-out/*
	rm -f waste.tf import.sh fix_terraform.sh resource_deletion.sh


