import { useEffect, useRef, useState } from 'react';
import { useT } from '../../i18n/i18n';
import { BrandMark } from '../BrandMark';
import { useDocs } from './content';
import {
  ConsoleBootDemo,
  DemoStage,
  HardwareDemo,
  NetworkModeDemo,
  PersonalizeDemo,
  ShareDropDemo,
  SnapshotDemo,
  StartStopDemo,
  StorageDemo,
  StorageAddDemo,
  StorageRemoveDemo,
  TerminalDemo,
  WizardDemo,
} from './Demos';

type SectionId =
  | 'welcome'
  | 'quickStart'
  | 'operate'
  | 'terminal'
  | 'hardware'
  | 'storage'
  | 'files'
  | 'snapshots'
  | 'network'
  | 'create'
  | 'personalize'
  | 'troubleshoot';

const ORDER: SectionId[] = [
  'welcome',
  'quickStart',
  'operate',
  'terminal',
  'hardware',
  'storage',
  'files',
  'snapshots',
  'network',
  'create',
  'personalize',
  'troubleshoot',
];

// DocsView is the in-app user manual. It inherits theme, language and accent from
// the app, and demonstrates features with the real controls and scripted
// animations rather than screenshots.
export function DocsView() {
  const d = useDocs();
  const { t } = useT();
  const [active, setActive] = useState<SectionId>('welcome');
  // Replay counters remount scripted-animation demos to restart them.
  const [plays, setPlays] = useState<Record<string, number>>({});
  const replay = (key: string) => setPlays((p) => ({ ...p, [key]: (p[key] ?? 0) + 1 }));
  const play = (key: string) => plays[key] ?? 0;

  const refs = useRef<Record<string, HTMLElement | null>>({});

  // Scroll-spy: highlight the section nearest the top of the scroll area.
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
        if (visible) setActive(visible.target.id as SectionId);
      },
      { rootMargin: '-20% 0px -70% 0px', threshold: 0 },
    );
    ORDER.forEach((id) => {
      const el = refs.current[id];
      if (el) observer.observe(el);
    });
    return () => observer.disconnect();
  }, []);

  const go = (id: SectionId) => refs.current[id]?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  const setRef = (id: SectionId) => (el: HTMLElement | null) => {
    refs.current[id] = el;
  };

  return (
    <div className="docs">
      <aside className="docs-nav" aria-label={d.sectionNav}>
        <div className="docs-nav-label">{d.sectionNav}</div>
        <ul>
          {ORDER.map((id) => (
            <li key={id}>
              <button
                type="button"
                className={`docs-nav-link ${active === id ? 'on' : ''}`}
                aria-current={active === id ? 'true' : undefined}
                onClick={() => go(id)}
              >
                {d.sections[id]}
              </button>
            </li>
          ))}
        </ul>
      </aside>

      <div className="docs-main">
        {/* Welcome */}
        <section id="welcome" ref={setRef('welcome')} className="docs-sec docs-hero tv-rise">
          <BrandMark className="docs-hero-mark" />
          <h1>{d.sections.welcome}</h1>
          <p className="docs-tag">{d.tagline}</p>
          <p className="docs-lead">{d.welcome.lead}</p>
          <div className="docs-cards">
            {d.welcome.cards.map((c, i) => (
              <div className="docs-card" style={{ '--i': i } as React.CSSProperties} key={c.title}>
                <span className="docs-card-dot" />
                <h4>{c.title}</h4>
                <p>{c.body}</p>
              </div>
            ))}
          </div>
          <p className="docs-hint">{d.welcome.tryHint}</p>
        </section>

        {/* Quick start */}
        <section id="quickStart" ref={setRef('quickStart')} className="docs-sec">
          <h2>{d.sections.quickStart}</h2>
          <p className="docs-lead">{d.quickStart.lead}</p>
          <ol className="docs-steps">
            {d.quickStart.steps.map((s, i) => (
              <li key={s.title} style={{ '--i': i } as React.CSSProperties}>
                <span className="docs-step-n">{i + 1}</span>
                <div>
                  <h4>{s.title}</h4>
                  <p>{s.body}</p>
                </div>
              </li>
            ))}
          </ol>
        </section>

        {/* Operate */}
        <section id="operate" ref={setRef('operate')} className="docs-sec">
          <h2>{d.sections.operate}</h2>
          <p className="docs-lead">{d.operate.lead}</p>
          <h4>{d.operate.startStop.title}</h4>
          <p>{d.operate.startStop.body}</p>
          <DemoStage caption="lab-vm">
            <StartStopDemo
              name="lab-vm"
              labels={{
                start: t('start'),
                stop: t('stop'),
                starting: t('starting…'),
                running: t('active'),
                stopped: t('off'),
              }}
            />
          </DemoStage>
          <h4>{d.operate.console.title}</h4>
          <p>{d.operate.console.body}</p>
          <DemoStage caption="console" replayLabel={t('Refresh')} onReplay={() => replay('boot')}>
            <ConsoleBootDemo playKey={play('boot')} />
          </DemoStage>
          <p className="docs-tip">{d.operate.tip}</p>
        </section>

        {/* Terminal */}
        <section id="terminal" ref={setRef('terminal')} className="docs-sec">
          <h2>{d.sections.terminal}</h2>
          <p className="docs-lead">{d.terminal.lead}</p>
          <h4>{d.terminal.enable.title}</h4>
          <p>{d.terminal.enable.body}</p>
          <h4>{d.terminal.open.title}</h4>
          <p>{d.terminal.open.body}</p>
          <DemoStage caption="serial · Linux" replayLabel={t('Refresh')} onReplay={() => replay('term')}>
            <TerminalDemo playKey={play('term')} />
          </DemoStage>
          <h4>{d.terminal.activate.title}</h4>
          <p>{d.terminal.activate.body}</p>
          <TipBox label={d.tipLabel}>{d.terminal.tip}</TipBox>
        </section>

        {/* Hardware */}
        <section id="hardware" ref={setRef('hardware')} className="docs-sec">
          <h2>{d.sections.hardware}</h2>
          <p className="docs-lead">{d.hardware.lead}</p>
          <h4>{d.hardware.edit.title}</h4>
          <p>{d.hardware.edit.body}</p>
          <h4>{d.hardware.limits.title}</h4>
          <p>{d.hardware.limits.body}</p>
          <DemoStage caption="vCPU · memory" replayLabel={t('Refresh')} onReplay={() => replay('hw')}>
            <HardwareDemo
              playKey={play('hw')}
              labels={{
                cpu: t('vCPU'),
                memory: t('Memory (MB)'),
                apply: t('Apply'),
                applied: t('active'),
              }}
            />
          </DemoStage>
          <TipBox label={d.tipLabel}>{d.hardware.tip}</TipBox>
        </section>

        {/* Storage */}
        <section id="storage" ref={setRef('storage')} className="docs-sec">
          <h2>{d.sections.storage}</h2>
          <p className="docs-lead">{d.storage.lead}</p>
          <h4>{d.storage.resize.title}</h4>
          <p>{d.storage.resize.body}</p>
          <DemoStage caption="disk size" replayLabel={t('Refresh')} onReplay={() => replay('store')}>
            <StorageDemo
              playKey={play('store')}
              labels={{ disk: 'disk1.vdi', resize: t('Resize'), done: t('active') }}
            />
          </DemoStage>
          <h4>{d.storage.add.title}</h4>
          <p>{d.storage.add.body}</p>
          <DemoStage caption="SATA ports" replayLabel={t('Refresh')} onReplay={() => replay('addport')}>
            <StorageAddDemo
              playKey={play('addport')}
              labels={{ free: t('free port'), attached: t('new disk attached') }}
            />
          </DemoStage>
          <h4>{d.storage.optical.title}</h4>
          <p>{d.storage.optical.body}</p>
          <h4>{d.storage.remove.title}</h4>
          <p>{d.storage.remove.body}</p>
          <DemoStage caption="detach vs delete">
            <StorageRemoveDemo
              labels={{
                detach: t('Detach'),
                delete: t('Delete'),
                kept: t('file kept'),
                erased: t('file erased'),
              }}
            />
          </DemoStage>
          <h4>{d.storage.limits.title}</h4>
          <p>{d.storage.limits.body}</p>
          <TipBox label={d.tipLabel}>{d.storage.tip}</TipBox>
        </section>

        {/* Files */}
        <section id="files" ref={setRef('files')} className="docs-sec">
          <h2>{d.sections.files}</h2>
          <p className="docs-lead">{d.files.lead}</p>
          <h4>{d.files.share.title}</h4>
          <p>{d.files.share.body}</p>
          <h4>{d.files.drop.title}</h4>
          <p>{d.files.drop.body}</p>
          <DemoStage caption="drag & drop">
            <ShareDropDemo playKey={play('drop')} dropLabel={t('Drop files to send to the VM')} />
          </DemoStage>
          <TipBox label={d.tipLabel}>{d.files.tip}</TipBox>
        </section>

        {/* Snapshots */}
        <section id="snapshots" ref={setRef('snapshots')} className="docs-sec">
          <h2>{d.sections.snapshots}</h2>
          <p className="docs-lead">{d.snapshots.lead}</p>
          <h4>{d.snapshots.take.title}</h4>
          <p>{d.snapshots.take.body}</p>
          <h4>{d.snapshots.restore.title}</h4>
          <p>{d.snapshots.restore.body}</p>
          <DemoStage caption="snapshots">
            <SnapshotDemo playKey={play('snap')} takeLabel={t('current')} restoreLabel={t('restore')} />
          </DemoStage>
          <TipBox label={d.tipLabel}>{d.snapshots.tip}</TipBox>
        </section>

        {/* Network */}
        <section id="network" ref={setRef('network')} className="docs-sec">
          <h2>{d.sections.network}</h2>
          <p className="docs-lead">{d.network.lead}</p>
          <DemoStage caption="adapter mode">
            <NetworkModeDemo
              modes={d.network.modes.map((m) => ({ id: m.title, label: m.title }))}
            />
          </DemoStage>
          <dl className="docs-defs">
            {d.network.modes.map((m) => (
              <div key={m.title}>
                <dt>{m.title}</dt>
                <dd>{m.body}</dd>
              </div>
            ))}
          </dl>
          <h3>{d.network.forwarding.title}</h3>
          <p>{d.network.forwarding.body}</p>
          <TipBox label={d.tipLabel}>{d.network.tip}</TipBox>
        </section>

        {/* Create */}
        <section id="create" ref={setRef('create')} className="docs-sec">
          <h2>{d.sections.create}</h2>
          <p className="docs-lead">{d.create.lead}</p>
          <div className="docs-two">
            <div className="docs-card">
              <h4>{d.create.importCard.title}</h4>
              <p>{d.create.importCard.body}</p>
            </div>
            <div className="docs-card">
              <h4>{d.create.installCard.title}</h4>
              <p>{d.create.installCard.body}</p>
            </div>
            <div className="docs-card">
              <h4>{d.create.manualCard.title}</h4>
              <p>{d.create.manualCard.body}</p>
            </div>
            <div className="docs-card">
              <h4>{d.create.cloneCard.title}</h4>
              <p>{d.create.cloneCard.body}</p>
            </div>
            <div className="docs-card">
              <h4>{d.create.exportCard.title}</h4>
              <p>{d.create.exportCard.body}</p>
            </div>
          </div>
          <DemoStage caption={t('New virtual machine')}>
            <WizardDemo
              playKey={play('wiz')}
              importLabel={t('Import image (.ova)')}
              installLabel={t('Install from ISO')}
              manualLabel={t('Other OS (manual install)')}
              workingLabel={t('Creating the VM and preparing the automated install…')}
              doneLabel={t('The VM is ready. Start it from the list.')}
            />
          </DemoStage>
          <p className="docs-tip">{d.create.note}</p>
        </section>

        {/* Personalize */}
        <section id="personalize" ref={setRef('personalize')} className="docs-sec">
          <h2>{d.sections.personalize}</h2>
          <p className="docs-lead">{d.personalize.lead}</p>
          <div className="docs-defs">
            <div>
              <dt>{d.personalize.theme.title}</dt>
              <dd>{d.personalize.theme.body}</dd>
            </div>
            <div>
              <dt>{d.personalize.lang.title}</dt>
              <dd>{d.personalize.lang.body}</dd>
            </div>
            <div>
              <dt>{d.personalize.accent.title}</dt>
              <dd>{d.personalize.accent.body}</dd>
            </div>
          </div>
          <p className="docs-hint">{d.personalize.tryHint}</p>
          <DemoStage caption="live">
            <PersonalizeDemo
              labels={{ light: t('Toggle theme'), dark: t('Toggle theme'), accent: t('Accent color') }}
            />
          </DemoStage>
        </section>

        {/* Troubleshoot */}
        <section id="troubleshoot" ref={setRef('troubleshoot')} className="docs-sec">
          <h2>{d.sections.troubleshoot}</h2>
          <p className="docs-lead">{d.troubleshoot.lead}</p>

          <h4>{d.troubleshoot.gaManual.title}</h4>
          <p>{d.troubleshoot.gaManual.intro}</p>
          <pre className="docs-code">
            <code>
              {d.troubleshoot.gaManual.commands.map((c) => (
                <span className="docs-code-line" key={c}>
                  <span className="docs-code-prompt">$</span>
                  {c}
                </span>
              ))}
            </code>
          </pre>
          <p>{d.troubleshoot.gaManual.after}</p>

          <div className="docs-faq">
            {d.troubleshoot.faqs.map((f) => (
              <details key={f.q}>
                <summary>{f.q}</summary>
                <p>{f.a}</p>
              </details>
            ))}
          </div>
        </section>

        <CreditsSignature by={d.credits.by} name={d.credits.name} />
      </div>
    </div>
  );
}

