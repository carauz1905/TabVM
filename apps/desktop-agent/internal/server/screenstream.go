package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tabvm/desktop-agent/internal/vmscreen"
)

// screenInputMsg is a client->server input event over the screen-stream
// WebSocket. "mouse" carries absolute guest pixel coordinates plus a button
// bitmask and optional wheel deltas; "key" carries PC/AT set-1 scancodes
// (make/break, extended keys pre-split with their 0xE0 prefix).
type screenInputMsg struct {
	Type      string `json:"type"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Buttons   int    `json:"buttons"`
	DZ        int    `json:"dz"`
	DW        int    `json:"dw"`
	Scancodes []int  `json:"scancodes"`
	// Width/Height carry the desired guest resolution for a "resize" message:
	// the client asks the guest to match the console viewport. Honored only when
	// Guest Additions is running (otherwise a silent no-op).
	Width  int `json:"width"`
	Height int `json:"height"`
}

// handleScreenInput parses one input message and injects it into the guest.
// Malformed messages and injection errors are logged and swallowed so a bad
// event never tears down the stream.
func (s *Server) handleScreenInput(id, vboxManage string, capturer *vmscreen.Capturer, data []byte) {
	var msg screenInputMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	switch msg.Type {
	case "mouse":
		if err := capturer.SendMouseAbsolute(msg.X, msg.Y, msg.Buttons, msg.DZ, msg.DW); err != nil {
			s.logger.Debug("screen stream: mouse inject failed", "vmId", id, "error", err)
		}
	case "key":
		if len(msg.Scancodes) == 0 {
			return
		}
		if err := capturer.SendScancodes(msg.Scancodes); err != nil {
			s.logger.Debug("screen stream: key inject failed", "vmId", id, "error", err)
		}
	case "resize":
		// Ask the guest to match the console viewport. No-op without Guest
		// Additions; when honored, the resolution poll picks up the change and
		// re-announces so the canvas fills with no letterboxing.
		if err := vmscreen.SetVideoModeHint(vboxManage, id, msg.Width, msg.Height); err != nil {
			s.logger.Debug("screen stream: resize hint failed", "vmId", id, "error", err)
		}
	}
}

// Screen-stream tuning. The capture itself is ~15-20ms/frame (see vmscreen);
// the frame interval below bounds the delivered frame rate and the JPEG
// quality trades bandwidth for fidelity. These are deliberately conservative
// defaults for a first end-to-end slice; tile-diffing (send only changed
// regions) is the planned next optimization.
// screenStreamTokenQueryParam is the query-string parameter carrying the
// session token on the screen-stream WebSocket route. Browsers' native
// WebSocket API cannot set request headers, so the token cannot travel in
// X-TabVM-Session-Token for this route; it is resolved and compared exactly
// like the header, only the transport differs (see withAuth).
const screenStreamTokenQueryParam = "token"

// screenStreamUpgrader upgrades screen-stream HTTP requests to WebSockets.
var screenStreamUpgrader = websocket.Upgrader{
	ReadBufferSize:  64 * 1024,
	WriteBufferSize: 64 * 1024,
	// The spike test page (apps/web-ui/spike/screen-test.html) is served from
	// the Vite dev server on a different origin than the agent, so origin
	// checking is relaxed. Tighten this (or serve same-origin) before this
	// leaves spike status.
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	// The stream runs over loopback (127.0.0.1), so bandwidth is effectively
	// free and these favor fidelity and smoothness over byte count. Capture is
	// ~15-20ms/frame (see vmscreen), leaving headroom under a ~30ms tick.
	screenFrameInterval = 30 * time.Millisecond // ~33 fps ceiling
	screenJPEGQuality   = 90                     // sharper text/edges; loopback pays no bandwidth cost
	// screenResolutionPoll is how often the stream re-checks the guest's native
	// resolution so it can follow a mode change (e.g. after Guest Additions
	// loads) without the user reopening the console.
	screenResolutionPoll = 3 * time.Second
	// screenTileSize is the width/height of each diff tile. Only tiles whose
	// pixels changed since the previous frame are re-encoded and sent, so a
	// mostly-static desktop costs almost nothing while a full-screen change
	// costs about the same as a whole-frame send.
	screenTileSize = 128
)

// isScreenStreamPath reports whether path is the /api-stripped screen-stream
// WebSocket route ("/vms/{id}/screen-stream"). Used to scope the query-param
// auth fallback in withAuth to this WebSocket route.
func isScreenStreamPath(path string) bool {
	if !strings.HasSuffix(path, "/screen-stream") {
		return false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 3 && parts[0] == "vms" && parts[2] == "screen-stream"
}

// resolveVBoxManagePath returns the first configured VBoxManage path that
// exists on disk, falling back to the well-known install locations.
func (s *Server) resolveVBoxManagePath() (string, error) {
	candidates := append([]string{}, s.cfg.VBoxManagePaths...)
	candidates = append(candidates,
		`C:\Program Files\Oracle\VirtualBox\VBoxManage.exe`,
		`C:\Program Files (x86)\Oracle\VirtualBox\VBoxManage.exe`,
	)
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("VBoxManage.exe not found in configured paths")
}

// handleVmScreenStream upgrades the request to a WebSocket and streams the
// VM's screen as JPEG frames captured through the VirtualBox COM API. The
// first message is a JSON text frame describing the resolution; every
// subsequent message is a binary JPEG frame.
func (s *Server) handleVmScreenStream(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vboxManage, err := s.resolveVBoxManagePath()
	if err != nil {
		s.logger.Error("screen stream: cannot locate VBoxManage", "vmId", id, "error", err)
		http.Error(w, "VirtualBox is not available.", http.StatusBadGateway)
		return
	}

	// Build the capturer before upgrading so a failure (e.g. VM not running)
	// surfaces as a clean HTTP error instead of a WebSocket close.
	capturer, err := vmscreen.New(vboxManage, id)
	if err != nil {
		s.logger.Error("screen stream: failed to start capturer", "vmId", id, "error", err)
		http.Error(w, "Could not capture the VM screen. Is the VM running?", http.StatusConflict)
		return
	}
	// The capturer is held in a swappable pointer: when the guest changes
	// resolution the loop replaces it with a fresh one at the new size. The
	// reader goroutine injects input into whichever capturer is current.
	var capHolder atomic.Pointer[vmscreen.Capturer]
	capHolder.Store(capturer)
	defer func() {
		if c := capHolder.Load(); c != nil {
			c.Close()
		}
	}()

	conn, err := screenStreamUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("screen stream: websocket upgrade failed", "vmId", id, "error", err)
		return
	}
	defer conn.Close()

	// announceResolution tells the client to (re)size its canvas. All websocket
	// writes happen on this goroutine, so it never races with frame writes.
	announceResolution := func(width, height int) error {
		return conn.WriteJSON(map[string]any{
			"type":   "resolution",
			"width":  width,
			"height": height,
		})
	}

	width, height := capturer.Size()
	s.logger.Info("screen stream: started", "vmId", id, "width", width, "height", height)
	if err := announceResolution(width, height); err != nil {
		return
	}

	// Reader goroutine: parses input events (keyboard/mouse) from the client
	// and injects them into the current capturer, and detects client-side close.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if msgType != websocket.TextMessage {
				continue
			}
			s.handleScreenInput(id, vboxManage, capHolder.Load(), data)
		}
	}()

	ticker := time.NewTicker(screenFrameInterval)
	defer ticker.Stop()
	resTicker := time.NewTicker(screenResolutionPoll)
	defer resTicker.Stop()

	var prev vmscreen.Frame
	msgBuf := new(bytes.Buffer)
	tileBuf := new(bytes.Buffer)
	// Debounce state for resolution changes: a candidate new size must be seen
	// on two consecutive polls before the capturer is swapped. Swapping is
	// disruptive (it briefly interrupts input), so a transient or misread size
	// must never trigger it.
	var pendW, pendH, pendStreak int
	for {
		select {
		case <-done:
			s.logger.Info("screen stream: client disconnected", "vmId", id)
			return
		case <-resTicker.C:
			nw, nh, derr := vmscreen.DetectResolution(vboxManage, id)
			cur := capHolder.Load()
			cw, ch := cur.Size()
			if derr != nil || nw <= 0 || nh <= 0 || (nw == cw && nh == ch) {
				pendStreak = 0
				continue
			}
			if nw == pendW && nh == pendH {
				pendStreak++
			} else {
				pendW, pendH, pendStreak = nw, nh, 1
			}
			if pendStreak < 2 {
				continue // wait for a stable second reading before swapping
			}
			pendStreak = 0
			newCap, nerr := vmscreen.New(vboxManage, id)
			if nerr != nil {
				s.logger.Debug("screen stream: could not re-open capturer after resolution change", "vmId", id, "error", nerr)
				continue
			}
			capHolder.Swap(newCap).Close()
			prev = vmscreen.Frame{} // force a full repaint at the new size
			gw, gh := newCap.Size()
			s.logger.Info("screen stream: resolution changed", "vmId", id, "width", gw, "height", gh)
			if err := announceResolution(gw, gh); err != nil {
				return
			}
		case <-ticker.C:
			frame, err := capHolder.Load().Grab()
			if err != nil {
				s.logger.Error("screen stream: grab failed", "vmId", id, "error", err)
				return
			}

			count, err := encodeChangedTiles(prev, frame, msgBuf, tileBuf)
			if err != nil {
				s.logger.Error("screen stream: tile encode failed", "vmId", id, "error", err)
				return
			}
			prev = frame
			if count == 0 {
				continue // nothing changed; keep the client's canvas as-is
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, msgBuf.Bytes()); err != nil {
				return
			}
		}
	}
}

// encodeChangedTiles compares cur against prev tile by tile and writes only the
// changed tiles into msgBuf using this wire format (all integers big-endian):
//
//	uint16 tileCount
//	repeat tileCount times:
//	  uint16 x, uint16 y, uint16 w, uint16 h, uint32 jpegLen, jpegLen bytes
//
// When prev is empty or a different size, every tile is treated as changed
// (full first paint). tileBuf is a reusable scratch buffer for per-tile JPEG
// encoding. It returns the number of tiles written.
func encodeChangedTiles(prev, cur vmscreen.Frame, msgBuf, tileBuf *bytes.Buffer) (int, error) {
	w, h := cur.Width, cur.Height
	stride := w * 4
	full := &image.RGBA{Pix: cur.Pix, Stride: stride, Rect: image.Rect(0, 0, w, h)}
	sameSize := len(prev.Pix) == len(cur.Pix) && prev.Width == w && prev.Height == h

	msgBuf.Reset()
	var count uint16
	// Reserve two bytes for the count, patched after the loop.
	msgBuf.Write([]byte{0, 0})

	var hdr [12]byte
	for ty := 0; ty < h; ty += screenTileSize {
		th := screenTileSize
		if ty+th > h {
			th = h - ty
		}
		for tx := 0; tx < w; tx += screenTileSize {
			tw := screenTileSize
			if tx+tw > w {
				tw = w - tx
			}
			if sameSize && !tileChanged(prev.Pix, cur.Pix, stride, tx, ty, tw, th) {
				continue
			}

			tileBuf.Reset()
			sub := full.SubImage(image.Rect(tx, ty, tx+tw, ty+th)).(*image.RGBA)
			if err := jpeg.Encode(tileBuf, sub, &jpeg.Options{Quality: screenJPEGQuality}); err != nil {
				return 0, err
			}

			binary.BigEndian.PutUint16(hdr[0:2], uint16(tx))
			binary.BigEndian.PutUint16(hdr[2:4], uint16(ty))
			binary.BigEndian.PutUint16(hdr[4:6], uint16(tw))
			binary.BigEndian.PutUint16(hdr[6:8], uint16(th))
			binary.BigEndian.PutUint32(hdr[8:12], uint32(tileBuf.Len()))
			msgBuf.Write(hdr[:])
			msgBuf.Write(tileBuf.Bytes())
			count++
		}
	}

	binary.BigEndian.PutUint16(msgBuf.Bytes()[0:2], count)
	return int(count), nil
}

// tileChanged reports whether any pixel in the tile at (tx,ty) of size (tw,th)
// differs between prev and cur, comparing row spans directly.
func tileChanged(prev, cur []byte, stride, tx, ty, tw, th int) bool {
	rowBytes := tw * 4
	for r := 0; r < th; r++ {
		off := (ty+r)*stride + tx*4
		if !bytes.Equal(prev[off:off+rowBytes], cur[off:off+rowBytes]) {
			return true
		}
	}
	return false
}
