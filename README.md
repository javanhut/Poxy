# Poxy

**Universal Package Manager for Any Operating System**

Poxy unifies package management across Linux distributions, macOS, and Windows. Use familiar commands regardless of your system's native package manager.

[![CI](https://github.com/yourusername/poxy/workflows/CI/badge.svg)](https://github.com/yourusername/poxy/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/poxy)](https://goreportcard.com/report/github.com/yourusername/poxy)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- **18+ Package Managers** - Support for all major Linux distros, macOS, and Windows
- **Multi-Source Search** - Search across all available sources with interactive selection
- **Unified Commands** - Same commands work everywhere: `install`, `uninstall`, `update`, `upgrade`, `search`
- **Operation History** - Track all operations with rollback support
- **TOML Configuration** - Customize behavior, set aliases, configure per-manager settings
- **Auto-Sudo** - Automatically elevates privileges when needed
- **Dry-Run Mode** - Preview operations before executing
- **Shell Completions** - Tab completion for bash, zsh, fish, and PowerShell

## Supported Package Managers

| Platform | Package Managers |
|----------|-----------------|
| **Debian/Ubuntu** | apt |
| **Fedora/RHEL** | dnf |
| **Arch Linux** | pacman, yay, paru (AUR) |
| **openSUSE** | zypper |
| **Void Linux** | xbps |
| **Alpine Linux** | apk |
| **Gentoo** | emerge |
| **Solus** | eopkg |
| **NixOS** | nix |
| **Slackware** | slackpkg |
| **Clear Linux** | swupd |
| **macOS** | brew |
| **Windows** | winget, chocolatey, scoop |
| **Universal** | flatpak, snap |

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/poxy.git
cd poxy

# Build and install
make build
sudo make install-system

# Or install to GOPATH/bin
make install
```

### Using Go

```bash
go install github.com/yourusername/poxy/cmd/poxy@latest
```

### Pre-built Binaries

Download from [Releases](https://github.com/yourusername/poxy/releases).

## Quick Start

```bash
# Install packages
poxy install vim git curl

# Search across all sources
poxy search firefox

# Install from a specific source
poxy install discord --source flatpak
poxy install -s flatpak discord

# Update package database
poxy update

# Upgrade all packages
poxy upgrade

# Remove packages
poxy uninstall vim

# Show system info
poxy system

# Run diagnostics
poxy doctor
```

## Commands

| Command | Description |
|---------|-------------|
| `install` | Install one or more packages |
| `uninstall` | Remove one or more packages |
| `update` | Update package database |
| `upgrade` | Upgrade installed packages |
| `search` | Search for packages across all sources |
| `info` | Show detailed package information |
| `list` | List installed packages |
| `clean` | Clean package cache |
| `autoremove` | Remove orphaned packages |
| `history` | Show operation history |
| `rollback` | Undo last operation |
| `system` | Show system information |
| `doctor` | Diagnose system issues |

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--source` | `-s` | Specify package source |
| `--dry-run` | `-n` | Show what would happen without executing |
| `--yes` | `-y` | Assume yes to all prompts |
| `--verbose` | `-v` | Verbose output |
| `--no-color` | | Disable colored output |
| `--config` | | Specify config file path |

## Configuration

Poxy uses a TOML configuration file located at:
- Linux: `~/.config/poxy/config.toml`
- macOS: `~/Library/Application Support/poxy/config.toml`
- Windows: `%APPDATA%\poxy\config.toml`

### Example Configuration

```toml
[general]
# Source priority - searched in order
source_priority = ["native", "flatpak", "snap", "aur"]
auto_confirm = false
dry_run = false

[output]
color = true
unicode = true
verbose = false

[managers.pacman]
aur_helper = "yay"  # or "paru"

[managers.apt]
use_nala = false  # Use nala frontend if available

[managers.flatpak]
default_remote = "flathub"

# Package aliases
[aliases]
code = "visual-studio-code"
vim = "neovim"
ff = "firefox"
```

## Shell Completions

Generate and install shell completions:

```bash
# Generate all completions
make completions

# Install for bash
make install-completions-bash

# Install for zsh
make install-completions-zsh

# Install for fish
make install-completions-fish

# Manual installation
poxy completion bash > /etc/bash_completion.d/poxy
poxy completion zsh > ~/.zsh/completions/_poxy
poxy completion fish > ~/.config/fish/completions/poxy.fish
```

## Examples

### Interactive Package Selection

When searching, poxy offers interactive selection:

```bash
$ poxy search firefox

Found 5 results across 3 sources

[apt] (2):
  firefox 125.0 [installed]
  firefox-esr 115.9

[flatpak] (2):
  org.mozilla.firefox 125.0
  org.mozilla.Firefox.Locale.en-US

[snap] (1):
  firefox 125.0

Use arrow keys to select, Enter to install
```

### Using Aliases

```bash
# In config.toml
[aliases]
vim = "neovim"

# Now you can use:
poxy install vim  # Actually installs neovim
```

### Source-Specific Installation

```bash
# Install Firefox from Flatpak instead of native
poxy install firefox -s flatpak

# Install development tools from native repos
poxy install -s native build-essential

# Install GUI apps from Snap
poxy install -s snap spotify
```

### Dry Run

```bash
$ poxy install vim git -n

[dry-run] Would execute (with sudo): sudo pacman -S --noconfirm vim git
```

### History and Rollback

```bash
$ poxy history
 1. 2024-01-15 10:30:45 install vim [pacman] (success) [reversible]
 2. 2024-01-15 10:25:12 upgrade [pacman] (success)
 3. 2024-01-15 10:20:00 install git [pacman] (success) [reversible]

$ poxy rollback
Rolling back: 2024-01-15 10:30:45 install vim [pacman] (success)
Reverse operation: uninstall
  - vim
Proceed with rollback? [y/N]
```

## Development

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Run linters
make lint
```

### Project Structure

```
poxy/
├── cmd/poxy/           # Main entry point
├── internal/
│   ├── cli/            # CLI commands
│   ├── config/         # Configuration handling
│   ├── executor/       # Command execution
│   ├── history/        # BoltDB history store
│   └── ui/             # Terminal UI helpers
├── pkg/manager/
│   ├── detector/       # OS/distro detection
│   ├── native/         # Native package managers
│   └── universal/      # Universal package managers
└── configs/            # Example configuration
```

### Adding a New Package Manager

1. Create a new file in `pkg/manager/native/` (or `universal/`)
2. Implement the `Manager` interface from `pkg/manager/manager.go`
3. Register it in `internal/cli/root.go`
4. Add distro detection in `pkg/manager/detector/linux.go` if needed

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [BoltDB](https://github.com/etcd-io/bbolt) - Key-value store for history
- [promptui](https://github.com/manifoldco/promptui) - Interactive prompts
- [fatih/color](https://github.com/fatih/color) - Terminal colors

---

Made with :heart: for the Linux community
