package vbox

import (
	"context"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tabvm/desktop-agent/internal/runner"
)

// guestControlRunner is a runner that dispatches on the VBoxManage subcommand so
// the full guest-control flows can be exercised even though the temp credential
// file path (and therefore the exact argv) is not predictable up front. It also
// records every argv it was asked to run so tests can assert on the issued
// command without knowing the random --passwordfile path.
type guestControlRunner struct {
	version  string
	vmState  string
	copyFrom runner.Result
	run      runner.Result
	calls    [][]string
}

func (g *guestControlRunner) RunContext(_ context.Context, _ string, args []string, _ time.Duration) (runner.Result, error) {
	g.calls = append(g.calls, args)
	switch {
	case len(args) == 1 && args[0] == "--version":
		return runner.Result{ExitCode: 0, StandardOutput: g.version}, nil
	case len(args) >= 1 && args[0] == "showvminfo":
		return runner.Result{ExitCode: 0, StandardOutput: `VMState="` + g.vmState + `"`}, nil
	case slices.Contains(args, "copyfrom"):
		return g.copyFrom, nil
	case slices.Contains(args, "run"):
		return g.run, nil
	default:
		return runner.Result{ExitCode: 1, StandardError: "unexpected command"}, nil
	}
}

func (g *guestControlRunner) lastMatching(sub string) ([]string, bool) {
	for i := len(g.calls) - 1; i >= 0; i-- {
		if slices.Contains(g.calls[i], sub) {
			return g.calls[i], true
		}
	}
	return nil, false
}

func TestGuestControlCopyFromArgs(t *testing.T) {
	args := guestControlCopyFromArgs("vm-1", "root", "/tmp/pw", "/home/root/report.txt", `C:\dst\report.txt`)

	// Credentials travel via --passwordfile, never as an argv value, and the
	// guest source precedes the host destination (symmetric with copyto).
	want := []string{
		"guestcontrol", "vm-1",
		"--username", "root",
		"--passwordfile", "/tmp/pw",
		"copyfrom", "/home/root/report.txt", `C:\dst\report.txt`,
	}
	if !slices.Equal(args, want) {
		t.Fatalf("guestControlCopyFromArgs = %v, want %v", args, want)
	}
}

func TestGuestControlRunArgs(t *testing.T) {
	args := guestControlRunArgs("vm-1", "root", "/tmp/pw", "/bin/ls", []string{"-la", "/home"})

	want := []string{
		"guestcontrol", "vm-1",
		"--username", "root",
		"--passwordfile", "/tmp/pw",
		"run",
		"--exe", "/bin/ls",
		"--timeout", "60000",
		"--wait-stdout",
		"--", "-la", "/home",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("guestControlRunArgs = %v, want %v", args, want)
	}
	// The program name is NOT repeated after "--": on VirtualBox 7.x `run` sets
	// argv[0] to --exe itself and treats tokens after "--" as argv[1..] (matching
	// the existing mkdir / sh -c arg builders in this package).
	for i, a := range args {
		if a == "--" && i+1 < len(args) && args[i+1] == "/bin/ls" {
			t.Fatalf("program name must not be repeated after --: %v", args)
		}
	}
}

func TestValidateGuestPath(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"absolute", "/home/user/report.txt", false},
		{"root file", "/etc/hostname", false},
		{"empty", "   ", true},
		{"relative", "home/user/report.txt", true},
		{"traversal", "/home/../etc/passwd", true},
		{"control char", "/home/user/a\x00b", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGuestPath(tc.in)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q", tc.in)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
		})
	}
}

func TestValidateGuestExe(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"absolute", "/bin/ls", false},
		{"empty", "  ", true},
		{"relative", "ls", true},
		{"comma", "/bin/l,s", true},
		{"control char", "/bin/l\ts", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGuestExe(tc.in)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q", tc.in)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
		})
	}
}

func TestCapGuestOutput(t *testing.T) {
	small := "hello world"
	if out, truncated := capGuestOutput(small); out != small || truncated {
		t.Fatalf("small output must pass through unchanged, got %q truncated=%v", out, truncated)
	}

	big := strings.Repeat("a", maxGuestOutputBytes+5000)
	out, truncated := capGuestOutput(big)
	if !truncated {
		t.Fatal("expected large output to be truncated")
	}
	if len(out) > maxGuestOutputBytes+64 {
		t.Fatalf("truncated output too long: %d bytes", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Fatalf("truncated output must carry a marker, got tail %q", out[len(out)-40:])
	}
}

func TestCopyFromGuest_RequiresCredentials(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	resp, err := svc.CopyFromGuest(context.Background(), "11111111-1111-1111-1111-111111111111", "/home/root/a.txt", `C:\dst`, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Success || !resp.CredentialsRequired {
		t.Fatalf("expected a needs-credentials response, got %+v", resp)
	}
}

func TestCopyFromGuest_RejectsInvalidGuestPath(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.CopyFromGuest(context.Background(), "11111111-1111-1111-1111-111111111111", "relative/path.txt", dir, "root", "secret")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a relative guest path, got %T (%v)", err, err)
	}
}

func TestCopyFromGuest_RejectsInvalidHostDir(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.CopyFromGuest(context.Background(), "11111111-1111-1111-1111-111111111111", "/home/root/a.txt", "relative/dir", "root", "secret")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a relative host dir, got %T (%v)", err, err)
	}
}

