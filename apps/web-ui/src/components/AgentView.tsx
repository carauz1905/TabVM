import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { useHealth } from '../hooks/useHealth';
import { useUpdateStatus } from '../hooks/useUpdateStatus';
import { useVmStatus } from '../hooks/useVmStatus';
import type { LocalStateStatusResponse } from '../types/api';
import { useT } from '../i18n/i18n';

function formatUptime(totalSeconds: number): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  const h = Math.floor(totalSeconds / 3600);
  const m = Math.floor((totalSeconds % 3600) / 60);
  const s = Math.floor(totalSeconds % 60);
  return `${pad(h)}:${pad(m)}:${pad(s)}`;
}

// AgentView surfaces the local agent's real health, VirtualBox discovery, and
// local-state readiness — the "System" status the sidebar links to.
export function AgentView() {
  const { t } = useT();
  const health = useHealth();
  const { discovery } = useVmStatus();
  // Best-effort latest-release info: shares useUpdateStatus (and its cache and
  // opt-out semantics) with the update banner; a failed or disabled check
  // simply leaves the row as a dash.
  const update = useUpdateStatus();
  const [localState, setLocalState] = useState<LocalStateStatusResponse | undefined>();

  useEffect(() => {
    let cancelled = false;
    api
      .getLocalStateStatus()
      .then((s) => {
        if (!cancelled) setLocalState(s);
      })
      .catch(() => {
        if (!cancelled) setLocalState(undefined);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const online = health.state === 'success' && health.data?.status === 'healthy';

  return (
    <>
      <div className="tv-page-head tv-rise d1">
        <h1>{t('Agent')}</h1>
        <p>{t('The local TabVM agent bound to 127.0.0.1, and its VirtualBox link.')}</p>
      </div>

      <section className="tv-sec tv-rise d2">
        <div className="tv-sec-top">
          <h2>{t('Runtime')}</h2>
        </div>
        <div className="tv-list">
          <div className="tv-kv">
            <span className="k">{t('Status')}</span>
            <span className="v">{online ? t('healthy') : health.state === 'loading' ? t('checking…') : t('unreachable')}</span>
          </div>
          <div className="tv-kv">
            <span className="k">{t('Uptime')}</span>
            <span className="v">
              {typeof health.data?.uptimeSeconds === 'number' ? formatUptime(health.data.uptimeSeconds) : '—'}
            </span>
          </div>
          <div className="tv-kv">
            <span className="k">{t('Bound')}</span>
            <span className="v">127.0.0.1</span>
          </div>
          <div className="tv-kv">
            <span className="k">{t('Latest available release')}</span>
            <span className="v">
              {update.latest
                ? update.updateAvailable
                  ? `v${update.latest} — ${t('update available')}`
                  : `v${update.latest}`
                : '—'}
            </span>
          </div>
        </div>
      </section>

      <section className="tv-sec tv-rise d3">
        <div className="tv-sec-top">
          <h2>VirtualBox</h2>
        </div>
        <div className="tv-list">
          <div className="tv-kv">
            <span className="k">VBoxManage</span>
            <span className="v">{discovery?.found ? t('found') : t('not found')}</span>
          </div>
          <div className="tv-kv">
            <span className="k">{t('Version')}</span>
            <span className="v">{discovery?.version ?? '—'}</span>
          </div>
        </div>
      </section>

      <section className="tv-sec tv-rise d4">
        <div className="tv-sec-top">
          <h2>{t('Local state')}</h2>
        </div>
        <div className="tv-list">
          <div className="tv-kv">
            <span className="k">{t('Store')}</span>
            <span className="v">{localState?.available ? t('ready') : localState ? t('unavailable') : '—'}</span>
          </div>
          <div className="tv-kv">
            <span className="k">{t('Schema')}</span>
            <span className="v">{localState ? localState.schema : '—'}</span>
          </div>
        </div>
      </section>
    </>
  );
}
