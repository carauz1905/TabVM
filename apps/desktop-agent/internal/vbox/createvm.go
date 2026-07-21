package vbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// vmNamePattern restricts VM names to a conservative, shell-safe set (spaces
// allowed, but no path or quoting metacharacters).
var vmNamePattern = regexp.MustCompile(`^[A-Za-z0-9 ._-]{1,64}$`)

// guestUserPattern matches the unattended install account name. It is broad
// enough for both Linux (lowercase) and Windows (mixed case) local accounts,
// while still rejecting shell/path metacharacters.
var guestUserPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

// supportedOsTypes is the allow-list of guest OS types offered for unattended
// installs. VirtualBox ships unattended templates for these; Kali is absent (it
// has no template) — that path uses the .ova import instead.
var supportedOsTypes = map[string]bool{
	// Linux
	"Ubuntu_64":       true,
	"Ubuntu22_LTS_64": true,
	"Ubuntu24_LTS_64": true,
	"Debian_64":       true,
	"Debian12_64":     true,
	// Windows
	"Windows10_64":   true,
	"Windows11_64":   true,
	"Windows2019_64": true,
	"Windows2022_64": true,
}

// manualOsTypes is the allow-list of guest OS types offered for manual installs.
// These are generic VirtualBox ostypes that work without an unattended template;
// the user installs the OS interactively from the attached ISO.
var manualOsTypes = map[string]bool{
	"Linux_64": true,
	"Other_64": true,
}

// createTimeout bounds a single quick VBoxManage step (createvm, modifyvm, …).
const createStepTimeout = 2 * time.Minute

// createMediumTimeout bounds disk creation, which pre-allocates metadata.
const createMediumTimeout = 5 * time.Minute

// importTimeout bounds a full appliance import, which decompresses and writes a
// whole disk image and can take many minutes.
const importTimeout = 30 * time.Minute

// cleanupStepTimeout bounds each best-effort cleanup command after a failed
// create, so cleanup can never hang a background job indefinitely.
const cleanupStepTimeout = 2 * time.Minute

// ImportAppliance imports a prebuilt .ova/.ovf appliance (which already ships
// Guest Additions) under the given VM name. It is long-running; callers run it
// on a background job.
func (s *service) ImportAppliance(ctx context.Context, ovaPath, name string) (models.VmCreateResponse, error) {
	if err := validateVmName(name); err != nil {
		return models.VmCreateResponse{}, err
	}
	if err := validateAppliancePath(ovaPath); err != nil {
		return models.VmCreateResponse{}, err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmCreateResponse{}, err
	}

	if err := s.runControlCommandTimeout(ctx, path, importApplianceArgs(ovaPath, name), "importing appliance", importTimeout); err != nil {
		s.logOperation(ctx, "", "vm.import", false, "VirtualBox appliance import failed.")
		return models.VmCreateResponse{}, err
	}

	uuid := s.resolveVmUUID(ctx, path, name)
	s.logOperation(ctx, uuid, "vm.import", true, "")
	return models.VmCreateResponse{
		Success: true,
		VMID:    uuid,
		Name:    name,
		Message: fmt.Sprintf("%q imported and ready to start.", name),
	}, nil
}

