package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	// ErrBubblewrapNotFound is returned when bwrap is not installed
	ErrBubblewrapNotFound = errors.New("bubblewrap (bwrap) is not installed")

	// ErrSandboxFailed is returned when the sandbox itself fails (setup, permissions, etc.)
	ErrSandboxFailed = errors.New("sandbox execution failed")

	// ErrCommandFailed is returned when the sandboxed command ran but exited with a non-zero code.
	// This means the sandbox worked fine but the command itself failed, so retrying
	// without sandbox would not help.
	ErrCommandFailed = errors.New("sandboxed command failed")
)

// Sandbox provides sandboxed execution using bubblewrap.
type Sandbox struct {
	bwrapPath string
	profile   *Profile
	workdir   string
	verbose   bool
}

// New creates a new sandbox with the given profile.
func New(profile *Profile) (*Sandbox, error) {
	bwrapPath, err := exec.LookPath("bwrap")
	if err != nil {
		return nil, ErrBubblewrapNotFound
	}

	return &Sandbox{
		bwrapPath: bwrapPath,
		profile:   profile,
	}, nil
}

// NewWithProfile creates a sandbox with a named profile.
func NewWithProfile(profileName string) (*Sandbox, error) {
	var profile *Profile
	switch profileName {
	case "build":
		profile = ProfileBuild.Clone()
	case "fetch":
		profile = ProfileFetch.Clone()
	case "minimal":
		profile = ProfileMinimal.Clone()
	default:
		return nil, fmt.Errorf("unknown profile: %s", profileName)
	}

	return New(profile)
}

// IsAvailable checks if bubblewrap is available on the system.
func IsAvailable() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

// SetWorkdir sets the working directory for sandboxed commands.
// This directory will be bind-mounted read-write.
func (s *Sandbox) SetWorkdir(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	s.workdir = absDir
	return nil
}

// SetVerbose enables verbose output of bwrap arguments.
func (s *Sandbox) SetVerbose(verbose bool) {
	s.verbose = verbose
}

// Profile returns the current profile.
func (s *Sandbox) Profile() *Profile {
	return s.profile
}

// buildArgs constructs the bwrap command line arguments.
func (s *Sandbox) buildArgs(cmd string, args []string) []string {
	var bwrapArgs []string

	// Unshare namespaces
	if s.profile.UnshareUser {
		bwrapArgs = append(bwrapArgs, "--unshare-user")
		if s.profile.UID > 0 {
			bwrapArgs = append(bwrapArgs, "--uid", fmt.Sprintf("%d", s.profile.UID))
		}
		if s.profile.GID > 0 {
			bwrapArgs = append(bwrapArgs, "--gid", fmt.Sprintf("%d", s.profile.GID))
		}
	}
	if s.profile.UnsharePID {
		bwrapArgs = append(bwrapArgs, "--unshare-pid")
	}
	if s.profile.UnshareNet {
		bwrapArgs = append(bwrapArgs, "--unshare-net")
	}
	if s.profile.UnshareIPC {
		bwrapArgs = append(bwrapArgs, "--unshare-ipc")
	}
	if s.profile.UnshareCgroup {
		bwrapArgs = append(bwrapArgs, "--unshare-cgroup")
	}

	// Process settings
	if s.profile.DieWithParent {
		bwrapArgs = append(bwrapArgs, "--die-with-parent")
	}
	if s.profile.NewSession {
		bwrapArgs = append(bwrapArgs, "--new-session")
	}

	// Read-only binds
	for _, bind := range s.profile.BindReadOnly {
		if pathExists(bind) {
			bwrapArgs = append(bwrapArgs, "--ro-bind", bind, bind)
		}
	}

	// Read-write binds
	for _, bind := range s.profile.BindReadWrite {
		if pathExists(bind) {
			bwrapArgs = append(bwrapArgs, "--bind", bind, bind)
		}
	}

	// Workdir (always read-write)
	if s.workdir != "" {
		bwrapArgs = append(bwrapArgs, "--bind", s.workdir, s.workdir)
		bwrapArgs = append(bwrapArgs, "--chdir", s.workdir)
	}

	// Create /proc (must come before symlinks that reference /proc/self/fd)
	bwrapArgs = append(bwrapArgs, "--proc", "/proc")

	// Device setup
	if s.profile.UseDev {
		// --dev /dev creates a proper devtmpfs with standard devices
		// (null, zero, random, urandom, tty) AND fd symlinks
		// (/dev/fd -> /proc/self/fd, /dev/stdin, /dev/stdout, /dev/stderr)
		bwrapArgs = append(bwrapArgs, "--dev", "/dev")
	} else {
		for _, dev := range s.profile.DevBinds {
			if pathExists(dev) {
				bwrapArgs = append(bwrapArgs, "--dev-bind", dev, dev)
			}
		}
	}

	// Tmpfs mounts
	for _, tmp := range s.profile.Tmpfs {
		bwrapArgs = append(bwrapArgs, "--tmpfs", tmp)
	}

	// Symlinks
	for dest, src := range s.profile.Symlinks {
		bwrapArgs = append(bwrapArgs, "--symlink", src, dest)
	}

	// Environment
	if s.profile.ClearEnv {
		bwrapArgs = append(bwrapArgs, "--clearenv")
	}

	// Pass through environment variables
	for _, envVar := range s.profile.EnvPass {
		if val, ok := os.LookupEnv(envVar); ok {
			bwrapArgs = append(bwrapArgs, "--setenv", envVar, val)
		}
	}

	// Set environment variables
	for key, val := range s.profile.Env {
		bwrapArgs = append(bwrapArgs, "--setenv", key, val)
	}

	// Capability dropping
	for _, cap := range s.profile.DropCaps {
		if cap == "ALL" {
			bwrapArgs = append(bwrapArgs, "--cap-drop", "ALL")
		} else {
			bwrapArgs = append(bwrapArgs, "--cap-drop", cap)
		}
	}

	// Add the command separator and command
	bwrapArgs = append(bwrapArgs, "--")
	bwrapArgs = append(bwrapArgs, cmd)
	bwrapArgs = append(bwrapArgs, args...)

	return bwrapArgs
}

