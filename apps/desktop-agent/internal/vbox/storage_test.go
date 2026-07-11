package vbox

import (
	"context"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

const (
	storageVMID   = "11111111-1111-1111-1111-111111111111"
	storageMedium = "ca9ba73f-d0d3-4184-86f1-7206a952bc10"
)

// mediumInfo builds a showmediuminfo body with the given format, variant,
// capacity, and optional child UUIDs (snapshots).
func mediumInfo(format, variant string, capacityMB int, child string) string {
	body := "UUID:           " + storageMedium + "\n" +
		"State:          created\n" +
		"Type:           normal (base)\n" +
		"Storage format: " + format + "\n" +
		"Format variant: " + variant + "\n" +
		"Capacity:       " + strconv.Itoa(capacityMB) + " MBytes\n" +
		"Size on disk:   2 MBytes\n"
	if child != "" {
		body += "Child UUIDs:    " + child + "\n"
	}
	return body
}

func storageRunner(t *testing.T, path, vmState, mediumBody string, extra map[string]runner.Result) *fakeRunner {
	t.Helper()
	results := map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
		path + " showvminfo " + storageVMID + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="` + vmState + `"` + "\n" +
			`"SATA-0-0"="C:\VMs\disk1.vdi"` + "\n" +
			`"SATA-ImageUUID-0-0"="` + storageMedium + `"`},
		path + " showmediuminfo " + storageMedium: {ExitCode: 0, StandardOutput: mediumBody},
	}
	for k, v := range extra {
		results[k] = v
	}
	return &fakeRunner{results: results}
}

func TestVmStorage_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	if _, err := svc.VmStorage(context.Background(), "bad id"); err == nil {
		t.Fatalf("expected error for invalid id")
	}
}

func TestVmStorage_ReportsResizableVdi(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := storageRunner(t, path, "poweroff", mediumInfo("VDI", "dynamic default", 10240, ""), nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.VmStorage(context.Background(), storageVMID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(resp.Disks))
	}
	d := resp.Disks[0]
	if d.UUID != storageMedium || d.Format != "VDI" || d.CapacityMB != 10240 {
		t.Fatalf("unexpected disk: %+v", d)
	}
	if !d.Resizable || !resp.Editable {
		t.Fatalf("expected a powered-off VDI to be resizable: %+v (editable=%v)", d, resp.Editable)
	}
}

func TestVmStorage_SnapshotDiskNotResizable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := storageRunner(t, path, "poweroff", mediumInfo("VDI", "dynamic default", 10240, "54d19b81-1466-4fdd-a76d-4a077aae7439"), nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.VmStorage(context.Background(), storageVMID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Disks[0].Resizable {
		t.Fatalf("a disk with snapshots must not be resizable: %+v", resp.Disks[0])
	}
	if resp.Disks[0].Reason == "" {
		t.Fatalf("expected a reason explaining why it is not resizable")
	}
}

func TestVmStorage_VmdkNotResizable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := storageRunner(t, path, "poweroff", mediumInfo("VMDK", "dynamic default", 10240, ""), nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.VmStorage(context.Background(), storageVMID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Disks[0].Resizable {
		t.Fatalf("a VMDK disk must not be resizable in this build: %+v", resp.Disks[0])
	}
}

func TestVmStorage_LiveVmNotEditable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := storageRunner(t, path, "running", mediumInfo("VDI", "dynamic default", 10240, ""), nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.VmStorage(context.Background(), storageVMID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Editable {
		t.Fatalf("a running VM must not be editable")
	}
}

func TestResizeDisk_RejectsInvalidIDs(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	if _, err := svc.ResizeDisk(context.Background(), "bad", storageMedium, 20480); err == nil {
		t.Fatalf("expected error for invalid VM id")
	}
	if _, err := svc.ResizeDisk(context.Background(), storageVMID, "bad", 20480); err == nil {
		t.Fatalf("expected error for invalid medium id")
	}
}

func TestResizeDisk_RefusesLiveVM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := storageRunner(t, path, "running", mediumInfo("VDI", "dynamic default", 10240, ""), nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.ResizeDisk(context.Background(), storageVMID, storageMedium, 20480); err == nil {
		t.Fatalf("expected ValidationError for a running VM")
	}
}

func TestResizeDisk_RefusesShrink(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := storageRunner(t, path, "poweroff", mediumInfo("VDI", "dynamic default", 10240, ""), nil)

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.ResizeDisk(context.Background(), storageVMID, storageMedium, 5120); err == nil {
		t.Fatalf("expected ValidationError when shrinking below current capacity")
	}
}

func TestParseStorageControllers_ReadsSataAndUsedPorts(t *testing.T) {
	out := `storagecontrollername0="SATA"
storagecontrollertype0="IntelAhci"
storagecontrollermaxportcount0="30"
storagecontrollerportcount0="2"
"SATA-0-0"="C:\VMs\disk1.vdi"
"SATA-1-0"="none"`

	cs := parseStorageControllers(out)
	if len(cs) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(cs))
	}
	c := cs[0]
	if c.name != "SATA" || c.portCount != 2 || c.maxPortCount != 30 {
		t.Fatalf("unexpected controller: %+v", c)
	}
	if !c.used[0] || c.used[1] {
		t.Fatalf("expected port 0 used and port 1 free: %+v", c.used)
	}
}

func TestPickSataTarget_FreePortNoBump(t *testing.T) {
	cs := []storageController{{name: "SATA", ctlType: "IntelAhci", portCount: 2, maxPortCount: 30, used: map[int]bool{0: true}}}
	name, port, bumpTo, err := pickSataTarget(cs)
	if err != nil || name != "SATA" || port != 1 || bumpTo != 0 {
		t.Fatalf("expected free port 1 no bump, got name=%s port=%d bump=%d err=%v", name, port, bumpTo, err)
	}
}

func TestPickSataTarget_BumpsWhenFull(t *testing.T) {
	cs := []storageController{{name: "SATA", ctlType: "IntelAhci", portCount: 2, maxPortCount: 30, used: map[int]bool{0: true, 1: true}}}
	name, port, bumpTo, err := pickSataTarget(cs)
	if err != nil || name != "SATA" || port != 2 || bumpTo != 3 {
		t.Fatalf("expected bump to port 2 / count 3, got name=%s port=%d bump=%d err=%v", name, port, bumpTo, err)
	}
}

func TestPickSataTarget_NoSataController(t *testing.T) {
	cs := []storageController{{name: "IDE", ctlType: "PIIX4", portCount: 2, maxPortCount: 2, used: map[int]bool{}}}
	if _, _, _, err := pickSataTarget(cs); err == nil {
		t.Fatalf("expected error when there is no SATA controller")
	}
}

func TestAddDisk_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	if _, err := svc.AddDisk(context.Background(), "bad", 5120); err == nil {
		t.Fatalf("expected error for invalid id")
	}
}

func TestAddDisk_RejectsBadSize(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	if _, err := svc.AddDisk(context.Background(), storageVMID, 0); err == nil {
		t.Fatalf("expected error for zero size")
	}
}

func TestAddDisk_RefusesLiveVM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
		path + " showvminfo " + storageVMID + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="running"`},
	}}
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.AddDisk(context.Background(), storageVMID, 5120); err == nil {
		t.Fatalf("expected ValidationError for a running VM")
	}
}