// CreateVmUnattended creates a new VM and runs an unattended OS install from an
// ISO with Guest Additions installed during setup. It is long-running; callers
// run it on a background job. The install itself continues inside the VM after
// this returns — the VM is registered with the unattended medium configured, so
// the user starts it and watches the install via the TabVM console.
func (s *service) CreateVmUnattended(ctx context.Context, req models.VmCreateRequest) (models.VmCreateResponse, error) {
	if err := validateVmName(req.Name); err != nil {
		return models.VmCreateResponse{}, err
	}
	if !supportedOsTypes[req.OsType] {
		return models.VmCreateResponse{}, &ValidationError{Message: "Unsupported OS type for unattended install."}
	}
	if err := validateIsoPath(req.IsoPath); err != nil {
		return models.VmCreateResponse{}, err
	}
	if !guestUserPattern.MatchString(req.Username) {
		return models.VmCreateResponse{}, &ValidationError{Message: "Guest username contains unsupported characters."}
	}
	if req.Password == "" {
		return models.VmCreateResponse{}, &ValidationError{Message: "A guest password is required."}
	}
	if req.MemoryMB < 512 || req.MemoryMB > 65536 {
		return models.VmCreateResponse{}, &ValidationError{Message: "Memory must be between 512 MB and 65536 MB."}
	}
	if req.Cpus < 1 || req.Cpus > 16 {
		return models.VmCreateResponse{}, &ValidationError{Message: "CPU count must be between 1 and 16."}
	}
	if req.DiskGB < 8 || req.DiskGB > 512 {
		return models.VmCreateResponse{}, &ValidationError{Message: "Disk size must be between 8 GB and 512 GB."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmCreateResponse{}, err
	}

	// 1. Register the VM and learn its UUID + settings folder in one call.
	createOut, err := s.runCapture(ctx, path, createVmArgs(req.Name, req.OsType), "creating VM", createStepTimeout)
	if err != nil {
		s.logOperation(ctx, "", "vm.create", false, "VBoxManage createvm failed.")
		return models.VmCreateResponse{}, err
	}
	uuid, settingsFile := parseCreateVmOutput(createOut)
	if uuid == "" {
		return models.VmCreateResponse{}, &ExecutionError{ExitCode: -1, Message: "Could not determine the new VM identifier."}
	}
	diskPath := filepath.Join(filepath.Dir(settingsFile), req.Name+".vdi")

	// 2. Hardware, disk, and controller.
	steps := []struct {
		args    []string
		desc    string
		timeout time.Duration
	}{
		{modifyVmArgs(uuid, req.OsType, req.MemoryMB, req.Cpus), "configuring VM", createStepTimeout},
		{createDiskArgs(diskPath, req.DiskGB), "creating disk", createMediumTimeout},
		{storageCtlArgs(uuid), "adding storage controller", createStepTimeout},
		{storageAttachDiskArgs(uuid, diskPath), "attaching disk", createStepTimeout},
	}
	for _, step := range steps {
		if err := s.runControlCommandTimeout(ctx, path, step.args, step.desc, step.timeout); err != nil {
			s.logOperation(ctx, uuid, "vm.create", false, "VBoxManage "+step.desc+" failed.")
			s.cleanupFailedCreate(ctx, path, uuid, diskPath)
			return models.VmCreateResponse{}, err
		}
	}

	// 3. Configure the unattended install. The password goes via a temp file so
	// it never appears in the process argument list.
	pwFile, err := os.CreateTemp("", "tabvm-unatt-*.txt")
	if err != nil {
		s.cleanupFailedCreate(ctx, path, uuid, diskPath)
		return models.VmCreateResponse{}, fmt.Errorf("creating credential file: %w", err)
	}
	pwPath := pwFile.Name()
	defer os.Remove(pwPath)
	_ = pwFile.Chmod(0o600)
	if _, err := pwFile.WriteString(req.Password + "\n"); err != nil {
		pwFile.Close()
		s.cleanupFailedCreate(ctx, path, uuid, diskPath)
		return models.VmCreateResponse{}, fmt.Errorf("writing credential file: %w", err)
	}
	pwFile.Close()

	if err := s.runControlCommandTimeout(ctx, path, unattendedInstallArgs(uuid, req, pwPath), "configuring unattended install", createStepTimeout); err != nil {
		s.logOperation(ctx, uuid, "vm.create", false, "VBoxManage unattended install failed.")
		s.cleanupFailedCreate(ctx, path, uuid, diskPath)
		return models.VmCreateResponse{}, err
	}

	s.logOperation(ctx, uuid, "vm.create", true, "")
	return models.VmCreateResponse{
		Success: true,
		VMID:    uuid,
		Name:    req.Name,
		Message: fmt.Sprintf("%q created. Start it to run the automated install with Guest Additions.", req.Name),
	}, nil
}

// CreateVmManual creates a new VM with the installer ISO attached as a DVD and
// no unattended install configured, for OSes without an unattended template
// (e.g. Alpine). It is long-running; callers run it on a background job. The
// user starts the VM and installs the OS interactively via the TabVM console.
func (s *service) CreateVmManual(ctx context.Context, req models.VmCreateManualRequest) (models.VmCreateResponse, error) {
	if err := validateVmName(req.Name); err != nil {
		return models.VmCreateResponse{}, err
	}
	if !manualOsTypes[req.OsType] {
		return models.VmCreateResponse{}, &ValidationError{Message: "Unsupported OS type for manual install."}
	}
	if err := validateIsoPath(req.IsoPath); err != nil {
		return models.VmCreateResponse{}, err
	}
	if req.MemoryMB < 512 || req.MemoryMB > 65536 {
		return models.VmCreateResponse{}, &ValidationError{Message: "Memory must be between 512 MB and 65536 MB."}
	}
	if req.Cpus < 1 || req.Cpus > 16 {
		return models.VmCreateResponse{}, &ValidationError{Message: "CPU count must be between 1 and 16."}
	}
	if req.DiskGB < 8 || req.DiskGB > 512 {
		return models.VmCreateResponse{}, &ValidationError{Message: "Disk size must be between 8 GB and 512 GB."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmCreateResponse{}, err
	}

	// 1. Register the VM and learn its UUID + settings folder in one call.
	createOut, err := s.runCapture(ctx, path, createVmArgs(req.Name, req.OsType), "creating VM", createStepTimeout)
	if err != nil {
		s.logOperation(ctx, "", "vm.create", false, "VBoxManage createvm failed.")
		return models.VmCreateResponse{}, err
	}
	uuid, settingsFile := parseCreateVmOutput(createOut)
	if uuid == "" {
		return models.VmCreateResponse{}, &ExecutionError{ExitCode: -1, Message: "Could not determine the new VM identifier."}
	}
	diskPath := filepath.Join(filepath.Dir(settingsFile), req.Name+".vdi")

	// 2. Hardware, disk, controller, and the installer ISO as a DVD. The disk
	// sits on port 0 and the ISO on port 1, so the VM boots the installer first
	// and falls back to the disk once the install is done and the ISO ejected.
	steps := []struct {
		args    []string
		desc    string
		timeout time.Duration
	}{
		{modifyVmArgs(uuid, req.OsType, req.MemoryMB, req.Cpus), "configuring VM", createStepTimeout},
		{createDiskArgs(diskPath, req.DiskGB), "creating disk", createMediumTimeout},
		{storageCtlArgs(uuid), "adding storage controller", createStepTimeout},
		{storageAttachDiskArgs(uuid, diskPath), "attaching disk", createStepTimeout},
		{storageAttachDvdArgs(uuid, req.IsoPath), "attaching installer ISO", createStepTimeout},
	}
	for _, step := range steps {
		if err := s.runControlCommandTimeout(ctx, path, step.args, step.desc, step.timeout); err != nil {
			s.logOperation(ctx, uuid, "vm.create", false, "VBoxManage "+step.desc+" failed.")
			s.cleanupFailedCreate(ctx, path, uuid, diskPath)
			return models.VmCreateResponse{}, err
		}
	}

	s.logOperation(ctx, uuid, "vm.create", true, "")
	return models.VmCreateResponse{
		Success: true,
		VMID:    uuid,
		Name:    req.Name,
		Message: fmt.Sprintf("%q created. Start it and install the OS from the attached ISO.", req.Name),
	}, nil
}

// cleanupFailedCreate best-effort undoes a create that failed after
// `createvm --register` succeeded. It unregisters the VM (deleting any
// attached media), then reclaims a disk image that was created but never
// attached: first via `closemedium --delete`, then by removing the file
// directly if it still exists. Failures are logged and never returned, so the
// caller's original error is always preserved.
func (s *service) cleanupFailedCreate(ctx context.Context, path, uuid, diskPath string) {
	// The create may have failed because ctx was cancelled or timed out;
	// cleanup still has to run, bounded by its own per-step timeouts.
	ctx = context.WithoutCancel(ctx)

	if err := s.runControlCommandTimeout(ctx, path, unregisterVmArgs(uuid), "cleaning up a failed create (unregister)", cleanupStepTimeout); err != nil {
		s.logOperation(ctx, uuid, "vm.create.cleanup", false, "VBoxManage unregistervm cleanup failed.")
	}
	if diskPath == "" {
		return
	}
	if _, err := statPath(diskPath); err != nil {
		return // No orphaned disk image left behind.
	}
	// The disk file survived the unregister, so it was never attached. Release
	// it from the media registry and delete it.
	if err := s.runControlCommandTimeout(ctx, path, closeMediumDeleteArgs(diskPath), "cleaning up a failed create (disk)", cleanupStepTimeout); err != nil {
		s.logOperation(ctx, uuid, "vm.create.cleanup", false, "VBoxManage closemedium cleanup failed.")
	}
	if _, err := statPath(diskPath); err == nil {
		if err := os.Remove(diskPath); err != nil {
			s.logOperation(ctx, uuid, "vm.create.cleanup", false, "Removing the orphaned disk image failed.")
		}
	}
}

// resolveVmUUID looks up a VM's UUID by name after a create/import. Best-effort;
// returns "" if it cannot be read.
func (s *service) resolveVmUUID(ctx context.Context, path, name string) string {
	result, err := s.runner.RunContext(ctx, path, []string{"showvminfo", name, "--machinereadable"}, 10*time.Second)
	if err != nil || result.ExitCode != 0 {
		return ""
	}
	for _, line := range strings.Split(result.StandardOutput, "\n") {
		if key, value, ok := splitMachineReadableRawKey(line); ok && key == "UUID" {
			return value
		}
	}
	return ""
}

// runCapture runs a command with a timeout and returns stdout, mapping failures
// to an ExecutionError like runControlCommand.
func (s *service) runCapture(ctx context.Context, path string, args []string, description string, timeout time.Duration) (string, error) {
	result, runErr := s.runner.RunContext(ctx, path, args, timeout)
	if runErr != nil {
		return "", &ExecutionError{ExitCode: result.ExitCode, StandardError: result.StandardError, Message: fmt.Sprintf("VBoxManage failed while %s: %v", description, runErr)}
	}
	if result.ExitCode != 0 {
		return "", &ExecutionError{ExitCode: result.ExitCode, StandardError: result.StandardError, Message: fmt.Sprintf("VBoxManage exited with code %d while %s", result.ExitCode, description)}
	}
	return result.StandardOutput, nil
}

// runControlCommandTimeout is runControlCommand with a caller-chosen timeout, for
// steps that take longer than the default 30s (disk creation, appliance import).
func (s *service) runControlCommandTimeout(ctx context.Context, path string, args []string, description string, timeout time.Duration) error {
	_, err := s.runCapture(ctx, path, args, description, timeout)
	return err
}

// parseCreateVmOutput extracts the UUID and settings-file path from `createvm`
// stdout, which lists lines like `UUID: <id>` and `Settings file: '<path>'`.
func parseCreateVmOutput(out string) (uuid, settingsFile string) {
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(trimmed, "UUID:"); ok {
			uuid = strings.TrimSpace(rest)
		} else if rest, ok := strings.CutPrefix(trimmed, "Settings file:"); ok {
			settingsFile = strings.Trim(strings.TrimSpace(rest), "'\"")
		}
	}
	return uuid, settingsFile
}

// --- argument builders ---

func importApplianceArgs(ovaPath, name string) []string {
	return []string{"import", ovaPath, "--vsys", "0", "--vmname", name}
}

func createVmArgs(name, osType string) []string {
	return []string{"createvm", "--name", name, "--ostype", osType, "--register"}
}

func modifyVmArgs(uuid, osType string, memoryMB, cpus int) []string {
	args := []string{
		"modifyvm", uuid,
		"--memory", fmt.Sprintf("%d", memoryMB),
		"--cpus", fmt.Sprintf("%d", cpus),
		"--ioapic", "on",
		"--nic1", "nat",
		"--vram", "33",
		"--graphicscontroller", "vmsvga",
	}
	// Windows guests boot via EFI; Windows 11 additionally requires a TPM 2.0 and
	// secure boot, which VBoxManage can attach to the EFI firmware.
	if isWindowsOsType(osType) {
		args = append(args, "--firmware", "efi")
		if osType == "Windows11_64" {
			args = append(args, "--tpm-type", "2.0")
		}
	}
	return args
}

// isWindowsOsType reports whether the guest OS type is a Windows variant.
func isWindowsOsType(osType string) bool {
	return strings.HasPrefix(osType, "Windows")
}

func createDiskArgs(diskPath string, diskGB int) []string {
	return []string{
		"createmedium", "disk",
		"--filename", diskPath,
		"--size", fmt.Sprintf("%d", diskGB*1024),
		"--format", "VDI",
	}
}

func storageCtlArgs(uuid string) []string {
	return []string{
		"storagectl", uuid,
		"--name", "SATA",
		"--add", "sata",
		"--controller", "IntelAhci",
		"--portcount", "2",
		"--bootable", "on",
	}
}

func storageAttachDiskArgs(uuid, diskPath string) []string {
	return []string{
		"storageattach", uuid,
		"--storagectl", "SATA",
		"--port", "0",
		"--device", "0",
		"--type", "hdd",
		"--medium", diskPath,
	}
}

// storageAttachDvdArgs attaches the installer ISO as a DVD on the second SATA
// port (the controller is created with --portcount 2). It mirrors the general
// optical-drive attach builder used by mount/eject.
func storageAttachDvdArgs(uuid, isoPath string) []string {
	return storageAttachDvdMediumArgs(uuid, "SATA", 1, 0, isoPath)
}

// unattendedInstallArgs configures (does not start) the automated install with
// Guest Additions. The install runs on the next VM boot, watched via the TabVM
// console. --hostname must be a dotted FQDN, so a lab suffix is appended.
func unattendedInstallArgs(uuid string, req models.VmCreateRequest, pwFilePath string) []string {
	return []string{
		"unattended", "install", uuid,
		"--iso=" + req.IsoPath,
		"--user=" + req.Username,
		"--password-file=" + pwFilePath,
		"--full-user-name=" + req.Username,
		"--hostname=" + hostnameFor(req.Hostname, req.Name),
		"--locale=en_US",
		"--country=US",
		"--time-zone=Etc/UTC",
		"--install-additions",
	}
}

// hostnameFor returns a dotted hostname VBox unattended accepts. It prefers the
// caller-supplied hostname, else derives one from the VM name, and always ends
// in a lab suffix so it contains the required dot.
func hostnameFor(hostname, name string) string {
	base := strings.TrimSpace(hostname)
	if base == "" {
		base = name
	}
	base = strings.ToLower(base)
	base = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "tabvm"
	}
	return base + ".tabvm.lab"
}

