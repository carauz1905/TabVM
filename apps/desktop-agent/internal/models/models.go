package models

import (
	"time"

	"github.com/tabvm/desktop-agent/internal/console"
)

// HealthStatus is the response shape for GET /health. UptimeSeconds is how long
// the agent process has been running.
type HealthStatus struct {
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	UptimeSeconds int64     `json:"uptimeSeconds"`
	Version       string    `json:"version,omitempty"`
}

// UpdateStatus is the response shape for GET /api/update-status. It reports
// whether a newer TabVM release exists on GitHub. The check is best-effort: on
// any failure (offline, rate-limited, malformed) the agent returns a safe
// payload with UpdateAvailable=false and an empty Latest, never an error, so the
// local-first UI is never blocked. Latest is the normalized version (no leading
// "v"); ReleaseURL links to the GitHub release download page.
type UpdateStatus struct {
	Current         string `json:"current"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl,omitempty"`
}

// VirtualBoxDiscovery is the response shape for GET /api/vbox/discovery.
// It intentionally omits the resolved VBoxManage path; host-side path details
// should only be exposed through a future authenticated diagnostics endpoint,
// not the normal discovery response.
type VirtualBoxDiscovery struct {
	Found   bool   `json:"found"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// VmInfo represents a single VirtualBox VM as expected by the web UI.
type VmInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// VmListResponse is the response shape for GET /api/vms.
type VmListResponse struct {
	VMs []VmInfo `json:"vms"`
}

// VmOperationResponse is the response shape for VM lifecycle operations.
type VmOperationResponse struct {
	Success bool   `json:"success"`
	VMID    string `json:"vmId"`
	Message string `json:"message"`
}

// VmStatusResponse is the response shape for GET /api/vms/{id}/status.
type VmStatusResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// ConsoleTarget is a single protocol-capable console endpoint.
type ConsoleTarget struct {
	Protocol    console.Protocol `json:"protocol"`
	Host        string           `json:"host"`
	Port        int              `json:"port"`
	Source      console.Source   `json:"source"`
	DisplayName string           `json:"displayName"`
	Ready       bool             `json:"ready"`
}

// ConsoleCapability describes a supported console protocol and what the agent
// can currently do with it.
type ConsoleCapability struct {
	ID               console.Protocol `json:"id"`
	DisplayName      string           `json:"displayName"`
	CanAutoConfigure bool             `json:"canAutoConfigure"`
	Description      string           `json:"description"`
}

// ConsoleProtocolsResponse is the response shape for GET /api/console/protocols.
type ConsoleProtocolsResponse struct {
	Protocols []ConsoleCapability `json:"protocols"`
}

// VmConsoleStatusResponse is the response shape for GET /api/vms/{id}/console.
type VmConsoleStatusResponse struct {
	ID       string           `json:"id"`
	Enabled  bool             `json:"enabled"`
	Protocol console.Protocol `json:"protocol,omitempty"`
	Source   console.Source   `json:"source,omitempty"`
	Address  string           `json:"address,omitempty"`
	Port     int              `json:"port,omitempty"`
	Ready    bool             `json:"ready"`
	Target   string           `json:"target,omitempty"`
	Targets  []ConsoleTarget  `json:"targets,omitempty"`
	Message  string           `json:"message,omitempty"`
}

// VmConsoleOperationResponse is the response shape for VM console operations.
type VmConsoleOperationResponse struct {
	Success bool   `json:"success"`
	VMID    string `json:"vmId"`
	Message string `json:"message"`
}

// ActivityEntry is one recorded VM lifecycle/console action for the activity
// feed (GET /api/activity).
type ActivityEntry struct {
	VMID       string    `json:"vmId"`
	Action     string    `json:"action"`
	Success    bool      `json:"success"`
	Message    string    `json:"message,omitempty"`
	RecordedAt time.Time `json:"recordedAt"`
}

// ActivityResponse is the response shape for GET /api/activity.
type ActivityResponse struct {
	Entries []ActivityEntry `json:"entries"`
}

// NetworkInterface describes one virtual NIC and the guest-reported IPv4
// addresses bound to it. IPv4 addresses come from Guest Additions, so they are
// only present when the guest is running with Guest Additions active.
type NetworkInterface struct {
	Slot int      `json:"slot"`
	Mode string   `json:"mode"`
	MAC  string   `json:"mac,omitempty"`
	IPv4 []string `json:"ipv4"`
}

// DiskUsage reports a VM disk's virtual capacity and its actual host-side
// allocation (both in bytes) plus the allocation percentage. These are
// host-side values (no Guest Additions required); they reflect how much of the
// virtual disk is physically allocated on the host, not guest filesystem usage.
type DiskUsage struct {
	Name           string `json:"name"`
	CapacityBytes  int64  `json:"capacityBytes"`
	AllocatedBytes int64  `json:"allocatedBytes"`
	Percent        int    `json:"percent"`
}

// SharedFolder describes one host directory shared into a guest. Transient
// mappings exist only while the VM is running; persistent (machine) mappings
// survive reboots. HostPath is a host filesystem path exposed to the guest, so
// it is only surfaced to the authenticated local UI.
type SharedFolder struct {
	Name      string `json:"name"`
	HostPath  string `json:"hostPath"`
	Transient bool   `json:"transient"`
}

// SharedFoldersResponse is the response shape for
// GET /api/vms/{id}/shared-folders.
type SharedFoldersResponse struct {
	Folders []SharedFolder `json:"folders"`
}

// SharedFolderOperationResponse is the response shape for shared folder add and
// remove operations.
type SharedFolderOperationResponse struct {
	Success bool   `json:"success"`
	VMID    string `json:"vmId"`
	Message string `json:"message"`
}

// ClipboardModeResponse is the response shape for the shared-clipboard mode
// endpoints (GET and POST /api/vms/{id}/clipboard). Mode is one of disabled,
// hosttoguest, guesttohost, bidirectional.
type ClipboardModeResponse struct {
	ID   string `json:"id"`
	Mode string `json:"mode"`
}

// GuestAdditionsStatusResponse is the response shape for
// GET /api/vms/{id}/guest-additions. Status is one of "installed",
// "not-detected", or "unknown" (the VM is not running, so it cannot be probed).
// HostVersion is the host VirtualBox version; UpdateAvailable is true when Guest
// Additions is installed but its version differs from the host (a mismatch that
// breaks features like dynamic resolution), so the UI can offer a one-click
// update via guest control.
type GuestAdditionsStatusResponse struct {
	ID              string `json:"id"`
	Installed       bool   `json:"installed"`
	Version         string `json:"version,omitempty"`
	HostVersion     string `json:"hostVersion,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Status          string `json:"status"`
}

