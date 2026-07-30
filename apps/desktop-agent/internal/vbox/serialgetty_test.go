package vbox

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// runGettyScript executes gettyEnableScript under a POSIX shell with a stub
// systemctl ahead of everything else on PATH, and reports the exit code plus the
// combined output.
//
// The script is executed rather than pattern-matched on purpose. What this
// guards against are control-flow defects -- a fallback branch that cannot be
// reached, and an exit code taken from the wrong command -- and a substring
// assertion sees neither. The test this replaces asserted the exact systemd
// command line and stayed green while three such defects shipped.
//
// Two literals are rewritten so a test run can never affect the machine running
// it: the inittab path becomes a temp file, and `kill -HUP 1`, which would
// signal the real init, becomes a no-op.
func runGettyScript(t *testing.T, systemctlStub, inittabPath string) (int, string) {
	t.Helper()

	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no POSIX sh on PATH to execute the guest script")
	}

	stubDir := t.TempDir()
	stub := filepath.Join(stubDir, "systemctl")
	if err := os.WriteFile(stub, []byte(systemctlStub), 0o755); err != nil {
		t.Fatalf("writing systemctl stub: %v", err)
	}

	// The shell needs a POSIX path even when the test runs on Windows.
	script := strings.ReplaceAll(gettyEnableScript, "/etc/inittab", filepath.ToSlash(inittabPath))
	script = strings.ReplaceAll(script, "kill -HUP 1", ":")

	cmd := exec.Command(sh, "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, runErr := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		return 0, string(out)
	case errors.As(runErr, &exitErr):
		return exitErr.ExitCode(), string(out)
	default:
		t.Skipf("could not run the guest script under %s: %v", sh, runErr)
		return 0, ""
	}
}

func TestGuestControlEnableGettyArgs_Root(t *testing.T) {
	args := guestControlEnableGettyArgs("vm-1", "root", "/tmp/pw")

	// Credentials travel via --passwordfile, never as an argv value.
	want := []string{
		"guestcontrol", "vm-1",
		"--username", "root",
		"--passwordfile", "/tmp/pw",
		"run",
		"--exe", "/bin/sh",
		"--timeout", "60000",
		"--wait-stdout",
		"--", "-c", gettyEnableScript + " 2>&1",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("guestControlEnableGettyArgs = %v, want %v", args, want)
	}
}

func TestGettyEnableScript_CoversInitSystems(t *testing.T) {
	// The script must handle both systemd and inittab-based inits, and must
	// contain no single quotes so it can be wrapped in sh -c '...' under sudo.
	if !strings.Contains(gettyEnableScript, "systemctl start serial-getty@ttyS0.service") {
		t.Error("script must cover systemd")
	}
	if !strings.Contains(gettyEnableScript, "/etc/inittab") {
		t.Error("script must cover inittab-based inits")
	}
	if strings.Contains(gettyEnableScript, "'") {
		t.Errorf("script must not contain single quotes (breaks sudo sh -c wrapping): %q", gettyEnableScript)
	}
	// `systemctl enable --now` only exists from systemd 220 on. Older guests --
	// CentOS 7 ships systemd 208 -- answer it with "unrecognized option".
	if strings.Contains(gettyEnableScript, "--now") {
		t.Error("script must not use --now: it does not exist before systemd 220")
	}
}

// systemd 208, as shipped by CentOS 7: `enable` refuses the unit because
// serial-getty@.service is static there (no [Install] section), while `start`
// works fine. Enabling only buys persistence across reboots; starting is what
// actually gives the user a login prompt, so the outcome must follow `start`.
const stubSystemctlStaticUnit = `#!/bin/sh
case "$1" in
  enable) echo "The unit files have no [Install] section." >&2; exit 1 ;;
  start) exit 0 ;;
esac
exit 2
`

// systemd is present but unusable, e.g. a broken or unreachable bus.
const stubSystemctlAlwaysFails = `#!/bin/sh
echo "Failed to connect to bus: No such file or directory" >&2
exit 1
`

func TestGettyEnableScript_SucceedsWhenOnlyEnableFails(t *testing.T) {
	inittab := filepath.Join(t.TempDir(), "inittab")
	if err := os.WriteFile(inittab, []byte("# empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := runGettyScript(t, stubSystemctlStaticUnit, inittab)

	if code != 0 {
		t.Errorf("a started getty must report success even when enable fails; exit=%d output=%q", code, out)
	}
	// The systemd path succeeded, so the inittab fallback must not have run.
	after, err := os.ReadFile(inittab)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "ttyS0") {
		t.Errorf("inittab must be left alone when systemd handled it, got %q", after)
	}
}

func TestGettyEnableScript_FallsBackWhenSystemdIsPresentButFails(t *testing.T) {
	inittab := filepath.Join(t.TempDir(), "inittab")
	if err := os.WriteFile(inittab, []byte("# empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := runGettyScript(t, stubSystemctlAlwaysFails, inittab)

	// The fallback used to sit behind `elif`, guarded by whether systemctl
	// exists at all, so a present-but-failing systemd skipped it entirely.
	after, err := os.ReadFile(inittab)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "ttyS0::respawn") {
		t.Errorf("inittab fallback must run when the systemd path fails, got %q", after)
	}
	if code != 0 {
		t.Errorf("the inittab fallback must report success; exit=%d output=%q", code, out)
	}
}

// guestControlEnableGettyArgs sends the script as `<script> 2>&1`, which only
// folds stderr for the whole thing while the script stays one compound command.
// Split it into two statements and the redirect would silently apply to the last
// one alone, dropping every earlier diagnostic on the floor -- exactly the output
// the operator needs when this fails on their guest.
func TestGettyEnableScript_FoldsAllStderrIntoStdout(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no POSIX sh on PATH to execute the guest script")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte(stubSystemctlAlwaysFails), 0o755); err != nil {
		t.Fatal(err)
	}
	// A path that does not exist, so no init system is usable and the script
	// takes its failure branch.
	missingInittab := filepath.ToSlash(filepath.Join(dir, "absent-inittab"))

	script := strings.ReplaceAll(gettyEnableScript, "/etc/inittab", missingInittab)
	cmd := exec.Command(sh, "-c", script+" 2>&1")
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	stdout, _ := cmd.Output() // non-zero exit is expected here

	if !strings.Contains(string(stdout), "no supported init system found") {
		t.Errorf("stderr from inside the script must reach stdout via the appended 2>&1, got %q", stdout)
	}
}

func TestGuestControlSudoEnableGettyArgs_NonRoot(t *testing.T) {
	args := guestControlSudoEnableGettyArgs("vm-1", "alice", "/tmp/pw")
	last := args[len(args)-1]

	if !strings.Contains(last, "sudo -S -p '' /bin/sh -c '") {
		t.Errorf("expected sudo wrapping the script in sh -c, got %q", last)
	}
	if !strings.Contains(last, gettyEnableScript) {
		t.Errorf("expected the getty enable script, got %q", last)
	}
	if !strings.Contains(last, "rm -f "+guestPwPath) {
		t.Errorf("expected the copied password file to be removed, got %q", last)
	}
	// The password itself must never appear in argv.
	for _, a := range args {
		if strings.Contains(a, "secret-password") {
			t.Fatalf("password leaked into argv: %q", a)
		}
	}
}