// --- validation ---

func validateVmName(name string) error {
	if !vmNamePattern.MatchString(strings.TrimSpace(name)) {
		return &ValidationError{Message: "VM name must be 1-64 characters using letters, digits, space, dot, dash or underscore."}
	}
	return nil
}

func validateAppliancePath(p string) error {
	return validateHostFile(p, []string{".ova", ".ovf"}, "The appliance must be a .ova or .ovf file.")
}

func validateIsoPath(p string) error {
	return validateHostFile(p, []string{".iso"}, "The installer must be a .iso file.")
}

// validateHostFile ensures p is an existing, absolute, traversal-free file with
// one of the allowed extensions.
func validateHostFile(p string, exts []string, extMsg string) error {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return &ValidationError{Message: "A host file path is required."}
	}
	if !filepath.IsAbs(trimmed) {
		return &ValidationError{Message: "The path must be absolute."}
	}
	if containsTraversal(trimmed) {
		return &ValidationError{Message: "The path must not contain '..' segments."}
	}
	lower := strings.ToLower(trimmed)
	matched := false
	for _, ext := range exts {
		if strings.HasSuffix(lower, ext) {
			matched = true
			break
		}
	}
	if !matched {
		return &ValidationError{Message: extMsg}
	}
	info, err := statPath(trimmed)
	if err != nil {
		return &ValidationError{Message: "The file does not exist or is not accessible."}
	}
	if info.IsDir() {
		return &ValidationError{Message: "The path must be a file, not a directory."}
	}
	return nil
}
