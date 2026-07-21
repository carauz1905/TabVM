import { useCallback, useState } from 'react';
import { api, ApiError } from '../api/client';
import { getGuestCreds, setGuestCreds } from '../hooks/guestCreds';
import { useT } from '../i18n/i18n';

interface GuestControlPanelProps {
  vmId: string;
  vmName?: string;
}

interface RunResult {
  exitCode: number;
  output: string;
  truncated: boolean;
}

function messageFor(err: unknown): string {
  if (err instanceof ApiError && err.body.trim() !== '') return err.body.trim();
  return err instanceof Error ? err.message : String(err);
}

// GuestControlPanel exposes two VBoxManage guest-control operations for a running
// Linux VM: running a command inside the guest and copying a file out of it. Both
// need guest credentials, which are entered once here and cached in memory for
// the session (shared with the drag-drop file transfer via the guestCreds cache),
// never stored. The credentials travel to the agent, which passes them to
// VBoxManage via a temporary --passwordfile, never on a command line.
export function GuestControlPanel({ vmId, vmName }: GuestControlPanelProps) {
  const { t, ts } = useT();

  const seeded = getGuestCreds(vmId);
  const [username, setUsername] = useState(seeded?.username ?? '');
  const [password, setPassword] = useState(seeded?.password ?? '');

  const [command, setCommand] = useState('');
  const [running, setRunning] = useState(false);
  const [runResult, setRunResult] = useState<RunResult | null>(null);
  const [runError, setRunError] = useState<string | null>(null);

  const [guestPath, setGuestPath] = useState('');
  const [hostDir, setHostDir] = useState('');
  const [copying, setCopying] = useState(false);
  const [picking, setPicking] = useState(false);
  const [copiedPath, setCopiedPath] = useState<string | null>(null);
  const [copyError, setCopyError] = useState<string | null>(null);

  // rememberCreds returns the current credentials when both are present, caching
  // them for the session so the other guest-control features reuse them; it
  // returns null (and the caller shows the prompt) when either is missing.
  const rememberCreds = useCallback((): { username: string; password: string } | null => {
    const u = username.trim();
    if (u === '' || password === '') return null;
    setGuestCreds(vmId, { username: u, password });
    return { username: u, password };
  }, [username, password, vmId]);

  const handleRun = useCallback(async () => {
    if (running) return;
    setRunError(null);
    setRunResult(null);

    const creds = rememberCreds();
    if (!creds) {
      setRunError(t('Enter the guest username and password first.'));
      return;
    }
    const parts = command.trim().split(/\s+/).filter((p) => p !== '');
    if (parts.length === 0) {
      setRunError(t('Enter a command to run (an absolute path, for example /usr/bin/uptime).'));
      return;
    }
    const [exe, ...args] = parts;

    setRunning(true);
    try {
      const res = await api.runInGuest(vmId, exe, args, creds.username, creds.password);
      if (res.credentialsRequired) {
        setRunError(t('Enter the guest username and password first.'));
      } else {
        setRunResult({ exitCode: res.exitCode, output: res.output ?? '', truncated: res.truncated });
      }
    } catch (err) {
      setRunError(messageFor(err));
    } finally {
      setRunning(false);
    }
  }, [running, rememberCreds, command, vmId, t]);

  const handlePick = useCallback(async () => {
    if (picking || copying) return;
    setCopyError(null);
    setPicking(true);
    try {
      const picked = await api.pickHostFolder();
      if (!picked.cancelled && picked.path.trim() !== '') setHostDir(picked.path);
    } catch (err) {
      setCopyError(messageFor(err));
    } finally {
      setPicking(false);
    }
  }, [picking, copying]);

  const handleCopy = useCallback(async () => {
    if (copying) return;
    setCopyError(null);
    setCopiedPath(null);

    const creds = rememberCreds();
    if (!creds) {
      setCopyError(t('Enter the guest username and password first.'));
      return;
    }
    if (guestPath.trim() === '') {
      setCopyError(t('Enter the guest file path to copy out.'));
      return;
    }
    if (hostDir.trim() === '') {
      setCopyError(t('Choose a host folder to copy the file into.'));
      return;
    }

    setCopying(true);
    try {
      const res = await api.copyFromGuest(vmId, guestPath.trim(), hostDir, creds.username, creds.password);
      if (res.credentialsRequired) {
        setCopyError(t('Enter the guest username and password first.'));
      } else if (res.success) {
        setCopiedPath(res.hostPath ?? '');
      } else {
        setCopyError(res.message);
      }
    } catch (err) {
      setCopyError(messageFor(err));
    } finally {
      setCopying(false);
    }
  }, [copying, rememberCreds, guestPath, hostDir, vmId, t]);

  return (
    <section className="guestctl-panel" aria-label={t('Guest control')}>
      <div className="files-h">
        <h3>{t('Guest control')}</h3>
        <span className="sub">{vmName ? vmName : t('run commands · copy files out')}</span>
      </div>

      <p className="guestctl-note">
        {t('Run a command inside this VM or copy a file out of it. Needs a running Linux guest with Guest Additions active and a guest login (used once for this session, never stored).')}
      </p>

      <div className="guestctl-creds">
        <label className="ga-field">
          <span>{t('Guest username')}</span>
          <input
            type="text"
            autoComplete="off"
            value={username}
            placeholder="root"
            onChange={(e) => setUsername(e.target.value)}
          />
        </label>
        <label className="ga-field">
          <span>{t('Guest password')}</span>
          <input
            type="password"
            autoComplete="off"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
      </div>

      <div className="guestctl-block">
        <h4>{t('Run in guest')}</h4>
        <div className="guestctl-row">
          <input
            type="text"
            className="guestctl-input"
            aria-label={t('Command to run')}
            placeholder="/usr/bin/uptime"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void handleRun();
            }}
          />
          <button type="button" className="tv-abtn go" onClick={() => void handleRun()} disabled={running}>
            {running ? t('Running…') : t('Run')}
          </button>
        </div>
        {runError && <div className="files-error">{ts(runError)}</div>}
        {runResult && (
          <div className="guestctl-result">
            <span className={`guestctl-exit ${runResult.exitCode === 0 ? 'ok' : 'bad'}`}>
              {t('exit code')} {runResult.exitCode}
            </span>
            <pre className="guestctl-output" aria-label={t('Command output')}>
              {runResult.output === '' ? t('(no output)') : runResult.output}
              {runResult.truncated ? `\n${t('(output truncated)')}` : ''}
            </pre>
          </div>
        )}
      </div>

      <div className="guestctl-block">
        <h4>{t('Copy from guest')}</h4>
        <div className="guestctl-row">
          <input
            type="text"
            className="guestctl-input"
            aria-label={t('Guest file path')}
            placeholder="/home/user/report.txt"
            value={guestPath}
            onChange={(e) => setGuestPath(e.target.value)}
          />
        </div>
        <div className="guestctl-row">
          <button type="button" className="files-add" onClick={() => void handlePick()} disabled={picking || copying}>
            <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M3 7a2 2 0 0 1 2-2h4l2 2h6a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
            </svg>
            {picking ? t('Choose a folder…') : hostDir === '' ? t('Choose host folder') : hostDir}
          </button>
          <button type="button" className="tv-abtn go" onClick={() => void handleCopy()} disabled={copying}>
            {copying ? t('Copying…') : t('Copy from guest')}
          </button>
        </div>
        {copyError && <div className="files-error">{ts(copyError)}</div>}
        {copiedPath !== null && (
          <div className="guestctl-copied">
            {t('Copied to')} <b>{copiedPath}</b>
          </div>
        )}
      </div>
    </section>
  );
}
