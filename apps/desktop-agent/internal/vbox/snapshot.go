package vbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// ListSnapshots returns a VM's snapshot tree, flattened with a depth for each
// entry so the UI can indent children.
func (s *service) ListSnapshots(ctx context.Context, id string) (models.SnapshotsResponse, error) {
	if !IsValidVmID(id) {
		return models.SnapshotsResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.SnapshotsResponse{}, err
	}

	result, runErr := s.runForVM(ctx, id, path, snapshotListArgs(id), 15*time.Second)
	if runErr != nil {
		return models.SnapshotsResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while listing snapshots: %v", runErr),
		}
	}
	// A VM with no snapshots makes `snapshot list` print "This machine does not
	// have any snapshots" and exit non-zero. That is an empty list, not an error.
	if strings.Contains(result.StandardOutput, "does not have any snapshots") ||
		strings.Contains(result.StandardError, "does not have any snapshots") {
		return models.SnapshotsResponse{Snapshots: []models.Snapshot{}}, nil
	}
	if result.ExitCode != 0 {
		return models.SnapshotsResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while listing snapshots", result.ExitCode),
		}
	}

	snaps, current := parseSnapshots(result.StandardOutput)
	return models.SnapshotsResponse{Snapshots: snaps, CurrentUUID: current}, nil
}

// TakeSnapshot captures the VM's current state. On a running VM VirtualBox takes
// an online snapshot (a brief pause); on a stopped VM it is instant.
func (s *service) TakeSnapshot(ctx context.Context, id, name, description string) (models.SnapshotOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.SnapshotOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if err := validateSnapshotName(name); err != nil {
		return models.SnapshotOperationResponse{}, err
	}
	if err := validateSnapshotDescription(description); err != nil {
		return models.SnapshotOperationResponse{}, err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "snapshot.take", false, "VirtualBox/VBoxManage not discovered.")
		return models.SnapshotOperationResponse{}, err
	}

	// An online snapshot of a large-RAM VM can take a while, so allow more time
	// than the 30s control-command default.
	result, runErr := s.runForVM(ctx, id, path, snapshotTakeArgs(id, name, description), 3*time.Minute)
	if runErr != nil || result.ExitCode != 0 {
		s.logOperation(ctx, id, "snapshot.take", false, "VBoxManage snapshot take failed.")
		return models.SnapshotOperationResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       "VBoxManage snapshot take failed",
		}
	}

	s.logOperation(ctx, id, "snapshot.take", true, "")
	return models.SnapshotOperationResponse{
		Success: true,
		VMID:    id,
		Message: fmt.Sprintf("Snapshot %q created.", strings.TrimSpace(name)),
	}, nil
}

