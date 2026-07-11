import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, cleanup, act, fireEvent } from '@testing-library/react';
import { ScreenConsole } from './ScreenConsole';
import { api } from '../api/client';

// The screen stream needs a WebSocket + canvas 2D context that jsdom lacks; the
// stream path short-circuits when getContext('2d') returns null, so these tests
// exercise the independent telemetry panel. Only the client is mocked.
vi.mock('../api/client', () => ({
  screenStreamUrl: () => 'ws://localhost/api/vms/x/screen-stream',
  ApiError: class ApiError extends Error {},
  api: {
    getVmTelemetry: vi.fn(),
    getSharedFolders: vi.fn(),
    addSharedFolder: vi.fn(),
    removeSharedFolder: vi.fn(),
    getClipboardMode: vi.fn(),
    setClipboardMode: vi.fn(),
  },
}));

describe('ScreenConsole telemetry', () => {
  beforeEach(() => {
    // The mock lives in the module factory for the whole file, so its call
    // history must be reset (not just restored) between tests.
    vi.mocked(api.getVmTelemetry).mockReset();
    vi.mocked(api.getSharedFolders).mockReset();
    vi.mocked(api.getSharedFolders).mockResolvedValue({ folders: [] });
    vi.mocked(api.getClipboardMode).mockReset();
    vi.mocked(api.setClipboardMode).mockReset();
    vi.mocked(api.getClipboardMode).mockResolvedValue({ id: 'x', mode: 'disabled' });
    vi.mocked(api.setClipboardMode).mockResolvedValue({ id: 'x', mode: 'bidirectional' });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('shows guest network IPs and configured CPU/RAM', async () => {
    vi.mocked(api.getVmTelemetry).mockResolvedValue({
      id: 'x',
      cpuCount: 4,
      ramMb: 8192,
      guestAdditions: true,
      networks: [{ slot: 1, mode: 'bridged', mac: '0800271122AA', ipv4: ['192.168.1.42'] }],
      disks: [{ name: 'lab.vdi', capacityBytes: 53687091200, allocatedBytes: 13421772800, percent: 25 }],
    });

    const { getByText } = render(<ScreenConsole vmId="x" vmName="lab" onClose={() => {}} />);

    await waitFor(() => expect(getByText(/192\.168\.1\.42/)).toBeTruthy());
    // Telemetry now lives in the collapsible rail: labelled CPU/Memory/Disk metrics.
    expect(getByText('CPU')).toBeTruthy();
    expect(getByText('Memory')).toBeTruthy();
    expect(getByText('8 GB')).toBeTruthy();
    expect(getByText('Disk')).toBeTruthy();
  });

  it('flags Guest Additions as not detected when no IPs are reported', async () => {
    vi.mocked(api.getVmTelemetry).mockResolvedValue({
      id: 'x',
      cpuCount: 2,
      ramMb: 4096,
      guestAdditions: false,
      networks: [{ slot: 1, mode: 'nat', ipv4: [] }],
      disks: [],
    });

    const { getByText } = render(<ScreenConsole vmId="x" vmName="lab" onClose={() => {}} />);

    await waitFor(() => expect(getByText(/Guest Additions not detected/i)).toBeTruthy());
  });

  it('loads the clipboard mode and changes it via the control', async () => {
    vi.mocked(api.getVmTelemetry).mockResolvedValue({
      id: 'x',
      cpuCount: 2,
      ramMb: 4096,
      guestAdditions: true,
      networks: [],
      disks: [],
    });
    vi.mocked(api.getClipboardMode).mockResolvedValue({ id: 'x', mode: 'disabled' });

    const { getByLabelText } = render(<ScreenConsole vmId="x" vmName="lab" onClose={() => {}} />);

    await waitFor(() => expect(api.getClipboardMode).toHaveBeenCalledWith('x'));
    const select = (await waitFor(() => getByLabelText('Shared clipboard mode'))) as HTMLSelectElement;
    expect(select.value).toBe('disabled');

    fireEvent.change(select, { target: { value: 'bidirectional' } });

    await waitFor(() => expect(api.setClipboardMode).toHaveBeenCalledWith('x', 'bidirectional'));
  });

  it('polls so a newly-assigned IP appears without reopening the console', async () => {
    vi.useFakeTimers();
    try {
      const mock = vi.mocked(api.getVmTelemetry);
      mock
        .mockResolvedValueOnce({
          id: 'x',
          cpuCount: 2,
          ramMb: 4096,
          guestAdditions: false,
          networks: [{ slot: 1, mode: 'nat', ipv4: [] }],
          disks: [],
        })
        .mockResolvedValue({
          id: 'x',
          cpuCount: 2,
          ramMb: 4096,
          guestAdditions: true,
          networks: [{ slot: 1, mode: 'nat', ipv4: ['10.0.2.15'] }],
          disks: [],
        });

      const { getByText, queryByText } = render(
        <ScreenConsole vmId="x" vmName="lab" onClose={() => {}} />,
      );

      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      expect(mock).toHaveBeenCalledTimes(1);
      expect(queryByText(/10\.0\.2\.15/)).toBeNull();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(8000);
      });
      expect(mock).toHaveBeenCalledTimes(2);
      expect(getByText(/10\.0\.2\.15/)).toBeTruthy();
    } finally {
      vi.useRealTimers();
    }
  });
});
