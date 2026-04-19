BINARY_NAME := pre-commit
BUILD_DIR := build
CMD_DIR := cmd/pre-commit
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-X github.com/blairham/go-pre-commit/v4/internal/config.Version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build clean test lint fmt vet install

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

install:
	go install $(LDFLAGS) ./$(CMD_DIR)

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html
	go clean

test:
	go test -v -race ./...

test-cover:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go tool golangci-lint run ./...

vet:
	go vet ./...

fmt:
	go tool gofumpt -w .

tidy:
	go mod tidy

check: fmt vet test
