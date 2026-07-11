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
  api: { getLocalStateStatus: vi.fn() },
}));

describe('AgentView', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => cleanup());

  it('shows real health, uptime, VirtualBox version and local state', async () => {
    vi.mocked(api.getLocalStateStatus).mockResolvedValue({ configured: true, available: true, schema: 1 });

    const { getByText } = render(<AgentView />);

    expect(getByText('healthy')).toBeTruthy();
    expect(getByText('04:12:38')).toBeTruthy();
    expect(getByText('7.2.12r174389')).toBeTruthy();
    await waitFor(() => expect(getByText('ready')).toBeTruthy());
  });
});
