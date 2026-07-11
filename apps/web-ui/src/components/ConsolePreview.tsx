import { useEffect, useRef, useState } from 'react';
import { screenStreamUrl } from '../api/client';
import { paintTiles } from '../lib/screenTiles';
import { useT } from '../i18n/i18n';

interface ConsolePreviewProps {
  vmId: string;
  onOpen: () => void;
}

type PreviewState = 'connecting' | 'live' | 'error';

// ConsolePreview renders a real, read-only live thumbnail of the VM screen using
// the same COM screen-stream as the full console. It sends no input; clicking it
// opens the full interactive console. In jsdom (tests) getContext('2d') returns
// null, so the stream short-circuits and only the status label renders.
export function ConsolePreview({ vmId, onOpen }: ConsolePreviewProps) {
  const { t } = useT();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [state, setState] = useState<PreviewState>('connecting');

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const socket = new WebSocket(screenStreamUrl(vmId));
    socket.binaryType = 'arraybuffer';

    socket.addEventListener('open', () => setState('live'));
    socket.addEventListener('error', () => setState('error'));
    socket.addEventListener('close', () => setState((s) => (s === 'error' ? s : 'error')));
    socket.addEventListener('message', async (evt) => {
      if (typeof evt.data === 'string') {
        try {
          const msg = JSON.parse(evt.data);
          if (msg.type === 'resolution') {
            canvas.width = msg.width;
            canvas.height = msg.height;
          }
        } catch {
          // ignore malformed control frames
        }
        return;
      }
      try {
        await paintTiles(ctx, evt.data as ArrayBuffer);
        setState('live');
      } catch {
        // a bad tile batch should not tear down the preview
      }
    });

    return () => socket.close();
  }, [vmId]);

  return (
    <button type="button" className="tv-preview" onClick={onOpen} aria-label={t('Open live console')}>
      <canvas ref={canvasRef} width={1280} height={800} className="tv-preview-canvas" />
      {state !== 'live' && (
        <span className="tv-preview-msg">{state === 'error' ? t('console unavailable') : t('connecting…')}</span>
      )}
      <span className="tv-preview-hint">{t('click to attach keyboard + mouse')}</span>
    </button>
  );
}
