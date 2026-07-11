package vbox

import (
	"context"
	"runtime"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

func TestDeleteVM_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})

	_, err := svc.DeleteVM(context.Background(), "not-a-vm; rm -rf /")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestDeleteVM_RefusesLiveVM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="running"`},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.DeleteVM(context.Background(), id)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a running VM, got %v", err)
	}
}

func TestDeleteVM_DeletesStoppedVM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			path + " unregistervm " + id + " --delete":        {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.DeleteVM(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.VMID != id {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestDeleteVM_ReturnsExecutionErrorOnFailure(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			// unregistervm intentionally absent -> fakeRunner returns ExitCode 1.
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.DeleteVM(context.Background(), id)
	if _, ok := err.(*ExecutionError); !ok {
		t.Fatalf("expected ExecutionError, got %v", err)
	}
}
