import { useCallback, useEffect, useRef, useState } from 'react';
import { api, screenStreamUrl } from '../api/client';
import type { VmTelemetryResponse } from '../types/api';
import { paintTiles } from '../lib/screenTiles';
import { GuestDropZone } from './GuestDropZone';
import { useT } from '../i18n/i18n';

// formatRam renders a configured memory size (MB) as GB when it divides evenly,
// otherwise as MB, matching how VirtualBox reports round memory allocations.
function formatRam(mb: number): string {
  if (mb <= 0) return '—';
  if (mb % 1024 === 0) return `${mb / 1024} GB`;
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${mb} MB`;
}

// formatBytes renders a byte count as GB (or MB below 1 GB) for the disk chips.
function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 GB';
  const gb = bytes / 1024 ** 3;
  if (gb >= 1) return `${gb.toFixed(gb >= 10 ? 0 : 1)} GB`;
  return `${Math.round(bytes / 1024 ** 2)} MB`;
}

// KeyboardEvent.code -> PC/AT scancode set 1. `ext` marks 0xE0-prefixed keys.
type ScanEntry = { code: number; ext?: boolean };
const SCANCODES: Record<string, ScanEntry> = {
  Escape: { code: 0x01 },
  Digit1: { code: 0x02 }, Digit2: { code: 0x03 }, Digit3: { code: 0x04 }, Digit4: { code: 0x05 },
  Digit5: { code: 0x06 }, Digit6: { code: 0x07 }, Digit7: { code: 0x08 }, Digit8: { code: 0x09 },
  Digit9: { code: 0x0a }, Digit0: { code: 0x0b }, Minus: { code: 0x0c }, Equal: { code: 0x0d },
  Backspace: { code: 0x0e }, Tab: { code: 0x0f },
  KeyQ: { code: 0x10 }, KeyW: { code: 0x11 }, KeyE: { code: 0x12 }, KeyR: { code: 0x13 },
  KeyT: { code: 0x14 }, KeyY: { code: 0x15 }, KeyU: { code: 0x16 }, KeyI: { code: 0x17 },
  KeyO: { code: 0x18 }, KeyP: { code: 0x19 }, BracketLeft: { code: 0x1a }, BracketRight: { code: 0x1b },
  Enter: { code: 0x1c }, ControlLeft: { code: 0x1d },
  KeyA: { code: 0x1e }, KeyS: { code: 0x1f }, KeyD: { code: 0x20 }, KeyF: { code: 0x21 },
  KeyG: { code: 0x22 }, KeyH: { code: 0x23 }, KeyJ: { code: 0x24 }, KeyK: { code: 0x25 },
  KeyL: { code: 0x26 }, Semicolon: { code: 0x27 }, Quote: { code: 0x28 }, Backquote: { code: 0x29 },
  ShiftLeft: { code: 0x2a }, Backslash: { code: 0x2b },
  KeyZ: { code: 0x2c }, KeyX: { code: 0x2d }, KeyC: { code: 0x2e }, KeyV: { code: 0x2f },
  KeyB: { code: 0x30 }, KeyN: { code: 0x31 }, KeyM: { code: 0x32 }, Comma: { code: 0x33 },
  Period: { code: 0x34 }, Slash: { code: 0x35 }, ShiftRight: { code: 0x36 },
  AltLeft: { code: 0x38 }, Space: { code: 0x39 }, CapsLock: { code: 0x3a },
  F1: { code: 0x3b }, F2: { code: 0x3c }, F3: { code: 0x3d }, F4: { code: 0x3e }, F5: { code: 0x3f },
  F6: { code: 0x40 }, F7: { code: 0x41 }, F8: { code: 0x42 }, F9: { code: 0x43 }, F10: { code: 0x44 },
  F11: { code: 0x57 }, F12: { code: 0x58 },
  ControlRight: { code: 0x1d, ext: true }, AltRight: { code: 0x38, ext: true },
  NumpadEnter: { code: 0x1c, ext: true }, NumpadDivide: { code: 0x35, ext: true },
  ArrowUp: { code: 0x48, ext: true }, ArrowDown: { code: 0x50, ext: true },
  ArrowLeft: { code: 0x4b, ext: true }, ArrowRight: { code: 0x4d, ext: true },
  Home: { code: 0x47, ext: true }, End: { code: 0x4f, ext: true },
  PageUp: { code: 0x49, ext: true }, PageDown: { code: 0x51, ext: true },
  Insert: { code: 0x52, ext: true }, Delete: { code: 0x53, ext: true },
  MetaLeft: { code: 0x5b, ext: true }, MetaRight: { code: 0x5c, ext: true },
};

// How often the console re-fetches telemetry so a guest IP appears shortly
// after the guest network comes up.
const TELEMETRY_POLL_MS = 8000;

function scancodesFor(code: string, down: boolean): number[] | null {
  const entry = SCANCODES[code];
  if (!entry) return null;
  const value = down ? entry.code : entry.code | 0x80;
  return entry.ext ? [0xe0, value] : [value];
}

type ConnState = 'connecting' | 'live' | 'closed' | 'error';

interface ScreenConsoleProps {
  vmId: string;
  vmName: string;
  onClose: () => void;
  // When true the console fills the whole viewport edge-to-edge (dedicated tab)
  // instead of floating as a centered modal over the dashboard.
  fullscreen?: boolean;
}

export function ScreenConsole({ vmId, vmName, onClose, fullscreen = false }: ScreenConsoleProps) {
  const { t, tf, ts } = useT();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const screenBoxRef = useRef<HTMLDivElement>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const [conn, setConn] = useState<ConnState>('connecting');
  const [dims, setDims] = useState<{ w: number; h: number }>({ w: 0, h: 0 });
  const [stats, setStats] = useState<string>('');
  const [errorText, setErrorText] = useState<string>('');
  const [telemetry, setTelemetry] = useState<VmTelemetryResponse | null>(null);
  const [clipboardMode, setClipboardMode] = useState<string | null>(null);
  const [clipboardBusy, setClipboardBusy] = useState(false);
  const [railOpen, setRailOpen] = useState(true);

  // Telemetry (configured CPU/RAM and guest-reported network IPs) is auxiliary
  // to the stream, so it is fetched independently and failures are ignored. It
  // is polled because a guest IP only appears once the guest network is up,
  // which can be seconds after the console opens.
  useEffect(() => {
    let cancelled = false;
    const load = () =>
      api
        .getVmTelemetry(vmId)
        .then((t) => {
          if (!cancelled) setTelemetry(t);
        })
        .catch(() => {
          /* telemetry is best-effort; the console still works without it */
        });

    load();
    const interval = setInterval(load, TELEMETRY_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [vmId]);

  // Load the VM's current shared-clipboard mode so the control reflects reality.
  useEffect(() => {
    let cancelled = false;
    api
      .getClipboardMode(vmId)
      .then((r) => {
        if (!cancelled) setClipboardMode(r.mode);
      })
      .catch(() => {
        /* clipboard control is optional; hide it if unavailable */
      });
    return () => {
      cancelled = true;
    };
  }, [vmId]);

  const changeClipboard = useCallback(
    (mode: string) => {
      setClipboardBusy(true);
      api
        .setClipboardMode(vmId, mode)
        .then((r) => setClipboardMode(r.mode))
        .catch(() => {
          /* leave the previous mode selected on failure */
        })
        .finally(() => setClipboardBusy(false));
    },
    [vmId],
  );

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const socket = new WebSocket(screenStreamUrl(vmId));
    socket.binaryType = 'arraybuffer';
    socketRef.current = socket;

    let frames = 0;
    let bytes = 0;
    let lastTick = performance.now();

    socket.addEventListener('open', () => setConn('live'));
    socket.addEventListener('error', () => {
      setConn('error');
      setErrorText('Connection failed. Is the VM running?');
    });
    socket.addEventListener('close', () => setConn((c) => (c === 'error' ? c : 'closed')));

    socket.addEventListener('message', async (evt) => {
      if (typeof evt.data === 'string') {
        try {
          const msg = JSON.parse(evt.data);
          if (msg.type === 'resolution') {
            canvas.width = msg.width;
            canvas.height = msg.height;
            setDims({ w: msg.width, h: msg.height });
          }
        } catch {
          // ignore malformed control frames
        }
        return;
      }
      const buf = evt.data as ArrayBuffer;
      frames++;
      bytes += buf.byteLength;
      try {
        await paintTiles(ctx, buf);
      } catch {
        // a single bad tile batch should not tear down the stream
      }
      const now = performance.now();
      const dt = now - lastTick;
      if (dt >= 1000) {
        setStats(`${Math.round((frames * 1000) / dt)} fps · ${Math.round((bytes * 1000) / dt / 1024)} KB/s`);
        frames = 0;
        bytes = 0;
        lastTick = now;
      }
    });

    return () => {
      socketRef.current = null;
      socket.close();
    };
  }, [vmId]);

  const sendJSON = useCallback((obj: unknown) => {
    const socket = socketRef.current;
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(obj));
    }
  }, []);

  // Ask the guest to match the console viewport so the stream fills it with no
  // letterboxing. Honored only when Guest Additions is running; otherwise it is
  // a no-op and object-fit: contain keeps the image fitted. Debounced, and it
  // observes the box (not the canvas) so the guest's own resize does not loop.
  useEffect(() => {
    const box = screenBoxRef.current;
    if (!box || conn !== 'live') return;
    let timer: number | undefined;
    const push = () => {
      const w = Math.round(box.clientWidth);
      const h = Math.round(box.clientHeight);
      if (w > 0 && h > 0) sendJSON({ type: 'resize', width: w, height: h });
    };
    const schedule = () => {
      window.clearTimeout(timer);
      timer = window.setTimeout(push, 400);
    };
    const observer = new ResizeObserver(schedule);
    observer.observe(box);
    return () => {
      window.clearTimeout(timer);
      observer.disconnect();
    };
  }, [conn, sendJSON]);

  const guestCoords = useCallback((e: React.MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current!;
    const rect = canvas.getBoundingClientRect();
    // The canvas element fills its container and the guest bitmap is drawn with
    // object-fit: contain, so the visible image is letterboxed inside the box.
    // Map the pointer through that same contain transform, otherwise clicks are
    // offset (and land in the black margins) on any non-matching aspect ratio.
    const scale = Math.min(rect.width / canvas.width, rect.height / canvas.height);
    const dispW = canvas.width * scale;
    const dispH = canvas.height * scale;
    const offX = (rect.width - dispW) / 2;
    const offY = (rect.height - dispH) / 2;
    const clamp = (v: number, max: number) => Math.max(0, Math.min(max, v));
    return {
      x: Math.round(clamp((e.clientX - rect.left - offX) / scale, canvas.width)),
      y: Math.round(clamp((e.clientY - rect.top - offY) / scale, canvas.height)),
    };
  }, []);

  const sendMouse = useCallback(
    (e: React.MouseEvent<HTMLCanvasElement>, dz = 0) => {
      const { x, y } = guestCoords(e);
      sendJSON({ type: 'mouse', x, y, buttons: e.buttons, dz, dw: 0 });
    },
    [guestCoords, sendJSON],
  );

  // Throttle pointer-move to ~60Hz. A raw mousemove fires hundreds of events per
  // second, each a synchronous input injection on the guest's single COM thread
  // that competes with frame capture — the flood is what makes control feel like
  // it freezes. Presses/releases/wheel below stay immediate for accuracy.
  const lastMoveRef = useRef(0);
  const onMouseMove = useCallback(
    (e: React.MouseEvent<HTMLCanvasElement>) => {
      const now = performance.now();
      if (now - lastMoveRef.current < 16) return;
      lastMoveRef.current = now;
      sendMouse(e);
    },
    [sendMouse],
  );

  const handleKey = useCallback(
    (e: React.KeyboardEvent<HTMLCanvasElement>, down: boolean) => {
      const scancodes = scancodesFor(e.code, down);
      if (!scancodes) return;
      e.preventDefault();
      sendJSON({ type: 'key', scancodes });
    },
    [sendJSON],
  );

  const sendCtrlAltDel = useCallback(() => {
    // Ctrl(0x1d) + Alt(0x38) + Delete(E0 53) make, then break in reverse.
    sendJSON({ type: 'key', scancodes: [0x1d, 0x38, 0xe0, 0x53] });
    sendJSON({ type: 'key', scancodes: [0xe0, 0xd3, 0xb8, 0x9d] });
  }, [sendJSON]);

  const screenCanvas = (
    <canvas
      ref={canvasRef}
      width={800}
      height={600}
      tabIndex={0}
      className="console-canvas"
      onMouseMove={onMouseMove}
      onMouseDown={(e) => {
        e.currentTarget.focus();
        sendMouse(e);
      }}
      onMouseUp={(e) => sendMouse(e)}
      onContextMenu={(e) => e.preventDefault()}
      onWheel={(e) => sendMouse(e, e.deltaY > 0 ? -1 : 1)}
      onKeyDown={(e) => handleKey(e, true)}
      onKeyUp={(e) => handleKey(e, false)}
    />
  );

  // The guest's first reported IPv4 (if any) is surfaced in the telemetry rail.
  const guestIp = telemetry?.networks.flatMap((n) => n.ipv4)[0];
  const disk = telemetry?.disks[0];
  // Hover toolbar: a small info pill plus Ctrl+Alt+Del and close, hidden until
  // the pointer enters the screen (see .console-toolbar CSS). The pill shows the
  // resolution once live (plus fps/throughput in the embedded console only, not
  // the distraction-free new-tab view); only while connecting/erroring does it
  // name the state — there is no persistent "live/connected" badge anywhere else.
  const toolbar = (
    <div className="console-toolbar">
      {/* Embedded: resolution (+fps/throughput) when live. New-tab: no info pill
          at all once live — only the connecting/error state is worth showing. */}
      {!(fullscreen && conn === 'live') && (
        <span className="console-pill">
          {conn === 'live' ? (
            <>
              {dims.w > 0 && (
                <span>
                  {dims.w}×{dims.h}
                </span>
              )}
              {stats && !fullscreen && (
                <>
                  {dims.w > 0 && <span className="sep">·</span>}
                  {stats}
                </>
              )}
            </>
          ) : (
            <span className="muted">{conn === 'connecting' ? t('connecting…') : conn}</span>
          )}
        </span>
      )}
      <button type="button" className="console-tbtn" onClick={sendCtrlAltDel} title={t('Send Ctrl+Alt+Del')}>
        Ctrl+Alt+Del
      </button>
      <button
        type="button"
        className="console-tbtn danger"
        onClick={onClose}
        title={t('Close console')}
        aria-label={t('Close console')}
      >
        <svg viewBox="0 0 24 24" fill="none" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
          <path d="M6 6l12 12M18 6L6 18" />
        </svg>
      </button>
    </div>
  );

  // Collapsible telemetry rail.
  const rail = (
    <aside className="console-rail">
      <div className="console-rail-h">{t('Telemetry')}</div>
      <span className={`console-ga-pill ${telemetry?.guestAdditions ? 'on' : 'off'}`}>
        {telemetry?.guestAdditions ? t('● Guest Additions active') : t('Guest Additions not detected')}
      </span>
      {guestIp && (
        <div className="console-metric ip">
          <span className="lab">IP</span>
          <span className="val">{guestIp}</span>
        </div>
      )}
      <div className="console-rail-div" />
      <div className="console-metric">
        <span className="lab">CPU</span>
        <span className="val">
          {telemetry?.cpuCount ?? '—'} <small>vCPU</small>
        </span>
      </div>
      <div className="console-metric">
        <span className="lab">{t('Memory')}</span>
        <span className="val">{telemetry ? formatRam(telemetry.ramMb) : '—'}</span>
      </div>
      {disk && (
        <div className="console-metric">
          <span className="lab">{t('Disk')}</span>
          <span className="val">
            {formatBytes(disk.allocatedBytes)} <small>/ {formatBytes(disk.capacityBytes)}</small>
          </span>
        </div>
      )}
      {clipboardMode !== null && (
        <>
          <div className="console-rail-div" />
          <label className="console-metric">
            <span className="lab">{t('Clipboard')}</span>
            <select
              className="console-clip-select"
              aria-label={t('Shared clipboard mode')}
              value={clipboardMode}
              disabled={clipboardBusy}
              onChange={(e) => changeClipboard(e.target.value)}
            >
              <option value="disabled">{t('off')}</option>
              <option value="bidirectional">{t('bidirectional')}</option>
              <option value="hosttoguest">{t('host → guest')}</option>
              <option value="guesttohost">{t('guest → host')}</option>
            </select>
          </label>
        </>
      )}
    </aside>
  );

  const screen = (
    <div ref={screenBoxRef} className="console-screen">
      {toolbar}
      {conn === 'error' && <div className="console-message error-state">{ts(errorText)}</div>}
      {screenCanvas}
    </div>
  );

  // Files dropped onto the screen are sent into the guest (hybrid transfer).
  const droppableScreen = (
    <GuestDropZone vmId={vmId} vmName={vmName} className="console-dropzone">
      {screen}
    </GuestDropZone>
  );

  // Dedicated-tab mode: fills the whole browser tab, screen-first, same hover
  // toolbar, no telemetry rail (distraction-free).
  if (fullscreen) {
    return (
      <div className="console-overlay console-overlay--tab" role="dialog" aria-label={tf('Console for {vm}', { vm: vmName })}>
        <div className="console-stage console-stage--tab">{droppableScreen}</div>
      </div>
    );
  }

  return (
    <div className="console-overlay" role="dialog" aria-label={tf('Console for {vm}', { vm: vmName })}>
      <div className={`console-window ${railOpen ? '' : 'railclosed'}`}>
        <div className="console-chrome">
          <span className="console-dots">
            <i />
            <i />
            <i />
          </span>
          <span className="console-chrome-title">
            <span className="console-tabmark" /> {vmName} — TabVM console
          </span>
        </div>
        <div className="console-stage">
          {droppableScreen}
          <button
            type="button"
            className="console-railtoggle"
            onClick={() => setRailOpen((o) => !o)}
            title={railOpen ? t('Collapse panel') : t('Expand panel')}
            aria-label={t('Toggle telemetry panel')}
          >
            <svg viewBox="0 0 24 24" fill="none" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M15 6l-6 6 6 6" />
            </svg>
          </button>
          {rail}
        </div>
      </div>
    </div>
  );
}
