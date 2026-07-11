import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import { MachinesView } from './MachinesView';
import { api, ApiError } from '../api/client';
import { useVmStatus } from '../hooks/useVmStatus';

const mockRefresh = vi.fn();
const RUNNING_ID = '11111111-1111-1111-1111-111111111111';

vi.mock('../hooks/useHealth', () => ({
  useHealth: () => ({
    state: 'success' as const,
    data: { status: 'healthy' as const, timestamp: '2026-01-01T00:00:00Z', uptimeSeconds: 15158 },
  }),
}));

vi.mock('../hooks/useVmStatus', () => ({
  useVmStatus: vi.fn(),
}));

// ScreenConsole opens a WebSocket jsdom cannot provide; stub it.
vi.mock('./ScreenConsole', () => ({
  ScreenConsole: ({ vmName, onClose }: { vmName: string; onClose: () => void }) => (
    <div data-testid="screen-console">
      console:{vmName}
      <button type="button" onClick={onClose}>
        close-console
      </button>
    </div>
  ),
}));

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
      getLocalStateStatus: vi.fn(),
      getVmTelemetry: vi.fn(),
      startVm: vi.fn(),
      stopVm: vi.fn(),
      resetVm: vi.fn(),
      forcePowerOffVm: vi.fn(),
      deleteVm: vi.fn(),
      getGuestAdditionsStatus: vi.fn(),
      installGuestAdditions: vi.fn(),
      getVmGuestOS: vi.fn(),
    },
  };
});

function runningVm() {
  return {
    state: 'success' as const,
    discovery: { found: true, version: '7.0.14r161095' },
    vms: [{ id: RUNNING_ID, name: 'VM One', state: 'running' }],
    refresh: mockRefresh,
  };
}

function stoppedVm() {
  return {
    state: 'success' as const,
    discovery: { found: true, version: '7.0.14r161095' },
    vms: [{ id: RUNNING_ID, name: 'VM One', state: 'powered off' }],
    refresh: mockRefresh,
  };
}

