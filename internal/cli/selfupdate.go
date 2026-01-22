package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"poxy/internal/ui"

	"github.com/spf13/cobra"
)

const (
	repoURL    = "https://github.com/javanhut/Poxy.git"
	repoBranch = "main"
)

var (
	forceUpdate bool
	keepTemp    bool
)

var selfUpdateCmd = &cobra.Command{
	Use:     "self-update",
	Aliases: []string{"selfupdate", "update-self"},
	Short:   "Update poxy to the latest version from source",
	Long: `Update poxy to the latest version by pulling the source code
from the main branch and building from source.

Requirements:
  - git: to clone the repository
  - go: to build the binary

The command will:
  1. Clone the latest source from the main branch
  2. Build a new binary with version information
  3. Verify the new binary works
  4. Replace the current binary (with backup)

Examples:
  poxy self-update              # Update to latest version
  poxy self-update --force      # Update even if already on latest
  poxy self-update --dry-run    # Show what would happen without updating`,
	RunE: runSelfUpdate,
}

func init() {
	selfUpdateCmd.Flags().BoolVar(&forceUpdate, "force", false, "update even if already on latest version")
	selfUpdateCmd.Flags().BoolVar(&keepTemp, "keep-temp", false, "keep temporary directory for debugging")
}

// SelfUpdater manages the self-update process.
type SelfUpdater struct {
	currentBinary string
	tempDir       string
	newBinary     string
	newVersion    string
	newCommit     string
}

func runSelfUpdate(cmd *cobra.Command, args []string) error {
	ui.HeaderMsg("Poxy Self-Update")
	ui.InfoMsg("Current version: %s", Version)

	updater := &SelfUpdater{}

	// Detect current binary location
	if err := updater.detectCurrentBinary(); err != nil {
		return fmt.Errorf("failed to detect current binary: %w", err)
	}

	// Check prerequisites
	ui.Println("")
	ui.InfoMsg("Checking prerequisites...")
	if err := updater.checkPrerequisites(); err != nil {
		return err
	}

	needsSudo := needsSudoForPath(updater.currentBinary)
	if needsSudo {
		ui.MutedMsg("  Binary location: %s (requires sudo)", updater.currentBinary)
	} else {
		ui.SuccessMsg("Binary location: %s", updater.currentBinary)
	}

	// Create temp directory
	var err error
	updater.tempDir, err = os.MkdirTemp("", "poxy-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Clean up temp dir on completion (unless --keep-temp or failure)
	cleanupTemp := true
	defer func() {
		if cleanupTemp && !keepTemp {
			os.RemoveAll(updater.tempDir)
		}
	}()

	// Fetch latest source
	ui.Println("")
	ui.InfoMsg("Fetching latest source...")
	if err := updater.fetchLatestSource(); err != nil {
		cleanupTemp = false
		ui.MutedMsg("  Temp directory kept at: %s", updater.tempDir)
		return fmt.Errorf("failed to fetch source: %w", err)
	}
	ui.SuccessMsg("Repository cloned")

	// Build new binary
	ui.Println("")
	ui.InfoMsg("Building new version...")
	if err := updater.buildBinary(); err != nil {
		cleanupTemp = false
		ui.MutedMsg("  Temp directory kept at: %s", updater.tempDir)
		return fmt.Errorf("failed to build binary: %w", err)
	}
	ui.SuccessMsg("Build successful (version %s)", updater.newVersion)

	// Verify new binary
	ui.Println("")
	ui.InfoMsg("Verifying new binary...")
	if err := updater.verifyNewBinary(); err != nil {
		cleanupTemp = false
		ui.MutedMsg("  Temp directory kept at: %s", updater.tempDir)
		return fmt.Errorf("verification failed: %w", err)
	}
	ui.SuccessMsg("Verification passed")

	// Check if we need to update
	if !forceUpdate && updater.newVersion == Version && updater.newCommit == Commit {
		ui.Println("")
		ui.SuccessMsg("Already running the latest version (%s)", Version)
		return nil
	}

	// Dry run check
	if cfg.General.DryRun {
		ui.Println("")
		ui.InfoMsg("Dry run: would update from %s to %s", Version, updater.newVersion)
		return nil
	}

	// Confirm update
	ui.Println("")
	if needsSudo {
		ui.WarningMsg("Install location: %s (requires sudo)", updater.currentBinary)
	}

	if !cfg.General.AutoConfirm {
		confirmed, err := ui.Confirm("Proceed with update?", true)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrAborted
		}
	}

	// Replace binary
	ui.Println("")
	ui.InfoMsg("Installing...")
	if err := updater.replaceBinary(needsSudo); err != nil {
		cleanupTemp = false
		ui.MutedMsg("  Temp directory kept at: %s", updater.tempDir)
		return fmt.Errorf("failed to install: %w", err)
	}
	ui.SuccessMsg("Updated successfully!")

	ui.Println("")
	ui.SuccessMsg("poxy updated from %s to %s", Version, updater.newVersion)

	return nil
}

func (u *SelfUpdater) detectCurrentBinary() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	// Resolve symlinks to get the actual binary path
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		// If we can't resolve symlinks, use the original path
		resolved = executable
	}

	u.currentBinary = resolved
	return nil
}

