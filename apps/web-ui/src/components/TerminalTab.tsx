import { useCallback, useEffect, useRef, useState } from 'react';
import { api, ApiError } from '../api/client';
import { useT } from '../i18n/i18n';
import { SerialTerminal, type SerialStatus } from './SerialTerminal';
import { TerminalIntro } from './TerminalIntro';
import type { VmSerialConsoleResponse } from '../types/api';

interface TerminalTabProps {
  vmId: string;
  vmName: string;
}

// How long after connecting we wait for any byte from the guest before deciding
// the port is silent (no login listener) and offering one-tap activation.
const SILENCE_TIMEOUT_MS = 2800;

// TerminalTab is the whole page when the app is opened at ?terminal=<id>: a
// full-bleed serial terminal that fills the tab. It self-diagnoses: if the guest
// stays silent after connecting (no login prompt), it surfaces a small, plain-
// language activation card instead of exposing a permanent "getty" control.
export function TerminalTab({ vmId, vmName }: TerminalTabProps) {
  const { t } = useT();
  const [status, setStatus] = useState<VmSerialConsoleResponse | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conn, setConn] = useState<SerialStatus>('connecting');
  const [showIntro, setShowIntro] = useState(true);

  // Silence self-diagnosis + one-tap activation.
  const [silent, setSilent] = useState(false);
  const [termKey, setTermKey] = useState(0);
  const receivedRef = useRef(false);
  const [user, setUser] = useState('');
  const [pass, setPass] = useState('');
  const [actMsg, setActMsg] = useState<string | null>(null);

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

  const handleData = useCallback(() => {
    receivedRef.current = true;
    setSilent(false);
  }, []);

  // After each (re)connect, wait a beat; if the guest never spoke, it is silent.
  useEffect(() => {
    if (conn !== 'open') return;
    receivedRef.current = false;
    setSilent(false);
    const id = window.setTimeout(() => {
      if (!receivedRef.current) setSilent(true);
    }, SILENCE_TIMEOUT_MS);
    return () => window.clearTimeout(id);
  }, [conn, termKey]);

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

  // Activate the login on a silent port, then reconnect so the fresh prompt shows.
  const activate = async () => {
    setBusy(true);
    setActMsg(null);
    try {
      const result = await api.enableSerialGetty(vmId, user, pass);
      setActMsg(result.message);
      if (result.success) {
        setPass('');
        receivedRef.current = false;
        setSilent(false);
        setTermKey((k) => k + 1);
      }
    } catch (err) {
      setActMsg(err instanceof Error ? err.message : String(err));
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
      {showIntro && <TerminalIntro onDone={() => setShowIntro(false)} />}

      <div className="tv-termtab-stage">
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
            <SerialTerminal key={termKey} vmId={vmId} onStatus={setConn} onData={handleData} />
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

        <div className="console-toolbar">
          {conn !== 'open' && (
            <span className="console-pill">
              <span className="muted">{conn === 'connecting' ? t('connecting…') : t('disconnected')}</span>
            </span>
          )}
          <button
            type="button"
            className="console-tbtn danger"
            onClick={handleClose}
            title={t('close')}
            aria-label={t('close')}
          >
            <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
              <path d="M6 6l12 12M18 6L6 18" />
            </svg>
          </button>
        </div>
      </div>

      {connected && silent && (
        <div className="tv-termtab-popover">
          <form
            className="tv-termtab-getty"
            onSubmit={(e) => {
              e.preventDefault();
              void activate();
            }}
          >
            <p className="tv-termtab-note">{t('The terminal is connected but the guest is not responding.')}</p>
            <p className="tv-termtab-note">{t('Activate it with a guest account (root or sudo). It is used once.')}</p>
            <input className="net-select" type="text" placeholder={t('Guest username')} value={user} autoComplete="off" onChange={(e) => setUser(e.target.value)} />
            <input className="net-select" type="password" placeholder={t('Guest password')} value={pass} autoComplete="off" onChange={(e) => setPass(e.target.value)} />
            <button type="submit" className="net-apply" disabled={busy || !user || !pass}>
              {t('Activate terminal')}
            </button>
            {actMsg && <p className="tv-termtab-note">{actMsg}</p>}
          </form>
        </div>
      )}
    </div>
  );
}
