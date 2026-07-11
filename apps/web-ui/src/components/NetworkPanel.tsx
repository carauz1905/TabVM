import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import type { NetworkAdapter, NetworkOptionsResponse } from '../types/api';
import { useT } from '../i18n/i18n';

interface NetworkPanelProps {
  vmId: string;
  // onChanged lets the parent refresh telemetry after a mode change (the guest
  // IP may change with the network).
  onChanged?: () => void;
}

// The three modes TabVM can switch between (matching the agent's allow-list).
const MODES: { value: string; label: string }[] = [
  { value: 'nat', label: 'NAT' },
  { value: 'bridged', label: 'Bridged' },
  { value: 'hostonly', label: 'Host-only' },
];

function modeLabel(mode: string): string {
  return MODES.find((m) => m.value === mode)?.label ?? mode;
}

// NetworkPanel switches a VM's NIC attachment (NAT / bridged / host-only) from
// the browser. Bridged and host-only require picking a host interface, which is
// populated from what the host actually offers. On a running VM the change is
// applied live; on a stopped VM it is written to the config.
export function NetworkPanel({ vmId, onChanged }: NetworkPanelProps) {
  const { t, ts } = useT();
  const [options, setOptions] = useState<NetworkOptionsResponse | null>(null);
  const [pending, setPending] = useState<Record<number, { mode: string; adapter: string }>>({});
  const [busySlot, setBusySlot] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const response = await api.getNetworkOptions(vmId);
      setOptions(response);
      setPending({});
    } catch {
      setOptions(null);
    }
  }, [vmId]);

  useEffect(() => {
    void load();
  }, [load]);

  function messageFor(err: unknown): string {
    if (err instanceof ApiError && err.body.trim() !== '') return err.body.trim();
    return err instanceof Error ? err.message : String(err);
  }

  function adaptersFor(mode: string): string[] {
    if (mode === 'bridged') return options?.bridgedAdapters ?? [];
    if (mode === 'hostonly') return options?.hostOnlyAdapters ?? [];
    return [];
  }

  function pendingFor(nic: NetworkAdapter): { mode: string; adapter: string } {
    return pending[nic.slot] ?? { mode: nic.mode, adapter: nic.adapter ?? '' };
  }

  // modeOptions includes the current mode even if it is outside the switchable
  // set (e.g. intnet), so the select never shows a blank value.
  function modeOptions(nic: NetworkAdapter): { value: string; label: string }[] {
    if (MODES.some((m) => m.value === nic.mode)) return MODES;
    return [{ value: nic.mode, label: `${nic.mode} (current)` }, ...MODES];
  }

  function changeMode(nic: NetworkAdapter, mode: string) {
    const list = adaptersFor(mode);
    const adapter = mode === 'bridged' || mode === 'hostonly' ? list[0] ?? '' : '';
    setPending((p) => ({ ...p, [nic.slot]: { mode, adapter } }));
  }

  function changeAdapter(nic: NetworkAdapter, adapter: string) {
    setPending((p) => ({ ...p, [nic.slot]: { mode: pendingFor(nic).mode, adapter } }));
  }

  const apply = useCallback(
    async (nic: NetworkAdapter, mode: string, adapter: string) => {
      setBusySlot(nic.slot);
      setError(null);
      setNotice(null);
      try {
        const res = await api.changeNetworkMode(vmId, nic.slot, mode, adapter);
        setNotice(res.message);
        await load();
        onChanged?.();
      } catch (err) {
        setError(messageFor(err));
      } finally {
        setBusySlot(null);
      }
    },
    [vmId, load, onChanged],
  );

  const adapters = options?.adapters ?? [];

  return (
    <section className="net-panel" aria-label="Network">
      <div className="net-h">
        <h3>{t('Network')}</h3>
        <span className="sub">{t('adapter mode')}</span>
      </div>

      {adapters.length === 0 ? (
        <div className="net-empty">
          <p>{t('No enabled network adapters on this VM.')}</p>
        </div>
      ) : (
        <ul className="net-list">
          {adapters.map((nic) => {
            const sel = pendingFor(nic);
            const needsAdapter = sel.mode === 'bridged' || sel.mode === 'hostonly';
            const available = adaptersFor(sel.mode);
            const noAdapter = needsAdapter && available.length === 0;
            const changed = sel.mode !== nic.mode || sel.adapter !== (nic.adapter ?? '');
            const busy = busySlot === nic.slot;

            return (
              <li className="net-row" key={nic.slot}>
                <div className="net-info">
                  <span className="net-slot">{t('Adapter')} {nic.slot}</span>
                  {nic.mac && <span className="net-mac">{nic.mac}</span>}
                  <span className="net-current">
                    {t('now')}: {t(modeLabel(nic.mode))}
                    {nic.adapter ? ` · ${nic.adapter}` : ''}
                  </span>
                </div>

                <div className="net-controls">
                  <select
                    className="net-select"
                    aria-label={`${t('Adapter')} ${nic.slot}`}
                    value={sel.mode}
                    disabled={busy}
                    onChange={(e) => changeMode(nic, e.target.value)}
                  >
                    {modeOptions(nic).map((m) => (
                      <option key={m.value} value={m.value}>
                        {t(m.label)}
                      </option>
                    ))}
                  </select>

                  {needsAdapter &&
                    (noAdapter ? (
                      <span className="net-hint">
                        {sel.mode === 'bridged'
                          ? t('No bridge-able host interface found.')
                          : t('No host-only adapter — create one in VirtualBox first.')}
                      </span>
                    ) : (
                      <select
                        className="net-select grow"
                        aria-label={`${t('Adapter')} ${nic.slot}`}
                        value={sel.adapter}
                        disabled={busy}
                        onChange={(e) => changeAdapter(nic, e.target.value)}
                      >
                        {available.map((name) => (
                          <option key={name} value={name}>
                            {name}
                          </option>
                        ))}
                      </select>
                    ))}

                  <button
                    type="button"
                    className="net-apply"
                    onClick={() => void apply(nic, sel.mode, sel.adapter)}
                    disabled={busy || !changed || noAdapter}
                  >
                    {busy ? t('Applying…') : t('Apply')}
                  </button>
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
