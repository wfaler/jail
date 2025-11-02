.PHONY: all help build clean test test-unit test-integration test-all coverage lint lint-fix install install-tools bench fmt vet

# Variables
BINARY_NAME=jail
CMD_DIR=./cmd
BUILD_DIR=.
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Default target
all: clean build test lint ## Build, test, and lint (default when running 'make')

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

build: ## Build the jail binary
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) $(CMD_DIR)/main.go
	@echo "✓ Build complete: $(BINARY_NAME)"

clean: ## Remove built binaries and test artifacts
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -f jail-test jail-bench
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	go clean -testcache
	@echo "✓ Clean complete"

install: build ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	go install $(CMD_DIR)/main.go
	@echo "✓ Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

##@ Testing

test: test-unit ## Run unit tests (default)

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	go test -v -race $(CMD_DIR)/...
	@echo "✓ Unit tests passed"

test-integration: build ## Run integration tests (requires Linux namespaces)
	@echo "Running integration tests..."
	go test -v -race -tags=integration $(CMD_DIR)/...
	@echo "✓ Integration tests passed"

test-all: ## Run all tests (unit + integration)
	@echo "Running all tests..."
	go test -v -race -tags=integration $(CMD_DIR)/...
	@echo "✓ All tests passed"

test-short: ## Run tests in short mode (skip long-running tests)
	@echo "Running tests in short mode..."
	go test -v -short -race $(CMD_DIR)/...
	@echo "✓ Short tests passed"

coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	go test -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(CMD_DIR)/...
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	go tool cover -func=$(COVERAGE_FILE)
	@echo "✓ Coverage report generated: $(COVERAGE_HTML)"

coverage-integration: ## Generate coverage report including integration tests
	@echo "Generating coverage report with integration tests..."
	go test -coverprofile=$(COVERAGE_FILE) -covermode=atomic -tags=integration $(CMD_DIR)/...
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	go tool cover -func=$(COVERAGE_FILE)
	@echo "✓ Coverage report generated: $(COVERAGE_HTML)"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem -tags=integration $(CMD_DIR)/...

##@ Code Quality

lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run $(CMD_DIR)/...; \
		echo "✓ Linting complete"; \
	else \
		echo "⚠ golangci-lint not installed. Run 'make install-tools' to install it."; \
		exit 1; \
	fi

lint-fix: ## Run golangci-lint and auto-fix issues
	@echo "Running golangci-lint with auto-fix..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --fix $(CMD_DIR)/...; \
		echo "✓ Linting and auto-fix complete"; \
	else \
		echo "⚠ golangci-lint not installed. Run 'make install-tools' to install it."; \
		exit 1; \
	fi

fmt: ## Format code with gofmt
	@echo "Formatting code..."
	gofmt -s -w $(CMD_DIR)
	@echo "✓ Code formatted"

vet: ## Run go vet
	@echo "Running go vet..."
	go vet $(CMD_DIR)/...
	@echo "✓ go vet passed"

##@ Tools

install-tools: ## Install development tools (golangci-lint, etc.)
	@echo "Installing development tools..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin; \
		echo "✓ golangci-lint installed"; \
	else \
		echo "✓ golangci-lint already installed"; \
	fi
	@echo "✓ All tools installed"

##@ CI/CD

ci: fmt vet lint test-unit ## Run CI pipeline (format, vet, lint, test)
	@echo "✓ CI pipeline complete"

ci-full: fmt vet lint test-all coverage ## Run full CI pipeline (format, vet, lint, all tests, coverage)
	@echo "✓ Full CI pipeline complete"

##@ Development

dev-setup: install-tools ## Setup development environment
	@echo "Setting up development environment..."
	go mod download
	go mod tidy
	@echo "✓ Development environment ready"

verify: build test lint ## Verify code is ready to commit (build, test, lint)
	@echo "✓ Code verification complete - ready to commit"

run-example: build ## Build and run an example command
	@echo "Running example: ./jail echo 'Hello from jail'"
	./$(BINARY_NAME) echo 'Hello from jail'
