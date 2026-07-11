export interface HealthStatus {
  status: 'healthy' | 'unhealthy';
  timestamp: string;
  uptimeSeconds?: number;
  version?: string;
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

// One enabled virtual NIC and how it is attached.
export interface NetworkAdapter {
  slot: number;
  mode: string;
  adapter?: string;
  mac?: string;
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
