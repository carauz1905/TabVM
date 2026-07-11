import { useState, type ReactNode } from 'react';
import { ACCENTS, applyAccent, applyTheme, resolveDark, type Accent } from '../AppShell';
import { useLang } from '../../i18n/i18n';

// DemoStage frames a demo in the same window chrome the app uses, so the demo
// reads as a piece of TabVM rather than a diagram. An optional replay control
// restarts scripted animations.
export function DemoStage({
  caption,
  replayLabel,
  onReplay,
  children,
}: {
  caption?: string;
  replayLabel?: string;
  onReplay?: () => void;
  children: ReactNode;
}) {
  return (
    <figure className="docs-stage">
      <div className="docs-stage-bar">
        <span className="docs-dots" aria-hidden="true">
          <i />
          <i />
          <i />
        </span>
        {caption && <span className="docs-stage-cap">{caption}</span>}
        {onReplay && (
          <button type="button" className="docs-replay" onClick={onReplay}>
            <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M3 12a9 9 0 1 0 3-6.7L3 8" />
              <path d="M3 3v5h5" />
            </svg>
            {replayLabel ?? 'Replay'}
          </button>
        )}
      </div>
      <div className="docs-stage-body">{children}</div>
    </figure>
  );
}

// StartStopDemo is a real, clickable mock of a VM row. Its start/stop button uses
// the actual app classes and flips a local state — no VirtualBox is touched.
export function StartStopDemo({ name, labels }: { name: string; labels: { start: string; stop: string; starting: string; running: string; stopped: string } }) {
  const [state, setState] = useState<'stopped' | 'starting' | 'running'>('stopped');

  const toggle = () => {
    if (state === 'stopped') {
      setState('starting');
      window.setTimeout(() => setState('running'), 1100);
    } else {
      setState('stopped');
    }
  };

  const cls = state === 'running' ? 'running' : state === 'starting' ? 'booting' : 'stopped';

  return (
    <div className={`tv-vm ${cls} docs-vm`}>
      <div className="tv-vm-main">
        <div className="tv-vm-id">
          <span className={`tv-state-badge ${cls}`}>
            {state === 'running' ? labels.running : state === 'starting' ? labels.starting : labels.stopped}
          </span>
          <span className="tv-vm-name">{name}</span>
        </div>
      </div>
      <div className="tv-vm-actions">
        {state === 'running' ? (
          <button type="button" className="tv-abtn" onClick={toggle}>
            {labels.stop}
          </button>
        ) : (
          <button type="button" className="tv-abtn go" disabled={state === 'starting'} onClick={toggle}>
            {state === 'starting' ? labels.starting : labels.start}
          </button>
        )}
      </div>
    </div>
  );
}

// ConsoleBootDemo is a scripted animation: a small screen that powers on, shows a
// cursor prompt, then "logs in". Replay restarts it by remounting on a key.
export function ConsoleBootDemo({ playKey }: { playKey: number }) {
  return (
    <div className="docs-screen" key={playKey}>
      <div className="docs-screen-glow" />
      <div className="docs-boot">
        <span className="docs-boot-logo">▚</span>
        <span className="docs-boot-line l1">booting guest…</span>
        <span className="docs-boot-line l2">guest additions: active</span>
        <span className="docs-boot-line l3">
          user@lab:~$ <i className="docs-caret" />
        </span>
      </div>
    </div>
  );
}

// TerminalDemo scripts a serial login on a dark screen: a boot line, a login
// prompt, then a command and its output — the accent tints the prompt and caret.
// It remounts on playKey to replay.
export function TerminalDemo({ playKey }: { playKey: number }) {
  return (
    <div className="docs-term" key={playKey}>
      <div className="docs-term-line l1">
        <span className="docs-term-dim">▚</span> opening /dev/ttyS0
      </div>
      <div className="docs-term-line l2">
        localhost login: <span className="docs-term-accent">root</span>
      </div>
      <div className="docs-term-line l3">
        <span className="docs-term-accent">localhost:~#</span> ls
      </div>
      <div className="docs-term-line l4">bin   etc   home   root   usr</div>
      <div className="docs-term-line l5">
        <span className="docs-term-accent">localhost:~#</span> <i className="docs-term-caret" />
      </div>
    </div>
  );
}

