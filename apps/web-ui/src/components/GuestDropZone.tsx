import { useState } from 'react';
import { useGuestFileDrop } from '../hooks/useGuestFileDrop';
import { useT } from '../i18n/i18n';

interface GuestDropZoneProps {
  vmId: string;
  vmName?: string;
  className?: string;
  // overlayLabel lets the console show a screen-sized hint while the panel shows
  // a compact one.
  overlayLabel?: string;
  children: React.ReactNode;
}

// GuestDropZone wraps any area so files dropped onto it are sent into the guest.
// It renders a drop overlay while dragging, a stack of transfer toasts, and the
// one-time guest credential prompt used when the VM has no shared folder.
export function GuestDropZone({ vmId, vmName, className, overlayLabel, children }: GuestDropZoneProps) {
  const { t, tf, ts } = useT();
  const { dragging, transfers, credPrompt, dropHandlers, submitCreds, cancelCreds, dismiss } =
    useGuestFileDrop(vmId);

  return (
    <div className={`gdz ${dragging ? 'is-dragging' : ''} ${className ?? ''}`} {...dropHandlers}>
      {children}

      {dragging && (
        <div className="gdz-overlay" aria-hidden="true">
          <div className="gdz-overlay-inner">
            <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 16V4M7 9l5-5 5 5" />
              <path d="M5 20h14" />
            </svg>
            <span>{overlayLabel ?? tf('Drop to send to {vm}', { vm: vmName ?? t('the VM') })}</span>
          </div>
        </div>
      )}

      {transfers.length > 0 && (
        <div className="gdz-toasts">
          {transfers.map((tr) => (
            <div key={tr.key} className={`gdz-toast ${tr.state}`}>
              <span className="gdz-toast-ico" aria-hidden="true">
                {tr.state === 'uploading' ? (
                  <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" className="spin">
                    <path d="M12 3a9 9 0 1 0 9 9" />
                  </svg>
                ) : tr.state === 'done' ? (
                  <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M5 12l5 5L20 7" />
                  </svg>
                ) : (
                  <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M12 8v5M12 16h.01" />
                    <circle cx="12" cy="12" r="9" />
                  </svg>
                )}
              </span>
              <div className="gdz-toast-body">
                <span className="gdz-toast-name">{tr.name}</span>
                {tr.message && <span className="gdz-toast-msg">{ts(tr.message)}</span>}
              </div>
              {tr.state !== 'uploading' && (
                <button type="button" className="gdz-toast-x" aria-label={t('Dismiss')} onClick={() => dismiss(tr.key)}>
                  ×
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {credPrompt && (
        <GuestCredPrompt
          vmName={vmName}
          fileCount={credPrompt.length}
          onSubmit={submitCreds}
          onCancel={cancelCreds}
        />
      )}
    </div>
  );
}

interface GuestCredPromptProps {
  vmName?: string;
  fileCount: number;
  onSubmit: (username: string, password: string) => void;
  onCancel: () => void;
}

// GuestCredPrompt reuses the Guest Additions modal styling. It appears only when
// the VM has no shared folder, so a guest-control copy is the only option.
function GuestCredPrompt({ vmName, fileCount, onSubmit, onCancel }: GuestCredPromptProps) {
  const { t, tf } = useT();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  function handleSubmit() {
    if (username.trim() === '' || password === '') return;
    const pw = password;
    setPassword(''); // drop it from state immediately
    onSubmit(username.trim(), pw);
  }

  return (
    <div className="ga-overlay" role="dialog" aria-label={t('Guest credentials')}>
      <div className="ga-modal">
        <div className="ga-modal-head">
          <h3>{t('Guest credentials')}</h3>
          <button type="button" className="ga-x" aria-label={t('close')} onClick={onCancel}>
            ×
          </button>
        </div>
        <p className="ga-modal-note">
          {tf(
            '{vm} has no shared folder, so {files} will be copied in over VirtualBox guest control. Enter a guest username and password — used once and reused only for this session, never stored. Tip: add a shared folder to skip this next time.',
            {
              vm: vmName ?? t('This VM'),
              files: fileCount === 1 ? t('the file') : tf('{n} files', { n: fileCount }),
            },
          )}
        </p>
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
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleSubmit();
            }}
          />
        </label>
        <div className="ga-modal-actions">
          <button type="button" className="tv-abtn" onClick={onCancel}>
            {t('Cancel')}
          </button>
          <button
            type="button"
            className="tv-abtn go"
            onClick={handleSubmit}
            disabled={username.trim() === '' || password === ''}
          >
            {t('Send')}
          </button>
        </div>
      </div>
    </div>
  );
}
