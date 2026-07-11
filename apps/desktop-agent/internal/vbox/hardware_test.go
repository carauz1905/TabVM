package vbox

import (
	"context"
	"runtime"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

const hardwareTestID = "11111111-1111-1111-1111-111111111111"

func hardwareTestRunner(t *testing.T, path, vmState string, extra map[string]runner.Result) *fakeRunner {
	t.Helper()
	results := map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
		path + " showvminfo " + hardwareTestID + " --machinereadable": {ExitCode: 0, StandardOutput: "cpus=\"2\"\nmemory=\"2048\"\nVMState=\"" + vmState + "\""},
		path + " list hostinfo": {ExitCode: 0, StandardOutput: "Processor count: 8\nMemory size: 16384 MByte\n"},
	}
	for key, value := range extra {
		results[key] = value
	}
	return &fakeRunner{results: results}
}

func TestVmHardware_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})

	_, err := svc.VmHardware(context.Background(), "bogus id")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestVmHardware_ReturnsConfiguredValuesAndHostLimits(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	run := hardwareTestRunner(t, path, "poweroff", nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.VmHardware(context.Background(), hardwareTestID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CPUs != 2 || resp.MemoryMB != 2048 {
		t.Fatalf("unexpected configured hardware: %+v", resp)
	}
	if resp.HostCPUs != 8 || resp.HostMemoryMB != 16384 {
		t.Fatalf("unexpected host limits: %+v", resp)
	}
	if !resp.Editable {
		t.Fatalf("expected a powered-off VM to be editable: %+v", resp)
	}
}

func TestVmHardware_LiveVmIsNotEditable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	run := hardwareTestRunner(t, path, "running", nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.VmHardware(context.Background(), hardwareTestID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Editable {
		t.Fatalf("expected a running VM to be read-only: %+v", resp)
	}
}

func TestSetVmHardware_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})

	_, err := svc.SetVmHardware(context.Background(), "bogus id", 2, 2048)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestSetVmHardware_RejectsOutOfRangeValues(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})

	if _, err := svc.SetVmHardware(context.Background(), hardwareTestID, 0, 2048); err == nil {
		t.Fatalf("expected error for zero CPUs")
	}
	if _, err := svc.SetVmHardware(context.Background(), hardwareTestID, 2, 64); err == nil {
		t.Fatalf("expected error for memory below 128 MB")
	}
}

func TestSetVmHardware_RefusesLiveVM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	run := hardwareTestRunner(t, path, "running", nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.SetVmHardware(context.Background(), hardwareTestID, 4, 4096)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a running VM, got %v", err)
	}
}

func TestSetVmHardware_RejectsValuesAboveHostLimits(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	run := hardwareTestRunner(t, path, "poweroff", nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.SetVmHardware(context.Background(), hardwareTestID, 16, 4096); err == nil {
		t.Fatalf("expected error for CPUs above host count")
	}
	if _, err := svc.SetVmHardware(context.Background(), hardwareTestID, 4, 32768); err == nil {
		t.Fatalf("expected error for memory above host size")
	}
}

func TestSetVmHardware_AppliesChangeOnStoppedVM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	run := hardwareTestRunner(t, path, "poweroff", map[string]runner.Result{
		path + " modifyvm " + hardwareTestID + " --cpus 4 --memory 4096": {ExitCode: 0},
	})

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.SetVmHardware(context.Background(), hardwareTestID, 4, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.VMID != hardwareTestID {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestSetVmHardware_ReturnsExecutionErrorOnFailure(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	// modifyvm result intentionally absent -> fakeRunner returns ExitCode 1.
	run := hardwareTestRunner(t, path, "poweroff", nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.SetVmHardware(context.Background(), hardwareTestID, 4, 4096)
	if _, ok := err.(*ExecutionError); !ok {
		t.Fatalf("expected ExecutionError, got %v", err)
	}
}
