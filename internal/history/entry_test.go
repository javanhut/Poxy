package history

import (
	"testing"
	"time"
)

func TestOperation(t *testing.T) {
	tests := []struct {
		op       Operation
		expected string
	}{
		{OpInstall, "install"},
		{OpUninstall, "uninstall"},
		{OpUpdate, "update"},
		{OpUpgrade, "upgrade"},
		{OpClean, "clean"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.op) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.op)
			}
		})
	}
}

func TestNewEntry(t *testing.T) {
	entry := NewEntry(OpInstall, "apt", []string{"vim", "git"})

	if entry.ID == "" {
		t.Error("entry ID should not be empty")
	}
	if entry.Operation != OpInstall {
		t.Errorf("expected Operation Install, got %s", entry.Operation)
	}
	if entry.Source != "apt" {
		t.Errorf("expected Source 'apt', got '%s'", entry.Source)
	}
	if len(entry.Packages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(entry.Packages))
	}
	if entry.Success {
		t.Error("new entry should have Success = false")
	}
	if entry.Timestamp.IsZero() {
		t.Error("entry timestamp should be set")
	}
}

func TestEntryMarkSuccess(t *testing.T) {
	entry := NewEntry(OpInstall, "apt", []string{"vim"})
	entry.MarkSuccess()

	if !entry.Success {
		t.Error("MarkSuccess() should set Success to true")
	}
}

func TestEntryMarkFailed(t *testing.T) {
	entry := NewEntry(OpInstall, "apt", []string{"vim"})

	// Test with error
	testErr := &testError{msg: "test error"}
	entry.MarkFailed(testErr)

	if entry.Success {
		t.Error("MarkFailed() should set Success to false")
	}
	if entry.Error != "test error" {
		t.Errorf("MarkFailed() should set Error message, got '%s'", entry.Error)
	}

	// Test with nil error
	entry2 := NewEntry(OpInstall, "apt", []string{"vim"})
	entry2.MarkFailed(nil)
	if entry2.Error != "" {
		t.Error("MarkFailed(nil) should not set Error")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestIsReversible(t *testing.T) {
	tests := []struct {
		op       Operation
		expected bool
	}{
		{OpInstall, true},
		{OpUninstall, true},
		{OpUpdate, false},
		{OpUpgrade, false},
		{OpClean, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			result := isReversible(tt.op)
			if result != tt.expected {
				t.Errorf("isReversible(%s) = %v, want %v", tt.op, result, tt.expected)
			}
		})
	}
}

func TestReverseOperation(t *testing.T) {
	tests := []struct {
		op       Operation
		expected Operation
	}{
		{OpInstall, OpUninstall},
		{OpUninstall, OpInstall},
		{OpUpdate, ""},
		{OpUpgrade, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			result := reverseOperation(tt.op)
			if result != tt.expected {
				t.Errorf("reverseOperation(%s) = %s, want %s", tt.op, result, tt.expected)
			}
		})
	}
}

func TestCanRollback(t *testing.T) {
	// Successful install with packages - can rollback
	entry1 := NewEntry(OpInstall, "apt", []string{"vim"})
	entry1.MarkSuccess()
	if !entry1.CanRollback() {
		t.Error("successful install should be rollbackable")
	}

	// Failed install - cannot rollback
	entry2 := NewEntry(OpInstall, "apt", []string{"vim"})
	if entry2.CanRollback() {
		t.Error("failed install should not be rollbackable")
	}

	// Successful update - cannot rollback (not reversible)
	entry3 := NewEntry(OpUpdate, "apt", nil)
	entry3.MarkSuccess()
	if entry3.CanRollback() {
		t.Error("update should not be rollbackable")
	}

	// Successful install with no packages - cannot rollback
	entry4 := NewEntry(OpInstall, "apt", []string{})
	entry4.MarkSuccess()
	if entry4.CanRollback() {
		t.Error("install with no packages should not be rollbackable")
	}
}

func TestFormatTime(t *testing.T) {
	entry := &Entry{
		Timestamp: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
	}

	result := entry.FormatTime()
	expected := "2024-01-15 10:30:45"

	if result != expected {
		t.Errorf("FormatTime() = %s, want %s", result, expected)
	}
}

func TestSummary(t *testing.T) {
	// Entry with packages
	entry1 := NewEntry(OpInstall, "apt", []string{"vim"})
	entry1.MarkSuccess()
	summary1 := entry1.Summary()

	if summary1 == "" {
		t.Error("Summary() should not be empty")
	}
	if len(summary1) < 10 {
		t.Error("Summary() seems too short")
	}

	// Entry without packages
	entry2 := NewEntry(OpUpdate, "apt", nil)
	entry2.MarkSuccess()
	summary2 := entry2.Summary()

	if summary2 == "" {
		t.Error("Summary() for update should not be empty")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	time.Sleep(1 * time.Millisecond)
	id2 := generateID()

	if id1 == "" {
		t.Error("generateID() should not return empty string")
	}
	if id1 == id2 {
		t.Error("generateID() should return unique IDs")
	}
}
