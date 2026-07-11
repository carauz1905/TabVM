package vbox

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// Fallback bounds used when the host limits cannot be read. VirtualBox itself
// accepts less memory, but below 128 MB no modern guest boots.
const (
	minVmMemoryMB    = 128
	maxFallbackCPUs  = 64
	maxFallbackMemMB = 1048576 // 1 TB
)

// VmHardware returns a VM's configured vCPU count and memory, plus the host's
// totals so the UI can bound its inputs. A live VM is reported as read-only
// because `modifyvm` only works on a powered-off machine.
func (s *service) VmHardware(ctx context.Context, id string) (models.VmHardwareResponse, error) {
	if !IsValidVmID(id) {
		return models.VmHardwareResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmHardwareResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM hardware")
	if err != nil {
		return models.VmHardwareResponse{}, err
	}

	cpus, memoryMB := parseConfiguredHardware(info)
	hostCPUs, hostMemMB := s.readHostLimits(ctx, path)
	return models.VmHardwareResponse{
		ID:           id,
		CPUs:         cpus,
		MemoryMB:     memoryMB,
		HostCPUs:     hostCPUs,
		HostMemoryMB: hostMemMB,
		Editable:     !vmStateIsLive(parseVmState(info)),
	}, nil
}

// SetVmHardware writes a new vCPU count and memory size into a stopped VM's
// configuration. VirtualBox cannot change either on a live machine, so a live
// VM is refused rather than powered off implicitly.
func (s *service) SetVmHardware(ctx context.Context, id string, cpus, memoryMB int) (models.VmOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if cpus < 1 {
		return models.VmOperationResponse{}, &ValidationError{Message: "vCPU count must be at least 1."}
	}
	if memoryMB < minVmMemoryMB {
		return models.VmOperationResponse{}, &ValidationError{Message: fmt.Sprintf("Memory must be at least %d MB.", minVmMemoryMB)}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "vm.hardware", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmOperationResponse{}, err
	}

	hostCPUs, hostMemMB := s.readHostLimits(ctx, path)
	if hostCPUs == 0 {
		hostCPUs = maxFallbackCPUs
	}
	if hostMemMB == 0 {
		hostMemMB = maxFallbackMemMB
	}
	if cpus > hostCPUs {
		return models.VmOperationResponse{}, &ValidationError{Message: fmt.Sprintf("vCPU count cannot exceed the host's %d processors.", hostCPUs)}
	}
	if memoryMB > hostMemMB {
		return models.VmOperationResponse{}, &ValidationError{Message: fmt.Sprintf("Memory cannot exceed the host's %d MB.", hostMemMB)}
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state before hardware change")
	if err != nil {
		return models.VmOperationResponse{}, err
	}
	if vmStateIsLive(parseVmState(info)) {
		return models.VmOperationResponse{}, &ValidationError{Message: "The VM is running. Power it off before changing vCPU or memory."}
	}

	if err := s.runControlCommand(ctx, path, modifyHardwareArgs(id, cpus, memoryMB), "changing VM hardware"); err != nil {
		s.logOperation(ctx, id, "vm.hardware", false, "VBoxManage modifyvm hardware change failed.")
		return models.VmOperationResponse{}, err
	}

	s.logOperation(ctx, id, "vm.hardware", true, "")
	return models.VmOperationResponse{
		Success: true,
		VMID:    id,
		Message: fmt.Sprintf("Hardware updated: %d vCPU, %d MB memory.", cpus, memoryMB),
	}, nil
}

// parseConfiguredHardware extracts the configured cpus/memory values from
// machine-readable showvminfo output.
func parseConfiguredHardware(output string) (cpus, memoryMB int) {
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadable(line)
		if !ok {
			continue
		}
		switch key {
		case "cpus":
			if n, err := strconv.Atoi(value); err == nil {
				cpus = n
			}
		case "memory":
			if n, err := strconv.Atoi(value); err == nil {
				memoryMB = n
			}
		}
	}
	return cpus, memoryMB
}

// readHostLimits returns the host's processor count and memory in MB from
// `list hostinfo`. Best-effort: zeros on failure so callers fall back to
// conservative static bounds.
func (s *service) readHostLimits(ctx context.Context, path string) (cpus, memoryMB int) {
	result, err := s.runner.RunContext(ctx, path, []string{"list", "hostinfo"}, 10*time.Second)
	if err != nil || result.ExitCode != 0 {
		return 0, 0
	}
	for _, line := range strings.Split(result.StandardOutput, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := afterLabel(line, "Processor count:"); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				cpus = n
			}
		} else if v, ok := afterLabel(line, "Memory size:"); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "MByte"))); err == nil {
				memoryMB = n
			}
		}
	}
	return cpus, memoryMB
}

func modifyHardwareArgs(id string, cpus, memoryMB int) []string {
	return []string{"modifyvm", id, "--cpus", strconv.Itoa(cpus), "--memory", strconv.Itoa(memoryMB)}
}
