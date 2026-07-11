import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, cleanup } from '@testing-library/react';
import { ActivityView } from './ActivityView';
import { api } from '../api/client';

vi.mock('../api/client', () => ({
  api: { getActivity: vi.fn() },
}));

describe('ActivityView', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => cleanup());

  it('renders recorded operations from the activity endpoint', async () => {
    vi.mocked(api.getActivity).mockResolvedValue({
      entries: [
        { vmId: 'vm-1', action: 'start', success: true, recordedAt: '2026-01-01T00:00:00Z' },
        { vmId: 'vm-2', action: 'stop', success: false, recordedAt: '2026-01-01T00:01:00Z' },
      ],
    });

    const { getByText } = render(<ActivityView />);

    await waitFor(() => expect(getByText('start')).toBeTruthy());
    expect(getByText('stop')).toBeTruthy();
    expect(getByText('vm-1')).toBeTruthy();
  });

  it('shows an empty state when there are no operations', async () => {
    vi.mocked(api.getActivity).mockResolvedValue({ entries: [] });

    const { getByText } = render(<ActivityView />);

    await waitFor(() => expect(getByText('No recorded operations yet.')).toBeTruthy());
  });

  it('shows an error state when the endpoint fails', async () => {
    vi.mocked(api.getActivity).mockRejectedValue(new Error('404'));

    const { getByText } = render(<ActivityView />);

    await waitFor(() => expect(getByText(/Activity is unavailable/)).toBeTruthy());
  });
});
