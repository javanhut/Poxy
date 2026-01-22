package native

import (
	"regexp"
	"strings"
)

// PacmanErrorType represents the type of pacman error.
type PacmanErrorType int

const (
	PacmanErrorUnknown PacmanErrorType = iota
	PacmanErrorDependencyConflict
	PacmanErrorPackageNotFound
	PacmanErrorDatabaseLocked
)

// PacmanError represents a structured error from pacman.
type PacmanError struct {
	ErrorType    PacmanErrorType
	RawOutput    string
	Packages     []string // Affected packages
	OriginalErr  error
	Suggestion   string
}

// Error implements the error interface.
func (e *PacmanError) Error() string {
	if e.OriginalErr != nil {
		return e.OriginalErr.Error()
	}
	return e.RawOutput
}

// Unwrap returns the original error.
func (e *PacmanError) Unwrap() error {
	return e.OriginalErr
}

// IsDependencyConflict returns true if this is a dependency conflict error.
func (e *PacmanError) IsDependencyConflict() bool {
	return e.ErrorType == PacmanErrorDependencyConflict
}

// Regular expressions for parsing pacman errors
var (
	// Matches: "error: failed to prepare transaction (could not satisfy dependencies)"
	dependencyFailurePattern = regexp.MustCompile(`failed to prepare transaction.*could not satisfy dependencies`)

	// Matches: ":: installing pkg (1.2.3-4) breaks dependency 'pkg=1.2.3-1' required by other-pkg"
	breaksDepPattern = regexp.MustCompile(`:: installing (\S+) .* breaks dependency .* required by (\S+)`)

	// Matches: ":: pkg and other-pkg are in conflict"
	conflictPattern = regexp.MustCompile(`:: (\S+) and (\S+) are in conflict`)

	// Matches: "error: target not found: pkg"
	notFoundPattern = regexp.MustCompile(`error: target not found: (\S+)`)

	// Matches: "error: failed to init transaction (unable to lock database)"
	dbLockedPattern = regexp.MustCompile(`failed to init transaction.*unable to lock database`)
)

// ParsePacmanError parses pacman stderr output and returns a structured error.
// If the error is not a known pacman error type, it returns nil.
func ParsePacmanError(stderr string, originalErr error) *PacmanError {
	if stderr == "" && originalErr == nil {
		return nil
	}

	pacErr := &PacmanError{
		ErrorType:   PacmanErrorUnknown,
		RawOutput:   stderr,
		OriginalErr: originalErr,
	}

	// Check for dependency conflict
	if dependencyFailurePattern.MatchString(stderr) {
		pacErr.ErrorType = PacmanErrorDependencyConflict
		pacErr.Packages = extractAffectedPackages(stderr)
		pacErr.Suggestion = "Run 'poxy upgrade' to update your system first"
		return pacErr
	}

	// Check for package conflict (also a form of dependency issue)
	if matches := conflictPattern.FindAllStringSubmatch(stderr, -1); len(matches) > 0 {
		pacErr.ErrorType = PacmanErrorDependencyConflict
		pacErr.Packages = extractAffectedPackages(stderr)
		pacErr.Suggestion = "Run 'poxy upgrade' to update your system first"
		return pacErr
	}

	// Check for package not found
	if matches := notFoundPattern.FindAllStringSubmatch(stderr, -1); len(matches) > 0 {
		pacErr.ErrorType = PacmanErrorPackageNotFound
		for _, m := range matches {
			if len(m) > 1 {
				pacErr.Packages = append(pacErr.Packages, m[1])
			}
		}
		return pacErr
	}

	// Check for database lock
	if dbLockedPattern.MatchString(stderr) {
		pacErr.ErrorType = PacmanErrorDatabaseLocked
		pacErr.Suggestion = "Another package manager may be running. Wait for it to finish or remove /var/lib/pacman/db.lck"
		return pacErr
	}

	// Unknown error type - return nil to indicate no special handling needed
	return nil
}

// extractAffectedPackages extracts package names from dependency conflict messages.
func extractAffectedPackages(stderr string) []string {
	seen := make(map[string]bool)
	var packages []string

	// Extract from "breaks dependency" messages
	if matches := breaksDepPattern.FindAllStringSubmatch(stderr, -1); len(matches) > 0 {
		for _, m := range matches {
			if len(m) > 1 && !seen[m[1]] {
				packages = append(packages, m[1])
				seen[m[1]] = true
			}
			if len(m) > 2 && !seen[m[2]] {
				packages = append(packages, m[2])
				seen[m[2]] = true
			}
		}
	}

	// Extract from conflict messages
	if matches := conflictPattern.FindAllStringSubmatch(stderr, -1); len(matches) > 0 {
		for _, m := range matches {
			if len(m) > 1 && !seen[m[1]] {
				packages = append(packages, m[1])
				seen[m[1]] = true
			}
			if len(m) > 2 && !seen[m[2]] {
				packages = append(packages, m[2])
				seen[m[2]] = true
			}
		}
	}

	return packages
}

// IsPacmanDependencyConflict checks if an error is a pacman dependency conflict.
func IsPacmanDependencyConflict(err error) (*PacmanError, bool) {
	if pacErr, ok := err.(*PacmanError); ok && pacErr.IsDependencyConflict() {
		return pacErr, true
	}
	return nil, false
}

// FormatDependencyConflictMessage returns a user-friendly message for dependency conflicts.
func FormatDependencyConflictMessage(pacErr *PacmanError) string {
	var sb strings.Builder
	sb.WriteString("Dependency conflict detected!\n")
	sb.WriteString("  This usually happens when packages in your system are out of date.\n")
	sb.WriteString("-> Suggestion: ")
	sb.WriteString(pacErr.Suggestion)
	sb.WriteString("\n")

	if len(pacErr.Packages) > 0 {
		sb.WriteString("  Affected packages:\n")
		for _, pkg := range pacErr.Packages {
			sb.WriteString("    - ")
			sb.WriteString(pkg)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
