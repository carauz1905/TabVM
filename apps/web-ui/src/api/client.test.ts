import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api, ApiError, configureSessionToken } from './client';

describe('api client', () => {
  beforeEach(() => {
    configureSessionToken('test-token');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function mockFetch(response: Partial<Response> & { text: () => Promise<string> }) {
    globalThis.fetch = vi.fn().mockResolvedValue(response as Response);
  }

  it('sends the configured session token header for API routes', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"vms":[{"id":"11111111-1111-1111-1111-111111111111","name":"VM One","state":"listed"}]}',
    });

    await api.getVms();

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    const requestInit = call[1] as RequestInit | undefined;
    const headers = requestInit?.headers as Headers;
    expect(headers.get('X-TabVM-Session-Token')).toBe('test-token');
  });

  it('does not send the session token to the health endpoint', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"status":"healthy","timestamp":"2026-01-01T00:00:00Z"}',
    });

    await api.getHealth();

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    const requestInit = call[1] as RequestInit | undefined;
    const headers = requestInit?.headers as Headers;
    expect(headers.has('X-TabVM-Session-Token')).toBe(false);
  });

  it('fetches the activity feed from the activity endpoint', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({
          entries: [
            { vmId: 'vm-1', action: 'start', success: true, recordedAt: '2026-01-01T00:00:00Z' },
          ],
        }),
    });

    const res = await api.getActivity();

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/api/activity');
    expect(res.entries[0].action).toBe('start');
    expect(res.entries[0].success).toBe(true);
  });

  it('fetches VM telemetry with network interfaces from the telemetry endpoint', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({
          id: '11111111-1111-1111-1111-111111111111',
          cpuCount: 4,
          ramMb: 8192,
          guestAdditions: true,
          networks: [{ slot: 1, mode: 'bridged', mac: '0800271122AA', ipv4: ['192.168.1.42'] }],
          disks: [
            { name: 'lab.vdi', capacityBytes: 53687091200, allocatedBytes: 13421772800, percent: 25 },
          ],
        }),
    });

    const telemetry = await api.getVmTelemetry('11111111-1111-1111-1111-111111111111');

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/api/vms/11111111-1111-1111-1111-111111111111/telemetry');
    expect(telemetry.networks[0].ipv4).toEqual(['192.168.1.42']);
    expect(telemetry.guestAdditions).toBe(true);
    expect(telemetry.disks[0].percent).toBe(25);
  });

  it('rejects a telemetry payload with a malformed network interface', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({
          id: '11111111-1111-1111-1111-111111111111',
          cpuCount: 2,
          ramMb: 4096,
          guestAdditions: false,
          networks: [{ slot: 1, mode: 'nat', ipv4: [42] }],
          disks: [],
        }),
    });

    await expect(
      api.getVmTelemetry('11111111-1111-1111-1111-111111111111'),
    ).rejects.toBeInstanceOf(ApiError);
  });

  it('lists shared folders from the shared-folders endpoint', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({
          folders: [{ name: 'labshare', hostPath: 'C:\\labs\\share', transient: false }],
        }),
    });

    const res = await api.getSharedFolders('11111111-1111-1111-1111-111111111111');

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/api/vms/11111111-1111-1111-1111-111111111111/shared-folders');
    expect(res.folders[0].name).toBe('labshare');
    expect(res.folders[0].transient).toBe(false);
  });

  it('adds a shared folder with a JSON body via POST', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({
          success: true,
          vmId: '11111111-1111-1111-1111-111111111111',
          message: 'added',
        }),
    });

    const res = await api.addSharedFolder(
      '11111111-1111-1111-1111-111111111111',
      'labshare',
      'C:\\labs\\share',
    );

    expect(res.success).toBe(true);
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    const requestInit = call[1] as RequestInit | undefined;
    expect(requestInit?.method).toBe('POST');
    expect(JSON.parse(requestInit?.body as string)).toEqual({
      name: 'labshare',
      hostPath: 'C:\\labs\\share',
    });
  });

  it('removes a shared folder with a JSON body via POST', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({
          success: true,
          vmId: '11111111-1111-1111-1111-111111111111',
          message: 'removed',
        }),
    });

    await api.removeSharedFolder('11111111-1111-1111-1111-111111111111', 'labshare');

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/shared-folders/remove');
    const requestInit = call[1] as RequestInit | undefined;
    expect(JSON.parse(requestInit?.body as string)).toEqual({ name: 'labshare' });
  });

  it('reads the clipboard mode from the clipboard endpoint', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({ id: '11111111-1111-1111-1111-111111111111', mode: 'bidirectional' }),
    });

    const res = await api.getClipboardMode('11111111-1111-1111-1111-111111111111');

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/api/vms/11111111-1111-1111-1111-111111111111/clipboard');
    expect(res.mode).toBe('bidirectional');
  });

  it('sets the clipboard mode with a JSON body via POST', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({ id: '11111111-1111-1111-1111-111111111111', mode: 'bidirectional' }),
    });

    await api.setClipboardMode('11111111-1111-1111-1111-111111111111', 'bidirectional');

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    const requestInit = call[1] as RequestInit | undefined;
    expect(requestInit?.method).toBe('POST');
    expect(JSON.parse(requestInit?.body as string)).toEqual({ mode: 'bidirectional' });
  });

  it('reads the Guest Additions status from the guest-additions endpoint', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({
          id: '11111111-1111-1111-1111-111111111111',
          installed: false,
          status: 'not-detected',
        }),
    });

    const res = await api.getGuestAdditionsStatus('11111111-1111-1111-1111-111111111111');

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/api/vms/11111111-1111-1111-1111-111111111111/guest-additions');
    expect(res.status).toBe('not-detected');
  });

  it('requests Guest Additions install via POST', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        JSON.stringify({
          success: true,
          vmId: '11111111-1111-1111-1111-111111111111',
          controller: 'IDE',
          port: 1,
          device: 0,
          message: 'Guest Additions disc inserted.',
        }),
    });

    await api.installGuestAdditions('11111111-1111-1111-1111-111111111111');

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain(
      '/api/vms/11111111-1111-1111-1111-111111111111/guest-additions/install',
    );
    expect((call[1] as RequestInit | undefined)?.method).toBe('POST');
  });

  it('throws ApiError for non-OK responses with the response body', async () => {
    mockFetch({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      text: async () => 'VBoxManage not discovered',
    });

    await expect(api.getVms()).rejects.toBeInstanceOf(ApiError);
  });

  it('throws ApiError when the response body is not valid JSON', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => 'not valid json',
    });

    await expect(api.getHealth()).rejects.toBeInstanceOf(ApiError);
  });

  it('throws ApiError when health JSON has the wrong shape', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"status":"healthy"}',
    });

    await expect(api.getHealth()).rejects.toBeInstanceOf(ApiError);
  });

  it('throws ApiError when discovery JSON has the wrong shape', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"found":"yes"}',
    });

    await expect(api.getVirtualBoxDiscovery()).rejects.toBeInstanceOf(ApiError);
  });

  it('throws ApiError when VM list JSON is missing the vms array', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{}',
    });

    await expect(api.getVms()).rejects.toBeInstanceOf(ApiError);
  });

  it('throws ApiError when VM list JSON contains an invalid VM entry', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"vms":[{"name":"VM A"}]}',
    });

    await expect(api.getVms()).rejects.toBeInstanceOf(ApiError);
  });

  it('parses and returns valid JSON', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"vms":[{"id":"11111111-1111-1111-1111-111111111111","name":"VM A","state":"listed"}]}',
    });

    const result = await api.getVms();

    expect(result.vms).toHaveLength(1);
    expect(result.vms[0].name).toBe('VM A');
  });

  it('does not send a token header when none is configured', async () => {
    configureSessionToken(undefined);
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"status":"healthy","timestamp":"2026-01-01T00:00:00Z"}',
    });

    await api.getHealth();

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    const requestInit = call[1] as RequestInit | undefined;
    const headers = requestInit?.headers as Headers;
    expect(headers.has('X-TabVM-Session-Token')).toBe(false);
  });

  it('starts a VM with POST', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"success":true,"vmId":"11111111-1111-1111-1111-111111111111","message":"VM start requested."}',
    });

    const result = await api.startVm('11111111-1111-1111-1111-111111111111');

    expect(result.success).toBe(true);
    expect(result.vmId).toBe('11111111-1111-1111-1111-111111111111');

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    const requestInit = call[1] as RequestInit | undefined;
    expect(requestInit?.method).toBe('POST');
    expect(call[0]).toContain('/api/vms/11111111-1111-1111-1111-111111111111/start');
  });

  it('stops a VM with POST', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"success":true,"vmId":"11111111-1111-1111-1111-111111111111","message":"VM stop requested."}',
    });

    const result = await api.stopVm('11111111-1111-1111-1111-111111111111');

    expect(result.success).toBe(true);
    expect(requestMethod()).toBe('POST');
  });

  it('saves a VM state (suspend) with POST', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        '{"success":true,"vmId":"11111111-1111-1111-1111-111111111111","message":"VM state saved. Start it to resume."}',
    });

    const result = await api.saveState('11111111-1111-1111-1111-111111111111');

    expect(result.success).toBe(true);
    expect(requestMethod()).toBe('POST');
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/api/vms/11111111-1111-1111-1111-111111111111/savestate');
  });

  it('resets a VM with POST', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"success":true,"vmId":"11111111-1111-1111-1111-111111111111","message":"VM reset requested."}',
    });

    const result = await api.resetVm('11111111-1111-1111-1111-111111111111');

    expect(result.success).toBe(true);
    expect(requestMethod()).toBe('POST');
  });

  it('force powers off a VM with POST', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"success":true,"vmId":"11111111-1111-1111-1111-111111111111","message":"VM power off forced."}',
    });

    const result = await api.forcePowerOffVm('11111111-1111-1111-1111-111111111111');

    expect(result.success).toBe(true);
    expect(requestMethod()).toBe('POST');
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/api/vms/11111111-1111-1111-1111-111111111111/poweroff');
  });

  it('fetches VM status', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () => '{"id":"11111111-1111-1111-1111-111111111111","state":"running"}',
    });

    const result = await api.getVmStatus('11111111-1111-1111-1111-111111111111');

    expect(result.id).toBe('11111111-1111-1111-1111-111111111111');
    expect(result.state).toBe('running');
  });


  it('runs a command in the guest and returns the exit code and output', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        '{"success":true,"vmId":"11111111-1111-1111-1111-111111111111","exitCode":0,"output":"hello\\n","truncated":false,"message":"Command finished with exit code 0.","credentialsRequired":false}',
    });

    const res = await api.runInGuest(
      '11111111-1111-1111-1111-111111111111',
      '/bin/echo',
      ['hello'],
      'root',
      'secret',
    );

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/api/vms/11111111-1111-1111-1111-111111111111/guest/run');
    expect(requestMethod()).toBe('POST');
    const body = JSON.parse((call[1] as RequestInit).body as string);
    expect(body).toEqual({ exe: '/bin/echo', args: ['hello'], username: 'root', password: 'secret' });
    expect(res.exitCode).toBe(0);
    expect(res.output).toBe('hello\n');
  });

  it('copies a file out of the guest and returns the host path', async () => {
    mockFetch({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: async () =>
        '{"success":true,"vmId":"11111111-1111-1111-1111-111111111111","hostPath":"C:\\\\dst\\\\report.txt","message":"ok","credentialsRequired":false}',
    });

    const res = await api.copyFromGuest(
      '11111111-1111-1111-1111-111111111111',
      '/home/root/report.txt',
      'C:\\dst',
      'root',
      'secret',
    );

    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toContain('/api/vms/11111111-1111-1111-1111-111111111111/guest/copyfrom');
    expect(requestMethod()).toBe('POST');
    const body = JSON.parse((call[1] as RequestInit).body as string);
    expect(body).toEqual({
      guestPath: '/home/root/report.txt',
      directory: 'C:\\dst',
      username: 'root',
      password: 'secret',
    });
    expect(res.hostPath).toBe('C:\\dst\\report.txt');
  });

  function requestMethod(): string | undefined {
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    const requestInit = call[1] as RequestInit | undefined;
    return requestInit?.method;
  }
});
