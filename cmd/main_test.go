package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseArgs tests the argument parsing logic
func TestParseArgs(t *testing.T) {
	// Save and restore working directory
	origWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origWd)

	t.Run("simple command without directory flag", func(t *testing.T) {
		args := []string{"/bin/bash"}
		result, err := parseArgs(args)

		require.NoError(t, err)
		assert.NotEmpty(t, result.jailDir, "jailDir should default to current directory")
		assert.Equal(t, "/bin/bash", result.cmdName)
		assert.Empty(t, result.cmdArgs)
	})

	t.Run("command with arguments", func(t *testing.T) {
		args := []string{"/bin/sh", "script.sh", "--verbose"}
		result, err := parseArgs(args)

		require.NoError(t, err)
		assert.Equal(t, "/bin/sh", result.cmdName)
		assert.Equal(t, []string{"script.sh", "--verbose"}, result.cmdArgs)
	})

	t.Run("with -d flag", func(t *testing.T) {
		args := []string{"-d", "/tmp/test", "/bin/bash"}
		result, err := parseArgs(args)

		require.NoError(t, err)
		assert.Equal(t, "/tmp/test", result.jailDir)
		assert.Equal(t, "/bin/bash", result.cmdName)
		assert.Empty(t, result.cmdArgs)
	})

	t.Run("with --dir flag", func(t *testing.T) {
		args := []string{"--dir", "/home/user/project", "ls", "-la"}
		result, err := parseArgs(args)

		require.NoError(t, err)
		assert.Equal(t, "/home/user/project", result.jailDir)
		assert.Equal(t, "ls", result.cmdName)
		assert.Equal(t, []string{"-la"}, result.cmdArgs)
	})

	t.Run("with -d flag and command arguments", func(t *testing.T) {
		args := []string{"-d", "/var/app", "cat", "file.txt", "--number"}
		result, err := parseArgs(args)

		require.NoError(t, err)
		assert.Equal(t, "/var/app", result.jailDir)
		assert.Equal(t, "cat", result.cmdName)
		assert.Equal(t, []string{"file.txt", "--number"}, result.cmdArgs)
	})

	t.Run("error when no command specified", func(t *testing.T) {
		args := []string{}
		result, err := parseArgs(args)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no command specified")
	})

	t.Run("error when only directory flag provided", func(t *testing.T) {
		args := []string{"-d", "/tmp/test"}
		result, err := parseArgs(args)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no command specified")
	})
}

// TestReadJailConfig tests the .jail file parsing
func TestReadJailConfig(t *testing.T) {
	t.Run("parse valid config file", func(t *testing.T) {
		// Create a temporary config file
		tmpFile, err := os.CreateTemp("", ".jail-test-*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := `# This is a comment
/home/user/.local/share/mise
/home/user/.config/mise

# Another comment
/opt/custom-tools

/usr/local/custom-lib
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		dirs, err := readJailConfig(tmpFile.Name())

		require.NoError(t, err)
		assert.Len(t, dirs, 4)
		assert.Equal(t, "/home/user/.local/share/mise", dirs[0])
		assert.Equal(t, "/home/user/.config/mise", dirs[1])
		assert.Equal(t, "/opt/custom-tools", dirs[2])
		assert.Equal(t, "/usr/local/custom-lib", dirs[3])
	})

	t.Run("empty config file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", ".jail-test-*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		dirs, err := readJailConfig(tmpFile.Name())

		require.NoError(t, err)
		assert.Empty(t, dirs)
	})

	t.Run("config file with only comments and whitespace", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", ".jail-test-*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := `# Comment 1
# Comment 2



# Comment 3
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		dirs, err := readJailConfig(tmpFile.Name())

		require.NoError(t, err)
		assert.Empty(t, dirs)
	})

	t.Run("config file does not exist", func(t *testing.T) {
		dirs, err := readJailConfig("/nonexistent/path/.jail")

		assert.Error(t, err)
		assert.Nil(t, dirs)
	})

	t.Run("config with mixed content", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", ".jail-test-*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := `/first/path
# Comment
/second/path
   /third/path/with/leading/whitespace
/fourth/path
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		dirs, err := readJailConfig(tmpFile.Name())

		require.NoError(t, err)
		assert.Len(t, dirs, 4)
		assert.Equal(t, "/first/path", dirs[0])
		assert.Equal(t, "/second/path", dirs[1])
		assert.Equal(t, "/third/path/with/leading/whitespace", dirs[2])
		assert.Equal(t, "/fourth/path", dirs[3])
	})
}

