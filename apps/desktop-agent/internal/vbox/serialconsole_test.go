package vbox

import (
	"slices"
	"strings"
	"testing"
)

func TestSerialPipeName(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	got := SerialPipeName(id)
	want := `\\.\pipe\tabvm-serial-` + id
	if got != want {
		t.Fatalf("SerialPipeName = %q, want %q", got, want)
	}
}

func TestEnableSerialConsoleArgs(t *testing.T) {
	id := "vm-1"
	pipe := `\\.\pipe\tabvm-serial-vm-1`
	got := enableSerialConsoleArgs(id, pipe)
	want := []string{"modifyvm", id, "--uart1", "0x3F8", "4", "--uartmode1", "server", pipe}
	if !slices.Equal(got, want) {
		t.Fatalf("enableSerialConsoleArgs = %v, want %v", got, want)
	}
}

func TestDisableSerialConsoleArgs(t *testing.T) {
	got := disableSerialConsoleArgs("vm-1")
	want := []string{"modifyvm", "vm-1", "--uart1", "off"}
	if !slices.Equal(got, want) {
		t.Fatalf("disableSerialConsoleArgs = %v, want %v", got, want)
	}
}

func TestSerialConsoleEnabledFor(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	other := "22222222-2222-2222-2222-222222222222"
	info := func(pipe string) string {
		return `uart1="0x03f8,4"` + "\n" + `uartmode1="server,` + pipe + `"` + "\n"
	}

	if !serialConsoleEnabledFor(info(SerialPipeName(id)), id) {
		t.Error("a port wired to this VM's own pipe must count as enabled")
	}

	// VirtualBox copies the pipe path verbatim into clones and exported
	// appliances, so a VM can carry a path naming a different machine. Its
	// terminal would wait on a pipe nobody creates, so it is not enabled.
	if serialConsoleEnabledFor(info(SerialPipeName(other)), id) {
		t.Error("a port wired to another VM's pipe must not count as enabled")
	}

	// Windows named pipes are case-insensitive.
	if !serialConsoleEnabledFor(info(strings.ToUpper(SerialPipeName(id))), id) {
		t.Error("pipe comparison must be case-insensitive")
	}

	if serialConsoleEnabledFor(`uart1="off"`+"\n", id) {
		t.Error("an off port must not count as enabled")
	}
}

func TestParseSerialConsole(t *testing.T) {
	enabledOut := `uart1="0x03f8,4"` + "\n" + `uartmode1="server,\\.\pipe\tabvm-serial-abc"` + "\n"
	enabled, pipe := parseSerialConsole(enabledOut)
	if !enabled {
		t.Fatal("expected enabled for a configured server pipe")
	}
	if pipe != `\\.\pipe\tabvm-serial-abc` {
		t.Fatalf("pipe = %q, want the server pipe path", pipe)
	}

	if off, _ := parseSerialConsole(`uart1="off"` + "\n"); off {
		t.Fatal("expected disabled when uart1 is off")
	}

	if missing, _ := parseSerialConsole(`name="lab"` + "\n"); missing {
		t.Fatal("expected disabled when no uart is configured")
	}
}
