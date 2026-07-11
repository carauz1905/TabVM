import { useCallback, useEffect, useRef, useState } from 'react';
import { api, ApiError } from '../api/client';
import { useT } from '../i18n/i18n';

interface CreateVmWizardProps {
  onClose: () => void;
  // Called once a create/import finishes successfully so the parent can refresh.
  onCreated: () => void;
}

type Mode = 'import' | 'install' | 'manual';
type Phase = 'form' | 'working' | 'done' | 'error';

// OS types offered for unattended install — must match the backend allow-list.
const OS_TYPES: { id: string; label: string }[] = [
  { id: 'Ubuntu_64', label: 'Ubuntu (64-bit)' },
  { id: 'Ubuntu24_LTS_64', label: 'Ubuntu 24.04 LTS' },
  { id: 'Ubuntu22_LTS_64', label: 'Ubuntu 22.04 LTS' },
  { id: 'Debian_64', label: 'Debian (64-bit)' },
  { id: 'Debian12_64', label: 'Debian 12' },
  { id: 'Windows11_64', label: 'Windows 11' },
  { id: 'Windows10_64', label: 'Windows 10' },
  { id: 'Windows2022_64', label: 'Windows Server 2022' },
  { id: 'Windows2019_64', label: 'Windows Server 2019' },
];

// OS types offered for manual install — must match the backend allow-list.
// Generic types only: the user installs the OS themselves from the ISO.
const MANUAL_OS_TYPES: { id: string; label: string }[] = [
  { id: 'Linux_64', label: 'Linux (64-bit)' },
  { id: 'Other_64', label: 'Other (64-bit)' },
];

// MAX_POLL_FAILURES bounds consecutive job-status poll failures before the
// wizard gives up and surfaces an error instead of spinning forever.
const MAX_POLL_FAILURES = 10;

