package server

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/tabvm/desktop-agent/internal/serialterm"
	"github.com/tabvm/desktop-agent/internal/vbox"
)

// isSerialStreamPath reports whether path is the /api-stripped serial-stream
// WebSocket route ("/vms/{id}/serial-stream"). Used to scope the query-param
// auth fallback in withAuth to this WebSocket route (browsers cannot set headers
// on a WebSocket upgrade).
func isSerialStreamPath(path string) bool {
	if !strings.HasSuffix(path, "/serial-stream") {
		return false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 3 && parts[0] == "vms" && parts[2] == "serial-stream"
}

// serialReadBufferSize bounds a single guest->client read. A login shell emits
// small bursts, so a modest buffer keeps latency low without fragmenting.
const serialReadBufferSize = 4096

// handleVmSerialStream upgrades the request to a WebSocket and bridges it to the
// VM's COM1 serial port (exposed by VirtualBox as a host named pipe). Bytes read
// from the pipe are forwarded to the browser as binary frames; binary/text
// frames from the browser (keystrokes) are written back into the pipe. The
// terminal emulation (xterm.js) lives entirely in the browser.
func (s *Server) handleVmSerialStream(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Dial before upgrading so a failure (serial not enabled, or VM not running,
	// so the pipe does not exist) surfaces as a clean HTTP error rather than a
	// WebSocket close.
	pipe, err := serialterm.Dial(vbox.SerialPipeName(id))
	if err != nil {
		s.logger.Error("serial stream: cannot open pipe", "vmId", id, "error", err)
		http.Error(w, "Serial terminal is not available. Enable it and start the VM.", http.StatusConflict)
		return
	}
	defer pipe.Close()

	conn, err := screenStreamUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("serial stream: websocket upgrade failed", "vmId", id, "error", err)
		return
	}
	defer conn.Close()

	s.logger.Info("serial stream: started", "vmId", id)

	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }

	// guest -> client: read raw bytes from the serial pipe and forward them as
	// binary WebSocket frames. This is the only goroutine that writes to conn,
	// so gorilla's single-writer constraint is respected.
	go func() {
		buf := make([]byte, serialReadBufferSize)
		for {
			n, rerr := pipe.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					stop()
					return
				}
			}
			if rerr != nil {
				stop()
				return
			}
		}
	}()

	// client -> guest: read keystroke frames from the browser and write them
	// into the serial pipe. This is the only goroutine that reads from conn.
	go func() {
		for {
			_, data, rerr := conn.ReadMessage()
			if rerr != nil {
				stop()
				return
			}
			if len(data) > 0 {
				if _, werr := pipe.Write(data); werr != nil {
					stop()
					return
				}
			}
		}
	}()

	<-done
	s.logger.Info("serial stream: closed", "vmId", id)
}
