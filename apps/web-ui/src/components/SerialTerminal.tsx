import { useEffect, useRef } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { serialStreamUrl } from '../api/client';

export type SerialStatus = 'connecting' | 'open' | 'closed' | 'error';

// SerialTerminal renders an xterm.js terminal bound to a VM's serial console
// WebSocket (/serial-stream). Bytes from the guest are written to the terminal;
// keystrokes are sent back as UTF-8 to the guest's COM1 login TTY. The terminal
// emulation lives entirely here — the agent only pumps raw bytes.
export function SerialTerminal({
  vmId,
  onStatus,
  onData,
}: {
  vmId: string;
  onStatus?: (status: SerialStatus) => void;
  // Fired when any bytes arrive from the guest, so the parent can tell a live
  // session (a login prompt or shell replies) from a silent one (no getty).
  onData?: () => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const styles = getComputedStyle(document.documentElement);
    const cssVar = (name: string, fallback: string) =>
      styles.getPropertyValue(name).trim() || fallback;

    const term = new Terminal({
      cursorBlink: true,
      fontFamily: cssVar('--font-mono', 'monospace'),
      fontSize: 13,
      theme: {
        background: cssVar('--bg-primary', '#0b0f14'),
        foreground: cssVar('--text-primary', '#d8dee9'),
        cursor: cssVar('--accent', '#7cc9c2'),
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(container);
    fit.fit();
    term.focus();

    onStatus?.('connecting');
    const socket = new WebSocket(serialStreamUrl(vmId));
    socket.binaryType = 'arraybuffer';

    socket.onopen = () => {
      onStatus?.('open');
      // Nudge the getty to (re)print its login prompt.
      socket.send(new Uint8Array([0x0d]));
    };
    socket.onmessage = (event) => {
      onData?.();
      if (typeof event.data === 'string') {
        term.write(event.data);
      } else {
        term.write(new Uint8Array(event.data as ArrayBuffer));
      }
    };
    socket.onclose = () => {
      onStatus?.('closed');
      term.write('\r\n\x1b[2m[disconnected]\x1b[0m\r\n');
    };
    socket.onerror = () => onStatus?.('error');

    const encoder = new TextEncoder();
    const dataDisposable = term.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(encoder.encode(data));
      }
    });

    // Serial lines carry no in-band window-size signal, so a resize only reflows
    // the local view; the guest getty keeps its own idea of the size until a
    // `stty`/`resize` is run inside it. Refit anyway so the view fills the panel.
    const observer = new ResizeObserver(() => {
      try {
        fit.fit();
      } catch {
        // fit can throw if the element is detached mid-teardown; ignore.
      }
    });
    observer.observe(container);

    return () => {
      observer.disconnect();
      dataDisposable.dispose();
      socket.close();
      term.dispose();
    };
  }, [vmId, onStatus, onData]);

  return <div ref={containerRef} className="tv-serial-term" />;
}
