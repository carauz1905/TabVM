import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import type { SharedFolder } from '../types/api';
import { useT } from '../i18n/i18n';

interface SharedFoldersProps {
  vmId: string;
}

// SharedFolders manages the host directories shared into a guest. It is the
// browser-side surface for host↔guest file sharing: students add a host folder
// by absolute path and it is mounted into the VM (persistently when the VM is
// stopped, or for the current session when it is running).
export function SharedFolders({ vmId }: SharedFoldersProps) {
  const { t, tf, ts } = useT();
  const [folders, setFolders] = useState<SharedFolder[]>([]);
  const [name, setName] = useState('');
  const [hostPath, setHostPath] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const response = await api.getSharedFolders(vmId);
      setFolders(response.folders);
    } catch {
      // Sharing is auxiliary; a failed list should not break the console view.
      setFolders([]);
    }
  }, [vmId]);

  useEffect(() => {
    void load();
  }, [load]);

  function messageFor(err: unknown): string {
    if (err instanceof ApiError && err.body.trim() !== '') {
      return err.body.trim();
    }
    return err instanceof Error ? err.message : String(err);
  }

  async function handleAdd(event: React.FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await api.addSharedFolder(vmId, name.trim(), hostPath.trim());
      setName('');
      setHostPath('');
      await load();
    } catch (err) {
      setError(messageFor(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleRemove(folderName: string) {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await api.removeSharedFolder(vmId, folderName);
      await load();
    } catch (err) {
      setError(messageFor(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="shared-folders" aria-label={t('Shared folders')}>
      <h3 className="shared-folders-title">{t('Shared folders')}</h3>

      {folders.length === 0 ? (
        <p className="shared-folders-empty">{t('No shared folders yet.')}</p>
      ) : (
        <ul className="shared-folders-list">
          {folders.map((folder) => (
            <li className="shared-folders-item" key={`${folder.name}-${folder.transient}`}>
              <span className="shared-folders-name">{folder.name}</span>
              <span className="shared-folders-path">{folder.hostPath}</span>
              {folder.transient && (
                <span className="shared-folders-badge" title={t('Only for the current session')}>
                  {t('session')}
                </span>
              )}
              <button
                type="button"
                className="btn btn--danger shared-folders-remove"
                onClick={() => void handleRemove(folder.name)}
                disabled={busy}
                aria-label={tf('Remove shared folder {name}', { name: folder.name })}
              >
                {t('Remove')}
              </button>
            </li>
          ))}
        </ul>
      )}

      <form className="shared-folders-form" onSubmit={handleAdd}>
        <input
          className="shared-folders-input"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t('Share name (e.g. labshare)')}
          aria-label={t('Share name')}
          required
        />
        <input
          className="shared-folders-input"
          type="text"
          value={hostPath}
          onChange={(e) => setHostPath(e.target.value)}
          placeholder={t('Host path (e.g. C:\\labs\\share)')}
          aria-label={t('Host path')}
          required
        />
        <button type="submit" className="btn" disabled={busy}>
          {busy ? t('Working…') : t('Share folder')}
        </button>
      </form>

      {error && <div className="error-state shared-folders-error">{ts(error)}</div>}
    </section>
  );
}