// ShareDropDemo scripts a file card flying from the host into a VM tile.
export function ShareDropDemo({ playKey, dropLabel }: { playKey: number; dropLabel: string }) {
  return (
    <div className="docs-drop" key={playKey}>
      <div className="docs-drop-file">
        <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
          <path d="M14 3v5h5" />
        </svg>
        report.pdf
      </div>
      <div className="docs-drop-target">
        <span>{dropLabel}</span>
      </div>
    </div>
  );
}

// SnapshotDemo scripts a small tree: take a snapshot, then a restore pulse.
export function SnapshotDemo({ playKey, takeLabel, restoreLabel }: { playKey: number; takeLabel: string; restoreLabel: string }) {
  return (
    <div className="docs-snap" key={playKey}>
      <div className="docs-snap-node base">base</div>
      <div className="docs-snap-link" />
      <div className="docs-snap-node snap">{takeLabel}</div>
      <div className="docs-snap-restore">{restoreLabel}</div>
    </div>
  );
}

// HardwareDemo scripts the hardware edit: the vCPU and memory readouts step up
// from their old values to the new ones, then an "applied" badge flashes. It
// remounts on playKey to replay.
export function HardwareDemo({
  playKey,
  labels,
}: {
  playKey: number;
  labels: { cpu: string; memory: string; apply: string; applied: string };
}) {
  return (
    <div className="docs-hw" key={playKey}>
      <div className="docs-hw-field">
        <span className="docs-hw-label">{labels.cpu}</span>
        <span className="docs-hw-value">
          <span className="docs-hw-from">2</span>
          <span className="docs-hw-arrow" aria-hidden="true">→</span>
          <span className="docs-hw-to">4</span>
        </span>
      </div>
      <div className="docs-hw-field">
        <span className="docs-hw-label">{labels.memory}</span>
        <span className="docs-hw-value">
          <span className="docs-hw-from">2048</span>
          <span className="docs-hw-arrow" aria-hidden="true">→</span>
          <span className="docs-hw-to">4096</span>
        </span>
      </div>
      <div className="docs-hw-apply">
        <span className="docs-hw-btn">{labels.apply}</span>
        <span className="docs-hw-done">{labels.applied}</span>
      </div>
    </div>
  );
}

// StorageDemo scripts a disk growing: a capacity bar extends from an old size to
// a larger one while the number counts up, then a done badge flashes. It
// remounts on playKey to replay.
export function StorageDemo({
  playKey,
  labels,
}: {
  playKey: number;
  labels: { disk: string; resize: string; done: string };
}) {
  return (
    <div className="docs-store" key={playKey}>
      <div className="docs-store-row">
        <span className="docs-store-name">{labels.disk}</span>
        <span className="docs-store-size">
          <span className="docs-store-from">10 GB</span>
          <span className="docs-store-arrow" aria-hidden="true">→</span>
          <span className="docs-store-to">20 GB</span>
        </span>
      </div>
      <div className="docs-store-bar">
        <div className="docs-store-fill" />
      </div>
      <div className="docs-store-apply">
        <span className="docs-store-btn">{labels.resize}</span>
        <span className="docs-store-done">{labels.done}</span>
      </div>
    </div>
  );
}

// StorageAddDemo scripts a new disk attaching to a free SATA port: two ports sit
// filled, the third (dashed and empty) fills with the accent as the disk lands.
// It mirrors the real port model the Storage panel works with.
export function StorageAddDemo({ playKey, labels }: { playKey: number; labels: { free: string; attached: string } }) {
  return (
    <div className="docs-addport" key={playKey}>
      <div className="docs-addport-strip">
        <span className="docs-addport-slot on">SATA 0</span>
        <span className="docs-addport-slot on">SATA 1</span>
        <span className="docs-addport-slot new">SATA 2</span>
      </div>
      <div className="docs-addport-caption">
        <span className="docs-addport-free">{labels.free}</span>
        <span className="docs-addport-attached">{labels.attached}</span>
      </div>
    </div>
  );
}

