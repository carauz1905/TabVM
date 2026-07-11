import type {
  ActivityEntry,
  ActivityResponse,
  ClipboardModeResponse,
  DiskUsage,
  GuestAdditionsInstallResponse,
  GuestAdditionsStatusResponse,
  GuestAdditionsUpdateResponse,
  HealthStatus,
  HostFolderPickResponse,
  HostFilePickResponse,
  VmCreateJobResponse,
  VmCreateStatusResponse,
  VmCreateRequest,
  LocalStateStatusResponse,
  NetworkInterface,
  SharedFolder,
  SharedFoldersResponse,
  SharedFolderOperationResponse,
  NetworkAdapter,
  NetworkOptionsResponse,
  NetworkOperationResponse,
  VmHardwareResponse,
  VmStorageResponse,
  DiskInfo,
  Snapshot,
  SnapshotsResponse,
  SnapshotOperationResponse,
  VmFileTransferResponse,
  VirtualBoxDiscovery,
  VmInfo,
  VmListResponse,
  VmOperationResponse,
  VmStatusResponse,
  VmTelemetryResponse,
} from '../types/api';

const API_BASE = '';

export interface ApiErrorDetails {
  status: number;
  statusText: string;
  body: string;
}

export class ApiError extends Error {
  status: number;
  statusText: string;
  body: string;

  constructor({ status, statusText, body }: ApiErrorDetails) {
    super(`Request failed: ${status} ${statusText}`);
    this.status = status;
    this.statusText = statusText;
    this.body = body;
  }
}

// In production the agent serves index.html with the token injected as a window
// global, so a freshly opened tab authenticates with the machine's own token. In
// development (Vite) that global is absent and the token comes from the build.
function resolveInitialToken(): string | undefined {
  if (typeof window !== 'undefined') {
    const injected = (window as unknown as { __TABVM_SESSION_TOKEN__?: unknown }).__TABVM_SESSION_TOKEN__;
    if (typeof injected === 'string' && injected !== '') {
      return injected;
    }
  }
  if (typeof import.meta !== 'undefined' && import.meta.env) {
    return (import.meta.env.VITE_TABVM_SESSION_TOKEN as string | undefined) || undefined;
  }
  return undefined;
}

let sessionToken: string | undefined = resolveInitialToken();

export function configureSessionToken(token: string | undefined): void {
  sessionToken = token;
}

export function getSessionToken(): string | undefined {
  return sessionToken;
}

// screenStreamUrl builds the WebSocket URL for a VM's live COM screen stream.
// It targets the current origin (the agent in production, or the Vite dev
// proxy in development) and carries the session token as a query parameter,
// because the browser WebSocket API cannot set the X-TabVM-Session-Token
// header.
export function screenStreamUrl(id: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const query = sessionToken ? `?token=${encodeURIComponent(sessionToken)}` : '';
  return `${proto}://${window.location.host}/api/vms/${encodeURIComponent(id)}/screen-stream${query}`;
}

function hasString(value: unknown, key: string): boolean {
  return typeof value === 'object' && value !== null && key in value && typeof (value as Record<string, unknown>)[key] === 'string';
}

function hasBoolean(value: unknown, key: string): boolean {
  return typeof value === 'object' && value !== null && key in value && typeof (value as Record<string, unknown>)[key] === 'boolean';
}

function hasNumber(value: unknown, key: string): boolean {
  return typeof value === 'object' && value !== null && key in value && typeof (value as Record<string, unknown>)[key] === 'number';
}

function isHealthStatus(value: unknown): value is HealthStatus {
  return hasString(value, 'status') && hasString(value, 'timestamp');
}

function isVirtualBoxDiscovery(value: unknown): value is VirtualBoxDiscovery {
  if (!hasBoolean(value, 'found')) {
    return false;
  }

  const record = value as Record<string, unknown>;
  const optionalStringKeys = ['version', 'error'];
  return optionalStringKeys.every(
    (key) => !(key in record) || record[key] === null || typeof record[key] === 'string',
  );
}

function isVmInfo(value: unknown): value is VmInfo {
  return (
    typeof value === 'object' &&
    value !== null &&
    hasString(value, 'id') &&
    hasString(value, 'name') &&
    hasString(value, 'state')
  );
}

function isVmListResponse(value: unknown): value is VmListResponse {
  if (typeof value !== 'object' || value === null || !('vms' in value)) {
    return false;
  }

  const vms = (value as Record<string, unknown>).vms;
  return Array.isArray(vms) && vms.every(isVmInfo);
}

