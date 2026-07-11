import { useEffect, useRef, useState } from 'react';
import { BrandMark } from './BrandMark';

interface SplashScreenProps {
  // Called once, after the intro has typed the slogan, held, and faded out.
  onDone: () => void;
  slogan?: string;
  // Delay before typing starts, covering the mark's assemble animation.
  startDelayMs?: number;
  typeSpeedMs?: number;
  // Pause after the slogan completes, before the fade.
  holdMs?: number;
  fadeMs?: number;
}

const DEFAULT_SLOGAN = 'every VM. one tab.';

function prefersReducedMotion(): boolean {
  return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
}

// SplashScreen is the launch intro: the animated brand mark centered on the
// canvas, with the slogan revealed by a console-style typing effect, then a
// fade-out that hands off to the dashboard already mounted underneath.
export function SplashScreen({
  onDone,
  slogan = DEFAULT_SLOGAN,
  startDelayMs = 1100,
  typeSpeedMs = 70,
  holdMs = 1100,
  fadeMs = 500,
}: SplashScreenProps) {
  const [typed, setTyped] = useState('');
  const [fading, setFading] = useState(false);
  const doneRef = useRef(false);

  useEffect(() => {
    const timeouts: ReturnType<typeof setTimeout>[] = [];
    let interval: ReturnType<typeof setInterval> | undefined;

    const finish = () => {
      if (doneRef.current) return;
      doneRef.current = true;
      onDone();
    };
    const beginFade = () => {
      setFading(true);
      timeouts.push(setTimeout(finish, fadeMs));
    };

    if (prefersReducedMotion()) {
      setTyped(slogan);
      timeouts.push(setTimeout(beginFade, holdMs));
    } else {
      timeouts.push(
        setTimeout(() => {
          let i = 0;
          interval = setInterval(() => {
            i += 1;
            setTyped(slogan.slice(0, i));
            if (i >= slogan.length) {
              if (interval) clearInterval(interval);
              timeouts.push(setTimeout(beginFade, holdMs));
            }
          }, typeSpeedMs);
        }, startDelayMs),
      );
    }

    return () => {
      timeouts.forEach(clearTimeout);
      if (interval) clearInterval(interval);
    };
  }, [slogan, startDelayMs, typeSpeedMs, holdMs, fadeMs, onDone]);

  return (
    <div className={`tv-splash${fading ? ' tv-splash--out' : ''}`} role="status" aria-label="TabVM">
      <div className="tv-splash-inner">
        <BrandMark animated className="tv-splash-mark" />
        <div className="tv-splash-word">
          Tab<i>VM</i>
        </div>
        <div className="tv-splash-slogan">
          <span className="tv-splash-type">{typed}</span>
          <span className="tv-splash-caret" aria-hidden="true" />
        </div>
      </div>
    </div>
  );
}