// TestSetOrUpdateEnv tests environment variable manipulation
func TestSetOrUpdateEnv(t *testing.T) {
	t.Run("add new environment variable", func(t *testing.T) {
		env := []string{"PATH=/usr/bin", "HOME=/home/user"}
		result := setOrUpdateEnv(env, "NEWVAR", "newvalue")

		assert.Len(t, result, 3)
		assert.Contains(t, result, "NEWVAR=newvalue")
		assert.Contains(t, result, "PATH=/usr/bin")
		assert.Contains(t, result, "HOME=/home/user")
	})

	t.Run("update existing environment variable", func(t *testing.T) {
		env := []string{"PATH=/usr/bin", "HOME=/home/user", "LANG=en_US.UTF-8"}
		result := setOrUpdateEnv(env, "HOME", "/root")

		assert.Len(t, result, 3)
		assert.Contains(t, result, "HOME=/root")
		assert.Contains(t, result, "PATH=/usr/bin")
		assert.Contains(t, result, "LANG=en_US.UTF-8")
		assert.NotContains(t, result, "HOME=/home/user")
	})

	t.Run("update first variable", func(t *testing.T) {
		env := []string{"PATH=/usr/bin", "HOME=/home/user"}
		result := setOrUpdateEnv(env, "PATH", "/bin:/usr/bin")

		assert.Len(t, result, 2)
		assert.Equal(t, "PATH=/bin:/usr/bin", result[0])
		assert.Equal(t, "HOME=/home/user", result[1])
	})

	t.Run("add to empty environment", func(t *testing.T) {
		env := []string{}
		result := setOrUpdateEnv(env, "VAR", "value")

		assert.Len(t, result, 1)
		assert.Equal(t, "VAR=value", result[0])
	})

	t.Run("handle variable with equals in value", func(t *testing.T) {
		env := []string{"PATH=/usr/bin"}
		result := setOrUpdateEnv(env, "COMPLEX", "key=value")

		assert.Len(t, result, 2)
		assert.Contains(t, result, "COMPLEX=key=value")
	})

	t.Run("handle empty value", func(t *testing.T) {
		env := []string{"PATH=/usr/bin"}
		result := setOrUpdateEnv(env, "EMPTY", "")

		assert.Len(t, result, 2)
		assert.Contains(t, result, "EMPTY=")
	})
}

// TestResolveCommand tests command path resolution
func TestResolveCommand(t *testing.T) {
	t.Run("absolute path command", func(t *testing.T) {
		result, err := resolveCommand("/bin/bash", []string{})

		require.NoError(t, err)
		assert.Equal(t, "/bin/bash", result)
	})

	t.Run("relative path with slash", func(t *testing.T) {
		result, err := resolveCommand("./mycommand", []string{})

		require.NoError(t, err)
		assert.Equal(t, "./mycommand", result)
	})

	t.Run("command in standard path", func(t *testing.T) {
		// Create a temporary directory structure to simulate /bin
		tmpDir, err := os.MkdirTemp("", "jail-test-bin-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create a fake executable
		testCmd := filepath.Join(tmpDir, "testcommand")
		err = os.WriteFile(testCmd, []byte("#!/bin/sh\necho test"), 0755)
		require.NoError(t, err)

		// Test resolving the command
		result, err := resolveCommand("testcommand", []string{tmpDir})

		require.NoError(t, err)
		assert.Equal(t, testCmd, result)
	})

	t.Run("command in custom directory bin subdirectory", func(t *testing.T) {
		// Create a temporary directory structure
		tmpDir, err := os.MkdirTemp("", "jail-test-custom-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		binDir := filepath.Join(tmpDir, "bin")
		err = os.MkdirAll(binDir, 0755)
		require.NoError(t, err)

		// Create a fake executable
		testCmd := filepath.Join(binDir, "customcmd")
		err = os.WriteFile(testCmd, []byte("#!/bin/sh\necho custom"), 0755)
		require.NoError(t, err)

		// Test resolving the command
		result, err := resolveCommand("customcmd", []string{tmpDir})

		require.NoError(t, err)
		assert.Equal(t, testCmd, result)
	})

	t.Run("command in shims subdirectory", func(t *testing.T) {
		// Create a temporary directory structure
		tmpDir, err := os.MkdirTemp("", "jail-test-shims-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		shimsDir := filepath.Join(tmpDir, "shims")
		err = os.MkdirAll(shimsDir, 0755)
		require.NoError(t, err)

		// Create a fake executable
		testCmd := filepath.Join(shimsDir, "shimcmd")
		err = os.WriteFile(testCmd, []byte("#!/bin/sh\necho shim"), 0755)
		require.NoError(t, err)

		// Test resolving the command
		result, err := resolveCommand("shimcmd", []string{tmpDir})

		require.NoError(t, err)
		assert.Equal(t, testCmd, result)
	})

	t.Run("command not found", func(t *testing.T) {
		result, err := resolveCommand("nonexistentcommand12345", []string{})

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "command not found")
	})

	t.Run("non-executable file is skipped", func(t *testing.T) {
		// Create a temporary directory
		tmpDir, err := os.MkdirTemp("", "jail-test-nonexec-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create a non-executable file
		testFile := filepath.Join(tmpDir, "notexecutable")
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)

		// Test resolving the command (should fail)
		result, err := resolveCommand("notexecutable", []string{tmpDir})

		assert.Error(t, err)
		assert.Empty(t, result)
	})

	t.Run("directory is skipped", func(t *testing.T) {
		// Create a temporary directory
		tmpDir, err := os.MkdirTemp("", "jail-test-dir-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create a subdirectory with a unique name that won't conflict with system commands
		uniqueCmdName := "uniquedirname12345xyz"
		dirName := filepath.Join(tmpDir, uniqueCmdName)
		err = os.MkdirAll(dirName, 0755)
		require.NoError(t, err)

		// Test resolving (should fail because it's a directory)
		result, err := resolveCommand(uniqueCmdName, []string{tmpDir})

		assert.Error(t, err)
		assert.Empty(t, result)
	})
}

// TestJailArgsStruct tests the jailArgs struct
func TestJailArgsStruct(t *testing.T) {
	t.Run("struct initialization", func(t *testing.T) {
		args := &jailArgs{
			jailDir: "/home/test",
			cmdName: "bash",
			cmdArgs: []string{"-c", "echo hello"},
		}

		assert.Equal(t, "/home/test", args.jailDir)
		assert.Equal(t, "bash", args.cmdName)
		assert.Equal(t, []string{"-c", "echo hello"}, args.cmdArgs)
	})

	t.Run("zero value struct", func(t *testing.T) {
		args := &jailArgs{}

		assert.Empty(t, args.jailDir)
		assert.Empty(t, args.cmdName)
		assert.Nil(t, args.cmdArgs)
	})
}
