package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const setupFlag = "__JAIL_SETUP__"

// jailArgs represents parsed command-line arguments
type jailArgs struct {
	jailDir string
	cmdName string
	cmdArgs []string
}

// parseArgs parses command-line arguments and returns the jail configuration
func parseArgs(args []string) (*jailArgs, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("no command specified")
	}

	result := &jailArgs{}
	remainingArgs := args

	// Check for -d or --dir flag
	if len(remainingArgs) >= 2 && (remainingArgs[0] == "-d" || remainingArgs[0] == "--dir") {
		result.jailDir = remainingArgs[1]
		remainingArgs = remainingArgs[2:]
	} else {
		// Default to current directory
		var err error
		result.jailDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current directory: %w", err)
		}
	}

	if len(remainingArgs) == 0 {
		return nil, fmt.Errorf("no command specified")
	}

	result.cmdName = remainingArgs[0]
	result.cmdArgs = remainingArgs[1:]

	return result, nil
}

// readJailConfig reads a .jail file and returns additional directories to bind mount
func readJailConfig(configPath string) ([]string, error) {
	file, err := os.Open(configPath) //nolint:gosec // Config file path comes from workspace directory
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	var dirs []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dirs = append(dirs, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return dirs, nil
}

func main() {
	// Stage 2: We're inside the namespace, set up bind mounts and exec
	if os.Getenv(setupFlag) == "1" {
		if err := setupJailAndExec(); err != nil {
			fmt.Fprintf(os.Stderr, "Setup error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Stage 1: Parse arguments and validate
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [-d <directory>] <command> [args...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s /bin/sh                  # jail in current directory\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -d /tmp/mydir /bin/sh    # jail in /tmp/mydir\n", os.Args[0])
		os.Exit(1)
	}

	parsedArgs, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	jailDir := parsedArgs.jailDir

	// Verify jail directory exists
	if info, err := os.Stat(jailDir); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a valid directory\n", jailDir)
		os.Exit(1)
	}

	// Re-exec ourselves with namespaces enabled
	// Pass all original arguments
	cmd := exec.Command("/proc/self/exe", os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), setupFlag+"=1")

	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Create new namespaces
		Cloneflags: syscall.CLONE_NEWNS | // Mount namespace - isolate filesystem
			syscall.CLONE_NEWUSER | // User namespace - run unprivileged
			syscall.CLONE_NEWPID | // PID namespace - process isolation
			syscall.CLONE_NEWUTS | // UTS namespace - hostname isolation
			syscall.CLONE_NEWIPC, // IPC namespace

		// Map current user to "root" inside namespace (but not real root!)
		UidMappings: []syscall.SysProcIDMap{{
			ContainerID: 0,
			HostID:      os.Getuid(),
			Size:        1,
		}},
		GidMappings: []syscall.SysProcIDMap{{
			ContainerID: 0,
			HostID:      os.Getgid(),
			Size:        1,
		}},

		// Prevent gaining privileges
		AmbientCaps: []uintptr{},
	}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// getDockerSocketPath detects the Docker socket location from environment or common paths
func getDockerSocketPath() string {
	// Check DOCKER_HOST environment variable
	if dockerHost := os.Getenv("DOCKER_HOST"); dockerHost != "" {
		// DOCKER_HOST can be unix:///path/to/socket or just /path/to/socket
		if strings.HasPrefix(dockerHost, "unix://") {
			return strings.TrimPrefix(dockerHost, "unix://")
		}
		// If it's a file path (starts with /), use it directly
		if strings.HasPrefix(dockerHost, "/") {
			return dockerHost
		}
	}

	// Check common Docker socket locations
	commonPaths := []string{
		"/var/run/docker.sock", // Standard Linux location
		"/run/docker.sock",     // Alternative Linux location
		filepath.Join(os.Getenv("HOME"), ".docker", "run", "docker.sock"), // Rootless Docker
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// mountDockerSocket mounts the Docker socket into the jail for Docker support
func mountDockerSocket(tmpRoot string) error {
	dockerSocketPath := getDockerSocketPath()
	if dockerSocketPath == "" {
		return fmt.Errorf("docker socket not found")
	}

	// Verify the socket exists and is accessible
	info, err := os.Stat(dockerSocketPath)
	if err != nil {
		return fmt.Errorf("docker socket at %s not accessible: %w", dockerSocketPath, err)
	}

	// Verify it's a socket
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("docker socket at %s is not a socket", dockerSocketPath)
	}

	// Create the parent directory structure in the jail
	jailSocketPath := filepath.Join(tmpRoot, strings.TrimPrefix(dockerSocketPath, "/"))
	jailSocketDir := filepath.Dir(jailSocketPath)

	if err := os.MkdirAll(jailSocketDir, 0755); err != nil { //nolint:gosec,mnd // 0755 is appropriate for directory permissions
		return fmt.Errorf("creating docker socket directory %s: %w", jailSocketDir, err)
	}

	// Create an empty file to bind mount over (sockets can't be created directly)
	// We use a regular file as the mount point
	if err := os.WriteFile(jailSocketPath, []byte{}, 0666); err != nil { //nolint:gosec,mnd // Will inherit actual socket permissions
		return fmt.Errorf("creating docker socket mount point: %w", err)
	}

	// Bind mount the socket
	if err := syscall.Mount(dockerSocketPath, jailSocketPath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind mounting docker socket: %w", err)
	}

	return nil
}

//nolint:gocognit,gocyclo // Complex namespace setup is inherently complex
func setupJailAndExec() error {
	// Parse arguments same as main()
	parsedArgs, err := parseArgs(os.Args[1:])
	if err != nil {
		return fmt.Errorf("parsing arguments: %w", err)
	}

	jailDir := parsedArgs.jailDir
	cmdName := parsedArgs.cmdName
	cmdArgs := parsedArgs.cmdArgs

	// Make jail directory absolute
	jailDir, err = filepath.Abs(jailDir)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}

	// Critical: Make all mounts private to prevent propagation issues
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("making root mount private: %w", err)
	}

	// Create a temporary root directory structure
	tmpRoot, err := os.MkdirTemp("", "jail-root-")
	if err != nil {
		return fmt.Errorf("creating temp root: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpRoot); rmErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up temp root %s: %v\n", tmpRoot, rmErr)
		}
	}()

	// Directories to bind mount from host (read-only access to tools)
	bindDirs := []string{
		"/bin",
		"/usr",
		"/lib",
		"/lib64",
		"/sbin",
		"/etc", // Needed for DNS resolution and network configs
	}

	// Read global .jail config from $HOME/.jail if it exists
	if hostHome := os.Getenv("HOME"); hostHome != "" {
		globalConfigFile := filepath.Join(hostHome, ".jail")
		if extraDirs, err := readJailConfig(globalConfigFile); err == nil {
			bindDirs = append(bindDirs, extraDirs...)
		}
	}

	// Read additional directories from workspace .jail file if it exists
	// This allows local config to add to or override global config
	jailConfigFile := filepath.Join(jailDir, ".jail")
	if extraDirs, err := readJailConfig(jailConfigFile); err == nil {
		bindDirs = append(bindDirs, extraDirs...)
	}

	// Create mount points in temp root and bind mount system directories
	for _, dir := range bindDirs {
		// Check if source exists on host
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // Skip if doesn't exist on this system
		}

		targetDir := filepath.Join(tmpRoot, dir)
		if err := os.MkdirAll(targetDir, 0755); err != nil { //nolint:gosec,mnd // 0755 is appropriate for directory permissions
			return fmt.Errorf("creating mount point %s: %w", targetDir, err)
		}

		// Bind mount (read-only)
		if err := syscall.Mount(dir, targetDir, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
			return fmt.Errorf("bind mounting %s: %w", dir, err)
		}

		// Make it read-only
		if err := syscall.Mount("", targetDir, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, ""); err != nil {
			return fmt.Errorf("remounting %s as read-only: %w", dir, err)
		}
	}

	// Create workspace mount point at /workspace/{basename}
	// This preserves project identity while providing a clean path structure
	workspaceDir := filepath.Join(tmpRoot, "workspace", filepath.Base(jailDir))
	if err := os.MkdirAll(workspaceDir, 0755); err != nil { //nolint:gosec,mnd // 0755 is appropriate for directory permissions
		return fmt.Errorf("creating workspace: %w", err)
	}

	if err := syscall.Mount(jailDir, workspaceDir, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting workspace: %w", err)
	}

	// Mount ~/.claude directory and ~/.claude.json file from host to preserve login state
	// Note: HOME is still /home/$USER inside jail, not /root
	hostHome := os.Getenv("HOME")
	if hostHome != "" {
		// Mount ~/.claude directory
		hostClaudeDir := filepath.Join(hostHome, ".claude")
		if _, err := os.Stat(hostClaudeDir); err == nil {
			jailClaudeDir := filepath.Join(tmpRoot, strings.TrimPrefix(hostHome, "/"), ".claude")
			if err := os.MkdirAll(jailClaudeDir, 0755); err != nil { //nolint:gosec,mnd // 0755 is appropriate for directory permissions
				return fmt.Errorf("creating %s/.claude: %w", hostHome, err)
			}

			// Bind mount (read-write for login persistence)
			if err := syscall.Mount(hostClaudeDir, jailClaudeDir, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
				return fmt.Errorf("bind mounting .claude: %w", err)
			}
		}

		// Mount ~/.claude.json file
		hostClaudeJSON := filepath.Join(hostHome, ".claude.json")
		if _, err := os.Stat(hostClaudeJSON); err == nil {
			jailClaudeJSON := filepath.Join(tmpRoot, strings.TrimPrefix(hostHome, "/"), ".claude.json")
			// Create parent directory if it doesn't exist
			if err := os.MkdirAll(filepath.Dir(jailClaudeJSON), 0755); err != nil { //nolint:gosec,mnd // 0755 is appropriate for directory permissions
				return fmt.Errorf("creating parent dir for .claude.json: %w", err)
			}
			// Create empty file to mount over
			if err := os.WriteFile(jailClaudeJSON, []byte{}, 0600); err != nil {
				return fmt.Errorf("creating .claude.json mount point: %w", err)
			}

			// Bind mount the file
			if err := syscall.Mount(hostClaudeJSON, jailClaudeJSON, "", syscall.MS_BIND, ""); err != nil {
				return fmt.Errorf("bind mounting .claude.json: %w", err)
			}
		}
	}

	// Mount XDG_RUNTIME_DIR for runtime data (needed by some tools like Claude)
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir != "" {
		if _, err := os.Stat(xdgRuntimeDir); err == nil {
			jailRuntimeDir := filepath.Join(tmpRoot, strings.TrimPrefix(xdgRuntimeDir, "/"))
			if err := os.MkdirAll(jailRuntimeDir, 0700); err != nil { //nolint:gosec,mnd // 0700 is appropriate for runtime directory permissions
				return fmt.Errorf("creating %s: %w", xdgRuntimeDir, err)
			}

			// Bind mount (read-write for runtime data)
			if err := syscall.Mount(xdgRuntimeDir, jailRuntimeDir, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
				return fmt.Errorf("bind mounting XDG_RUNTIME_DIR: %w", err)
			}
		}
	}

	// Mount Docker socket for Docker support
	if err := mountDockerSocket(tmpRoot); err != nil {
		// Non-fatal: Docker might not be installed or running
		// Don't return error, just log warning to stderr
		fmt.Fprintf(os.Stderr, "Warning: Docker socket not mounted: %v\n", err)
	}

	// Create essential directories
	essentialDirs := []string{"/proc", "/dev", "/tmp"}
	for _, dir := range essentialDirs {
		targetDir := filepath.Join(tmpRoot, dir)
		if err := os.MkdirAll(targetDir, 0755); err != nil { //nolint:gosec,mnd // 0755 is appropriate for directory permissions
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	// Mount /proc (needed for many commands)
	procDir := filepath.Join(tmpRoot, "proc")
	if err := syscall.Mount("proc", procDir, "proc", 0, ""); err != nil {
		return fmt.Errorf("mounting proc: %w", err)
	}

	// Bind mount /dev from host
	devDir := filepath.Join(tmpRoot, "dev")
	if err := syscall.Mount("/dev", devDir, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting /dev: %w", err)
	}

	// Chroot into temp root
	if err := syscall.Chroot(tmpRoot); err != nil {
		return fmt.Errorf("chroot: %w", err)
	}

	// Change to /workspace/{basename} directory
	workspacePath := filepath.Join("/workspace", filepath.Base(jailDir))
	if err := os.Chdir(workspacePath); err != nil {
		return fmt.Errorf("chdir to %s: %w", workspacePath, err)
	}

	// Resolve command path if it's not absolute
	resolvedCmd, err := resolveCommand(cmdName, bindDirs)
	if err != nil {
		return fmt.Errorf("finding command %s: %w", cmdName, err)
	}

	// Ensure HOME is set correctly so Claude can find its config at $HOME/.claude
	env := os.Environ()
	if hostHome != "" {
		env = setOrUpdateEnv(env, "HOME", hostHome)
	}

	// Execute the actual command
	if err := syscall.Exec(resolvedCmd, append([]string{cmdName}, cmdArgs...), env); err != nil {
		return fmt.Errorf("exec %s: %w", cmdName, err)
	}

	return nil
}

// setOrUpdateEnv updates an environment variable in the env slice, or adds it if not present
func setOrUpdateEnv(env []string, key, value string) []string {
	prefix := key + "="
	newEntry := prefix + value

	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = newEntry
			return env
		}
	}

	return append(env, newEntry)
}

// resolveCommand finds the full path to a command by searching standard directories
func resolveCommand(cmdName string, searchDirs []string) (string, error) {
	// If it's already an absolute path or contains a slash, use it as-is
	if filepath.IsAbs(cmdName) || strings.Contains(cmdName, "/") {
		return cmdName, nil
	}

	// Search in common executable directories
	pathDirs := []string{
		"/bin",
		"/usr/bin",
		"/sbin",
		"/usr/sbin",
		"/usr/local/bin",
	}

	// Add any custom directories from searchDirs that might contain executables
	for _, dir := range searchDirs {
		// Add the directory itself
		pathDirs = append(pathDirs, dir)
		// Also check common subdirectories
		pathDirs = append(pathDirs, filepath.Join(dir, "bin"))
		pathDirs = append(pathDirs, filepath.Join(dir, "shims"))
	}

	// Search for the executable
	for _, dir := range pathDirs {
		candidatePath := filepath.Join(dir, cmdName)
		if info, err := os.Stat(candidatePath); err == nil && !info.IsDir() {
			// Check if executable
			if info.Mode()&0111 != 0 {
				return candidatePath, nil
			}
		}
	}

	return "", fmt.Errorf("command not found in PATH")
}
