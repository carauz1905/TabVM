package vbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tabvm/desktop-agent/internal/runner"
	"github.com/tabvm/desktop-agent/internal/store"
)

type fakeRunner struct {
	results map[string]runner.Result
}

func (f *fakeRunner) RunContext(ctx context.Context, name string, args []string, timeout time.Duration) (runner.Result, error) {
	key := name + " " + joinArgs(args)
	if result, ok := f.results[key]; ok {
		return result, nil
	}
	return runner.Result{ExitCode: 1, StandardError: "unexpected command"}, nil
}

func joinArgs(args []string) string {
	out := ""
	for i, arg := range args {
		if i > 0 {
			out += " "
		}
		out += arg
	}
	return out
}

// queuedRunner returns results in order for each command key. It is useful
// when a single command is invoked multiple times and must return different
// outputs, such as showvminfo before and after a modifyvm call.
type queuedRunner struct {
	queues map[string][]runner.Result
}

func (q *queuedRunner) RunContext(ctx context.Context, name string, args []string, timeout time.Duration) (runner.Result, error) {
	key := name + " " + joinArgs(args)
	queue := q.queues[key]
	if len(queue) == 0 {
		return runner.Result{ExitCode: 1, StandardError: "unexpected command: " + key}, nil
	}
	result := queue[0]
	q.queues[key] = queue[1:]
	return result, nil
}

func TestListVMs_ParsesVms(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {11111111-1111-1111-1111-111111111111}\r\n\"VM Two\" {22222222-2222-2222-2222-222222222222}"},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	resp, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.VMs) != 2 {
		t.Fatalf("expected 2 VMs, got %d", len(resp.VMs))
	}
	if resp.VMs[0].Name != "VM One" || resp.VMs[0].ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("first VM mismatch: %+v", resp.VMs[0])
	}
	if resp.VMs[1].Name != "VM Two" || resp.VMs[1].ID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("second VM mismatch: %+v", resp.VMs[1])
	}
}

func TestVmTelemetry_CorrelatesGuestIPToNIC(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `cpus="4"
memory="8192"
nic1="bridged"
macaddress1="0800271122AA"`},
			path + " guestproperty enumerate " + id + " --patterns /VirtualBox/GuestInfo/*": {ExitCode: 0, StandardOutput: `Name: /VirtualBox/GuestInfo/Net/0/V4/IP, value: 192.168.1.42, timestamp: 1, flags:
Name: /VirtualBox/GuestInfo/Net/0/MAC, value: 0800271122AA, timestamp: 1, flags:`},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	tel, err := svc.VmTelemetry(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tel.CPUCount != 4 || tel.RAMMB != 8192 {
		t.Fatalf("expected 4 cpu / 8192 MB, got %d / %d", tel.CPUCount, tel.RAMMB)
	}
	if !tel.GuestAdditions {
		t.Fatal("expected GuestAdditions=true")
	}
	if len(tel.Networks) != 1 {
		t.Fatalf("expected 1 network interface, got %d", len(tel.Networks))
	}
	nic := tel.Networks[0]
	if nic.Mode != "bridged" || len(nic.IPv4) != 1 || nic.IPv4[0] != "192.168.1.42" {
		t.Fatalf("expected bridged NIC with IP 192.168.1.42, got %+v", nic)
	}
}

func TestVmTelemetry_ReportsDiskUsage(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	diskUUID := "6f3c1e2a-1111-2222-3333-444455556666"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `cpus="2"
memory="4096"
nic1="nat"
macaddress1="080027A1B2C3"
"SATA-0-0"="C:\lab\lab.vdi"
"SATA-ImageUUID-0-0"="` + diskUUID + `"`},
			path + " guestproperty enumerate " + id + " --patterns /VirtualBox/GuestInfo/*": {ExitCode: 1, StandardError: "no ga"},
			path + " showmediuminfo " + diskUUID: {ExitCode: 0, StandardOutput: `Capacity:       51200 MBytes
Size on disk:   12800 MBytes`},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	tel, err := svc.VmTelemetry(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tel.Disks) != 1 {
		t.Fatalf("expected 1 disk, got %d: %#v", len(tel.Disks), tel.Disks)
	}
	disk := tel.Disks[0]
	if disk.Name != "lab.vdi" {
		t.Errorf("expected disk name lab.vdi, got %q", disk.Name)
	}
	if disk.CapacityBytes != 51200*1024*1024 || disk.AllocatedBytes != 12800*1024*1024 {
		t.Errorf("unexpected disk sizes: %+v", disk)
	}
	if disk.Percent != 25 {
		t.Errorf("expected 25%% allocation, got %d", disk.Percent)
	}
}

