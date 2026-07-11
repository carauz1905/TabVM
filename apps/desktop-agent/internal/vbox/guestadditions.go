package vbox

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/runner"
)

// gaVersionProperty is the guest property VirtualBox populates with the running
// Guest Additions version. It is only set while the guest is running with Guest
// Additions active, so it doubles as an installed/active signal.
const gaVersionProperty = "/VirtualBox/GuestAdd/Version"

// opticalSlot identifies a DVD drive attachment point on a VM: the storage
// controller name plus the port/device coordinates VBoxManage addresses it with.
// empty means an attached-but-empty removable drive (insert ejects nothing).
// free means a bay with no drive attached at all (attaching adds one, ejecting
// nothing). A slot that is neither holds an ISO and would be swapped.
type opticalSlot struct {
	controller string
	port       int
	device     int
	empty      bool
	free       bool
}

// GuestAdditionsStatus reports whether Guest Additions is active in the guest.
// The version property is only populated while the VM is running, so a stopped
// VM is reported as "unknown" rather than "not-detected" — we cannot tell.
func (s *service) GuestAdditionsStatus(ctx context.Context, id string) (models.GuestAdditionsStatusResponse, error) {
	if !IsValidVmID(id) {
		return models.GuestAdditionsStatusResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.GuestAdditionsStatusResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state for Guest Additions status")
	if err != nil {
		return models.GuestAdditionsStatusResponse{}, err
	}
	if !vmStateIsLive(parseVmState(info)) {
		return models.GuestAdditionsStatusResponse{ID: id, Installed: false, Status: "unknown"}, nil
	}

	result, runErr := s.runner.RunContext(ctx, path, guestPropertyGetArgs(id, gaVersionProperty), 10*time.Second)
	if runErr != nil {
		return models.GuestAdditionsStatusResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while reading Guest Additions version: %v", runErr),
		}
	}
	if result.ExitCode != 0 {
		return models.GuestAdditionsStatusResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while reading Guest Additions version", result.ExitCode),
		}
	}

	if version, present := parseGuestPropertyValue(result.StandardOutput); present {
		// Compare with the host so the UI can offer a one-click update when the
		// guest additions are older/newer than the host (a mismatch breaks
		// dynamic resolution and other features). Host version is best-effort.
		hostVersion, _ := s.readVersion(ctx, path)
		update := hostVersion != "" && baseVersion(version) != baseVersion(hostVersion)
		return models.GuestAdditionsStatusResponse{
			ID:              id,
			Installed:       true,
			Version:         version,
			HostVersion:     baseVersion(hostVersion),
			UpdateAvailable: update,
			Status:          "installed",
		}, nil
	}
	return models.GuestAdditionsStatusResponse{ID: id, Installed: false, Status: "not-detected"}, nil
}

// baseVersion reduces a VirtualBox version string to its dotted numeric core for
// comparison: "7.2.12r169901" -> "7.2.12", "7.0.20_Debian..." -> "7.0.20". The
// guest property and `VBoxManage --version` decorate the version differently, so
// both are normalized before comparing.
func baseVersion(v string) string {
	v = strings.TrimSpace(v)
	// Cut at the first character that is neither a digit nor a dot.
	for i, r := range v {
		if (r < '0' || r > '9') && r != '.' {
			return strings.TrimRight(v[:i], ".")
		}
	}
	return strings.TrimRight(v, ".")
}

