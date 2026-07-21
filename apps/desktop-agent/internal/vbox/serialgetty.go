package vbox

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// gettyEnableScript enables and immediately starts a login getty on the guest's
// first serial port, detecting the init system so it works across most distros:
// systemd (systemctl) for the majority, and an /etc/inittab entry + init reload
// (kill -HUP 1) for busybox/sysvinit/Alpine/Devuan. It deliberately contains no
// single quotes so it can be wrapped in `sh -c '...'` under sudo without escaping.
const gettyEnableScript = "if command -v systemctl >/dev/null 2>&1; then " +
	"systemctl enable --now serial-getty@ttyS0.service; " +
	"elif [ -f /etc/inittab ]; then " +
	"grep -q ttyS0 /etc/inittab || echo ttyS0::respawn:/sbin/getty -L 115200 ttyS0 vt100 >> /etc/inittab; " +
	"kill -HUP 1; " +
	"else echo no supported init system found >&2; exit 1; fi"

// guestControlEnableGettyArgs runs the getty script directly as root. The
// password travels via --passwordfile, never in argv. Stderr is folded into
// stdout (2>&1) so a single --wait-stdout captures everything (combining
// --wait-stdout and --wait-stderr triggers VERR_DUPLICATE on this VBoxManage).
func guestControlEnableGettyArgs(id, username, pwFilePath string) []string {
	return []string{
		"guestcontrol", id,
		"--username", username,
		"--passwordfile", pwFilePath,
		"run",
		"--exe", "/bin/sh",
		"--timeout", "60000",
		"--wait-stdout",
		"--", "-c", gettyEnableScript + " 2>&1",
	}
}

// guestControlSudoEnableGettyArgs runs the getty script under sudo for a non-root
// account. sudo -S reads the password from stdin (the file copied into the guest
// by guestControlCopyPwArgs), never from argv; the copy is removed afterward
// regardless of exit code. The script is wrapped in `sh -c '...'` (safe because
// it has no single quotes) so the whole compound command runs as root.
func guestControlSudoEnableGettyArgs(id, username, pwFilePath string) []string {
	outer := "sudo -S -p '' /bin/sh -c '" + gettyEnableScript + "' < " + guestPwPath +
		" 2>&1; rc=$?; rm -f " + guestPwPath + "; exit $rc"
	return []string{
		"guestcontrol", id,
		"--username", username,
		"--passwordfile", pwFilePath,
		"run",
		"--exe", "/bin/sh",
		"--timeout", "60000",
		"--wait-stdout",
		"--", "-c", outer,
	}
}

// EnableSerialGetty turns on a login getty on the guest's serial port via guest
// control, so a serial console shows a real login prompt. It requires a running
// Linux guest with Guest Additions active and credentials for a root or sudo
// account. The credentials are used once and never stored.
func (s *service) EnableSerialGetty(ctx context.Context, id, username, password string) (models.SerialGettyResponse, error) {
	if !IsValidVmID(id) {
		return models.SerialGettyResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if strings.TrimSpace(username) == "" || password == "" {
		return models.SerialGettyResponse{}, &ValidationError{Message: "Guest username and password are required."}
	}
	if !isPlausibleGuestUsername(username) {
		return models.SerialGettyResponse{}, &ValidationError{Message: "Guest username contains unsupported characters."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "serial.getty", false, "VirtualBox/VBoxManage not discovered.")
		return models.SerialGettyResponse{}, err
	}

	// Reject non-Linux guests up front for a clearer message than a raw guest
	// control failure. Best-effort: if showvminfo cannot be read, proceed.
	if info, infoErr := s.readShowVmInfo(ctx, path, id, "reading guest OS before enabling getty"); infoErr == nil {
		if guestFamily(parseGuestOSType(info)) != "linux" {
			return models.SerialGettyResponse{}, &ValidationError{Message: "The serial terminal is only available for Linux guests."}
		}
	}

	pwFile, err := os.CreateTemp("", "tabvm-getty-*.txt")
	if err != nil {
		return models.SerialGettyResponse{}, fmt.Errorf("creating credential file: %w", err)
	}
	pwPath := pwFile.Name()
	defer os.Remove(pwPath)
	_ = pwFile.Chmod(0o600)
	// Trailing newline so `sudo -S` accepts the single stdin line; VBoxManage
	// --passwordfile trims trailing whitespace, so this is harmless there.
	if _, err := pwFile.WriteString(password + "\n"); err != nil {
		pwFile.Close()
		return models.SerialGettyResponse{}, fmt.Errorf("writing credential file: %w", err)
	}
	pwFile.Close()

	const failMsg = "Could not enable the serial login inside the guest. Check the username/password, that the account is root or has sudo, and that this is a running Linux guest with Guest Additions active."

	root := strings.EqualFold(username, "root")
	if !root {
		// Copy the password into the guest so the sudo -S path can read it from
		// stdin.
		if cp, cpErr := s.runForVM(ctx, id, path, guestControlCopyPwArgs(id, username, pwPath), 30*time.Second); cpErr != nil || cp.ExitCode != 0 {
			s.logOperation(ctx, id, "serial.getty", false, "Copying credentials into guest failed.")
			return models.SerialGettyResponse{
				Success: false,
				VMID:    id,
				Message: failMsg,
				Output:  combinedOutput(cp.StandardOutput, cp.StandardError),
			}, nil
		}
	}

	args := guestControlEnableGettyArgs(id, username, pwPath)
	if !root {
		args = guestControlSudoEnableGettyArgs(id, username, pwPath)
	}
	result, runErr := s.runForVM(ctx, id, path, args, 90*time.Second)
	if runErr != nil || result.ExitCode != 0 {
		s.logOperation(ctx, id, "serial.getty", false, "Enabling serial getty failed.")
		return models.SerialGettyResponse{
			Success: false,
			VMID:    id,
			Message: failMsg,
			Output:  combinedOutput(result.StandardOutput, result.StandardError),
		}, nil
	}

	s.logOperation(ctx, id, "serial.getty", true, "")
	return models.SerialGettyResponse{
		Success: true,
		VMID:    id,
		Message: "Serial login enabled. Open the terminal to connect.",
		Output:  combinedOutput(result.StandardOutput, result.StandardError),
	}, nil
}
