package vbox

import "testing"

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