// StorageRemoveDemo contrasts the two removal paths so the reversible/irreversible
// distinction reads at a glance: detach sends the disk away but the file stays;
// delete erases the file. The delete side carries the app's danger red, thinly.
export function StorageRemoveDemo({ labels }: { labels: { detach: string; delete: string; kept: string; erased: string } }) {
  return (
    <div className="docs-rm">
      <div className="docs-rm-card">
        <span className="docs-rm-label">{labels.detach}</span>
        <span className="docs-rm-flow">
          <span className="docs-rm-disk leaving">disk</span>
          <span className="docs-rm-file">{labels.kept}</span>
        </span>
      </div>
      <div className="docs-rm-card danger">
        <span className="docs-rm-label">{labels.delete}</span>
        <span className="docs-rm-flow">
          <span className="docs-rm-disk leaving">disk</span>
          <span className="docs-rm-file erasing">{labels.erased}</span>
        </span>
      </div>
    </div>
  );
}

// NetworkModeDemo is a real segmented control the reader can toggle, mirroring the
// Network panel's mode switch.
export function NetworkModeDemo({ modes }: { modes: { id: string; label: string }[] }) {
  const [active, setActive] = useState(modes[0]?.id ?? 'nat');
  return (
    <div className="docs-seg" role="tablist">
      {modes.map((m) => (
        <button
          key={m.id}
          type="button"
          role="tab"
          aria-selected={active === m.id}
          className={`docs-seg-btn ${active === m.id ? 'on' : ''}`}
          onClick={() => setActive(m.id)}
        >
          {m.label}
        </button>
      ))}
    </div>
  );
}

// WizardDemo scripts the create flow: two tabs and a filling progress bar.
export function WizardDemo({ playKey, importLabel, installLabel, workingLabel, doneLabel }: { playKey: number; importLabel: string; installLabel: string; workingLabel: string; doneLabel: string }) {
  return (
    <div className="docs-wiz" key={playKey}>
      <div className="docs-wiz-tabs">
        <span className="docs-wiz-tab on">{importLabel}</span>
        <span className="docs-wiz-tab">{installLabel}</span>
      </div>
      <div className="docs-wiz-bar">
        <div className="docs-wiz-fill" />
      </div>
      <div className="docs-wiz-status">
        <span className="docs-wiz-working">{workingLabel}</span>
        <span className="docs-wiz-done">{doneLabel}</span>
      </div>
    </div>
  );
}

// PersonalizeDemo exposes the real theme, language and accent controls. Changing
// them here changes the whole app live — the most honest possible demonstration.
export function PersonalizeDemo({ labels }: { labels: { light: string; dark: string; accent: string } }) {
  const { lang, toggle: toggleLang } = useLang();
  const [dark, setDark] = useState(resolveDark());
  const [accent, setAccent] = useState<Accent>('teal');

  const flipTheme = () => {
    const next = dark ? 'light' : 'dark';
    applyTheme(next);
    setDark(!dark);
  };
  const chooseAccent = (a: Accent) => {
    applyAccent(a);
    setAccent(a);
  };

  return (
    <div className="docs-personalize">
      <button type="button" className="tv-btn" onClick={flipTheme}>
        {dark ? labels.light : labels.dark}
      </button>
      <button type="button" className="tv-langbtn" onClick={toggleLang}>
        <span className={lang === 'en' ? 'on' : ''}>EN</span>
        <span className="div">/</span>
        <span className={lang === 'es' ? 'on' : ''}>ES</span>
      </button>
      <div className="docs-accent-row" aria-label={labels.accent}>
        {ACCENTS.map((a) => (
          <button
            key={a.id}
            type="button"
            className={`docs-swatch ${accent === a.id ? 'on' : ''}`}
            style={{ background: a.swatch }}
            aria-label={a.label}
            onClick={() => chooseAccent(a.id)}
          />
        ))}
      </div>
    </div>
  );
}
