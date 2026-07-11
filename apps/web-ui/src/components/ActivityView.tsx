import { useEffect, useState } from 'react';
import { api } from '../api/client';
import type { ActivityEntry } from '../types/api';
import { useT } from '../i18n/i18n';

// ActivityView renders the agent's recorded operation log (start/stop/reset,
// console prepare, shared-folder changes) from GET /api/activity.
export function ActivityView() {
  const { t } = useT();
  const [entries, setEntries] = useState<ActivityEntry[]>([]);
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading');

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
            {entries.map((entry, index) => (
              <div className="tv-log" key={`${entry.recordedAt}-${index}`}>
                <span className={`tv-log-dot ${entry.success ? 'ok' : 'fail'}`} />
                <span className="tv-log-action">{entry.action}</span>
                <span className="tv-log-vm">{entry.vmId}</span>
                <span className="tv-log-time">{new Date(entry.recordedAt).toLocaleString()}</span>
              </div>
            ))}
          </div>
        )}
      </section>
    </>
  );
}
