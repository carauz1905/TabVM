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

// sharedFolderNamePattern restricts share names to a conservative set that is
// safe as both a VBoxManage argument and a guest mount point. It excludes path
// separators, whitespace, and shell metacharacters.
var sharedFolderNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// statPath wraps os.Stat so host path validation can be stubbed in tests without
// touching the real filesystem.
var statPath = os.Stat

// ListSharedFolders returns the persistent and transient shared folders
// configured on a VM.
func (s *service) ListSharedFolders(ctx context.Context, id string) (models.SharedFoldersResponse, error) {
	if !IsValidVmID(id) {
		return models.SharedFoldersResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.SharedFoldersResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading shared folders")
	if err != nil {
		return models.SharedFoldersResponse{}, err
	}

	parsed := parseSharedFolders(info)
	folders := make([]models.SharedFolder, 0, len(parsed))
	for _, f := range parsed {
		folders = append(folders, models.SharedFolder{
			Name:      f.name,
			HostPath:  f.hostPath,
			Transient: f.transient,
		})
	}
	return models.SharedFoldersResponse{Folders: folders}, nil
}

// AddSharedFolder validates the name and host path, then shares the host
// directory into the guest. When the VM is running the mapping is added as
// transient (persistent config cannot be modified on a running VM); otherwise it
// is added as a persistent machine mapping.
func (s *service) AddSharedFolder(ctx context.Context, id, name, hostPath string) (models.SharedFolderOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.SharedFolderOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if err := validateSharedFolderName(name); err != nil {
		return models.SharedFolderOperationResponse{}, err
	}
	if err := validateHostPath(hostPath); err != nil {
		return models.SharedFolderOperationResponse{}, err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "sharedfolder.add", false, "VirtualBox/VBoxManage not discovered.")
		return models.SharedFolderOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state for shared folder add")
	if err != nil {
		return models.SharedFolderOperationResponse{}, err
	}
	transient := vmStateIsLive(parseVmState(info))

	if err := s.runControlCommand(ctx, id, path, addSharedFolderArgs(id, name, hostPath, transient), "adding shared folder"); err != nil {
		s.logOperation(ctx, id, "sharedfolder.add", false, "VirtualBox shared folder add failed.")
		return models.SharedFolderOperationResponse{}, err
	}

	s.logOperation(ctx, id, "sharedfolder.add", true, "")
	message := fmt.Sprintf("Shared folder %q added.", name)
	if transient {
		message = fmt.Sprintf("Shared folder %q added for the current session (VM is running).", name)
	}
	return models.SharedFolderOperationResponse{Success: true, VMID: id, Message: message}, nil
}

// RemoveSharedFolder removes a shared folder by name. The transient flag is
// inferred from the VM's current configuration so the correct removal mode is
// used.
func (s *service) RemoveSharedFolder(ctx context.Context, id, name string) (models.SharedFolderOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.SharedFolderOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if err := validateSharedFolderName(name); err != nil {
		return models.SharedFolderOperationResponse{}, err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "sharedfolder.remove", false, "VirtualBox/VBoxManage not discovered.")
		return models.SharedFolderOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading shared folders for removal")
	if err != nil {
		return models.SharedFolderOperationResponse{}, err
	}

	var target *sharedFolderInfo
	for _, f := range parseSharedFolders(info) {
		if f.name == name {
			found := f
			target = &found
			break
		}
	}
	if target == nil {
		return models.SharedFolderOperationResponse{}, &ValidationError{Message: "Shared folder not found."}
	}

	if err := s.runControlCommand(ctx, id, path, removeSharedFolderArgs(id, name, target.transient, target.global), "removing shared folder"); err != nil {
		s.logOperation(ctx, id, "sharedfolder.remove", false, "VirtualBox shared folder remove failed.")
		return models.SharedFolderOperationResponse{}, err
	}

	s.logOperation(ctx, id, "sharedfolder.remove", true, "")
	return models.SharedFolderOperationResponse{
		Success: true,
		VMID:    id,
		Message: fmt.Sprintf("Shared folder %q removed.", name),
	}, nil
}

// readShowVmInfo runs showvminfo --machinereadable and returns stdout, mapping
// runner and non-zero exit failures to an ExecutionError.
func (s *service) readShowVmInfo(ctx context.Context, path, id, description string) (string, error) {
	result, runErr := s.runForVM(ctx, id, path, showVmInfoArgs(id), 10*time.Second)
	if runErr != nil {
		return "", &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while %s: %v", description, runErr),
		}
	}
	if result.ExitCode != 0 {
		return "", &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while %s", result.ExitCode, description),
		}
	}
	return result.StandardOutput, nil
}

// vmStateIsLive reports whether a VMState value means the VM is executing, in
// which case only transient shared folders can be changed.
func vmStateIsLive(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running", "paused", "stuck", "starting", "restoring":
		return true
	default:
		return false
	}
}

// validateSharedFolderName enforces a conservative share name policy.
func validateSharedFolderName(name string) error {
	if !sharedFolderNamePattern.MatchString(name) {
		return &ValidationError{
			Message: "Shared folder name must be 1-64 characters using letters, digits, dot, dash or underscore.",
		}
	}
	return nil
}

// validateHostPath ensures the host path is an existing, absolute directory and
// contains no traversal segments. Sharing a host directory into a guest exposes
// that directory tree to the VM, so the path is validated strictly before use.
func validateHostPath(hostPath string) error {
	trimmed := strings.TrimSpace(hostPath)
	if trimmed == "" {
		return &ValidationError{Message: "Host path is required."}
	}
	if !filepath.IsAbs(trimmed) {
		return &ValidationError{Message: "Host path must be an absolute path."}
	}
	if containsTraversal(trimmed) {
		return &ValidationError{Message: "Host path must not contain '..' segments."}
	}

	info, err := statPath(trimmed)
	if err != nil {
		return &ValidationError{Message: "Host path does not exist or is not accessible."}
	}
	if !info.IsDir() {
		return &ValidationError{Message: "Host path must be a directory."}
	}
	return nil
}

// containsTraversal reports whether any path segment is exactly "..".
func containsTraversal(p string) bool {
	for _, seg := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return true
		}
	}
	return false
}

func addSharedFolderArgs(id, name, hostPath string, transient bool) []string {
	args := []string{"sharedfolder", "add", id, "--name", name, "--hostpath", hostPath}
	if transient {
		args = append(args, "--transient")
	}
	// --automount makes Guest Additions mount the share inside the guest without
	// any manual `mount -t vboxsf` step. The default mount point follows the
	// sf_<name> convention (e.g. /media/sf_Shared on Linux), which is exactly the
	// guest path the UI predicts. For a transient share on a running VM the GA
	// automounter picks it up within seconds, so it appears without a reboot.
	args = append(args, "--automount")
	return args
}

func removeSharedFolderArgs(id, name string, transient, global bool) []string {
	args := []string{"sharedfolder", "remove", id, "--name", name}
	if transient {
		args = append(args, "--transient")
	}
	if global {
		args = append(args, "--global")
	}
	return args
}
