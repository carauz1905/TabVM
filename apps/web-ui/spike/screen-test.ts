// SPIKE — validates the VirtualBox COM screen-stream path end to end in the
// browser (replaces the abandoned IronRDP/VRDE spike). It opens the
// /api/vms/{id}/screen-stream WebSocket on the desktop-agent, sizes the canvas
// from the first (JSON) resolution message, and paints each subsequent binary
// JPEG frame onto the canvas. Not production UI.

const logEl = document.getElementById('log') as HTMLPreElement;
const statsEl = document.getElementById('stats') as HTMLDivElement;
const agentBaseUrlInput = document.getElementById('agentBaseUrl') as HTMLInputElement;
const vmIdInput = document.getElementById('vmId') as HTMLInputElement;
const sessionTokenInput = document.getElementById('sessionToken') as HTMLInputElement;
const canvas = document.getElementById('screen') as HTMLCanvasElement;
const ctx = canvas.getContext('2d')!;

let socket: WebSocket | null = null;
let frames = 0;
let bytes = 0;
let lastTick = performance.now();

function log(message: string): void {
  const line = `[${new Date().toISOString()}] ${message}`;
  console.log(line);
  logEl.textContent = `${logEl.textContent}\n${line}`;
}

function wsUrl(httpBaseUrl: string, vmId: string, token: string): string {
  const wsBase = httpBaseUrl.replace(/^http/i, 'ws').replace(/\/$/, '');
  const query = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${wsBase}/api/vms/${encodeURIComponent(vmId)}/screen-stream${query}`;
}

// Wire format (big-endian): uint16 tileCount, then per tile
// uint16 x, uint16 y, uint16 w, uint16 h, uint32 jpegLen, jpegLen bytes.
async function paintTiles(data: ArrayBuffer): Promise<void> {
  const view = new DataView(data);
  const count = view.getUint16(0, false);
  let off = 2;
  const jobs: Promise<void>[] = [];
  for (let i = 0; i < count; i++) {
    const x = view.getUint16(off, false);
    const y = view.getUint16(off + 2, false);
    const w = view.getUint16(off + 4, false);
    const h = view.getUint16(off + 6, false);
    const len = view.getUint32(off + 8, false);
    const jpegStart = off + 12;
    const jpegBytes = new Uint8Array(data, jpegStart, len);
    off = jpegStart + len;

    const blob = new Blob([jpegBytes], { type: 'image/jpeg' });
    jobs.push(
      createImageBitmap(blob).then((bmp) => {
        ctx.drawImage(bmp, x, y, w, h);
        bmp.close();
      }),
    );
  }
  await Promise.all(jobs);
}

function updateStats(): void {
  const now = performance.now();
  const dt = now - lastTick;
  if (dt >= 1000) {
    const fps = (frames * 1000) / dt;
    const kbps = (bytes * 1000) / dt / 1024;
    statsEl.textContent = `${fps.toFixed(1)} fps · ${kbps.toFixed(0)} KB/s · ${canvas.width}x${canvas.height}`;
    frames = 0;
    bytes = 0;
    lastTick = now;
  }
}

function disconnect(): void {
  if (socket) {
    socket.close();
    socket = null;
  }
}

document.getElementById('btnConnect')?.addEventListener('click', () => {
  disconnect();
  const url = wsUrl(agentBaseUrlInput.value, vmIdInput.value, sessionTokenInput.value);
  log(`Connecting to ${url} ...`);
  socket = new WebSocket(url);
  socket.binaryType = 'arraybuffer';

  socket.addEventListener('open', () => log('WebSocket open; awaiting resolution + frames.'));
  socket.addEventListener('error', () => log('WebSocket error (see devtools Network tab).'));
  socket.addEventListener('close', (evt) => log(`WebSocket closed code=${evt.code} reason=${evt.reason || '(none)'}`));

  socket.addEventListener('message', async (evt) => {
    if (typeof evt.data === 'string') {
      const msg = JSON.parse(evt.data);
      if (msg.type === 'resolution') {
        canvas.width = msg.width;
        canvas.height = msg.height;
        log(`Resolution: ${msg.width}x${msg.height}. Canvas resized.`);
      }
      return;
    }
    const buf = evt.data as ArrayBuffer;
    frames++;
    bytes += buf.byteLength;
    try {
      await paintTiles(buf);
    } catch (err) {
      log(`paint failed: ${String(err)}`);
    }
    updateStats();
  });
});

document.getElementById('btnDisconnect')?.addEventListener('click', () => {
  disconnect();
  log('Disconnected.');
});

// --- Input: mouse + keyboard -> guest via COM (server side). ---

function sendJSON(obj: unknown): void {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(obj));
  }
}

// Browser MouseEvent.buttons bitmask (1=left,2=right,4=middle) matches the
// VirtualBox IMouse buttonState bitmask directly.
function guestCoords(e: MouseEvent): { x: number; y: number } {
  const rect = canvas.getBoundingClientRect();
  const x = Math.round(((e.clientX - rect.left) / rect.width) * canvas.width);
  const y = Math.round(((e.clientY - rect.top) / rect.height) * canvas.height);
  return { x, y };
}

function sendMouse(e: MouseEvent, dz = 0): void {
  const { x, y } = guestCoords(e);
  sendJSON({ type: 'mouse', x, y, buttons: e.buttons, dz, dw: 0 });
}

canvas.addEventListener('mousemove', (e) => sendMouse(e));
canvas.addEventListener('mousedown', (e) => { canvas.focus(); sendMouse(e); });
canvas.addEventListener('mouseup', (e) => sendMouse(e));
canvas.addEventListener('contextmenu', (e) => e.preventDefault());
canvas.addEventListener('wheel', (e) => {
  e.preventDefault();
  sendMouse(e, e.deltaY > 0 ? -1 : 1);
}, { passive: false });
canvas.tabIndex = 0; // make the canvas focusable for keyboard events

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

function scancodesFor(code: string, down: boolean): number[] | null {
  const entry = SCANCODES[code];
  if (!entry) return null;
  const value = down ? entry.code : entry.code | 0x80;
  return entry.ext ? [0xe0, value] : [value];
}

function handleKey(e: KeyboardEvent, down: boolean): void {
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  const scancodes = scancodesFor(e.code, down);
  if (!scancodes) return;
  e.preventDefault();
  sendJSON({ type: 'key', scancodes });
}

canvas.addEventListener('keydown', (e) => handleKey(e, true));
canvas.addEventListener('keyup', (e) => handleKey(e, false));

log('Page loaded. Fill in the token, then click Connect & stream. Click the canvas to give it keyboard focus.');

export {};