// InstallGuestAdditions inserts the Guest Additions disc into the VM's optical
// drive (the `additions` medium is the ISO bundled with VirtualBox, so nothing
// is downloaded). This is the host-side half of installation; the guest-side
// installer still has to be run inside the VM to finish, which the host cannot
// drive without guest credentials. Inserting the disc swaps whatever medium is
// currently in the optical drive, matching VirtualBox's own "Insert Guest
// Additions CD image" behavior.
func (s *service) InstallGuestAdditions(ctx context.Context, id string) (models.GuestAdditionsInstallResponse, error) {
	if !IsValidVmID(id) {
		return models.GuestAdditionsInstallResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "guest-additions.install", false, "VirtualBox/VBoxManage not discovered.")
		return models.GuestAdditionsInstallResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading storage for Guest Additions install")
	if err != nil {
		return models.GuestAdditionsInstallResponse{}, err
	}

	slot, ok := chooseOpticalTarget(parseOpticalSlots(info))
	if !ok {
		return models.GuestAdditionsInstallResponse{}, &ValidationError{
			Message: "This VM has no optical (DVD) drive to insert the Guest Additions disc into. Add a DVD drive in VirtualBox, then try again.",
		}
	}

	if err := s.runControlCommand(ctx, path, insertGuestAdditionsArgs(id, slot), "inserting Guest Additions disc"); err != nil {
		s.logOperation(ctx, id, "guest-additions.install", false, "VirtualBox Guest Additions disc insert failed.")
		return models.GuestAdditionsInstallResponse{}, err
	}

	s.logOperation(ctx, id, "guest-additions.install", true, "")
	return models.GuestAdditionsInstallResponse{
		Success:    true,
		VMID:       id,
		Controller: slot.controller,
		Port:       slot.port,
		Device:     slot.device,
		Message:    "Guest Additions disc inserted. Run the installer inside the VM to finish setup.",
	}, nil
}

