import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, cleanup, fireEvent } from '@testing-library/react';
import { ActivityView } from './ActivityView';
import { LanguageProvider } from '../i18n/i18n';
import { api } from '../api/client';

vi.mock('../api/client', () => ({
  api: { getActivity: vi.fn() },
}));

describe('ActivityView', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

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

  it('renders humanized action labels with success and failure dots', async () => {
    vi.mocked(api.getActivity).mockResolvedValue({
      entries: [
        { vmId: 'vm-1', action: 'vm.start', success: true, recordedAt: '2026-03-05T12:00:00Z' },
        { vmId: 'vm-2', action: 'vm.stop', success: false, recordedAt: '2026-03-05T12:01:00Z' },
      ],
    });

    const { getByText, container } = render(<ActivityView />);

    await waitFor(() => expect(getByText('Start VM')).toBeTruthy());
    expect(getByText('Stop VM (ACPI)')).toBeTruthy();
    const dots = container.querySelectorAll('.tv-log-dot');
    expect(dots).toHaveLength(2);
    expect(dots[0].classList.contains('ok')).toBe(true);
    expect(dots[1].classList.contains('fail')).toBe(true);
  });

  it('renders localized action labels when the language is Spanish', async () => {
    localStorage.setItem('tabvm.lang', 'es');
    vi.mocked(api.getActivity).mockResolvedValue({
      entries: [
        { vmId: 'vm-1', action: 'vm.start', success: true, recordedAt: '2026-03-05T12:00:00Z' },
      ],
    });

    const { getByText, container } = render(
      <LanguageProvider>
        <ActivityView />
      </LanguageProvider>,
    );

    await waitFor(() => expect(getByText('Iniciar VM')).toBeTruthy());
    expect(container.querySelector('.tv-log-dot')?.classList.contains('ok')).toBe(true);
  });

  it('filters by the localized label and by the raw action code', async () => {
    vi.mocked(api.getActivity).mockResolvedValue({
      entries: [
        { vmId: 'vm-1', action: 'vm.start', success: true, recordedAt: '2026-03-05T12:00:00Z' },
        { vmId: 'vm-2', action: 'snapshot.take', success: true, recordedAt: '2026-03-05T12:01:00Z' },
      ],
    });

    const { getByText, queryByText, getByLabelText } = render(<ActivityView />);
    await waitFor(() => expect(getByText('Start VM')).toBeTruthy());

    const input = getByLabelText('Filter activity');
    fireEvent.change(input, { target: { value: 'Start VM' } });
    expect(getByText('Start VM')).toBeTruthy();
    expect(queryByText('Take snapshot')).toBeNull();

    fireEvent.change(input, { target: { value: 'snapshot.take' } });
    expect(getByText('Take snapshot')).toBeTruthy();
    expect(queryByText('Start VM')).toBeNull();
  });

  it('formats timestamps month-first in English', async () => {
    vi.mocked(api.getActivity).mockResolvedValue({
      entries: [
        { vmId: 'vm-1', action: 'vm.start', success: true, recordedAt: '2026-03-05T12:00:00Z' },
      ],
    });

    const { getByText, container } = render(<ActivityView />);

    await waitFor(() => expect(getByText('Start VM')).toBeTruthy());
    const time = container.querySelector('.tv-log-time')?.textContent ?? '';
    expect(time.startsWith('3/5/2026')).toBe(true);
  });

  it('formats timestamps day-first in Spanish', async () => {
    localStorage.setItem('tabvm.lang', 'es');
    vi.mocked(api.getActivity).mockResolvedValue({
      entries: [
        { vmId: 'vm-1', action: 'vm.start', success: true, recordedAt: '2026-03-05T12:00:00Z' },
      ],
    });

    const { getByText, container } = render(
      <LanguageProvider>
        <ActivityView />
      </LanguageProvider>,
    );

    await waitFor(() => expect(getByText('Iniciar VM')).toBeTruthy());
    const time = container.querySelector('.tv-log-time')?.textContent ?? '';
    expect(time.startsWith('5/3/2026')).toBe(true);
  });
});
