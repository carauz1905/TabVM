package vbox

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/console"
	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/runner"
	"github.com/tabvm/desktop-agent/internal/store"
)

const (
	minConsolePort = 5000
	maxConsolePort = 5999
)

// ConsolePortRange is the inclusive range of deterministic local VRDE ports.
var ConsolePortRange = struct {
	Min int
	Max int
}{Min: minConsolePort, Max: maxConsolePort}

// Runner is the subset of runner.Runner used by the VirtualBox service.
type Runner interface {
	RunContext(ctx context.Context, name string, args []string, timeout time.Duration) (runner.Result, error)
}

// Config provides the settings needed to discover and invoke VBoxManage.
type Config struct {
	CandidatePaths []string
	// Store is optional. When provided, console port assignments are persisted
	// across agent restarts.
	Store *store.Store
}

// Default discovery paths on Windows when none are configured.
var defaultPaths = []string{
	`C:\Program Files\Oracle\VirtualBox\VBoxManage.exe`,
	`C:\Program Files (x86)\Oracle\VirtualBox\VBoxManage.exe`,
}

// NewService creates a VirtualBox service using the provided runner, config,
// and optional store.
func NewService(runner Runner, cfg Config) Service {
	paths := cfg.CandidatePaths
	if len(paths) == 0 {
		paths = defaultPaths
	}
	return &service{runner: runner, paths: paths, store: cfg.Store}
}

type service struct {
	runner Runner
	paths  []string
	store  *store.Store
}

// Discover reports whether VirtualBox / VBoxManage is available. The response
// intentionally does not include the resolved executable path; see
// models.VirtualBoxDiscovery for details.
func (s *service) Discover(ctx context.Context) models.VirtualBoxDiscovery {
	_, version, found, errMsg := s.findVBoxManage(ctx)
	return models.VirtualBoxDiscovery{
		Found:   found,
		Version: version,
		Error:   errMsg,
	}
}

// findVBoxManage attempts to locate a working VBoxManage executable. It returns
// the resolved path, the reported version, true on success, or an empty path
// and an error message on failure.
func (s *service) findVBoxManage(ctx context.Context) (string, string, bool, string) {
	if !isWindows() {
		return "", "", false, "TabVM currently supports Windows hosts only."
	}

	for _, path := range s.paths {
		if path == "" {
			continue
		}

		resolved, err := resolvePath(path)
		if err != nil {
			continue
		}

		version, err := s.readVersion(ctx, resolved)
		if err != nil {
			continue
		}

		return resolved, version, true, ""
	}

	return "", "", false, "VBoxManage was not found in the configured search paths."
}

func (s *service) ListVMs(ctx context.Context) (models.VmListResponse, error) {
	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmListResponse{}, err
	}

	result, err := s.runner.RunContext(ctx, path, listVmsArgs(), 30*time.Second)
	if err != nil {
		return models.VmListResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while listing VMs: %v", err),
		}
	}

	if result.ExitCode != 0 {
		return models.VmListResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while listing VMs", result.ExitCode),
		}
	}

	vms := parseListVmsOutput(result.StandardOutput)
	s.enhanceVmStates(ctx, path, vms)

	return models.VmListResponse{VMs: vms}, nil
}

func (s *service) VMStatus(ctx context.Context, id string) (models.VmStatusResponse, error) {
	if !IsValidVmID(id) {
		return models.VmStatusResponse{}, &ExecutionError{
			ExitCode: -1,
			Message:  "Invalid VM identifier.",
		}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmStatusResponse{}, err
	}

	result, runErr := s.runner.RunContext(ctx, path, showVmInfoArgs(id), 10*time.Second)
	if runErr != nil {
		return models.VmStatusResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while reading VM status: %v", runErr),
		}
	}

	if result.ExitCode != 0 {
		return models.VmStatusResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while reading VM status", result.ExitCode),
		}
	}

	state := parseVmState(result.StandardOutput)
	if state == "" {
		state = "unknown"
	}

	return models.VmStatusResponse{ID: id, State: state}, nil
}

