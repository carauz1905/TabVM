import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { NetworkPanel } from './NetworkPanel';
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
    },
  };
});

function natOptions(forwarding: PortForwardingRule[] = []) {
  return {
    adapters: [{ slot: 1, mode: 'nat', mac: '08:00:27:11:22:AA', forwarding }],
    bridgedAdapters: [],
    hostOnlyAdapters: [],
  };
}

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
      adapters: [{ slot: 1, mode: 'bridged', adapter: 'Ethernet', mac: '08:00:27:11:22:AA' }],
      bridgedAdapters: ['Ethernet'],
      hostOnlyAdapters: [],
    });

    const { queryByText, queryByRole } = render(<NetworkPanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getNetworkOptions).toHaveBeenCalled());

    expect(queryByText('Port forwarding')).toBeNull();
    expect(queryByRole('button', { name: 'Add rule' })).toBeNull();
  });
});
