.PHONY: help test lint lint-fix fmt tidy clean coverage install-tools check integration-up integration-down integration-test

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
	@echo "  make install-tools - Install required tools"
	@echo "  make check         - Run full checks (fmt + lint + test)"
	@echo "  make integration-up   - Start integration test services (docker)"
	@echo "  make integration-down - Stop integration test services"
	@echo "  make integration-test - Run stealth integration tests"

# Install required tools
install-tools:
	@echo "Installing required tools..."
	@command -v golangci-lint >/dev/null || \
		(command -v brew >/dev/null && brew install golangci-lint) || \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Done!"

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
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Run linter with auto-fix
lint-fix:
	@echo "Running linter with auto-fix..."
	@golangci-lint run --fix ./...

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