// VmTelemetry returns the configured CPU/RAM of a VM plus its network interfaces
// with guest-reported IPv4 addresses. CPU, RAM, NIC mode and MAC come from
// showvminfo (host-side, always available). IPv4 addresses come from Guest
// Additions via guestproperty and are correlated to each NIC by MAC, so they
// work across all network modes (NAT, bridged, host-only). When the guest is
// not running or has no Guest Additions, GuestAdditions is false and addresses
// are empty rather than an error.
func (s *service) VmTelemetry(ctx context.Context, id string) (models.VmTelemetryResponse, error) {
	if !IsValidVmID(id) {
		return models.VmTelemetryResponse{}, &ExecutionError{
			ExitCode: -1,
			Message:  "Invalid VM identifier.",
		}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmTelemetryResponse{}, err
	}

	info, runErr := s.runner.RunContext(ctx, path, showVmInfoArgs(id), 10*time.Second)
	if runErr != nil {
		return models.VmTelemetryResponse{}, &ExecutionError{
			ExitCode:      info.ExitCode,
			StandardError: info.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while reading VM telemetry: %v", runErr),
		}
	}
	if info.ExitCode != 0 {
		return models.VmTelemetryResponse{}, &ExecutionError{
			ExitCode:      info.ExitCode,
			StandardError: info.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while reading VM telemetry", info.ExitCode),
		}
	}

	cpuCount, ramMB := parseVmResources(info.StandardOutput)
	nics := parseVmNICs(info.StandardOutput)

	// guestproperty requires a running guest with Guest Additions. A failure or
	// non-zero exit here is expected for stopped VMs, so degrade gracefully
	// instead of failing the whole telemetry read.
	gaPresent := false
	ipsByMAC := map[string][]string{}
	if gp, gpErr := s.runner.RunContext(ctx, path, guestPropertyEnumerateArgs(id), 10*time.Second); gpErr == nil && gp.ExitCode == 0 {
		gaPresent, ipsByMAC = parseGuestNetworks(gp.StandardOutput)
	}

	networks := make([]models.NetworkInterface, 0, len(nics))
	for _, nic := range nics {
		ips := ipsByMAC[nic.mac]
		if ips == nil {
			ips = []string{}
		}
		networks = append(networks, models.NetworkInterface{
			Slot: nic.slot,
			Mode: nic.mode,
			MAC:  nic.mac,
			IPv4: ips,
		})
	}

	// Disk capacity/allocation is host-side (no Guest Additions). Each attached
	// disk needs its own showmediuminfo call; a failure on one disk is skipped
	// rather than failing the whole telemetry read.
	disks := make([]models.DiskUsage, 0)
	for _, att := range parseDiskAttachments(info.StandardOutput) {
		if att.uuid == "" {
			continue
		}
		med, medErr := s.runner.RunContext(ctx, path, showMediumInfoArgs(att.uuid), 10*time.Second)
		if medErr != nil || med.ExitCode != 0 {
			continue
		}
		capacity, allocated := parseMediumInfo(med.StandardOutput)
		percent := 0
		if capacity > 0 {
			percent = int((allocated * 100) / capacity)
		}
		disks = append(disks, models.DiskUsage{
			Name:           att.name,
			CapacityBytes:  capacity,
			AllocatedBytes: allocated,
			Percent:        percent,
		})
	}

	return models.VmTelemetryResponse{
		ID:             id,
		CPUCount:       cpuCount,
		RAMMB:          ramMB,
		GuestAdditions: gaPresent,
		Networks:       networks,
		Disks:          disks,
	}, nil
}

