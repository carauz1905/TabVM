package vbox

import (
	"context"
	"runtime"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

const usbHostSample = `Host USB Devices:

UUID:               2b7e1a10-1234-4abc-8def-0123456789ab
VendorId:           0x0781 (0781)
ProductId:          0x5567 (5567)
Revision:           1.0 (0100)
Manufacturer:       SanDisk
Product:            Cruzer Blade
SerialNumber:       4C530001
Address:            {abc}
Current State:      Available

UUID:               99887766-5544-4332-8110-aabbccddeeff
VendorId:           0x046d (046d)
ProductId:          0xc52b (c52b)
Revision:           12.3 (1203)
Manufacturer:       Logitech
Product:            USB Receiver
Current State:      Captured
`

func TestParseUsbHostDevices_MultiDevice(t *testing.T) {
	devices := parseUsbHostDevices(usbHostSample)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d: %+v", len(devices), devices)
	}
	d0 := devices[0]
	if d0.UUID != "2b7e1a10-1234-4abc-8def-0123456789ab" {
		t.Fatalf("device 0 UUID wrong: %q", d0.UUID)
	}
	if d0.VendorID != "0x0781" || d0.ProductID != "0x5567" {
		t.Fatalf("device 0 ids wrong: vendor=%q product=%q", d0.VendorID, d0.ProductID)
	}
	if d0.Manufacturer != "SanDisk" || d0.Product != "Cruzer Blade" {
		t.Fatalf("device 0 name wrong: manufacturer=%q product=%q", d0.Manufacturer, d0.Product)
	}
	if d0.State != "Available" {
		t.Fatalf("device 0 state wrong: %q", d0.State)
	}
	if devices[1].UUID != "99887766-5544-4332-8110-aabbccddeeff" || devices[1].State != "Captured" {
		t.Fatalf("device 1 wrong: %+v", devices[1])
	}
}

func TestParseUsbHostDevices_Empty(t *testing.T) {
	devices := parseUsbHostDevices("Host USB Devices:\n\n<none>\n")
	if devices == nil {
		t.Fatal("expected a non-nil empty slice so JSON serializes as []")
	}
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices, got %d", len(devices))
	}
}

func TestParseUsbHostDevices_MissingFields(t *testing.T) {
	out := `Host USB Devices:

UUID:               2b7e1a10-1234-4abc-8def-0123456789ab
VendorId:           0x1234 (1234)
ProductId:          0x5678 (5678)
Current State:      Busy
`
	devices := parseUsbHostDevices(out)
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	d := devices[0]
	if d.Manufacturer != "" || d.Product != "" {
		t.Fatalf("expected empty manufacturer/product, got %q/%q", d.Manufacturer, d.Product)
	}
	if d.State != "Busy" {
		t.Fatalf("expected state Busy, got %q", d.State)
	}
}

func TestParseAttachedUsbUUIDs(t *testing.T) {
	human := `USB:                 enabled

Currently Attached USB Devices:

UUID:               2b7e1a10-1234-4abc-8def-0123456789ab
VendorId:           0x0781 (0781)
ProductId:          0x5567 (5567)
Manufacturer:       SanDisk
Product:            Cruzer Blade

Available remote USB devices:

<none>

USB Device Filters:

<none>`

	attached := parseAttachedUsbUUIDs(human)
	if len(attached) != 1 {
		t.Fatalf("expected 1 attached UUID, got %d: %+v", len(attached), attached)
	}
	if _, ok := attached["2b7e1a10-1234-4abc-8def-0123456789ab"]; !ok {
		t.Fatalf("expected the SanDisk UUID to be attached: %+v", attached)
	}
}

func TestParseAttachedUsbUUIDs_None(t *testing.T) {
	human := `Currently Attached USB Devices:  <none>

USB Device Filters:

<none>`
	attached := parseAttachedUsbUUIDs(human)
	if len(attached) != 0 {
		t.Fatalf("expected no attached UUIDs, got %d", len(attached))
	}
}

func TestParseAttachedUsbUUIDs_StopsAtNextSection(t *testing.T) {
	// A UUID that appears in a LATER section (remote devices) must not be
	// treated as attached to this VM.
	human := `Currently Attached USB Devices:

UUID:               11111111-1111-4111-8111-111111111111

Available remote USB devices:

UUID:               22222222-2222-4222-8222-222222222222`
	attached := parseAttachedUsbUUIDs(human)
	if len(attached) != 1 {
		t.Fatalf("expected exactly 1 attached UUID, got %d: %+v", len(attached), attached)
	}
	if _, ok := attached["22222222-2222-4222-8222-222222222222"]; ok {
		t.Fatal("remote-device UUID must not be counted as attached to this VM")
	}
}

