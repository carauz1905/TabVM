import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { GuestControlPanel, tokenizeCommand } from './GuestControlPanel';
import { api } from '../api/client';
import { clearGuestCreds } from '../hooks/guestCreds';

describe('tokenizeCommand', () => {
  it('splits on whitespace', () => {
    expect(tokenizeCommand('/bin/ls -la /home')).toEqual(['/bin/ls', '-la', '/home']);
  });
  it('keeps a double-quoted argument with spaces intact', () => {
    expect(tokenizeCommand('/bin/cat "/home/user/my file.txt"')).toEqual([
      '/bin/cat',
      '/home/user/my file.txt',
    ]);
  });
  it('keeps a single-quoted argument intact', () => {
    expect(tokenizeCommand("/bin/echo 'a b c'")).toEqual(['/bin/echo', 'a b c']);
  });
  it('ignores surrounding/extra whitespace and returns empty for blank input', () => {
    expect(tokenizeCommand('   ')).toEqual([]);
    expect(tokenizeCommand('  /bin/true   ')).toEqual(['/bin/true']);
  });
});

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
      runInGuest: vi.fn(),
      copyFromGuest: vi.fn(),
      pickHostFolder: vi.fn(),
    },
  };
});

const VM = '11111111-1111-1111-1111-111111111111';

function fillCreds(getByLabelText: (t: string) => HTMLElement) {
  fireEvent.change(getByLabelText('Guest username'), { target: { value: 'root' } });
  fireEvent.change(getByLabelText('Guest password'), { target: { value: 'secret' } });
}

describe('GuestControlPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    clearGuestCreds(VM);
  });

  afterEach(() => {
    cleanup();
  });

  it('runs a command and shows the exit code and output', async () => {
    vi.mocked(api.runInGuest).mockResolvedValue({
      success: true,
      vmId: VM,
      exitCode: 0,
      output: 'hello world',
      truncated: false,
      message: 'Command finished with exit code 0.',
      credentialsRequired: false,
    });

    const { getByLabelText, getByRole, getByText } = render(<GuestControlPanel vmId={VM} />);
    fillCreds(getByLabelText);
    fireEvent.change(getByLabelText('Command to run'), { target: { value: '/bin/echo hello' } });
    fireEvent.click(getByRole('button', { name: 'Run' }));

    await waitFor(() =>
      expect(api.runInGuest).toHaveBeenCalledWith(VM, '/bin/echo', ['hello'], 'root', 'secret'),
    );
    await waitFor(() => expect(getByText('hello world')).toBeTruthy());
    expect(getByText(/exit code/i)).toBeTruthy();
  });

  it('copies a file out of the guest and shows the written host path', async () => {
    vi.mocked(api.pickHostFolder).mockResolvedValue({ path: 'C:\\dst', cancelled: false });
    vi.mocked(api.copyFromGuest).mockResolvedValue({
      success: true,
      vmId: VM,
      hostPath: 'C:\\dst\\report.txt',
      message: 'Copied "report.txt" from the guest to C:\\dst\\report.txt.',
      credentialsRequired: false,
    });

    const { getByLabelText, getByRole, getByText } = render(<GuestControlPanel vmId={VM} />);
    fillCreds(getByLabelText);
    fireEvent.change(getByLabelText('Guest file path'), { target: { value: '/home/root/report.txt' } });

    fireEvent.click(getByRole('button', { name: 'Choose host folder' }));
    await waitFor(() => expect(api.pickHostFolder).toHaveBeenCalled());

    fireEvent.click(getByRole('button', { name: 'Copy from guest' }));
    await waitFor(() =>
      expect(api.copyFromGuest).toHaveBeenCalledWith(VM, '/home/root/report.txt', 'C:\\dst', 'root', 'secret'),
    );
    await waitFor(() => expect(getByText('C:\\dst\\report.txt')).toBeTruthy());
  });

  it('prompts for credentials instead of running when they are missing', async () => {
    const { getByLabelText, getByRole, getByText } = render(<GuestControlPanel vmId={VM} />);
    // No credentials entered.
    fireEvent.change(getByLabelText('Command to run'), { target: { value: '/bin/ls' } });
    fireEvent.click(getByRole('button', { name: 'Run' }));

    await waitFor(() => expect(getByText(/Enter the guest username and password/i)).toBeTruthy());
    expect(api.runInGuest).not.toHaveBeenCalled();
  });
});
