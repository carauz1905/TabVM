import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import { CreateVmWizard } from './CreateVmWizard';
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
      pickHostFile: vi.fn(),
      importVm: vi.fn(),
      createVm: vi.fn(),
      createVmManual: vi.fn(),
      getCreateStatus: vi.fn(),
    },
  };
});

const OVA = 'C:\\images\\kali.ova';
const ISO = 'C:\\iso\\alpine.iso';

describe('CreateVmWizard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getCreateStatus).mockResolvedValue({ state: 'running', message: '' });
  });

  afterEach(() => {
    vi.useRealTimers();
    cleanup();
  });

  it('renders the three creation mode tabs', () => {
    const { getByRole } = render(<CreateVmWizard onClose={vi.fn()} onCreated={vi.fn()} />);

    expect(getByRole('tab', { name: 'Import image (.ova)' })).toBeTruthy();
    expect(getByRole('tab', { name: 'Install from ISO' })).toBeTruthy();
    expect(getByRole('tab', { name: 'Other OS (manual install)' })).toBeTruthy();
  });

  it('manual mode hides guest credentials and offers only generic OS types', () => {
    const { getByRole, queryByText, getAllByRole } = render(
      <CreateVmWizard onClose={vi.fn()} onCreated={vi.fn()} />,
    );

    fireEvent.click(getByRole('tab', { name: 'Other OS (manual install)' }));

    expect(queryByText('Guest username')).toBeNull();
    expect(queryByText('Guest password')).toBeNull();
    const options = getAllByRole('option').map((o) => o.textContent);
    expect(options).toEqual(['Linux (64-bit)', 'Other (64-bit)']);
  });

  it('submits an import with the chosen appliance', async () => {
    vi.mocked(api.pickHostFile).mockResolvedValue({ path: OVA, cancelled: false });
    vi.mocked(api.importVm).mockResolvedValue({ jobId: 'j-import' });

    const { getByRole, getByPlaceholderText, findByText } = render(
      <CreateVmWizard onClose={vi.fn()} onCreated={vi.fn()} />,
    );

    fireEvent.change(getByPlaceholderText('lab-vm'), { target: { value: 'kali' } });
    fireEvent.click(getByRole('button', { name: 'Choose .ova/.ovf…' }));
    await findByText(OVA);

    fireEvent.click(getByRole('button', { name: 'Import' }));

    await waitFor(() => expect(api.importVm).toHaveBeenCalledWith(OVA, 'kali'));
  });

  it('submits an unattended install with the expected payload', async () => {
    vi.mocked(api.pickHostFile).mockResolvedValue({ path: ISO, cancelled: false });
    vi.mocked(api.createVm).mockResolvedValue({ jobId: 'j-install' });

    const { getByRole, getByPlaceholderText, getByLabelText, findByText } = render(
      <CreateVmWizard onClose={vi.fn()} onCreated={vi.fn()} />,
    );

    fireEvent.click(getByRole('tab', { name: 'Install from ISO' }));
    fireEvent.change(getByPlaceholderText('lab-vm'), { target: { value: 'ubu' } });
    fireEvent.click(getByRole('button', { name: 'Choose .iso…' }));
    await findByText(ISO);
    fireEvent.change(getByLabelText('Guest password'), { target: { value: 'pw123' } });

    fireEvent.click(getByRole('button', { name: 'Create' }));

    await waitFor(() =>
      expect(api.createVm).toHaveBeenCalledWith({
        name: 'ubu',
        osType: 'Ubuntu_64',
        isoPath: ISO,
        memoryMb: 2048,
        cpus: 2,
        diskGb: 25,
        username: 'student',
        password: 'pw123',
        hostname: '',
      }),
    );
  });

  it('submits a manual create with the expected payload', async () => {
    vi.mocked(api.pickHostFile).mockResolvedValue({ path: ISO, cancelled: false });
    vi.mocked(api.createVmManual).mockResolvedValue({ jobId: 'j-manual' });

    const { getByRole, getByPlaceholderText, findByText } = render(
      <CreateVmWizard onClose={vi.fn()} onCreated={vi.fn()} />,
    );

    fireEvent.click(getByRole('tab', { name: 'Other OS (manual install)' }));
    fireEvent.change(getByPlaceholderText('lab-vm'), { target: { value: 'alp' } });
    fireEvent.click(getByRole('button', { name: 'Choose .iso…' }));
    await findByText(ISO);

    fireEvent.click(getByRole('button', { name: 'Create' }));

    await waitFor(() =>
      expect(api.createVmManual).toHaveBeenCalledWith({
        name: 'alp',
        osType: 'Linux_64',
        isoPath: ISO,
        memoryMb: 2048,
        cpus: 2,
        diskGb: 25,
      }),
    );
  });

  it('keeps the submit button disabled until the required fields per mode are filled', async () => {
    vi.mocked(api.pickHostFile).mockResolvedValue({ path: ISO, cancelled: false });

    const { getByRole, getByPlaceholderText, findByText } = render(
      <CreateVmWizard onClose={vi.fn()} onCreated={vi.fn()} />,
    );

    // Import mode: a name alone is not enough — the appliance is required.
    fireEvent.change(getByPlaceholderText('lab-vm'), { target: { value: 'vm1' } });
    expect((getByRole('button', { name: 'Import' }) as HTMLButtonElement).disabled).toBe(true);

    // Install mode: name + ISO still needs the guest password.
    fireEvent.click(getByRole('tab', { name: 'Install from ISO' }));
    fireEvent.click(getByRole('button', { name: 'Choose .iso…' }));
    await findByText(ISO);
    expect((getByRole('button', { name: 'Create' }) as HTMLButtonElement).disabled).toBe(true);

    // Manual mode: name + ISO is enough — no credentials are required.
    fireEvent.click(getByRole('tab', { name: 'Other OS (manual install)' }));
    expect((getByRole('button', { name: 'Create' }) as HTMLButtonElement).disabled).toBe(false);
  });

  it('stops polling and shows an error when the job is unknown (agent restarted)', async () => {
    vi.mocked(api.pickHostFile).mockResolvedValue({ path: ISO, cancelled: false });
    vi.mocked(api.createVmManual).mockResolvedValue({ jobId: 'j-gone' });
    vi.mocked(api.getCreateStatus).mockRejectedValue(
      new ApiError({ status: 404, statusText: 'Not Found', body: 'Unknown job.' }),
    );

    const { getByRole, getByPlaceholderText, getByText, findByText } = render(
      <CreateVmWizard onClose={vi.fn()} onCreated={vi.fn()} />,
    );

    fireEvent.click(getByRole('tab', { name: 'Other OS (manual install)' }));
    fireEvent.change(getByPlaceholderText('lab-vm'), { target: { value: 'alp' } });
    fireEvent.click(getByRole('button', { name: 'Choose .iso…' }));
    await findByText(ISO);

    vi.useFakeTimers();
    fireEvent.click(getByRole('button', { name: 'Create' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    vi.useRealTimers();

    expect(api.getCreateStatus).toHaveBeenCalledTimes(1);
    expect(
      getByText(
        'The creation job is no longer available. The agent may have restarted; check the machine list before retrying.',
      ),
    ).toBeTruthy();
  });

  it('gives up after repeated poll failures instead of spinning forever', async () => {
    vi.mocked(api.pickHostFile).mockResolvedValue({ path: ISO, cancelled: false });
    vi.mocked(api.createVmManual).mockResolvedValue({ jobId: 'j-flaky' });
    vi.mocked(api.getCreateStatus).mockRejectedValue(new Error('network down'));

    const { getByRole, getByPlaceholderText, getByText, findByText } = render(
      <CreateVmWizard onClose={vi.fn()} onCreated={vi.fn()} />,
    );

    fireEvent.click(getByRole('tab', { name: 'Other OS (manual install)' }));
    fireEvent.change(getByPlaceholderText('lab-vm'), { target: { value: 'alp' } });
    fireEvent.click(getByRole('button', { name: 'Choose .iso…' }));
    await findByText(ISO);

    vi.useFakeTimers();
    fireEvent.click(getByRole('button', { name: 'Create' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000 * 10);
    });
    vi.useRealTimers();

    expect(api.getCreateStatus).toHaveBeenCalledTimes(10);
    expect(
      getByText('Lost contact with the agent while creating the VM. Check the machine list before retrying.'),
    ).toBeTruthy();
  });
});