// GuestAdditionsInstallResponse is the response shape for
// POST /api/vms/{id}/guest-additions/install. It reports where the Guest
// Additions disc was inserted.
type GuestAdditionsInstallResponse struct {
	Success    bool   `json:"success"`
	VMID       string `json:"vmId"`
	Controller string `json:"controller,omitempty"`
	Port       int    `json:"port"`
	Device     int    `json:"device"`
	Message    string `json:"message"`
}

// GuestAdditionsUpdateRequest is the body for POST
// /api/vms/{id}/guest-additions/update. The credentials are used once for a
// single VBoxManage guest-control call and never stored.
type GuestAdditionsUpdateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// GuestAdditionsUpdateResponse is the response shape for POST
// /api/vms/{id}/guest-additions/update. Output carries the guest-side installer
// log (stdout/stderr) so the UI can show what happened.
type GuestAdditionsUpdateResponse struct {
	Success bool   `json:"success"`
	VMID    string `json:"vmId"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

// PortForwardingRule is one NAT port-forwarding rule: a host address/port that
// VirtualBox maps to a guest address/port. HostIP and GuestIP are optional; an
// empty HostIP means the rule was created without an explicit bind address.
type PortForwardingRule struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"` // tcp | udp
	HostIP    string `json:"hostIp,omitempty"`
	HostPort  int    `json:"hostPort"`
	GuestIP   string `json:"guestIp,omitempty"`
	GuestPort int    `json:"guestPort"`
}