// StartVM brings a VM up, inspecting its current state first so it neither
// double-starts a running VM nor destroys a saved session. A running or starting
// VM is treated as an idempotent success, a paused VM is resumed, and everything
// else takes the normal startvm path. It never auto-powers-off (which would lose
// a saved state). Transient "already locked" contention is retried a few times.
func (s *service) StartVM(ctx context.Context, id string) error {
	if !IsValidVmID(id) {
		return &ExecutionError{
			ExitCode: -1,
			Message:  "Invalid VM identifier.",
		}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "vm.start", false, "VirtualBox/VBoxManage not discovered.")
		return err
	}

	// The state read is best-effort: if it fails we fall through to the normal
	// startvm path rather than block a start on a diagnostic call.
	state := ""
	if info, infoErr := s.readShowVmInfo(ctx, path, id, "reading VM state before start"); infoErr == nil {
		state = strings.ToLower(strings.TrimSpace(parseVmState(info)))
	}

	switch state {
	case "running", "starting":
		// Already up (or on its way); a second startvm would error. No-op success.
		s.logOperation(ctx, id, "vm.start", true, "VM already running.")
		return nil
	case "paused":
		// A paused VM must be resumed, not started.
		if err := s.runControlCommand(ctx, path, resumeVmArgs(id), "resuming VM"); err != nil {
			s.logOperation(ctx, id, "vm.start", false, controlFailureMessage("resuming VM", err))
			return err
		}
		s.logOperation(ctx, id, "vm.start", true, "")
		return nil
	default:
		// saved, poweroff, aborted, or unknown: normal start (never poweroff first).
		if err := s.startWithRetry(ctx, path, id); err != nil {
			s.logOperation(ctx, id, "vm.start", false, controlFailureMessage("starting VM", err))
			return err
		}
		s.logOperation(ctx, id, "vm.start", true, "")
		return nil
	}
}

// startWithRetry issues startvm and retries a bounded number of times when it
// fails because the VM is momentarily locked by another VirtualBox session, a
// transient condition behind intermittent start failures. It is context-aware
// and returns the last *ExecutionError when contention persists.
func (s *service) startWithRetry(ctx context.Context, path, id string) error {
	const maxAttempts = 3
	const backoff = 400 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := s.runControlCommand(ctx, path, startVmArgs(id), "starting VM")
		if err == nil {
			return nil
		}
		lastErr = err
		if !isLockContention(err) {
			return err
		}
		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return lastErr
}

// isLockContention reports whether an execution error indicates the VM was
// momentarily locked by another VirtualBox session, which is typically
// transient and worth retrying.
func isLockContention(err error) bool {
	execErr, ok := err.(*ExecutionError)
	if !ok {
		return false
	}
	stderr := strings.ToLower(execErr.StandardError)
	return strings.Contains(stderr, "already locked") ||
		strings.Contains(stderr, strings.ToLower("VBOX_E_INVALID_OBJECT_STATE"))
}

func (s *service) StopVM(ctx context.Context, id string) error {
	return s.runLoggedControlCommand(ctx, id, "vm.stop", stopVmArgs, "stopping VM")
}

func (s *service) ResetVM(ctx context.Context, id string) error {
	return s.runLoggedControlCommand(ctx, id, "vm.reset", resetVmArgs, "resetting VM")
}

// ForcePowerOff hard-stops a VM ("controlvm poweroff"). It is the fallback for
// guests that never answer the ACPI power button (no OS installed, stuck in an
// installer) and is equivalent to pulling the power plug.
func (s *service) ForcePowerOff(ctx context.Context, id string) error {
	return s.runLoggedControlCommand(ctx, id, "vm.poweroff", powerOffVmArgs, "forcing VM power off")
}

// runLoggedControlCommand validates the VM ID, resolves VBoxManage, runs the
// control command, and records the outcome in the operation log when a store
// is configured.
func (s *service) runLoggedControlCommand(
	ctx context.Context,
	id string,
	action string,
	argsFn func(string) []string,
	description string,
) error {
	if !IsValidVmID(id) {
		return &ExecutionError{
			ExitCode: -1,
			Message:  "Invalid VM identifier.",
		}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, action, false, "VirtualBox/VBoxManage not discovered.")
		return err
	}

	err = s.runControlCommand(ctx, path, argsFn(id), description)
	if err != nil {
		s.logOperation(ctx, id, action, false, controlFailureMessage(description, err))
		return err
	}
	s.logOperation(ctx, id, action, true, "")
	return nil
}

