export interface HealthStatus {
  status: 'healthy' | 'unhealthy';
  timestamp: string;
  uptimeSeconds?: number;
  version?: string;
}

// Whether a newer TabVM release exists on GitHub. Best-effort and cached by the
// agent; on any failure the agent returns updateAvailable=false with no latest,
// never an error, so the local-first UI is never blocked. latest is normalized
// (no leading "v"); releaseUrl links to the GitHub release download page.
export interface UpdateStatus {
  current: string;
  latest?: string;
  updateAvailable: boolean;
  releaseUrl?: string;
}

// Intentionally omits the resolved VBoxManage path. Host-side path details
// should only be exposed through a future authenticated diagnostics endpoint.
export interface VirtualBoxDiscovery {
  found: boolean;
  version?: string;
  error?: string;
}

export interface VmInfo {
  id: string;
  name: string;
  state: string;
}

export interface VmListResponse {
  vms: VmInfo[];
}

export interface VmOperationResponse {
  success: boolean;
  vmId: string;
  message: string;
}

export interface VmStatusResponse {
  id: string;
  state: string;
}

export interface LocalStateStatusResponse {
  configured: boolean;
  available: boolean;
  schema: number;
}

export interface ActivityEntry {
  vmId: string;
  action: string;
  success: boolean;
  message?: string;
  recordedAt: string;
}

export interface ActivityResponse {
  entries: ActivityEntry[];
}

export interface NetworkInterface {
  slot: number;
  mode: string;
  mac?: string;
  ipv4: string[];
}

export interface DiskUsage {
  name: string;
  capacityBytes: number;
  allocatedBytes: number;
  percent: number;
}

export interface SharedFolder {
  name: string;
  hostPath: string;
  transient: boolean;
}

export interface SharedFoldersResponse {
  folders: SharedFolder[];
}

// Result of the native host folder picker. path is the chosen absolute host
// directory; it is empty and cancelled is true when the user dismissed the
// dialog.
export interface HostFolderPickResponse {
  path: string;
  cancelled: boolean;
}

export interface HostFilePickResponse {
  path: string;
  cancelled: boolean;
}

// Returned immediately when a create/import is accepted; poll by jobId.
export interface VmCreateJobResponse {
  jobId: string;
}

// Progress of a background create/import job. state: 'running' | 'done' | 'error'.
export interface VmCreateStatusResponse {
  state: string;
  message: string;
  vmId?: string;
  name?: string;
}

// Body for an unattended create request.
export interface VmCreateRequest {
  name: string;
  osType: string;
  isoPath: string;
  memoryMb: number;
  cpus: number;
  diskGb: number;
  username: string;
  password: string;
  hostname: string;
}

// Body for a manual-install create request: the VM is created with the ISO
// attached as a DVD and the user installs the OS interactively via the console.
export interface VmCreateManualRequest {
  name: string;
  osType: string;
  isoPath: string;
  memoryMb: number;
  cpus: number;
  diskGb: number;
}

// Body for cloning a stopped VM. linked selects a linked clone (requires the
// source to have a snapshot); otherwise a full, independent copy is made. The
// clone runs as a background job polled with the same create status endpoint.
export interface VmCloneRequest {
  name: string;
  linked: boolean;
}

// Body for exporting a stopped VM to an .ova appliance. directory is the
// destination folder chosen via the host folder picker; the agent derives the
// filename from the VM name. The export runs as a background job polled with the
// same create status endpoint.
export interface VmExportRequest {
  directory: string;
}

// One NAT port-forwarding rule: a host address/port mapped to a guest
// address/port. hostIp and guestIp are optional.
export interface PortForwardingRule {
  name: string;
  protocol: string; // 'tcp' | 'udp'
  hostIp?: string;
  hostPort: number;
  guestIp?: string;
  guestPort: number;
}

// One enabled virtual NIC and how it is attached. forwarding is present only for
// NAT adapters that have port-forwarding rules.
export interface NetworkAdapter {
  slot: number;
  mode: string;
  adapter?: string;
  mac?: string;
  forwarding?: PortForwardingRule[];
}