// UpdateGuestAdditions installs/updates the Guest Additions inside a running
// Linux guest via VBoxManage guest control. It first inserts the host's bundled
// Guest Additions ISO, then runs the .run installer inside the guest with the
// supplied credentials. The password is passed via a temp --passwordfile so it
// never appears on the command line (where the OS process list would expose it),
// and the file is deleted immediately after the call. The file lives in the
// current user's private temp directory; Chmod(0o600) is best-effort and only
// meaningful on POSIX (on Windows it does not alter the NTFS ACL, so there the
// protection is the per-user %TEMP% ACL, not permission bits). Guest control
// requires Guest Additions to already be running in the guest, so this covers
// the "installed but mismatched" case; the account must be root (the installer
// needs it). Windows guests are not handled here.
func (s *service) UpdateGuestAdditions(ctx context.Context, id, username, password string) (models.GuestAdditionsUpdateResponse, error) {
	if !IsValidVmID(id) {
		return models.GuestAdditionsUpdateResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if strings.TrimSpace(username) == "" || password == "" {
		return models.GuestAdditionsUpdateResponse{}, &ValidationError{Message: "Guest username and password are required."}
	}
	if !isPlausibleGuestUsername(username) {
		return models.GuestAdditionsUpdateResponse{}, &ValidationError{Message: "Guest username contains unsupported characters."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "guest-additions.update", false, "VirtualBox/VBoxManage not discovered.")
		return models.GuestAdditionsUpdateResponse{}, err
	}

	// Make sure the Guest Additions disc is present so the installer exists in the
	// guest. Best-effort: if no free/empty optical slot is found a disc is likely
	// already inserted, so proceed to run the installer regardless.
	if info, infoErr := s.readShowVmInfo(ctx, path, id, "reading storage for Guest Additions update"); infoErr == nil {
		if slot, ok := chooseOpticalTarget(parseOpticalSlots(info)); ok {
			_ = s.runControlCommand(ctx, path, insertGuestAdditionsArgs(id, slot), "inserting Guest Additions disc")
		}
	}

	pwFile, err := os.CreateTemp("", "tabvm-ga-*.txt")
	if err != nil {
		return models.GuestAdditionsUpdateResponse{}, fmt.Errorf("creating credential file: %w", err)
	}
	pwPath := pwFile.Name()
	defer os.Remove(pwPath)
	_ = pwFile.Chmod(0o600)
	// Trailing newline so `sudo -S` (which reads one line from stdin) accepts it;
	// VBoxManage --passwordfile trims trailing whitespace, so this is harmless there.
	if _, err := pwFile.WriteString(password + "\n"); err != nil {
		pwFile.Close()
		return models.GuestAdditionsUpdateResponse{}, fmt.Errorf("writing credential file: %w", err)
	}
	pwFile.Close()

	const failMsg = "Could not update Guest Additions inside the guest. Check the username/password, that the account is root or has sudo, and that the guest is a running Linux VM with Guest Additions already active."

	var result runner.Result
	var runErr error
	if strings.EqualFold(username, "root") {
		result, runErr = s.runner.RunContext(ctx, path, guestControlUpdateGAArgs(id, username, pwPath), 2*time.Minute)
	} else {
		// Non-root account: the installer needs root, so copy the password into
		// the guest and run it under `sudo -S` (password fed on the guest
		// process's stdin, still never on any argv), then delete the copy.
		if cp, cpErr := s.runner.RunContext(ctx, path, guestControlCopyPwArgs(id, username, pwPath), 30*time.Second); cpErr != nil || cp.ExitCode != 0 {
			s.logOperation(ctx, id, "guest-additions.update", false, "Copying credentials into guest failed.")
			return models.GuestAdditionsUpdateResponse{
				Success: false,
				VMID:    id,
				Message: failMsg,
				Output:  combinedOutput(cp.StandardOutput, cp.StandardError),
			}, nil
		}
		result, runErr = s.runner.RunContext(ctx, path, guestControlSudoInstallArgs(id, username, pwPath), 2*time.Minute)
	}
	if runErr != nil || result.ExitCode != 0 {
		s.logOperation(ctx, id, "guest-additions.update", false, "Guest Additions guest-control install failed.")
		return models.GuestAdditionsUpdateResponse{
			Success: false,
			VMID:    id,
			Message: failMsg,
			Output:  combinedOutput(result.StandardOutput, result.StandardError),
		}, nil
	}

	s.logOperation(ctx, id, "guest-additions.update", true, "")
	return models.GuestAdditionsUpdateResponse{
		Success: true,
		VMID:    id,
		Message: "Guest Additions update started. The VM installs it in the background and reboots automatically in 1–3 minutes — reopen the console once it is back. (Guest log: /var/log/tabvm-ga.log)",
		Output:  combinedOutput(result.StandardOutput, result.StandardError),
	}, nil
}

// isPlausibleGuestUsername is a conservative defense-in-depth check on the guest
// account name before it is passed to VBoxManage's --username option. exec.Command
// bypasses the shell so this is not shell-injectable, but rejecting control
// characters, whitespace and a leading dash keeps unvalidated input from reaching
// a third-party CLI's argument parser (mirroring the strict validation applied to
// the VM id). Linux account names — the only supported guests here — never need
// spaces or a leading dash.
func isPlausibleGuestUsername(u string) bool {
	if len(u) == 0 || len(u) > 104 || u[0] == '-' {
		return false
	}
	for _, r := range u {
		if r < 0x20 || r == 0x7f || r == ' ' || r == '\t' {
			return false
		}
	}
	return true
}

// gaInstallScript is the DETACHED payload that actually installs the Guest
// Additions. It runs as a background systemd unit (or setsid fallback), NOT
// directly in the guest-control session: the installer restarts VBoxService (the
// guest-control channel itself), which would otherwise terminate the session
// mid-install. Detached, the session returns immediately while the install
// proceeds and reboots the guest on its own. Output is logged inside the guest
// for diagnosis. This script is base64-encoded before transport, so it may
// freely contain any quoting.
const gaInstallScript = "#!/bin/sh\n" +
	"exec >/var/log/tabvm-ga.log 2>&1\n" +
	"export DEBIAN_FRONTEND=noninteractive\n" +
	// The GA installer compiles kernel modules, which needs the running kernel's
	// headers + toolchain. On Debian/Kali/Ubuntu install them first (best-effort,
	// needs guest network); other distros are skipped and rely on preinstalled
	// headers.
	"if command -v apt-get >/dev/null 2>&1; then\n" +
	"apt-get update || true\n" +
	"apt-get install -y linux-headers-$(uname -r) build-essential dkms || true\n" +
	"fi\n" +
	"D=$(mktemp -d)\n" +
	"mount -o ro /dev/sr0 \"$D\" 2>/dev/null || mount -o ro /dev/cdrom \"$D\"\n" +
	"printf 'yes\\nyes\\nyes\\nyes\\nyes\\n' | sh \"$D/VBoxLinuxAdditions.run\" --nox11\n" +
	"umount \"$D\" 2>/dev/null || true\n" +
	// Rebuild the modules against the now-present headers before rebooting.
	"[ -x /sbin/rcvboxadd ] && /sbin/rcvboxadd setup || true\n" +
	"reboot\n"

// gaScriptPath / guestPwPath are guest-side paths TabVM writes to.
const gaScriptPath = "/tmp/tabvm-ga.sh"
const guestPwPath = "/tmp/.tabvm-ga-pw"

// gaLauncherCommand builds the SHORT command run inside the guest-control
// session: it base64-decodes gaInstallScript into a guest file and launches it
// detached (a transient systemd unit if available, else setsid), then returns
// immediately. Being quick, the guest-control session stays alive long enough to
// succeed even though the detached install later restarts VBoxService. The
// base64 blob is only [A-Za-z0-9+/=], so this string has no single quotes and
// embeds safely inside the sudo `sh -c '...'` wrapper.
func gaLauncherCommand() string {
	enc := base64.StdEncoding.EncodeToString([]byte(gaInstallScript))
	return "printf %s \"" + enc + "\" | base64 -d > " + gaScriptPath + "; " +
		"chmod +x " + gaScriptPath + "; " +
		"if command -v systemd-run >/dev/null 2>&1; then " +
		"systemd-run --unit=tabvm-ga --collect /bin/sh " + gaScriptPath + "; " +
		"else setsid /bin/sh " + gaScriptPath + " </dev/null >/dev/null 2>&1 & fi"
}

// guestControlUpdateGAArgs launches the detached installer directly as root.
// pwFilePath is a --passwordfile so the password never appears in argv.
func guestControlUpdateGAArgs(id, username, pwFilePath string) []string {
	return []string{
		"guestcontrol", id,
		"--username", username,
		"--passwordfile", pwFilePath,
		"run",
		"--exe", "/bin/sh",
		// The launcher returns in seconds (the real install runs detached), so a
		// short timeout suffices. Wait on stdout only: combining --wait-stdout and
		// --wait-stderr triggers a VERR_DUPLICATE on this VBoxManage version.
		"--timeout", "90000",
		"--wait-stdout",
		// VBoxManage sets argv[0] to --exe itself, so tokens after -- are argv[1..];
		// pass only "-c <script>", not a repeated "sh".
		"--", "-c", gaLauncherCommand(),
	}
}

// guestControlCopyPwArgs copies the host password file into the guest so the
// sudo -S path can read it from stdin.
func guestControlCopyPwArgs(id, username, pwFilePath string) []string {
	return []string{
		"guestcontrol", id,
		"--username", username,
		"--passwordfile", pwFilePath,
		"copyto", pwFilePath, guestPwPath,
	}
}

// guestControlSudoInstallArgs runs the installer under sudo for a non-root
// account. sudo -S reads the password from stdin (the copied file), never argv;
// the copied file is deleted afterward regardless of the installer's exit code.
func guestControlSudoInstallArgs(id, username, pwFilePath string) []string {
	outer := "sudo -S -p '' /bin/sh -c '" + gaLauncherCommand() + "' < " + guestPwPath +
		"; rc=$?; rm -f " + guestPwPath + "; exit $rc"
	return []string{
		"guestcontrol", id,
		"--username", username,
		"--passwordfile", pwFilePath,
		"run",
		"--exe", "/bin/sh",
		// The launcher returns in seconds (the real install runs detached), so a
		// short timeout suffices. Wait on stdout only to avoid VERR_DUPLICATE.
		"--timeout", "90000",
		"--wait-stdout",
		"--", "-c", outer,
	}
}

// combinedOutput joins captured stdout and stderr for surfacing to the UI.
func combinedOutput(stdout, stderr string) string {
	out := strings.TrimSpace(stdout)
	errOut := strings.TrimSpace(stderr)
	switch {
	case out != "" && errOut != "":
		return out + "\n" + errOut
	case errOut != "":
		return errOut
	default:
		return out
	}
}

// parseGuestPropertyValue parses `VBoxManage guestproperty get` output. A set
// property prints "Value: <value>"; an unset one prints "No value set!".
func parseGuestPropertyValue(output string) (string, bool) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		const prefix = "Value:"
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if value == "" {
			return "", false
		}
		return value, true
	}
	return "", false
}