// controlFailureMessage builds an operation-log message that preserves the exit
// code and the raw stderr, giving an actionable reason instead of a generic
// failure. The store sanitizes host paths and secret-like tokens before
// persisting, so echoing stderr here is safe. This is the operation-log sink
// only; the UI-facing message is mapped separately in the server.
func controlFailureMessage(description string, err error) string {
	if execErr, ok := err.(*ExecutionError); ok {
		return fmt.Sprintf("%s failed (exit code %d): %s", description, execErr.ExitCode, execErr.StandardError)
	}
	return "VirtualBox control command failed."
}

func (s *service) VmConsoleStatus(ctx context.Context, id string) (models.VmConsoleStatusResponse, error) {
	if !IsValidVmID(id) {
		return models.VmConsoleStatusResponse{}, &ExecutionError{
			ExitCode: -1,
			Message:  "Invalid VM identifier.",
		}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmConsoleStatusResponse{}, err
	}

	result, runErr := s.runner.RunContext(ctx, path, showVmInfoArgs(id), 10*time.Second)
	if runErr != nil {
		return models.VmConsoleStatusResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while reading console status: %v", runErr),
		}
	}

	if result.ExitCode != 0 {
		return models.VmConsoleStatusResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while reading console status", result.ExitCode),
		}
	}

	return buildConsoleStatusResponse(id, result.StandardOutput), nil
}

func (s *service) PrepareVmConsole(ctx context.Context, id string) (models.VmConsoleStatusResponse, error) {
	if !IsValidVmID(id) {
		return models.VmConsoleStatusResponse{}, &ExecutionError{
			ExitCode: -1,
			Message:  "Invalid VM identifier.",
		}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmConsoleStatusResponse{}, err
	}

	port, err := s.findConsolePort(ctx, path, id)
	if err != nil {
		return models.VmConsoleStatusResponse{}, err
	}

	args := enableVrdeArgs(id, port)
	result, runErr := s.runner.RunContext(ctx, path, args, 30*time.Second)
	if runErr != nil {
		return models.VmConsoleStatusResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while preparing console: %v", runErr),
		}
	}

	if result.ExitCode != 0 {
		return models.VmConsoleStatusResponse{}, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while preparing console", result.ExitCode),
		}
	}

	status, err := s.VmConsoleStatus(ctx, id)
	if err != nil {
		return models.VmConsoleStatusResponse{}, err
	}

	// Persist the port only after prepare and status verification succeed.
	// If status reports a valid local port, use that; otherwise fall back to
	// the port that was passed to VBoxManage.
	verifiedPort := port
	if status.Ready && status.Port != 0 && IsValidConsolePort(status.Port) {
		verifiedPort = status.Port
	}
	s.persistConsolePort(ctx, id, verifiedPort)

	if s.store != nil {
		if logErr := s.store.LogOperation(ctx, id, "console.prepare", true, fmt.Sprintf("VRDE prepared on 127.0.0.1:%d", verifiedPort)); logErr != nil {
			slogDefault().Error("failed to log console prepare operation", "vmId", id, "error", logErr)
		}
	}

	return status, nil
}

// findConsolePort chooses a VRDE port for the VM. A previously persisted port
// is only reused after validating that it does not collide with another VM's
// VRDE assignment and is still available locally. If the VM already has a
// valid, collision-free VRDE port configured, that port is kept. Otherwise the
// search probes forward from the hash-derived candidate.
func (s *service) findConsolePort(ctx context.Context, path, id string) (int, error) {
	persisted, err := s.loadPersistedConsolePort(ctx, id)
	if err != nil {
		return 0, err
	}

	currentPort, err := s.readCurrentVRDEPort(ctx, path, id)
	if err != nil {
		return 0, err
	}

	usedPorts, err := s.collectUsedVRDEPorts(ctx, path, id)
	if err != nil {
		return 0, err
	}

	// If the VM already has a valid, collision-free, locally available port,
	// keep using it and persist it for the next restart.
	if currentPort != 0 && !s.isPortOccupied(currentPort, usedPorts) {
		return currentPort, nil
	}

	// Only reuse a persisted port when it is still collision-free and locally
	// available. Otherwise it is stale and a new port should be selected.
	if persisted != 0 && !s.isPortOccupied(persisted, usedPorts) {
		return persisted, nil
	}

	rangeSize := maxConsolePort - minConsolePort + 1
	start := VmIDToConsolePort(id)

	for offset := 0; offset < rangeSize; offset++ {
		candidate := minConsolePort + ((start - minConsolePort + offset) % rangeSize)
		if s.isPortOccupied(candidate, usedPorts) {
			continue
		}
		return candidate, nil
	}

	return 0, &ExecutionError{
		ExitCode: -1,
		Message:  "No available VRDE port in the configured range 5000-5999.",
	}
}