func TestVmTelemetry_DegradesWithoutGuestAdditions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `cpus="2"
memory="4096"
nic1="nat"
macaddress1="080027A1B2C3"`},
			// guestproperty returns non-zero (e.g. VM not running / no GA).
			path + " guestproperty enumerate " + id + " --patterns /VirtualBox/GuestInfo/*": {ExitCode: 1, StandardError: "not running"},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	tel, err := svc.VmTelemetry(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tel.GuestAdditions {
		t.Fatal("expected GuestAdditions=false when guestproperty fails")
	}
	if len(tel.Networks) != 1 {
		t.Fatalf("expected 1 configured NIC, got %d", len(tel.Networks))
	}
	if len(tel.Networks[0].IPv4) != 0 {
		t.Fatalf("expected no IPs without Guest Additions, got %+v", tel.Networks[0].IPv4)
	}
}

func TestListVMs_ReturnsEmptyList(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: ""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	resp, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.VMs) != 0 {
		t.Fatalf("expected empty VM list, got %d", len(resp.VMs))
	}
}

func TestListVMs_ReturnsNotDiscoveredWhenMissing(t *testing.T) {
	runner := &fakeRunner{}
	svc := NewService(runner, Config{CandidatePaths: []string{`C:\This\Path\Does\Not\Exist\VBoxManage.exe`}})

	_, err := svc.ListVMs(context.Background())
	if err == nil {
		t.Fatal("expected error when VBoxManage is missing")
	}
	if _, ok := err.(*NotDiscoveredError); !ok {
		t.Fatalf("expected NotDiscoveredError, got %T", err)
	}
}

func TestListVMs_ReturnsExecutionErrorOnNonZeroExit(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 1, StandardError: "VBoxManage error"},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	_, err := svc.ListVMs(context.Background())
	if err == nil {
		t.Fatal("expected error when VBoxManage fails")
	}
	execErr, ok := err.(*ExecutionError)
	if !ok {
		t.Fatalf("expected ExecutionError, got %T", err)
	}
	if execErr.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", execErr.ExitCode)
	}
	if execErr.StandardError != "VBoxManage error" {
		t.Fatalf("expected standard error %q, got %q", "VBoxManage error", execErr.StandardError)
	}
}

func TestListVMs_EnhancesStatesFromShowVmInfo(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	one := "11111111-1111-1111-1111-111111111111"
	two := "22222222-2222-2222-2222-222222222222"
	three := "33333333-3333-3333-3333-333333333333"
	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {" + one + "}\r\n\"VM Two\" {" + two + "}\r\n\"VM Three\" {" + three + "}"},
			path + " showvminfo " + one + " --machinereadable":   {ExitCode: 0, StandardOutput: `VMState="running"`},
			path + " showvminfo " + two + " --machinereadable":   {ExitCode: 0, StandardOutput: `VMState="starting"`},
			path + " showvminfo " + three + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"`},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	resp, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.VMs) != 3 {
		t.Fatalf("expected 3 VMs, got %d", len(resp.VMs))
	}
	if resp.VMs[0].State != "running" {
		t.Errorf("expected VM One running, got %q", resp.VMs[0].State)
	}
	if resp.VMs[1].State != "booting" {
		t.Errorf("expected VM Two booting (starting), got %q", resp.VMs[1].State)
	}
	if resp.VMs[2].State != "powered off" {
		t.Errorf("expected VM Three powered off, got %q", resp.VMs[2].State)
	}
}

func TestListVMs_KeepsPlaceholderWhenStateUnreadable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	one := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {" + one + "}"},
			// showvminfo intentionally absent -> fakeRunner returns ExitCode 1.
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	resp, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.VMs) != 1 || resp.VMs[0].State != "listed" {
		t.Fatalf("expected placeholder state preserved, got %+v", resp.VMs)
	}
}

