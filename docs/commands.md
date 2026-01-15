# Commands Reference

## Global Flags

These flags work with all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | | Path to config file |
| `--source` | `-s` | Package source (apt, pacman, aur, flatpak, etc.) |
| `--dry-run` | `-n` | Show what would happen without executing |
| `--yes` | `-y` | Assume yes to all prompts |
| `--verbose` | `-v` | Verbose output |
| `--no-color` | | Disable colored output |

## Package Management

### install

Install one or more packages.

```bash
poxy install <packages...> [flags]
```

**Examples:**
```bash
poxy install vim                    # Auto-detect source
poxy install discord                # Finds in AUR automatically
poxy install firefox -s flatpak    # Force Flatpak
poxy install vim git curl          # Multiple packages
poxy install -y neovim             # No confirmation
```

**Behavior:**
1. If `-s` specified, uses that source directly
2. Otherwise, checks native repos first
3. If not found, searches AUR, Flatpak, Snap (in priority order)
4. Groups packages by source for efficient installation

### uninstall

Remove one or more packages. Aliases: `remove`, `rm`

```bash
poxy uninstall <packages...> [flags]
```

**Examples:**
```bash
poxy uninstall vim
poxy remove firefox -s flatpak
poxy rm discord
```

### update

Refresh package database/repository cache.

```bash
poxy update [flags]
```

**Examples:**
```bash
poxy update              # Update native manager
poxy update -s flatpak   # Update Flatpak only
```

### upgrade

Upgrade installed packages to latest versions.

```bash
poxy upgrade [packages...] [flags]
```

**Examples:**
```bash
poxy upgrade            # Upgrade all packages
poxy upgrade vim git    # Upgrade specific packages
poxy upgrade -y         # No confirmation
```

### search

Search for packages across all available sources.

```bash
poxy search <query> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--installed` | Search installed packages only |
| `--limit, -l` | Limit results per source |

**Examples:**
```bash
poxy search firefox           # Search all sources
poxy search vim -s pacman     # Search pacman only
poxy search --installed vim   # Search installed only
poxy search -l 5 editor       # Limit to 5 results per source
```

### info

Display detailed information about a package.

```bash
poxy info <package> [flags]
```

**Examples:**
```bash
poxy info vim
poxy info firefox -s flatpak
```

### list

List installed packages.

```bash
poxy list [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--limit, -l` | Limit number of results |
| `--pattern, -p` | Filter by name pattern |

**Examples:**
```bash
poxy list                 # List all installed
poxy list -s aur          # List AUR packages only
poxy list -p vim          # Filter by pattern
```

## Maintenance

### clean

Remove cached package files.

```bash
poxy clean [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--all, -a` | Clean all cached data |

**Examples:**
```bash
poxy clean       # Clean old cache
poxy clean -a    # Clean everything
```

### autoremove

Remove orphaned packages (unused dependencies).

```bash
poxy autoremove [flags]
```

## History & Rollback

### history

Show operation history.

```bash
poxy history [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--limit, -l` | Number of entries to show (default: 20) |
| `--clear` | Clear all history |

**Examples:**
```bash
poxy history           # Show recent history
poxy history -l 50     # Show last 50 entries
poxy history --clear   # Clear history
```

### rollback

Undo the last reversible operation.

```bash
poxy rollback [flags]
```

**Examples:**
```bash
poxy rollback          # Undo last operation
poxy rollback -y       # No confirmation
```

## System

### system

Display system information.

```bash
poxy system
```

Shows:
- Operating system and architecture
- Detected distribution
- Native package manager
- Available package sources

### doctor

Run diagnostics and check for issues.

```bash
poxy doctor
```

Checks:
- System detection
- Package manager availability
- Configuration validity
- Search functionality

### version

Print poxy version.

```bash
poxy version
```

## Interactive

### tui

Launch the interactive terminal user interface.

```bash
poxy tui
```

See [TUI Mode](tui.md) for details.

## Shell Completions

### completion

Generate shell completion scripts.

```bash
poxy completion <shell>
```

**Supported shells:** bash, zsh, fish, powershell

**Examples:**
```bash
# Bash
poxy completion bash > /etc/bash_completion.d/poxy

# Zsh
poxy completion zsh > ~/.zsh/completions/_poxy

# Fish
poxy completion fish > ~/.config/fish/completions/poxy.fish
```
