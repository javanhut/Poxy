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
	dryRun    bool
	verbose   bool
	sudoAsked bool // Whether we've already asked for sudo password this session
}

// New creates a new Executor with the given options.
func New(dryRun, verbose bool) *Executor {
	return &Executor{
		dryRun:  dryRun,
		verbose: verbose,
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
