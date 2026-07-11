package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Result captures the outcome of running an external process.
type Result struct {
	ExitCode       int
	StandardOutput string
	StandardError  string
}

// Runner executes external processes with stdout/stderr capture,
// exit code handling, timeout, and cancellation support.
type Runner struct{}

// NewRunner creates a new process runner.
func NewRunner() *Runner {
	return &Runner{}
}

// Run executes the given command with arguments and waits for completion.
// If timeout is zero, a default of 30 seconds is used.
// The caller's request context is not propagated; use RunContext for cancellation support.
func (r *Runner) Run(name string, args []string, timeout time.Duration) (Result, error) {
	return r.RunContext(context.Background(), name, args, timeout)
}

// RunContext executes the given command with arguments and waits for completion.
// It respects the provided context for cancellation and uses timeout as an upper bound.
// If timeout is zero, a default of 30 seconds is used.
func (r *Runner) RunContext(ctx context.Context, name string, args []string, timeout time.Duration) (Result, error) {
	if name == "" {
		return Result{}, fmt.Errorf("command name is required")
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = newSysProcAttr()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		ExitCode:       -1,
		StandardOutput: stdout.String(),
		StandardError:  stderr.String(),
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("process %q timed out after %s", name, timeout)
	}

	return result, err
}
