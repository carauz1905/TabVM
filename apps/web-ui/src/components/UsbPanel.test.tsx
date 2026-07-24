import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { UsbPanel } from './UsbPanel';
import { api, ApiError } from '../api/client';
import { LanguageProvider } from '../i18n/i18n';
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

// busyDevice builds a device whose state exercises the tooltip mapping.
function deviceWithState(state: string): UsbDevice {
  return {
    uuid: SANDISK,
    vendorId: '0x0781',
    productId: '0x5567',
    manufacturer: 'SanDisk',
    product: 'Cruzer Blade',
    state,
    attachedHere: false,
  };
}

describe('UsbPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
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

  it('disables attach with an explanatory tooltip when the VM has no USB controller', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({ usbControllerEnabled: false }));

    const { findByRole } = render(<UsbPanel vmId={VM_ID} />);
    const attach = (await findByRole('button', { name: /^Attach/ })) as HTMLButtonElement;

    expect(attach.disabled).toBe(true);
    expect(attach.title).toMatch(/no USB controller/);
  });

  it('keeps attach enabled when only the Extension Pack is missing', async () => {
    // USB 1.1 (OHCI) passthrough works without the Extension Pack, so a missing
    // pack must stay a soft warning and never block the attach action.
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({ extensionPackInstalled: false }));

    const { findByRole } = render(<UsbPanel vmId={VM_ID} />);
    const attach = (await findByRole('button', { name: /^Attach/ })) as HTMLButtonElement;

    expect(attach.disabled).toBe(false);
  });

  it('never disables detach by prerequisites: a captured device must be releasable', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(
      usbResponse({ usbControllerEnabled: false, extensionPackInstalled: false }, [
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

    const { findByRole } = render(<UsbPanel vmId={VM_ID} />);
    const detach = (await findByRole('button', { name: /^Detach/ })) as HTMLButtonElement;

    expect(detach.disabled).toBe(false);
  });

  it('renders an empty state when the host has no USB devices', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({}, []));

    const { findByText } = render(<UsbPanel vmId={VM_ID} />);
    expect(await findByText('No USB devices detected on the host.')).toBeTruthy();
  });

  it('shows an explanatory tooltip for the Busy state', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({}, [deviceWithState('Busy')]));

    const { findByText } = render(<UsbPanel vmId={VM_ID} />);
    const state = await findByText('Busy');

    expect(state.getAttribute('title')).toBe('In use by the host or another program');
  });

  it('translates the state label and its tooltip in Spanish', async () => {
    localStorage.setItem('tabvm.lang', 'es');
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({}, [deviceWithState('Busy')]));

    const { findByText } = render(
      <LanguageProvider>
        <UsbPanel vmId={VM_ID} />
      </LanguageProvider>,
    );
    const state = await findByText('ocupado');

    expect(state.getAttribute('title')).toBe('En uso por el anfitrión u otro programa');
  });

  it('matches device states case-insensitively', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({}, [deviceWithState('busy')]));

    const { findByText } = render(<UsbPanel vmId={VM_ID} />);
    const state = await findByText('Busy');

    expect(state.getAttribute('title')).toBe('In use by the host or another program');
  });

  it('falls back to the raw string with no tooltip for unknown states', async () => {
    vi.mocked(api.getVmUsb).mockResolvedValue(usbResponse({}, [deviceWithState('Flux')]));

    const { findByText } = render(<UsbPanel vmId={VM_ID} />);
    const state = await findByText('Flux');

    expect(state.getAttribute('title')).toBeNull();
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
