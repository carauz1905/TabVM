package runner

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func skipIfNotWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("process runner integration tests require Windows")
	}
}

func TestRun_ReturnsOutputAndZeroExitCode_WhenCommandSucceeds(t *testing.T) {
	skipIfNotWindows(t)
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	r := NewRunner()
	result, err := r.Run("cmd.exe", []string{"/c", "echo hello"}, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.StandardOutput, "hello") {
		t.Fatalf("expected stdout to contain 'hello', got %q", result.StandardOutput)
	}
}

func TestRun_ReturnsNonZeroExitCode_WhenCommandFails(t *testing.T) {
	skipIfNotWindows(t)
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	r := NewRunner()
	result, err := r.Run("cmd.exe", []string{"/c", "exit 7"}, 5*time.Second)
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if result.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", result.ExitCode)
	}
}

func TestRun_ReturnsTimeoutError_WhenCommandExceedsTimeout(t *testing.T) {
	skipIfNotWindows(t)
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	r := NewRunner()
	_, err := r.Run("cmd.exe", []string{"/c", "ping -n 10 127.0.0.1 >nul"}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout message, got %q", err.Error())
	}
}

func TestRun_RequiresCommandName(t *testing.T) {
	r := NewRunner()
	_, err := r.Run("", nil, 5*time.Second)
	if err == nil {
		t.Fatal("expected error for empty command name")
	}
}

func TestRun_ReturnsNegativeExitCode_WhenCommandNotFound(t *testing.T) {
	r := NewRunner()
	result, err := r.Run("this-command-does-not-exist-xyz", nil, 5*time.Second)
	if err == nil {
		t.Fatal("expected error when command is not found")
	}
	if result.ExitCode != -1 {
		t.Fatalf("expected exit code -1 for missing command, got %d", result.ExitCode)
	}
}

func TestRunContext_HonoursCancellation(t *testing.T) {
	skipIfNotWindows(t)
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	r := NewRunner()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel before starting the long-running command.
	cancel()

	_, err := r.RunContext(ctx, "cmd.exe", []string{"/c", "ping -n 10 127.0.0.1 >nul"}, 5*time.Second)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled message, got %q", err.Error())
	}
}

func TestRunContext_HonoursTimeout(t *testing.T) {
	skipIfNotWindows(t)
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	r := NewRunner()
	ctx := context.Background()
	_, err := r.RunContext(ctx, "cmd.exe", []string{"/c", "ping -n 10 127.0.0.1 >nul"}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout message, got %q", err.Error())
	}
}