// parseOpticalSlots finds candidate DVD attachment points on a VM from
// machine-readable showvminfo output: empty removable drives ("emptydrive"),
// ISO-holding optical drives, and free bays ("none") on controllers that can
// hold an optical drive. Hard-disk images are ignored so the Guest Additions
// disc is never attached over a system disk. Controller names (which may
// contain spaces or dashes) and types are read from the storagecontroller keys
// so the port/device suffix parses unambiguously and NVMe/floppy free bays are
// excluded (they cannot host a DVD drive).
func parseOpticalSlots(output string) []opticalSlot {
	nameByIndex := map[string]string{}
	typeByIndex := map[string]string{}
	type attachment struct{ key, value string }
	attachments := make([]attachment, 0, 8)

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadableRawKey(line)
		if !ok {
			continue
		}
		if idx, ok := strings.CutPrefix(key, "storagecontrollername"); ok {
			nameByIndex[idx] = value
			continue
		}
		if idx, ok := strings.CutPrefix(key, "storagecontrollertype"); ok {
			typeByIndex[idx] = value
			continue
		}
		attachments = append(attachments, attachment{key: key, value: value})
	}

	names := make([]string, 0, len(nameByIndex))
	supportsOptical := map[string]bool{}
	for idx, name := range nameByIndex {
		names = append(names, name)
		supportsOptical[name] = controllerSupportsOptical(typeByIndex[idx])
	}
	// Match longer controller names first so a name that is a prefix of another
	// cannot steal its attachments.
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })

	slots := make([]opticalSlot, 0, len(attachments))
	for _, att := range attachments {
		kind, empty, free := classifyOpticalValue(att.value)
		if !kind {
			continue
		}
		for _, name := range names {
			prefix := name + "-"
			if !strings.HasPrefix(att.key, prefix) {
				continue
			}
			rest := att.key[len(prefix):]
			dash := strings.LastIndexByte(rest, '-')
			if dash <= 0 {
				break
			}
			port, err1 := strconv.Atoi(rest[:dash])
			device, err2 := strconv.Atoi(rest[dash+1:])
			if err1 != nil || err2 != nil {
				break
			}
			// A free bay is only usable if its controller can host an optical
			// drive; an existing optical drive already proves it can.
			if free && !supportsOptical[name] {
				break
			}
			slots = append(slots, opticalSlot{
				controller: name,
				port:       port,
				device:     device,
				empty:      empty,
				free:       free,
			})
			break
		}
	}
	return slots
}

