// Package native implements native system package managers.
package native

import (
	"os/exec"

	"poxy/internal/executor"
	"poxy/pkg/manager"
)

// BaseManager provides common functionality for all native package managers.
type BaseManager struct {
	name        string
	displayName string
	binary      string
	managerType manager.ManagerType
	needsSudo   bool
	exec        *executor.Executor
}

// NewBaseManager creates a new BaseManager with the given parameters.
func NewBaseManager(name, displayName, binary string, needsSudo bool) *BaseManager {
	return &BaseManager{
		name:        name,
		displayName: displayName,
		binary:      binary,
		managerType: manager.TypeNative,
		needsSudo:   needsSudo,
		exec:        executor.New(false, false),
	}
}

// Name returns the short identifier for this manager.
func (b *BaseManager) Name() string {
	return b.name
}

// DisplayName returns the human-readable name.
func (b *BaseManager) DisplayName() string {
	return b.displayName
}

// Type returns the manager type.
func (b *BaseManager) Type() manager.ManagerType {
	return b.managerType
}

// IsAvailable returns true if this package manager is installed.
func (b *BaseManager) IsAvailable() bool {
	_, err := exec.LookPath(b.binary)
	return err == nil
}

// NeedsSudo returns true if this manager requires root privileges.
func (b *BaseManager) NeedsSudo() bool {
	return b.needsSudo
}

// Binary returns the primary binary name for this manager.
func (b *BaseManager) Binary() string {
	return b.binary
}

// SetBinary changes the binary to use (e.g., switching from apt to nala).
func (b *BaseManager) SetBinary(binary string) {
	b.binary = binary
}

// Executor returns the executor instance.
func (b *BaseManager) Executor() *executor.Executor {
	return b.exec
}

// SetExecutor sets the executor instance.
func (b *BaseManager) SetExecutor(exec *executor.Executor) {
	b.exec = exec
}

// SetDryRun enables or disables dry-run mode.
func (b *BaseManager) SetDryRun(dryRun bool) {
	b.exec.SetDryRun(dryRun)
}

// SetVerbose enables or disables verbose mode.
func (b *BaseManager) SetVerbose(verbose bool) {
	b.exec.SetVerbose(verbose)
}
