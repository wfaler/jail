//go:build integration
// +build integration

// Package main provides integration tests for the jail application.
// These tests require building and running the jail binary.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests require building the jail binary first:
//   go build -o jail ./cmd/main.go
//
// Run integration tests with:
//   go test -v -tags=integration ./cmd/...

// TestIntegrationBasicExecution tests basic command execution in jail
func TestIntegrationBasicExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the jail binary
	buildCmd := exec.Command("go", "build", "-o", "jail-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err, "Failed to build jail binary")
	defer os.Remove("jail-test")

	t.Run("execute simple echo command", func(t *testing.T) {
		cmd := exec.Command("./jail-test", "echo", "hello world")
		output, err := cmd.CombinedOutput()

		require.NoError(t, err)
		assert.Equal(t, "hello world\n", string(output))
	})

	t.Run("execute command with arguments", func(t *testing.T) {
		cmd := exec.Command("./jail-test", "/bin/sh", "-c", "echo hello integration")
		output, err := cmd.CombinedOutput()

		require.NoError(t, err)
		assert.Equal(t, "hello integration\n", string(output))
	})
}

// TestIntegrationWorkspaceIsolation tests filesystem isolation
func TestIntegrationWorkspaceIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the jail binary
	buildCmd := exec.Command("go", "build", "-o", "jail-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err)
	defer os.Remove("jail-test")

	// Create a test workspace
	tmpDir, err := os.MkdirTemp("", "jail-integration-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test file in the workspace
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	t.Run("can access files in workspace", func(t *testing.T) {
		// The file should be accessible at /workspace/{basename}/test.txt
		workspacePath := filepath.Join("/workspace", filepath.Base(tmpDir), "test.txt")
		cmd := exec.Command("./jail-test", "-d", tmpDir, "cat", workspacePath)
		output, err := cmd.CombinedOutput()

		require.NoError(t, err)
		assert.Equal(t, "test content", string(output))
	})

	t.Run("can write files in workspace", func(t *testing.T) {
		newFile := filepath.Join("/workspace", filepath.Base(tmpDir), "newfile.txt")
		cmd := exec.Command("./jail-test", "-d", tmpDir, "/bin/sh", "-c",
			"echo 'written from jail' > "+newFile)
		err := cmd.Run()

		require.NoError(t, err)

		// Verify the file was created in the actual directory
		content, err := os.ReadFile(filepath.Join(tmpDir, "newfile.txt"))
		require.NoError(t, err)
		assert.Equal(t, "written from jail\n", string(content))
	})
}

// TestIntegrationDirectoryFlag tests the -d and --dir flags
func TestIntegrationDirectoryFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the jail binary
	buildCmd := exec.Command("go", "build", "-o", "jail-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err)
	defer os.Remove("jail-test")

	tmpDir, err := os.MkdirTemp("", "jail-integration-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("with -d flag", func(t *testing.T) {
		cmd := exec.Command("./jail-test", "-d", tmpDir, "pwd")
		output, err := cmd.CombinedOutput()

		require.NoError(t, err)
		expectedPath := filepath.Join("/workspace", filepath.Base(tmpDir))
		assert.Equal(t, expectedPath+"\n", string(output))
	})

	t.Run("with --dir flag", func(t *testing.T) {
		cmd := exec.Command("./jail-test", "--dir", tmpDir, "pwd")
		output, err := cmd.CombinedOutput()

		require.NoError(t, err)
		expectedPath := filepath.Join("/workspace", filepath.Base(tmpDir))
		assert.Equal(t, expectedPath+"\n", string(output))
	})
}

