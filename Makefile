.PHONY: run build test coverage-gate help

CONFIG ?= config.local
BINARY ?= platform-service

help:
	@echo "Available commands:"
	@echo "  make run           - Run platform service locally (uses config.local.yaml by default)"
	@echo "  make build         - Build server binary"
	@echo "  make test          - Run go tests"
	@echo "  make coverage-gate - Run Go coverage regression gate"

run:
	@if [ ! -f $(CONFIG).yaml ]; then 		echo "Error: $(CONFIG).yaml not found!"; 		exit 1; 	fi
	go run ./cmd/server -config $(CONFIG)

build:
	go build -o $(BINARY) ./cmd/server

test:
	go test ./...

coverage-gate:
	bash scripts/coverage-gate.sh
