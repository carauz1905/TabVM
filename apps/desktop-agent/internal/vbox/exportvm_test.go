package vbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tabvm/desktop-agent/internal/runner"
)

func TestExportVmArgs(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	out := filepath.Join(`C:\out`, "lab-vm.ova")
	args := exportVmArgs(id, out)
	if strings.Join(args, " ") != "export "+id+" --output "+out {
		t.Fatalf("unexpected export args: %v", args)
	}
}

func TestSanitizeExportFileBase(t *testing.T) {
	cases := map[string]string{
		"lab-vm":           "lab-vm",
		"My Lab VM":        "My Lab VM",
		"bad/name\\here":   "badnamehere",
		"  spaced.  ":      "spaced",
		`weird:*?"<>|name`: "weirdname",
		"...":              "",
		"/\\":              "",
	}
	for in, want := range cases {
		if got := sanitizeExportFileBase(in); got != want {
			t.Fatalf("sanitizeExportFileBase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateExportDir(t *testing.T) {
	dir := t.TempDir()
	if err := validateExportDir(dir); err != nil {
		t.Fatalf("expected an existing absolute directory to validate, got %v", err)
	}

	if err := validateExportDir("   "); err == nil {
		t.Fatal("expected an empty directory to be rejected")
	}
	if err := validateExportDir("relative/dir"); err == nil {
		t.Fatal("expected a relative directory to be rejected")
	}

	// A '..' segment must be rejected. Build the raw path by hand so filepath.Join
	// does not clean the traversal away before validation sees it.
	sep := string(os.PathSeparator)
	if err := validateExportDir(dir + sep + ".." + sep + "evil"); err == nil {
		t.Fatal("expected a directory with '..' to be rejected")
	}

	if err := validateExportDir(dir + "\x01"); err == nil {
		t.Fatal("expected a directory with a control character to be rejected")
	}
	if err := validateExportDir(filepath.Join(dir, "does-not-exist")); err == nil {
		t.Fatal("expected a nonexistent directory to be rejected")
	}

	file := filepath.Join(dir, "afile")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateExportDir(file); err == nil {
		t.Fatal("expected a file path to be rejected as a directory")
	}
}

func TestExportVM_RejectsInvalidSourceID(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.ExportVM(context.Background(), "not-a-vm; rm -rf /", `C:\out`)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestExportVM_RejectsRelativeDirectory(t *testing.T) {
	// A valid source id but a relative directory must fail before any VBoxManage
	// call, so this passes even without a discovered VBoxManage.
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.ExportVM(context.Background(), "11111111-1111-1111-1111-111111111111", "relative/dir")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a relative directory, got %v", err)
	}
}

func TestExportVM_RefusesRunningSource(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	dir := t.TempDir()
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: "VMState=\"running\"\nname=\"lab-vm\""},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.ExportVM(context.Background(), id, dir)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a running source, got %v", err)
	}
}

func TestExportVM_HappyPathIssuesExportAndReturnsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	dir := t.TempDir()
	path := createTempExecutable(t)
	out := filepath.Join(dir, "lab-vm.ova")
	run := &recordingRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: "VMState=\"poweroff\"\nname=\"lab-vm\""},
			path + " export " + id + " --output " + out:       {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.ExportVM(context.Background(), id, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || !strings.Contains(resp.Message, out) {
		t.Fatalf("unexpected response: %+v (want the written path %q in the message)", resp, out)
	}
	joined := strings.Join(run.calls, "\n")
	if !strings.Contains(joined, "export "+id+" --output "+out) {
		t.Fatalf("expected the export command; calls:\n%s", joined)
	}
}

func TestExportVM_DerivesFilenameFromVmName(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	dir := t.TempDir()
	path := createTempExecutable(t)
	// A name with a path separator must be sanitized into a safe filename base
	// before the .ova extension is appended.
	out := filepath.Join(dir, "Kali Linux 20241.ova")
	run := &recordingRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: "VMState=\"poweroff\"\nname=\"Kali Linux 2024/1\""},
			path + " export " + id + " --output " + out:       {ExitCode: 0},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if _, err := svc.ExportVM(context.Background(), id, dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(run.calls, "\n")
	if !strings.Contains(joined, "export "+id+" --output "+out) {
		t.Fatalf("expected export with a sanitized filename; calls:\n%s", joined)
	}
}

func TestExportVM_RejectsExistingTargetFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	dir := t.TempDir()
	// Pre-create the derived target so the export would clobber it.
	if err := os.WriteFile(filepath.Join(dir, "lab-vm.ova"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := createTempExecutable(t)
	run := &recordingRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: "VMState=\"poweroff\"\nname=\"lab-vm\""},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.ExportVM(context.Background(), id, dir)
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for an existing target file, got %v", err)
	}
	// It must not have attempted the export.
	if strings.Contains(strings.Join(run.calls, "\n"), "export "+id) {
		t.Fatalf("export must not run when the target file already exists; calls:\n%s", strings.Join(run.calls, "\n"))
	}
}

func TestValidateExport_RefusesRunningSource(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	dir := t.TempDir()
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: "VMState=\"running\"\nname=\"lab-vm\""},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if err := svc.ValidateExport(context.Background(), id, dir); err == nil {
		t.Fatal("expected ValidateExport to refuse a running source")
	} else if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestValidateExport_AcceptsStoppedSource(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	dir := t.TempDir()
	path := createTempExecutable(t)
	run := &fakeRunner{
		results: map[string]runner.Result{
			path + " --version": {ExitCode: 0, StandardOutput: "7.0.14r161095\n"},
			path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: "VMState=\"poweroff\"\nname=\"lab-vm\""},
		},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	if err := svc.ValidateExport(context.Background(), id, dir); err != nil {
		t.Fatalf("expected a stopped source with a valid directory to validate, got %v", err)
	}
}
