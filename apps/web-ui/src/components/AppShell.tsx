import { useEffect, useRef, useState, type ReactNode } from 'react';
import { BrandMark } from './BrandMark';
import { UpdateBanner } from './UpdateBanner';
import { useLang, useT } from '../i18n/i18n';
import type { UpdateStatus } from '../types/api';

export type ShellView = 'machines' | 'activity' | 'agent' | 'docs';

// Accent color options. 'teal' is the default (no data-accent attribute); the
// others map to the data-accent values styled in shell.css. `swatch` is a vivid
// representative hex just for the picker dot, so it reads on either theme.
export type Accent = 'teal' | 'pink' | 'orange' | 'yellow' | 'purple' | 'blue';
export const ACCENTS: { id: Accent; label: string; swatch: string }[] = [
  { id: 'teal', label: 'Teal', swatch: '#2fe3ce' },
  { id: 'pink', label: 'Pink', swatch: '#f472b6' },
  { id: 'orange', label: 'Orange', swatch: '#ffbc80' },
  { id: 'yellow', label: 'Yellow', swatch: '#ffe27a' },
  { id: 'purple', label: 'Purple', swatch: '#b89cff' },
  { id: 'blue', label: 'Blue', swatch: '#3b9eff' },
];

// initialAccent reads the persisted accent; anything unknown falls back to teal.
function initialAccent(): Accent {
  try {
    const stored = localStorage.getItem('tabvm.accent');
    if (
      stored === 'pink' ||
      stored === 'orange' ||
      stored === 'yellow' ||
      stored === 'purple' ||
      stored === 'blue'
    )
      return stored;
  } catch {
    // localStorage may be unavailable.
  }
  return 'teal';
}

// applyAccent sets (or clears, for the default teal) the data-accent attribute
// and persists the choice so the index.html boot script restores it with no flash.
export function applyAccent(accent: Accent) {
  const root = document.documentElement;
  if (accent === 'teal') root.removeAttribute('data-accent');
  else root.setAttribute('data-accent', accent);
  try {
    localStorage.setItem('tabvm.accent', accent);
  } catch {
    // in-session change still applies even if it cannot be persisted
  }
}

interface AppShellProps {
  active: ShellView;
  onNavigate: (view: ShellView) => void;
  crumb: string;
  agentOnline: boolean;
  version?: string;
  // Best-effort "update available" status from useUpdateStatus. Optional so the
  // shell renders without it (and never triggers the outbound check itself).
  update?: UpdateStatus;
  children: ReactNode;
}

interface NavItem {
  id: ShellView;
  label: string;
  icon: ReactNode;
}

const workspaceNav: NavItem[] = [
  {
    id: 'machines',
    label: 'Virtual machines',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="5" width="18" height="14" rx="2" />
        <path d="M3 9h18" />
        <path d="M6.5 7h.01" />
      </svg>
    ),
  },
  {
    id: 'activity',
    label: 'Activity',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <path d="M3 12h4l2 6 4-14 2 8h6" />
      </svg>
    ),
  },
];

const systemNav: NavItem[] = [
  {
    id: 'agent',
    label: 'Agent',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="4" y="4" width="16" height="16" rx="2" />
        <rect x="9" y="9" width="6" height="6" />
        <path d="M9 2v2M15 2v2M9 20v2M15 20v2M2 9h2M2 15h2M20 9h2M20 15h2" />
      </svg>
    ),
  },
];

const helpNav: NavItem[] = [
  {
    id: 'docs',
    label: 'Docs',
    // Lifebuoy: a ring of 8 segments, 4 filled with the live accent and 4 empty.
    // The dashed accent circle draws the coloured arcs; the faint currentColor
    // outlines give the ring its silhouette where the arcs are transparent.
    icon: (
      <svg className="tv-buoy" viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.3" opacity="0.4" />
        <circle cx="12" cy="12" r="4.5" stroke="currentColor" strokeWidth="1.3" opacity="0.4" />
        <circle
          cx="12"
          cy="12"
          r="6.75"
          stroke="var(--accent)"
          strokeWidth="4.5"
          strokeDasharray="5.3 5.3"
        />
      </svg>
    ),
  },
];