// NetworkAdapter is one configured, enabled virtual NIC: its slot, attachment
// mode (nat, bridged, hostonly, ...), the host interface it is bound to (for
// bridged/hostonly), its MAC, and (NAT only) any port-forwarding rules.
type NetworkAdapter struct {
	Slot       int                  `json:"slot"`
	Mode       string               `json:"mode"`
	Adapter    string               `json:"adapter,omitempty"`
	MAC        string               `json:"mac,omitempty"`
	Forwarding []PortForwardingRule `json:"forwarding,omitempty"`
}

// NetworkOptionsResponse is the response shape for GET /api/vms/{id}/network. It
// carries the VM's NICs plus the host interfaces available for bridged and
// host-only attachment so the UI can offer a valid choice.
type NetworkOptionsResponse struct {
	Adapters         []NetworkAdapter `json:"adapters"`
	BridgedAdapters  []string         `json:"bridgedAdapters"`
	HostOnlyAdapters []string         `json:"hostOnlyAdapters"`
}

// NetworkModeRequest is the body for POST /api/vms/{id}/network. Adapter is the
// host interface name, required only for bridged and host-only modes.
type NetworkModeRequest struct {
	Slot    int    `json:"slot"`
	Mode    string `json:"mode"`
	Adapter string `json:"adapter"`
}

// NetworkOperationResponse is the response shape for a network mode change or a
// port-forwarding add/remove.
type NetworkOperationResponse struct {
	Success bool   `json:"success"`
	VMID    string `json:"vmId"`
	Message string `json:"message"`
}

// PortForwardingRequest is the body for POST /api/vms/{id}/network/forwarding.
// Slot is the NAT NIC the rule is added to; HostIP and GuestIP are optional.
type PortForwardingRequest struct {
	Slot      int    `json:"slot"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	HostIP    string `json:"hostIp"`
	HostPort  int    `json:"hostPort"`
	GuestIP   string `json:"guestIp"`
	GuestPort int    `json:"guestPort"`
}

// PortForwardingDeleteRequest is the body for POST
// /api/vms/{id}/network/forwarding/delete. It identifies a rule by NIC slot and
// name.
type PortForwardingDeleteRequest struct {
	Slot int    `json:"slot"`
	Name string `json:"name"`
}

// UsbDevice is one USB device present on the host, as reported by
// `VBoxManage list usbhost`. VendorID/ProductID are kept in their hex form
// (e.g. "0x0781"). State is the VirtualBox capture state (Available, Busy,
// Captured, Unavailable). AttachedHere is true when this device is currently
// captured by the VM the response is about.
type UsbDevice struct {
	UUID         string `json:"uuid"`
	VendorID     string `json:"vendorId"`
	ProductID    string `json:"productId"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Product      string `json:"product,omitempty"`
	State        string `json:"state"`
	AttachedHere bool   `json:"attachedHere"`
}

// VmUsbResponse is the response shape for GET /api/vms/{id}/usb. It lists the
// host's USB devices plus the two prerequisites the UI must surface: the Oracle
// Extension Pack (required for USB 2.0/3.0 passthrough) and whether the VM has a
// USB controller enabled (which cannot be toggled while the VM is running).
type VmUsbResponse struct {
	Devices                []UsbDevice `json:"devices"`
	ExtensionPackInstalled bool        `json:"extensionPackInstalled"`
	USBControllerEnabled   bool        `json:"usbControllerEnabled"`
}

// UsbActionRequest is the body for POST /api/vms/{id}/usb/attach and
// /api/vms/{id}/usb/detach. DeviceUUID is the host device UUID from
// VmUsbResponse.Devices.
type UsbActionRequest struct {
	DeviceUUID string `json:"deviceUuid"`
}

// UsbOperationResponse is the response shape for a USB attach or detach.
type UsbOperationResponse struct {
	Success bool   `json:"success"`
	VMID    string `json:"vmId"`
	Message string `json:"message"`
}

// Snapshot is one VirtualBox snapshot in a VM's snapshot tree. Depth is the
// nesting level (0 = a root snapshot), so the UI can indent children. Current is
// true for the snapshot the VM's current state descends from.
type Snapshot struct {
	Name        string `json:"name"`
	UUID        string `json:"uuid"`
	Description string `json:"description,omitempty"`
	Current     bool   `json:"current"`
	Depth       int    `json:"depth"`
}

