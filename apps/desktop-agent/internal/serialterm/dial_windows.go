//go:build windows

// Package serialterm bridges a VM's COM1 serial port, exposed by VirtualBox as a
// host named pipe, to an io.ReadWriteCloser the agent can pump over a WebSocket.
package serialterm

import (
	"io"
	"time"

	winio "github.com/Microsoft/go-winio"
)

// dialTimeout bounds how long we wait for the VirtualBox-created pipe to appear.
// VBox creates the server pipe when the VM starts; if the VM is not running (or
// the serial console is not enabled) the dial fails fast and the caller reports
// a clean error instead of hanging.
const dialTimeout = 5 * time.Second

// Dial connects to the VirtualBox serial named pipe (server mode) and returns a
// read/write stream to the guest's COM1. VirtualBox owns the pipe as server; the
// agent connects as client.
func Dial(pipe string) (io.ReadWriteCloser, error) {
	timeout := dialTimeout
	return winio.DialPipe(pipe, &timeout)
}
