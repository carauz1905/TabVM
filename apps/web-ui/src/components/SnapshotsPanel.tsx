import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import type { Snapshot } from '../types/api';
import { useT } from '../i18n/i18n';

interface SnapshotsPanelProps {
  vmId: string;
  vmName?: string;
  // onChanged lets the parent refresh the VM list after a restore, which powers
  // the VM off and so changes its state.
  onChanged?: () => void;
}

// defaultSnapshotName produces a timestamped name so a snapshot can be taken in
// one click without typing.
function defaultSnapshotName(): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  return `Snapshot ${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// SnapshotsPanel manages a VM's restore points: take a snapshot, roll back to
// one (which powers the VM off), or delete one. It lives in the VM focus
// section next to Files.
export function SnapshotsPanel({ vmId, vmName, onChanged }: SnapshotsPanelProps) {
  const { t, tf, ts } = useT();
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [name, setName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const response = await api.getSnapshots(vmId);
      setSnapshots(response.snapshots);
    } catch {
      setSnapshots([]);
    }
  }, [vmId]);

  useEffect(() => {
    void load();
  }, [load]);

  function messageFor(err: unknown): string {
    if (err instanceof ApiError && err.body.trim() !== '') return err.body.trim();
    return err instanceof Error ? err.message : String(err);
  }

  const take = useCallback(async () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const res = await api.takeSnapshot(vmId, name.trim() || defaultSnapshotName(), '');
      setName('');
      setNotice(res.message);
      await load();
    } catch (err) {
      setError(messageFor(err));
    } finally {
      setBusy(false);
    }
  }, [busy, name, vmId, load]);

  const restore = useCallback(
    async (snap: Snapshot) => {
      if (busy) return;
      if (
        !window.confirm(
          tf('Restore "{name}"? This powers off {vm} and rolls it back — anything not captured in a snapshot is lost.', {
            name: snap.name,
            vm: vmName ?? t('the VM'),
          }),
        )
      )
        return;
      setBusy(true);
      setError(null);
      setNotice(null);
      try {
        const res = await api.restoreSnapshot(vmId, snap.uuid);
        setNotice(res.message);
        await load();
        onChanged?.();
      } catch (err) {
        setError(messageFor(err));
      } finally {
        setBusy(false);
      }
    },
    [busy, vmId, vmName, load, onChanged, t, tf],
  );

  const remove = useCallback(
    async (snap: Snapshot) => {
      if (busy) return;
      if (
        !window.confirm(
          tf('Delete snapshot "{name}"? Its changes merge into the parent and it cannot be recovered.', {
            name: snap.name,
          }),
        )
      )
        return;
      setBusy(true);
      setError(null);
      setNotice(null);
      try {
        const res = await api.deleteSnapshot(vmId, snap.uuid);
        setNotice(res.message);
        await load();
      } catch (err) {
        setError(messageFor(err));
      } finally {
        setBusy(false);
      }
    },
    [busy, vmId, load, tf],
  );

  return (
    <section className="snap-panel" aria-label={t('Snapshots')}>
      <div className="snap-h">
        <h3>{t('Snapshots')}</h3>
        <span className="sub">{t('restore points')}</span>
      </div>

      {snapshots.length === 0 ? (
        <div className="snap-empty">
          <p>{t('No snapshots yet. Take one before you experiment, then roll back anytime.')}</p>
        </div>
      ) : (
        <ul className="snap-list">
          {snapshots.map((snap) => (
            <li
              className={`snap-row ${snap.current ? 'is-current' : ''}`}
              key={snap.uuid}
              style={{ '--depth': snap.depth } as React.CSSProperties}
            >
              <span className="snap-node" aria-hidden="true">
                <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="13" r="4" />
                  <path d="M4 8a2 2 0 0 1 2-2h1.5l1-1.5h5L16 6h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2z" />
                </svg>
              </span>
              <div className="body">
                <div className="r1">
                  <span className="snap-name">{snap.name}</span>
                  {snap.current && <span className="badge current">{t('current')}</span>}
                </div>
                {snap.description && <span className="snap-desc">{snap.description}</span>}
              </div>
              <div className="snap-actions">
                <button
                  type="button"
                  className="snap-btn"
                  onClick={() => void restore(snap)}
                  disabled={busy}
                  title={t('Power off and roll the VM back to this snapshot')}
                >
                  {t('restore')}
                </button>
                <button
                  type="button"
                  className="snap-btn danger"
                  onClick={() => void remove(snap)}
                  disabled={busy}
                  aria-label={`${t('Delete this snapshot')} ${snap.name}`}
                  title={t('Delete this snapshot')}
                >
                  <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M4 7h16M9 7V5h6v2M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12" />
                  </svg>
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

      <div className="snap-take">
        <input
          className="snap-input"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void take();
          }}
          placeholder={t('Snapshot name (optional)')}
          aria-label={t('Snapshot name (optional)')}
          disabled={busy}
        />
        <button type="button" className="snap-add" onClick={() => void take()} disabled={busy}>
          <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M12 5v14M5 12h14" />
          </svg>
          {busy ? t('Working…') : t('Take snapshot')}
        </button>
      </div>

      {notice && <div className="snap-notice">{ts(notice)}</div>}
      {error && <div className="files-error">{ts(error)}</div>}
    </section>
  );
}