func TestVMStatus_ReturnsParsedState(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo 11111111-1111-1111-1111-111111111111 --machinereadable": {ExitCode: 0, StandardOutput: "name=\"VM One\"\nVMState=\"running\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	status, err := svc.VMStatus(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected ID 11111111-1111-1111-1111-111111111111, got %q", status.ID)
	}
	if status.State != "running" {
		t.Fatalf("expected state running, got %q", status.State)
	}
}

func TestVMStatus_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.VMStatus(context.Background(), "bad id")
	if err == nil {
		t.Fatal("expected error for invalid VM ID")
	}
}

func TestStartVM_InvokesStartHeadless(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " startvm 11111111-1111-1111-1111-111111111111 --type headless": {ExitCode: 0},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	if err := svc.StartVM(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopVM_InvokesAcpiPowerButton(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " controlvm 11111111-1111-1111-1111-111111111111 acpipowerbutton": {ExitCode: 0},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	if err := svc.StopVM(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResetVM_InvokesReset(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " controlvm 11111111-1111-1111-1111-111111111111 reset": {ExitCode: 0},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	if err := svc.ResetVM(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLifecycleMethods_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})

	methods := []struct {
		name string
		call func() error
	}{
		{"StartVM", func() error { return svc.StartVM(context.Background(), "bad id") }},
		{"StopVM", func() error { return svc.StopVM(context.Background(), "bad id") }},
		{"ResetVM", func() error { return svc.ResetVM(context.Background(), "bad id") }},
		{"DisableVmConsole", func() error { return svc.DisableVmConsole(context.Background(), "bad id") }},
	}

	for _, m := range methods {
		t.Run(m.name, func(t *testing.T) {
			if err := m.call(); err == nil {
				t.Fatal("expected error for invalid VM ID")
			}
		})
	}
}

func TestVmConsoleStatus_ParsesVRDE(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo 11111111-1111-1111-1111-111111111111 --machinereadable": {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"5432\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	status, err := svc.VmConsoleStatus(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected ID 11111111-1111-1111-1111-111111111111, got %q", status.ID)
	}
	if !status.Enabled {
		t.Fatal("expected VRDE enabled")
	}
	if status.Port != 5432 {
		t.Fatalf("expected port 5432, got %d", status.Port)
	}
	if !status.Ready {
		t.Fatal("expected console ready")
	}
	if status.Target != "127.0.0.1:5432" {
		t.Fatalf("expected target 127.0.0.1:5432, got %q", status.Target)
	}
	if status.Protocol != "rdp" {
		t.Fatalf("expected protocol rdp, got %q", status.Protocol)
	}
	if status.Source != "virtualbox-vrde" {
		t.Fatalf("expected source virtualbox-vrde, got %q", status.Source)
	}
	if len(status.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(status.Targets))
	}
	target := status.Targets[0]
	if target.Protocol != "rdp" || target.Host != "127.0.0.1" || target.Port != 5432 || target.Source != "virtualbox-vrde" {
		t.Fatalf("unexpected target: %+v", target)
	}
	if !target.Ready {
		t.Fatal("expected target ready")
	}
}

func TestVmConsoleStatus_NotReadyWhenAddressIsNotLocalhost(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo 11111111-1111-1111-1111-111111111111 --machinereadable": {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"0.0.0.0\"\nvrdeport=\"5432\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	status, err := svc.VmConsoleStatus(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Ready {
		t.Fatal("expected console not ready when bound to non-loopback address")
	}
	if status.Protocol != "rdp" {
		t.Fatalf("expected protocol rdp, got %q", status.Protocol)
	}
	if status.Source != "virtualbox-vrde" {
		t.Fatalf("expected source virtualbox-vrde, got %q", status.Source)
	}
	if status.Address != "" {
		t.Fatalf("expected sanitized address, got %q", status.Address)
	}
	if status.Port != 0 {
		t.Fatalf("expected sanitized port 0, got %d", status.Port)
	}
	if status.Target != "" {
		t.Fatalf("expected sanitized target, got %q", status.Target)
	}
	if len(status.Targets) != 0 {
		t.Fatalf("expected 0 sanitized targets, got %d", len(status.Targets))
	}
}

func TestVmConsoleStatus_NotReadyAndSanitizedWhenPortOutOfRange(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo 11111111-1111-1111-1111-111111111111 --machinereadable": {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"1234\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	status, err := svc.VmConsoleStatus(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Ready {
		t.Fatal("expected console not ready when port is outside the local range")
	}
	if status.Address != "" {
		t.Fatalf("expected sanitized address, got %q", status.Address)
	}
	if status.Port != 0 {
		t.Fatalf("expected sanitized port 0, got %d", status.Port)
	}
	if status.Target != "" {
		t.Fatalf("expected sanitized target, got %q", status.Target)
	}
	if len(status.Targets) != 0 {
		t.Fatalf("expected 0 sanitized targets, got %d", len(status.Targets))
	}
}

func TestPrepareVmConsole_InvokesModifyVm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	originalCheck := portAvailable
	portAvailable = func(port int) bool { return true }
	defer func() { portAvailable = originalCheck }()

	path := createTempExecutable(t)
	id := "11111111-1111-1111-1111-111111111111"
	port := VmIDToConsolePort(id)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {" + id + "}"},
			path + " showvminfo " + id + " --machinereadable":                                                {ExitCode: 0, StandardOutput: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\""},
			path + " modifyvm " + id + " --vrde on --vrdeaddress 127.0.0.1 --vrdeport " + strconv.Itoa(port): {ExitCode: 0},
			path + " showvminfo " + id + " --machinereadable":                                                {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"" + strconv.Itoa(port) + "\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	status, err := svc.PrepareVmConsole(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.Ready {
		t.Fatalf("expected console ready after prepare, got %+v", status)
	}
	if status.Port != port {
		t.Fatalf("expected port %d, got %d", port, status.Port)
	}
}

func TestPrepareVmConsole_SkipsPortUsedByAnotherVm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	originalCheck := portAvailable
	portAvailable = func(port int) bool { return true }
	defer func() { portAvailable = originalCheck }()

	path := createTempExecutable(t)
	id := "11111111-1111-1111-1111-111111111111"
	otherID := "22222222-2222-2222-2222-222222222222"
	collidingPort := VmIDToConsolePort(id)
	expectedPort := collidingPort + 1
	if expectedPort > maxConsolePort {
		expectedPort = minConsolePort
	}

	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {" + id + "}\r\n\"VM Two\" {" + otherID + "}"},
			path + " showvminfo " + id + " --machinereadable":                                                        {ExitCode: 0, StandardOutput: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\""},
			path + " showvminfo " + otherID + " --machinereadable":                                                   {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"" + strconv.Itoa(collidingPort) + "\""},
			path + " modifyvm " + id + " --vrde on --vrdeaddress 127.0.0.1 --vrdeport " + strconv.Itoa(expectedPort): {ExitCode: 0},
			path + " showvminfo " + id + " --machinereadable":                                                        {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"" + strconv.Itoa(expectedPort) + "\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	status, err := svc.PrepareVmConsole(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Port != expectedPort {
		t.Fatalf("expected port %d after collision skip, got %d", expectedPort, status.Port)
	}
}

