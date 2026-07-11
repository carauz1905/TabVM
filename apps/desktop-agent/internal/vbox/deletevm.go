package vbox

import (
	"context"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// DeleteVM unregisters a VM and deletes its files (disk images, saved state,
// logs, and the machine folder). Deletion is irreversible, so a live VM is
// refused instead of being powered off implicitly — the user must stop it
// deliberately first.
func (s *service) DeleteVM(ctx context.Context, id string) (models.VmOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "vm.delete", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state before delete")
	if err != nil {
		return models.VmOperationResponse{}, err
	}
	if vmStateIsLive(parseVmState(info)) {
		return models.VmOperationResponse{}, &ValidationError{Message: "The VM is running. Power it off before deleting it."}
	}

	// Deleting large disk images can take a while.
	result, runErr := s.runner.RunContext(ctx, path, unregisterVmArgs(id), 5*time.Minute)
	if runErr != nil || result.ExitCode != 0 {
		s.logOperation(ctx, id, "vm.delete", false, "VBoxManage unregistervm failed.")
		return models.VmOperationResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       "VBoxManage unregistervm failed",
		}
	}

	s.logOperation(ctx, id, "vm.delete", true, "")
	return models.VmOperationResponse{
		Success: true,
		VMID:    id,
		Message: "VM deleted. Its disks and configuration files were removed.",
	}, nil
}

func unregisterVmArgs(id string) []string {
	return []string{"unregistervm", id, "--delete"}
}
