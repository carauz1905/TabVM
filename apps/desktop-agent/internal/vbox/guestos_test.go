package vbox

import "testing"

func TestParseGuestOSType(t *testing.T) {
	machineReadable := `name="lab"
ostype="Ubuntu_64"
VMState="running"
`
	if got := parseGuestOSType(machineReadable); got != "Ubuntu_64" {
		t.Fatalf("expected Ubuntu_64, got %q", got)
	}

	if got := parseGuestOSType(`name="lab"` + "\n"); got != "" {
		t.Fatalf("expected empty when ostype absent, got %q", got)
	}
}

func TestGuestFamily(t *testing.T) {
	cases := map[string]string{
		"Ubuntu_64":       "linux",
		"Ubuntu22_LTS_64": "linux",
		"Debian12_64":     "linux",
		"Fedora_64":       "linux",
		"ArchLinux_64":    "linux",
		"openSUSE_64":     "linux",
		"RedHat_64":       "linux",
		"Oracle_64":       "linux",
		"Linux26_64":      "linux",
		"Gentoo_64":       "linux",
		"Windows11_64":    "windows",
		"Windows10_64":    "windows",
		"WindowsXP":       "windows",
		"Windows2019_64":  "windows",
		"MacOS_64":        "other",
		"Solaris_64":      "other",
		"FreeBSD_64":      "other",
		"OS2Warp45":       "other",
		"":                "",
	}
	for osType, want := range cases {
		if got := guestFamily(osType); got != want {
			t.Errorf("guestFamily(%q) = %q, want %q", osType, got, want)
		}
	}
}

func TestGuestTerminalCapable(t *testing.T) {
	if !guestTerminalCapable("linux") {
		t.Error("linux guests should be terminal-capable")
	}
	for _, family := range []string{"windows", "other", ""} {
		if guestTerminalCapable(family) {
			t.Errorf("family %q should not be terminal-capable", family)
		}
	}
}
