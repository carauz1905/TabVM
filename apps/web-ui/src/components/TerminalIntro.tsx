import { useEffect, useState } from 'react';
import { BrandMark } from './BrandMark';

// TerminalIntro is the intro painted over the terminal tab's dark surface: the
// animated TabVM brand mark (the real SVG, with its draw-in animation and the
// brand accent) centered on the terminal background, then a fade-out that hands
// off to the live terminal underneath.
export function TerminalIntro({ onDone }: { onDone: () => void }) {
  const [fading, setFading] = useState(false);

  useEffect(() => {
    const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
    const hold = reduce ? 300 : 1650;
    const t1 = window.setTimeout(() => setFading(true), hold);
    const t2 = window.setTimeout(onDone, hold + 450);
    return () => {
      window.clearTimeout(t1);
      window.clearTimeout(t2);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className={`tv-tintro ${fading ? 'fade' : ''}`} aria-hidden="true">
      <BrandMark animated className="tv-tintro-mark" />
      <div className="tv-tintro-word">TabVM</div>
      <div className="tv-tintro-slogan">every VM. one tab.</div>
    </div>
  );
}
