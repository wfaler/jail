# CLAUDE.md
 
This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Jail is a lightweight Linux process isolation tool that provides filesystem jailing without requiring root privileges. It uses Linux namespaces (mount, user, PID, UTS, IPC) and bind mounts to create isolated execution environments.

## Key Architecture

### Two-Stage Execution Model

The application uses a two-stage execution pattern implemented in `cmd/main.go`:

1. **Stage 1** (main function, lines 41-126): Parses arguments, validates the jail directory, then re-executes itself with namespace flags via `/proc/self/exe`. Sets environment variable `__JAIL_SETUP__=1` to signal stage 2.

2. **Stage 2** (setupJailAndExec function, lines 129-326): Runs inside the namespaces. Sets up the isolated environment by:
   - Making all mounts private to prevent propagation
   - Creating a temporary root directory structure
   - Bind mounting system directories (read-only)
   - Mounting the workspace directory at `/workspace/{basename}`
   - Mounting Claude-specific directories (~/.claude, XDG_RUNTIME_DIR) for tool integration
   - Chrooting into the new root and executing the target command

### Critical Implementation Details

**User Namespace Mapping** (lines 105-114): Maps the current host user to UID/GID 0 inside the namespace. This provides privilege-like operations (chroot, mount) without actual root access on the host.

**Mount Propagation** (line 161): All mounts are made `MS_PRIVATE` to prevent mount events from propagating to/from the host.

**System Directory Mounts** (lines 173-209): Standard directories (/bin, /usr, /lib, /etc, etc.) are bind mounted read-only to provide access to system tools while preventing modification.

**Workspace Mounting** (lines 211-220): The jail directory is mounted at `/workspace/{basename}` (preserving the directory name for context) rather than directly at root to provide a clean, isolated workspace view.

**Claude Integration** (lines 237-295): Special handling for Claude Code integration:
- Mounts `~/.claude` directory (read-write) to preserve authentication state
- Mounts `~/.claude.json` file directly (requires creating mount point first)
- Mounts `XDG_RUNTIME_DIR` for runtime data
- Preserves HOME environment variable so Claude finds config at `$HOME/.claude`

**Docker Support** (lines 159-227): Automatic Docker socket detection and mounting:
- Detects Docker socket from `DOCKER_HOST` environment variable or standard locations
- Supports `/var/run/docker.sock`, `/run/docker.sock`, and rootless Docker
- Mounts Docker socket if available (non-fatal if Docker is not running)
- Preserves `DOCKER_HOST` environment variable for Docker CLI

**Configuration Files**: Reads additional directories to mount from two `.jail` files:
- Global: `$HOME/.jail` - applies to all jailed processes (read first)
- Local: `<workspace>/.jail` - applies to specific workspace (read second, can add to or override global)
Each line is an absolute path. Lines starting with `#` are comments. Both configs are merged with global mounts first, then local additions.

**Command Resolution** (lines 343-380): Searches for executables in standard paths plus any custom directories from `.jail` file. Checks directories and their `bin/` and `shims/` subdirectories.

## Common Commands

### Building
```bash
go build -o jail ./cmd/main.go
```

### Running
```bash
# Basic usage (current directory as workspace)
./jail <command> [args...]

# Specify workspace directory
./jail -d <directory> <command> [args...]

# Examples
./jail /bin/bash
./jail -d /tmp/project python3 app.py
./jail docker ps  # Docker support (auto-detected)
```

### Testing in Development
Since this is a namespace-based tool, testing requires running the compiled binary:
```bash
go build -o jail ./cmd/main.go
./jail /bin/bash  # Test basic execution
./jail echo "test"  # Test simple command
```

## Important Constraints

- **Linux-only**: Requires Linux kernel 3.8+ with namespace support
- **No network isolation**: Uses host network stack (no CLONE_NEWNET)
- **System directories are read-only**: Cannot modify /usr, /bin, /lib, etc.
- **PATH resolution happens inside jail**: Commands are resolved after chroot
- **Mount points must exist**: Cannot bind mount non-existent directories

## Working with Claude Code

When Claude Code runs inside jail:
- Authentication persists via mounted `~/.claude` directory
- HOME environment variable points to the host home directory
- Runtime data goes to mounted `XDG_RUNTIME_DIR`
- Working directory is `/workspace/{basename}`

To test Claude Code integration:
```bash
./jail claude -p "say hi"
```
