package vbox

import (
	"context"
	"reflect"
	"runtime"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

func TestParseGuestPropertyValue(t *testing.T) {
	cases := []struct {
		output      string
		wantValue   string
		wantPresent bool
	}{
		{"Value: 7.0.14\n", "7.0.14", true},
		{"Value: 7.2.12 r174389\n", "7.2.12 r174389", true},
		{"No value set!\n", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		value, present := parseGuestPropertyValue(c.output)
		if value != c.wantValue || present != c.wantPresent {
			t.Errorf("parseGuestPropertyValue(%q) = (%q, %v), want (%q, %v)",
				c.output, value, present, c.wantValue, c.wantPresent)
		}
	}
}

func TestParseOpticalSlots(t *testing.T) {
	output := `VMState="running"
storagecontrollername0="IDE"
storagecontrollertype0="PIIX4"
storagecontrollername1="SATA Controller"
storagecontrollertype1="IntelAhci"
"IDE-0-0"="/vms/lab/disk.vdi"
"IDE-0-0-ImageUUID"="aaaa"
"IDE-1-0"="/vms/lab/os.iso"
"IDE-1-0-ImageUUID"="bbbb"
"IDE-1-1"="none"
"SATA Controller-0-0"="emptydrive"`

	got := parseOpticalSlots(output)
	want := []opticalSlot{
		{controller: "IDE", port: 1, device: 0},                       // ISO: swap only as last resort
		{controller: "IDE", port: 1, device: 1, free: true},           // free bay on an optical-capable controller
		{controller: "SATA Controller", port: 0, device: 0, empty: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseOpticalSlots() = %+v, want %+v", got, want)
	}
}

func TestParseOpticalSlots_ExcludesFreeBayOnNvme(t *testing.T) {
	output := `storagecontrollername0="NVMe"
storagecontrollertype0="NVMe"
"NVMe-0-0"="none"`

	if got := parseOpticalSlots(output); len(got) != 0 {
		t.Fatalf("expected no optical slots on an NVMe controller, got %+v", got)
	}
}

func TestChooseOpticalTarget_PrefersEmpty(t *testing.T) {
	slots := []opticalSlot{
		{controller: "IDE", port: 1, device: 0, free: true},
		{controller: "SATA Controller", port: 0, device: 0, empty: true},
	}
	target, ok := chooseOpticalTarget(slots)
	if !ok {
		t.Fatal("expected a target")
	}
	if target.controller != "SATA Controller" || !target.empty {
		t.Fatalf("expected the empty SATA slot, got %+v", target)
	}
}

func TestChooseOpticalTarget_PrefersFreeBayOverSwappingIso(t *testing.T) {
	slots := []opticalSlot{
		{controller: "IDE", port: 0, device: 0},             // holds an ISO
		{controller: "IDE", port: 1, device: 0, free: true}, // free bay
	}
	target, ok := chooseOpticalTarget(slots)
	if !ok || !target.free || target.port != 1 {
		t.Fatalf("expected the free bay, got %+v ok=%v", target, ok)
	}
}

func TestChooseOpticalTarget_FallsBackToFirst(t *testing.T) {
	slots := []opticalSlot{{controller: "IDE", port: 1, device: 0}}
	target, ok := chooseOpticalTarget(slots)
	if !ok || target.controller != "IDE" {
		t.Fatalf("expected the IDE slot, got %+v ok=%v", target, ok)
	}
}

func TestChooseOpticalTarget_NoneWhenEmpty(t *testing.T) {
	if _, ok := chooseOpticalTarget(nil); ok {
		t.Fatal("expected no target for an empty slot list")
	}
}

func TestInsertGuestAdditionsArgs(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	got := insertGuestAdditionsArgs(id, opticalSlot{controller: "SATA Controller", port: 0, device: 0})
	want := []string{
		"storageattach", id,
		"--storagectl", "SATA Controller",
		"--port", "0", "--device", "0",
		"--type", "dvddrive", "--medium", "additions",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("insertGuestAdditionsArgs() = %v, want %v", got, want)
	}
}

func TestGuestAdditionsStatus_Installed(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version":                              {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="running"`},
		path + " guestproperty get " + id + " /VirtualBox/GuestAdd/Version": {ExitCode: 0, StandardOutput: "Value: 7.0.14\n"},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.GuestAdditionsStatus(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Installed || resp.Version != "7.0.14" || resp.Status != "installed" {
		t.Fatalf("unexpected status: %+v", resp)
	}
}

func TestGuestAdditionsStatus_NotDetected(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version":                              {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="running"`},
		path + " guestproperty get " + id + " /VirtualBox/GuestAdd/Version": {ExitCode: 0, StandardOutput: "No value set!\n"},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.GuestAdditionsStatus(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Installed || resp.Status != "not-detected" {
		t.Fatalf("expected not-detected, got %+v", resp)
	}
}

func TestGuestAdditionsStatus_UnknownWhenStopped(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version":                              {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.GuestAdditionsStatus(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Installed || resp.Status != "unknown" {
		t.Fatalf("expected unknown for a stopped VM, got %+v", resp)
	}
}

func TestInstallGuestAdditions_MountsIso(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="running"
storagecontrollername0="IDE"
"IDE-1-0"="emptydrive"`},
		path + " storageattach " + id + " --storagectl IDE --port 1 --device 0 --type dvddrive --medium additions": {ExitCode: 0},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.InstallGuestAdditions(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.Controller != "IDE" || resp.Port != 1 || resp.Device != 0 {
		t.Fatalf("unexpected install response: %+v", resp)
	}
}

func TestInstallGuestAdditions_NoOpticalDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="running"
storagecontrollername0="IDE"
"IDE-0-0"="/vms/lab/disk.vdi"`},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.InstallGuestAdditions(context.Background(), id)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError when no optical drive exists, got %T: %v", err, err)
	}
}