// resolveDark reports whether the dark theme is currently active, honoring an
// explicit data-theme override before falling back to the OS preference.
export function resolveDark(): boolean {
  const explicit = document.documentElement.getAttribute('data-theme');
  if (explicit) return explicit === 'dark';
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

// applyTheme sets and persists the theme, mirroring the top-bar toggle so other
// surfaces (e.g. the docs "personalize" demo) can change it consistently.
export function applyTheme(next: 'light' | 'dark') {
  document.documentElement.setAttribute('data-theme', next);
  try {
    localStorage.setItem('tabvm.theme', next);
  } catch {
    // localStorage may be unavailable; the in-session change still applies.
  }
}

export function AppShell({ active, onNavigate, crumb, agentOnline, version, update, children }: AppShellProps) {
  const [collapsed, setCollapsed] = useState(false);
  const { t } = useT();
  const { lang, toggle: toggleLang } = useLang();

  const [accent, setAccentState] = useState<Accent>(initialAccent);
  const [accentOpen, setAccentOpen] = useState(false);
  const accentRef = useRef<HTMLDivElement>(null);

  const chooseAccent = (next: Accent) => {
    setAccentState(next);
    applyAccent(next);
    setAccentOpen(false);
  };

  // Close the accent popover when clicking outside it or pressing Escape.
  useEffect(() => {
    if (!accentOpen) return;
    const onDown = (e: MouseEvent) => {
      if (accentRef.current && !accentRef.current.contains(e.target as Node)) setAccentOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setAccentOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [accentOpen]);

  const toggleSidebar = () => setCollapsed((c) => !c);
  const toggleTheme = () => {
    const next = resolveDark() ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    try {
      // Persist so a reload or a freshly launched tab keeps this choice; the
      // boot script in index.html reads it before first paint.
      localStorage.setItem('tabvm.theme', next);
    } catch {
      // localStorage may be unavailable; the in-session toggle still applies.
    }
  };

  const renderItem = (item: NavItem) => (
    <button
      key={item.id}
      type="button"
      className={`tv-item${active === item.id ? ' active' : ''}`}
      aria-label={t(item.label)}
      aria-current={active === item.id ? 'page' : undefined}
      onClick={() => onNavigate(item.id)}
    >
      {item.icon}
      <span className="tv-lbl">{t(item.label)}</span>
      <span className="tv-tip">{t(item.label)}</span>
    </button>
  );

  return (
    <div className="tv-root">
      <div className={`tv-app${collapsed ? ' collapsed' : ''}`}>
        <aside className="tv-side">
          <div className="tv-brand">
            <span
              className="tv-mark"
              role="button"
              tabIndex={0}
              aria-label={t('Toggle sidebar')}
              onClick={toggleSidebar}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  toggleSidebar();
                }
              }}
            >
              <BrandMark animated />
            </span>
            <span className="tv-word">
              Tab<i>VM</i>
            </span>
            <button
              type="button"
              className="tv-collapse"
              aria-label={t('Collapse sidebar')}
              onClick={toggleSidebar}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M15 6l-6 6 6 6" />
              </svg>
            </button>
          </div>

          <nav className="tv-nav">
            <div className="tv-nav-sec">{t('Workspace')}</div>
            {workspaceNav.map(renderItem)}
            <div className="tv-nav-sec">{t('System')}</div>
            {systemNav.map(renderItem)}
            <div className="tv-nav-sec">{t('Help')}</div>
            {helpNav.map(renderItem)}
          </nav>

          <div className="tv-side-foot">
            <span className={`tv-dot${agentOnline ? '' : ' off'}`} />
            <span className="tv-lbl">{version ? `TabVM v${version}` : 'TabVM'}</span>
          </div>
        </aside>

        <div className="tv-main">
          <div className="tv-topbar">
            <div className="tv-crumb">
              <span>{t('workspace')}</span>
              <span className="sep">/</span>
              <b>{t(crumb)}</b>
            </div>
            <div className="tv-top-actions">
              {!agentOnline && (
                <span className="tv-live off">
                  <span className="tv-dot off" />
                  {t('agent offline')}
                </span>
              )}
              <button type="button" className="tv-iconbtn" aria-label={t('Toggle theme')} onClick={toggleTheme}>
                <svg className="tv-moon" viewBox="0 0 24 24" fill="none" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
                </svg>
                <svg className="tv-sun" viewBox="0 0 24 24" fill="none" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="4.2" />
                  <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
                </svg>
              </button>
              <button
                type="button"
                className="tv-langbtn"
                aria-label={t('Switch language')}
                title={t('Switch language')}
                onClick={toggleLang}
              >
                <span className={lang === 'en' ? 'on' : ''}>EN</span>
                <span className="div">/</span>
                <span className={lang === 'es' ? 'on' : ''}>ES</span>
              </button>
              <div className="tv-accent" ref={accentRef}>
                <button
                  type="button"
                  className="tv-iconbtn"
                  aria-label={t('Accent color')}
                  title={t('Accent color')}
                  aria-haspopup="true"
                  aria-expanded={accentOpen}
                  onClick={() => setAccentOpen((o) => !o)}
                >
                  <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                    {/* handle strokes with the icon color; the bristle dab is
                        filled with the live accent so the brush carries the
                        currently selected "ink". */}
                    <path d="m9.06 11.9 8.07-8.06a2.85 2.85 0 1 1 4.03 4.03l-8.06 8.08" stroke="currentColor" />
                    <path
                      className="tv-brush-ink"
                      d="M7.07 14.94c-1.66 0-3 1.35-3 3.02 0 1.33-2.5 1.52-2 2.02 1.08 1.1 2.49 2.02 4 2.02 2.2 0 4-1.8 4-4.04a3.01 3.01 0 0 0-3-3.02z"
                    />
                  </svg>
                </button>
                {accentOpen && (
                  <div className="tv-accent-pop" role="menu" aria-label={t('Accent color')}>
                    {ACCENTS.map((a) => (
                      <button
                        key={a.id}
                        type="button"
                        role="menuitemradio"
                        aria-checked={accent === a.id}
                        className={`tv-swatch ${accent === a.id ? 'on' : ''}`}
                        title={t(a.label)}
                        aria-label={t(a.label)}
                        onClick={() => chooseAccent(a.id)}
                      >
                        <span className="dot" style={{ background: a.swatch }} />
                        <span className="name">{t(a.label)}</span>
                        {accent === a.id && (
                          <svg className="chk" viewBox="0 0 24 24" fill="none" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M5 12l5 5L20 7" />
                          </svg>
                        )}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>

          <div className="tv-content">
            {update && <UpdateBanner status={update} />}
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}
