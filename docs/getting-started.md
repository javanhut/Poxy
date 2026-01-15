# Getting Started

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/poxy.git
cd poxy

# Build and install
make install
```

This installs poxy to `~/.local/bin/poxy`. Make sure `~/.local/bin` is in your PATH.

### System-wide Installation

```bash
make install-system
```

This installs to `/usr/local/bin/poxy` (requires sudo).

### Shell Completions

```bash
# Generate completions
make completions

# Install for your shell
make install-completions-bash  # Bash
make install-completions-zsh   # Zsh
make install-completions-fish  # Fish
```

## First Run

After installation, verify poxy is working:

```bash
# Check version
poxy version

# Run diagnostics
poxy doctor
```

The `doctor` command shows:
- Detected operating system
- Available package managers
- Configuration status
- Any issues that need attention

## Basic Usage

### Installing Packages

```bash
# Install a package (auto-detects best source)
poxy install vim

# Install multiple packages
poxy install vim git curl

# Install from specific source
poxy install firefox -s flatpak
```

Poxy automatically:
1. Checks if the package exists in official repos
2. If not found, searches AUR, Flatpak, Snap
3. Installs from the best available source

### Searching Packages

```bash
# Search all sources
poxy search vscode

# Search specific source
poxy search firefox -s flatpak
```

### Removing Packages

```bash
# Remove a package
poxy uninstall vim

# Remove with dependencies
poxy uninstall vim  # Prompts to remove unused deps
```

### Updating

```bash
# Update package database
poxy update

# Upgrade all packages
poxy upgrade

# Upgrade specific packages
poxy upgrade vim git
```

## Interactive Mode

Launch the TUI for a visual interface:

```bash
poxy tui
```

Navigation:
- `j/k` or Arrow keys - Move up/down
- `1-5` - Switch tabs
- `/` - Search
- `i` - Install selected package
- `r` - Remove selected package
- `?` - Help
- `q` - Quit

## Configuration

Create a config file at `~/.config/poxy/poxy.toml`:

```toml
[general]
source_priority = ["native", "aur", "flatpak", "snap"]
auto_confirm = false

[output]
color = true
unicode = true

[managers.aur]
use_native = true
review_pkgbuild = true
use_sandbox = true

[aliases]
code = "visual-studio-code-bin"
chrome = "google-chrome"
```

See [Configuration](configuration.md) for all options.

## Next Steps

- [Commands Reference](commands.md) - All available commands
- [TUI Mode](tui.md) - Interactive terminal interface
- [AUR Support](aur.md) - Building AUR packages
- [Configuration](configuration.md) - All config options
