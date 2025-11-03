# Jail

A lightweight Linux process isolation tool that provides filesystem jailing without requiring root privileges.
Allows you to run any app with full access to your local dev toolchain and project directory, without any access outside said directory, or privileged access to your system.

Example:
```bash

jail claude

```

## Overview

Jail creates an isolated execution environment for processes using Linux namespaces and bind mounts. It restricts filesystem access to a designated workspace directory while maintaining full access to system binaries and libraries.

**Key Features:**
- No sudo/root required - uses unprivileged user namespaces
- Full access to system tools and libraries
- Filesystem isolation - processes can only read/write within workspace
- Network access with DNS resolution
- Custom directory mounts via `.jail` configuration file
- Clean `/workspace` directory view for better navigation

## How It Works

Jail uses a two-stage execution model:

1. **Stage 1**: Creates Linux namespaces (mount, user, PID, UTS, IPC) and re-executes itself
2. **Stage 2**: Sets up bind mounts for system directories and the workspace, then chroots and executes the target command

The workspace directory appears as `/workspace` inside the jail, providing a clean view without system directory clutter.

## Building

```bash
go build -o jail ./cmd/main.go
```

## Usage

```bash
# Run command in current directory as workspace
jail <command> [args...]

# Run command with specified workspace directory
jail -d <directory> <command> [args...]
```

### Help, I get "Error: fork/exec /proc/self/exe: permission denied" when I run jail!
This is likely due to SELinux/AppArmor not allowing unprivileged namespaces, especially on Ubuntu based systems.
There are two potential fixes for this:

#### Option 1: Enable Unprivileged User Namespaces (Recommended for Development)
```
# Temporarily (until reboot)
sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0

# Permanently
echo 'kernel.apparmor_restrict_unprivileged_userns = 0' | sudo tee /etc/sysctl.d/99-enable-unpriv-userns.conf
sudo sysctl -p /etc/sysctl.d/99-enable-unpriv-userns.conf
```
#### Option 2 (recommended): Create an App Armor profile for Jail
```
# Create profile
sudo tee /etc/apparmor.d/jail << 'EOF'
abi <abi/4.0>,
include <tunables/global>

/path/to/your/jail {
  include <abstractions/base>
  capability sys_admin,
  capability sys_chroot,
  
  # Allow namespace creation
  userns,
  
  # Allow re-exec
  /proc/self/exe r,
  
  # Allow executing itself
  /path/to/your/jail rix,
}
EOF

# Load the profile
sudo apparmor_parser -r /etc/apparmor.d/jail
```
### Examples

```bash
# Start a shell in current directory
jail /bin/bash

# Run a shell in specific directory
jail -d /tmp/myproject /bin/bash

# Execute a command with arguments
jail node index.js

# Use with pipes and redirects
echo "console.log('hello')" | jail node

# Run Python script
jail -d /home/user/project python3 app.py

# Use Docker inside jail
jail docker ps
jail docker run --rm alpine echo "Hello from Docker"
```

## Configuration

### `.jail` File

Jail supports two configuration files for mounting additional directories:

1. **Global config**: `$HOME/.jail` - applies to all jailed processes
2. **Local config**: `<workspace>/.jail` - applies only to that workspace

Both configs are merged, with the local config additions coming after global ones. This allows you to define common mounts globally while adding project-specific mounts locally.

**Format:**
- One absolute path per line
- Lines starting with `#` are comments
- Empty lines are ignored

**Example global `$HOME/.jail` file:**
```
# Mise tool manager (used across all projects)
/home/user/.local/share/mise
/home/user/.config/mise

# Docker configuration
/home/user/.docker
```

**Example workspace `.jail` file:**
```
# Custom toolchain for this project only
/opt/custom-tools

# Additional libraries
/usr/local/custom-lib
```

## What Gets Mounted

### Default System Directories (Read-Only)
- `/bin` - Essential binaries
- `/usr` - User programs and libraries
- `/lib`, `/lib64` - System libraries
- `/sbin` - System binaries
- `/etc` - Configuration files (needed for DNS/network)

### Special Directories
- `/proc` - Process information filesystem
- `/dev` - Device files
- `/tmp` - Temporary files (isolated)
- `/workspace` - Your workspace directory (read-write)

### Docker Support
- Docker socket (auto-detected from `DOCKER_HOST` or standard locations)
- Allows running Docker commands inside jail
- Requires Docker to be installed and running on the host

### Custom Directories
Any paths listed in `$HOME/.jail` (global) or `<workspace>/.jail` (local) files (read-only by default)

## Security Model

**Isolation Provided:**
- ✅ Filesystem isolation - cannot access files outside workspace
- ✅ Process isolation - separate PID namespace
- ✅ Hostname isolation - separate UTS namespace
- ✅ IPC isolation - separate IPC namespace
- ✅ System directories are read-only

**Shared with Host:**
- ⚠️ Network stack - uses host networking
- ⚠️ User namespace mapping - appears as "root" inside but unprivileged outside

**Not Suitable For:**
- Multi-tenant security isolation
- Running untrusted code
- Production container workloads

**Best Used For:**
- Development environments
- Build sandboxing
- Testing with filesystem isolation
- Preventing accidental file modifications outside project directories

## How Jail Differs from Docker/Containers

| Feature | Jail | Docker |
|---------|------|--------|
| Privileges | No root required | Requires root or docker group |
| Setup | Single binary | Daemon + images |
| Overhead | Minimal | Image layers + daemon |
| Network | Shared host network | Isolated (by default) |
| Use Case | Lightweight dev isolation | Full application containers |
| Tool Access | Direct host tools | Must be in image |

## Limitations

- **Linux only** - requires Linux kernel namespace support
- **No network isolation** - shares host network stack
- **Read-only system dirs** - cannot modify system files
- **Not a security sandbox** - not designed for untrusted code execution
- **PATH resolution** - commands are resolved at execution time within the jail

## Requirements

- Linux kernel with namespace support (3.8+)
- Go 1.16+ (for building)
- User namespace support enabled in kernel

## Troubleshooting

### Command not found
- Ensure the command exists in standard paths (`/bin`, `/usr/bin`, etc.)
- For custom tool locations, add them to `.jail` file
- Check that the executable has execute permissions

### Network issues
- DNS problems: Check `/etc/resolv.conf` is accessible
- Connection issues: Verify host has network connectivity

### Permission denied
- Cannot write to system directories: This is intentional
- Cannot access host files: Files must be in workspace directory
- Cannot create devices: Not supported in user namespaces

### Mise/asdf tools not working
Add both the tool directory and config directory to `$HOME/.jail` (for all projects) or workspace `.jail` (for one project):
```
/home/user/.local/share/mise
/home/user/.config/mise
```

## License

MIT

## Contributing

Contributions welcome! Please open an issue or pull request.

## Similar Projects

- [bubblewrap](https://github.com/containers/bubblewrap) - More feature-rich sandboxing
- [firejail](https://github.com/netblue30/firejail) - Security-focused sandbox
- [nsjail](https://github.com/google/nsjail) - Lightweight process isolation for security research
