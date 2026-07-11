import { useEffect, useState } from 'react';

// TerminalBoot is the intro painted when the terminal tab opens: a short,
// console-style boot sequence typed line by line on the dark surface, instead of
// the brand splash. Accent-colored markers keep it responsive to the user's
// chosen accent. It fades out and hands off to the live terminal underneath.
export function TerminalBoot({ vmName, onDone }: { vmName: string; onDone: () => void }) {
  const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;

  const lines: { mark: string; text: string; cls: string }[] = [
    { mark: '', text: 'TabVM · serial console', cls: 'head' },
    { mark: '▚', text: ' opening /dev/ttyS0', cls: 'step' },
    { mark: '▚', text: ` bridge 127.0.0.1 ⇄ ${vmName}`, cls: 'step' },
    { mark: '[ ok ]', text: ' link up — attaching terminal', cls: 'ok' },
  ];

  const [shown, setShown] = useState(reduce ? lines.length : 0);
  const [fading, setFading] = useState(false);

  useEffect(() => {
    const step = 200;
    const timers: number[] = [];
    if (!reduce) {
      lines.forEach((_, i) => {
        timers.push(window.setTimeout(() => setShown(i + 1), step * (i + 1)));
      });
    }
    const total = reduce ? 500 : step * lines.length + 450;
    timers.push(window.setTimeout(() => setFading(true), total));
    timers.push(window.setTimeout(onDone, total + 400));
    return () => timers.forEach((id) => window.clearTimeout(id));
    // Run once on mount; vmName is stable for the tab's lifetime.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className={`tv-tboot ${fading ? 'fade' : ''}`} aria-hidden="true">
      <div className="tv-tboot-inner">
        {lines.slice(0, shown).map((l, i) => (
          <div key={i} className={`tv-tboot-line ${l.cls}`}>
            {l.mark && <span className="tv-tboot-mark">{l.mark}</span>}
            {l.text}
          </div>
        ))}
        {shown < lines.length && <span className="tv-tboot-caret" />}
      </div>
    </div>
  );
}