// RestoreSnapshot rolls the VM back to a snapshot. VirtualBox cannot restore a
// running VM, so a live VM is powered off first (its unsaved state is discarded,
// which is the whole point of restoring). The VM is left powered off afterward.
func (s *service) RestoreSnapshot(ctx context.Context, id, snapshotID string) (models.SnapshotOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.SnapshotOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if !IsValidVmID(snapshotID) {
		return models.SnapshotOperationResponse{}, &ValidationError{Message: "Invalid snapshot identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "snapshot.restore", false, "VirtualBox/VBoxManage not discovered.")
		return models.SnapshotOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state for snapshot restore")
	if err != nil {
		return models.SnapshotOperationResponse{}, err
	}
	if vmStateIsLive(parseVmState(info)) {
		if err := s.runControlCommand(ctx, id, path, powerOffVmArgs(id), "powering off before snapshot restore"); err != nil {
			s.logOperation(ctx, id, "snapshot.restore", false, "Power off before restore failed.")
			return models.SnapshotOperationResponse{}, err
		}
		if err := s.waitUntilNotLive(ctx, path, id, 20*time.Second); err != nil {
			s.logOperation(ctx, id, "snapshot.restore", false, "VM did not power off in time.")
			return models.SnapshotOperationResponse{}, &ExecutionError{Message: "The VM did not power off in time to restore the snapshot."}
		}
	}

	result, runErr := s.runForVM(ctx, id, path, snapshotRestoreArgs(id, snapshotID), 2*time.Minute)
	if runErr != nil || result.ExitCode != 0 {
		s.logOperation(ctx, id, "snapshot.restore", false, "VBoxManage snapshot restore failed.")
		return models.SnapshotOperationResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       "VBoxManage snapshot restore failed",
		}
	}

	s.logOperation(ctx, id, "snapshot.restore", true, "")
	return models.SnapshotOperationResponse{
		Success: true,
		VMID:    id,
		Message: "Snapshot restored. The VM was rolled back and is powered off — start it to boot the restored state.",
	}, nil
}

// DeleteSnapshot removes a snapshot, merging its differencing disk into its
// parent. VirtualBox can delete online, so the VM need not be stopped.
func (s *service) DeleteSnapshot(ctx context.Context, id, snapshotID string) (models.SnapshotOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.SnapshotOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if !IsValidVmID(snapshotID) {
		return models.SnapshotOperationResponse{}, &ValidationError{Message: "Invalid snapshot identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "snapshot.delete", false, "VirtualBox/VBoxManage not discovered.")
		return models.SnapshotOperationResponse{}, err
	}

	// Merging a large differencing disk can take a while.
	result, runErr := s.runForVM(ctx, id, path, snapshotDeleteArgs(id, snapshotID), 5*time.Minute)
	if runErr != nil || result.ExitCode != 0 {
		s.logOperation(ctx, id, "snapshot.delete", false, "VBoxManage snapshot delete failed.")
		return models.SnapshotOperationResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       "VBoxManage snapshot delete failed",
		}
	}

	s.logOperation(ctx, id, "snapshot.delete", true, "")
	return models.SnapshotOperationResponse{
		Success: true,
		VMID:    id,
		Message: "Snapshot deleted.",
	}, nil
}

// waitUntilNotLive polls the VM state until it is no longer executing, so a
// power-off completes and releases the session before a restore is attempted.
func (s *service) waitUntilNotLive(ctx context.Context, path, id string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if info, err := s.readShowVmInfo(ctx, path, id, "polling VM state"); err == nil {
			if !vmStateIsLive(parseVmState(info)) {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for the VM to power off")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// validateSnapshotName enforces a conservative snapshot name: 1-100 characters,
// no control characters, and no leading dash (so it cannot be read as a flag by
// VBoxManage). Spaces and punctuation common in "Before update - 2026" names are
// allowed.
func validateSnapshotName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" || len(n) > 100 || n[0] == '-' {
		return &ValidationError{Message: "Snapshot name must be 1-100 characters and cannot start with a dash."}
	}
	for _, r := range n {
		if r < 0x20 || r == 0x7f {
			return &ValidationError{Message: "Snapshot name contains unsupported characters."}
		}
	}
	return nil
}

// validateSnapshotDescription allows an optional, longer free-text description.
func validateSnapshotDescription(description string) error {
	if len(description) > 512 {
		return &ValidationError{Message: "Snapshot description must be 512 characters or fewer."}
	}
	for _, r := range description {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f {
			return &ValidationError{Message: "Snapshot description contains unsupported characters."}
		}
	}
	return nil
}

func snapshotListArgs(id string) []string {
	return []string{"snapshot", id, "list", "--machinereadable"}
}

func snapshotTakeArgs(id, name, description string) []string {
	args := []string{"snapshot", id, "take", strings.TrimSpace(name)}
	if strings.TrimSpace(description) != "" {
		args = append(args, "--description", description)
	}
	return args
}

func snapshotRestoreArgs(id, snapshotID string) []string {
	return []string{"snapshot", id, "restore", snapshotID}
}

func snapshotDeleteArgs(id, snapshotID string) []string {
	return []string{"snapshot", id, "delete", snapshotID}
}

func powerOffVmArgs(id string) []string {
	return []string{"controlvm", id, "poweroff"}
}
