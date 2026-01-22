package native

import (
	"errors"
	"testing"
)

func TestParsePacmanError_DependencyConflict(t *testing.T) {
	stderr := `resolving dependencies...
looking for conflicting packages...
error: failed to prepare transaction (could not satisfy dependencies)
:: installing gst-plugins-base-libs (1.26.10-3) breaks dependency 'gst-plugins-base-libs=1.26.10-1' required by gst-plugins-bad-libs`

	originalErr := errors.New("exit status 1")
	pacErr := ParsePacmanError(stderr, originalErr)

	if pacErr == nil {
		t.Fatal("expected PacmanError, got nil")
	}

	if pacErr.ErrorType != PacmanErrorDependencyConflict {
		t.Errorf("expected ErrorType=%d, got %d", PacmanErrorDependencyConflict, pacErr.ErrorType)
	}

	if !pacErr.IsDependencyConflict() {
		t.Error("expected IsDependencyConflict() to return true")
	}

	if len(pacErr.Packages) != 2 {
		t.Errorf("expected 2 affected packages, got %d: %v", len(pacErr.Packages), pacErr.Packages)
	}

	if pacErr.Suggestion == "" {
		t.Error("expected non-empty Suggestion")
	}
}

func TestParsePacmanError_MultipleConflicts(t *testing.T) {
	stderr := `resolving dependencies...
looking for conflicting packages...
error: failed to prepare transaction (could not satisfy dependencies)
:: installing gst-plugins-base-libs (1.26.10-3) breaks dependency 'gst-plugins-base-libs=1.26.10-1' required by gst-plugins-bad-libs
:: installing pipewire (1.2.3-4) breaks dependency 'pipewire=1.2.3-1' required by wireplumber`

	pacErr := ParsePacmanError(stderr, errors.New("exit status 1"))

	if pacErr == nil {
		t.Fatal("expected PacmanError, got nil")
	}

	if pacErr.ErrorType != PacmanErrorDependencyConflict {
		t.Errorf("expected ErrorType=%d, got %d", PacmanErrorDependencyConflict, pacErr.ErrorType)
	}

	if len(pacErr.Packages) != 4 {
		t.Errorf("expected 4 affected packages, got %d: %v", len(pacErr.Packages), pacErr.Packages)
	}
}

func TestParsePacmanError_PackageConflict(t *testing.T) {
	stderr := `:: python-pip and python-pipx are in conflict`

	pacErr := ParsePacmanError(stderr, errors.New("exit status 1"))

	if pacErr == nil {
		t.Fatal("expected PacmanError, got nil")
	}

	if pacErr.ErrorType != PacmanErrorDependencyConflict {
		t.Errorf("expected ErrorType=%d (dependency conflict), got %d", PacmanErrorDependencyConflict, pacErr.ErrorType)
	}

	if len(pacErr.Packages) != 2 {
		t.Errorf("expected 2 affected packages, got %d: %v", len(pacErr.Packages), pacErr.Packages)
	}
}

func TestParsePacmanError_PackageNotFound(t *testing.T) {
	stderr := `error: target not found: nonexistent-package`

	pacErr := ParsePacmanError(stderr, errors.New("exit status 1"))

	if pacErr == nil {
		t.Fatal("expected PacmanError, got nil")
	}

	if pacErr.ErrorType != PacmanErrorPackageNotFound {
		t.Errorf("expected ErrorType=%d, got %d", PacmanErrorPackageNotFound, pacErr.ErrorType)
	}

	if len(pacErr.Packages) != 1 || pacErr.Packages[0] != "nonexistent-package" {
		t.Errorf("expected ['nonexistent-package'], got %v", pacErr.Packages)
	}
}

func TestParsePacmanError_MultipleNotFound(t *testing.T) {
	stderr := `error: target not found: pkg1
error: target not found: pkg2
error: target not found: pkg3`

	pacErr := ParsePacmanError(stderr, errors.New("exit status 1"))

	if pacErr == nil {
		t.Fatal("expected PacmanError, got nil")
	}

	if pacErr.ErrorType != PacmanErrorPackageNotFound {
		t.Errorf("expected ErrorType=%d, got %d", PacmanErrorPackageNotFound, pacErr.ErrorType)
	}

	if len(pacErr.Packages) != 3 {
		t.Errorf("expected 3 packages, got %d: %v", len(pacErr.Packages), pacErr.Packages)
	}
}