// isPortOccupied reports whether a port is already assigned to another VM or
// cannot be bound locally.
func (s *service) isPortOccupied(port int, usedPorts map[int]struct{}) bool {
	if _, used := usedPorts[port]; used {
		return true
	}
	return !portAvailable(port)
}

// loadPersistedConsolePort returns a previously persisted console port for the
// VM, or 0 if none exists or persistence is disabled.
func (s *service) loadPersistedConsolePort(ctx context.Context, id string) (int, error) {
	if s.store == nil {
		return 0, nil
	}
	record, err := s.store.GetVmConsolePort(ctx, id)
	if err != nil {
		return 0, &ExecutionError{
			ExitCode: -1,
			Message:  "Failed to read persisted console port assignment.",
		}
	}
	if record == nil {
		return 0, nil
	}
	if !IsValidConsolePort(record.Port) {
		return 0, nil
	}
	return record.Port, nil
}

// persistConsolePort saves the assigned console port to the store. Errors are
// logged but not propagated because persistence is best-effort: the VM has
// already been configured successfully.
func (s *service) persistConsolePort(ctx context.Context, id string, port int) {
	if s.store == nil {
		return
	}
	err := s.store.SetVmConsolePort(ctx, store.VmConsolePort{
		VMID:     id,
		Port:     port,
		Address:  "127.0.0.1",
		Protocol: "rdp",
		Source:   "virtualbox-vrde",
	})
	if err != nil {
		// Persistence failure is not fatal to the console operation, but it is
		// logged so operators can detect a misconfigured data directory.
		slogDefault().Error("failed to persist console port", "vmId", id, "port", port, "error", err)
	}
}

// slogDefault returns the default logger. It is a variable so tests do not need
// to depend on the global default.
var slogDefault = func() *slog.Logger { return slog.Default() }

// readCurrentVRDEPort returns the VRDE port currently configured for the VM,
// or 0 if VRDE is not enabled or no valid port is set.
func (s *service) readCurrentVRDEPort(ctx context.Context, path, id string) (int, error) {
	result, runErr := s.runner.RunContext(ctx, path, showVmInfoArgs(id), 10*time.Second)
	if runErr != nil {
		return 0, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while reading current console port: %v", runErr),
		}
	}
	if result.ExitCode != 0 {
		return 0, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while reading current console port", result.ExitCode),
		}
	}

	info := parseVRDEInfo(result.StandardOutput)
	if !info.enabled {
		return 0, nil
	}
	port, err := strconv.Atoi(info.port)
	if err != nil || !IsValidConsolePort(port) {
		return 0, nil
	}
	return port, nil
}

// collectUsedVRDEPorts returns the set of VRDE ports currently enabled on all
// VMs except the one being prepared. This avoids assigning a port that would
// collide with another VM's remote display server.
func (s *service) collectUsedVRDEPorts(ctx context.Context, path, currentID string) (map[int]struct{}, error) {
	result, runErr := s.runner.RunContext(ctx, path, listVmsArgs(), 30*time.Second)
	if runErr != nil {
		return nil, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while listing VMs for port collision check: %v", runErr),
		}
	}
	if result.ExitCode != 0 {
		return nil, &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while listing VMs for port collision check", result.ExitCode),
		}
	}

	vms := parseListVmsOutput(result.StandardOutput)
	used := make(map[int]struct{})

	for _, vm := range vms {
		if vm.ID == currentID || !IsValidVmID(vm.ID) {
			continue
		}

		infoResult, runErr := s.runner.RunContext(ctx, path, showVmInfoArgs(vm.ID), 10*time.Second)
		if runErr != nil || infoResult.ExitCode != 0 {
			// If we cannot determine another VM's VRDE state, we cannot safely
			// guarantee a collision-free port. Surface the failure sanitized.
			return nil, &ExecutionError{
				ExitCode:      infoResult.ExitCode,
				StandardError: infoResult.StandardError,
				Message:       "VBoxManage failed while checking VRDE ports of other VMs.",
			}
		}

		info := parseVRDEInfo(infoResult.StandardOutput)
		if !info.enabled {
			continue
		}

		port, err := strconv.Atoi(info.port)
		if err != nil || !IsValidConsolePort(port) {
			continue
		}
		used[port] = struct{}{}
	}

	return used, nil
}

