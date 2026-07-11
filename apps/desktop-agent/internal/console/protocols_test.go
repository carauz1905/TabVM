package console

import "testing"

func TestCapabilities_IncludesRdpVncSsh(t *testing.T) {
	caps := Capabilities()

	if len(caps) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(caps))
	}

	found := make(map[Protocol]Capability)
	for _, c := range caps {
		found[c.ID] = c
	}

	if _, ok := found[RDP]; !ok {
		t.Fatal("expected RDP capability")
	}
	if _, ok := found[VNC]; !ok {
		t.Fatal("expected VNC capability")
	}
	if _, ok := found[SSH]; !ok {
		t.Fatal("expected SSH capability")
	}
}

func TestCapabilities_RdpIsAutoConfigured(t *testing.T) {
	for _, c := range Capabilities() {
		if c.ID == RDP && !c.CanAutoConfigure {
			t.Fatal("expected RDP to be auto-configured")
		}
		if c.ID == VNC && c.CanAutoConfigure {
			t.Fatal("expected VNC not to be auto-configured")
		}
		if c.ID == SSH && c.CanAutoConfigure {
			t.Fatal("expected SSH not to be auto-configured")
		}
	}
}

func TestCapabilities_StableValues(t *testing.T) {
	a := Capabilities()
	b := Capabilities()

	if len(a) != len(b) {
		t.Fatal("capabilities length should be stable")
	}

	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("capability %d not stable: %+v vs %+v", i, a[i], b[i])
		}
	}
}