func TestParsePacmanError_DatabaseLocked(t *testing.T) {
	stderr := `error: failed to init transaction (unable to lock database)
error: could not lock database: File exists
  if you're sure a package manager is not already running, you can remove /var/lib/pacman/db.lck`

	pacErr := ParsePacmanError(stderr, errors.New("exit status 1"))

	if pacErr == nil {
		t.Fatal("expected PacmanError, got nil")
	}

	if pacErr.ErrorType != PacmanErrorDatabaseLocked {
		t.Errorf("expected ErrorType=%d, got %d", PacmanErrorDatabaseLocked, pacErr.ErrorType)
	}

	if pacErr.Suggestion == "" {
		t.Error("expected non-empty Suggestion for database lock")
	}
}

func TestParsePacmanError_UnknownError(t *testing.T) {
	stderr := `some random pacman output that doesn't match any pattern`

	pacErr := ParsePacmanError(stderr, errors.New("exit status 1"))

	// Unknown errors should return nil (no special handling)
	if pacErr != nil {
		t.Errorf("expected nil for unknown error, got %+v", pacErr)
	}
}

func TestParsePacmanError_EmptyInput(t *testing.T) {
	pacErr := ParsePacmanError("", nil)

	if pacErr != nil {
		t.Errorf("expected nil for empty input, got %+v", pacErr)
	}
}

func TestIsPacmanDependencyConflict(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		wantOk bool
	}{
		{
			name: "dependency conflict",
			err: &PacmanError{
				ErrorType: PacmanErrorDependencyConflict,
			},
			wantOk: true,
		},
		{
			name: "not found error",
			err: &PacmanError{
				ErrorType: PacmanErrorPackageNotFound,
			},
			wantOk: false,
		},
		{
			name:   "regular error",
			err:    errors.New("some error"),
			wantOk: false,
		},
		{
			name:   "nil error",
			err:    nil,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pacErr, ok := IsPacmanDependencyConflict(tt.err)
			if ok != tt.wantOk {
				t.Errorf("IsPacmanDependencyConflict() ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && pacErr == nil {
				t.Error("IsPacmanDependencyConflict() returned ok=true but pacErr=nil")
			}
		})
	}
}

func TestFormatDependencyConflictMessage(t *testing.T) {
	pacErr := &PacmanError{
		ErrorType:  PacmanErrorDependencyConflict,
		Packages:   []string{"pkg1", "pkg2"},
		Suggestion: "Run 'poxy upgrade' to update your system first",
	}

	msg := FormatDependencyConflictMessage(pacErr)

	if msg == "" {
		t.Error("expected non-empty message")
	}

	// Check that it contains expected elements
	checks := []string{
		"Dependency conflict detected",
		"poxy upgrade",
		"pkg1",
		"pkg2",
	}

	for _, check := range checks {
		if !contains(msg, check) {
			t.Errorf("message should contain %q, got: %s", check, msg)
		}
	}
}

func TestPacmanError_Error(t *testing.T) {
	originalErr := errors.New("original error message")
	pacErr := &PacmanError{
		ErrorType:   PacmanErrorDependencyConflict,
		RawOutput:   "raw output",
		OriginalErr: originalErr,
	}

	if pacErr.Error() != "original error message" {
		t.Errorf("Error() should return original error message, got: %s", pacErr.Error())
	}

	pacErr2 := &PacmanError{
		ErrorType: PacmanErrorDependencyConflict,
		RawOutput: "raw output only",
	}

	if pacErr2.Error() != "raw output only" {
		t.Errorf("Error() should return raw output when no original error, got: %s", pacErr2.Error())
	}
}

func TestPacmanError_Unwrap(t *testing.T) {
	originalErr := errors.New("original")
	pacErr := &PacmanError{
		OriginalErr: originalErr,
	}

	if pacErr.Unwrap() != originalErr {
		t.Error("Unwrap() should return original error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