function isVmOperationResponse(value: unknown): value is VmOperationResponse {
  return (
    typeof value === 'object' &&
    value !== null &&
    hasBoolean(value, 'success') &&
    hasString(value, 'vmId') &&
    hasString(value, 'message')
  );
}

function isVmStatusResponse(value: unknown): value is VmStatusResponse {
  return (
    typeof value === 'object' &&
    value !== null &&
    hasString(value, 'id') &&
    hasString(value, 'state')
  );
}

function isLocalStateStatusResponse(value: unknown): value is LocalStateStatusResponse {
  return (
    typeof value === 'object' &&
    value !== null &&
    hasBoolean(value, 'configured') &&
    hasBoolean(value, 'available') &&
    typeof (value as Record<string, unknown>).schema === 'number'
  );
}

function isActivityEntry(value: unknown): value is ActivityEntry {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return (
    typeof record.vmId === 'string' &&
    typeof record.action === 'string' &&
    typeof record.success === 'boolean' &&
    typeof record.recordedAt === 'string'
  );
}

function isActivityResponse(value: unknown): value is ActivityResponse {
  if (typeof value !== 'object' || value === null || !('entries' in value)) {
    return false;
  }
  const entries = (value as Record<string, unknown>).entries;
  return Array.isArray(entries) && entries.every(isActivityEntry);
}

function isNetworkInterface(value: unknown): value is NetworkInterface {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return (
    typeof record.slot === 'number' &&
    typeof record.mode === 'string' &&
    Array.isArray(record.ipv4) &&
    record.ipv4.every((ip) => typeof ip === 'string')
  );
}

function isDiskUsage(value: unknown): value is DiskUsage {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return (
    typeof record.name === 'string' &&
    typeof record.capacityBytes === 'number' &&
    typeof record.allocatedBytes === 'number' &&
    typeof record.percent === 'number'
  );
}

function isSharedFolder(value: unknown): value is SharedFolder {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return (
    typeof record.name === 'string' &&
    typeof record.hostPath === 'string' &&
    typeof record.transient === 'boolean'
  );
}

function isSharedFoldersResponse(value: unknown): value is SharedFoldersResponse {
  if (typeof value !== 'object' || value === null || !('folders' in value)) {
    return false;
  }
  const folders = (value as Record<string, unknown>).folders;
  return Array.isArray(folders) && folders.every(isSharedFolder);
}

function isSharedFolderOperationResponse(
  value: unknown,
): value is SharedFolderOperationResponse {
  return (
    typeof value === 'object' &&
    value !== null &&
    hasBoolean(value, 'success') &&
    hasString(value, 'vmId') &&
    hasString(value, 'message')
  );
}

function isHostFolderPickResponse(value: unknown): value is HostFolderPickResponse {
  return hasString(value, 'path') && hasBoolean(value, 'cancelled');
}

function isHostFilePickResponse(value: unknown): value is HostFilePickResponse {
  return hasString(value, 'path') && hasBoolean(value, 'cancelled');
}

function isVmCreateJobResponse(value: unknown): value is VmCreateJobResponse {
  return hasString(value, 'jobId');
}

function isVmCreateStatusResponse(value: unknown): value is VmCreateStatusResponse {
  return hasString(value, 'state') && hasString(value, 'message');
}

function isSnapshot(value: unknown): value is Snapshot {
  if (typeof value !== 'object' || value === null) return false;
  const r = value as Record<string, unknown>;
  return (
    typeof r.name === 'string' &&
    typeof r.uuid === 'string' &&
    typeof r.current === 'boolean' &&
    typeof r.depth === 'number'
  );
}

function isSnapshotsResponse(value: unknown): value is SnapshotsResponse {
  if (typeof value !== 'object' || value === null || !('snapshots' in value)) return false;
  const snaps = (value as Record<string, unknown>).snapshots;
  return Array.isArray(snaps) && snaps.every(isSnapshot);
}

function isSnapshotOperationResponse(value: unknown): value is SnapshotOperationResponse {
  return hasBoolean(value, 'success') && hasString(value, 'vmId') && hasString(value, 'message');
}

function isNetworkAdapter(value: unknown): value is NetworkAdapter {
  if (typeof value !== 'object' || value === null) return false;
  const r = value as Record<string, unknown>;
  return typeof r.slot === 'number' && typeof r.mode === 'string';
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((v) => typeof v === 'string');
}