func TestPrepareVmConsole_ProbesForwardWhenLocalPortOccupied(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	originalCheck := portAvailable
	portAvailable = func(port int) bool {
		return port != VmIDToConsolePort("11111111-1111-1111-1111-111111111111")
	}
	defer func() { portAvailable = originalCheck }()

	path := createTempExecutable(t)
	id := "11111111-1111-1111-1111-111111111111"
	collidingPort := VmIDToConsolePort(id)
	expectedPort := collidingPort + 1
	if expectedPort > maxConsolePort {
		expectedPort = minConsolePort
	}

	runner := &queuedRunner{
		queues: map[string][]runner.Result{
			path + " --version": {
				{ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
				{ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			},
			path + " list vms": {
				{ExitCode: 0, StandardOutput: "\"VM One\" {" + id + "}"},
			},
			path + " showvminfo " + id + " --machinereadable": {
				{ExitCode: 0, StandardOutput: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\""},
				{ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"" + strconv.Itoa(expectedPort) + "\""},
			},
			path + " modifyvm " + id + " --vrde on --vrdeaddress 127.0.0.1 --vrdeport " + strconv.Itoa(expectedPort): {
				{ExitCode: 0},
			},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	status, err := svc.PrepareVmConsole(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Port != expectedPort {
		t.Fatalf("expected port %d after probing forward, got %d", expectedPort, status.Port)
	}
	if !status.Ready {
		t.Fatalf("expected console ready, got %+v", status)
	}
}

func TestPrepareVmConsole_ReturnsConflictWhenRangeExhausted(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	originalCheck := portAvailable
	portAvailable = func(port int) bool { return false }
	defer func() { portAvailable = originalCheck }()

	path := createTempExecutable(t)
	id := "11111111-1111-1111-1111-111111111111"
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {" + id + "}"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	_, err := svc.PrepareVmConsole(context.Background(), id)
	if err == nil {
		t.Fatal("expected error when no VRDE port is available")
	}

	execErr, ok := err.(*ExecutionError)
	if !ok {
		t.Fatalf("expected ExecutionError, got %T", err)
	}
	if execErr.ExitCode != -1 {
		t.Fatalf("expected exit code -1, got %d", execErr.ExitCode)
	}
	if !strings.Contains(execErr.Message, "No available VRDE port") {
		t.Fatalf("expected exhausted range message, got %q", execErr.Message)
	}
}

func TestPrepareVmConsole_UsesPersistedPort(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	originalCheck := portAvailable
	portAvailable = func(port int) bool { return true }
	defer func() { portAvailable = originalCheck }()

	ctx := context.Background()
	db, err := store.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error opening store: %v", err)
	}
	defer db.Close()

	if err := db.SetVmConsolePort(ctx, store.VmConsolePort{
		VMID:     "11111111-1111-1111-1111-111111111111",
		Port:     5555,
		Address:  "127.0.0.1",
		Protocol: "rdp",
		Source:   "virtualbox-vrde",
	}); err != nil {
		t.Fatalf("unexpected error seeding port: %v", err)
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {11111111-1111-1111-1111-111111111111}"},
			path + " showvminfo 11111111-1111-1111-1111-111111111111 --machinereadable":                               {ExitCode: 0, StandardOutput: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\""},
			path + " modifyvm 11111111-1111-1111-1111-111111111111 --vrde on --vrdeaddress 127.0.0.1 --vrdeport 5555": {ExitCode: 0},
			path + " showvminfo 11111111-1111-1111-1111-111111111111 --machinereadable":                               {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"5555\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}, Store: db})
	status, err := svc.PrepareVmConsole(ctx, "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Port != 5555 {
		t.Fatalf("expected persisted port 5555, got %d", status.Port)
	}
}

func TestPrepareVmConsole_PersistedPortCollidesWithAnotherVm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	originalCheck := portAvailable
	portAvailable = func(port int) bool { return true }
	defer func() { portAvailable = originalCheck }()

	ctx := context.Background()
	db, err := store.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error opening store: %v", err)
	}
	defer db.Close()

	id := "11111111-1111-1111-1111-111111111111"
	otherID := "22222222-2222-2222-2222-222222222222"
	persistedPort := 5555
	expectedPort := persistedPort + 1
	if expectedPort > maxConsolePort {
		expectedPort = minConsolePort
	}

	if err := db.SetVmConsolePort(ctx, store.VmConsolePort{
		VMID:     id,
		Port:     persistedPort,
		Address:  "127.0.0.1",
		Protocol: "rdp",
		Source:   "virtualbox-vrde",
	}); err != nil {
		t.Fatalf("unexpected error seeding port: %v", err)
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {" + id + "}\r\n\"VM Two\" {" + otherID + "}"},
			path + " showvminfo " + id + " --machinereadable":                                                        {ExitCode: 0, StandardOutput: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\""},
			path + " showvminfo " + otherID + " --machinereadable":                                                   {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"" + strconv.Itoa(persistedPort) + "\""},
			path + " modifyvm " + id + " --vrde on --vrdeaddress 127.0.0.1 --vrdeport " + strconv.Itoa(expectedPort): {ExitCode: 0},
			path + " showvminfo " + id + " --machinereadable":                                                        {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"" + strconv.Itoa(expectedPort) + "\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}, Store: db})
	status, err := svc.PrepareVmConsole(ctx, id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Port != expectedPort {
		t.Fatalf("expected port %d after collision skip, got %d", expectedPort, status.Port)
	}

	record, err := db.GetVmConsolePort(ctx, id)
	if err != nil {
		t.Fatalf("unexpected error reading persisted port: %v", err)
	}
	if record == nil || record.Port != expectedPort {
		t.Fatalf("expected persisted port to be updated to %d, got %+v", expectedPort, record)
	}
}

func TestPrepareVmConsole_PersistedPortLocallyOccupied(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	originalCheck := portAvailable
	portAvailable = func(port int) bool { return port != 5555 }
	defer func() { portAvailable = originalCheck }()

	ctx := context.Background()
	db, err := store.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error opening store: %v", err)
	}
	defer db.Close()

	id := "11111111-1111-1111-1111-111111111111"
	persistedPort := 5555
	expectedPort := persistedPort + 1
	if expectedPort > maxConsolePort {
		expectedPort = minConsolePort
	}

	if err := db.SetVmConsolePort(ctx, store.VmConsolePort{
		VMID:     id,
		Port:     persistedPort,
		Address:  "127.0.0.1",
		Protocol: "rdp",
		Source:   "virtualbox-vrde",
	}); err != nil {
		t.Fatalf("unexpected error seeding port: %v", err)
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {" + id + "}"},
			path + " showvminfo " + id + " --machinereadable":                                                        {ExitCode: 0, StandardOutput: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\""},
			path + " modifyvm " + id + " --vrde on --vrdeaddress 127.0.0.1 --vrdeport " + strconv.Itoa(expectedPort): {ExitCode: 0},
			path + " showvminfo " + id + " --machinereadable":                                                        {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"" + strconv.Itoa(expectedPort) + "\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}, Store: db})
	status, err := svc.PrepareVmConsole(ctx, id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Port != expectedPort {
		t.Fatalf("expected port %d after skipping occupied persisted port, got %d", expectedPort, status.Port)
	}

	record, err := db.GetVmConsolePort(ctx, id)
	if err != nil {
		t.Fatalf("unexpected error reading persisted port: %v", err)
	}
	if record == nil || record.Port != expectedPort {
		t.Fatalf("expected persisted port to be updated to %d, got %+v", expectedPort, record)
	}
}

func TestPrepareVmConsole_PersistsSelectedPort(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	originalCheck := portAvailable
	portAvailable = func(port int) bool { return true }
	defer func() { portAvailable = originalCheck }()

	ctx := context.Background()
	db, err := store.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("unexpected error opening store: %v", err)
	}
	defer db.Close()

	path := createTempExecutable(t)
	id := "11111111-1111-1111-1111-111111111111"
	port := VmIDToConsolePort(id)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {" + id + "}"},
			path + " showvminfo " + id + " --machinereadable":                                                {ExitCode: 0, StandardOutput: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\""},
			path + " modifyvm " + id + " --vrde on --vrdeaddress 127.0.0.1 --vrdeport " + strconv.Itoa(port): {ExitCode: 0},
			path + " showvminfo " + id + " --machinereadable":                                                {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"" + strconv.Itoa(port) + "\""},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}, Store: db})
	_, err = svc.PrepareVmConsole(ctx, id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	record, err := db.GetVmConsolePort(ctx, id)
	if err != nil {
		t.Fatalf("unexpected error reading persisted port: %v", err)
	}
	if record == nil {
		t.Fatal("expected port to be persisted after prepare")
	}
	if record.Port != port {
		t.Fatalf("expected persisted port %d, got %d", port, record.Port)
	}
}

func TestPrepareVmConsole_StoreFailureDoesNotFailOperation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	originalCheck := portAvailable
	portAvailable = func(port int) bool { return true }
	defer func() { portAvailable = originalCheck }()

	path := createTempExecutable(t)
	id := "11111111-1111-1111-1111-111111111111"
	port := VmIDToConsolePort(id)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " list vms":  {ExitCode: 0, StandardOutput: "\"VM One\" {" + id + "}"},
			path + " showvminfo " + id + " --machinereadable":                                                {ExitCode: 0, StandardOutput: "vrde=\"off\"\nvrdeaddress=\"\"\nvrdeport=\"0\""},
			path + " modifyvm " + id + " --vrde on --vrdeaddress 127.0.0.1 --vrdeport " + strconv.Itoa(port): {ExitCode: 0},
			path + " showvminfo " + id + " --machinereadable":                                                {ExitCode: 0, StandardOutput: "vrde=\"on\"\nvrdeaddress=\"127.0.0.1\"\nvrdeport=\"" + strconv.Itoa(port) + "\""},
		},
	}

	// A nil store should not prevent the prepare operation from succeeding.
	svc := NewService(runner, Config{CandidatePaths: []string{path}, Store: nil})
	status, err := svc.PrepareVmConsole(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Port != port {
		t.Fatalf("expected port %d, got %d", port, status.Port)
	}
}

func TestDisableVmConsole_InvokesModifyVm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " modifyvm 11111111-1111-1111-1111-111111111111 --vrde off": {ExitCode: 0},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	if err := svc.DisableVmConsole(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVmIDToConsolePort_IsInRange(t *testing.T) {
	port := VmIDToConsolePort("11111111-1111-1111-1111-111111111111")
	if port < minConsolePort || port > maxConsolePort {
		t.Fatalf("port %d out of range [%d,%d]", port, minConsolePort, maxConsolePort)
	}
}

func TestVmIDToConsolePort_IsStable(t *testing.T) {
	if VmIDToConsolePort("11111111-1111-1111-1111-111111111111") != VmIDToConsolePort("11111111-1111-1111-1111-111111111111") {
		t.Fatal("expected deterministic port for the same VM ID")
	}
}

func TestIsValidConsolePort(t *testing.T) {
	if !IsValidConsolePort(5000) {
		t.Fatal("expected 5000 to be valid")
	}
	if !IsValidConsolePort(5999) {
		t.Fatal("expected 5999 to be valid")
	}
	if IsValidConsolePort(4999) {
		t.Fatal("expected 4999 to be invalid")
	}
	if IsValidConsolePort(6000) {
		t.Fatal("expected 6000 to be invalid")
	}
}

func TestEnableVrdeArgs(t *testing.T) {
	args := enableVrdeArgs("11111111-1111-1111-1111-111111111111", 5432)
	expected := []string{"modifyvm", "11111111-1111-1111-1111-111111111111", "--vrde", "on", "--vrdeaddress", "127.0.0.1", "--vrdeport", "5432"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d mismatch: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

func TestDisableVrdeArgs(t *testing.T) {
	args := disableVrdeArgs("11111111-1111-1111-1111-111111111111")
	expected := []string{"modifyvm", "11111111-1111-1111-1111-111111111111", "--vrde", "off"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d mismatch: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

func TestStartVMArgs(t *testing.T) {
	args := startVmArgs("11111111-1111-1111-1111-111111111111")
	expected := []string{"startvm", "11111111-1111-1111-1111-111111111111", "--type", "headless"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d mismatch: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

func TestStopVmArgs(t *testing.T) {
	args := stopVmArgs("11111111-1111-1111-1111-111111111111")
	expected := []string{"controlvm", "11111111-1111-1111-1111-111111111111", "acpipowerbutton"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d mismatch: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

func TestResetVmArgs(t *testing.T) {
	args := resetVmArgs("11111111-1111-1111-1111-111111111111")
	expected := []string{"controlvm", "11111111-1111-1111-1111-111111111111", "reset"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d mismatch: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

func TestForcePowerOff_InvokesPowerOff(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	runner := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " controlvm 11111111-1111-1111-1111-111111111111 poweroff": {ExitCode: 0},
		},
	}

	svc := NewService(runner, Config{CandidatePaths: []string{path}})
	if err := svc.ForcePowerOff(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForcePowerOff_RejectsInvalidID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	if err := svc.ForcePowerOff(context.Background(), "bad id"); err == nil {
		t.Fatal("expected error for invalid VM ID")
	}
}

func TestPowerOffVmArgs(t *testing.T) {
	args := powerOffVmArgs("11111111-1111-1111-1111-111111111111")
	expected := []string{"controlvm", "11111111-1111-1111-1111-111111111111", "poweroff"}
	if len(args) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d mismatch: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

func createTempExecutable(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "VBoxManage.exe")
	if err := writeEmptyFile(path); err != nil {
		t.Fatalf("failed to create temp executable: %v", err)
	}
	return path
}

func writeEmptyFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}