// SnapshotsResponse is the response shape for GET /api/vms/{id}/snapshots.
type SnapshotsResponse struct {
	Snapshots   []Snapshot `json:"snapshots"`
	CurrentUUID string     `json:"currentUuid,omitempty"`
}

// VmHardwareResponse is the response shape for GET /api/vms/{id}/hardware.
// Host totals let the UI bound its inputs; Editable is false while the VM is
// live because modifyvm only works on a powered-off machine.
type VmHardwareResponse struct {
	ID           string `json:"id"`
	CPUs         int    `json:"cpus"`
	MemoryMB     int    `json:"memoryMb"`
	HostCPUs     int    `json:"hostCpus"`
	HostMemoryMB int    `json:"hostMemoryMb"`
	Editable     bool   `json:"editable"`
}

// VmGuestOSResponse reports a VM's declared guest OS type and a coarse family
// classification ("linux", "windows", "other", or ""), plus whether the
// serial-console terminal can be offered for it.
type VmGuestOSResponse struct {
	ID              string `json:"id"`
	OSType          string `json:"osType"`
	Family          string `json:"family"`
	TerminalCapable bool   `json:"terminalCapable"`
}

// VmSerialConsoleResponse reports the state of a VM's serial-console terminal:
// whether COM1 is wired to the host pipe, whether the guest is terminal-capable
// (Linux), whether the VM is running (the getty is only reachable while live),
// and whether the serial port can be toggled now (only on a powered-off VM).
type VmSerialConsoleResponse struct {
	ID              string `json:"id"`
	Enabled         bool   `json:"enabled"`
	TerminalCapable bool   `json:"terminalCapable"`
	Running         bool   `json:"running"`
	Editable        bool   `json:"editable"`
}

