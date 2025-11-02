# Makefile Guide

This document provides a quick reference for all available Makefile targets in the jail project.

## Quick Start

```bash
# Default: Build, test, and lint
make

# or explicitly
make all

# Show all available targets
make help
```

## Build Targets

### `make build`
Builds the jail binary.

```bash
make build
# Output: ./jail
```

### `make clean`
Removes built binaries, test artifacts, and clears test cache.

```bash
make clean
```

### `make install`
Builds and installs the binary to `$GOPATH/bin`.

```bash
make install
# Installs to: $(go env GOPATH)/bin/jail
```

## Testing Targets

### `make test` (alias: `make test-unit`)
Runs unit tests with race detection.

```bash
make test
# or
make test-unit
```

### `make test-integration`
Runs integration tests that require building and executing the jail binary.

```bash
make test-integration
```

**Note**: Integration tests require Linux namespace support.

### `make test-all`
Runs both unit and integration tests.

```bash
make test-all
```

### `make test-short`
Runs tests in short mode, skipping long-running tests.

```bash
make test-short
```

### `make coverage`
Generates test coverage report (HTML and terminal output).

```bash
make coverage
# Generates: coverage.out, coverage.html
```

### `make coverage-integration`
Generates coverage report including integration tests.

```bash
make coverage-integration
```

### `make bench`
Runs benchmark tests.

```bash
make bench
```

## Code Quality Targets

### `make lint`
Runs golangci-lint to check code quality.

```bash
make lint
```

**Prerequisites**: Requires golangci-lint to be installed. Run `make install-tools` if not installed.

### `make lint-fix`
Runs golangci-lint with auto-fix enabled.

```bash
make lint-fix
```

### `make fmt`
Formats code using `gofmt -s`.

```bash
make fmt
```

### `make vet`
Runs `go vet` to detect suspicious constructs.

```bash
make vet
```

## Development Tools

### `make install-tools`
Installs development tools (golangci-lint, etc.).

```bash
make install-tools
```

### `make dev-setup`
Sets up the complete development environment.

```bash
make dev-setup
```

Equivalent to:
- `make install-tools`
- `go mod download`
- `go mod tidy`

## CI/CD Targets

### `make ci`
Runs the standard CI pipeline: format, vet, lint, and unit tests.

```bash
make ci
```

Equivalent to:
- `make fmt`
- `make vet`
- `make lint`
- `make test-unit`

### `make ci-full`
Runs the full CI pipeline including integration tests and coverage.

```bash
make ci-full
```

Equivalent to:
- `make fmt`
- `make vet`
- `make lint`
- `make test-all`
- `make coverage`

### `make verify`
Verifies code is ready to commit: build, test, and lint.

```bash
make verify
```

Equivalent to:
- `make build`
- `make test`
- `make lint`

## Utility Targets

### `make run-example`
Builds and runs an example jail command.

```bash
make run-example
# Runs: ./jail echo 'Hello from jail'
```

### `make all` (default)
Runs the complete build, test, and lint workflow.

```bash
make all
# or just
make
```

Equivalent to:
- `make clean`
- `make build`
- `make test`
- `make lint`

## Common Workflows

### Daily Development
```bash
# Before starting work
make dev-setup

# During development (run frequently)
make verify

# Before committing
make verify
```

### Pre-Commit Checks
```bash
make verify
```

### CI/CD Pipeline
```bash
# Fast CI (for every commit)
make ci

# Comprehensive CI (for PRs/releases)
make ci-full
```

### Code Quality Review
```bash
make fmt      # Format code
make vet      # Check for issues
make lint     # Comprehensive linting
```

### Testing Workflow
```bash
# Quick feedback loop
make test-unit

# Full testing before merge
make test-all

# Check coverage
make coverage
```

## Tips

1. **Parallel Execution**: Use `-j` flag for parallel execution (where safe):
   ```bash
   make -j4 fmt vet lint
   ```

2. **Silent Mode**: Use `-s` flag to suppress recipe echoing:
   ```bash
   make -s build
   ```

3. **Dry Run**: Use `-n` flag to see what would be executed:
   ```bash
   make -n verify
   ```

4. **Check Dependencies**: See which targets would be rebuilt:
   ```bash
   make -p | grep '^[a-zA-Z]'
   ```

## Troubleshooting

### "golangci-lint not found"
```bash
make install-tools
```

### Integration tests fail
Ensure you're on Linux with namespace support:
```bash
# Check user namespace support
cat /proc/sys/kernel/unprivileged_userns_clone
# Should output: 1
```

### Tests use cached results
```bash
make clean  # Clears test cache
```

## Environment Variables

The Makefile respects these environment variables:

- `GOPATH`: Go workspace path
- `GOBIN`: Go binary installation path

## Configuration Files

- `.golangci.yml`: golangci-lint configuration
- `go.mod`: Go module dependencies

## Related Documentation

- [TESTING.md](TESTING.md) - Detailed testing guide
- [README.md](README.md) - Project overview
- [CLAUDE.md](CLAUDE.md) - Development guidelines
