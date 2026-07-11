package vbox

import (
	"reflect"
	"testing"
)

func TestParseVmResources(t *testing.T) {
	output := `name="ubuntu-lab"
cpus="4"
memory="8192"
VMState="running"`

	cpu, ram := parseVmResources(output)
	if cpu != 4 {
		t.Errorf("expected 4 cpus, got %d", cpu)
	}
	if ram != 8192 {
		t.Errorf("expected 8192 MB ram, got %d", ram)
	}
}

func TestParseVmResources_MissingValues(t *testing.T) {
	cpu, ram := parseVmResources(`name="x"`)
	if cpu != 0 || ram != 0 {
		t.Errorf("expected zero cpu/ram when absent, got %d/%d", cpu, ram)
	}
}

func TestParseVmNICs_SkipsNoneAndParsesModeAndMac(t *testing.T) {
	output := `nic1="nat"
macaddress1="080027A1B2C3"
nic2="bridged"
macaddress2="0800271122AA"
nic3="none"`

	nics := parseVmNICs(output)
	want := []nicInfo{
		{slot: 1, mode: "nat", mac: "080027A1B2C3"},
		{slot: 2, mode: "bridged", mac: "0800271122AA"},
	}
	if !reflect.DeepEqual(nics, want) {
		t.Fatalf("unexpected nics:\n got %#v\nwant %#v", nics, want)
	}
}

func TestParseGuestNetworks_CorrelatesByMac(t *testing.T) {
	output := `Name: /VirtualBox/GuestInfo/OS/Product, value: Ubuntu, timestamp: 1600000000000, flags:
Name: /VirtualBox/GuestInfo/Net/Count, value: 1, timestamp: 1600000000000, flags:
Name: /VirtualBox/GuestInfo/Net/0/V4/IP, value: 192.168.1.42, timestamp: 1600000000000, flags:
Name: /VirtualBox/GuestInfo/Net/0/MAC, value: 0800271122AA, timestamp: 1600000000000, flags:`

	ga, byMAC := parseGuestNetworks(output)
	if !ga {
		t.Fatal("expected Guest Additions to be reported present")
	}
	got := byMAC["0800271122AA"]
	if !reflect.DeepEqual(got, []string{"192.168.1.42"}) {
		t.Fatalf("expected IP correlated to MAC, got %#v", got)
	}
}

// VBox 7.x `guestproperty enumerate --patterns` prints the terse
// "<name> = 'value' @ <timestamp> <flags>" form, not the verbose "Name:/value:".
func TestParseGuestNetworks_TerseFormat(t *testing.T) {
	output := `/VirtualBox/GuestInfo/Net/0/MAC          = '080027D0F71C'   @ 2026-07-10T05:24:52.422Z TRANSIENT, TRANSRESET
/VirtualBox/GuestInfo/Net/0/V4/IP        = '192.168.0.19'   @ 2026-07-10T05:24:52.421Z TRANSIENT, TRANSRESET
/VirtualBox/GuestInfo/OS/Product         = 'Linux'          @ 2026-07-10T05:24:42.388Z`

	ga, byMAC := parseGuestNetworks(output)
	if !ga {
		t.Fatal("expected Guest Additions present for terse-format output")
	}
	if got := byMAC["080027D0F71C"]; !reflect.DeepEqual(got, []string{"192.168.0.19"}) {
		t.Fatalf("expected IP correlated to MAC, got %#v", got)
	}
}

func TestParseGuestNetworks_NoGuestAdditions(t *testing.T) {
	ga, byMAC := parseGuestNetworks("")
	if ga {
		t.Error("expected GA absent for empty output")
	}
	if len(byMAC) != 0 {
		t.Errorf("expected no IPs, got %#v", byMAC)
	}
}

func TestParseGuestNetworks_SkipsUnsetIP(t *testing.T) {
	output := `Name: /VirtualBox/GuestInfo/Net/0/V4/IP, value: 0.0.0.0, timestamp: 1, flags:
Name: /VirtualBox/GuestInfo/Net/0/MAC, value: 080027A1B2C3, timestamp: 1, flags:`

	_, byMAC := parseGuestNetworks(output)
	if len(byMAC) != 0 {
		t.Errorf("expected 0.0.0.0 to be skipped, got %#v", byMAC)
	}
}

func TestParseDiskAttachments_SkipsDvdAndEmpty(t *testing.T) {
	output := `storagecontrollername0="SATA"
"SATA-0-0"="C:\Users\x\VirtualBox VMs\lab\lab.vdi"
"SATA-ImageUUID-0-0"="6f3c1e2a-1111-2222-3333-444455556666"
"SATA-1-0"="C:\isos\ubuntu.iso"
"SATA-2-0"="emptydrive"`

	disks := parseDiskAttachments(output)
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk (ISO and empty skipped), got %d: %#v", len(disks), disks)
	}
	if disks[0].name != "lab.vdi" {
		t.Errorf("expected name lab.vdi, got %q", disks[0].name)
	}
	if disks[0].uuid != "6f3c1e2a-1111-2222-3333-444455556666" {
		t.Errorf("expected medium UUID, got %q", disks[0].uuid)
	}
}

func TestParseMediumInfo(t *testing.T) {
	output := `UUID:           6f3c1e2a-1111-2222-3333-444455556666
Parent UUID:    base
State:          created
Type:           normal (base)
Location:       C:\lab\lab.vdi
Storage format: VDI
Capacity:       51200 MBytes
Size on disk:   12040 MBytes`

	capacity, allocated := parseMediumInfo(output)
	if capacity != 51200*1024*1024 {
		t.Errorf("expected capacity 51200 MBytes in bytes, got %d", capacity)
	}
	if allocated != 12040*1024*1024 {
		t.Errorf("expected allocated 12040 MBytes in bytes, got %d", allocated)
	}
}

func TestParseSizeWithUnit(t *testing.T) {
	cases := map[string]int64{
		"51200 MBytes": 51200 * 1024 * 1024,
		"2 GBytes":     2 * 1024 * 1024 * 1024,
		"1048576":      1048576, // bare number = bytes
	}
	for in, want := range cases {
		if got := parseSizeWithUnit(in); got != want {
			t.Errorf("parseSizeWithUnit(%q)=%d, want %d", in, got, want)
		}
	}
}

func TestNormalizeMAC(t *testing.T) {
	cases := map[string]string{
		"08:00:27:a1:b2:c3": "080027A1B2C3",
		"08-00-27-A1-B2-C3": "080027A1B2C3",
		"080027a1b2c3":      "080027A1B2C3",
	}
	for in, want := range cases {
		if got := normalizeMAC(in); got != want {
			t.Errorf("normalizeMAC(%q)=%q, want %q", in, got, want)
		}
	}
}
