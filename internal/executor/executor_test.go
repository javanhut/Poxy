package executor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	exec := New(false, false)
	if exec == nil {
		t.Fatal("New() returned nil")
	}
}

func TestSetDryRun(t *testing.T) {
	exec := New(false, false)
	exec.SetDryRun(true)
	// No direct way to check, but should not panic
}

func TestSetVerbose(t *testing.T) {
	exec := New(false, false)
	exec.SetVerbose(true)
	// No direct way to check, but should not panic
}

func TestOutput(t *testing.T) {
	exec := New(false, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run a simple command
	output, err := exec.Output(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("Output() error: %v", err)
	}

	if !strings.Contains(output, "hello") {
		t.Errorf("Output() = %s, want to contain 'hello'", output)
	}
}

func TestOutputDryRun(t *testing.T) {
	exec := New(true, false) // dry-run mode
	ctx := context.Background()

	// In dry-run mode, should return empty string and no error
	output, err := exec.Output(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("Output() in dry-run mode error: %v", err)
	}

	if output != "" {
		t.Errorf("Output() in dry-run mode should be empty, got: %s", output)
	}
}

func TestRun(t *testing.T) {
	exec := New(false, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run a simple command
	err := exec.Run(ctx, "true")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestRunFailing(t *testing.T) {
	exec := New(false, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run a failing command
	err := exec.Run(ctx, "false")
	if err == nil {
		t.Error("Run() should return error for failing command")
	}
}

func TestRunDryRun(t *testing.T) {
	exec := New(true, false) // dry-run mode
	ctx := context.Background()

	// In dry-run mode, should return no error even for commands that would fail
	err := exec.Run(ctx, "false")
	if err != nil {
		t.Errorf("Run() in dry-run mode should not error: %v", err)
	}
}

func TestOutputCombined(t *testing.T) {
	exec := New(false, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := exec.OutputCombined(ctx, "echo", "test")
	if err != nil {
		t.Fatalf("OutputCombined() error: %v", err)
	}

	if !strings.Contains(output, "test") {
		t.Errorf("OutputCombined() = %s, want to contain 'test'", output)
	}
}

func TestContextCancellation(t *testing.T) {
	exec := New(false, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should fail due to cancelled context
	_, err := exec.Output(ctx, "sleep", "10")
	if err == nil {
		t.Error("Output() should error with cancelled context")
	}
}
