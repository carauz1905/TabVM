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

// usbStateLabel maps VirtualBox capture states to the keys translated in the
// dictionary. Unknown states pass through unchanged.
function usbStateLabel(state: string): string {
  switch (state) {
    case 'Available':
    case 'Busy':
    case 'Captured':
    case 'Unavailable':
      return state;
    default:
      return state;
  }
}

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

  return (
    <section className="net-panel usb-panel" aria-label="USB">
      <div className="net-h">
        <h3>{t('USB')}</h3>
        <span className="sub">{t('device passthrough')}</span>
      </div>

      {controllerMissing && (
        <div className="usb-notice">
          {t(
            'This VM has no USB controller enabled. Power the VM off and enable USB in its settings — the controller cannot be turned on while the VM is running.',
          )}
        </div>
      )}
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
            return (
              <li className="net-row" key={device.uuid}>
                <div className="net-info">
                  <span className="net-slot">{label}</span>
                  <span className="net-mac">
                    {device.vendorId}:{device.productId}
                  </span>
                  <span className="net-current">{t(usbStateLabel(device.state))}</span>
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
                      disabled={busy}
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
