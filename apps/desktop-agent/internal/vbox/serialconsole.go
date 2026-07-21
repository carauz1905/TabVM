package vbox

import (
	"context"
	"strings"

	"github.com/tabvm/desktop-agent/internal/models"
)

// SerialConsoleStatus reports whether a VM's COM1 serial console is wired to the
// host pipe, whether the guest is terminal-capable (Linux), whether it is
// running (the getty is only reachable on a live VM), and whether the serial
// port can currently be toggled (only on a powered-off VM).
func (s *service) SerialConsoleStatus(ctx context.Context, id string) (models.VmSerialConsoleResponse, error) {
	if !IsValidVmID(id) {
		return models.VmSerialConsoleResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmSerialConsoleResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading serial console status")
	if err != nil {
		return models.VmSerialConsoleResponse{}, err
	}

	enabled, _ := parseSerialConsole(info)
	family := guestFamily(parseGuestOSType(info))
	live := vmStateIsLive(parseVmState(info))
	return models.VmSerialConsoleResponse{
		ID:              id,
		Enabled:         enabled,
		TerminalCapable: guestTerminalCapable(family),
		Running:         live,
		Editable:        !live,
	}, nil
}

// EnableSerialConsole wires COM1 to a deterministic host named pipe in server
// mode. It is refused for non-Linux guests (the feature only targets Linux) and
// for a running VM (modifyvm only works on a powered-off machine).
func (s *service) EnableSerialConsole(ctx context.Context, id string) (models.VmOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "vm.serial.enable", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM before enabling serial console")
	if err != nil {
		return models.VmOperationResponse{}, err
	}
	if guestFamily(parseGuestOSType(info)) != "linux" {
		return models.VmOperationResponse{}, &ValidationError{Message: "The serial terminal is only available for Linux guests."}
	}
	if vmStateIsLive(parseVmState(info)) {
		return models.VmOperationResponse{}, &ValidationError{Message: "The VM is running. Power it off before enabling the serial terminal."}
	}

	if err := s.runControlCommand(ctx, id, path, enableSerialConsoleArgs(id, SerialPipeName(id)), "enabling serial console"); err != nil {
		s.logOperation(ctx, id, "vm.serial.enable", false, "VBoxManage modifyvm uart failed.")
		return models.VmOperationResponse{}, err
	}

	s.logOperation(ctx, id, "vm.serial.enable", true, "")
	return models.VmOperationResponse{
		Success: true,
		VMID:    id,
		Message: "Serial terminal enabled. Start the VM to use it.",
	}, nil
}

// DisableSerialConsole turns COM1 off. It is refused for a running VM.
func (s *service) DisableSerialConsole(ctx context.Context, id string) (models.VmOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "vm.serial.disable", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM before disabling serial console")
	if err != nil {
		return models.VmOperationResponse{}, err
	}
	if vmStateIsLive(parseVmState(info)) {
		return models.VmOperationResponse{}, &ValidationError{Message: "The VM is running. Power it off before disabling the serial terminal."}
	}

	if err := s.runControlCommand(ctx, id, path, disableSerialConsoleArgs(id), "disabling serial console"); err != nil {
		s.logOperation(ctx, id, "vm.serial.disable", false, "VBoxManage modifyvm uart off failed.")
		return models.VmOperationResponse{}, err
	}

	s.logOperation(ctx, id, "vm.serial.disable", true, "")
	return models.VmOperationResponse{
		Success: true,
		VMID:    id,
		Message: "Serial terminal disabled.",
	}, nil
}

// SerialPipeName returns the deterministic Windows named-pipe path used to
// bridge a VM's first serial port (COM1) to the host agent.
func SerialPipeName(id string) string {
	return `\\.\pipe\tabvm-serial-` + id
}

// enableSerialConsoleArgs builds the modifyvm command that wires COM1 (0x3F8,
// IRQ 4) to a host named pipe in server mode, so the agent can connect to it.
// The VM must be powered off for this to take effect.
func enableSerialConsoleArgs(id, pipeName string) []string {
	return []string{"modifyvm", id, "--uart1", "0x3F8", "4", "--uartmode1", "server", pipeName}
}

// disableSerialConsoleArgs builds the modifyvm command that turns COM1 off.
func disableSerialConsoleArgs(id string) []string {
	return []string{"modifyvm", id, "--uart1", "off"}
}

// parseSerialConsole reports whether COM1 is wired to a host pipe in server mode
// and returns that pipe path, from machine-readable showvminfo output. The
// relevant lines look like `uart1="0x03f8,4"` and `uartmode1="server,<pipe>"`.
func parseSerialConsole(output string) (enabled bool, pipe string) {
	var uart1, uartmode1 string
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadable(line)
		if !ok {
			continue
		}
		switch key {
		case "uart1":
			uart1 = value
		case "uartmode1":
			uartmode1 = value
		}
	}
	if uart1 == "" || strings.EqualFold(uart1, "off") {
		return false, ""
	}
	const serverPrefix = "server,"
	if !strings.HasPrefix(uartmode1, serverPrefix) {
		return false, ""
	}
	return true, strings.TrimPrefix(uartmode1, serverPrefix)
}
