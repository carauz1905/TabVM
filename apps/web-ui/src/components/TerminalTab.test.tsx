import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, act, cleanup } from '@testing-library/react';
import { TerminalTab } from './TerminalTab';
import { api } from '../api/client';

const VM_ID = '11111111-1111-1111-1111-111111111111';

vi.mock('./SplashScreen', () => ({ SplashScreen: () => null }));

// xterm.js cannot run under jsdom. The stub also reports the socket as open,
// which is what starts the parent's silence timer — the only path to the
// activation card.
vi.mock('./SerialTerminal', async () => {
  const { useEffect } = await import('react');
  return {
    SerialTerminal: ({ onStatus }: { onStatus?: (status: string) => void }) => {
      useEffect(() => {
        onStatus?.('open');
      }, [onStatus]);
      return <div data-testid="serial-terminal" />;
    },
  };
});

vi.mock('../api/client', () => {
  class ApiError extends Error {}
  return {
    ApiError,
    api: {
      getSerialConsole: vi.fn(),
      enableSerialConsole: vi.fn(),
      enableSerialGetty: vi.fn(),
    },
  };
});

// Drives the tab to the one state where the activation card is on screen: the
// serial port is wired, the VM runs, the socket opens, and the guest sends
// nothing before the silence timeout expires.
async function renderSilentTerminal() {
  const view = render(<TerminalTab vmId={VM_ID} vmName="VM One" />);
  await act(async () => {}); // resolve getSerialConsole
  await act(async () => {
    vi.advanceTimersByTime(3000); // outlast SILENCE_TIMEOUT_MS
  });
  return view;
}

async function submitCredentials(view: Awaited<ReturnType<typeof renderSilentTerminal>>) {
  fireEvent.change(view.getByPlaceholderText('Guest username'), { target: { value: 'root' } });
  fireEvent.change(view.getByPlaceholderText('Guest password'), { target: { value: 'hunter2' } });
  await act(async () => {
    fireEvent.click(view.getByRole('button', { name: 'Activate terminal' }));
  });
}

describe('TerminalTab activation card', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    vi.mocked(api.getSerialConsole).mockResolvedValue({
      id: VM_ID,
      enabled: true,
      terminalCapable: true,
      running: true,
      editable: false,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    cleanup();
  });

  // The agent already captures the guest's stdout+stderr and ships it in the
  // `output` field; only the summary was ever painted. That cost a full session
  // of host-side forensics to recover `systemctl: unrecognized option '--now'`,
  // a string the browser had been handed all along.
  it('shows the guest output when activation fails', async () => {
    vi.mocked(api.enableSerialGetty).mockResolvedValue({
      success: false,
      vmId: VM_ID,
      message: 'Could not enable the serial login inside the guest.',
      output: "systemctl: unrecognized option '--now'",
    });

    const view = await renderSilentTerminal();
    await submitCredentials(view);

    // The summary stays, so the user still gets the plain-language framing...
    expect(view.getByText(/Could not enable the serial login/)).toBeTruthy();
    // ...and the reason the agent already knew is finally on screen.
    expect(view.getByText(/unrecognized option/)).toBeTruthy();
  });

  it('renders no output block when the guest said nothing', async () => {
    vi.mocked(api.enableSerialGetty).mockResolvedValue({
      success: false,
      vmId: VM_ID,
      message: 'Could not enable the serial login inside the guest.',
      output: '',
    });

    const view = await renderSilentTerminal();
    await submitCredentials(view);

    expect(view.getByText(/Could not enable the serial login/)).toBeTruthy();
    expect(view.container.querySelector('pre')).toBeNull();
  });
});
