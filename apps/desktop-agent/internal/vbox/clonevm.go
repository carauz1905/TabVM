package vbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// cloneTimeout bounds a whole clone. A full clone copies every disk image, so it
// can take many minutes; callers run it on a background job.
const cloneTimeout = 30 * time.Minute

// ValidateClone runs the synchronous preconditions for a clone (valid ID, valid
// name, the source powered off, and — for a linked clone — the source has at
// least one snapshot). The server calls it before starting the background clone
// job so the user gets an immediate error instead of a job that fails later.
func (s *service) ValidateClone(ctx context.Context, sourceID, name string, linked bool) error {
	if !IsValidVmID(sourceID) {
		return &ValidationError{Message: "Invalid VM identifier."}
	}
	if err := validateVmName(name); err != nil {
		return err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return err
	}
	return s.checkCloneSource(ctx, path, sourceID, linked)
}

// CloneVM clones a stopped source VM into a new registered VM, either a full copy
// (independent disks) or a linked clone (which shares the source's disks via a
// differencing image and therefore requires the source to have a snapshot). It
// is long-running for full clones; callers run it on a background job.
func (s *service) CloneVM(ctx context.Context, sourceID, name string, linked bool) (models.VmCreateResponse, error) {
	// Name and ID are validated up front, before any VBoxManage call, so a bad
	// request fails fast and identically whether or not VBoxManage is present.
	if !IsValidVmID(sourceID) {
		return models.VmCreateResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if err := validateVmName(name); err != nil {
		return models.VmCreateResponse{}, err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, sourceID, "vm.clone", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmCreateResponse{}, err
	}

	if err := s.checkCloneSource(ctx, path, sourceID, linked); err != nil {
		return models.VmCreateResponse{}, err
	}

	if err := s.runControlCommandTimeout(ctx, path, cloneVmArgs(sourceID, name, linked), "cloning VM", cloneTimeout); err != nil {
		s.logOperation(ctx, sourceID, "vm.clone", false, "VBoxManage clonevm failed.")
		return models.VmCreateResponse{}, err
	}

	uuid := s.resolveVmUUID(ctx, path, name)
	s.logOperation(ctx, uuid, "vm.clone", true, "")

	kind := "Full"
	if linked {
		kind = "Linked"
	}
	return models.VmCreateResponse{
		Success: true,
		VMID:    uuid,
		Name:    name,
		Message: fmt.Sprintf("%s clone %q created and registered.", kind, name),
	}, nil
}

// checkCloneSource verifies the source VM is in a state that can be cloned: it
// must be powered off (a running or otherwise-live VM is refused, mirroring the
// delete guard), and a linked clone additionally requires an existing snapshot.
func (s *service) checkCloneSource(ctx context.Context, path, sourceID string, linked bool) error {
	info, err := s.readShowVmInfo(ctx, path, sourceID, "reading VM state before clone")
	if err != nil {
		return err
	}
	if vmStateIsLive(parseVmState(info)) {
		return &ValidationError{Message: "The VM is running. Power it off before cloning it."}
	}

	if linked {
		hasSnapshot, err := s.sourceHasSnapshot(ctx, path, sourceID)
		if err != nil {
			return err
		}
		if !hasSnapshot {
			return &ValidationError{Message: "A linked clone requires a snapshot. Take a snapshot of the source VM first, then clone it."}
		}
	}
	return nil
}

// sourceHasSnapshot reports whether the VM has at least one snapshot. A VM with
// no snapshots makes `snapshot list` print "does not have any snapshots" and
// exit non-zero — that is an empty list, not an error (mirrors ListSnapshots).
func (s *service) sourceHasSnapshot(ctx context.Context, path, id string) (bool, error) {
	result, runErr := s.runner.RunContext(ctx, path, snapshotListArgs(id), 15*time.Second)
	if runErr != nil {
		return false, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while listing snapshots: %v", runErr),
		}
	}
	if strings.Contains(result.StandardOutput, "does not have any snapshots") ||
		strings.Contains(result.StandardError, "does not have any snapshots") {
		return false, nil
	}
	if result.ExitCode != 0 {
		return false, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while listing snapshots", result.ExitCode),
		}
	}
	snaps, _ := parseSnapshots(result.StandardOutput)
	return len(snaps) > 0, nil
}

// cloneVmArgs builds the VBoxManage clonevm command. A full clone copies the
// disks; a linked clone adds `--options link` so it is created against the
// source's current snapshot.
func cloneVmArgs(sourceID, name string, linked bool) []string {
	args := []string{"clonevm", sourceID, "--name", name, "--register"}
	if linked {
		args = append(args, "--options", "link")
	}
	return args
}