// portAvailable reports whether a local TCP port can be bound on the loopback
// interface. It is package-level so tests can override it to simulate occupied
// ports without creating real sockets.
var portAvailable = func(port int) bool {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func (s *service) DisableVmConsole(ctx context.Context, id string) error {
	if !IsValidVmID(id) {
		return &ExecutionError{
			ExitCode: -1,
			Message:  "Invalid VM identifier.",
		}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "console.disable", false, "VirtualBox/VBoxManage not discovered.")
		return err
	}

	err = s.runControlCommand(ctx, path, disableVrdeArgs(id), "disabling console")
	if err != nil {
		s.logOperation(ctx, id, "console.disable", false, "VirtualBox control command failed.")
		return err
	}
	s.logOperation(ctx, id, "console.disable", true, "")
	return nil
}

// logOperation records a lifecycle or console action when a store is available.
// Errors are swallowed to avoid interfering with the primary operation.
func (s *service) logOperation(ctx context.Context, vmID, action string, success bool, message string) {
	if s.store == nil {
		return
	}
	if err := s.store.LogOperation(ctx, vmID, action, success, message); err != nil {
		slogDefault().Error("failed to log operation", "vmId", vmID, "action", action, "error", err)
	}
}

func (s *service) resolveVBoxManage(ctx context.Context) (string, error) {
	path, _, found, _ := s.findVBoxManage(ctx)
	if !found || path == "" {
		return "", &NotDiscoveredError{
			Message: "VirtualBox/VBoxManage was not discovered. VM operations are unavailable.",
		}
	}
	return path, nil
}

func (s *service) runControlCommand(ctx context.Context, path string, args []string, description string) error {
	result, runErr := s.runner.RunContext(ctx, path, args, 30*time.Second)
	if runErr != nil {
		return &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while %s: %v", description, runErr),
		}
	}

	if result.ExitCode != 0 {
		return &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while %s", result.ExitCode, description),
		}
	}

	return nil
}

// enhanceVmStates replaces each VM's placeholder state with its real, normalized
// VirtualBox state (running, booting, paused, saved, powered off, aborted, ...)
// read from `showvminfo --machinereadable`. This is one call per VM, which is
// acceptable because the VM list is refreshed on demand (not polled) and a
// student host runs only a handful of VMs. It is best-effort: if a VM's state
// cannot be read, that VM keeps its placeholder state and the rest still resolve.
func (s *service) enhanceVmStates(ctx context.Context, path string, vms []models.VmInfo) {
	for i := range vms {
		if !IsValidVmID(vms[i].ID) {
			continue
		}
		result, err := s.runner.RunContext(ctx, path, showVmInfoArgs(vms[i].ID), 10*time.Second)
		if err != nil || result.ExitCode != 0 {
			continue
		}
		if state := normalizeVmState(parseVmState(result.StandardOutput)); state != "" {
			vms[i].State = state
		}
	}
}

func listVmsArgs() []string {
	return []string{"list", "vms"}
}

func listRunningVmsArgs() []string {
	return []string{"list", "runningvms"}
}

func showVmInfoArgs(id string) []string {
	return []string{"showvminfo", id, "--machinereadable"}
}

