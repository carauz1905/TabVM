package vbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tabvm/desktop-agent/internal/models"
)

func TestParseCreateVmOutput(t *testing.T) {
	out := `Virtual machine 'lab-vm' is created and registered.
UUID: 12345678-1234-1234-1234-1234567890ab
Settings file: '/home/u/VirtualBox VMs/lab-vm/lab-vm.vbox'
`
	uuid, settings := parseCreateVmOutput(out)
	if uuid != "12345678-1234-1234-1234-1234567890ab" {
		t.Errorf("unexpected uuid: %q", uuid)
	}
	if settings != "/home/u/VirtualBox VMs/lab-vm/lab-vm.vbox" {
		t.Errorf("unexpected settings file: %q", settings)
	}
}

func TestValidateVmName(t *testing.T) {
	valid := []string{"lab", "Kali 2025", "vm_1.test-2"}
	for _, n := range valid {
		if err := validateVmName(n); err != nil {
			t.Errorf("expected %q valid, got %v", n, err)
		}
	}
	invalid := []string{"", "bad/name", "a;rm -rf", strings.Repeat("x", 65)}
	for _, n := range invalid {
		if err := validateVmName(n); err == nil {
			t.Errorf("expected %q invalid", n)
		}
	}
}

func TestValidateHostFileExtAndExistence(t *testing.T) {
	dir := t.TempDir()
	iso := filepath.Join(dir, "installer.iso")
	if err := os.WriteFile(iso, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := validateIsoPath(iso); err != nil {
		t.Errorf("expected valid iso, got %v", err)
	}
	// Wrong extension.
	txt := filepath.Join(dir, "installer.txt")
	_ = os.WriteFile(txt, []byte("x"), 0o600)
	if err := validateIsoPath(txt); err == nil {
		t.Error("expected wrong-extension rejection")
	}
	// Missing file.
	if err := validateIsoPath(filepath.Join(dir, "nope.iso")); err == nil {
		t.Error("expected missing-file rejection")
	}
	// Traversal.
	if err := validateIsoPath(filepath.Join(dir, "..", "x.iso")); err == nil {
		t.Error("expected traversal rejection")
	}
	// Relative path.
	if err := validateIsoPath("installer.iso"); err == nil {
		t.Error("expected relative-path rejection")
	}
}

func TestValidateAppliancePath(t *testing.T) {
	dir := t.TempDir()
	ova := filepath.Join(dir, "kali.ova")
	_ = os.WriteFile(ova, []byte("x"), 0o600)
	if err := validateAppliancePath(ova); err != nil {
		t.Errorf("expected valid ova, got %v", err)
	}
	iso := filepath.Join(dir, "kali.iso")
	_ = os.WriteFile(iso, []byte("x"), 0o600)
	if err := validateAppliancePath(iso); err == nil {
		t.Error("expected .iso rejected for appliance path")
	}
}

func TestHostnameFor(t *testing.T) {
	cases := map[string]string{
		"Kali 2025": "kali-2025.tabvm.lab",
		"":          "tabvm.tabvm.lab", // derived from empty -> fallback base
	}
	// When hostname empty, it derives from the name argument.
	if got := hostnameFor("", "Lab VM"); got != "lab-vm.tabvm.lab" {
		t.Errorf("derive from name: got %q", got)
	}
	for in, want := range cases {
		if got := hostnameFor(in, "fallback"); got != want {
			// The empty case uses the name fallback ("fallback"), so adjust:
			if in == "" {
				if got != "fallback.tabvm.lab" {
					t.Errorf("hostnameFor(%q): got %q", in, got)
				}
				continue
			}
			t.Errorf("hostnameFor(%q): got %q want %q", in, got, want)
		}
	}
}

func TestUnattendedInstallArgs(t *testing.T) {
	req := models.VmCreateRequest{
		Name:     "lab",
		OsType:   "Ubuntu_64",
		IsoPath:  "/iso/ubuntu.iso",
		Username: "student",
		Password: "secretpw",
	}
	args := unattendedInstallArgs("uuid-1", req, "/tmp/pw.txt")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"unattended install uuid-1",
		"--iso=/iso/ubuntu.iso",
		"--user=student",
		"--password-file=/tmp/pw.txt",
		"--install-additions",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q; got %q", want, joined)
		}
	}
	// The password itself must never be an argument.
	for _, a := range args {
		if a == "secretpw" {
			t.Error("password leaked into argv")
		}
	}
}

func TestCreateDiskArgsSizeIsMB(t *testing.T) {
	args := createDiskArgs("/vms/lab.vdi", 20)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--size 20480") {
		t.Errorf("expected 20 GB -> 20480 MB, got %q", joined)
	}
}
