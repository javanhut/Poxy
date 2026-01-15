// Package history provides operation history tracking with BoltDB.
package history

import (
	"time"
)

// Operation represents the type of package operation.
type Operation string

const (
	OpInstall   Operation = "install"
	OpUninstall Operation = "uninstall"
	OpUpdate    Operation = "update"
	OpUpgrade   Operation = "upgrade"
	OpClean     Operation = "clean"
)

// Entry represents a single operation in the history.
type Entry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Operation Operation `json:"operation"`
	Source    string    `json:"source"`   // Package manager used
	Packages  []string  `json:"packages"` // Packages affected
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`

	// Rollback support
	Reversible bool      `json:"reversible"`
	ReverseOp  Operation `json:"reverse_op,omitempty"`
}

// NewEntry creates a new history entry.
func NewEntry(op Operation, source string, packages []string) *Entry {
	return &Entry{
		ID:         generateID(),
		Timestamp:  time.Now(),
		Operation:  op,
		Source:     source,
		Packages:   packages,
		Success:    false, // Will be updated after operation completes
		Reversible: isReversible(op),
		ReverseOp:  reverseOperation(op),
	}
}

// MarkSuccess marks the entry as successful.
func (e *Entry) MarkSuccess() {
	e.Success = true
}

// MarkFailed marks the entry as failed with an error message.
func (e *Entry) MarkFailed(err error) {
	e.Success = false
	if err != nil {
		e.Error = err.Error()
	}
}

// generateID generates a unique ID for the entry.
func generateID() string {
	return time.Now().Format("20060102150405.000000")
}

// isReversible returns whether an operation can be reversed.
func isReversible(op Operation) bool {
	switch op {
	case OpInstall, OpUninstall:
		return true
	case OpUpdate, OpUpgrade, OpClean:
		return false
	}
	return false
}

// reverseOperation returns the operation that would reverse this one.
func reverseOperation(op Operation) Operation {
	switch op {
	case OpInstall:
		return OpUninstall
	case OpUninstall:
		return OpInstall
	}
	return ""
}

// CanRollback returns true if this operation can be rolled back.
func (e *Entry) CanRollback() bool {
	return e.Reversible && e.Success && len(e.Packages) > 0
}

// FormatTime returns a human-readable timestamp.
func (e *Entry) FormatTime() string {
	return e.Timestamp.Format("2006-01-02 15:04:05")
}

// Summary returns a brief summary of the operation.
func (e *Entry) Summary() string {
	status := "success"
	if !e.Success {
		status = "failed"
	}

	pkgCount := len(e.Packages)
	if pkgCount == 0 {
		return e.FormatTime() + " " + string(e.Operation) + " (" + status + ")"
	}

	return e.FormatTime() + " " + string(e.Operation) + " " +
		e.Packages[0] + " [" + e.Source + "] (" + status + ")"
}