// TipBox is a labelled callout that highlights a practical tip inside a section.
function TipBox({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <aside className="docs-tipbox" role="note">
      <span className="docs-tipbox-badge">
        <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
          <path
            d="M9 18h6M10 21h4M12 3a6 6 0 0 0-4 10.5c.6.6 1 1.3 1 2.1V16h6v-.4c0-.8.4-1.5 1-2.1A6 6 0 0 0 12 3z"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.7"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        {label}
      </span>
      <p>{children}</p>
    </aside>
  );
}


// CreditsSignature closes the manual the way the app opens: the brand mark
// assembles itself, then a console prompt types the author line. It runs once,
// when scrolled into view, and the name lands in the accent color.
function CreditsSignature({ by, name }: { by: string; name: string }) {
  const full = `${by} ${name}`;
  const [armed, setArmed] = useState(false);
  const [count, setCount] = useState(0);
  const ref = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
      setArmed(true);
      setCount(full.length);
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          setArmed(true);
          observer.disconnect();
        }
      },
      { threshold: 0.9 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [full.length]);

  useEffect(() => {
    if (!armed || count >= full.length) return;
    // Let the mark finish assembling before the prompt starts typing,
    // matching the splash screen's choreography.
    let interval: ReturnType<typeof setInterval> | undefined;
    const start = setTimeout(() => {
      interval = setInterval(() => {
        setCount((c) => {
          if (c >= full.length) {
            if (interval) clearInterval(interval);
            return c;
          }
          return c + 1;
        });
      }, 70);
    }, 900);
    return () => {
      clearTimeout(start);
      if (interval) clearInterval(interval);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [armed, full.length]);

  // Reveal by slicing so the prefix stays muted and the name takes the accent.
  const prefixLen = by.length + 1;
  const typedPrefix = full.slice(0, Math.min(count, prefixLen));
  const typedName = count > prefixLen ? full.slice(prefixLen, count) : '';

  return (
    <footer ref={ref} className="docs-sign" aria-label={full}>
      {armed && <BrandMark animated className="docs-sign-mark" />}
      <span className="docs-sign-type" aria-hidden="true">
        <span className="docs-sign-prompt">&gt;</span>
        {typedPrefix}
        <span className="docs-sign-name">{typedName}</span>
        <span className="docs-sign-caret" />
      </span>
    </footer>
  );
}
