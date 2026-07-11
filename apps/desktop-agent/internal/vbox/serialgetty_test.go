package vbox

import (
	"slices"
	"strings"
	"testing"
)

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
	if !strings.Contains(gettyEnableScript, "systemctl enable --now serial-getty@ttyS0.service") {
		t.Error("script must cover systemd")
	}
	if !strings.Contains(gettyEnableScript, "/etc/inittab") {
		t.Error("script must cover inittab-based inits")
	}
	if strings.Contains(gettyEnableScript, "'") {
		t.Errorf("script must not contain single quotes (breaks sudo sh -c wrapping): %q", gettyEnableScript)
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
