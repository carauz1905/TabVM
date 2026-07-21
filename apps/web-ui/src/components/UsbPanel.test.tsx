import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { UsbPanel } from './UsbPanel';
import { api, ApiError } from '../api/client';
import type { UsbDevice, VmUsbResponse } from '../types/api';

const VM_ID = '11111111-1111-1111-1111-111111111111';
const SANDISK = '2b7e1a10-1234-4abc-8def-0123456789ab';

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
      getVmUsb: vi.fn(),
      attachUsb: vi.fn(),
      detachUsb: vi.fn(),
    },
  };
});

function usbResponse(overrides: Partial<VmUsbResponse> = {}, devices?: UsbDevice[]): VmUsbResponse {
  return {
    devices: devices ?? [
      {
        uuid: SANDISK,
        vendorId: '0x0781',
        productId: '0x5567',
        manufacturer: 'SanDisk',
        product: 'Cruzer Blade',
        state: 'Available',
        attachedHere: false,
      },
    ],
    extensionPackInstalled: true,
    usbControllerEnabled: true,
    ...overrides,
  };
}

describe('UsbPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse());
  });

  afterEach(() => cleanup());

  it('lists host USB devices with vendor:product and state', async () => {
    const { container, findByText } = render(<UsbPanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmUsb).toHaveBeenCalledWith(VM_ID));

    expect(await findByText('SanDisk Cruzer Blade')).toBeTruthy();
    const meta = container.querySelector('.net-mac');
    expect(meta?.textContent).toBe('0x0781:0x5567');
    expect(await findByText('Available')).toBeTruthy();
  });

  it('attaches a device that is not attached here', async () => {
    vi.mocked(api.attachUsb).mockResolvedValue({ success: true, vmId: VM_ID, message: 'attached' });
    const onChanged = vi.fn();

    const { getByRole } = render(<UsbPanel vmId={VM_ID} onChanged={onChanged} />);
    await waitFor(() => expect(api.getVmUsb).toHaveBeenCalled());

    fireEvent.click(getByRole('button', { name: /^Attach/ }));

    await waitFor(() => expect(api.attachUsb).toHaveBeenCalledWith(VM_ID, SANDISK));
    // Reloads after the operation and notifies the parent.
    await waitFor(() => expect(api.getVmUsb).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('detaches a device that is attached here', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(
      usbResponse({}, [
        {
          uuid: SANDISK,
          vendorId: '0x0781',
          productId: '0x5567',
          manufacturer: 'SanDisk',
          product: 'Cruzer Blade',
          state: 'Captured',
          attachedHere: true,
        },
      ]),
    );
    vi.mocked(api.detachUsb).mockResolvedValue({ success: true, vmId: VM_ID, message: 'detached' });

    const { getByRole } = render(<UsbPanel vmId={VM_ID} />);
    await waitFor(() => expect(api.getVmUsb).toHaveBeenCalled());

    fireEvent.click(getByRole('button', { name: /^Detach/ }));

    await waitFor(() => expect(api.detachUsb).toHaveBeenCalledWith(VM_ID, SANDISK));
  });

  it('shows a notice when the Extension Pack is missing', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({ extensionPackInstalled: false }));

    const { findByText } = render(<UsbPanel vmId={VM_ID} />);
    expect(await findByText(/Oracle VirtualBox Extension Pack/)).toBeTruthy();
  });

  it('shows a notice when the USB controller is disabled and explains it is locked while running', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({ usbControllerEnabled: false }));

    const { findByText } = render(<UsbPanel vmId={VM_ID} />);
    expect(await findByText(/cannot be turned on while the VM is running/)).toBeTruthy();
  });

  it('renders an empty state when the host has no USB devices', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({}, []));

    const { findByText } = render(<UsbPanel vmId={VM_ID} />);
    expect(await findByText('No USB devices detected on the host.')).toBeTruthy();
  });

  it('surfaces a load error instead of the misleading empty state when enumeration fails', async () => {
    vi.mocked(api.getVmUsb).mockRejectedValue(
      new ApiError({
        status: 500,
        statusText: 'Internal Server Error',
        body: 'USB enumeration failed on the host.',
      }),
    );

    const { findByText, queryByText } = render(<UsbPanel vmId={VM_ID} />);
    expect(await findByText('USB enumeration failed on the host.')).toBeTruthy();
    // The empty state must not appear: a failed enumeration is not "zero devices".
    expect(queryByText('No USB devices detected on the host.')).toBeNull();
  });
});
