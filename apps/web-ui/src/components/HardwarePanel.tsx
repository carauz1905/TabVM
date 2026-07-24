import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import type { VmHardwareResponse } from '../types/api';
import { useT } from '../i18n/i18n';

interface HardwarePanelProps {
  vmId: string;
  // running mirrors the focused VM's live state so the panel re-gates the
  // moment the machine starts or stops, without waiting for a refocus. When it
  // changes the hardware is re-fetched, and while true the panel is read-only
  // even if the last response predates the start.
  running?: boolean;
  // onChanged lets the parent refresh telemetry after a hardware change.
  onChanged?: () => void;
}

const MIN_MEMORY_MB = 128;

// HardwarePanel edits a VM's vCPU count and memory size. VirtualBox can only
// change these on a powered-off machine, so a live VM renders read-only. The
// inputs are bounded by what the host actually has.
export function HardwarePanel({ vmId, running, onChanged }: HardwarePanelProps) {
  const { t, ts } = useT();
  const [hardware, setHardware] = useState<VmHardwareResponse | null>(null);
  const [cpus, setCpus] = useState('');
  const [memory, setMemory] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    // Focusing a VM fires several panels at once, each hitting VBoxManage. Under
    // that concurrent load VirtualBox's COM layer can transiently return "object
    // is not ready", so retry a few times before giving up.
    for (let attempt = 0; attempt < 4; attempt += 1) {
      try {
        const response = await api.getVmHardware(vmId);
        setHardware(response);
        setCpus(String(response.cpus));
        setMemory(String(response.memoryMb));
        return;
      } catch {
        if (attempt < 3) await new Promise((r) => setTimeout(r, 300 * (attempt + 1)));
      }
    }
    setHardware(null);
  }, [vmId]);

  // Reload on focus change and whenever the VM's running state flips, so the
  // editable flag always reflects the machine's current power state.
  useEffect(() => {
    void load();
  }, [load, running]);

  const cpuValue = Number.parseInt(cpus, 10);
  const memValue = Number.parseInt(memory, 10);
  const cpuMax = hardware?.hostCpus || undefined;
  const memMax = hardware?.hostMemoryMb || undefined;
  const validCpu = Number.isInteger(cpuValue) && cpuValue >= 1 && (!cpuMax || cpuValue <= cpuMax);
  const validMem =
    Number.isInteger(memValue) && memValue >= MIN_MEMORY_MB && (!memMax || memValue <= memMax);
  const changed =
    hardware !== null && (cpuValue !== hardware.cpus || memValue !== hardware.memoryMb);
  // A live VM is never editable, even while the post-start re-fetch is still
  // in flight and the last response predates the state change.
  const editable = (hardware?.editable ?? false) && running !== true;

  const apply = useCallback(async () => {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const res = await api.setVmHardware(vmId, cpuValue, memValue);
      setNotice(res.message);
      await load();
      onChanged?.();
    } catch (err) {
      if (err instanceof ApiError && err.body.trim() !== '') setError(err.body.trim());
      else setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }, [vmId, cpuValue, memValue, load, onChanged]);

  return (
    <section className="net-panel" aria-label="Hardware">
      <div className="net-h">
        <h3>{t('Hardware')}</h3>
        <span className="sub">{t('vCPU · memory')}</span>
      </div>

      {hardware === null ? (
        <div className="net-empty">
          <p>{t('Hardware information unavailable.')}</p>
        </div>
      ) : (
        <div className="net-row">
          <div className="net-controls">
            <label className="net-field">
              <span>{t('vCPU')}</span>
              <input
                type="number"
                className="net-select"
                aria-label="vCPU"
                min={1}
                max={cpuMax}
                value={cpus}
                disabled={busy || !editable}
                onChange={(e) => setCpus(e.target.value)}
              />
            </label>
            <label className="net-field">
              <span>{t('Memory (MB)')}</span>
              <input
                type="number"
                className="net-select"
                aria-label="Memory (MB)"
                min={MIN_MEMORY_MB}
                max={memMax}
                step={128}
                value={memory}
                disabled={busy || !editable}
                onChange={(e) => setMemory(e.target.value)}
              />
            </label>
            <button
              type="button"
              className="net-apply"
              onClick={() => void apply()}
              disabled={busy || !editable || !changed || !validCpu || !validMem}
            >
              {busy ? t('Applying…') : t('Apply')}
            </button>
          </div>
          {!editable && <span className="net-hint">{t('Power off the VM to change hardware.')}</span>}
        </div>
      )}

      {notice && <div className="net-notice">{ts(notice)}</div>}
      {error && <div className="files-error">{ts(error)}</div>}
    </section>
  );
}
