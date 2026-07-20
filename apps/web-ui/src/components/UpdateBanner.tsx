import { useState } from 'react';
import { useT } from '../i18n/i18n';
import type { UpdateStatus } from '../types/api';

const DISMISSED_KEY = 'tabvm.updateDismissed';

interface UpdateBannerProps {
  status: UpdateStatus;
}

function readDismissedVersion(): string | null {
  try {
    return localStorage.getItem(DISMISSED_KEY);
  } catch {
    return null;
  }
}

// UpdateBanner shows a dismissible notice when a newer TabVM release exists. It
// is presentational: the data comes from useUpdateStatus (see AppShell). The
// dismissal is keyed by version, so dismissing v0.1.3 hides it for good, but the
// banner returns the moment a newer version ships.
export function UpdateBanner({ status }: UpdateBannerProps) {
  const { t, tf } = useT();
  const [dismissedVersion, setDismissedVersion] = useState<string | null>(readDismissedVersion);

  if (!status.updateAvailable || !status.latest) return null;
  if (dismissedVersion === status.latest) return null;

  const latest = status.latest;
  const dismiss = () => {
    try {
      localStorage.setItem(DISMISSED_KEY, latest);
    } catch {
      // localStorage may be unavailable; the in-session dismissal still applies.
    }
    setDismissedVersion(latest);
  };

  return (
    <div className="update-banner" role="status">
      <svg
        className="update-banner-icon"
        viewBox="0 0 24 24"
        fill="none"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M12 3v12" />
        <path d="m7 10 5 5 5-5" />
        <path d="M5 21h14" />
      </svg>
      <div className="update-banner-text">
        <span className="update-banner-msg">{tf('TabVM v{version} is available', { version: latest })}</span>
        <span className="update-banner-hint">{t('Installed with Scoop? Run `scoop update tabvm`')}</span>
      </div>
      {status.releaseUrl && (
        <a
          className="update-banner-link"
          href={status.releaseUrl}
          target="_blank"
          rel="noopener noreferrer"
        >
          {t('Download')}
        </a>
      )}
      <button
        type="button"
        className="update-banner-dismiss"
        aria-label={t('Dismiss')}
        title={t('Dismiss')}
        onClick={dismiss}
      >
        <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M6 6l12 12M18 6L6 18" />
        </svg>
      </button>
    </div>
  );
}