describe('MachinesView', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockRefresh.mockResolvedValue(undefined);
    window.confirm = vi.fn(() => true);
    vi.mocked(useVmStatus).mockReturnValue(runningVm());
    vi.mocked(api.getLocalStateStatus).mockResolvedValue({ configured: true, available: true, schema: 1 });
    vi.mocked(api.getVmGuestOS).mockResolvedValue({
      id: RUNNING_ID,
      osType: 'Ubuntu_64',
      family: 'linux',
      terminalCapable: true,
    });
    vi.mocked(api.getVmTelemetry).mockResolvedValue({
      id: RUNNING_ID,
      cpuCount: 2,
      ramMb: 4096,
      guestAdditions: true,
      networks: [{ slot: 1, mode: 'nat', ipv4: ['10.0.2.15'] }],
      disks: [{ name: 'lab.vdi', capacityBytes: 4096, allocatedBytes: 1024, percent: 25 }],
    });
    // Default: Guest Additions already present, so the install shortcut is hidden.
    vi.mocked(api.getGuestAdditionsStatus).mockResolvedValue({
      id: RUNNING_ID,
      installed: true,
      version: '7.0.14',
      hostVersion: '7.0.14',
      updateAvailable: false,
      status: 'installed',
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    cleanup();
  });

  it('starts a stopped VM and refreshes', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.startVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'started' });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Start VM One' }));

    await waitFor(() => expect(api.startVm).toHaveBeenCalledWith(RUNNING_ID));
    expect(mockRefresh).toHaveBeenCalled();
  });

  it('stops a running VM and refreshes', async () => {
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));

    await waitFor(() => expect(api.stopVm).toHaveBeenCalledWith(RUNNING_ID));
    expect(mockRefresh).toHaveBeenCalled();
  });

  it('confirms before resetting a running VM', async () => {
    vi.mocked(api.resetVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'reset' });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Reset VM One' }));

    expect(window.confirm).toHaveBeenCalled();
    await waitFor(() => expect(api.resetVm).toHaveBeenCalledWith(RUNNING_ID));
  });

  it('does not reset when the confirmation is cancelled', () => {
    window.confirm = vi.fn(() => false);

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Reset VM One' }));

    expect(api.resetVm).not.toHaveBeenCalled();
  });

  it('confirms before deleting a stopped VM and refreshes', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.deleteVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'deleted' });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Delete VM One' }));

    expect(window.confirm).toHaveBeenCalled();
    await waitFor(() => expect(api.deleteVm).toHaveBeenCalledWith(RUNNING_ID));
    expect(mockRefresh).toHaveBeenCalled();
  });

  it('does not delete when the confirmation is cancelled', () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    window.confirm = vi.fn(() => false);

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Delete VM One' }));

    expect(api.deleteVm).not.toHaveBeenCalled();
  });

  it('does not offer delete for a running VM', () => {
    const { queryByRole } = render(<MachinesView />);
    expect(queryByRole('button', { name: 'Delete VM One' })).toBeNull();
  });

  it('shows the sanitized backend error body when an action fails', async () => {
    vi.mocked(api.stopVm).mockRejectedValue(
      new ApiError({ status: 502, statusText: 'Bad Gateway', body: 'VirtualBox operation failed.' }),
    );

    const { getByRole, getByText } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));

    await waitFor(() => expect(getByText('VirtualBox operation failed.')).toBeTruthy());
  });

  it('opens the console full-screen in a new tab', () => {
    const open = vi.spyOn(window, 'open').mockReturnValue(null);

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Open VM One console in a new tab' }));

    expect(open).toHaveBeenCalledTimes(1);
    const url = open.mock.calls[0][0] as string;
    expect(url).toContain(`console=${RUNNING_ID}`);
    expect(url).toContain('name=VM%20One');
    expect(open.mock.calls[0][1]).toBe('_blank');

    open.mockRestore();
  });

  it('opens the live console for a running VM', async () => {
    const { getByRole, getByTestId, queryByTestId } = render(<MachinesView />);

    expect(queryByTestId('screen-console')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Open console for VM One' }));
    expect(getByTestId('screen-console').textContent).toContain('VM One');
  });

  it('renders the agent meta line with uptime and vboxmanage version', () => {
    const { getByText } = render(<MachinesView />);
    expect(getByText('04:12:38')).toBeTruthy();
    expect(getByText('7.0.14r161095')).toBeTruthy();
  });

  it('shows local state schema once the status resolves', async () => {
    const { getByText } = render(<MachinesView />);
    await waitFor(() => expect(api.getLocalStateStatus).toHaveBeenCalled());
    await waitFor(() => expect(getByText('schema 1')).toBeTruthy());
  });

  it('hides the Guest Additions shortcut when additions are installed', async () => {
    const { queryByRole } = render(<MachinesView />);
    await waitFor(() => expect(api.getGuestAdditionsStatus).toHaveBeenCalledWith(RUNNING_ID));
    expect(queryByRole('button', { name: 'Install Guest Additions on VM One' })).toBeNull();
  });

  it('does not offer force power off before the grace period', async () => {
    vi.useFakeTimers();
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });

    const { getByRole, queryByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(api.stopVm).toHaveBeenCalledWith(RUNNING_ID);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(9000);
    });
    expect(queryByRole('button', { name: 'Force power off VM One' })).toBeNull();
  });

  it('offers force power off when the VM is still running after the grace period', async () => {
    vi.useFakeTimers();
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });

    expect(getByRole('button', { name: 'Force power off VM One' })).toBeTruthy();
  });

  it('confirms and force powers off through the API', async () => {
    vi.useFakeTimers();
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });
    vi.mocked(api.forcePowerOffVm).mockResolvedValue({
      success: true,
      vmId: RUNNING_ID,
      message: 'powered off',
    });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });

    fireEvent.click(getByRole('button', { name: 'Force power off VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(window.confirm).toHaveBeenCalled();
    expect(api.forcePowerOffVm).toHaveBeenCalledWith(RUNNING_ID);
  });

  it('keeps offering force power off when the API call fails', async () => {
    vi.useFakeTimers();
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });
    vi.mocked(api.forcePowerOffVm).mockRejectedValue(new Error('force power off failed'));

    const { getByRole, getByText } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });

    fireEvent.click(getByRole('button', { name: 'Force power off VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(api.forcePowerOffVm).toHaveBeenCalledWith(RUNNING_ID);
    expect(getByText('force power off failed')).toBeTruthy();
    expect(getByRole('button', { name: 'Force power off VM One' })).toBeTruthy();
  });

  it('does not force power off when the confirmation is cancelled', async () => {
    vi.useFakeTimers();
    window.confirm = vi.fn(() => false);
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });

    fireEvent.click(getByRole('button', { name: 'Force power off VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(api.forcePowerOffVm).not.toHaveBeenCalled();
  });

  it('never offers force power off when the VM stops on its own', async () => {
    vi.useFakeTimers();
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });

    const { getByRole, queryByRole, rerender } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    // The ACPI signal worked: the next poll reports the VM as powered off.
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    rerender(<MachinesView />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });

    expect(queryByRole('button', { name: 'Force power off VM One' })).toBeNull();
  });

  it('offers and triggers Guest Additions install when not detected', async () => {
    vi.mocked(api.getGuestAdditionsStatus).mockResolvedValue({
      id: RUNNING_ID,
      installed: false,
      updateAvailable: false,
      status: 'not-detected',
    });
    vi.mocked(api.installGuestAdditions).mockResolvedValue({
      success: true,
      vmId: RUNNING_ID,
      controller: 'IDE',
      port: 1,
      device: 0,
      message: 'Guest Additions disc inserted.',
    });

    const { getByRole, getByText } = render(<MachinesView />);

    const button = await waitFor(() =>
      getByRole('button', { name: 'Install Guest Additions on VM One' }),
    );
    fireEvent.click(button);

    await waitFor(() => expect(api.installGuestAdditions).toHaveBeenCalledWith(RUNNING_ID));
    await waitFor(() => expect(getByText(/disc inserted/i)).toBeTruthy());
  });
});