func TestAddDisk_CreatesAndAttachesToFreePort(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	dir := t.TempDir()
	cfg := filepath.Join(dir, "vm.vbox")
	expectedDisk := filepath.Join(dir, "vm_1.vdi")
	path := createTempExecutable(t)

	info := `VMState="poweroff"` + "\n" +
		`CfgFile="` + cfg + `"` + "\n" +
		`storagecontrollername0="SATA"` + "\n" +
		`storagecontrollertype0="IntelAhci"` + "\n" +
		`storagecontrollermaxportcount0="30"` + "\n" +
		`storagecontrollerportcount0="2"` + "\n" +
		`"SATA-0-0"="` + filepath.Join(dir, "disk1.vdi") + `"` + "\n" +
		`"SATA-1-0"="none"`

	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
		path + " showvminfo " + storageVMID + " --machinereadable":                                                            {ExitCode: 0, StandardOutput: info},
		path + " createmedium disk --filename " + expectedDisk + " --size 5120 --format VDI":                                  {ExitCode: 0, StandardOutput: "Medium created. UUID: 43984aad-3422-4335-a216-139e1d2f0ab2"},
		path + " storageattach " + storageVMID + " --storagectl SATA --port 1 --device 0 --type hdd --medium " + expectedDisk: {ExitCode: 0},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.AddDisk(context.Background(), storageVMID, 5120)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.VMID != storageVMID {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestAddDisk_BumpsPortCountWhenFull(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	dir := t.TempDir()
	cfg := filepath.Join(dir, "vm.vbox")
	expectedDisk := filepath.Join(dir, "vm_1.vdi")
	path := createTempExecutable(t)

	info := `VMState="poweroff"` + "\n" +
		`CfgFile="` + cfg + `"` + "\n" +
		`storagecontrollername0="SATA"` + "\n" +
		`storagecontrollertype0="IntelAhci"` + "\n" +
		`storagecontrollermaxportcount0="30"` + "\n" +
		`storagecontrollerportcount0="2"` + "\n" +
		`"SATA-0-0"="` + filepath.Join(dir, "disk1.vdi") + `"` + "\n" +
		`"SATA-1-0"="` + filepath.Join(dir, "disk2.vdi") + `"`

	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
		path + " showvminfo " + storageVMID + " --machinereadable":                                                            {ExitCode: 0, StandardOutput: info},
		path + " createmedium disk --filename " + expectedDisk + " --size 5120 --format VDI":                                  {ExitCode: 0, StandardOutput: "Medium created."},
		path + " storagectl " + storageVMID + " --name SATA --portcount 3":                                                    {ExitCode: 0},
		path + " storageattach " + storageVMID + " --storagectl SATA --port 2 --device 0 --type hdd --medium " + expectedDisk: {ExitCode: 0},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.AddDisk(context.Background(), storageVMID, 5120)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestFindDiskAttachment_LocatesControllerPort(t *testing.T) {
	out := `"SATA-0-0"="C:\VMs\d1.vdi"` + "\n" +
		`"SATA-ImageUUID-0-0"="aaaaaaaa-1111-1111-1111-111111111111"` + "\n" +
		`"SATA-1-0"="C:\VMs\d2.vdi"` + "\n" +
		`"SATA-ImageUUID-1-0"="` + storageMedium + `"`

	ctl, port, device, found := findDiskAttachment(out, storageMedium)
	if !found || ctl != "SATA" || port != 1 || device != 0 {
		t.Fatalf("expected SATA port 1 device 0, got ctl=%s port=%d device=%d found=%v", ctl, port, device, found)
	}
}

func TestFindDiskAttachment_NotFound(t *testing.T) {
	out := `"SATA-ImageUUID-0-0"="aaaaaaaa-1111-1111-1111-111111111111"`
	if _, _, _, found := findDiskAttachment(out, storageMedium); found {
		t.Fatalf("expected not found for an unattached uuid")
	}
}

func detachRunner(t *testing.T, path, vmState, mediumBody string, extra map[string]runner.Result) *fakeRunner {
	t.Helper()
	results := map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
		path + " showvminfo " + storageVMID + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="` + vmState + `"` + "\n" +
			`"SATA-1-0"="C:\VMs\d2.vdi"` + "\n" +
			`"SATA-ImageUUID-1-0"="` + storageMedium + `"`},
		path + " showmediuminfo " + storageMedium: {ExitCode: 0, StandardOutput: mediumBody},
	}
	for k, v := range extra {
		results[k] = v
	}
	return &fakeRunner{results: results}
}

func TestDetachDisk_RejectsInvalidIDs(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	if _, err := svc.DetachDisk(context.Background(), "bad", storageMedium, false); err == nil {
		t.Fatalf("expected error for invalid VM id")
	}
	if _, err := svc.DetachDisk(context.Background(), storageVMID, "bad", false); err == nil {
		t.Fatalf("expected error for invalid medium id")
	}
}

func TestDetachDisk_RefusesLiveVM(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := detachRunner(t, path, "running", mediumInfo("VDI", "dynamic default", 4096, ""), nil)
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.DetachDisk(context.Background(), storageVMID, storageMedium, false); err == nil {
		t.Fatalf("expected ValidationError for a running VM")
	}
}

func TestDetachDisk_NotAttached(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
		path + " showvminfo " + storageVMID + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
	}}
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.DetachDisk(context.Background(), storageVMID, storageMedium, false); err == nil {
		t.Fatalf("expected error when the disk is not attached")
	}
}

func TestDetachDisk_DetachOnlyKeepsFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := detachRunner(t, path, "poweroff", mediumInfo("VDI", "dynamic default", 4096, ""), map[string]runner.Result{
		path + " storageattach " + storageVMID + " --storagectl SATA --port 1 --device 0 --type hdd --medium none": {ExitCode: 0},
	})
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.DetachDisk(context.Background(), storageVMID, storageMedium, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestDetachDisk_DeleteBlockedBySnapshots(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := detachRunner(t, path, "poweroff", mediumInfo("VDI", "dynamic default", 4096, "child-uuid-here"), nil)
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.DetachDisk(context.Background(), storageVMID, storageMedium, true); err == nil {
		t.Fatalf("expected ValidationError when deleting a disk with snapshots")
	}
}

func TestDetachDisk_DeleteRemovesFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := detachRunner(t, path, "poweroff", mediumInfo("VDI", "dynamic default", 4096, ""), map[string]runner.Result{
		path + " storageattach " + storageVMID + " --storagectl SATA --port 1 --device 0 --type hdd --medium none": {ExitCode: 0},
		path + " closemedium disk " + storageMedium + " --delete":                                                  {ExitCode: 0},
	})
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.DetachDisk(context.Background(), storageVMID, storageMedium, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestResizeDisk_GrowsPoweredOffVdi(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	path := createTempExecutable(t)
	run := storageRunner(t, path, "poweroff", mediumInfo("VDI", "dynamic default", 10240, ""), map[string]runner.Result{
		path + " modifymedium disk " + storageMedium + " --resize 20480": {ExitCode: 0},
	})

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.ResizeDisk(context.Background(), storageVMID, storageMedium, 20480)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.VMID != storageVMID {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
