package vbox

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

func TestCloneVmArgs_FullVsLinked(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"

	full := cloneVmArgs(id, "lab-clone", false, "")
	if strings.Join(full, " ") != "clonevm "+id+" --name lab-clone --register" {
		t.Fatalf("unexpected full clone args: %v", full)
	}

	snap := "33333333-3333-3333-3333-333333333333"
	linked := cloneVmArgs(id, "lab-clone", true, snap)
	if strings.Join(linked, " ") != "clonevm "+id+" --name lab-clone --register --options link --snapshot "+snap {
		t.Fatalf("unexpected linked clone args: %v", linked)
	}
}

func TestCloneVM_RejectsInvalidSourceID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.CloneVM(context.Background(), "not-a-vm; rm -rf /", "lab-clone", false)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestCloneVM_RejectsInvalidName(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	// A valid source id but an invalid name must fail before any VBoxManage call,
	// so this passes even without a discovered VBoxManage.
	_, err := svc.CloneVM(context.Background(), "11111111-1111-1111-1111-111111111111", "bad/name", false)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a bad name, got %v", err)
	}
}

func TestCloneVM_RefusesRunningSource(t *testing.T) {
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
	_, err := svc.CloneVM(context.Background(), id, "lab-clone", false)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a running source, got %v", err)
	}
}

func TestCloneVM_RejectsLinkedWithoutSnapshot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &recordingRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			// A VM with no snapshots makes `snapshot list` exit non-zero with this text.
			path + " snapshot " + id + " list --machinereadable": {ExitCode: 1, StandardError: "This machine does not have any snapshots"},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.CloneVM(context.Background(), id, "lab-clone", true)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a linked clone without a snapshot, got %v", err)
	}
	// It must not have attempted the clone.
	if strings.Contains(strings.Join(run.calls, "\n"), "clonevm") {
		t.Fatalf("clonevm must not run when the linked-clone precondition fails; calls:\n%s", strings.Join(run.calls, "\n"))
	}
}

func TestCloneVM_FullHappyPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	newUUID := "22222222-2222-2222-2222-222222222222"
	path := createTempExecutable(t)
	run := &recordingRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable":        {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			path + " clonevm " + id + " --name lab-clone --register": {ExitCode: 0},
			path + " showvminfo lab-clone --machinereadable":         {ExitCode: 0, StandardOutput: `UUID="` + newUUID + `"`},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.CloneVM(context.Background(), id, "lab-clone", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.VMID != newUUID || resp.Name != "lab-clone" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	joined := strings.Join(run.calls, "\n")
	if !strings.Contains(joined, "clonevm "+id+" --name lab-clone --register") {
		t.Fatalf("expected the clonevm command; calls:\n%s", joined)
	}
	// A full clone must never request a linked clone.
	if strings.Contains(joined, "--options link") {
		t.Fatalf("full clone must not pass --options link; calls:\n%s", joined)
	}
}

// VirtualBox copies the source's COM1 pipe path into the clone verbatim, so the
// clone would dial a pipe nobody creates -- and if both machines ran at once
// they would fight over one pipe name. The clone must be re-pointed at its own.
func TestCloneVM_RepointsInheritedSerialPipeAtTheClone(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	newUUID := "22222222-2222-2222-2222-222222222222"
	path := createTempExecutable(t)
	inherited := `uart1="0x03f8,4"` + "\n" + `uartmode1="server,` + SerialPipeName(id) + `"` + "\n"
	run := &recordingRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable":        {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			path + " clonevm " + id + " --name lab-clone --register": {ExitCode: 0},
			path + " showvminfo lab-clone --machinereadable":         {ExitCode: 0, StandardOutput: `UUID="` + newUUID + `"`},
			path + " showvminfo " + newUUID + " --machinereadable":   {ExitCode: 0, StandardOutput: inherited},
			path + " modifyvm " + newUUID + " --uart1 0x3F8 4 --uartmode1 server " + SerialPipeName(newUUID): {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.CloneVM(context.Background(), id, "lab-clone", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	joined := strings.Join(run.calls, "\n")
	if !strings.Contains(joined, "modifyvm "+newUUID+" --uart1 0x3F8 4 --uartmode1 server "+SerialPipeName(newUUID)) {
		t.Fatalf("expected the clone's serial port re-pointed at its own pipe; calls:\n%s", joined)
	}
	// The source's pipe must not survive into any command aimed at the clone.
	if strings.Contains(joined, "modifyvm "+newUUID+" --uart1 0x3F8 4 --uartmode1 server "+SerialPipeName(id)) {
		t.Fatalf("the clone must not keep the source's pipe; calls:\n%s", joined)
	}
}

func TestCloneVM_LeavesSerialAloneWhenSourceHadNone(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	newUUID := "22222222-2222-2222-2222-222222222222"
	path := createTempExecutable(t)
	run := &recordingRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable":        {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			path + " clonevm " + id + " --name lab-clone --register": {ExitCode: 0},
			path + " showvminfo lab-clone --machinereadable":         {ExitCode: 0, StandardOutput: `UUID="` + newUUID + `"`},
			path + " showvminfo " + newUUID + " --machinereadable":   {ExitCode: 0, StandardOutput: `uart1="off"` + "\n"},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.CloneVM(context.Background(), id, "lab-clone", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if joined := strings.Join(run.calls, "\n"); strings.Contains(joined, "--uart1") {
		t.Fatalf("a clone of a VM with no serial port must not gain one; calls:\n%s", joined)
	}
}

func TestCloneVM_LinkedHappyPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	snapUUID := "33333333-3333-3333-3333-333333333333"
	newUUID := "22222222-2222-2222-2222-222222222222"
	path := createTempExecutable(t)
	run := &recordingRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			path + " snapshot " + id + " list --machinereadable": {ExitCode: 0, StandardOutput: `SnapshotName="Base"
SnapshotUUID="` + snapUUID + `"
CurrentSnapshotUUID="` + snapUUID + `"`},
			path + " clonevm " + id + " --name lab-clone --register --options link --snapshot " + snapUUID: {ExitCode: 0},
			path + " showvminfo lab-clone --machinereadable":                                               {ExitCode: 0, StandardOutput: `UUID="` + newUUID + `"`},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.CloneVM(context.Background(), id, "lab-clone", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.VMID != newUUID {
		t.Fatalf("unexpected response: %+v", resp)
	}
	joined := strings.Join(run.calls, "\n")
	if !strings.Contains(joined, "clonevm "+id+" --name lab-clone --register --options link --snapshot "+snapUUID) {
		t.Fatalf("expected a linked clonevm command with --snapshot; calls:\n%s", joined)
	}
}

func TestCloneVM_ReturnsExecutionErrorOnCloneFailure(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
			// clonevm intentionally absent -> fakeRunner returns ExitCode 1.
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.CloneVM(context.Background(), id, "lab-clone", false)
	if _, ok := err.(*ExecutionError); !ok {
		t.Fatalf("expected ExecutionError when clonevm fails, got %v", err)
	}
}

func TestValidateClone_RefusesRunningSource(t *testing.T) {
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
	if err := svc.ValidateClone(context.Background(), id, "lab-clone", false); err == nil {
		t.Fatal("expected ValidateClone to refuse a running source")
	} else if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestValidateClone_AcceptsStoppedSource(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if err := svc.ValidateClone(context.Background(), id, "lab-clone", false); err != nil {
		t.Fatalf("expected a stopped source to validate, got %v", err)
	}
}
