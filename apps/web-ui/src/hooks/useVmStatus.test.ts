import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useVmStatus } from './useVmStatus';

interface MockResponse {
  ok: boolean;
  status: number;
  statusText: string;
  text: () => Promise<string>;
}

function mockFetchSequence(responses: MockResponse[]) {
  const queue = [...responses];
  globalThis.fetch = vi.fn().mockImplementation(() => {
    const next = queue.shift();
    if (!next) {
      return Promise.reject(new Error('Unexpected fetch call'));
    }
    return Promise.resolve(next as Response);
  });
}

describe('useVmStatus', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows discovery not found and does not fetch VM list', async () => {
    mockFetchSequence([
      {
        ok: true,
        status: 200,
        statusText: 'OK',
        text: async () =>
          JSON.stringify({
            found: false,
            error: 'VBoxManage was not found in the configured search paths.',
          }),
      },
    ]);

    const { result } = renderHook(() => useVmStatus());

    expect(result.current.state).toBe('loading');

    await waitFor(() => expect(result.current.state).toBe('success'));

    expect(result.current.discovery?.found).toBe(false);
    expect(result.current.discovery?.error).toContain('VBoxManage was not found');
    expect(result.current.vms).toHaveLength(0);
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/vbox/discovery'),
      expect.any(Object),
    );
  });

  it('fetches the VM list when VirtualBox is discovered', async () => {
    mockFetchSequence([
      {
        ok: true,
        status: 200,
        statusText: 'OK',
        text: async () =>
          JSON.stringify({
            found: true,
            version: '7.0.14r161095',
          }),
      },
      {
        ok: true,
        status: 200,
        statusText: 'OK',
        text: async () =>
          JSON.stringify({
            vms: [{ id: '11111111-1111-1111-1111-111111111111', name: 'VM One', state: 'listed' }],
          }),
      },
    ]);

    const { result } = renderHook(() => useVmStatus());

    await waitFor(() => expect(result.current.state).toBe('success'));

    expect(result.current.discovery?.found).toBe(true);
    expect(result.current.discovery?.version).toBe('7.0.14r161095');
    expect(result.current.vms).toHaveLength(1);
    expect(result.current.vms[0].name).toBe('VM One');
    expect(globalThis.fetch).toHaveBeenCalledTimes(2);
    expect(globalThis.fetch).toHaveBeenNthCalledWith(
      1,
      expect.stringContaining('/api/vbox/discovery'),
      expect.any(Object),
    );
    expect(globalThis.fetch).toHaveBeenNthCalledWith(
      2,
      expect.stringContaining('/api/vms'),
      expect.any(Object),
    );
  });

  it('surfaces an error when discovery fails', async () => {
    mockFetchSequence([
      {
        ok: false,
        status: 503,
        statusText: 'Service Unavailable',
        text: async () => 'VBoxManage not discovered',
      },
    ]);

    const { result } = renderHook(() => useVmStatus());

    await waitFor(() => expect(result.current.state).toBe('error'));

    expect(result.current.error).toContain('503');
    expect(result.current.vms).toHaveLength(0);
  });

  it('refreshes the VM list when refresh is called', async () => {
    mockFetchSequence([
      {
        ok: true,
        status: 200,
        statusText: 'OK',
        text: async () =>
          JSON.stringify({
            found: true,
            version: '7.0.14r161095',
          }),
      },
      {
        ok: true,
        status: 200,
        statusText: 'OK',
        text: async () =>
          JSON.stringify({
            vms: [{ id: '11111111-1111-1111-1111-111111111111', name: 'VM One', state: 'running' }],
          }),
      },
      {
        ok: true,
        status: 200,
        statusText: 'OK',
        text: async () =>
          JSON.stringify({
            found: true,
            version: '7.0.14r161095',
          }),
      },
      {
        ok: true,
        status: 200,
        statusText: 'OK',
        text: async () =>
          JSON.stringify({
            vms: [
              { id: '11111111-1111-1111-1111-111111111111', name: 'VM One', state: 'running' },
              { id: '22222222-2222-2222-2222-222222222222', name: 'VM Two', state: 'not running' },
            ],
          }),
      },
    ]);

    const { result } = renderHook(() => useVmStatus());

    await waitFor(() => expect(result.current.state).toBe('success'));
    expect(result.current.vms).toHaveLength(1);

    await result.current.refresh();

    await waitFor(() => expect(result.current.vms).toHaveLength(2));
    expect(result.current.vms[1].name).toBe('VM Two');
    expect(globalThis.fetch).toHaveBeenCalledTimes(4);
  });

});