// Body for adding a NAT port-forwarding rule. hostIp/guestIp are optional; an
// empty hostIp defaults to 127.0.0.1 on the agent.
export interface PortForwardingRequest {
  slot: number;
  name: string;
  protocol: string;
  hostIp?: string;
  hostPort: number;
  guestIp?: string;
  guestPort: number;
}

export interface NetworkOptionsResponse {
  adapters: NetworkAdapter[];
  bridgedAdapters: string[];
  hostOnlyAdapters: string[];
}

export interface NetworkOperationResponse {
  success: boolean;
  vmId: string;
  message: string;
}

export interface VmHardwareResponse {
  id: string;
  cpus: number;
  memoryMb: number;
  hostCpus: number;
  hostMemoryMb: number;
  // false while the VM is live — modifyvm only works on a powered-off machine
  editable: boolean;
}

export interface DiskInfo {
  uuid: string;
  name: string;
  format: string;
  capacityMb: number;
  allocatedMb: number;
  resizable: boolean;
  // when resizable is false, why (wrong format, fixed, snapshots, VM running)
  reason?: string;
}

export interface VmStorageResponse {
  id: string;
  disks: DiskInfo[];
  editable: boolean;
}

// One VirtualBox snapshot. depth is the nesting level (0 = root) for indenting
// the tree; current marks the snapshot the VM's state descends from.
export interface Snapshot {
  name: string;
  uuid: string;
  description?: string;
  current: boolean;
  depth: number;
}

export interface SnapshotsResponse {
  snapshots: Snapshot[];
  currentUuid?: string;
}

export interface SnapshotOperationResponse {
  success: boolean;
  vmId: string;
  message: string;
}

// Result of dropping a file onto a VM. method is how it reached the guest
// ("shared-folder" | "guest-control"); guestPath is where it landed.
// credentialsRequired is true when the VM has no shared folder and no guest
// credentials were supplied, so the UI should prompt for them and retry.
export interface VmFileTransferResponse {
  success: boolean;
  vmId: string;
  method?: 'shared-folder' | 'guest-control';
  guestPath?: string;
  message: string;
  credentialsRequired: boolean;
}

export interface SharedFolderOperationResponse {
  success: boolean;
  vmId: string;
  message: string;
}

export type ClipboardMode = 'disabled' | 'hosttoguest' | 'guesttohost' | 'bidirectional';

export interface ClipboardModeResponse {
  id: string;
  mode: string;
}

export type GuestAdditionsStatus = 'installed' | 'not-detected' | 'unknown';

export interface GuestAdditionsStatusResponse {
  id: string;
  installed: boolean;
  version?: string;
  hostVersion?: string;
  updateAvailable: boolean;
  status: GuestAdditionsStatus;
}

export interface GuestAdditionsInstallResponse {
  success: boolean;
  vmId: string;
  controller?: string;
  port: number;
  device: number;
  message: string;
}

export interface GuestAdditionsUpdateResponse {
  success: boolean;
  vmId: string;
  message: string;
  output?: string;
}

export interface VmTelemetryResponse {
  id: string;
  cpuCount: number;
  ramMb: number;
  // Guest Additions must be active for guest IPv4 addresses to be reported.
  guestAdditions: boolean;
  networks: NetworkInterface[];
  disks: DiskUsage[];
}

// Coarse guest OS classification. terminalCapable is true only for Linux guests,
// which are the only ones that expose a real login TTY over the serial port.
export interface VmGuestOSResponse {
  id: string;
  osType: string;
  family: string;
  terminalCapable: boolean;
}

// State of a VM's serial-console terminal. enabled: COM1 is wired to the host
// pipe. running: the VM is live (the getty is only reachable then). editable:
// the serial port can be toggled now (only on a powered-off VM).
export interface VmSerialConsoleResponse {
  id: string;
  enabled: boolean;
  terminalCapable: boolean;
  running: boolean;
  editable: boolean;
}

export interface SerialGettyResponse {
  success: boolean;
  vmId: string;
  message: string;
  output?: string;
}
