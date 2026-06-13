.PHONY: help test lint lint-fix fmt tidy clean coverage install-tools ensure-lint check integration-up integration-down integration-test

# golangci-lint 版本锁定（随项目走，不依赖全局安装；升级改 .golangci-lint-version 即可）
GOLANGCI_VERSION := $(shell cat .golangci-lint-version)
GOLANGCI := ./bin/golangci-lint

# Default target
help:
	@echo "Available targets:"
	@echo "  make test          - Run tests"
	@echo "  make lint          - Run linter"
	@echo "  make lint-fix      - Run linter with auto-fix"
	@echo "  make fmt           - Format code"
	@echo "  make tidy          - Tidy go modules"
	@echo "  make coverage      - Generate coverage report"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make install-tools - Install pinned golangci-lint to ./bin"
	@echo "  make check         - Run full checks (fmt + lint + test)"
	@echo "  make integration-up   - Start integration test services (docker)"
	@echo "  make integration-down - Stop integration test services"
	@echo "  make integration-test - Run stealth integration tests"

# 确保 ./bin 下是项目锁定版本的 golangci-lint，版本不符则自动下载（不依赖 brew/全局安装）
ensure-lint:
	@if [ ! -x "$(GOLANGCI)" ] || ! "$(GOLANGCI)" version 2>/dev/null | grep -q "$(GOLANGCI_VERSION:v%=%)"; then \
		echo "Installing golangci-lint $(GOLANGCI_VERSION) -> ./bin ..."; \
		curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b ./bin "$(GOLANGCI_VERSION)"; \
	fi

# Install required tools
install-tools: ensure-lint
	@echo "golangci-lint $(GOLANGCI_VERSION) ready at $(GOLANGCI)"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -count=1 ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
lint: ensure-lint
	@echo "Running linter ($(GOLANGCI_VERSION))..."
	@$(GOLANGCI) run ./...

# Run linter with auto-fix
lint-fix: ensure-lint
	@echo "Running linter with auto-fix ($(GOLANGCI_VERSION))..."
	@$(GOLANGCI) run --fix ./...

# Format code
fmt: lint-fix
	@echo "Code formatted successfully!"

# Tidy dependencies
tidy:
	@echo "Tidying go modules..."
	@go mod tidy
	@echo "Dependencies tidied!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f coverage.txt coverage.html
	@go clean -cache -testcache
	@echo "Clean complete!"

# Start integration test dependencies
integration-up:
	@echo "Starting integration test services..."
	@docker compose -f browser/stealthtest/docker-compose.yml up -d
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "Services started!"

# Stop integration test dependencies
integration-down:
	@echo "Stopping integration test services..."
	@docker compose -f browser/stealthtest/docker-compose.yml down
	@echo "Services stopped!"

# Run integration tests (requires services running)
integration-test:
	@echo "Running integration tests..."
	@go test -tags integration -v -timeout 600s ./browser/stealthtest/

# Run all checks
check: fmt lint test
	@echo "All checks passed!"
