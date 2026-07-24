import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, cleanup } from '@testing-library/react';
import { AgentView } from './AgentView';
import { api } from '../api/client';

vi.mock('../hooks/useHealth', () => ({
  useHealth: () => ({
    state: 'success' as const,
    data: { status: 'healthy' as const, timestamp: '2026-01-01T00:00:00Z', uptimeSeconds: 15158 },
  }),
}));

vi.mock('../hooks/useVmStatus', () => ({
  useVmStatus: () => ({
    state: 'success' as const,
    discovery: { found: true, version: '7.2.12r174389' },
    vms: [],
    refresh: vi.fn(),
  }),
}));

vi.mock('../api/client', () => ({
  api: { getLocalStateStatus: vi.fn(), getUpdateStatus: vi.fn() },
}));

describe('AgentView', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.mocked(api.getLocalStateStatus).mockResolvedValue({ configured: true, available: true, schema: 1 });
    vi.mocked(api.getUpdateStatus).mockResolvedValue({ current: '0.3.2', updateAvailable: false });
  });
  afterEach(() => cleanup());

  it('shows real health, uptime, VirtualBox version and local state', async () => {
    const { getByText } = render(<AgentView />);

    expect(getByText('healthy')).toBeTruthy();
    expect(getByText('04:12:38')).toBeTruthy();
    expect(getByText('7.2.12r174389')).toBeTruthy();
    await waitFor(() => expect(getByText('ready')).toBeTruthy());
  });

  it('shows the latest available release when the update check resolves', async () => {
    vi.mocked(api.getUpdateStatus).mockResolvedValue({
      current: '0.3.2',
      latest: '0.3.2',
      updateAvailable: false,
      releaseUrl: 'https://github.com/example/tabvm/releases/tag/v0.3.2',
    });

    const { getByText, findByText } = render(<AgentView />);

    expect(getByText('Latest available release')).toBeTruthy();
    expect(await findByText('v0.3.2')).toBeTruthy();
  });

  it('hints when the latest release is newer than the current one', async () => {
    vi.mocked(api.getUpdateStatus).mockResolvedValue({
      current: '0.3.2',
      latest: '0.4.0',
      updateAvailable: true,
      releaseUrl: 'https://github.com/example/tabvm/releases/tag/v0.4.0',
    });

    const { findByText } = render(<AgentView />);

    expect(await findByText('v0.4.0 — update available')).toBeTruthy();
  });

  it('shows a dash when the update check is unavailable', async () => {
    vi.mocked(api.getUpdateStatus).mockRejectedValue(new Error('offline'));

    const { getByText } = render(<AgentView />);
    await waitFor(() => expect(api.getUpdateStatus).toHaveBeenCalled());

    const row = getByText('Latest available release').closest('.tv-kv');
    expect(row?.querySelector('.v')?.textContent).toBe('—');
  });
});
