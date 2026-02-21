SHELL := /usr/bin/env bash

GO ?= go
APP ?= netcheck
CMD ?= ./cmd/netcheck
BUILD_DIR ?= ./bin
BIN ?= $(BUILD_DIR)/$(APP)
GOCACHE ?= $(CURDIR)/.gocache
PREFIX ?= $(HOME)/.local
BIN_DIR ?= $(PREFIX)/bin

.PHONY: help build run soak test coverage fmt vet clean install uninstall

help:
	@echo "Targets:"
	@echo "  make build      Build $(APP) into $(BIN)"
	@echo "  make run        Run netcheck run with ./netcheck.yaml"
	@echo "  make soak       Run netcheck soak with ./netcheck.yaml"
	@echo "  make test       Run go test ./..."
	@echo "  make coverage   Run tests and write coverage profile to $(BUILD_DIR)/coverage.out"
	@echo "  make fmt        Run go fmt ./..."
	@echo "  make vet        Run go vet ./..."
	@echo "  make install    Install binary to BIN_DIR (default: $(BIN_DIR))"
	@echo "  make uninstall  Remove binary from BIN_DIR"
	@echo "  make clean      Remove build artifacts"

build:
	@mkdir -p "$(BUILD_DIR)"
	GOCACHE="$(GOCACHE)" $(GO) build -o "$(BIN)" "$(CMD)"

run:
	GOCACHE="$(GOCACHE)" $(GO) run "$(CMD)" run --config netcheck.yaml

soak:
	GOCACHE="$(GOCACHE)" $(GO) run "$(CMD)" soak --config netcheck.yaml

test:
	GOCACHE="$(GOCACHE)" $(GO) test ./...

coverage:
	@mkdir -p "$(BUILD_DIR)"
	GOCACHE="$(GOCACHE)" $(GO) test ./... -coverprofile="$(BUILD_DIR)/coverage.out"

fmt:
	GOCACHE="$(GOCACHE)" $(GO) fmt ./...

vet:
	GOCACHE="$(GOCACHE)" $(GO) vet ./...

install:
	bash ./scripts/install.sh --bin-dir "$(BIN_DIR)"

uninstall:
	rm -f "$(BIN_DIR)/$(APP)"

clean:
	rm -rf "$(BUILD_DIR)"
