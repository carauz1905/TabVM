import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { HardwarePanel } from './HardwarePanel';
import { api } from '../api/client';

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
      getVmHardware: vi.fn(),
      setVmHardware: vi.fn(),
    },
  };
});

describe('HardwarePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getVmHardware).mockResolvedValue({
      id: VM_ID,
      cpus: 2,
      memoryMb: 2048,
      hostCpus: 8,
      hostMemoryMb: 16384,
      editable: true,
    });
  });

  afterEach(() => cleanup());

  it('renders configured values bounded by the host limits', async () => {
    const { getByLabelText } = render(<HardwarePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmHardware).toHaveBeenCalledWith(VM_ID));

    const cpu = getByLabelText('vCPU') as HTMLInputElement;
    const mem = getByLabelText('Memory (MB)') as HTMLInputElement;
    expect(cpu.value).toBe('2');
    expect(mem.value).toBe('2048');
    expect(cpu.max).toBe('8');
    expect(mem.max).toBe('16384');
  });

  it('applies a change and notifies the parent', async () => {
    vi.mocked(api.setVmHardware).mockResolvedValue({
      success: true,
      vmId: VM_ID,
      message: 'Hardware updated: 4 vCPU, 4096 MB memory.',
    });
    const onChanged = vi.fn();

    const { getByLabelText, getByRole } = render(<HardwarePanel vmId={VM_ID} onChanged={onChanged} />);
    await waitFor(() => expect(api.getVmHardware).toHaveBeenCalled());

    fireEvent.change(getByLabelText('vCPU'), { target: { value: '4' } });
    fireEvent.change(getByLabelText('Memory (MB)'), { target: { value: '4096' } });
    fireEvent.click(getByRole('button', { name: 'Apply' }));

    await waitFor(() => expect(api.setVmHardware).toHaveBeenCalledWith(VM_ID, 4, 4096));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('keeps Apply disabled until a value changes', async () => {
    const { getByRole } = render(<HardwarePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmHardware).toHaveBeenCalled());

    expect((getByRole('button', { name: 'Apply' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('renders read-only while the VM is running', async () => {
    vi.mocked(api.getVmHardware).mockResolvedValue({
      id: VM_ID,
      cpus: 2,
      memoryMb: 2048,
      hostCpus: 8,
      hostMemoryMb: 16384,
      editable: false,
    });

    const { getByLabelText, getByText } = render(<HardwarePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmHardware).toHaveBeenCalled());

    expect((getByLabelText('vCPU') as HTMLInputElement).disabled).toBe(true);
    expect((getByLabelText('Memory (MB)') as HTMLInputElement).disabled).toBe(true);
    expect(getByText('Power off the VM to change hardware.')).toBeTruthy();
  });

  it('blocks Apply on out-of-range values', async () => {
    const { getByLabelText, getByRole } = render(<HardwarePanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmHardware).toHaveBeenCalled());

    fireEvent.change(getByLabelText('Memory (MB)'), { target: { value: '64' } });
    expect((getByRole('button', { name: 'Apply' }) as HTMLButtonElement).disabled).toBe(true);
  });
});