func TestCopyFromGuest_RefusesClobber(t *testing.T) {
	dir := t.TempDir()
	// Pre-create the destination file so the no-clobber guard fires.
	existing := filepath.Join(dir, "report.txt")
	if err := writeEmptyFile(existing); err != nil {
		t.Fatalf("failed to seed destination file: %v", err)
	}

	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.CopyFromGuest(context.Background(), "11111111-1111-1111-1111-111111111111", "/home/root/report.txt", dir, "root", "secret")
	if ve, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a clobber, got %T (%v)", err, err)
	} else if !strings.Contains(ve.Message, "already exists") {
		t.Fatalf("expected an already-exists message, got %q", ve.Message)
	}
}

func TestCopyFromGuest_HappyPathIssuesCopyFrom(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	dir := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &guestControlRunner{
		version:  "7.2.12r174389\n",
		vmState:  "running",
		copyFrom: runner.Result{ExitCode: 0},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.CopyFromGuest(context.Background(), id, "/home/root/report.txt", dir, "root", "secret-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	want := filepath.Join(dir, "report.txt")
	if resp.HostPath != want {
		t.Fatalf("expected host path %q, got %q", want, resp.HostPath)
	}
	if !strings.Contains(resp.Message, want) {
		t.Fatalf("expected the written path in the message, got %q", resp.Message)
	}

	args, ok := run.lastMatching("copyfrom")
	if !ok {
		t.Fatal("expected a copyfrom command to be issued")
	}
	if !slices.Contains(args, "--passwordfile") {
		t.Fatalf("expected --passwordfile in the command, got %v", args)
	}
	if !slices.Contains(args, "/home/root/report.txt") || !slices.Contains(args, want) {
		t.Fatalf("expected guest source and host destination in the command, got %v", args)
	}
	for _, a := range args {
		if strings.Contains(a, "secret-password") {
			t.Fatalf("password leaked into argv: %q", a)
		}
	}
}

func TestRunInGuest_RequiresCredentials(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	resp, err := svc.RunInGuest(context.Background(), "11111111-1111-1111-1111-111111111111", "/bin/ls", nil, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Success || !resp.CredentialsRequired {
		t.Fatalf("expected a needs-credentials response, got %+v", resp)
	}
}

func TestRunInGuest_RejectsInvalidExe(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.RunInGuest(context.Background(), "11111111-1111-1111-1111-111111111111", "ls", nil, "root", "secret")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a relative exe, got %T (%v)", err, err)
	}
}

func TestRunInGuest_HappyPathReturnsExitCodeAndOutput(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &guestControlRunner{
		version: "7.2.12r174389\n",
		vmState: "running",
		run:     runner.Result{ExitCode: 0, StandardOutput: "hello\n"},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.RunInGuest(context.Background(), id, "/bin/echo", []string{"hello"}, "root", "secret-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success || resp.ExitCode != 0 {
		t.Fatalf("expected success with exit code 0, got %+v", resp)
	}
	if !strings.Contains(resp.Output, "hello") {
		t.Fatalf("expected captured output, got %q", resp.Output)
	}

	args, ok := run.lastMatching("run")
	if !ok {
		t.Fatal("expected a run command to be issued")
	}
	if !slices.Contains(args, "--exe") || !slices.Contains(args, "/bin/echo") {
		t.Fatalf("expected --exe and the program in the command, got %v", args)
	}
	for _, a := range args {
		if strings.Contains(a, "secret-password") {
			t.Fatalf("password leaked into argv: %q", a)
		}
	}
}

func TestRunInGuest_ReturnsGuestExitCode(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	// A non-zero *guest* exit is a completed run, not a transport failure: the
	// service reports it as a success with the guest's exit code, not an error.
	run := &guestControlRunner{
		version: "7.2.12r174389\n",
		vmState: "running",
		run:     runner.Result{ExitCode: 2, StandardOutput: "", StandardError: "grep: no match"},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.RunInGuest(context.Background(), id, "/bin/grep", []string{"x", "/etc/hostname"}, "root", "secret-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("a completed guest command must be a success, got %+v", resp)
	}
	if resp.ExitCode != 2 {
		t.Fatalf("expected guest exit code 2, got %d", resp.ExitCode)
	}
}

func TestRunInGuest_CapsOutput(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &guestControlRunner{
		version: "7.2.12r174389\n",
		vmState: "running",
		run:     runner.Result{ExitCode: 0, StandardOutput: strings.Repeat("a", maxGuestOutputBytes+5000)},
	}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.RunInGuest(context.Background(), id, "/bin/cat", []string{"/big"}, "root", "secret-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Truncated {
		t.Fatal("expected the output to be truncated")
	}
	if len(resp.Output) > maxGuestOutputBytes+64 {
		t.Fatalf("expected the output to be capped, got %d bytes", len(resp.Output))
	}
}

func TestRunInGuest_RejectsNotRunning(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &guestControlRunner{version: "7.2.12r174389\n", vmState: "poweroff"}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.RunInGuest(context.Background(), id, "/bin/ls", nil, "root", "secret")
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for a stopped VM, got %T (%v)", err, err)
	}
}
