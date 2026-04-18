// Package executor handles command execution with privilege escalation support.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Executor handles command execution with optional sudo elevation.
type Executor struct {
	dryRun  bool
	verbose bool
}

// defaultVerbose is applied as an OR to every executor created via New.
// It lets the CLI layer set verbose mode globally after flag parsing
// without touching every executor.New call site.
var defaultVerbose bool

// SetDefaultVerbose sets the package-level default verbose flag.
// Executors created after this call inherit the flag (ORed with their
// explicit argument).
func SetDefaultVerbose(v bool) {
	defaultVerbose = v
}

// New creates a new Executor with the given options.
func New(dryRun, verbose bool) *Executor {
	return &Executor{
		dryRun:  dryRun,
		verbose: verbose || defaultVerbose,
	}
}

// SetDryRun enables or disables dry-run mode.
func (e *Executor) SetDryRun(dryRun bool) {
	e.dryRun = dryRun
}

// SetVerbose enables or disables verbose mode.
func (e *Executor) SetVerbose(verbose bool) {
	e.verbose = verbose
}

// Run executes a command without sudo.
func (e *Executor) Run(ctx context.Context, name string, args ...string) error {
	if e.dryRun {
		e.printDryRun(name, args)
		return nil
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if e.verbose {
		fmt.Printf("Executing: %s %s\n", name, strings.Join(args, " "))
	}

	return cmd.Run()
}

// RunSudo executes a command with sudo if not already root.
func (e *Executor) RunSudo(ctx context.Context, name string, args ...string) error {
	if e.dryRun {
		e.printDryRunSudo(name, args)
		return nil
	}

	var cmd *exec.Cmd
	if isRoot() {
		cmd = exec.CommandContext(ctx, name, args...)
	} else if hasSudo() {
		sudoArgs := append([]string{name}, args...)
		cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
	} else {
		return fmt.Errorf("this operation requires root privileges, but sudo is not available")
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if e.verbose {
		if isRoot() {
			fmt.Printf("Executing (as root): %s %s\n", name, strings.Join(args, " "))
		} else {
			fmt.Printf("Executing (with sudo): %s %s\n", name, strings.Join(args, " "))
		}
	}

	return cmd.Run()
}

// RunSudoWithStderr executes a command with sudo while capturing stderr.
// It streams both stdout and stderr to the terminal while also capturing stderr
// for error analysis. Returns the captured stderr and any error.
func (e *Executor) RunSudoWithStderr(ctx context.Context, name string, args ...string) (string, error) {
	if e.dryRun {
		e.printDryRunSudo(name, args)
		return "", nil
	}

	var cmd *exec.Cmd
	if isRoot() {
		cmd = exec.CommandContext(ctx, name, args...)
	} else if hasSudo() {
		sudoArgs := append([]string{name}, args...)
		cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
	} else {
		return "", fmt.Errorf("this operation requires root privileges, but sudo is not available")
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	// Capture stderr while still streaming it to terminal
	var stderrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if e.verbose {
		if isRoot() {
			fmt.Printf("Executing (as root): %s %s\n", name, strings.Join(args, " "))
		} else {
			fmt.Printf("Executing (with sudo): %s %s\n", name, strings.Join(args, " "))
		}
	}

	err := cmd.Run()
	return stderrBuf.String(), err
}

// Output runs a command and returns its stdout.
func (e *Executor) Output(ctx context.Context, name string, args ...string) (string, error) {
	if e.dryRun {
		e.printDryRun(name, args)
		return "", nil
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if e.verbose {
		fmt.Printf("Executing: %s %s\n", name, strings.Join(args, " "))
	}

	err := cmd.Run()
	return stdout.String(), err
}

// OutputQuiet runs a command and returns its stdout, suppressing stderr.
func (e *Executor) OutputQuiet(ctx context.Context, name string, args ...string) (string, error) {
	if e.dryRun {
		e.printDryRun(name, args)
		return "", nil
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	// Suppress stderr

	if e.verbose {
		fmt.Printf("Executing: %s %s\n", name, strings.Join(args, " "))
	}

	err := cmd.Run()
	return stdout.String(), err
}

// OutputSudo runs a command with sudo and returns its stdout.
func (e *Executor) OutputSudo(ctx context.Context, name string, args ...string) (string, error) {
	if e.dryRun {
		e.printDryRunSudo(name, args)
		return "", nil
	}

	var cmd *exec.Cmd
	if isRoot() {
		cmd = exec.CommandContext(ctx, name, args...)
	} else if hasSudo() {
		sudoArgs := append([]string{name}, args...)
		cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
	} else {
		return "", fmt.Errorf("this operation requires root privileges, but sudo is not available")
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if e.verbose {
		if isRoot() {
			fmt.Printf("Executing (as root): %s %s\n", name, strings.Join(args, " "))
		} else {
			fmt.Printf("Executing (with sudo): %s %s\n", name, strings.Join(args, " "))
		}
	}

	err := cmd.Run()
	return stdout.String(), err
}

// OutputCombined runs a command and returns both stdout and stderr combined.
func (e *Executor) OutputCombined(ctx context.Context, name string, args ...string) (string, error) {
	if e.dryRun {
		e.printDryRun(name, args)
		return "", nil
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	if e.verbose {
		fmt.Printf("Executing: %s %s\n", name, strings.Join(args, " "))
	}

	err := cmd.Run()
	return combined.String(), err
}

// RunInteractive runs a command that requires user interaction.
func (e *Executor) RunInteractive(ctx context.Context, name string, args ...string) error {
	if e.dryRun {
		e.printDryRun(name, args)
		return nil
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunWithOutput runs a command and streams output while also capturing it.
func (e *Executor) RunWithOutput(ctx context.Context, name string, args ...string) (string, error) {
	if e.dryRun {
		e.printDryRun(name, args)
		return "", nil
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin

	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	return buf.String(), err
}

// RunCaptured runs a command, capturing stdout+stderr to a single buffer
// instead of inheriting the user's TTY. Returns the combined output.
// When verbose, mirrors output to the terminal as well.
func (e *Executor) RunCaptured(ctx context.Context, name string, args ...string) (string, error) {
	if e.dryRun {
		e.printDryRun(name, args)
		return "", nil
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var combined bytes.Buffer
	if e.verbose {
		fmt.Printf("Executing: %s %s\n", name, strings.Join(args, " "))
		cmd.Stdout = io.MultiWriter(os.Stdout, &combined)
		cmd.Stderr = io.MultiWriter(os.Stderr, &combined)
	} else {
		cmd.Stdout = &combined
		cmd.Stderr = &combined
	}

	err := cmd.Run()
	return combined.String(), err
}

// RunSudoCaptured runs a command with sudo, capturing stdout+stderr to a
// single buffer instead of inheriting the user's TTY. Returns the combined
// output. When verbose, mirrors output to the terminal as well. Callers
// should invoke EnsureSudo beforehand so password prompts don't get
// swallowed by the captured streams.
func (e *Executor) RunSudoCaptured(ctx context.Context, name string, args ...string) (string, error) {
	if e.dryRun {
		e.printDryRunSudo(name, args)
		return "", nil
	}

	var cmd *exec.Cmd
	if isRoot() {
		cmd = exec.CommandContext(ctx, name, args...)
	} else if hasSudo() {
		sudoArgs := append([]string{"-n", name}, args...)
		cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
	} else {
		return "", fmt.Errorf("this operation requires root privileges, but sudo is not available")
	}

	var combined bytes.Buffer
	if e.verbose {
		if isRoot() {
			fmt.Printf("Executing (as root): %s %s\n", name, strings.Join(args, " "))
		} else {
			fmt.Printf("Executing (with sudo): %s %s\n", name, strings.Join(args, " "))
		}
		cmd.Stdout = io.MultiWriter(os.Stdout, &combined)
		cmd.Stderr = io.MultiWriter(os.Stderr, &combined)
	} else {
		cmd.Stdout = &combined
		cmd.Stderr = &combined
	}

	err := cmd.Run()
	return combined.String(), err
}

// EnsureSudo pre-warms the sudo credential cache so subsequent captured
// sudo commands don't prompt for a password (which would be swallowed by
// the captured streams). No-op if running as root or if sudo credentials
// are already cached.
func (e *Executor) EnsureSudo(ctx context.Context) error {
	if isRoot() || !hasSudo() {
		return nil
	}
	// Test for cached credentials without prompting.
	test := exec.CommandContext(ctx, "sudo", "-n", "-v")
	if test.Run() == nil {
		return nil
	}
	// Not cached: prompt interactively.
	prompt := exec.CommandContext(ctx, "sudo", "-v")
	prompt.Stdin = os.Stdin
	prompt.Stdout = os.Stdout
	prompt.Stderr = os.Stderr
	return prompt.Run()
}

func (e *Executor) printDryRun(name string, args []string) {
	fmt.Printf("[dry-run] Would execute: %s %s\n", name, strings.Join(args, " "))
}

func (e *Executor) printDryRunSudo(name string, args []string) {
	if isRoot() {
		fmt.Printf("[dry-run] Would execute (as root): %s %s\n", name, strings.Join(args, " "))
	} else {
		fmt.Printf("[dry-run] Would execute (with sudo): sudo %s %s\n", name, strings.Join(args, " "))
	}
}
