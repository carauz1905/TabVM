import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import { MachinesView } from './MachinesView';
import { api, ApiError } from '../api/client';
import { useVmStatus } from '../hooks/useVmStatus';

const mockRefresh = vi.fn();
const RUNNING_ID = '11111111-1111-1111-1111-111111111111';
const SECOND_ID = '22222222-2222-2222-2222-222222222222';

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
      saveState: vi.fn(),
      resetVm: vi.fn(),
      forcePowerOffVm: vi.fn(),
      deleteVm: vi.fn(),
      getGuestAdditionsStatus: vi.fn(),
      installGuestAdditions: vi.fn(),
      getVmGuestOS: vi.fn(),
      getVmHardware: vi.fn(),
      cloneVm: vi.fn(),
      exportVm: vi.fn(),
      pickHostFolder: vi.fn(),
      getCreateStatus: vi.fn(),
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

function twoStoppedVms() {
  return {
    state: 'success' as const,
    discovery: { found: true, version: '7.0.14r161095' },
    vms: [
      { id: RUNNING_ID, name: 'VM One', state: 'powered off' },
      { id: SECOND_ID, name: 'VM Two', state: 'powered off' },
    ],
    refresh: mockRefresh,
  };
}

function stoppedThenRunning() {
  return {
    ...twoStoppedVms(),
    vms: [
      { id: RUNNING_ID, name: 'VM One', state: 'powered off' },
      { id: SECOND_ID, name: 'VM Two', state: 'running' },
    ],
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
    vi.mocked(api.getVmHardware).mockResolvedValue({
      id: RUNNING_ID,
      cpus: 2,
      memoryMb: 4096,
      hostCpus: 8,
      hostMemoryMb: 16384,
      editable: true,
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

  it('suspends (saves state of) a running VM and refreshes', async () => {
    vi.mocked(api.saveState).mockResolvedValue({
      success: true,
      vmId: RUNNING_ID,
      message: 'VM state saved. Start it to resume.',
    });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Suspend VM One' }));

    await waitFor(() => expect(api.saveState).toHaveBeenCalledWith(RUNNING_ID));
    expect(mockRefresh).toHaveBeenCalled();
  });

  it('does not offer suspend for a stopped VM', () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());

    const { queryByRole } = render(<MachinesView />);
    expect(queryByRole('button', { name: 'Suspend VM One' })).toBeNull();
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

  it('offers the terminal for a running terminal-capable VM', async () => {
    const { findByRole } = render(<MachinesView />);
    expect(await findByRole('button', { name: 'Open VM One terminal in a new tab' })).toBeTruthy();
  });

  it('does not offer the terminal for a stopped VM', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());

    const { queryByRole } = render(<MachinesView />);
    // Flush the guest-OS probe so termCapable is resolved before asserting.
    await act(async () => {});

    expect(queryByRole('button', { name: 'Open VM One terminal in a new tab' })).toBeNull();
  });

  it('shows crash-specific placeholder copy when the focused machine is aborted', async () => {
    vi.mocked(useVmStatus).mockReturnValue({
      ...stoppedVm(),
      vms: [{ id: RUNNING_ID, name: 'VM One', state: 'aborted' }],
    });

    // The single machine is auto-focused, so no click is needed.
    const { findByText } = render(<MachinesView />);

    expect(
      await findByText('This machine stopped unexpectedly (aborted). Start it to boot again.'),
    ).toBeTruthy();
  });

  it('shows resume placeholder copy when the focused machine has a saved state', async () => {
    vi.mocked(useVmStatus).mockReturnValue({
      ...stoppedVm(),
      vms: [{ id: RUNNING_ID, name: 'VM One', state: 'saved' }],
    });

    // The single machine is auto-focused, so no click is needed.
    const { findByText } = render(<MachinesView />);

    expect(
      await findByText('This machine is suspended — start it to resume exactly where it left off.'),
    ).toBeTruthy();
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
      await vi.advanceTimersByTimeAsync(14000);
    });
    expect(queryByRole('button', { name: 'Force power off VM One' })).toBeNull();
  });

  it('offers force power off when the VM is still running after the grace period', async () => {
    vi.useFakeTimers();
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
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
      await vi.advanceTimersByTimeAsync(15000);
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
      await vi.advanceTimersByTimeAsync(15000);
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
      await vi.advanceTimersByTimeAsync(15000);
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
      await vi.advanceTimersByTimeAsync(15000);
    });

    expect(queryByRole('button', { name: 'Force power off VM One' })).toBeNull();
  });

  it('surfaces a notice and a visible force button when the guest ignores the stop signal', async () => {
    vi.useFakeTimers();
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });

    const { getByRole, getByText, container } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
    });

    expect(
      getByText('The guest did not respond to the shutdown signal. You can force power off.'),
    ).toBeTruthy();
    expect(getByRole('button', { name: 'Force power off VM One' })).toBeTruthy();
    // The quiet action group is forced visible so force power off is not hover-only.
    expect(container.querySelector('.tv-quiet--open')).toBeTruthy();
  });

  it('shows no unresponsive-guest notice when the VM stops before the grace period', async () => {
    vi.useFakeTimers();
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });

    const { getByRole, queryByText, rerender } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    // The ACPI signal worked: the next poll reports the VM as powered off.
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    rerender(<MachinesView />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
    });

    expect(
      queryByText('The guest did not respond to the shutdown signal. You can force power off.'),
    ).toBeNull();
  });

  it('dismisses the unresponsive-guest notice while keeping force power off available', async () => {
    vi.useFakeTimers();
    vi.mocked(api.stopVm).mockResolvedValue({ success: true, vmId: RUNNING_ID, message: 'stopped' });

    const { getByRole, queryByText } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Stop VM One' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
    });

    fireEvent.click(getByRole('button', { name: 'Dismiss shutdown notice for VM One' }));

    expect(
      queryByText('The guest did not respond to the shutdown signal. You can force power off.'),
    ).toBeNull();
    expect(getByRole('button', { name: 'Force power off VM One' })).toBeTruthy();
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

  it('does not offer clone for a running VM', () => {
    const { queryByRole } = render(<MachinesView />);
    expect(queryByRole('button', { name: 'Clone VM One' })).toBeNull();
  });

  it('offers clone for a stopped VM and opens the clone form', () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());

    const { getByRole, getByLabelText } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Clone VM One' }));

    // The modal submit button and the new-name field appear.
    expect(getByRole('button', { name: 'Clone' })).toBeTruthy();
    expect((getByLabelText('New VM name') as HTMLInputElement).value).toBe('VM One clone');
  });

  it('shows the snapshot hint only when the linked clone type is selected', () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());

    const { getByRole, queryByText, getByText } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Clone VM One' }));

    const hint = 'A linked clone requires the source VM to have at least one snapshot. Take a snapshot first if it has none.';
    expect(queryByText(hint)).toBeNull();

    fireEvent.click(getByRole('radio', { name: 'Linked clone (faster, shares the source disk)' }));
    expect(getByText(hint)).toBeTruthy();
  });

  it('submits a full clone with the entered name', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.cloneVm).mockResolvedValue({ jobId: 'j-clone' });
    vi.mocked(api.getCreateStatus).mockResolvedValue({ state: 'running', message: '' });

    const { getByRole, getByLabelText } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Clone VM One' }));
    fireEvent.change(getByLabelText('New VM name'), { target: { value: 'my-copy' } });
    fireEvent.click(getByRole('button', { name: 'Clone' }));

    await waitFor(() =>
      expect(api.cloneVm).toHaveBeenCalledWith(RUNNING_ID, { name: 'my-copy', linked: false }),
    );
  });

  it('submits a linked clone when linked is selected', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.cloneVm).mockResolvedValue({ jobId: 'j-clone' });
    vi.mocked(api.getCreateStatus).mockResolvedValue({ state: 'running', message: '' });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Clone VM One' }));
    fireEvent.click(getByRole('radio', { name: 'Linked clone (faster, shares the source disk)' }));
    fireEvent.click(getByRole('button', { name: 'Clone' }));

    await waitFor(() =>
      expect(api.cloneVm).toHaveBeenCalledWith(RUNNING_ID, { name: 'VM One clone', linked: true }),
    );
  });

  it('refreshes the list and closes the form once the clone job is done', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.cloneVm).mockResolvedValue({ jobId: 'j-clone' });
    vi.mocked(api.getCreateStatus).mockResolvedValue({ state: 'done', message: 'Full clone created.' });

    const { getByRole, queryByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Clone VM One' }));

    vi.useFakeTimers();
    fireEvent.click(getByRole('button', { name: 'Clone' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    vi.useRealTimers();

    expect(api.getCreateStatus).toHaveBeenCalledWith('j-clone');
    expect(mockRefresh).toHaveBeenCalled();
    // The modal closed: its submit button is gone.
    expect(queryByRole('button', { name: 'Clone' })).toBeNull();
  });

  it('surfaces a clone error and keeps the form open', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.cloneVm).mockRejectedValue(
      new ApiError({
        status: 400,
        statusText: 'Bad Request',
        body: 'A linked clone requires a snapshot. Take a snapshot of the source VM first, then clone it.',
      }),
    );

    const { getByRole, getByText } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Clone VM One' }));
    fireEvent.click(getByRole('button', { name: 'Clone' }));

    await waitFor(() =>
      expect(
        getByText('A linked clone requires a snapshot. Take a snapshot of the source VM first, then clone it.'),
      ).toBeTruthy(),
    );
  });

  it('does not offer export for a running VM', () => {
    const { queryByRole } = render(<MachinesView />);
    expect(queryByRole('button', { name: 'Export VM One' })).toBeNull();
  });

  it('offers export for a stopped VM and opens the export dialog', () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Export VM One' }));

    expect(getByRole('button', { name: 'Choose folder & export' })).toBeTruthy();
  });

  it('picks a destination folder and posts the export directory', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.pickHostFolder).mockResolvedValue({ path: 'C:\\out', cancelled: false });
    vi.mocked(api.exportVm).mockResolvedValue({ jobId: 'j-export' });
    vi.mocked(api.getCreateStatus).mockResolvedValue({ state: 'running', message: '' });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Export VM One' }));
    fireEvent.click(getByRole('button', { name: 'Choose folder & export' }));

    await waitFor(() =>
      expect(api.exportVm).toHaveBeenCalledWith(RUNNING_ID, { directory: 'C:\\out' }),
    );
  });

  it('does not export when the folder picker is cancelled', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.pickHostFolder).mockResolvedValue({ path: '', cancelled: true });

    const { getByRole } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Export VM One' }));
    fireEvent.click(getByRole('button', { name: 'Choose folder & export' }));

    await waitFor(() => expect(api.pickHostFolder).toHaveBeenCalled());
    expect(api.exportVm).not.toHaveBeenCalled();
  });

  it('shows the written .ova path once the export job is done', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.pickHostFolder).mockResolvedValue({ path: 'C:\\out', cancelled: false });
    vi.mocked(api.exportVm).mockResolvedValue({ jobId: 'j-export' });
    vi.mocked(api.getCreateStatus).mockResolvedValue({
      state: 'done',
      message: 'Exported to C:\\out\\VM One.ova',
    });

    const { getByRole, findByText } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Export VM One' }));
    fireEvent.click(getByRole('button', { name: 'Choose folder & export' }));

    // The poll interval is 2s; give findByText room to see the resolved path.
    expect(await findByText('Exported to C:\\out\\VM One.ova', undefined, { timeout: 4000 })).toBeTruthy();
    expect(mockRefresh).toHaveBeenCalled();
  });

  it('auto-focuses the first VM when nothing is selected and none is running', async () => {
    vi.mocked(useVmStatus).mockReturnValue(twoStoppedVms());

    const { container } = render(<MachinesView />);

    await waitFor(() =>
      expect(container.querySelector('.tv-vm.is-focused h3')?.textContent).toBe('VM One'),
    );
  });

  it('auto-focuses the first running VM over an earlier stopped one', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedThenRunning());

    const { container } = render(<MachinesView />);

    await waitFor(() =>
      expect(container.querySelector('.tv-vm.is-focused h3')?.textContent).toBe('VM Two'),
    );
  });

  it('keeps the auto-focused running VM selected after it stops', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedThenRunning());

    const { container, rerender } = render(<MachinesView />);
    await waitFor(() =>
      expect(container.querySelector('.tv-vm.is-focused h3')?.textContent).toBe('VM Two'),
    );

    vi.mocked(useVmStatus).mockReturnValue(twoStoppedVms());
    rerender(<MachinesView />);

    await waitFor(() =>
      expect(container.querySelector('.tv-vm.is-focused h3')?.textContent).toBe('VM Two'),
    );
  });

  it('preserves the user selection across a refresh', async () => {
    vi.mocked(useVmStatus).mockReturnValue(twoStoppedVms());

    const { container, getByText, rerender } = render(<MachinesView />);
    await waitFor(() =>
      expect(container.querySelector('.tv-vm.is-focused h3')?.textContent).toBe('VM One'),
    );

    fireEvent.click(getByText('VM Two'));
    expect(container.querySelector('.tv-vm.is-focused h3')?.textContent).toBe('VM Two');

    // A refresh delivers a fresh array with the same machines: focus must stay.
    vi.mocked(useVmStatus).mockReturnValue(twoStoppedVms());
    rerender(<MachinesView />);

    await waitFor(() =>
      expect(container.querySelector('.tv-vm.is-focused h3')?.textContent).toBe('VM Two'),
    );
  });

  it('shows configured vCPU and memory in the rail for a stopped focused VM', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.getVmHardware).mockResolvedValue({
      id: RUNNING_ID,
      cpus: 3,
      memoryMb: 2048,
      hostCpus: 8,
      hostMemoryMb: 16384,
      editable: true,
    });

    const { container, getAllByText, queryByText, findByText } = render(<MachinesView />);

    await waitFor(() =>
      expect(container.querySelector('.tv-tele .big')?.textContent).toContain('3'),
    );
    expect(await findByText('2 GB')).toBeTruthy();
    // Both rail groups read "Configured": these are settings, not live session data.
    expect(getAllByText('Configured').length).toBe(2);
    expect(queryByText('Session')).toBeNull();
  });

  it('shows the Guest Additions CTA as a focus notice instead of a row button', async () => {
    vi.mocked(api.getGuestAdditionsStatus).mockResolvedValue({
      id: RUNNING_ID,
      installed: false,
      updateAvailable: false,
      status: 'not-detected',
    });

    const { container, findByRole } = render(<MachinesView />);

    const install = await findByRole('button', { name: 'Install Guest Additions on VM One' });
    const notice = container.querySelector('.tv-ga-notice');
    expect(notice).toBeTruthy();
    expect(notice?.contains(install)).toBe(true);
    // The row action bar no longer hosts the CTA.
    expect(container.querySelector('.tv-vm-actions .tv-abtn.ga')).toBeNull();
  });

  it('installs from the notice and shows the disc-inserted state there', async () => {
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

    const { container, findByRole } = render(<MachinesView />);
    fireEvent.click(await findByRole('button', { name: 'Install Guest Additions on VM One' }));

    await waitFor(() => expect(api.installGuestAdditions).toHaveBeenCalledWith(RUNNING_ID));
    await waitFor(() =>
      expect(container.querySelector('.tv-ga-notice')?.textContent).toContain(
        'disc inserted · run installer in VM',
      ),
    );
  });

  it('offers the Guest Additions update in the focus notice, not the row', async () => {
    vi.mocked(api.getGuestAdditionsStatus).mockResolvedValue({
      id: RUNNING_ID,
      installed: true,
      version: '7.0.12',
      hostVersion: '7.0.14',
      updateAvailable: true,
      status: 'installed',
    });

    const { container, findByRole } = render(<MachinesView />);

    const update = await findByRole('button', { name: 'Update Guest Additions on VM One' });
    const notice = container.querySelector('.tv-ga-notice');
    expect(notice?.contains(update)).toBe(true);
    expect(container.querySelector('.tv-vm-actions .tv-abtn.ga')).toBeNull();
    // The notice explains the version jump.
    expect(notice?.textContent).toContain('7.0.12');
    expect(notice?.textContent).toContain('7.0.14');
  });

  it('surfaces an export error and keeps the dialog open', async () => {
    vi.mocked(useVmStatus).mockReturnValue(stoppedVm());
    vi.mocked(api.pickHostFolder).mockResolvedValue({ path: 'C:\\out', cancelled: false });
    vi.mocked(api.exportVm).mockRejectedValue(
      new ApiError({
        status: 400,
        statusText: 'Bad Request',
        body: 'A file named "VM One.ova" already exists in the destination folder. Choose another folder or remove it first.',
      }),
    );

    const { getByRole, getByText } = render(<MachinesView />);
    fireEvent.click(getByRole('button', { name: 'Export VM One' }));
    fireEvent.click(getByRole('button', { name: 'Choose folder & export' }));

    await waitFor(() =>
      expect(
        getByText(
          'A file named "VM One.ova" already exists in the destination folder. Choose another folder or remove it first.',
        ),
      ).toBeTruthy(),
    );
  });
});