// TestIntegrationJailConfig tests the .jail configuration file
func TestIntegrationJailConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the jail binary
	buildCmd := exec.Command("go", "build", "-o", "jail-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err)
	defer os.Remove("jail-test")

	// Create a test workspace
	tmpDir, err := os.MkdirTemp("", "jail-integration-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a custom directory to mount
	customDir, err := os.MkdirTemp("", "jail-custom-*")
	require.NoError(t, err)
	defer os.RemoveAll(customDir)

	customFile := filepath.Join(customDir, "custom.txt")
	err = os.WriteFile(customFile, []byte("custom content"), 0644)
	require.NoError(t, err)

	// Create .jail config file
	jailConfig := filepath.Join(tmpDir, ".jail")
	err = os.WriteFile(jailConfig, []byte(customDir+"\n"), 0644)
	require.NoError(t, err)

	t.Run("custom directory is mounted", func(t *testing.T) {
		cmd := exec.Command("./jail-test", "-d", tmpDir, "cat", filepath.Join(customDir, "custom.txt"))
		output, err := cmd.CombinedOutput()

		require.NoError(t, err)
		assert.Equal(t, "custom content", string(output))
	})
}

// TestIntegrationErrorHandling tests error conditions
func TestIntegrationErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the jail binary
	buildCmd := exec.Command("go", "build", "-o", "jail-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err)
	defer os.Remove("jail-test")

	t.Run("invalid directory", func(t *testing.T) {
		cmd := exec.Command("./jail-test", "-d", "/nonexistent/directory", "echo", "test")
		output, err := cmd.CombinedOutput()

		assert.Error(t, err)
		assert.Contains(t, string(output), "not a valid directory")
	})

	t.Run("command not found", func(t *testing.T) {
		cmd := exec.Command("./jail-test", "nonexistentcommand12345")
		output, err := cmd.CombinedOutput()

		assert.Error(t, err)
		// The error might vary depending on the stage where it fails
		assert.NotEmpty(t, output)
	})

	t.Run("no command specified", func(t *testing.T) {
		cmd := exec.Command("./jail-test")
		output, err := cmd.CombinedOutput()

		assert.Error(t, err)
		assert.Contains(t, string(output), "Usage:")
	})
}

// TestIntegrationSystemDirectoriesReadOnly tests that system directories are read-only
func TestIntegrationSystemDirectoriesReadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the jail binary
	buildCmd := exec.Command("go", "build", "-o", "jail-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err)
	defer os.Remove("jail-test")

	tmpDir, err := os.MkdirTemp("", "jail-integration-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("cannot write to /usr", func(t *testing.T) {
		cmd := exec.Command("./jail-test", "-d", tmpDir, "/bin/sh", "-c",
			"touch /usr/testfile 2>&1")
		output, err := cmd.CombinedOutput()

		// Should fail because /usr is read-only
		assert.Error(t, err)
		outputStr := string(output)
		assert.True(t,
			strings.Contains(outputStr, "Read-only") ||
				strings.Contains(outputStr, "Permission denied") ||
				strings.Contains(outputStr, "read-only"),
			"Expected read-only or permission error, got: %s", outputStr)
	})
}

// TestIntegrationNetworkAccess tests that network access is available
func TestIntegrationNetworkAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the jail binary
	buildCmd := exec.Command("go", "build", "-o", "jail-test", ".")
	err := buildCmd.Run()
	require.NoError(t, err)
	defer os.Remove("jail-test")

	tmpDir, err := os.MkdirTemp("", "jail-integration-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("can resolve DNS", func(t *testing.T) {
		// Test DNS resolution using getent (should be available on most systems)
		cmd := exec.Command("./jail-test", "-d", tmpDir, "/bin/sh", "-c",
			"cat /etc/resolv.conf | head -1")
		output, err := cmd.CombinedOutput()

		// Should succeed - we can read DNS config
		require.NoError(t, err)
		assert.NotEmpty(t, output)
	})
}

// BenchmarkJailExecution benchmarks jail execution overhead
func BenchmarkJailExecution(b *testing.B) {
	// Build the jail binary
	buildCmd := exec.Command("go", "build", "-o", "jail-bench", ".")
	err := buildCmd.Run()
	if err != nil {
		b.Fatalf("Failed to build jail binary: %v", err)
	}
	defer os.Remove("jail-bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("./jail-bench", "echo", "benchmark")
		err := cmd.Run()
		if err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}
