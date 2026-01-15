# Poxy Documentation

Poxy is a universal package manager that provides a unified interface across Linux, macOS, and Windows.

## Table of Contents

- [Getting Started](getting-started.md)
- [Commands](commands.md)
- [Configuration](configuration.md)
- [TUI Mode](tui.md)
- [AUR Support](aur.md)
- [Smart Search](smart-search.md)
- [Architecture](architecture.md)

## Quick Start

```bash
# Install a package (automatically finds best source)
poxy install firefox

# Search across all sources
poxy search vscode

# Launch interactive TUI
poxy tui

# Check system status
poxy doctor
```

## Why Poxy?

| Feature | apt | pacman | yay | brew | Poxy |
|---------|-----|--------|-----|------|------|
| Cross-platform | No | No | No | Partial | Yes |
| Multi-source search | No | No | Yes | No | Yes |
| Native AUR building | N/A | No | Yes | N/A | Yes |
| Sandboxed builds | No | No | No | No | Yes |
| TF-IDF smart search | No | No | No | No | Yes |
| Interactive TUI | No | No | No | No | Yes |
| PKGBUILD security scan | N/A | N/A | Limited | N/A | Yes |
| Operation history | No | No | No | No | Yes |

## Supported Package Managers

### Linux
- **apt** - Debian, Ubuntu, Linux Mint
- **dnf** - Fedora, RHEL, CentOS
- **pacman** - Arch Linux, Manjaro, EndeavourOS
- **zypper** - openSUSE
- **xbps** - Void Linux
- **apk** - Alpine Linux
- **emerge** - Gentoo
- **eopkg** - Solus
- **nix** - NixOS
- **slackpkg** - Slackware
- **swupd** - Clear Linux

### macOS
- **brew** - Homebrew

### Windows
- **winget** - Windows Package Manager
- **chocolatey** - Chocolatey
- **scoop** - Scoop

### Universal
- **flatpak** - Flatpak (Linux)
- **snap** - Snap (Linux)
- **aur** - Arch User Repository (Arch Linux)