// CreateVmWizard is the "New VM" modal. It has three paths: import a prebuilt
// .ova appliance (works for Kali, Guest Additions already inside), run an
// unattended install from an Ubuntu/Debian/Windows ISO with Guest Additions
// baked in during setup, or a manual install from any bootable ISO (the VM is
// created with the ISO attached and the user installs via the console). All
// run as background jobs on the agent and are polled to completion.
export function CreateVmWizard({ onClose, onCreated }: CreateVmWizardProps) {
  const { t, ts } = useT();
  const [mode, setMode] = useState<Mode>('import');
  const [phase, setPhase] = useState<Phase>('form');
  const [message, setMessage] = useState('');

  // Shared + import fields.
  const [name, setName] = useState('');
  const [ovaPath, setOvaPath] = useState('');

  // Install fields (shared by unattended and manual modes).
  const [isoPath, setIsoPath] = useState('');
  const [osType, setOsType] = useState('Ubuntu_64');
  const [manualOsType, setManualOsType] = useState('Linux_64');
  const [memoryMb, setMemoryMb] = useState(2048);
  const [cpus, setCpus] = useState(2);
  const [diskGb, setDiskGb] = useState(25);
  const [username, setUsername] = useState('student');
  const [password, setPassword] = useState('');

  const pollRef = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearInterval(pollRef.current), []);

  const pickFile = useCallback(
    async (set: (p: string) => void) => {
      try {
        const picked = await api.pickHostFile();
        if (!picked.cancelled && picked.path.trim() !== '') set(picked.path);
      } catch {
        // dialog failure is non-fatal; the user can retry
      }
    },
    [],
  );

  // poll watches a job to completion, updating phase/message. It gives up on a
  // 404 (the agent restarted and lost its in-memory jobs) or after too many
  // consecutive failures, instead of retrying forever.
  const poll = useCallback(
    (jobId: string) => {
      let consecutiveFailures = 0;
      pollRef.current = window.setInterval(async () => {
        try {
          const status = await api.getCreateStatus(jobId);
          consecutiveFailures = 0;
          if (status.state === 'done') {
            window.clearInterval(pollRef.current);
            setMessage(status.message);
            setPhase('done');
            onCreated();
          } else if (status.state === 'error') {
            window.clearInterval(pollRef.current);
            setMessage(status.message);
            setPhase('error');
          }
        } catch (err) {
          if (err instanceof ApiError && err.status === 404) {
            // The job no longer exists — jobs live in agent memory, so an
            // agent restart makes them unrecoverable. Stop immediately.
            window.clearInterval(pollRef.current);
            setMessage(t('The creation job is no longer available. The agent may have restarted; check the machine list before retrying.'));
            setPhase('error');
            return;
          }
          consecutiveFailures += 1;
          if (consecutiveFailures >= MAX_POLL_FAILURES) {
            window.clearInterval(pollRef.current);
            setMessage(t('Lost contact with the agent while creating the VM. Check the machine list before retrying.'));
            setPhase('error');
          }
          // Otherwise: transient poll failure; keep trying.
        }
      }, 2000);
    },
    [onCreated, t],
  );

  const submit = useCallback(async () => {
    setPhase('working');
    setMessage('');
    try {
      const job =
        mode === 'import'
          ? await api.importVm(ovaPath, name.trim())
          : mode === 'manual'
            ? await api.createVmManual({
                name: name.trim(),
                osType: manualOsType,
                isoPath,
                memoryMb,
                cpus,
                diskGb,
              })
            : await api.createVm({
                name: name.trim(),
                osType,
                isoPath,
                memoryMb,
                cpus,
                diskGb,
                username: username.trim(),
                password,
                hostname: '',
              });
      setPassword(''); // drop the secret from state once submitted
      poll(job.jobId);
    } catch (err) {
      const msg =
        err instanceof ApiError && err.body.trim() !== ''
          ? err.body.trim()
          : err instanceof Error
            ? err.message
            : String(err);
      setMessage(msg);
      setPhase('error');
    }
  }, [mode, ovaPath, name, osType, manualOsType, isoPath, memoryMb, cpus, diskGb, username, password, poll]);

  const canSubmit =
    phase === 'form' &&
    name.trim() !== '' &&
    (mode === 'import'
      ? ovaPath.trim() !== ''
      : mode === 'manual'
        ? isoPath.trim() !== ''
        : isoPath.trim() !== '' && username.trim() !== '' && password !== '');

  return (
    <div className="ga-overlay" role="dialog" aria-label={t('Create a virtual machine')}>
      <div className="tv-wiz">
        <div className="ga-modal-head">
          <h3>{t('New virtual machine')}</h3>
          <button type="button" className="ga-x" aria-label={t('close')} onClick={onClose}>
            ×
          </button>
        </div>

        {phase === 'form' && (
          <>
            <div className="tv-wiz-tabs" role="tablist">
              <button
                type="button"
                role="tab"
                aria-selected={mode === 'import'}
                className={`tv-wiz-tab ${mode === 'import' ? 'on' : ''}`}
                onClick={() => setMode('import')}
              >
                {t('Import image (.ova)')}
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={mode === 'install'}
                className={`tv-wiz-tab ${mode === 'install' ? 'on' : ''}`}
                onClick={() => setMode('install')}
              >
                {t('Install from ISO')}
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={mode === 'manual'}
                className={`tv-wiz-tab ${mode === 'manual' ? 'on' : ''}`}
                onClick={() => setMode('manual')}
              >
                {t('Other OS (manual install)')}
              </button>
            </div>

            <p className="tv-wiz-note">
              {mode === 'import'
                ? t('Import a prebuilt appliance that already has Guest Additions. Best for Kali. One click, no install.')
                : mode === 'manual'
                  ? t('Create a VM with any bootable ISO attached. You install the OS yourself in the console; Guest Additions are not installed automatically.')
                  : t('Create a VM and run an automated Ubuntu, Debian or Windows install with Guest Additions included. Kali is not supported here.')}
            </p>

            <label className="ga-field">
              <span>{t('VM name')}</span>
              <input type="text" value={name} placeholder="lab-vm" onChange={(e) => setName(e.target.value)} />
            </label>

            {mode === 'import' ? (
              <div className="tv-wiz-file">
                <button type="button" className="tv-abtn" onClick={() => void pickFile(setOvaPath)}>
                  {t('Choose .ova/.ovf…')}
                </button>
                <span className="tv-wiz-path" title={ovaPath}>
                  {ovaPath || t('No file chosen')}
                </span>
              </div>
            ) : (
              <>
                <div className="tv-wiz-file">
                  <button type="button" className="tv-abtn" onClick={() => void pickFile(setIsoPath)}>
                    {t('Choose .iso…')}
                  </button>
                  <span className="tv-wiz-path" title={isoPath}>
                    {isoPath || t('No file chosen')}
                  </span>
                </div>
                {mode === 'manual' ? (
                  <label className="ga-field">
                    <span>{t('Operating system')}</span>
                    <select value={manualOsType} onChange={(e) => setManualOsType(e.target.value)}>
                      {MANUAL_OS_TYPES.map((o) => (
                        <option key={o.id} value={o.id}>
                          {o.label}
                        </option>
                      ))}
                    </select>
                  </label>
                ) : (
                  <label className="ga-field">
                    <span>{t('Operating system')}</span>
                    <select value={osType} onChange={(e) => setOsType(e.target.value)}>
                      {OS_TYPES.map((o) => (
                        <option key={o.id} value={o.id}>
                          {o.label}
                        </option>
                      ))}
                    </select>
                  </label>
                )}
                <div className="tv-wiz-grid3">
                  <label className="ga-field">
                    <span>{t('Memory (MB)')}</span>
                    <input type="number" min={512} step={512} value={memoryMb} onChange={(e) => setMemoryMb(Number(e.target.value))} />
                  </label>
                  <label className="ga-field">
                    <span>{t('CPUs')}</span>
                    <input type="number" min={1} max={16} value={cpus} onChange={(e) => setCpus(Number(e.target.value))} />
                  </label>
                  <label className="ga-field">
                    <span>{t('Disk (GB)')}</span>
                    <input type="number" min={8} max={512} value={diskGb} onChange={(e) => setDiskGb(Number(e.target.value))} />
                  </label>
                </div>
                {mode === 'install' && (
                  <div className="tv-wiz-grid2">
                    <label className="ga-field">
                      <span>{t('Guest username')}</span>
                      <input type="text" autoComplete="off" value={username} onChange={(e) => setUsername(e.target.value)} />
                    </label>
                    <label className="ga-field">
                      <span>{t('Guest password')}</span>
                      <input type="password" autoComplete="off" value={password} onChange={(e) => setPassword(e.target.value)} />
                    </label>
                  </div>
                )}
              </>
            )}

            <div className="ga-modal-actions">
              <button type="button" className="tv-abtn" onClick={onClose}>
                {t('cancel')}
              </button>
              <button type="button" className="tv-abtn go" disabled={!canSubmit} onClick={() => void submit()}>
                {mode === 'import' ? t('Import') : t('Create')}
              </button>
            </div>
          </>
        )}

        {phase === 'working' && (
          <div className="tv-wiz-status">
            <span className="tv-wiz-spin" aria-hidden="true" />
            <p>
              {mode === 'import'
                ? t('Importing the appliance… this can take several minutes.')
                : mode === 'manual'
                  ? t('Creating the VM and attaching the installer ISO…')
                  : t('Creating the VM and preparing the automated install…')}
            </p>
          </div>
        )}

        {phase === 'done' && (
          <div className="tv-wiz-status">
            <p className="tv-wiz-ok">{ts(message)}</p>
            <p className="tv-wiz-sub">
              {mode === 'import'
                ? t('The VM is ready. Start it from the list.')
                : mode === 'manual'
                  ? t('Start the VM and install the OS yourself in the console.')
                  : t('Start the VM to run the install; watch it in the console.')}
            </p>
            <div className="ga-modal-actions">
              <button type="button" className="tv-abtn go" onClick={onClose}>
                {t('Done')}
              </button>
            </div>
          </div>
        )}

        {phase === 'error' && (
          <div className="tv-wiz-status">
            <p className="tv-wiz-err">{ts(message)}</p>
            <div className="ga-modal-actions">
              <button type="button" className="tv-abtn" onClick={onClose}>
                {t('close')}
              </button>
              <button type="button" className="tv-abtn go" onClick={() => setPhase('form')}>
                {t('Back')}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
