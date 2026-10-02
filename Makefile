BINARY_NAME := pre-commit
BUILD_DIR := build
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-X github.com/blairham/go-pre-commit/v4/internal/config.Version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build clean test test-cover fmt vet tidy check install

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

install:
	go install $(LDFLAGS) .

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html
	go clean

test:
	go test -v -race ./...

test-cover:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

vet:
	go vet ./...

fmt:
	go tool gofumpt -w .

tidy:
	go mod tidy

# There is no lint target: golangci-lint runs as a pre-commit hook and in CI.
check: vet test ## What CI runs (minus lint, which is the commit hook's job)
	go build ./...