// SerialGettyRequest is the body for POST /api/vms/{id}/serial-console/enable-getty.
// The credentials are used once for a single guest-control call and never stored.
type SerialGettyRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SerialGettyResponse reports the result of enabling the guest's serial login.
// Output carries the guest-side command output (stdout+stderr) for the UI.
type SerialGettyResponse struct {
	Success bool   `json:"success"`
	VMID    string `json:"vmId"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

// VmHardwareRequest is the body for POST /api/vms/{id}/hardware.
type VmHardwareRequest struct {
	CPUs     int `json:"cpus"`
	MemoryMB int `json:"memoryMb"`
}

// DiskInfo is one attached hard disk with the metadata the UI needs to resize
// it. Reason explains why Resizable is false (wrong format, fixed-size,
// snapshots present, or VM running).
type DiskInfo struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Format      string `json:"format"`
	CapacityMB  int64  `json:"capacityMb"`
	AllocatedMB int64  `json:"allocatedMb"`
	Resizable   bool   `json:"resizable"`
	Reason      string `json:"reason,omitempty"`
}

// OpticalDrive describes a VM's DVD/optical drive and the medium currently in
// it, surfaced by GET /api/vms/{id}/storage. Present is false when the VM has no
// optical drive at all. Medium is the absolute ISO path when a disc is inserted,
// or empty when the drive holds no disc; Name is the ISO file's basename for
// display. Controller/Port/Device locate the drive so it can be mounted/ejected.
type OpticalDrive struct {
	Present    bool   `json:"present"`
	Medium     string `json:"medium"`
	Name       string `json:"name"`
	Controller string `json:"controller"`
	Port       int    `json:"port"`
	Device     int    `json:"device"`
}

// VmStorageResponse is the response shape for GET /api/vms/{id}/storage.
// Editable is false while the VM is live, since disks cannot be resized in use.
// Optical describes the DVD drive; unlike disks, its medium can be changed while
// the VM is running, so it is reported regardless of Editable.
type VmStorageResponse struct {
	ID       string       `json:"id"`
	Disks    []DiskInfo   `json:"disks"`
	Optical  OpticalDrive `json:"optical"`
	Editable bool         `json:"editable"`
}

// DvdMountRequest is the body for POST /api/vms/{id}/storage/dvd. IsoPath is the
// absolute host path of the .iso to insert into the VM's DVD drive.
type DvdMountRequest struct {
	IsoPath string `json:"isoPath"`
}

// DiskResizeRequest is the body for POST /api/vms/{id}/storage/resize.
type DiskResizeRequest struct {
	UUID   string `json:"uuid"`
	SizeMB int64  `json:"sizeMb"`
}

// DiskAddRequest is the body for POST /api/vms/{id}/storage/add.
type DiskAddRequest struct {
	SizeMB int64 `json:"sizeMb"`
}

// DiskDetachRequest is the body for POST /api/vms/{id}/storage/detach. When
// DeleteFile is true the disk image is permanently deleted after detaching.
type DiskDetachRequest struct {
	UUID       string `json:"uuid"`
	DeleteFile bool   `json:"deleteFile"`
}

// SnapshotOperationResponse is the response shape for snapshot take/restore/delete.
type SnapshotOperationResponse struct {
	Success bool   `json:"success"`
	VMID    string `json:"vmId"`
	Message string `json:"message"`
}

// SnapshotTakeRequest is the body for POST /api/vms/{id}/snapshots.
type SnapshotTakeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SnapshotRequest is the body for snapshot restore/delete; it identifies the
// target snapshot by UUID.
type SnapshotRequest struct {
	UUID string `json:"uuid"`
}

// VmFileTransferResponse is the response shape for POST /api/vms/{id}/files.
// Method is how the file reached the guest: "shared-folder" (written into an
// existing shared folder's host path — no guest credentials needed) or
// "guest-control" (copied in via VBoxManage guest control). GuestPath is where
// the file landed inside the guest. CredentialsRequired is true when the VM has
// no shared folder and no guest credentials were supplied, so the UI should
// prompt for them and retry.
type VmFileTransferResponse struct {
	Success             bool   `json:"success"`
	VMID                string `json:"vmId"`
	Method              string `json:"method,omitempty"`
	GuestPath           string `json:"guestPath,omitempty"`
	Message             string `json:"message"`
	CredentialsRequired bool   `json:"credentialsRequired"`
}

// VmGuestRunRequest is the body for POST /api/vms/{id}/guest/run. Exe is the
// absolute path of the program to run inside the guest; Args are its arguments.
// The credentials are used once for a single VBoxManage guest-control call and
// never stored.
type VmGuestRunRequest struct {
	Exe      string   `json:"exe"`
	Args     []string `json:"args"`
	Username string   `json:"username"`
	Password string   `json:"password"`
}

// VmGuestRunResponse is the response shape for POST /api/vms/{id}/guest/run.
// ExitCode is the guest process's exit code (a non-zero code is a completed run,
// not a transport failure). Output carries the captured guest output; Truncated
// is true when it was capped. CredentialsRequired is true when no guest
// credentials were supplied, so the UI should prompt for them and retry.
type VmGuestRunResponse struct {
	Success             bool   `json:"success"`
	VMID                string `json:"vmId"`
	ExitCode            int    `json:"exitCode"`
	Output              string `json:"output,omitempty"`
	Truncated           bool   `json:"truncated"`
	Message             string `json:"message"`
	CredentialsRequired bool   `json:"credentialsRequired"`
}

// VmGuestCopyFromRequest is the body for POST /api/vms/{id}/guest/copyfrom.
// GuestPath is the absolute path of the file to copy out of the guest;
// Directory is the host destination folder chosen via the host folder picker.
// The credentials are used once for a single VBoxManage guest-control call and
// never stored.
type VmGuestCopyFromRequest struct {
	GuestPath string `json:"guestPath"`
	Directory string `json:"directory"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

// VmGuestCopyFromResponse is the response shape for POST
// /api/vms/{id}/guest/copyfrom. HostPath is the absolute host path the guest
// file was written to. CredentialsRequired is true when no guest credentials
// were supplied, so the UI should prompt for them and retry.
type VmGuestCopyFromResponse struct {
	Success             bool   `json:"success"`
	VMID                string `json:"vmId"`
	HostPath            string `json:"hostPath,omitempty"`
	Message             string `json:"message"`
	CredentialsRequired bool   `json:"credentialsRequired"`
}

// HostFolderPickResponse is the response shape for POST /api/host/pick-folder.
// Path is the absolute host directory the user selected in the native dialog; it
// is empty and Cancelled is true when the user dismissed the dialog. The path is
// only returned to the authenticated local UI.
type HostFolderPickResponse struct {
	Path      string `json:"path"`
	Cancelled bool   `json:"cancelled"`
}

// HostFilePickResponse is the response shape for POST /api/host/pick-file. It
// mirrors HostFolderPickResponse but returns a file rather than a directory.
type HostFilePickResponse struct {
	Path      string `json:"path"`
	Cancelled bool   `json:"cancelled"`
}

// VmImportRequest is the body for POST /api/vms/import — importing a prebuilt
// appliance (.ova/.ovf) that already ships Guest Additions.
type VmImportRequest struct {
	OvaPath string `json:"ovaPath"`
	Name    string `json:"name"`
}

// VmCreateRequest is the body for POST /api/vms/create — an unattended install
// from an OS ISO with Guest Additions baked in during setup.
type VmCreateRequest struct {
	Name     string `json:"name"`
	OsType   string `json:"osType"`
	IsoPath  string `json:"isoPath"`
	MemoryMB int    `json:"memoryMb"`
	Cpus     int    `json:"cpus"`
	DiskGB   int    `json:"diskGb"`
	Username string `json:"username"`
	Password string `json:"password"`
	Hostname string `json:"hostname"`
}

// VmCreateManualRequest is the body for POST /api/vms/create-manual — creating
// a VM with the installer ISO attached as a DVD, for OSes without an unattended
// template. The user installs interactively via the console; no guest
// credentials are involved.
type VmCreateManualRequest struct {
	Name     string `json:"name"`
	OsType   string `json:"osType"`
	IsoPath  string `json:"isoPath"`
	MemoryMB int    `json:"memoryMb"`
	Cpus     int    `json:"cpus"`
	DiskGB   int    `json:"diskGb"`
}

// VmCloneRequest is the body for POST /api/vms/{id}/clone — cloning a stopped
// source VM. Linked selects a linked clone (which requires the source to have at
// least one snapshot); otherwise a full clone (independent copy of the disks) is
// made. The clone runs as a background job and is polled like a create.
type VmCloneRequest struct {
	Name   string `json:"name"`
	Linked bool   `json:"linked"`
}

// VmExportRequest is the body for POST /api/vms/{id}/export — exporting a
// stopped VM to an .ova appliance. Directory is the destination folder chosen
// via the host folder picker; the agent derives the filename from the VM name
// and writes <directory>/<sanitized-vm-name>.ova. The export runs as a
// background job and is polled like a create.
type VmExportRequest struct {
	Directory string `json:"directory"`
}

// VmCreateResponse is what the service returns once a create/import completes.
type VmCreateResponse struct {
	Success bool   `json:"success"`
	VMID    string `json:"vmId"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// VmCreateJobResponse is returned immediately when a create/import is accepted;
// the long-running work proceeds in the background and is polled by JobID.
type VmCreateJobResponse struct {
	JobID string `json:"jobId"`
}

// VmCreateStatusResponse reports the progress of a background create/import job.
// State is one of "running", "done", or "error".
type VmCreateStatusResponse struct {
	State   string `json:"state"`
	Message string `json:"message"`
	VMID    string `json:"vmId,omitempty"`
	Name    string `json:"name,omitempty"`
}

// VmTelemetryResponse is the response shape for GET /api/vms/{id}/telemetry.
// CPU and RAM are the configured (static) values; network IPv4 addresses are
// guest-reported. GuestAdditions reflects whether the guest was actively
// reporting GuestInfo properties, so the UI can prompt to install Guest
// Additions when addresses are unavailable.
type VmTelemetryResponse struct {
	ID             string             `json:"id"`
	CPUCount       int                `json:"cpuCount"`
	RAMMB          int                `json:"ramMb"`
	GuestAdditions bool               `json:"guestAdditions"`
	Networks       []NetworkInterface `json:"networks"`
	Disks          []DiskUsage        `json:"disks"`
}