function isNetworkOptionsResponse(value: unknown): value is NetworkOptionsResponse {
  if (typeof value !== 'object' || value === null) return false;
  const r = value as Record<string, unknown>;
  return (
    Array.isArray(r.adapters) &&
    r.adapters.every(isNetworkAdapter) &&
    isStringArray(r.bridgedAdapters) &&
    isStringArray(r.hostOnlyAdapters)
  );
}

function isNetworkOperationResponse(value: unknown): value is NetworkOperationResponse {
  return hasBoolean(value, 'success') && hasString(value, 'vmId') && hasString(value, 'message');
}

function isVmHardwareResponse(value: unknown): value is VmHardwareResponse {
  return (
    hasString(value, 'id') &&
    hasNumber(value, 'cpus') &&
    hasNumber(value, 'memoryMb') &&
    hasNumber(value, 'hostCpus') &&
    hasNumber(value, 'hostMemoryMb') &&
    hasBoolean(value, 'editable')
  );
}

function isDiskInfo(value: unknown): value is DiskInfo {
  return (
    hasString(value, 'uuid') &&
    hasString(value, 'name') &&
    hasString(value, 'format') &&
    hasNumber(value, 'capacityMb') &&
    hasNumber(value, 'allocatedMb') &&
    hasBoolean(value, 'resizable')
  );
}

function isVmStorageResponse(value: unknown): value is VmStorageResponse {
  if (typeof value !== 'object' || value === null) return false;
  const r = value as Record<string, unknown>;
  return hasString(value, 'id') && hasBoolean(value, 'editable') && Array.isArray(r.disks) && r.disks.every(isDiskInfo);
}

function isVmFileTransferResponse(value: unknown): value is VmFileTransferResponse {
  return (
    hasBoolean(value, 'success') &&
    hasString(value, 'vmId') &&
    hasString(value, 'message') &&
    hasBoolean(value, 'credentialsRequired')
  );
}

// requestUpload posts a multipart form (a dropped file plus optional guest
// credentials). It cannot use request(): the browser must set the multipart
// Content-Type with its boundary, and the body is FormData, not JSON.
async function requestUpload(path: string, form: FormData): Promise<VmFileTransferResponse> {
  const headers = new Headers();
  if (sessionToken) headers.set('X-TabVM-Session-Token', sessionToken);

  const response = await fetch(`${API_BASE}${path}`, { method: 'POST', headers, body: form });
  const bodyText = await response.text();
  if (!response.ok) {
    throw new ApiError({ status: response.status, statusText: response.statusText, body: bodyText });
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(bodyText);
  } catch {
    throw new ApiError({ status: response.status, statusText: response.statusText, body: bodyText });
  }
  if (!isVmFileTransferResponse(parsed)) {
    throw new ApiError({ status: response.status, statusText: response.statusText, body: bodyText });
  }
  return parsed;
}

function isClipboardModeResponse(value: unknown): value is ClipboardModeResponse {
  return hasString(value, 'id') && hasString(value, 'mode');
}

function isGuestAdditionsStatusResponse(
  value: unknown,
): value is GuestAdditionsStatusResponse {
  return hasString(value, 'id') && hasBoolean(value, 'installed') && hasString(value, 'status');
}

function isGuestAdditionsInstallResponse(
  value: unknown,
): value is GuestAdditionsInstallResponse {
  return hasBoolean(value, 'success') && hasString(value, 'vmId') && hasString(value, 'message');
}

function isGuestAdditionsUpdateResponse(
  value: unknown,
): value is GuestAdditionsUpdateResponse {
  return hasBoolean(value, 'success') && hasString(value, 'vmId') && hasString(value, 'message');
}

function isVmTelemetryResponse(value: unknown): value is VmTelemetryResponse {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return (
    hasString(value, 'id') &&
    typeof record.cpuCount === 'number' &&
    typeof record.ramMb === 'number' &&
    typeof record.guestAdditions === 'boolean' &&
    Array.isArray(record.networks) &&
    record.networks.every(isNetworkInterface) &&
    Array.isArray(record.disks) &&
    record.disks.every(isDiskUsage)
  );
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'DELETE';
  auth?: boolean;
  body?: unknown;
}