// guestPropertyEnumerateArgs lists the guest-reported GuestInfo properties,
// which include per-NIC IPv4 addresses when Guest Additions is active.
func guestPropertyEnumerateArgs(id string) []string {
	return []string{"guestproperty", "enumerate", id, "--patterns", "/VirtualBox/GuestInfo/*"}
}

// showMediumInfoArgs reads a medium's capacity and host-side allocation.
func showMediumInfoArgs(uuid string) []string {
	return []string{"showmediuminfo", uuid}
}

func startVmArgs(id string) []string {
	return []string{"startvm", id, "--type", "headless"}
}

func stopVmArgs(id string) []string {
	return []string{"controlvm", id, "acpipowerbutton"}
}

func resumeVmArgs(id string) []string {
	return []string{"controlvm", id, "resume"}
}

func resetVmArgs(id string) []string {
	return []string{"controlvm", id, "reset"}
}

func enableVrdeArgs(id string, port int) []string {
	return []string{
		"modifyvm", id,
		"--vrde", "on",
		"--vrdeaddress", "127.0.0.1",
		"--vrdeport", strconv.Itoa(port),
	}
}

func disableVrdeArgs(id string) []string {
	return []string{"modifyvm", id, "--vrde", "off"}
}

// VmIDToConsolePort returns a deterministic port in the range 5000-5999 derived
// from the VM identifier. The mapping is stable for a given ID and avoids
// scanning or binding sockets, but it does not prevent collisions with other
// host services that may already occupy the chosen port.
func VmIDToConsolePort(id string) int {
	h := fnv.New32a()
	// Error is intentionally ignored: Write to fnv.Hash never returns an error.
	_, _ = h.Write([]byte(id))
	offset := int(h.Sum32() % uint32(maxConsolePort-minConsolePort+1))
	return minConsolePort + offset
}

// IsValidConsolePort reports whether port is within the allowed local VRDE range.
func IsValidConsolePort(port int) bool {
	return port >= minConsolePort && port <= maxConsolePort
}

func buildConsoleStatusResponse(id string, output string) models.VmConsoleStatusResponse {
	info := parseVRDEInfo(output)

	port := 0
	if p, err := strconv.Atoi(info.port); err == nil {
		port = p
	}

	ready := info.enabled && info.address == "127.0.0.1" && IsValidConsolePort(port)

	// Only expose loopback address/port/target in normal status metadata.
	// A non-loopback VRDE binding is treated as not ready and sanitized so
	// the response does not advertise a remotely reachable endpoint.
	address := ""
	responsePort := 0
	target := ""
	var targets []models.ConsoleTarget
	if ready {
		address = "127.0.0.1"
		responsePort = port
		target = fmt.Sprintf("127.0.0.1:%d", port)
		targets = append(targets, models.ConsoleTarget{
			Protocol:    console.RDP,
			Host:        "127.0.0.1",
			Port:        port,
			Source:      console.SourceVirtualBoxVRDE,
			DisplayName: "VirtualBox VRDE/RDP",
			Ready:       true,
		})
	}

	message := ""
	if !info.enabled {
		message = "VRDE is not enabled for this VM."
	} else if !ready {
		message = "VRDE is enabled but not configured for a local-only target."
	}

	return models.VmConsoleStatusResponse{
		ID:       id,
		Enabled:  info.enabled,
		Protocol: console.RDP,
		Source:   console.SourceVirtualBoxVRDE,
		Address:  address,
		Port:     responsePort,
		Ready:    ready,
		Target:   target,
		Targets:  targets,
		Message:  message,
	}
}

func (s *service) readVersion(ctx context.Context, path string) (string, error) {
	result, err := s.runner.RunContext(ctx, path, []string{"--version"}, 10*time.Second)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("VBoxManage --version exited with code %d", result.ExitCode)
	}

	lines := strings.Split(strings.TrimSpace(result.StandardOutput), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}

	return "", fmt.Errorf("no version output from VBoxManage")
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			return "", err
		}
		return filepath.Clean(path), nil
	}

	// Relative names such as "VBoxManage" are resolved against PATH.
	resolved, err := execLookPath(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

// execLookPath wraps exec.LookPath so it can be stubbed in tests.
var execLookPath = exec.LookPath
