import { useCallback, useEffect, useState } from 'react';
import { api, ApiError } from '../api/client';
import { useT } from '../i18n/i18n';
import { SerialTerminal, type SerialStatus } from './SerialTerminal';
import type { VmSerialConsoleResponse } from '../types/api';

// SerialTerminalPanel is the self-contained serial-console terminal flow for the
// focused VM. It renders nothing for non-Linux guests, so MachinesView can drop
// it in unconditionally. Flow: enable serial (stopped VM) -> start VM -> enable
// login getty (running, credentials) -> open the terminal.
export function SerialTerminalPanel({ vmId, running }: { vmId: string; running: boolean }) {
  const { t } = useT();
  const [status, setStatus] = useState<VmSerialConsoleResponse | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [conn, setConn] = useState<SerialStatus>('connecting');
  const [gettyOpen, setGettyOpen] = useState(false);
  const [user, setUser] = useState('');
  const [pass, setPass] = useState('');
  const [gettyMsg, setGettyMsg] = useState<string | null>(null);

  const load = useCallback(async () => {
    // Focusing a VM fires several panels at once; VBoxManage's COM layer can
    // transiently fail under that load, so retry a few times before giving up.
    for (let attempt = 0; attempt < 4; attempt += 1) {
      try {
        setStatus(await api.getSerialConsole(vmId));
        return;
      } catch {
        if (attempt < 3) await new Promise((r) => setTimeout(r, 300 * (attempt + 1)));
      }
    }
    setStatus(null);
  }, [vmId]);

  useEffect(() => {
    setOpen(false);
    void load();
  }, [load, running]);

  if (!status || !status.terminalCapable) return null;

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
        setGettyOpen(false);
        setPass('');
      }
    } catch (err) {
      setGettyMsg(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const statusLabel =
    conn === 'open' ? t('connected') : conn === 'connecting' ? t('connecting') : t('disconnected');

  return (
    <section className="net-panel" aria-label="Terminal">
      <div className="net-h">
        <h3>{t('Terminal')}</h3>
        <span className="sub">{open ? statusLabel : t('serial · Linux')}</span>
      </div>

      {!status.enabled && (
        <div className="net-row">
          <p className="net-hint">{t('A serial console gives you a shell in a tab, no GUI window.')}</p>
          {status.editable ? (
            <button type="button" className="net-apply" disabled={busy} onClick={() => run(() => api.enableSerialConsole(vmId))}>
              {t('Enable serial terminal')}
            </button>
          ) : (
            <p className="net-hint">{t('Power off the VM to enable the serial terminal.')}</p>
          )}
        </div>
      )}

      {status.enabled && !running && (
        <div className="net-row">
          <p className="net-hint">{t('Start the VM to use the terminal.')}</p>
          {status.editable && (
            <button type="button" className="net-apply" disabled={busy} onClick={() => run(() => api.disableSerialConsole(vmId))}>
              {t('Disable serial terminal')}
            </button>
          )}
        </div>
      )}

      {status.enabled && running && (
        <div className="net-row">
          <div className="tv-serial-actions">
            {!open && (
              <button type="button" className="net-apply" onClick={() => setOpen(true)}>
                {t('Open terminal')}
              </button>
            )}
            <button type="button" className="net-apply" onClick={() => setGettyOpen((v) => !v)}>
              {t('Enable login (getty)')}
            </button>
          </div>

          {gettyOpen && (
            <form
              className="tv-serial-getty"
              onSubmit={(e) => {
                e.preventDefault();
                void enableGetty();
              }}
            >
              <p className="net-hint">{t('Turns on a login prompt on the serial port. Needs a root or sudo account.')}</p>
              <label className="net-field">
                <span>{t('Guest username')}</span>
                <input className="net-select" type="text" value={user} autoComplete="off" onChange={(e) => setUser(e.target.value)} />
              </label>
              <label className="net-field">
                <span>{t('Guest password')}</span>
                <input className="net-select" type="password" value={pass} autoComplete="off" onChange={(e) => setPass(e.target.value)} />
              </label>
              <button type="submit" className="net-apply" disabled={busy || !user || !pass}>
                {t('Enable login')}
              </button>
              {gettyMsg && <p className="net-hint">{gettyMsg}</p>}
            </form>
          )}

          {open && (
            <div className="tv-serial-wrap">
              <SerialTerminal vmId={vmId} onStatus={setConn} />
            </div>
          )}
        </div>
      )}

      {error && <p className="files-error">{error}</p>}
    </section>
  );
}
