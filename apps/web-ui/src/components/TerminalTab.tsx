import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import { useT } from '../i18n/i18n';
import { SerialTerminal, type SerialStatus } from './SerialTerminal';
import { SplashScreen } from './SplashScreen';
import type { VmSerialConsoleResponse } from '../types/api';

interface TerminalTabProps {
  vmId: string;
  vmName: string;
}

// TerminalTab is the whole page when the app is opened at ?terminal=<id>: a
// full-bleed serial terminal that fills the tab, with only a thin top bar. Setup
// steps (enable serial, start hint) center in the empty area; the login-getty
// action lives in the bar and opens a small floating panel so it never pushes
// the terminal into a box.
export function TerminalTab({ vmId, vmName }: TerminalTabProps) {
  const { t } = useT();
  const [status, setStatus] = useState<VmSerialConsoleResponse | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conn, setConn] = useState<SerialStatus>('connecting');
  const [gettyOpen, setGettyOpen] = useState(false);
  const [user, setUser] = useState('');
  const [pass, setPass] = useState('');
  const [gettyMsg, setGettyMsg] = useState<string | null>(null);
  const [showSplash, setShowSplash] = useState(true);

  useEffect(() => {
    const previous = document.title;
    document.title = `${vmName} — TabVM terminal`;
    return () => {
      document.title = previous;
    };
  }, [vmName]);

  const load = useCallback(async () => {
    try {
      setStatus(await api.getSerialConsole(vmId));
    } catch {
      setStatus(null);
    } finally {
      setLoaded(true);
    }
  }, [vmId]);

  useEffect(() => {
    void load();
  }, [load]);

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.body.trim() !== '') setError(err.body.trim());
      else setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const enableGetty = async () => {
    setBusy(true);
    setGettyMsg(null);
    try {
      const result = await api.enableSerialGetty(vmId, user, pass);
      setGettyMsg(result.message);
      if (result.success) {
        setPass('');
      }
    } catch (err) {
      setGettyMsg(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  // window.close() only works for a tab the app opened via window.open. If the
  // user navigated here directly, the browser blocks it, so fall back to the
  // dashboard (dropping the ?terminal param).
  const handleClose = () => {
    window.close();
    window.setTimeout(() => {
      if (!window.closed) window.location.href = window.location.pathname;
    }, 150);
  };

  const connected = status?.enabled && status.running;

  return (
    <div className="tv-termtab">
      {showSplash && <SplashScreen onDone={() => setShowSplash(false)} />}

      <header className="tv-termtab-bar">
        <span className="tv-termtab-title">
          <span className="tv-termtab-dot" data-state={conn} />
          {vmName}
          <span className="tv-termtab-sub">{t('serial terminal')}</span>
        </span>
        <div className="tv-termtab-headactions">
          {connected && (
            <button type="button" className="tv-abtn" onClick={() => setGettyOpen((v) => !v)}>
              {t('Enable login (getty)')}
            </button>
          )}
          <button type="button" className="tv-abtn" onClick={handleClose}>
            {t('close')}
          </button>
        </div>
      </header>

      <div className="tv-termtab-body">
        {!loaded ? (
          <div className="tv-termtab-center">
            <p className="tv-termtab-note">{t('Loading…')}</p>
          </div>
        ) : !status || !status.terminalCapable ? (
          <div className="tv-termtab-center">
            <p className="tv-termtab-note">{t('The serial terminal is only available for Linux guests.')}</p>
          </div>
        ) : connected ? (
          <div className="tv-termtab-screen">
            <SerialTerminal vmId={vmId} onStatus={setConn} />
          </div>
        ) : (
          <div className="tv-termtab-center">
            <div className="tv-termtab-setup">
              {!status.enabled && status.editable && (
                <>
                  <p className="tv-termtab-note">{t('A serial console gives you a shell in a tab, no GUI window.')}</p>
                  <button type="button" className="net-apply" disabled={busy} onClick={() => run(() => api.enableSerialConsole(vmId))}>
                    {t('Enable serial terminal')}
                  </button>
                </>
              )}
              {!status.enabled && !status.editable && (
                <p className="tv-termtab-note">{t('Power off the VM to enable the serial terminal.')}</p>
              )}
              {status.enabled && !status.running && (
                <p className="tv-termtab-note">{t('Start the VM to use the terminal.')}</p>
              )}
              {error && <p className="files-error">{error}</p>}
            </div>
          </div>
        )}
      </div>

      {connected && gettyOpen && (
        <div className="tv-termtab-popover">
          <form
            className="tv-termtab-getty"
            onSubmit={(e) => {
              e.preventDefault();
              void enableGetty();
            }}
          >
            <p className="tv-termtab-note">{t('Turns on a login prompt on the serial port. Needs a root or sudo account.')}</p>
            <input className="net-select" type="text" placeholder={t('Guest username')} value={user} autoComplete="off" onChange={(e) => setUser(e.target.value)} />
            <input className="net-select" type="password" placeholder={t('Guest password')} value={pass} autoComplete="off" onChange={(e) => setPass(e.target.value)} />
            <button type="submit" className="net-apply" disabled={busy || !user || !pass}>
              {t('Enable login')}
            </button>
            {gettyMsg && <p className="tv-termtab-note">{gettyMsg}</p>}
          </form>
        </div>
      )}
    </div>
  );
}
