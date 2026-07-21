package vbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

const dvdVMID = "11111111-1111-1111-1111-111111111111"

// humanStorageWithIso is a real-shaped human-readable showvminfo storage block
// (VBoxManage 7.2.x) with a hard disk on SATA port 0 and an ISO in the optical
// drive on SATA port 1.
const humanStorageWithIso = `Storage Controllers:
#0: 'SATA', Type: IntelAhci, Instance: 0, Ports: 2 (max 30), Bootable
  Port 0, Unit 0: UUID: 846b821b-1f6c-4c75-88c8-d462b6596f87
    Location: "C:\VMs\vm\disk.vdi"
  Port 1, Unit 0: UUID: 107cb914-878a-4b6f-a2fe-8a0d93173323
    Location: "C:\ISOs\alpine.iso"
NIC 1:                       MAC: 080027C2FE52, Attachment: NAT`

// humanStorageEmptyDvd shows a disk on SATA port 0 and an empty optical drive on
// SATA port 1 ("Empty").
const humanStorageEmptyDvd = `Storage Controllers:
#0: 'SATA', Type: IntelAhci, Instance: 0, Ports: 2 (max 30), Bootable
  Port 0, Unit 0: UUID: 846b821b-1f6c-4c75-88c8-d462b6596f87
    Location: "C:\VMs\vm\disk.vdi"
  Port 1, Unit 0: Empty`

// humanStorageNoOptical shows only a hard disk and no optical drive at all.
const humanStorageNoOptical = `Storage Controllers:
#0: 'SATA', Type: IntelAhci, Instance: 0, Ports: 2 (max 30), Bootable
  Port 0, Unit 0: UUID: 846b821b-1f6c-4c75-88c8-d462b6596f87
    Location: "C:\VMs\vm\disk.vdi"`

func TestParseOpticalDrive_WithIso(t *testing.T) {
	od := parseOpticalDrive(humanStorageWithIso)
	if !od.present {
		t.Fatalf("expected an optical drive to be present")
	}
	if od.controller != "SATA" || od.port != 1 || od.device != 0 {
		t.Fatalf("unexpected location: %+v", od)
	}
	if od.medium != `C:\ISOs\alpine.iso` {
		t.Fatalf("expected the ISO path as medium, got %q", od.medium)
	}
}

func TestParseOpticalDrive_Empty(t *testing.T) {
	od := parseOpticalDrive(humanStorageEmptyDvd)
	if !od.present {
		t.Fatalf("expected an (empty) optical drive to be present")
	}
	if od.controller != "SATA" || od.port != 1 || od.device != 0 {
		t.Fatalf("unexpected location: %+v", od)
	}
	if od.medium != "" {
		t.Fatalf("expected an empty medium, got %q", od.medium)
	}
}

func TestParseOpticalDrive_NoOptical(t *testing.T) {
	od := parseOpticalDrive(humanStorageNoOptical)
	if od.present {
		t.Fatalf("expected no optical drive, got %+v", od)
	}
}

func TestStorageAttachDvdMediumArgs_Mount(t *testing.T) {
	got := storageAttachDvdMediumArgs(dvdVMID, "SATA", 1, 0, `C:\ISOs\ubuntu.iso`)
	want := []string{
		"storageattach", dvdVMID,
		"--storagectl", "SATA",
		"--port", "1",
		"--device", "0",
		"--type", "dvddrive",
		"--medium", `C:\ISOs\ubuntu.iso`,
	}
	assertArgs(t, got, want)
}

func TestStorageAttachDvdMediumArgs_Eject(t *testing.T) {
	got := storageAttachDvdMediumArgs(dvdVMID, "IDE", 0, 0, "emptydrive")
	want := []string{
		"storageattach", dvdVMID,
		"--storagectl", "IDE",
		"--port", "0",
		"--device", "0",
		"--type", "dvddrive",
		"--medium", "emptydrive",
	}
	assertArgs(t, got, want)
}

func TestPickDvdTarget_PrefersSataFreePort(t *testing.T) {
	cs := []storageController{
		{name: "IDE", ctlType: "PIIX4", portCount: 2, maxPortCount: 2, used: map[int]bool{}},
		{name: "SATA", ctlType: "IntelAhci", portCount: 2, maxPortCount: 30, used: map[int]bool{0: true}},
	}
	name, port, bumpTo, ok := pickDvdTarget(cs)
	if !ok || name != "SATA" || port != 1 || bumpTo != 0 {
		t.Fatalf("expected SATA free port 1 no bump, got name=%s port=%d bump=%d ok=%v", name, port, bumpTo, ok)
	}
}

func TestPickDvdTarget_FallsBackToOtherController(t *testing.T) {
	cs := []storageController{
		{name: "IDE", ctlType: "PIIX4", portCount: 2, maxPortCount: 2, used: map[int]bool{0: true}},
	}
	name, port, bumpTo, ok := pickDvdTarget(cs)
	if !ok || name != "IDE" || port != 1 || bumpTo != 0 {
		t.Fatalf("expected IDE free port 1, got name=%s port=%d bump=%d ok=%v", name, port, bumpTo, ok)
	}
}

