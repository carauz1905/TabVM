package vbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/runner"
)

// recordingRunner behaves like fakeRunner but also records every command key,
// so tests can assert step order and the absence of steps (e.g. no unattended
// install during a manual create).
type recordingRunner struct {
	results map[string]runner.Result
	calls   []string
}

func (r *recordingRunner) RunContext(ctx context.Context, name string, args []string, timeout time.Duration) (runner.Result, error) {
	key := name + " " + joinArgs(args)
	r.calls = append(r.calls, key)
	if result, ok := r.results[key]; ok {
		return result, nil
	}
	return runner.Result{ExitCode: 1, StandardError: "unexpected command: " + key}, nil
}

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

func TestStorageAttachDvdArgs(t *testing.T) {
	args := storageAttachDvdArgs("uuid-1", "/iso/alpine.iso")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"storageattach uuid-1",
		"--storagectl SATA",
		"--port 1",
		"--device 0",
		"--type dvddrive",
		"--medium /iso/alpine.iso",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q; got %q", want, joined)
		}
	}
}

func TestCreateVmManual_HappyPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	path := createTempExecutable(t)
	dir := t.TempDir()
	iso := filepath.Join(dir, "alpine.iso")
	if err := os.WriteFile(iso, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	uuid := "12345678-1234-1234-1234-1234567890ab"
	settings := filepath.Join(dir, "lab", "lab.vbox")
	disk := filepath.Join(dir, "lab", "lab.vdi")

	run := &recordingRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " createvm --name lab --ostype Linux_64 --register": {
				ExitCode:       0,
				StandardOutput: "UUID: " + uuid + "\nSettings file: '" + settings + "'\n",
			},
			path + " modifyvm " + uuid + " --memory 2048 --cpus 2 --ioapic on --nic1 nat --vram 33 --graphicscontroller vmsvga": {ExitCode: 0},
			path + " createmedium disk --filename " + disk + " --size 20480 --format VDI":                                       {ExitCode: 0},
			path + " storagectl " + uuid + " --name SATA --add sata --controller IntelAhci --portcount 2 --bootable on":         {ExitCode: 0},
			path + " storageattach " + uuid + " --storagectl SATA --port 0 --device 0 --type hdd --medium " + disk:              {ExitCode: 0},
			path + " storageattach " + uuid + " --storagectl SATA --port 1 --device 0 --type dvddrive --medium " + iso:          {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.CreateVmManual(context.Background(), models.VmCreateManualRequest{
		Name:     "lab",
		OsType:   "Linux_64",
		IsoPath:  iso,
		MemoryMB: 2048,
		Cpus:     2,
		DiskGB:   20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.VMID != uuid || resp.Name != "lab" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !strings.Contains(resp.Message, "install the OS from the attached ISO") {
		t.Errorf("unexpected message: %q", resp.Message)
	}

	joinedCalls := strings.Join(run.calls, "\n")
	if !strings.Contains(joinedCalls, "--type dvddrive --medium "+iso) {
		t.Errorf("expected ISO attached as dvddrive; calls:\n%s", joinedCalls)
	}
	// Manual mode must never run an unattended install or handle credentials.
	for _, banned := range []string{"unattended", "--password-file", "--user="} {
		if strings.Contains(joinedCalls, banned) {
			t.Errorf("manual create ran forbidden step %q; calls:\n%s", banned, joinedCalls)
		}
	}
	// The disk attach must come before the DVD attach so the boot order stays
	// disk-first once the install is done.
	diskIdx, dvdIdx := -1, -1
	for i, c := range run.calls {
		if strings.Contains(c, "--type hdd") {
			diskIdx = i
		}
		if strings.Contains(c, "--type dvddrive") {
			dvdIdx = i
		}
	}
	if diskIdx == -1 || dvdIdx == -1 || diskIdx > dvdIdx {
		t.Errorf("expected disk attach before dvd attach; calls:\n%s", joinedCalls)
	}
}

func TestCreateVmManual_RejectsUnsupportedOsType(t *testing.T) {
	dir := t.TempDir()
	iso := filepath.Join(dir, "alpine.iso")
	if err := os.WriteFile(iso, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := NewService(&fakeRunner{}, Config{})

	// Unattended-only and unknown types are both rejected for manual installs.
	for _, osType := range []string{"Ubuntu_64", "Windows11_64", "Kali_64", ""} {
		_, err := svc.CreateVmManual(context.Background(), models.VmCreateManualRequest{
			Name:     "lab",
			OsType:   osType,
			IsoPath:  iso,
			MemoryMB: 2048,
			Cpus:     2,
			DiskGB:   20,
		})
		vErr, ok := err.(*ValidationError)
		if !ok {
			t.Errorf("osType %q: expected ValidationError, got %v", osType, err)
			continue
		}
		if vErr.Message != "Unsupported OS type for manual install." {
			t.Errorf("osType %q: unexpected message %q", osType, vErr.Message)
		}
	}
}

func TestCreateVmManual_RejectsMissingIso(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.CreateVmManual(context.Background(), models.VmCreateManualRequest{
		Name:     "lab",
		OsType:   "Linux_64",
		IsoPath:  filepath.Join(t.TempDir(), "nope.iso"),
		MemoryMB: 2048,
		Cpus:     2,
		DiskGB:   20,
	})
	if _, ok := err.(*ValidationError); !ok {
		t.Errorf("expected ValidationError for missing iso, got %v", err)
	}
}

func TestCreateVmManual_RejectsOutOfRangeHardware(t *testing.T) {
	dir := t.TempDir()
	iso := filepath.Join(dir, "alpine.iso")
	if err := os.WriteFile(iso, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := NewService(&fakeRunner{}, Config{})

	base := models.VmCreateManualRequest{
		Name: "lab", OsType: "Linux_64", IsoPath: iso, MemoryMB: 2048, Cpus: 2, DiskGB: 20,
	}
	cases := []struct {
		mutate func(*models.VmCreateManualRequest)
		desc   string
	}{
		{func(r *models.VmCreateManualRequest) { r.MemoryMB = 256 }, "memory too low"},
		{func(r *models.VmCreateManualRequest) { r.MemoryMB = 131072 }, "memory too high"},
		{func(r *models.VmCreateManualRequest) { r.Cpus = 0 }, "cpus too low"},
		{func(r *models.VmCreateManualRequest) { r.Cpus = 32 }, "cpus too high"},
		{func(r *models.VmCreateManualRequest) { r.DiskGB = 4 }, "disk too small"},
		{func(r *models.VmCreateManualRequest) { r.DiskGB = 1024 }, "disk too large"},
	}
	for _, c := range cases {
		req := base
		c.mutate(&req)
		if _, err := svc.CreateVmManual(context.Background(), req); err == nil {
			t.Errorf("%s: expected validation error", c.desc)
		} else if _, ok := err.(*ValidationError); !ok {
			t.Errorf("%s: expected ValidationError, got %v", c.desc, err)
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