func TestParseUsbControllerEnabled(t *testing.T) {
	if !parseUsbControllerEnabled(`usb="off"` + "\n" + `ehci="on"`) {
		t.Fatal("expected controller enabled when ehci is on")
	}
	if !parseUsbControllerEnabled(`xhci="on"`) {
		t.Fatal("expected controller enabled when xhci is on")
	}
	if parseUsbControllerEnabled(`usb="off"` + "\n" + `ehci="off"` + "\n" + `xhci="off"`) {
		t.Fatal("expected controller disabled when all off")
	}
}

func TestParseExtensionPackInstalled(t *testing.T) {
	out := `Extension Packs: 1
Pack no. 0:   Oracle VirtualBox Extension Pack
Version:      7.2.12
Usable:       true`
	if !parseExtensionPackInstalled(out) {
		t.Fatal("expected extension pack detected")
	}
	if parseExtensionPackInstalled("Extension Packs: 0\n") {
		t.Fatal("expected no extension pack detected")
	}
}

func TestUsbArgs(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	uuid := "2b7e1a10-1234-4abc-8def-0123456789ab"
	assertArgs(t, usbAttachArgs(id, uuid), []string{"controlvm", id, "usbattach", uuid})
	assertArgs(t, usbDetachArgs(id, uuid), []string{"controlvm", id, "usbdetach", uuid})
	assertArgs(t, listUsbHostArgs(), []string{"list", "usbhost"})
	assertArgs(t, listExtpacksArgs(), []string{"list", "extpacks"})
}

func TestAttachUsb_RejectsInvalidUUID(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
	}}
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.AttachUsb(context.Background(), id, "not-a-uuid")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for bad UUID, got %T: %v", err, err)
	}
}

func TestAttachUsb_RejectsNonRunning(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	uuid := "2b7e1a10-1234-4abc-8def-0123456789ab"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
	}}
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.AttachUsb(context.Background(), id, uuid)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for non-running VM, got %T: %v", err, err)
	}
}

func TestAttachUsb_IssuesUsbattach(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	uuid := "2b7e1a10-1234-4abc-8def-0123456789ab"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable": {{ExitCode: 0, StandardOutput: `VMState="running"`}},
		path + " controlvm " + id + " usbattach " + uuid:  {{ExitCode: 0}},
	}}
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.AttachUsb(context.Background(), id, uuid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.VMID != id {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !run.issued("controlvm " + id + " usbattach " + uuid) {
		t.Fatalf("expected usbattach to be issued, calls: %v", run.calls)
	}
}

func TestDetachUsb_IssuesUsbdetach(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	uuid := "2b7e1a10-1234-4abc-8def-0123456789ab"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable": {{ExitCode: 0, StandardOutput: `VMState="running"`}},
		path + " controlvm " + id + " usbdetach " + uuid:  {{ExitCode: 0}},
	}}
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.DetachUsb(context.Background(), id, uuid); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !run.issued("controlvm " + id + " usbdetach " + uuid) {
		t.Fatalf("expected usbdetach to be issued, calls: %v", run.calls)
	}
}

func TestAttachUsb_MapsMissingExtensionPack(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	uuid := "2b7e1a10-1234-4abc-8def-0123456789ab"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable": {{ExitCode: 0, StandardOutput: `VMState="running"`}},
		path + " controlvm " + id + " usbattach " + uuid:  {{ExitCode: 1, StandardError: "VBoxManage: error: Implementation of the USB 2.0 controller not found because the extension pack is either not installed"}},
	}}
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.AttachUsb(context.Background(), id, uuid)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError mapping the extension-pack failure, got %T: %v", err, err)
	}
}

func TestVmUsb_MarksAttachedHereAndPrereqs(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	human := `USB:                 enabled

Currently Attached USB Devices:

UUID:               2b7e1a10-1234-4abc-8def-0123456789ab
Manufacturer:       SanDisk
Product:            Cruzer Blade

Available remote USB devices:

<none>`
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="running"` + "\n" + `usb="on"`},
		path + " showvminfo " + id:                        {ExitCode: 0, StandardOutput: human},
		path + " list usbhost":                            {ExitCode: 0, StandardOutput: usbHostSample},
		path + " list extpacks":                           {ExitCode: 0, StandardOutput: "Pack no. 0:   Oracle VirtualBox Extension Pack\n"},
	}}
	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.VmUsb(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.ExtensionPackInstalled {
		t.Fatal("expected extension pack reported installed")
	}
	if !resp.USBControllerEnabled {
		t.Fatal("expected USB controller reported enabled")
	}
	if len(resp.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(resp.Devices))
	}
	var sandisk, logitech *bool
	for i := range resp.Devices {
		d := resp.Devices[i]
		attached := d.AttachedHere
		if d.UUID == "2b7e1a10-1234-4abc-8def-0123456789ab" {
			sandisk = &attached
		}
		if d.UUID == "99887766-5544-4332-8110-aabbccddeeff" {
			logitech = &attached
		}
	}
	if sandisk == nil || !*sandisk {
		t.Fatal("expected the SanDisk device to be marked attachedHere")
	}
	if logitech == nil || *logitech {
		t.Fatal("expected the Logitech device NOT to be marked attachedHere")
	}
}