// classifyOpticalValue interprets a storage attachment value. It reports whether
// the slot is a usable optical target and, if so, whether it is an empty
// removable drive or a free (undriven) bay. An ISO value is optical but neither
// empty nor free, so it is only used as a last resort (its medium is swapped).
func classifyOpticalValue(value string) (optical, empty, free bool) {
	switch v := strings.ToLower(strings.TrimSpace(value)); {
	case v == "emptydrive":
		return true, true, false
	case v == "none":
		return true, false, true
	case strings.HasSuffix(v, ".iso"):
		return true, false, false
	default:
		return false, false, false
	}
}

// controllerSupportsOptical reports whether a storage controller type can host a
// DVD drive. NVMe and the i82078 floppy controller cannot; everything else
// VirtualBox offers (IDE, SATA/AHCI, SCSI, SAS, USB, virtio-scsi) can.
func controllerSupportsOptical(controllerType string) bool {
	switch strings.ToLower(strings.TrimSpace(controllerType)) {
	case "nvme", "i82078":
		return false
	default:
		return true
	}
}

// chooseOpticalTarget picks where to insert the Guest Additions disc, preferring
// options that eject nothing: an empty removable drive first, then a free bay
// (a new drive is added), and only as a last resort an occupied optical drive
// (its current medium is swapped out).
func chooseOpticalTarget(slots []opticalSlot) (opticalSlot, bool) {
	for _, slot := range slots {
		if slot.empty {
			return slot, true
		}
	}
	for _, slot := range slots {
		if slot.free {
			return slot, true
		}
	}
	if len(slots) > 0 {
		return slots[0], true
	}
	return opticalSlot{}, false
}

func guestPropertyGetArgs(id, property string) []string {
	return []string{"guestproperty", "get", id, property}
}

func insertGuestAdditionsArgs(id string, slot opticalSlot) []string {
	return []string{
		"storageattach", id,
		"--storagectl", slot.controller,
		"--port", strconv.Itoa(slot.port),
		"--device", strconv.Itoa(slot.device),
		"--type", "dvddrive",
		"--medium", "additions",
	}
}
