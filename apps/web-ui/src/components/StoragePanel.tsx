import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import type { DiskInfo, VmStorageResponse } from '../types/api';
import { useT } from '../i18n/i18n';

interface StoragePanelProps {
  vmId: string;
  // onChanged lets the parent refresh telemetry after a disk resize.
  onChanged?: () => void;
}

function formatGb(mb: number): string {
  return (mb / 1024).toFixed(mb % 1024 === 0 ? 0 : 1);
}

// StoragePanel lists a VM's disks and grows the resizable ones. VirtualBox can
// only enlarge a disk (never shrink) and only while the VM is off, so the input
// is bounded to grow-only and the whole panel goes read-only for a live VM.
export function StoragePanel({ vmId, onChanged }: StoragePanelProps) {
  const { t, tf, ts } = useT();
  const [storage, setStorage] = useState<VmStorageResponse | null>(null);
  const [sizes, setSizes] = useState<Record<string, string>>({});
  const [busyUuid, setBusyUuid] = useState<string | null>(null);
  const [addSize, setAddSize] = useState('');
  const [adding, setAdding] = useState(false);
  const [dvdBusy, setDvdBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    // Focusing a VM fires several panels at once, each hitting VBoxManage; under
    // that concurrent load the COM layer can transiently fail, so retry.
    for (let attempt = 0; attempt < 4; attempt += 1) {
      try {
        const response = await api.getVmStorage(vmId);
        setStorage(response);
        setSizes({});
        return;
      } catch {
        if (attempt < 3) await new Promise((r) => setTimeout(r, 300 * (attempt + 1)));
      }
    }
    setStorage(null);
  }, [vmId]);

  useEffect(() => {
    void load();
  }, [load]);

  const resize = useCallback(
    async (disk: DiskInfo, sizeMb: number) => {
      setBusyUuid(disk.uuid);
      setError(null);
      setNotice(null);
      try {
        const res = await api.resizeDisk(vmId, disk.uuid, sizeMb);
        setNotice(res.message);
        await load();
        onChanged?.();
      } catch (err) {
        if (err instanceof ApiError && err.body.trim() !== '') setError(err.body.trim());
        else setError(err instanceof Error ? err.message : String(err));
      } finally {
        setBusyUuid(null);
      }
    },
    [vmId, load, onChanged],
  );

  const addDisk = useCallback(async () => {
    const gb = Number.parseInt(addSize, 10);
    if (!Number.isInteger(gb) || gb < 1) return;
    setAdding(true);
    setError(null);
    setNotice(null);
    try {
      const res = await api.addDisk(vmId, gb * 1024);
      setNotice(res.message);
      setAddSize('');
      await load();
      onChanged?.();
    } catch (err) {
      if (err instanceof ApiError && err.body.trim() !== '') setError(err.body.trim());
      else setError(err instanceof Error ? err.message : String(err));
    } finally {
      setAdding(false);
    }
  }, [vmId, addSize, load, onChanged]);

  const detach = useCallback(
    async (disk: DiskInfo, deleteFile: boolean) => {
      const question = deleteFile
        ? tf(
            'Permanently delete "{name}"? Its disk image file will be removed from this computer. This cannot be undone.',
            { name: disk.name },
          )
        : tf('Detach "{name}" from this VM? The disk file is kept and can be re-attached later.', {
            name: disk.name,
          });
      if (!window.confirm(question)) return;

      setBusyUuid(disk.uuid);
      setError(null);
      setNotice(null);
      try {
        const res = await api.detachDisk(vmId, disk.uuid, deleteFile);
        setNotice(res.message);
        await load();
        onChanged?.();
      } catch (err) {
        if (err instanceof ApiError && err.body.trim() !== '') setError(err.body.trim());
        else setError(err instanceof Error ? err.message : String(err));
      } finally {
        setBusyUuid(null);
      }
    },
    [vmId, tf, load, onChanged],
  );

  // pickAndMount opens the host ISO picker and, unless the dialog is cancelled,
  // mounts the chosen ISO into the VM's optical drive. It doubles as "Change" —
  // mounting a new ISO replaces whatever is in the drive.
  const pickAndMount = useCallback(async () => {
    setDvdBusy(true);
    setError(null);
    setNotice(null);
    try {
      const picked = await api.pickHostFile();
      if (picked.cancelled || picked.path === '') return;
      const res = await api.mountDvd(vmId, picked.path);
      setNotice(res.message);
      await load();
      onChanged?.();
    } catch (err) {
      if (err instanceof ApiError && err.body.trim() !== '') setError(err.body.trim());
      else setError(err instanceof Error ? err.message : String(err));
    } finally {
      setDvdBusy(false);
    }
  }, [vmId, load, onChanged]);

  const eject = useCallback(async () => {
    setDvdBusy(true);
    setError(null);
    setNotice(null);
    try {
      const res = await api.ejectDvd(vmId);
      setNotice(res.message);
      await load();
      onChanged?.();
    } catch (err) {
      if (err instanceof ApiError && err.body.trim() !== '') setError(err.body.trim());
      else setError(err instanceof Error ? err.message : String(err));
    } finally {
      setDvdBusy(false);
    }
  }, [vmId, load, onChanged]);

  const disks = storage?.disks ?? [];
  const editable = storage?.editable ?? false;
  const optical = storage?.optical ?? {
    present: false,
    medium: '',
    name: '',
    controller: '',
    port: 0,
    device: 0,
  };
  const hasDisc = optical.present && optical.medium !== '';
  const addGb = Number.parseInt(addSize, 10);
  const validAdd = Number.isInteger(addGb) && addGb >= 1;

  return (
    <section className="net-panel" aria-label="Storage">
      <div className="net-h">
        <h3>{t('Storage')}</h3>
        <span className="sub">{t('disk size')}</span>
      </div>

      {storage === null ? (
        <div className="net-empty">
          <p>{t('Storage information unavailable.')}</p>
        </div>
      ) : disks.length === 0 ? (
        <div className="net-empty">
          <p>{t('No hard disks attached to this VM.')}</p>
        </div>
      ) : (
        <ul className="net-list">
          {disks.map((disk) => {
            const currentGb = disk.capacityMb / 1024;
            const raw = sizes[disk.uuid] ?? '';
            const nextGb = Number.parseFloat(raw);
            const validGrow = Number.isFinite(nextGb) && nextGb > currentGb;
            const busy = busyUuid === disk.uuid;

            return (
              <li className="net-row" key={disk.uuid}>
                <div className="net-info">
                  <span className="net-slot">{disk.name}</span>
                  <span className="net-current">
                    {disk.format} · {formatGb(disk.capacityMb)} GB
                  </span>
                </div>

                <div className="net-controls">
                  {disk.resizable ? (
                    <>
                      <label className="net-field">
                        <span>{t('New size (GB)')}</span>
                        <input
                          type="number"
                          className="net-select"
                          aria-label={t('New size (GB)')}
                          min={Math.ceil(currentGb) + 1}
                          step={1}
                          value={raw}
                          placeholder={String(Math.ceil(currentGb) + 1)}
                          disabled={busy}
                          onChange={(e) => setSizes((s) => ({ ...s, [disk.uuid]: e.target.value }))}
                        />
                      </label>
                      <button
                        type="button"
                        className="net-apply"
                        onClick={() => void resize(disk, Math.round(nextGb * 1024))}
                        disabled={busy || !validGrow}
                      >
                        {busy ? t('Resizing…') : t('Resize')}
                      </button>
                    </>
                  ) : (
                    <span className="net-hint">{ts(disk.reason || t('This disk cannot be resized.'))}</span>
                  )}
                  {editable && (
                    <>
                      <button
                        type="button"
                        className="net-apply quiet"
                        aria-label={tf('Detach {name}', { name: disk.name })}
                        onClick={() => void detach(disk, false)}
                        disabled={busy}
                      >
                        {t('Detach')}
                      </button>
                      <button
                        type="button"
                        className="net-apply danger"
                        aria-label={tf('Delete {name}', { name: disk.name })}
                        onClick={() => void detach(disk, true)}
                        disabled={busy}
                      >
                        {t('Delete')}
                      </button>
                    </>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {storage !== null && (
        <div className="net-row" aria-label={t('DVD drive')}>
          <div className="net-info">
            <span className="net-slot">{t('DVD drive')}</span>
            <span className="net-current">
              {hasDisc ? optical.name : optical.present ? t('empty') : t('no optical drive')}
            </span>
          </div>
          <div className="net-controls">
            <button
              type="button"
              className="net-apply"
              onClick={() => void pickAndMount()}
              disabled={dvdBusy}
            >
              {dvdBusy ? t('Working…') : hasDisc ? t('Change ISO') : t('Mount ISO')}
            </button>
            {hasDisc && (
              <button
                type="button"
                className="net-apply quiet"
                onClick={() => void eject()}
                disabled={dvdBusy}
              >
                {t('Eject')}
              </button>
            )}
          </div>
        </div>
      )}

      {storage !== null && editable && (
        <div className="net-row net-add">
          <div className="net-info">
            <span className="net-slot">{t('Add a disk')}</span>
            <span className="net-current">{t('a new VDI attached to a free SATA port')}</span>
          </div>
          <div className="net-controls">
            <label className="net-field">
              <span>{t('New disk size (GB)')}</span>
              <input
                type="number"
                className="net-select"
                aria-label={t('New disk size (GB)')}
                min={1}
                step={1}
                value={addSize}
                placeholder="10"
                disabled={adding}
                onChange={(e) => setAddSize(e.target.value)}
              />
            </label>
            <button
              type="button"
              className="net-apply"
              onClick={() => void addDisk()}
              disabled={adding || !validAdd}
            >
              {adding ? t('Adding…') : t('Add disk')}
            </button>
          </div>
        </div>
      )}

      {disks.length > 0 && (
        <p className="docs-tip net-foot">
          {t('Resizing only enlarges the virtual disk. Expand the partition inside the guest OS to use the new space.')}
        </p>
      )}

      {notice && <div className="net-notice">{ts(notice)}</div>}
      {error && <div className="files-error">{ts(error)}</div>}
    </section>
  );
}
