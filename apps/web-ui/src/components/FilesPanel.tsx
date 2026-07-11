import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import { deriveShareName, guestMountPath } from '../lib/shareName';
import type { SharedFolder } from '../types/api';
import { GuestDropZone } from './GuestDropZone';
import { useT } from '../i18n/i18n';

interface FilesPanelProps {
  vmId: string;
  vmName?: string;
}

// FilesPanel is the redesigned host↔guest file-sharing surface. Instead of two
// naked text inputs, the user clicks "Add folder", the host agent opens the
// native OS folder picker, and the chosen absolute path becomes a VirtualBox
// shared folder with an auto-derived name. Running VMs only take transient
// ("session") shares; stopped VMs take persistent ones — the badge reflects
// whichever VBox actually created.
export function FilesPanel({ vmId, vmName }: FilesPanelProps) {
  const { t, ts } = useT();
  const [folders, setFolders] = useState<SharedFolder[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [picking, setPicking] = useState(false);

  const load = useCallback(async () => {
    try {
      const response = await api.getSharedFolders(vmId);
      setFolders(response.folders);
    } catch {
      // Sharing is auxiliary; a failed list should not break the focus view.
      setFolders([]);
    }
  }, [vmId]);

  // Re-read the share list on an interval, not just on mount. VBox's view can
  // change without any UI action — a transient share appears once its auto-mount
  // is registered after the guest boots, and it drops when the VM powers off —
  // so a one-shot load would leave the panel stale. Polling keeps it in sync
  // with what VirtualBox actually reports.
  useEffect(() => {
    void load();
    const interval = setInterval(() => void load(), 5000);
    return () => clearInterval(interval);
  }, [load]);

  function messageFor(err: unknown): string {
    if (err instanceof ApiError && err.body.trim() !== '') return err.body.trim();
    return err instanceof Error ? err.message : String(err);
  }

  const handleAdd = useCallback(async () => {
    if (busy || picking) return;
    setError(null);
    setPicking(true);
    try {
      const picked = await api.pickHostFolder();
      if (picked.cancelled || picked.path.trim() === '') return; // user dismissed the dialog
      setBusy(true);
      const name = deriveShareName(
        picked.path,
        folders.map((f) => f.name),
      );
      await api.addSharedFolder(vmId, name, picked.path);
      await load();
    } catch (err) {
      setError(messageFor(err));
    } finally {
      setPicking(false);
      setBusy(false);
    }
  }, [busy, picking, folders, vmId, load]);

  const handleRemove = useCallback(
    async (name: string) => {
      if (busy) return;
      setBusy(true);
      setError(null);
      try {
        await api.removeSharedFolder(vmId, name);
        await load();
      } catch (err) {
        setError(messageFor(err));
      } finally {
        setBusy(false);
      }
    },
    [busy, vmId, load],
  );

  const working = busy || picking;

  return (
    <GuestDropZone vmId={vmId} vmName={vmName} className="files-dropzone" overlayLabel={t('Drop files to send to the VM')}>
      <section className="files-panel" aria-label={t('Files')}>
        <div className="files-h">
          <h3>{t('Files')}</h3>
          <span className="sub">{t('shared folders')}</span>
        </div>

        {folders.length === 0 ? (
        <div className="files-empty">
          <span className="glyph" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 7a2 2 0 0 1 2-2h4l2 2h6a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
              <path d="M12 11v5M9.5 13.5h5" />
            </svg>
          </span>
          <p>{t('No folders shared yet. Pick a host folder to make it appear inside this VM.')}</p>
        </div>
      ) : (
        <ul className="files-list">
          {folders.map((folder) => (
            <li className="files-row" key={`${folder.name}-${folder.transient}`}>
              <span className="ico" aria-hidden="true">
                <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M3 7a2 2 0 0 1 2-2h4l2 2h6a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
                </svg>
              </span>
              <div className="body">
                <div className="r1">
                  <span className="fname">{folder.name}</span>
                  {folder.transient ? (
                    <span className="badge session" title={t('Shared only until this VM reboots (the VM is running)')}>
                      {t('session')}
                    </span>
                  ) : (
                    <span className="badge persist" title={t('Persistent share; survives reboots')}>
                      {t('persistent')}
                    </span>
                  )}
                </div>
                <span className="path" title={folder.hostPath}>
                  {folder.hostPath}
                </span>
                <span className="mount">
                  guest → <b>{guestMountPath(folder.name)}</b>
                </span>
              </div>
              <button
                type="button"
                className="rm"
                onClick={() => void handleRemove(folder.name)}
                disabled={busy}
                aria-label={`${t('Remove')} ${folder.name}`}
                title={t('Remove')}
              >
                <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M6 6l12 12M18 6L6 18" />
                </svg>
              </button>
            </li>
          ))}
        </ul>
      )}

      <button type="button" className="files-add" onClick={() => void handleAdd()} disabled={working}>
        <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M12 5v14M5 12h14" />
        </svg>
        {picking ? t('Choose a folder…') : busy ? t('Working…') : t('Add folder')}
      </button>

      {error && <div className="files-error">{ts(error)}</div>}
      </section>
    </GuestDropZone>
  );
}
