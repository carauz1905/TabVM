import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { NetworkPanel, formatMac } from './NetworkPanel';
import { api } from '../api/client';
import type { PortForwardingRule } from '../types/api';

const VM_ID = '11111111-1111-1111-1111-111111111111';

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
      getNetworkOptions: vi.fn(),
      changeNetworkMode: vi.fn(),
      addPortForwarding: vi.fn(),
      deletePortForwarding: vi.fn(),
      setLinkState: vi.fn(),
    },
  };
});

function natOptions(forwarding: PortForwardingRule[] = [], cableConnected = true) {
  return {
    adapters: [{ slot: 1, mode: 'nat', mac: '08:00:27:11:22:AA', cableConnected, forwarding }],
    bridgedAdapters: [],
    hostOnlyAdapters: [],
  };
}

describe('formatMac', () => {
  it('groups a bare 12-hex-digit MAC into colon-separated pairs', () => {
    expect(formatMac('080027C2FE52')).toBe('08:00:27:C2:FE:52');
  });

  it('preserves the original casing', () => {
    expect(formatMac('080027c2fe52')).toBe('08:00:27:c2:fe:52');
  });

  it('returns an already-separated MAC unchanged', () => {
    expect(formatMac('08:00:27:C2:FE:52')).toBe('08:00:27:C2:FE:52');
  });

  it('returns wrong-length or non-hex input unchanged', () => {
    expect(formatMac('080027C2FE5')).toBe('080027C2FE5');
    expect(formatMac('080027C2FE521')).toBe('080027C2FE521');
    expect(formatMac('080027C2FE5G')).toBe('080027C2FE5G');
    expect(formatMac('')).toBe('');
  });
});

describe('NetworkPanel adapter info', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => cleanup());

  it('renders a bare VBoxManage MAC with colon separators', async () => {
    vi.mocked(api.getNetworkOptions).mockResolvedValue({
      adapters: [{ slot: 1, mode: 'nat', mac: '080027C2FE52', cableConnected: true, forwarding: [] }],
      bridgedAdapters: [],
      hostOnlyAdapters: [],
    });

    const { container, findByText } = render(<NetworkPanel vmId={VM_ID} />);

    expect(await findByText('08:00:27:C2:FE:52')).toBeTruthy();
    expect(container.querySelector('.net-mac')?.textContent).toBe('08:00:27:C2:FE:52');
  });

  it('shows visible labels for the add-rule fields', async () => {
    vi.mocked(api.getNetworkOptions).mockResolvedValue(natOptions());

    const { getByText, getByLabelText } = render(<NetworkPanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getNetworkOptions).toHaveBeenCalled());

    // Small visible labels above each field of the add-rule row.
    expect(getByText('Rule name')).toBeTruthy();
    expect(getByText('Protocol')).toBeTruthy();
    expect(getByText('Host port')).toBeTruthy();
    expect(getByText('Guest port')).toBeTruthy();
    expect(getByText('Host IP (optional)')).toBeTruthy();
    // The slot-scoped aria-labels stay for uniqueness across adapters.
    expect(getByLabelText('Host port (Adapter 1)')).toBeTruthy();
    expect(getByLabelText('Guest port (Adapter 1)')).toBeTruthy();
  });
});