async function request<T>(
  path: string,
  validate: (value: unknown) => value is T,
  options: RequestOptions = {},
): Promise<T> {
  const headers = new Headers();
  // Only authenticated /api/* routes (or explicitly flagged requests) receive
  // the session token. /health remains unauthenticated.
  if (sessionToken && (path.startsWith('/api/') || options.auth)) {
    headers.set('X-TabVM-Session-Token', sessionToken);
  }

  let bodyInit: string | undefined;
  if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json');
    bodyInit = JSON.stringify(options.body);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    method: options.method ?? 'GET',
    headers,
    body: bodyInit,
  });
  const bodyText = await response.text();

  if (!response.ok) {
    throw new ApiError({
      status: response.status,
      statusText: response.statusText,
      body: bodyText,
    });
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(bodyText);
  } catch {
    throw new ApiError({
      status: response.status,
      statusText: response.statusText,
      body: bodyText,
    });
  }

  if (!validate(parsed)) {
    throw new ApiError({
      status: response.status,
      statusText: response.statusText,
      body: bodyText,
    });
  }

  return parsed;
}

export const api = {
  getHealth: () => request<HealthStatus>('/health', isHealthStatus),
  getVirtualBoxDiscovery: () =>
    request<VirtualBoxDiscovery>('/api/vbox/discovery', isVirtualBoxDiscovery),
  getVms: () => request<VmListResponse>('/api/vms', isVmListResponse),
  getVmStatus: (id: string) =>
    request<VmStatusResponse>(`/api/vms/${encodeURIComponent(id)}/status`, isVmStatusResponse),
  startVm: (id: string) =>
    request<VmOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/start`,
      isVmOperationResponse,
      { method: 'POST' },
    ),
  stopVm: (id: string) =>
    request<VmOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/stop`,
      isVmOperationResponse,
      { method: 'POST' },
    ),
  resetVm: (id: string) =>
    request<VmOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/reset`,
      isVmOperationResponse,
      { method: 'POST' },
    ),
  // deleteVm unregisters the VM and removes its disks and configuration files.
  // Irreversible; the agent refuses to delete a running VM.
  deleteVm: (id: string) =>
    request<VmOperationResponse>(`/api/vms/${encodeURIComponent(id)}`, isVmOperationResponse, {
      method: 'DELETE',
    }),
  getLocalStateStatus: () =>
    request<LocalStateStatusResponse>('/api/local-state/status', isLocalStateStatusResponse),
  getActivity: () => request<ActivityResponse>('/api/activity', isActivityResponse),
  getVmTelemetry: (id: string) =>
    request<VmTelemetryResponse>(
      `/api/vms/${encodeURIComponent(id)}/telemetry`,
      isVmTelemetryResponse,
    ),
  getSharedFolders: (id: string) =>
    request<SharedFoldersResponse>(
      `/api/vms/${encodeURIComponent(id)}/shared-folders`,
      isSharedFoldersResponse,
    ),
  addSharedFolder: (id: string, name: string, hostPath: string) =>
    request<SharedFolderOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/shared-folders`,
      isSharedFolderOperationResponse,
      { method: 'POST', body: { name, hostPath } },
    ),
  removeSharedFolder: (id: string, name: string) =>
    request<SharedFolderOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/shared-folders/remove`,
      isSharedFolderOperationResponse,
      { method: 'POST', body: { name } },
    ),
  // pickHostFolder opens the native OS folder dialog on the host and returns the
  // chosen absolute path (a browser cannot read absolute host paths itself).
  pickHostFolder: () =>
    request<HostFolderPickResponse>('/api/host/pick-folder', isHostFolderPickResponse, {
      method: 'POST',
    }),
  // pickHostFile opens the native OS file dialog (for .ova/.iso selection).
  pickHostFile: () =>
    request<HostFilePickResponse>('/api/host/pick-file', isHostFilePickResponse, {
      method: 'POST',
    }),
  // importVm starts a background appliance (.ova) import; poll getCreateStatus.
  importVm: (ovaPath: string, name: string) =>
    request<VmCreateJobResponse>('/api/vms/import', isVmCreateJobResponse, {
      method: 'POST',
      body: { ovaPath, name },
    }),
  // createVm starts a background unattended install (Ubuntu/Debian) with Guest
  // Additions baked in; poll getCreateStatus.
  createVm: (req: VmCreateRequest) =>
    request<VmCreateJobResponse>('/api/vms/create', isVmCreateJobResponse, {
      method: 'POST',
      body: req as unknown as Record<string, unknown>,
    }),
  getCreateStatus: (jobId: string) =>
    request<VmCreateStatusResponse>(
      `/api/vms/create/status?job=${encodeURIComponent(jobId)}`,
      isVmCreateStatusResponse,
    ),
  getNetworkOptions: (id: string) =>
    request<NetworkOptionsResponse>(
      `/api/vms/${encodeURIComponent(id)}/network`,
      isNetworkOptionsResponse,
    ),
  changeNetworkMode: (id: string, slot: number, mode: string, adapter: string) =>
    request<NetworkOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/network`,
      isNetworkOperationResponse,
      { method: 'POST', body: { slot, mode, adapter } },
    ),
  getVmHardware: (id: string) =>
    request<VmHardwareResponse>(
      `/api/vms/${encodeURIComponent(id)}/hardware`,
      isVmHardwareResponse,
    ),
  setVmHardware: (id: string, cpus: number, memoryMb: number) =>
    request<VmOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/hardware`,
      isVmOperationResponse,
      { method: 'POST', body: { cpus, memoryMb } },
    ),
  getVmStorage: (id: string) =>
    request<VmStorageResponse>(`/api/vms/${encodeURIComponent(id)}/storage`, isVmStorageResponse),
  resizeDisk: (id: string, uuid: string, sizeMb: number) =>
    request<VmOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/storage/resize`,
      isVmOperationResponse,
      { method: 'POST', body: { uuid, sizeMb } },
    ),
  addDisk: (id: string, sizeMb: number) =>
    request<VmOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/storage/add`,
      isVmOperationResponse,
      { method: 'POST', body: { sizeMb } },
    ),
  detachDisk: (id: string, uuid: string, deleteFile: boolean) =>
    request<VmOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/storage/detach`,
      isVmOperationResponse,
      { method: 'POST', body: { uuid, deleteFile } },
    ),
  getSnapshots: (id: string) =>
    request<SnapshotsResponse>(`/api/vms/${encodeURIComponent(id)}/snapshots`, isSnapshotsResponse),
  takeSnapshot: (id: string, name: string, description: string) =>
    request<SnapshotOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/snapshots`,
      isSnapshotOperationResponse,
      { method: 'POST', body: { name, description } },
    ),
  restoreSnapshot: (id: string, uuid: string) =>
    request<SnapshotOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/snapshots/restore`,
      isSnapshotOperationResponse,
      { method: 'POST', body: { uuid } },
    ),
  deleteSnapshot: (id: string, uuid: string) =>
    request<SnapshotOperationResponse>(
      `/api/vms/${encodeURIComponent(id)}/snapshots/delete`,
      isSnapshotOperationResponse,
      { method: 'POST', body: { uuid } },
    ),
  // transferFileToGuest uploads a dropped file to a VM. Guest credentials are
  // only needed (and only sent) for the guest-control fallback when the VM has
  // no shared folder.
  transferFileToGuest: (id: string, file: File, creds?: { username: string; password: string }) => {
    const form = new FormData();
    form.append('file', file);
    if (creds) {
      form.append('username', creds.username);
      form.append('password', creds.password);
    }
    return requestUpload(`/api/vms/${encodeURIComponent(id)}/files`, form);
  },
  getClipboardMode: (id: string) =>
    request<ClipboardModeResponse>(
      `/api/vms/${encodeURIComponent(id)}/clipboard`,
      isClipboardModeResponse,
    ),
  setClipboardMode: (id: string, mode: string) =>
    request<ClipboardModeResponse>(
      `/api/vms/${encodeURIComponent(id)}/clipboard`,
      isClipboardModeResponse,
      { method: 'POST', body: { mode } },
    ),
  getGuestAdditionsStatus: (id: string) =>
    request<GuestAdditionsStatusResponse>(
      `/api/vms/${encodeURIComponent(id)}/guest-additions`,
      isGuestAdditionsStatusResponse,
    ),
  installGuestAdditions: (id: string) =>
    request<GuestAdditionsInstallResponse>(
      `/api/vms/${encodeURIComponent(id)}/guest-additions/install`,
      isGuestAdditionsInstallResponse,
      { method: 'POST' },
    ),
  updateGuestAdditions: (id: string, username: string, password: string) =>
    request<GuestAdditionsUpdateResponse>(
      `/api/vms/${encodeURIComponent(id)}/guest-additions/update`,
      isGuestAdditionsUpdateResponse,
      { method: 'POST', body: { username, password } },
    ),
};
