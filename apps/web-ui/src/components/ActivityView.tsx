import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { useVmStatus } from '../hooks/useVmStatus';
import type { ActivityEntry } from '../types/api';
import { useT } from '../i18n/i18n';

// ActivityView renders the agent's recorded operation log (start/stop/reset,
// console prepare, shared-folder changes) from GET /api/activity. Each row shows
// the machine name for quick reading; clicking it expands the full detail (log
// message + the exact machine id) in place. Rows are collapsed by default and
// only one is open at a time.
export function ActivityView() {
  const { t } = useT();
  const { vms } = useVmStatus();
  const [entries, setEntries] = useState<ActivityEntry[]>([]);
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [openKey, setOpenKey] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .getActivity()
      .then((response) => {
        if (!cancelled) {
          setEntries(response.entries);
          setState('ready');
        }
      })
      .catch(() => {
        if (!cancelled) setState('error');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Resolve a machine id to its current name; fall back to the id when the
  // machine no longer exists (e.g. it was deleted after the operation).
  const vmName = (id: string) => vms.find((vm) => vm.id === id)?.name ?? id;

  return (
    <>
      <div className="tv-page-head tv-rise d1">
        <h1>{t('Activity')}</h1>
        <p>{t('Recorded machine operations, newest first.')}</p>
      </div>

      <section className="tv-sec tv-rise d2">
        <div className="tv-sec-top">
          <h2>{t('Operation log')}</h2>
          <span className="count">{entries.length}</span>
        </div>

        {state === 'loading' && <div className="tv-empty">{t('Loading activity…')}</div>}
        {state === 'error' && (
          <div className="tv-error">{t('Activity is unavailable. The agent may not expose the log yet.')}</div>
        )}
        {state === 'ready' && entries.length === 0 && (
          <div className="tv-empty">{t('No recorded operations yet.')}</div>
        )}

        {state === 'ready' && entries.length > 0 && (
          <div className="tv-list">
            {entries.map((entry, index) => {
              const key = `${entry.recordedAt}-${index}`;
              const isOpen = openKey === key;
              const name = vmName(entry.vmId);
              return (
                <div className="tv-log-entry" key={key}>
                  <button
                    type="button"
                    className="tv-log"
                    aria-expanded={isOpen}
                    onClick={() => setOpenKey(isOpen ? null : key)}
                  >
                    <span className={`tv-log-dot ${entry.success ? 'ok' : 'fail'}`} />
                    <span className="tv-log-action">{entry.action}</span>
                    <span className="tv-log-vm">{name}</span>
                    <span className="tv-log-time">{new Date(entry.recordedAt).toLocaleString()}</span>
                    <span className="tv-log-chev" aria-hidden="true">
                      ▸
                    </span>
                  </button>
                  {isOpen && (
                    <div className="tv-log-detail">
                      <p className="tv-log-msg">
                        {entry.message?.trim() ? entry.message : t('No additional detail was recorded.')}
                      </p>
                      <div className="tv-log-meta">
                        <span>{entry.success ? t('Succeeded') : t('Failed')}</span>
                        {entry.vmId && <span className="tv-log-uuid">{entry.vmId}</span>}
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </section>
    </>
  );
}