// Run executes a command in the sandbox.
func (s *Sandbox) Run(ctx context.Context, cmd string, args ...string) error {
	bwrapArgs := s.buildArgs(cmd, args)

	if s.verbose {
		fmt.Fprintf(os.Stderr, "bwrap %s\n", strings.Join(bwrapArgs, " "))
	}

	execCmd := exec.CommandContext(ctx, s.bwrapPath, bwrapArgs...)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// The sandbox worked but the command itself failed
			return fmt.Errorf("%w: %s exited with code %d", ErrCommandFailed, cmd, exitErr.ExitCode())
		}
		// The sandbox itself failed to run (setup, permissions, etc.)
		return fmt.Errorf("%w: %v", ErrSandboxFailed, err)
	}

	return nil
}

// RunOutput executes a command in the sandbox and returns its output.
func (s *Sandbox) RunOutput(ctx context.Context, cmd string, args ...string) (string, error) {
	bwrapArgs := s.buildArgs(cmd, args)

	if s.verbose {
		fmt.Fprintf(os.Stderr, "bwrap %s\n", strings.Join(bwrapArgs, " "))
	}

	execCmd := exec.CommandContext(ctx, s.bwrapPath, bwrapArgs...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// The sandbox worked but the command itself failed
			return string(output), fmt.Errorf("%w: %s exited with code %d: %s",
				ErrCommandFailed, cmd, exitErr.ExitCode(), string(output))
		}
		// The sandbox itself failed to run
		return string(output), fmt.Errorf("%w: %v", ErrSandboxFailed, err)
	}

	return string(output), nil
}

// RunShell executes a shell command in the sandbox.
func (s *Sandbox) RunShell(ctx context.Context, shellCmd string) error {
	return s.Run(ctx, "/bin/bash", "-c", shellCmd)
}

// pathExists checks if a path exists.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// BuildSandbox creates a sandbox configured for building packages.
func BuildSandbox(workdir string, extraBinds ...string) (*Sandbox, error) {
	profile := ProfileBuild.Clone()
	profile.AddBindReadWrite(extraBinds...)

	sandbox, err := New(profile)
	if err != nil {
		return nil, err
	}

	if err := sandbox.SetWorkdir(workdir); err != nil {
		return nil, err
	}

	return sandbox, nil
}

// FetchSandbox creates a sandbox configured for fetching sources.
func FetchSandbox(workdir string) (*Sandbox, error) {
	profile := ProfileFetch.Clone()

	sandbox, err := New(profile)
	if err != nil {
		return nil, err
	}

	if err := sandbox.SetWorkdir(workdir); err != nil {
		return nil, err
	}

	return sandbox, nil
}
