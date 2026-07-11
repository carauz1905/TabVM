import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { SharedFolders } from './SharedFolders';
import { api, ApiError } from '../api/client';

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
      getSharedFolders: vi.fn(),
      addSharedFolder: vi.fn(),
      removeSharedFolder: vi.fn(),
    },
  };
});

const VM = '11111111-1111-1111-1111-111111111111';

describe('SharedFolders', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getSharedFolders).mockResolvedValue({ folders: [] });
  });

  afterEach(() => {
    cleanup();
  });

  it('lists the existing shared folders', async () => {
    vi.mocked(api.getSharedFolders).mockResolvedValue({
      folders: [{ name: 'labshare', hostPath: 'C:\\labs\\share', transient: false }],
    });

    const { getByText } = render(<SharedFolders vmId={VM} />);

    await waitFor(() => expect(getByText('labshare')).toBeTruthy());
    expect(getByText('C:\\labs\\share')).toBeTruthy();
  });

  it('adds a shared folder and reloads the list', async () => {
    vi.mocked(api.addSharedFolder).mockResolvedValue({ success: true, vmId: VM, message: 'added' });

    const { getByLabelText, getByRole } = render(<SharedFolders vmId={VM} />);
    await waitFor(() => expect(api.getSharedFolders).toHaveBeenCalledTimes(1));

    fireEvent.change(getByLabelText('Share name'), { target: { value: 'labshare' } });
    fireEvent.change(getByLabelText('Host path'), { target: { value: 'C:\\labs\\share' } });
    fireEvent.click(getByRole('button', { name: 'Share folder' }));

    await waitFor(() =>
      expect(api.addSharedFolder).toHaveBeenCalledWith(VM, 'labshare', 'C:\\labs\\share'),
    );
    // A reload runs after a successful add.
    await waitFor(() => expect(api.getSharedFolders).toHaveBeenCalledTimes(2));
  });

  it('shows the sanitized backend error when adding fails', async () => {
    vi.mocked(api.addSharedFolder).mockRejectedValue(
      new ApiError({ status: 400, statusText: 'Bad Request', body: 'Host path must be a directory.' }),
    );

    const { getByLabelText, getByRole, getByText } = render(<SharedFolders vmId={VM} />);
    await waitFor(() => expect(api.getSharedFolders).toHaveBeenCalled());

    fireEvent.change(getByLabelText('Share name'), { target: { value: 'labshare' } });
    fireEvent.change(getByLabelText('Host path'), { target: { value: 'C:\\labs\\file.txt' } });
    fireEvent.click(getByRole('button', { name: 'Share folder' }));

    await waitFor(() => expect(getByText('Host path must be a directory.')).toBeTruthy());
  });

  it('removes a shared folder', async () => {
    vi.mocked(api.getSharedFolders).mockResolvedValue({
      folders: [{ name: 'labshare', hostPath: 'C:\\labs\\share', transient: false }],
    });
    vi.mocked(api.removeSharedFolder).mockResolvedValue({ success: true, vmId: VM, message: 'removed' });

    const { getByRole } = render(<SharedFolders vmId={VM} />);
    await waitFor(() => expect(getByRole('button', { name: 'Remove shared folder labshare' })).toBeTruthy());

    fireEvent.click(getByRole('button', { name: 'Remove shared folder labshare' }));

    await waitFor(() => expect(api.removeSharedFolder).toHaveBeenCalledWith(VM, 'labshare'));
  });
});
