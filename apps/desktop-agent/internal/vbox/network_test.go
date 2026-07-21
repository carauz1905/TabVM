package vbox

import (
	"context"
	"runtime"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

func TestParseNetworkAdapters(t *testing.T) {
	out := `nic1="nat"
macaddress1="0800271122AA"
nic2="bridged"
bridgeadapter2="Intel(R) Wi-Fi 6 AX201 160MHz"
macaddress2="080027BBCCDD"
nic3="none"
nic4="hostonly"
hostonlyadapter4="VirtualBox Host-Only Ethernet Adapter"`

	adapters := parseNetworkAdapters(out)
	if len(adapters) != 3 {
		t.Fatalf("expected 3 enabled adapters (nic3 is none), got %d", len(adapters))
	}
	if adapters[0].Slot != 1 || adapters[0].Mode != "nat" || adapters[0].MAC != "0800271122AA" {
		t.Fatalf("nic1 wrong: %+v", adapters[0])
	}
	if adapters[1].Mode != "bridged" || adapters[1].Adapter != "Intel(R) Wi-Fi 6 AX201 160MHz" {
		t.Fatalf("nic2 wrong: %+v", adapters[1])
	}
	if adapters[2].Slot != 4 || adapters[2].Mode != "hostonly" || adapters[2].Adapter != "VirtualBox Host-Only Ethernet Adapter" {
		t.Fatalf("nic4 wrong: %+v", adapters[2])
	}
}

func TestParseHostInterfaceNames(t *testing.T) {
	out := `Name:            Intel(R) Wi-Fi 6 AX201 160MHz
GUID:            abc
DHCP:            Disabled

Name:            Ethernet
GUID:            def`

	names := parseHostInterfaceNames(out)
	if len(names) != 2 || names[0] != "Intel(R) Wi-Fi 6 AX201 160MHz" || names[1] != "Ethernet" {
		t.Fatalf("host interface names wrong: %#v", names)
	}
}

func TestIsPlausibleHostInterface(t *testing.T) {
	if !isPlausibleHostInterface("Intel(R) Wi-Fi 6 AX201 160MHz") {
		t.Fatal("expected a normal adapter name to pass")
	}
	if isPlausibleHostInterface("") {
		t.Fatal("expected empty to fail")
	}
	if isPlausibleHostInterface("-flag") {
		t.Fatal("expected leading dash to fail")
	}
	if isPlausibleHostInterface("bad\x01name") {
		t.Fatal("expected control char to fail")
	}
}

func TestParseNetworkAdapters_CableConnected(t *testing.T) {
	out := `nic1="nat"
macaddress1="0800271122AA"
cableconnected1="on"
nic2="bridged"
bridgeadapter2="Ethernet"
macaddress2="080027BBCCDD"
cableconnected2="off"
nic3="hostonly"
hostonlyadapter3="VirtualBox Host-Only Ethernet Adapter"
macaddress3="080027CCDDEE"`

	adapters := parseNetworkAdapters(out)
	if len(adapters) != 3 {
		t.Fatalf("expected 3 enabled adapters, got %d", len(adapters))
	}
	if !adapters[0].CableConnected {
		t.Fatalf("nic1 cableconnected=on should be true: %+v", adapters[0])
	}
	if adapters[1].CableConnected {
		t.Fatalf("nic2 cableconnected=off should be false: %+v", adapters[1])
	}
	// nic3 has no cableconnected key; VBoxManage defaults a cable to connected.
	if !adapters[2].CableConnected {
		t.Fatalf("nic3 with no cableconnected key should default to true: %+v", adapters[2])
	}
}

func TestSetLinkStateArgs_LiveAndStopped(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"

	assertArgs(t, setLinkStateArgs(id, 1, false), []string{"controlvm", id, "setlinkstate1", "off"})
	assertArgs(t, setLinkStateArgs(id, 3, true), []string{"controlvm", id, "setlinkstate3", "on"})
	assertArgs(t, modifyLinkStateArgs(id, 1, true), []string{"modifyvm", id, "--cableconnected1", "on"})
	assertArgs(t, modifyLinkStateArgs(id, 2, false), []string{"modifyvm", id, "--cableconnected2", "off"})
}

func TestSetLinkState_RejectsBadSlot(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	id := "11111111-1111-1111-1111-111111111111"
	for _, slot := range []int{0, 9} {
		_, err := svc.SetLinkState(context.Background(), id, slot, false)
		if _, ok := err.(*ValidationError); !ok {
			t.Fatalf("expected *ValidationError for slot %d, got %T: %v", slot, err, err)
		}
	}
}

func TestSetLinkState_LiveUsesControlvm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable": {{ExitCode: 0, StandardOutput: "VMState=\"running\""}},
		path + " controlvm " + id + " setlinkstate1 off":  {{ExitCode: 0}},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.SetLinkState(context.Background(), id, 1, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	if !run.issued("controlvm " + id + " setlinkstate1 off") {
		t.Fatalf("expected controlvm setlinkstate1 off; calls: %v", run.calls)
	}
	if run.issued("modifyvm") {
		t.Fatal("running VM must use controlvm, not modifyvm")
	}
}

func TestSetLinkState_StoppedUsesModifyvm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable":  {{ExitCode: 0, StandardOutput: "VMState=\"poweroff\""}},
		path + " modifyvm " + id + " --cableconnected1 on": {{ExitCode: 0}},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.SetLinkState(context.Background(), id, 1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	if !run.issued("modifyvm " + id + " --cableconnected1 on") {
		t.Fatalf("expected modifyvm --cableconnected1 on; calls: %v", run.calls)
	}
	if run.issued("controlvm") {
		t.Fatal("stopped VM must use modifyvm, not controlvm")
	}
}
