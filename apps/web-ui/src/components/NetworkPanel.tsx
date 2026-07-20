import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import type { NetworkAdapter, NetworkOptionsResponse, PortForwardingRule } from '../types/api';
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

// A blank add-rule form. Ports are kept as strings while typing and parsed on
// submit; host IP is optional and defaults to 127.0.0.1 on the agent.
interface ForwardForm {
  name: string;
  protocol: string;
  hostIp: string;
  hostPort: string;
  guestPort: string;
}

const EMPTY_FORM: ForwardForm = { name: '', protocol: 'tcp', hostIp: '', hostPort: '', guestPort: '' };

// NetworkPanel switches a VM's NIC attachment (NAT / bridged / host-only) from
// the browser and, for NAT adapters, manages NAT port-forwarding rules. Bridged
// and host-only require picking a host interface, which is populated from what
// the host actually offers. On a running VM changes are applied live; on a
// stopped VM they are written to the config.
export function NetworkPanel({ vmId, onChanged }: NetworkPanelProps) {
  const { t, ts } = useT();
  const [options, setOptions] = useState<NetworkOptionsResponse | null>(null);
  const [pending, setPending] = useState<Record<number, { mode: string; adapter: string }>>({});
  const [busySlot, setBusySlot] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  // Add-rule form state and the slot whose forwarding action is in flight.
  const [forwardForms, setForwardForms] = useState<Record<number, ForwardForm>>({});
  const [busyForward, setBusyForward] = useState<number | null>(null);

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

  function formFor(slot: number): ForwardForm {
    return forwardForms[slot] ?? EMPTY_FORM;
  }

  function setForm(slot: number, patch: Partial<ForwardForm>) {
    setForwardForms((f) => ({ ...f, [slot]: { ...formFor(slot), ...patch } }));
  }

  function formIsValid(form: ForwardForm): boolean {
    const hostPort = Number(form.hostPort);
    const guestPort = Number(form.guestPort);
    return (
      form.name.trim() !== '' &&
      Number.isInteger(hostPort) &&
      hostPort >= 1 &&
      hostPort <= 65535 &&
      Number.isInteger(guestPort) &&
      guestPort >= 1 &&
      guestPort <= 65535
    );
  }

  const addRule = useCallback(
    async (nic: NetworkAdapter) => {
      const form = formFor(nic.slot);
      if (!formIsValid(form)) return;
      setBusyForward(nic.slot);
      setError(null);
      setNotice(null);
      try {
        const res = await api.addPortForwarding(vmId, {
          slot: nic.slot,
          name: form.name.trim(),
          protocol: form.protocol,
          hostIp: form.hostIp.trim(),
          hostPort: Number(form.hostPort),
          guestIp: '',
          guestPort: Number(form.guestPort),
        });
        setNotice(res.message);
        setForwardForms((f) => ({ ...f, [nic.slot]: EMPTY_FORM }));
        await load();
        onChanged?.();
      } catch (err) {
        setError(messageFor(err));
      } finally {
        setBusyForward(null);
      }
    },
    // formFor reads current state via closure; forwardForms is the dependency.
    [vmId, load, onChanged, forwardForms],
  );

  const removeRule = useCallback(
    async (nic: NetworkAdapter, rule: PortForwardingRule) => {
      setBusyForward(nic.slot);
      setError(null);
      setNotice(null);
      try {
        const res = await api.deletePortForwarding(vmId, nic.slot, rule.name);
        setNotice(res.message);
        await load();
        onChanged?.();
      } catch (err) {
        setError(messageFor(err));
      } finally {
        setBusyForward(null);
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
            const isNat = nic.mode === 'nat';
            const rules = nic.forwarding ?? [];
            const form = formFor(nic.slot);
            const fwBusy = busyForward === nic.slot;

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

                {isNat && (
                  <div className="net-fwd">
                    <div className="net-fwd-h">
                      <span className="net-fwd-title">{t('Port forwarding')}</span>
                    </div>

                    {rules.length > 0 && (
                      <ul className="net-fwd-list">
                        {rules.map((rule) => (
                          <li className="net-fwd-rule" key={rule.name}>
                            <span
                              className="net-fwd-map"
                              title={rule.hostIp ? undefined : t('* = bound to all host interfaces')}
                            >
                              {(rule.hostIp || '*')}:{rule.hostPort} → {rule.guestPort}/{rule.protocol}
                            </span>
                            <span className="net-fwd-name">{rule.name}</span>
                            <button
                              type="button"
                              className="net-fwd-del"
                              aria-label={`${t('Remove rule')} ${rule.name}`}
                              disabled={fwBusy}
                              onClick={() => void removeRule(nic, rule)}
                            >
                              {t('Remove')}
                            </button>
                          </li>
                        ))}
                      </ul>
                    )}

                    <div className="net-fwd-form">
                      <input
                        className="net-fwd-input"
                        type="text"
                        aria-label={`${t('Rule name')} (${t('Adapter')} ${nic.slot})`}
                        placeholder={t('name')}
                        value={form.name}
                        disabled={fwBusy}
                        onChange={(e) => setForm(nic.slot, { name: e.target.value })}
                      />
                      <select
                        className="net-fwd-input"
                        aria-label={`${t('Protocol')} (${t('Adapter')} ${nic.slot})`}
                        value={form.protocol}
                        disabled={fwBusy}
                        onChange={(e) => setForm(nic.slot, { protocol: e.target.value })}
                      >
                        <option value="tcp">TCP</option>
                        <option value="udp">UDP</option>
                      </select>
                      <input
                        className="net-fwd-input"
                        type="number"
                        min={1}
                        max={65535}
                        aria-label={`${t('Host port')} (${t('Adapter')} ${nic.slot})`}
                        placeholder={t('host port')}
                        value={form.hostPort}
                        disabled={fwBusy}
                        onChange={(e) => setForm(nic.slot, { hostPort: e.target.value })}
                      />
                      <input
                        className="net-fwd-input"
                        type="number"
                        min={1}
                        max={65535}
                        aria-label={`${t('Guest port')} (${t('Adapter')} ${nic.slot})`}
                        placeholder={t('guest port')}
                        value={form.guestPort}
                        disabled={fwBusy}
                        onChange={(e) => setForm(nic.slot, { guestPort: e.target.value })}
                      />
                      <input
                        className="net-fwd-input"
                        type="text"
                        aria-label={`${t('Host IP (optional)')} (${t('Adapter')} ${nic.slot})`}
                        placeholder="127.0.0.1"
                        value={form.hostIp}
                        disabled={fwBusy}
                        onChange={(e) => setForm(nic.slot, { hostIp: e.target.value })}
                      />
                      <button
                        type="button"
                        className="net-apply"
                        onClick={() => void addRule(nic)}
                        disabled={fwBusy || !formIsValid(form)}
                      >
                        {fwBusy ? t('Applying…') : t('Add rule')}
                      </button>
                    </div>
                  </div>
                )}
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