func (u *SelfUpdater) checkPrerequisites() error {
	// Check for git
	if _, err := exec.LookPath("git"); err != nil {
		ui.ErrorMsg("git not found")
		ui.MutedMsg("  Please install git: https://git-scm.com/downloads")
		return fmt.Errorf("git is required for self-update")
	}
	ui.SuccessMsg("git found")

	// Check for go
	if _, err := exec.LookPath("go"); err != nil {
		ui.ErrorMsg("go not found")
		ui.MutedMsg("  Please install Go: https://go.dev/dl/")
		return fmt.Errorf("go is required for self-update")
	}
	ui.SuccessMsg("go found")

	return nil
}

func (u *SelfUpdater) fetchLatestSource() error {
	repoDir := filepath.Join(u.tempDir, "poxy")

	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", repoBranch, repoURL, repoDir)
	cmd.Dir = u.tempDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %s", stderr.String())
	}

	return nil
}

func (u *SelfUpdater) buildBinary() error {
	repoDir := filepath.Join(u.tempDir, "poxy")

	// Get version info from the new source
	version, commit := u.getVersionInfo(repoDir)
	u.newVersion = version
	u.newCommit = commit

	buildTime := time.Now().UTC().Format(time.RFC3339)

	// Determine output binary name (use poxy-new to avoid conflict with cloned repo dir)
	binaryName := "poxy-new"
	if runtime.GOOS == "windows" {
		binaryName = "poxy-new.exe"
	}
	u.newBinary = filepath.Join(u.tempDir, binaryName)

	// Detect the correct build path (./cmd/poxy or ./cmd/main.go)
	buildPath := "./cmd/poxy"
	if _, err := os.Stat(filepath.Join(repoDir, "cmd", "poxy")); os.IsNotExist(err) {
		// Fallback: if cmd/poxy doesn't exist, use cmd/main.go directly
		buildPath = "./cmd/main.go"
	}

	// Build with ldflags
	ldflags := fmt.Sprintf("-s -w -X poxy/internal/cli.Version=%s -X poxy/internal/cli.Commit=%s -X poxy/internal/cli.BuildTime=%s",
		version, commit, buildTime)

	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", u.newBinary, buildPath)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %s", stderr.String())
	}

	// Ensure the binary is executable
	if err := os.Chmod(u.newBinary, 0755); err != nil {
		return fmt.Errorf("failed to set permissions on new binary: %w", err)
	}

	return nil
}

func (u *SelfUpdater) getVersionInfo(repoDir string) (version, commit string) {
	// Default values
	version = "unknown"
	commit = "unknown"

	// Try to get the version from root.go
	rootFile := filepath.Join(repoDir, "internal", "cli", "root.go")
	if data, err := os.ReadFile(rootFile); err == nil {
		content := string(data)
		// Look for Version = "..."
		if idx := strings.Index(content, `Version = "`); idx != -1 {
			start := idx + len(`Version = "`)
			end := strings.Index(content[start:], `"`)
			if end != -1 {
				version = content[start : start+end]
			}
		}
	}

	// Get commit hash
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoDir
	if output, err := cmd.Output(); err == nil {
		commit = strings.TrimSpace(string(output))
	}

	return version, commit
}

func (u *SelfUpdater) verifyNewBinary() error {
	// Run the new binary with version flag to verify it works
	cmd := exec.Command(u.newBinary, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("new binary failed verification: %w\nOutput: %s", err, output)
	}

	if verbose {
		ui.MutedMsg("  Version output: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

func (u *SelfUpdater) replaceBinary(needsSudo bool) error {
	backupPath := u.currentBinary + ".bak"

	// Create backup of current binary
	if err := copyFile(u.currentBinary, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	var installErr error
	if needsSudo {
		installErr = u.installWithSudo()
	} else {
		installErr = u.installDirect()
	}

	if installErr != nil {
		// Restore from backup
		ui.WarningMsg("Installation failed, restoring backup...")
		if restoreErr := copyFile(backupPath, u.currentBinary); restoreErr != nil {
			ui.ErrorMsg("Failed to restore backup: %v", restoreErr)
			ui.MutedMsg("  Backup is available at: %s", backupPath)
			return fmt.Errorf("installation failed and restore failed: %w (restore error: %v)", installErr, restoreErr)
		}
		os.Remove(backupPath)
		return installErr
	}

	// Remove backup on success
	os.Remove(backupPath)
	return nil
}

func (u *SelfUpdater) installDirect() error {
	// Copy new binary to destination
	if err := copyFile(u.newBinary, u.currentBinary); err != nil {
		return fmt.Errorf("failed to copy new binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(u.currentBinary, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}

func (u *SelfUpdater) installWithSudo() error {
	// Use sudo cp to copy the binary
	cmd := exec.Command("sudo", "cp", u.newBinary, u.currentBinary)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo cp failed: %w", err)
	}

	// Make executable with sudo
	cmd = exec.Command("sudo", "chmod", "755", u.currentBinary)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo chmod failed: %w", err)
	}

	return nil
}

// needsSudoForPath checks if writing to the binary's directory requires elevated privileges.
func needsSudoForPath(path string) bool {
	dir := filepath.Dir(path)
	testFile := filepath.Join(dir, ".poxy-write-test")

	f, err := os.Create(testFile)
	if err != nil {
		return true
	}
	f.Close()
	os.Remove(testFile)
	return false
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
