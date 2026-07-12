import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { StoragePanel } from './StoragePanel';
import { api } from '../api/client';

const VM_ID = '11111111-1111-1111-1111-111111111111';
const DISK_UUID = 'ca9ba73f-d0d3-4184-86f1-7206a952bc10';

vi.mock('../api/client', () => {
  class ApiError extends Error {
    status: number;
    statusText: string;
    body: string;
    constructor({ status, statusText, body }: { status: number; statusText: string; body: string }) {
      super(`Request failed: ${status} ${statusText}`);
      this.status = status;
      this.statusText = statusText;
      this.body = body;
    }
  }
  return {
    ApiError,
    api: {
      getVmStorage: vi.fn(),
      resizeDisk: vi.fn(),
      addDisk: vi.fn(),
      detachDisk: vi.fn(),
    },
  };
});

function resizableDisk(diskOverrides = {}, responseOverrides = {}) {
  return {
    id: VM_ID,
    editable: true,
    disks: [
      {
        uuid: DISK_UUID,
        name: 'disk1.vdi',
        format: 'VDI',
        capacityMb: 10240,
        allocatedMb: 2,
        resizable: true,
        reason: '',
        ...diskOverrides,
      },
    ],
    ...responseOverrides,
  };
}

describe('StoragePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getVmStorage).mockResolvedValue(resizableDisk());
    window.confirm = vi.fn(() => true);
  });

  afterEach(() => cleanup());

  it('lists a disk with its format and current capacity', async () => {
    const { findByText } = render(<StoragePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmStorage).toHaveBeenCalledWith(VM_ID));
    // findByText waits for the render that commits after getVmStorage resolves;
    // getByText would race the promise flush and flake on slower CI machines.
    expect(await findByText('disk1.vdi')).toBeTruthy();
    expect(await findByText(/VDI · 10 GB/)).toBeTruthy();
  });

  it('grows a disk and notifies the parent', async () => {
    vi.mocked(api.resizeDisk).mockResolvedValue({ success: true, vmId: VM_ID, message: 'Disk resized.' });
    const onChanged = vi.fn();

    const { getByLabelText, getByRole } = render(<StoragePanel vmId={VM_ID} onChanged={onChanged} />);
    await waitFor(() => expect(api.getVmStorage).toHaveBeenCalled());

    fireEvent.change(getByLabelText('New size (GB)'), { target: { value: '20' } });
    fireEvent.click(getByRole('button', { name: 'Resize' }));

    // 20 GB -> 20480 MB
    await waitFor(() => expect(api.resizeDisk).toHaveBeenCalledWith(VM_ID, DISK_UUID, 20480));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('keeps Resize disabled until the size grows beyond current capacity', async () => {
    const { getByLabelText, getByRole } = render(<StoragePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmStorage).toHaveBeenCalled());

    // Current is 10 GB; entering 10 (or less) must not enable Resize.
    fireEvent.change(getByLabelText('New size (GB)'), { target: { value: '10' } });
    expect((getByRole('button', { name: 'Resize' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('adds a new disk and notifies the parent', async () => {
    vi.mocked(api.addDisk).mockResolvedValue({ success: true, vmId: VM_ID, message: 'Added a 5120 MB disk.' });
    const onChanged = vi.fn();

    const { getByLabelText, getByRole } = render(<StoragePanel vmId={VM_ID} onChanged={onChanged} />);
    await waitFor(() => expect(api.getVmStorage).toHaveBeenCalled());

    fireEvent.change(getByLabelText('New disk size (GB)'), { target: { value: '5' } });
    fireEvent.click(getByRole('button', { name: 'Add disk' }));

    // 5 GB -> 5120 MB
    await waitFor(() => expect(api.addDisk).toHaveBeenCalledWith(VM_ID, 5120));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('hides the add-disk control while the VM is running', async () => {
    vi.mocked(api.getVmStorage).mockResolvedValue(resizableDisk({}, { editable: false }));

    const { queryByLabelText } = render(<StoragePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmStorage).toHaveBeenCalled());

    expect(queryByLabelText('New disk size (GB)')).toBeNull();
  });

  it('detaches a disk (keeping the file) after confirmation', async () => {
    vi.mocked(api.detachDisk).mockResolvedValue({ success: true, vmId: VM_ID, message: 'Disk detached.' });
    const onChanged = vi.fn();

    const { getByRole } = render(<StoragePanel vmId={VM_ID} onChanged={onChanged} />);
    await waitFor(() => expect(api.getVmStorage).toHaveBeenCalled());

    fireEvent.click(getByRole('button', { name: 'Detach disk1.vdi' }));

    expect(window.confirm).toHaveBeenCalled();
    await waitFor(() => expect(api.detachDisk).toHaveBeenCalledWith(VM_ID, DISK_UUID, false));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('deletes a disk file after confirmation', async () => {
    vi.mocked(api.detachDisk).mockResolvedValue({ success: true, vmId: VM_ID, message: 'Disk deleted.' });

    const { getByRole } = render(<StoragePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmStorage).toHaveBeenCalled());

    fireEvent.click(getByRole('button', { name: 'Delete disk1.vdi' }));

    expect(window.confirm).toHaveBeenCalled();
    await waitFor(() => expect(api.detachDisk).toHaveBeenCalledWith(VM_ID, DISK_UUID, true));
  });

  it('does not detach when the confirmation is cancelled', async () => {
    window.confirm = vi.fn(() => false);

    const { getByRole } = render(<StoragePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmStorage).toHaveBeenCalled());

    fireEvent.click(getByRole('button', { name: 'Detach disk1.vdi' }));
    expect(api.detachDisk).not.toHaveBeenCalled();
  });

  it('shows the reason and no input when a disk is not resizable', async () => {
    vi.mocked(api.getVmStorage).mockResolvedValue(
      resizableDisk({ resizable: false, reason: 'This disk has snapshots. Delete them before resizing.' }),
    );

    const { getByText, queryByLabelText } = render(<StoragePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmStorage).toHaveBeenCalled());

    expect(getByText('This disk has snapshots. Delete them before resizing.')).toBeTruthy();
    expect(queryByLabelText('New size (GB)')).toBeNull();
  });
});
