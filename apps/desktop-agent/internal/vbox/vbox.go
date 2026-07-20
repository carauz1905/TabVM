package vbox

import (
	"context"
	"fmt"
	"regexp"

	"github.com/tabvm/desktop-agent/internal/models"
)

// uuidPattern matches canonical UUID-shaped identifiers as produced by
// VirtualBox (e.g. 550e8400-e29b-41d4-a716-446655440000).
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Service discovers VirtualBox and controls registered VMs.
type Service interface {
	Discover(ctx context.Context) models.VirtualBoxDiscovery
	ListVMs(ctx context.Context) (models.VmListResponse, error)
	VMStatus(ctx context.Context, id string) (models.VmStatusResponse, error)
	StartVM(ctx context.Context, id string) error
	StopVM(ctx context.Context, id string) error
	ResetVM(ctx context.Context, id string) error
	DeleteVM(ctx context.Context, id string) (models.VmOperationResponse, error)
	VmGuestOS(ctx context.Context, id string) (models.VmGuestOSResponse, error)
	SerialConsoleStatus(ctx context.Context, id string) (models.VmSerialConsoleResponse, error)
	EnableSerialConsole(ctx context.Context, id string) (models.VmOperationResponse, error)
	DisableSerialConsole(ctx context.Context, id string) (models.VmOperationResponse, error)
	EnableSerialGetty(ctx context.Context, id, username, password string) (models.SerialGettyResponse, error)
	VmHardware(ctx context.Context, id string) (models.VmHardwareResponse, error)
	SetVmHardware(ctx context.Context, id string, cpus, memoryMB int) (models.VmOperationResponse, error)
	VmStorage(ctx context.Context, id string) (models.VmStorageResponse, error)
	ResizeDisk(ctx context.Context, id, uuid string, sizeMB int64) (models.VmOperationResponse, error)
	AddDisk(ctx context.Context, id string, sizeMB int64) (models.VmOperationResponse, error)
	DetachDisk(ctx context.Context, id, uuid string, deleteFile bool) (models.VmOperationResponse, error)
	VmConsoleStatus(ctx context.Context, id string) (models.VmConsoleStatusResponse, error)
	PrepareVmConsole(ctx context.Context, id string) (models.VmConsoleStatusResponse, error)
	DisableVmConsole(ctx context.Context, id string) error
	VmTelemetry(ctx context.Context, id string) (models.VmTelemetryResponse, error)
	ListSharedFolders(ctx context.Context, id string) (models.SharedFoldersResponse, error)
	AddSharedFolder(ctx context.Context, id, name, hostPath string) (models.SharedFolderOperationResponse, error)
	RemoveSharedFolder(ctx context.Context, id, name string) (models.SharedFolderOperationResponse, error)
	GetClipboardMode(ctx context.Context, id string) (models.ClipboardModeResponse, error)
	SetClipboardMode(ctx context.Context, id, mode string) (models.ClipboardModeResponse, error)
	GuestAdditionsStatus(ctx context.Context, id string) (models.GuestAdditionsStatusResponse, error)
	InstallGuestAdditions(ctx context.Context, id string) (models.GuestAdditionsInstallResponse, error)
	UpdateGuestAdditions(ctx context.Context, id, username, password string) (models.GuestAdditionsUpdateResponse, error)
	TransferFileToGuest(ctx context.Context, id, filename string, data []byte, username, password string) (models.VmFileTransferResponse, error)
	NetworkOptions(ctx context.Context, id string) (models.NetworkOptionsResponse, error)
	ChangeNetworkMode(ctx context.Context, id string, slot int, mode, adapter string) (models.NetworkOperationResponse, error)
	AddPortForwarding(ctx context.Context, id string, req models.PortForwardingRequest) (models.NetworkOperationResponse, error)
	DeletePortForwarding(ctx context.Context, id string, slot int, name string) (models.NetworkOperationResponse, error)
	ListSnapshots(ctx context.Context, id string) (models.SnapshotsResponse, error)
	TakeSnapshot(ctx context.Context, id, name, description string) (models.SnapshotOperationResponse, error)
	RestoreSnapshot(ctx context.Context, id, snapshotID string) (models.SnapshotOperationResponse, error)
	DeleteSnapshot(ctx context.Context, id, snapshotID string) (models.SnapshotOperationResponse, error)
	ImportAppliance(ctx context.Context, ovaPath, name string) (models.VmCreateResponse, error)
	CreateVmUnattended(ctx context.Context, req models.VmCreateRequest) (models.VmCreateResponse, error)
	ForcePowerOff(ctx context.Context, id string) error
	CreateVmManual(ctx context.Context, req models.VmCreateManualRequest) (models.VmCreateResponse, error)
	ValidateClone(ctx context.Context, sourceID, name string, linked bool) error
	CloneVM(ctx context.Context, sourceID, name string, linked bool) (models.VmCreateResponse, error)
}

// NotDiscoveredError indicates that VirtualBox/VBoxManage could not be located.
type NotDiscoveredError struct {
	Message string
}

func (e *NotDiscoveredError) Error() string {
	return e.Message
}

// ValidationError indicates that caller-supplied input (e.g. a shared folder
// name or host path) failed validation before any VBoxManage command was run.
// It is mapped to an HTTP 400 by the server.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ExecutionError indicates that VBoxManage ran but returned a non-zero exit code.
type ExecutionError struct {
	ExitCode      int
	StandardError string
	Message       string
}

func (e *ExecutionError) Error() string {
	return fmt.Sprintf("%s. Exit code: %d. Standard error: %s", e.Message, e.ExitCode, e.StandardError)
}

// IsValidVmID reports whether id is a safe VirtualBox VM identifier.
// VirtualBox lists VMs with canonical UUID-shaped identifiers, so only
// 8-4-4-4-12 UUID strings are accepted. This rejects shell metacharacters,
// path traversal, arbitrary test IDs, and overly long input before they
// reach VBoxManage.
func IsValidVmID(id string) bool {
	return uuidPattern.MatchString(id)
}
