import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import type { UsbDevice, VmUsbResponse } from '../types/api';
import { useT } from '../i18n/i18n';

interface UsbPanelProps {
  vmId: string;
  // onChanged lets the parent refresh after an attach/detach (telemetry and
  // other panels do not depend on USB today, but keep the same contract as the
  // sibling panels).
  onChanged?: () => void;
}

// Canonical VirtualBox capture states, matched case-insensitively against the
// raw VBoxManage string. label is the dictionary key rendered (and translated);
// hint is the tooltip explaining what the state means for the user. Unknown
// states render the raw string with no tooltip.
const USB_STATES: Record<string, { label: string; hint: string }> = {
  available: { label: 'Available', hint: 'Free to attach' },
  busy: { label: 'Busy', hint: 'In use by the host or another program' },
  captured: { label: 'Captured', hint: 'Attached to a virtual machine' },
  unavailable: { label: 'Unavailable', hint: 'Cannot be captured' },
};

// deviceLabel builds a human name from the manufacturer and product, falling
// back to a generic label when the host reported neither.
function deviceLabel(device: UsbDevice, fallback: string): string {
  const name = [device.manufacturer, device.product].filter(Boolean).join(' ').trim();
  return name !== '' ? name : fallback;
}

// UsbPanel lists the host's USB devices for a running VM and attaches or detaches
// each one live. It surfaces the two prerequisites explicitly: the Oracle
// Extension Pack (needed for USB 2.0/3.0 passthrough) and a VM USB controller,
// which cannot be enabled while the VM is running.
export function UsbPanel({ vmId, onChanged }: UsbPanelProps) {
  const { t, ts } = useT();
  const [data, setData] = useState<VmUsbResponse | null>(null);
  const [busyUuid, setBusyUuid] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      const res = await api.getVmUsb(vmId);
      setData(res);
    } catch (err) {
      setData(null);
      setLoadError(messageFor(err));
    }
  }, [vmId]);

  useEffect(() => {
    void load();
  }, [load]);

  function messageFor(err: unknown): string {
    if (err instanceof ApiError && err.body.trim() !== '') return err.body.trim();
    return err instanceof Error ? err.message : String(err);
  }

  const run = useCallback(
    async (device: UsbDevice, action: (id: string, uuid: string) => Promise<{ message: string }>) => {
      setBusyUuid(device.uuid);
      setError(null);
      setNotice(null);
      try {
        const res = await action(vmId, device.uuid);
        setNotice(res.message);
        await load();
        onChanged?.();
      } catch (err) {
        setError(messageFor(err));
      } finally {
        setBusyUuid(null);
      }
    },
    [vmId, load, onChanged],
  );

  const devices = data?.devices ?? [];
  const controllerMissing = data ? !data.usbControllerEnabled : false;
  const extensionMissing = data ? !data.extensionPackInstalled : false;
  // Without a USB controller every attach is guaranteed to fail with a 400, so
  // the buttons are disabled and carry this explanation as their tooltip. A
  // missing Extension Pack must NOT block attach: USB 1.1 (OHCI) passthrough
  // works without it, so that notice stays a soft warning only.
  const controllerNotice = t(
    'This VM has no USB controller enabled. Power the VM off and enable USB in its settings — the controller cannot be turned on while the VM is running.',
  );

  return (
    <section className="net-panel usb-panel" aria-label="USB">
      <div className="net-h">
        <h3>{t('USB')}</h3>
        <span className="sub">{t('device passthrough')}</span>
      </div>

      {controllerMissing && <div className="usb-notice">{controllerNotice}</div>}
      {extensionMissing && (
        <div className="usb-notice">
          {t(
            'USB 2.0 and 3.0 passthrough needs the Oracle VirtualBox Extension Pack. Install it to attach most devices.',
          )}
        </div>
      )}

      {loadError ? (
        <div className="files-error">{ts(loadError)}</div>
      ) : devices.length === 0 ? (
        <div className="net-empty">
          <p>{t('No USB devices detected on the host.')}</p>
        </div>
      ) : (
        <ul className="net-list">
          {devices.map((device) => {
            const busy = busyUuid === device.uuid;
            const label = deviceLabel(device, t('USB device'));
            const knownState = USB_STATES[device.state.trim().toLowerCase()];
            return (
              <li className="net-row" key={device.uuid}>
                <div className="net-info">
                  <span className="net-slot">{label}</span>
                  <span className="net-mac">
                    {device.vendorId}:{device.productId}
                  </span>
                  <span className="net-current" title={knownState ? t(knownState.hint) : undefined}>
                    {knownState ? t(knownState.label) : device.state}
                  </span>
                </div>
                <div className="net-controls">
                  {device.attachedHere ? (
                    <button
                      type="button"
                      className="net-apply"
                      aria-label={`${t('Detach')} ${label}`}
                      disabled={busy}
                      onClick={() => void run(device, api.detachUsb)}
                    >
                      {busy ? t('Working…') : t('Detach')}
                    </button>
                  ) : (
                    <button
                      type="button"
                      className="net-apply"
                      aria-label={`${t('Attach')} ${label}`}
                      disabled={busy || controllerMissing}
                      title={controllerMissing ? controllerNotice : undefined}
                      onClick={() => void run(device, api.attachUsb)}
                    >
                      {busy ? t('Working…') : t('Attach')}
                    </button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {notice && <div className="net-notice">{ts(notice)}</div>}
      {error && <div className="files-error">{ts(error)}</div>}
    </section>
  );
}
