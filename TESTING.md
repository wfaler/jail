# Testing Guide

This document describes the testing approach for the Jail project.

## Overview

The project includes two types of tests:
- **Unit Tests**: Test individual functions in isolation (always run)
- **Integration Tests**: Test the complete jail functionality end-to-end (opt-in with build tag)

## Running Tests

### Unit Tests

Run all unit tests with:
```bash
go test -v ./cmd/...
```

Run with coverage:
```bash
go test -v -cover ./cmd/...
```

Generate coverage report:
```bash
go test -coverprofile=coverage.out ./cmd/...
go tool cover -html=coverage.out
```

### Integration Tests

Integration tests require the `-tags=integration` flag:

```bash
go test -v -tags=integration ./cmd/...
```

Run only integration tests:
```bash
go test -v -tags=integration -run Integration ./cmd/...
```

Skip integration tests in short mode:
```bash
go test -short -tags=integration ./cmd/...
```

### Running Benchmarks

```bash
go test -bench=. -tags=integration ./cmd/...
```

## Test Structure

### Unit Tests (`cmd/main_test.go`)

Tests for pure functions that don't require namespace/syscall operations:

1. **`TestParseArgs`** - Tests command-line argument parsing
   - Simple commands
   - Commands with arguments
   - Directory flags (`-d`, `--dir`)
   - Error handling

2. **`TestReadJailConfig`** - Tests `.jail` configuration file parsing
   - Valid config files
   - Comments and whitespace handling
   - Empty files
   - Non-existent files

3. **`TestSetOrUpdateEnv`** - Tests environment variable manipulation
   - Adding new variables
   - Updating existing variables
   - Edge cases (empty values, special characters)

4. **`TestResolveCommand`** - Tests command path resolution
   - Absolute paths
   - Relative paths
   - Standard PATH directories
   - Custom directories (bin/, shims/)
   - Error cases

5. **`TestJailArgsStruct`** - Tests the `jailArgs` data structure

### Integration Tests (`cmd/integration_test.go`)

Tests that require building and running the jail binary:

1. **`TestIntegrationBasicExecution`** - Basic command execution
   - Simple echo command
   - Commands with arguments

2. **`TestIntegrationWorkspaceIsolation`** - Filesystem isolation
   - Reading files from workspace
   - Writing files to workspace

3. **`TestIntegrationDirectoryFlag`** - Directory flag functionality
   - `-d` flag
   - `--dir` flag

4. **`TestIntegrationJailConfig`** - `.jail` configuration
   - Custom directory mounting

5. **`TestIntegrationErrorHandling`** - Error conditions
   - Invalid directories
   - Command not found
   - Missing arguments

6. **`TestIntegrationSystemDirectoriesReadOnly`** - Security
   - Verify system directories are read-only

7. **`TestIntegrationNetworkAccess`** - Network functionality
   - DNS resolution
   - Network stack access

8. **`BenchmarkJailExecution`** - Performance benchmarking

## Test Dependencies

The project uses [Testify](https://github.com/stretchr/testify) for assertions:

```bash
go get github.com/stretchr/testify
```

Testify provides:
- `require.*` - Assertions that stop the test on failure
- `assert.*` - Assertions that continue the test on failure

## Refactoring for Testability

The code has been refactored to separate pure functions from syscall/exec logic:

### Extracted Functions

1. **`parseArgs(args []string) (*jailArgs, error)`**
   - Pure function for parsing command-line arguments
   - Returns structured data (`jailArgs`) instead of multiple variables
   - No side effects, easy to test

2. **`readJailConfig(configPath string) ([]string, error)`**
   - Reads and parses `.jail` configuration files
   - Returns slice of directory paths
   - Testable with temporary files

3. **`setOrUpdateEnv(env []string, key, value string) []string`**
   - Pure function for environment variable manipulation
   - No global state modification
   - Completely deterministic

4. **`resolveCommand(cmdName string, searchDirs []string) (string, error)`**
   - Command path resolution logic
   - Testable with temporary directory structures

### Design Pattern

The refactoring follows this pattern:
```
┌─────────────────┐
│  main()         │  ← Not directly testable (syscalls, exec)
│  setupJailExec()│
└────────┬────────┘
         │ calls
         ▼
┌─────────────────┐
│ parseArgs()     │  ← Pure functions, easily testable
│ readJailConfig()│
│ setOrUpdateEnv()│
│ resolveCommand()│
└─────────────────┘
```

## Writing New Tests

### Unit Test Template

```go
func TestNewFunction(t *testing.T) {
    t.Run("descriptive test case name", func(t *testing.T) {
        // Arrange
        input := "test input"

        // Act
        result, err := newFunction(input)

        // Assert
        require.NoError(t, err)
        assert.Equal(t, "expected", result)
    })
}
```

### Integration Test Template

```go
func TestIntegrationNewFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Build the jail binary
    buildCmd := exec.Command("go", "build", "-o", "jail-test", ".")
    err := buildCmd.Run()
    require.NoError(t, err)
    defer os.Remove("jail-test")

    t.Run("test case", func(t *testing.T) {
        cmd := exec.Command("./jail-test", "echo", "test")
        output, err := cmd.CombinedOutput()

        require.NoError(t, err)
        assert.Equal(t, "test\n", string(output))
    })
}
```

## Continuous Integration

For CI/CD pipelines:

```yaml
# Run unit tests only
- go test -v ./cmd/...

# Run all tests including integration
- go test -v -tags=integration ./cmd/...

# Generate coverage
- go test -coverprofile=coverage.out ./cmd/...
```

## Test Coverage

Current coverage includes:
- ✅ Argument parsing (all branches)
- ✅ Configuration file parsing (all branches)
- ✅ Environment variable manipulation
- ✅ Command resolution
- ✅ Integration tests for main functionality
- ❌ Namespace setup (requires root/complex mocking)
- ❌ Mount operations (requires root/complex mocking)

Functions like `setupJailAndExec()` are tested via integration tests rather than unit tests because they require Linux namespaces and syscalls.

## Troubleshooting

### Integration Tests Fail

**Issue**: Integration tests fail with "permission denied" or namespace errors

**Solution**: Ensure your system supports user namespaces:
```bash
# Check if user namespaces are enabled
cat /proc/sys/kernel/unprivileged_userns_clone
# Should output: 1

# Enable if needed (requires root)
echo 1 | sudo tee /proc/sys/kernel/unprivileged_userns_clone
```

### Build Tag Not Recognized

**Issue**: Integration tests run with normal `go test`

**Solution**: Use `-tags=integration` flag:
```bash
go test -tags=integration ./cmd/...
```

### Import Cycle Errors

**Issue**: Cannot import test utilities

**Solution**: Keep test files in the same package (`package main`), use `_test.go` suffix

## Best Practices

1. **Use table-driven tests** for multiple similar test cases
2. **Use subtests** (`t.Run()`) to organize related tests
3. **Clean up resources** with `defer` statements
4. **Use `require` for critical assertions** (test should stop if they fail)
5. **Use `assert` for informational assertions** (test continues)
6. **Test error cases** as thoroughly as success cases
7. **Use descriptive test names** that explain what is being tested
8. **Avoid machine-specific tests** (don't rely on node, python, etc. being installed)
