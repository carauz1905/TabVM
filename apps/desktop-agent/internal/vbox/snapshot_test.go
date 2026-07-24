package vbox

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/tabvm/desktop-agent/internal/runner"
)

// erringRunner mimics the REAL runner semantics that fakeRunner/queuedRunner do
// not: exec.Cmd.Run returns a non-nil *exec.ExitError for a non-zero exit, so
// RunContext returns (populated Result, non-nil error). ListSnapshots' empty-
// list special case must survive that pairing.
type erringRunner struct {
	results map[string]struct {
		res runner.Result
		err error
	}
}

func (e *erringRunner) RunContext(ctx context.Context, name string, args []string, timeout time.Duration) (runner.Result, error) {
	key := name + " " + joinArgs(args)
	if entry, ok := e.results[key]; ok {
		return entry.res, entry.err
	}
	return runner.Result{ExitCode: 1, StandardError: "unexpected command: " + key}, errors.New("exit status 1")
}

// TestListSnapshots_NoSnapshotsIsEmptyListNotError reproduces the real-world
// failure: `VBoxManage snapshot <id> list` on a VM without snapshots prints
// "This machine does not have any snapshots" and exits 1, which the real
// runner surfaces as (Result, exit-status-1 error). That must be an empty
// list, not a 502.
func TestListSnapshots_NoSnapshotsIsEmptyListNotError(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	id := "11111111-1111-1111-1111-111111111111"
	run := &erringRunner{
		results: map[string]struct {
			res runner.Result
			err error
		}{
			path + " --version": {res: runner.Result{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
			path + " snapshot " + id + " list --machinereadable": {
				res: runner.Result{
					ExitCode:       1,
					StandardOutput: "This machine does not have any snapshots\n",
				},
				err: errors.New("exit status 1"),
			},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	res, err := svc.ListSnapshots(context.Background(), id)
	if err != nil {
		t.Fatalf("expected empty snapshot list, got error: %v", err)
	}
	if res.Snapshots == nil || len(res.Snapshots) != 0 {
		t.Fatalf("expected empty non-nil snapshot slice, got %#v", res.Snapshots)
	}
}

// TestListSnapshots_RealFailureStillErrors ensures the fix does not swallow a
// genuine failure: a non-zero exit WITHOUT the no-snapshots marker must still
// surface as an ExecutionError carrying the exit code.
func TestListSnapshots_RealFailureStillErrors(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	id := "11111111-1111-1111-1111-111111111111"
	run := &erringRunner{
		results: map[string]struct {
			res runner.Result
			err error
		}{
			path + " --version": {res: runner.Result{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
			path + " snapshot " + id + " list --machinereadable": {
				res: runner.Result{ExitCode: 1, StandardError: "VBoxManage.exe: error: Could not find a registered machine"},
				err: errors.New("exit status 1"),
			},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.ListSnapshots(context.Background(), id)
	if err == nil {
		t.Fatal("expected an error for a genuine snapshot list failure, got nil")
	}
	var execErr *ExecutionError
	if !errors.As(err, &execErr) {
		t.Fatalf("expected an *ExecutionError, got %T: %v", err, err)
	}
	if execErr.ExitCode != 1 {
		t.Fatalf("expected exit code 1 in the ExecutionError, got %d", execErr.ExitCode)
	}
}
