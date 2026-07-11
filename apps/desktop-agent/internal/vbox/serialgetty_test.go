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
		"--", "-c", gettyServiceCommand + " 2>&1",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("guestControlEnableGettyArgs = %v, want %v", args, want)
	}
	if gettyServiceCommand != "systemctl enable --now serial-getty@ttyS0.service" {
		t.Fatalf("unexpected getty command: %q", gettyServiceCommand)
	}
}

func TestGuestControlSudoEnableGettyArgs_NonRoot(t *testing.T) {
	args := guestControlSudoEnableGettyArgs("vm-1", "alice", "/tmp/pw")
	last := args[len(args)-1]

	if !strings.Contains(last, "sudo -S -p ''") {
		t.Errorf("expected sudo -S with no prompt, got %q", last)
	}
	if !strings.Contains(last, gettyServiceCommand) {
		t.Errorf("expected the systemctl getty command, got %q", last)
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