describe('NetworkPanel port forwarding', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getNetworkOptions).mockResolvedValue(natOptions());
  });

  afterEach(() => cleanup());

  it('renders existing forwarding rules for a NAT adapter', async () => {
    vi.mocked(api.getNetworkOptions).mockResolvedValue(
      natOptions([{ name: 'ssh', protocol: 'tcp', hostIp: '127.0.0.1', hostPort: 2222, guestPort: 22 }]),
    );

    const { container, findByText } = render(<NetworkPanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getNetworkOptions).toHaveBeenCalledWith(VM_ID));

    expect(await findByText('ssh')).toBeTruthy();
    const map = container.querySelector('.net-fwd-map');
    expect(map?.textContent).toContain('127.0.0.1:2222');
    expect(map?.textContent).toContain('22/tcp');
  });

  it('shows * for a rule with no host IP instead of faking loopback', async () => {
    vi.mocked(api.getNetworkOptions).mockResolvedValue(
      natOptions([{ name: 'open', protocol: 'tcp', hostIp: '', hostPort: 8080, guestPort: 80 }]),
    );

    const { container, findByText } = render(<NetworkPanel vmId={VM_ID} />);
    await findByText('open');
    const map = container.querySelector('.net-fwd-map');
    expect(map?.textContent).toContain('*:8080');
    expect(map?.textContent).not.toContain('127.0.0.1');
  });

  it('submits the right payload when adding a rule', async () => {
    vi.mocked(api.addPortForwarding).mockResolvedValue({ success: true, vmId: VM_ID, message: 'added' });
    const onChanged = vi.fn();

    const { getByLabelText, getByRole } = render(<NetworkPanel vmId={VM_ID} onChanged={onChanged} />);
    await waitFor(() => expect(api.getNetworkOptions).toHaveBeenCalled());

    fireEvent.change(getByLabelText('Rule name (Adapter 1)'), { target: { value: 'web' } });
    fireEvent.change(getByLabelText('Host port (Adapter 1)'), { target: { value: '8080' } });
    fireEvent.change(getByLabelText('Guest port (Adapter 1)'), { target: { value: '80' } });
    fireEvent.click(getByRole('button', { name: 'Add rule' }));

    await waitFor(() =>
      expect(api.addPortForwarding).toHaveBeenCalledWith(
        VM_ID,
        expect.objectContaining({ slot: 1, name: 'web', protocol: 'tcp', hostPort: 8080, guestPort: 80 }),
      ),
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('calls the delete API when removing a rule', async () => {
    vi.mocked(api.getNetworkOptions).mockResolvedValue(
      natOptions([{ name: 'ssh', protocol: 'tcp', hostIp: '127.0.0.1', hostPort: 2222, guestPort: 22 }]),
    );
    vi.mocked(api.deletePortForwarding).mockResolvedValue({ success: true, vmId: VM_ID, message: 'removed' });

    const { getByRole } = render(<NetworkPanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getNetworkOptions).toHaveBeenCalled());

    fireEvent.click(getByRole('button', { name: 'Remove rule ssh' }));

    await waitFor(() => expect(api.deletePortForwarding).toHaveBeenCalledWith(VM_ID, 1, 'ssh'));
  });

  it('shows no forwarding UI for a non-NAT adapter', async () => {
    vi.mocked(api.getNetworkOptions).mockResolvedValue({
      adapters: [{ slot: 1, mode: 'bridged', adapter: 'Ethernet', mac: '08:00:27:11:22:AA', cableConnected: true }],
      bridgedAdapters: ['Ethernet'],
      hostOnlyAdapters: [],
    });

    const { queryByText, queryByRole } = render(<NetworkPanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getNetworkOptions).toHaveBeenCalled());

    expect(queryByText('Port forwarding')).toBeNull();
    expect(queryByRole('button', { name: 'Add rule' })).toBeNull();
  });
});

describe('NetworkPanel link state', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getNetworkOptions).mockResolvedValue(natOptions());
  });

  afterEach(() => cleanup());

  it('renders the current cable state and its toggle', async () => {
    vi.mocked(api.getNetworkOptions).mockResolvedValue(natOptions([], true));

    const { findByText, getByRole } = render(<NetworkPanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getNetworkOptions).toHaveBeenCalled());

    expect(await findByText('connected')).toBeTruthy();
    // A connected cable offers the Disconnect action.
    expect(getByRole('button', { name: 'Disconnect' })).toBeTruthy();
  });

  it('posts the toggled state and reloads when disconnecting', async () => {
    vi.mocked(api.getNetworkOptions).mockResolvedValue(natOptions([], true));
    vi.mocked(api.setLinkState).mockResolvedValue({
      success: true,
      vmId: VM_ID,
      message: 'Adapter 1 cable disconnected.',
    });
    const onChanged = vi.fn();

    const { getByRole } = render(<NetworkPanel vmId={VM_ID} onChanged={onChanged} />);
    await waitFor(() => expect(api.getNetworkOptions).toHaveBeenCalledTimes(1));

    fireEvent.click(getByRole('button', { name: 'Disconnect' }));

    await waitFor(() => expect(api.setLinkState).toHaveBeenCalledWith(VM_ID, 1, false));
    // Reloads the adapters after the toggle so the new cable state shows.
    await waitFor(() => expect(api.getNetworkOptions).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('connects a disconnected cable', async () => {
    vi.mocked(api.getNetworkOptions).mockResolvedValue(natOptions([], false));
    vi.mocked(api.setLinkState).mockResolvedValue({
      success: true,
      vmId: VM_ID,
      message: 'Adapter 1 cable connected.',
    });

    const { getByRole, findByText } = render(<NetworkPanel vmId={VM_ID} />);
    await findByText('disconnected');

    fireEvent.click(getByRole('button', { name: 'Connect' }));

    await waitFor(() => expect(api.setLinkState).toHaveBeenCalledWith(VM_ID, 1, true));
  });
});