func TestPickDvdTarget_NoFreePort(t *testing.T) {
	cs := []storageController{
		{name: "IDE", ctlType: "PIIX4", portCount: 2, maxPortCount: 2, used: map[int]bool{0: true, 1: true}},
	}
	if _, _, _, ok := pickDvdTarget(cs); ok {
		t.Fatalf("expected no free target when every port is used and no controller can grow")
	}
}

func TestMountDvd_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	if _, err := svc.MountDvd(context.Background(), "bad", `C:\ISOs\x.iso`); err == nil {
		t.Fatalf("expected error for invalid id")
	}
}

func TestMountDvd_RejectsBadIsoPath(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	// Not absolute and not a .iso: validateIsoPath must reject before any command.
	_, err := svc.MountDvd(context.Background(), dvdVMID, "not-an-iso.txt")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected a *ValidationError for a bad ISO path, got %T: %v", err, err)
	}
}

func TestMountDvd_MountsIntoExistingDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	dir := t.TempDir()
	iso := filepath.Join(dir, "ubuntu.iso")
	if err := os.WriteFile(iso, []byte("x"), 0o600); err != nil {
		t.Fatalf("writing temp iso: %v", err)
	}
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version":             {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + dvdVMID: {ExitCode: 0, StandardOutput: humanStorageEmptyDvd},
		path + " storageattach " + dvdVMID + " --storagectl SATA --port 1 --device 0 --type dvddrive --medium " + iso: {ExitCode: 0},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.MountDvd(context.Background(), dvdVMID, iso)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.VMID != dvdVMID {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestMountDvd_AddsDriveWhenNoneExists(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	dir := t.TempDir()
	iso := filepath.Join(dir, "ubuntu.iso")
	if err := os.WriteFile(iso, []byte("x"), 0o600); err != nil {
		t.Fatalf("writing temp iso: %v", err)
	}
	path := createTempExecutable(t)
	machine := `VMState="poweroff"` + "\n" +
		`storagecontrollername0="SATA"` + "\n" +
		`storagecontrollertype0="IntelAhci"` + "\n" +
		`storagecontrollermaxportcount0="30"` + "\n" +
		`storagecontrollerportcount0="2"` + "\n" +
		`"SATA-0-0"="C:\VMs\vm\disk.vdi"` + "\n" +
		`"SATA-ImageUUID-0-0"="846b821b-1f6c-4c75-88c8-d462b6596f87"` + "\n" +
		`"SATA-1-0"="none"`
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version":                                    {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + dvdVMID:                        {ExitCode: 0, StandardOutput: humanStorageNoOptical},
		path + " showvminfo " + dvdVMID + " --machinereadable": {ExitCode: 0, StandardOutput: machine},
		path + " storageattach " + dvdVMID + " --storagectl SATA --port 1 --device 0 --type dvddrive --medium " + iso: {ExitCode: 0},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.MountDvd(context.Background(), dvdVMID, iso)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestEjectDvd_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	if _, err := svc.EjectDvd(context.Background(), "bad"); err == nil {
		t.Fatalf("expected error for invalid id")
	}
}

func TestEjectDvd_EmptiesTheDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version":             {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + dvdVMID: {ExitCode: 0, StandardOutput: humanStorageWithIso},
		path + " storageattach " + dvdVMID + " --storagectl SATA --port 1 --device 0 --type dvddrive --medium emptydrive": {ExitCode: 0},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.EjectDvd(context.Background(), dvdVMID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestEjectDvd_NoOpticalDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version":             {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + dvdVMID: {ExitCode: 0, StandardOutput: humanStorageNoOptical},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.EjectDvd(context.Background(), dvdVMID); err == nil {
		t.Fatalf("expected a ValidationError when there is no optical drive to eject")
	}
}

func TestVmStorage_IncludesOpticalMedium(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := storageRunner(t, path, "poweroff", mediumInfo("VDI", "dynamic default", 10240, ""), map[string]runner.Result{
		path + " showvminfo " + storageVMID: {ExitCode: 0, StandardOutput: humanStorageWithIso},
	})

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.VmStorage(context.Background(), storageVMID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Optical.Present {
		t.Fatalf("expected the optical drive to be reported present")
	}
	if resp.Optical.Medium != `C:\ISOs\alpine.iso` || resp.Optical.Controller != "SATA" || resp.Optical.Port != 1 {
		t.Fatalf("unexpected optical drive: %+v", resp.Optical)
	}
	if resp.Optical.Name != "alpine.iso" {
		t.Fatalf("expected the ISO basename as the display name, got %q", resp.Optical.Name)
	}
}
